package memory

import (
	"fmt"
	"time"
)

// AgeDays returns whole days since t (UTC wall clock), floored at 0.
func AgeDays(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	d := time.Since(t.UTC())
	if d < 0 {
		return 0
	}
	return int(d / (24 * time.Hour))
}

// AgeHint is a short Chinese phrase for models (e.g. "3 天前写入").
func AgeHint(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := AgeDays(t)
	switch d {
	case 0:
		return "今天写入"
	case 1:
		return "昨天写入"
	default:
		return fmt.Sprintf("%d 天前写入", d)
	}
}

// FreshnessNote warns when memory is older than ~1 day (aligns with Claude Code drift mindset).
func FreshnessNote(t time.Time) string {
	if t.IsZero() || AgeDays(t) <= 1 {
		return ""
	}
	return "（较旧，请以当前对话与工具结果为准；可能已过时）"
}
