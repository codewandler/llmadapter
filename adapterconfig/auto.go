package adapterconfig

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/providerregistry"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

var autoAWSCredentialsAvailable = defaultAutoAWSCredentialsAvailable

type AutoOptions struct {
	EnableEnv         bool
	EnableLocalClaude bool
	EnableLocalCodex  bool
	UseModelDB        bool
	DynamicModels     bool
	SourceAPI         adapt.ApiKind
	Intents           []AutoIntent
	ModelDBAliases    []ModelDBAliasConfig
}

type AutoIntent struct {
	Name      string
	SourceAPI adapt.ApiKind
}

type AutoResult struct {
	Client  unified.Client
	Config  Config
	Enabled []AutoProvider
	Skipped []AutoProvider
}

type AutoProvider struct {
	Name   string        `json:"name"`
	Type   string        `json:"type"`
	Reason string        `json:"reason,omitempty"`
	API    adapt.ApiKind `json:"api,omitempty"`
	Model  string        `json:"model,omitempty"`
}

func AutoMuxClient(opts AutoOptions) (AutoResult, error) {
	if !opts.EnableEnv && !opts.EnableLocalClaude && !opts.EnableLocalCodex {
		opts.EnableEnv = true
		opts.EnableLocalClaude = true
		opts.EnableLocalCodex = true
	}
	cfg := Config{ModelDB: autoModelDBConfig(opts)}
	var enabled []AutoProvider
	var skipped []AutoProvider
	for _, descriptor := range providerregistry.List() {
		provider, status, ok := autoProvider(descriptor, opts)
		if !ok {
			skipped = append(skipped, status)
			continue
		}
		cfg.Providers = append(cfg.Providers, provider)
		enabled = append(enabled, status)
	}
	routes, err := autoRoutes(cfg, opts)
	if err != nil {
		return AutoResult{Config: cfg, Enabled: enabled, Skipped: skipped}, err
	}
	cfg.Routes = routes
	ApplyDefaults(&cfg)
	client, err := NewMuxClient(cfg, WithSourceAPI(opts.SourceAPI))
	if err != nil {
		return AutoResult{Config: cfg, Enabled: enabled, Skipped: skipped}, err
	}
	return AutoResult{Client: client, Config: cfg, Enabled: enabled, Skipped: skipped}, nil
}

func autoProvider(descriptor providerregistry.Descriptor, opts AutoOptions) (ProviderConfig, AutoProvider, bool) {
	model := modelFromEnv(descriptor)
	status := AutoProvider{Name: descriptor.Type, Type: descriptor.Type, API: descriptor.APIKind, Model: model}
	if descriptor.Type == "claude" {
		if opts.EnableLocalClaude && anthropic.LocalTokenStoreAvailable() {
			status.Reason = "local_claude_oauth"
			return autoProviderConfig(descriptor, "", model, opts), status, true
		}
		status.Reason = "missing local Claude OAuth credentials"
		return ProviderConfig{}, status, false
	}
	if descriptor.Type == "codex_responses" {
		if opts.EnableEnv {
			if key, envName := firstEnvWithName(descriptor.DefaultAPIKeyEnvs...); key != "" {
				status.Reason = "env:" + envName
				return autoProviderConfig(descriptor, envName, model, opts), status, true
			}
		}
		if opts.EnableLocalCodex && codex.LocalAvailable() {
			status.Reason = "local_codex_oauth"
			return autoProviderConfig(descriptor, "", model, opts), status, true
		}
		status.Reason = "missing Codex OAuth env/local credentials"
		return ProviderConfig{}, status, false
	}
	if descriptor.Type == "bedrock_converse" {
		if opts.EnableEnv && autoAWSCredentialsAvailable() {
			status.Reason = autoAWSReason()
			return autoProviderConfig(descriptor, "", model, opts), status, true
		}
		if !opts.EnableEnv {
			status.Reason = "env auto detection disabled"
		} else {
			status.Reason = "missing AWS SDK credentials"
		}
		return ProviderConfig{}, status, false
	}
	if !opts.EnableEnv {
		status.Reason = "env auto detection disabled"
		return ProviderConfig{}, status, false
	}
	if _, envName := firstEnvWithName(descriptor.DefaultAPIKeyEnvs...); envName != "" {
		status.Reason = "env:" + envName
		return autoProviderConfig(descriptor, envName, model, opts), status, true
	}
	status.Reason = "missing env credentials"
	return ProviderConfig{}, status, false
}

