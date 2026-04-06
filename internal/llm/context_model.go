package llm

import (
	"context"
	"strings"
)

type chatModelCtxKey struct{}

// ContextWithChatModel 为单次 Chat / Stream 请求注入 OpenAI-compatible 的 model 名（覆盖 Client.Model）。
func ContextWithChatModel(ctx context.Context, model string) context.Context {
	m := strings.TrimSpace(model)
	if m == "" {
		return ctx
	}
	return context.WithValue(ctx, chatModelCtxKey{}, m)
}

func chatModelFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(chatModelCtxKey{}).(string)
	return strings.TrimSpace(s)
}
