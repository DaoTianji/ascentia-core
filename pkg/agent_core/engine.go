package agent_core

import (
	"context"
	"fmt"
	"strings"

	"ascentia-core/pkg/agent_core/compaction"
	"ascentia-core/pkg/agent_core/identity"
	"ascentia-core/pkg/agent_core/llm"
	"ascentia-core/pkg/agent_core/loop"
	"ascentia-core/pkg/agent_core/memory"
	"ascentia-core/pkg/agent_core/planner"
	"ascentia-core/pkg/agent_core/profile"
	"ascentia-core/pkg/agent_core/prompt"
	"ascentia-core/pkg/agent_core/reflection"
	"ascentia-core/pkg/agent_core/tools"
	"ascentia-core/pkg/agent_core/transcript"
)

// Engine orchestrates prompt assembly, optional side memory, planner focus,
// compaction, the ReAct loop, and post-turn hooks.
type Engine struct{}

// NewEngine returns a zero-config engine instance.
func NewEngine() *Engine { return &Engine{} }

// RunRequest is built by the host service (gateway, job worker, tests). All tenant isolation inputs flow here.
type RunRequest struct {
	Scope     identity.TenantScope
	SessionID string
	RequestID string

	History     []transcript.Message
	UserMessage transcript.Message

	AllowedSkillIDs []string
	TodoStore       planner.Store

	SideQuery memory.SideQuerySelector
	Profile   profile.Provider

	Assembler prompt.Assembler
	Prompt    prompt.AssembleInput

	Catalog tools.SkillCatalog
	Model   llm.ModelClient

	Compactor  compaction.Compactor
	ToolPruner compaction.ToolResultPruner

	PostTurn reflection.PostTurnExtractor
	Estimate compaction.TokenEstimator

	Options RunOptions
}

// RunOptions tunes the tool loop and breaker.
type RunOptions struct {
	MaxToolTurns               int
	MaxConsecutiveToolFailures int
}

// RunResult contains the full message list after the turn and the new suffix.
type RunResult struct {
	Messages  []transcript.Message
	Delta     []transcript.Message
	FinalText string
}

// PreparedLoop is the assembled state before the ReAct loop (tool registry + messages).
type PreparedLoop struct {
	Messages   []transcript.Message
	PreLoopLen int
	Registry   *tools.Registry
	ToolCtx    tools.Context
}

// PrepareLoop builds system + history + user messages, compacts, merges tools.
// Callers use this with loop.Run or loop.RunStreamed for custom streaming wiring.
func (e *Engine) PrepareLoop(ctx context.Context, req RunRequest) (PreparedLoop, error) {
	_ = e
	var prep PreparedLoop
	if err := req.Scope.Validate(); err != nil {
		return prep, err
	}
	if req.Catalog == nil {
		return prep, fmt.Errorf("agent_core: SkillCatalog is nil")
	}

	assembler := req.Assembler
	if assembler == nil {
		assembler = prompt.DefaultAssembler{}
	}
	comp := req.Compactor
	if comp == nil {
		comp = compaction.NoopCompactor{}
	}
	pruner := req.ToolPruner
	if pruner == nil {
		pruner = compaction.NoopToolPruner{}
	}

	in := req.Prompt
	in.Scope = req.Scope
	in.SessionID = req.SessionID

	if req.Profile != nil {
		p, err := req.Profile.Persona(ctx, req.Scope)
		if err != nil {
			return prep, err
		}
		if in.Persona == "" {
			in.Persona = p
		}
	}

	if req.SideQuery != nil {
		q := strings.TrimSpace(req.UserMessage.Content)
		rec, err := req.SideQuery.SelectForTurn(ctx, req.Scope, q, nil, 5)
		if err != nil {
			return prep, err
		}
		in.MemoryRecall = formatMemoryRecall(rec)
	}

	if req.TodoStore != nil {
		in.TodoFocus = planner.FocusPrompt(req.SessionID, req.TodoStore)
	}

	sys, err := assembler.BuildSystemMessages(ctx, in)
	if err != nil {
		return prep, err
	}

	msgs := make([]transcript.Message, 0, len(sys)+len(req.History)+1)
	msgs = append(msgs, sys...)
	for _, m := range req.History {
		msgs = append(msgs, pruner.Prune(m.Clone()))
	}
	msgs = append(msgs, req.UserMessage.Clone())

	tok := 0
	if req.Estimate != nil {
		tok = req.Estimate(msgs)
	}
	msgs, _, err = comp.MaybeCompact(ctx, req.Scope, req.SessionID, msgs, tok)
	if err != nil {
		return prep, err
	}

	skillTools, err := req.Catalog.ToolsForSkills(req.AllowedSkillIDs)
	if err != nil {
		return prep, err
	}
	plannerTools := planner.BuiltinTools(req.TodoStore)
	allTools := append(append([]tools.Tool{}, skillTools...), plannerTools...)
	reg, err := tools.NewRegistry(allTools)
	if err != nil {
		return prep, err
	}

	allowed := make(map[string]struct{})
	for _, t := range allTools {
		allowed[t.Name()] = struct{}{}
	}
	tcx := tools.Context{
		Scope:     req.Scope,
		SessionID: req.SessionID,
		RequestID: req.RequestID,
		Allowed:   allowed,
	}

	prep.Messages = msgs
	prep.PreLoopLen = len(msgs)
	prep.Registry = reg
	prep.ToolCtx = tcx
	return prep, nil
}

