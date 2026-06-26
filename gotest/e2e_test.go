//go:build e2e

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/AndriyKalashnykov/authentik-k8s/gotest/internal/authentik"
	api "goauthentik.io/api/v3"
)

// TestE2E_ProvisionAndVerify drives the full provisioning flow against a LIVE
// Authentik (Compose via `make e2e-compose`, or KinD via `make e2e`) and then
// independently verifies persistence with the admin token and the created
// user's own token. Config comes from env (AUTHENTIK_E2E_*); build-tagged `e2e`
// so it never runs in `make test`.
func TestE2E_ProvisionAndVerify(t *testing.T) {
	host := os.Getenv("AUTHENTIK_E2E_HOST")
	if host == "" {
		t.Skip("AUTHENTIK_E2E_HOST not set; run via `make e2e-compose` or `make e2e`")
	}
	scheme := env("AUTHENTIK_E2E_SCHEME", defaultScheme)
	token := env("AUTHENTIK_E2E_TOKEN", defaultBootstrapToken)
	password := env("AUTHENTIK_E2E_PASSWORD", defaultUserPassword)

	uniq := strconv.FormatInt(time.Now().UnixNano(), 10)
	group := "e2e-grp-" + uniq
	user := "e2e-usr-" + uniq
	tokenID := user + "-token"
	customToken := "e2e" + fmt.Sprintf("%057d", time.Now().UnixNano()) // 60 chars, unique per run

	ctx := context.Background()
	admin := api.NewAPIClient(authentik.CreateConfiguration(scheme, host, token))

	// Wait for the bootstrap admin token to become live. Authentik reports
	// /-/health/ready/ once the server can serve HTTP, but the worker applies
	// the bootstrap blueprint (which creates the akadmin API token) shortly
	// after — so the first authenticated call 403s on a freshly-started stack.
	deadline := time.Now().Add(180 * time.Second)
	for {
		if _, _, err := authentik.MeRetrieveUser(ctx, admin); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("bootstrap admin token not usable within 180s: %v", err)
		}
		time.Sleep(3 * time.Second)
	}

	// 1. Drive the entire real provisioning flow.
	if err := CreateGroupsAndUsers(ctx, scheme, host, token, group, true, user, "orgs/e2e", password, tokenID, customToken); err != nil {
		t.Fatalf("CreateGroupsAndUsers against %s://%s: %v", scheme, host, err)
	}

	// 2. Independent verification with the admin client (reused): user persisted.
	pl, _, err := authentik.ListUser(ctx, admin, user)
	if err != nil {
		t.Fatalf("admin ListUser(%q): %v", user, err)
	}
	if pl == nil || len(pl.Results) == 0 {
		t.Fatalf("user %q not found via admin ListUser — it was not persisted", user)
	}

	// 3. The custom token authenticates AS the created user and sees its group.
	asUser := api.NewAPIClient(authentik.CreateConfiguration(scheme, host, customToken))
	su, _, err := authentik.MeRetrieveUser(ctx, asUser)
	if err != nil {
		t.Fatalf("MeRetrieveUser with the created user's custom token: %v", err)
	}
	if len(su.GetUser().Groups) == 0 {
		t.Errorf("created user %q has no groups", user)
	}
	t.Logf("e2e OK: provisioned user %q in group %q against %s://%s", user, group, scheme, host)
}
