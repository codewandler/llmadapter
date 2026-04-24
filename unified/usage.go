package unified

import "encoding/json"

type TokenKind string

const (
	TokenKindInputNew        TokenKind = "input.new"
	TokenKindInputCacheRead  TokenKind = "input.cache_read"
	TokenKindInputCacheWrite TokenKind = "input.cache_write"
	TokenKindOutput          TokenKind = "output"
	TokenKindOutputReasoning TokenKind = "output.reasoning"
)

type TokenItem struct {
	Kind  TokenKind `json:"kind"`
	Count int       `json:"count"`
}

type TokenItems []TokenItem

func (items TokenItems) Count(kind TokenKind) int {
	total := 0
	for _, item := range items {
		if item.Kind == kind {
			total += item.Count
		}
	}
	return total
}

func (items TokenItems) NonZero() TokenItems {
	out := make(TokenItems, 0, len(items))
	for _, item := range items {
		if item.Count != 0 {
			out = append(out, item)
		}
	}
	return out
}

func (items TokenItems) InputTotal() int {
	return items.Count(TokenKindInputNew) + items.Count(TokenKindInputCacheRead) + items.Count(TokenKindInputCacheWrite)
}

func (items TokenItems) OutputTotal() int {
	return items.Count(TokenKindOutput) + items.Count(TokenKindOutputReasoning)
}

func (items TokenItems) Total() int {
	return items.InputTotal() + items.OutputTotal()
}

type CostKind string

const (
	CostKindInput           CostKind = "input"
	CostKindInputCacheRead  CostKind = "input.cache_read"
	CostKindInputCacheWrite CostKind = "input.cache_write"
	CostKindOutput          CostKind = "output"
	CostKindReasoning       CostKind = "reasoning"
	CostKindImage           CostKind = "image"
	CostKindAudio           CostKind = "audio"
	CostKindVideo           CostKind = "video"
	CostKindRequest         CostKind = "request"
	CostKindWebSearch       CostKind = "web_search"
)

type CostItem struct {
	Kind   CostKind `json:"kind"`
	Amount float64  `json:"amount"`
}

type CostItems []CostItem

func (items CostItems) Total() float64 {
	var total float64
	for _, item := range items {
		total += item.Amount
	}
	return total
}

func (items CostItems) ByKind(kind CostKind) float64 {
	var total float64
	for _, item := range items {
		if item.Kind == kind {
			total += item.Amount
		}
	}
	return total
}

func (items CostItems) NonZero() CostItems {
	out := make(CostItems, 0, len(items))
	for _, item := range items {
		if item.Amount != 0 {
			out = append(out, item)
		}
	}
	return out
}

type Usage struct {
	Tokens      TokenItems      `json:"tokens,omitempty"`
	Costs       CostItems       `json:"costs,omitempty"`
	ProviderRaw json.RawMessage `json:"provider_raw,omitempty"`
}

func usageFromEvent(ev UsageEvent) Usage {
	return ev.Usage()
}

func NewUsageEvent(tokens TokenItems, costs CostItems) UsageEvent {
	return UsageEvent{Tokens: tokens.NonZero(), Costs: costs.NonZero()}
}

func (ev UsageEvent) Usage() Usage {
	return Usage{
		Tokens:      ev.Tokens.NonZero(),
		Costs:       ev.Costs.NonZero(),
		ProviderRaw: ev.ProviderRaw,
	}
}

func (u Usage) InputTokens() int {
	return u.Tokens.InputTotal()
}

func (u Usage) InputNewTokens() int {
	return u.Tokens.Count(TokenKindInputNew)
}

func (u Usage) OutputTokens() int {
	return u.Tokens.OutputTotal()
}

func (u Usage) ReasoningTokens() int {
	return u.Tokens.Count(TokenKindOutputReasoning)
}

func (u Usage) CacheReadTokens() int {
	return u.Tokens.Count(TokenKindInputCacheRead)
}

func (u Usage) CacheWriteTokens() int {
	return u.Tokens.Count(TokenKindInputCacheWrite)
}

func (u Usage) TotalTokens() int {
	return u.Tokens.Total()
}

func (u Usage) HasTokens() bool {
	return u.TotalTokens() != 0
}
