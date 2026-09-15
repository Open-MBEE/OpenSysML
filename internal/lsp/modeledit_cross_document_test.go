package lsp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// The two documents of a workspace whose view draws what the other declares.
const (
	engineParts = "package Machinery {\n\t// The engine and what turns in it.\n\tpart def Engine {\n\t\tpart rotor;\n\t\tpart stator;\n\t\tconnection connect rotor to stator;\n\t}\n}\n"

	engineViews = "package EngineViews {\n\tprivate import Views::*;\n\tprivate import StandardViewDefinitions::*;\n\n\tview engineView : InterconnectionView {\n\t\texpose Machinery::Engine;\n\t}\n}\n"
)

// engineWorkspace serves views.sysml, opened first, and parts.sysml.
func engineWorkspace(t *testing.T) (s *Server, viewsURI, partsURI uri.URI) {
	t.Helper()
	s, viewsURI = renderServer(t, "views.sysml", engineViews)
	partsURI = uri.File("parts.sysml")
	openDoc(t, s, partsURI, engineParts)
	return s, viewsURI, partsURI
}

// nodeNamed is the rendered node of that name.
func nodeNamed(t *testing.T, r *renderResult, name string) renderNode {
	t.Helper()
	for _, n := range r.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no node %q among %+v", name, r.Nodes)
	return renderNode{}
}

// tellChanged sends the document's new text to the server at version.
func tellChanged(t *testing.T, s *Server, docURI uri.URI, content string, version int32) {
	t.Helper()
	encoded, _ := json.Marshal(map[string]string{"text": content})
	sendDidChange(t, s, docURI, version, []json.RawMessage{encoded})
}

// A node drawn from another document carries what a layout operation targets
// it by — its qualified name and owners, else its declaration range — and is
// located in that document, whose range the declaration is.
func TestRenderLocatesNodesInTheDocumentDeclaringThem(t *testing.T) {
	s, viewsURI, partsURI := engineWorkspace(t)
	drawn := render(t, s, viewsURI, "EngineViews::engineView")
	rotor := nodeNamed(t, drawn, "rotor")
	if rotor.FQN != "Machinery::Engine::rotor" || rotor.Notation != "part" {
		t.Errorf("rotor = %+v, want its qualified name and notation", rotor)
	}
	if want := []renderOwner{{FQN: "Machinery::Engine"}, {FQN: "Machinery"}}; !reflect.DeepEqual(rotor.Owners, want) {
		t.Errorf("rotor owners = %+v, want %+v", rotor.Owners, want)
	}
	if rotor.Origin == nil || rotor.Origin.URI != partsURI {
		t.Fatalf("rotor origin = %+v, want one in %s", rotor.Origin, partsURI)
	}
	if got := rotor.Origin.Range.Start; got != (protocol.Position{Line: 3, Character: 2}) {
		t.Errorf("rotor is located at %+v of parts.sysml, want line 3 where `part rotor;` is", got)
	}
	if rotor.Declaration != nil {
		t.Errorf("rotor has a declaration range %+v besides its fqn", *rotor.Declaration)
	}
	if len(drawn.Edges) != 1 {
		t.Fatalf("edges = %+v, want the one connection", drawn.Edges)
	}
	edge := drawn.Edges[0]
	if edge.FQN != "" || edge.Declaration == nil || edge.Origin == nil || edge.Origin.URI != partsURI {
		t.Fatalf("edge = %+v, want an unnamed one located in parts.sysml with a declaration range", edge)
	}
	if *edge.Declaration != edge.Origin.Range || edge.Declaration.Start.Line != 5 {
		t.Errorf("edge declaration = %+v, want the origin's range on line 5 of parts.sysml", *edge.Declaration)
	}
	engine := nodeNamed(t, drawn, "Machinery::Engine")
	if engine.FQN != "Machinery::Engine" || engine.Origin == nil || engine.Origin.URI != partsURI {
		t.Errorf("Engine = %+v, want its fqn, located in parts.sysml", engine)
	}
	if rotor.DeclaredHere || engine.DeclaredHere {
		t.Errorf("rotor %v, Engine %v: another document's declarations are marked as views.sysml's own", rotor.DeclaredHere, engine.DeclaredHere)
	}
}

