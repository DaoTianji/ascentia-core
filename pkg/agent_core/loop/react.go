package loop

import (
	"context"
	"encoding/json"
	"strings"

	"ascentia-core/pkg/agent_core/compaction"
	"ascentia-core/pkg/agent_core/llm"
	"ascentia-core/pkg/agent_core/tools"
	"ascentia-core/pkg/agent_core/transcript"
)

// Options configures the ReAct tool loop.
type Options struct {
	MaxTurns                   int
	MaxConsecutiveToolFailures int
	// ToolPruner rewrites each tool result before it is appended (same as engine PrepareLoop history pruner).
	ToolPruner compaction.ToolResultPruner
}

// Run executes complete → tool results until no tool calls or limits hit.
func Run(ctx context.Context, tcx tools.Context, msgs []transcript.Message, reg *tools.Registry, client llm.ModelClient, opt Options) ([]transcript.Message, string, error) {
	if opt.MaxTurns <= 0 {
		opt.MaxTurns = 32
	}
	if opt.MaxConsecutiveToolFailures <= 0 {
		opt.MaxConsecutiveToolFailures = 3
	}
	out := append([]transcript.Message(nil), msgs...)
	failures := 0

	for turn := 0; turn < opt.MaxTurns; turn++ {
		resp, err := client.Complete(ctx, llm.CompleteRequest{
			Messages: out,
			ToolDefs: reg.Definitions(),
		})
		if err != nil {
			return out, "", err
		}
		am := resp.Assistant
		if am.Role == "" {
			am.Role = transcript.RoleAssistant
		}
		out = append(out, am)
		if len(am.ToolCalls) == 0 {
			return out, strings.TrimSpace(am.Content), nil
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
