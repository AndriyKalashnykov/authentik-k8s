package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

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
		writeJSON(w, http.StatusCreated, `{"pk":"group-uuid-1","num_pk":1,"name":"g-admins","parents_obj":[],"users_obj":[],"roles_obj":[],"inherited_roles_obj":[],"children":[],"children_obj":[]}`)
	})
	mux.HandleFunc("POST /api/v3/core/users/", func(w http.ResponseWriter, r *http.Request) {
		record("user")
		writeJSON(w, http.StatusCreated, `{"pk":42,"username":"alice","name":"alice","date_joined":"2024-01-01T00:00:00Z","is_superuser":false,"groups_obj":[],"roles_obj":[],"avatar":"","uid":"u1","uuid":"uu1","password_change_date":"2024-01-01T00:00:00Z","last_updated":"2024-01-01T00:00:00Z"}`)
	})
	mux.HandleFunc("POST /api/v3/core/users/{id}/set_password/", func(w http.ResponseWriter, r *http.Request) {
		record("set_password")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /api/v3/core/tokens/", func(w http.ResponseWriter, r *http.Request) {
		record("create_token")
		writeJSON(w, http.StatusCreated, `{"pk":"tok-pk","identifier":"alice-token","user_obj":{"pk":42,"username":"alice","name":"alice","date_joined":"2024-01-01T00:00:00Z","is_superuser":false,"groups_obj":[],"roles_obj":[],"avatar":"","uid":"u1","uuid":"uu1","password_change_date":"2024-01-01T00:00:00Z","last_updated":"2024-01-01T00:00:00Z"}}`)
	})
	mux.HandleFunc("GET /api/v3/core/tokens/{identifier}/view_key/", func(w http.ResponseWriter, r *http.Request) {
		record("view_key")
		// Always return the custom token so the "custom key was set" equality holds.
		writeJSON(w, http.StatusOK, `{"key":"`+customToken+`"}`)
	})
	mux.HandleFunc("POST /api/v3/core/tokens/{identifier}/set_key/", func(w http.ResponseWriter, r *http.Request) {
		record("set_key")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v3/core/users/me/", func(w http.ResponseWriter, r *http.Request) {
		record("me")
		writeJSON(w, http.StatusOK, `{"user":{"pk":42,"username":"alice","name":"alice","uid":"u1","avatar":"","is_active":true,"is_superuser":false,"groups":[{"pk":"group-uuid-1","name":"g-admins"}],"roles":[],"settings":{},"system_permissions":[]}}`)
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

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