// Which operations reach a node is stated, not inferred from its qualified
// name: the requested document's own declarations are marked so, another
// workspace document's carry a name for a layout alone, and a bundled library's
// carry none, only where to navigate to.
func TestRenderMarksWhatEachDocumentsNodesAdmit(t *testing.T) {
	const views = "package EngineViews {\n\tprivate import Views::*;\n\tprivate import StandardViewDefinitions::*;\n\n\tpart def Mount;\n\tview engineView : InterconnectionView {\n\t\texpose Mount;\n\t\texpose Machinery::Engine;\n\t\texpose Parts::Part;\n\t}\n}\n"
	s, viewsURI := renderServer(t, "views.sysml", views)
	partsURI := uri.File("parts.sysml")
	openDoc(t, s, partsURI, engineParts)
	drawn := render(t, s, viewsURI, "EngineViews::engineView")

	mount := nodeNamed(t, drawn, "EngineViews::Mount")
	if mount.FQN != "EngineViews::Mount" || !mount.DeclaredHere || mount.Origin == nil || mount.Origin.URI != viewsURI {
		t.Errorf("Mount = %+v, want its fqn marked as views.sysml's own", mount)
	}
	rotor := nodeNamed(t, drawn, "rotor")
	if rotor.FQN != "Machinery::Engine::rotor" || rotor.DeclaredHere || rotor.Origin == nil || rotor.Origin.URI != partsURI {
		t.Errorf("rotor = %+v, want its fqn, not marked as views.sysml's own", rotor)
	}
	libraryURI := libraryURI("Systems Library/Parts.sysml")
	var library []renderNode
	for _, n := range drawn.Nodes {
		if n.Origin != nil && n.Origin.URI == libraryURI {
			library = append(library, n)
		}
	}
	if len(library) == 0 {
		t.Fatalf("nodes = %+v, want some located in the bundled Parts.sysml", drawn.Nodes)
	}
	for _, n := range library {
		if n.FQN != "" || n.Declaration != nil || n.DeclaredHere || n.Notation != "" || n.Owners != nil {
			t.Errorf("library node %+v, want its origin alone: no operation reaches a bundled library file", n)
		}
		if n.Origin.Digest == "" {
			t.Errorf("library node %q carries no digest", n.Name)
		}
	}
	for _, e := range drawn.Edges {
		if e.Origin != nil && e.Origin.URI == libraryURI && (e.FQN != "" || e.Declaration != nil) {
			t.Errorf("library edge %+v, want its origin alone", e)
		}
	}
}

// Dragging a node the view draws from another document writes the Layout into
// the view's own body: one change, to the rendered document alone.
func TestApplyModelEditSetLayoutInViewOfAnotherDocumentsElement(t *testing.T) {
	s, viewsURI, partsURI := engineWorkspace(t)
	drawn := render(t, s, viewsURI, "EngineViews::engineView")
	rotor := nodeNamed(t, drawn, "rotor")
	drag := modelEditOperation{Kind: EditSetLayout, Target: rotor.FQN, View: drawn.View, Layout: &modelEditLayout{X: 120, Y: 40}}
	out := applyModelEdit(t, s, viewsURI, drawn.Version, drag)
	applied, redrawn := redraw(t, s, viewsURI, engineViews, out, 2, drawn.View)
	if want := golden(t, s, viewsURI.Filename(), drag); applied != want {
		t.Errorf("applied edit:\n%s\nwant:\n%s", applied, want)
	}
	if !strings.Contains(applied, "\t\texpose Machinery::Engine;\n\t\tmetadata DiagramLayout::Layout about Machinery::Engine::rotor { x = 120; y = 40; }\n\t}\n") {
		t.Errorf("Layout not stated in the view body:\n%s", applied)
	}
	if string(s.ws.Document(partsURI.Filename()).Content) != engineParts {
		t.Error("parts.sysml changed for a view-local layout")
	}
	placed := nodeNamed(t, redrawn, "rotor")
	if placed.X == nil || placed.Y == nil || *placed.X != 120 || *placed.Y != 40 {
		t.Errorf("redrawn rotor = %+v, want x 120, y 40", placed)
	}
}

