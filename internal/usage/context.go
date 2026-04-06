package usage

import "context"

type ctxKey struct{}

// Attribution carries tenant and trace fields for token accounting (from gateway / runtime).
type Attribution struct {
	UserID       string
	AgentID      string
	OperatorRole string
	SessionID    string
	RequestID    string
}

// WithAttribution stores attribution on ctx for the HTTP LLM client to merge into usage rows.
func WithAttribution(ctx context.Context, a Attribution) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// FromContext returns attribution if present.
func FromContext(ctx context.Context) (Attribution, bool) {
	a, ok := ctx.Value(ctxKey{}).(Attribution)
	return a, ok
}
