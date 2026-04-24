package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/adapterconfig"
	"github.com/codewandler/llmadapter/gatewayserver"
	"github.com/codewandler/llmadapter/muxclient"
	"github.com/codewandler/llmadapter/providerregistry"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
	"github.com/spf13/cobra"
)

func main() {
	cmd := newRootCommand(os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(out, errOut io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llmadapter",
		Short: "LLM adapter provider, route, and smoke-test tooling",
		Long:  "llmadapter provides operator commands for provider endpoint discovery, route inspection, and smoke testing.",
	}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.AddCommand(newProvidersCommand())
	cmd.AddCommand(newRoutesCommand())
	cmd.AddCommand(newModelsCommand())
	cmd.AddCommand(newResolveCommand())
	cmd.AddCommand(newServeCommand())
	cmd.AddCommand(newSmokeCommand())
	return cmd
}

func run(args []string) error {
	cmd := newRootCommand(os.Stdout, os.Stderr)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func newProvidersCommand() *cobra.Command {
	var jsonOut bool
	var auto bool
	var status bool
	var configPath string
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List provider endpoint types or credential status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if auto {
				return runProvidersAuto(cmd.OutOrStdout(), jsonOut)
			}
			if status || configPath != "" {
				return runProvidersStatus(cmd.OutOrStdout(), configPath, jsonOut)
			}
			descriptors := providerregistry.List()
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), descriptors)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TYPE\tAPI_KIND\tFAMILY\tMODEL_ENV\tDEFAULT_MODEL")
			for _, descriptor := range descriptors {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", descriptor.Type, descriptor.APIKind, descriptor.Family, descriptor.DefaultModelEnv, descriptor.DefaultModel)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print providers as JSON")
	cmd.Flags().BoolVar(&auto, "auto", false, "show auto-detected provider endpoint credential status")
	cmd.Flags().BoolVar(&status, "status", false, "show configured provider credential status")
	cmd.Flags().StringVar(&configPath, "config", "", "llmadapter JSON config path for --status")
	return cmd
}

func newRoutesCommand() *cobra.Command {
	var configPath string
	var sourceAPI string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "List configured or auto-detected routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCLIConfig(configPath, adapt.ApiKind(sourceAPI))
			if err != nil {
				return err
			}
			routes := routeInfos(cfg, adapt.ApiKind(sourceAPI))
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"routes": routes})
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SOURCE_API\tMODEL\tPROVIDER\tPROVIDER_API\tNATIVE_MODEL\tWEIGHT")
			for _, route := range routes {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", route.SourceAPI, route.Model, route.Provider, route.ProviderAPI, route.NativeModel, route.Weight)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "llmadapter JSON config path; defaults to auto-detected env/local credentials")
	cmd.Flags().StringVar(&sourceAPI, "source-api", "", "filter by source API")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print routes as JSON")
	return cmd
}

func newModelsCommand() *cobra.Command {
	var configPath string
	var sourceAPI string
	var query string
	var jsonOut bool
	var catalogMode bool
	var catalogFlags catalogModelFlags
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List route models or modeldb catalog models",
		RunE: func(cmd *cobra.Command, args []string) error {
			if catalogMode {
				catalog, err := loadCLICatalog(configPath)
				if err != nil {
					return err
				}
				catalogFlags.Query = query
				models := catalogModelInfos(catalog, catalogFlags)
				if jsonOut {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"models": models})
				}
				return printCatalogModelInfos(cmd.OutOrStdout(), models, catalogFlags.Offerings || catalogFlags.Service != "")
			}
			cfg, err := loadCLIConfig(configPath, adapt.ApiKind(sourceAPI))
			if err != nil {
				return err
			}
			models := modelInfos(cfg, adapt.ApiKind(sourceAPI), query)
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"models": models})
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MODEL\tNATIVE_MODEL\tSOURCE_API\tPROVIDER\tPROVIDER_API")
			for _, model := range models {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", model.Model, model.NativeModel, model.SourceAPI, model.Provider, model.ProviderAPI)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "llmadapter JSON config path; defaults to auto-detected env/local credentials")
	cmd.Flags().StringVar(&sourceAPI, "source-api", "", "filter by source API")
	cmd.Flags().StringVarP(&query, "query", "q", "", "filter models by substring")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print models as JSON")
	cmd.Flags().BoolVar(&catalogMode, "catalog", false, "list modeldb catalog models instead of configured routes")
	cmd.Flags().StringVar(&catalogFlags.ID, "id", "", "filter catalog models by exact canonical model ID")
	cmd.Flags().StringVar(&catalogFlags.Name, "name", "", "filter catalog models by logical model name or alias")
	cmd.Flags().StringVar(&catalogFlags.Creator, "creator", "", "filter catalog models by creator")
	cmd.Flags().StringVar(&catalogFlags.Service, "service", "", "filter catalog offerings by service ID")
	cmd.Flags().StringVar(&catalogFlags.APIType, "api-type", "", "filter catalog offerings by modeldb API type")
	cmd.Flags().StringVar(&catalogFlags.Parameter, "parameter", "", "filter catalog offerings by normalized parameter")
	cmd.Flags().StringVar(&catalogFlags.Family, "family", "", "filter catalog models by family")
	cmd.Flags().StringVar(&catalogFlags.Series, "series", "", "filter catalog models by series")
	cmd.Flags().StringVar(&catalogFlags.Version, "version", "", "filter catalog models by version")
	cmd.Flags().StringVar(&catalogFlags.ReleaseDate, "release-date", "", "filter catalog models by release date")
	cmd.Flags().BoolVar(&catalogFlags.Offerings, "offerings", false, "expand catalog models to per-service offerings")
	return cmd
}

