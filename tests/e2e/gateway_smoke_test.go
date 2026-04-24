package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	"github.com/codewandler/llmadapter/gateway"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	"github.com/codewandler/llmadapter/router"
)

func TestGatewaySmokeNonStreaming(t *testing.T) {
	handler, model := newAnthropicGateway(t)
	body := `{
		"model":` + jsonQuote(model) + `,
		"messages":[{"role":"user","content":"Reply with exactly: llmadapter gateway smoke ok"}],
		"max_tokens":64
	}`

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Object  string `json:"object"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Object != "chat.completion" || len(resp.Choices) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !strings.Contains(strings.ToLower(resp.Choices[0].Message.Content), "llmadapter gateway smoke ok") {
		t.Fatalf("unexpected content: %q", resp.Choices[0].Message.Content)
	}
}

func TestGatewaySmokeStreaming(t *testing.T) {
	handler, model := newAnthropicGateway(t)
	body := `{
		"model":` + jsonQuote(model) + `,
		"messages":[{"role":"user","content":"Reply with exactly: llmadapter gateway stream ok"}],
		"max_tokens":64,
		"stream":true
	}`

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	text, done, err := collectOpenAIStreamText(w.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatalf("stream did not include [DONE]: %s", w.Body.String())
	}
	if !strings.Contains(strings.ToLower(text), "llmadapter gateway stream ok") {
		t.Fatalf("unexpected stream text: %q", text)
	}
}

func newAnthropicGateway(t *testing.T) (http.Handler, string) {
	t.Helper()
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("set ANTHROPIC_API_KEY to run Anthropic gateway e2e smoke tests")
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	client, err := anthropic.NewClient(
		anthropic.WithAPIKey(apiKey),
		anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
			req.Unified.Stream = true
			return nil
		})),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return timeoutHandler{
		ctx: ctx,
		handler: gateway.Handler{
			Endpoint: chat.Codec{},
			Router: router.NewStaticRouter(router.StaticRoute{
				SourceAPI: adapt.ApiOpenAIChatCompletions,
				Client:    client,
			}),
		},
	}, model
}

type timeoutHandler struct {
	ctx     context.Context
	handler http.Handler
}

func (h timeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r.WithContext(h.ctx))
}

func collectOpenAIStreamText(body []byte) (string, bool, error) {
	var text strings.Builder
	done := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			done = true
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", false, err
		}
		for _, choice := range chunk.Choices {
			text.WriteString(choice.Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return text.String(), done, nil
}

func jsonQuote(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
}
