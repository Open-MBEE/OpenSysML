package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPTools are the sysml-mcp tools the MCP arm offers the model.
var MCPTools = []string{"parse", "explain_diagnostic", "symbol", "validate_instance", "tool_catalog"}

// ToolCallRecord is one tool call the model made, with the answer digested.
type ToolCallRecord struct {
	Round     int             `json:"round"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	IsError   bool            `json:"isError,omitempty"`
	// Refused is set when the call exceeded MaxToolCallsPerRound and was not made.
	Refused      bool   `json:"refused,omitempty"`
	ResultDigest string `json:"resultDigest"`
	ResultBytes  int    `json:"resultBytes"`
	// Result keeps the first ResultPreview bytes of the answer the model saw.
	Result string `json:"result,omitempty"`
}

// ResultPreview is how much of a tool answer a record keeps verbatim.
const ResultPreview = 400

// MCPArm runs the sysml-mcp server in-process and bridges it to a provider's
// tool protocol.
type MCPArm struct {
	server  *mcp.Server
	session *sdk.ClientSession
	specs   []ToolSpec
	root    string
}

// StartMCP starts the server over an in-memory transport with a scratch
// workspace root.
func StartMCP(ctx context.Context, version string) (*MCPArm, error) {
	root, err := os.MkdirTemp("", "llm-bench-mcp-")
	if err != nil {
		return nil, err
	}
	srv, err := mcp.New(ctx, mcp.Config{Root: root, Groups: []mcp.Group{mcp.GroupLSP, mcp.GroupRuntime}, Version: version})
	if err != nil {
		_ = os.RemoveAll(root)
		return nil, err
	}
	serverT, clientT := sdk.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverT) }()
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "llm-bench", Version: version}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		srv.Close(ctx)
		_ = os.RemoveAll(root)
		return nil, err
	}
	arm := &MCPArm{server: srv, session: cs, root: root}
	if err := arm.loadSpecs(ctx); err != nil {
		arm.Close(ctx)
		return nil, err
	}
	return arm, nil
}

func (a *MCPArm) loadSpecs(ctx context.Context) error {
	res, err := a.session.ListTools(ctx, nil)
	if err != nil {
		return err
	}
	want := map[string]bool{}
	for _, n := range MCPTools {
		want[n] = true
	}
	for _, t := range res.Tools {
		if !want[t.Name] {
			continue
		}
		schema, err := json.Marshal(t.InputSchema)
		if err != nil {
			return err
		}
		a.specs = append(a.specs, ToolSpec{Name: t.Name, Description: t.Description, InputSchema: schema})
		delete(want, t.Name)
	}
	if len(want) > 0 {
		missing := make([]string, 0, len(want))
		for n := range want {
			missing = append(missing, n)
		}
		sort.Strings(missing)
		return fmt.Errorf("sysml-mcp offers no tool named %s", strings.Join(missing, ", "))
	}
	return nil
}

// Specs are the tool descriptions offered to the model.
func (a *MCPArm) Specs() []ToolSpec { return a.specs }

// Call forwards one tool call and returns the text the model is given.
func (a *MCPArm) Call(ctx context.Context, round int, call ToolCall) (string, ToolCallRecord) {
	var args map[string]any
	if len(call.Arguments) > 0 {
		_ = json.Unmarshal(call.Arguments, &args)
	}
	rec := ToolCallRecord{Round: round, Name: call.Name, Arguments: call.Arguments}
	res, err := a.session.CallTool(ctx, &sdk.CallToolParams{Name: call.Name, Arguments: args})
	var text string
	switch {
	case err != nil:
		text = "tool error: " + err.Error()
		rec.IsError = true
	default:
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*sdk.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		text = b.String()
		if text == "" && res.StructuredContent != nil {
			data, _ := json.Marshal(res.StructuredContent)
			text = string(data)
		}
		rec.IsError = res.IsError
	}
	rec.answer(text)
	return text, rec
}

// Refuse answers a call that exceeded MaxToolCallsPerRound without making it.
func (a *MCPArm) Refuse(round int, call ToolCall) (string, ToolCallRecord) {
	rec := ToolCallRecord{Round: round, Name: call.Name, Arguments: call.Arguments, IsError: true, Refused: true}
	text := fmt.Sprintf("tool budget of %d calls per round exhausted; the call was not made", MaxToolCallsPerRound)
	rec.answer(text)
	return text, rec
}

// answer records the text the model was given: its digest, size and preview.
func (rec *ToolCallRecord) answer(text string) {
	sum := sha256.Sum256([]byte(text))
	rec.ResultDigest = hex.EncodeToString(sum[:])
	rec.ResultBytes = len(text)
	rec.Result = text
	if len(rec.Result) > ResultPreview {
		rec.Result = rec.Result[:ResultPreview] + "…"
	}
}

// Close stops the server and removes the scratch root.
func (a *MCPArm) Close(ctx context.Context) {
	if a.session != nil {
		_ = a.session.Close()
	}
	if a.server != nil {
		a.server.Close(ctx)
	}
	_ = os.RemoveAll(a.root)
}
