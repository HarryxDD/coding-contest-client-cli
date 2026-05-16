package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionPersistence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := SaveSession("token-123", "alice"); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	session, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.Token != "token-123" || session.Username != "alice" {
		t.Fatalf("unexpected session %+v", session)
	}

	path := filepath.Join(home, configDir, configFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected session file: %v", err)
	}

	if err := ClearSession(); err != nil {
		t.Fatalf("ClearSession() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected session file to be removed, got err=%v", err)
	}
}

func TestLoginAndPATEndpoints(t *testing.T) {
	var loginBody map[string]string
	var createPATBody map[string]any
	var deleteCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth/login":
			_ = json.NewDecoder(r.Body).Decode(&loginBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "jwt-123",
				"user": map[string]any{
					"username": "alice",
					"email":    "alice@example.com",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/pat":
			_ = json.NewDecoder(r.Body).Decode(&createPATBody)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":       "pat-raw",
				"id":          "pat-1",
				"enabled":     true,
				"permissions": []string{"contest:read"},
				"createdAt":   time.Now().UTC(),
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/pat":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":          "pat-1",
				"enabled":     true,
				"role":        "participant",
				"permissions": []string{"contest:read"},
				"createdAt":   time.Now().UTC(),
			}})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/pat/pat-1":
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	token, username, err := Login(server.URL, "/api", "alice@example.com", "secret")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if token != "jwt-123" || username != "alice" {
		t.Fatalf("unexpected login result token=%q username=%q", token, username)
	}
	if loginBody["email"] != "alice@example.com" || loginBody["password"] != "secret" {
		t.Fatalf("unexpected login body %+v", loginBody)
	}

	pat, err := CreatePAT(server.URL, "/api", token, "ignored-name", 7)
	if err != nil {
		t.Fatalf("CreatePAT() error = %v", err)
	}
	if pat.ID != "pat-1" || pat.Token != "pat-raw" || !pat.Enabled {
		t.Fatalf("unexpected PAT %+v", pat)
	}
	if _, ok := createPATBody["permissions"]; !ok {
		t.Fatalf("expected permissions in create PAT body, got %+v", createPATBody)
	}

	rows, err := GetPATs(server.URL, "/api", token)
	if err != nil {
		t.Fatalf("GetPATs() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "pat-1" {
		t.Fatalf("unexpected PAT rows %+v", rows)
	}

	if err := DeletePAT(server.URL, "/api", token, "pat-1"); err != nil {
		t.Fatalf("DeletePAT() error = %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected delete to be called")
	}
}
