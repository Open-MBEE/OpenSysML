package runtimetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// ConvertInput asks for a model in another representation.
type ConvertInput struct {
	ModelInput
	FilePath                string   `json:"filePath,omitempty" jsonschema:"a file to convert, read afresh in the format its extension names (or fromFormat), absolute or relative to the workspace root; instead of modelHash, paths or sources"`
	Content                 *string  `json:"content,omitempty" jsonschema:"the document to convert, inline (an empty string is an empty document); fromFormat must then say what it is"`
	FromFormat              string   `json:"fromFormat,omitempty" jsonschema:"sysml, kerml, text, ttl, turtle, rdf, api-json or json; empty infers from filePath's extension and is notation for a parsed model"`
	ToFormat                string   `json:"toFormat" jsonschema:"format to write: sysml, kerml, text, ttl, turtle, rdf, api-json or json"`
	TolerateSyntaxErrors    bool     `json:"tolerateSyntaxErrors,omitempty" jsonschema:"notation to notation only: write the notation back out even when the parser could not read all of it, reporting syntax errors as diagnostics"`
	IDForm                  string   `json:"idForm,omitempty" jsonschema:"how derived element ids are spelled when notation is written as a graph (ttl or api-json): qualified (the default) or uuid"`
	Documents               []string `json:"documents,omitempty" jsonschema:"for a parsed model, the documents whose elements are written, named as the parse named them; empty writes every document"`
	Compact                 bool     `json:"compact,omitempty" jsonschema:"for api-json from notation: write the compact document (api-json-compact/1) instead of the element array"`
	OmitDerived             bool     `json:"omitDerived,omitempty" jsonschema:"with compact: leave out every derived property of the metamodel except keepDerived"`
	KeepDerived             []string `json:"keepDerived,omitempty" jsonschema:"with omitDerived: the derived properties still written, as owner or qualifiedName"`
	IncludeLibraryStubs     bool     `json:"includeLibraryStubs,omitempty" jsonschema:"for api-json: include ownerless standard-library stubs for legacy readers; off by default"`
	IncludeCheckDiagnostics bool     `json:"includeCheckDiagnostics,omitempty" jsonschema:"notation sources: report the semantic check's diagnostics alongside the output, each with its severity and code as parse reports them; off, only syntax errors are reported"`
}

// MigrateInput asks for a SysML v1 model migrated to v2.
type MigrateInput struct {
	FilePath      string  `json:"filePath,omitempty" jsonschema:"the v1 model file to migrate (.xmi, .uml or .mdzip), absolute or relative to the workspace root"`
	Content       *string `json:"content,omitempty" jsonschema:"the v1 document inline, base64-encoded (an .mdzip archive is binary); fromFormat must then say what it is"`
	FromFormat    string  `json:"fromFormat,omitempty" jsonschema:"xmi (UML XMI 2.5.1 with the SysML profile), uml (Eclipse UML2) or mdzip (Cameo/MagicDraw); empty infers from filePath's extension"`
	ToFormat      string  `json:"toFormat" jsonschema:"format to write: sysml, kerml, text, ttl, turtle, rdf, api-json or json"`
	Report        bool    `json:"report,omitempty" jsonschema:"carry every element's verdict (mapped, approximated, unmapped, skipped) and the report text; the summary and counts come regardless"`
	Results       bool    `json:"results,omitempty" jsonschema:"carry the result snapshots the v1 tool stored with the model, as sysml -compare-results reads them"`
	LayoutPath    string  `json:"layoutPath,omitempty" jsonschema:"an MTIP export of the model's diagrams (Model_mtip.xml) laying the migrated views out, absolute or relative to the workspace root"`
	LayoutContent string  `json:"layoutContent,omitempty" jsonschema:"the MTIP export inline, instead of layoutPath"`
	ImageBaseURL  string  `json:"imageBaseUrl,omitempty" jsonschema:"absolute http(s) URL a server-relative image reference in a v1 comment is resolved against"`
	Strict        bool    `json:"strict,omitempty" jsonschema:"write only notation a pinned SysML v2 production admits; a construct whose only v2 form is an OpenSysML extension is reported unmapped instead"`
}

