package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// documentModel is a native telescope model with a query library and a document
// definition, so every document-authoring feature has something to work on.
const documentModel = `package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	part def Subsystem {
		attribute mass : Real;
	}

	part telescope {
		part optics : Subsystem {
			attribute redefines mass = 8.5;
		}
	}

	/* Subsystems finds every part usage below the root. */
	calc def Subsystems :> Query {
		in root : Element;
		WhereType(
			source = Descendants(source = root, maxDepth = 3),
			type = "PartUsage"
		)
	}

	calc def SubsystemTable :> Query {
		in root : Element;
		Project(
			source = Subsystems(root = root),
			properties = ("name", "mass")
		)
	}

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part masses : Table {
			attribute redefines caption = "All subsystems by mass";
			calc rows : SubsystemTable {
				in root = telescope;
			}
		}
	}
}
`

func openDocumentModel(t *testing.T) (*model.Workspace, *Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	name := uri.File("/tmp/document.sysml").Filename()
	ws.Open(name, []byte(documentModel), 1)
	return ws, s, name
}

func documentPosParams(name string, src, anchor string, t *testing.T) protocol.TextDocumentPositionParams {
	t.Helper()
	off := strings.Index(src, anchor)
	if off < 0 {
		t.Fatalf("anchor %q not in fixture", anchor)
	}
	return protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
		Position:     offsetToPosition([]byte(src), off),
	}
}

func TestDefinitionQueryInvocationToCalcDef(t *testing.T) {
	_, s, name := openDocumentModel(t)
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: documentPosParams(name, documentModel, "SubsystemTable {", t),
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1", len(locs))
	}
	want := offsetToPosition([]byte(documentModel), strings.Index(documentModel, "calc def SubsystemTable"))
	if locs[0].Range.Start.Line != want.Line {
		t.Errorf("decl line = %d, want %d (calc def SubsystemTable)", locs[0].Range.Start.Line, want.Line)
	}
}

func TestDefinitionBindingNameToQueryParameter(t *testing.T) {
	_, s, name := openDocumentModel(t)
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: documentPosParams(name, documentModel, "root = telescope", t),
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1 (the query's `in root` parameter)", len(locs))
	}
	// SubsystemTable's own `in root : Element;`, not Subsystems'.
	table := strings.Index(documentModel, "calc def SubsystemTable")
	want := offsetToPosition([]byte(documentModel), table+strings.Index(documentModel[table:], "in root"))
	if locs[0].Range.Start.Line != want.Line {
		t.Errorf("param line = %d, want %d (SubsystemTable's in root)", locs[0].Range.Start.Line, want.Line)
	}
}

func TestDefinitionBindingValueToModelElement(t *testing.T) {
	_, s, name := openDocumentModel(t)
	locs, err := s.Definition(context.Background(), &protocol.DefinitionParams{
		TextDocumentPositionParams: documentPosParams(name, documentModel, "telescope;", t),
	})
	if err != nil {
		t.Fatalf("Definition err = %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("locations = %d, want 1 (part telescope)", len(locs))
	}
	want := offsetToPosition([]byte(documentModel), strings.Index(documentModel, "part telescope"))
	if locs[0].Range.Start.Line != want.Line {
		t.Errorf("decl line = %d, want %d (part telescope)", locs[0].Range.Start.Line, want.Line)
	}
}

func TestHoverQueryInvocationShowsQueryDef(t *testing.T) {
	_, s, name := openDocumentModel(t)
	hov, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: documentPosParams(name, documentModel, "SubsystemTable {", t),
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hov == nil || !strings.Contains(hov.Contents.Value, "calc def SubsystemTable") {
		t.Fatalf("hover = %+v, want the query's calc def signature", hov)
	}
}

func TestHoverQueryReferenceShowsDocComment(t *testing.T) {
	_, s, name := openDocumentModel(t)
	hov, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: documentPosParams(name, documentModel, "Subsystems(root", t),
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hov == nil || !strings.Contains(hov.Contents.Value, "finds every part usage") {
		t.Fatalf("hover = %+v, want the referenced query's doc comment", hov)
	}
}

func TestHoverDocumentLibraryType(t *testing.T) {
	_, s, name := openDocumentModel(t)
	hov, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: documentPosParams(name, documentModel, "Document {", t),
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hov == nil || !strings.Contains(hov.Contents.Value, "part def") ||
		!strings.Contains(hov.Contents.Value, "Document") {
		t.Fatalf("hover = %+v, want the library Document part def", hov)
	}
}

func completionLabels(t *testing.T, s *Server, name, anchor string) []string {
	t.Helper()
	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: documentPosParams(name, documentModel, anchor, t),
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	labels := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		labels = append(labels, item.Label)
	}
	return labels
}

