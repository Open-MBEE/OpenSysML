package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// renderModel declares one view per rendering kind this package produces, over a
// model that has parts to connect, a state machine and an action flow, so a
// rendering of each kind has something in it.
const renderModel = `package Kit {
	part def Widget {
		part cog : Cog;
		part gear : Cog;
		connect cog to gear;
	}
	part def Cog;

	state def WidgetStates {
		entry; then off;
		state off;
		state running;
		transition first off then running;
	}

	action def Assemble {
		action cut;
		action fit;
		first cut then fit;
	}
}

package KitViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view widgetTree {
		expose Kit::Widget;
	}

	view widgetParts : InterconnectionView {
		expose Kit::Widget;
	}

	view widgetStates : StateTransitionView {
		expose Kit::WidgetStates;
	}

	view widgetActions : ActionFlowView {
		expose Kit::Assemble;
	}

	view widgetTable : GridView {
		expose Kit::Widget;
	}

	view widgetSequence : SequenceView {
		expose Kit::Widget;
	}

	view widgetGeometry : GeometryView {
		expose Kit::Widget;
	}
}
`

// recorder is a client that records diagnostics and custom notifications in the
// order they were sent, so their ordering is testable.
type recorder struct {
	baseClient
	mu   sync.Mutex
	sent []string
}

func (r *recorder) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	r.record("publishDiagnostics")
	return nil
}

func (r *recorder) Notify(ctx context.Context, method string, params interface{}) error {
	r.record(method)
	return nil
}

func (r *recorder) record(what string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, what)
}

func (r *recorder) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.sent...)
}

// renderServer is a server holding one open document, as a session of an
// editor speaking the cross-document diagram contract does.
func renderServer(t *testing.T, name, src string) (*Server, uri.URI) {
	t.Helper()
	s := initializedServer(t, map[string]any{CrossDocumentCapability: true})
	docURI := uri.File(name)
	if err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: docURI, LanguageID: "sysml", Version: 1, Text: src,
		},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}
	return s, docURI
}

// initializedServer is a server a client initialized with those experimental
// capabilities, none for a client predating them.
func initializedServer(t *testing.T, experimental map[string]any) *Server {
	t.Helper()
	s := NewServer(model.NewWorkspace())
	s.client = &recorder{}
	var caps protocol.ClientCapabilities
	if experimental != nil {
		caps.Experimental = experimental
	}
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{Capabilities: caps}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	return s
}

// call dispatches a custom request the way a served session does, through the
// handler chain, and returns the raw result the client would receive.
func call(t *testing.T, s *Server, method string, params any) (json.RawMessage, error) {
	t.Helper()
	req, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), method, params)
	if err != nil {
		t.Fatalf("build %s request: %v", method, err)
	}
	var (
		raw     json.RawMessage
		callErr error
	)
	reply := func(ctx context.Context, result interface{}, err error) error {
		if err != nil {
			callErr = err
			return nil
		}
		encoded, mErr := json.Marshal(result)
		if mErr != nil {
			t.Fatalf("marshal %s result: %v", method, mErr)
		}
		raw = encoded
		return nil
	}
	handler := s.renderHandler(s.modelEditHandler(func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		t.Fatalf("%s was not handled: it fell through to the next handler", req.Method())
		return nil
	}))
	if err := handler(context.Background(), reply, req); err != nil {
		t.Fatalf("dispatch %s: %v", method, err)
	}
	return raw, callErr
}

// render is one opensysml/render request, decoded.
func render(t *testing.T, s *Server, docURI uri.URI, viewName string) *renderResult {
	return renderWithLinkTemplate(t, s, docURI, viewName, "")
}

func renderWithLinkTemplate(t *testing.T, s *Server, docURI uri.URI, viewName, template string) *renderResult {
	t.Helper()
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         viewName,
		LinkTemplate: template,
	})
	if err != nil {
		t.Fatalf("render %q: %v", viewName, err)
	}
	var out renderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	return &out
}

func TestRenderWritesSourceLinks(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	template := "https://example.test/src/{file}#L{line}:{col}"
	params, err := json.Marshal(renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		LinkTemplate: template,
	})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(params, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["linkTemplate"] != template {
		t.Errorf("wire linkTemplate = %v, want %q", wire["linkTemplate"], template)
	}
	out := renderWithLinkTemplate(t, s, docURI, "KitViews::widgetTree", template)
	if !strings.Contains(out.Artifact, `click n0 href "https://example.test/src/`) ||
		!strings.Contains(out.Artifact, "#L") {
		t.Errorf("render artifact lacks source links:\n%s", out.Artifact)
	}
}

// Every rendering kind this package produces is served over the protocol, with
// the artifact a client draws and the nodes it is made of.
func TestRenderServesEverySupportedKind(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	cases := []struct {
		view string
		kind view.Kind
		form view.Form
	}{
		{"KitViews::widgetTree", view.KindTree, view.FormMermaid},
		{"KitViews::widgetParts", view.KindInterconnection, view.FormMermaid},
		{"KitViews::widgetStates", view.KindState, view.FormMermaid},
		{"KitViews::widgetActions", view.KindAction, view.FormMermaid},
		{"KitViews::widgetTable", view.KindTable, view.FormMarkdown},
		{"KitViews::widgetSequence", view.KindSequence, view.FormMermaid},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			out := render(t, s, docURI, tc.view)
			if out.View != tc.view {
				t.Errorf("view = %q, want %q", out.View, tc.view)
			}
			if out.Kind != string(tc.kind) {
				t.Errorf("kind = %q, want %q", out.Kind, tc.kind)
			}
			if out.Form != string(tc.form) {
				t.Errorf("form = %q, want %q", out.Form, tc.form)
			}
			if strings.TrimSpace(out.Artifact) == "" {
				t.Error("artifact is empty")
			}
			if out.Version != 1 {
				t.Errorf("version = %d, want the version the document was opened at", out.Version)
			}
			if tc.kind == view.KindTable {
				if len(out.Rows) == 0 {
					t.Error("a table rendering carries no rows")
				}
				return
			}
			if len(out.Nodes) == 0 {
				t.Fatalf("%s rendering carries no nodes", tc.kind)
			}
			located := 0
			for _, node := range out.Nodes {
				if node.Origin == nil {
					continue
				}
				located++
				if node.Origin.URI != docURI {
					t.Errorf("node %q is located in %q, want %q", node.ID, node.Origin.URI, docURI)
				}
			}
			if located == 0 {
				t.Errorf("no node of the %s rendering is located in the source", tc.kind)
			}
		})
	}
}

