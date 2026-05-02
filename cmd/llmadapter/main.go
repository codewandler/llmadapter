package main

import (
	"bytes"
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
	"github.com/codewandler/llmadapter/compatibility"
	"github.com/codewandler/llmadapter/conformance"
	"github.com/codewandler/llmadapter/gatewayserver"
	"github.com/codewandler/llmadapter/muxclient"
	"github.com/codewandler/llmadapter/providerregistry"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
	"github.com/spf13/cobra"
)

const (
	ansiDim   = "\033[2m"
	ansiReset = "\033[0m"
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
	cmd.AddCommand(newCompatibilityCommand())
	cmd.AddCommand(newCompatibilityRecordCommand())
	cmd.AddCommand(newConformanceCommand())
	cmd.AddCommand(newServeCommand())
	cmd.AddCommand(newSmokeCommand())
	cmd.AddCommand(newInferCommand())
	cmd.AddCommand(newProxyCommand())
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
			fmt.Fprintln(w, "SOURCE_API\tMODEL\tPROVIDER\tPROVIDER_API\tDYNAMIC\tNATIVE_MODEL\tWEIGHT\tCONSUMER_CONTINUATION\tINTERNAL_CONTINUATION\tTRANSPORT")
			for _, route := range routes {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\t%s\t%d\t%s\t%s\t%s\n", route.SourceAPI, route.Model, route.Provider, route.ProviderAPI, route.Dynamic, route.NativeModel, route.Weight, route.ConsumerContinuation, route.InternalContinuation, route.Transport)
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
	var useCase string
	var approvedOnly bool
	var evidencePath string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "resolve <model>",
		Short: "Explain how a model routes to a provider endpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadResolveConfig(configPath, adapt.ApiKind(sourceAPI), args[0])
			if err != nil {
				return err
			}
			if useCase != "" {
				if approvedOnly {
					parsedUseCase := compatibility.UseCase(useCase)
					if evidencePath == "" {
						evidencePath = adapterconfig.DefaultCompatibilityEvidencePath(parsedUseCase)
					}
					evidence, err := adapterconfig.LoadCompatibilityEvidence(evidencePath)
					if err != nil {
						return err
					}
					selections, err := adapterconfig.SelectModelsForUseCase(cfg, args[0], adapt.ApiKind(sourceAPI), adapterconfig.UseCaseSelectionOptions{
						UseCase:  parsedUseCase,
						Evidence: evidence,
					})
					if err != nil {
						return err
					}
					if jsonOut {
						return writeJSON(cmd.OutOrStdout(), map[string]any{
							"input":      args[0],
							"use_case":   useCase,
							"evidence":   evidencePath,
							"selections": selections,
						})
					}
					printUseCaseSelections(cmd.OutOrStdout(), selections)
					return nil
				}
				evaluations, err := resolveCompatibilityEvaluations(cfg, args[0], adapt.ApiKind(sourceAPI), compatibility.UseCase(useCase))
				if err != nil {
					return err
				}
				if jsonOut {
					return writeJSON(cmd.OutOrStdout(), map[string]any{
						"input":       args[0],
						"use_case":    useCase,
						"evaluations": evaluations,
					})
				}
				printCompatibilityEvaluations(cmd.OutOrStdout(), evaluations)
				return nil
			}
			if sourceAPI == "" {
				resolutions, err := resolveModelCandidates(cfg, args[0])
				if err != nil {
					return err
				}
				if jsonOut {
					return writeJSON(cmd.OutOrStdout(), map[string]any{
						"input":      args[0],
						"candidates": resolutions,
					})
				}
				printResolutionCandidates(cmd.OutOrStdout(), resolutions)
				return nil
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
	cmd.Flags().StringVar(&sourceAPI, "source-api", "", "source API to resolve; omit for candidates across all source APIs")
	cmd.Flags().StringVar(&useCase, "use-case", "", "annotate candidates for use case: agentic_coding or summarization")
	cmd.Flags().BoolVar(&approvedOnly, "approved-only", false, "with --use-case, return only candidates approved by compatibility evidence")
	cmd.Flags().StringVar(&evidencePath, "compatibility-evidence", "", "compatibility evidence JSON path for --approved-only")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print resolution as JSON")
	return cmd
}

func newCompatibilityCommand() *cobra.Command {
	var configPath string
	var sourceAPI string
	var model string
	var useCase string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "compatibility",
		Short: "Evaluate route candidates for a workload use case",
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedUseCase, err := compatibility.ParseUseCase(useCase)
			if err != nil {
				return err
			}
			cfg, err := loadCompatibilityConfig(configPath, adapt.ApiKind(sourceAPI), model)
			if err != nil {
				return err
			}
			evaluations, err := compatibilityEvaluations(cfg, model, adapt.ApiKind(sourceAPI), parsedUseCase)
			if err != nil {
				return err
			}
			if jsonOut {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"use_case":    parsedUseCase,
					"model":       model,
					"evaluations": evaluations,
				})
			}
			printCompatibilityEvaluations(cmd.OutOrStdout(), evaluations)
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "", "llmadapter JSON config path; defaults to auto-detected env/local credentials")
	cmd.Flags().StringVar(&sourceAPI, "source-api", "", "source API to evaluate")
	cmd.Flags().StringVarP(&model, "model", "m", "", "model alias or provider-native model to evaluate")
	cmd.Flags().StringVar(&useCase, "use-case", string(compatibility.UseCaseAgenticCoding), "use case: agentic_coding or summarization")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print compatibility result as JSON")
	return cmd
}

