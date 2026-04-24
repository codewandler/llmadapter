package codex

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/codewandler/llmadapter/transport"
)

type tokenStore struct {
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id"`
}

type authFile struct {
	AuthMode    string     `json:"auth_mode"`
	APIKey      *string    `json:"OPENAI_API_KEY"`
	Tokens      tokenStore `json:"tokens"`
	LastRefresh time.Time  `json:"last_refresh"`
}

type Auth struct {
	mu         sync.Mutex
	auth       authFile
	path       string
	expiry     time.Time
	httpClient *http.Client
}

func LoadAuth() (*Auth, error) {
	return LoadAuthFrom(authPathFromEnv())
}

func LoadAuthFrom(path string) (*Auth, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("codex: get home dir: %w", err)
		}
		path = filepath.Join(home, AuthFilePath)
	}
	return loadAuthFrom(path)
}

func NewStaticAuth(accessToken string) *Auth {
	return &Auth{
		auth: authFile{Tokens: tokenStore{AccessToken: accessToken}},
	}
}

func LocalAvailable() bool {
	auth, err := LoadAuth()
	return err == nil && (auth.auth.Tokens.AccessToken != "" || auth.auth.Tokens.RefreshToken != "")
}

func (a *Auth) WithHTTPClient(client *http.Client) *Auth {
	if client != nil {
		a.httpClient = client
	}
	return a
}

func (a *Auth) SetHeaders(ctx context.Context, h http.Header) error {
	token, err := a.Token(ctx)
	if err != nil {
		return err
	}
	h.Set("Authorization", "Bearer "+token)
	if accountID := a.AccountID(); accountID != "" {
		h.Set(HeaderChatGPTAccountID, accountID)
	}
	return nil
}

func (a *Auth) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.expiry.IsZero() && time.Now().Add(TokenExpiryBuffer).Before(a.expiry) {
		return a.auth.Tokens.AccessToken, nil
	}
	if a.expiry.IsZero() && a.auth.Tokens.AccessToken != "" && a.auth.Tokens.RefreshToken == "" {
		return a.auth.Tokens.AccessToken, nil
	}
	if a.auth.Tokens.RefreshToken == "" {
		if a.auth.Tokens.AccessToken != "" {
			return a.auth.Tokens.AccessToken, nil
		}
		return "", fmt.Errorf("codex: no access token")
	}
	return a.refreshLocked(ctx)
}

func (a *Auth) AccountID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.auth.Tokens.AccountID
}

func loadAuthFrom(path string) (*Auth, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("codex: read %s: %w", path, err)
	}
	var auth authFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("codex: parse auth file: %w", err)
	}
	if auth.AuthMode != "" && auth.AuthMode != ChatGPTAuthMode {
		return nil, fmt.Errorf("codex: unsupported auth mode %q", auth.AuthMode)
	}
	if auth.Tokens.AccessToken == "" && auth.Tokens.RefreshToken == "" {
		return nil, fmt.Errorf("codex: no tokens in %s", path)
	}
	a := &Auth{auth: auth, path: path, httpClient: transport.DefaultHTTPClient()}
	if exp, err := jwtExpiry(auth.Tokens.AccessToken); err == nil {
		a.expiry = exp
	}
	return a, nil
}

func (a *Auth) refreshLocked(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {a.auth.Tokens.RefreshToken},
		"client_id":     {ClientID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("codex: build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := a.httpClient
	if client == nil {
		client = transport.DefaultHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("codex: token refresh: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("codex: decode refresh response (status %d): %w", resp.StatusCode, err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("codex: token refresh failed: %s: %s", result.Error, result.ErrorDesc)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("codex: empty access token in refresh response (status %d)", resp.StatusCode)
	}

	a.auth.Tokens.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		a.auth.Tokens.RefreshToken = result.RefreshToken
	}
	if result.ExpiresIn > 0 {
		a.expiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	} else if exp, err := jwtExpiry(result.AccessToken); err == nil {
		a.expiry = exp
	} else {
		a.expiry = time.Time{}
	}
	a.saveLocked()
	return result.AccessToken, nil
}

func (a *Auth) saveLocked() {
	if a.path == "" {
		return
	}
	a.auth.LastRefresh = time.Now().UTC()
	data, err := json.MarshalIndent(a.auth, "", "  ")
	if err == nil {
		_ = os.WriteFile(a.path, data, 0o600)
	}
}

func jwtExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&claims); err != nil {
		return time.Time{}, fmt.Errorf("decode JWT claims: %w", err)
	}
	if claims.Exp == 0 {
		return time.Time{}, fmt.Errorf("JWT has no exp claim")
	}
	return time.Unix(claims.Exp, 0), nil
}

func authPathFromEnv() string {
	if path := os.Getenv(EnvAuthPath); path != "" {
		return path
	}
	return ""
}
