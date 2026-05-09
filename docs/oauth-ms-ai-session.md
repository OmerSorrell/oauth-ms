# OAuth Connector — AI Session Log

A narrative of the design and build session for the Vorlon "Universal OAuth Connector"
home assignment, intended as the AI-session reference the assignment guidelines ask for.
Decisions and trade-offs only — no code listings or file-by-file detail (the repo
itself is the source of truth for that).

## What we set out to build

A Go microservice that authenticates a user via an OAuth web flow, fetches data from a
third-party provider (GitHub first), enriches it with a mock internal service, and
stays extensible so new providers (Slack, Salesforce) drop in as their own package
without touching domain or use-case code.

User goals beyond the assignment spec:

- DI under `cmd/dependencies/` (the container-provider pattern from a prior production service the user owns).
- A cache layer ready for Redis but in-memory today, backing OAuth state, sessions, and per-user internal-service responses.
- Tilt + local k8s for the dev loop.
- Provider-as-package extensibility.
- Parallel fan-out anywhere two calls are independent.

## Session arc

### 1. Plan mode

I read the assignment PDF and the user's high-level architecture diagram, then proposed
an implementation plan covering:

- Hexagonal layout: `domain`, `port`, `usecase`, `adapter`, `transport`, plus a thin `cmd/`.
- Core abstractions — a bytes-level cache port, an OAuth-provider strategy, a registry, typed stores for state and sessions, an internal-service port.
- The OAuth flow with PKCE S256, a server-side single-use state record, and a paired short-lived state cookie.
- The `/profile` use case fanning out via `errgroup` so the provider call and the internal-service call run concurrently.
- A DI container with one `Provide()` per process and per-category factory files.
- Dev loop via Tilt + two k8s deployments (connector and mock internal service).

Three clarifying questions before leaving plan mode:

- Module path → `github.com/OmerSorrell/oauth-ms`.
- In-memory cache library → `jellydator/ttlcache/v3`.
- UI delivery → embedded via `go:embed` and served by the connector at `/`.

### 2. Skeleton build

Built top-down: domain types, port interfaces, adapters (cache, stores, internal-service
HTTP client and its caching decorator, GitHub provider, registry, crypto helpers), use
cases, chi transport with middleware, the DI container, the embedded UI, the standalone
mock service, Dockerfiles, k8s manifests, Tiltfile, Makefile, README, and a base unit
test suite.

End-to-end smoke verified the happy paths — health, the GitHub authorize redirect
(state + S256 challenge + scopes + redirect_uri), the unknown-provider 404, the
session-required 401 on `/profile`, and the embedded UI at `/`.

### 3. Naming corrections during build

A few directory and package renames to avoid stdlib clashes:

- The transport package (`internal/transport/http/`) became `transport/httpapi/` — the package name `http` collides with `net/http`.
- The internal-service HTTP adapter (`adapter/internalsvc/http/`) became `internalsvc/httpclient/` for the same reason.
- The two cached store packages (`store/session/cached/` and `store/state/cached/`) collapsed into a single `store/cached/`.

Same architecture, simpler imports.

### 4. Dependency hygiene

Audited what we had pulled in:

- Direct deps were all from established orgs (chi, ttlcache, viper, golang.org/x/sync).
- Viper drags ~10 indirect dependencies (the spf13 / sagikazarmark / sourcegraph cluster) — flagged as a trim opportunity.
- Installed `govulncheck` and scanned. Two stdlib vulnerabilities were reachable from the connector code, both fixed in Go 1.26.3 (we are on 1.26.2). Six other findings were in transitive modules with no reachable call paths from our code.

Discussed config-library alternatives:

- **viper** — works, heavyweight transitive closure.
- **koanf** — designed for layered config, smaller per-package, modular providers.
- **alecthomas/kong** — primarily a CLI parser; ideal if the service ever grows subcommands, but an awkward shape for a flag-less HTTP service.
- **hand-rolled** — smallest, but you own the env-overlay logic forever.

The user picked koanf. The honest scorecard after the swap:

- Total module count actually nudged from 16 to 17, because koanf publishes each provider as its own go module.
- The win is qualitative: the spf13 / sagikazarmark / sourcegraph mix viper dragged is gone, all koanf packages are owned by one author (single audit surface), and the env-name → key mapping is now explicit instead of relying on AutomaticEnv reflection.

The swap also caught a real bug worth knowing about: koanf's `env/v2.Provider` passes
the **full** environment-variable name to `TransformFunc` — the `Prefix` option only
filters which vars are collected, it does **not** strip them. My initial transform
assumed prefix-stripping. Verified against koanf's source, fixed, and locked it down
with a unit test covering every known env key plus an unknown-section drop case so it
cannot silently regress.

A final smoke test confirmed the env path end-to-end: setting `OAUTH_MS_AUTH_STATE_TTL=7m`
produced the expected `Set-Cookie: oauth_state=…; Max-Age=420`.

## Decisions worth remembering

| Decision | Why |
|---|---|
| Hexagonal layout with a strict `port` boundary | Provider strategy means new provider = new package; no domain or use-case edits. |
| `port.Cache` as a bytes-level KV | Typed wrappers (StateStore, SessionStore, cached InternalService) sit on top; a Redis adapter drops in without touching them. |
| Capture provider Identity at callback time | Lets `/profile` fan out without a serial `GetIdentity` round trip. |
| `errgroup` in `/profile` | One branch's error cancels the other; `singleflight` on the cached internal service collapses concurrent identical calls. |
| State-cookie + server-side single-use state record | Defense-in-depth CSRF: even if the cookie leaks, replay fails after first `Consume`. |
| Server-side session with random ID in cookie only | The browser never holds the access token. |
| `cmd/dependencies/` container-provider DI | Matches the user's existing convention; thin `main.go`, no DI framework, no codegen, no reflection. |
| Embedded UI (`go:embed`) | One process, one origin, no CORS, one fewer Tilt resource. |
| koanf over viper | Cleaner authorship, explicit env mapping, fewer long-tail transitive deps. |

## Trade-offs we explicitly chose against

- **wire / fx for DI** — manual container-provider was already the user's convention and scales fine at this size; no codegen step, no hidden lifecycle.
- **Separate static service for the UI** — adds CORS config and another Tilt target for what is essentially two files.
- **`tools.go` pinning** — overkill for a small repo; trivial to add when CI grows.
- **kong as the config library** — perfect if `oauth-ms` ever grows subcommands (`server`, `migrate`, `providers list`); overshoots a flag-less HTTP service.
- **Hand-rolled config** — possible, but defaults < yaml < env is exactly the shape koanf is for.

## Edge cases we discussed

- **State replay** — blocked by single-use `Consume`.
- **State / provider mismatch on callback** — typed error, surfaced as 400.
- **Internal service down** — `/profile` returns 502, error logged with provider/user context.
- **Internal user not found** — `/profile` returns 404 (no internal record for this provider user).
- **Concurrent `/profile` for the same user** — coalesced via `singleflight`.
- **Cookie loss between start and callback** — CSRF rejected (400).
- **Provider returning `error` on callback** — surfaced as 400 with the provider's message.

## Follow-ups left on the table

- Bump Go to **1.26.3** to clear the two stdlib vulnerabilities `govulncheck` flagged.
- Pagination over GitHub repos (today, first page only).
- A real Redis adapter behind `port.Cache` (the seam exists; the driver string is wired and intentionally stubbed).
- More integration tests around the auth handler (cookie semantics, error-mapping table).
- Switch to `alecthomas/kong` if and when CLI subcommands become a thing.

## On the AI assistance

The skeleton — directory layout, container wiring, strategy interface, parallel
`/profile` use case, security defaults — was generated through a structured
plan-then-build session with Claude. The high-level architecture diagram, the DI
convention, and the provider-as-package extensibility were the user's pre-stated
requirements; the model produced the implementation that satisfies them and surfaced
trade-offs (cache library, config library, package-naming collisions with stdlib
`net/http`) for explicit user decisions before committing to either path.