// One drag may write into several documents: a Layout into the view's body and
// a Route inline into the connection another document declares, targeted by
// the declaration range of that document. The edit holds one versioned change
// per document, the requested one first, each at the version the server holds.
func TestApplyModelEditLayoutReachesSeveralDocumentsInOneRequest(t *testing.T) {
	s, viewsURI, partsURI := engineWorkspace(t)
	tellChanged(t, s, partsURI, engineParts, 4)
	drawn := render(t, s, viewsURI, "EngineViews::engineView")
	rotor := nodeNamed(t, drawn, "rotor")
	edge := drawn.Edges[0]

	out := applyModelEdit(t, s, viewsURI, drawn.Version,
		modelEditOperation{Kind: EditSetLayout, Target: rotor.FQN, View: drawn.View, Layout: &modelEditLayout{X: 5, Y: 6}},
		modelEditOperation{Kind: EditSetRoute, Declaration: edge.Declaration, DeclaredIn: protocol.DocumentURI(edge.Origin.URI), Digest: edge.Origin.Digest,
			Route: []renderPoint{{X: 30, Y: 90}, {X: 30, Y: 10}}},
	)
	if out.Edit == nil || out.Refused != nil || out.Stale {
		t.Fatalf("result = %+v, want an edit", out)
	}
	if got := documentURIs(out.Edit); !reflect.DeepEqual(got, []uri.URI{viewsURI, partsURI}) {
		t.Fatalf("documentChanges = %v, want views.sysml then parts.sysml", got)
	}
	viewsChange := documentChangeFor(t, out.Edit, viewsURI)
	if v := viewsChange.TextDocument.Version; v == nil || *v != 1 {
		t.Errorf("views version = %v, want the request's 1", v)
	}
	partsChange := documentChangeFor(t, out.Edit, partsURI)
	if v := partsChange.TextDocument.Version; v == nil || *v != 4 {
		t.Errorf("parts version = %v, want the server's 4", v)
	}
	views := applyDocumentChange(t, engineViews, viewsChange)
	if !strings.Contains(views, "\t\tmetadata DiagramLayout::Layout about Machinery::Engine::rotor { x = 5; y = 6; }\n") {
		t.Errorf("views.sysml:\n%s", views)
	}
	parts := applyDocumentChange(t, engineParts, partsChange)
	wantParts := strings.Replace(engineParts, "\t\tconnection connect rotor to stator;\n",
		"\t\tconnection connect rotor to stator {\n\t\t\t@DiagramLayout::Route { points = (30, 90, 30, 10); }\n\t\t}\n", 1)
	if parts != wantParts {
		t.Errorf("parts.sysml:\n--- want\n%s\n--- got\n%s", wantParts, parts)
	}

	tellChanged(t, s, viewsURI, views, 2)
	tellChanged(t, s, partsURI, parts, 5)
	redrawn := render(t, s, viewsURI, drawn.View)
	placed := nodeNamed(t, redrawn, "rotor")
	if placed.X == nil || *placed.X != 5 || placed.Y == nil || *placed.Y != 6 {
		t.Errorf("redrawn rotor = %+v, want x 5, y 6", placed)
	}
	if got := redrawn.Edges[0].Route; len(got) != 2 || got[0] != (renderPoint{X: 30, Y: 90}) || got[1] != (renderPoint{X: 30, Y: 10}) {
		t.Errorf("redrawn route = %+v", got)
	}
	if redrawn.Edges[0].Declaration == nil || redrawn.Edges[0].Declaration.Start.Line != 5 {
		t.Errorf("redrawn edge = %+v, want its declaration still on line 5 of parts.sysml", redrawn.Edges[0])
	}

	// Dragging again updates the annotations in place, and clearing removes them with their lines.
	out = applyModelEdit(t, s, viewsURI, 2,
		modelEditOperation{Kind: EditSetLayout, Target: rotor.FQN, View: drawn.View, Layout: &modelEditLayout{X: 7, Y: 8}},
		modelEditOperation{Kind: EditSetRoute, Declaration: redrawn.Edges[0].Declaration, DeclaredIn: protocol.DocumentURI(partsURI), Digest: redrawn.Edges[0].Origin.Digest,
			Route: []renderPoint{{X: 1, Y: 2}}},
	)
	if out.Edit == nil || out.Refused != nil {
		t.Fatalf("second drag = %+v, want an edit", out)
	}
	views = applyDocumentChange(t, views, documentChangeFor(t, out.Edit, viewsURI))
	parts = applyDocumentChange(t, parts, documentChangeFor(t, out.Edit, partsURI))
	if strings.Count(views, "DiagramLayout::Layout") != 1 || !strings.Contains(views, "{ x = 7; y = 8; }") {
		t.Errorf("views.sysml after the second drag:\n%s", views)
	}
	if strings.Count(parts, "DiagramLayout::Route") != 1 || !strings.Contains(parts, "points = (1, 2);") {
		t.Errorf("parts.sysml after the second drag:\n%s", parts)
	}
	tellChanged(t, s, viewsURI, views, 3)
	tellChanged(t, s, partsURI, parts, 6)
	redrawn = render(t, s, viewsURI, drawn.View)
	out = applyModelEdit(t, s, viewsURI, 3,
		modelEditOperation{Kind: EditSetLayout, Target: rotor.FQN, View: drawn.View},
		modelEditOperation{Kind: EditSetRoute, Declaration: redrawn.Edges[0].Declaration, DeclaredIn: protocol.DocumentURI(partsURI), Digest: redrawn.Edges[0].Origin.Digest},
	)
	if out.Edit == nil || out.Refused != nil {
		t.Fatalf("clearing = %+v, want an edit", out)
	}
	if got := applyDocumentChange(t, views, documentChangeFor(t, out.Edit, viewsURI)); got != engineViews {
		t.Errorf("clearing did not restore views.sysml:\n%s", got)
	}
	if got := applyDocumentChange(t, parts, documentChangeFor(t, out.Edit, partsURI)); got != engineParts {
		t.Errorf("clearing did not restore parts.sysml:\n%s", got)
	}
}

