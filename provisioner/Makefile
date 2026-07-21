.DEFAULT_GOAL := help

APP_NAME := provisioner
GOFLAGS  ?= -mod=mod

# --- Image ---
IMAGE_REPO ?= provisioner
IMAGE_TAG  ?= dev
IMAGE      := $(IMAGE_REPO):$(IMAGE_TAG)

# --- Compose (Authentik stack — lightweight local alternative to KinD) ---
COMPOSE_DIR := ../compose
COMPOSE     := docker compose --project-directory $(COMPOSE_DIR) -f $(COMPOSE_DIR)/docker-compose.yml
# Base stack + the Traefik/whoami forward-auth overlay.
COMPOSE_FA  := $(COMPOSE) -f $(COMPOSE_DIR)/forward-auth/docker-compose.forward-auth.yml
WHOAMI_URL  ?= https://whoami.127-0-0-1.sslip.io
# Playwright full-login browser e2e (e2e-forward-auth-browser).
BROWSER_E2E_DIR        := ../e2e/browser
AK_ADMIN_USER          ?= akadmin
# Extra flags for `playwright install` — CI sets this to `--with-deps` (needs sudo,
# available on the runner) to pull the chromium OS libraries; empty locally.
PLAYWRIGHT_BROWSER_DEPS ?=

# --- KinD e2e ---
KIND_CLUSTER_NAME ?= authentik-e2e
KUBECTL           := kubectl --context=kind-$(KIND_CLUSTER_NAME)
MANIFEST          := ../k8s/postgresql/authentik-postgresql.yml
K8S_DIR           := ../k8s/postgresql
KIND_CONFIG       := ../k8s/kind-config.yaml
AUTHENTIK_NS      := default
HELM_REPO_URL     := https://charts.goauthentik.io
# Authentik Helm chart version — the SOURCE for `make k8s-generate`. The committed
# k8s manifest is regenerated from this chart + k8s/postgresql/values.yml; a CI
# drift gate (k8s-drift) fails if they're out of sync. Chart version == app version.
# renovate: datasource=helm depName=authentik registryUrl=https://charts.goauthentik.io
AUTHENTIK_CHART_VERSION := 2026.5.5
# Track the REGISTRY image (the consumed sink), not the GitHub release: registry.k8s.io
# image promotion lags the upstream release, so github-releases proposes un-pullable tags.
# The `# renovate:` line MUST stay immediately above the constant (customManager adjacency).
# renovate: datasource=docker depName=registry.k8s.io/cloud-provider-kind/cloud-controller-manager extractVersion=^v(?<version>.*)$
CLOUD_PROVIDER_KIND_VERSION := 0.11.1
# renovate: datasource=docker depName=kindest/node
KIND_NODE_IMAGE := kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5

# mermaid-cli is a docker-only tool (no mise backend); pinned here, Renovate-tracked
# via the Makefile customManager. Used by `mermaid-lint` to validate README diagrams.
# renovate: datasource=docker depName=minlag/mermaid-cli
MERMAID_CLI_VERSION := 11.16.0

# dclint (docker-compose-linter) is a docker-only tool (no mise backend); pinned
# here, Renovate-tracked via the Makefile customManager. Used by `compose-lint`
# to validate the Compose file (rule config in repo-root .dclintrc.yaml).
# renovate: datasource=docker depName=zavoloklom/dclint
DCLINT_VERSION := 3.1.0

# Make recipes don't source shell rc files; put mise's shims dir on PATH so
# every mise-managed tool (go, golangci-lint, govulncheck, hadolint, kind,
# kubectl) resolves inside recipes.
export PATH := $(HOME)/.local/share/mise/shims:$(HOME)/.local/bin:$(PATH)

# Go version is mirrored in go.mod, .mise.toml and Dockerfile — read back here
# only to verify they agree (check-toolchain-alignment). awk avoids depending
# on a particular grep flavor.
GO_MOD_VERSION    := $(shell awk '/^go /{print $$2; exit}' go.mod)
MISE_GO_VERSION   := $(shell awk -F'"' '/^go[[:space:]]*=/{print $$2; exit}' .mise.toml)
DOCKER_GO_VERSION := $(shell awk -F= '/^ARG GO_VERSION=/{print $$2; exit}' Dockerfile)

