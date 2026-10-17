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
}

func (g *group) registerDiagnostics(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "diagnostics",
		Description: "Report every diagnostic of a parsed model (the GetDiagnostics RPC), as `sysml -validate` does: each with its " +
			"severity, code, message and position. Parse with `strictConformance` for the strict mode of `sysml -validate -strict`. " +
			"The result is the DiagnosticsResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Report diagnostics"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DiagnosticsInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.GetDiagnostics(ctx, &pb.DiagnosticsRequest{ModelHash: m.hash})
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
