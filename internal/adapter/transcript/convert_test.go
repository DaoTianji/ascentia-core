package transcript

import (
	"testing"

	"ascentia-core/internal/types"
	"ascentia-core/pkg/agent_core/transcript"
)

func TestRoundTripAssistantToolCalls(t *testing.T) {
	orig := types.Message{
		Role:    types.RoleAssistant,
		Content: "hi",
		ToolCalls: []types.ToolCall{{
			ID:   "1",
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      "SpawnPet",
				Arguments: `{"pet_type":"x"}`,
			},
		}},
	}
	tm := FromTypes(orig)
	back := ToTypes(tm)
	if back.Role != orig.Role || back.Content != orig.Content {
		t.Fatalf("role/content %+v vs %+v", back, orig)
	}
	if len(back.ToolCalls) != 1 || back.ToolCalls[0].Function.Name != "SpawnPet" {
		t.Fatalf("toolcalls %+v", back.ToolCalls)
	}
}

func TestToolMessageRoundTrip(t *testing.T) {
	orig := types.Message{
		Role:       types.RoleTool,
		ToolCallID: "call_1",
		Content:    "ok",
	}
	tm := FromTypes(orig)
	if tm.Role != transcript.RoleTool || tm.ToolCallID != "call_1" {
		t.Fatal(tm)
	}
	back := ToTypes(tm)
	if back.ToolCallID != orig.ToolCallID || back.Content != "ok" {
		t.Fatal(back)
	}
}
