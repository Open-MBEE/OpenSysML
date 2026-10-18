package runtimetools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Open-MBEE/OpenSysML/internal/check/codes"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// ExplainDiagnosticInput names the code to explain; empty lists the catalogue.
type ExplainDiagnosticInput struct {
	Code string `json:"code,omitempty" jsonschema:"the diagnostic code, as a diagnostic's code field or a text line's error[code] carries it; empty lists every code with its one-line summary"`
}

// DiagnosticExplanation is one code's catalogue entry.
type DiagnosticExplanation struct {
	Code     string              `json:"code" jsonschema:"the stable code"`
	Severity string              `json:"severity" jsonschema:"error, warning or info"`
	Tier     string              `json:"tier" jsonschema:"the validation tier that emits it"`
	Summary  string              `json:"summary" jsonschema:"one-line summary"`
	Meaning  string              `json:"meaning" jsonschema:"what the diagnostic means and how to resolve it"`
	Spec     string              `json:"spec,omitempty" jsonschema:"the specification clause or validation constraint behind the rule"`
	Rule     string              `json:"rule,omitempty" jsonschema:"for a member of a code family, the rule the member names"`
	Codes    []DiagnosticSummary `json:"codes,omitempty" jsonschema:"with no code asked for, every code with its one-line summary"`
}

// DiagnosticSummary is one line of the catalogue listing.
type DiagnosticSummary struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Tier     string `json:"tier"`
	Summary  string `json:"summary"`
}

func (g *group) registerExplain(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "explain_diagnostic",
		Description: "Explain a diagnostic code (as the CLI's -explain does): what it means, how to resolve it and the specification clause or " +
			"validation constraint behind it. Every diagnostic the `parse`, `diagnostics` and `workspace_diagnostics` tools report carries one of these codes. " +
			"With no code, list every code with its one-line summary.",
		Annotations: toolkit.ReadOnly("Explain diagnostic"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in ExplainDiagnosticInput) (*mcp.CallToolResult, DiagnosticExplanation, error) {
		if in.Code == "" {
			rows := codes.Rows()
			out := DiagnosticExplanation{Codes: make([]DiagnosticSummary, 0, len(rows))}
			for _, r := range rows {
				out.Codes = append(out.Codes, DiagnosticSummary{Code: r.Code, Severity: r.Severity, Tier: string(r.Tier), Summary: r.Summary})
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: codes.List()}}}, out, nil
		}
		e, rule, ok := codes.Lookup(in.Code)
		if !ok {
			_, err := codes.Explain(in.Code)
			return nil, DiagnosticExplanation{}, toolError(err)
		}
		text, _ := codes.Explain(in.Code)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, DiagnosticExplanation{
			Code: in.Code, Severity: e.Severity, Tier: string(e.Tier), Summary: e.Summary, Meaning: e.Meaning, Spec: e.Spec, Rule: rule,
		}, nil
	})
}
