package authentik

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	api "goauthentik.io/api/v3"
)

// Minimal-but-complete JSON bodies. The goauthentik.io/api/v3 models enforce
// their `required` properties at unmarshal time, so each canned body must carry
// every required field even when the test only asserts a couple of them.
const (
	paginationOne = `"pagination":{"next":0,"previous":0,"count":1,"current":1,"total_pages":1,"start_index":1,"end_index":1}`

	flowAuthzJSON = `{"pk":"flow-uuid-authz","policybindingmodel_ptr_id":"pbm-1","name":"Authorize","slug":"default-provider-authorization-implicit-consent","title":"Authorize Application","designation":"authorization","background_url":"","background_themed_urls":{},"stages":[],"policies":[],"cache_count":0,"export_url":""}`

	proxyProviderJSON = `{"pk":7,"name":"whoami","authorization_flow":"flow-uuid-authz","invalidation_flow":"flow-uuid-inval","component":"ak-provider-proxy-form","assigned_application_slug":"","assigned_application_name":null,"assigned_backchannel_application_slug":"","assigned_backchannel_application_name":null,"verbose_name":"Proxy Provider","verbose_name_plural":"Proxy Providers","meta_model_name":"authentik_providers_proxy.proxyprovider","client_id":"cid","external_host":"https://whoami.127-0-0-1.sslip.io","redirect_uris":[],"outpost_set":[]}`

	applicationJSON = `{"pk":"app-uuid","name":"whoami","slug":"whoami","provider_obj":null,"backchannel_providers_obj":[],"launch_url":null,"meta_icon_url":null,"meta_icon_themed_urls":{}}`
)

// paged wraps result objects in the paginated-list envelope. The list models
// require both `pagination` and `autocomplete`, so both are always present.
func paged(results string) string {
	return `{` + paginationOne + `,"results":[` + results + `],"autocomplete":{}}`
}

// embeddedOutpostJSON builds an Outpost body with the given provider PKs bound.
func embeddedOutpostJSON(providers string) string {
	return `{"pk":"outpost-uuid","name":"authentik Embedded Outpost","type":"proxy","providers":[` + providers + `],"providers_obj":[],"service_connection_obj":null,"refresh_interval_s":60,"token_identifier":"tok","config":{},"managed":"goauthentik.io/outposts/embedded"}`
}

func testCfg() ForwardAuthConfig {
	return ForwardAuthConfig{
		ProviderName:          "whoami",
		AppName:               "whoami",
		AppSlug:               "whoami",
		ExternalHost:          "https://whoami.127-0-0-1.sslip.io",
		Mode:                  "forward_single",
		CookieDomain:          "127-0-0-1.sslip.io",
		AuthorizationFlowSlug: "default-provider-authorization-implicit-consent",
		InvalidationFlowSlug:  "default-provider-invalidation-flow",
	}
}

func TestResolveFlowPK(t *testing.T) {
	var method, path, slug string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		slug = r.URL.Query().Get("slug")
		writeJSON(t, w, http.StatusOK, paged(flowAuthzJSON))
	})

	pk, err := ResolveFlowPK(context.Background(), client, "default-provider-authorization-implicit-consent")
	if err != nil {
		t.Fatalf("ResolveFlowPK: %v", err)
	}
	if method != http.MethodGet || path != "/api/v3/flows/instances/" {
		t.Errorf("request = %s %s, want GET /api/v3/flows/instances/", method, path)
	}
	if slug != "default-provider-authorization-implicit-consent" {
		t.Errorf("slug query = %q, want the authorization flow slug", slug)
	}
	if pk != "flow-uuid-authz" {
		t.Errorf("pk = %q, want flow-uuid-authz", pk)
	}
}

func TestResolveFlowPKNotFound(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, paged(""))
	})
	if _, err := ResolveFlowPK(context.Background(), client, "missing-flow"); err == nil {
		t.Fatal("ResolveFlowPK: want error for missing flow, got nil")
	}
}

