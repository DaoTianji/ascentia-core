package skills

import (
	"fmt"
	"os"
	"slices"
	"strings"

	pgintegration "ascentia-core/internal/integration/pg"
	"ascentia-core/internal/types"
	"ascentia-core/pkg/agent_core/tools"
)

// DefaultSkillID is the built-in bundle (spawn + memory tools).
const DefaultSkillID = "default"

// Catalog resolves skill IDs to tools for agent_core.
type Catalog struct {
	PG   *pgintegration.MemoryStore
	NATS types.NATSPublisher
}

func (c *Catalog) ToolsForSkills(ids []string) ([]tools.Tool, error) {
	if len(ids) == 0 {
		return c.coreTools(), nil
	}
	var out []tools.Tool
	seen := make(map[string]struct{})
	for _, id := range ids {
		switch id {
		case DefaultSkillID:
			for _, t := range c.coreTools() {
				n := t.Name()
				if _, ok := seen[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				out = append(out, t)
			}
		default:
			return nil, fmt.Errorf("skills: unknown skill id %q", id)
		}
	}
	slices.SortFunc(out, func(a, b tools.Tool) int {
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

func spawnPetDisabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DISABLE_SPAWN_PET")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (c *Catalog) coreTools() []tools.Tool {
	out := []tools.Tool{
		NewReadMemoryTool(c.PG),
		NewWriteMemoryTool(c.PG),
	}
	if !spawnPetDisabled() {
		out = append([]tools.Tool{NewSpawnPetTool(c.NATS)}, out...)
	}
	return out
}
