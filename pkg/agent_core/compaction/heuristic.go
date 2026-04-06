package compaction

import (
	"context"
	"fmt"

	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/transcript"
)

// RoughTokenEstimator approximates tokens as rune_count/3 + small per-message overhead (no vectors, no external tokenizer).
func RoughTokenEstimator(msgs []transcript.Message) int {
	n := 0
	for _, m := range msgs {
		n += estimateMsgTokens(m)
	}
	return n
}

func estimateMsgTokens(m transcript.Message) int {
	t := 4 + len(m.Content)/3
	for _, c := range m.ToolCalls {
		t += 8 + len(c.Name)/3 + len(c.Arguments)/3
	}
	return t
}

// ThresholdCompactor drops oldest non-system messages (whole tool rounds) until RoughTokenEstimator is under MaxTokens.
type ThresholdCompactor struct {
	MaxTokens int
	// MinTail is the minimum number of messages to keep between system prefix and the final message (usually the current user turn).
	MinTail int
}

// NewThresholdCompactor returns a compactor with sane defaults when maxTokens <= 0 (48000) or minTail <= 0 (8).
func NewThresholdCompactor(maxTokens, minTail int) *ThresholdCompactor {
	return &ThresholdCompactor{MaxTokens: maxTokens, MinTail: minTail}
}

func (c *ThresholdCompactor) effectiveMax() int {
	if c == nil || c.MaxTokens <= 0 {
		return 48000
	}
	return c.MaxTokens
}

func (c *ThresholdCompactor) effectiveMinTail() int {
	if c == nil || c.MinTail <= 0 {
		return 8
	}
	return c.MinTail
}

// MaybeCompact implements Compactor.
func (c *ThresholdCompactor) MaybeCompact(ctx context.Context, scope identity.TenantScope, sessionID string, msgs []transcript.Message, estTokens int) ([]transcript.Message, Report, error) {
	_ = ctx
	_ = scope
	_ = sessionID
	if c == nil {
		return msgs, Report{}, nil
	}
	max := c.effectiveMax()
	tok := estTokens
	if tok <= 0 {
		tok = RoughTokenEstimator(msgs)
	}
	if tok <= max {
		return msgs, Report{}, nil
	}

	i := 0
	for i < len(msgs) && msgs[i].Role == transcript.RoleSystem {
		i++
	}
	prefix := msgs[:i]
	rest := msgs[i:]
	if len(rest) == 0 {
		return msgs, Report{Applied: false, Reason: "no messages after system prefix"}, nil
	}
	last := rest[len(rest)-1]
	mid := rest[:len(rest)-1]
	minTail := c.effectiveMinTail()

	droppedBlocks := 0
	for RoughTokenEstimator(squash(prefix, mid, last)) > max && len(mid) > minTail {
		n, newMid := consumeBlockFromFront(mid)
		if n == 0 {
			break
		}
		mid = newMid
		droppedBlocks += n
	}
	if droppedBlocks == 0 {
		return msgs, Report{Applied: false, Reason: "over budget but min tail reached"}, nil
	}

	notice := transcript.Message{
		Role:    transcript.RoleSystem,
		Content: fmt.Sprintf("（上下文已压缩：已省略较早的 %d 条消息；请依据下文与工具结果继续。）", droppedBlocks),
	}
	out := make([]transcript.Message, 0, len(prefix)+1+len(mid)+1)
	out = append(out, prefix...)
	out = append(out, notice)
	out = append(out, mid...)
	out = append(out, last)
	return out, Report{Applied: true, Reason: "threshold_drop_oldest"}, nil
}

func squash(prefix, mid []transcript.Message, last transcript.Message) []transcript.Message {
	out := make([]transcript.Message, 0, len(prefix)+len(mid)+1)
	out = append(out, prefix...)
	out = append(out, mid...)
	out = append(out, last)
	return out
}

func consumeBlockFromFront(mid []transcript.Message) (int, []transcript.Message) {
	if len(mid) == 0 {
		return 0, mid
	}
	first := mid[0]
	switch first.Role {
	case transcript.RoleUser:
		return 1, mid[1:]
	case transcript.RoleAssistant:
		if len(first.ToolCalls) == 0 {
			return 1, mid[1:]
		}
		ids := make(map[string]struct{}, len(first.ToolCalls))
		for _, tc := range first.ToolCalls {
			if tc.ID != "" {
				ids[tc.ID] = struct{}{}
			}
		}
		n := 1
		rest := mid[1:]
		for len(rest) > 0 && rest[0].Role == transcript.RoleTool {
			if _, ok := ids[rest[0].ToolCallID]; ok {
				n++
				rest = rest[1:]
				continue
			}
			break
		}
		return n, rest
	case transcript.RoleTool:
		return 1, mid[1:]
	default:
		return 1, mid[1:]
	}
}
