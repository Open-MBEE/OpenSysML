package model

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// openDoc opens a document in a workspace, as an editor does.
func openDoc(t *testing.T, name, src string) *Workspace {
	t.Helper()
	ws := NewWorkspace()
	ws.Open(name, []byte(src), 1)
	return ws
}

const twoViewModel = `package Kit {
	part def Widget {
		part cog : Cog;
	}
	part def Cog;
}

package KitViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view widgetTree {
		expose Kit::Widget;
	}

	view widgetTable : GridView {
		expose Kit::Widget;
	}
}
`

// The views a document declares are listed with the rendering kind each states.
func TestViewsListsDeclaredViewsAndKinds(t *testing.T) {
	ws := openDoc(t, "kit.sysml", twoViewModel)
	views, doc := ws.Views("kit.sysml")
	if len(views) != 2 {
		t.Fatalf("listed %d views, want 2: %+v", len(views), views)
	}
	if doc == nil || doc.Name != "kit.sysml" {
		t.Fatalf("listing came with document %+v, want kit.sysml", doc)
	}
	want := map[string]view.Kind{"KitViews::widgetTable": view.KindTable, "KitViews::widgetTree": view.KindTree}
	for _, info := range views {
		if !info.Supported {
			t.Errorf("%s listed as unsupported: %s", info.Name, info.Reason)
		}
		if info.Kind != want[info.Name] {
			t.Errorf("%s: kind = %q, want %q", info.Name, info.Kind, want[info.Name])
		}
	}
}

// Each listed view is located at its declaration in the document, the whole
// declaration and its name, so a client can tell which view the cursor is in.
func TestViewsLocateEachDeclaration(t *testing.T) {
	ws := openDoc(t, "kit.sysml", twoViewModel)
	views, doc := ws.Views("kit.sysml")
	decls := map[string]string{
		"KitViews::widgetTree":  "view widgetTree {\n\t\texpose Kit::Widget;\n\t}",
		"KitViews::widgetTable": "view widgetTable : GridView {\n\t\texpose Kit::Widget;\n\t}",
	}
	for _, info := range views {
		if !info.Origin.Located() || info.Origin.Doc != "kit.sysml" {
			t.Fatalf("%s: origin %+v is not located in kit.sysml", info.Name, info.Origin)
		}
		if got := string(doc.Content[info.Origin.Span.Offset:info.Origin.Span.End()]); got != decls[info.Name] {
			t.Errorf("%s: declaration = %q, want %q", info.Name, got, decls[info.Name])
		}
		wantName := strings.TrimPrefix(info.Name, "KitViews::")
		if got := string(doc.Content[info.Origin.Name.Offset:info.Origin.Name.End()]); got != wantName {
			t.Errorf("%s: name span = %q, want %q", info.Name, got, wantName)
		}
	}
	if views, doc := ws.Views("gone.sysml"); views != nil || doc != nil {
		t.Errorf("an unheld document listed %+v in %+v, want nothing", views, doc)
	}
}

// A view stating a rendering this implementation does not produce is listed, so
// a client can say why it cannot be drawn rather than omitting it.
func TestViewsReportsUnsupportedKindsWithAReason(t *testing.T) {
	ws := openDoc(t, "geom.sysml", `package Kit {
	part def Widget;
}

package GeomViews {
	private import StandardViewDefinitions::*;

	view geometryView : GeometryView {
		expose Kit::Widget;
	}
}
`)
	views, _ := ws.Views("geom.sysml")
	if len(views) != 1 {
		t.Fatalf("listed %d views, want 1: %+v", len(views), views)
	}
	info := views[0]
	if info.Supported {
		t.Fatal("a geometry rendering is not produced, but the view is listed as supported")
	}
	if info.Kind != view.KindGeometry {
		t.Errorf("kind = %q, want %q", info.Kind, view.KindGeometry)
	}
	if !strings.Contains(info.Reason, "geometry rendering") || !strings.Contains(info.Reason, "is not supported") {
		t.Errorf("reason = %q, want it to say a geometry rendering is not supported", info.Reason)
	}
}

