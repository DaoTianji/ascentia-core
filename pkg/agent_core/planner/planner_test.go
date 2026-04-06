package planner

import (
	"strings"
	"testing"
)

func TestMemoryStoreReplaceAndMarkDone(t *testing.T) {
	s := NewMemoryStore()
	s.ReplaceTasks("sess1", []Task{
		{ID: "a", Title: "first", Status: StatusInProgress, Order: 0},
		{ID: "b", Title: "second", Status: StatusPending, Order: 1},
	})
	list := s.List("sess1")
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	if err := s.MarkDone("sess1", "a"); err != nil {
		t.Fatal(err)
	}
	list = s.List("sess1")
	if list[0].Status != StatusDone {
		t.Fatalf("got %s", list[0].Status)
	}
}

func TestFocusPromptInProgress(t *testing.T) {
	s := NewMemoryStore()
	s.ReplaceTasks("s", []Task{{ID: "1", Title: "Do thing", Status: StatusInProgress, Order: 0}})
	p := FocusPrompt("s", s)
	if p == "" || !strings.Contains(p, "Do thing") || !strings.Contains(p, "MUST prioritize") {
		t.Fatalf("unexpected focus: %q", p)
	}
}

func TestFocusPromptNoInProgress(t *testing.T) {
	s := NewMemoryStore()
	s.ReplaceTasks("s", []Task{{ID: "1", Title: "x", Status: StatusPending, Order: 0}})
	p := FocusPrompt("s", s)
	if !strings.Contains(p, "Nothing is in_progress") {
		t.Fatalf("unexpected: %q", p)
	}
}
