package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Canned goauthentik.io/api/v3 success bodies. Every field the generated models
// mark `required` MUST be present or the client errors at UNMARSHAL (before any
// assertion) — see rules/golang/testing.md. Shared by the whole-path and
// error-branch tests so the fixture shape lives in exactly one place.
const (
	bodyGroup = `{"pk":"group-uuid-1","num_pk":1,"name":"g-admins","parents_obj":[],"users_obj":[],"roles_obj":[],"inherited_roles_obj":[],"children":[],"children_obj":[]}`
	bodyUser  = `{"pk":42,"username":"alice","name":"alice","date_joined":"2024-01-01T00:00:00Z","is_superuser":false,"groups_obj":[],"roles_obj":[],"avatar":"","uid":"u1","uuid":"uu1","password_change_date":"2024-01-01T00:00:00Z","last_updated":"2024-01-01T00:00:00Z"}`
	bodyToken = `{"pk":"tok-pk","identifier":"alice-token","user_obj":{"pk":42,"username":"alice","name":"alice","date_joined":"2024-01-01T00:00:00Z","is_superuser":false,"groups_obj":[],"roles_obj":[],"avatar":"","uid":"u1","uuid":"uu1","password_change_date":"2024-01-01T00:00:00Z","last_updated":"2024-01-01T00:00:00Z"}}`
	bodyMe    = `{"user":{"pk":42,"username":"alice","name":"alice","uid":"u1","avatar":"","is_active":true,"is_superuser":false,"groups":[{"pk":"group-uuid-1","name":"g-admins"}],"roles":[],"settings":{},"system_permissions":[]}}`
)

func viewKeyBody(key string) string { return `{"key":"` + key + `"}` }

// TestCreateGroupsAndUsers_WholePath drives the entire provisioning sequence
// (group -> user -> password -> token -> read-back -> set custom key -> read-back
// -> me) against a mock Authentik that returns canned responses at the exact
// goauthentik.io/api/v3 paths. It is the whole-path contract test: a unit test on
// any single wrapper would not catch a break in how main.go chains them.
func TestCreateGroupsAndUsers_WholePath(t *testing.T) {
	const customToken = "custom-token-value"

	var mu sync.Mutex
	hits := map[string]int{}
	record := func(key string) {
		mu.Lock()
		hits[key]++
		mu.Unlock()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v3/core/groups/", func(w http.ResponseWriter, r *http.Request) {
		record("group")
		writeJSON(w, http.StatusCreated, bodyGroup)
	})
	mux.HandleFunc("POST /api/v3/core/users/", func(w http.ResponseWriter, r *http.Request) {
		record("user")
		writeJSON(w, http.StatusCreated, bodyUser)
	})
	mux.HandleFunc("POST /api/v3/core/users/{id}/set_password/", func(w http.ResponseWriter, r *http.Request) {
		record("set_password")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/v3/core/tokens/", func(w http.ResponseWriter, r *http.Request) {
		record("create_token")
		writeJSON(w, http.StatusCreated, bodyToken)
	})
	mux.HandleFunc("GET /api/v3/core/tokens/{identifier}/view_key/", func(w http.ResponseWriter, r *http.Request) {
		record("view_key")
		// Always return the custom token so the "custom key was set" equality holds.
		writeJSON(w, http.StatusOK, viewKeyBody(customToken))
	})
	mux.HandleFunc("POST /api/v3/core/tokens/{identifier}/set_key/", func(w http.ResponseWriter, r *http.Request) {
		record("set_key")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v3/core/users/me/", func(w http.ResponseWriter, r *http.Request) {
		record("me")
		writeJSON(w, http.StatusOK, bodyMe)
	})
	var unmatched []string
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		unmatched = append(unmatched, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	err := CreateGroupsAndUsers(context.Background(), "http", host, "boot-token",
		"g-admins", true, "alice", "orgs/o1", "pw", "alice-token", customToken)

	mu.Lock()
	defer mu.Unlock()
	if len(unmatched) > 0 {
		t.Fatalf("requests hit unmatched routes (path/shape mismatch): %v", unmatched)
	}
	if err != nil {
		t.Fatalf("CreateGroupsAndUsers returned error: %v", err)
	}

	want := map[string]int{
		"group": 1, "user": 1, "set_password": 1, "create_token": 1,
		"view_key": 2, "set_key": 1, "me": 1, // view_key is called twice (generated + custom)
	}
	for step, n := range want {
		if hits[step] != n {
			t.Errorf("step %q hit %d times, want %d", step, hits[step], n)
		}
	}
}