func TestCompletionCalcTypingPositionOffersQueries(t *testing.T) {
	_, s, name := openDocumentModel(t)
	labels := completionLabels(t, s, name, "SubsystemTable {")
	if !containsLabel(labels, "SubsystemTable") || !containsLabel(labels, "Subsystems") {
		t.Fatalf("labels = %v, want the workspace query definitions", labels)
	}
	if containsLabel(labels, "Subsystem") || containsLabel(labels, "telescope") {
		t.Fatalf("labels = %v, want no non-query part definitions or usages", labels)
	}
	if !containsLabel(labels, "DocumentQueries") {
		t.Fatalf("labels = %v, want packages so a qualified query stays reachable", labels)
	}
}

func TestCompletionBindingPositionOffersQueryParameters(t *testing.T) {
	_, s, name := openDocumentModel(t)
	labels := completionLabels(t, s, name, "root = telescope")
	if len(labels) != 1 || labels[0] != "root" {
		t.Fatalf("labels = %v, want exactly the query's `in` parameters", labels)
	}
}

// queryLibModel is a query library another package imports, so completion is
// exercised across documents and through package qualifiers.
const queryLibModel = `package QueryLib {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	calc def Everything :> Query {
		in root : Element;
		Descendants(source = root, maxDepth = 3)
	}

	calc def NoInputs :> Query {
		Descendants(source = Everything, maxDepth = 1)
	}

	part def Widget;
}
`

const reportModel = `package Reports {
	private import DocumentQueries::*;
	private import QueryLib::*;

	alias TableQuery for QueryLib::Everything;

	part def WidgetReport :> Document {
		attribute redefines title = "Widgets";

		part w : Table {
			calc rows : Everything {
				in root = QueryLib;
			}
		}

		part all : Table {
			calc every : QueryLib:: {
			}
		}

		part none : Table {
			calc bare : NoInputs {
			}
		}

		part q : Table {
			calc qual : QueryLib::NoInputs {
			}
		}
	}
}
`

func openReportModel(t *testing.T) (*Server, string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	ws.Open(uri.File("/tmp/lib.sysml").Filename(), []byte(queryLibModel), 1)
	name := uri.File("/tmp/report.sysml").Filename()
	ws.Open(name, []byte(reportModel), 1)
	return s, name
}

func completionLabelsIn(t *testing.T, s *Server, name, src, anchor string, delta int) []string {
	t.Helper()
	off := strings.Index(src, anchor)
	if off < 0 {
		t.Fatalf("anchor %q not in fixture", anchor)
	}
	list, err := s.Completion(context.Background(), &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(src), off+delta),
		},
	})
	if err != nil {
		t.Fatalf("Completion err = %v", err)
	}
	labels := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		labels = append(labels, item.Label)
	}
	return labels
}

func TestCompletionCalcTypingPositionOffersImportedQueries(t *testing.T) {
	s, name := openReportModel(t)
	labels := completionLabelsIn(t, s, name, reportModel, "Everything {", 0)
	if !containsLabel(labels, "Everything") || !containsLabel(labels, "NoInputs") {
		t.Fatalf("labels = %v, want the queries imported from QueryLib", labels)
	}
	if containsLabel(labels, "Widget") {
		t.Fatalf("labels = %v, want no imported non-query definitions", labels)
	}
}

func TestCompletionQualifiedCalcTypingPositionFiltersToQueries(t *testing.T) {
	s, name := openReportModel(t)
	anchor := "QueryLib:: {"
	labels := completionLabelsIn(t, s, name, reportModel, anchor, len("QueryLib::"))
	if !containsLabel(labels, "Everything") || !containsLabel(labels, "NoInputs") {
		t.Fatalf("labels = %v, want QueryLib's query definitions", labels)
	}
	if containsLabel(labels, "Widget") {
		t.Fatalf("labels = %v, want no non-query package members", labels)
	}
}

