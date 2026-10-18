package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// openAIProvider speaks the chat-completions protocol OpenAI, xAI, OpenRouter,
// Ollama and vLLM share.
type openAIProvider struct {
	endpoint Endpoint
	model    string
	key      string
}

func (p *openAIProvider) Name() string  { return p.endpoint.Name }
func (p *openAIProvider) Model() string { return p.model }

type oaMessage struct {
	Role       string       `json:"role"`
	Content    string       `json:"content,omitempty"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
	Name       string       `json:"name,omitempty"`
}

type oaToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (p *openAIProvider) Complete(ctx context.Context, req Request) (Response, error) {
	msgs := []oaMessage{{Role: "system", Content: req.System}}
	for _, m := range req.Messages {
		om := oaMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID, Name: m.ToolName}
		for _, c := range m.ToolCalls {
			var tc oaToolCall
			tc.ID, tc.Type = c.ID, "function"
			tc.Function.Name = c.Name
			tc.Function.Arguments = string(c.Arguments)
			om.ToolCalls = append(om.ToolCalls, tc)
		}
		msgs = append(msgs, om)
	}
	body := map[string]any{
		"model":       p.model,
		"messages":    msgs,
		"temperature": req.Temperature,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": t.Name, "description": t.Description, "parameters": t.InputSchema,
				},
			})
		}
		body["tools"] = tools
	}
	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message oaMessage `json:"message"`
		} `json:"choices"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
		} `json:"usage"`
	}
	headers := map[string]string{}
	if p.key != "" {
		headers["Authorization"] = "Bearer " + p.key
	}
	if err := postJSON(ctx, p.endpoint.BaseURL+"/chat/completions", headers, body, &out); err != nil {
		return Response{}, err
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("%s: no choices in response", p.endpoint.Name)
	}
	m := out.Choices[0].Message
	resp := Response{Content: m.Content, Model: out.Model, Usage: Usage{InputTokens: out.Usage.Prompt, OutputTokens: out.Usage.Completion}}
	for _, tc := range m.ToolCalls {
		args := json.RawMessage(tc.Function.Arguments)
		if !json.Valid(args) {
			args, _ = json.Marshal(map[string]string{"raw": tc.Function.Arguments})
		}
		resp.ToolCalls = append(resp.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: args})
	}
	return resp, nil
}

var httpClient = &http.Client{Timeout: 10 * time.Minute}

// postJSON posts a JSON body and decodes a JSON answer; the error text carries
// the status and the body but never the request headers.
func postJSON(ctx context.Context, url string, headers map[string]string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return &HTTPError{Status: resp.StatusCode, Body: string(answer)}
	}
	return json.Unmarshal(answer, out)
}

// HTTPError is a non-2xx answer; status 429 is quota exhaustion.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	body := e.Body
	if len(body) > 600 {
		body = body[:600] + "…"
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, body)
}

// Quota reports whether the error is a rate or quota limit.
func (e *HTTPError) Quota() bool { return e.Status == http.StatusTooManyRequests }
