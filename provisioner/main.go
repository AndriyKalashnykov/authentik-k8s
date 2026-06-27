package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/AndriyKalashnykov/authentik-k8s/provisioner/internal/authentik"
	api "goauthentik.io/api/v3"
)

// Fallback defaults. These mirror .env.example so the program works without a
// .env file; the Makefile/Docker run paths source .env and override them. They
// are the same demo values already committed in the k8s manifests / compose,
// not new secrets — operators override via env for a real deployment.
const (
	defaultScheme         = "https"
	defaultHost           = "127.0.0.1:9443" // k8s LoadBalancer form: "<LB-IP>:443"
	defaultBootstrapToken = "NoMlxBQuYgfu3j19ygGqhjXenAjrJgOfN5naqmSDBUhdLjYqHKze7yyzY07H"
	defaultUserPassword   = "Authentik01234567890!"
	defaultOrg1           = "org-01"
	defaultOrg2           = "org-02"
	defaultOrg1AdminToken = "ZId4CDEtmHbnuxkJH2ehUzHgYeTmOansuCO0JsTTsZnYB1z9N0WoAutpyH4i"
	defaultOrg1UserToken  = "e3qVF1uGTL5DKjglvMKpDYk60X7s89jnbNh9nPEpFYzSgoOHYDSMi0xxYhYr"
	defaultOrg2AdminToken = "ZId4CDEtmHbnuxkJH2ehUzHgYeTmOansuCO0JsTTsZnYB1z9N0WoAutpyH4i"
	defaultOrg2UserToken  = "svkH90FMYlnXPA5JHxePVQkozTjXReT6rsdQ2BXedwI5mtrFYR5mfrunMt4B"
)

// Forward-auth demo fallback defaults (opt-in via AUTHENTIK_FORWARD_AUTH_ENABLED).
// These wire an Authentik proxy provider in forward_domain mode + an application,
// bound to the built-in embedded outpost, so Traefik's forwardAuth middleware can
// gate the traefik/whoami app (see compose/forward-auth/). The flow slugs are the
// stock flows every Authentik ships; PKs are resolved at runtime, never hardcoded.
const (
	defaultFwdProviderName     = "whoami"
	defaultFwdAppName          = "whoami"
	defaultFwdAppSlug          = "whoami"
	defaultFwdExternalHost     = "https://whoami.127-0-0-1.sslip.io"
	defaultFwdMode             = "forward_single" // single whoami app; forward_domain for SSO
	defaultFwdCookieDomain     = "127-0-0-1.sslip.io"
	defaultFwdAuthzFlow        = "default-provider-authorization-implicit-consent"
	defaultFwdInvalidationFlow = "default-provider-invalidation-flow"
)

// Demo naming convention. Group / user / token-identifier names are derived
// from the org name (see deriveNames + .env.example); these are the fixed
// role/group segments and join tokens, single-sourced so the scheme lives in
// exactly one place rather than as inline literals in the provisioning table.
const (
	roleAdmin      = "admin"  // user-name segment for the admin user
	roleUser       = "user"   // user-name segment for the regular user
	groupAdmins    = "admins" // group-name segment for admins (superusers)
	groupUsers     = "users"  // group-name segment for regular users
	nameSep        = "-"      // joins <org> with <role>/<group>
	tokenSuffix    = "-token" // appended to the user name to form the token identifier
	userPathPrefix = "orgs/"  // Authentik user `path` prefix, namespaced per org
)

// deriveNames builds the Authentik object names for one (org, role, group) from
// the demo naming convention. Pure + exported-to-the-test so the scheme is
// locked by a unit test rather than only exercised live.
func deriveNames(org, role, group string) (groupName, userName, tokenIdentifier, userPath string) {
	groupName = org + nameSep + group
	userName = org + nameSep + role
	tokenIdentifier = userName + tokenSuffix
	userPath = userPathPrefix + org
	return groupName, userName, tokenIdentifier, userPath
}

