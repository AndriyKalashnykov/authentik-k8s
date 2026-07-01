package main

import (
	"testing"

	"github.com/AndriyKalashnykov/authentik-k8s/provisioner/internal/authentik"
)

// fakeEnv returns a (key, fallback)->value lookup backed by a map, matching the
// production env() semantics (a missing OR empty key falls back).
func fakeEnv(m map[string]string) func(key, fallback string) string {
	return func(key, fallback string) string {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
		return fallback
	}
}

// TestEnv covers the three branches of the env() helper: set-non-empty returns
// the value; set-empty and unset both return the fallback.
func TestEnv(t *testing.T) {
	t.Setenv("PROV_TEST_SET", "value")
	t.Setenv("PROV_TEST_EMPTY", "")
	cases := []struct {
		name, key, fallback, want string
	}{
		{"set non-empty returns value", "PROV_TEST_SET", "fb", "value"},
		{"set empty returns fallback", "PROV_TEST_EMPTY", "fb", "fb"},
		{"unset returns fallback", "PROV_TEST_UNSET_XYZ", "fb", "fb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := env(c.key, c.fallback); got != c.want {
				t.Errorf("env(%q, %q) = %q, want %q", c.key, c.fallback, got, c.want)
			}
		})
	}
}

// TestBuildProvisionRequests_Defaults locks the 4-entry provisioning table
// (two orgs × admin/user), the superuser flags, and the default token keys.
func TestBuildProvisionRequests_Defaults(t *testing.T) {
	reqs := buildProvisionRequests(defaultOrg1, defaultOrg2, fakeEnv(nil))
	want := []provision{
		{defaultOrg1, roleAdmin, groupAdmins, true, defaultOrg1AdminToken},
		{defaultOrg1, roleUser, groupUsers, false, defaultOrg1UserToken},
		{defaultOrg2, roleAdmin, groupAdmins, true, defaultOrg2AdminToken},
		{defaultOrg2, roleUser, groupUsers, false, defaultOrg2UserToken},
	}
	if len(reqs) != len(want) {
		t.Fatalf("want %d requests, got %d", len(want), len(reqs))
	}
	for i, w := range want {
		if reqs[i] != w {
			t.Errorf("request[%d] = %+v, want %+v", i, reqs[i], w)
		}
	}
}

// TestBuildProvisionRequests_EnvOverride confirms per-user token keys are read
// from env (overriding defaults) and the org names thread through positionally.
func TestBuildProvisionRequests_EnvOverride(t *testing.T) {
	reqs := buildProvisionRequests("o1", "o2", fakeEnv(map[string]string{
		"AUTHENTIK_ORG1_ADMIN_TOKEN": "custom-admin-1",
		"AUTHENTIK_ORG2_USER_TOKEN":  "custom-user-2",
	}))
	if reqs[0].token != "custom-admin-1" {
		t.Errorf("org1 admin token = %q, want custom-admin-1", reqs[0].token)
	}
	if reqs[3].token != "custom-user-2" {
		t.Errorf("org2 user token = %q, want custom-user-2", reqs[3].token)
	}
	if reqs[1].token != defaultOrg1UserToken {
		t.Errorf("non-overridden org1 user token = %q, want default", reqs[1].token)
	}
	if reqs[0].org != "o1" || reqs[2].org != "o2" {
		t.Errorf("org names not threaded: [0]=%q [2]=%q", reqs[0].org, reqs[2].org)
	}
}

// TestBuildForwardAuthConfig_Defaults asserts the whole config is assembled from
// the documented defaults, including the AuthentikHostInsecure bool parse.
func TestBuildForwardAuthConfig_Defaults(t *testing.T) {
	got := buildForwardAuthConfig(fakeEnv(nil))
	want := authentik.ForwardAuthConfig{
		ProviderName:          defaultFwdProviderName,
		AppName:               defaultFwdAppName,
		AppSlug:               defaultFwdAppSlug,
		ExternalHost:          defaultFwdExternalHost,
		Mode:                  defaultFwdMode,
		CookieDomain:          defaultFwdCookieDomain,
		AuthentikHost:         defaultFwdAuthentikHost,
		AuthentikHostInsecure: true, // default "true" parses to true
		AuthorizationFlowSlug: defaultFwdAuthzFlow,
		InvalidationFlowSlug:  defaultFwdInvalidationFlow,
	}
	if got != want {
		t.Errorf("buildForwardAuthConfig(defaults) =\n  %+v\nwant\n  %+v", got, want)
	}
}

// TestBuildForwardAuthConfig_EnvOverride covers the env-driven overrides,
// including AuthentikHostInsecure being disabled by the literal "false".
func TestBuildForwardAuthConfig_EnvOverride(t *testing.T) {
	got := buildForwardAuthConfig(fakeEnv(map[string]string{
		"AUTHENTIK_FORWARD_AUTH_MODE":          "forward_domain",
		"AUTHENTIK_FORWARD_AUTH_HOST":          "https://auth.example.com",
		"AUTHENTIK_FORWARD_AUTH_HOST_INSECURE": "false",
	}))
	if got.Mode != "forward_domain" {
		t.Errorf("Mode = %q, want forward_domain", got.Mode)
	}
	if got.AuthentikHost != "https://auth.example.com" {
		t.Errorf("AuthentikHost = %q, want https://auth.example.com", got.AuthentikHost)
	}
	if got.AuthentikHostInsecure {
		t.Error(`AuthentikHostInsecure = true, want false (env "false")`)
	}
	// Non-overridden fields keep their defaults.
	if got.ProviderName != defaultFwdProviderName {
		t.Errorf("ProviderName = %q, want default", got.ProviderName)
	}
}
