package runtime

// StreamConn carries per-connection overrides (e.g. WebSocket handshake query params).
// Zero value means: use Service.Scope.UserID and Service.DefaultPersona as configured at boot.
type StreamConn struct {
	UserID       string
	AgentID      string // assistant logical id（WS query: agent_id），与 LTM / 反思租户一致
	Persona      string
	OperatorRole string // optional caller role label for usage / audit (e.g. RBAC name from your IdP)
	Model        string // optional OpenAI-compatible model id（WS query: model）
}
