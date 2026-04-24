package responses

import (
	"encoding/json"
	"strings"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func encodeRequest(req unified.Request) requestWire {
	out := requestWire{
		Model:           req.Model,
		MaxOutputTokens: req.MaxOutputTokens,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		TopK:            req.TopK,
		Stream:          req.Stream,
		User:            req.User,
	}
	out.Instructions = contentText(instructionParts(req.Instructions))
	for _, msg := range req.Messages {
		item := inputItemWire{Type: "message", Role: string(msg.Role), ID: msg.ID}
		if msg.Role == unified.RoleAssistant {
			item.Status = "completed"
		}
		for _, part := range msg.Content {
			text, ok := part.(unified.TextPart)
			if !ok {
				continue
			}
			partType := "input_text"
			if msg.Role == unified.RoleAssistant {
				partType = "output_text"
			}
			item.Content = append(item.Content, contentPartWire{Type: partType, Text: text.Text})
		}
		if len(item.Content) > 0 {
			out.Input = append(out.Input, item)
		}
	}
	return out
}

func instructionParts(instructions []unified.Instruction) []unified.ContentPart {
	var out []unified.ContentPart
	for _, instruction := range instructions {
		out = append(out, instruction.Content...)
	}
	return out
}

func contentText(parts []unified.ContentPart) string {
	var out []string
	for _, part := range parts {
		if text, ok := part.(unified.TextPart); ok {
			out = append(out, text.Text)
		}
	}
	return strings.Join(out, "\n")
}

type streamDecoder struct {
	id           string
	model        string
	started      bool
	startedBlock bool
}

func (d *streamDecoder) push(raw []byte) ([]unified.Event, error) {
	frame, err := transport.ParseSSEFrame(raw)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(frame.Data))) == 0 || string(frame.Data) == "[DONE]" {
		return nil, nil
	}
	var ev eventWire
	if err := json.Unmarshal(frame.Data, &ev); err != nil {
		return nil, err
	}
	ev.Raw = append(json.RawMessage(nil), frame.Data...)
	if ev.Error != nil {
		return []unified.Event{unified.ErrorEvent{Err: &unified.APIError{Type: ev.Error.Type, Code: ev.Error.Code, Message: ev.Error.Message}}}, nil
	}
	if ev.Response != nil && ev.Response.Error != nil {
		return []unified.Event{unified.ErrorEvent{Err: &unified.APIError{Type: ev.Response.Error.Type, Code: ev.Response.Error.Code, Message: ev.Response.Error.Message}}}, nil
	}
	if ev.Response != nil {
		if ev.Response.ID != "" {
			d.id = ev.Response.ID
		}
		if ev.Response.Model != "" {
			d.model = ev.Response.Model
		}
	}
	if ev.ResponseID != "" {
		d.id = ev.ResponseID
	}

	var out []unified.Event
	switch ev.Type {
	case "response.created":
		out = append(out, d.start()...)
	case "response.content_part.added":
		out = append(out, d.start()...)
		if ev.Part == nil || ev.Part.Type == "output_text" {
			out = append(out, d.startBlock()...)
		}
	case "response.content_part.delta", "response.output_text.delta":
		out = append(out, d.start()...)
		out = append(out, d.startBlock()...)
		out = append(out, unified.TextDeltaEvent{Index: 0, Text: ev.Delta})
	case "response.content_part.done":
		if d.startedBlock {
			out = append(out, unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText})
			d.startedBlock = false
		}
	case "response.done", "response.completed":
		out = append(out, d.done(ev.Response)...)
	default:
		if ev.Response != nil && ev.Response.Status == "completed" {
			out = append(out, d.done(ev.Response)...)
		}
	}
	return out, nil
}

func (d *streamDecoder) start() []unified.Event {
	if d.started {
		return nil
	}
	d.started = true
	return []unified.Event{unified.MessageStartEvent{ID: d.id, Model: d.model, Role: unified.RoleAssistant}}
}

func (d *streamDecoder) startBlock() []unified.Event {
	if d.startedBlock {
		return nil
	}
	d.startedBlock = true
	return []unified.Event{unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}}
}

func (d *streamDecoder) done(resp *responseWire) []unified.Event {
	out := d.start()
	if d.startedBlock {
		out = append(out, unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText})
		d.startedBlock = false
	}
	if resp != nil && resp.Usage != nil {
		out = append(out, unified.UsageEvent{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		})
	}
	out = append(out, unified.CompletedEvent{FinishReason: finishReason(resp), MessageID: d.id})
	out = append(out, unified.MessageDoneEvent{ID: d.id})
	return out
}

func finishReason(resp *responseWire) unified.FinishReason {
	if resp == nil {
		return unified.FinishReasonStop
	}
	switch resp.Status {
	case "completed", "":
		return unified.FinishReasonStop
	case "incomplete":
		return unified.FinishReasonLength
	default:
		return unified.FinishReasonUnknown
	}
}
