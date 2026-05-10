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

## Prerequisites

| Tool | Version | Used for |
|---|---|---|
| Go | 1.22+ (Dockerfile uses 1.26) | building & running the binary |
| `make` | any | shortcut targets |
| `jq` | any | reading JSON responses in the test steps below |
| Docker | any recent | building images for the k8s path |
| A local k8s cluster | Docker Desktop / kind / k3d / minikube | only for the Tilt path |
| Tilt | 0.33+ | only for the Tilt path |

Local-only path needs just Go + make + jq. Browser is needed for the OAuth handshake either way.

## One-time setup (do this once, reuse for both run modes)

### 1. Create a GitHub OAuth app

GitHub → **Settings → Developer settings → OAuth Apps → New OAuth App**

| Field | Value |
|---|---|
| Application name | anything, e.g. `oauth-ms-dev` |
| Homepage URL | `http://localhost:8080` |
| Authorization callback URL | `http://localhost:8080/auth/github/callback` |

Save the **Client ID** and generate a **Client Secret**. Hold onto both.

The same callback URL works for the local binary and the Tilt/k8s path because Tilt port-forwards the connector to `localhost:8080`. One OAuth app covers both.

### 2. Find your GitHub numeric user ID

The connector keys the internal-service lookup off the GitHub numeric ID, not your login. The mock ships with a fallback row at id `"0"` only — until you add your real ID, `/profile` will return 404 from the internal service even though OAuth itself succeeds.

```bash
curl -s https://api.github.com/users/<your-github-login> | jq .id
# -> 49481430   (example; yours will differ)
```

Keep that number — you'll paste it into `MOCK_USERS` in step 2 of whichever run mode you pick below.

---

## Run locally (no Kubernetes)

### Start the mock internal service

```bash
# Terminal 1
export MOCK_USERS='{
  "49481430": {"id":"49481430","name":"Your Name","license":"Pro","role":"admin"}
}'   # replace 49481430 with the GitHub ID you grabbed above
make run-mock
```

You should see it listening on `:8081`. Verify:

```bash
curl -s http://localhost:8081/healthz
# -> ok
curl -s -H 'X-Internal-API-Key: super-secret-key' \
  http://localhost:8081/api/users/49481430 | jq .
# -> {"id":"49481430","name":"Your Name","license":"Pro","role":"admin"}
```

### Start the connector

```bash
# Terminal 2
export OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID=<paste client id>
export OAUTH_MS_PROVIDERS_GITHUB_CLIENT_SECRET=<paste client secret>
export OAUTH_MS_INTERNAL_API_KEY=super-secret-key
export OAUTH_MS_INTERNAL_BASE_URL=http://localhost:8081
make run
```

Verify it's up:

```bash
curl -s http://localhost:8080/healthz   # -> ok
curl -s http://localhost:8080/readyz    # -> ok
```

