package runtime

import "strings"

// ScopedChatSessionKey namespaces STM (e.g. Redis) and PG memory session_id so the same
// client session_id cannot share chat history or LTM rows across agents or users.
func ScopedChatSessionKey(userID, agentID, clientSessionID string) string {
	return strings.Join([]string{userID, agentID, clientSessionID}, "\x1f")
}
