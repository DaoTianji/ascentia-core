package identity

// MaxTenantFieldLen caps UserID / AgentID length (Redis keys, logs, query params).
const MaxTenantFieldLen = 128

// ValidationError is returned for client-relevant input problems (safe to surface via WS).
type ValidationError struct {
	Msg string
}

func (e ValidationError) Error() string { return e.Msg }

// ClientMessage is shown to end users (English, aligned with other WS protocol errors).
func (e ValidationError) ClientMessage() string {
	if e.Msg != "" {
		return e.Msg
	}
	return "Invalid input."
}

// TenantScope identifies a user and a specific agent instance. All memory and
// tool execution contexts must carry this scope; adapters must enforce isolation.
type TenantScope struct {
	UserID  string
	AgentID string
}

// Validate returns an error if required fields are empty or oversized.
func (s TenantScope) Validate() error {
	if s.UserID == "" {
		return ValidationError{Msg: "User identity (user_id) is required."}
	}
	if len(s.UserID) > MaxTenantFieldLen {
		return ValidationError{Msg: "User identity (user_id) is too long."}
	}
	if s.AgentID == "" {
		return ValidationError{Msg: "Agent identity (agent_id) is required."}
	}
	if len(s.AgentID) > MaxTenantFieldLen {
		return ValidationError{Msg: "Agent identity (agent_id) is too long."}
	}
	return nil
}
