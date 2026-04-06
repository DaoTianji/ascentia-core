package llm

import (
	"context"
	"errors"
	"sync"

	"ascentia-core/pkg/agent_core/thinking"
)

// MockClient is a scripted ModelClient for tests.
type MockClient struct {
	mu    sync.Mutex
	Queue []CompleteResponse
	Err   error

	// StreamChunks, if non-empty, Stream sends these as text (after optional demux).
	StreamChunks []string
	StreamErr    error
}

// CompleteStreamTurn emits the full assistant text as one delta then returns the same as Complete.
func (m *MockClient) CompleteStreamTurn(ctx context.Context, req CompleteRequest, onTextDelta func(string)) (CompleteResponse, error) {
	r, err := m.Complete(ctx, req)
	if err != nil {
		return CompleteResponse{}, err
	}
	if onTextDelta != nil && r.Assistant.Content != "" {
		onTextDelta(r.Assistant.Content)
	}
	return r, nil
}

func (m *MockClient) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	_ = ctx
	_ = req
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return CompleteResponse{}, m.Err
	}
	if len(m.Queue) == 0 {
		return CompleteResponse{}, errors.New("llm.MockClient: empty Complete queue")
	}
	r := m.Queue[0]
	m.Queue = m.Queue[1:]
	return r, nil
}

func (m *MockClient) Stream(ctx context.Context, req StreamRequest) error {
	_ = ctx
	_ = req
	m.mu.Lock()
	chunks := m.StreamChunks
	err := m.StreamErr
	m.mu.Unlock()
	if err != nil {
		if req.Sink != nil {
			req.Sink.OnError(err)
		}
		return err
	}
	if len(chunks) == 0 {
		return ErrStreamingNotSupported
	}
	if req.Sink == nil {
		return nil
	}
	var st thinking.ParserState
	h := thinking.HandlerFunc{
		T: req.Sink.OnThinking,
		X: req.Sink.OnText,
	}
	for _, c := range chunks {
		thinking.ProcessChunk(c, &st, h)
	}
	thinking.Flush(&st, h)
	return nil
}