// A document drawn directly places what it draws from another document inline,
// in that document: the requested document, unchanged, is not among the changes.
// A declaration range is read in the document declaredIn names; without it, the
// range is one of the requested document, where nothing is declared.
func TestApplyModelEditPlacesInlineIntoTheDeclaringDocument(t *testing.T) {
	const motor = "package Motors {\n\tstate def Motor {\n\t\tstate off;\n\t\tstate on;\n\t\ttransition first off then on;\n\t}\n}\n"
	const fleet = "package Fleet {\n\tstate motor : Motors::Motor;\n}\n"
	s, fleetURI := renderServer(t, "fleet.sysml", fleet)
	motorURI := uri.File("motor.sysml")
	openDoc(t, s, motorURI, motor)
	tellChanged(t, s, motorURI, motor, 2)

	drawn := render(t, s, fleetURI, "#state")
	if drawn.View != "" {
		t.Fatalf("a direct rendering names view %q", drawn.View)
	}
	off := nodeNamed(t, drawn, "off")
	if off.FQN != "Motors::Motor::off" || off.Origin == nil || off.Origin.URI != motorURI {
		t.Fatalf("off = %+v, want Motors::Motor::off located in motor.sysml", off)
	}
	if len(drawn.Edges) != 1 || drawn.Edges[0].Declaration == nil || drawn.Edges[0].Origin.URI != motorURI {
		t.Fatalf("edges = %+v, want the unnamed transition located in motor.sysml", drawn.Edges)
	}
	edge := drawn.Edges[0]

	out := applyModelEdit(t, s, fleetURI, drawn.Version,
		modelEditOperation{Kind: EditSetLayout, Target: off.FQN, Layout: &modelEditLayout{X: 10, Y: 20}},
		modelEditOperation{Kind: EditSetRoute, Declaration: edge.Declaration, DeclaredIn: protocol.DocumentURI(motorURI), Digest: edge.Origin.Digest, Route: []renderPoint{{X: 3, Y: 4}}},
	)
	if out.Edit == nil || out.Refused != nil || out.Stale {
		t.Fatalf("result = %+v, want an edit", out)
	}
	if got := documentURIs(out.Edit); !reflect.DeepEqual(got, []uri.URI{motorURI}) {
		t.Fatalf("documentChanges = %v, want motor.sysml alone", got)
	}
	change := documentChangeFor(t, out.Edit, motorURI)
	if v := change.TextDocument.Version; v == nil || *v != 2 {
		t.Errorf("motor version = %v, want the server's 2", v)
	}
	placed := applyDocumentChange(t, motor, change)
	want := strings.Replace(motor, "\t\tstate off;\n", "\t\tstate off {\n\t\t\t@DiagramLayout::Layout { x = 10; y = 20; }\n\t\t}\n", 1)
	want = strings.Replace(want, "\t\ttransition first off then on;\n", "\t\ttransition first off then on {\n\t\t\t@DiagramLayout::Route { points = (3, 4); }\n\t\t}\n", 1)
	if placed != want {
		t.Errorf("motor.sysml:\n--- want\n%s\n--- got\n%s", want, placed)
	}
	tellChanged(t, s, motorURI, placed, 3)
	redrawn := render(t, s, fleetURI, "#state")
	if redrawn.Version != drawn.Version {
		t.Errorf("fleet.sysml rendered at version %d, want %d: it was not edited", redrawn.Version, drawn.Version)
	}
	if got := nodeNamed(t, redrawn, "off"); got.X == nil || *got.X != 10 || got.Y == nil || *got.Y != 20 {
		t.Errorf("redrawn off = %+v, want x 10, y 20", got)
	}
	if got := redrawn.Edges[0].Route; len(got) != 1 || got[0] != (renderPoint{X: 3, Y: 4}) {
		t.Errorf("redrawn route = %+v, want (3, 4)", got)
	}

	unplaced := applyModelEdit(t, s, fleetURI, drawn.Version,
		modelEditOperation{Kind: EditSetRoute, Declaration: edge.Declaration, Route: []renderPoint{{X: 3, Y: 4}}})
	if unplaced.Edit != nil || len(unplaced.Refused) != 1 || unplaced.Refused[0].Failure != "unknown-target" {
		t.Fatalf("a declaration range of motor.sysml read in fleet.sysml: %+v, want an unknown-target refusal", unplaced)
	}
}

