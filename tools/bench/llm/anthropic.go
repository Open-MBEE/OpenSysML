package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

// anthropicProvider speaks the Anthropic Messages API.
type anthropicProvider struct {
	endpoint Endpoint
	model    string
	key      string
}

func (p *anthropicProvider) Name() string  { return p.endpoint.Name }
func (p *anthropicProvider) Model() string { return p.model }

type anBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anMessage struct {
	Role    string    `json:"role"`
	Content []anBlock `json:"content"`
}

func (p *anthropicProvider) Complete(ctx context.Context, req Request) (Response, error) {
	var msgs []anMessage
	for _, m := range req.Messages {
		switch m.Role {
		case "tool":
			block := anBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content}
			// Consecutive tool results share one user turn.
			if n := len(msgs); n > 0 && msgs[n-1].Role == "user" && len(msgs[n-1].Content) > 0 && msgs[n-1].Content[0].Type == "tool_result" {
				msgs[n-1].Content = append(msgs[n-1].Content, block)
			} else {
				msgs = append(msgs, anMessage{Role: "user", Content: []anBlock{block}})
			}
		case "assistant":
			var blocks []anBlock
			if m.Content != "" {
				blocks = append(blocks, anBlock{Type: "text", Text: m.Content})
			}
			for _, c := range m.ToolCalls {
				input := c.Arguments
				if len(input) == 0 {
					input = json.RawMessage("{}")
				}
				blocks = append(blocks, anBlock{Type: "tool_use", ID: c.ID, Name: c.Name, Input: input})
			}
			msgs = append(msgs, anMessage{Role: "assistant", Content: blocks})
		default:
			msgs = append(msgs, anMessage{Role: "user", Content: []anBlock{{Type: "text", Text: m.Content}}})
		}
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	body := map[string]any{
		"model":       p.model,
		"system":      req.System,
		"messages":    msgs,
		"max_tokens":  maxTokens,
		"temperature": req.Temperature,
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{"name": t.Name, "description": t.Description, "input_schema": t.InputSchema})
		}
		body["tools"] = tools
	}
	var out struct {
		Model   string    `json:"model"`
		Content []anBlock `json:"content"`
		Usage   struct {
			Input  int `json:"input_tokens"`
			Output int `json:"output_tokens"`
		} `json:"usage"`
	}
	headers := map[string]string{"x-api-key": p.key, "anthropic-version": "2023-06-01"}
	if err := postJSON(ctx, p.endpoint.BaseURL+"/messages", headers, body, &out); err != nil {
		return Response{}, err
	}
	resp := Response{Model: out.Model, Usage: Usage{InputTokens: out.Usage.Input, OutputTokens: out.Usage.Output}}
	for _, b := range out.Content {
		switch b.Type {
		case "text":
			resp.Content += b.Text
		case "tool_use":
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{ID: b.ID, Name: b.Name, Arguments: b.Input})
		}
	}
	if resp.Content == "" && len(resp.ToolCalls) == 0 {
		return Response{}, fmt.Errorf("anthropic: empty answer")
	}
	return resp, nil
}
