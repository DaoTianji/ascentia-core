package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	adapterllm "ascentia-core/internal/adapter/llm"
	adaptermem "ascentia-core/internal/adapter/memory"
	transadapter "ascentia-core/internal/adapter/transcript"
	pgintegration "ascentia-core/internal/integration/pg"
	httpllm "ascentia-core/internal/llm"
	"ascentia-core/internal/session"
	"ascentia-core/internal/skills"
	"ascentia-core/internal/types"
	"ascentia-core/internal/usage"
	"ascentia-core/pkg/agent_core"
	"ascentia-core/pkg/agent_core/compaction"
	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/loop"
	"ascentia-core/pkg/agent_core/planner"
	"ascentia-core/pkg/agent_core/prompt"
	"ascentia-core/pkg/agent_core/reflection"
	"ascentia-core/pkg/agent_core/transcript"
)

// AssistantNotifier streams assistant output to the transport layer (e.g. WebSocket).
type AssistantNotifier interface {
	SendSystemStatus(ctx context.Context, sessionID, text string) error
	SendAssistantStream(ctx context.Context, sessionID, text string, isFinal bool) error
}

// Service wires agent_core to session storage and infrastructure adapters.
type Service struct {
	Engine   *agent_core.Engine
	Sessions *session.Store
	Bridge   *adapterllm.Bridge
	PG       *pgintegration.MemoryStore
	NATS     types.NATSPublisher
	Scope    identity.TenantScope
	Todos    *planner.MemoryStore

	// PostTurn runs after a successful streamed turn (e.g. async LLM memory extraction).
	PostTurn reflection.PostTurnExtractor

	Compactor  compaction.Compactor
	ToolPruner compaction.ToolResultPruner
	TokenEst   compaction.TokenEstimator

	MaxTurns               int
	MaxConsecutiveFailures int

	// DefaultPersona is injected into AssembleInput.Persona when the transport does not override it (e.g. admin backend / env at boot).
	DefaultPersona string
	// RuntimeHint is appended into BuildCoreRules (optional host/build line); nil or empty body is fine.
	RuntimeHint func() string
}

func (s *Service) maxBreaker() int {
	if s.MaxConsecutiveFailures > 0 {
		return s.MaxConsecutiveFailures
	}
	return 3
}

// ChatStream runs one user turn with streaming assistant text per model round (agent_core tool loop).
func (s *Service) ChatStream(ctx context.Context, sessionID, userText string, n AssistantNotifier, conn StreamConn) error {
	scope := s.Scope
	if u := strings.TrimSpace(conn.UserID); u != "" {
		scope.UserID = u
	}
	if a := strings.TrimSpace(conn.AgentID); a != "" {
		scope.AgentID = a
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	stmKey := ScopedChatSessionKey(scope.UserID, scope.AgentID, sessionID)

	s.Sessions.Append(stmKey, types.Message{
		Role:    types.RoleUser,
		Content: userText,
	})
	full := s.Sessions.Get(stmKey)
	if len(full) == 0 {
		return fmt.Errorf("runtime: empty session")
	}
	history := full[:len(full)-1]
	userTail := full[len(full)-1]

	hint := ""
	if s.RuntimeHint != nil {
		hint = strings.TrimSpace(s.RuntimeHint())
	}

	persona := strings.TrimSpace(s.DefaultPersona)
	if p := strings.TrimSpace(conn.Persona); p != "" {
		persona = p
	}

	var sideQ *adaptermem.SideQuery
	if s.PG != nil {
		sideQ = &adaptermem.SideQuery{PG: s.PG, SessionID: stmKey}
	}

	req := agent_core.RunRequest{
		Scope:           scope,
		SessionID:       stmKey,
		RequestID:       stmKey + "-" + time.Now().UTC().Format("150405.000000000"),
		History:         transadapter.MessagesFromTypes(history),
		UserMessage:     transadapter.FromTypes(userTail),
		AllowedSkillIDs: []string{skills.DefaultSkillID},
		TodoStore:       s.Todos,
		SideQuery:       sideQ,
		Assembler:       prompt.DefaultAssembler{},
		Prompt: prompt.AssembleInput{
			CoreRules: BuildCoreRules(time.Now(), hint),
			Persona:   persona,
		},
		Catalog: &skills.Catalog{PG: s.PG, NATS: s.NATS},
		Model:   s.Bridge,
		Options: agent_core.RunOptions{
			MaxToolTurns:               s.MaxTurns,
			MaxConsecutiveToolFailures: s.maxBreaker(),
		},
	}
	if s.Compactor != nil {
		req.Compactor = s.Compactor
	}
	if s.ToolPruner != nil {
		req.ToolPruner = s.ToolPruner
	}
	if s.TokenEst != nil {
		req.Estimate = s.TokenEst
	}

	ctx = usage.WithAttribution(ctx, usage.Attribution{
		UserID:       scope.UserID,
		AgentID:      scope.AgentID,
		OperatorRole: strings.TrimSpace(conn.OperatorRole),
		SessionID:    sessionID,
		RequestID:    req.RequestID,
	})
	ctx = httpllm.ContextWithChatModel(ctx, conn.Model)

	out, preLen, finalText, err := s.Engine.RunStreamedToolLoop(ctx, req, s.Bridge, loop.StreamHooks{
		OnTextDelta: func(delta string) {
			if assistantDeltaIsInvisible(delta) {
				return
			}
			_ = n.SendAssistantStream(ctx, sessionID, delta, false)
		},
		AfterAssistantComplete: func() {
			_ = n.SendAssistantStream(ctx, sessionID, "", true)
		},
		BeforeToolBatch: func(names []string) {
			if len(names) == 0 {
				return
			}
			status := fmt.Sprintf("正在执行工具：%s …", strings.Join(names, ", "))
			_ = n.SendSystemStatus(ctx, sessionID, status)
		},
		AfterToolBatch: func(names []string) {
			if len(names) == 0 {
				return
			}
			status := fmt.Sprintf("工具已完成（%s），正在生成回复…", strings.Join(names, ", "))
			_ = n.SendSystemStatus(ctx, sessionID, status)
		},
	})
	if err != nil {
		return err
	}

	delta := out[preLen:]
	wroteAssistant := strings.TrimSpace(finalText) != ""
	if !wroteAssistant {
		for _, m := range delta {
			if m.Role == transcript.RoleAssistant && strings.TrimSpace(m.Content) != "" {
				wroteAssistant = true
				break
			}
		}
	}
	if !wroteAssistant {
		_ = n.SendSystemStatus(ctx, sessionID,
			"本轮未收到模型的可见文字回复（可能仍在处理或上游超时）；请稍后重试或简化问题。")
	}

	if len(delta) > 0 {
		s.Sessions.Append(stmKey, transadapter.MessagesToTypes(delta)...)
	}

	if s.PostTurn != nil && len(delta) > 0 {
		turnCopy := append([]transcript.Message(nil), delta...)
		sc := scope
		sid := stmKey
		pt := s.PostTurn
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			ctx = usage.WithAttribution(ctx, usage.Attribution{
				UserID:    sc.UserID,
				AgentID:   sc.AgentID,
				SessionID: sid,
				RequestID: req.RequestID + "/memory_extract",
			})
			_ = pt.AfterTurn(ctx, sc, sid, turnCopy)
		}()
	}
	return nil
}

// assistantDeltaIsInvisible：仅空白、零宽、BOM 等，不应在客户端打开「空流式气泡」。
func assistantDeltaIsInvisible(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\ufeff':
			continue
		}
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
