package unified

import "testing"

func TestTokenItemsTotals(t *testing.T) {
	tokens := TokenItems{
		{Kind: TokenKindInputNew, Count: 10},
		{Kind: TokenKindInputCacheRead, Count: 3},
		{Kind: TokenKindInputCacheWrite, Count: 2},
		{Kind: TokenKindOutput, Count: 7},
		{Kind: TokenKindOutputReasoning, Count: 4},
	}
	if got, want := tokens.InputTotal(), 15; got != want {
		t.Fatalf("input total = %d, want %d", got, want)
	}
	if got, want := tokens.OutputTotal(), 11; got != want {
		t.Fatalf("output total = %d, want %d", got, want)
	}
	if got, want := tokens.Total(), 26; got != want {
		t.Fatalf("total = %d, want %d", got, want)
	}
}

func TestUsageEventDerivesFlatCountersFromTokens(t *testing.T) {
	usage := UsageEvent{Tokens: TokenItems{
		{Kind: TokenKindInputNew, Count: 8},
		{Kind: TokenKindInputCacheRead, Count: 3},
		{Kind: TokenKindInputCacheWrite, Count: 2},
		{Kind: TokenKindOutput, Count: 7},
		{Kind: TokenKindOutputReasoning, Count: 4},
	}}.Usage()
	if got, want := usage.InputTokens(), 13; got != want {
		t.Fatalf("input tokens = %d, want %d", got, want)
	}
	if got, want := usage.OutputTokens(), 11; got != want {
		t.Fatalf("output tokens = %d, want %d", got, want)
	}
	if got, want := usage.ReasoningTokens(), 4; got != want {
		t.Fatalf("reasoning tokens = %d, want %d", got, want)
	}
	if got, want := usage.TotalTokens(), 24; got != want {
		t.Fatalf("total tokens = %d, want %d", got, want)
	}
}

func TestCostItemsTotals(t *testing.T) {
	costs := CostItems{
		{Kind: CostKindInput, Amount: 0.01},
		{Kind: CostKindInput, Amount: 0.02},
		{Kind: CostKindOutput, Amount: 0.03},
	}
	if got, want := costs.ByKind(CostKindInput), 0.03; got != want {
		t.Fatalf("input cost = %g, want %g", got, want)
	}
	if got, want := costs.Total(), 0.06; got != want {
		t.Fatalf("total cost = %g, want %g", got, want)
	}
}
