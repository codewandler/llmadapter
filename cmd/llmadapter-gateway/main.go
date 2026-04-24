package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/codewandler/llmadapter/adapt"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	"github.com/codewandler/llmadapter/gateway"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
)

func main() {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}
	addr := getenv("LLMADAPTER_ADDR", ":8080")
	modelOverride := os.Getenv("LLMADAPTER_UPSTREAM_MODEL")

	client, err := anthropic.NewClient(
		anthropic.WithAPIKey(apiKey),
		anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
			// The Anthropic client path is stream-first; endpoint codecs can still collect into non-stream JSON.
			req.Unified.Stream = true
			if modelOverride != "" {
				req.Unified.Model = modelOverride
			}
			return nil
		})),
	)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/chat/completions", gateway.Handler{
		Endpoint: chat.Codec{},
		Client:   client,
	})

	log.Printf("llmadapter gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
}