// newProvisionMux serves the whole provisioning sequence successfully, except
// the route named by failStep, which returns 500. Steps BEFORE the failing one
// must still return valid bodies (the client unmarshals them), so the shared
// fixture consts are reused.
func newProvisionMux(failStep, customToken string) http.Handler {
	mux := http.NewServeMux()
	h := func(step string, status int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if step == failStep {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if body == "" {
				w.WriteHeader(status)
				return
			}
			writeJSON(w, status, body)
		}
	}
	mux.HandleFunc("POST /api/v3/core/groups/", h("group", http.StatusCreated, bodyGroup))
	mux.HandleFunc("POST /api/v3/core/users/", h("user", http.StatusCreated, bodyUser))
	mux.HandleFunc("POST /api/v3/core/users/{id}/set_password/", h("set_password", http.StatusNoContent, ""))
	mux.HandleFunc("POST /api/v3/core/tokens/", h("create_token", http.StatusCreated, bodyToken))
	mux.HandleFunc("GET /api/v3/core/tokens/{identifier}/view_key/", h("view_key", http.StatusOK, viewKeyBody(customToken)))
	mux.HandleFunc("POST /api/v3/core/tokens/{identifier}/set_key/", h("set_key", http.StatusNoContent, ""))
	mux.HandleFunc("GET /api/v3/core/users/me/", h("me", http.StatusOK, bodyMe))
	return mux
}

// TestCreateGroupsAndUsers_ErrorBranches asserts that a failure at each step is
// surfaced (not swallowed) with the step's context wrapped into the error.
func TestCreateGroupsAndUsers_ErrorBranches(t *testing.T) {
	const customToken = "custom-token-value"
	cases := []struct {
		failStep string // route that returns 500
		wantErr  string // substring the wrapped error must contain
	}{
		{"group", "create group"},
		{"user", "create user"},
		{"set_password", "set password"},
		{"create_token", "create token"},
		{"me", "retrieve self"},
	}
	for _, c := range cases {
		t.Run(c.failStep, func(t *testing.T) {
			srv := httptest.NewServer(newProvisionMux(c.failStep, customToken))
			defer srv.Close()
			host := strings.TrimPrefix(srv.URL, "http://")

			err := CreateGroupsAndUsers(context.Background(), "http", host, "boot-token",
				"g-admins", true, "alice", "orgs/o1", "pw", "alice-token", customToken)
			if err == nil {
				t.Fatalf("want error when step %q fails, got nil", c.failStep)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

// TestDeriveNames locks the demo naming convention: group/user/token/path are
// derived from (org, role, group) via the single-sourced naming constants.
func TestDeriveNames(t *testing.T) {
	cases := []struct {
		org, role, group                             string
		wantGroup, wantUser, wantToken, wantUserPath string
	}{
		{"org-01", roleAdmin, groupAdmins, "org-01-admins", "org-01-admin", "org-01-admin-token", "orgs/org-01"},
		{"org-01", roleUser, groupUsers, "org-01-users", "org-01-user", "org-01-user-token", "orgs/org-01"},
		{"org-02", roleAdmin, groupAdmins, "org-02-admins", "org-02-admin", "org-02-admin-token", "orgs/org-02"},
	}
	for _, c := range cases {
		g, u, tok, p := deriveNames(c.org, c.role, c.group)
		if g != c.wantGroup || u != c.wantUser || tok != c.wantToken || p != c.wantUserPath {
			t.Errorf("deriveNames(%q,%q,%q) = (%q,%q,%q,%q), want (%q,%q,%q,%q)",
				c.org, c.role, c.group, g, u, tok, p,
				c.wantGroup, c.wantUser, c.wantToken, c.wantUserPath)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
