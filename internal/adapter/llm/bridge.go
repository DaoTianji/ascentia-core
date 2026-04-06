package llm

import (
	"context"
	"fmt"

	transadapter "ascentia-core/internal/adapter/transcript"
	httpllm "ascentia-core/internal/llm"
	"ascentia-core/internal/types"
	aclellm "ascentia-core/pkg/agent_core/llm"
	"ascentia-core/pkg/agent_core/tools"
)

// Bridge adapts the HTTP OpenAI-compatible client to agent_core llm interfaces.
// LLM 指向可热替换的 Holder（配置变更后 Store 新 *Client 即可）。
type Bridge struct {
	LLM *httpllm.Holder
}

func (b *Bridge) inner() *httpllm.Client {
	if b == nil || b.LLM == nil {
		return nil
	}
	return b.LLM.Load()
}

func (b *Bridge) Complete(ctx context.Context, req aclellm.CompleteRequest) (aclellm.CompleteResponse, error) {
	inner := b.inner()
	if inner == nil {
		return aclellm.CompleteResponse{}, fmt.Errorf("llm bridge: no client")
	}
	msgs := transadapter.MessagesToTypes(req.Messages)
	tdefs := toolDefsToTypes(req.ToolDefs)
	resp, err := inner.Chat(ctx, msgs, tdefs)
	if err != nil {
		return aclellm.CompleteResponse{}, err
	}
	return aclellm.CompleteResponse{Assistant: transadapter.FromTypes(resp.AssistantMsg)}, nil
}

func (b *Bridge) Stream(ctx context.Context, req aclellm.StreamRequest) error {
	_ = ctx
	_ = req
	return aclellm.ErrStreamingNotSupported
}

func (b *Bridge) CompleteStreamTurn(ctx context.Context, req aclellm.CompleteRequest, onTextDelta func(string)) (aclellm.CompleteResponse, error) {
	inner := b.inner()
	if inner == nil {
		return aclellm.CompleteResponse{}, fmt.Errorf("llm bridge: no client")
	}
	msgs := transadapter.MessagesToTypes(req.Messages)
	tdefs := toolDefsToTypes(req.ToolDefs)
	asst, _, err := inner.StreamChat(ctx, msgs, tdefs, onTextDelta)
	if err != nil {
		return aclellm.CompleteResponse{}, err
	}
	return aclellm.CompleteResponse{Assistant: transadapter.FromTypes(asst)}, nil
}

func toolDefsToTypes(defs []tools.Definition) []types.ToolDefinition {
	out := make([]types.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		typ := d.Type
		if typ == "" || typ == "function" {
			typ = "custom"
		}
		out = append(out, types.ToolDefinition{
			Type:        typ,
			Name:        d.Name,
			Description: d.Description,
			InputSchema: d.Parameters,
		})
	}
	return out
}

var _ aclellm.ModelClient = (*Bridge)(nil)
var _ aclellm.StreamTurnCompleter = (*Bridge)(nil)