#help: @ List available tasks
help:
	@echo "Usage: make COMMAND"
	@echo "Commands :"
	@grep -E '[a-zA-Z\.\-]+:.*?@ .*$$' $(MAKEFILE_LIST) | tr -d '#' | awk 'BEGIN {FS = ":.*?@ "}; {printf "\033[32m%-28s\033[0m - %s\n", $$1, $$2}'

#deps: @ Install the pinned toolchain (Go + lint/vuln/kind/kubectl tools) via mise
deps:
	@if [ -z "$$CI" ] && ! command -v mise >/dev/null 2>&1; then \
		echo "Installing mise (no root required; installs to ~/.local/bin)..."; \
		curl -fsSL https://mise.run | sh; \
		echo ""; \
		echo "mise installed. Activate it in your shell, then re-run 'make deps':"; \
		echo "  echo 'eval \"\$$(~/.local/bin/mise activate bash)\"' >> ~/.bashrc"; \
		exit 0; \
	fi
	@mise install --yes

#deps-check: @ Show pinned tools and their install status
deps-check:
	@command -v mise >/dev/null 2>&1 && mise list || echo "mise not installed — run 'make deps'"

#check-toolchain-alignment: @ Verify the Go version matches across go.mod, .mise.toml and Dockerfile
check-toolchain-alignment:
	@if [ "$(GO_MOD_VERSION)" != "$(MISE_GO_VERSION)" ] || [ "$(GO_MOD_VERSION)" != "$(DOCKER_GO_VERSION)" ]; then \
		echo "ERROR: Go version mismatch:"; \
		printf "  %-12s %s\n" go.mod "$(GO_MOD_VERSION)" .mise.toml "$(MISE_GO_VERSION)" Dockerfile "$(DOCKER_GO_VERSION)"; \
		exit 1; \
	fi

#build: @ Compile the POC binary into bin/
build: deps
	@export GOFLAGS=$(GOFLAGS) CGO_ENABLED=0; go build -o bin/$(APP_NAME) .

#run: @ Run the POC against a running Authentik (loads .env.example then .env)
run: deps
	@set -a; [ -f .env.example ] && . ./.env.example; [ -f .env ] && . ./.env; set +a; \
	 export GOFLAGS=$(GOFLAGS) TZ=UTC; go run main.go

#test: @ Run unit + httptest contract tests (race detector + coverage)
test: deps
	@export GOFLAGS=$(GOFLAGS) TZ=UTC; go test -race -cover ./...

#lint: @ Run golangci-lint and verify go.mod/go.sum are tidy
lint: deps
	@golangci-lint run ./...
	@export GOFLAGS=$(GOFLAGS); go mod tidy && git diff --exit-code go.mod go.sum || { echo "Error: go.mod/go.sum not tidy. Run 'go mod tidy'."; exit 1; }

#lint-docker: @ Lint the Dockerfile with hadolint
lint-docker: deps
	@hadolint Dockerfile

#vulncheck: @ Scan dependencies for known vulnerabilities
vulncheck: deps
	@govulncheck ./...

#trivy-fs: @ Filesystem CVE scan of dependencies (HIGH/CRITICAL, fixed-only)
trivy-fs: deps
	@trivy fs --scanners vuln --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1 --no-progress --quiet ..

#cve-scan: @ CVE scanning (govulncheck + trivy filesystem). CI runs this on release TAGS only, not on every PR — a freshly-disclosed CVE in the vuln DB must not block unrelated PRs; it gates the release (publish).
cve-scan: vulncheck trivy-fs
	@echo "CVE scan passed."

