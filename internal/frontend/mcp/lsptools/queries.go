package lsptools

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/lsp"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// DiagnosticsInput names one document, or none for the whole workspace.
type DiagnosticsInput struct {
	Path string `json:"path,omitempty" jsonschema:"one document, absolute or relative to the workspace root; omitted, every document of the workspace"`
}

// Diagnostic is one finding of the analysis.
type Diagnostic struct {
	Path     string `json:"path"`
	Range    Range  `json:"range"`
	Severity string `json:"severity" jsonschema:"error, warning, info or hint"`
	Code     string `json:"code,omitempty" jsonschema:"the stable identifier of the check"`
	Message  string `json:"message"`
	Source   string `json:"source,omitempty" jsonschema:"the pass that produced the diagnostic"`
}

// DiagnosticsResult lists diagnostics with counts.
type DiagnosticsResult struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Errors      int          `json:"errors"`
	Warnings    int          `json:"warnings"`
	Documents   int          `json:"documents" jsonschema:"documents analyzed"`
}

// PositionInput is a document position.
type PositionInput struct {
	Path     string           `json:"path" jsonschema:"the document, absolute or relative to the workspace root; a sysml://stdlib/<file> URI for a standard library document"`
	Position toolkit.Position `json:"position" jsonschema:"the position (1-based line and character column)"`
}

// HoverResult is what hovering a position shows.
type HoverResult struct {
	Markdown string `json:"markdown" jsonschema:"the hover text, Markdown; empty when nothing is at the position"`
	Range    *Range `json:"range,omitempty" jsonschema:"the span the hover applies to"`
}

// Location is a span of a document.
type Location struct {
	Path  string `json:"path"`
	Range Range  `json:"range"`
}

// LocationsResult lists locations.
type LocationsResult struct {
	Locations []Location `json:"locations"`
}

// ReferencesInput adds whether the declaration counts as a reference.
type ReferencesInput struct {
	PositionInput
	IncludeDeclaration bool `json:"includeDeclaration,omitempty" jsonschema:"also list the declaration itself"`
}

// PathInput names a document.
type PathInput struct {
	Path string `json:"path" jsonschema:"the document, absolute or relative to the workspace root; a sysml://stdlib/<file> URI for a standard library document"`
}

// Symbol is an element a document declares, with the elements it owns.
type Symbol struct {
	Name           string   `json:"name"`
	Kind           string   `json:"kind" jsonschema:"the LSP symbol kind name (Class, Property, Package, ...)"`
	Detail         string   `json:"detail,omitempty"`
	Range          Range    `json:"range" jsonschema:"the whole declaration"`
	SelectionRange Range    `json:"selectionRange" jsonschema:"the name"`
	Children       []Symbol `json:"children,omitempty"`
}

// DocumentSymbolsResult is a document's outline.
type DocumentSymbolsResult struct {
	Symbols []Symbol `json:"symbols"`
}

// WorkspaceSymbolsInput filters symbols by name.
type WorkspaceSymbolsInput struct {
	Query string `json:"query,omitempty" jsonschema:"case-insensitive substring of the name; empty lists every named element"`
	Limit int    `json:"limit,omitempty" jsonschema:"at most this many matches; default 200"`
}

// WorkspaceSymbol is an element matched by name.
type WorkspaceSymbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Container string `json:"container,omitempty" jsonschema:"the name of the owning element"`
	Location  Location
}

// WorkspaceSymbolsResult lists matches.
type WorkspaceSymbolsResult struct {
	Symbols []WorkspaceSymbol `json:"symbols"`
	Total   int               `json:"total" jsonschema:"matches before the limit"`
}

// CompletionItem is one completion proposal.
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          string `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty" jsonschema:"the text to insert when it differs from the label"`
}

// CompletionResult lists proposals.
type CompletionResult struct {
	Items        []CompletionItem `json:"items"`
	IsIncomplete bool             `json:"isIncomplete,omitempty"`
}

// SignatureHelpResult describes the call around a position.
type SignatureHelpResult struct {
	Signatures      []Signature `json:"signatures"`
	ActiveSignature int         `json:"activeSignature"`
	ActiveParameter int         `json:"activeParameter"`
}