func newResolveCommand() *cobra.Command {
	var configPath string
	var sourceAPI string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "resolve <model>",
		Short: "Explain how a model routes to a provider endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCLIConfig(configPath, adapt.ApiKind(sourceAPI))
			if err != nil {
				return err
			}
			resolution, err := resolveModel(cfg, args[0], adapt.ApiKind(sourceAPI))
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), resolution)
			}
			printResolution(cmd.OutOrStdout(), resolution)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "llmadapter JSON config path; defaults to auto-detected env/local credentials")
	cmd.Flags().StringVar(&sourceAPI, "source-api", string(adapt.ApiOpenAIResponses), "source API to resolve")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print resolution as JSON")
	return cmd
}

func newServeCommand() *cobra.Command {
	var configPath string
	var addr string
	var inspectConfig bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the OpenAI/Anthropic-compatible gateway server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadServeConfig(configPath, addr)
			if err != nil {
				return err
			}
			if inspectConfig {
				return writeJSON(cmd.OutOrStdout(), inspectServeConfig(cfg))
			}
			return gatewayserver.ListenAndServe(cfg)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "llmadapter JSON config path; defaults to LLMADAPTER_CONFIG or env defaults")
	cmd.Flags().StringVar(&addr, "addr", "", "listen address override, for example :8080")
	cmd.Flags().BoolVar(&inspectConfig, "inspect-config", false, "print resolved gateway config metadata as JSON and exit")
	return cmd
}