// A Canvas sizes the view in the view's document, wherever the request comes from.
func TestApplyModelEditSetCanvasOfViewInAnotherDocument(t *testing.T) {
	s, viewsURI, partsURI := engineWorkspace(t)
	out := applyModelEdit(t, s, partsURI, 1,
		modelEditOperation{Kind: EditSetCanvas, Target: "EngineViews::engineView", Canvas: &renderCanvas{Unit: "px", Width: float(1200), Height: float(800)}})
	if out.Edit == nil || out.Refused != nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	if got := documentURIs(out.Edit); !reflect.DeepEqual(got, []uri.URI{viewsURI}) {
		t.Fatalf("documentChanges = %v, want views.sysml alone", got)
	}
	sized := applyDocumentChange(t, engineViews, documentChangeFor(t, out.Edit, viewsURI))
	if !strings.Contains(sized, "\t\texpose Machinery::Engine;\n\t\t@DiagramLayout::Canvas { unit = \"px\"; width = 1200; height = 800; }\n\t}\n") {
		t.Errorf("views.sysml:\n%s", sized)
	}
}

// A document the server read from disk is written at no version; a bundled
// library declaration is never written, and the refusal names its file.
func TestApplyModelEditLayoutIntoDiskAndLibraryDocuments(t *testing.T) {
	s, viewsURI, _ := engineWorkspace(t)
	depotName := uri.File("depot.sysml").Filename()
	const depot = "package Depot {\n\tpart spare : Machinery::Engine;\n}\n"
	s.ws.SetOnDisk(depotName, []byte(depot))

	out := applyModelEdit(t, s, viewsURI, 1,
		modelEditOperation{Kind: EditSetLayout, Target: "Depot::spare", Layout: &modelEditLayout{X: 1, Y: 2}})
	if out.Edit == nil || out.Refused != nil {
		t.Fatalf("result = %+v, want an edit", out)
	}
	if got := documentURIs(out.Edit); !reflect.DeepEqual(got, []uri.URI{uri.File(depotName)}) {
		t.Fatalf("documentChanges = %v, want depot.sysml alone", got)
	}
	change := documentChangeFor(t, out.Edit, uri.File(depotName))
	if change.TextDocument.Version != nil {
		t.Errorf("depot version = %d, want none for a document read from disk", *change.TextDocument.Version)
	}
	if got := applyDocumentChange(t, depot, change); !strings.Contains(got, "\tpart spare : Machinery::Engine {\n\t\t@DiagramLayout::Layout { x = 1; y = 2; }\n\t}\n") {
		t.Errorf("depot.sysml:\n%s", got)
	}

	out = applyModelEdit(t, s, viewsURI, 1,
		modelEditOperation{Kind: EditSetLayout, Target: "Parts::Part", Layout: &modelEditLayout{X: 1, Y: 2}})
	if out.Edit != nil || len(out.Refused) != 1 {
		t.Fatalf("layout of a library declaration: %+v, want one refusal", out)
	}
	if r := out.Refused[0]; r.Failure != "referenced-elsewhere" || !strings.Contains(r.Message, "bundled library file Systems Library/Parts.sysml") {
		t.Errorf("refusal = %+v, want referenced-elsewhere naming the library file", r)
	}
}