// Signature is one callable signature.
type Signature struct {
	Label         string   `json:"label"`
	Documentation string   `json:"documentation,omitempty"`
	Parameters    []string `json:"parameters"`
}

// RangeInput names a range of a document.
type RangeInput struct {
	Path  string `json:"path" jsonschema:"the document, absolute or relative to the workspace root"`
	Range *Range `json:"range,omitempty" jsonschema:"the lines of interest (1-based); omitted, the whole document"`
}

// InlayHint is text shown inline without being in the document.
type InlayHint struct {
	Position toolkit.Position `json:"position"`
	Label    string           `json:"label"`
	Kind     string           `json:"kind,omitempty" jsonschema:"type or parameter"`
	Tooltip  string           `json:"tooltip,omitempty"`
}

// InlayHintsResult lists hints.
type InlayHintsResult struct {
	Hints []InlayHint `json:"hints"`
}

// CodeAction is a quick fix or refactoring with its edits.
type CodeAction struct {
	Title       string      `json:"title"`
	Kind        string      `json:"kind,omitempty"`
	IsPreferred bool        `json:"isPreferred,omitempty"`
	Diagnostics []string    `json:"diagnostics,omitempty" jsonschema:"messages of the diagnostics the action addresses"`
	Changes     []FileEdits `json:"changes" jsonschema:"the edits the action makes, per document"`
}

// CodeActionsResult lists actions.
type CodeActionsResult struct {
	Actions []CodeAction `json:"actions"`
}

func registerQueries(s *mcp.Server, h *Host) {
	registerDiagnosticTools(s, h)
	registerNavigationTools(s, h)
	registerSymbolTools(s, h)
	registerEditingTools(s, h)
	registerAnnotationTools(s, h)
}

// registerDiagnosticTools adds the tool that analyzes a document or the workspace.
func registerDiagnosticTools(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "workspace_diagnostics",
		Description: "Analyze a SysML v2 / KerML document, or the whole workspace, and list the errors, warnings and hints: " +
			"syntax, unresolved names, type and constraint violations. Positions are 1-based lines and columns.",
		Annotations: toolkit.ReadOnly("Diagnostics"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in DiagnosticsInput) (*mcp.CallToolResult, DiagnosticsResult, error) {
		refs, err := h.diagnosed(ctx, in.Path)
		if err != nil {
			return nil, DiagnosticsResult{}, err
		}
		res := DiagnosticsResult{Diagnostics: []Diagnostic{}, Documents: len(refs)}
		for _, ref := range refs {
			res.Diagnostics = append(res.Diagnostics, h.diagnosticsOf(ref)...)
		}
		res.Errors, res.Warnings = countSeverities(res.Diagnostics)
		return text(renderDiagnostics(res.Diagnostics, fmt.Sprintf("%d documents", len(refs)))), res, nil
	}))
}

// registerNavigationTools adds the tools that explain and follow the name at a position.
func registerNavigationTools(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "hover",
		Description: "Explain the element at a position: its kind, qualified name, type, value and documentation, as Markdown.",
		Annotations: toolkit.ReadOnly("Hover"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in PositionInput) (*mcp.CallToolResult, HoverResult, error) {
		ref, content, pos, err := h.at(ctx, in.Path, in.Position)
		if err != nil {
			return nil, HoverResult{}, err
		}
		hover, err := h.Server.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Position: pos}})
		if err != nil {
			return nil, HoverResult{}, err
		}
		if hover == nil {
			return text("nothing at " + at(h.display(ref.uri()), in.Position)), HoverResult{}, nil
		}
		res := HoverResult{Markdown: hover.Contents.Value}
		if hover.Range != nil {
			r := rangeFromProtocol(content, *hover.Range)
			res.Range = &r
		}
		return text(res.Markdown), res, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "definition",
		Description: "Find where the name at a position is declared. Standard library declarations are reported as sysml://stdlib/<file> resources.",
		Annotations: toolkit.ReadOnly("Go to definition"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in PositionInput) (*mcp.CallToolResult, LocationsResult, error) {
		ref, _, pos, err := h.at(ctx, in.Path, in.Position)
		if err != nil {
			return nil, LocationsResult{}, err
		}
		locs, err := h.Server.Definition(ctx, &protocol.DefinitionParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Position: pos}})
		if err != nil {
			return nil, LocationsResult{}, err
		}
		return h.locations(locs)
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "references",
		Description: "Find every place in the workspace that refers to the element at a position.",
		Annotations: toolkit.ReadOnly("Find references"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in ReferencesInput) (*mcp.CallToolResult, LocationsResult, error) {
		ref, _, pos, err := h.at(ctx, in.Path, in.Position)
		if err != nil {
			return nil, LocationsResult{}, err
		}
		locs, err := h.Server.References(ctx, &protocol.ReferenceParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Position: pos},
			Context:                    protocol.ReferenceContext{IncludeDeclaration: in.IncludeDeclaration},
		})
		if err != nil {
			return nil, LocationsResult{}, err
		}
		return h.locations(locs)
	}))
}