func newSmokeCommand() *cobra.Command {
	var mode string
	var configPath string
	var sourceAPI string
	var providerType string
	var model string
	var nativeModel string
	var apiKey string
	var apiKeyEnv string
	var baseURL string
	var prompt string
	var timeout time.Duration
	var maxTokens int
	cmd := &cobra.Command{
		Use:   "smoke",
		Short: "Run a minimal provider or mux text smoke request",
		RunE: func(cmd *cobra.Command, args []string) error {
			requestModel := model
			if requestModel == "" && configPath != "" {
				requestModel = defaultModelFromConfig(configPath, adapt.ApiKind(sourceAPI))
			}
			if mode == "mux" && configPath != "" {
				if requestModel == "" {
					return fmt.Errorf("model is required when config has no route model")
				}
				cfg, err := adapterconfig.Load(configPath)
				if err != nil {
					return err
				}
				client, err := adapterconfig.NewMuxClient(cfg, adapterconfig.WithSourceAPI(adapt.ApiKind(sourceAPI)))
				if err != nil {
					return err
				}
				return runSmokeRequest(cmd.Context(), cmd.OutOrStdout(), client, requestModel, prompt, timeout, maxTokens)
			}
			if mode == "auto" {
				result, err := adapterconfig.AutoMuxClient(adapterconfig.AutoOptions{
					EnableEnv:         true,
					EnableLocalClaude: true,
					UseModelDB:        true,
					SourceAPI:         adapt.ApiKind(sourceAPI),
				})
				if err != nil {
					return err
				}
				if requestModel == "" {
					requestModel = defaultModelFromRoutes(result.Config.Routes, adapt.ApiKind(sourceAPI))
				}
				if requestModel == "" {
					return fmt.Errorf("model is required when auto detection produced no route for %s", sourceAPI)
				}
				return runSmokeRequest(cmd.Context(), cmd.OutOrStdout(), result.Client, requestModel, prompt, timeout, maxTokens)
			}
			descriptor, ok := providerregistry.Lookup(providerType)
			if !ok {
				return fmt.Errorf("unknown provider type %q", providerType)
			}
			key := apiKey
			if key == "" && apiKeyEnv != "" {
				key = os.Getenv(apiKeyEnv)
			}
			if key == "" {
				key = firstEnv(descriptor.DefaultAPIKeyEnvs...)
			}
			if requestModel == "" && descriptor.DefaultModelEnv != "" {
				requestModel = os.Getenv(descriptor.DefaultModelEnv)
			}
			if requestModel == "" {
				requestModel = descriptor.DefaultModel
			}
			providerClient, err := providerregistry.NewClient(providerregistry.ClientConfig{
				Type:    descriptor.Type,
				APIKey:  key,
				BaseURL: baseURL,
			})
			if err != nil {
				return err
			}
			client := providerClient
			if mode == "mux" {
				routeModel := requestModel
				if nativeModel == "" {
					nativeModel = requestModel
				}
				client = muxclient.New(router.NewStaticRouter(router.StaticRoute{
					SourceAPI:   adapt.ApiKind(sourceAPI),
					Model:       routeModel,
					NativeModel: nativeModel,
					Endpoint: router.ProviderEndpoint{
						ProviderName: descriptor.Type,
						APIKind:      descriptor.APIKind,
						Family:       descriptor.Family,
						Client:       providerClient,
						Capabilities: descriptor.Capabilities,
					},
				}), muxclient.WithSourceAPI(adapt.ApiKind(sourceAPI)))
			} else if mode != "direct" {
				return fmt.Errorf("unknown smoke mode %q", mode)
			}
			return runSmokeRequest(cmd.Context(), cmd.OutOrStdout(), client, requestModel, prompt, timeout, maxTokens)
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "direct", "smoke mode: direct, mux, or auto")
	cmd.Flags().StringVar(&configPath, "config", "", "llmadapter JSON config path for mux mode")
	cmd.Flags().StringVar(&sourceAPI, "source-api", string(adapt.ApiOpenAIResponses), "source API for mux mode")
	cmd.Flags().StringVar(&providerType, "type", "openai_responses", "provider endpoint type")
	cmd.Flags().StringVar(&model, "model", "", "model to request")
	cmd.Flags().StringVar(&nativeModel, "native-model", "", "native model for mux mode; defaults to model")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key; prefer env vars in normal use")
	cmd.Flags().StringVar(&apiKeyEnv, "api-key-env", "", "environment variable containing the API key")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "provider base URL override")
	cmd.Flags().StringVar(&prompt, "prompt", "Reply with exactly: llmadapter cli smoke ok", "prompt text")
	cmd.Flags().DurationVar(&timeout, "timeout", 45*time.Second, "request timeout")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 64, "maximum output tokens")
	return cmd
}

type routeInfo struct {
	SourceAPI   adapt.ApiKind `json:"source_api"`
	Model       string        `json:"model,omitempty"`
	Provider    string        `json:"provider"`
	ProviderAPI adapt.ApiKind `json:"provider_api,omitempty"`
	NativeModel string        `json:"native_model,omitempty"`
	Weight      int           `json:"weight,omitempty"`
}

type modelInfo struct {
	Model       string        `json:"model"`
	NativeModel string        `json:"native_model,omitempty"`
	SourceAPI   adapt.ApiKind `json:"source_api"`
	Provider    string        `json:"provider"`
	ProviderAPI adapt.ApiKind `json:"provider_api,omitempty"`
}

type providerStatusInfo struct {
	Name             string          `json:"name"`
	Type             string          `json:"type"`
	APIKind          adapt.ApiKind   `json:"api_kind,omitempty"`
	Family           adapt.ApiFamily `json:"family,omitempty"`
	Model            string          `json:"model,omitempty"`
	Status           string          `json:"status"`
	Reason           string          `json:"reason,omitempty"`
	CredentialSource string          `json:"credential_source,omitempty"`
}

