package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	configDir  = ".coding-contest-cli"
	configFile = "session.json"
)

type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Username  string    `json:"username"`
}

type PAT struct {
	ID          string     `json:"id"`
	Enabled     bool       `json:"enabled"`
	Role        string     `json:"role"`
	Permissions []string   `json:"permissions"`
	Token       string     `json:"token"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       string `json:"id,omitempty"`
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"user"`
}

type PATResponse struct {
	PersonalAccessToken struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	} `json:"personalAccessToken"`
}

func LoadSession() (*Session, error) {
	configPath := getSessionPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}

	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt) {
		return nil, nil
	}

	return &session, nil
}

func SaveSession(token, username string) error {
	configPath := getSessionPath()
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	session := Session{
		Token:     token,
		Username:  username,
		ExpiresAt: time.Time{},
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

func ClearSession() error {
	configPath := getSessionPath()
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func Login(baseURL, apiPrefix, username, password string) (string, string, error) {
	url := fmt.Sprintf("%s%s/auth/login", baseURL, apiPrefix)

	reqBody := LoginRequest{
		Email:    username,
		Password: password,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("login failed: %s", string(bodyBytes))
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", "", fmt.Errorf("parse login response: %w", err)
	}

	return loginResp.Token, loginResp.User.Username, nil
}

func CreatePAT(baseURL, apiPrefix, token, name string, expiresInDays int) (*PAT, error) {
	url := fmt.Sprintf("%s%s/pat", baseURL, apiPrefix)

	// server expects permissions array and optional expiresAt; name is not supported server-side
	reqBody := map[string]interface{}{
		"permissions": []string{"contest:read", "submission:write"},
	}
	if expiresInDays > 0 {
		expiresAt := time.Now().AddDate(0, 0, expiresInDays)
		reqBody["expiresAt"] = expiresAt.Format(time.RFC3339)
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create PAT request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create PAT failed: %s", string(bodyBytes))
	}

	var patResp struct {
		Token       string     `json:"token"`
		ID          string     `json:"id"`
		Enabled     bool       `json:"enabled"`
		Permissions []string   `json:"permissions"`
		ExpiresAt   *time.Time `json:"expiresAt"`
		CreatedAt   time.Time  `json:"createdAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&patResp); err != nil {
		return nil, fmt.Errorf("parse PAT response: %w", err)
	}

	return &PAT{
		ID:          patResp.ID,
		Enabled:     patResp.Enabled,
		Permissions: patResp.Permissions,
		Token:       patResp.Token,
		CreatedAt:   patResp.CreatedAt,
		ExpiresAt:   patResp.ExpiresAt,
	}, nil
}

func GetPATs(baseURL, apiPrefix, token string) ([]PAT, error) {
	url := fmt.Sprintf("%s%s/pat", baseURL, apiPrefix)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get PATs request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get PATs failed: %s", string(bodyBytes))
	}

	var rows []struct {
		ID          string     `json:"id"`
		Enabled     bool       `json:"enabled"`
		Role        string     `json:"role"`
		Permissions []string   `json:"permissions"`
		ExpiresAt   *time.Time `json:"expiresAt"`
		LastUsedAt  *time.Time `json:"lastUsedAt"`
		CreatedAt   time.Time  `json:"createdAt"`
		UpdatedAt   time.Time  `json:"updatedAt"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return nil, fmt.Errorf("parse PATs response: %w", err)
	}

	pats := make([]PAT, len(rows))
	for i, p := range rows {
		pats[i] = PAT{
			ID:          p.ID,
			Enabled:     p.Enabled,
			Role:        p.Role,
			Permissions: p.Permissions,
			CreatedAt:   p.CreatedAt,
			ExpiresAt:   p.ExpiresAt,
		}
	}
	return pats, nil
}

func DeletePAT(baseURL, apiPrefix, token, patID string) error {
	url := fmt.Sprintf("%s%s/pat/%s", baseURL, apiPrefix, patID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete PAT request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete PAT failed: %s", string(bodyBytes))
	}

	return nil
}

func getSessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, configDir, configFile)
}