func newCompatibilityRecordCommand() *cobra.Command {
	var useCase string
	var artifactPath string
	var matrixPath string
	var command string
	cmd := &cobra.Command{
		Use:   "compatibility-record",
		Short: "Refresh generated compatibility documentation from an artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedUseCase, err := compatibility.ParseUseCase(useCase)
			if err != nil {
				return err
			}
			if artifactPath == "" {
				artifactPath = adapterconfig.DefaultCompatibilityEvidencePath(parsedUseCase)
			}
			artifact, err := compatibility.LoadArtifact(artifactPath)
			if err != nil {
				return err
			}
			if artifact.UseCase != parsedUseCase {
				return fmt.Errorf("artifact use case %q does not match requested use case %q", artifact.UseCase, parsedUseCase)
			}
			enrichCompatibilityArtifactMetadata(&artifact)
			if command != "" {
				artifact.Command = command
			}
			if err := compatibility.SaveArtifact(artifactPath, artifact); err != nil {
				return err
			}
			if matrixPath != "" {
				if err := updateCompatibilityMatrix(matrixPath, artifact); err != nil {
					return err
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", artifactPath)
			if matrixPath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "updated %s\n", matrixPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&useCase, "use-case", string(compatibility.UseCaseAgenticCoding), "use case to record")
	cmd.Flags().StringVar(&artifactPath, "artifact", "", "compatibility artifact path; defaults by use case")
	cmd.Flags().StringVar(&matrixPath, "matrix", "docs/USE_CASE_MATRIX.md", "markdown matrix path to refresh; empty disables markdown update")
	cmd.Flags().StringVar(&command, "command", "", "override recorded command in the artifact")
	return cmd
}

func newConformanceCommand() *cobra.Command {
	var artifactPath string
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "conformance",
		Short: "Report provider endpoint descriptors and compatibility evidence",
		RunE: func(cmd *cobra.Command, args []string) error {
			if artifactPath == "" {
				artifactPath = adapterconfig.DefaultCompatibilityEvidencePath(compatibility.UseCaseAgenticCoding)
			}
			report, err := conformance.Build(conformance.Options{CompatibilityArtifactPath: artifactPath})
			if err != nil {
				return err
			}
			if jsonOut {
				if err := writeJSON(cmd.OutOrStdout(), report); err != nil {
					return err
				}
				return conformanceFailure(report)
			}
			printConformanceReport(cmd.OutOrStdout(), report)
			return conformanceFailure(report)
		},
	}
	cmd.Flags().StringVar(&artifactPath, "compatibility-artifact", "", "compatibility artifact path; defaults to docs/compatibility/agentic_coding.json")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print report as JSON")
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
				inspection, err := adapterconfig.InspectConfig(cfg)
				if err != nil {
					return err
				}
				return writeJSON(cmd.OutOrStdout(), inspection)
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
					EnableLocalCodex:  true,
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

type inferParams struct {
	configPath  string
	sourceAPI   string
	model       string
	system      string
	maxTokens   int
	temperature float64
	thinking    string
	effort      string
	timeout     time.Duration
	noCache     bool
	interaction string
	session     string
	branch      string
	debugScopes []string
}

func newInferCommand() *cobra.Command {
	params := inferParams{
		model:       "haiku",
		maxTokens:   8000,
		timeout:     2 * time.Minute,
		interaction: string(unified.InteractionOneShot),
	}
	cmd := &cobra.Command{
		Use:   "infer <message>",
		Short: "Send a prompt through the configured or auto-detected mux client",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			args, err := normalizeInferArgs(&params, args)
			if err != nil {
				return err
			}
			if params.session != "" && !cmd.Flags().Changed("interaction") {
				params.interaction = string(unified.InteractionSession)
			}
			return runInferCommand(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), params, args[0])
		},
	}
	cmd.Flags().StringVar(&params.configPath, "config", "", "llmadapter JSON config path; defaults to auto-detected credentials")
	cmd.Flags().StringVar(&params.sourceAPI, "source-api", "", "source API for mux routing; omit to use the best resolved candidate")
	cmd.Flags().StringVarP(&params.model, "model", "m", params.model, "model alias or provider-native model")
	cmd.Flags().StringVarP(&params.system, "system", "s", "", "system prompt")
	cmd.Flags().IntVar(&params.maxTokens, "max-tokens", params.maxTokens, "maximum output tokens")
	cmd.Flags().Float64Var(&params.temperature, "temperature", 0, "sampling temperature 0.0-2.0; 0 uses provider default")
	cmd.Flags().StringVar(&params.thinking, "thinking", "", "thinking mode: auto, on, off")
	cmd.Flags().StringVar(&params.effort, "effort", "", "reasoning effort: low, medium, high, max")
	cmd.Flags().DurationVar(&params.timeout, "timeout", params.timeout, "request timeout")
	cmd.Flags().BoolVar(&params.noCache, "no-cache", false, "disable prompt cache policy for this request")
	cmd.Flags().StringVar(&params.interaction, "interaction", string(unified.InteractionOneShot), "interaction mode: one_shot, session, or auto")
	cmd.Flags().StringVar(&params.session, "session", "", "stable session/cache key for continuation diagnostics")
	cmd.Flags().StringVar(&params.branch, "branch", "", "stable branch key for continuation diagnostics")
	cmd.Flags().StringSliceVar(&params.debugScopes, "debug", nil, "print redacted debug diagnostics to stderr; optional scopes: request,response,stream,events")
	cmd.Flags().Lookup("debug").NoOptDefVal = "all"
	return cmd
}

