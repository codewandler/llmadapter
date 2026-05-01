package codex

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/codewandler/llmadapter/unified"
)

const quotaUsageEventType = "llmadapter.quota_usage"

func codexQuotaFramesFromHeader(header http.Header) [][]byte {
	event, ok := codexQuotaUsageFromHeader(header)
	if !ok {
		return nil
	}
	frame, err := quotaUsageFrame(event)
	if err != nil {
		return nil
	}
	return [][]byte{frame}
}

func codexQuotaUsageFromHeader(header http.Header) (unified.QuotaUsageEvent, bool) {
	limitIDs := codexQuotaLimitIDs(header)
	if len(limitIDs) == 0 {
		return unified.QuotaUsageEvent{}, false
	}
	rawHeader := http.Header{}
	limits := make([]unified.QuotaLimitUsage, 0, len(limitIDs))
	for _, id := range limitIDs {
		prefix := "x-" + strings.ReplaceAll(id, "_", "-")
		limit := unified.QuotaLimitUsage{
			ID:      id,
			Name:    strings.TrimSpace(headerValue(header, prefix+"-limit-name")),
			Windows: codexQuotaWindowsFromHeader(header, prefix),
		}
		if id == "codex" {
			limit.Credits = codexQuotaCreditsFromHeader(header)
		}
		if len(limit.Windows) > 0 || limit.Name != "" || limit.Credits != nil {
			limits = append(limits, limit)
		}
		captureCodexQuotaRaw(rawHeader, header, prefix+"-limit-name")
		for _, window := range []string{"primary", "secondary"} {
			captureCodexQuotaRaw(rawHeader, header, prefix+"-"+window+"-over-secondary-limit-percent")
			captureCodexQuotaRaw(rawHeader, header, prefix+"-"+window+"-reset-after-seconds")
			captureCodexQuotaRaw(rawHeader, header, prefix+"-"+window+"-used-percent")
			captureCodexQuotaRaw(rawHeader, header, prefix+"-"+window+"-window-minutes")
			captureCodexQuotaRaw(rawHeader, header, prefix+"-"+window+"-reset-at")
		}
	}
	if len(limits) == 0 {
		return unified.QuotaUsageEvent{}, false
	}
	event := unified.QuotaUsageEvent{
		Provider: ProviderName,
		Plan:     strings.TrimSpace(headerValue(header, "x-codex-plan-type")),
		Limits:   limits,
	}
	for _, key := range []string{
		"x-codex-active-limit",
		"x-codex-plan-type",
		"x-codex-credits-has-credits",
		"x-codex-credits-unlimited",
		"x-codex-credits-balance",
	} {
		captureCodexQuotaRaw(rawHeader, header, key)
	}
	if len(rawHeader) > 0 {
		if raw, err := json.Marshal(rawHeader); err == nil {
			event.ProviderRaw = raw
		}
	}
	return event, true
}

func captureCodexQuotaRaw(out http.Header, header http.Header, name string) {
	if value := headerValue(header, name); value != "" {
		out.Set(name, value)
	}
}

func codexQuotaUsageJSONFromRateLimitEvent(raw []byte) []byte {
	event, ok := codexQuotaUsageFromRateLimitEvent(raw)
	if !ok {
		return nil
	}
	frame, err := quotaUsageJSON(event)
	if err != nil {
		return nil
	}
	return frame
}

