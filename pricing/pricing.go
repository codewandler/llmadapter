package pricing

import (
	"context"

	"github.com/codewandler/llmadapter/unified"
	"github.com/codewandler/modeldb"
)

type Calculator struct {
	catalog     modeldb.Catalog
	serviceID   string
	wireModelID string
}

func NewCalculator(catalog modeldb.Catalog, serviceID, wireModelID string) Calculator {
	return Calculator{catalog: catalog, serviceID: serviceID, wireModelID: wireModelID}
}

func (c Calculator) Costs(tokens unified.TokenItems) unified.CostItems {
	if len(tokens) == 0 {
		return nil
	}
	offering, ok := c.catalog.OfferingByRef(modeldb.OfferingRef{ServiceID: c.serviceID, WireModelID: c.wireModelID})
	if !ok || offering.Pricing == nil {
		return nil
	}
	return Costs(tokens, offering.Pricing)
}

func Costs(tokens unified.TokenItems, p *modeldb.Pricing) unified.CostItems {
	if p == nil {
		return nil
	}
	var costs unified.CostItems
	add := func(kind unified.CostKind, count int, rate float64) {
		if count > 0 && rate > 0 {
			costs = append(costs, unified.CostItem{Kind: kind, Amount: float64(count) * rate / 1_000_000})
		}
	}
	add(unified.CostKindInput, tokens.Count(unified.TokenKindInputNew), p.Input)
	add(unified.CostKindInputCacheRead, tokens.Count(unified.TokenKindInputCacheRead), p.CachedInput)
	add(unified.CostKindInputCacheWrite, tokens.Count(unified.TokenKindInputCacheWrite), p.CacheWrite)
	add(unified.CostKindOutput, tokens.Count(unified.TokenKindOutput), p.Output)
	add(unified.CostKindReasoning, tokens.Count(unified.TokenKindOutputReasoning), reasoningRate(p))
	return costs.NonZero()
}

func reasoningRate(p *modeldb.Pricing) float64 {
	if p.Reasoning > 0 {
		return p.Reasoning
	}
	return p.Output
}

type Processor struct {
	Calculator Calculator
}

func NewProcessor(catalog modeldb.Catalog, serviceID, wireModelID string) *Processor {
	return &Processor{Calculator: NewCalculator(catalog, serviceID, wireModelID)}
}

func (p *Processor) Push(_ context.Context, ev unified.Event) ([]unified.Event, error) {
	usage, ok := ev.(unified.UsageEvent)
	if !ok {
		return []unified.Event{ev}, nil
	}
	if len(usage.Costs) > 0 {
		return []unified.Event{ev}, nil
	}
	if costs := p.Calculator.Costs(usage.Tokens); len(costs) > 0 {
		usage.Costs = costs
	}
	return []unified.Event{usage}, nil
}

func (p *Processor) Close(context.Context) ([]unified.Event, error) {
	return nil, nil
}