// A node's origin is the range of the declaration it was built from, so clicking
// it lands on that declaration.
func TestRenderOriginsLocateTheDeclaration(t *testing.T) {
	src := "package Kit {\n\tpart def Widget {\n\t\tpart cog : Cog;\n\t}\n\tpart def Cog;\n}\n"
	s, docURI := renderServer(t, "kit.sysml", src)
	out := render(t, s, docURI, "#tree")
	for _, node := range out.Nodes {
		if node.Name != "cog" {
			continue
		}
		if node.Origin == nil {
			t.Fatal("the node for cog carries no origin")
		}
		// `part cog : Cog;` is the third line, indented by one tab.
		if node.Origin.Range.Start.Line != 2 || node.Origin.Range.Start.Character != 2 {
			t.Fatalf("origin starts at %+v, want line 2 character 2", node.Origin.Range.Start)
		}
		// The selection range is `cog` alone, so clicking selects the name
		// rather than the whole declaration.
		sel := node.Origin.SelectionRange
		if sel == nil {
			t.Fatal("the node for cog carries no selection range")
		}
		if sel.Start.Line != 2 || sel.Start.Character != 7 || sel.End.Character != 10 {
			t.Fatalf("selection range = %+v, want `cog` on line 2", *sel)
		}
		return
	}
	t.Fatalf("the #tree rendering has no node for cog: %+v", out.Nodes)
}

