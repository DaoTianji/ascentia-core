package loop

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"

	"ascentia-core/pkg/agent_core/llm"
	"ascentia-core/pkg/agent_core/tools"
	"ascentia-core/pkg/agent_core/transcript"
)

// StreamHooks are optional UI callbacks between streaming model rounds (e.g. WebSocket framing).
type StreamHooks struct {
	OnTextDelta            func(string)
	AfterAssistantComplete func() // e.g. mark end of one assistant segment before tool execution
	BeforeToolBatch        func(toolNames []string)
	// AfterToolBatch runs after all tools in the batch return, before the next model stream starts.
	// Use this to show "still working" — the gap can be tens of seconds and feels like a hang.
	AfterToolBatch func(toolNames []string)
}

// RunStreamed is like Run but streams assistant text for each model call via streamer.
func RunStreamed(
	ctx context.Context,
	tcx tools.Context,
	msgs []transcript.Message,
	reg *tools.Registry,
	streamer llm.StreamTurnCompleter,
	opt Options,
	hooks StreamHooks,
) ([]transcript.Message, string, error) {
	if opt.MaxTurns <= 0 {
		opt.MaxTurns = 32
	}
	if opt.MaxConsecutiveToolFailures <= 0 {
		opt.MaxConsecutiveToolFailures = 3
	}
	out := append([]transcript.Message(nil), msgs...)
	failures := 0

	for turn := 0; turn < opt.MaxTurns; turn++ {
		// Align with runtime.assistantDeltaIsInvisible: whitespace-only deltas are dropped client-side.
		gotVisibleDelta := false
		var wrapDelta func(string)
		if hooks.OnTextDelta != nil {
			wrapDelta = func(d string) {
				if deltaHasVisibleText(d) {
					gotVisibleDelta = true
				}
				hooks.OnTextDelta(d)
			}
		}

		resp, err := streamer.CompleteStreamTurn(ctx, llm.CompleteRequest{
			Messages: out,
			ToolDefs: reg.Definitions(),
		}, wrapDelta)
		if err != nil {
			return out, "", err
		}
		am := resp.Assistant
		if am.Role == "" {
			am.Role = transcript.RoleAssistant
		}
		// Some OpenAI-compatible backends return assistant text only in the final message object
		// and omit per-chunk deltas; without this the client sees a blank segment after tools.
		if !gotVisibleDelta && strings.TrimSpace(am.Content) != "" {
			if hooks.OnTextDelta != nil {
				hooks.OnTextDelta(am.Content)
			}
		}
		if hooks.AfterAssistantComplete != nil {
			hooks.AfterAssistantComplete()
		}
		out = append(out, am)
		if len(am.ToolCalls) == 0 {
			return out, strings.TrimSpace(am.Content), nil
		}

		toolNames := make([]string, 0, len(am.ToolCalls))
		for _, tc := range am.ToolCalls {
			if tc.Name != "" {
				toolNames = append(toolNames, tc.Name)
			}
		}
		if hooks.BeforeToolBatch != nil && len(toolNames) > 0 {
			hooks.BeforeToolBatch(toolNames)
		}

		batchFailed := false
		for _, tc := range am.ToolCalls {
			raw := json.RawMessage(tc.Arguments)
			result, err := reg.Execute(ctx, tcx, tc.Name, raw)
			if err != nil {
				batchFailed = true
				result = "<tool_use_error>" + err.Error() + "</tool_use_error>"
			}
			tm := transcript.Message{
				Role:       transcript.RoleTool,
				ToolCallID: tc.ID,
				Content:    result,
			}
			if opt.ToolPruner != nil {
				tm = opt.ToolPruner.Prune(tm)
			}
			out = append(out, tm)
		}
		if hooks.AfterToolBatch != nil && len(toolNames) > 0 {
			hooks.AfterToolBatch(toolNames)
		}
		if batchFailed {
			failures++
			if failures >= opt.MaxConsecutiveToolFailures {
				return out, "", ErrCircuitOpen
			}
		} else {
			failures = 0
		}
	}
	return out, "", ErrMaxTurns
}

func deltaHasVisibleText(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\ufeff':
			continue
		}
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}