func TestCompletionBindingPositionOfParameterlessQueryOffersNothing(t *testing.T) {
	s, name := openReportModel(t)
	anchor := "calc bare : NoInputs {"
	labels := completionLabelsIn(t, s, name, reportModel, anchor, len(anchor))
	if len(labels) != 0 {
		t.Fatalf("labels = %v, want none: the query declares no parameters", labels)
	}
}

func TestCompletionCalcTypingPositionOffersQueryAlias(t *testing.T) {
	s, name := openReportModel(t)
	labels := completionLabelsIn(t, s, name, reportModel, "Everything {", 0)
	if !containsLabel(labels, "TableQuery") {
		t.Fatalf("labels = %v, want the TableQuery alias spelling", labels)
	}
}

func reportHover(t *testing.T, s *Server, name, anchor string, delta int) *protocol.Hover {
	t.Helper()
	off := strings.Index(reportModel, anchor)
	if off < 0 {
		t.Fatalf("anchor %q not in fixture", anchor)
	}
	hov, err := s.Hover(context.Background(), &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition([]byte(reportModel), off+delta),
		},
	})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	return hov
}

func TestHoverQualifiedReferenceSegments(t *testing.T) {
	s, name := openReportModel(t)
	anchor := "QueryLib::NoInputs {"
	if hov := reportHover(t, s, name, anchor, 0); hov == nil ||
		!strings.Contains(hov.Contents.Value, "package QueryLib") {
		t.Fatalf("qualifier hover = %+v, want package QueryLib", hov)
	}
	if hov := reportHover(t, s, name, anchor, len("QueryLib::")); hov == nil ||
		!strings.Contains(hov.Contents.Value, "calc def NoInputs") {
		t.Fatalf("member hover = %+v, want calc def NoInputs", hov)
	}
	if hov := reportHover(t, s, name, anchor, len("QueryLib:")); hov != nil &&
		strings.Contains(hov.Contents.Value, "NoInputs") {
		t.Fatalf("separator hover = %+v, want not the reference target", hov)
	}
}

func TestRenderDocumentAmbiguousName(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	ws.Open("one.sysml", []byte(documentModel), 1)
	ws.Open("two.sysml", []byte(documentModel), 1)
	if _, err := s.RenderDocument(&renderDocumentParams{Name: "Observatory::MassReport"}); err == nil ||
		!strings.Contains(err.Error(), "names 2 elements") {
		t.Fatalf("err = %v, want an ambiguity error", err)
	}
}

func containsLabel(labels []string, want string) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}

func TestPublishDiagnosticsDocumentPlanLive(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	name := "report.sysml"
	// The document is missing its title, a document-plan diagnostic.
	broken := strings.Replace(documentModel,
		"attribute redefines title = \"Telescope Mass Report\";\n", "", 1)
	ws.Open(name, []byte(broken), 1)

	s.publishDiagnostics(context.Background(), name)
	published := fc.all()
	if len(published) != 1 {
		t.Fatalf("published = %d, want 1", len(published))
	}
	var found *protocol.Diagnostic
	for i, d := range published[0].Diagnostics {
		if d.Code == "document-plan-missing-title" {
			found = &published[0].Diagnostics[i]
		}
	}
	if found == nil {
		t.Fatalf("diagnostics = %v, want document-plan-missing-title", published[0].Diagnostics)
	}
	if found.Source != "document-plan" || found.Severity != protocol.DiagnosticSeverityError {
		t.Errorf("diagnostic = %+v, want source document-plan at error severity", found)
	}
	want := offsetToPosition([]byte(broken), strings.Index(broken, "part def MassReport"))
	if found.Range.Start.Line != want.Line {
		t.Errorf("range starts line %d, want %d (the document definition)", found.Range.Start.Line, want.Line)
	}

	// Restoring the title clears the diagnostic on the next publish.
	ws.Update(name, []byte(documentModel), 2)
	s.publishDiagnostics(context.Background(), name)
	published = fc.all()
	last := published[len(published)-1]
	for _, d := range last.Diagnostics {
		if d.Source == "document-plan" || d.Source == "document-query" {
			t.Fatalf("edit left stale diagnostic: %+v", d)
		}
	}
}

func TestDocumentsListsWorkspaceDocuments(t *testing.T) {
	_, s, name := openDocumentModel(t)
	res := s.Documents()
	if len(res.Documents) != 1 {
		t.Fatalf("documents = %+v, want the one MassReport", res.Documents)
	}
	if res.Documents[0].Name != "Observatory::MassReport" {
		t.Errorf("name = %q, want Observatory::MassReport", res.Documents[0].Name)
	}
	if res.Documents[0].URI != string(uri.File(name)) {
		t.Errorf("uri = %q, want %q", res.Documents[0].URI, uri.File(name))
	}
}