func normalizeInferArgs(params *inferParams, args []string) ([]string, error) {
	if len(args) > 1 && len(params.debugScopes) == 1 && params.debugScopes[0] == "all" {
		if _, err := parseInferDebugScopes([]string{args[0]}); err == nil {
			params.debugScopes = []string{args[0]}
			args = args[1:]
		}
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("infer requires exactly one message argument, got %d", len(args))
	}
	return args, nil
}

type routeInfo struct {
	SourceAPI            adapt.ApiKind            `json:"source_api"`
	Model                string                   `json:"model,omitempty"`
	Provider             string                   `json:"provider"`
	ProviderAPI          adapt.ApiKind            `json:"provider_api,omitempty"`
	Dynamic              bool                     `json:"dynamic_models,omitempty"`
	NativeModel          string                   `json:"native_model,omitempty"`
	Weight               int                      `json:"weight,omitempty"`
	ConsumerContinuation unified.ContinuationMode `json:"consumer_continuation,omitempty"`
	InternalContinuation unified.ContinuationMode `json:"internal_continuation,omitempty"`
	Transport            unified.TransportKind    `json:"transport,omitempty"`
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
	Input                string                   `json:"input"`
	MatchedAs            string                   `json:"matched_as"`
	SourceAPI            adapt.ApiKind            `json:"source_api"`
	PublicModel          string                   `json:"public_model,omitempty"`
	NativeModel          string                   `json:"native_model,omitempty"`
	Provider             string                   `json:"provider"`
	ProviderType         string                   `json:"provider_type"`
	ProviderAPI          adapt.ApiKind            `json:"provider_api"`
	Family               adapt.ApiFamily          `json:"family"`
	Weight               int                      `json:"weight,omitempty"`
	Priority             int                      `json:"priority,omitempty"`
	ModelDBService       string                   `json:"modeldb_service_id,omitempty"`
	Capabilities         router.CapabilitySet     `json:"capabilities"`
	CapabilitySource     string                   `json:"capability_source,omitempty"`
	ConsumerContinuation unified.ContinuationMode `json:"consumer_continuation,omitempty"`
	InternalContinuation unified.ContinuationMode `json:"internal_continuation,omitempty"`
	Transport            unified.TransportKind    `json:"transport,omitempty"`
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
		EnableLocalCodex:  true,
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
	if provider.Type == "claude" {
		return "local_claude_oauth"
	}
	if provider.Type == "codex_responses" {
		if codex.LocalAvailable() {
			return "local_codex_oauth:set"
		}
		return "local_codex_oauth:missing"
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
		EnableLocalCodex:  true,
		UseModelDB:        true,
		SourceAPI:         sourceAPI,
	})
	return result.Config, err
}

func loadResolveConfig(path string, sourceAPI adapt.ApiKind, model string) (adapterconfig.Config, error) {
	if path != "" {
		return adapterconfig.Load(path)
	}
	result, err := adapterconfig.AutoMuxClient(adapterconfig.AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: true,
		EnableLocalCodex:  true,
		UseModelDB:        true,
		DynamicModels:     true,
		SourceAPI:         sourceAPI,
		Intents:           inferAutoIntents(model, sourceAPI),
	})
	return result.Config, err
}

