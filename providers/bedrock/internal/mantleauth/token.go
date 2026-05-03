package mantleauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

const (
	EnvAPIKey        = "BEDROCK_API_KEY"
	EnvBearerToken   = "AWS_BEARER_TOKEN_BEDROCK"
	EnvRegion        = "AWS_REGION"
	EnvDefaultRegion = "AWS_DEFAULT_REGION"
	DefaultRegion    = "us-east-1"

	defaultTokenExpiresInSeconds = 12 * 60 * 60
	maxTokenExpiresInSeconds     = 12 * 60 * 60
	tokenRefreshSkew             = 5 * time.Minute
	emptyPayloadSHA256           = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	bedrockTokenService          = "bedrock"
	bedrockTokenHost             = "bedrock.amazonaws.com"
	bedrockTokenAction           = "CallWithBearerToken"
	BedrockTokenPrefix           = "bedrock-api-key-"
	bedrockTokenVersion          = "&Version=1"
)

type TokenProvider interface {
	Token(context.Context) (string, error)
}

type TokenProviderFunc func(context.Context) (string, error)

func (f TokenProviderFunc) Token(ctx context.Context) (string, error) {
	return f(ctx)
}

type AWSTokenProvider struct {
	region           string
	expiresInSeconds int
	now              func() time.Time
	loadConfig       func(context.Context, string) (aws.Config, error)
	presign          func(context.Context, aws.Credentials, string, int, time.Time) (string, error)

	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewAWSTokenProvider(region string) *AWSTokenProvider {
	return &AWSTokenProvider{region: region}
}

func (p *AWSTokenProvider) Token(ctx context.Context) (string, error) {
	now := p.clock()()
	p.mu.Lock()
	if p.token != "" && now.Add(tokenRefreshSkew).Before(p.expires) {
		token := p.token
		p.mu.Unlock()
		return token, nil
	}
	p.mu.Unlock()

	cfg, err := p.configLoader()(ctx, RegionFromEnv(p.region))
	if err != nil {
		return "", fmt.Errorf("load AWS config for Bedrock token: %w", err)
	}
	region := p.region
	if region == "" {
		region = cfg.Region
	}
	if region == "" {
		region = DefaultRegion
	}
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return "", fmt.Errorf("retrieve AWS credentials for Bedrock token: %w", err)
	}
	expiresIn := p.tokenTTL(creds, now)
	token, err := p.presigner()(ctx, creds, region, expiresIn, now)
	if err != nil {
		return "", fmt.Errorf("generate Bedrock token: %w", err)
	}
	expiry := now.Add(time.Duration(expiresIn) * time.Second)
	if creds.CanExpire && !creds.Expires.IsZero() && creds.Expires.Before(expiry) {
		expiry = creds.Expires
	}

	p.mu.Lock()
	p.token = token
	p.expires = expiry
	p.mu.Unlock()
	return token, nil
}

func (p *AWSTokenProvider) clock() func() time.Time {
	if p.now != nil {
		return p.now
	}
	return time.Now
}

func (p *AWSTokenProvider) configLoader() func(context.Context, string) (aws.Config, error) {
	if p.loadConfig != nil {
		return p.loadConfig
	}
	return func(ctx context.Context, region string) (aws.Config, error) {
		opts := []func(*awsconfig.LoadOptions) error{}
		if region != "" {
			opts = append(opts, awsconfig.WithRegion(region))
		}
		return awsconfig.LoadDefaultConfig(ctx, opts...)
	}
}

func RegionFromEnv(fallback string) string {
	if region := os.Getenv(EnvRegion); region != "" {
		return region
	}
	if region := os.Getenv(EnvDefaultRegion); region != "" {
		return region
	}
	return fallback
}

func (p *AWSTokenProvider) presigner() func(context.Context, aws.Credentials, string, int, time.Time) (string, error) {
	if p.presign != nil {
		return p.presign
	}
	return GenerateToken
}

func (p *AWSTokenProvider) tokenTTL(creds aws.Credentials, now time.Time) int {
	expiresIn := p.expiresInSeconds
	if expiresIn <= 0 || expiresIn > maxTokenExpiresInSeconds {
		expiresIn = defaultTokenExpiresInSeconds
	}
	if creds.CanExpire && !creds.Expires.IsZero() {
		until := int(creds.Expires.Sub(now).Seconds())
		if until > 0 && until < expiresIn {
			expiresIn = until
		}
	}
	if expiresIn <= 0 {
		expiresIn = 1
	}
	return expiresIn
}

func GenerateToken(ctx context.Context, creds aws.Credentials, region string, expiresInSeconds int, signingTime time.Time) (string, error) {
	if region == "" {
		region = DefaultRegion
	}
	if expiresInSeconds <= 0 || expiresInSeconds > maxTokenExpiresInSeconds {
		return "", fmt.Errorf("expiresInSeconds must be in range (0, %d]", maxTokenExpiresInSeconds)
	}
	reqURL := url.URL{
		Scheme: "https",
		Host:   bedrockTokenHost,
		Path:   "/",
	}
	query := reqURL.Query()
	query.Set("Action", bedrockTokenAction)
	query.Set("X-Amz-Expires", strconv.Itoa(expiresInSeconds))
	reqURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Host = bedrockTokenHost
	req.Header.Set("Host", bedrockTokenHost)
	signedURL, _, err := v4.NewSigner().PresignHTTP(ctx, creds, req, emptyPayloadSHA256, bedrockTokenService, region, signingTime)
	if err != nil {
		return "", err
	}
	stripped := strings.TrimPrefix(signedURL, "https://")
	encoded := base64.StdEncoding.EncodeToString([]byte(stripped + bedrockTokenVersion))
	return BedrockTokenPrefix + encoded, nil
}

func ExplicitTokenFromEnv() string {
	if token := os.Getenv(EnvAPIKey); token != "" {
		return token
	}
	if token := os.Getenv(EnvBearerToken); token != "" {
		return token
	}
	return ""
}
