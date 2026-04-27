package gatewayserver

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/codewandler/llmadapter/adapterconfig"
	anthropicendpoint "github.com/codewandler/llmadapter/endpoints/anthropicmessages"
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	responsesendpoint "github.com/codewandler/llmadapter/endpoints/openairesponses"
	"github.com/codewandler/llmadapter/gateway"
)

const (
	defaultReadHeaderTimeout = 10 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = 30 * time.Minute
	defaultIdleTimeout       = 2 * time.Minute
)

func Handler(cfg adapterconfig.Config) (http.Handler, error) {
	if err := adapterconfig.Validate(cfg); err != nil {
		return nil, err
	}
	r, err := adapterconfig.BuildRouter(cfg)
	if err != nil {
		return nil, err
	}
	cooldown, err := healthCooldown(cfg)
	if err != nil {
		return nil, err
	}
	health := gateway.NewHealthTracker(cooldown)
	mux := http.NewServeMux()
	mux.Handle("/v1/chat/completions", gateway.Handler{
		Endpoint:    chat.Codec{},
		Router:      r,
		Health:      health,
		MaxAttempts: cfg.MaxAttempts,
	})
	mux.Handle("/v1/messages", gateway.Handler{
		Endpoint:    anthropicendpoint.Codec{},
		Router:      r,
		Health:      health,
		MaxAttempts: cfg.MaxAttempts,
	})
	mux.Handle("/v1/responses", gateway.Handler{
		Endpoint:    responsesendpoint.Codec{},
		Router:      r,
		Health:      health,
		MaxAttempts: cfg.MaxAttempts,
	})
	return mux, nil
}

func ListenAndServe(cfg adapterconfig.Config) error {
	handler, err := Handler(cfg)
	if err != nil {
		return err
	}
	log.Printf("llmadapter gateway listening on %s", cfg.Addr)
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		ReadTimeout:       defaultReadTimeout,
		WriteTimeout:      defaultWriteTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func healthCooldown(cfg adapterconfig.Config) (time.Duration, error) {
	if cfg.HealthCooldown == "" {
		return 30 * time.Second, nil
	}
	cooldown, err := time.ParseDuration(cfg.HealthCooldown)
	if err != nil {
		return 0, fmt.Errorf("invalid health_cooldown %q: %w", cfg.HealthCooldown, err)
	}
	return cooldown, nil
}
