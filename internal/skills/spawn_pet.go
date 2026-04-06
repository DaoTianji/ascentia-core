package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"ascentia-core/internal/types"
	"ascentia-core/pkg/agent_core/tools"
)

// SpawnPetTool publishes sandbox spawn commands over NATS when configured.
type SpawnPetTool struct {
	NATS types.NATSPublisher
}

func NewSpawnPetTool(n types.NATSPublisher) tools.Tool {
	return &SpawnPetTool{NATS: n}
}

func (SpawnPetTool) Name() string { return "SpawnPet" }

func (t SpawnPetTool) Definition() tools.Definition {
	return tools.Definition{
		Type:        "function",
		Name:        "SpawnPet",
		Description: "Spawn a pet in the sandbox at coordinates (x,y).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pet_type": map[string]any{"type": "string", "description": "Pet type (e.g. cat, slime)."},
				"x":        map[string]any{"type": "number", "description": "X coordinate."},
				"y":        map[string]any{"type": "number", "description": "Y coordinate."},
				"pet_id":   map[string]any{"type": "string", "description": "Optional stable id."},
			},
			"required": []string{"pet_type", "x", "y"},
		},
	}
}

func (t *SpawnPetTool) Execute(ctx context.Context, tcx tools.Context, input json.RawMessage) (string, error) {
	_ = ctx
	var in struct {
		PetType string  `json:"pet_type"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		PetID   string  `json:"pet_id,omitempty"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("SpawnPet: %w", err)
	}
	if in.PetType == "" {
		return "", fmt.Errorf("pet_type required")
	}

	if t.NATS != nil {
		payload := map[string]any{
			"action": "spawn_pet",
			"type":   in.PetType,
			"x":      in.X,
			"y":      in.Y,
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		log.Printf("[tool:SpawnPet] publish world.admin.command: %s", string(b))
		if err := t.NATS.Publish("world.admin.command", b); err != nil {
			return "", fmt.Errorf("nats publish: %w", err)
		}
	}

	if in.PetID != "" {
		return fmt.Sprintf("Spawned pet: %s (id=%s) at (%.3f, %.3f).", in.PetType, in.PetID, in.X, in.Y), nil
	}
	return fmt.Sprintf("Spawned pet: %s at (%.3f, %.3f).", in.PetType, in.X, in.Y), nil
}
