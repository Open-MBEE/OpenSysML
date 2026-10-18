package runtimetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// DiagnosticsInput names the model whose diagnostics are reported.
type DiagnosticsInput struct {
	ModelInput
	DisableLint []string `json:"disableLint,omitempty" jsonschema:"lints left out of the diagnostics, by code (sysml -disable-lint): undeclared-signal, port-type-mismatch, deferred-keeper-unmarked or rounded-real-literal"`
	EnableLint  []string `json:"enableLint,omitempty" jsonschema:"opt-in lints kept in the diagnostics, by code (sysml -enable-lint): rounded-real-literal; off unless named, disableLint wins"`
}

func (g *group) registerDiagnostics(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "diagnostics",
		Description: "Report every diagnostic of a parsed model (the GetDiagnostics RPC), as `sysml -validate` does: each with its " +
			"severity, code, message and position. Parse with `strictConformance` for the strict mode of `sysml -validate -strict`; " +
			"`disableLint` and `enableLint` select the lints as `sysml -disable-lint`/`-enable-lint` do. The result is the " +
			"DiagnosticsResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Report diagnostics"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DiagnosticsInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		req := &pb.DiagnosticsRequest{}
		if err := unmarshal(in, m.hash, req); err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.GetDiagnostics(ctx, req)
		if err != nil {
			return nil, nil, toolError(err)
		}
		out, err := result(resp, m)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		if m.parsed != nil {
			fmt.Fprintf(&b, "model %s\n", m.hash)
		}
		if diags := resp.GetDiagnostics(); len(diags) > 0 {
			b.WriteString(strings.TrimRight(diagnosticsText(diags), "\n"))
		} else {
			b.WriteString("no diagnostics")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: b.String()}}}, out, nil
	})
}
