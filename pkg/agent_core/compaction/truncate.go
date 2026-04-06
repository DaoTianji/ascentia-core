package compaction

import (
	"unicode/utf8"

	"ascentia-core/pkg/agent_core/transcript"
)

// TruncateToolPruner shortens tool role message bodies; other roles unchanged.
type TruncateToolPruner struct {
	MaxRunes int
	Ellipsis string
}

// Prune implements ToolResultPruner.
func (p TruncateToolPruner) Prune(msg transcript.Message) transcript.Message {
	if msg.Role != transcript.RoleTool {
		return msg
	}
	max := p.MaxRunes
	if max <= 0 {
		max = 20000
	}
	m := msg.Clone()
	if utf8.RuneCountInString(m.Content) <= max {
		return m
	}
	ell := p.Ellipsis
	if ell == "" {
		ell = "\n…(truncated)"
	}
	runes := []rune(m.Content)
	ellR := []rune(ell)
	if max <= len(ellR)+16 {
		m.Content = string(ellR)
		return m
	}
	cut := max - len(ellR)
	if cut < 1 {
		cut = 1
	}
	m.Content = string(runes[:cut]) + string(ellR)
	return m
}