func TestCreateProxyProvider(t *testing.T) {
	var method, path string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(t, w, http.StatusCreated, proxyProviderJSON)
	})

	pp, _, err := CreateProxyProvider(context.Background(), client, testCfg(), "flow-uuid-authz", "flow-uuid-inval")
	if err != nil {
		t.Fatalf("CreateProxyProvider: %v", err)
	}
	if method != http.MethodPost || path != "/api/v3/providers/proxy/" {
		t.Errorf("request = %s %s, want POST /api/v3/providers/proxy/", method, path)
	}
	if body["mode"] != "forward_single" {
		t.Errorf("mode = %v, want forward_single", body["mode"])
	}
	if body["external_host"] != "https://whoami.127-0-0-1.sslip.io" {
		t.Errorf("external_host = %v, want the whoami URL", body["external_host"])
	}
	// forward_single must NOT send cookie_domain (it is forward_domain-only).
	if _, present := body["cookie_domain"]; present {
		t.Errorf("cookie_domain present in forward_single request (%v), want omitted", body["cookie_domain"])
	}
	if body["authorization_flow"] != "flow-uuid-authz" {
		t.Errorf("authorization_flow = %v, want flow-uuid-authz", body["authorization_flow"])
	}
	if body["invalidation_flow"] != "flow-uuid-inval" {
		t.Errorf("invalidation_flow = %v, want flow-uuid-inval", body["invalidation_flow"])
	}
	if pp.Pk != 7 {
		t.Errorf("pp.Pk = %d, want 7", pp.Pk)
	}
}

func TestCreateProxyProviderForwardDomain(t *testing.T) {
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(t, w, http.StatusCreated, proxyProviderJSON)
	})

	cfg := testCfg()
	cfg.Mode = "forward_domain"
	if _, _, err := CreateProxyProvider(context.Background(), client, cfg, "flow-uuid-authz", "flow-uuid-inval"); err != nil {
		t.Fatalf("CreateProxyProvider: %v", err)
	}
	if body["mode"] != "forward_domain" {
		t.Errorf("mode = %v, want forward_domain", body["mode"])
	}
	// forward_domain MUST carry the cookie domain.
	if body["cookie_domain"] != "127-0-0-1.sslip.io" {
		t.Errorf("cookie_domain = %v, want 127-0-0-1.sslip.io", body["cookie_domain"])
	}
}

func TestCreateOrGetProxyProviderExisting(t *testing.T) {
	var posted bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		// List already contains a provider named "whoami" → no create.
		writeJSON(t, w, http.StatusOK, paged(proxyProviderJSON))
	})

	pk, err := CreateOrGetProxyProvider(context.Background(), client, testCfg(), "flow-uuid-authz", "flow-uuid-inval")
	if err != nil {
		t.Fatalf("CreateOrGetProxyProvider: %v", err)
	}
	if posted {
		t.Error("CreateOrGetProxyProvider issued a POST for an already-existing provider")
	}
	if pk != 7 {
		t.Errorf("pk = %d, want 7 (existing provider)", pk)
	}
}

func TestCreateApplication(t *testing.T) {
	var method, path string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(t, w, http.StatusCreated, applicationJSON)
	})

	app, _, err := CreateApplication(context.Background(), client, testCfg(), 7)
	if err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}
	if method != http.MethodPost || path != "/api/v3/core/applications/" {
		t.Errorf("request = %s %s, want POST /api/v3/core/applications/", method, path)
	}
	if body["slug"] != "whoami" || body["name"] != "whoami" {
		t.Errorf("slug/name = %v/%v, want whoami/whoami", body["slug"], body["name"])
	}
	if body["provider"] != float64(7) { // JSON numbers decode to float64
		t.Errorf("provider = %v, want 7", body["provider"])
	}
	if app.Slug != "whoami" {
		t.Errorf("app.Slug = %q, want whoami", app.Slug)
	}
}

func TestCreateOrGetApplicationExisting(t *testing.T) {
	var posted bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		writeJSON(t, w, http.StatusOK, paged(applicationJSON))
	})

	if err := CreateOrGetApplication(context.Background(), client, testCfg(), 7); err != nil {
		t.Fatalf("CreateOrGetApplication: %v", err)
	}
	if posted {
		t.Error("CreateOrGetApplication issued a POST for an already-existing application")
	}
}

func TestFindEmbeddedOutpost(t *testing.T) {
	var method, path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		// A non-embedded outpost first, then the embedded one.
		other := strings.Replace(embeddedOutpostJSON(""), `"managed":"goauthentik.io/outposts/embedded"`, `"managed":"something-else","pk":"other-uuid"`, 1)
		writeJSON(t, w, http.StatusOK, paged(other+","+embeddedOutpostJSON("")))
	})

	op, err := FindEmbeddedOutpost(context.Background(), client)
	if err != nil {
		t.Fatalf("FindEmbeddedOutpost: %v", err)
	}
	if method != http.MethodGet || path != "/api/v3/outposts/instances/" {
		t.Errorf("request = %s %s, want GET /api/v3/outposts/instances/", method, path)
	}
	if op.Pk != "outpost-uuid" {
		t.Errorf("op.Pk = %q, want outpost-uuid (the embedded one)", op.Pk)
	}
	if op.Managed.Get() == nil || *op.Managed.Get() != EmbeddedOutpostManaged {
		t.Errorf("op.Managed = %v, want %q", op.Managed.Get(), EmbeddedOutpostManaged)
	}
}