// A node carries its declared type as a field of its own, so a client never
// parses the detail — which holds only the notes — to recover it.
func TestRenderNodesCarryTheTypeApartFromTheDetail(t *testing.T) {
	src := "package Kit {\n\tpart def Widget {\n\t\tpart cog : Cog;\n\t\tpart gear;\n\t}\n\tpart def Cog;\n}\n"
	s, docURI := renderServer(t, "kit.sysml", src)
	out := render(t, s, docURI, "#tree")
	want := map[string]renderNode{
		"Kit::Widget": {Kind: "part def"},
		"cog":         {Kind: "part", Type: "Cog"},
		"gear":        {Kind: "part"},
	}
	for _, node := range out.Nodes {
		expected, ok := want[node.Name]
		if !ok {
			continue
		}
		delete(want, node.Name)
		if node.Kind != expected.Kind || node.Type != expected.Type || node.Detail != "" {
			t.Errorf("node %s = kind %q type %q detail %q, want kind %q type %q and no detail",
				node.Name, node.Kind, node.Type, node.Detail, expected.Kind, expected.Type)
		}
	}
	for name := range want {
		t.Errorf("the #tree rendering has no node named %s: %+v", name, out.Nodes)
	}
	wire, err := json.Marshal(out.Nodes)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"type":"Cog"`, `"type":""`, `"detail":""`} {
		if !strings.Contains(string(wire), field) {
			t.Errorf("the wire form lacks %s:\n%s", field, wire)
		}
	}
}

// A behavior rendering serves the type of a typed action or state usage the same
// way, apart from the notes the detail holds.
func TestRenderBehaviorNodesCarryTheTypeApartFromTheDetail(t *testing.T) {
	src := "package Ops {\n" +
		"\taction def Warm;\n" +
		"\taction run {\n\t\tfirst start;\n\t\taction warm : Warm;\n\t\tsuccession first start then warm;\n\t}\n" +
		"\tstate def Heating;\n" +
		"\tstate def Boiler {\n\t\tentry; then idle;\n\t\tstate idle;\n\t\tstate heating : Heating;\n" +
		"\t\ttransition first idle then heating;\n\t}\n" +
		"}\n"
	s, docURI := renderServer(t, "ops.sysml", src)
	want := map[string]renderNode{
		"warm":    {Kind: "action", Type: "Warm"},
		"start":   {Kind: "initial"},
		"heating": {Kind: "state", Type: "Heating"},
		"idle":    {Kind: "state", Detail: "initial"},
	}
	for view, field := range map[string]string{"#action:Ops::run": `"type":"Warm"`, "#state:Ops::Boiler": `"type":"Heating"`} {
		out := render(t, s, docURI, view)
		for _, node := range out.Nodes {
			expected, ok := want[node.Name]
			if !ok {
				continue
			}
			delete(want, node.Name)
			if node.Kind != expected.Kind || node.Type != expected.Type || node.Detail != expected.Detail {
				t.Errorf("%s: node %s = kind %q type %q detail %q, want kind %q type %q detail %q", view,
					node.Name, node.Kind, node.Type, node.Detail, expected.Kind, expected.Type, expected.Detail)
			}
		}
		wire, err := json.Marshal(out.Nodes)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(wire), field) || !strings.Contains(string(wire), `"type":""`) {
			t.Errorf("%s: the wire form lacks %s beside an empty type:\n%s", view, field, wire)
		}
	}
	for name := range want {
		t.Errorf("no behavior rendering has a node named %s", name)
	}
}

// A pseudo-view renders a document that declares no view, and says so.
func TestRenderPseudoViewOfADocumentWithNoViews(t *testing.T) {
	s, docURI := renderServer(t, "plain.sysml", "package Kit {\n\tpart def Widget {\n\t\tpart cog : Cog;\n\t}\n\tpart def Cog;\n}\n")
	out := render(t, s, docURI, "#tree")
	if out.Kind != string(view.KindTree) {
		t.Errorf("kind = %q, want %q", out.Kind, view.KindTree)
	}
	if out.View != "" {
		t.Errorf("view = %q, want empty: no view was declared", out.View)
	}
	if !strings.Contains(out.Stated, "no view declared") {
		t.Errorf("stated = %q, want it to say no view was declared", out.Stated)
	}

	// The same document without a view named is refused, pointing at pseudo-views.
	if _, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	}); err == nil || !strings.Contains(err.Error(), "#tree") {
		t.Errorf("err = %v, want it to point at a pseudo-view", err)
	}
}

// A view name no longer in the document is refused with a message naming it,
// which is what a panel holding a stale pick receives.
func TestRenderRefusesAStaleViewName(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	if _, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::goneView",
	}); err == nil || !strings.Contains(err.Error(), "no view named KitViews::goneView") {
		t.Errorf("err = %v, want it to say there is no such view", err)
	}
	if _, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File("gone.sysml")},
	}); err == nil || !strings.Contains(err.Error(), "no such document") {
		t.Errorf("err = %v, want it to say there is no such document", err)
	}
}

// An unsupported rendering kind is refused with the reason, and the view stays
// in the listing so a panel can say why it cannot be drawn.
func TestRenderAndViewsReportAnUnsupportedKind(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	_, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetGeometry",
	})
	if err == nil || !strings.Contains(err.Error(), "geometry rendering") {
		t.Fatalf("err = %v, want it to say a geometry rendering is not supported", err)
	}

	raw, err := call(t, s, MethodViews, &viewsParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})
	if err != nil {
		t.Fatalf("views: %v", err)
	}
	var listing viewsResult
	if err := json.Unmarshal(raw, &listing); err != nil {
		t.Fatalf("decode views result: %v", err)
	}
	if !slices.Equal(listing.PseudoViews, view.PseudoViewSpecs()) {
		t.Errorf("pseudoViews = %v, want %v", listing.PseudoViews, view.PseudoViewSpecs())
	}
	if !slices.Contains(listing.PseudoViews, "#sequence") {
		t.Errorf("pseudoViews = %v, want it to contain #sequence", listing.PseudoViews)
	}
	if len(listing.Views) != 7 {
		t.Fatalf("listed %d views, want 7: %+v", len(listing.Views), listing.Views)
	}
	kinds := map[string]viewInfo{}
	for _, info := range listing.Views {
		kinds[info.Name] = info
	}
	for name, kind := range map[string]view.Kind{
		"KitViews::widgetTree":     view.KindTree,
		"KitViews::widgetParts":    view.KindInterconnection,
		"KitViews::widgetStates":   view.KindState,
		"KitViews::widgetActions":  view.KindAction,
		"KitViews::widgetTable":    view.KindTable,
		"KitViews::widgetSequence": view.KindSequence,
	} {
		info, ok := kinds[name]
		if !ok {
			t.Fatalf("%s is not listed", name)
		}
		if info.Kind != string(kind) || !info.Supported {
			t.Errorf("%s: kind = %q supported = %v, want %q supported", name, info.Kind, info.Supported, kind)
		}
	}
	geometry := kinds["KitViews::widgetGeometry"]
	if geometry.Supported {
		t.Error("the geometry view is listed as supported")
	}
	if !strings.Contains(geometry.Reason, "geometry rendering") {
		t.Errorf("reason = %q, want it to say a geometry rendering is not supported", geometry.Reason)
	}
}

// Each listed view carries the range of its declaration and of its name in the
// document, in LSP positions, so a client can tell which view the cursor is in.
func TestViewsLocateEachDeclaration(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	raw, err := call(t, s, MethodViews, &viewsParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
	})
	if err != nil {
		t.Fatalf("views: %v", err)
	}
	var listing viewsResult
	if err := json.Unmarshal(raw, &listing); err != nil {
		t.Fatalf("decode views result: %v", err)
	}
	lines := strings.Split(renderModel, "\n")
	text := func(r protocol.Range) string {
		if r.Start.Line != r.End.Line {
			return strings.Join(append([]string{lines[r.Start.Line][r.Start.Character:]}, lines[r.Start.Line+1:r.End.Line]...), "\n") +
				"\n" + lines[r.End.Line][:r.End.Character]
		}
		return lines[r.Start.Line][r.Start.Character:r.End.Character]
	}
	for _, info := range listing.Views {
		if info.Range == nil || info.SelectionRange == nil {
			t.Fatalf("%s: range = %v selectionRange = %v, want both", info.Name, info.Range, info.SelectionRange)
		}
		short := strings.TrimPrefix(info.Name, "KitViews::")
		if got := text(*info.SelectionRange); got != short {
			t.Errorf("%s: selectionRange covers %q, want %q", info.Name, got, short)
		}
		decl := text(*info.Range)
		if !strings.HasPrefix(decl, "view "+short) || !strings.HasSuffix(decl, "}") {
			t.Errorf("%s: range covers %q, want the whole view declaration", info.Name, decl)
		}
	}
	// Two declarations never overlap, so a cursor is in at most one of them.
	before := func(a, b protocol.Position) bool {
		return a.Line < b.Line || (a.Line == b.Line && a.Character <= b.Character)
	}
	for i, a := range listing.Views {
		for _, b := range listing.Views[i+1:] {
			if !before(a.Range.End, b.Range.Start) && !before(b.Range.End, a.Range.Start) {
				t.Errorf("%s and %s overlap: %v and %v", a.Name, b.Name, *a.Range, *b.Range)
			}
		}
	}
}

// A form the writer does not know is refused rather than silently replaced, and
// a known one is honored.
func TestRenderHonorsTheFormAsked(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		Form:         string(view.FormText),
	})
	if err != nil {
		t.Fatalf("render as text: %v", err)
	}
	var out renderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	if out.Form != string(view.FormText) {
		t.Errorf("form = %q, want %q", out.Form, view.FormText)
	}
	if _, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		Form:         "png",
	}); err == nil || !strings.Contains(err.Error(), "no rendering form") || !strings.Contains(err.Error(), `"dot"`) || !strings.Contains(err.Error(), `"plantuml"`) {
		t.Errorf("err = %v, want it to refuse the form and offer dot and plantuml", err)
	}
}

// Every form the server advertises in initialize is answered in that form when
// asked for by a view that has it, and the advertised list is the writer's own.
func TestRenderAnswersEveryAdvertisedForm(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental, ok := res.Capabilities.Experimental.(map[string]any)
	if !ok {
		t.Fatalf("Experimental = %#v, want a map", res.Capabilities.Experimental)
	}
	advertised, ok := experimental[RenderFormsCapability].([]string)
	if !ok {
		t.Fatalf("%s = %#v, want a list of forms", RenderFormsCapability, experimental[RenderFormsCapability])
	}
	if want := []string{"text", "mermaid", "markdown", "dot", "plantuml", "csv", "tsv"}; !slices.Equal(advertised, want) {
		t.Fatalf("%s = %v, want %v", RenderFormsCapability, advertised, want)
	}
	// A table is the one kind written in Markdown, CSV and TSV; the tree view has every other form.
	viewFor := map[string]string{"markdown": "KitViews::widgetTable", "csv": "KitViews::widgetTable", "tsv": "KitViews::widgetTable"}
	for _, form := range advertised {
		name := viewFor[form]
		if name == "" {
			name = "KitViews::widgetTree"
		}
		raw, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         name,
			Form:         form,
		})
		if err != nil {
			t.Errorf("render %s as %s: %v", name, form, err)
			continue
		}
		var out renderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode %s render result: %v", form, err)
		}
		if out.Form != form {
			t.Errorf("%s: form = %q, want %q", name, out.Form, form)
		}
		if out.Artifact == "" {
			t.Errorf("%s as %s: empty artifact", name, form)
		}
	}
}

// The DOT form is honored for a graph-shaped view, is the same rendering as a
// digraph, and is refused for a kind that has none with the forms it has.
func TestRenderWritesDotWhenAskedFor(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	for _, name := range []string{"KitViews::widgetTree", "KitViews::widgetParts", "KitViews::widgetStates", "KitViews::widgetActions"} {
		raw, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         name,
			Form:         string(view.FormDot),
		})
		if err != nil {
			t.Fatalf("%s as dot: %v", name, err)
		}
		var out renderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode render result: %v", err)
		}
		if out.Form != string(view.FormDot) {
			t.Errorf("%s: form = %q, want %q", name, out.Form, view.FormDot)
		}
		for _, want := range []string{"// view: " + name, "// layout: dot", "digraph \"" + name + "\" {"} {
			if !strings.Contains(out.Artifact, want) {
				t.Errorf("%s: artifact is missing %q:\n%s", name, want, out.Artifact)
			}
		}
		if len(out.Nodes) == 0 {
			t.Errorf("%s: the DOT result carries no nodes", name)
		}
	}
	for _, name := range []string{"KitViews::widgetTable", "KitViews::widgetSequence"} {
		_, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         name,
			Form:         string(view.FormDot),
		})
		if err == nil || !strings.Contains(err.Error(), "not written as dot") {
			t.Errorf("%s as dot: err = %v, want the form refused", name, err)
		}
	}
}

// The PlantUML form is honored for every graph-shaped view, the sequence
// included, and is refused for a table with the forms it has.
func TestRenderWritesPlantUMLWhenAskedFor(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	for _, name := range []string{"KitViews::widgetTree", "KitViews::widgetParts", "KitViews::widgetStates", "KitViews::widgetActions", "KitViews::widgetSequence"} {
		raw, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         name,
			Form:         string(view.FormPlantUML),
		})
		if err != nil {
			t.Fatalf("%s as plantuml: %v", name, err)
		}
		var out renderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode render result: %v", err)
		}
		if out.Form != string(view.FormPlantUML) {
			t.Errorf("%s: form = %q, want %q", name, out.Form, view.FormPlantUML)
		}
		for _, want := range []string{"@startuml\n' " + name + " — ", "<style>\n", "</style>\n", "\n@enduml\n"} {
			if !strings.Contains(out.Artifact, want) {
				t.Errorf("%s: artifact is missing %q:\n%s", name, want, out.Artifact)
			}
		}
		if len(out.Nodes) == 0 {
			t.Errorf("%s: the PlantUML result carries no nodes", name)
		}
	}
	_, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTable",
		Form:         string(view.FormPlantUML),
	})
	if err == nil || !strings.Contains(err.Error(), "not written as plantuml") {
		t.Errorf("table as plantuml: err = %v, want the form refused", err)
	}
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		Form:         string(view.FormPlantUML),
		Palette:      string(view.PaletteOkabeIto),
	})
	if err != nil {
		t.Fatalf("render plantuml with a palette: %v", err)
	}
	var out renderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	if !strings.Contains(out.Artifact, ">> #") {
		t.Errorf("the PlantUML artifact is not filled from the palette:\n%s", out.Artifact)
	}
	if _, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		Form:         string(view.FormPlantUML),
		Palette:      "rainbow",
	}); err == nil || !strings.Contains(err.Error(), `unknown palette "rainbow"`) {
		t.Errorf("err = %v, want it to refuse the palette by name", err)
	}
}

// A palette in the request fills the DOT artifact's nodes, is noted as not
// represented in a Mermaid artifact, and an unknown one is refused by name
// with the palettes there are.
func TestRenderFillsFromThePaletteAsked(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		Form:         string(view.FormDot),
		Palette:      string(view.PaletteOkabeIto),
	})
	if err != nil {
		t.Fatalf("render with a palette: %v", err)
	}
	var out renderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	if !strings.Contains(out.Artifact, `fillcolor="#`) || !strings.Contains(out.Artifact, "penwidth=1, label=<") {
		t.Errorf("the DOT artifact is not filled from the palette:\n%s", out.Artifact)
	}
	raw, err = call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		Form:         string(view.FormMermaid),
		Palette:      string(view.PaletteViridis),
	})
	if err != nil {
		t.Fatalf("render Mermaid with a palette: %v", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	if !strings.Contains(out.Artifact, "classDef palette0 fill:#") || strings.Contains(out.Artifact, "not represented: palette") {
		t.Errorf("Mermaid does not draw the palette:\n%s", out.Artifact)
	}
	_, err = call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree",
		Form:         string(view.FormDot),
		Palette:      "rainbow",
	})
	want := `unknown palette "rainbow"; the palettes are okabe-ito, tol-bright, tol-muted, tol-light, brewer-set2, brewer-dark2, viridis, cividis`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to refuse the palette by name", err)
	}
}