// ApplyEditsInput applies source-preserving edits to a parsed model.
type ApplyEditsInput struct {
	ModelInput
	Operations      []map[string]any `json:"operations,omitempty" jsonschema:"edits to apply, all or none, each a protojson EditOperation with one arm: {\"setValue\":{\"target\":qualified name,\"value\":expression text}}, {\"rename\":{\"target\":..,\"newName\":..}}, {\"addMember\":{\"owner\":..,\"kind\":\"part\",\"name\":..,...}}, {\"delete\":{\"target\":..}}, {\"move\":..}, {\"addConnection\":..}, {\"addSatisfy\":..}, {\"addRequirementConstraint\":..}, {\"addTransition\":..}, {\"addImport\":..}, {\"addDocumentation\":..}, {\"addVerify\":..}, {\"addMetadata\":..}, {\"addMetadataPrefix\":..}, {\"addSequence\":..}, {\"addComment\":..}, {\"addNote\":..}; see api/proto/sysml.proto for each arm's fields"`
	Document        string           `json:"document,omitempty" jsonschema:"the document whose declarations the operations target, named as the parse named it; empty is the model's first document"`
	AcceptDocuments bool             `json:"acceptDocuments,omitempty" jsonschema:"read the edited documents from the documents field, which a model of several documents needs; without it such a model is refused"`
}

func (g *group) registerConversion(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "convert",
		Description: "Convert a SysML v2 model between the representations OpenSysML writes — textual notation, RDF Turtle, the API's " +
			"JSON element form — (the Convert RPC, the CLI's -convert). Name the model by modelHash, paths or sources (the source the parse " +
			"read is converted), or by filePath or inline content read afresh. Nothing is written to disk: the converted document comes " +
			"back as `content` for the agent to write. A SysML v1 model is refused with INVALID_ARGUMENT: use migrate. The result is the " +
			"ConvertResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Convert model"),
	}, g.convert)

	mcp.AddTool(s, &mcp.Tool{
		Name: "migrate",
		Description: "Migrate a SysML v1 model — UML XMI with the SysML profile, an Eclipse UML2 .uml file or a Cameo/MagicDraw .mdzip " +
			"archive — to SysML v2 in one of the representations convert writes (the Migrate RPC, the CLI's -migrate). Migration is not " +
			"lossless: every v1 element is mapped, approximated, left unmapped or skipped, and `report` accounts for each. Nothing is " +
			"written to disk: the migrated model comes back as `content`, with any image files it refers to in `files`. The result is " +
			"the MigrateResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Migrate SysML v1 model"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in MigrateInput) (*mcp.CallToolResult, Result, error) {
		if (in.FilePath == "") == (in.Content == nil) {
			return nil, nil, argumentError("migrate needs exactly one source: filePath or content")
		}
		if in.LayoutPath != "" && in.LayoutContent != "" {
			return nil, nil, argumentError("give layoutPath or layoutContent, not both")
		}
		req := &pb.MigrateRequest{}
		if err := unmarshal(in, "", req); err != nil {
			return nil, nil, err
		}
		if path, ok := req.GetSource().(*pb.MigrateRequest_FilePath); ok {
			resolved, err := g.path(path.FilePath, sourceNameRole(req.FromFormat))
			if err != nil {
				return nil, nil, err
			}
			path.FilePath = resolved
		}
		if layout, ok := req.GetLayout().(*pb.MigrateRequest_LayoutPath); ok {
			resolved, err := g.path(layout.LayoutPath, toolkit.NameIgnored)
			if err != nil {
				return nil, nil, err
			}
			layout.LayoutPath = resolved
		}
		resp, err := g.svc.Migrate(ctx, req)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return answer(resp, model{}, migrateText(resp))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "apply_edits",
		Description: "Apply source-preserving edits to a parsed model and return the edited notation (the ApplyEdits RPC): set a " +
			"value, rename with every reference, add a member, import, documentation, comment, connection, transition, satisfy or " +
			"metadata, delete, move. Edits are located from the parsed spans, so comments and layout outside them are kept, and the " +
			"result is re-parsed and validated before it is answered; a refused batch answers `error` and `failure` naming why, and " +
			"nothing is applied. Nothing is written to disk: the edited text comes back as `content` (one document) or `documents` " +
			"for the agent to write. The result is the ApplyEditsResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Apply edits"),
	}, g.applyEdits)
}

