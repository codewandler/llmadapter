package adapterconfig

import (
	"testing"

	"github.com/codewandler/llmadapter/adapt"
)

func TestAutoResultRouteSummaryFromConfig(t *testing.T) {
	result := AutoResult{
		Config: Config{Routes: []RouteConfig{{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "default",
			Provider:    "openai_responses",
			ProviderAPI: adapt.ApiOpenAIResponses,
			NativeModel: "gpt-test",
		}}},
		Enabled: []AutoProvider{{Name: "openai_responses", Type: "openai_responses", Reason: "env:OPENAI_API_KEY"}},
	}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "default")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.Provider != "openai_responses" || summary.NativeModel != "gpt-test" || summary.EnabledReason != "env:OPENAI_API_KEY" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestAutoResultRouteSummaryDefaultsSourceAPI(t *testing.T) {
	result := AutoResult{Config: Config{Routes: []RouteConfig{{
		SourceAPI: adapt.ApiOpenAIResponses,
		Model:     "default",
		Provider:  "openai_responses",
	}}}}

	summary, ok := result.RouteSummary("", "")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.SourceAPI != adapt.ApiOpenAIResponses || summary.NativeModel != "default" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestAutoResultRouteSummaryAutoSourcePrefersAnthropicMessages(t *testing.T) {
	result := AutoResult{Config: Config{Routes: []RouteConfig{
		{
			SourceAPI:   adapt.ApiOpenAIResponses,
			Model:       "haiku",
			Provider:    "claude",
			ProviderAPI: adapt.ApiAnthropicMessages,
			NativeModel: "claude-haiku",
			Weight:      100,
		},
		{
			SourceAPI:   adapt.ApiAnthropicMessages,
			Model:       "haiku",
			Provider:    "claude",
			ProviderAPI: adapt.ApiAnthropicMessages,
			NativeModel: "claude-haiku",
			Weight:      100,
		},
	}}}

	summary, ok := result.RouteSummary("", "haiku")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.SourceAPI != adapt.ApiAnthropicMessages || summary.ProviderAPI != adapt.ApiAnthropicMessages {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestAutoResultRouteSummaryDynamicModel(t *testing.T) {
	result := AutoResult{Config: Config{Routes: []RouteConfig{{
		SourceAPI:     adapt.ApiOpenAIResponses,
		Provider:      "openai_responses",
		ProviderAPI:   adapt.ApiOpenAIResponses,
		DynamicModels: true,
	}}}}

	summary, ok := result.RouteSummary(adapt.ApiOpenAIResponses, "gpt-new")
	if !ok {
		t.Fatal("expected summary")
	}
	if summary.Model != "gpt-new" || summary.NativeModel != "gpt-new" {
		t.Fatalf("unexpected dynamic summary: %+v", summary)
	}
}
