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
make e2e-forward-auth            # up -> configure -> assert unauth request 302s to login -> down
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
3. Unauthenticated → Authentik replies `302` to its login flow; after login,
   whoami echoes the `X-authentik-*` identity headers Authentik injects.

Notes:
- The `127-0-0-1.sslip.io` wildcard resolves `*.127-0-0-1.sslip.io` → `127.0.0.1`
  with no `/etc/hosts` edits. TLS is Traefik's default self-signed cert (use
  `curl -k` / accept the browser warning).
- Login is `akadmin` plus the `AUTHENTIK_BOOTSTRAP_PASSWORD` value in
  `../.env.example`.
- Traefik dashboard: <http://127.0.0.1:8081>.
- `trustForwardHeader` is intentionally **not** set — it is deprecated since
  Traefik v3.6.14; the `websecure` entrypoint's `forwardedHeaders.insecure=true`
  is the modern replacement that lets Authentik reconstruct the original URL.
