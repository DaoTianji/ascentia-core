package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// MockCatalog maps skill IDs to tool slices for tests.
type MockCatalog struct {
	BySkill map[string][]Tool
}

func (m *MockCatalog) ToolsForSkills(skillIDs []string) ([]Tool, error) {
	if m.BySkill == nil {
		return nil, nil
	}
	var out []Tool
	seen := make(map[string]struct{})
	for _, sid := range skillIDs {
		ts, ok := m.BySkill[sid]
		if !ok {
			return nil, fmt.Errorf("tools.MockCatalog: unknown skill %q", sid)
		}
		for _, t := range ts {
			n := t.Name()
			if _, dup := seen[n]; dup {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b Tool) int {
		if a.Name() < b.Name() {
			return -1
		}
		if a.Name() > b.Name() {
			return 1
		}
		return 0
	})
	return out, nil
}

// FuncTool adapts a closure to Tool (for tests).
type FuncTool struct {
	N string
	D Definition
	F func(ctx context.Context, tcx Context, raw json.RawMessage) (string, error)
}

func (f FuncTool) Name() string { return f.N }

func (f FuncTool) Definition() Definition {
	d := f.D
	if d.Type == "" {
		d.Type = "function"
	}
	if d.Name == "" {
		d.Name = f.N
	}
	return d
}

func (f FuncTool) Execute(ctx context.Context, tcx Context, input json.RawMessage) (string, error) {
	return f.F(ctx, tcx, input)
}