// registerSymbolTools adds the tools that outline a document and search the workspace.
func registerSymbolTools(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name:         "document_symbols",
		Description:  "Outline a document: the elements it declares, nested as they are owned, with their kinds and ranges.",
		Annotations:  toolkit.ReadOnly("Document symbols"),
		OutputSchema: schemaWithRecursion(reflect.TypeFor[DocumentSymbolsResult](), reflect.TypeFor[Symbol](), "symbol"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in PathInput) (*mcp.CallToolResult, DocumentSymbolsResult, error) {
		ref, err := h.document(ctx, in.Path)
		if err != nil {
			return nil, DocumentSymbolsResult{}, err
		}
		content, err := h.content(ref)
		if err != nil {
			return nil, DocumentSymbolsResult{}, err
		}
		raw, err := h.Server.DocumentSymbol(ctx, &protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}})
		if err != nil {
			return nil, DocumentSymbolsResult{}, err
		}
		res := DocumentSymbolsResult{Symbols: []Symbol{}}
		for _, item := range raw {
			switch sym := item.(type) {
			case protocol.DocumentSymbol:
				res.Symbols = append(res.Symbols, convertSymbol(content, sym))
			case *protocol.DocumentSymbol:
				res.Symbols = append(res.Symbols, convertSymbol(content, *sym))
			}
		}
		var b strings.Builder
		writeOutline(&b, res.Symbols, 0)
		return text(strings.TrimSpace(b.String())), res, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "workspace_symbols",
		Description: "Search the workspace's declared elements by name substring.",
		Annotations: toolkit.ReadOnly("Workspace symbols"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in WorkspaceSymbolsInput) (*mcp.CallToolResult, WorkspaceSymbolsResult, error) {
		h.Refresh(ctx)
		syms, err := h.Server.Symbols(ctx, &protocol.WorkspaceSymbolParams{Query: in.Query})
		if err != nil {
			return nil, WorkspaceSymbolsResult{}, err
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 200
		}
		res := WorkspaceSymbolsResult{Symbols: []WorkspaceSymbol{}, Total: len(syms)}
		var b strings.Builder
		for _, sym := range syms {
			if len(res.Symbols) == limit {
				break
			}
			loc := Location{Path: h.display(sym.Location.URI), Range: rangeFromProtocol(h.contentOf(sym.Location.URI), sym.Location.Range)}
			res.Symbols = append(res.Symbols, WorkspaceSymbol{Name: sym.Name, Kind: sym.Kind.String(), Container: sym.ContainerName, Location: loc})
			fmt.Fprintf(&b, "%s %s (%s) at %s\n", sym.Kind.String(), sym.Name, sym.ContainerName, at(loc.Path, loc.Range.Start))
		}
		if res.Total > len(res.Symbols) {
			fmt.Fprintf(&b, "%d of %d matches shown", len(res.Symbols), res.Total)
		}
		return text(strings.TrimSpace(b.String())), res, nil
	}))
}

