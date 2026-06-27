# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A proof-of-concept for driving [Authentik](https://goauthentik.io/) programmatically via its
[Go client library](https://github.com/goauthentik/client-go). Two independent halves:

1. **Deployment** — manifests + scripts to run an Authentik instance, either on Kubernetes (kind) or via Docker Compose.
2. **`provisioner/`** — a Go program that talks to a *running* Authentik instance's REST API to create groups, users, passwords, and OAuth tokens, then re-authenticates as a created user to read its group membership.

The Go POC and the deployment are decoupled: you stand up Authentik first (compose or k8s), then point the POC at it.

## Layout

- `provisioner/` — the Go POC (module `github.com/AndriyKalashnykov/authentik-k8s/provisioner`, Go 1.26.x via `provisioner/.mise.toml`, client `goauthentik.io/api/v3`).
  - `main.go` — orchestration; all config read from env with fallback defaults (see `provisioner/.env.example`).
  - `internal/authentik/api.go` — thin wrappers over the client's `CoreApi` (CreateGroup, CreateUser, UpdateUserPassword, CreateUserToken, UpdateUserToken, RetrieveUserToken, MeRetrieveUser).
  - `internal/authentik/forwardauth.go` — opt-in forward-auth wiring (ProxyProvider + Application create, Flow-slug→PK resolution, embedded-outpost bind) + the idempotent `SetupForwardAuth` orchestration. Gated by `AUTHENTIK_FORWARD_AUTH_ENABLED=true` in `main.go` (off by default; `AUTHENTIK_PROVISION_ORGS=false` runs ONLY forward-auth). Contract-tested in `forwardauth_httptest_test.go`.
  - `internal/util/utils.go` — TLS transport (skips verify), pointer helpers (`*bool`, `*int32`, `*string` — the client takes pointers for optional fields).
  - `Dockerfile` — multi-stage, static binary on distroless-nonroot; a one-shot job (no port/healthcheck), config supplied via env at runtime (`make image-build` / `image-run`).
  - `.env.example` — committed source of truth for the POC's env vars; copy to `.env` (gitignored). `e2e_test.go` is the build-tagged (`//go:build e2e`) live test driven by `make e2e-compose` / `make e2e`.
- `k8s/postgresql/` — Authentik manifests (PostgreSQL backend). **Generated** by `make k8s-generate` from the pinned Authentik Helm chart (`AUTHENTIK_CHART_VERSION` in the Makefile) + `values.yml` (Bitnami subcharts disabled), concatenated with `oss-datastores.yml` (the OSS `postgres:18-alpine` workload), with Helm metadata labels stripped. A CI `k8s-drift` job (`make k8s-generate-check`) fails if the committed manifest is stale. No Redis/Valkey — Authentik 2026.5 is PostgreSQL-only.
- `k8s/kind-config.yaml` — single-node KinD cluster config used by `make e2e`.
- `compose/` — Docker Compose stack (Authentik server + worker + PostgreSQL; **no Redis** — Authentik dropped the Redis dependency in 2025.10, everything is PostgreSQL-backed now). `compose/.env.example` is the committed source of truth; `compose/.env` is gitignored. Linted by `make compose-lint` (dclint; rule config in `.dclintrc.yaml`).
- `compose/forward-auth/` — optional overlay (`docker-compose.forward-auth.yml` + `traefik/dynamic.yml`) adding Traefik v3.7.5 + `traefik/whoami` to the base stack to demo Authentik forward-auth. Brought up by `make compose-forward-auth-up` (which also configures the provider via the POC); verified by `make e2e-forward-auth`. Both compose files are dclint-linted by `make compose-lint`.
- `docs/` — `web-ui.md` (admin-UI screenshots + how to regenerate them) and `spikes/` (the CockroachDB/YugabyteDB datastore investigation — manifests since removed as non-working; PostgreSQL is the only backend). Screenshots live in `docs/img/`.
- `scripts/capture-web-ui-screenshots.cjs` — Playwright script that regenerates the `docs/img/*.jpg` admin-UI screenshots against a live, provisioned stack (see `docs/web-ui.md`). NOT part of deploy/teardown — that is driven entirely by the `provisioner/` Makefile (`make -C provisioner kind-up`/`kind-down` for KinD, `compose-up`/`compose-down` for Docker Compose), with no standalone shell scripts.
- `renovate.json` — tracks every pinned version (go.mod, `.mise.toml`, Dockerfile FROM/ARG, Makefile `_VERSION` constants incl. `DCLINT_VERSION`/`MERMAID_CLI_VERSION`, compose + k8s image tags, GitHub Action SHAs).

## Common commands

```bash
# --- Deploy Authentik (pick one) — via the provisioner/ Makefile ---
make -C provisioner compose-up   # Docker Compose: server on https://localhost:9443
make -C provisioner kind-up      # Kubernetes: KinD + cloud-provider-kind, applies k8s/postgresql/

# --- Go POC: toolchain via mise (provisioner/.mise.toml), targets in provisioner/Makefile ---
cd provisioner
cp .env.example .env   # one-time: per-dev config (gitignored); main.go also has the same fallbacks
make deps     # install mise (if missing) + pinned Go/golangci-lint/govulncheck/hadolint/kind/kubectl
make ci       # full local pipeline: static-check (align+lint+hadolint+mermaid+compose+vulncheck+trivy-fs+secrets) + test + build
make run      # run the POC against a running Authentik (sources .env.example then .env)
make test     # go test ./...  (unit + hermetic httptest contract tests)
make image-build / image-run   # build/run the POC container (config via --env-file)
make help     # list all targets

# --- Run Authentik + POC end-to-end (two alternatives) ---
make e2e-compose   # lightweight: Authentik via Docker Compose, then provision + verify, then down
make e2e           # full cluster: KinD + cloud-provider-kind (== kind-up + test + kind-down)
make kind-up / kind-down        # bring the KinD Authentik stack up / tear it down
make compose-up / compose-down  # bring the Compose Authentik stack up / tear it down

# --- Forward-auth demo (Traefik + traefik/whoami gated by Authentik) ---
make compose-forward-auth-up    # base stack + Traefik + whoami, then configure forward-auth (browse https://whoami.127-0-0-1.sslip.io)
make compose-forward-auth-down  # tear the forward-auth demo down (removes volumes)
make e2e-forward-auth           # up -> configure -> follow redirect -> assert Authentik login reachable (HTTP 200) -> down

make renovate-validate          # validate renovate.json

# --- Regenerate the k8s manifest from the pinned Authentik Helm chart ---
make -C provisioner k8s-generate        # helm template (subcharts off) + oss-datastores.yml + strip Helm labels
make -C provisioner k8s-generate-check  # drift gate: fail if the committed manifest is stale vs chart+values
```

Default admin login — **username** `akadmin` (Authentik's fixed bootstrap user;
hardcoded, no env var, so stated literally). The **password** is the *value* of
`AUTHENTIK_BOOTSTRAP_PASSWORD` (not restated here; single source of truth:
`compose/.env.example` for Compose, `k8s/postgresql/authentik-postgresql.yml` for
KinD) — extract just the value with
`grep AUTHENTIK_BOOTSTRAP_PASSWORD compose/.env.example | cut -d= -f2`.

## Environment configuration (`.env.example`)

Every operator-tunable value is externalized to env vars — **no hardcoded
hosts/ports/secrets in code**. Each consumer has a committed `.env.example`
(the source of truth, documenting every variable + default) and a gitignored
`.env` (per-developer override). Copy and edit:

```bash
cp provisioner/.env.example   provisioner/.env     # Go POC: AUTHENTIK_SCHEME/HOST/BOOTSTRAP_TOKEN/USER_PASSWORD/ORG*/…
cp compose/.env.example  compose/.env    # Compose stack: PG_*, AUTHENTIK_SECRET_KEY, BOOTSTRAP_*, AUTHENTIK_TAG, …
```

- `provisioner/main.go` reads each var via `os.LookupEnv` with a fallback that
  mirrors `.env.example`, so it works with or without a `.env`. `make run`
  sources `.env.example` then `.env`; `make image-run` passes `--env-file`.
- `make compose-up`/`make e2e-compose` auto-seed `compose/.env` from the
  example if it is missing.
- **Contract**: `AUTHENTIK_BOOTSTRAP_TOKEN` in `provisioner/.env*` must match the
  token in whichever deployment the POC targets (`compose/.env*` for compose;
  the committed `AUTHENTIK_BOOTSTRAP_TOKEN` in `k8s/postgresql/authentik-postgresql.yml`
  for k8s). The shipped defaults already match.
- The values in the committed `.env.example` files are **demo credentials** —
  rotate them for any real deployment.

## Architecture notes that span files

- **The bootstrap token is a shared secret across the POC and the deployment.** The POC's `AUTHENTIK_BOOTSTRAP_TOKEN` (env, default in `provisioner/.env.example`) MUST equal the `AUTHENTIK_BOOTSTRAP_TOKEN` of whatever it targets: `compose/.env*` for the Compose stack, or the committed value in `k8s/postgresql/authentik-postgresql.yml` (on both the `authentik-server` and `authentik-worker` Deployments) for k8s. The POC authenticates as admin using this token. The shipped defaults already match across all three; change it in lockstep.

- **The POC's target is env-driven** — `AUTHENTIK_SCHEME` (default `https`) + `AUTHENTIK_HOST` (default `127.0.0.1:9443`, the Compose endpoint). For k8s use `AUTHENTIK_HOST=<LB-IP>:443`. The KinD e2e (`make e2e`) resolves the LoadBalancer IP and passes it automatically; for a manual `make run` against k8s, set `AUTHENTIK_HOST` in `provisioner/.env`.

- **TLS verification is intentionally skipped** (`util.GetTLSTransport(true)`) because the dev instances use self-signed certs. Do not "fix" this without changing the deployment to use trusted certs.

- **POC flow** (`CreateGroupsAndUsers`, called 4× for org-01/org-02 × admin/user): create group → create user in group → set password → create API token → retrieve the Authentik-generated key → overwrite it with a known key → re-create an API client *as that user* with the known token → call `MeRetrieveUser` to read the user's groups. The final step demonstrates a non-privileged token can read its own group membership.

- **Forward-auth demo flow** (`SetupForwardAuth`, opt-in, idempotent). Two `main.go` gates (env, defaults in `provisioner/.env.example`): `AUTHENTIK_FORWARD_AUTH_ENABLED=true` turns it on (default `false`); `AUTHENTIK_PROVISION_ORGS=false` skips the *non-idempotent* org/user/token provisioning so the idempotent forward-auth setup runs alone — exactly the pair `make compose-forward-auth-up` sets (`AUTHENTIK_PROVISION_ORGS=false AUTHENTIK_FORWARD_AUTH_ENABLED=true`). The flow: resolve the authorization + invalidation flow PKs from their slugs (`FlowsInstancesList(slug)` — PKs are instance-specific, never hardcoded) → create-or-get a `forward_single` ProxyProvider (forward_domain also supported; `cookie_domain` only sent in domain mode) → create-or-get the Application bound to it → **configure the embedded outpost** (the one whose `managed == "goauthentik.io/outposts/embedded"`, served in-process by `authentik-server` on `:9000`, so there is no separate outpost container): bind the provider AND set its `authentik_host` (+ `authentik_host_insecure`). Traefik's `forwardAuth` middleware then calls `http://server:9000/outpost.goauthentik.io/auth/traefik`; an unauthenticated request gets a 302 to login. **`authentik_host` is load-bearing (not optional):** it is the browser-reachable Authentik URL the outpost redirects the login to (`AUTHENTIK_FORWARD_AUTH_HOST`, default `https://127.0.0.1:9443`). If unset, the embedded outpost falls back to `http://localhost` and the browser hits a 404 — this is NOT controlled by `trustForwardHeader` or `authentik_host_browser` (the *primary* `authentik_host` field is the lever). It must resolve from the browser AND from inside the server container (in-process outpost); for Compose, `127.0.0.1:9443` is both. Traefik's `trustForwardHeader: true` (in `dynamic.yml`) is also required so the outpost sees the real request host for the callback. **Startup race (important):** the worker imports the default blueprint flows (incl. `default-provider-authorization-implicit-consent`) ~60s *after* the server's `/-/health/ready/` returns 200 — so `make compose-forward-auth-up` polls the flows API for that slug before running the provisioner, or `SetupForwardAuth` fails with "flow not found".

- **The k8s manifest is GENERATED, not hand-edited** — `make k8s-generate` (don't edit `k8s/postgresql/authentik-postgresql.yml` by hand; the `k8s-drift` CI gate will fail). It renders the pinned Authentik Helm chart with the bundled **Bitnami `postgresql` subchart disabled** (the Bitnami image was removed from Docker Hub) + `values.yml` (external DB host, `secret_key`, the `AUTHENTIK_BOOTSTRAP_PASSWORD`/`TOKEN` env on server+worker), strips the Helm metadata labels, and concatenates `oss-datastores.yml` — the OSS `postgres:18-alpine` Deployment+Service. The postgres Deployment reads creds from the chart-generated `authentik` Secret (`AUTHENTIK_POSTGRESQL__*`), so it stays in sync with `values.yml`, and sets `PGDATA=/var/lib/postgresql/data/pgdata` (explicit subdir) so the postgres:18 image — whose default data dir moved to `/var/lib/postgresql` — works with the `/var/lib/postgresql/data` mount (the compose stack, no explicit `PGDATA`, mounts the volume at `/var/lib/postgresql`). **No Redis/Valkey** — Authentik dropped Redis in 2025.10 (cache, task broker, and WebSocket channels are all PostgreSQL-backed). To change the deployment, edit `values.yml` / `oss-datastores.yml` and re-run `make k8s-generate`.

- **Namespace is `default` end-to-end.** Every resource in `k8s/postgresql/authentik-postgresql.yml` is pinned to `namespace: "default"`, and the `provisioner/` Makefile deploy path (`kind-deploy`) uses `AUTHENTIK_NS := default` — they match. (A former standalone `deploy-authentik-k8s.sh` that patched/waited against `-n threeport-api` was removed; the Makefile is the only deploy path now.)

## Conventions

- Config is externalized to env vars with fallback defaults (no hardcoded hosts/ports/secrets); see the Environment configuration section. Renovate tracks every pinned version (`renovate.json`); validate with `make renovate-validate`.
- The client library takes pointers for optional request fields; use the `util.*ToPointer` helpers rather than inlining `&`.
- **CI**: `.github/workflows/ci.yml` runs `make static-check` + `make build` + `make test` + `make image-scan` (the `docker` job — builds the provisioner image and Trivy-scans it for HIGH/CRITICAL, since hadolint only *lints* the Dockerfile) + `make e2e-compose` (the `e2e` job — live Authentik via Compose) via `jdx/mise-action` (toolchain from `provisioner/.mise.toml`). The Go project is in `provisioner/`, so jobs set `working-directory: provisioner`. A `dorny/paths-filter` `changes` job gates the heavy jobs on `provisioner/**`/`.github/workflows/**`/`CLAUDE.md`/`compose/**`/`.dclintrc.yaml` edits (compose is included so `static-check`'s `compose-lint` + the `e2e` job run on Compose changes) — doc/k8s changes skip CI; a `ci-pass` job aggregates results. No tags/publish (the POC has no registry — the `docker` job builds + scans but does not push). No secrets required.
- The local quality gate (`provisioner/Makefile` → `make ci`: golangci-lint + govulncheck + go-mod-tidy + toolchain-alignment) mirrors CI. `.golangci.yml` runs the standard linters; gosec is intentionally omitted (the POC hardcodes test tokens and skips TLS verify by design — documented in `.golangci.yml`).
- **Test layers**:
  - *Unit + hermetic httptest contracts* (`make test`, no infra): `internal/authentik` (~83%) — `CreateConfiguration` auth-header contract + httptest contracts for every `CoreApi` wrapper at the real `/api/v3/...` paths, plus the forward-auth wrappers (`forwardauth_httptest_test.go`: flow resolution, ProxyProvider/Application create + idempotency, embedded-outpost find + bind, and the whole `SetupForwardAuth` flow); `internal/util` (87.5%); `main` (66%) — `CreateGroupsAndUsers` whole-flow vs a mock Authentik (`main_test.go`). `CreateGroupsAndUsers` returns `error` (not `log.Panicf`) so the flow is testable.
  - *Live e2e* (`e2e_test.go`, build tag `e2e`) — drives the full flow against a real Authentik and verifies persistence with the admin + the created user's token. Two ways to run it: `make e2e-compose` (Authentik via Docker Compose — lightweight) or `make e2e` (KinD + cloud-provider-kind — full cluster). Both read `AUTHENTIK_E2E_*` env and self-tear-down. Excluded from `make test`.
  - *Live forward-auth e2e* (`make e2e-forward-auth`, Makefile/shell-driven — not a Go test): brings up the `compose/forward-auth/` overlay, configures the provider via the POC, then asserts an unauthenticated request to whoami returns **HTTP 302** (whoami never 302s on its own, so a 302 proves the middleware intercepted the request and redirected to Authentik login). Self-tears-down.

## Skills

Infrastructure files in this repo map to these maintenance skills — run the matching skill when editing the file:

| File | Skill |
|------|-------|
| `provisioner/Makefile` | `/makefile` |
| `.github/workflows/ci.yml` | `/ci-workflow` |
| `renovate.json` | `/renovate` |
| `README.md` | `/readme` |
| `CLAUDE.md` | `/claude` |

When spawning subagents to work on any of the above, always pass the relevant skill's full conventions into the agent prompt — agents cannot read skill files themselves.

## Upgrade Backlog

Deferred / monitor items from `/upgrade-analysis` (verified current 2026-06-27). The repo is otherwise fully current — Go 1.26.4, all mise tools at latest, Authentik 2026.5 (chart 2026.5.3), postgres 18, kubectl 1.36.2 / kind 0.32 / node v1.36.1 (aligned), cloud-provider-kind 0.11.1, all Action SHAs at current major tags; `govulncheck` clean. Renovate is alive but `automerge` is **off** (`renovate.json` → `"automerge": false`) — dependency PRs require a manual merge.

- [ ] **Alternative datastores (CockroachDB / YugabyteDB) — closed, not deferred.** Both were evaluated and removed as non-working (CockroachDB lacks session `pg_advisory_lock()` — [cockroachdb#169981](https://github.com/cockroachdb/cockroach/issues/169981), open; YugabyteDB aborts Authentik's migrations on `YB001` even with every documented mitigation). Full investigation retained in `docs/spikes/authentik-cockroachdb-yugabytedb.md`. PostgreSQL is the only supported backend; no revisit planned unless that analysis's "revisit when…" conditions are met.
- [ ] **Renovate `lock-file-maintenance`** is in "Awaiting Schedule" (working as designed — periodic `go.sum` refresh); no action needed.