type resolutionInfo struct {
	Input          string               `json:"input"`
	MatchedAs      string               `json:"matched_as"`
	SourceAPI      adapt.ApiKind        `json:"source_api"`
	PublicModel    string               `json:"public_model,omitempty"`
	NativeModel    string               `json:"native_model,omitempty"`
	Provider       string               `json:"provider"`
	ProviderType   string               `json:"provider_type"`
	ProviderAPI    adapt.ApiKind        `json:"provider_api"`
	Family         adapt.ApiFamily      `json:"family"`
	Weight         int                  `json:"weight,omitempty"`
	Priority       int                  `json:"priority,omitempty"`
	ModelDBService string               `json:"modeldb_service_id,omitempty"`
	Capabilities   router.CapabilitySet `json:"capabilities"`
}

type serveConfigInspection struct {
	Addr           string                    `json:"addr"`
	HealthCooldown string                    `json:"health_cooldown,omitempty"`
	Providers      []serveProviderInspection `json:"providers,omitempty"`
	Routes         []serveRouteInspection    `json:"routes,omitempty"`
}

type serveProviderInspection struct {
	Name             string               `json:"name"`
	Type             string               `json:"type"`
	APIKind          adapt.ApiKind        `json:"api_kind"`
	Family           adapt.ApiFamily      `json:"family"`
	Model            string               `json:"model,omitempty"`
	ModelDBServiceID string               `json:"modeldb_service_id,omitempty"`
	APIKeyEnv        string               `json:"api_key_env,omitempty"`
	InlineAPIKey     bool                 `json:"inline_api_key,omitempty"`
	BaseURL          string               `json:"base_url,omitempty"`
	Priority         int                  `json:"priority,omitempty"`
	Capabilities     router.CapabilitySet `json:"capabilities"`
}

type serveRouteInspection struct {
	SourceAPI          adapt.ApiKind `json:"source_api"`
	Model              string        `json:"model,omitempty"`
	Provider           string        `json:"provider"`
	ProviderAPI        adapt.ApiKind `json:"provider_api,omitempty"`
	NativeModel        string        `json:"native_model,omitempty"`
	ModelDBModel       string        `json:"modeldb_model,omitempty"`
	ModelDBWireModelID string        `json:"modeldb_wire_model_id,omitempty"`
	Weight             int           `json:"weight,omitempty"`
}

type catalogModelFlags struct {
	ID          string
	Query       string
	Name        string
	Creator     string
	Service     string
	APIType     string
	Parameter   string
	Family      string
	Series      string
	Version     string
	ReleaseDate string
	Offerings   bool
}

type catalogModelInfo struct {
	ID        string                `json:"id"`
	Name      string                `json:"name,omitempty"`
	Model     modeldb.ModelRecord   `json:"model,omitempty"`
	Services  []string              `json:"services,omitempty"`
	Offerings []catalogOfferingInfo `json:"offerings,omitempty"`
}

type catalogOfferingInfo struct {
	ServiceID   string            `json:"service_id"`
	ServiceName string            `json:"service_name,omitempty"`
	WireModelID string            `json:"wire_model_id"`
	APITypes    []modeldb.APIType `json:"api_types,omitempty"`
}

func runProvidersAuto(w io.Writer, jsonOut bool) error {
	result, err := adapterconfig.AutoMuxClient(adapterconfig.AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: true,
		UseModelDB:        true,
	})
	if err != nil && len(result.Enabled) == 0 && len(result.Skipped) == 0 {
		return err
	}
	statuses := make([]providerStatusInfo, 0, len(result.Enabled)+len(result.Skipped))
	for _, provider := range result.Enabled {
		statuses = append(statuses, providerStatusInfo{
			Name:             provider.Name,
			Type:             provider.Type,
			APIKind:          provider.API,
			Model:            provider.Model,
			Status:           "enabled",
			Reason:           provider.Reason,
			CredentialSource: provider.Reason,
		})
	}
	for _, provider := range result.Skipped {
		statuses = append(statuses, providerStatusInfo{
			Name:    provider.Name,
			Type:    provider.Type,
			APIKind: provider.API,
			Model:   provider.Model,
			Status:  "skipped",
			Reason:  provider.Reason,
		})
	}
	if jsonOut {
		return writeJSON(w, map[string]any{"providers": statuses})
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tAPI_KIND\tSTATUS\tSOURCE\tMODEL")
	for _, provider := range statuses {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", provider.Type, provider.APIKind, provider.Status, provider.Reason, provider.Model)
	}
	return tw.Flush()
}

