package codex

import (
	"encoding/json"
	"testing"
)

func TestContinuationManagerUsesPreviousResponseIDForAppendOnlySameBranch(t *testing.T) {
	manager := newContinuationManager()
	first := []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user"}]}`)
	decision := manager.prepare("sess", "main", first)
	if decision.previousResponseID != "" {
		t.Fatalf("first previous_response_id = %q", decision.previousResponseID)
	}
	decision.commit("resp_1")

	second := []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user"},{"type":"message","role":"assistant"}]}`)
	decision = manager.prepare("sess", "main", second)
	if decision.previousResponseID != "resp_1" {
		t.Fatalf("previous_response_id = %q, want resp_1", decision.previousResponseID)
	}
}

func TestContinuationManagerRejectsBranchAndPromptChanges(t *testing.T) {
	manager := newContinuationManager()
	first := []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user"}]}`)
	manager.prepare("sess", "main", first).commit("resp_1")

	cases := []struct {
		name    string
		session string
		branch  string
		body    []byte
	}{
		{name: "branch", session: "sess", branch: "other", body: []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user"},{"type":"message","role":"assistant"}]}`)},
		{name: "model", session: "sess", branch: "main", body: []byte(`{"model":"gpt-5.4-mini","instructions":"same","input":[{"type":"message","role":"user"},{"type":"message","role":"assistant"}]}`)},
		{name: "instructions", session: "sess", branch: "main", body: []byte(`{"model":"gpt-5.4","instructions":"changed","input":[{"type":"message","role":"user"},{"type":"message","role":"assistant"}]}`)},
		{name: "rewrite", session: "sess", branch: "main", body: []byte(`{"model":"gpt-5.4","instructions":"same","input":[]}`)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			decision := manager.prepare(tt.session, tt.branch, tt.body)
			if decision.previousResponseID != "" {
				t.Fatalf("previous_response_id = %q, want empty", decision.previousResponseID)
			}
		})
	}
}

func TestContinuationManagerRejectsSameLengthInputRewrite(t *testing.T) {
	manager := newContinuationManager()
	first := []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}]}`)
	manager.prepare("sess", "main", first).commit("resp_1")

	rewrite := []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"rewritten"}]}]}`)
	decision := manager.prepare("sess", "main", rewrite)
	if decision.previousResponseID != "" {
		t.Fatalf("previous_response_id = %q, want empty for same-length rewrite", decision.previousResponseID)
	}
}

func TestContinuationManagerRejectsLongerInputWithRewrittenPrefix(t *testing.T) {
	manager := newContinuationManager()
	first := []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]}]}`)
	manager.prepare("sess", "main", first).commit("resp_1")

	rewrite := []byte(`{"model":"gpt-5.4","instructions":"same","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"rewritten"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"two"}]}]}`)
	decision := manager.prepare("sess", "main", rewrite)
	if decision.previousResponseID != "" {
		t.Fatalf("previous_response_id = %q, want empty for rewritten prefix", decision.previousResponseID)
	}
}

func TestWithPreviousResponseIDAddsOnlyWebSocketField(t *testing.T) {
	body := withPreviousResponseID([]byte(`{"type":"response.create","model":"gpt-5.4"}`), "resp_1", 0)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["previous_response_id"] != "resp_1" {
		t.Fatalf("previous_response_id = %v", payload["previous_response_id"])
	}
	if payload["type"] != "response.create" {
		t.Fatalf("type = %v", payload["type"])
	}
}

func TestWithPreviousResponseIDSendsOnlyIncrementalUserInput(t *testing.T) {
	body := withPreviousResponseID([]byte(`{"type":"response.create","model":"gpt-5.4","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"one"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"two"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"three"}]}]}`), "resp_1", 1)
	var payload struct {
		PreviousResponseID string `json:"previous_response_id"`
		Input              []struct {
			Role string `json:"role"`
		} `json:"input"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.PreviousResponseID != "resp_1" {
		t.Fatalf("previous_response_id = %q, want resp_1", payload.PreviousResponseID)
	}
	if len(payload.Input) != 1 || payload.Input[0].Role != "user" {
		t.Fatalf("input = %+v, want only incremental user message", payload.Input)
	}
}
