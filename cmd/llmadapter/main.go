package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/adapterconfig"
	"github.com/codewandler/llmadapter/muxclient"
	"github.com/codewandler/llmadapter/providerregistry"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
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
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "List registered provider endpoint types",
		RunE: func(cmd *cobra.Command, args []string) error {
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
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List public/native models from configured or auto-detected routes",
		RunE: func(cmd *cobra.Command, args []string) error {
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
