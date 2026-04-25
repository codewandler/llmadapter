package unified

import (
	"bytes"
	"context"
	"encoding/json"
)

type Response struct {
	ID           string         `json:"id,omitempty"`
	Model        string         `json:"model,omitempty"`
	Content      []ContentPart  `json:"content,omitempty"`
	ToolCalls    []ToolCall     `json:"tool_calls,omitempty"`
	FinishReason FinishReason   `json:"finish_reason,omitempty"`
	Usage        Usage          `json:"usage,omitempty"`
	Warnings     []WarningEvent `json:"warnings,omitempty"`
	Raw          []RawEvent     `json:"raw,omitempty"`
}

func Collect(ctx context.Context, events <-chan Event) (Response, error) {
	var resp Response
	texts := map[int]*bytes.Buffer{}
	reasoning := map[int]*bytes.Buffer{}
	reasoningSignatures := map[int]*bytes.Buffer{}
	refusals := map[int]*bytes.Buffer{}
	kinds := map[int]ContentKind{}
	toolIndexes := map[int]int{}
	toolArgs := map[int]*bytes.Buffer{}

	flushBlock := func(index int) {
		switch kinds[index] {
		case ContentKindText:
			if b := texts[index]; b != nil && b.Len() > 0 {
				resp.Content = append(resp.Content, TextPart{Text: b.String()})
			}
		case ContentKindReasoning:
			b := reasoning[index]
			sig := reasoningSignatures[index]
			if (b != nil && b.Len() > 0) || (sig != nil && sig.Len() > 0) {
				part := ReasoningPart{}
				if b != nil {
					part.Text = b.String()
				}
				if sig != nil {
					part.Signature = sig.String()
				}
				resp.Content = append(resp.Content, part)
			}
		case ContentKindRefusal:
			if b := refusals[index]; b != nil && b.Len() > 0 {
				resp.Content = append(resp.Content, RefusalPart{Text: b.String()})
			}
		}
		delete(texts, index)
		delete(reasoning, index)
		delete(reasoningSignatures, index)
		delete(refusals, index)
		delete(kinds, index)
	}

	for {
		select {
		case <-ctx.Done():
			return resp, ctx.Err()
		case ev, ok := <-events:
			if !ok {
				for index := range kinds {
					flushBlock(index)
				}
				return resp, nil
			}

			switch e := ev.(type) {
			case MessageStartEvent:
				resp.ID = e.ID
				resp.Model = e.Model
			case ContentBlockStartEvent:
				kinds[e.Index] = e.Kind
				if e.Kind == ContentKindText {
					texts[e.Index] = &bytes.Buffer{}
				}
				if e.Kind == ContentKindReasoning {
					reasoning[e.Index] = &bytes.Buffer{}
					reasoningSignatures[e.Index] = &bytes.Buffer{}
				}
				if e.Kind == ContentKindRefusal {
					refusals[e.Index] = &bytes.Buffer{}
				}
			case ContentBlockDoneEvent:
				flushBlock(e.Index)
			case TextDeltaEvent:
				if texts[e.Index] == nil {
					texts[e.Index] = &bytes.Buffer{}
					kinds[e.Index] = ContentKindText
				}
				texts[e.Index].WriteString(e.Text)
			case ReasoningDeltaEvent:
				if reasoning[e.Index] == nil {
					reasoning[e.Index] = &bytes.Buffer{}
					kinds[e.Index] = ContentKindReasoning
				}
				if reasoningSignatures[e.Index] == nil {
					reasoningSignatures[e.Index] = &bytes.Buffer{}
				}
				reasoning[e.Index].WriteString(e.Text)
				reasoningSignatures[e.Index].WriteString(e.Signature)
			case RefusalDeltaEvent:
				if refusals[e.Index] == nil {
					refusals[e.Index] = &bytes.Buffer{}
					kinds[e.Index] = ContentKindRefusal
				}
				refusals[e.Index].WriteString(e.Text)
			case ToolCallStartEvent:
				resp.ToolCalls = append(resp.ToolCalls, ToolCall{ID: e.ID, Name: e.Name, Index: e.Index})
				toolIndexes[e.Index] = len(resp.ToolCalls) - 1
				toolArgs[e.Index] = &bytes.Buffer{}
			case ToolCallArgsDeltaEvent:
				if toolArgs[e.Index] == nil {
					toolArgs[e.Index] = &bytes.Buffer{}
				}
				toolArgs[e.Index].WriteString(e.Delta)
			case ToolCallDoneEvent:
				i, ok := toolIndexes[e.Index]
				if !ok {
					resp.ToolCalls = append(resp.ToolCalls, ToolCall{Index: e.Index})
					i = len(resp.ToolCalls) - 1
				}
				resp.ToolCalls[i].ID = firstNonEmpty(e.ID, resp.ToolCalls[i].ID)
				resp.ToolCalls[i].Name = firstNonEmpty(e.Name, resp.ToolCalls[i].Name)
				if len(e.Args) > 0 {
					resp.ToolCalls[i].Arguments = append(json.RawMessage(nil), e.Args...)
				} else if b := toolArgs[e.Index]; b != nil && b.Len() > 0 {
					resp.ToolCalls[i].Arguments = json.RawMessage(b.String())
				}
			case UsageEvent:
				resp.Usage = usageFromEvent(e)
			case CompletedEvent:
				resp.FinishReason = e.FinishReason
				if e.MessageID != "" {
					resp.ID = e.MessageID
				}
			case WarningEvent:
				resp.Warnings = append(resp.Warnings, e)
			case RawEvent:
				resp.Raw = append(resp.Raw, e)
			case ErrorEvent:
				if e.Err != nil {
					return resp, e.Err
				}
			}
		}
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