func runProvidersStatus(w io.Writer, configPath string, jsonOut bool) error {
	var (
		cfg adapterconfig.Config
		err error
	)
	if configPath != "" {
		cfg, err = adapterconfig.Load(configPath)
	} else {
		cfg, err = adapterconfig.LoadFromEnv()
	}
	if err != nil {
		return err
	}
	statuses := configuredProviderStatuses(cfg)
	if jsonOut {
		return writeJSON(w, map[string]any{"providers": statuses})
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tAPI_KIND\tSTATUS\tSOURCE\tMODEL")
	for _, provider := range statuses {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", provider.Name, provider.Type, provider.APIKind, provider.Status, provider.CredentialSource, provider.Model)
	}
	return tw.Flush()
}

func configuredProviderStatuses(cfg adapterconfig.Config) []providerStatusInfo {
	out := make([]providerStatusInfo, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		endpoint, err := adapterconfig.ProviderEndpointConfig(provider)
		info := providerStatusInfo{
			Name:             provider.Name,
			Type:             provider.Type,
			Model:            provider.Model,
			Status:           "configured",
			CredentialSource: credentialSource(provider),
		}
		if err != nil {
			info.Status = "invalid"
			info.Reason = err.Error()
		} else {
			info.APIKind = endpoint.APIKind
			info.Family = endpoint.Family
		}
		out = append(out, info)
	}
	return out
}

func credentialSource(provider adapterconfig.ProviderConfig) string {
	if provider.APIKey != "" {
		return "inline_api_key"
	}
	if provider.APIKeyEnv != "" {
		if os.Getenv(provider.APIKeyEnv) != "" {
			return "env:" + provider.APIKeyEnv + ":set"
		}
		return "env:" + provider.APIKeyEnv + ":missing"
	}
	descriptor, ok := providerregistry.Lookup(provider.Type)
	if !ok {
		return "unknown"
	}
	for _, key := range descriptor.DefaultAPIKeyEnvs {
		if os.Getenv(key) != "" {
			return "env:" + key + ":set"
		}
	}
	if provider.Type == "claude_messages" {
		return "local_claude_oauth_or_default_env"
	}
	if len(descriptor.DefaultAPIKeyEnvs) != 0 {
		return "default_env:missing"
	}
	return "none"
}

func loadCLIConfig(path string, sourceAPI adapt.ApiKind) (adapterconfig.Config, error) {
	if path != "" {
		return adapterconfig.Load(path)
	}
	result, err := adapterconfig.AutoMuxClient(adapterconfig.AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: true,
		UseModelDB:        true,
		SourceAPI:         sourceAPI,
	})
	return result.Config, err
}

func loadServeConfig(path string, addr string) (adapterconfig.Config, error) {
	var (
		cfg adapterconfig.Config
		err error
	)
	if path != "" {
		cfg, err = adapterconfig.Load(path)
	} else {
		cfg, err = adapterconfig.LoadFromEnv()
	}
	if err != nil {
		return adapterconfig.Config{}, err
	}
	if addr != "" {
		cfg.Addr = addr
	}
	return cfg, nil
}

func inspectServeConfig(cfg adapterconfig.Config) serveConfigInspection {
	inspection := serveConfigInspection{
		Addr:           cfg.Addr,
		HealthCooldown: cfg.HealthCooldown,
		Providers:      make([]serveProviderInspection, 0, len(cfg.Providers)),
		Routes:         make([]serveRouteInspection, 0, len(cfg.Routes)),
	}
	for _, provider := range cfg.Providers {
		endpoint, err := adapterconfig.ProviderEndpointConfig(provider)
		if err != nil {
			inspection.Providers = append(inspection.Providers, serveProviderInspection{
				Name:         provider.Name,
				Type:         provider.Type,
				Model:        provider.Model,
				APIKeyEnv:    provider.APIKeyEnv,
				InlineAPIKey: provider.APIKey != "",
				BaseURL:      provider.BaseURL,
				Priority:     provider.Priority,
			})
			continue
		}
		inspection.Providers = append(inspection.Providers, serveProviderInspection{
			Name:             provider.Name,
			Type:             provider.Type,
			APIKind:          endpoint.APIKind,
			Family:           endpoint.Family,
			Model:            provider.Model,
			ModelDBServiceID: endpoint.Tags[adapterconfig.TagModelDBServiceID],
			APIKeyEnv:        provider.APIKeyEnv,
			InlineAPIKey:     provider.APIKey != "",
			BaseURL:          provider.BaseURL,
			Priority:         provider.Priority,
			Capabilities:     endpoint.Capabilities,
		})
	}
	for _, route := range cfg.Routes {
		inspection.Routes = append(inspection.Routes, serveRouteInspection{
			SourceAPI:          route.SourceAPI,
			Model:              route.Model,
			Provider:           route.Provider,
			ProviderAPI:        route.ProviderAPI,
			NativeModel:        route.NativeModel,
			ModelDBModel:       route.ModelDBModel,
			ModelDBWireModelID: route.ModelDBWireModelID,
			Weight:             route.Weight,
		})
	}
	return inspection
}

