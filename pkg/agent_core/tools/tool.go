package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"ascentia-core/pkg/agent_core/identity"
)

// Definition is exposed to the model (OpenAI-style function tool).
type Definition struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Context is passed to every tool execution (tenant + session + allowlist).
type Context struct {
	Scope     identity.TenantScope
	SessionID string
	RequestID string
	// Allowed, if non-nil, must contain the tool name for execution to proceed.
	Allowed map[string]struct{}
}

// Tool is a pure input/output function; integrations implement this interface.
type Tool interface {
	Name() string
	Definition() Definition
	Execute(ctx context.Context, tcx Context, input json.RawMessage) (string, error)
}

// SkillCatalog resolves skill IDs to tool implementations for a request.
type SkillCatalog interface {
	ToolsForSkills(skillIDs []string) ([]Tool, error)
}

func ensureAllowed(tcx Context, name string) error {
	if tcx.Allowed == nil {
		return nil
	}
	if _, ok := tcx.Allowed[name]; !ok {
		return fmt.Errorf("tools: %q not in allowed set for this request", name)
	}
	return nil
}
