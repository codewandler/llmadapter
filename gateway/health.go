package gateway

import (
	"sync"
	"time"

	"github.com/codewandler/llmadapter/router"
)

type HealthTracker struct {
	mu       sync.Mutex
	cooldown time.Duration
	failures map[string]time.Time
}

func NewHealthTracker(cooldown time.Duration) *HealthTracker {
	return &HealthTracker{cooldown: cooldown, failures: make(map[string]time.Time)}
}

func (h *HealthTracker) MarkFailure(route router.Route) {
	if h == nil || h.cooldown <= 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failures == nil {
		h.failures = make(map[string]time.Time)
	}
	h.failures[routeKey(route)] = time.Now()
}

func (h *HealthTracker) MarkSuccess(route router.Route) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.failures, routeKey(route))
}

func (h *HealthTracker) unhealthy(route router.Route, now time.Time) bool {
	if h == nil || h.cooldown <= 0 {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	failedAt, ok := h.failures[routeKey(route)]
	if !ok {
		return false
	}
	if now.Sub(failedAt) >= h.cooldown {
		delete(h.failures, routeKey(route))
		return false
	}
	return true
}

func routeKey(route router.Route) string {
	return route.ProviderName + "/" + string(route.TargetAPI)
}
