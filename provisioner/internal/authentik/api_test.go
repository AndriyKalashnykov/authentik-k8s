package authentik

import "testing"

// TestCreateConfiguration is the auth-wiring contract: a regression here silently
// breaks every API call. It asserts scheme/host/debug AND the Bearer default
// header — pure, no network.
func TestCreateConfiguration(t *testing.T) {
	cfg := CreateConfiguration("https", "example.com:9443", "tok123")

	if cfg == nil {
		t.Fatal("CreateConfiguration returned nil")
	}
	if cfg.Scheme != "https" {
		t.Errorf("Scheme = %q, want %q", cfg.Scheme, "https")
	}
	if cfg.Host != "example.com:9443" {
		t.Errorf("Host = %q, want %q", cfg.Host, "example.com:9443")
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
	if got, want := cfg.DefaultHeader["Authorization"], "Bearer tok123"; got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
	if cfg.HTTPClient == nil {
		t.Error("HTTPClient = nil, want a configured client (TLS transport)")
	}
}