// A palette colours the result's nodes as the DOT artifact of the same request
// does, hex for hex, whatever form is asked for; a control node stays uncoloured,
// and no node is coloured when no palette is asked for.
func TestRenderNodesCarryThePaletteFills(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	fillLine := regexp.MustCompile(`^\s*"([^"]+)" \[.*fillcolor="(#[0-9A-F]{6})", color="(#[0-9A-F]{6})"`)
	for _, form := range []view.Form{view.FormDot, view.FormMermaid} {
		raw, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "KitViews::widgetStates",
			Form:         string(form),
			Palette:      string(view.PaletteTolBright),
		})
		if err != nil {
			t.Fatalf("render %s with a palette: %v", form, err)
		}
		var out renderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode render result: %v", err)
		}
		got := map[string][2]string{}
		for _, n := range out.Nodes {
			if n.Fill != "" || n.Border != "" {
				got[n.ID] = [2]string{n.Fill, n.Border}
			}
			if n.Kind == "initial" && (n.Fill != "" || n.Border != "") {
				t.Errorf("%s: the initial pseudostate %s is coloured %s/%s", form, n.ID, n.Fill, n.Border)
			}
		}
		if len(got) < 2 {
			t.Errorf("%s: only %d nodes coloured: %+v", form, len(got), out.Nodes)
		}
		if form == view.FormDot {
			want := map[string][2]string{}
			for _, line := range strings.Split(out.Artifact, "\n") {
				if m := fillLine.FindStringSubmatch(line); m != nil {
					want[m[1]] = [2]string{m[2], m[3]}
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("nodes coloured %v, the DOT artifact %v", got, want)
			}
		}
	}
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetStates",
	})
	if err != nil {
		t.Fatalf("render without a palette: %v", err)
	}
	if strings.Contains(string(raw), `"fill"`) || strings.Contains(string(raw), `"border"`) {
		t.Errorf("a rendering without a palette carries colours:\n%s", raw)
	}
}