func TestBindProviderToOutpost(t *testing.T) {
	var method, path string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(t, w, http.StatusOK, embeddedOutpostJSON("7"))
	})

	op := outpostFixture(t,"")
	resp, err := BindProviderToOutpost(context.Background(), client, op, 7)
	if err != nil {
		t.Fatalf("BindProviderToOutpost: %v", err)
	}
	if method != http.MethodPatch || path != "/api/v3/outposts/instances/outpost-uuid/" {
		t.Errorf("request = %s %s, want PATCH /api/v3/outposts/instances/outpost-uuid/", method, path)
	}
	provs, ok := body["providers"].([]any)
	if !ok || len(provs) != 1 || provs[0] != float64(7) {
		t.Errorf("providers = %v, want [7]", body["providers"])
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Errorf("resp = %v, want 200", resp)
	}
}

func TestBindProviderToOutpostAlreadyBound(t *testing.T) {
	var patched bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			patched = true
		}
		writeJSON(t, w, http.StatusOK, embeddedOutpostJSON("7"))
	})

	op := outpostFixture(t,"7") // already has provider 7
	resp, err := BindProviderToOutpost(context.Background(), client, op, 7)
	if err != nil {
		t.Fatalf("BindProviderToOutpost: %v", err)
	}
	if patched {
		t.Error("BindProviderToOutpost issued a PATCH for an already-bound provider")
	}
	if resp != nil {
		t.Errorf("resp = %v, want nil (no-op)", resp)
	}
}

// outpostFixture decodes an Outpost model from canned JSON for unit assertions.
func outpostFixture(t *testing.T, providers string) *api.Outpost {
	t.Helper()
	var op api.Outpost
	if err := json.Unmarshal([]byte(embeddedOutpostJSON(providers)), &op); err != nil {
		t.Fatalf("decode outpost fixture: %v", err)
	}
	return &op
}

func TestSetupForwardAuth(t *testing.T) {
	var sawProviderPost, sawAppPost, sawOutpostPatch bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/flows/instances/":
			// Echo a flow matching whichever slug was requested (authz or invalidation).
			slug := r.URL.Query().Get("slug")
			flow := strings.Replace(flowAuthzJSON,
				`"slug":"default-provider-authorization-implicit-consent"`,
				`"slug":"`+slug+`"`, 1)
			writeJSON(t, w, http.StatusOK, paged(flow))
		case r.URL.Path == "/api/v3/providers/proxy/" && r.Method == http.MethodGet:
			writeJSON(t, w, http.StatusOK, paged(""))
		case r.URL.Path == "/api/v3/providers/proxy/" && r.Method == http.MethodPost:
			sawProviderPost = true
			writeJSON(t, w, http.StatusCreated, proxyProviderJSON)
		case r.URL.Path == "/api/v3/core/applications/" && r.Method == http.MethodGet:
			writeJSON(t, w, http.StatusOK, paged(""))
		case r.URL.Path == "/api/v3/core/applications/" && r.Method == http.MethodPost:
			sawAppPost = true
			writeJSON(t, w, http.StatusCreated, applicationJSON)
		case r.URL.Path == "/api/v3/outposts/instances/":
			writeJSON(t, w, http.StatusOK, paged(embeddedOutpostJSON("")))
		case strings.HasPrefix(r.URL.Path, "/api/v3/outposts/instances/") && r.Method == http.MethodPatch:
			sawOutpostPatch = true
			writeJSON(t, w, http.StatusOK, embeddedOutpostJSON("7"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			writeJSON(t, w, http.StatusNotFound, `{}`)
		}
	})

	if err := SetupForwardAuth(context.Background(), client, testCfg()); err != nil {
		t.Fatalf("SetupForwardAuth: %v", err)
	}
	if !sawProviderPost {
		t.Error("SetupForwardAuth did not create the proxy provider")
	}
	if !sawAppPost {
		t.Error("SetupForwardAuth did not create the application")
	}
	if !sawOutpostPatch {
		t.Error("SetupForwardAuth did not bind the provider to the embedded outpost")
	}
}
