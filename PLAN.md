# Plan

What buunk-hub is, the decisions already made, and the milestones in order.
Work through the milestones top to bottom; tick a box only after its
**Done when** check passed for real. This is step 9 of the
[hgd1-k8s plan](https://github.com/jurrebuunk/hgd1-k8s/blob/main/PLAN.md).

## What it is

A hub for people, behind Keycloak: a bit like Keycloak's account console, but
ours, and each person can make their own corner of it.

```text
┌──────────────┬──────────────────────────────────────────┐
│ Home         │  Home: every app I have access to        │
│ ──────────── │  ┌───────────┐ ┌──────┐ ┌─────────────┐  │
│ My pages     │  │ Cloudreve │ │ …    │ │ lab-console │  │
│  Job issues  │  └───────────┘ └──────┘ │ needs WARP  │  │
│  …           │                         └─────────────┘  │
│ + New page   │                                          │
│ ──────────── │                                          │
│ Settings     │                                          │
└──────────────┴──────────────────────────────────────────┘
```

| Part | What | Who |
| --- | --- | --- |
| **Home** | Every app the person has access to. Apps behind WARP show as *not available, connect WARP* until the person is on WARP; then they show like the rest. Nothing personal here | Shared, defined by the owner in Git |
| **My pages** (sidebar) | Pages a person made; **+ New page** creates one and embeds a site or tool in it | Personal, only theirs |
| **Settings** | Profile and picture, their browser start page, their quick links | Personal |

The **browser start page** (`/start`) is something else than Home: a separate,
light page a person sets as their browser's home or new-tab page. Personal
things never show up on Home or for other people, so the hub stays clean.

## Decisions

Made up front so work can start; change one here first if it turns out wrong.

| Topic | Decision | Why |
| --- | --- | --- |
| Name, address | Repo `buunk-hub`, `https://hub.buunk.org`, start page `https://hub.buunk.org/start` | `home.buunk.org` is taken |
| Exposure | **Public** through the `hgd1-apps` tunnel, everything behind sign-in except `/avatars/*` and `/healthz` | People must reach it without WARP, to see what needs WARP |
| Backend | **Go** (stdlib `net/http`, `pgx`, `coreos/go-oidc`, `golang.org/x/oauth2`, AWS SDK v2 for S3), one binary that also serves the built frontend | Nodes have 4 GiB RAM; same language as `clusterd` |
| Frontend | **React + TypeScript + Vite** with **`@flatwork-ui/react`** (the `flatwork-saas` theme, same as the Keycloak login) | Same look as `auth.buunk.org` |
| Start page | Server-rendered HTML from Go templates, no React, tiny CSS, no JS needed | Opens with every browser window: must be instant |
| Sign-in | OIDC code flow with PKCE, **confidential client `hub`**, done by the server (backend-for-frontend): the browser only gets an `HttpOnly`, `Secure`, `SameSite=Lax` session cookie, never a token | No tokens in the browser |
| Sessions | Stored in Postgres, **30 days, sliding**. Roles are re-read from Keycloak with the refresh token at most once an hour; if Keycloak refuses (account disabled, token revoked), the session ends. The hub asks for `offline_access`, so the `hub` client needs it as an optional scope | The start page must stay signed in for weeks; a normal refresh token dies with the Keycloak SSO session after hours |
| Data | Own Postgres `hub-db` (CloudNativePG, 2 instances, `components/postgres`) | Platform rule: databases are Postgres |
| Files | Profile pictures in S3 bucket `hub-avatars` (own S3 user `hub`) | Platform rule: files go to S3 |
| Apps on Home | A catalogue in Git (`deploy/catalogue.yaml`, a ConfigMap): name, URL, description, icon, the client role that grants it, `internal: true` for apps behind WARP. Shown when the person's token has that role | "Shared, defined by the owner"; no admin rights needed to list apps |
| WARP check | For each internal app the browser fetches `https://<app>/` with `mode: "no-cors"` and a 1.5 s timeout: it resolves only on WARP *and* when Gateway lets the person in. Fails → greyed out with a link to the WARP how-to | Shows exactly what the person can use right now |
| Picture | Upload → server re-encodes to a 512×512 JPEG (strips metadata), stores `hub-avatars/<user-id>/<hash>.jpg`, serves it publicly at `https://hub.buunk.org/avatars/<user-id>/<hash>.jpg` (immutable cache), and writes that URL into the Keycloak user's `picture` attribute | S3 is never published; other apps (Cloudreve) read the `picture` claim |
| Keycloak writes | The `hub` client gets a service account with `realm-management` role `manage-users`, used **only** to set `picture` | Simplest; it's our own app. Revisit with fine-grained admin permissions if that grows |
| Embeds | `<iframe sandbox="allow-scripts allow-forms allow-popups allow-same-origin">` of another origin. On save the server checks the target's `X-Frame-Options` and CSP `frame-ancestors`; refused → a link card instead | Many sites refuse framing |
| Languages | English and Dutch, from the token's `locale` claim, else `Accept-Language` | Family is Dutch |
| Image, CI | `ghcr.io/jurrebuunk/buunk-hub:<VERSION>` from GitHub Actions, published once per version (copy `hgd1-cloudreve/.github/workflows/image.yml`), distroless, non-root | Same as Cloudreve |
| Dev | A Nix flake dev shell (go, nodejs, kustomize, sops, ssh-to-age) and `dev/compose.yaml` with Postgres, Keycloak (a test realm with test users) and MinIO | Runs without the cluster |

## Milestones

### 1. Skeleton

- [x] `flake.nix` dev shell; Go module `github.com/jurrebuunk/buunk-hub`;
      `web/` (Vite, React, TypeScript, `@flatwork-ui/react`); the Go binary
      embeds `web/dist`.
- [x] `GET /healthz` (no auth); config from environment variables only
      (document each in README).
- [x] `Dockerfile` (multi-stage: node build, go build, distroless static,
      non-root) and `VERSION` (start at `0.1.0`).
- [x] CI: tests and lint on pull requests; the image workflow on `main`.
- [x] Done when: `docker build .` works, the container answers `/healthz`,
      and CI published `ghcr.io/jurrebuunk/buunk-hub:0.1.0`.

### 2. Sign-in, locally

- [x] `dev/compose.yaml`: Postgres, Keycloak 26.8.2 with
      `dev/realm.json` (client `hub`, groups `admins`/`family`/`friends`,
      client roles like the real realm, test users with throwaway
      passwords), MinIO.
- [x] Database migrations (embedded, run at start); tables for users
      (Keycloak `sub`, cached name, email, picture) and sessions.
- [x] Login, callback (state, nonce, PKCE checked), logout (also ends the
      Keycloak session), session middleware, CSRF: `SameSite=Lax` plus an
      `Origin` check on every non-GET request.
- [x] Done when: against the dev Keycloak a test user signs in, reloads,
      stays signed in, signs out; a forged `state` is refused (test).

### 3. On the cluster

- [ ] `deploy/`: namespace `hub`, `hub-db` Cluster, Deployment (2 replicas,
      zone spread, requests/limits, `prometheus.io` annotations if it has
      metrics), Service, sops Secrets (OIDC client secret, session key, S3
      key), components `postgres`, `egress/default`, `egress/s3`.
- [ ] In `hgd1-k8s` (see AGENTS.md, *Changes outside this repo*):
      `apps/hub.yaml` (copy `apps/cloudreve.yaml`), client `hub` in
      `platform/keycloak-realm/hgd1.yaml` (like `dev/realm.json`: optional
      scope `offline_access`, all client roles in the ID token as
      `resource_access`), tunnel route `hub.buunk.org` in
      `infrastructure/cloudflared/config.yaml`, DNS record (proxied CNAME to
      the tunnel).
- [ ] S3 user `hub` and bucket `hub-avatars` (`labctl s3 user create`,
      `labctl s3 bucket create`).
- [ ] Done when: from outside the network (no WARP) the owner signs in at
      `https://hub.buunk.org`, and `labctl vm stop` on the VM of one replica
      keeps it working.

### 4. Home

- [ ] `deploy/catalogue.yaml` with Cloudreve (role `cloudreve/cloudreve-user`
      or `cloudreve-admin`) and the test app `whoami.buunk.org` (internal).
- [ ] Home lists the apps whose role is in the person's token
      (`resource_access.<client>.roles`), as cards; the WARP check for
      internal ones.
- [ ] Done when: the owner sees Cloudreve; a `friends` test user sees only
      what `friends` has; `whoami` shows *connect WARP* off WARP and as normal
      on WARP.

### 5. Profile picture

- [ ] In `hgd1-k8s`: realm user profile attribute `picture` (`https://` URL,
      edit: admin only, view: user and admin), the `hub` service account
      with `manage-users`.
- [ ] Settings: upload (max 5 MB, JPEG/PNG/WebP in, 512×512 JPEG out),
      crop to square, remove picture.
- [ ] Done when: after upload the hub shows it, `https://hub.buunk.org/avatars/…`
      loads without signing in, and a fresh ID token of another client that
      asks for `profile` has the `picture` claim.

### 6. Start page

- [ ] `/start`: a DuckDuckGo search form (`GET https://duckduckgo.com/`, field
      `q`, nothing logged on our side), the person's apps and their quick
      links. Signed out → straight to sign-in and back.
- [ ] Settings: add, edit, reorder and remove quick links (title, URL).
- [ ] Done when: set as a browser's home page it shows within 200 ms on a
      warm connection, under 50 KB, and still works signed in after a week.

### 7. My pages

- [ ] Sidebar: the person's pages; **+ New page** (title, URL to embed);
      rename, reorder, delete.
- [ ] Embed in a sandboxed iframe; the framing check on save (https only,
      5 s timeout, no redirects to private addresses: the egress policy
      blocks the site anyway); refused → link card.
- [ ] Done when: a page embedding an issue tracker works, a site that
      refuses framing shows as a card, and another user can't open the page
      (test).

### Later

- Cloudreve shows the `picture` as its avatar: a patch in
  [hgd1-cloudreve#2](https://github.com/jurrebuunk/hgd1-cloudreve/issues/2),
  after milestone 5.
- More apps in the catalogue as they are published.