// env returns the value of key, or fallback when unset/empty.
func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// CreateGroupsAndUsers provisions one group + one user against Authentik, sets the
// user's password and OAuth token, then re-authenticates as that user to read its
// group membership. It returns an error (rather than panicking) so callers — and
// the whole-flow contract test — can drive every step and assert the outcome.
func CreateGroupsAndUsers(ctx context.Context, authentikServerScheme string, authentikServerHost string, authentikBootstrapToken string, groupName string, isGroupSuperUser bool,
	userName string, userPath string, userPassword string, userTokenIdentifier string, userToken string) error {

	// create authentic API client using AuthentikBootstrapToken used during Authentik deployment
	akadminConfig := authentik.CreateConfiguration(authentikServerScheme, authentikServerHost, authentikBootstrapToken)
	akadminApiClient := api.NewAPIClient(akadminConfig)

	// create a group (creates a new group with a fresh pk)
	grp, _, err := authentik.CreateGroup(ctx, akadminApiClient, groupName, isGroupSuperUser)
	if err != nil {
		return fmt.Errorf("create group %q: %w", groupName, err)
	}
	groupUID := grp.Pk
	log.Printf("groupUID: %v\n", groupUID)

	// create a user and assign it to the previously created group
	usr, _, err := authentik.CreateUser(ctx, akadminApiClient, groupUID, userName, userPath)
	if err != nil {
		return fmt.Errorf("create user %q: %w", userName, err)
	}
	userUID := usr.Pk
	log.Printf("userUID: %v\n", userUID)

	// set the user's password
	if _, err = authentik.UpdateUserPassword(ctx, akadminApiClient, userUID, userPassword); err != nil {
		return fmt.Errorf("set password for user %q: %w", userName, err)
	}

	// create the user's OAuth token
	token, _, err := authentik.CreateUserToken(ctx, akadminApiClient, userUID, userTokenIdentifier, userTokenIdentifier)
	if err != nil {
		return fmt.Errorf("create token %q: %w", userTokenIdentifier, err)
	}
	if token != nil {
		log.Printf("Token: %v", token)
	}

	// retrieve the Authentik-generated token key
	tv, _, err := authentik.RetrieveUserToken(ctx, akadminApiClient, userTokenIdentifier)
	if err != nil {
		return fmt.Errorf("retrieve generated token %q: %w", userTokenIdentifier, err)
	}
	if tv != nil {
		log.Printf("OAuth token: %v", tv.Key)
	}

	// overwrite the token key with a custom value
	resp, err := authentik.UpdateUserToken(ctx, akadminApiClient, userTokenIdentifier, userToken)
	if err != nil {
		return fmt.Errorf("set custom key for token %q: %w", userTokenIdentifier, err)
	}
	if resp != nil {
		log.Printf("resp: %v", resp.Body)
	}

	// retrieve the token again to confirm the custom key took effect
	tvnew, _, err := authentik.RetrieveUserToken(ctx, akadminApiClient, userTokenIdentifier)
	if err != nil {
		return fmt.Errorf("retrieve custom token %q: %w", userTokenIdentifier, err)
	}
	log.Printf("OAuth token: %v", tvnew.Key)

	// confirm the custom key was set
	if tvnew.Key == userToken {
		log.Printf("custom token for %q was set", userName)
	} else {
		log.Printf("something went wrong setting the custom token for %q", userName)
	}

	// create an API client as the new user using its OAuth token
	userConfig := authentik.CreateConfiguration(authentikServerScheme, authentikServerHost, tvnew.Key)
	userApiClient := api.NewAPIClient(userConfig)

	// read the user's own info (which groups it belongs to)
	su, _, err := authentik.MeRetrieveUser(ctx, userApiClient)
	if err != nil {
		return fmt.Errorf("retrieve self for user %q: %w", userName, err)
	}
	if su != nil {
		log.Printf("User Groups: %v", su.GetUser().Groups)
	}

	return nil
}