func routeInfos(cfg adapterconfig.Config, sourceAPI adapt.ApiKind) []routeInfo {
	out := make([]routeInfo, 0, len(cfg.Routes))
	for _, route := range cfg.Routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		out = append(out, routeInfo{
			SourceAPI:   route.SourceAPI,
			Model:       route.Model,
			Provider:    route.Provider,
			ProviderAPI: route.ProviderAPI,
			NativeModel: route.NativeModel,
			Weight:      route.Weight,
		})
	}
	return out
}

func modelInfos(cfg adapterconfig.Config, sourceAPI adapt.ApiKind, query string) []modelInfo {
	seen := map[modelInfo]bool{}
	var out []modelInfo
	query = strings.ToLower(query)
	for _, route := range cfg.Routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		model := route.Model
		if model == "" {
			model = route.NativeModel
		}
		if query != "" && !strings.Contains(strings.ToLower(model+" "+route.NativeModel), query) {
			continue
		}
		info := modelInfo{
			Model:       model,
			NativeModel: route.NativeModel,
			SourceAPI:   route.SourceAPI,
			Provider:    route.Provider,
			ProviderAPI: route.ProviderAPI,
		}
		if seen[info] {
			continue
		}
		seen[info] = true
		out = append(out, info)
	}
	return out
}

func loadCLICatalog(configPath string) (modeldb.Catalog, error) {
	if configPath == "" {
		return adapterconfig.LoadModelDBCatalog(adapterconfig.ModelDBConfig{})
	}
	cfg, err := adapterconfig.Load(configPath)
	if err != nil {
		return modeldb.Catalog{}, err
	}
	return adapterconfig.LoadModelDBCatalog(cfg.ModelDB)
}

func catalogModelInfos(catalog modeldb.Catalog, flags catalogModelFlags) []catalogModelInfo {
	matches := catalog.FindModels(modeldb.ModelSelector{
		ID:          flags.ID,
		Name:        flags.Name,
		Creator:     flags.Creator,
		ServiceID:   flags.Service,
		APIType:     modeldb.APIType(flags.APIType),
		Parameter:   modeldb.NormalizedParameter(flags.Parameter),
		Family:      flags.Family,
		Series:      flags.Series,
		Version:     flags.Version,
		ReleaseDate: flags.ReleaseDate,
	})
	matches = filterCatalogMatchesByQuery(matches, flags.Query)
	matches = filterCatalogMatchesWithOfferings(matches)

	out := make([]catalogModelInfo, 0, len(matches))
	for _, match := range matches {
		info := catalogModelInfo{
			ID:        catalogModelID(match.Model),
			Name:      match.Model.Name,
			Model:     match.Model,
			Services:  catalogServiceIDs(match.Offerings),
			Offerings: make([]catalogOfferingInfo, 0, len(match.Offerings)),
		}
		for _, offering := range match.Offerings {
			info.Offerings = append(info.Offerings, catalogOfferingInfo{
				ServiceID:   offering.Service.ID,
				ServiceName: offering.Service.Name,
				WireModelID: offering.Offering.WireModelID,
				APITypes:    catalogAPITypes(offering.Offering.Exposures),
			})
		}
		out = append(out, info)
	}
	return out
}

