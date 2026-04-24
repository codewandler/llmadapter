package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/adapterconfig"
	"github.com/codewandler/llmadapter/muxclient"
	"github.com/codewandler/llmadapter/providerregistry"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	switch args[0] {
	case "providers":
		return runProviders(args[1:])
	case "smoke":
		return runSmoke(args[1:])
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runProviders(args []string) error {
	fs := flag.NewFlagSet("providers", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print providers as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	descriptors := providerregistry.List()
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(descriptors)
	}
	for _, descriptor := range descriptors {
		fmt.Printf("%s\t%s\t%s\tmodel_env=%s\tdefault_model=%s\n", descriptor.Type, descriptor.APIKind, descriptor.Family, descriptor.DefaultModelEnv, descriptor.DefaultModel)
	}
	return nil
}

func runSmoke(args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	mode := fs.String("mode", "direct", "smoke mode: direct or mux")
	configPath := fs.String("config", "", "llmadapter JSON config path for mux mode")
	sourceAPI := fs.String("source-api", string(adapt.ApiOpenAIResponses), "source API for mux mode")
	providerType := fs.String("type", "openai_responses", "provider endpoint type")
	model := fs.String("model", "", "model to request")
	nativeModel := fs.String("native-model", "", "native model for mux mode; defaults to model")
	apiKey := fs.String("api-key", "", "API key; prefer env vars in normal use")
	apiKeyEnv := fs.String("api-key-env", "", "environment variable containing the API key")
	baseURL := fs.String("base-url", "", "provider base URL override")
	prompt := fs.String("prompt", "Reply with exactly: llmadapter cli smoke ok", "prompt text")
	timeout := fs.Duration("timeout", 45*time.Second, "request timeout")
	maxTokens := fs.Int("max-tokens", 64, "maximum output tokens")
	if err := fs.Parse(args); err != nil {
		return err
	}
	requestModel := *model
	if requestModel == "" && *configPath != "" {
		requestModel = defaultModelFromConfig(*configPath, adapt.ApiKind(*sourceAPI))
	}
	if *mode == "mux" && *configPath != "" {
		if requestModel == "" {
			return fmt.Errorf("model is required when config has no route model")
		}
		cfg, err := adapterconfig.Load(*configPath)
		if err != nil {
			return err
		}
		client, err := adapterconfig.NewMuxClient(cfg, adapterconfig.WithSourceAPI(adapt.ApiKind(*sourceAPI)))
		if err != nil {
			return err
		}
		return runSmokeRequest(client, requestModel, *prompt, *timeout, *maxTokens)
	}
	descriptor, ok := providerregistry.Lookup(*providerType)
	if !ok {
		return fmt.Errorf("unknown provider type %q", *providerType)
	}
	key := *apiKey
	if key == "" && *apiKeyEnv != "" {
		key = os.Getenv(*apiKeyEnv)
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
		BaseURL: *baseURL,
	})
	if err != nil {
		return err
	}
	client := providerClient
	if *mode == "mux" {
		routeModel := requestModel
		if *nativeModel == "" {
			*nativeModel = requestModel
		}
		client = muxclient.New(router.NewStaticRouter(router.StaticRoute{
			SourceAPI:   adapt.ApiKind(*sourceAPI),
			Model:       routeModel,
			NativeModel: *nativeModel,
			Endpoint: router.ProviderEndpoint{
				ProviderName: descriptor.Type,
				APIKind:      descriptor.APIKind,
				Family:       descriptor.Family,
				Client:       providerClient,
				Capabilities: descriptor.Capabilities,
			},
		}), muxclient.WithSourceAPI(adapt.ApiKind(*sourceAPI)))
	} else if *mode != "direct" {
		return fmt.Errorf("unknown smoke mode %q", *mode)
	}

	return runSmokeRequest(client, requestModel, *prompt, *timeout, *maxTokens)
}

func runSmokeRequest(client unified.Client, model, prompt string, timeout time.Duration, maxTokens int) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
	fmt.Println(text)
	return nil
}

func defaultModelFromConfig(path string, sourceAPI adapt.ApiKind) string {
	cfg, err := adapterconfig.Load(path)
	if err != nil {
		return ""
	}
	for _, route := range cfg.Routes {
		if route.SourceAPI == sourceAPI && route.Model != "" {
			return route.Model
		}
	}
	for _, route := range cfg.Routes {
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

func usage() {
	fmt.Fprintln(os.Stderr, "usage: llmadapter <providers|smoke> [flags]")
}
