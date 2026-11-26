# buunk-hub

The people hub for buunk.org, at `https://hub.buunk.org`: everyone signs in
with their buunk account and sees the apps they can use, sets their profile
picture, keeps a browser start page with search and quick links, and makes
their own pages that embed the sites and tools they need. Keycloak stays where
accounts and passwords live.

It runs on the [hgd1 cluster](https://github.com/jurrebuunk/hgd1-k8s) and is
deployed by Flux from this repo's `deploy/`.

**Status:** milestone 2 (sign-in, locally) of [PLAN.md](PLAN.md); agents
start with [AGENTS.md](AGENTS.md).

## Develop

`nix develop` gives Go, Node, kustomize and sops. `dev/compose.yaml` runs
what the hub needs: Postgres, Keycloak with a test realm, and MinIO.

```bash
docker compose -f dev/compose.yaml up -d
npm --prefix web ci
npm --prefix web run build          # web/dist, embedded by web/embed.go
set -a; . dev/hub.env; set +a
go run .                            # http://localhost:8080
```

Sign in as `owner` (group admins), `family` or `friend`; the password is
`<username>-dev-password`. Keycloak's admin console is at
<http://localhost:8180/admin/> (`admin` / `admin`).

For frontend work, sign in once at :8080, then run `npm --prefix web run dev`
next to the server: Vite serves the app with hot reload at :5173 and proxies
`/api` and `/auth` to the server.

Tests: `go test ./...`. The database tests run when `HUB_TEST_DATABASE_URL`
is set, e.g. to the compose Postgres, `postgres://hub:hub@localhost:15432/hub`.

The image: `docker build .`; CI publishes `ghcr.io/jurrebuunk/buunk-hub:$(cat VERSION)`
from `main`. Bump `VERSION` with every change to what goes into the image.

## How signing in works

Everything except `/healthz` and `/auth/` needs a session. Without one, a
page redirects to `/auth/login`, which goes to Keycloak and comes back to
`/auth/callback` and then to the page; `/api/` answers 401 instead. The
server does the OIDC code flow (PKCE, state and nonce checked) with the
confidential client `hub` and keeps the tokens; the browser only gets an
`HttpOnly` session cookie.

- Sessions are in Postgres and last 30 days, sliding. The cookie holds a
  random value; the table holds its SHA-256, and the tokens sealed with
  `HUB_SESSION_KEY`.
- The hub asks for `offline_access`: a normal Keycloak refresh token ends
  with its SSO session after hours, an offline one lasts while it is used.
  With it, the roles are re-read at most once an hour. If Keycloak refuses
  (account disabled or token revoked) the session ends; if Keycloak can't
  be reached, the session keeps its roles and tries again later.
- Signing out (`POST /auth/logout`) deletes the session, revokes the
  offline token, ends the Keycloak session, and lands on `/auth/signed-out`.
- Every request other than `GET`/`HEAD`/`OPTIONS` must carry an `Origin`
  equal to `HUB_PUBLIC_URL` (with `SameSite=Lax` cookies, that stops CSRF).

## Configuration

Environment variables only; `dev/hub.env` has values for development.

| Variable | Default | What |
| --- | --- | --- |
| `HUB_ADDR` | `:8080` | Where the HTTP server listens, `host:port` |
| `HUB_PUBLIC_URL` | required | The hub's address, `https://hub.buunk.org`: the OIDC redirect, the Origin check and `__Host-`/`Secure` cookies (for `https`) come from it |
| `HUB_DATABASE_URL` | required | Postgres connection string |
| `HUB_OIDC_ISSUER` | required | `https://auth.buunk.org/realms/hgd1` |
| `HUB_OIDC_CLIENT_ID` | `hub` | The Keycloak client |
| `HUB_OIDC_CLIENT_SECRET` | required | Its secret |
| `HUB_SESSION_KEY` | required | 32 random bytes, base64 (`openssl rand -base64 32`); seals the sign-in cookie and the stored tokens. Changing it signs everyone out |

The database schema is migrated at start (`internal/db/migrations`).

`GET /healthz` answers `ok` without signing in, for Kubernetes probes.