func printCatalogModelInfos(w io.Writer, models []catalogModelInfo, includeOfferings bool) error {
	if len(models) == 0 {
		_, err := fmt.Fprintln(w, "No models found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if includeOfferings {
		fmt.Fprintln(tw, "MODEL\tSERVICE\tAPI_TYPES\tWIRE_MODEL")
		for _, model := range models {
			for _, offering := range model.Offerings {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", model.ID, offering.ServiceID, joinAPITypes(offering.APITypes), offering.WireModelID)
			}
		}
		return tw.Flush()
	}
	fmt.Fprintln(tw, "MODEL\tSERVICES")
	for _, model := range models {
		fmt.Fprintf(tw, "%s\t%s\n", model.ID, strings.Join(model.Services, ","))
	}
	return tw.Flush()
}

func filterCatalogMatchesByQuery(matches []modeldb.ModelMatch, query string) []modeldb.ModelMatch {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return matches
	}
	filtered := make([]modeldb.ModelMatch, 0, len(matches))
	for _, match := range matches {
		if catalogMatchQuery(match, query) {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

func filterCatalogMatchesWithOfferings(matches []modeldb.ModelMatch) []modeldb.ModelMatch {
	filtered := make([]modeldb.ModelMatch, 0, len(matches))
	for _, match := range matches {
		if len(match.Offerings) != 0 {
			filtered = append(filtered, match)
		}
	}
	return filtered
}

func catalogMatchQuery(match modeldb.ModelMatch, query string) bool {
	key := modeldb.NormalizeKey(match.Model.Key)
	values := []string{
		catalogModelID(match.Model),
		match.Model.Name,
		key.Creator,
		key.Family,
		key.Series,
		key.Version,
		key.ReleaseDate,
	}
	values = append(values, match.Model.Aliases...)
	for _, offering := range match.Offerings {
		values = append(values, offering.Service.ID, offering.Service.Name, offering.Offering.WireModelID)
		values = append(values, offering.Offering.Aliases...)
		for _, exposure := range offering.Offering.Exposures {
			values = append(values, string(exposure.APIType))
			for _, parameter := range exposure.SupportedParameters {
				values = append(values, string(parameter))
			}
			for _, mapping := range exposure.ParameterMappings {
				values = append(values, string(mapping.Normalized), mapping.WireName)
			}
		}
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func catalogModelID(model modeldb.ModelRecord) string {
	if releaseID := modeldb.ReleaseID(model.Key); releaseID != "" {
		return releaseID
	}
	return modeldb.LineID(model.Key)
}

func catalogServiceIDs(offerings []modeldb.ServiceOffering) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(offerings))
	for _, offering := range offerings {
		if offering.Service.ID == "" || seen[offering.Service.ID] {
			continue
		}
		seen[offering.Service.ID] = true
		out = append(out, offering.Service.ID)
	}
	sort.Strings(out)
	return out
}

func catalogAPITypes(exposures []modeldb.OfferingExposure) []modeldb.APIType {
	seen := map[modeldb.APIType]bool{}
	out := make([]modeldb.APIType, 0, len(exposures))
	for _, exposure := range exposures {
		if exposure.APIType == "" || seen[exposure.APIType] {
			continue
		}
		seen[exposure.APIType] = true
		out = append(out, exposure.APIType)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

func joinAPITypes(types []modeldb.APIType) string {
	out := make([]string, 0, len(types))
	for _, apiType := range types {
		out = append(out, string(apiType))
	}
	return strings.Join(out, ",")
}

func resolveModel(cfg adapterconfig.Config, model string, sourceAPI adapt.ApiKind) (resolutionInfo, error) {
	for _, route := range cfg.Routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		matchedAs := ""
		switch {
		case route.Model == model:
			matchedAs = "public_model"
		case route.NativeModel == model:
			matchedAs = "native_model"
		default:
			continue
		}
		provider, endpoint, err := routeEndpoint(cfg, route)
		if err != nil {
			return resolutionInfo{}, err
		}
		nativeModel := route.NativeModel
		if nativeModel == "" {
			nativeModel = provider.Model
		}
		providerAPI := route.ProviderAPI
		if providerAPI == "" {
			providerAPI = endpoint.APIKind
		}
		return resolutionInfo{
			Input:          model,
			MatchedAs:      matchedAs,
			SourceAPI:      route.SourceAPI,
			PublicModel:    route.Model,
			NativeModel:    nativeModel,
			Provider:       route.Provider,
			ProviderType:   provider.Type,
			ProviderAPI:    providerAPI,
			Family:         endpoint.Family,
			Weight:         route.Weight,
			Priority:       provider.Priority,
			ModelDBService: endpoint.Tags[adapterconfig.TagModelDBServiceID],
			Capabilities:   endpoint.Capabilities,
		}, nil
	}
	if sourceAPI != "" {
		return resolutionInfo{}, fmt.Errorf("no route found for model %q and source_api %s", model, sourceAPI)
	}
	return resolutionInfo{}, fmt.Errorf("no route found for model %q", model)
}

func routeEndpoint(cfg adapterconfig.Config, route adapterconfig.RouteConfig) (adapterconfig.ProviderConfig, router.ProviderEndpoint, error) {
	var provider adapterconfig.ProviderConfig
	var endpoint router.ProviderEndpoint
	matches := 0
	for _, candidate := range cfg.Providers {
		if candidate.Name != route.Provider {
			continue
		}
		candidateEndpoint, err := adapterconfig.ProviderEndpointConfig(candidate)
		if err != nil {
			return adapterconfig.ProviderConfig{}, router.ProviderEndpoint{}, err
		}
		if route.ProviderAPI != "" && candidateEndpoint.APIKind != route.ProviderAPI {
			continue
		}
		provider = candidate
		endpoint = candidateEndpoint
		matches++
	}
	switch matches {
	case 0:
		return adapterconfig.ProviderConfig{}, router.ProviderEndpoint{}, fmt.Errorf("route references unknown provider endpoint %q %q", route.Provider, route.ProviderAPI)
	case 1:
		return provider, endpoint, nil
	default:
		return adapterconfig.ProviderConfig{}, router.ProviderEndpoint{}, fmt.Errorf("route references provider %q without provider_api but multiple endpoints match", route.Provider)
	}
}

func printResolution(w io.Writer, resolution resolutionInfo) {
	fmt.Fprintf(w, "Input:        %s\n", resolution.Input)
	fmt.Fprintf(w, "Matched as:   %s\n", resolution.MatchedAs)
	fmt.Fprintf(w, "Source API:   %s\n", resolution.SourceAPI)
	fmt.Fprintf(w, "Public model: %s\n", resolution.PublicModel)
	fmt.Fprintf(w, "Native model: %s\n", resolution.NativeModel)
	fmt.Fprintf(w, "Provider:     %s\n", resolution.Provider)
	fmt.Fprintf(w, "Provider type: %s\n", resolution.ProviderType)
	fmt.Fprintf(w, "Provider API: %s\n", resolution.ProviderAPI)
	fmt.Fprintf(w, "Family:       %s\n", resolution.Family)
	fmt.Fprintf(w, "Weight:       %d\n", resolution.Weight)
	fmt.Fprintf(w, "Priority:     %d\n", resolution.Priority)
	if resolution.ModelDBService != "" {
		fmt.Fprintf(w, "ModelDB svc:  %s\n", resolution.ModelDBService)
	}
	fmt.Fprintf(w, "Capabilities: streaming=%t tools=%t vision=%t audio_input=%t json_mode=%t json_schema=%t reasoning=%t\n",
		resolution.Capabilities.Streaming,
		resolution.Capabilities.Tools,
		resolution.Capabilities.Vision,
		resolution.Capabilities.AudioInput,
		resolution.Capabilities.JSONMode,
		resolution.Capabilities.JSONSchema,
		resolution.Capabilities.Reasoning,
	)
}

func runSmokeRequest(ctx context.Context, w io.Writer, client unified.Client, model, prompt string, timeout time.Duration, maxTokens int) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	events, err := client.Request(ctx, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: prompt}},
		}},
		Stream: true,
	})
	if err != nil {
		return err
	}
	resp, err := unified.Collect(ctx, events)
	if err != nil {
		return err
	}
	text := responseText(resp)
	if text == "" {
		return fmt.Errorf("empty response content")
	}
	fmt.Fprintln(w, text)
	return nil
}

func defaultModelFromConfig(path string, sourceAPI adapt.ApiKind) string {
	cfg, err := adapterconfig.Load(path)
	if err != nil {
		return ""
	}
	return defaultModelFromRoutes(cfg.Routes, sourceAPI)
}

func defaultModelFromRoutes(routes []adapterconfig.RouteConfig, sourceAPI adapt.ApiKind) string {
	for _, route := range routes {
		if route.SourceAPI == sourceAPI && route.Model != "" {
			return route.Model
		}
	}
	for _, route := range routes {
		if route.Model != "" {
			return route.Model
		}
	}
	return ""
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func responseText(resp unified.Response) string {
	var b strings.Builder
	for _, part := range resp.Content {
		text, ok := part.(unified.TextPart)
		if ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
