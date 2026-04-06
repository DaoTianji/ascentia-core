package planner

import (
	"sort"
	"strings"
)

// FocusPrompt returns a high-priority system fragment describing the current todo focus.
func FocusPrompt(sessionID string, store Store) string {
	if store == nil {
		return ""
	}
	tasks := store.List(sessionID)
	if len(tasks) == 0 {
		return "## Current focus (todo)\nNo tasks yet. Call update_todo to create a plan and set exactly one item to in_progress before using other tools."
	}
	var inProgress []Task
	var pending []Task
	for _, t := range tasks {
		switch t.Status {
		case StatusInProgress:
			inProgress = append(inProgress, t)
		case StatusPending:
			pending = append(pending, t)
		}
	}
	sort.Slice(inProgress, func(i, j int) bool { return inProgress[i].Order < inProgress[j].Order })
	sort.Slice(pending, func(i, j int) bool { return pending[i].Order < pending[j].Order })

	if len(inProgress) > 0 {
		t := inProgress[0]
		var b strings.Builder
		b.WriteString("## Current focus (todo)\n")
		b.WriteString("You MUST prioritize this item until it is done or explicitly re-planned:\n")
		b.WriteString("- **")
		b.WriteString(t.Title)
		b.WriteString("** (id `")
		b.WriteString(t.ID)
		b.WriteString("`)\n")
		if t.ParentID != "" {
			b.WriteString("- Parent: `")
			b.WriteString(t.ParentID)
			b.WriteString("`\n")
		}
		return b.String()
	}

	return "## Current focus (todo)\nNothing is in_progress. Call update_todo to set one task to status in_progress before calling other tools."
}