func main() {
	ctx := context.Background()

	// Connection + shared config, externalized to env (see .env.example).
	scheme := env("AUTHENTIK_SCHEME", defaultScheme)
	host := env("AUTHENTIK_HOST", defaultHost)
	bootstrapToken := env("AUTHENTIK_BOOTSTRAP_TOKEN", defaultBootstrapToken)
	password := env("AUTHENTIK_USER_PASSWORD", defaultUserPassword)
	org1 := env("AUTHENTIK_ORG1", defaultOrg1)
	org2 := env("AUTHENTIK_ORG2", defaultOrg2)

	// One provisioning request per (org, role). Names are derived from the org;
	// the per-user OAuth token keys are externalized to env.
	type provision struct {
		org       string
		role      string // roleAdmin | roleUser
		group     string // groupAdmins | groupUsers
		superuser bool   // admins can log into the Authentik admin UI
		token     string
	}
	requests := []provision{
		{org1, roleAdmin, groupAdmins, true, env("AUTHENTIK_ORG1_ADMIN_TOKEN", defaultOrg1AdminToken)},
		{org1, roleUser, groupUsers, false, env("AUTHENTIK_ORG1_USER_TOKEN", defaultOrg1UserToken)},
		{org2, roleAdmin, groupAdmins, true, env("AUTHENTIK_ORG2_ADMIN_TOKEN", defaultOrg2AdminToken)},
		{org2, roleUser, groupUsers, false, env("AUTHENTIK_ORG2_USER_TOKEN", defaultOrg2UserToken)},
	}

	// The org/user/token provisioning is not idempotent (re-creating an existing
	// group/user fails), so it can be skipped — e.g. when only (re)running the
	// idempotent forward-auth setup against an already-provisioned instance.
	if env("AUTHENTIK_PROVISION_ORGS", "true") == "true" {
		for _, r := range requests {
			groupName, userName, tokenIdentifier, userPath := deriveNames(r.org, r.role, r.group)
			if err := CreateGroupsAndUsers(ctx, scheme, host, bootstrapToken,
				groupName, r.superuser, userName, userPath, password, tokenIdentifier, r.token); err != nil {
				log.Fatalf("error: %v", err)
			}
		}
	}

	// Opt-in: wire a forward-auth proxy provider + application for the
	// traefik/whoami demo and bind it to the embedded outpost. Disabled by
	// default so the core POC stays a pure user/group/token demo.
	if env("AUTHENTIK_FORWARD_AUTH_ENABLED", "false") == "true" {
		cfg := authentik.ForwardAuthConfig{
			ProviderName:          env("AUTHENTIK_FORWARD_AUTH_PROVIDER_NAME", defaultFwdProviderName),
			AppName:               env("AUTHENTIK_FORWARD_AUTH_APP_NAME", defaultFwdAppName),
			AppSlug:               env("AUTHENTIK_FORWARD_AUTH_APP_SLUG", defaultFwdAppSlug),
			ExternalHost:          env("AUTHENTIK_FORWARD_AUTH_EXTERNAL_HOST", defaultFwdExternalHost),
			Mode:                  env("AUTHENTIK_FORWARD_AUTH_MODE", defaultFwdMode),
			CookieDomain:          env("AUTHENTIK_FORWARD_AUTH_COOKIE_DOMAIN", defaultFwdCookieDomain),
			AuthorizationFlowSlug: env("AUTHENTIK_FORWARD_AUTH_AUTHZ_FLOW", defaultFwdAuthzFlow),
			InvalidationFlowSlug:  env("AUTHENTIK_FORWARD_AUTH_INVALIDATION_FLOW", defaultFwdInvalidationFlow),
		}
		akadminConfig := authentik.CreateConfiguration(scheme, host, bootstrapToken)
		akadminApiClient := api.NewAPIClient(akadminConfig)
		if err := authentik.SetupForwardAuth(ctx, akadminApiClient, cfg); err != nil {
			log.Fatalf("forward-auth setup: %v", err)
		}
		log.Printf("forward-auth configured: provider %q, app %q (%s) bound to embedded outpost",
			cfg.ProviderName, cfg.AppName, cfg.ExternalHost)
	}
}