// A sequence participant carries the fill alone, as PlantUML colours no
// participant border; the border key is absent from the wire, not empty.
func TestRenderSequenceParticipantsFillWithoutBorder(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetSequence",
		Form:         string(view.FormPlantUML),
		Palette:      string(view.PaletteOkabeIto),
	})
	if err != nil {
		t.Fatalf("render the sequence with a palette: %v", err)
	}
	var out renderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	filled := 0
	for _, n := range out.Nodes {
		if n.Fill != "" {
			filled++
		}
		if n.Border != "" {
			t.Errorf("participant %s has border %s, want none", n.ID, n.Border)
		}
	}
	if filled == 0 {
		t.Errorf("no participant filled: %+v", out.Nodes)
	}
	if strings.Contains(string(raw), `"border"`) {
		t.Errorf("a sequence rendering carries a border key:\n%s", raw)
	}
	if strings.Contains(out.Artifact, ";line:") {
		t.Errorf("the PlantUML artifact colours a participant border:\n%s", out.Artifact)
	}
}

func TestRenderMermaidSequenceReturnsParticipantFills(t *testing.T) {
	s, docURI := renderServer(t, "kit.sysml", renderModel)
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetSequence",
		Form:         string(view.FormMermaid),
		Palette:      string(view.PaletteOkabeIto),
	})
	if err != nil {
		t.Fatalf("render the Mermaid sequence with a palette: %v", err)
	}
	var out renderResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	filled := 0
	for _, n := range out.Nodes {
		if n.Fill != "" {
			filled++
		}
		if n.Border != "" {
			t.Errorf("participant %s has border %s, want none", n.ID, n.Border)
		}
	}
	if filled == 0 {
		t.Errorf("no participant filled: %+v", out.Nodes)
	}
	if strings.Contains(string(raw), `"border"`) {
		t.Errorf("a Mermaid sequence rendering carries a border key:\n%s", raw)
	}
	if !strings.Contains(out.Artifact, "palette okabe-ito; Mermaid's sequence diagram cannot fill individual participants") {
		t.Errorf("Mermaid sequence palette notice changed:\n%s", out.Artifact)
	}
}

