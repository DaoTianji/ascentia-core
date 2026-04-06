package thinking

import (
	"strings"
	"testing"
)

func TestProcessChunkSimple(t *testing.T) {
	var th, tx strings.Builder
	h := HandlerFunc{
		T: func(s string) { th.WriteString(s) },
		X: func(s string) { tx.WriteString(s) },
	}
	var st ParserState
	ProcessChunk("Hello <thinking>reasoning here</thinking> world", &st, h)
	Flush(&st, h)
	if th.String() != "reasoning here" {
		t.Fatalf("thinking %q", th.String())
	}
	if tx.String() != "Hello  world" {
		t.Fatalf("text %q", tx.String())
	}
}

func TestProcessChunkSplitAcrossChunks(t *testing.T) {
	var th, tx strings.Builder
	h := HandlerFunc{
		T: func(s string) { th.WriteString(s) },
		X: func(s string) { tx.WriteString(s) },
	}
	var st ParserState
	ProcessChunk("pre <thinking>in", &st, h)
	ProcessChunk("side</thinking> after", &st, h)
	Flush(&st, h)
	if th.String() != "inside" {
		t.Fatalf("thinking %q", th.String())
	}
	if tx.String() != "pre  after" {
		t.Fatalf("text %q", tx.String())
	}
}