func codexQuotaUsageFromRateLimitEvent(raw []byte) (unified.QuotaUsageEvent, bool) {
	var payload struct {
		Type             string `json:"type"`
		Plan             string `json:"plan_type"`
		MeteredLimitName string `json:"metered_limit_name"`
		LimitName        string `json:"limit_name"`
		RateLimits       struct {
			Primary   *codexQuotaEventWindow `json:"primary"`
			Secondary *codexQuotaEventWindow `json:"secondary"`
		} `json:"rate_limits"`
		Credits *struct {
			HasCredits bool   `json:"has_credits"`
			Unlimited  bool   `json:"unlimited"`
			Balance    string `json:"balance"`
		} `json:"credits"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Type != "codex.rate_limits" {
		return unified.QuotaUsageEvent{}, false
	}
	var windows []unified.QuotaWindowUsage
	if payload.RateLimits.Primary != nil {
		windows = append(windows, payload.RateLimits.Primary.quotaWindow("primary"))
	}
	if payload.RateLimits.Secondary != nil {
		windows = append(windows, payload.RateLimits.Secondary.quotaWindow("secondary"))
	}
	credits := (*unified.QuotaCredits)(nil)
	if payload.Credits != nil {
		credits = &unified.QuotaCredits{
			HasCredits: boolPtr(payload.Credits.HasCredits),
			Unlimited:  boolPtr(payload.Credits.Unlimited),
			Balance:    payload.Credits.Balance,
		}
	}
	if len(windows) == 0 && credits == nil {
		return unified.QuotaUsageEvent{}, false
	}
	limitID := "codex"
	if payload.MeteredLimitName != "" {
		limitID = normalizeCodexQuotaID(payload.MeteredLimitName)
	} else if payload.LimitName != "" {
		limitID = normalizeCodexQuotaID(payload.LimitName)
	}
	return unified.QuotaUsageEvent{
		Provider: ProviderName,
		Plan:     payload.Plan,
		Limits: []unified.QuotaLimitUsage{{
			ID:      limitID,
			Windows: windows,
			Credits: credits,
		}},
		ProviderRaw: append(json.RawMessage(nil), raw...),
	}, true
}

type codexQuotaEventWindow struct {
	UsedPercent   float64 `json:"used_percent"`
	WindowMinutes *int    `json:"window_minutes"`
	ResetsAtUnix  *int64  `json:"reset_at"`
}

func (w codexQuotaEventWindow) quotaWindow(name string) unified.QuotaWindowUsage {
	return unified.QuotaWindowUsage{
		Name:          name,
		UsedPercent:   w.UsedPercent,
		WindowMinutes: w.WindowMinutes,
		ResetsAtUnix:  w.ResetsAtUnix,
	}
}

func codexQuotaLimitIDs(header http.Header) []string {
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for name := range header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-codex-") && strings.HasSuffix(lower, "-primary-used-percent") {
			prefix := strings.TrimSuffix(lower, "-primary-used-percent")
			add(normalizeCodexQuotaID(strings.TrimPrefix(prefix, "x-")))
		}
	}
	if headerValue(header, "x-codex-secondary-used-percent") != "" ||
		headerValue(header, "x-codex-secondary-window-minutes") != "" ||
		headerValue(header, "x-codex-secondary-reset-at") != "" ||
		codexQuotaCreditsFromHeader(header) != nil {
		add("codex")
	}
	sort.Strings(ids)
	return ids
}

func normalizeCodexQuotaID(value string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "-", "_")
}

func codexQuotaWindowsFromHeader(header http.Header, prefix string) []unified.QuotaWindowUsage {
	var windows []unified.QuotaWindowUsage
	if window, ok := codexQuotaWindowFromHeader(header, "primary", prefix+"-primary"); ok {
		windows = append(windows, window)
	}
	if window, ok := codexQuotaWindowFromHeader(header, "secondary", prefix+"-secondary"); ok {
		windows = append(windows, window)
	}
	return windows
}

func codexQuotaWindowFromHeader(header http.Header, name, prefix string) (unified.QuotaWindowUsage, bool) {
	used, ok := parseHeaderFloat(header, prefix+"-used-percent")
	if !ok {
		return unified.QuotaWindowUsage{}, false
	}
	return unified.QuotaWindowUsage{
		Name:          name,
		UsedPercent:   used,
		WindowMinutes: parseHeaderIntPtr(header, prefix+"-window-minutes"),
		ResetsAtUnix:  parseHeaderInt64Ptr(header, prefix+"-reset-at"),
	}, true
}

func codexQuotaCreditsFromHeader(header http.Header) *unified.QuotaCredits {
	hasCredits, hasHasCredits := parseHeaderBoolPtr(header, "x-codex-credits-has-credits")
	unlimited, hasUnlimited := parseHeaderBoolPtr(header, "x-codex-credits-unlimited")
	balance := strings.TrimSpace(headerValue(header, "x-codex-credits-balance"))
	if !hasHasCredits && !hasUnlimited && balance == "" {
		return nil
	}
	return &unified.QuotaCredits{
		HasCredits: hasCredits,
		Unlimited:  unlimited,
		Balance:    balance,
	}
}

func quotaUsageFrame(event unified.QuotaUsageEvent) ([]byte, error) {
	raw, err := quotaUsageJSON(event)
	if err != nil {
		return nil, err
	}
	return append([]byte("data: "), raw...), nil
}

func quotaUsageJSON(event unified.QuotaUsageEvent) ([]byte, error) {
	payload := struct {
		Type string `json:"type"`
		unified.QuotaUsageEvent
	}{
		Type:            quotaUsageEventType,
		QuotaUsageEvent: event,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func parseHeaderFloat(header http.Header, name string) (float64, bool) {
	value := strings.TrimSpace(headerValue(header, name))
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func parseHeaderIntPtr(header http.Header, name string) *int {
	value := strings.TrimSpace(headerValue(header, name))
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
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

func parseHeaderBoolPtr(header http.Header, name string) (*bool, bool) {
	value := strings.TrimSpace(headerValue(header, name))
	if value == "" {
		return nil, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, false
	}
	return &parsed, true
}

func boolPtr(value bool) *bool {
	return &value
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
