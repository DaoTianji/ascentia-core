package ws

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"ascentia-core/internal/runtime"
)

// Tunable input limits (cost / Redis key / log safety).
const (
	maxUserIDLen        = 128
	maxAgentIDLen       = 128
	maxSessionIDLen     = 128
	maxPersonaRunes     = 48000
	maxModelLen         = 256
	maxOperatorRoleLen  = 128
	maxUserMessageRunes = 128000
)

func validateSessionID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("empty session_id")
	}
	if len(id) > maxSessionIDLen {
		return fmt.Errorf("session_id too long")
	}
	for _, r := range id {
		if r == '\x1f' || r == '\n' || r == '\r' {
			return fmt.Errorf("session_id contains illegal character")
		}
	}
	return nil
}

func validateUserMessageText(text string) error {
	if utf8.RuneCountInString(text) > maxUserMessageRunes {
		return fmt.Errorf("message text too long")
	}
	return nil
}

// validateStreamConn checks query-sized fields before LLM / storage.
func validateStreamConn(c runtime.StreamConn) error {
	if err := validateOptionalQueryField("user_id", c.UserID, maxUserIDLen); err != nil {
		return err
	}
	if err := validateOptionalQueryField("agent_id", c.AgentID, maxAgentIDLen); err != nil {
		return err
	}
	if err := validateOptionalQueryField("operator_role", c.OperatorRole, maxOperatorRoleLen); err != nil {
		return err
	}
	if err := validateOptionalQueryField("model", c.Model, maxModelLen); err != nil {
		return err
	}
	if c.Persona != "" && utf8.RuneCountInString(c.Persona) > maxPersonaRunes {
		return fmt.Errorf("persona too long")
	}
	return nil
}

func validateOptionalQueryField(name, v string, maxBytes int) error {
	if v == "" {
		return nil
	}
	if len(v) > maxBytes {
		return fmt.Errorf("%s too long", name)
	}
	return nil
}
