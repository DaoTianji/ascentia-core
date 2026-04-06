package llm

import (
	"context"
	"errors"

	"ascentia-core/pkg/agent_core/tools"
	"ascentia-core/pkg/agent_core/transcript"
)

// ErrStreamingNotSupported is returned by ModelClient.Stream when unsupported.
var ErrStreamingNotSupported = errors.New("llm: streaming not supported")

// CompleteRequest is a single non-streaming model call.
type CompleteRequest struct {
	Messages []transcript.Message
	ToolDefs []tools.Definition
}

// CompleteResponse is the assistant turn from the model.
type CompleteResponse struct {
	Assistant transcript.Message
}

// StreamSink receives demultiplexed stream events (callback style for WebSocket bridges).
type StreamSink interface {
	OnThinking(delta string)
	OnText(delta string)
	OnError(err error)
}

// StreamRequest streams one model response; Sink may receive thinking and text deltas.
type StreamRequest struct {
	Messages []transcript.Message
	ToolDefs []tools.Definition
	Sink     StreamSink
}

// ModelClient talks to the underlying LLM API (implemented outside agent_core).
type ModelClient interface {
	Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
	// Stream is optional; return ErrStreamingNotSupported if not available.
	Stream(ctx context.Context, req StreamRequest) error
}

// StreamTurnCompleter performs one assistant generation with optional streaming text deltas
// (used by loop.RunStreamed between tool rounds).
type StreamTurnCompleter interface {
	CompleteStreamTurn(ctx context.Context, req CompleteRequest, onTextDelta func(string)) (CompleteResponse, error)
}
