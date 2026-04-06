package memory

import (
	"testing"
	"time"
)

func TestAgeHint(t *testing.T) {
	if AgeHint(time.Time{}) != "" {
		t.Fatal()
	}
	now := time.Now().UTC()
	if got := AgeHint(now); got != "今天写入" {
		t.Fatalf("%q", got)
	}
}
