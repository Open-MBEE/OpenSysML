package runtimetools

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// ParseInput names the documents to parse as one model.
type ParseInput struct {
	Paths             []string `json:"paths,omitempty" jsonschema:"model files to parse as one model, absolute or relative to the workspace root; a name declared in one resolves in the others"`
	Sources           []Source `json:"sources,omitempty" jsonschema:"documents given inline, parsed with paths as one model"`
	StrictConformance bool     `json:"strictConformance,omitempty" jsonschema:"judge the documents as conforming SysML v2: notation no pinned production admits is an error rather than a warning"`
	BaseModelHash     string   `json:"baseModelHash,omitempty" jsonschema:"the modelHash of an earlier parse of these documents; the answer's affected then lists the documents whose results may differ from it"`
}

// SymbolInput names one element of a model.
type SymbolInput struct {
	ModelInput
	SymbolID string `json:"symbolId" jsonschema:"the element's qualified name, as Demo::Vehicle::mass"`
}

func (g *group) registerParse(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "parse",
		Description: "Parse SysML v2 / KerML documents as one model and return its modelHash, the root namespace of each document and " +
			"every diagnostic (the ParseSources RPC). Parsing is content-addressed: the same documents give the same modelHash, and the " +
			"hash is what every other runtime tool takes as modelHash, so parse once and pass the hash, or give those tools the same " +
			"paths/sources and they parse implicitly. A model is only readable when no diagnostic has severity error. The result is the " +
			"ParseSourcesResponse as protojson writes it; the text lists each root and the diagnostics as the CLI prints them.",
		Annotations: toolkit.ReadOnly("Parse model"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ParseInput) (*mcp.CallToolResult, Result, error) {
		if len(in.Paths)+len(in.Sources) == 0 {
			return nil, nil, argumentError("paths and/or sources must name at least one document")
		}
		resp, err := g.parse(ctx, ModelInput{Paths: in.Paths, Sources: in.Sources, StrictConformance: in.StrictConformance}, in.BaseModelHash)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "model %s\n", resp.GetModelHash())
		for i, root := range resp.GetRoots() {
			fmt.Fprintf(&b, "document %d declares: %s\n", i+1, strings.Join(root.GetChildIds(), ", "))
		}
		errors := 0
		for _, d := range resp.GetDiagnostics() {
			if d.GetSeverity() == "error" {
				errors++
			}
		}
		fmt.Fprintf(&b, "%d diagnostics, %d errors\n", len(resp.GetDiagnostics()), errors)
		b.WriteString(diagnosticsText(resp.GetDiagnostics()))
		if len(resp.GetAffected()) > 0 {
			fmt.Fprintf(&b, "affected since %s: %s\n", in.BaseModelHash, strings.Join(resp.GetAffected(), ", "))
		}
		return answer(resp, model{}, b.String())
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "symbol",
		Description: "Describe one element of a parsed model by qualified name (the GetSymbol RPC): its kind, metadata, children, " +
			"attributes with their declared values, static type facts, multiplicity and specializations. An unknown name answers with " +
			"`error` set rather than failing. The result is the SymbolResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Describe symbol"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SymbolInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.GetSymbol(ctx, &pb.GetSymbolRequest{ModelHash: m.hash, SymbolId: in.SymbolID})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return answer(resp, m, symbolText(resp))
	})
}

func symbolText(resp *pb.SymbolResponse) string {
	if resp.GetError() != "" {
		return errorText(resp.GetError())
	}
	sym := resp.GetSymbol()
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", sym.GetKind(), sym.GetId())
	if meta := sym.GetMetadata(); len(meta) > 0 {
		keys := slices.Sorted(maps.Keys(meta))
		for i, k := range keys {
			keys[i] = k + "=" + meta[k]
		}
		fmt.Fprintf(&b, "metadata: %s\n", strings.Join(keys, ", "))
	}
	if ti := sym.GetTypeInfo(); ti != nil && ti.GetDeclared() != "" {
		fmt.Fprintf(&b, "type: %s", ti.GetDeclared())
		if ti.GetResolvedId() != "" && ti.GetResolvedId() != ti.GetDeclared() {
			fmt.Fprintf(&b, " (%s)", ti.GetResolvedId())
		}
		b.WriteByte('\n')
	}
	for _, sp := range sym.GetSpecializations() {
		fmt.Fprintf(&b, "%s %s\n", sp.GetKind(), sp.GetTargetId())
	}
	for _, a := range sym.GetAttributes() {
		if a.GetValue() != nil {
			fmt.Fprintf(&b, "attribute %s = %s\n", a.GetName(), valueText(a.GetValue()))
		} else {
			fmt.Fprintf(&b, "attribute %s\n", a.GetName())
		}
	}
	if len(sym.GetChildIds()) > 0 {
		fmt.Fprintf(&b, "children: %s\n", strings.Join(sym.GetChildIds(), ", "))
	}
	if n := sym.GetWithheldLibraryAttributes(); n > 0 {
		fmt.Fprintf(&b, "%d library-inherited attributes withheld\n", n)
	}
	return b.String()
}