func loadCompatibilityConfig(path string, sourceAPI adapt.ApiKind, model string) (adapterconfig.Config, error) {
	if model != "" {
		return loadResolveConfig(path, sourceAPI, model)
	}
	return loadCLIConfig(path, sourceAPI)
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

func routeInfos(cfg adapterconfig.Config, sourceAPI adapt.ApiKind) []routeInfo {
	endpoints := make([]router.ProviderEndpoint, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		endpoint, err := adapterconfig.ProviderEndpointConfig(provider)
		if err == nil {
			endpoints = append(endpoints, endpoint)
		}
	}
	out := make([]routeInfo, 0, len(cfg.Routes))
	for _, route := range cfg.Routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		endpoint, ok, _ := adapterconfig.FindProviderEndpoint(endpoints, route.Provider, route.ProviderAPI)
		out = append(out, routeInfo{
			SourceAPI:            route.SourceAPI,
			Model:                route.Model,
			Provider:             route.Provider,
			ProviderAPI:          route.ProviderAPI,
			Dynamic:              route.DynamicModels,
			NativeModel:          route.NativeModel,
			Weight:               route.Weight,
			ConsumerContinuation: routeContinuation(ok, endpoint.ConsumerContinuation),
			InternalContinuation: routeContinuation(ok, endpoint.InternalContinuation),
			Transport:            routeTransport(ok, endpoint.Transport),
		})
	}
	return out
}

func routeContinuation(ok bool, value unified.ContinuationMode) unified.ContinuationMode {
	if !ok {
		return ""
	}
	return value
}

func routeTransport(ok bool, value unified.TransportKind) unified.TransportKind {
	if !ok {
		return ""
	}
	return value
}

