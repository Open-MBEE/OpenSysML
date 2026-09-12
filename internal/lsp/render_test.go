package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
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

// renderServer is a server holding one open document, as an editor session does.
func renderServer(t *testing.T, name, src string) (*Server, uri.URI) {
	t.Helper()
	s := NewServer(model.NewWorkspace())
	s.client = &recorder{}
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
	handler := s.renderHandler(func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		t.Fatalf("%s was not handled: it fell through to the next handler", req.Method())
		return nil
	})
	if err := handler(context.Background(), reply, req); err != nil {
		t.Fatalf("dispatch %s: %v", method, err)
	}
	return raw, callErr
}

// render is one opensysml/render request, decoded.
func render(t *testing.T, s *Server, docURI uri.URI, viewName string) *renderResult {
	t.Helper()
	raw, err := call(t, s, MethodRender, &renderParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		View:         viewName,
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
	}); err == nil || !strings.Contains(err.Error(), "no rendering form") {
		t.Errorf("err = %v, want it to refuse the form", err)
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
		@Canvas { unit = "px"; width = 640; }
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
	if got := fmt.Sprintf("%s|%s|%s", out.Canvas["unit"], out.Canvas["width"], out.Canvas["height"]); got != `"px"|640|` {
		t.Errorf("canvas = %s, want unit px, width 640 and no height", got)
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
}
