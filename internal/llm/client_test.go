package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ascentia-core/internal/types"
)

func TestStreamChat_OpenAI_TextAndToolCalls(t *testing.T) {
	// Simulate OpenAI SSE stream with:
	// - text deltas
	// - tool_calls arguments split across chunks
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)

		events := []string{
			`data: {"choices":[{"delta":{"content":"你好，"}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"content":"我来生成一只猫。"}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"SpawnPet","arguments":"{\"pet_type\":\"cat\","}}]}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"x\":100,\"y\":200}"}}]}}]}` + "\n\n",
			`data: [DONE]` + "\n\n",
		}
		for _, e := range events {
			_, _ = w.Write([]byte(e))
		}
	})

	srv := httptest.NewServer(handler)
	defer srv.Close()

	c := NewClient(srv.URL, "", "test-key", "gpt-test", 5*time.Second)

	var deltas []string
	onDelta := func(s string) { deltas = append(deltas, s) }

	msgs := []types.Message{
		{Role: types.RoleSystem, Content: "sys"},
		{Role: types.RoleUser, Content: "生成一只猫"},
	}

	tools := []types.ToolDefinition{
		{
			Type:        "custom",
			Name:        "SpawnPet",
			Description: "Spawn a pet",
			InputSchema: map[string]any{"type": "object"},
		},
	}

	assistantMsg, toolUses, err := c.StreamChat(context.Background(), msgs, tools, onDelta)
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}

	if got := assistantMsg.Content; !strings.Contains(got, "你好") {
		t.Fatalf("assistant content mismatch: %q", got)
	}
	if len(deltas) == 0 {
		t.Fatalf("expected text deltas")
	}
	if len(toolUses) != 1 {
		t.Fatalf("expected 1 tool use, got %d", len(toolUses))
	}
	if toolUses[0].Name != "SpawnPet" {
		t.Fatalf("tool name mismatch: %s", toolUses[0].Name)
	}
	if fmt.Sprint(toolUses[0].Input["pet_type"]) != "cat" {
		t.Fatalf("pet_type mismatch: %#v", toolUses[0].Input)
	}
}
