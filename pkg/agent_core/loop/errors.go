package loop

import "errors"

var (
	// ErrCircuitOpen means too many consecutive tool failures.
	ErrCircuitOpen = errors.New("agent_core: tool circuit breaker tripped")

	// ErrMaxTurns means the ReAct loop exceeded MaxTurns.
	ErrMaxTurns = errors.New("agent_core: max tool turns exceeded")
)
