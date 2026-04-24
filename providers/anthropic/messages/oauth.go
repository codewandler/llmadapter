package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	claudeCredentialsFile = ".credentials.json"
	claudeLocalTokenKey   = "default"
	anthropicOAuthClient  = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
)

var claudeOAuthTokenEndpoint = "https://console.anthropic.com/v1/oauth/token"

type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func (t *Token) expired() bool {
	return t == nil || t.AccessToken == "" || time.Now().Add(30*time.Second).After(t.ExpiresAt)
}

type TokenProvider interface {
	Token(context.Context) (*Token, error)
}

type TokenStore interface {
	Load(context.Context, string) (*Token, error)
	Save(context.Context, string, *Token) error
}

type staticTokenProvider struct {
	token *Token
}

func NewStaticTokenProvider(token *Token) TokenProvider {
	return &staticTokenProvider{token: token}
}

func (p *staticTokenProvider) Token(context.Context) (*Token, error) {
	if p.token == nil || p.token.AccessToken == "" {
		return nil, fmt.Errorf("anthropic OAuth token is empty")
	}
	return p.token, nil
}

type ManagedTokenProvider struct {
	key    string
	store  TokenStore
	client *http.Client

	mu     sync.Mutex
	cached *Token
}

func NewManagedTokenProvider(key string, store TokenStore) *ManagedTokenProvider {
	return &ManagedTokenProvider{key: key, store: store, client: http.DefaultClient}
}

func (p *ManagedTokenProvider) Token(ctx context.Context) (*Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cached != nil && !p.cached.expired() {
		return p.cached, nil
	}
	token, err := p.store.Load(ctx, p.key)
	if err != nil {
		return nil, fmt.Errorf("load token: %w", err)
	}
	if token == nil {
		return nil, fmt.Errorf("no Claude OAuth token found")
	}
	if token.expired() {
		if token.RefreshToken == "" {
			return nil, fmt.Errorf("Claude OAuth token is expired and has no refresh token")
		}
		token, err = p.refresh(ctx, token.RefreshToken)
		if err != nil {
			return nil, err
		}
		if err := p.store.Save(ctx, p.key, token); err != nil {
			return nil, fmt.Errorf("save refreshed token: %w", err)
		}
	}
	p.cached = token
	return token, nil
}

func (p *ManagedTokenProvider) refresh(ctx context.Context, refreshToken string) (*Token, error) {
	body, err := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     anthropicOAuthClient,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeOAuthTokenEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh Claude OAuth token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh Claude OAuth token: HTTP %d", resp.StatusCode)
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode refreshed Claude OAuth token: %w", err)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("refreshed Claude OAuth token is empty")
	}
	if out.RefreshToken == "" {
		out.RefreshToken = refreshToken
	}
	return &Token{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(out.ExpiresIn) * time.Second),
	}, nil
}

type LocalTokenStore struct {
	path string
}

func NewLocalTokenStore() (*LocalTokenStore, error) {
	dir, err := DefaultClaudeDir()
	if err != nil {
		return nil, err
	}
	return NewLocalTokenStoreWithPath(filepath.Join(dir, claudeCredentialsFile))
}

func NewLocalTokenStoreWithPath(path string) (*LocalTokenStore, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("claude credentials not found at %s: %w", path, err)
	}
	return &LocalTokenStore{path: path}, nil
}

func DefaultClaudeDir() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

func LocalTokenStoreAvailable() bool {
	dir, err := DefaultClaudeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, claudeCredentialsFile))
	return err == nil
}

func (s *LocalTokenStore) Load(ctx context.Context, key string) (*Token, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var creds struct {
		ClaudeAiOauth *struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresAt    int64  `json:"expiresAt"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	if creds.ClaudeAiOauth == nil || creds.ClaudeAiOauth.AccessToken == "" {
		return nil, nil
	}
	return &Token{
		AccessToken:  creds.ClaudeAiOauth.AccessToken,
		RefreshToken: creds.ClaudeAiOauth.RefreshToken,
		ExpiresAt:    time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt),
	}, nil
}

func (s *LocalTokenStore) Save(ctx context.Context, key string, token *Token) error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if root == nil {
		root = make(map[string]json.RawMessage)
	}
	var oauth map[string]any
	if raw := root["claudeAiOauth"]; raw != nil {
		_ = json.Unmarshal(raw, &oauth)
	}
	if oauth == nil {
		oauth = make(map[string]any)
	}
	oauth["accessToken"] = token.AccessToken
	oauth["refreshToken"] = token.RefreshToken
	oauth["expiresAt"] = token.ExpiresAt.UnixMilli()
	oauthBytes, err := json.Marshal(oauth)
	if err != nil {
		return err
	}
	root["claudeAiOauth"] = oauthBytes
	out, err := json.Marshal(root)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
