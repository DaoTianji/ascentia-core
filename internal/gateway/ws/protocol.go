package ws

// Protocol: JSON WebSocket framing for any client (web, mobile, CLI, other services).
// All messages MUST follow these JSON structures.
//
// WebSocket URL (optional query at handshake):
//   ?user_id=&agent_id=&persona=&operator_role=&model=
// overrides tenant user, assistant id for LTM/reflection/STM namespacing, persona,
// token-ledger operator_role, and OpenAI-compatible model id for that connection.

// Frontend -> Go
type UserMessage struct {
	Type      string `json:"type"` // "user_message"
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

// Go -> Frontend
type AssistantTextMessage struct {
	Type      string `json:"type"` // "assistant_text"
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

type AssistantStreamMessage struct {
	Type      string `json:"type"` // "assistant_stream"
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	IsFinal   bool   `json:"is_final"`
}

type SystemStatusMessage struct {
	Type      string `json:"type"` // "system_status"
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}