func modelInfos(cfg adapterconfig.Config, sourceAPI adapt.ApiKind, query string) []modelInfo {
	seen := map[modelInfo]bool{}
	var out []modelInfo
	query = strings.ToLower(query)
	for _, route := range cfg.Routes {
		if sourceAPI != "" && route.SourceAPI != sourceAPI {
			continue
		}
		if route.DynamicModels {
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

func compatibilityEvaluations(cfg adapterconfig.Config, model string, sourceAPI adapt.ApiKind, useCase compatibility.UseCase) ([]compatibility.Evaluation, error) {
	profile, ok := compatibility.BuiltinProfile(useCase)
	if !ok {
		return nil, fmt.Errorf("unknown use case %q", useCase)
	}
	if model != "" {
		return resolveCompatibilityEvaluations(cfg, model, sourceAPI, useCase)
	}
	models := modelInfos(cfg, sourceAPI, "")
	evaluations := make([]compatibility.Evaluation, 0, len(models))
	seen := map[string]bool{}
	for _, info := range models {
		modelName := info.Model
		if modelName == "" {
			modelName = info.NativeModel
		}
		if modelName == "" {
			continue
		}
		key := string(info.SourceAPI) + "\x00" + modelName
		if seen[key] {
			continue
		}
		seen[key] = true
		candidates, err := adapterconfig.CompatibilityCandidates(cfg, modelName, info.SourceAPI)
		if err != nil {
			return nil, err
		}
		evaluations = append(evaluations, compatibility.EvaluateMany(candidates, profile)...)
	}
	return evaluations, nil
}

func resolveCompatibilityEvaluations(cfg adapterconfig.Config, model string, sourceAPI adapt.ApiKind, useCase compatibility.UseCase) ([]compatibility.Evaluation, error) {
	profile, ok := compatibility.BuiltinProfile(useCase)
	if !ok {
		return nil, fmt.Errorf("unknown use case %q", useCase)
	}
	candidates, err := adapterconfig.CompatibilityCandidates(cfg, model, sourceAPI)
	if err != nil {
		return nil, err
	}
	return compatibility.EvaluateMany(candidates, profile), nil
}

func printCompatibilityEvaluations(w io.Writer, evaluations []compatibility.Evaluation) {
	if len(evaluations) == 0 {
		fmt.Fprintln(w, "No compatibility candidates")
		return
	}
	if len(evaluations) == 1 {
		printCompatibilityEvaluation(w, evaluations[0])
		return
	}
	fmt.Fprintf(w, "Matches: %d candidates\n", len(evaluations))
	for i, evaluation := range evaluations {
		candidate := evaluation.Candidate
		fmt.Fprintf(w, "\n[%.2d] status=%s provider=%s type=%s source=%s api=%s model=%s native=%s\n",
			i+1,
			evaluation.Status,
			candidate.Provider,
			candidate.ProviderType,
			candidate.SourceAPI,
			candidate.ProviderAPI,
			candidate.PublicModel,
			candidate.NativeModel,
		)
		printCompatibilityEvaluation(w, evaluation)
	}
}

func printConformanceReport(w io.Writer, report conformance.Report) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tAPI_KIND\tFAMILY\tTEXT\tTOOLS\tREASONING\tCACHE_ACCOUNTING\tAGENTIC_APPROVED\tAGENTIC_VALID\tAGENTIC_CONTRACT\tWARNINGS")
	for _, provider := range report.Providers {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s\t%d\n",
			provider.Type,
			provider.APIKind,
			provider.Family,
			provider.Coverage.Text,
			provider.Coverage.Tools,
			provider.Coverage.Reasoning,
			provider.Coverage.PromptCacheAccounting,
			provider.AgenticCoding.ApprovedCount,
			provider.AgenticCoding.ValidApprovedCount,
			provider.AgenticCoding.ContractStatus,
			len(provider.Warnings),
		)
	}
	_ = tw.Flush()
	if report.CompatibilityArtifact != "" {
		fmt.Fprintf(w, "\nCompatibility artifact: %s\n", report.CompatibilityArtifact)
	}
}

func conformanceFailure(report conformance.Report) error {
	var violations int
	for _, provider := range report.Providers {
		violations += len(provider.AgenticCoding.ContractViolations)
	}
	if violations == 0 {
		return nil
	}
	return fmt.Errorf("conformance contract failed: %d approved agentic_coding row(s) violate required evidence", violations)
}

func printUseCaseSelections(w io.Writer, selections []adapterconfig.UseCaseModelSelection) {
	if len(selections) == 0 {
		fmt.Fprintln(w, "No approved use-case selections")
		return
	}
	if len(selections) > 1 {
		fmt.Fprintf(w, "Approved matches: %d candidates\n", len(selections))
	}
	for i, selection := range selections {
		if len(selections) > 1 {
			fmt.Fprintf(w, "\n[%.2d] provider=%s api=%s native=%s status=%s\n",
				i+1,
				selection.Resolution.Provider,
				selection.Resolution.ProviderAPI,
				selection.Resolution.NativeModel,
				selection.Evaluation.Status,
			)
		}
		fmt.Fprintf(w, "Runtime ID:   %s\n", selection.RuntimeID)
		printCompatibilityEvaluation(w, selection.Evaluation)
	}
}

func printCompatibilityEvaluation(w io.Writer, evaluation compatibility.Evaluation) {
	candidate := evaluation.Candidate
	fmt.Fprintf(w, "Use case:     %s\n", evaluation.UseCase)
	fmt.Fprintf(w, "Status:       %s\n", evaluation.Status)
	fmt.Fprintf(w, "Input:        %s\n", candidate.Input)
	fmt.Fprintf(w, "Source API:   %s\n", candidate.SourceAPI)
	fmt.Fprintf(w, "Public model: %s\n", candidate.PublicModel)
	fmt.Fprintf(w, "Native model: %s\n", candidate.NativeModel)
	fmt.Fprintf(w, "Provider:     %s\n", candidate.Provider)
	fmt.Fprintf(w, "Provider type: %s\n", candidate.ProviderType)
	fmt.Fprintf(w, "Provider API: %s\n", candidate.ProviderAPI)
	fmt.Fprintf(w, "Family:       %s\n", candidate.Family)
	if candidate.ModelDBService != "" {
		fmt.Fprintf(w, "ModelDB svc:  %s\n", candidate.ModelDBService)
	}
	if candidate.CapabilitySource != "" {
		fmt.Fprintf(w, "Capability source: %s\n", candidate.CapabilitySource)
	}
	if len(evaluation.MissingRequired) > 0 {
		fmt.Fprintf(w, "Missing required: %s\n", joinFeatures(evaluation.MissingRequired))
	}
	if len(evaluation.UntestedRequired) > 0 {
		fmt.Fprintf(w, "Untested required: %s\n", joinFeatures(evaluation.UntestedRequired))
	}
	if len(evaluation.DegradedPreferred) > 0 {
		fmt.Fprintf(w, "Degraded preferred: %s\n", joinFeatures(evaluation.DegradedPreferred))
	}
	fmt.Fprintln(w, "Features:")
	for _, feature := range evaluation.Features {
		fmt.Fprintf(w, "  %s: requirement=%s supported=%t evidence=%s",
			feature.Feature,
			feature.Requirement,
			feature.Supported,
			feature.Evidence,
		)
		if feature.Detail != "" {
			fmt.Fprintf(w, " detail=%q", feature.Detail)
		}
		fmt.Fprintln(w)
	}
}

func joinFeatures(features []compatibility.Feature) string {
	out := make([]string, 0, len(features))
	for _, feature := range features {
		out = append(out, string(feature))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func enrichCompatibilityArtifactMetadata(artifact *compatibility.Artifact) {
	if artifact == nil {
		return
	}
	for i := range artifact.Rows {
		row := &artifact.Rows[i]
		descriptor, ok := providerregistry.Lookup(row.Provider)
		if !ok {
			continue
		}
		if row.ProviderAPI == "" {
			row.ProviderAPI = descriptor.APIKind
		}
		if row.Family == "" {
			row.Family = descriptor.Family
		}
		if row.ConsumerContinuation == "" {
			row.ConsumerContinuation = descriptor.ConsumerContinuation
		}
		if row.InternalContinuation == "" {
			row.InternalContinuation = descriptor.InternalContinuation
		}
		if row.Transport == "" {
			row.Transport = descriptor.Transport
		}
	}
}

func updateCompatibilityMatrix(path string, artifact compatibility.Artifact) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read compatibility matrix %q: %w", path, err)
	}
	updated, err := compatibility.ReplaceGeneratedSection(string(data), compatibility.RenderArtifactMarkdown(artifact))
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write compatibility matrix %q: %w", path, err)
	}
	return nil
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
	resolution, err := adapterconfig.ResolveModel(cfg, model, sourceAPI)
	if err != nil {
		return resolutionInfo{}, err
	}
	return resolutionInfoFromAdapter(resolution), nil
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
	if resolution.CapabilitySource != "" {
		fmt.Fprintf(w, "Capability source: %s\n", resolution.CapabilitySource)
	}
	if resolution.ConsumerContinuation != "" || resolution.InternalContinuation != "" || resolution.Transport != "" {
		fmt.Fprintf(w, "Continuation: consumer=%s internal=%s transport=%s\n",
			resolution.ConsumerContinuation,
			resolution.InternalContinuation,
			resolution.Transport,
		)
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

func printResolutionCandidates(w io.Writer, resolutions []resolutionInfo) {
	if len(resolutions) == 1 {
		printResolution(w, resolutions[0])
		return
	}
	fmt.Fprintf(w, "Matches: %d candidates\n", len(resolutions))
	for i, resolution := range resolutions {
		fmt.Fprintf(w, "\n[%.2d] provider=%s type=%s source=%s api=%s weight=%d priority=%d\n",
			i+1,
			resolution.Provider,
			resolution.ProviderType,
			resolution.SourceAPI,
			resolution.ProviderAPI,
			resolution.Weight,
			resolution.Priority,
		)
		printResolution(w, resolution)
	}
}

func resolveModelCandidates(cfg adapterconfig.Config, model string) ([]resolutionInfo, error) {
	candidates, err := adapterconfig.ResolveModelCandidates(cfg, model, "")
	if err != nil {
		return nil, err
	}
	resolutions := make([]resolutionInfo, 0, len(candidates))
	for _, candidate := range candidates {
		resolutions = append(resolutions, resolutionInfoFromAdapter(candidate))
	}
	return resolutions, nil
}

func resolutionInfoFromAdapter(resolution adapterconfig.ModelResolutionCandidate) resolutionInfo {
	return resolutionInfo{
		Input:                resolution.Input,
		MatchedAs:            resolution.MatchedAs,
		SourceAPI:            resolution.SourceAPI,
		PublicModel:          resolution.PublicModel,
		NativeModel:          resolution.NativeModel,
		Provider:             resolution.Provider,
		ProviderType:         resolution.ProviderType,
		ProviderAPI:          resolution.ProviderAPI,
		Family:               resolution.Family,
		Weight:               resolution.Weight,
		Priority:             resolution.Priority,
		ModelDBService:       resolution.ModelDBService,
		Capabilities:         resolution.Capabilities,
		CapabilitySource:     resolution.CapabilitySource,
		ConsumerContinuation: resolution.ConsumerContinuation,
		InternalContinuation: resolution.InternalContinuation,
		Transport:            resolution.Transport,
	}
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

func runInferCommand(ctx context.Context, w io.Writer, debugw io.Writer, params inferParams, prompt string) error {
	sourceAPI := adapt.ApiKind(params.sourceAPI)
	model := params.model
	debugScopes, err := parseInferDebugScopes(params.debugScopes)
	if err != nil {
		return err
	}
	cfg, err := inferConfig(params.configPath, sourceAPI, model)
	if err != nil {
		return err
	}
	if model == "" {
		model = defaultModelFromRoutes(cfg.Routes, sourceAPI)
	}
	if model == "" {
		return fmt.Errorf("model is required when no default route model exists")
	}
	resolution, err := resolveInferModel(cfg, model, sourceAPI)
	if err != nil {
		return err
	}
	muxOptions := []adapterconfig.MuxClientOption{adapterconfig.WithSourceAPI(resolution.SourceAPI)}
	if debugScopes.httpEnabled() {
		muxOptions = append(muxOptions, adapterconfig.WithProviderTransport(newInferDebugTransport(debugw, debugScopes)))
		muxOptions = append(muxOptions, adapterconfig.WithProviderWebSocketTransport(newInferDebugWebSocketTransport(debugw, debugScopes)))
	}
	client, err := adapterconfig.NewMuxClient(cfg, muxOptions...)
	if err != nil {
		return err
	}
	printResolution(w, resolution)
	fmt.Fprintln(w)
	return runInferRequest(ctx, w, debugw, client, model, prompt, params, debugScopes)
}

func inferConfig(configPath string, sourceAPI adapt.ApiKind, model string) (adapterconfig.Config, error) {
	if configPath != "" {
		cfg, err := adapterconfig.Load(configPath)
		if err != nil {
			return adapterconfig.Config{}, err
		}
		return cfg, nil
	}
	result, err := adapterconfig.AutoMuxClient(adapterconfig.AutoOptions{
		EnableEnv:         true,
		EnableLocalClaude: true,
		EnableLocalCodex:  true,
		UseModelDB:        true,
		DynamicModels:     true,
		SourceAPI:         sourceAPI,
		Intents:           inferAutoIntents(model, sourceAPI),
	})
	if err != nil {
		return result.Config, err
	}
	return result.Config, nil
}

func inferAutoIntents(model string, sourceAPI adapt.ApiKind) []adapterconfig.AutoIntent {
	if model == "" {
		return nil
	}
	return []adapterconfig.AutoIntent{{
		Name:      model,
		SourceAPI: sourceAPI,
	}}
}

func resolveInferModel(cfg adapterconfig.Config, model string, sourceAPI adapt.ApiKind) (resolutionInfo, error) {
	if sourceAPI != "" {
		return resolveModel(cfg, model, sourceAPI)
	}
	resolutions, err := resolveModelCandidates(cfg, model)
	if err != nil {
		return resolutionInfo{}, err
	}
	return resolutions[0], nil
}

func runInferRequest(ctx context.Context, w io.Writer, debugw io.Writer, client unified.Client, model, prompt string, params inferParams, debugScopes inferDebugScopes) error {
	ctx, cancel := context.WithTimeout(ctx, params.timeout)
	defer cancel()
	req, err := inferRequest(model, prompt, params)
	if err != nil {
		return err
	}
	printInferRequestMetadata(w, req)
	events, err := client.Request(ctx, req)
	if err != nil {
		return err
	}
	var (
		inReasoning bool
		hadOutput   bool
		lastUsage   *unified.UsageEvent
		quotas      []unified.QuotaUsageEvent
		routeEvent  *unified.RouteEvent
	)
	for ev := range events {
		if debugScopes.events {
			writeInferDebugEvent(debugw, ev)
		}
		switch e := ev.(type) {
		case unified.RouteEvent:
			if debugScopes.enabled {
				writeInferDebugMode(debugw, "route", e.Transport, e.InternalContinuation)
			}
			copy := e
			routeEvent = &copy
		case unified.ProviderExecutionEvent:
			if debugScopes.enabled {
				writeInferDebugMode(debugw, "provider", e.Transport, e.InternalContinuation)
			}
			if routeEvent == nil {
				routeEvent = &unified.RouteEvent{}
			}
			if e.InternalContinuation != "" {
				routeEvent.InternalContinuation = e.InternalContinuation
			}
			if e.Transport != "" {
				routeEvent.Transport = e.Transport
			}
		case unified.ReasoningDeltaEvent:
			if !inReasoning {
				fmt.Fprint(w, ansiDim)
				inReasoning = true
			}
			fmt.Fprint(w, e.Text)
			hadOutput = true
		case unified.TextDeltaEvent:
			if inReasoning {
				fmt.Fprint(w, ansiReset)
				inReasoning = false
			}
			fmt.Fprint(w, e.Text)
			hadOutput = true
		case unified.UsageEvent:
			copy := e
			lastUsage = &copy
		case unified.QuotaUsageEvent:
			quotas = append(quotas, e)
		case unified.ErrorEvent:
			if inReasoning {
				fmt.Fprint(w, ansiReset)
			}
			return e.Err
		}
	}
	if inReasoning {
		fmt.Fprint(w, ansiReset)
	}
	if hadOutput {
		fmt.Fprintln(w)
	}
	printInferRouteMetadata(w, routeEvent)
	printInferUsage(w, lastUsage)
	printInferQuotas(w, quotas)
	return nil
}

func inferRequest(model, prompt string, params inferParams) (unified.Request, error) {
	req := unified.Request{
		Model:           model,
		MaxOutputTokens: &params.maxTokens,
		CachePolicy:     unified.CachePolicyOn,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: prompt}},
		}},
		Stream: true,
	}
	if params.noCache {
		req.CachePolicy = unified.CachePolicyOff
	}
	mode, err := inferInteractionMode(params)
	if err != nil {
		return unified.Request{}, err
	}
	codexExt := unified.CodexExtensions{
		InteractionMode: mode,
		SessionID:       params.session,
		BranchID:        params.branch,
	}
	if params.session != "" && !params.noCache {
		req.CacheKey = params.session
	}
	if err := unified.SetCodexExtensions(&req.Extensions, codexExt); err != nil {
		return unified.Request{}, err
	}
	if params.system != "" {
		req.Instructions = append(req.Instructions, unified.Instruction{
			Kind:    unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{Text: params.system}},
		})
	}
	if params.temperature > 0 {
		req.Temperature = &params.temperature
	}
	reasoning, err := inferReasoning(params.thinking, params.effort)
	if err != nil {
		return unified.Request{}, err
	}
	req.Reasoning = reasoning
	return req, nil
}

