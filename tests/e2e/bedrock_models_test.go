package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	bedrockmessages "github.com/codewandler/llmadapter/providers/bedrock/messages"
	bedrockresponses "github.com/codewandler/llmadapter/providers/bedrock/responses"
	"github.com/codewandler/llmadapter/unified"
)

func TestBedrockResponsesModelsList(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	if firstSetEnv(bedrockresponses.EnvAPIKey, bedrockresponses.EnvBearerToken) == "" && os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("set a Bedrock bearer token or AWS_PROFILE/AWS_ACCESS_KEY_ID to run Bedrock models list smoke test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token := firstSetEnv(bedrockresponses.EnvAPIKey, bedrockresponses.EnvBearerToken)
	if token == "" {
		var err error
		token, err = bedrockresponses.NewAWSTokenProvider("").Token(ctx)
		if err != nil {
			t.Fatalf("generate Bedrock token: %v", err)
		}
	}
	region := os.Getenv(bedrockresponses.EnvRegion)
	if region == "" {
		region = os.Getenv(bedrockresponses.EnvDefaultRegion)
	}
	if region == "" {
		region = "us-east-1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://bedrock-mantle."+region+".api.aws/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("list models status = %s", resp.Status)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode models response: %v", err)
	}
	var ids []string
	for _, model := range body.Data {
		if model.ID != "" {
			ids = append(ids, model.ID)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		t.Fatalf("empty models list")
	}
	t.Logf("bedrock mantle models (%d): %s", len(ids), strings.Join(ids, ", "))
}

func TestBedrockMessagesModelsList(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	if firstSetEnv(bedrockmessages.EnvAPIKey, bedrockmessages.EnvBearerToken) == "" && os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("set a Bedrock bearer token or AWS_PROFILE/AWS_ACCESS_KEY_ID to run Bedrock Messages models list smoke test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token := firstSetEnv(bedrockmessages.EnvAPIKey, bedrockmessages.EnvBearerToken)
	if token == "" {
		var err error
		token, err = bedrockresponses.NewAWSTokenProvider("").Token(ctx)
		if err != nil {
			t.Fatalf("generate Bedrock token: %v", err)
		}
	}
	region := os.Getenv(bedrockmessages.EnvRegion)
	if region == "" {
		region = os.Getenv(bedrockmessages.EnvDefaultRegion)
	}
	if region == "" {
		region = "us-east-1"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://bedrock-mantle."+region+".api.aws/anthropic/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list messages models: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		t.Logf("bedrock mantle anthropic models endpoint is not exposed at /anthropic/v1/models")
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("list messages models status = %s", resp.Status)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode messages models response: %v", err)
	}
	var ids []string
	for _, model := range body.Data {
		if model.ID != "" {
			ids = append(ids, model.ID)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		t.Fatalf("empty messages models list")
	}
	t.Logf("bedrock mantle anthropic models (%d): %s", len(ids), strings.Join(ids, ", "))
}

func TestBedrockResponsesProbeModels(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	rawModels := os.Getenv("BEDROCK_RESPONSES_PROBE_MODELS")
	if rawModels == "" {
		t.Skip("set BEDROCK_RESPONSES_PROBE_MODELS to a comma-separated list of model IDs to probe")
	}
	if firstSetEnv(bedrockresponses.EnvAPIKey, bedrockresponses.EnvBearerToken) == "" && os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("set a Bedrock bearer token or AWS_PROFILE/AWS_ACCESS_KEY_ID to run Bedrock responses model probe")
	}

	client, err := bedrockresponses.NewClient()
	if err != nil {
		t.Fatalf("new Bedrock Responses client: %v", err)
	}

	var models []string
	for _, model := range strings.Split(rawModels, ",") {
		model = strings.TrimSpace(model)
		if model != "" {
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		t.Fatalf("BEDROCK_RESPONSES_PROBE_MODELS did not contain any model IDs")
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()

			maxTokens := 64
			resp, err := collectSmokeResponse(ctx, client, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{{
					Role:    unified.RoleUser,
					Content: []unified.ContentPart{unified.TextPart{Text: "Reply with exactly: ok"}},
				}},
				Stream: true,
			})
			if err != nil {
				var apiErr *unified.APIError
				if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest && strings.Contains(apiErr.Message, "does not support") && strings.Contains(apiErr.Message, "/v1/responses") {
					t.Logf("model %s responses probe unsupported: %s", model, apiErr.Message)
					return
				}
				t.Fatalf("probe %s: %v", model, err)
			}
			if resp.ID == "" && responseText(resp) == "" && resp.FinishReason == "" {
				t.Fatalf("probe %s returned an empty successful response: %+v", model, resp)
			}
			t.Logf("model %s responses probe ok: id=%s finish=%s text=%q", model, resp.ID, resp.FinishReason, responseText(resp))
		})
	}
}

func TestBedrockMessagesProbeModels(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	rawModels := os.Getenv("BEDROCK_MESSAGES_PROBE_MODELS")
	if rawModels == "" {
		t.Skip("set BEDROCK_MESSAGES_PROBE_MODELS to a comma-separated list of model IDs to probe")
	}
	if firstSetEnv(bedrockmessages.EnvAPIKey, bedrockmessages.EnvBearerToken) == "" && os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("set a Bedrock bearer token or AWS_PROFILE/AWS_ACCESS_KEY_ID to run Bedrock messages model probe")
	}

	client, err := bedrockmessages.NewClient()
	if err != nil {
		t.Fatalf("new Bedrock Messages client: %v", err)
	}

	var models []string
	for _, model := range strings.Split(rawModels, ",") {
		model = strings.TrimSpace(model)
		if model != "" {
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		t.Fatalf("BEDROCK_MESSAGES_PROBE_MODELS did not contain any model IDs")
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
			defer cancel()

			maxTokens := 512
			resp, err := collectSmokeResponse(ctx, client, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{{
					Role:    unified.RoleUser,
					Content: []unified.ContentPart{unified.TextPart{Text: "Reply with exactly: ok"}},
				}},
				Stream: true,
			})
			if err != nil {
				var apiErr *unified.APIError
				if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest && strings.Contains(apiErr.Message, "does not support") && strings.Contains(apiErr.Message, "/v1/messages") {
					t.Logf("model %s messages probe unsupported: %s", model, apiErr.Message)
					return
				}
				if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound && strings.Contains(apiErr.Message, "does not exist") {
					t.Logf("model %s messages probe unavailable: %s", model, apiErr.Message)
					return
				}
				t.Fatalf("probe %s: %v", model, err)
			}
			if resp.ID == "" && responseText(resp) == "" && resp.FinishReason == "" {
				t.Fatalf("probe %s returned an empty successful response: %+v", model, resp)
			}
			t.Logf("model %s messages probe ok: id=%s finish=%s text=%q", model, resp.ID, resp.FinishReason, responseText(resp))
		})
	}
}