// Validation judges every document written: when the annotation would not
// resolve in the other document, the whole request is refused with that
// document's diagnostics, and neither document changes.
func TestApplyModelEditLayoutRefusesWhenAnotherDocumentBecomesInvalid(t *testing.T) {
	shadowed := strings.Replace(engineViews, "\tview engineView", "\tpart def DiagramLayout;\n\tview engineView", 1)
	s, partsURI := renderServer(t, "parts.sysml", engineParts)
	viewsURI := uri.File("views.sysml")
	openDoc(t, s, viewsURI, shadowed)

	out := applyModelEdit(t, s, partsURI, 1,
		modelEditOperation{Kind: EditSetLayout, Target: "Machinery::Engine::stator", Layout: &modelEditLayout{X: 1, Y: 2}},
		modelEditOperation{Kind: EditSetLayout, Target: "Machinery::Engine::rotor", View: "EngineViews::engineView", Layout: &modelEditLayout{X: 5, Y: 6}},
	)
	if out.Edit != nil || len(out.Refused) != 1 {
		t.Fatalf("result = %+v, want one refusal and no edit", out)
	}
	r := out.Refused[0]
	if r.Failure != "result-invalid" || r.Operation != -1 || !strings.Contains(r.Message, "in "+viewsURI.Filename()) {
		t.Errorf("refusal = %+v, want result-invalid naming views.sysml", r)
	}
	if len(r.Diagnostics) == 0 || !strings.Contains(r.Diagnostics[0].Message, "DiagramLayout::Layout") {
		t.Errorf("diagnostics = %+v, want the unresolved DiagramLayout::Layout", r.Diagnostics)
	}
	if string(s.ws.Document(partsURI.Filename()).Content) != engineParts || string(s.ws.Document(viewsURI.Filename()).Content) != shadowed {
		t.Error("a refused request changed a document")
	}
}

