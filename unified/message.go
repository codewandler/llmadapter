package unified

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

type Message struct {
	Role        Role           `json:"role"`
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Content     []ContentPart  `json:"content,omitempty"`
	ToolCalls   []ToolCall     `json:"tool_calls,omitempty"`
	ToolResults []ToolResult   `json:"tool_results,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

// AssistantMessageFromResponse returns the safe replay shape for an assistant
// response, including text/reasoning content and tool calls.
func AssistantMessageFromResponse(resp Response) Message {
	return Message{
		Role:      RoleAssistant,
		ID:        resp.ID,
		Content:   append([]ContentPart(nil), resp.Content...),
		ToolCalls: cloneToolCalls(resp.ToolCalls),
	}
}

func cloneToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ToolCall, len(calls))
	for i, call := range calls {
		out[i] = call
		if call.Arguments != nil {
			out[i].Arguments = append([]byte(nil), call.Arguments...)
		}
	}
	return out
}

type InstructionKind string

const (
	InstructionSystem    InstructionKind = "system"
	InstructionDeveloper InstructionKind = "developer"
	InstructionPolicy    InstructionKind = "policy"
)

type Instruction struct {
	Kind    InstructionKind `json:"kind"`
	Content []ContentPart   `json:"content,omitempty"`
	Name    string          `json:"name,omitempty"`
	Meta    map[string]any  `json:"meta,omitempty"`
}
