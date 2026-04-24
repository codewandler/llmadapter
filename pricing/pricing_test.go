package pricing

import (
	"context"
	"testing"

	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

func TestCostsDerivesTokenCategoryCosts(t *testing.T) {
	tokens := unified.TokenItems{
		{Kind: unified.TokenKindInputNew, Count: 1000},
		{Kind: unified.TokenKindInputCacheRead, Count: 2000},
		{Kind: unified.TokenKindInputCacheWrite, Count: 3000},
		{Kind: unified.TokenKindOutput, Count: 4000},
		{Kind: unified.TokenKindOutputReasoning, Count: 5000},
	}
	costs := Costs(tokens, &modeldb.Pricing{
		Input:       3.0,
		CachedInput: 0.3,
		CacheWrite:  3.75,
		Output:      15.0,
		Reasoning:   20.0,
	})

	cases := []struct {
		kind unified.CostKind
		want float64
	}{
		{unified.CostKindInput, 1000 * 3.0 / 1_000_000},
		{unified.CostKindInputCacheRead, 2000 * 0.3 / 1_000_000},
		{unified.CostKindInputCacheWrite, 3000 * 3.75 / 1_000_000},
		{unified.CostKindOutput, 4000 * 15.0 / 1_000_000},
		{unified.CostKindReasoning, 5000 * 20.0 / 1_000_000},
	}
	for _, tc := range cases {
		if got := costs.ByKind(tc.kind); got != tc.want {
			t.Fatalf("%s cost = %g, want %g", tc.kind, got, tc.want)
		}
	}
}

func TestCostsUsesOutputRateForReasoningWhenNoReasoningRate(t *testing.T) {
	costs := Costs(unified.TokenItems{
		{Kind: unified.TokenKindOutputReasoning, Count: 1000},
	}, &modeldb.Pricing{Output: 15.0})
	if got, want := costs.ByKind(unified.CostKindReasoning), 1000*15.0/1_000_000; got != want {
		t.Fatalf("reasoning cost = %g, want %g", got, want)
	}
}

func TestCalculatorLooksUpOfferingPricing(t *testing.T) {
	catalog := fixtureCatalog()
	calc := NewCalculator(catalog, "anthropic", "claude-test")
	costs := calc.Costs(unified.TokenItems{{Kind: unified.TokenKindInputNew, Count: 1000}})
	if got, want := costs.ByKind(unified.CostKindInput), 1000*3.0/1_000_000; got != want {
		t.Fatalf("input cost = %g, want %g", got, want)
	}
}

func TestCalculatorMissingPricingReturnsNil(t *testing.T) {
	calc := NewCalculator(fixtureCatalog(), "anthropic", "missing")
	if costs := calc.Costs(unified.TokenItems{{Kind: unified.TokenKindInputNew, Count: 1000}}); len(costs) != 0 {
		t.Fatalf("expected no costs, got %+v", costs)
	}
}

func TestProcessorEnrichesUsageEvents(t *testing.T) {
	proc := NewProcessor(fixtureCatalog(), "anthropic", "claude-test")
	out, err := proc.Push(context.Background(), unified.NewUsageEvent(unified.TokenItems{
		{Kind: unified.TokenKindInputNew, Count: 1000},
		{Kind: unified.TokenKindOutput, Count: 2000},
	}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("events = %d, want 1", len(out))
	}
	usage, ok := out[0].(unified.UsageEvent)
	if !ok {
		t.Fatalf("event = %T, want UsageEvent", out[0])
	}
	if got, want := usage.Costs.ByKind(unified.CostKindOutput), 2000*15.0/1_000_000; got != want {
		t.Fatalf("output cost = %g, want %g", got, want)
	}
}

func TestProcessorLeavesExistingCostsUntouched(t *testing.T) {
	proc := NewProcessor(fixtureCatalog(), "anthropic", "claude-test")
	want := unified.CostItems{{Kind: unified.CostKindInput, Amount: 42}}
	out, err := proc.Push(context.Background(), unified.NewUsageEvent(unified.TokenItems{
		{Kind: unified.TokenKindInputNew, Count: 1000},
	}, want))
	if err != nil {
		t.Fatal(err)
	}
	got := out[0].(unified.UsageEvent).Costs
	if got.ByKind(unified.CostKindInput) != 42 {
		t.Fatalf("costs = %+v, want %+v", got, want)
	}
}

func TestProcessorPassesThroughNonUsageEvents(t *testing.T) {
	proc := NewProcessor(fixtureCatalog(), "anthropic", "claude-test")
	ev := unified.TextDeltaEvent{Index: 0, Text: "hello"}
	out, err := proc.Push(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0] != ev {
		t.Fatalf("events = %+v, want passthrough", out)
	}
}

func fixtureCatalog() modeldb.Catalog {
	catalog := modeldb.NewCatalog()
	ref := modeldb.OfferingRef{ServiceID: "anthropic", WireModelID: "claude-test"}
	catalog.Offerings[ref] = modeldb.Offering{
		ServiceID:   ref.ServiceID,
		WireModelID: ref.WireModelID,
		Pricing: &modeldb.Pricing{
			Input:       3.0,
			CachedInput: 0.3,
			CacheWrite:  3.75,
			Output:      15.0,
		},
	}
	return catalog
}