Now jump to **[Manual end-to-end test](#manual-end-to-end-test)** below.

---

## Run on local Kubernetes (Tilt)

### 1. Create the secret manifest

```bash
cp deploy/k8s/connector-secret.example.yaml deploy/k8s/connector-secret.yaml
# Edit deploy/k8s/connector-secret.yaml and fill in:
#   OAUTH_MS_PROVIDERS_GITHUB_CLIENT_ID
#   OAUTH_MS_PROVIDERS_GITHUB_CLIENT_SECRET
# Leave OAUTH_MS_INTERNAL_API_KEY as "super-secret-key" (or change it; both
# the connector and the mock read it from this secret).
```

`connector-secret.yaml` is gitignored — keep it that way.

### 2. Add your GitHub user ID to the mock

Edit `deploy/k8s/internal-deployment.yaml`, find the `MOCK_USERS` env var, and replace `REPLACE_WITH_GITHUB_ID_2` (or add a new entry) with the GitHub numeric ID you grabbed above:

```yaml
- name: MOCK_USERS
  value: |
    {
      "49481430": {"id":"49481430","name":"You","license":"Pro","role":"admin"}
    }
```

### 3. Bring it up

```bash
make tilt-up
# Tilt UI: http://localhost:10350
# Wait until both 'connector' and 'internal-service' resources go green.
```

Tilt port-forwards `connector` → `localhost:8080` and `internal-service` → `localhost:8081`.

Verify:

```bash
curl -s http://localhost:8080/readyz                 # -> ok
kubectl -n oauth-ms get pods                         # both Running, 1/1 Ready
```

To stop: `make tilt-down`.

Now do the manual end-to-end test below.

---

## Manual end-to-end test

Same flow whichever run mode you used; the connector is at `http://localhost:8080`.

### A. Browser path (the happy path)

1. Open `http://localhost:8080` in a browser.
2. You should see the **OAuth Connector** landing page with a **Login with GitHub** button.
3. Click it → you're redirected to GitHub's authorize page.
4. Approve. GitHub redirects back to `http://localhost:8080/auth/github/callback?code=...&state=...`.
5. The page reloads to `/`, which now shows your name, two pills (Tier / Role from the mock), and a table of public repos sorted by stars.

If you see your name + repos but no pills, the OAuth half worked but the internal-service lookup didn't — see Troubleshooting.

### B. Curl path (scriptable verification)

```bash
# 1. Kick off the redirect — confirm the connector is willing to start the flow.
curl -i http://localhost:8080/auth/github
# Expect:
#   HTTP/1.1 302 Found
#   Location: https://github.com/login/oauth/authorize?...client_id=...&state=...&code_challenge=...
#   Set-Cookie: oauth_state=...; HttpOnly; SameSite=Lax; ...

# 2. Complete the flow in a browser (curl alone can't, GitHub renders a consent screen).
#    After the redirect lands you back on '/', open devtools → Application → Cookies
#    and copy the value of session_id.

# 3. Hit /profile with that cookie:
SID='paste-session-id-here'
curl -s --cookie "session_id=$SID" http://localhost:8080/profile | jq .
```

Expected `/profile` shape:

```json
{
  "user": {
    "id": "49481430",
    "name": "Your Name",
    "license": "Pro",
    "role": "admin"
  },
  "resources": [
    { "name": "some-repo", "description": "...", "metric": 42 },
    ...
  ]
}
```

### C. Logout

```bash
curl -i -X POST --cookie "session_id=$SID" http://localhost:8080/logout
# -> 204; Set-Cookie clears session_id. Subsequent /profile returns 401.
```

Or click **Logout** in the UI.

### What "passing" looks like

- [ ] `GET /healthz` returns `ok`.
- [ ] `GET /auth/github` returns a 302 to `github.com/login/oauth/authorize` with both `state` and `code_challenge` query params, and sets an `oauth_state` cookie.
- [ ] After completing the GitHub consent, the UI shows your name, **both** pills (Tier and Role), and a non-empty repos table.
- [ ] `GET /profile` (with the `session_id` cookie) returns 200 and includes `user.license` and `user.role` populated from the mock.
- [ ] `POST /logout` returns 204; a follow-up `GET /profile` returns 401.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `/profile` returns 401 right after login | `session_id` cookie not sent (e.g. you tested in a different browser/incognito) | reload `/` in the same browser; or pass `--cookie "session_id=..."` to curl |
| `/profile` returns 404 | the GitHub user ID isn't in `MOCK_USERS` | add it to `MOCK_USERS` and restart the mock (local) or `kubectl apply -f deploy/k8s/internal-deployment.yaml && kubectl -n oauth-ms rollout restart deploy/internal-service` (k8s) |
| `/profile` returns 502 | mock isn't reachable from the connector | check `OAUTH_MS_INTERNAL_BASE_URL` (local) or the ConfigMap `internal.base_url` (k8s); verify `curl http://localhost:8081/healthz` |
| Browser shows `state expired or already used` after callback | you reloaded the callback URL or your `oauth_state` cookie was dropped | start over from `http://localhost:8080`, don't refresh on the callback URL |
| GitHub redirects to a `redirect_uri_mismatch` error page | callback URL in the GitHub OAuth app doesn't match | must be exactly `http://localhost:8080/auth/github/callback` |
| Tilt: `connector` pod CrashLoopBackOff | secret values still say `REPLACE_ME` | edit `deploy/k8s/connector-secret.yaml` with real values; Tilt auto-reapplies and `connector-secret-reload` bounces the deploy |
| Tilt: changed `MOCK_USERS` but pod still serves old map | env vars are read at start | `kubectl -n oauth-ms rollout restart deploy/internal-service` |

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

## Mock internal service

`mocks/internal-service/` is a standalone Go binary that satisfies
`GET /api/users/{id}` with the `X-Internal-API-Key` header. It is configured
entirely via env vars:

| Env          | Default            | Notes                                                                 |
|--------------|--------------------|-----------------------------------------------------------------------|
| `ADDR`       | `:8081`            | Listen address.                                                       |
| `API_KEY`    | `super-secret-key` | Must match the connector's `OAUTH_MS_INTERNAL_API_KEY`.               |
| `MOCK_USERS` | _(unset)_          | JSON map keyed by **GitHub numeric user ID**. See below.              |

If `MOCK_USERS` is unset, the mock seeds a single fallback row at id `"0"`
(`{"id":"0","name":"Demo User","license":"Pro","role":"admin"}`) so the binary
boots in isolation. **Your real GitHub user ID will not be `0`**, so without
`MOCK_USERS` the connector's `/profile` returns 404 from the internal service.
To find your ID:

```bash
curl -s https://api.github.com/users/<your-login> | jq .id
```

Then point the mock at one or more users:

```bash
export MOCK_USERS='{
  "49481430": {"id":"49481430","name":"Alice","license":"Pro","role":"admin"},
  "12345678": {"id":"12345678","name":"Bob",  "license":"Free","role":"viewer"}
}'
make run-mock
```

The Tilt + k8s path bakes the same env into
`deploy/k8s/internal-deployment.yaml` — edit the `MOCK_USERS` value there and
re-apply.

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

Per the spec guideline (`docs/ai-session.md`), the conversation transcript with Claude is captured separately. The skeleton itself was generated in one session that produced this layout, the DI container, the strategy interface, and the parallel `/profile` use case.
