package messages

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/codewandler/llmadapter/unified"
)

type anthropicQuotaHeaderSpec struct {
	name string
	base string
}

var anthropicQuotaHeaderSpecs = []anthropicQuotaHeaderSpec{
	{name: "requests", base: "anthropic-ratelimit-requests"},
	{name: "tokens", base: "anthropic-ratelimit-tokens"},
	{name: "input_tokens", base: "anthropic-ratelimit-input-tokens"},
	{name: "output_tokens", base: "anthropic-ratelimit-output-tokens"},
	{name: "priority_input_tokens", base: "anthropic-priority-input-tokens"},
	{name: "priority_output_tokens", base: "anthropic-priority-output-tokens"},
}

type anthropicUnifiedQuotaHeaderSpec struct {
	name          string
	base          string
	windowMinutes int
}

var anthropicUnifiedQuotaHeaderSpecs = []anthropicUnifiedQuotaHeaderSpec{
	{name: "primary", base: "anthropic-ratelimit-unified-5h", windowMinutes: 300},
	{name: "secondary", base: "anthropic-ratelimit-unified-7d", windowMinutes: 10080},
}

func anthropicQuotaUsageFromHeader(header http.Header, provider string) (QuotaUsageEvent, bool) {
	var windows []QuotaWindowUsage
	raw := http.Header{}
	for _, spec := range anthropicUnifiedQuotaHeaderSpecs {
		utilization := parseHeaderFloatPtr(header, spec.base+"-utilization")
		reset := parseHeaderInt64Ptr(header, spec.base+"-reset")
		status := strings.TrimSpace(headerValue(header, spec.base+"-status"))
		if utilization == nil && reset == nil && status == "" {
			continue
		}
		windows = append(windows, QuotaWindowUsage{
			Name:          spec.name,
			Utilization:   utilization,
			WindowMinutes: intPtr(spec.windowMinutes),
			ResetUnix:     reset,
		})
		captureQuotaRaw(raw, header, spec.base+"-utilization")
		captureQuotaRaw(raw, header, spec.base+"-reset")
		captureQuotaRaw(raw, header, spec.base+"-status")
	}
	for _, spec := range anthropicQuotaHeaderSpecs {
		limit := parseHeaderInt64Ptr(header, spec.base+"-limit")
		remaining := parseHeaderInt64Ptr(header, spec.base+"-remaining")
		reset := strings.TrimSpace(headerValue(header, spec.base+"-reset"))
		if limit == nil && remaining == nil && reset == "" {
			continue
		}
		windows = append(windows, QuotaWindowUsage{
			Name:         spec.name,
			Limit:        limit,
			Remaining:    remaining,
			ResetRFC3339: reset,
		})
		captureQuotaRaw(raw, header, spec.base+"-limit")
		captureQuotaRaw(raw, header, spec.base+"-remaining")
		captureQuotaRaw(raw, header, spec.base+"-reset")
	}
	if len(windows) == 0 {
		return QuotaUsageEvent{}, false
	}
	return QuotaUsageEvent{
		Type:     "anthropic.quota_usage",
		Provider: provider,
		Windows:  windows,
		Raw:      raw,
	}, true
}

func captureQuotaRaw(out http.Header, header http.Header, name string) {
	if value := headerValue(header, name); value != "" {
		out.Set(name, value)
	}
}

func quotaEventToUnified(event QuotaUsageEvent) unified.QuotaUsageEvent {
	windows := make([]unified.QuotaWindowUsage, 0, len(event.Windows))
	for _, window := range event.Windows {
		windows = append(windows, unified.QuotaWindowUsage{
			Name:          window.Name,
			UsedPercent:   quotaUsedPercent(window),
			Limit:         window.Limit,
			Remaining:     window.Remaining,
			WindowMinutes: window.WindowMinutes,
			ResetsAtUnix:  firstInt64Ptr(window.ResetUnix, resetUnix(window.ResetRFC3339)),
		})
	}
	out := unified.QuotaUsageEvent{
		Provider: event.Provider,
		Limits: []unified.QuotaLimitUsage{{
			ID:      "anthropic_rate_limit",
			Windows: windows,
		}},
	}
	if len(event.Raw) > 0 {
		if raw, err := json.Marshal(event.Raw); err == nil {
			out.ProviderRaw = raw
		}
	}
	return out
}

func quotaUsedPercent(window QuotaWindowUsage) float64 {
	if window.Utilization != nil {
		return utilizationPercent(*window.Utilization)
	}
	return usedPercent(window.Limit, window.Remaining)
}

func utilizationPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value <= 1 {
		return value * 100
	}
	if value > 100 {
		return 100
	}
	return value
}

func usedPercent(limit, remaining *int64) float64 {
	if limit == nil || remaining == nil || *limit <= 0 {
		return 0
	}
	used := *limit - *remaining
	if used < 0 {
		used = 0
	}
	if used > *limit {
		used = *limit
	}
	return float64(used) / float64(*limit) * 100
}

func firstInt64Ptr(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func intPtr(value int) *int {
	return &value
}

func resetUnix(value string) *int64 {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	unix := parsed.Unix()
	return &unix
}

func parseHeaderInt64Ptr(header http.Header, name string) *int64 {
	value := strings.TrimSpace(headerValue(header, name))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseHeaderFloatPtr(header http.Header, name string) *float64 {
	value := strings.TrimSpace(headerValue(header, name))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func headerValue(header http.Header, name string) string {
	if value := header.Get(name); value != "" {
		return value
	}
	lower := strings.ToLower(name)
	for key, values := range header {
		if strings.ToLower(key) == lower && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}
