# buunk-hub

The people hub for buunk.org, at `https://hub.buunk.org`: everyone signs in
with their buunk account and sees the apps they can use, sets their profile
picture, keeps a browser start page with search and quick links, and makes
their own pages that embed the sites and tools they need. Keycloak stays where
accounts and passwords live.

It runs on the [hgd1 cluster](https://github.com/jurrebuunk/hgd1-k8s) and is
deployed by Flux from this repo's `deploy/`.

**Status:** milestone 1 (skeleton) of [PLAN.md](PLAN.md); agents start with
[AGENTS.md](AGENTS.md).

## Develop

`nix develop` gives Go, Node, kustomize and sops. The Go binary embeds the
built frontend, so build `web/` first:

```bash
npm --prefix web ci
npm --prefix web run build   # web/dist, embedded by web/embed.go
go test ./...
go run .                     # http://localhost:8080
```

For frontend work, run `go run .` and `npm --prefix web run dev` side by
side: Vite serves the app with hot reload and proxies the server's paths.

The image: `docker build .`; CI publishes `ghcr.io/jurrebuunk/buunk-hub:$(cat VERSION)`
from `main`. Bump `VERSION` with every change to what goes into the image.

## Configuration

Environment variables only.

| Variable | Default | What |
| --- | --- | --- |
| `HUB_ADDR` | `:8080` | Where the HTTP server listens, `host:port` |

`GET /healthz` answers `ok` without signing in, for Kubernetes probes.