// registerEditingTools adds the tools an editor offers while typing: completion
// and signature help.
func registerEditingTools(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "completion",
		Description: "Propose completions at a position: keywords, names in scope and members of the type being accessed.",
		Annotations: toolkit.ReadOnly("Completion"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in PositionInput) (*mcp.CallToolResult, CompletionResult, error) {
		ref, _, pos, err := h.at(ctx, in.Path, in.Position)
		if err != nil {
			return nil, CompletionResult{}, err
		}
		list, err := h.Server.Completion(ctx, &protocol.CompletionParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Position: pos}})
		if err != nil {
			return nil, CompletionResult{}, err
		}
		res := CompletionResult{Items: []CompletionItem{}}
		var b strings.Builder
		if list != nil {
			res.IsIncomplete = list.IsIncomplete
			for _, item := range list.Items {
				res.Items = append(res.Items, completionItem(item))
				b.WriteString(completionLine(item))
			}
		}
		return text(strings.TrimSpace(b.String())), res, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "signature_help",
		Description: "Describe the invocation enclosing a position: the callable's parameters and which one the position is at.",
		Annotations: toolkit.ReadOnly("Signature help"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in PositionInput) (*mcp.CallToolResult, SignatureHelpResult, error) {
		ref, _, pos, err := h.at(ctx, in.Path, in.Position)
		if err != nil {
			return nil, SignatureHelpResult{}, err
		}
		help, err := h.Server.SignatureHelp(ctx, &protocol.SignatureHelpParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Position: pos}})
		if err != nil {
			return nil, SignatureHelpResult{}, err
		}
		res := SignatureHelpResult{Signatures: []Signature{}}
		if help == nil {
			return text("no invocation at " + at(h.display(ref.uri()), in.Position)), res, nil
		}
		res.ActiveSignature, res.ActiveParameter = int(help.ActiveSignature), int(help.ActiveParameter)
		var b strings.Builder
		for _, sig := range help.Signatures {
			s := Signature{Label: sig.Label, Documentation: docString(sig.Documentation), Parameters: []string{}}
			for _, p := range sig.Parameters {
				s.Parameters = append(s.Parameters, docString(p.Label))
			}
			res.Signatures = append(res.Signatures, s)
			b.WriteString(sig.Label + "\n")
		}
		return text(strings.TrimSpace(b.String())), res, nil
	}))
}

// registerAnnotationTools adds the tools that annotate a document or range: inlay
// hints and code actions.
func registerAnnotationTools(s *mcp.Server, h *Host) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "inlay_hints",
		Description: "List the inferred types and values the editor would show inline for the features of a document or range.",
		Annotations: toolkit.ReadOnly("Inlay hints"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in RangeInput) (*mcp.CallToolResult, InlayHintsResult, error) {
		ref, content, r, err := h.span(ctx, in.Path, in.Range)
		if err != nil {
			return nil, InlayHintsResult{}, err
		}
		hints, err := h.Server.InlayHint(&lsp.InlayHintParams{TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()}, Range: r})
		if err != nil {
			return nil, InlayHintsResult{}, err
		}
		res := InlayHintsResult{Hints: []InlayHint{}}
		var b strings.Builder
		for _, hint := range hints {
			ih := InlayHint{Position: fromProtocol(content, hint.Position), Label: hint.Label, Tooltip: hint.Tooltip}
			switch hint.Kind {
			case 1:
				ih.Kind = "type"
			case 2:
				ih.Kind = "parameter"
			}
			res.Hints = append(res.Hints, ih)
			fmt.Fprintf(&b, "%s: %s\n", at(h.display(ref.uri()), ih.Position), hint.Label)
		}
		return text(strings.TrimSpace(b.String())), res, nil
	}))
	mcp.AddTool(s, &mcp.Tool{
		Name:        "code_actions",
		Description: "List the quick fixes and refactorings available for a range of a document, each with the edits it would make. Nothing is applied; write the edits with your own file tools or use the rename/format/apply_model_edit tools with apply=true.",
		Annotations: toolkit.ReadOnly("Code actions"),
	}, reading(h, func(ctx context.Context, _ *mcp.CallToolRequest, in RangeInput) (*mcp.CallToolResult, CodeActionsResult, error) {
		ref, content, r, err := h.span(ctx, in.Path, in.Range)
		if err != nil {
			return nil, CodeActionsResult{}, err
		}
		actions, err := h.Server.CodeAction(ctx, &protocol.CodeActionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: ref.uri()},
			Range:        r,
			Context:      protocol.CodeActionContext{Diagnostics: h.diagnosticsWithin(ref, content, r)},
		})
		if err != nil {
			return nil, CodeActionsResult{}, err
		}
		res := CodeActionsResult{Actions: []CodeAction{}}
		var b strings.Builder
		for _, a := range actions {
			ca := CodeAction{Title: a.Title, Kind: string(a.Kind), IsPreferred: a.IsPreferred, Changes: []FileEdits{}}
			for _, d := range a.Diagnostics {
				ca.Diagnostics = append(ca.Diagnostics, d.Message)
			}
			if a.Edit != nil {
				_, edited, err := h.workspaceEdit(ctx, a.Edit, false, "code action")
				if err != nil {
					return nil, CodeActionsResult{}, err
				}
				ca.Changes = edited.Changes
			}
			res.Actions = append(res.Actions, ca)
			fmt.Fprintf(&b, "%s (%s)\n", a.Title, a.Kind)
		}
		return text(strings.TrimSpace(b.String())), res, nil
	}))
}

