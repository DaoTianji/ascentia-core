package ws

import (
	"errors"

	"ascentia-core/pkg/agent_core/identity"
)

// clientVisibleChatError returns a safe message for WebSocket assistant_text (no stack / upstream details).
func clientVisibleChatError(err error) string {
	if err == nil {
		return ""
	}
	var ve identity.ValidationError
	if errors.As(err, &ve) {
		return ve.ClientMessage()
	}
	return "Request failed. Please try again later."
}
