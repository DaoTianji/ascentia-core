package thinking

import (
	"strings"
)

const (
	tagOpen  = "<thinking>"
	tagClose = "</thinking>"
)

// ParserState buffers partial tags across stream chunks.
type ParserState struct {
	inThinking bool
	carry      strings.Builder
}

// StreamHandler receives demuxed thinking vs user-visible text.
type StreamHandler interface {
	OnThinking(s string)
	OnText(s string)
}

// HandlerFunc adapts funcs to StreamHandler.
type HandlerFunc struct {
	T func(s string)
	X func(s string)
}

func (h HandlerFunc) OnThinking(s string) {
	if h.T != nil {
		h.T(s)
	}
}

func (h HandlerFunc) OnText(s string) {
	if h.X != nil {
		h.X(s)
	}
}

// ProcessChunk parses chunk for interleaved <thinking>...</thinking> and emits via h.
func ProcessChunk(chunk string, st *ParserState, h StreamHandler) {
	if st == nil || h == nil {
		return
	}
	full := st.carry.String() + chunk
	st.carry.Reset()
	for {
		if !st.inThinking {
			i := strings.Index(full, tagOpen)
			if i < 0 {
				k := partialSuffix(full, tagOpen)
				if k > 0 {
					st.carry.WriteString(full[len(full)-k:])
					full = full[:len(full)-k]
				}
				if full != "" {
					h.OnText(full)
				}
				return
			}
			if i > 0 {
				h.OnText(full[:i])
			}
			full = full[i+len(tagOpen):]
			st.inThinking = true
			continue
		}
		i := strings.Index(full, tagClose)
		if i < 0 {
			k := partialSuffix(full, tagClose)
			if k > 0 {
				st.carry.WriteString(full[len(full)-k:])
				full = full[:len(full)-k]
			}
			if full != "" {
				h.OnThinking(full)
			}
			return
		}
		if i > 0 {
			h.OnThinking(full[:i])
		}
		full = full[i+len(tagClose):]
		st.inThinking = false
	}
}

// Flush emits any buffered tail after the stream ends.
func Flush(st *ParserState, h StreamHandler) {
	if st == nil || h == nil {
		return
	}
	rest := st.carry.String()
	st.carry.Reset()
	if rest == "" {
		return
	}
	if st.inThinking {
		h.OnThinking(rest)
	} else {
		h.OnText(rest)
	}
}

func partialSuffix(s, tag string) int {
	max := min(len(s), len(tag)-1)
	for k := max; k > 0; k-- {
		suf := s[len(s)-k:]
		if strings.HasPrefix(tag, suf) {
			return k
		}
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