// diagnosed lists the documents workspace_diagnostics analyzes: the one at path,
// or every document of the workspace but the library's.
func (h *Host) diagnosed(ctx context.Context, path string) ([]docRef, error) {
	if strings.TrimSpace(path) != "" {
		ref, err := h.document(ctx, path)
		if err != nil {
			return nil, err
		}
		if _, err := h.content(ref); err != nil {
			return nil, err
		}
		return []docRef{ref}, nil
	}
	h.Refresh(ctx)
	names := h.Workspace.DocumentNames()
	sort.Strings(names)
	var refs []docRef
	for _, name := range names {
		if !h.Workspace.IsLibraryDocument(name) {
			refs = append(refs, docRef{name: name})
		}
	}
	return refs, nil
}

// countSeverities counts the errors and the warnings among diagnostics.
func countSeverities(diags []Diagnostic) (errors, warnings int) {
	for _, d := range diags {
		switch d.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		}
	}
	return errors, warnings
}

// completionItem converts a protocol completion item, keeping an insert text
// only when it differs from the label.
func completionItem(item protocol.CompletionItem) CompletionItem {
	ci := CompletionItem{Label: item.Label, Detail: item.Detail, Documentation: docString(item.Documentation)}
	if item.Kind != 0 {
		ci.Kind = item.Kind.String()
	}
	if item.InsertText != "" && item.InsertText != item.Label {
		ci.InsertText = item.InsertText
	} else if item.TextEdit != nil && item.TextEdit.NewText != item.Label {
		ci.InsertText = item.TextEdit.NewText
	}
	return ci
}

// completionLine is the text line of a completion item: its label and detail.
func completionLine(item protocol.CompletionItem) string {
	if item.Detail != "" {
		return item.Label + "  " + item.Detail + "\n"
	}
	return item.Label + "\n"
}

// diagnosticsWithin converts the document's own diagnostics whose lines overlap
// r into the protocol's form, for the code actions that address them.
func (h *Host) diagnosticsWithin(ref docRef, content []byte, r protocol.Range) []protocol.Diagnostic {
	var diags []protocol.Diagnostic
	_, raw, _ := h.Workspace.AnalyzedContent(ref.name)
	for _, d := range raw {
		start, end := offsetPosition(content, d.Span.Offset), offsetPosition(content, d.Span.End())
		pr := protocol.Range{Start: toolkit.Position(start).ToProtocol(lineAt(content, uint32(start.Line-1))), End: toolkit.Position(end).ToProtocol(lineAt(content, uint32(end.Line-1)))} // #nosec G115 -- lines are 1-based counts
		if pr.End.Line < r.Start.Line || pr.Start.Line > r.End.Line {
			continue
		}
		diags = append(diags, protocol.Diagnostic{Range: pr, Severity: protocol.DiagnosticSeverity(int(d.Severity) + 1), Code: d.Code, Source: d.Source, Message: d.Message}) // #nosec G115 -- four severities
	}
	return diags
}

// at resolves a document and converts a 1-based position into the protocol's.
func (h *Host) at(ctx context.Context, path string, p toolkit.Position) (docRef, []byte, protocol.Position, error) {
	ref, err := h.document(ctx, path)
	if err != nil {
		return docRef{}, nil, protocol.Position{}, err
	}
	content, err := h.content(ref)
	if err != nil {
		return docRef{}, nil, protocol.Position{}, err
	}
	pos, err := toProtocol(content, p)
	if err != nil {
		return docRef{}, nil, protocol.Position{}, fmt.Errorf("%s: %w", h.display(ref.uri()), err)
	}
	return ref, content, pos, nil
}