// declaredIn must name a document the server holds, must accompany a
// declaration range, and a range of another document must come with the digest
// of the text it was read from: each fault is an invalid request, not a refusal.
func TestApplyModelEditRejectsMisplacedDeclaredIn(t *testing.T) {
	s, viewsURI, partsURI := engineWorkspace(t)
	drawn := render(t, s, viewsURI, "EngineViews::engineView")
	decl := drawn.Edges[0].Declaration
	digest := drawn.Edges[0].Origin.Digest
	for _, tc := range []struct {
		op   modelEditOperation
		want string
	}{
		{modelEditOperation{Kind: EditSetRoute, Declaration: decl, DeclaredIn: protocol.DocumentURI(uri.File("elsewhere.sysml")), Digest: digest, Route: []renderPoint{{X: 1, Y: 2}}}, "no document the server holds"},
		{modelEditOperation{Kind: EditSetRoute, Target: "Machinery::Engine", DeclaredIn: protocol.DocumentURI(partsURI), Route: []renderPoint{{X: 1, Y: 2}}}, "there is none"},
		{modelEditOperation{Kind: EditSetRoute, Declaration: decl, DeclaredIn: protocol.DocumentURI(partsURI), Route: []renderPoint{{X: 1, Y: 2}}}, "needs the digest"},
	} {
		_, err := call(t, s, MethodApplyModelEdit, &applyModelEditParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI},
			Version:      1,
			Operations:   []modelEditOperation{tc.op},
		})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: err = %v, want an invalid-params error saying %q", tc.op, err, tc.want)
		}
	}
	// Naming the requested document is the same as naming none.
	own := render(t, s, partsURI, "#interconnection:Machinery::Engine")
	if len(own.Edges) != 1 || own.Edges[0].Declaration == nil {
		t.Fatalf("edges of parts.sysml drawn directly = %+v", own.Edges)
	}
	out := applyModelEdit(t, s, partsURI, 1,
		modelEditOperation{Kind: EditSetRoute, Declaration: own.Edges[0].Declaration, DeclaredIn: protocol.DocumentURI(partsURI), Route: []renderPoint{{X: 1, Y: 2}}})
	if out.Edit == nil || out.Refused != nil {
		t.Fatalf("declaredIn naming the requested document: %+v, want an edit", out)
	}
	if got := documentURIs(out.Edit); !reflect.DeepEqual(got, []uri.URI{partsURI}) {
		t.Errorf("documentChanges = %v, want parts.sysml", got)
	}
}

// A declaration range of another document is a range of the text it was
// rendered from. When that document changes before the edit — here a line is
// added above the connection, so the range now spells a different declaration —
// the request is answered stale rather than written where the range now falls;
// the same text at a new version is not stale, since the range still holds.
func TestApplyModelEditIsStaleWhenAnotherDocumentsDeclarationMoved(t *testing.T) {
	s, viewsURI, partsURI := engineWorkspace(t)
	drawn := render(t, s, viewsURI, "EngineViews::engineView")
	edge := drawn.Edges[0]
	if edge.Declaration == nil || edge.Origin == nil || edge.Origin.URI != partsURI || edge.Origin.Digest == "" {
		t.Fatalf("edge = %+v, want a declaration range of parts.sysml with its digest", edge)
	}
	steer := modelEditOperation{Kind: EditSetRoute, Declaration: edge.Declaration, DeclaredIn: protocol.DocumentURI(partsURI),
		Digest: edge.Origin.Digest, Route: []renderPoint{{X: 1, Y: 2}}}

	shifted := strings.Replace(engineParts, "\t\tpart rotor;\n", "\t\tpart rotor;\n\t\tpart shaft;\n", 1)
	tellChanged(t, s, partsURI, shifted, 2)
	out := applyModelEdit(t, s, viewsURI, drawn.Version, steer)
	if !out.Stale || out.Edit != nil || out.Refused != nil {
		t.Fatalf("after parts.sysml moved the connection: %+v, want stale", out)
	}
	if out.Version != drawn.Version {
		t.Errorf("stale version = %d, want the requested document's %d, which did not change", out.Version, drawn.Version)
	}
	if string(s.ws.Document(partsURI.Filename()).Content) != shifted {
		t.Error("a stale request changed parts.sysml")
	}

	tellChanged(t, s, partsURI, engineParts, 3)
	out = applyModelEdit(t, s, viewsURI, drawn.Version, steer)
	if out.Edit == nil || out.Stale || out.Refused != nil {
		t.Fatalf("parts.sysml restored to the rendered text: %+v, want an edit", out)
	}
	change := documentChangeFor(t, out.Edit, partsURI)
	if v := change.TextDocument.Version; v == nil || *v != 3 {
		t.Errorf("parts version = %v, want the server's 3", v)
	}
	if got := applyDocumentChange(t, engineParts, change); !strings.Contains(got, "connect rotor to stator {\n\t\t\t@DiagramLayout::Route { points = (1, 2); }") {
		t.Errorf("parts.sysml:\n%s", got)
	}
}