// A view's layout annotations reach the client as geometry on nodes and edges
// and a canvas on the result; a rendering without any carries none of the fields.
func TestRenderCarriesLayoutGeometry(t *testing.T) {
	const src = `package Kit {
	private import DiagramLayout::*;
	part def Widget {
		part cog : Cog;
		part gear : Cog { @Layout { x = 5; y = 6; } }
		connection mesh : Mesh connect cog to gear;
	}
	part def Cog;
	connection def Mesh;
}

package KitViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view placed : InterconnectionView {
		expose Kit::Widget;
		@Canvas { unit = "px"; width = 640; height = 0; }
		metadata Layout about Kit::Widget::cog { x = 10; y = 20; width = 90; height = 40; collapsed = true; }
		metadata Route about Kit::Widget::mesh { points = (1, 2, 3, 4); }
	}
}
`
	s, docURI := renderServer(t, "kit.sysml", src)
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::placed",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var out struct {
		Nodes  []map[string]json.RawMessage `json:"nodes"`
		Edges  []map[string]json.RawMessage `json:"edges"`
		Canvas map[string]json.RawMessage   `json:"canvas"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode render result: %v", err)
	}
	if got := fmt.Sprintf("%s|%s|%s", out.Canvas["unit"], out.Canvas["width"], out.Canvas["height"]); got != `"px"|640|0` {
		t.Errorf("canvas = %s, want unit px, width 640 and the explicit height 0", got)
	}
	nodeBy := func(name string) map[string]json.RawMessage {
		for _, n := range out.Nodes {
			if string(n["name"]) == fmt.Sprintf("%q", name) {
				return n
			}
		}
		t.Fatalf("no node named %s in %v", name, out.Nodes)
		return nil
	}
	cog := nodeBy("cog")
	if got := fmt.Sprintf("%s %s %s %s %s", cog["x"], cog["y"], cog["width"], cog["height"], cog["collapsed"]); got != "10 20 90 40 true" {
		t.Errorf("cog geometry = %q, want the view-local Layout", got)
	}
	gear := nodeBy("gear")
	if got := fmt.Sprintf("%s %s %s %s %s", gear["x"], gear["y"], gear["width"], gear["height"], gear["collapsed"]); got != "5 6   " {
		t.Errorf("gear geometry = %q, want the inline Layout with no size and no collapsed field", got)
	}
	for _, n := range out.Nodes {
		if name := string(n["name"]); name == `"cog"` || name == `"gear"` {
			continue
		}
		for _, field := range []string{"x", "y", "width", "height", "collapsed"} {
			if _, ok := n[field]; ok {
				t.Errorf("unplaced node %s carries %q", n["name"], field)
			}
		}
	}
	if len(out.Edges) != 1 {
		t.Fatalf("edges = %v, want the one connection", out.Edges)
	}
	if got := string(out.Edges[0]["route"]); got != `[{"x":1,"y":2},{"x":3,"y":4}]` {
		t.Errorf("route = %s, want the connector's Route waypoints", got)
	}

	s, docURI = renderServer(t, "plain.sysml", renderModel)
	plain := render(t, s, docURI, "KitViews::widgetParts")
	if plain.Canvas != nil {
		t.Errorf("a view without a Canvas carries %+v", plain.Canvas)
	}
	for _, n := range plain.Nodes {
		if n.X != nil || n.Y != nil || n.Width != nil || n.Height != nil || n.Collapsed {
			t.Errorf("unannotated node %+v carries geometry", n)
		}
	}
	for _, e := range plain.Edges {
		if e.Route != nil {
			t.Errorf("unannotated edge %+v carries a route", e)
		}
	}
}

// An edit is followed by the notification that the renderings went stale, after
// the diagnostics of the same analysis, at the version the edit produced.
func TestDidChangeNotifiesRenderChangedAfterDiagnostics(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	rec := &recorder{}
	s.client = rec
	s.notifier = rec
	ctx := context.Background()
	docURI := uri.File("kit.sysml")
	if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: docURI, LanguageID: "sysml", Version: 1, Text: "package Kit {\n\tpart def Widget;\n}\n",
		},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}
	// A burst of edits, as typing is.
	for version := 2; version <= 6; version++ {
		s.applyDidChange(ctx, uriToName(docURI), []rawContentChange{
			{Text: fmt.Sprintf("package Kit {\n\tpart def Widget;\n\tpart def Cog%d;\n}\n", version)},
		}, version)
	}

	// The notification arrives once the burst settles rather than per keystroke.
	deadline := time.Now().Add(5 * time.Second)
	var sent []string
	for time.Now().Before(deadline) {
		sent = rec.all()
		if len(sent) > 0 && sent[len(sent)-1] == MethodRenderChanged {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(sent) == 0 || sent[len(sent)-1] != MethodRenderChanged {
		t.Fatalf("sent %v, want it to end with %s", sent, MethodRenderChanged)
	}
	firstRender := -1
	for i, what := range sent {
		if what == MethodRenderChanged {
			firstRender = i
			break
		}
	}
	if firstRender <= 0 || sent[firstRender-1] != "publishDiagnostics" {
		t.Fatalf("sent %v, want the diagnostics of an analysis before its %s", sent, MethodRenderChanged)
	}
	if renders, publishes := countOf(sent, MethodRenderChanged), countOf(sent, "publishDiagnostics"); renders >= publishes {
		t.Errorf("sent %d %s notifications for %d publications, want fewer: the burst is debounced", renders, MethodRenderChanged, publishes)
	}

	// The notification reports the version the rendering would be made from.
	out := render(t, s, docURI, "#tree")
	if out.Version != 6 {
		t.Errorf("version = %d, want the version the edit produced", out.Version)
	}
}

// countOf is how many times what was sent.
func countOf(sent []string, what string) int {
	n := 0
	for _, s := range sent {
		if s == what {
			n++
		}
	}
	return n
}

// The server tells a client it speaks the render methods, so an old server and a
// new client degrade instead of erroring.
func TestInitializeAdvertisesTheRenderCapability(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental, ok := res.Capabilities.Experimental.(map[string]any)
	if !ok {
		t.Fatalf("Experimental = %#v, want a map", res.Capabilities.Experimental)
	}
	if experimental["openSysmlRender"] != true {
		t.Errorf("openSysmlRender = %#v, want true", experimental["openSysmlRender"])
	}
	if experimental[CrossDocumentCapability] != true {
		t.Errorf("%s = %#v, want true", CrossDocumentCapability, experimental[CrossDocumentCapability])
	}
	if experimental[RenderPaletteCapability] != true {
		t.Errorf("%s = %#v, want true", RenderPaletteCapability, experimental[RenderPaletteCapability])
	}
}

// A rendering's version, node names, FQNs and ranges all describe one document
// snapshot, however the document changes while renders are in flight: a node
// built from one revision is never named through the scope of another.
func TestRenderSnapshotsOneDocumentRevision(t *testing.T) {
	// Both names are five letters, so the declarations share a span across
	// revisions and a scope of the wrong revision would still find a symbol.
	revisions := []string{"package P {\n    part def Alpha;\n}\n", "package P {\n    part def Bravo;\n}\n"}
	s, docURI := renderServer(t, "flip.sysml", revisions[0])
	name := docURI.Filename()

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for version := 2; ; version++ {
			select {
			case <-stop:
				return
			default:
			}
			s.ws.Update(name, []byte(revisions[(version-1)%2]), version)
		}
	}()
	defer func() { close(stop); <-done }()

	params := &renderParams{TextDocument: protocol.TextDocumentIdentifier{URI: docURI}, View: "#tree"}
	for i := 0; i < 300; i++ {
		out, err := s.Render(params)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		declared := []string{"Alpha", "Bravo"}[(out.Version-1)%2]
		other := []string{"Bravo", "Alpha"}[(out.Version-1)%2]
		for _, n := range out.Nodes {
			if strings.Contains(n.Name, other) || strings.Contains(n.FQN, other) {
				t.Fatalf("version %d declares %s, rendering has node %q with fqn %q", out.Version, declared, n.Name, n.FQN)
			}
			if strings.Contains(n.Name, declared) && n.FQN != "P::"+declared {
				t.Fatalf("version %d node %q has fqn %q, want %q", out.Version, n.Name, n.FQN, "P::"+declared)
			}
		}
	}
}

// A drawing style in the request draws the DOT artifact in it and is named in
// the result, the default pilot when none is asked; the styles are advertised
// in initialize; Mermaid draws the style too; a style there is
// none of is refused by name. A node's own Style annotation reaches the client
// and wins over the palette's fill and border.
func TestRenderDrawsInTheStyleAsked(t *testing.T) {
	const styled = renderModel + `
package StyledViews {
	private import Views::*;
	private import StandardViewDefinitions::*;
	private import DiagramLayout::*;

	view styledTree {
		expose Kit::Widget;
		metadata Style about Kit::Widget::cog { fill = "#FFE8BD"; line = "#333333"; bold = true; }
	}
}
`
	s, docURI := renderServer(t, "kit.sysml", styled)
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental := res.Capabilities.Experimental.(map[string]any)
	if advertised, _ := experimental[RenderStylesCapability].([]string); !slices.Equal(advertised, []string{"pilot", "cameo"}) {
		t.Fatalf("%s = %#v, want pilot then cameo", RenderStylesCapability, experimental[RenderStylesCapability])
	}
	render := func(t *testing.T, view, form, style, palette string) renderResult {
		t.Helper()
		raw, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         view, Form: form, Style: style, Palette: palette,
		})
		if err != nil {
			t.Fatalf("render %s in %q: %v", form, style, err)
		}
		var out renderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode render result: %v", err)
		}
		return out
	}
	cameo := render(t, "KitViews::widgetTree", "dot", "cameo", "")
	if cameo.Style != "cameo" || !strings.Contains(cameo.Artifact, `subgraph "cluster_frame"`) || !strings.Contains(cameo.Artifact, `fontname="Arial"`) {
		t.Errorf("cameo result style %q, artifact:\n%s", cameo.Style, cameo.Artifact)
	}
	plain := render(t, "KitViews::widgetTree", "dot", "", "")
	pilot := render(t, "KitViews::widgetTree", "dot", "pilot", "")
	if plain.Style != "pilot" || pilot.Artifact != plain.Artifact || strings.Contains(plain.Artifact, "cluster_frame") {
		t.Errorf("default style %q; the pilot artifact differs from the default or frames the diagram:\n%s", plain.Style, plain.Artifact)
	}
	mermaid := render(t, "KitViews::widgetTree", "mermaid", "cameo", "")
	if !strings.Contains(mermaid.Artifact, "fontFamily: \"Arial, Helvetica, sans-serif\"") ||
		strings.Contains(mermaid.Artifact, "not represented: style cameo") {
		t.Errorf("Mermaid does not draw the style:\n%s", mermaid.Artifact)
	}
	_, err = call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "KitViews::widgetTree", Form: "dot", Style: "magicdraw",
	})
	if err == nil || !strings.Contains(err.Error(), `unknown drawing style "magicdraw"; the styles are pilot, cameo`) {
		t.Errorf("err = %v, want it to refuse the style by name", err)
	}

	out := render(t, "StyledViews::styledTree", "dot", "", string(view.PaletteOkabeIto))
	var cog *renderNode
	for i := range out.Nodes {
		if out.Nodes[i].Name == "cog" {
			cog = &out.Nodes[i]
		}
	}
	if cog == nil || cog.Style == nil || cog.Style.Fill != "#FFE8BD" || !cog.Style.Bold {
		t.Fatalf("cog carries no Style: %+v", cog)
	}
	if cog.Fill != "#FFE8BD" || cog.Border != "#333333" {
		t.Errorf("cog fill %q border %q: the Style does not win over the palette", cog.Fill, cog.Border)
	}
	if !strings.Contains(out.Artifact, `fillcolor="#FFE8BD"`) {
		t.Errorf("the DOT artifact does not draw the Style:\n%s", out.Artifact)
	}
}

// A request's `ports` chooses how much of a part's ports an interconnection
// draws — minimal, the default, the ports a connector ends at, named alone, or
// full, every port typed — the displays listed under the capability; a name
// that is neither is refused with the two there are.
func TestRenderTakesAPortDisplay(t *testing.T) {
	const ported = `package Demo {
    port def Signal;
    part def Sender { port out1 : Signal; }
    part def Receiver { port in1 : ~Signal; port spare : Signal; }
    part def Link {
        part sender : Sender;
        part receiver : Receiver;
        interface wire connect sender.out1 to receiver.in1;
    }
    view link { expose Demo::Link::*; render Views::asInterconnectionDiagram; }
}
`
	s, docURI := renderServer(t, "ported.sysml", ported)
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	experimental := res.Capabilities.Experimental.(map[string]any)
	if advertised, _ := experimental[RenderPortsCapability].([]string); !slices.Equal(advertised, []string{"minimal", "full"}) {
		t.Fatalf("%s = %#v, want minimal then full", RenderPortsCapability, experimental[RenderPortsCapability])
	}
	render := func(t *testing.T, ports string) *renderResult {
		t.Helper()
		raw, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "Demo::link", Form: string(view.FormPlantUML), Ports: ports,
		})
		if err != nil {
			t.Fatalf("render ports=%q: %v", ports, err)
		}
		var out renderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode render result: %v", err)
		}
		return &out
	}
	minimalResult := render(t, "")
	minimal := minimalResult.Artifact
	for _, want := range []string{`port "out1" as `, `port "in1" as `} {
		if !strings.Contains(minimal, want) {
			t.Errorf("the default lacks %q:\n%s", want, minimal)
		}
	}
	minimalPorts := map[string]renderPort{}
	for _, node := range minimalResult.Nodes {
		for _, port := range node.Ports {
			minimalPorts[port.Name] = port
		}
	}
	if len(minimalPorts) != 2 || minimalPorts["out1"].ID == "" || minimalPorts["in1"].ID == "" {
		t.Errorf("default node ports = %+v, want only connected out1 and in1 pins", minimalPorts)
	}
	if len(minimalResult.Edges) != 1 || minimalResult.Edges[0].FromPort != minimalPorts["out1"].ID ||
		minimalResult.Edges[0].ToPort != minimalPorts["in1"].ID {
		t.Errorf("default edges = %+v, want endpoints at out1 and in1", minimalResult.Edges)
	}
	if strings.Contains(minimal, "spare") || strings.Contains(minimal, "Signal") {
		t.Errorf("the default drew an unconnected port or a type:\n%s", minimal)
	}
	if got := render(t, "minimal").Artifact; got != minimal {
		t.Errorf("ports=minimal differs from the default")
	}
	fullResult := render(t, "full")
	full := fullResult.Artifact
	for _, want := range []string{`port "out1 : Signal" as `, `port "in1 : <U+007E>Signal" as `, `port "spare : Signal" as `} {
		if !strings.Contains(full, want) {
			t.Errorf("ports=full lacks %q:\n%s", want, full)
		}
	}
	fullPorts := map[string]renderPort{}
	for _, node := range fullResult.Nodes {
		for _, port := range node.Ports {
			fullPorts[port.Name] = port
		}
	}
	if len(fullPorts) != 3 || fullPorts["spare"].Type != "Signal" ||
		fullPorts["in1"].Type != "~Signal" || fullPorts["out1"].Type != "Signal" {
		t.Errorf("full node ports = %+v, want all three typed pins", fullPorts)
	}
	if len(fullResult.Edges) != 1 || fullResult.Edges[0].FromPort != fullPorts["out1"].ID ||
		fullResult.Edges[0].ToPort != fullPorts["in1"].ID {
		t.Errorf("full edges = %+v, want endpoints at out1 and in1", fullResult.Edges)
	}
	assertEngineRenderMatchesLSP(t, ported, minimalResult, "")
	assertEngineRenderMatchesLSP(t, ported, fullResult, "full")
	_, err = call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         "Demo::link", Form: string(view.FormDot), Ports: "all",
	})
	want := `unknown port display "all"; the displays are minimal, full`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want it to refuse the port display by name", err)
	}
}

func assertEngineRenderMatchesLSP(t *testing.T, src string, lspResult *renderResult, ports string) {
	t.Helper()
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	body, err := json.Marshal(map[string]any{"documents": []map[string]string{
		{"content": src, "name": "ported.sysml"},
	}})
	if err != nil {
		t.Fatalf("marshal ParseSources request: %v", err)
	}
	parsedBody, err := eng.Call(context.Background(), "ParseSources", body)
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	var parsed engine.JParseSourcesResponse
	if err := json.Unmarshal(parsedBody, &parsed); err != nil {
		t.Fatalf("decode ParseSources: %v", err)
	}
	renderBody, err := json.Marshal(map[string]string{
		"modelHash": parsed.ModelHash, "view": "Demo::link", "ports": ports,
	})
	if err != nil {
		t.Fatalf("marshal RenderView request: %v", err)
	}
	raw, err := eng.Call(context.Background(), "RenderView", renderBody)
	if err != nil {
		t.Fatalf("RenderView ports=%q: %v", ports, err)
	}
	var rendered engine.JRenderViewResponse
	if err := json.Unmarshal(raw, &rendered); err != nil {
		t.Fatalf("decode RenderView: %v", err)
	}
	nodes := make([]engine.JRenderNode, 0, len(lspResult.Nodes))
	for _, node := range lspResult.Nodes {
		converted := engine.JRenderNode{
			ID: node.ID, Kind: node.Kind, Name: node.Name, NameSynthesized: node.NameSynthesized,
			Type: node.Type, Detail: node.Detail, Parent: node.Parent, Fill: node.Fill, Border: node.Border,
			X: node.X, Y: node.Y, Width: node.Width, Height: node.Height, Collapsed: node.Collapsed,
		}
		if node.Style != nil {
			converted.Style = &engine.JRenderStyle{
				Fill: node.Style.Fill, Line: node.Style.Line, Text: node.Style.Text, Font: node.Style.Font,
				FontSize: node.Style.FontSize, Bold: node.Style.Bold, Italic: node.Style.Italic,
			}
		}
		for _, port := range node.Ports {
			converted.Ports = append(converted.Ports, engine.JRenderPort{
				ID: port.ID, Name: port.Name, Type: port.Type, Direction: port.Direction,
			})
		}
		nodes = append(nodes, converted)
	}
	if !reflect.DeepEqual(rendered.Nodes, nodes) {
		t.Errorf("engine nodes for ports=%q differ from LSP:\nengine: %+v\nLSP: %+v", ports, rendered.Nodes, nodes)
	}
	edges := make([]engine.JRenderEdge, 0, len(lspResult.Edges))
	for _, edge := range lspResult.Edges {
		converted := engine.JRenderEdge{
			From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort,
			Label: edge.Label, Kind: edge.Kind,
		}
		if edge.Style != nil {
			converted.Style = &engine.JRenderStyle{
				Fill: edge.Style.Fill, Line: edge.Style.Line, Text: edge.Style.Text, Font: edge.Style.Font,
				FontSize: edge.Style.FontSize, Bold: edge.Style.Bold, Italic: edge.Style.Italic,
			}
		}
		for _, point := range edge.Route {
			converted.Route = append(converted.Route, engine.JRenderPoint{X: point.X, Y: point.Y})
		}
		edges = append(edges, converted)
	}
	if !reflect.DeepEqual(rendered.Edges, edges) {
		t.Errorf("engine edges for ports=%q differ from LSP:\nengine: %+v\nLSP: %+v", ports, rendered.Edges, edges)
	}
}

func TestRenderKeepsActionPinsForEveryPortDisplay(t *testing.T) {
	const src = `package Pins {
	item def Bread;
	item def Toast;

	action def Heat {
		in b : Bread;
		out t : Toast;
	}

	action def ToastBread {
		in bread : Bread;
		out toast : Toast;
		action heat : Heat { in b = bread; }
		first start; then heat; then done;
	}
}
`
	s, docURI := renderServer(t, "action-pins.sysml", src)
	renderWithPorts := func(ports string) *renderResult {
		t.Helper()
		raw, err := call(t, s, MethodRender, &renderParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			View:         "#action:Pins::ToastBread",
			Ports:        ports,
		})
		if err != nil {
			t.Fatalf("render action ports=%q: %v", ports, err)
		}
		var out renderResult
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode action rendering: %v", err)
		}
		return &out
	}
	minimal, full := renderWithPorts(""), renderWithPorts("full")
	var minimalRoot, fullRoot *renderNode
	for i := range minimal.Nodes {
		if minimal.Nodes[i].Name == "Pins::ToastBread" {
			minimalRoot = &minimal.Nodes[i]
		}
	}
	for i := range full.Nodes {
		if full.Nodes[i].Name == "Pins::ToastBread" {
			fullRoot = &full.Nodes[i]
		}
	}
	if minimalRoot == nil || fullRoot == nil {
		t.Fatalf("action frame nodes: minimal=%+v full=%+v", minimal.Nodes, full.Nodes)
	}
	if !reflect.DeepEqual(minimalRoot.Ports, fullRoot.Ports) {
		t.Errorf("action frame ports differ by display: default=%+v full=%+v", minimalRoot.Ports, fullRoot.Ports)
	}
	directions := map[string]string{}
	for _, port := range minimalRoot.Ports {
		directions[port.Name] = port.Direction
	}
	if !reflect.DeepEqual(directions, map[string]string{"bread": "in", "toast": "out"}) {
		t.Errorf("action frame ports = %+v, want input and output pins", minimalRoot.Ports)
	}
	countPorts := func(nodes []renderNode) (count int) {
		for _, node := range nodes {
			count += len(node.Ports)
		}
		return count
	}
	if countPorts(minimal.Nodes) != countPorts(full.Nodes) {
		t.Errorf("action ports count differs by display: default=%d full=%d",
			countPorts(minimal.Nodes), countPorts(full.Nodes))
	}
}