// span resolves a document and a range, the whole document when none is given.
func (h *Host) span(ctx context.Context, path string, r *Range) (docRef, []byte, protocol.Range, error) {
	ref, err := h.document(ctx, path)
	if err != nil {
		return docRef{}, nil, protocol.Range{}, err
	}
	content, err := h.content(ref)
	if err != nil {
		return docRef{}, nil, protocol.Range{}, err
	}
	if r == nil {
		ls := lines(content)
		last := len(ls) - 1
		return ref, content, protocol.Range{End: toolkit.Position{Line: last + 1, Column: len([]rune(ls[last])) + 1}.ToProtocol(ls[last])}, nil
	}
	pr, err := rangeToProtocol(content, *r)
	if err != nil {
		return docRef{}, nil, protocol.Range{}, fmt.Errorf("%s: %w", h.display(ref.uri()), err)
	}
	return ref, content, pr, nil
}

// locations converts protocol locations into paths and 1-based ranges.
func (h *Host) locations(locs []protocol.Location) (*mcp.CallToolResult, LocationsResult, error) {
	res := LocationsResult{Locations: []Location{}}
	var b strings.Builder
	for _, loc := range locs {
		l := Location{Path: h.display(loc.URI), Range: rangeFromProtocol(h.contentOf(loc.URI), loc.Range)}
		res.Locations = append(res.Locations, l)
		b.WriteString(at(l.Path, l.Range.Start) + "\n")
	}
	if len(locs) == 0 {
		b.WriteString("none")
	}
	return text(strings.TrimSpace(b.String())), res, nil
}

// diagnosticsOf lists a document's diagnostics in 1-based positions, from the
// workspace's own analysis.
func (h *Host) diagnosticsOf(ref docRef) []Diagnostic {
	out := []Diagnostic{}
	if ref.library {
		return out
	}
	content, diags, ok := h.Workspace.AnalyzedContent(ref.name)
	if !ok {
		return out
	}
	path := h.display(ref.uri())
	for _, d := range diags {
		out = append(out, Diagnostic{
			Path:     path,
			Range:    offsetRange(content, d.Span.Offset, d.Span.Len),
			Severity: d.Severity.String(),
			Code:     d.Code,
			Message:  d.Message,
			Source:   d.Source,
		})
	}
	return out
}

// renderDiagnostics prints diagnostics one per line, as the CLI does.
func renderDiagnostics(diags []Diagnostic, subject string) string {
	if len(diags) == 0 {
		return subject + ": no diagnostics"
	}
	var b strings.Builder
	for _, d := range diags {
		fmt.Fprintf(&b, "%s: %s: %s", at(d.Path, d.Range.Start), d.Severity, d.Message)
		if d.Code != "" {
			fmt.Fprintf(&b, " [%s]", d.Code)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func at(path string, p toolkit.Position) string {
	return fmt.Sprintf("%s:%d:%d", path, p.Line, p.Column)
}

func convertSymbol(content []byte, sym protocol.DocumentSymbol) Symbol {
	out := Symbol{Name: sym.Name, Kind: sym.Kind.String(), Detail: sym.Detail, Range: rangeFromProtocol(content, sym.Range), SelectionRange: rangeFromProtocol(content, sym.SelectionRange)}
	for _, child := range sym.Children {
		out.Children = append(out.Children, convertSymbol(content, child))
	}
	return out
}

func writeOutline(b *strings.Builder, syms []Symbol, depth int) {
	for _, s := range syms {
		fmt.Fprintf(b, "%s%s %s (line %d)\n", strings.Repeat("  ", depth), s.Kind, s.Name, s.Range.Start.Line)
		writeOutline(b, s.Children, depth+1)
	}
}

// docString flattens the protocol's string-or-MarkupContent documentation.
func docString(v any) string {
	switch d := v.(type) {
	case string:
		return d
	case protocol.MarkupContent:
		return d.Value
	case *protocol.MarkupContent:
		if d != nil {
			return d.Value
		}
	case map[string]any:
		if s, ok := d["value"].(string); ok {
			return s
		}
	}
	return ""
}
