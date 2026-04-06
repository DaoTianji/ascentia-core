package transcript

// Role matches chat-completions style roles.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ToolCall is emitted by the assistant when requesting tool execution.
type ToolCall struct {
	ID        string
	Type      string // "function"
	Name      string
	Arguments string // JSON object as string
}

// Message is a neutral transcript row for compaction, reflection, and LLM adapters.
type Message struct {
	Role    Role
	Content string

	ToolCalls []ToolCall

	ToolCallID string
}

// Clone returns a shallow copy (ToolCalls slice is copied).
func (m Message) Clone() Message {
	out := m
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = append([]ToolCall(nil), m.ToolCalls...)
	}
	return out
}
