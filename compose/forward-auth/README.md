# Forward-auth demo overlay

An optional add-on to the base Compose stack that demonstrates Authentik as an
**identity-aware proxy**: [Traefik](https://traefik.io/) gates a sample app
([`traefik/whoami`](https://github.com/traefik/whoami)) with its `forwardAuth`
middleware, and the Authentik side (proxy provider, application, embedded-outpost
binding) is configured **programmatically by the Go provisioner** — the same
client the core POC uses for users/groups/tokens.

## Files

| File | Purpose |
|------|---------|
| `docker-compose.forward-auth.yml` | Overlay adding `traefik` + `whoami` (`traefik/whoami`) to the base `authentik` Compose project. Image tags are pinned (and Renovate-tracked) in that file — the single source of truth. |
| `traefik/dynamic.yml` | Traefik dynamic config: the `authentik` forwardAuth middleware + a router proxying the `/outpost.goauthentik.io/` handshake paths back to the Authentik server. |

## Run it

From `provisioner/`:

```bash
make compose-forward-auth-up     # base stack + traefik + whoami, then configure forward-auth
# browse https://whoami.127-0-0-1.sslip.io  (accept the self-signed cert)
make compose-forward-auth-down   # tear down + remove volumes
make e2e-forward-auth            # fast: up -> configure -> follow the redirect -> assert the Authentik login is reachable (HTTP 200) -> down
make e2e-forward-auth-browser    # full: a real headless browser signs in -> asserts whoami serves the X-authentik-* identity headers (Playwright, ../../e2e/browser)
```

`make compose-forward-auth-up` runs the provisioner with
`AUTHENTIK_PROVISION_ORGS=false AUTHENTIK_FORWARD_AUTH_ENABLED=true`, so only the
(idempotent) forward-auth setup runs — re-running it is safe.

## How it works

1. The provisioner resolves the stock `default-provider-authorization-implicit-consent`
   and `default-provider-invalidation-flow` flows (PKs resolved at runtime, never
   hardcoded), creates a `forward_single` proxy provider + application for
   `whoami`, and binds the provider to the built-in **embedded outpost** (which
   runs in-process in `authentik-server`, so no extra container).
2. A request to `https://whoami.127-0-0-1.sslip.io` hits Traefik. The
   `authentik@file` middleware calls
   `http://server:9000/outpost.goauthentik.io/auth/traefik`.
3. Unauthenticated → Authentik replies `302`, redirecting the browser to the
   login flow at the embedded outpost's `authentik_host` (the provisioner sets
   this to `https://127.0.0.1:9443`); after login, whoami echoes the
   `X-authentik-*` identity headers Authentik injects.

Notes:
- The `127-0-0-1.sslip.io` wildcard resolves `*.127-0-0-1.sslip.io` → `127.0.0.1`
  with no `/etc/hosts` edits. TLS is Traefik's default self-signed cert (use
  `curl -k` / accept the browser warning); the login at `https://127.0.0.1:9443`
  presents Authentik's own self-signed cert — accept that one too.
- **`authentik_host` is load-bearing.** The provisioner sets the embedded
  outpost's `authentik_host` (`AUTHENTIK_FORWARD_AUTH_HOST`, default
  `https://127.0.0.1:9443`) so the login redirect targets a browser-reachable
  host. Without it the outpost falls back to `http://localhost` → 404. This is
  the *primary* `authentik_host` field; `authentik_host_browser` and
  `trustForwardHeader` do **not** control it.
- `trustForwardHeader: true` is set on the forwardAuth middleware so the outpost
  receives the original request host (for the callback). (The entrypoint's
  `forwardedHeaders.insecure` governs trusting client headers — a separate knob.)
- Login is `akadmin` plus the `AUTHENTIK_BOOTSTRAP_PASSWORD` value in
  `../.env.example`.
- Traefik dashboard: <http://127.0.0.1:8081>.
