package llm

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"ascentia-core/internal/usage"
)

// BuildClientFromEnv 按当前进程环境变量构造 LLM 客户端（与 cmd/ascentia-core 启动逻辑一致）。
// recorder 非 nil 时挂到 Client.Recorder（如 token usage 账本）。
func BuildClientFromEnv(recorder usage.AsyncRecorder) (*Client, error) {
	baseURL := strings.TrimSpace(os.Getenv("ANTHROPIC_BASE_URL"))
	if baseURL == "" {
		return nil, errors.New("ANTHROPIC_BASE_URL is empty")
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = os.Getenv("ANTHROPIC_DEFAULT_SONNET_MODEL")
	}
	if model == "" {
		model = os.Getenv("ANTHROPIC_DEFAULT_HAIKU_MODEL")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("ANTHROPIC_MODEL (or default sonnet/haiku) is empty")
	}
	authToken := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")

	timeoutMs := 300000
	if v := os.Getenv("API_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutMs = n
		}
	}

	c := NewClient(baseURL, authToken, apiKey, model, time.Duration(timeoutMs)*time.Millisecond)
	if recorder != nil {
		c.Recorder = recorder
	}
	return c, nil
}
