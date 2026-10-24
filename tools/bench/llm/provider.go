package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Message is one turn of the conversation a provider sees.
type Message struct {
	// Role is "user", "assistant" or "tool".
	Role    string `json:"role"`
	Content string `json:"content,omitempty"`
	// ToolCalls are the calls an assistant turn made.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// ToolCallID names the call a tool turn answers.
	ToolCallID string `json:"toolCallId,omitempty"`
	// ToolName names the tool a tool turn answers (Anthropic needs only the id,
	// OpenAI-compatible servers want both).
	ToolName string `json:"toolName,omitempty"`
}

// ToolCall is a tool invocation the model asked for.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolSpec describes a tool offered to the model.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// Request is one completion request.
type Request struct {
	System      string     `json:"system"`
	Messages    []Message  `json:"messages"`
	Tools       []ToolSpec `json:"tools,omitempty"`
	Temperature float64    `json:"temperature"`
	MaxTokens   int        `json:"maxTokens"`
}

// Usage is the token accounting a provider reports.
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// Response is what the model answered: text, tool calls, or both.
type Response struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	Usage     Usage      `json:"usage"`
	// Model is the model identifier the provider reported, when it does.
	Model string `json:"model,omitempty"`
}

// Provider completes a request against one model.
type Provider interface {
	Name() string
	Model() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// ErrMissingKey reports that a provider cannot run because its environment
// variable is unset; the runner skips the provider rather than failing.
type ErrMissingKey struct {
	Provider string
	Variable string
}

func (e ErrMissingKey) Error() string {
	return fmt.Sprintf("%s: %s is not set; skipping", e.Provider, e.Variable)
}

// Endpoint is one OpenAI-compatible or Anthropic endpoint the driver knows.
type Endpoint struct {
	Name    string
	BaseURL string
	// KeyVar is the environment variable the key is read from; empty means the
	// endpoint needs none (local servers).
	KeyVar string
	// Anthropic selects the Messages API instead of chat completions.
	Anthropic bool
}

// Endpoints are the built-in providers by name.
var Endpoints = map[string]Endpoint{
	"openai":     {Name: "openai", BaseURL: "https://api.openai.com/v1", KeyVar: "OPENAI_API_KEY"},
	"xai":        {Name: "xai", BaseURL: "https://api.x.ai/v1", KeyVar: "XAI_API_KEY"},
	"openrouter": {Name: "openrouter", BaseURL: "https://openrouter.ai/api/v1", KeyVar: "OPENROUTER_API_KEY"},
	"ollama":     {Name: "ollama", BaseURL: "http://localhost:11434/v1"},
	"vllm":       {Name: "vllm", BaseURL: "http://localhost:8000/v1"},
	"anthropic":  {Name: "anthropic", BaseURL: "https://api.anthropic.com/v1", KeyVar: "ANTHROPIC_API_KEY", Anthropic: true},
}

// NewProvider builds the named provider for a model, reading its key from the
// environment. baseURL overrides the endpoint's default when non-empty.
func NewProvider(name, model, baseURL string) (Provider, error) {
	ep, ok := Endpoints[name]
	if !ok {
		names := make([]string, 0, len(Endpoints))
		for n := range Endpoints {
			names = append(names, n)
		}
		return nil, fmt.Errorf("unknown provider %q (known: %s, replay)", name, strings.Join(names, ", "))
	}
	if baseURL != "" {
		ep.BaseURL = baseURL
	}
	key := ""
	if ep.KeyVar != "" {
		key = os.Getenv(ep.KeyVar)
		if key == "" {
			return nil, ErrMissingKey{Provider: name, Variable: ep.KeyVar}
		}
	}
	if ep.Anthropic {
		return &anthropicProvider{endpoint: ep, model: model, key: key}, nil
	}
	return &openAIProvider{endpoint: ep, model: model, key: key}, nil
}
