# Working on buunk-hub

Instructions for agents (and me). Read this, then [PLAN.md](PLAN.md): the
decisions there are made; the milestones are the work, in order.

## How to start

1. Find the first milestone in PLAN.md with an unticked box and do that one.
   One milestone per session unless asked for more.
2. Follow the decisions table. If one turns out wrong or blocks you, stop and
   propose the change in your reply; don't quietly do something else.
3. Before calling a box done, run its **Done when** check for real and say
   what you ran and what you saw. Then tick it in PLAN.md in the same commit.

## Where things live

| What | Where |
| --- | --- |
| This app: code, `deploy/` (kustomize), image CI | this repo |
| Flux pointer, Keycloak clients and realm, tunnel routes, shared components | [`hgd1-k8s`](https://github.com/jurrebuunk/hgd1-k8s), locally `~/repos/hgd1-k8s` |
| Hosts, VMs, `labctl`, WARP and kubeconfig | [`nix-hgd1-cluster`](https://github.com/jurrebuunk/nix-hgd1-cluster), locally `~/repos/nix-hgd1-cluster` |
| Design: tokens, components | [`flatwork-ui`](https://github.com/jurrebuunk/flatwork-ui) (`@flatwork-ui/react` on npm, theme `flatwork-saas`) |
| The example to copy | [`hgd1-cloudreve`](https://github.com/jurrebuunk/hgd1-cloudreve): `deploy/`, `.github/workflows/image.yml`, sops files |

Read before platform work: hgd1-k8s `docs/how-to/build-an-app.md`,
`publish-an-app.md`, `add-a-database.md`, `manage-secrets.md` and
`docs/concepts/keycloak.md`. The look: `~/repos/hgd1-k8s/platform/keycloak/theme/DESIGN.md`
(the sign-in page people see right before the hub).

## Rules

- **Secrets** only as sops files (`*.sops.yaml`, rules in `.sops.yaml`).
  Encrypting needs only the public keys:
  `sops encrypt --filename-override deploy/x.sops.yaml /dev/stdin > deploy/x.sops.yaml`.
  Never commit, print or log a secret value. Generated secrets (session key,
  client secret) are made with `openssl rand` straight into sops.
- **Flux deploys `main`.** A push to `main` of this repo's `deploy/` goes live
  within minutes. Run `kustomize build deploy` (with hgd1-k8s's `components/`
  next to it, as Flux does) before pushing.
- **Images** are never overwritten: bump `VERSION` with every change to what
  goes into the image, then point `deploy/` at the new tag.
- **Never** `kubectl apply`, edit or delete anything Flux owns; change Git.
  Reading (`get`, `logs`, `describe`) is fine.
- **Network:** pods reach the cluster, the internet and S3, not the site
  (`10.10.0.0/16`). Don't add egress beyond `components/egress/{default,s3}`.
- **Resources:** a node has 4 GiB RAM in total. Keep requests small
  (aim: under 64 MiB for the Go server) and always set limits.
- **No tokens in the browser.** The server holds them; the browser holds a
  session cookie.
- **Accessible and bilingual:** real buttons and links, labels on inputs,
  4.5:1 contrast; every user-visible string in English and Dutch.
- **Tests:** Go tests for the security-relevant parts (OIDC callback checks,
  session expiry, CSRF/Origin check, page ownership, avatar re-encoding),
  `go vet`, and `tsc --noEmit` for the web. CI runs them.
- **Deferred work becomes a GitHub issue** on the repo that owns it, linked
  from PLAN.md.

## Changes outside this repo

Milestones 3 and 5 also change `hgd1-k8s` (Flux pointer, Keycloak client and
user profile, tunnel route). That repo is shared by every app and deploys from
`main` at once, so:

1. Make the change on a local branch there, following its docs and the style
   of what's already in the file (comments explain why).
2. `kustomize build` the parts you changed.
3. **Show the diff and ask the owner before pushing.** Never push there
   without a yes.

Steps that need the owner, so ask instead of working around them:

- Cluster access: WARP must be on virtual network `hgd1-admin`
  (`warp-cli vnet 84c40b26-4fac-4131-a48a-9db3ca63267c`), then
  `cd ~/repos/nix-hgd1-cluster && nix develop` gives `kubectl` and `labctl`.
  Switch back to `default` (`warp-cli vnet 21a5b1d6-194b-4826-963d-d3352e64dfe5`)
  when done.
- The DNS record for `hub.buunk.org` (Cloudflare).
- Anything that needs a password, an account or a payment.

## Style

- Code reads like the surrounding code; comments say *why*, briefly.
- Docs and commit messages: plain, short sentences; say what changed and why.
- Commits end with the co-author lines your harness gives you, if any.
