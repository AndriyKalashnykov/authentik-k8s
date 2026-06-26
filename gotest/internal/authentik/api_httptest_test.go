package authentik

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "goauthentik.io/api/v3"
)

// newTestClient spins up an httptest server and returns an API client pointed at
// it via the REAL CreateConfiguration (so the Bearer default header, scheme, and
// host wiring are all exercised — these are hermetic contract tests, no live
// Authentik). The canned responses match the goauthentik.io/api/v3 client's
// expected paths (/api/v3/core/...) and JSON shapes.
func newTestClient(t *testing.T, h http.HandlerFunc) *api.APIClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	return api.NewAPIClient(CreateConfiguration("http", host, "test-token"))
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != "" {
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response body: %v", err)
		}
	}
}

func TestCreateGroup(t *testing.T) {
	var method, path, auth string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(t, w, http.StatusCreated, `{"pk":"group-uuid-1","num_pk":1,"name":"admins","parents_obj":[],"users_obj":[],"roles_obj":[],"inherited_roles_obj":[],"children":[],"children_obj":[]}`)
	})

	grp, resp, err := CreateGroup(context.Background(), client, "admins", true)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if method != http.MethodPost || path != "/api/v3/core/groups/" {
		t.Errorf("request = %s %s, want POST /api/v3/core/groups/", method, path)
	}
	if auth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
	}
	if body["name"] != "admins" {
		t.Errorf("request body name = %v, want admins", body["name"])
	}
	if body["is_superuser"] != true {
		t.Errorf("request body is_superuser = %v, want true", body["is_superuser"])
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
	if grp.Pk != "group-uuid-1" {
		t.Errorf("grp.Pk = %q, want group-uuid-1", grp.Pk)
	}
}

func TestCreateUser(t *testing.T) {
	var method, path string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(t, w, http.StatusCreated, `{"pk":42,"username":"alice","name":"alice","date_joined":"2024-01-01T00:00:00Z","is_superuser":false,"groups_obj":[],"roles_obj":[],"avatar":"","uid":"u1","uuid":"uu1","password_change_date":"2024-01-01T00:00:00Z","last_updated":"2024-01-01T00:00:00Z"}`)
	})

	usr, _, err := CreateUser(context.Background(), client, "group-uuid-1", "alice", "orgs/o1")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if method != http.MethodPost || path != "/api/v3/core/users/" {
		t.Errorf("request = %s %s, want POST /api/v3/core/users/", method, path)
	}
	if body["username"] != "alice" || body["name"] != "alice" {
		t.Errorf("request body username/name = %v/%v, want alice/alice", body["username"], body["name"])
	}
	if body["path"] != "orgs/o1" {
		t.Errorf("request body path = %v, want orgs/o1", body["path"])
	}
	groups, ok := body["groups"].([]any)
	if !ok || len(groups) != 1 || groups[0] != "group-uuid-1" {
		t.Errorf("request body groups = %v, want [group-uuid-1]", body["groups"])
	}
	if usr.Pk != 42 {
		t.Errorf("usr.Pk = %d, want 42", usr.Pk)
	}
}

