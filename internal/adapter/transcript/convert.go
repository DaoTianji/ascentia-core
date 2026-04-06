package transcript

import (
	"ascentia-core/internal/types"
	"ascentia-core/pkg/agent_core/transcript"
)

// MessagesFromTypes converts API-shaped history to agent_core transcript messages.
func MessagesFromTypes(msgs []types.Message) []transcript.Message {
	out := make([]transcript.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, FromTypes(m))
	}
	return out
}

// MessagesToTypes converts transcript messages back for Redis persistence.
func MessagesToTypes(msgs []transcript.Message) []types.Message {
	out := make([]types.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, ToTypes(m))
	}
	return out
}

// FromTypes maps one message.
func FromTypes(m types.Message) transcript.Message {
	tm := transcript.Message{
		Role:       transcript.Role(m.Role),
		Content:    m.Content,
		ToolCallID: m.ToolCallID,
	}
	if len(m.ToolCalls) > 0 {
		tm.ToolCalls = make([]transcript.ToolCall, 0, len(m.ToolCalls))
		for _, c := range m.ToolCalls {
			tm.ToolCalls = append(tm.ToolCalls, transcript.ToolCall{
				ID:        c.ID,
				Type:      c.Type,
				Name:      c.Function.Name,
				Arguments: c.Function.Arguments,
			})
		}
	}
	return tm
}

// ToTypes maps one transcript message to API shape.
func ToTypes(m transcript.Message) types.Message {
	out := types.Message{
		Role:       types.Role(m.Role),
		Content:    m.Content,
		ToolCallID: m.ToolCallID,
	}
	if len(m.ToolCalls) > 0 {
		out.ToolCalls = make([]types.ToolCall, 0, len(m.ToolCalls))
		for _, c := range m.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, types.ToolCall{
				ID:   c.ID,
				Type: c.Type,
				Function: types.ToolCallFunction{
					Name:      c.Name,
					Arguments: c.Arguments,
				},
			})
		}
	}
	return out
}
