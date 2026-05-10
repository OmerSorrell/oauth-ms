# Universal OAuth Connector

A Go microservice that authenticates users via a provider-agnostic OAuth web flow,
fetches data from the provider's API, and enriches it with data from a mock internal
service. Ships with GitHub today; new providers are added in their own package
without touching domain or use-case code.

## Architecture

```
        ┌────────────┐                          ┌──────────────┐
        │  Browser   │ ──── HTTPS / cookie ───▶ │              │     ┌────────────┐
        │   (UI)     │                          │  Connector   │ ──▶ │   GitHub   │
        └────────────┘                          │  service     │     │ OAuth+REST │
                                                │              │     └────────────┘
                                                │  chi HTTP    │
                                                │  use cases   │     ┌────────────────────┐
                                                │  registry +  │ ──▶ │ Internal service   │
                                                │  GitHub strat│     │ /api/users/{id}    │
                                                │  port.Cache  │     │ X-Internal-API-Key │
                                                └──────────────┘     └────────────────────┘
                                                  in-mem (Redis-ready)
```

- **chi** for HTTP routing.
- **knadh/koanf** for layered config (defaults < yaml < env).
- **`port.Cache`** — bytes-level KV; in-memory (`jellydator/ttlcache/v3`) today, Redis-ready (sibling adapter under `internal/adapter/cache/`).
- **StateStore / SessionStore / cached InternalService** all sit on top of `port.Cache`.
- **`/profile`** fans out the provider call and the internal-service call concurrently via `errgroup`; an error in either cancels the other.
- **DI** under `cmd/dependencies/` (container-provider pattern). `main.go` is ~30 lines.
- **UI** embedded with `go:embed`; one process, one origin, no CORS.

```
oauth-ms/
├── cmd/server/main.go
├── cmd/dependencies/        # DI container (one Provide per process)
├── internal/
│   ├── domain/              # plain types (auth, profile)
│   ├── port/                # interfaces only (cache, store, provider, internalsvc)
│   ├── usecase/             # auth + profile
│   ├── adapter/             # cache, stores, github, internal-service, crypto
│   └── transport/httpapi/   # chi router, handlers, middleware
├── mocks/internal-service/  # standalone Go binary; mock for /api/users/{id}
├── web/                     # embedded HTML/JS UI
└── deploy/                  # Dockerfile, k8s manifests, Tiltfile
```

## Quick start (local, no k8s)

```bash
# 1. Create a GitHub OAuth app:
#      Settings → Developer settings → OAuth Apps → New
#      Authorization callback URL: http://localhost:8080/auth/github/callback

# 2. Mock internal service in one terminal:
make run-mock     # listens on :8081, API key "super-secret-key"

# 3. Connector in another terminal:
export OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID=<your client id>
export OAUTH_MS_PROVIDERS_GITHUB_CLIENT_SECRET=<your client secret>
export OAUTH_MS_INTERNAL_API_KEY=super-secret-key
export OAUTH_MS_INTERNAL_BASE_URL=http://localhost:8081
make run

# 4. Visit http://localhost:8080
```

## Quick start (Tilt + local k8s)

```bash
cp deploy/k8s/connector-secret.example.yaml deploy/k8s/connector-secret.yaml
# Fill in OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID / _SECRET in that file.
make tilt-up
# Tilt port-forwards connector to :8080 and the mock service to :8081.

# Once Tilt shows both resources green, kick off the GitHub auth flow:
curl -i http://localhost:8080/auth/github
# -> 302 Location: https://github.com/login/oauth/authorize?...  (open in a browser to complete)

# After signing in via the browser, grab the session_id cookie from devtools and:
curl -s --cookie "session_id=<paste>" http://localhost:8080/profile | jq .
# -> { "user": {...}, "resources": [...] }
```

## Endpoints

```
GET  /healthz, /readyz
GET  /auth/{provider}                  302 to provider authorize URL; sets oauth_state cookie
GET  /auth/{provider}/callback         exchanges code; sets session_id cookie; 302 to /
POST /logout                           clears server session + cookie
GET  /profile                          aggregated user + resources JSON (requires session)
GET  /                                 minimal embedded UI
```

## Configuration