func TestUpdateUserPassword(t *testing.T) {
	var method, path string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusNoContent)
	})

	resp, err := UpdateUserPassword(context.Background(), client, 42, "s3cr3t")
	if err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}
	if method != http.MethodPost || path != "/api/v3/core/users/42/set_password/" {
		t.Errorf("request = %s %s, want POST /api/v3/core/users/42/set_password/", method, path)
	}
	if body["password"] != "s3cr3t" {
		t.Errorf("request body password = %v, want s3cr3t", body["password"])
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestCreateUserToken(t *testing.T) {
	var method, path string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(t, w, http.StatusCreated, `{"pk":"tok-pk","identifier":"alice-token","user_obj":{"pk":42,"username":"alice","name":"alice","date_joined":"2024-01-01T00:00:00Z","is_superuser":false,"groups_obj":[],"roles_obj":[],"avatar":"","uid":"u1","uuid":"uu1","password_change_date":"2024-01-01T00:00:00Z","last_updated":"2024-01-01T00:00:00Z"}}`)
	})

	token, _, err := CreateUserToken(context.Background(), client, 42, "alice-token", "alice-token")
	if err != nil {
		t.Fatalf("CreateUserToken: %v", err)
	}
	if method != http.MethodPost || path != "/api/v3/core/tokens/" {
		t.Errorf("request = %s %s, want POST /api/v3/core/tokens/", method, path)
	}
	if body["identifier"] != "alice-token" {
		t.Errorf("request body identifier = %v, want alice-token", body["identifier"])
	}
	if body["intent"] != "api" {
		t.Errorf("request body intent = %v, want api", body["intent"])
	}
	if body["expiring"] != false {
		t.Errorf("request body expiring = %v, want false", body["expiring"])
	}
	// JSON numbers decode to float64.
	if body["user"] != float64(42) {
		t.Errorf("request body user = %v, want 42", body["user"])
	}
	if token.Identifier != "alice-token" {
		t.Errorf("token.Identifier = %q, want alice-token", token.Identifier)
	}
}

func TestUpdateUserToken(t *testing.T) {
	var method, path string
	var body map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusNoContent)
	})

	resp, err := UpdateUserToken(context.Background(), client, "alice-token", "custom-key-value")
	if err != nil {
		t.Fatalf("UpdateUserToken: %v", err)
	}
	if method != http.MethodPost || path != "/api/v3/core/tokens/alice-token/set_key/" {
		t.Errorf("request = %s %s, want POST /api/v3/core/tokens/alice-token/set_key/", method, path)
	}
	if body["key"] != "custom-key-value" {
		t.Errorf("request body key = %v, want custom-key-value", body["key"])
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestRetrieveUserToken(t *testing.T) {
	var method, path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		writeJSON(t, w, http.StatusOK, `{"key":"resolved-token-key"}`)
	})

	tv, _, err := RetrieveUserToken(context.Background(), client, "alice-token")
	if err != nil {
		t.Fatalf("RetrieveUserToken: %v", err)
	}
	if method != http.MethodGet || path != "/api/v3/core/tokens/alice-token/view_key/" {
		t.Errorf("request = %s %s, want GET /api/v3/core/tokens/alice-token/view_key/", method, path)
	}
	if tv.Key != "resolved-token-key" {
		t.Errorf("tv.Key = %q, want resolved-token-key", tv.Key)
	}
}

func TestListUser(t *testing.T) {
	var method, path, username string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		username = r.URL.Query().Get("username")
		writeJSON(t, w, http.StatusOK, `{"pagination":{"next":0,"previous":0,"count":0,"current":1,"total_pages":1,"start_index":0,"end_index":0},"results":[],"autocomplete":{}}`)
	})

	pl, _, err := ListUser(context.Background(), client, "alice")
	if err != nil {
		t.Fatalf("ListUser: %v", err)
	}
	if method != http.MethodGet || path != "/api/v3/core/users/" {
		t.Errorf("request = %s %s, want GET /api/v3/core/users/", method, path)
	}
	if username != "alice" {
		t.Errorf("username query = %q, want alice", username)
	}
	if pl == nil {
		t.Fatal("ListUser returned nil PaginatedUserList")
	}
}

func TestMeRetrieveUser(t *testing.T) {
	var method, path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		writeJSON(t, w, http.StatusOK, `{"user":{"pk":42,"username":"alice","name":"alice","uid":"u1","avatar":"","is_active":true,"is_superuser":false,"groups":[{"pk":"group-uuid-1","name":"admins"}],"roles":[],"settings":{},"system_permissions":[]}}`)
	})

	su, _, err := MeRetrieveUser(context.Background(), client)
	if err != nil {
		t.Fatalf("MeRetrieveUser: %v", err)
	}
	if method != http.MethodGet || path != "/api/v3/core/users/me/" {
		t.Errorf("request = %s %s, want GET /api/v3/core/users/me/", method, path)
	}
	groups := su.GetUser().Groups
	if len(groups) != 1 || groups[0].Name != "admins" {
		t.Errorf("user groups = %v, want one group named admins", groups)
	}
}