// convert serves the convert tool: it resolves the named source and converts it.
func (g *group) convert(ctx context.Context, _ *mcp.CallToolRequest, in ConvertInput) (*mcp.CallToolResult, Result, error) {
	m, err := g.convertSource(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	req := &pb.ConvertRequest{}
	if err := unmarshal(in, m.hash, req); err != nil {
		return nil, nil, err
	}
	if path, ok := req.GetSource().(*pb.ConvertRequest_FilePath); ok {
		resolved, err := g.path(path.FilePath, sourceNameRole(req.FromFormat))
		if err != nil {
			return nil, nil, err
		}
		path.FilePath = resolved
	}
	resp, err := g.svc.Convert(ctx, req)
	if err != nil {
		return nil, nil, toolError(err)
	}
	return answer(resp, m, convertText(resp))
}

// applyEdits serves the apply_edits tool over the resolved model.
func (g *group) applyEdits(ctx context.Context, _ *mcp.CallToolRequest, in ApplyEditsInput) (*mcp.CallToolResult, Result, error) {
	m, err := g.resolve(ctx, in.ModelInput)
	if err != nil {
		return nil, nil, err
	}
	req := &pb.ApplyEditsRequest{}
	if err := unmarshal(in, m.hash, req); err != nil {
		return nil, nil, err
	}
	resp, err := g.svc.ApplyEdits(ctx, req)
	if err != nil {
		return nil, nil, toolError(err)
	}
	return answer(resp, m, applyEditsText(resp))
}

// convertSource checks that convert names exactly one source and resolves the
// model when the source is one; a file path or inline content resolves nothing.
func (g *group) convertSource(ctx context.Context, in ConvertInput) (model, error) {
	named := in.ModelHash != "" || len(in.Paths)+len(in.Sources) > 0
	sources := 0
	for _, given := range []bool{named, in.FilePath != "", in.Content != nil} {
		if given {
			sources++
		}
	}
	if sources != 1 {
		return model{}, argumentError("convert needs exactly one source: a model (modelHash, paths or sources), filePath, or content")
	}
	if !named {
		return model{}, nil
	}
	return g.resolve(ctx, in.ModelInput)
}

// convertText reports a conversion: its error or notice, diagnostics and content.
func convertText(resp *pb.ConvertResponse) string {
	var b strings.Builder
	b.WriteString(errorText(resp.GetError()))
	if resp.GetExperimentalNotice() != "" {
		b.WriteString("notice: " + resp.GetExperimentalNotice() + "\n")
	}
	b.WriteString(diagnosticsText(resp.GetDiagnostics()))
	if resp.GetError() == "" {
		fmt.Fprintf(&b, "%s -> %s:\n%s", resp.GetFromFormat(), resp.GetToFormat(), resp.GetContent())
	}
	return b.String()
}

// migrateText reports a migration: its error or notice, report, files and content.
func migrateText(resp *pb.MigrateResponse) string {
	var b strings.Builder
	b.WriteString(errorText(resp.GetError()))
	if resp.GetExperimentalNotice() != "" {
		b.WriteString("notice: " + resp.GetExperimentalNotice() + "\n")
	}
	if report := resp.GetReport(); report != nil {
		if report.GetSummary() != "" {
			b.WriteString(report.GetSummary() + "\n")
		}
		if report.GetText() != "" {
			b.WriteString(report.GetText())
			if !strings.HasSuffix(report.GetText(), "\n") {
				b.WriteByte('\n')
			}
		}
	}
	for _, f := range resp.GetFiles() {
		fmt.Fprintf(&b, "file %s (%d bytes)\n", f.GetPath(), len(f.GetContent()))
	}
	if resp.GetError() == "" {
		fmt.Fprintf(&b, "%s -> %s:\n%s", resp.GetFromFormat(), resp.GetToFormat(), resp.GetContent())
	}
	return b.String()
}

// applyEditsText reports an edit batch: its refusal, the edits applied, the
// diagnostics and the edited documents.
func applyEditsText(resp *pb.ApplyEditsResponse) string {
	var b strings.Builder
	if resp.GetError() != "" {
		fmt.Fprintf(&b, "error: %s (%s)\n", resp.GetError(), resp.GetFailure().String())
		for _, r := range resp.GetReferrers() {
			fmt.Fprintf(&b, "referrer %s in %s\n", r.GetName(), r.GetDocument())
		}
		for _, r := range resp.GetReferringElements() {
			fmt.Fprintf(&b, "referrer %s\n", r)
		}
	}
	for _, a := range resp.GetApplied() {
		fmt.Fprintf(&b, "edit %d on %s", a.GetOperationIndex(), a.GetTarget())
		if a.GetDocument() != "" {
			fmt.Fprintf(&b, " in %s", a.GetDocument())
		}
		fmt.Fprintf(&b, ": %q -> %q\n", a.GetOldText(), a.GetNewText())
	}
	b.WriteString(diagnosticsText(resp.GetDiagnostics()))
	switch {
	case len(resp.GetDocuments()) > 0:
		for _, d := range resp.GetDocuments() {
			fmt.Fprintf(&b, "--- %s\n%s", d.GetName(), d.GetContent())
			if !strings.HasSuffix(d.GetContent(), "\n") {
				b.WriteByte('\n')
			}
		}
	case resp.GetError() == "":
		b.WriteString(resp.GetContent())
	}
	return b.String()
}

// sourceNameRole is what the service reads from a convert or migrate source's
// name: its format and language, or only its language once fromFormat is given.
func sourceNameRole(fromFormat string) toolkit.NameRole {
	if fromFormat != "" {
		return toolkit.NameLanguage
	}
	return toolkit.NameFormat
}
