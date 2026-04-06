package configenv

import (
	"os"
	"strings"
)

// MergeableKeys 允许从控制面（PG 快照 / Redis L2）覆盖进程环境变量的键（不含 DATABASE_URL，避免引导悖论）。
var MergeableKeys = map[string]struct{}{
	"ANTHROPIC_BASE_URL":             {},
	"ANTHROPIC_AUTH_TOKEN":           {},
	"ANTHROPIC_API_KEY":              {},
	"ANTHROPIC_MODEL":                {},
	"ANTHROPIC_DEFAULT_SONNET_MODEL": {},
	"ANTHROPIC_DEFAULT_HAIKU_MODEL":  {},
	"API_TIMEOUT_MS":                 {},
	"PORT":                           {},
	"WS_PATH":                        {},
	"MAX_TURNS":                      {},
	"MAX_TOOL_FAILURE_STREAK":        {},
	"DEFAULT_USER_ID":                {},
	"DEFAULT_AGENT_ID":               {},
	"NATS_URL":                       {},
	"REDIS_URL":                      {},
	"REDIS_ADDR":                     {},
	"REDIS_PASSWORD":                 {},
	"REDIS_DB":                       {},
	"SESSION_MAX_MESSAGES":           {},
	"SESSION_TTL_SECONDS":            {},
	"CLAUDE_CODE_DISABLE_THINKING":   {},
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": {},
	"DISABLE_TELEMETRY":                        {},
	"MEMORY_AUTO_EXTRACT":                      {},
	"MEMORY_EXTRACT_MAX_TOKENS":                {},
	"MEMORY_DREAM":                             {},
	"MEMORY_DREAM_INTERVAL_MINUTES":            {},
	"MEMORY_DREAM_MAX_TOKENS":                  {},
	"CONTEXT_TOKEN_BUDGET":                     {},
	"CONTEXT_COMPACT_MIN_TAIL":                 {},
	"TOOL_RESULT_MAX_RUNES":                    {},
	"AGENT_PERSONA":                            {},
	"RUNTIME_HINT":                             {},
	"WS_ALLOWED_ORIGINS":                       {},
	"WS_STRICT_ORIGIN":                         {},
	"WS_AUTH_MODE":                             {},
	"WS_BEARER_TOKEN":                          {},
	"WS_JWT_HS256_SECRET":                      {},
	"GVA_JWT_SIGNING_KEY":                      {}, // deprecated alias of WS_JWT_HS256_SECRET (L2 compat)
	"WS_ALLOW_QUERY_BEARER":                    {},
	"WS_ALLOW_QUERY_JWT":                       {},
	"WS_MAX_MESSAGES_PER_MINUTE":               {},
	"DISABLE_SPAWN_PET":                        {},
}

// ApplyMap 将 map 中非空白键值按白名单写入 os.Setenv，返回生效条数。
func ApplyMap(m map[string]string) (applied int) {
	if len(m) == 0 {
		return 0
	}
	for k, v := range m {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		if _, ok := MergeableKeys[k]; !ok {
			continue
		}
		_ = os.Setenv(k, v)
		applied++
	}
	return applied
}
