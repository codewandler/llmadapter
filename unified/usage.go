package unified

import "encoding/json"

type Usage struct {
	InputTokens      int             `json:"input_tokens,omitempty"`
	OutputTokens     int             `json:"output_tokens,omitempty"`
	ReasoningTokens  int             `json:"reasoning_tokens,omitempty"`
	CacheReadTokens  int             `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int             `json:"cache_write_tokens,omitempty"`
	TotalTokens      int             `json:"total_tokens,omitempty"`
	ProviderRaw      json.RawMessage `json:"provider_raw,omitempty"`
}

func usageFromEvent(ev UsageEvent) Usage {
	total := ev.TotalTokens
	if total == 0 {
		total = ev.InputTokens + ev.OutputTokens + ev.ReasoningTokens
	}
	return Usage{
		InputTokens:      ev.InputTokens,
		OutputTokens:     ev.OutputTokens,
		ReasoningTokens:  ev.ReasoningTokens,
		CacheReadTokens:  ev.CacheReadTokens,
		CacheWriteTokens: ev.CacheWriteTokens,
		TotalTokens:      total,
		ProviderRaw:      ev.ProviderRaw,
	}
}