func autoProviderConfig(descriptor providerregistry.Descriptor, apiKeyEnv string, model string, opts AutoOptions) ProviderConfig {
	return ProviderConfig{
		Name:             descriptor.Type,
		Type:             descriptor.Type,
		APIKeyEnv:        apiKeyEnv,
		Model:            model,
		ModelDBServiceID: autoModelDBServiceID(descriptor.Type, opts.UseModelDB),
	}
}

func autoRoutes(cfg Config, opts AutoOptions) ([]RouteConfig, error) {
	catalog, modelDBEnabled, err := autoModelDBCatalog(cfg, opts)
	if err != nil {
		return nil, err
	}
	if len(opts.Intents) > 0 {
		var out []RouteConfig
		dynamic := map[adapt.ApiKind]bool{}
		for _, intent := range opts.Intents {
			for _, sourceAPI := range intentSourceAPIs(intent, opts) {
				intent.SourceAPI = sourceAPI
				route, ok := routeForIntent(cfg, intent, opts, catalog, modelDBEnabled)
				if !ok {
					continue
				}
				out = append(out, route)
				if opts.DynamicModels && !dynamic[route.SourceAPI] {
					out = append(out, dynamicRoute(route))
					dynamic[route.SourceAPI] = true
				}
			}
		}
		out = append(out, autoModelAliasRoutes(cfg, opts, catalog, modelDBEnabled, out)...)
		return out, nil
	}
	var routes []RouteConfig
	for _, sourceAPI := range []adapt.ApiKind{adapt.ApiOpenAIResponses, adapt.ApiOpenAIChatCompletions, adapt.ApiAnthropicMessages} {
		route, ok := bestRouteForAPI(cfg, sourceAPI)
		if ok {
			routes = append(routes, route)
			if opts.DynamicModels {
				routes = append(routes, dynamicRoute(route))
			}
		}
	}
	routes = append(routes, autoModelAliasRoutes(cfg, opts, catalog, modelDBEnabled, routes)...)
	return routes, nil
}

func autoModelAliasRoutes(cfg Config, opts AutoOptions, catalog modeldb.Catalog, modelDBEnabled bool, existing []RouteConfig) []RouteConfig {
	if !opts.UseModelDB || !modelDBEnabled {
		return nil
	}
	var out []RouteConfig
	for _, sourceAPI := range modelAliasRouteSourceAPIs(opts) {
		for _, alias := range modelDBAliasNames(cfg.ModelDB) {
			if hasRouteModel(existing, sourceAPI, alias) || hasRouteModel(out, sourceAPI, alias) {
				continue
			}
			route, ok := modelDBRouteForIntent(cfg, alias, sourceAPI, catalog)
			if !ok {
				continue
			}
			out = append(out, route)
		}
	}
	return out
}

func modelAliasRouteSourceAPIs(opts AutoOptions) []adapt.ApiKind {
	if opts.SourceAPI != "" {
		return []adapt.ApiKind{opts.SourceAPI}
	}
	return autoSourceAPIs()
}

func intentSourceAPIs(intent AutoIntent, opts AutoOptions) []adapt.ApiKind {
	if intent.SourceAPI != "" {
		return []adapt.ApiKind{intent.SourceAPI}
	}
	if opts.SourceAPI != "" {
		return []adapt.ApiKind{opts.SourceAPI}
	}
	return autoSourceAPIs()
}

func autoSourceAPIs() []adapt.ApiKind {
	return []adapt.ApiKind{adapt.ApiOpenAIResponses, adapt.ApiOpenAIChatCompletions, adapt.ApiAnthropicMessages}
}