func inferInteractionMode(params inferParams) (unified.InteractionMode, error) {
	mode := unified.InteractionMode(strings.ToLower(strings.TrimSpace(params.interaction)))
	if mode == "" {
		mode = unified.InteractionOneShot
	}
	if !unified.ValidInteractionMode(mode) {
		return "", fmt.Errorf("invalid interaction mode %q", params.interaction)
	}
	return mode, nil
}

func inferReasoning(thinking, effort string) (*unified.ReasoningConfig, error) {
	thinking = strings.ToLower(strings.TrimSpace(thinking))
	effort = strings.ToLower(strings.TrimSpace(effort))
	if thinking != "" {
		switch thinking {
		case "auto", "on", "off":
		default:
			return nil, fmt.Errorf("invalid thinking mode %q", thinking)
		}
	}
	if effort != "" {
		switch effort {
		case "low", "medium", "high", "max":
		default:
			return nil, fmt.Errorf("invalid effort %q", effort)
		}
	}
	if thinking == "" && effort == "" {
		return nil, nil
	}
	if thinking == "off" {
		return nil, nil
	}
	cfg := &unified.ReasoningConfig{}
	if thinking == "on" || effort != "" {
		cfg.Expose = true
	}
	if effort != "" {
		cfg.Effort = unified.ReasoningEffort(effort)
	}
	return cfg, nil
}