// Naming a view renders it; naming none renders the document's own view when it
// declares exactly one.
func TestRenderViewRendersTheNamedAndTheSoleView(t *testing.T) {
	ws := openDoc(t, "kit.sysml", twoViewModel)
	rendering, _, err := ws.RenderView("kit.sysml", "KitViews::widgetTable")
	if err != nil {
		t.Fatalf("render the table view: %v", err)
	}
	if rendering.Kind != view.KindTable {
		t.Errorf("kind = %q, want %q", rendering.Kind, view.KindTable)
	}

	sole := openDoc(t, "sole.sysml", `package Kit {
	part def Widget;
}

package KitViews {
	private import Views::*;

	view widgetTree {
		expose Kit::Widget;
	}
}
`)
	rendering, _, err = sole.RenderView("sole.sysml", "")
	if err != nil {
		t.Fatalf("render the sole view: %v", err)
	}
	if rendering.View != "KitViews::widgetTree" {
		t.Errorf("View = %q, want KitViews::widgetTree", rendering.View)
	}
}

// The snapshot a rendering comes with holds every document as the rendering
// read it — the one asked of and the others it draws from — so that a span the
// rendering locates in any of them is placed in the text it was read from,
// however the workspace has changed since.
func TestRenderViewSnapshotHoldsEveryDocumentAsRendered(t *testing.T) {
	const parts = "package Machinery {\n\tpart def Engine {\n\t\tpart rotor;\n\t}\n}\n"
	const views = "package EngineViews {\n\tprivate import Views::*;\n\tprivate import StandardViewDefinitions::*;\n\n\tview engineView : InterconnectionView {\n\t\texpose Machinery::Engine;\n\t}\n}\n"
	ws := openDoc(t, "views.sysml", views)
	ws.Open("parts.sysml", []byte(parts), 1)
	rendering, snapshot, err := ws.RenderView("views.sysml", "EngineViews::engineView")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if snapshot.Rendered != ws.Document("views.sysml") {
		t.Error("Rendered is not the document the rendering was asked of")
	}
	rendered := snapshot.Document("parts.sysml")
	if rendered == nil || rendered != ws.Document("parts.sysml") {
		t.Fatalf("snapshot holds parts.sysml as %v, want the workspace's document", rendered)
	}
	if got := snapshot.Document("Views.sysml"); got != nil {
		t.Errorf("snapshot holds a bundled library file as %v, want none", got)
	}

	ws.Update("parts.sysml", []byte("// The engine, restated.\n"+parts), 2)
	if snapshot.Document("parts.sysml") != rendered || rendered.Version != 1 || string(rendered.Content) != parts {
		t.Errorf("after parts.sysml changed, the snapshot holds it as %+v, want the text rendered at version 1", snapshot.Document("parts.sysml"))
	}
	if current := ws.Document("parts.sysml"); current == rendered || current.Version != 2 {
		t.Errorf("the workspace holds parts.sysml at %d, want the newer version 2", current.Version)
	}
	var rotor view.NodeData
	for _, node := range rendering.Data().Nodes {
		if node.Name == "rotor" {
			rotor = node
		}
	}
	if rotor.Origin.Doc != "parts.sysml" || !rotor.Origin.Located() {
		t.Fatalf("rotor origin = %+v, want one located in parts.sysml", rotor.Origin)
	}
	if got := string(rendered.Content[rotor.Origin.Span.Offset : rotor.Origin.Span.Offset+rotor.Origin.Span.Len]); !strings.HasPrefix(got, "part rotor;") {
		t.Errorf("rotor's span in the snapshot's text spells %q, want the declaration", got)
	}
}

// A transition a usage inherits from a definition in another document is
// located there and labelled with its trigger and guard as written there.
func TestRenderViewLocatesInheritedTransitionsInTheirDocument(t *testing.T) {
	const defs = "package Plant {\n\tattribute def Fault;\n\tstate def Machine {\n\t\tattribute load;\n\t\tentry; then off;\n\t\tstate off;\n\t\tstate on;\n\t\ttransition first off accept Fault if load > 3 then on;\n\t}\n}\n"
	const uses = "package Uses {\n\tprivate import Views::*;\n\tprivate import StandardViewDefinitions::*;\n\n\tstate machine : Plant::Machine;\n\n\tview machineView : StateTransitionView {\n\t\texpose machine;\n\t}\n}\n"
	ws := openDoc(t, "uses.sysml", uses)
	ws.Open("defs.sysml", []byte(defs), 1)
	rendering, _, err := ws.RenderView("uses.sysml", "Uses::machineView")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var labels []string
	for _, edge := range rendering.Data().Edges {
		if edge.Label != "" {
			labels = append(labels, edge.Label)
		}
		if edge.Origin.Doc != "defs.sysml" {
			t.Errorf("edge %s -> %s is located in %q, want defs.sysml", edge.From, edge.To, edge.Origin.Doc)
		}
	}
	if want := []string{"accept Fault [load > 3]"}; !reflect.DeepEqual(labels, want) {
		t.Errorf("labels = %q, want %q; notices %v", labels, want, rendering.Notices)
	}
}

