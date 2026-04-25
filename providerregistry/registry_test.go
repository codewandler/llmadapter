package providerregistry

import (
	"testing"

	"github.com/codewandler/llmadapter/adapt"
)

func TestLookup(t *testing.T) {
	descriptor, ok := Lookup("openai_responses")
	if !ok {
		t.Fatalf("missing openai_responses descriptor")
	}
	if descriptor.APIKind != adapt.ApiOpenAIResponses || descriptor.Family != adapt.FamilyOpenAIResponses {
		t.Fatalf("unexpected descriptor: %+v", descriptor)
	}
	if !descriptor.Capabilities.Streaming || !descriptor.Capabilities.Tools || !descriptor.Capabilities.JSONSchema {
		t.Fatalf("unexpected capabilities: %+v", descriptor.Capabilities)
	}
	if descriptor.Factory == nil {
		t.Fatalf("missing client factory")
	}
}

func TestAnthropicFamilyDescriptorsAdvertiseReasoning(t *testing.T) {
	for _, providerType := range []string{"anthropic", "claude", "openrouter_messages", "minimax_messages"} {
		descriptor, ok := Lookup(providerType)
		if !ok {
			t.Fatalf("missing descriptor %q", providerType)
		}
		if !descriptor.Capabilities.Reasoning || !descriptor.Capabilities.ReasoningDeltas {
			t.Fatalf("%s should advertise reasoning: %+v", providerType, descriptor.Capabilities)
		}
	}
}

func TestResponsesDescriptorsAdvertiseReasoning(t *testing.T) {
	for _, providerType := range []string{"openai_responses", "openrouter_responses", "codex_responses"} {
		descriptor, ok := Lookup(providerType)
		if !ok {
			t.Fatalf("missing descriptor %q", providerType)
		}
		if !descriptor.Capabilities.Reasoning {
			t.Fatalf("%s should advertise reasoning: %+v", providerType, descriptor.Capabilities)
		}
	}
}

func TestAnthropicCacheControlDescriptorsAdvertisePromptCaching(t *testing.T) {
	for _, providerType := range []string{"anthropic", "claude", "openrouter_messages", "minimax_messages"} {
		descriptor, ok := Lookup(providerType)
		if !ok {
			t.Fatalf("missing descriptor %q", providerType)
		}
		if !descriptor.Capabilities.PromptCaching {
			t.Fatalf("%s should advertise prompt caching: %+v", providerType, descriptor.Capabilities)
		}
	}
}

func TestResponsesCacheKeyDescriptorsAdvertisePromptCaching(t *testing.T) {
	for _, providerType := range []string{"openai_responses", "openrouter_responses", "codex_responses"} {
		descriptor, ok := Lookup(providerType)
		if !ok {
			t.Fatalf("missing descriptor %q", providerType)
		}
		if !descriptor.Capabilities.PromptCaching {
			t.Fatalf("%s should advertise prompt caching: %+v", providerType, descriptor.Capabilities)
		}
	}
}

func TestListSorted(t *testing.T) {
	list := List()
	if len(list) == 0 {
		t.Fatalf("expected descriptors")
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].Type > list[i].Type {
			t.Fatalf("descriptors are not sorted: %+v", list)
		}
	}
	for _, descriptor := range list {
		if descriptor.Factory == nil {
			t.Fatalf("%s missing factory", descriptor.Type)
		}
	}
}

func TestNewClientRequiresKnownType(t *testing.T) {
	_, err := NewClient(ClientConfig{Type: "missing", APIKey: "key"})
	if err == nil {
		t.Fatalf("expected unsupported provider type error")
	}
}

func TestNewClientRequiresAPIKey(t *testing.T) {
	_, err := NewClient(ClientConfig{Type: "openai_responses"})
	if err == nil {
		t.Fatalf("expected missing api key error")
	}
}