func TestRenderDocumentMarkdown(t *testing.T) {
	_, s, _ := openDocumentModel(t)
	res, err := s.RenderDocument(&renderDocumentParams{Name: "Observatory::MassReport"})
	if err != nil {
		t.Fatalf("RenderDocument err = %v", err)
	}
	for _, want := range []string{
		"# Telescope Mass Report",
		"<!-- caption -->\n*All subsystems by mass*",
		"| name | mass |",
		"| optics | 8.5 |",
	} {
		if !strings.Contains(res.Markdown, want) {
			t.Errorf("markdown missing %q:\n%s", want, res.Markdown)
		}
	}
}

// diagramDocumentModel adds a report whose diagram draws a view, so the
// render's diagramForm has a graph-shaped block to act on.
const diagramDocumentModel = `package Imaging {
	private import Views::*;
	private import DocumentQueries::*;

	port def DataPort;
	part def Camera { port output : DataPort; }
	part def Recorder { port input : DataPort; }

	part imagingChain {
		part camera : Camera;
		part recorder : Recorder;
		connection link connect camera.output to recorder.input;
	}

	view chainView {
		expose imagingChain;
		render asInterconnectionDiagram;
	}

	part def ChainReport :> Document {
		attribute redefines title = "Imaging Chain";

		part chain : Diagram {
			attribute redefines caption = "The imaging chain";
			ref redefines source = chainView;
		}
	}
}
`

// TestRenderDocumentDiagramForm writes the document's graph-shaped diagrams
// as Mermaid when diagramForm is absent and as DOT when it is "dot".
func TestRenderDocumentDiagramForm(t *testing.T) {
	ws, s, _ := openDocumentModel(t)
	ws.Open(uri.File("/tmp/imaging.sysml").Filename(), []byte(diagramDocumentModel), 1)
	cases := map[string]struct{ fence, header string }{
		"":        {"```mermaid\n", "%% Imaging::chainView — interconnection rendering"},
		"mermaid": {"```mermaid\n", "%% Imaging::chainView — interconnection rendering"},
		"dot":     {"```dot\n", "// view: Imaging::chainView\n// kind: interconnection\n"},
	}
	for form, want := range cases {
		res, err := s.RenderDocument(&renderDocumentParams{Name: "Imaging::ChainReport", DiagramForm: form})
		if err != nil {
			t.Fatalf("diagramForm %q: %v", form, err)
		}
		if !strings.Contains(res.Markdown, want.fence+want.header) {
			t.Errorf("diagramForm %q: markdown missing %q:\n%s", form, want.fence+want.header, res.Markdown)
		}
		if strings.Count(res.Markdown, "```") != 2 {
			t.Errorf("diagramForm %q: want exactly one fenced block:\n%s", form, res.Markdown)
		}
	}
	if _, err := s.RenderDocument(&renderDocumentParams{Name: "Imaging::ChainReport", DiagramForm: "svg"}); err == nil ||
		!strings.Contains(err.Error(), `no diagram form is named "svg"`) {
		t.Fatalf("err = %v, want an unknown-form error", err)
	}
	res, err := s.RenderDocument(&renderDocumentParams{Name: "Observatory::MassReport", DiagramForm: "dot"})
	if err != nil {
		t.Fatalf("table-only document as dot: %v", err)
	}
	if !strings.Contains(res.Markdown, "| name | mass |") || strings.Contains(res.Markdown, "```") {
		t.Errorf("a table is not a table under dot:\n%s", res.Markdown)
	}
}

func TestRenderDocumentTypedErrors(t *testing.T) {
	_, s, _ := openDocumentModel(t)
	if _, err := s.RenderDocument(&renderDocumentParams{Name: "Observatory::Subsystem"}); err == nil ||
		!strings.Contains(err.Error(), "is not a document") {
		t.Fatalf("err = %v, want a not-a-document error", err)
	}
	if _, err := s.RenderDocument(&renderDocumentParams{Name: "Observatory::Nothing"}); err == nil ||
		!strings.Contains(err.Error(), "no element named") {
		t.Fatalf("err = %v, want a no-element error", err)
	}
}