// Run executes one user turn (multi-step tool loop until assistant text without tools).
func (e *Engine) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	_ = e
	if req.Model == nil {
		return RunResult{}, fmt.Errorf("agent_core: Model is nil")
	}

	prep, err := e.PrepareLoop(ctx, req)
	if err != nil {
		return RunResult{}, err
	}
	preLoopLen := prep.PreLoopLen

	out, final, err := loop.Run(ctx, prep.ToolCtx, prep.Messages, prep.Registry, req.Model, loop.Options{
		MaxTurns:                   req.Options.MaxToolTurns,
		MaxConsecutiveToolFailures: req.Options.MaxConsecutiveToolFailures,
		ToolPruner:                 req.ToolPruner,
	})
	if err != nil {
		return RunResult{Messages: out, FinalText: final}, err
	}

	delta := out[preLoopLen:]
	result := RunResult{
		Messages:  out,
		Delta:     delta,
		FinalText: final,
	}

	if req.PostTurn != nil {
		turn := append([]transcript.Message(nil), delta...)
		if err := req.PostTurn.AfterTurn(ctx, req.Scope, req.SessionID, turn); err != nil {
			return result, err
		}
	}
	return result, nil
}

// RunStreamedToolLoop runs PrepareLoop then the multi-turn streaming tool loop (same path as production WS).
// streamer must implement llm.StreamTurnCompleter (e.g. internal/adapter/llm.Bridge).
func (e *Engine) RunStreamedToolLoop(ctx context.Context, req RunRequest, streamer llm.StreamTurnCompleter, hooks loop.StreamHooks) (messages []transcript.Message, preLoopLen int, finalText string, err error) {
	_ = e
	prep, err := e.PrepareLoop(ctx, req)
	if err != nil {
		return nil, 0, "", err
	}
	out, final, err := loop.RunStreamed(ctx, prep.ToolCtx, prep.Messages, prep.Registry, streamer, loop.Options{
		MaxTurns:                   req.Options.MaxToolTurns,
		MaxConsecutiveToolFailures: req.Options.MaxConsecutiveToolFailures,
		ToolPruner:                 req.ToolPruner,
	}, hooks)
	if err != nil {
		return out, prep.PreLoopLen, "", err
	}
	return out, prep.PreLoopLen, final, nil
}

// RunStream performs a single streaming completion (no multi-turn tool loop yet).
// Thinking/text demux is the responsibility of the ModelClient.Stream implementation
// or the provided Sink (e.g. wrap with thinking.HandlerFunc).
func (e *Engine) RunStream(ctx context.Context, req RunRequest, sink llm.StreamSink) error {
	_ = e
	if err := req.Scope.Validate(); err != nil {
		return err
	}
	if req.Model == nil || sink == nil {
		return fmt.Errorf("agent_core: Model and Sink required")
	}
	assembler := req.Assembler
	if assembler == nil {
		assembler = prompt.DefaultAssembler{}
	}
	in := req.Prompt
	in.Scope = req.Scope
	in.SessionID = req.SessionID
	if req.Profile != nil {
		p, err := req.Profile.Persona(ctx, req.Scope)
		if err != nil {
			return err
		}
		if in.Persona == "" {
			in.Persona = p
		}
	}
	if req.SideQuery != nil {
		q := strings.TrimSpace(req.UserMessage.Content)
		rec, err := req.SideQuery.SelectForTurn(ctx, req.Scope, q, nil, 5)
		if err != nil {
			return err
		}
		in.MemoryRecall = formatMemoryRecall(rec)
	}
	if req.TodoStore != nil {
		in.TodoFocus = planner.FocusPrompt(req.SessionID, req.TodoStore)
	}
	sys, err := assembler.BuildSystemMessages(ctx, in)
	if err != nil {
		return err
	}
	msgs := make([]transcript.Message, 0, len(sys)+len(req.History)+1)
	msgs = append(msgs, sys...)
	for _, m := range req.History {
		msgs = append(msgs, m)
	}
	msgs = append(msgs, req.UserMessage)

	return req.Model.Stream(ctx, llm.StreamRequest{
		Messages: msgs,
		ToolDefs: nil,
		Sink:     sink,
	})
}

func formatMemoryRecall(rec []memory.Record) string {
	if len(rec) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("以下记忆由系统从长期存储中召回，**可能已过时**；涉及关键事实与外部状态时请用工具核对：\n")
	for _, r := range rec {
		b.WriteString("- [")
		b.WriteString(string(r.Category))
		b.WriteString("] ")
		b.WriteString(r.Key)
		if hint := memory.AgeHint(r.CreatedAt); hint != "" {
			b.WriteString(" — ")
			b.WriteString(hint)
		}
		if note := memory.FreshnessNote(r.CreatedAt); note != "" {
			b.WriteString(" ")
			b.WriteString(note)
		}
		b.WriteString("\n  ")
		b.WriteString(strings.TrimSpace(r.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
