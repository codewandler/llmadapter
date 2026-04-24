package messages

import (
	"context"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/pipeline"
	"github.com/codewandler/llmadapter/unified"
)

type AdaptedClient struct {
	native          adapt.NativeClient[MessageRequest, Event]
	codec           Codec
	reqProcs        []adapt.RequestProcessor
	provReqProcs    []adapt.ProviderRequestProcessor[MessageRequest]
	provEvtProcs    []pipeline.Processor[Event]
	unifiedEvtProcs []pipeline.Processor[unified.Event]
}

func (c *AdaptedClient) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	areq := adapt.Request{
		SourceAPI:   adapt.ApiAnthropicMessages,
		Unified:     req,
		MappingMode: adapt.MappingModeBestEffort,
	}
	for _, proc := range c.reqProcs {
		if err := proc.ProcessRequest(ctx, &areq); err != nil {
			return nil, err
		}
	}
	wireReq, err := c.codec.EncodeRequest(ctx, &areq)
	if err != nil {
		return nil, err
	}
	for _, proc := range c.provReqProcs {
		if err := proc.ProcessProviderRequest(ctx, &wireReq); err != nil {
			return nil, err
		}
	}
	providerEvents, err := c.native.Request(ctx, wireReq)
	if err != nil {
		return nil, err
	}

	out := make(chan unified.Event)
	go c.stream(ctx, areq.Warnings, providerEvents, out)
	return out, nil
}

func (c *AdaptedClient) stream(ctx context.Context, warnings []adapt.Warning, in <-chan Event, out chan<- unified.Event) {
	defer close(out)
	providerChain := pipeline.NewChain(c.provEvtProcs...)
	unifiedChain := pipeline.NewChain(c.unifiedEvtProcs...)
	decoder := c.codec.NewEventDecoder()

	for _, warning := range warnings {
		select {
		case <-ctx.Done():
			return
		case out <- unified.WarningEvent{
			Code:    warning.Code,
			Message: warning.Message,
			Source:  string(c.codec.ApiKind()),
			Meta:    warningMeta(warning),
		}:
		}
	}

	emit := func(events []unified.Event) bool {
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return false
			case out <- ev:
			}
		}
		return true
	}
	fail := func(err error) {
		if err == nil {
			return
		}
		select {
		case <-ctx.Done():
		case out <- unified.ErrorEvent{Err: err}:
		}
	}

	for {
		select {
		case <-ctx.Done():
			fail(ctx.Err())
			return
		case ev, ok := <-in:
			if !ok {
				provFlushed, err := providerChain.Close(ctx)
				if err != nil {
					fail(err)
					return
				}
				for _, pev := range provFlushed {
					decoded, err := decoder.Push(ctx, pev)
					if err != nil {
						fail(err)
						return
					}
					processed, err := pushUnified(ctx, unifiedChain, decoded)
					if err != nil {
						fail(err)
						return
					}
					if !emit(processed) {
						return
					}
				}
				decoded, err := decoder.Close(ctx)
				if err != nil {
					fail(err)
					return
				}
				processed, err := pushUnified(ctx, unifiedChain, decoded)
				if err != nil {
					fail(err)
					return
				}
				if !emit(processed) {
					return
				}
				flushed, err := unifiedChain.Close(ctx)
				if err != nil {
					fail(err)
					return
				}
				emit(flushed)
				return
			}
			processedProvider, err := providerChain.Push(ctx, ev)
			if err != nil {
				fail(err)
				return
			}
			for _, pev := range processedProvider {
				decoded, err := decoder.Push(ctx, pev)
				if err != nil {
					fail(err)
					return
				}
				processed, err := pushUnified(ctx, unifiedChain, decoded)
				if err != nil {
					fail(err)
					return
				}
				if !emit(processed) {
					return
				}
			}
		}
	}
}

func warningMeta(warning adapt.Warning) map[string]any {
	if warning.Field == "" {
		return nil
	}
	return map[string]any{"field": warning.Field}
}

func pushUnified(ctx context.Context, chain *pipeline.Chain[unified.Event], events []unified.Event) ([]unified.Event, error) {
	var out []unified.Event
	for _, ev := range events {
		produced, err := chain.Push(ctx, ev)
		if err != nil {
			return nil, err
		}
		out = append(out, produced...)
	}
	return out, nil
}