// Naming no view where the document declares several reports the ambiguity and
// names the candidates, rather than picking one.
func TestRenderViewReportsAmbiguity(t *testing.T) {
	ws := openDoc(t, "kit.sysml", twoViewModel)
	_, _, err := ws.RenderView("kit.sysml", "")
	if err == nil {
		t.Fatal("rendering an ambiguous document succeeded")
	}
	for _, want := range []string{"declares 2 views", "KitViews::widgetTree", "KitViews::widgetTable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// A document declaring no view says so, and says what to render instead.
func TestRenderViewOnADocumentWithNoViews(t *testing.T) {
	ws := openDoc(t, "plain.sysml", "package Kit {\n\tpart def Widget;\n}\n")
	_, _, err := ws.RenderView("plain.sysml", "")
	if !errors.Is(err, ErrNoView) {
		t.Fatalf("err = %v, want ErrNoView", err)
	}
	if !strings.Contains(err.Error(), "#tree") {
		t.Errorf("error %q does not point at a pseudo-view", err)
	}
}

// A pseudo-view renders a document that declares no view, through the same
// renderer, naming no view and stating why.
func TestRenderViewPseudoViewRendersWithoutADeclaredView(t *testing.T) {
	ws := openDoc(t, "plain.sysml", `package Kit {
	part def Widget {
		part cog : Cog;
	}
	part def Cog;
}
`)
	rendering, _, err := ws.RenderView("plain.sysml", "#tree")
	if err != nil {
		t.Fatalf("render #tree: %v", err)
	}
	if rendering.Kind != view.KindTree {
		t.Errorf("kind = %q, want %q", rendering.Kind, view.KindTree)
	}
	if rendering.View != "" {
		t.Errorf("View = %q, want empty: no view was declared", rendering.View)
	}
	if !strings.Contains(rendering.Stated, "no view declared") {
		t.Errorf("Stated = %q, want it to say no view was declared", rendering.Stated)
	}
	names := map[string]bool{}
	for _, node := range rendering.Data().Nodes {
		names[node.Name] = true
	}
	for _, want := range []string{"Kit::Widget", "cog"} {
		if !names[want] {
			t.Errorf("#tree rendering has no node %q; nodes: %v", want, names)
		}
	}

	// Nothing was added to the index: a later render still finds one document's
	// declarations and no view.
	if views, _ := ws.Views("plain.sysml"); len(views) != 0 {
		t.Errorf("rendering a pseudo-view added views to the document: %+v", views)
	}
}

// A pseudo-view naming an element renders that element, and locates it.
func TestRenderViewPseudoViewOfANamedElement(t *testing.T) {
	ws := openDoc(t, "states.sysml", `package Machines {
	state def VehicleStates {
		entry; then off;
		state off;
		state running;
		transition first off then running;
	}
}
`)
	rendering, _, err := ws.RenderView("states.sysml", "#state:Machines::VehicleStates")
	if err != nil {
		t.Fatalf("render #state: %v", err)
	}
	if rendering.Kind != view.KindState {
		t.Errorf("kind = %q, want %q", rendering.Kind, view.KindState)
	}
	if !strings.Contains(rendering.Stated, "Machines::VehicleStates") {
		t.Errorf("Stated = %q, want it to name the element rendered", rendering.Stated)
	}
	located := 0
	for _, node := range rendering.Data().Nodes {
		if node.Origin.Located() {
			located++
		}
	}
	if located == 0 {
		t.Error("no node of the state rendering carries an origin")
	}
}

// An unknown view name, and an element a pseudo-view names that the document
// does not declare, are refused with a message naming what was asked for.
func TestRenderViewRefusesUnknownNames(t *testing.T) {
	ws := openDoc(t, "kit.sysml", twoViewModel)
	if _, _, err := ws.RenderView("kit.sysml", "KitViews::goneView"); err == nil ||
		!strings.Contains(err.Error(), "no view named KitViews::goneView") {
		t.Errorf("err = %v, want it to say there is no such view", err)
	}
	if _, _, err := ws.RenderView("kit.sysml", "#state:Kit::Nothing"); err == nil ||
		!strings.Contains(err.Error(), "Kit::Nothing") {
		t.Errorf("err = %v, want it to name the element that is missing", err)
	}
	if _, _, err := ws.RenderView("kit.sysml", "#not-a-rendering"); err == nil ||
		!strings.Contains(err.Error(), "#tree") {
		t.Errorf("err = %v, want it to list the pseudo-views", err)
	}
	if _, _, err := ws.RenderView("gone.sysml", ""); err == nil ||
		!strings.Contains(err.Error(), "no such document") {
		t.Errorf("err = %v, want it to say there is no such document", err)
	}
}

// A view stating an unsupported rendering is refused with the reason and the
// remedy, not rendered as another kind.
func TestRenderViewRefusesAnUnsupportedKind(t *testing.T) {
	ws := openDoc(t, "textual.sysml", `package Kit {
	part def Widget;
}

package TextViews {
	private import Views::*;

	view textView {
		expose Kit::Widget;
		render Views::asTextualNotation;
	}
}
`)
	_, _, err := ws.RenderView("textual.sysml", "TextViews::textView")
	var unsupported *view.UnsupportedKindError
	if !errors.As(err, &unsupported) {
		t.Fatalf("err = %v, want an *UnsupportedKindError", err)
	}
	if unsupported.Kind != view.KindTextual {
		t.Errorf("kind = %q, want %q", unsupported.Kind, view.KindTextual)
	}
}

const shortNameModel = `package Kit {
	part def <w> Widget;
}

package KitViews {
	private import Views::*;

	view <v> widgetTree {
		expose Kit::Widget;
	}
}
`

// A declaration carrying a short name as well as a name is one declaration: it is
// listed once, and it is not an ambiguity.
func TestShortNamedDeclarationsAreCountedOnce(t *testing.T) {
	ws := openDoc(t, "kit.sysml", shortNameModel)
	if views, _ := ws.Views("kit.sysml"); len(views) != 1 {
		t.Fatalf("listed %d views, want 1: %+v", len(views), views)
	}
	if _, _, err := ws.RenderView("kit.sysml", ""); err != nil {
		t.Fatalf("rendering the document's only view: %v", err)
	}
	rendering, _, err := ws.RenderView("kit.sysml", "#tree")
	if err != nil {
		t.Fatalf("rendering #tree: %v", err)
	}
	seen := map[string]int{}
	for _, node := range rendering.Data().Nodes {
		seen[node.Name]++
	}
	for label, count := range seen {
		if count > 1 {
			t.Errorf("node %q drawn %d times, want once", label, count)
		}
	}
}

// A transition a machine inherits from a definition in another document is
// labelled as written there: its timed trigger and guard verbatim.
func TestRenderViewLabelsInheritedTransitionsFromTheirDocument(t *testing.T) {
	ws := openDoc(t, "base.sysml", `package Plant {
	state def Machine {
		attribute ready : Boolean;
		entry; then idle;
		state idle;
		transition first idle accept after 5 [s] if ready then done;
		state done;
	}
}
`)
	ws.Open("derived.sysml", []byte(`package Derived {
	private import StandardViewDefinitions::*;
	state def Special :> Plant::Machine;
	view specialStates : StateTransitionView { expose Derived::Special; }
}
`), 1)
	rendering, _, err := ws.RenderView("derived.sysml", "Derived::specialStates")
	if err != nil {
		t.Fatalf("render the derived machine: %v", err)
	}
	var labels []string
	for _, edge := range rendering.Data().Edges {
		labels = append(labels, edge.Label)
	}
	want := "after 5 [s] [ready]"
	for _, label := range labels {
		if label == want {
			return
		}
	}
	t.Errorf("edge labels = %q, want %q among them", labels, want)
}
