package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ascentia-core/internal/types"
	"ascentia-core/internal/usage"
)

// Client is a minimal OpenAI-compatible Chat Completions API client
// with support for tool calling + SSE streaming.
type Client struct {
	BaseURL    string
	AuthToken  string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	// Recorder receives token usage after successful non-empty usage from the provider (optional).
	Recorder usage.AsyncRecorder
}

type ChatResponse struct {
	AssistantText string
	ToolUses      []types.ToolUse
	AssistantMsg  types.Message
}

type openAITool struct {
	Type     string         `json:"type"` // "function"
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIChatCompletionResponse struct {
	ID      string `json:"id,omitempty"`
	Object  string `json:"object,omitempty"`
	Created int64  `json:"created,omitempty"`
	Model   string `json:"model,omitempty"`
	Choices []struct {
		Index        int           `json:"index"`
		FinishReason string        `json:"finish_reason,omitempty"`
		Message      types.Message `json:"message"`
	} `json:"choices"`
	Usage *openAIUsage `json:"usage,omitempty"`
}

// NewClient builds a Claude-compatible API client.
func NewClient(baseURL, authToken, apiKey, model string, timeout time.Duration) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		AuthToken: authToken,
		APIKey:    apiKey,
		Model:     model,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Chat(ctx context.Context, messages []types.Message, tools []types.ToolDefinition) (ChatResponse, error) {
	return c.completeChat(ctx, messages, tools, 1024, 0.2, "chat_completion")
}

// ChatTextOnly is a non-streaming completion without tools (memory extraction, classification, etc.).
func (c *Client) ChatTextOnly(ctx context.Context, messages []types.Message, maxTokens int) (ChatResponse, error) {
	if maxTokens <= 0 {
		maxTokens = 512
	}
	return c.completeChat(ctx, messages, nil, maxTokens, 0.1, "text_only")
}

func (c *Client) completeChat(ctx context.Context, messages []types.Message, tools []types.ToolDefinition, maxTokens int, temperature float64, usageChannel string) (ChatResponse, error) {
	endpoint := c.BaseURL + "/v1/chat/completions"

	payload := map[string]any{
		"model":       c.Model,
		"messages":    messages,
		"stream":      false,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	}
	if len(tools) > 0 {
		openAITools := make([]openAITool, 0, len(tools))
		for _, t := range tools {
			openAITools = append(openAITools, openAITool{
				Type: "function",
				Function: openAIFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
		payload["tools"] = openAITools
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ChatResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	} else if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	req.Header.Set("User-Agent", "ascentia-core/0.1")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return ChatResponse{}, fmt.Errorf("llm http %d: %s", resp.StatusCode, buf.String())
	}

	var res openAIChatCompletionResponse
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&res); err != nil {
		return ChatResponse{}, err
	}
	if len(res.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("llm: empty choices")
	}

	assistantMsg := res.Choices[0].Message
	assistantText := assistantMsg.Content
	toolUses := extractToolUsesFromAssistant(assistantMsg)

	c.emitUsage(ctx, usageChannel, res.ID, res.Model, res.Usage)
	return ChatResponse{
		AssistantText: assistantText,
		ToolUses:      toolUses,
		AssistantMsg:  assistantMsg,
	}, nil
}

func (c *Client) emitUsage(ctx context.Context, channel, providerID, responseModel string, u *openAIUsage) {
	if c == nil || c.Recorder == nil || u == nil {
		return
	}
	pt, ct, tt := u.PromptTokens, u.CompletionTokens, u.TotalTokens
	if tt == 0 {
		tt = pt + ct
	}
	if pt == 0 && ct == 0 && tt == 0 {
		return
	}
	a, _ := usage.FromContext(ctx)
	model := responseModel
	if model == "" {
		model = c.Model
	}
	c.Recorder.RecordAsync(usage.Row{
		UserID:            a.UserID,
		AgentID:           a.AgentID,
		OperatorRole:      a.OperatorRole,
		SessionID:         a.SessionID,
		RequestID:         a.RequestID,
		Model:             model,
		Channel:           channel,
		PromptTokens:      pt,
		CompletionTokens:  ct,
		TotalTokens:       tt,
		ProviderRequestID: providerID,
	})
}

// StreamChat performs a Claude-compatible streaming request (SSE).
//
// It collects:
// - assistant text blocks via content_block_delta{type:text_delta}
// - tool_use blocks via content_block_start + input_json_delta (partial JSON)
//
// Once a tool_use content block hits content_block_stop, its input JSON is parsed
// and included in the returned toolUses slice.
func (c *Client) StreamChat(
	ctx context.Context,
	messages []types.Message,
	tools []types.ToolDefinition,
	onTextDelta func(string),
) (assistantMsg types.Message, toolUses []types.ToolUse, err error) {
	endpoint := c.BaseURL + "/v1/chat/completions"

	openAITools := make([]openAITool, 0, len(tools))
	for _, t := range tools {
		openAITools = append(openAITools, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	modelName := c.Model
	if m := chatModelFromContext(ctx); m != "" {
		modelName = m
	}
	payload := map[string]any{
		"model":       modelName,
		"messages":    messages,
		"tools":       openAITools,
		"stream":      true,
		"temperature": 0.2,
		"max_tokens":  1024,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return types.Message{}, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return types.Message{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	} else if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	req.Header.Set("User-Agent", "ascentia-core/0.1")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return types.Message{}, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return types.Message{}, nil, fmt.Errorf("llm stream http %d: %s", resp.StatusCode, buf.String())
	}

	// OpenAI streaming: data: {id, choices:[{delta:{content, tool_calls...}}]}
	type toolCallAccum struct {
		id   string
		name string
		args strings.Builder
	}

	accums := make(map[int]*toolCallAccum) // index within tool_calls array
	var assistantText strings.Builder
	var lastUsage *openAIUsage
	var streamID string
	var streamModel string

	reader := bufio.NewReader(resp.Body)

	for {
		line, rerr := reader.ReadString('\n')
		if rerr != nil && rerr != io.EOF {
			return types.Message{}, nil, rerr
		}
		line = strings.TrimRight(line, "\r\n")

		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "" {
				continue
			}
			if data == "[DONE]" {
				break
			}

			var chunk struct {
				ID      string       `json:"id,omitempty"`
				Model   string       `json:"model,omitempty"`
				Usage   *openAIUsage `json:"usage,omitempty"`
				Choices []struct {
					Delta struct {
						Content   *string `json:"content,omitempty"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id,omitempty"`
							Type     string `json:"type,omitempty"`
							Function struct {
								Name      string `json:"name,omitempty"`
								Arguments string `json:"arguments,omitempty"`
							} `json:"function,omitempty"`
						} `json:"tool_calls,omitempty"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason,omitempty"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				// Some vendors send keepalive/ping lines; ignore unparsable payloads.
				continue
			}
			if chunk.ID != "" {
				streamID = chunk.ID
			}
			if chunk.Model != "" {
				streamModel = chunk.Model
			}
			if chunk.Usage != nil {
				lastUsage = chunk.Usage
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			delta := chunk.Choices[0].Delta
			if delta.Content != nil && *delta.Content != "" {
				assistantText.WriteString(*delta.Content)
				onTextDelta(*delta.Content)
			}

			for _, tc := range delta.ToolCalls {
				acc := accums[tc.Index]
				if acc == nil {
					acc = &toolCallAccum{}
					accums[tc.Index] = acc
				}
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					acc.args.WriteString(tc.Function.Arguments)
				}
			}
		}

		if rerr == io.EOF {
			break
		}
	}

	toolUses = make([]types.ToolUse, 0, len(accums))
	toolCalls := make([]types.ToolCall, 0, len(accums))
	for i := 0; i < len(accums); i++ {
		acc := accums[i]
		if acc == nil || acc.name == "" {
			continue
		}
		argsStr := acc.args.String()
		input := map[string]any{}
		if strings.TrimSpace(argsStr) != "" {
			if err := json.Unmarshal([]byte(argsStr), &input); err != nil {
				input = map[string]any{}
			}
		}
		toolUses = append(toolUses, types.ToolUse{ID: acc.id, Name: acc.name, Input: input})
		toolCalls = append(toolCalls, types.ToolCall{
			ID:   acc.id,
			Type: "function",
			Function: types.ToolCallFunction{
				Name:      acc.name,
				Arguments: argsStr,
			},
		})
	}

	assistantMsg = types.Message{
		Role:      types.RoleAssistant,
		Content:   assistantText.String(),
		ToolCalls: toolCalls,
	}
	c.emitUsage(ctx, "stream_chat", streamID, streamModel, lastUsage)
	return assistantMsg, toolUses, nil
}

func extractToolUsesFromAssistant(m types.Message) []types.ToolUse {
	if len(m.ToolCalls) == 0 {
		return nil
	}
	out := make([]types.ToolUse, 0, len(m.ToolCalls))
	for _, tc := range m.ToolCalls {
		if tc.Function.Name == "" {
			continue
		}
		input := map[string]any{}
		if strings.TrimSpace(tc.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = map[string]any{}
			}
		}
		out = append(out, types.ToolUse{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}
	return out
}
