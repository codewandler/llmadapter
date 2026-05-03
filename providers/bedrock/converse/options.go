package converse

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

const (
	EnvRegion        = "AWS_REGION"
	EnvDefaultRegion = "AWS_DEFAULT_REGION"
	EnvModel         = "BEDROCK_CONVERSE_MODEL"

	ModelClaudeSonnet46 = "anthropic.claude-sonnet-4-6"
	ModelClaudeHaiku45  = "anthropic.claude-haiku-4-5-20251001-v1:0"
	ModelClaudeOpus47   = "anthropic.claude-opus-4-7"
	ModelClaudeOpus46   = "anthropic.claude-opus-4-6-v1"
	ModelClaudeSonnet45 = "anthropic.claude-sonnet-4-5-20250929-v1:0"

	DefaultModel = ModelClaudeSonnet46
)

type Option interface {
	applyBedrockConverse(*config)
}

type config struct {
	region              string
	profile             string
	baseEndpoint        string
	credentialsProvider aws.CredentialsProvider
	httpClient          *http.Client
	client              converseStreamClient
	loadConfig          func(context.Context, []func(*awsconfig.LoadOptions) error) (aws.Config, error)
}

type optionFunc func(*config)

func (f optionFunc) applyBedrockConverse(c *config) { f(c) }

func WithRegion(region string) Option {
	return optionFunc(func(c *config) {
		c.region = strings.TrimSpace(region)
	})
}

func WithProfile(profile string) Option {
	return optionFunc(func(c *config) {
		c.profile = strings.TrimSpace(profile)
	})
}

func WithBaseEndpoint(endpoint string) Option {
	return optionFunc(func(c *config) {
		c.baseEndpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	})
}

func WithCredentialsProvider(provider aws.CredentialsProvider) Option {
	return optionFunc(func(c *config) {
		c.credentialsProvider = provider
	})
}

func WithHTTPClient(client *http.Client) Option {
	return optionFunc(func(c *config) {
		c.httpClient = client
	})
}

func WithRuntimeClient(client converseStreamClient) Option {
	return optionFunc(func(c *config) {
		c.client = client
	})
}

type Client struct {
	mu       sync.Mutex
	client   converseStreamClient
	cfg      config
	region   string
	prefix   string
	initOnce sync.Once
	initErr  error
}

func NewClient(opts ...Option) (*Client, error) {
	cfg := config{region: defaultRegionFromEnv()}
	for _, opt := range opts {
		opt.applyBedrockConverse(&cfg)
	}
	if cfg.region == "" {
		cfg.region = DefaultRegion
	}
	client := &Client{
		client: cfg.client,
		cfg:    cfg,
		region: cfg.region,
		prefix: computeRegionPrefix(cfg.region),
	}
	return client, nil
}

func defaultRegionFromEnv() string {
	if region := os.Getenv(EnvRegion); region != "" {
		return region
	}
	if region := os.Getenv(EnvDefaultRegion); region != "" {
		return region
	}
	return DefaultRegion
}

func (c *Client) runtimeClient(ctx context.Context) (converseStreamClient, error) {
	c.mu.Lock()
	if c.client != nil {
		client := c.client
		c.mu.Unlock()
		return client, nil
	}
	c.mu.Unlock()

	c.initOnce.Do(func() {
		loader := c.cfg.loadConfig
		if loader == nil {
			loader = func(ctx context.Context, opts []func(*awsconfig.LoadOptions) error) (aws.Config, error) {
				return awsconfig.LoadDefaultConfig(ctx, opts...)
			}
		}
		var opts []func(*awsconfig.LoadOptions) error
		if c.cfg.region != "" {
			opts = append(opts, awsconfig.WithRegion(c.cfg.region))
		}
		if c.cfg.profile != "" {
			opts = append(opts, awsconfig.WithSharedConfigProfile(c.cfg.profile))
		}
		if c.cfg.credentialsProvider != nil {
			opts = append(opts, awsconfig.WithCredentialsProvider(c.cfg.credentialsProvider))
		}
		if c.cfg.httpClient != nil {
			opts = append(opts, awsconfig.WithHTTPClient(c.cfg.httpClient))
		}
		awsCfg, err := loader(ctx, opts)
		if err != nil {
			c.initErr = fmt.Errorf("load AWS config for Bedrock Converse: %w", err)
			return
		}
		if c.region == "" {
			c.region = awsCfg.Region
		}
		if c.region == "" {
			c.region = DefaultRegion
		}
		c.prefix = computeRegionPrefix(c.region)
		c.client = bedrockruntime.NewFromConfig(awsCfg, func(o *bedrockruntime.Options) {
			if c.cfg.baseEndpoint != "" {
				o.BaseEndpoint = aws.String(c.cfg.baseEndpoint)
			}
		})
	})
	if c.initErr != nil {
		return nil, c.initErr
	}
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil {
		return nil, fmt.Errorf("bedrock converse runtime client was not initialized")
	}
	return client, nil
}
