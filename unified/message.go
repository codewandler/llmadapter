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