func modelDBAliasNames(cfg ModelDBConfig) []string {
	seen := map[string]bool{}
	var out []string
	for _, alias := range cfg.Aliases {
		name := normalizeModelDBAlias(alias.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func hasRouteModel(routes []RouteConfig, sourceAPI adapt.ApiKind, model string) bool {
	for _, route := range routes {
		if route.SourceAPI == sourceAPI && route.Model == model {
			return true
		}
	}
	return false
}

func dynamicRoute(route RouteConfig) RouteConfig {
	route.Model = ""
	route.NativeModel = ""
	route.ModelDBModel = ""
	route.ModelDBWireModelID = ""
	route.DynamicModels = true
	if route.Weight == 0 || route.Weight >= 100 {
		route.Weight = 1
	}
	return route
}

func routeForIntent(cfg Config, intent AutoIntent, opts AutoOptions, catalog modeldb.Catalog, modelDBEnabled bool) (RouteConfig, bool) {
	sourceAPI := intent.SourceAPI
	if sourceAPI == "" {
		sourceAPI = adapt.ApiOpenAIResponses
	}
	if opts.UseModelDB && modelDBEnabled {
		if route, ok := modelDBRouteForIntent(cfg, intent.Name, sourceAPI, catalog); ok {
			return route, true
		}
	}
	return RouteConfig{}, false
}

func modelDBRouteForIntent(cfg Config, intentName string, sourceAPI adapt.ApiKind, catalog modeldb.Catalog) (RouteConfig, bool) {
	type candidate struct {
		route            RouteConfig
		providerType     string
		serviceID        string
		creator          string
		requestedService string
	}
	requestedService, _, _ := modelDBServiceQualifiedName(intentName)
	var candidates []candidate
	for _, providerType := range preferredProviderTypes(sourceAPI) {
		for _, provider := range cfg.Providers {
			if provider.Type != providerType {
				continue
			}
			descriptor, ok := descriptorForProvider(provider)
			if !ok {
				continue
			}
			serviceID := providerModelDBServiceID(provider)
			if serviceID == "" {
				continue
			}
			apiType, ok := modelmeta.APITypeForFamily(descriptor.Family)
			if !ok {
				continue
			}
			item, ok := resolveModelDBItem(catalog, cfg.ModelDB, serviceID, apiType, intentName)
			if !ok {
				continue
			}
			candidates = append(candidates, candidate{
				route: RouteConfig{
					SourceAPI:    sourceAPI,
					Model:        intentName,
					Provider:     provider.Name,
					ProviderAPI:  descriptor.APIKind,
					ModelDBModel: intentName,
					Weight:       100,
				},
				providerType:     provider.Type,
				serviceID:        serviceID,
				creator:          item.Model.Key.Creator,
				requestedService: requestedService,
			})
		}
	}
	if len(candidates) == 0 {
		return RouteConfig{}, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return modelDBRouteCandidateRank(candidates[i]) < modelDBRouteCandidateRank(candidates[j])
	})
	return candidates[0].route, true
}

func modelDBRouteCandidateRank(c struct {
	route            RouteConfig
	providerType     string
	serviceID        string
	creator          string
	requestedService string
}) int {
	rank := 1000
	if c.requestedService != "" && c.serviceID == c.requestedService {
		rank = -100
	} else if c.requestedService != "" {
		rank = 10000
	} else if c.creator != "" && c.serviceID == c.creator {
		rank = 0
	} else if c.creator != "" {
		rank = 100
	}
	return rank + modelDBProviderPreference(c.providerType)
}

func modelDBProviderPreference(providerType string) int {
	switch providerType {
	case "claude":
		return 0
	case "anthropic":
		return 1
	case "openai_responses", "openai_chat":
		return 2
	case "codex_responses":
		return 3
	case "bedrock_responses", "bedrock_messages", "bedrock_converse":
		return 4
	case "minimax_messages", "minimax_chat":
		return 5
	case "openrouter_messages", "openrouter_responses", "openrouter_chat":
		return 50
	default:
		return 100
	}
}

func autoModelDBConfig(opts AutoOptions) ModelDBConfig {
	if !opts.UseModelDB {
		return ModelDBConfig{}
	}
	return ModelDBConfig{Aliases: opts.ModelDBAliases}
}

func autoModelDBCatalog(cfg Config, opts AutoOptions) (modeldb.Catalog, bool, error) {
	if !opts.UseModelDB {
		return modeldb.Catalog{}, false, nil
	}
	catalog, err := LoadModelDBCatalog(cfg.ModelDB)
	if err != nil {
		return modeldb.Catalog{}, false, fmt.Errorf("load modeldb catalog for auto routes: %w", err)
	}
	return catalog, true, nil
}

func bestRouteForAPI(cfg Config, sourceAPI adapt.ApiKind) (RouteConfig, bool) {
	provider, ok := bestProviderForAPI(cfg, sourceAPI)
	if !ok {
		return RouteConfig{}, false
	}
	descriptor, _ := descriptorForProvider(provider)
	publicModel := descriptor.DefaultModel
	if provider.Model != "" {
		publicModel = provider.Model
	}
	return RouteConfig{
		SourceAPI:   sourceAPI,
		Model:       publicModel,
		Provider:    provider.Name,
		ProviderAPI: descriptor.APIKind,
		NativeModel: provider.Model,
		Weight:      100,
	}, true
}

func bestProviderForAPI(cfg Config, sourceAPI adapt.ApiKind) (ProviderConfig, bool) {
	preferred := preferredProviderTypes(sourceAPI)
	for _, providerType := range preferred {
		for _, provider := range cfg.Providers {
			if provider.Type == providerType {
				return provider, true
			}
		}
	}
	for _, provider := range cfg.Providers {
		descriptor, ok := descriptorForProvider(provider)
		if ok && descriptor.APIKind == sourceAPI {
			return provider, true
		}
	}
	return ProviderConfig{}, false
}

func preferredProviderTypes(sourceAPI adapt.ApiKind) []string {
	switch sourceAPI {
	case adapt.ApiOpenAIResponses:
		return []string{"openai_responses", "openrouter_responses", "codex_responses", "bedrock_responses", "anthropic", "claude", "bedrock_messages", "bedrock_converse", "openrouter_messages", "minimax_messages"}
	case adapt.ApiOpenAIChatCompletions:
		return []string{"openai_chat", "openrouter_chat", "minimax_chat", "anthropic", "claude", "bedrock_converse"}
	case adapt.ApiAnthropicMessages:
		return []string{"anthropic", "claude", "bedrock_messages", "bedrock_converse", "openrouter_messages", "minimax_messages"}
	default:
		return nil
	}
}

func autoModelDBServiceID(providerType string, enabled bool) string {
	if !enabled {
		return ""
	}
	return modelDBServiceIDForProviderType(providerType)
}

func modelFromEnv(descriptor providerregistry.Descriptor) string {
	if descriptor.DefaultModelEnv != "" {
		if value := os.Getenv(descriptor.DefaultModelEnv); value != "" {
			return value
		}
	}
	return descriptor.DefaultModel
}

func firstEnvWithName(keys ...string) (string, string) {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value, key
		}
	}
	return "", ""
}

func defaultAutoAWSCredentialsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(autoAWSRegion())}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return false
	}
	_, err = cfg.Credentials.Retrieve(ctx)
	return err == nil
}

func autoAWSRegion() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}
	return "us-east-1"
}

func autoAWSReason() string {
	if os.Getenv("AWS_PROFILE") != "" {
		return "aws_sdk:AWS_PROFILE"
	}
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		return "aws_sdk:AWS_ACCESS_KEY_ID"
	}
	if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" || os.Getenv("AWS_ROLE_ARN") != "" {
		return "aws_sdk:web_identity"
	}
	return "aws_sdk_credentials"
}
