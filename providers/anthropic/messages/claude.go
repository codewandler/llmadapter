package messages

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/codewandler/llmadapter/transport"
)

const (
	claudeUserAgent       = "claude-cli/2.1.85 (external, sdk-cli)"
	claudeBeta            = "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,context-management-2025-06-27,prompt-caching-scope-2026-01-05,effort-2025-11-24"
	claudeBillingHeader   = "x-anthropic-billing-header: cc_version=2.1.85.613; cc_entrypoint=sdk-cli; cch=1757e;"
	claudeSystemCore      = "You are a Claude agent, built on Anthropic's Claude Agent SDK."
	claudeStainlessPkgVer = "0.74.0"
	claudeStainlessNode   = "v24.3.0"
)

func WithBearerTokenProvider(provider TokenProvider) Option {
	return optionFunc(func(c *Config) error {
		if provider == nil {
			return fmt.Errorf("anthropic bearer token provider is required")
		}
		c.APIKey = ""
		c.NoAPIKeyAuth = true
		c.HeaderFns = append(c.HeaderFns, func(ctx context.Context, req *http.Request) error {
			token, err := provider.Token(ctx)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			return nil
		})
		return nil
	})
}

func WithLocalClaudeOAuth() Option {
	return optionFunc(func(c *Config) error {
		store, err := NewLocalTokenStore()
		if err != nil {
			return err
		}
		return WithBearerTokenProvider(NewManagedTokenProvider(claudeLocalTokenKey, store)).applyAnthropic(c)
	})
}

func WithClaudeCode() Option {
	return optionFunc(func(c *Config) error {
		for _, opt := range []Option{
			WithLocalClaudeOAuth(),
			WithClaudeHeaders(),
			WithClaudeCodePreflight(),
			WithSystemCacheControl(""),
		} {
			if err := opt.applyAnthropic(c); err != nil {
				return err
			}
		}
		return nil
	})
}

func WithClaudeHeaders() Option {
	return WithHeaderFunc(func(ctx context.Context, req *http.Request) error {
		q := req.URL.Query()
		q.Set("beta", "true")
		req.URL.RawQuery = q.Encode()

		req.Header.Set("User-Agent", claudeUserAgent)
		req.Header.Set("Anthropic-Beta", claudeBeta)
		req.Header.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")
		req.Header.Set("X-App", "cli")
		req.Header.Set("X-Stainless-Lang", "js")
		req.Header.Set("X-Stainless-Os", claudeStainlessOS())
		req.Header.Set("X-Stainless-Arch", claudeStainlessArch())
		req.Header.Set("X-Stainless-Package-Version", claudeStainlessPkgVer)
		req.Header.Set("X-Stainless-Retry-Count", "0")
		req.Header.Set("X-Stainless-Runtime", "node")
		req.Header.Set("X-Stainless-Runtime-Version", claudeStainlessNode)
		req.Header.Set("X-Stainless-Timeout", "600")
		req.Header.Set("Accept-Encoding", transport.ExtendedAcceptEncoding)
		req.Header.Set("Connection", "keep-alive")
		return nil
	})
}

func WithClaudeCodePreflight() Option {
	return optionFunc(func(c *Config) error {
		c.ProviderRequestProcessors = append(c.ProviderRequestProcessors, NewClaudeCodePreflightProcessor())
		return nil
	})
}

type ClaudeCodePreflightProcessor struct {
	sessionID string
}

func NewClaudeCodePreflightProcessor() *ClaudeCodePreflightProcessor {
	return &ClaudeCodePreflightProcessor{sessionID: randomUUID()}
}

func (p *ClaudeCodePreflightProcessor) ProcessProviderRequest(ctx context.Context, req *MessageRequest) error {
	if req.System == nil {
		req.System = &SystemContent{}
	}
	req.System.Prepend(
		ContentBlock{Type: "text", Text: claudeBillingHeader},
		ContentBlock{Type: "text", Text: claudeSystemCore},
	)

	if userID := buildClaudeUserID(p.sessionID); userID != "" {
		if req.Metadata == nil {
			req.Metadata = make(map[string]any)
		}
		req.Metadata["user_id"] = userID
	}
	return nil
}

func WithSystemCacheControl(ttl string) Option {
	return optionFunc(func(c *Config) error {
		c.ProviderRequestProcessors = append(c.ProviderRequestProcessors, systemCacheControlProcessor{ttl: ttl})
		return nil
	})
}

type systemCacheControlProcessor struct {
	ttl string
}

func (p systemCacheControlProcessor) ProcessProviderRequest(ctx context.Context, req *MessageRequest) error {
	if req.System == nil {
		return nil
	}
	req.System.ApplyCacheToLastText(&CacheControl{Type: "ephemeral", TTL: p.ttl})
	return nil
}

func NewStaticBearerToken(token string) *Token {
	return &Token{AccessToken: token, ExpiresAt: time.Now().Add(24 * time.Hour)}
}

func claudeStainlessOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "MacOS"
	case "windows":
		return "Windows"
	default:
		return "Linux"
	}
}

func claudeStainlessArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	default:
		return "x64"
	}
}

func buildClaudeUserID(sessionID string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return ""
	}
	var cfg struct {
		UserID       string `json:"userID"`
		OAuthAccount struct {
			AccountUUID string `json:"accountUuid"`
		} `json:"oauthAccount"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.UserID == "" {
		return ""
	}
	meta := map[string]string{
		"device_id":  cfg.UserID,
		"session_id": sessionID,
	}
	if cfg.OAuthAccount.AccountUUID != "" {
		meta["account_uuid"] = cfg.OAuthAccount.AccountUUID
	}
	out, err := json.Marshal(meta)
	if err != nil {
		return ""
	}
	return string(out)
}

func randomUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}
