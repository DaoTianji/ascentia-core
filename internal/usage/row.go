package usage

// Row is one LLM call worth of token usage (append-only ledger).
type Row struct {
	UserID            string
	AgentID           string
	OperatorRole      string
	SessionID         string
	RequestID         string
	Model             string
	Channel           string // chat_completion | stream_chat | text_only
	PromptTokens      int
	CompletionTokens  int
	TotalTokens       int
	ProviderRequestID string
}

// AsyncRecorder receives usage after successful upstream calls (implementations may buffer or go async).
type AsyncRecorder interface {
	RecordAsync(row Row)
}
