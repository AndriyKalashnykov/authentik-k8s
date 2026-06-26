# Spike: `authentik_traefik` DNS Records — what authentik-k8s can leverage

> Research spike. Investigates [`brokenscripts/authentik_traefik`](https://github.com/brokenscripts/authentik_traefik)
> (default branch `traefik3`) with a focus on its **DNS Records** area, and maps the findings back to this
> repository (`authentik-k8s`). Recommendations are POC-scoped, version-pinned, and honest about effort.
>
> Date: 2026-06-26 · Authentik pinned here: `2026.5` · Go client: `goauthentik.io/api/v3 v3.2026050.3`

## 1. Summary

`authentik_traefik` is a **Docker-Compose reference guide** for putting [Traefik](https://traefik.io/) 3.x in
front of [Authentik](https://goauthentik.io/) and using Authentik's **embedded outpost** as a Traefik
`forwardAuth` middleware — so Traefik delegates authn/authz to Authentik for every protected host. Its
**"DNS Records" department is small but load-bearing**: it requires a public DNS record for
`authentik.domain.tld` (plus per-app CNAMEs), and it wires Traefik's **Let's Encrypt ACME `dnsChallenge`
through Cloudflare's DNS API** to mint a **wildcard `*.domain.tld` certificate** without ever exposing an
HTTP-01 challenge. The headline opportunity for `authentik-k8s` is to add a **reverse-proxy + forward-auth
ingress story** (today the repo exposes Authentik raw on a LoadBalancer with a self-signed cert) and — the
higher-value, repo-native angle — to **extend the Go provisioner to create the Proxy Provider, Application,
and embedded-outpost binding programmatically**, automating the entire click-path that `authentik_traefik`
documents by hand.

## 2. What `authentik_traefik` provides

Inventory of the repo (verified against the `traefik3` git tree, not memory):

| Area | Files (on branch `traefik3`) | What it does |
|------|------------------------------|--------------|
| Guide | `README.md` | Step-by-step: DNS Record → compose → Traefik middleware → Authentik providers/applications → users → Yubikey WebAuthn MFA → domain-wide policies |
| Traefik static config | `appdata/traefik/config/traefik.yaml` | entryPoints `web`/`websecure`, HTTP→HTTPS redirect, docker+file providers, **ACME `le` resolver with Cloudflare `dnsChallenge`**, wildcard TLS on `websecure` |
| Traefik dynamic rules | `appdata/traefik/rules/*.yaml` | `forwardAuth-authentik.yaml` + `middlewares-authentik.yaml` (the forwardAuth middleware), `chain-no-auth.yaml`, plus rate-limit / secure-headers / compress / buffering / https-redirect / tls-opts |
| Compose (modular) | `my-compose/compose.yaml` + `my-compose/{traefik,authentik,socket-proxy,whoami}/compose.yaml` | Traefik `3.0.4`, Authentik server+worker+PostgreSQL+Redis, a `socket-proxy` (read-only Docker socket), and a `whoami` demo backend |
| Compose (minimal) | `minimal_compose.yaml` | A single-file simplified variant of the stack |
| Secrets | `secrets/cf_email`, plus `*_FILE`-referenced Docker secrets in `.env` | Cloudflare email/token, Postgres creds, Authentik secret key, Gmail SMTP, MaxMind GeoIP license — all via `/run/secrets/*` |
| Env | `my-compose/.env` | `DOMAINNAME`, `PUID/PGID/TZ`, **Cloudflare IP ranges** (`CLOUDFLARE_IPS`), local CIDRs, Authentik + GeoIP config |
| Images | `images/*.png` | Screenshots of the Authentik admin click-path (providers, applications, outpost edit, MFA) |

Key technical facts (quoted/derived from fetched files):

- **Embedded outpost, not a separate proxy container.** README: *"The embedded outpost requires version
  `2021.8.1` or newer. This prevents needing the separate Forward Auth / Proxy Provider container."*
- **forwardAuth address** points at the in-cluster Authentik server's outpost endpoint:
  `http://authentik_server:9000/outpost.goauthentik.io/auth/traefik`
  (`appdata/traefik/rules/forwardAuth-authentik.yaml`).
- **Cloudflare is trusted at the edge** — Traefik `websecure.forwardedHeaders.trustedIPs` lists every
  Cloudflare CIDR + RFC1918 (`traefik.yaml`); the same list is in `.env` `CLOUDFLARE_IPS`.
- Traefik talks to Docker via a **`socket-proxy`** (`tcp://socket-proxy:2375`) rather than mounting the raw
  socket — a hardening pattern.

## 3. The "DNS Records" department — deep dive

In this project "DNS Records" spans **two distinct concerns** that both depend on public DNS:

### 3a. Hostname records (so browsers/Traefik can resolve services)

README *DNS Record* section, verbatim:

> *"Ensure that a DNS record exists for `authentik.domain.tld` as the compose and all material here assumes
> that will be the record name."*

Per the repo's stated convention, records resolve to the Traefik host (`192.168.1.26` in the examples):

| Record | Type | Target | Purpose |
|--------|------|--------|---------|
| `authentik.domain.tld` | A | host IP | The Authentik UI + outpost endpoint (the authentication URL) |
| `traefik.domain.tld` | A / CNAME | host IP / `authentik` | Traefik dashboard |
| `whoami.domain.tld` (and `whoami-test`) | CNAME | primary Traefik domain | Demo protected app |

Per-app subdomains are CNAMEs onto the primary Traefik domain, which is why a **wildcard certificate**
(`*.domain.tld`) is desirable — every new app reuses one cert and one DNS pattern.

### 3b. ACME DNS-01 challenge (so Traefik can mint a wildcard TLS cert)

This is the heart of the DNS Records area. Traefik's static config (`appdata/traefik/config/traefik.yaml`):

```yaml
certificatesResolvers:
  le:
    acme:
      email: "CHANGEME@gmail.com"
      storage: "/data/acme.json"
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1:53"
          - "8.8.8.8:53"
```

…paired with the wildcard request on the HTTPS entrypoint:

```yaml
entryPoints:
  websecure:
    address: ":443"
    http:
      tls:
        options: tls-opts@file
        certResolver: le
        domains:
          - main: "domain.tld"
            sans:
              - "*.domain.tld"
```

How it works, end to end:

1. Traefik asks Let's Encrypt for a cert covering `domain.tld` + `*.domain.tld`.
2. LE issues a **DNS-01 challenge**; Traefik's Cloudflare provider uses the **Cloudflare DNS API token**
   (`CF_DNS_API_TOKEN_FILE` → `/run/secrets/cf_dns_api_token`, with `secrets/cf_email`) to create the
   `_acme-challenge` TXT record, then removes it.
3. `resolvers: 1.1.1.1:53 / 8.8.8.8:53` are the **DNS servers Traefik queries to confirm TXT propagation**
   before telling LE to validate (a common gotcha — without explicit resolvers, propagation checks can race
   against a local/split-horizon resolver).
4. Because the challenge is DNS-based, **no HTTP-01 / port-80 exposure is needed**, and **wildcard certs**
   (which HTTP-01 cannot issue) become possible.

**Net:** the "DNS Records" area = *(public hostname records pointing at Traefik)* + *(a Cloudflare-DNS-API
ACME `dnsChallenge` that produces a wildcard TLS cert)*. It is Cloudflare-specific and assumes a real,
internet-resolvable domain with a Cloudflare-hosted zone.

## 4. What authentik-k8s can leverage / add

Context for the mapping:

- **Today** `authentik-k8s` exposes Authentik directly on a `cloud-provider-kind` **LoadBalancer** with a
  **self-signed cert** (`util.GetTLSTransport(true)` skips verification by design), and the **provisioner only
  touches CoreAPI** (groups/users/passwords/tokens — see `provisioner/internal/authentik/api.go`). It never
  creates providers, applications, or outposts.
- `authentik_traefik` is **Compose + Cloudflare + a real public domain**; KinD is a **local, ephemeral,
  no-public-DNS** environment. So the deployment-side ideas need adaptation, not a copy-paste.

Prioritized:

| # | Idea | What it adds | Side | Effort | Risk / caveats |
|---|------|-------------|------|--------|----------------|
| 1 | **Provisioner creates Proxy Provider + Application + binds embedded outpost** | Automates the entire `authentik_traefik` admin click-path (§3 of its README) via the Go client — the most repo-native, demonstrable win | Provisioner | **M** | Needs the default authorization flow PK (lookup via `FlowsInstancesList`); idempotency (list-before-create); keep TLS-skip transport |
| 2 | **Add a `forwardAuth` + `whoami` demo to the Compose stack** | Shows Traefik→Authentik forward-auth locally; mirrors `my-compose/whoami` + `forwardAuth-authentik.yaml` | Deployment (compose) | **M** | Adds Traefik + a demo backend to `compose/`; needs the outpost objects from #1 (or manual) to actually gate traffic |
| 3 | **Traefik IngressRoute + forwardAuth Middleware on KinD** | A real ingress story on k8s instead of raw LoadBalancer; CRD `Middleware` pointing at the embedded outpost service | Deployment (k8s) | **L** | New controller (Traefik) in `k8s/`; Gateway/Ingress wiring; more moving parts than the POC needs |
| 4 | **ACME `dnsChallenge` (Cloudflare/other) for real TLS** | Replace self-signed certs with trusted wildcard certs; lets the POC drop `InsecureSkipVerify` | Deployment | **L** | **Requires a real public domain + Cloudflare (or other) API token**; impossible on bare KinD/`*.local`; out of scope for an offline POC |
| 5 | **Split-horizon / local DNS note (`nip.io`, `/etc/hosts`, CoreDNS rewrite)** | Documents how to get hostnames in a no-public-DNS KinD/compose lab so forward-auth host rules work | Deployment (docs) | **S** | Doc-only; `nip.io`-style wildcard DNS or hostAliases; clearly mark as lab-only |
| 6 | **Provisioner creates an admin API token (Cloudflare-style secret externalization)** | Reinforce the existing env-driven secret pattern; not DNS-specific | Provisioner | **S** | Marginal; the bootstrap-token contract already exists |

**Recommendation:** lead with **#1** (highest value, lowest blast radius, plays to the repo's existing
provisioner strength), optionally followed by **#2** to make #1 observable end-to-end on Compose. #4 is the
"real DNS Records" feature but is **gated on a public domain** and should stay a documented option, not a POC
default. #3/#4 on KinD are L-effort and arguably beyond a POC.

## 5. Proposed POC / implementation outline

### Highest value — Idea #1: provision the forward-auth objects in Go

The Go client (`v3.2026050.3`, already in `provisioner/go.mod`) exposes everything needed — verified present
in the module cache:

- `ProvidersAPIService.ProvidersProxyCreate` (model `ProxyProviderRequest`)
- `CoreAPIService.CoreApplicationsCreate` (model `ApplicationRequest`)
- `OutpostsAPIService.OutpostsInstancesList` / `OutpostsInstancesUpdate` / `OutpostsInstancesPartialUpdate`
- `FlowsAPIService.FlowsInstancesList` (to resolve the default authorization/invalidation flow PKs)
- `ProxyMode` enum: `PROXYMODE_PROXY`, `PROXYMODE_FORWARD_SINGLE`, `PROXYMODE_FORWARD_DOMAIN`

`ProxyProviderRequest` carries the exact fields `authentik_traefik` sets by hand in the UI:
`Name`, `Mode *ProxyMode`, `ExternalHost`, `CookieDomain`, `InternalHost`, `AuthorizationFlow string`,
`InvalidationFlow string`. `ApplicationRequest` carries `Name`, `Slug`, `Provider`, `MetaLaunchUrl`.

**New wrappers** in `provisioner/internal/authentik/api.go` (mirroring the existing thin-wrapper style):

```go
// CreateForwardAuthProvider creates a domain-level or single-app forward-auth Proxy Provider.
func CreateForwardAuthProvider(ctx context.Context, apiClient *api.APIClient,
    name string, mode api.ProxyMode, externalHost, cookieDomain string,
    authzFlow, invalidationFlow string) (*api.ProxyProvider, *http.Response, error)

// CreateApplication binds an application to a provider PK.
func CreateApplication(ctx context.Context, apiClient *api.APIClient,
    name, slug string, providerPk int32, launchURL string) (*api.Application, *http.Response, error)

// BindAppToEmbeddedOutpost adds an application PK to the embedded outpost's providers list.
func BindAppToEmbeddedOutpost(ctx context.Context, apiClient *api.APIClient, providerPk int32) (*http.Response, error)
```

**Orchestration** in `provisioner/main.go` (after the existing groups/users flow), domain-wide variant:

1. `FlowsInstancesList(...)` filtered by slug → resolve `default-authentication-flow` /
   `default-provider-invalidation-flow` PKs (do **not** hardcode PKs — they differ per instance).
2. `CreateForwardAuthProvider(ctx, c, "Domain Forward Auth Provider", api.PROXYMODE_FORWARD_DOMAIN, "https://"+host, cookieDomain, authzFlow, invFlow)`.
3. `CreateApplication(ctx, c, "Domain Forward Auth Application", "domain-forward-auth-application", provider.Pk, "")`.
4. `OutpostsInstancesList(...)` → find the embedded outpost (`managed == "goauthentik.io/outposts/embedded"`),
   append the new provider PK, `OutpostsInstancesUpdate(...)`.
5. **Idempotency:** each step lists-by-name/slug first and skips if present (the repo already favors this
   pattern; `ListUser` exists). Return `error` rather than `log.Panicf`, consistent with `CreateGroupsAndUsers`,
   so `main_test.go`-style mock-flow tests can cover it.

**Config additions** (`provisioner/.env.example` — the committed source of truth, per the repo's
parameter-externalization rule; no hardcoded hosts/domains):

```
AUTHENTIK_FORWARD_AUTH_ENABLED=false          # opt-in; default off so existing POC behavior is unchanged
AUTHENTIK_COOKIE_DOMAIN=domain.tld
AUTHENTIK_FORWARD_AUTH_EXTERNAL_HOST=https://authentik.domain.tld
AUTHENTIK_DEFAULT_AUTHZ_FLOW_SLUG=default-provider-authorization-implicit-consent
AUTHENTIK_DEFAULT_INVALIDATION_FLOW_SLUG=default-provider-invalidation-flow
```

**Tests:** extend the existing **hermetic `httptest` contract tests**
(`provisioner/internal/authentik/api_httptest_test.go`) with the real paths
`/api/v3/providers/proxy/`, `/api/v3/core/applications/`, `/api/v3/outposts/instances/` — asserting method,
path, and body shape, exactly as the current CoreAPI wrappers are tested. No live Authentik required for
`make test`.

**What stays declarative (NOT automated by Go):** Traefik's own static/dynamic config (the `forwardAuth`
middleware address, entrypoints, ACME) is Traefik-owned and belongs in compose/k8s manifests — the provisioner
configures **Authentik's** side of the contract, Traefik config configures **Traefik's** side.

### Optional follow-on — Idea #2: observable forward-auth on Compose

- Add a `traefik` service + a `whoami` backend to `compose/` (model on `my-compose/traefik/compose.yaml` and
  `my-compose/whoami/compose.yaml`), plus a `forwardAuth-authentik.yaml` dynamic rule pointing at
  `http://authentik-server:9000/outpost.goauthentik.io/auth/traefik`.
- For local TLS, use a **self-signed/`mkcert`** path (keep `InsecureSkipVerify` for the POC) — **do not** wire
  Cloudflare ACME here; that needs a public domain (see Idea #4, deferred).
- All ports/hosts/domains via `compose/.env.example` `${VAR:-default}` (repo convention).

## 6. Open questions / risks

| Topic | Note |
|-------|------|
| **Version discipline** | Confirm `forwardAuth` + embedded-outpost behavior on **Authentik 2026.5** (this repo) vs the guide's 2024-era assumptions, and on **current Traefik** (the guide pins `traefik:3.0.4`, June 2024 — newer 3.x exists). Read the **2026.5** outpost docs before wiring; do not transcribe flow slugs/PKs from the guide — resolve them at runtime via `FlowsInstancesList`. |
| **Flow slugs are not stable contracts** | Default flow slugs can change across Authentik releases. The provisioner must **look them up**, not pin them; treat the `.env` slug values as overridable hints with a list-and-resolve fallback. |
| **DNS Records are Cloudflare- and public-domain-specific** | The guide's ACME `dnsChallenge` requires a real internet-resolvable zone + Cloudflare API token. **KinD/compose labs have no public DNS** → Idea #4 cannot be a default; needs a user decision on whether a real domain is in scope. Alternatives: `nip.io`-style wildcard DNS, `/etc/hosts`/`hostAliases`, or a different lego DNS provider. |
| **k8s outpost endpoint** | On k8s the forwardAuth address must target the Authentik **Service** DNS (e.g. `http://authentik-server.default.svc:9000/outpost.goauthentik.io/auth/traefik`), not the compose name `authentik_server`. Reconcile with the existing `namespace: default` vs `threeport-api` mismatch noted in `CLAUDE.md`. |
| **Secret handling** | If Idea #4 is pursued, the Cloudflare token must follow the repo's externalization + no-secret-in-argv rules (Docker secret / k8s Secret / `*_FILE`, never inline). |
| **TLS-skip coupling** | `InsecureSkipVerify` is intentional for self-signed dev certs. Only an ACME-trusted-cert path (Idea #4) justifies removing it; don't change it for Ideas #1–#3. |
| **Scope creep** | Ideas #3/#4 (Traefik on KinD + real ACME) are **L** effort and arguably beyond a POC. Recommend #1 (+ optional #2) and keep #3/#4 documented-but-deferred. |

## 7. References

- `authentik_traefik` repo (default branch `traefik3`): <https://github.com/brokenscripts/authentik_traefik>
- README (rendered): <https://github.com/brokenscripts/authentik_traefik/blob/traefik3/README.md>
- Traefik static config (ACME `dnsChallenge` / Cloudflare / wildcard TLS): <https://github.com/brokenscripts/authentik_traefik/blob/traefik3/appdata/traefik/config/traefik.yaml>
- forwardAuth dynamic rule: <https://github.com/brokenscripts/authentik_traefik/blob/traefik3/appdata/traefik/rules/forwardAuth-authentik.yaml>
- Traefik compose service (Cloudflare token secret): <https://github.com/brokenscripts/authentik_traefik/blob/traefik3/my-compose/traefik/compose.yaml>
- whoami demo compose: <https://github.com/brokenscripts/authentik_traefik/blob/traefik3/my-compose/whoami/compose.yaml>
- `.env` (Cloudflare IPs, Authentik config): <https://github.com/brokenscripts/authentik_traefik/blob/traefik3/my-compose/.env>
- Traefik ACME DNS challenge docs: <https://doc.traefik.io/traefik/https/acme/>
- Authentik Traefik / forward-auth (proxy provider) docs: <https://docs.goauthentik.io/docs/providers/proxy/>
- Authentik Go client: <https://github.com/goauthentik/client-go> (`goauthentik.io/api/v3 v3.2026050.3`)
- This repo's provisioner CoreAPI wrappers: `provisioner/internal/authentik/api.go`

---

*Sources cited above were fetched during this spike (2026-06-26). Where a file could not be retrieved it is
not cited. Authentik object names/slugs in §5 follow the `authentik_traefik` guide and must be re-verified
against Authentik 2026.5 before implementation.*
