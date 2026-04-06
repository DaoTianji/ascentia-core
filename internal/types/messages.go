package types

// Role matches OpenAI Chat Completions "messages[].role".
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role Role `json:"role"`

	// Content is used by system/user/assistant/tool messages.
	// For assistant messages that only contain tool_calls, this may be empty.
	Content string `json:"content,omitempty"`

	// ToolCalls are emitted by assistant when it wants the server to call tools.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID is used by role=tool messages to associate result with a prior tool call.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolCall matches OpenAI Chat Completions tool_calls.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"` // "function"
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolUse is a server-side extracted tool invocation for execution.
type ToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}