func printInferUsage(w io.Writer, usage *unified.UsageEvent) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s── usage ──%s\n", ansiDim, ansiReset)
	if usage == nil || (!usage.Usage().HasTokens() && usage.Costs.Total() == 0) {
		fmt.Fprintln(w, "  unavailable")
		return
	}
	for _, item := range usage.Tokens {
		if item.Count != 0 {
			fmt.Fprintf(w, "  %s: %d\n", item.Kind, item.Count)
		}
	}
	if total := usage.Costs.Total(); total > 0 {
		fmt.Fprintf(w, "  cost: %s\n", formatCost(total))
	}
}

func printInferQuotas(w io.Writer, quotas []unified.QuotaUsageEvent) {
	if len(quotas) == 0 {
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s── quotas ──%s\n", ansiDim, ansiReset)
	for i, quota := range quotas {
		fmt.Fprintf(w, "  [%d] provider: %s\n", i+1, quota.Provider)
		if quota.Plan != "" {
			fmt.Fprintf(w, "      plan: %s\n", quota.Plan)
		}
		if len(quota.ProviderRaw) > 0 {
			fmt.Fprintf(w, "      raw: %s\n", compactJSON(quota.ProviderRaw))
		}
		for _, limit := range quota.Limits {
			fmt.Fprintf(w, "      limit: %s\n", limit.ID)
			if limit.Name != "" {
				fmt.Fprintf(w, "        name: %s\n", limit.Name)
			}
			for _, window := range limit.Windows {
				fmt.Fprintf(w, "        window=%s used=%.2f%%", window.Name, window.UsedPercent)
				if window.Limit != nil {
					fmt.Fprintf(w, " limit=%d", *window.Limit)
				}
				if window.Remaining != nil {
					fmt.Fprintf(w, " remaining=%d", *window.Remaining)
				}
				if window.WindowMinutes != nil {
					fmt.Fprintf(w, " minutes=%d", *window.WindowMinutes)
				}
				if window.ResetsAtUnix != nil {
					fmt.Fprintf(w, " resets_at=%d", *window.ResetsAtUnix)
				}
				fmt.Fprintln(w)
			}
		}
	}
}

func compactJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func printInferRequestMetadata(w io.Writer, req unified.Request) {
	codexExt, _ := unified.CodexExtensionsFrom(req.Extensions)
	if codexExt.InteractionMode == "" && codexExt.SessionID == "" && codexExt.BranchID == "" && req.CacheKey == "" {
		return
	}
	fmt.Fprintf(w, "%s── request ──%s\n", ansiDim, ansiReset)
	fmt.Fprintf(w, "  interaction: %s\n", codexExt.InteractionMode)
	if codexExt.SessionID != "" {
		fmt.Fprintf(w, "  session: %s\n", codexExt.SessionID)
	}
	if codexExt.BranchID != "" {
		fmt.Fprintf(w, "  branch: %s\n", codexExt.BranchID)
	}
	if req.CacheKey != "" {
		fmt.Fprintf(w, "  cache_key: %s\n", req.CacheKey)
	}
	fmt.Fprintln(w)
}

func printInferRouteMetadata(w io.Writer, route *unified.RouteEvent) {
	if route == nil {
		return
	}
	if route.ConsumerContinuation == "" && route.InternalContinuation == "" && route.Transport == "" {
		return
	}
	fmt.Fprintf(w, "%s── route ──%s\n", ansiDim, ansiReset)
	fmt.Fprintf(w, "  consumer_continuation: %s\n", route.ConsumerContinuation)
	fmt.Fprintf(w, "  internal_continuation: %s\n", route.InternalContinuation)
	fmt.Fprintf(w, "  transport: %s\n", route.Transport)
}

func formatCost(cost float64) string {
	switch {
	case cost < 0.0001:
		return fmt.Sprintf("$%.8f", cost)
	case cost < 0.01:
		return fmt.Sprintf("$%.6f", cost)
	case cost < 1:
		return fmt.Sprintf("$%.4f", cost)
	default:
		return fmt.Sprintf("$%.2f", cost)
	}
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
