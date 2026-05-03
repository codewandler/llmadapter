package converse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

const defaultAPIKind = adapt.ApiBedrockConverse

type converseStreamClient interface {
	ConverseStream(context.Context, *bedrockruntime.ConverseStreamInput, ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseStreamOutput, error)
}

func (c *Client) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	runtimeClient, err := c.runtimeClient(ctx)
	if err != nil {
		return nil, err
	}
	resolvedModel, err := c.resolveModel(req.Model)
	if err != nil {
		return nil, err
	}
	built, err := c.buildRequest(req, resolvedModel)
	if err != nil {
		return nil, err
	}
	output, err := runtimeClient.ConverseStream(ctx, built.input)
	if err != nil {
		return nil, normalizeError(err)
	}
	out := make(chan unified.Event)
	go c.readStream(ctx, output, built.warnings, stripInferenceProfilePrefix(resolvedModel), out)
	return out, nil
}

func (c *Client) readStream(ctx context.Context, output *bedrockruntime.ConverseStreamOutput, warnings []unified.WarningEvent, model string, out chan<- unified.Event) {
	defer close(out)
	stream := output.GetStream()
	defer stream.Close()

	for _, warning := range warnings {
		if !sendEvent(ctx, out, warning) {
			return
		}
	}

	type blockState struct {
		kind   unified.ContentKind
		id     string
		name   string
		args   strings.Builder
		sigBuf strings.Builder
	}
	blocks := map[int]*blockState{}
	started := false
	var stop unified.FinishReason

	emitStart := func(role unified.Role) bool {
		if started {
			return true
		}
		started = true
		return sendEvent(ctx, out, unified.MessageStartEvent{Model: model, Role: role, Time: time.Now()})
	}
	ensureContent := func(index int, kind unified.ContentKind) bool {
		state := blocks[index]
		if state == nil {
			state = &blockState{kind: kind}
			blocks[index] = state
		}
		if state.kind == "" {
			state.kind = kind
		}
		return sendEvent(ctx, out, unified.ContentBlockStartEvent{Index: index, Kind: state.kind, ID: state.id, Name: state.name})
	}

	for event := range stream.Events() {
		switch e := event.(type) {
		case *types.ConverseStreamOutputMemberMessageStart:
			role := unified.RoleAssistant
			if e.Value.Role == types.ConversationRoleUser {
				role = unified.RoleUser
			}
			if !emitStart(role) {
				return
			}
		case *types.ConverseStreamOutputMemberContentBlockStart:
			if !emitStart(unified.RoleAssistant) {
				return
			}
			index := int(aws.ToInt32(e.Value.ContentBlockIndex))
			if start, ok := e.Value.Start.(*types.ContentBlockStartMemberToolUse); ok {
				state := &blockState{kind: unified.ContentKindToolCall, id: aws.ToString(start.Value.ToolUseId), name: aws.ToString(start.Value.Name)}
				blocks[index] = state
				if !sendEvent(ctx, out, unified.ToolCallStartEvent{Index: index, ID: state.id, Name: state.name}) {
					return
				}
			}
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if !emitStart(unified.RoleAssistant) {
				return
			}
			index := int(aws.ToInt32(e.Value.ContentBlockIndex))
			switch delta := e.Value.Delta.(type) {
			case *types.ContentBlockDeltaMemberText:
				if blocks[index] == nil {
					blocks[index] = &blockState{kind: unified.ContentKindText}
					if !ensureContent(index, unified.ContentKindText) {
						return
					}
				}
				if !sendEvent(ctx, out, unified.TextDeltaEvent{Index: index, Text: delta.Value}) {
					return
				}
			case *types.ContentBlockDeltaMemberToolUse:
				state := blocks[index]
				if state == nil {
					state = &blockState{kind: unified.ContentKindToolCall}
					blocks[index] = state
				}
				if delta.Value.Input != nil {
					state.args.WriteString(*delta.Value.Input)
					if !sendEvent(ctx, out, unified.ToolCallArgsDeltaEvent{Index: index, ID: state.id, Delta: *delta.Value.Input}) {
						return
					}
				}
			case *types.ContentBlockDeltaMemberReasoningContent:
				if blocks[index] == nil {
					blocks[index] = &blockState{kind: unified.ContentKindReasoning}
					if !ensureContent(index, unified.ContentKindReasoning) {
						return
					}
				}
				switch reasoning := delta.Value.(type) {
				case *types.ReasoningContentBlockDeltaMemberText:
					if !sendEvent(ctx, out, unified.ReasoningDeltaEvent{Index: index, Text: reasoning.Value}) {
						return
					}
				case *types.ReasoningContentBlockDeltaMemberSignature:
					blocks[index].sigBuf.WriteString(reasoning.Value)
					if !sendEvent(ctx, out, unified.ReasoningDeltaEvent{Index: index, Signature: reasoning.Value}) {
						return
					}
				}
			}
		case *types.ConverseStreamOutputMemberContentBlockStop:
			if !emitStart(unified.RoleAssistant) {
				return
			}
			index := int(aws.ToInt32(e.Value.ContentBlockIndex))
			state := blocks[index]
			if state == nil {
				continue
			}
			if state.kind == unified.ContentKindToolCall {
				args := json.RawMessage(`{}`)
				if strings.TrimSpace(state.args.String()) != "" {
					args = json.RawMessage(state.args.String())
				}
				if !sendEvent(ctx, out, unified.ToolCallDoneEvent{Index: index, ID: state.id, Name: state.name, Args: args}) {
					return
				}
				continue
			}
			if !sendEvent(ctx, out, unified.ContentBlockDoneEvent{Index: index, Kind: state.kind}) {
				return
			}
		case *types.ConverseStreamOutputMemberMessageStop:
			stop = finishReason(e.Value.StopReason)
		case *types.ConverseStreamOutputMemberMetadata:
			raw, _ := json.Marshal(e.Value.Usage)
			if e.Value.Usage != nil {
				usage := unified.TokenItems{
					{Kind: unified.TokenKindInputNew, Count: int(aws.ToInt32(e.Value.Usage.InputTokens))},
					{Kind: unified.TokenKindInputCacheRead, Count: int(aws.ToInt32(e.Value.Usage.CacheReadInputTokens))},
					{Kind: unified.TokenKindInputCacheWrite, Count: int(aws.ToInt32(e.Value.Usage.CacheWriteInputTokens))},
					{Kind: unified.TokenKindOutput, Count: int(aws.ToInt32(e.Value.Usage.OutputTokens))},
				}.NonZero()
				if !sendEvent(ctx, out, unified.UsageEvent{Tokens: usage, ProviderRaw: raw}) {
					return
				}
			}
			if stop == "" {
				stop = unified.FinishReasonStop
			}
			sendEvent(ctx, out, unified.MessageDoneEvent{})
			sendEvent(ctx, out, unified.CompletedEvent{FinishReason: stop})
			return
		default:
			raw, _ := json.Marshal(event)
			if !sendEvent(ctx, out, unified.RawEvent{APIKind: string(defaultAPIKind), Type: fmt.Sprintf("%T", event), JSON: raw, Value: event}) {
				return
			}
		}
	}
	if err := stream.Err(); err != nil && !errors.Is(err, io.EOF) {
		sendEvent(ctx, out, unified.ErrorEvent{Err: normalizeError(err)})
	}
}

func finishReason(reason types.StopReason) unified.FinishReason {
	switch reason {
	case types.StopReasonEndTurn:
		return unified.FinishReasonStop
	case types.StopReasonToolUse:
		return unified.FinishReasonToolCall
	case types.StopReasonMaxTokens:
		return unified.FinishReasonLength
	case types.StopReasonContentFiltered:
		return unified.FinishReasonError
	default:
		return unified.FinishReasonUnknown
	}
}

func sendEvent(ctx context.Context, out chan<- unified.Event, ev unified.Event) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- ev:
		return true
	}
}

func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	return &unified.APIError{Message: err.Error()}
}