#secrets: @ Scan the working tree for leaked secrets (gitleaks; demo creds allowlisted in .gitleaks.toml)
# The scan MUST run with cwd = repo root: gitleaks reports finding paths relative to cwd, and
# .gitleaks.toml anchors its allowlist at the scan root (`^k8s/.+\.ya?ml$`, `(^|/)\.env…`).
# Running it from here with `--source ..` instead yields `../k8s/…`, which those anchors miss —
# measured: 4 intentional demo credentials resurface as "leaks".
# But mise's config lives HERE (provisioner/.mise.toml; there is no root-level mise config), so
# the binary must be resolved BEFORE leaving this directory. Once mise-action puts SHIMS on PATH
# (v4.2.1+, upstream b107e20a "exclude PATH from environment export"), a shim invoked from the
# repo root cannot resolve a version -> `mise ERROR No version is set for shim: gitleaks`.
# `mise which` prints the absolute install path, which is cwd-independent.
secrets: deps
	@GITLEAKS="$$(mise which gitleaks 2>/dev/null || command -v gitleaks)"; \
	case "$$GITLEAKS" in */mise/shims/*) GITLEAKS="";; esac; \
	[ -x "$$GITLEAKS" ] || { echo "ERROR: could not resolve a real gitleaks binary (a mise SHIM is not usable here — it is cwd-dependent); run 'make deps'"; exit 1; }; \
	cd .. && "$$GITLEAKS" detect --no-git --source . -c .gitleaks.toml --redact --no-banner

#mermaid-lint: @ Validate README/CLAUDE Mermaid diagrams render cleanly (minlag/mermaid-cli)
mermaid-lint:
	@command -v docker >/dev/null 2>&1 || { echo "ERROR: docker is required for mermaid-lint"; exit 1; }
	@set -e; \
	MD_FILES=$$(cd .. && grep -lF '```mermaid' README.md CLAUDE.md 2>/dev/null || true); \
	if [ -z "$$MD_FILES" ]; then echo "No Mermaid blocks found — skipping."; exit 0; fi; \
	IMAGE=minlag/mermaid-cli:$(MERMAID_CLI_VERSION); \
	FAILED=0; \
	for md in $$MD_FILES; do \
		echo "Validating Mermaid blocks in $$md..."; \
		if docker run --rm -v "$(CURDIR)/..:/data:ro" "$$IMAGE" \
			-i "/data/$$md" -o "/tmp/$$(basename $$md .md).svg" >/tmp/mmlint.log 2>&1; then \
			echo "  OK"; \
		else \
			echo "  PARSE ERROR in $$md:"; cat /tmp/mmlint.log; FAILED=1; \
		fi; \
	done; \
	[ $$FAILED -eq 0 ] || { echo "mermaid-lint: FAILED"; exit 1; }
	@echo "mermaid-lint: OK"

#compose-lint: @ Lint the Compose file (zavoloklom/dclint; rules in .dclintrc.yaml, --max-warnings 0)
compose-lint:
	@command -v docker >/dev/null 2>&1 || { echo "ERROR: docker is required for compose-lint"; exit 1; }
	@docker run --rm -v "$(CURDIR)/..:/w:ro" -w /w zavoloklom/dclint:$(DCLINT_VERSION) \
		--max-warnings 0 compose/docker-compose.yml compose/forward-auth/docker-compose.forward-auth.yml
	@echo "compose-lint: OK"

#static-check: @ Composite quality gate (alignment + lint + hadolint + mermaid + compose + secrets). CVE scans (vulncheck + trivy-fs) run on release tags only — see cve-scan.
static-check: deps check-toolchain-alignment lint lint-docker mermaid-lint compose-lint secrets
	@echo "Static check passed."

#image-build: @ Build the POC container image
image-build: deps
	@docker build -t $(IMAGE) -f Dockerfile .

#image-run: @ Run the POC image against a running Authentik (loads .env / .env.example)
image-run: image-build
	@ENVFILE=$$([ -f .env ] && echo .env || echo .env.example); \
	 docker run --rm --env-file "$$ENVFILE" $(IMAGE)

#image-scan: @ Build the image and scan it for HIGH/CRITICAL CVEs (Trivy; what CI's docker job runs)
image-scan: image-build
	@trivy image --severity HIGH,CRITICAL --ignore-unfixed --exit-code 1 --no-progress --quiet $(IMAGE)
	@echo "image-scan: OK"

#update: @ Update dependencies to latest versions
update: deps
	@export GOFLAGS=$(GOFLAGS); go get -u ./... && go mod tidy

#clean: @ Remove build artifacts
clean:
	@rm -rf bin/

#renovate-validate: @ Validate renovate.json
renovate-validate:
	@npx --yes --package renovate renovate-config-validator ../renovate.json

# ---------------------------------------------------------------------------
# Authentik via Docker Compose (lightweight local alternative to KinD)
# ---------------------------------------------------------------------------

#compose-up: @ Start Authentik (PostgreSQL+server+worker) via Compose and wait until ready
compose-up:
	@[ -f $(COMPOSE_DIR)/.env ] || cp $(COMPOSE_DIR)/.env.example $(COMPOSE_DIR)/.env
	@$(COMPOSE) up -d
	@echo "Waiting for Authentik server readiness (https://127.0.0.1:9443/-/health/ready/)..."
	@for i in $$(seq 1 60); do \
		code=$$(curl -sk -o /dev/null -w '%{http_code}' --max-time 5 https://127.0.0.1:9443/-/health/ready/ 2>/dev/null || true); \
		if [ "$$code" = "204" ] || [ "$$code" = "200" ]; then echo "Authentik ready (HTTP $$code)"; exit 0; fi; \
		sleep 5; \
	done; \
	echo "ERROR: Authentik not ready within 300s"; $(COMPOSE) ps; $(COMPOSE) logs --tail=50 server; exit 1

#compose-down: @ Stop the Authentik compose stack and remove volumes
compose-down:
	@$(COMPOSE) down -v

#compose-logs: @ Follow Authentik compose logs
compose-logs:
	@$(COMPOSE) logs -f

# ---------------------------------------------------------------------------
# Forward-auth demo (Traefik + traefik/whoami gated by Authentik)
# ---------------------------------------------------------------------------

#compose-forward-auth-up: @ Start Authentik + Traefik + whoami and configure forward-auth (browse $(WHOAMI_URL))
compose-forward-auth-up: deps
	@[ -f $(COMPOSE_DIR)/.env ] || cp $(COMPOSE_DIR)/.env.example $(COMPOSE_DIR)/.env
	@$(COMPOSE_FA) up -d
	@echo "Waiting for Authentik server readiness (https://127.0.0.1:9443/-/health/ready/)..."
	@for i in $$(seq 1 60); do \
		code=$$(curl -sk -o /dev/null -w '%{http_code}' --max-time 5 https://127.0.0.1:9443/-/health/ready/ 2>/dev/null || true); \
		if [ "$$code" = "204" ] || [ "$$code" = "200" ]; then echo "Authentik ready (HTTP $$code)"; break; fi; \
		if [ "$$i" = "60" ]; then echo "ERROR: Authentik not ready within 300s"; $(COMPOSE_FA) ps; exit 1; fi; \
		sleep 5; \
	done
	@echo "Waiting for default blueprint flows to import (the worker imports them after server-ready)..."
	@set -a; . $(COMPOSE_DIR)/.env; set +a; \
	 for i in $$(seq 1 60); do \
	   if printf 'Authorization: Bearer %s' "$$AUTHENTIK_BOOTSTRAP_TOKEN" \
	        | curl -sk -H @- \
	            "https://127.0.0.1:9443/api/v3/flows/instances/?slug=default-provider-authorization-implicit-consent" \
	            2>/dev/null | grep -q 'default-provider-authorization-implicit-consent'; then echo "blueprint flows ready"; break; fi; \
	   if [ "$$i" = "60" ]; then echo "ERROR: authorization flow not imported within 300s"; $(COMPOSE_FA) logs --tail=40 worker; exit 1; fi; \
	   sleep 5; \
	 done
	@echo "Configuring forward-auth (proxy provider + application + embedded outpost binding)..."
	@set -a; . $(COMPOSE_DIR)/.env; set +a; \
	 AUTHENTIK_SCHEME=https AUTHENTIK_HOST=127.0.0.1:9443 \
	 AUTHENTIK_BOOTSTRAP_TOKEN="$$AUTHENTIK_BOOTSTRAP_TOKEN" \
	 AUTHENTIK_PROVISION_ORGS=false AUTHENTIK_FORWARD_AUTH_ENABLED=true \
	 go run .
	@echo "Waiting for the embedded outpost to serve the new authentik_host (login reachable)..."
	@for i in $$(seq 1 30); do \
	   code=$$(curl -Lsk -o /tmp/fa-ready.html -w '%{http_code}' --max-time 10 "$(WHOAMI_URL)/" 2>/dev/null || true); \
	   if [ "$$code" = "200" ] && grep -q 'ak-flow-executor' /tmp/fa-ready.html 2>/dev/null; then echo "login reachable (outpost config live)"; break; fi; \
	   if [ "$$i" = "30" ]; then echo "ERROR: login not reachable within 60s (outpost authentik_host may not have propagated)"; exit 1; fi; \
	   sleep 2; \
	 done
	@echo ""
	@echo "Forward-auth demo ready. Browse $(WHOAMI_URL) (accept the self-signed cert);"
	@echo "Traefik redirects to the Authentik login (akadmin / the bootstrap password),"
	@echo "then whoami echoes the X-authentik-* identity headers. Traefik dashboard: http://127.0.0.1:8081"

#compose-forward-auth-down: @ Stop the forward-auth demo stack and remove volumes
compose-forward-auth-down:
	@$(COMPOSE_FA) down -v

#e2e-forward-auth: @ E2E: up -> configure forward-auth -> assert unauth request REACHES the Authentik login -> down
e2e-forward-auth: deps
	@trap '$(COMPOSE_FA) down -v >/dev/null 2>&1 || true' EXIT INT TERM; \
	 $(MAKE) --no-print-directory compose-forward-auth-up && { \
	   echo "Asserting an unauthenticated request to whoami FOLLOWS THROUGH to the Authentik login..."; \
	   url="$(WHOAMI_URL)"; \
	   for i in $$(seq 1 30); do \
	     code=$$(curl -Lsk -o /tmp/fa-login.html -w '%{http_code}' --max-time 10 "$$url/" 2>/dev/null || true); \
	     if [ "$$code" = "200" ] && grep -q 'ak-flow-executor' /tmp/fa-login.html 2>/dev/null; then \
	       echo "PASS: unauth request to $$url followed the redirect chain to the Authentik login (HTTP 200, flow executor present)"; exit 0; fi; \
	     sleep 2; \
	   done; \
	   echo "FAIL: $$url did not reach a working Authentik login (last HTTP $$code; the embedded outpost likely redirects to an unreachable host — check authentik_host)"; \
	   echo "--- final page (head) ---"; head -c 400 /tmp/fa-login.html 2>/dev/null; echo; \
	   $(COMPOSE_FA) logs --tail=40 traefik server; exit 1; \
	 }

#e2e-forward-auth-browser: @ E2E: real browser completes the Authentik login -> whoami serves identity headers -> down (Playwright)
# node/npm/npx are mise-managed (node is pinned in .mise.toml, which lives HERE) but are
# invoked after `cd $(BROWSER_E2E_DIR)` — a directory OUTSIDE this one, with no mise config
# above it. So the old `command -v node` guard checked node HERE and the recipe used node
# THERE: a guard that cannot vouch for the thing it guards. Since mise-action v4.2.1 puts
# SHIMS on PATH (upstream b107e20a stopped exporting mise env's PATH), a shim resolves its
# version from the config applying to the CURRENT directory — which is why the sibling
# `secrets` target broke the same way.
# Fix: resolve node's real bin dir BEFORE leaving and prepend it to PATH, so node AND the
# npm/npx that internally re-invoke it all resolve cwd-independently.
# NOTE: the node resolution above is defensive — CI passes this target today, so no RED was
# demonstrated for it; the guard/use mismatch is the defect being fixed.
#
# The `( ... )` subshell around the cd, by contrast, fixes a MEASURED leak. The EXIT trap
# runs in THIS shell, and a bare `cd $(BROWSER_E2E_DIR)` left that shell in e2e/browser — so
# the trap's `$(COMPOSE_FA)` relative paths resolved to e2e/compose/docker-compose.yml
# (nonexistent), `docker compose down` failed, and `|| true` swallowed it. Every run leaked
# the whole stack (5 containers + volumes) silently. Verified: running the trap's exact
# command from e2e/browser errors with "open .../e2e/compose/docker-compose.yml: no such
# file or directory". The subshell keeps the trap's cwd at provisioner/, where those
# relative paths are correct. Do NOT flatten it back to a bare `cd`.
e2e-forward-auth-browser: deps
	@NODE_BIN="$$(mise which node 2>/dev/null || command -v node)"; \
	case "$$NODE_BIN" in */mise/shims/*) NODE_BIN="";; esac; \
	[ -x "$$NODE_BIN" ] || { echo "could not resolve a real node binary (a mise SHIM is not usable here — it is cwd-dependent, and prepending the shims dir is a no-op); run 'make deps'"; exit 1; }; \
	PATH="$$(dirname "$$NODE_BIN"):$$PATH"; export PATH; \
	trap '$(COMPOSE_FA) down -v >/dev/null 2>&1 || true' EXIT INT TERM; \
	$(MAKE) --no-print-directory compose-forward-auth-up && { \
	  set -a; . $(COMPOSE_DIR)/.env; set +a; \
	  ( cd $(BROWSER_E2E_DIR) && npm ci && npx playwright install $(PLAYWRIGHT_BROWSER_DEPS) chromium && \
	    APP_URL=$(WHOAMI_URL)/ AK_ADMIN_USER=$(AK_ADMIN_USER) AK_ADMIN_PASS="$$AUTHENTIK_BOOTSTRAP_PASSWORD" \
	    npx playwright test ); \
	}

# ---------------------------------------------------------------------------
# End-to-end test (build-tagged Go test against a live Authentik)
# ---------------------------------------------------------------------------

#e2e-compose: @ E2E against Authentik on Docker Compose (up -> provision+verify -> down)
e2e-compose: deps
	@trap '$(COMPOSE) down -v >/dev/null 2>&1 || true' EXIT INT TERM; \
	 $(MAKE) --no-print-directory compose-up && { \
	   set -a; . $(COMPOSE_DIR)/.env; set +a; \
	   AUTHENTIK_E2E_SCHEME=https \
	   AUTHENTIK_E2E_HOST=127.0.0.1:9443 \
	   AUTHENTIK_E2E_TOKEN="$$AUTHENTIK_BOOTSTRAP_TOKEN" \
	   AUTHENTIK_E2E_PASSWORD="$$AUTHENTIK_BOOTSTRAP_PASSWORD" \
	   go test -tags=e2e -count=1 -v -run TestE2E ./...; \
	 }

# ---------------------------------------------------------------------------
# KinD-based e2e (full-cluster, prod-parity alternative to Compose)
# ---------------------------------------------------------------------------

#deps-kind: @ Verify kind + kubectl are present (installed via mise)
deps-kind: deps
	@command -v kind >/dev/null 2>&1 || { echo "kind missing — run 'make deps'"; exit 1; }
	@command -v kubectl >/dev/null 2>&1 || { echo "kubectl missing — run 'make deps'"; exit 1; }

#kind-create: @ Create the KinD cluster + cloud-provider-kind LoadBalancer controller
kind-create: deps-kind
	@if kind get clusters 2>/dev/null | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		echo "KinD cluster '$(KIND_CLUSTER_NAME)' already exists"; \
	else \
		kind create cluster --name $(KIND_CLUSTER_NAME) --image $(KIND_NODE_IMAGE) --config $(KIND_CONFIG) --wait 120s; \
	fi
	@docker rm -f cloud-provider-kind >/dev/null 2>&1 || true
	@docker run -d --rm --name cloud-provider-kind --network kind \
		-v /var/run/docker.sock:/var/run/docker.sock \
		registry.k8s.io/cloud-provider-kind/cloud-controller-manager:v$(CLOUD_PROVIDER_KIND_VERSION) >/dev/null

#k8s-generate: @ Regenerate the k8s manifest from the pinned Authentik chart + OSS datastores
k8s-generate: deps
	@helm repo add authentik $(HELM_REPO_URL) >/dev/null 2>&1 || true; helm repo update authentik >/dev/null 2>&1
	@{ helm template authentik authentik/authentik --version $(AUTHENTIK_CHART_VERSION) -f $(K8S_DIR)/values.yml --namespace $(AUTHENTIK_NS) \
	   | sed -E '/^[[:space:]]*helm\.sh\/chart:/d; /^[[:space:]]*app\.kubernetes\.io\/version:/d; /^[[:space:]]*app\.kubernetes\.io\/managed-by:[[:space:]]*"?Helm"?[[:space:]]*$$/d'; \
	   cat $(K8S_DIR)/oss-datastores.yml; } > $(MANIFEST)
	@echo "k8s-generate: wrote $(MANIFEST)"

#k8s-generate-check: @ Drift gate — fail if the committed k8s manifest is out of sync with the chart + values.yml
k8s-generate-check: deps
	@helm repo add authentik $(HELM_REPO_URL) >/dev/null 2>&1 || true; helm repo update authentik >/dev/null 2>&1
	@{ helm template authentik authentik/authentik --version $(AUTHENTIK_CHART_VERSION) -f $(K8S_DIR)/values.yml --namespace $(AUTHENTIK_NS) \
	   | sed -E '/^[[:space:]]*helm\.sh\/chart:/d; /^[[:space:]]*app\.kubernetes\.io\/version:/d; /^[[:space:]]*app\.kubernetes\.io\/managed-by:[[:space:]]*"?Helm"?[[:space:]]*$$/d'; \
	   cat $(K8S_DIR)/oss-datastores.yml; } > /tmp/k8s-gen-check.yml
	@if diff -u $(MANIFEST) /tmp/k8s-gen-check.yml >/dev/null 2>&1; then \
	   echo "k8s-generate-check: in sync"; \
	 else \
	   echo "DRIFT: $(MANIFEST) is stale vs the pinned chart + values.yml — run 'make k8s-generate' and commit:"; \
	   diff -u $(MANIFEST) /tmp/k8s-gen-check.yml | head -50; exit 1; \
	 fi

#kind-deploy: @ Deploy Authentik to the KinD cluster and wait until ready
kind-deploy: kind-create
	@$(KUBECTL) -n $(AUTHENTIK_NS) apply --server-side --force-conflicts -f $(MANIFEST)
	@echo "Waiting for PostgreSQL (OSS postgres:18)..."
	@$(KUBECTL) -n $(AUTHENTIK_NS) rollout status deployment/authentik-postgresql --timeout=180s
	@echo "Waiting for Authentik server + worker..."
	@$(KUBECTL) -n $(AUTHENTIK_NS) rollout status deployment/authentik-server --timeout=300s
	@$(KUBECTL) -n $(AUTHENTIK_NS) rollout status deployment/authentik-worker --timeout=300s
	@echo "Exposing authentik-server via LoadBalancer..."
	@$(KUBECTL) -n $(AUTHENTIK_NS) patch svc authentik-server --type=merge -p '{"spec":{"type":"LoadBalancer"}}'
	@for i in $$(seq 1 60); do \
		ip=$$($(KUBECTL) -n $(AUTHENTIK_NS) get svc authentik-server -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null); \
		if [ -n "$$ip" ]; then echo "LoadBalancer IP: $$ip"; exit 0; fi; \
		sleep 2; \
	done; echo "ERROR: authentik-server got no LoadBalancer IP"; exit 1

#e2e: @ E2E against Authentik on KinD (create -> deploy -> provision+verify -> destroy)
e2e: deps-kind
	@trap '$(MAKE) --no-print-directory kind-destroy >/dev/null 2>&1 || true' EXIT INT TERM; \
	 $(MAKE) --no-print-directory kind-deploy && { \
	   ip=$$($(KUBECTL) -n $(AUTHENTIK_NS) get svc authentik-server -o jsonpath='{.status.loadBalancer.ingress[0].ip}'); \
	   echo "Polling LoadBalancer route https://$$ip:443/-/health/ready/ ..."; \
	   ready=""; \
	   for i in $$(seq 1 60); do \
	     code=$$(curl -sk -o /dev/null -w '%{http_code}' --max-time 5 https://$$ip:443/-/health/ready/ 2>/dev/null || true); \
	     if [ "$$code" = "204" ] || [ "$$code" = "200" ]; then ready=1; break; fi; sleep 2; \
	   done; \
	   if [ -z "$$ready" ]; then echo "ERROR: LB route not reachable"; exit 1; fi; \
	   AUTHENTIK_E2E_SCHEME=https AUTHENTIK_E2E_HOST="$$ip:443" \
	   go test -tags=e2e -count=1 -v -run TestE2E ./...; \
	 }

#kind-destroy: @ Delete the KinD cluster, stop cloud-provider-kind, prune kindccm-* sidecars
kind-destroy:
	@docker rm -f cloud-provider-kind >/dev/null 2>&1 || true
	@orphans=$$(docker ps -aq --filter name=kindccm- 2>/dev/null); \
	 if [ -n "$$orphans" ]; then docker rm -f $$orphans >/dev/null 2>&1 || true; fi
	@kind delete cluster --name $(KIND_CLUSTER_NAME) >/dev/null 2>&1 || true

#kind-up: @ Alias for kind-deploy
kind-up: kind-deploy

#kind-down: @ Alias for kind-destroy
kind-down: kind-destroy

#ci: @ Full local CI pipeline (fail-fast order)
ci: static-check test build
	@echo "Local CI pipeline passed."

.PHONY: help deps deps-check check-toolchain-alignment build run test lint lint-docker \
	vulncheck cve-scan mermaid-lint compose-lint trivy-fs secrets static-check image-build image-run image-scan k8s-generate k8s-generate-check update clean renovate-validate \
	compose-up compose-down compose-logs \
	compose-forward-auth-up compose-forward-auth-down e2e-forward-auth e2e-forward-auth-browser \
	e2e-compose deps-kind kind-create kind-deploy \
	e2e kind-destroy kind-up kind-down ci
