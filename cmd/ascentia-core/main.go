package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"

	adapterllm "ascentia-core/internal/adapter/llm"
	"ascentia-core/internal/configbus"
	"ascentia-core/internal/gateway/ws"
	natsintegration "ascentia-core/internal/integration/nats"
	pgintegration "ascentia-core/internal/integration/pg"
	httpllm "ascentia-core/internal/llm"
	"ascentia-core/internal/memorywork"
	"ascentia-core/internal/runtime"
	"ascentia-core/internal/session"
	"ascentia-core/internal/types"
	"ascentia-core/pkg/agent_core"
	"ascentia-core/pkg/agent_core/compaction"
	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/planner"
	"ascentia-core/pkg/agent_core/reflection"
)

func envDisabled(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "0" || v == "false" || v == "off" || v == "no"
}

func envEnabled(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	// 可选联调捷径（非目标架构）：仅当 Core 的 DATABASE_URL 所连库上存在 sys_core_settings 时，
	// 用表中非空项覆盖白名单环境变量。生产形态应为 Admin→PG(控制面库)→Redis/NATS→Core 读缓存或调 Admin API，
	// 不要求 Admin 与 Core 共库。见 CORE_CONFIG_FROM_PG。
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" && strings.TrimSpace(os.Getenv("CORE_CONFIG_FROM_PG")) == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Printf("[config] CORE_CONFIG_FROM_PG: pool failed: %v", err)
		} else {
			n, _ := pgintegration.MergeCoreSettingsEnv(ctx, pool)
			if n > 0 {
				log.Printf("[config] merged %d keys from sys_core_settings", n)
			}
			pool.Close()
		}
		cancel()
	}

	// L2：与控制面分库时，由 Admin 写 Redis 快照 + NATS 广播；启动时可选先合并一轮（需 REDIS_URL）
	if strings.TrimSpace(os.Getenv("CORE_CONFIG_BUS")) == "1" {
		if redisURL := strings.TrimSpace(os.Getenv("REDIS_URL")); redisURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			opt, err := redis.ParseURL(redisURL)
			if err != nil {
				log.Printf("[configbus] bootstrap: parse REDIS_URL: %v", err)
			} else {
				rdb := redis.NewClient(opt)
				n, mErr := configbus.MergeFromRedis(ctx, rdb)
				_ = rdb.Close()
				if mErr != nil {
					log.Printf("[configbus] bootstrap redis: %v", mErr)
				} else if n > 0 {
					log.Printf("[configbus] merged %d keys from Redis (L2)", n)
				}
			}
			cancel()
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	wsPath := os.Getenv("WS_PATH")
	if wsPath == "" {
		wsPath = "/ws"
	}

	maxTurns := 8
	if v := os.Getenv("MAX_TURNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxTurns = n
		}
	}

	maxBreaker := 3
	if v := os.Getenv("MAX_TOOL_FAILURE_STREAK"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxBreaker = n
		}
	}

	userID := os.Getenv("DEFAULT_USER_ID")
	if userID == "" {
		userID = "default-user"
	}
	agentID := os.Getenv("DEFAULT_AGENT_ID")
	if agentID == "" {
		agentID = "overseer"
	}
	scope := identity.TenantScope{UserID: userID, AgentID: agentID}
	if err := scope.Validate(); err != nil {
		log.Fatalf("DEFAULT_USER_ID / DEFAULT_AGENT_ID: %v", err)
	}

	var natsPub types.NATSPublisher
	var natsConn *nats.Conn
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		var err error
		natsConn, err = nats.Connect(natsURL)
		if err != nil {
			log.Printf("[nats] connect failed (%s): %v", natsURL, err)
			natsConn = nil
		} else {
			defer natsConn.Close()
			natsPub = natsintegration.NewConnPublisher(natsConn)
			log.Printf("[nats] connected: %s", natsURL)
		}
	}

	var memStore *pgintegration.MemoryStore
	var tokenUsage *pgintegration.TokenUsageStore
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		pool, err := pgxpool.New(context.Background(), dbURL)
		if err != nil {
			log.Printf("[pg] pool init failed: %v", err)
		} else {
			defer pool.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			tu := pgintegration.NewTokenUsageStore(pool)
			if tu != nil {
				if err := tu.EnsureSchema(ctx); err != nil {
					log.Printf("[pg] token_usage schema failed: %v", err)
				} else {
					tokenUsage = tu
					log.Printf("[pg] token usage ledger enabled (overseer_token_usage)")
				}
			}
			store := pgintegration.NewMemoryStore(pool)
			if err := store.EnsureSchema(ctx); err != nil {
				log.Printf("[pg] ensure schema failed: %v", err)
			} else {
				memStore = store
				log.Printf("[pg] memory store enabled (tenant-scoped reads)")
			}
			cancel()
		}
	}

	httpLLM, err := httpllm.BuildClientFromEnv(tokenUsage)
	if err != nil {
		log.Fatalf("llm client: %v", err)
	}
	llmHolder := httpllm.NewHolder(httpLLM)
	bridge := &adapterllm.Bridge{LLM: llmHolder}

	sessions := session.NewStore()
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("[redis] invalid REDIS_URL: %v", err)
		} else {
			rdb := redis.NewClient(opt)
			sessions.Redis = rdb
			log.Printf("[redis] session store enabled via REDIS_URL")
		}
	} else if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		redisDB := 0
		if v := os.Getenv("REDIS_DB"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				redisDB = n
			}
		}
		rdb := redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       redisDB,
		})
		sessions.Redis = rdb
		if v := os.Getenv("SESSION_MAX_MESSAGES"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				sessions.MaxMessages = n
			}
		}
		if v := os.Getenv("SESSION_TTL_SECONDS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				sessions.TTL = time.Duration(n) * time.Second
			}
		}
		log.Printf("[redis] session store enabled: %s db=%d", redisAddr, redisDB)
	}

	// NATS 通知后从会话 Redis 客户端拉取 L2 快照合并 env（与 Admin 分库部署）
	if strings.TrimSpace(os.Getenv("CORE_CONFIG_BUS")) == "1" && natsConn != nil && sessions.Redis != nil {
		subj := configbus.NATSSubjectCoreConfigUpdated
		_, err := natsConn.Subscribe(subj, func(_ *nats.Msg) {
			c, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			n, err := configbus.MergeFromRedis(c, sessions.Redis)
			if err != nil {
				log.Printf("[configbus] hot reload: %v", err)
				return
			}
			log.Printf("[configbus] hot reload merged %d keys from Redis", n)
			if envDisabled("CORE_CONFIG_HOT_LLM") {
				return
			}
			c2, berr := httpllm.BuildClientFromEnv(tokenUsage)
			if berr != nil {
				log.Printf("[configbus] hot LLM rebuild skipped: %v (keeping previous client)", berr)
				return
			}
			llmHolder.Store(c2)
			log.Printf("[configbus] LLM client rebuilt from merged env (set CORE_CONFIG_HOT_LLM=0 to disable)")
		})
		if err != nil {
			log.Printf("[configbus] subscribe %s: %v", subj, err)
		} else {
			log.Printf("[configbus] subscribed %s", subj)
		}
	}

	var postTurn reflection.PostTurnExtractor = reflection.NoopPostTurn{}
	if memStore != nil && !envDisabled("MEMORY_AUTO_EXTRACT") {
		ext := memorywork.NewLLMExtractor(llmHolder, memStore)
		if v := os.Getenv("MEMORY_EXTRACT_MAX_TOKENS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				ext.MaxTokens = n
			}
		}
		postTurn = ext
		log.Printf("[memory] auto post-turn LLM extraction enabled (set MEMORY_AUTO_EXTRACT=0 to disable)")
	}

	var comp compaction.Compactor
	if raw := strings.TrimSpace(os.Getenv("CONTEXT_TOKEN_BUDGET")); raw == "0" || strings.EqualFold(raw, "off") || strings.EqualFold(raw, "false") {
		comp = compaction.NoopCompactor{}
	} else {
		budget := 48000
		if raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				budget = n
			}
		}
		minTail := 8
		if v := strings.TrimSpace(os.Getenv("CONTEXT_COMPACT_MIN_TAIL")); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				minTail = n
			}
		}
		comp = compaction.NewThresholdCompactor(budget, minTail)
	}

	var pruner compaction.ToolResultPruner = compaction.TruncateToolPruner{}
	if raw := strings.TrimSpace(os.Getenv("TOOL_RESULT_MAX_RUNES")); raw == "0" || strings.EqualFold(raw, "off") || strings.EqualFold(raw, "false") {
		pruner = compaction.NoopToolPruner{}
	} else if raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			pruner = compaction.TruncateToolPruner{MaxRunes: n}
		}
	}

	tokEst := compaction.TokenEstimator(compaction.RoughTokenEstimator)
	if _, ok := comp.(compaction.NoopCompactor); ok {
		tokEst = nil
	}

	defaultPersona := strings.TrimSpace(os.Getenv("AGENT_PERSONA"))
	var runtimeHint func() string
	if hint := strings.TrimSpace(os.Getenv("RUNTIME_HINT")); hint != "" {
		runtimeHint = func() string { return hint }
	}

	// runtime.Service wires physical adapters into agent_core: Redis (session), PG (LTM), Bridge (LLM), NATS (optional).
	chat := &runtime.Service{
		Engine:                 agent_core.NewEngine(),
		Sessions:               sessions,
		Bridge:                 bridge,
		PG:                     memStore,
		NATS:                   natsPub,
		Scope:                  scope,
		Todos:                  planner.NewMemoryStore(),
		PostTurn:               postTurn,
		Compactor:              comp,
		ToolPruner:             pruner,
		TokenEst:               tokEst,
		DefaultPersona:         defaultPersona,
		RuntimeHint:            runtimeHint,
		MaxTurns:               maxTurns,
		MaxConsecutiveFailures: maxBreaker,
	}

	addr := ":" + port

	authMode, err := ws.ParseAuthMode(os.Getenv("WS_AUTH_MODE"))
	if err != nil {
		log.Fatalf("WS_AUTH_MODE: %v", err)
	}
	strictOrigin := envEnabled("WS_STRICT_ORIGIN")
	allowedOrigins := strings.TrimSpace(os.Getenv("WS_ALLOWED_ORIGINS"))
	checkOrigin := ws.NewOriginChecker(allowedOrigins, strictOrigin)

	maxMsgPerMin := 0
	if v := strings.TrimSpace(os.Getenv("WS_MAX_MESSAGES_PER_MINUTE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxMsgPerMin = n
		}
	}

	wsCfg := ws.Config{
		CheckOrigin:      checkOrigin,
		AuthMode:         authMode,
		BearerToken:      strings.TrimSpace(os.Getenv("WS_BEARER_TOKEN")),
		JWTSigningKey:    jwtHS256SecretFromEnv(),
		AllowQueryBearer: envEnabled("WS_ALLOW_QUERY_BEARER"),
		AllowQueryJWT:    envEnabled("WS_ALLOW_QUERY_JWT"),
		MaxMsgsPerMinute: maxMsgPerMin,
	}

	log.Printf("[boot] ascentia-core listen=%s%s WS_AUTH_MODE=%s WS_STRICT_ORIGIN=%v WS_MAX_MSG_PER_MIN=%d",
		addr, wsPath, authMode.String(), strictOrigin, maxMsgPerMin)
	server := ws.NewServer(addr, wsPath, chat, wsCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if memStore != nil && !envDisabled("MEMORY_DREAM") {
		// 默认每 30 分钟跑一次；间隔见 .env.example 中 MEMORY_DREAM_INTERVAL_MINUTES。
		// 显式设为 0 可关闭定时 Dream（仍保留 MEMORY_DREAM=0 总开关）。
		intervalMin := 30
		if v := strings.TrimSpace(os.Getenv("MEMORY_DREAM_INTERVAL_MINUTES")); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				log.Printf("[dream] disabled: MEMORY_DREAM_INTERVAL_MINUTES=%q (positive = interval min; unset = default 30)", v)
				intervalMin = 0
			} else {
				intervalMin = n
			}
		}
		if intervalMin > 0 {
			dreamer := memorywork.NewDreamConsolidator(llmHolder, memStore)
			if t := os.Getenv("MEMORY_DREAM_MAX_TOKENS"); t != "" {
				if n, err := strconv.Atoi(t); err == nil && n > 0 {
					dreamer.MaxTokens = n
				}
			}
			go func() {
				ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						dc, dcancel := context.WithTimeout(context.Background(), 15*time.Minute)
						if err := dreamer.Consolidate(dc, scope, reflection.ConsolidationOptions{}); err != nil {
							log.Printf("[dream] consolidate failed: %v", err)
						}
						dcancel()
					}
				}
			}()
			log.Printf("[dream] periodic consolidation every %d min (scope user=%s agent=%s; MEMORY_DREAM=0 to disable)", intervalMin, scope.UserID, scope.AgentID)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := server.Run(ctx); err != nil {
		log.Printf("server stopped: %v", err)
	}
}