| Key                                       | Env                                          | Default                          |
|-------------------------------------------|----------------------------------------------|----------------------------------|
| `server.addr`                             | `OAUTH_MS_SERVER_ADDR`                       | `:8080`                          |
| `server.base_url`                         | `OAUTH_MS_SERVER_BASE_URL`                   | `http://localhost:8080`          |
| `auth.state_ttl`                          | `OAUTH_MS_AUTH_STATE_TTL`                    | `10m`                            |
| `auth.session_ttl`                        | `OAUTH_MS_AUTH_SESSION_TTL`                  | `24h`                            |
| `auth.cookie_secure`                      | `OAUTH_MS_AUTH_COOKIE_SECURE`                | `false` (enable behind HTTPS)    |
| `auth.cookie_samesite`                    | `OAUTH_MS_AUTH_COOKIE_SAMESITE`              | `lax`                            |
| `internal.base_url`                       | `OAUTH_MS_INTERNAL_BASE_URL`                 | `http://internal-service:8081`   |
| `internal.api_key`                        | `OAUTH_MS_INTERNAL_API_KEY`                  | **required**                     |
| `internal.cache_ttl`                      | `OAUTH_MS_INTERNAL_CACHE_TTL`                | `60s`                            |
| `providers.github.client_id`              | `OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID`        | **required**                     |
| `providers.github.client_secret`          | `OAUTH_MS_PROVIDERS_GITHUB_CLIENT_SECRET`    | **required**                     |
| `cache.driver`                            | `OAUTH_MS_CACHE_DRIVER`                      | `memory` (`redis` is a stub)     |

Precedence: env > yaml > defaults. Yaml is read from `./config/config.yaml` or `/etc/oauth-ms/config.yaml`.

## Internal service URL

The connector hits `GET {internal.base_url}/api/users/{user_id}` with
`X-Internal-API-Key: {internal.api_key}`. Path and header match the assignment
spec exactly; the base URL is configurable.

| Environment | Base URL today |
|---|---|
| Local dev (`make run`) | whatever `OAUTH_MS_INTERNAL_BASE_URL` is set to (the README quick-start uses `http://localhost:8081`) |
| Tilt + k8s | `http://internal-service.oauth-ms.svc.cluster.local:8081` (in the ConfigMap) |
| Assignment literal | `https://internal-service.local` |

To match the assignment literal locally:

```bash
# /etc/hosts:
127.0.0.1   internal-service.local
# then either run plain HTTP and drop the 's':
export OAUTH_MS_INTERNAL_BASE_URL=http://internal-service.local:8081
# or run the mock behind a local TLS cert (mkcert) and keep https.
```

## Adding a new provider

1. Create `internal/adapter/provider/<name>/` implementing `port/provider.Provider`.
2. Add `provideXProvider(...)` in `cmd/dependencies/providers.go`.
3. Pass it into `provideProviderRegistry(logger, githubProv, xProv)`.

No domain, use-case, transport, or registry code changes.

## Security

- **CSRF + PKCE**: every flow has a server-side single-use state record (`StateStore.Consume`) AND an `oauth_state` cookie whose value must match the query param. PKCE method `S256`.
- **Session**: random 32-byte session ID; only the ID is in the cookie. Access tokens never leave the server.
- **Cookies**: `HttpOnly`, `Secure` (prod), `SameSite=Lax` (so the OAuth redirect-back works), path-scoped.
- **Provider validation**: name must be in the registry before any redirect — no open redirect via `/auth/{anything}`.
- **Secrets**: GitHub client secret + internal API key from env only; never logged.
- **Outbound timeouts** on every external call; context propagation cancels in-flight work when the request is cancelled.
- **Distroless nonroot** runtime images; readonly root filesystem and dropped Linux capabilities in k8s.

## Edge cases considered

- **State replay** — `Consume` is read-and-delete; replays return `state expired or already used`.
- **State / provider mismatch** — consumed record's provider must match the callback URL's provider.
- **Internal service down** — `/profile` returns 502, error logged with provider/user context.
- **Internal user 404** — `/profile` returns 404 (no internal record for this provider user).
- **Concurrent `/profile` for same user** — cached internal service collapses via `singleflight`.
- **Cookie loss between start and callback** — missing state cookie ⇒ CSRF rejected (400).
- **Provider error param on callback** — surfaces the provider's `error` query param as a 400.

## Testing

```
make test          # go test ./... -race -count=1
make vet           # go vet ./...
```

The skeleton ships unit tests for `crypto/pkce`, `cache/memory`, `store/cached` (single-use state), and `usecase/profile` (parallel/error semantics). Provider strategy tests come with the GitHub HTTP-mocked implementation.

## AI session log

Per the assignment guideline (`docs/ai-session.md`), the conversation transcript with Claude is captured separately. The skeleton itself was generated in one session that produced this layout, the DI container, the strategy interface, and the parallel `/profile` use case.
