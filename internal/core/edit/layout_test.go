package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// plantModel is a model with an interconnection view over parts and a
// connection, and a state machine with a transition, none of them placed.
const plantModel = `package Plant {
    part def Pump;
    part def Tank;
    part def Loop {
        // The pump feeds the tank.
        part pump : Pump;
        part tank : Tank;
        connection supply connect pump to tank;
    }
    state def Motor {
        state off;
        state on;
        transition t first off then on;
    }
}
package PlantViews {
    private import Views::*;
    private import StandardViewDefinitions::*;
    view loopView {
        expose Plant::Loop;
        render asInterconnectionDiagram;
    }
    view motorView : StateTransitionView {
        expose Plant::Motor;
    }
}
`

func at(x, y float64) *semantics.Layout {
	return &semantics.Layout{X: x, Y: y}
}

func applyLayout(t *testing.T, content string, ops ...Operation) string {
	t.Helper()
	m := loadContent(t, "plant.sysml", content)
	requireClean(t, m)
	res, err := Apply(m, ops)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertOnlySpanChanged(t, m, res)
	return string(res.Content)
}

func requireReplaced(t *testing.T, content, old, replacement, got string) {
	t.Helper()
	want := strings.Replace(content, old, replacement, 1)
	if want == content {
		t.Fatalf("test does not find %q in its fixture", old)
	}
	if got != want {
		t.Fatalf("result differs from expectation:\n--- want\n%s\n--- got\n%s", want, got)
	}
}

func TestSetLayoutInlineOpensBodylessDeclaration(t *testing.T) {
	got := applyLayout(t, plantModel, SetLayout("Plant::Loop::pump", "", at(40, 60.5)))
	requireReplaced(t, plantModel,
		"        part pump : Pump;\n",
		"        part pump : Pump {\n            @DiagramLayout::Layout { x = 40; y = 60.5; }\n        }\n",
		got)
}

func TestSetLayoutInViewBody(t *testing.T) {
	got := applyLayout(t, plantModel, SetLayout("Plant::Loop::pump", "PlantViews::loopView", at(40, 60)))
	requireReplaced(t, plantModel,
		"        render asInterconnectionDiagram;\n    }\n",
		"        render asInterconnectionDiagram;\n        metadata DiagramLayout::Layout about Plant::Loop::pump { x = 40; y = 60; }\n    }\n",
		got)
}

func TestSetLayoutWithSizeAndCollapsed(t *testing.T) {
	layout := &semantics.Layout{X: 1, Y: 2, Width: 300, Height: 120, HasSize: true, Collapsed: true}
	got := applyLayout(t, plantModel, SetLayout("Plant::Loop::pump", "PlantViews::loopView", layout))
	if !strings.Contains(got, "{ x = 1; y = 2; width = 300; height = 120; collapsed = true; }") {
		t.Fatalf("size and collapsed not written:\n%s", got)
	}
}

func TestSetLayoutUpdatesValuesInPlace(t *testing.T) {
	placed := strings.Replace(plantModel,
		"        render asInterconnectionDiagram;\n",
		"        render asInterconnectionDiagram;\n"+
			"        // Where the pump sits.\n"+
			"        metadata DiagramLayout::Layout about Plant::Loop::pump {\n"+
			"            x = 10; // left\n"+
			"            y = 20;\n"+
			"            collapsed = true;\n"+
			"        }\n", 1)
	got := applyLayout(t, placed, SetLayout("Plant::Loop::pump", "PlantViews::loopView", at(300, 45.25)))
	requireReplaced(t, placed,
		"            x = 10; // left\n            y = 20;\n            collapsed = true;\n",
		"            x = 300; // left\n            y = 45.25;\n",
		got)
}

func TestSetLayoutAddsBindingsToExistingAnnotation(t *testing.T) {
	placed := strings.Replace(plantModel,
		"        part pump : Pump;\n",
		"        part pump : Pump {\n            @DiagramLayout::Layout {\n                x = 10;\n                y = 20;\n            }\n        }\n", 1)
	layout := &semantics.Layout{X: 10, Y: 20, Width: 200, Height: 80, HasSize: true}
	got := applyLayout(t, placed, SetLayout("Plant::Loop::pump", "", layout))
	requireReplaced(t, placed,
		"                y = 20;\n",
		"                y = 20;\n                width = 200;\n                height = 80;\n",
		got)

	oneLine := strings.Replace(plantModel,
		"        part pump : Pump;\n",
		"        part pump : Pump { @DiagramLayout::Layout { x = 10; y = 20; } }\n", 1)
	got = applyLayout(t, oneLine, SetLayout("Plant::Loop::pump", "", &semantics.Layout{X: 10, Y: 20, Collapsed: true}))
	requireReplaced(t, oneLine,
		"{ x = 10; y = 20; }",
		"{ x = 10; y = 20; collapsed = true; }",
		got)
}

func TestSetLayoutPrefersTheViewLocalAnnotation(t *testing.T) {
	placed := strings.Replace(plantModel,
		"        part pump : Pump;\n",
		"        part pump : Pump { @DiagramLayout::Layout { x = 1; y = 1; } }\n", 1)
	placed = strings.Replace(placed,
		"        render asInterconnectionDiagram;\n",
		"        render asInterconnectionDiagram;\n        metadata DiagramLayout::Layout about Plant::Loop::pump { x = 2; y = 2; }\n", 1)
	got := applyLayout(t, placed, SetLayout("Plant::Loop::pump", "PlantViews::loopView", at(3, 4)))
	requireReplaced(t, placed, "{ x = 2; y = 2; }", "{ x = 3; y = 4; }", got)
	got = applyLayout(t, placed, SetLayout("Plant::Loop::pump", "", at(5, 6)))
	requireReplaced(t, placed, "{ x = 1; y = 1; }", "{ x = 5; y = 6; }", got)
}

func TestClearLayoutRemovesAnnotationWithOwnedTrivia(t *testing.T) {
	placed := strings.Replace(plantModel,
		"        render asInterconnectionDiagram;\n",
		"        render asInterconnectionDiagram;\n"+
			"        // Where the pump sits.\n"+
			"        metadata DiagramLayout::Layout about Plant::Loop::pump { x = 10; y = 20; }\n", 1)
	got := applyLayout(t, placed, SetLayout("Plant::Loop::pump", "PlantViews::loopView", nil))
	if got != plantModel {
		t.Fatalf("clearing did not restore the unplaced model:\n%s", got)
	}
}

func TestClearInlineLayoutClosesTheBodyItOpened(t *testing.T) {
	placed := applyLayout(t, plantModel, SetLayout("Plant::Loop::pump", "", at(40, 60)))
	got := applyLayout(t, placed, SetLayout("Plant::Loop::pump", "", nil))
	if got != plantModel {
		t.Fatalf("clearing did not restore the bodyless declaration:\n%s", got)
	}

	withSibling := strings.Replace(plantModel,
		"        part pump : Pump;\n",
		"        part pump : Pump {\n            @DiagramLayout::Layout { x = 1; y = 2; }\n            attribute mass : ScalarValues::Real;\n        }\n", 1)
	got = applyLayout(t, withSibling, SetLayout("Plant::Loop::pump", "", nil))
	requireReplaced(t, withSibling, "            @DiagramLayout::Layout { x = 1; y = 2; }\n", "", got)
}

func TestSetRouteOnTransitionAndConnection(t *testing.T) {
	route := &semantics.Route{Points: []semantics.Waypoint{{X: 100, Y: 50}, {X: 150, Y: 75.5}}}
	got := applyLayout(t, plantModel, SetRoute("Plant::Motor::t", "", route))
	requireReplaced(t, plantModel,
		"        transition t first off then on;\n",
		"        transition t first off then on {\n            @DiagramLayout::Route { points = (100, 50, 150, 75.5); }\n        }\n",
		got)

	got = applyLayout(t, plantModel, SetRoute("Plant::Loop::supply", "PlantViews::loopView", route))
	if !strings.Contains(got, "        metadata DiagramLayout::Route about Plant::Loop::supply { points = (100, 50, 150, 75.5); }\n    }\n") {
		t.Fatalf("route not stated in the view body:\n%s", got)
	}

	routed := strings.Replace(plantModel,
		"        transition t first off then on;\n",
		"        transition t first off then on { @DiagramLayout::Route { points = (1, 2, 3, 4); } }\n", 1)
	got = applyLayout(t, routed, SetRoute("Plant::Motor::t", "", &semantics.Route{Points: []semantics.Waypoint{{X: 9, Y: 8}}}))
	requireReplaced(t, routed, "points = (1, 2, 3, 4);", "points = (9, 8);", got)
}

// unnamedModel is plantModel with a transition and a connection no qualified
// name reaches: the transition is unnamed, the part declaring the connection is.
var unnamedModel = strings.Replace(strings.Replace(plantModel,
	"        transition t first off then on;\n",
	"        transition t first off then on;\n        transition first on then off;\n", 1),
	"        connection supply connect pump to tank;\n",
	"        connection supply connect pump to tank;\n        part : Pump { part a; part b; connection line connect a to b; }\n", 1)

// declaredAt is the span of the declaration a model's source spells as decl.
func declaredAt(t *testing.T, content, decl string) source.Span {
	t.Helper()
	m := loadContent(t, "plant.sysml", content)
	offset := strings.Index(content, decl)
	if offset < 0 {
		t.Fatalf("fixture lacks %q", decl)
	}
	var span source.Span
	var walk func(*symbols.Scope)
	walk = func(s *symbols.Scope) {
		s.ForEachMember(func(sym *symbols.Symbol) bool {
			if sym.DeclSpan.Offset == offset {
				span = sym.DeclSpan
			}
			return span.Len == 0
		})
		for _, child := range s.Children() {
			if span.Len == 0 {
				walk(child)
			}
		}
	}
	walk(m.Index.DocumentRoot("plant.sysml"))
	if span.Len == 0 {
		t.Fatalf("nothing declared at %q", decl)
	}
	return span
}

func TestSetRouteAtOfUnnamedTransition(t *testing.T) {
	unnamed := declaredAt(t, unnamedModel, "transition first on then off;")
	route := &semantics.Route{Points: []semantics.Waypoint{{X: 30, Y: 90}, {X: 30, Y: 10}}}
	got := applyLayout(t, unnamedModel, SetRouteAt(unnamed, "", route))
	requireReplaced(t, unnamedModel,
		"        transition first on then off;\n",
		"        transition first on then off {\n            @DiagramLayout::Route { points = (30, 90, 30, 10); }\n        }\n",
		got)

	routed := got
	decl := declaredAt(t, routed, "transition first on then off {")
	got = applyLayout(t, routed, SetRouteAt(decl, "", &semantics.Route{Points: []semantics.Waypoint{{X: 5, Y: 6}}}))
	requireReplaced(t, routed, "points = (30, 90, 30, 10);", "points = (5, 6);", got)

	got = applyLayout(t, routed, SetRouteAt(decl, "", nil))
	if got != unnamedModel {
		t.Fatalf("clearing the route did not restore the model:\n%s", got)
	}

	_, err := Apply(loadContent(t, "plant.sysml", unnamedModel), []Operation{SetRouteAt(unnamed, "PlantViews::motorView", route)})
	if e := editError(t, err); e.Failure != FailureNotNamed || !strings.Contains(e.Message, "no qualified name") {
		t.Fatalf("view-local route of an unnamed transition: got %v", err)
	}
}

func TestSetRouteAtOfConnectionInUnnamedPart(t *testing.T) {
	decl := declaredAt(t, unnamedModel, "connection line connect a to b;")
	route := &semantics.Route{Points: []semantics.Waypoint{{X: 1, Y: 2}}}
	got := applyLayout(t, unnamedModel, SetRouteAt(decl, "", route))
	requireReplaced(t, unnamedModel,
		"connection line connect a to b; }\n",
		"connection line connect a to b {\n            @DiagramLayout::Route { points = (1, 2); }\n        } }\n",
		got)

	_, err := Apply(loadContent(t, "plant.sysml", unnamedModel), []Operation{SetRouteAt(decl, "PlantViews::loopView", route)})
	if e := editError(t, err); e.Failure != FailureNotNamed || !strings.Contains(e.Message, "no qualified name") {
		t.Fatalf("view-local route of a connection in an unnamed part: got %v", err)
	}
}

// applyOps applies ops to content as one request and returns the notation.
func applyOps(t *testing.T, content string, ops ...Operation) string {
	t.Helper()
	res, err := Apply(loadContent(t, "plant.sysml", content), ops)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return string(res.Content)
}

// One drag places an unnamed part and steers the connection inside it; whichever
// is written first moves or resizes the other's declaration before it is found.
func TestLayoutBatchFollowsDeclarationsAnEarlierOperationMoved(t *testing.T) {
	part := declaredAt(t, unnamedModel, "part : Pump {")
	line := declaredAt(t, unnamedModel, "connection line connect a to b;")
	route := &semantics.Route{Points: []semantics.Waypoint{{X: 1, Y: 2}}}
	want := strings.Replace(unnamedModel,
		"        part : Pump { part a; part b; connection line connect a to b; }\n",
		"        part : Pump { part a; part b; connection line connect a to b {\n            @DiagramLayout::Route { points = (1, 2); }\n        } \n            @DiagramLayout::Layout { x = 7; y = 8; }\n        }\n", 1)

	got := applyOps(t, unnamedModel, SetLayoutAt(part, "", at(7, 8)), SetRouteAt(line, "", route))
	if got != want {
		t.Fatalf("part then connection:\n--- want\n%s\n--- got\n%s", want, got)
	}
	got = applyOps(t, unnamedModel, SetRouteAt(line, "", route), SetLayoutAt(part, "", at(7, 8)))
	if got != want {
		t.Fatalf("connection then part:\n--- want\n%s\n--- got\n%s", want, got)
	}

	// A Layout ahead of the connection lengthens in place, moving the connection.
	placed := strings.Replace(unnamedModel,
		"part : Pump { part a; part b; connection line connect a to b; }",
		"part : Pump { @DiagramLayout::Layout { x = 7; y = 8; } part a; part b; connection line connect a to b { @DiagramLayout::Route { points = (1, 2); } } }", 1)
	part = declaredAt(t, placed, "part : Pump {")
	line = declaredAt(t, placed, "connection line connect a to b {")
	steered := &semantics.Route{Points: []semantics.Waypoint{{X: 3, Y: 4}, {X: 5, Y: 6}}}
	want = strings.Replace(strings.Replace(placed, "x = 7;", "x = 700;", 1), "points = (1, 2);", "points = (3, 4, 5, 6);", 1)
	got = applyOps(t, placed, SetLayoutAt(part, "", at(700, 8)), SetRouteAt(line, "", steered))
	if got != want {
		t.Fatalf("part then connection, in place:\n--- want\n%s\n--- got\n%s", want, got)
	}
	got = applyOps(t, placed, SetRouteAt(line, "", steered), SetLayoutAt(part, "", at(700, 8)))
	if got != want {
		t.Fatalf("connection then part, in place:\n--- want\n%s\n--- got\n%s", want, got)
	}

	// A named transition gaining a body moves the unnamed one after it.
	unnamed := declaredAt(t, unnamedModel, "transition first on then off;")
	got = applyOps(t, unnamedModel, SetRoute("Plant::Motor::t", "", route), SetRouteAt(unnamed, "", steered))
	want = strings.Replace(unnamedModel,
		"        transition t first off then on;\n        transition first on then off;\n",
		"        transition t first off then on {\n            @DiagramLayout::Route { points = (1, 2); }\n        }\n        transition first on then off {\n            @DiagramLayout::Route { points = (3, 4, 5, 6); }\n        }\n", 1)
	if got != want {
		t.Fatalf("named then unnamed transition:\n--- want\n%s\n--- got\n%s", want, got)
	}

	// A declaration an earlier operation removed is not found again.
	_, err := Apply(loadContent(t, "plant.sysml", placed), []Operation{Delete("Plant::Loop", true), SetRouteAt(line, "", nil)})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || e.OperationIndex != 1 {
		t.Fatalf("route of a deleted declaration: got %v", err)
	}
}

func TestDeclarationRefusals(t *testing.T) {
	m := loadContent(t, "plant.sysml", plantModel)
	nowhere := source.Span{Offset: 3, Len: 4}
	_, err := Apply(m, []Operation{SetLayoutAt(nowhere, "", at(1, 2))})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || !strings.Contains(e.Message, "nothing is declared at 1:4") {
		t.Fatalf("layout at no declaration: got %v", err)
	}
	decl := declaredAt(t, plantModel, "part pump : Pump;")
	_, err = Apply(m, []Operation{{Kind: OpDelete, Declaration: decl}})
	if e := editError(t, err); e.Failure != FailureInvalidValue || !strings.Contains(e.Message, "layout operation") {
		t.Fatalf("delete by declaration: got %v", err)
	}
}

func TestSetCanvas(t *testing.T) {
	canvas := &semantics.Canvas{Unit: "px", Width: 1200, Height: 800, HasSize: true}
	got := applyLayout(t, plantModel, SetCanvas("PlantViews::loopView", canvas))
	requireReplaced(t, plantModel,
		"        render asInterconnectionDiagram;\n    }\n",
		"        render asInterconnectionDiagram;\n        @DiagramLayout::Canvas { unit = \"px\"; width = 1200; height = 800; }\n    }\n",
		got)
	sized := got
	got = applyLayout(t, sized, SetCanvas("PlantViews::loopView", &semantics.Canvas{Width: 640, Height: 480, HasSize: true}))
	requireReplaced(t, sized, `{ unit = "px"; width = 1200; height = 800; }`, "{ width = 640; height = 480; }", got)
	got = applyLayout(t, sized, SetCanvas("PlantViews::loopView", nil))
	if got != plantModel {
		t.Fatalf("clearing the canvas did not restore the model:\n%s", got)
	}
}

// engineParts and engineViews are a workspace of two documents: one declares
// the parts, the other a view exposing them.
const engineParts = "package Machinery {\n    part def Engine {\n        part rotor;\n        part stator;\n        connection connect rotor to stator;\n    }\n}\n"

const engineViews = "package EngineViews {\n    private import Views::*;\n    private import StandardViewDefinitions::*;\n    view engineView {\n        expose Machinery::Engine;\n        render asInterconnectionDiagram;\n    }\n}\n"

const rotorInView = "metadata DiagramLayout::Layout about Machinery::Engine::rotor { x = 5; y = 6; }"

// A view of the edited document places an element another document declares
// in its own body; an inline annotation belongs to the element's document, which
// refuses when the model hands out no source for it.
func TestSetLayoutOfElementInAnotherDocumentFromAView(t *testing.T) {
	m := loadWorkspace(t, "views.sysml", engineViews, map[string]string{"parts.sysml": engineParts})
	requireClean(t, m)
	res, err := Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6))})
	if err != nil {
		t.Fatalf("view-local layout of a sibling document's element: %v", err)
	}
	if !strings.Contains(string(res.Content), rotorInView) || len(res.Others) != 0 {
		t.Fatalf("layout not stated in the view alone:\n%s\nothers: %v", res.Content, otherNames(res))
	}
	_, err = Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "", at(5, 6))})
	if e := editError(t, err); e.Failure != FailureReferencedElsewhere || !strings.Contains(e.Message, "parts.sysml") {
		t.Fatalf("inline layout of another document's element: got %v", err)
	}
}

// Placing an element in a view another document declares writes the view's
// document and leaves the edited one alone; dragging again updates the
// annotation there in place, and clearing it removes it with its line.
func TestSetLayoutInViewOfAnotherDocument(t *testing.T) {
	m := loadEditableWorkspace(t, "parts.sysml", engineParts, map[string]string{"views.sysml": engineViews})
	requireClean(t, m)
	res := applyOne(t, m, SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6)))
	if string(res.Content) != engineParts || len(res.Applied) != 0 {
		t.Fatalf("parts.sysml changed:\n%s\napplied %+v", res.Content, res.Applied)
	}
	placed := strings.Replace(engineViews,
		"        render asInterconnectionDiagram;\n    }\n",
		"        render asInterconnectionDiagram;\n        "+rotorInView+"\n    }\n", 1)
	if got := otherContent(t, res, "views.sysml"); got != placed {
		t.Fatalf("views.sysml:\n--- want\n%s\n--- got\n%s", placed, got)
	}
	if a := res.Others[0].Applied; len(a) != 1 || a[0].OperationIndex != 0 || a[0].Target != "Machinery::Engine::rotor" {
		t.Fatalf("views.sysml applied = %+v, want one insertion of operation 0", a)
	}

	m = loadEditableWorkspace(t, "parts.sysml", engineParts, map[string]string{"views.sysml": placed})
	requireClean(t, m)
	res = applyOne(t, m, SetLayout("Machinery::Engine::rotor", "EngineViews::engineView",
		&semantics.Layout{X: 50, Y: 6, Width: 200, Height: 80, HasSize: true}))
	want := strings.Replace(placed, "{ x = 5; y = 6; }", "{ x = 50; y = 6; width = 200; height = 80; }", 1)
	if got := otherContent(t, res, "views.sysml"); got != want {
		t.Fatalf("views.sysml in place:\n--- want\n%s\n--- got\n%s", want, got)
	}

	res = applyOne(t, m, SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", nil))
	if got := otherContent(t, res, "views.sysml"); got != engineViews {
		t.Fatalf("clearing did not restore views.sysml:\n%s", got)
	}
}

// An inline Layout or Route of an element another document declares is written
// into that element's body in its own document, opening the body when it has
// none and closing it again when the annotation is cleared.
func TestSetLayoutInlineIntoAnotherDocument(t *testing.T) {
	m := loadEditableWorkspace(t, "views.sysml", engineViews, map[string]string{"parts.sysml": engineParts})
	requireClean(t, m)
	res := applyOne(t, m, SetLayout("Machinery::Engine::rotor", "", at(5, 6)))
	if string(res.Content) != engineViews {
		t.Fatalf("views.sysml changed:\n%s", res.Content)
	}
	placed := strings.Replace(engineParts, "        part rotor;\n",
		"        part rotor {\n            @DiagramLayout::Layout { x = 5; y = 6; }\n        }\n", 1)
	if got := otherContent(t, res, "parts.sysml"); got != placed {
		t.Fatalf("parts.sysml:\n--- want\n%s\n--- got\n%s", placed, got)
	}

	m = loadEditableWorkspace(t, "views.sysml", engineViews, map[string]string{"parts.sysml": placed})
	requireClean(t, m)
	res = applyOne(t, m, SetLayout("Machinery::Engine::rotor", "", nil))
	if got := otherContent(t, res, "parts.sysml"); got != engineParts {
		t.Fatalf("clearing did not restore parts.sysml:\n%s", got)
	}

	m = loadEditableWorkspace(t, "views.sysml", engineViews, map[string]string{"parts.sysml": engineParts})
	line := declaredAt(t, engineParts, "connection connect rotor to stator;")
	route := &semantics.Route{Points: []semantics.Waypoint{{X: 1, Y: 2}}}
	_, err := Apply(m, []Operation{SetRouteAt(line, "", route)})
	if e := editError(t, err); e.Failure != FailureUnknownTarget {
		t.Fatalf("a declaration span names the edited document's declarations unless told otherwise: got %v", err)
	}
	routed := strings.Replace(engineParts, "        connection connect rotor to stator;\n",
		"        connection connect rotor to stator {\n            @DiagramLayout::Route { points = (1, 2); }\n        }\n", 1)
	res = applyOne(t, m, SetRouteAt(line, "", route).DeclaredIn("parts.sysml"))
	if got := otherContent(t, res, "parts.sysml"); got != routed || string(res.Content) != engineViews {
		t.Fatalf("route of another document's unnamed connection:\n--- want\n%s\n--- got\n%s", routed, got)
	}
	_, err = Apply(m, []Operation{SetRouteAt(line, "", route).DeclaredIn("gone.sysml")})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || !strings.Contains(e.Message, "gone.sysml") {
		t.Fatalf("a declaration in no document: got %v", err)
	}
	_, err = Apply(m, []Operation{SetRouteAt(source.Span{Offset: 3, Len: 4}, "", route).DeclaredIn("parts.sysml")})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || !strings.Contains(e.Message, "1:4 of parts.sysml") {
		t.Fatalf("a span declaring nothing in another document: got %v", err)
	}
}

// A named target stated to be declared in a document is placed only when that
// document declares it: a namesake declared elsewhere is refused, so that a
// declaration replaced since it was rendered is not placed in its stead.
func TestSetLayoutByNameDeclaredInADocumentRefusesANamesakeElsewhere(t *testing.T) {
	m := loadEditableWorkspace(t, "views.sysml", engineViews, map[string]string{"parts.sysml": engineParts})
	requireClean(t, m)
	res := applyOne(t, m, SetLayout("Machinery::Engine::rotor", "", at(5, 6)).DeclaredIn("parts.sysml"))
	if got := otherContent(t, res, "parts.sysml"); !strings.Contains(got, "part rotor {\n            @DiagramLayout::Layout { x = 5; y = 6; }") {
		t.Fatalf("rotor, declared in parts.sysml as stated, was not placed there:\n%s", got)
	}

	_, err := Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "", at(5, 6)).DeclaredIn("views.sysml")})
	e := editError(t, err)
	if e.Failure != FailureUnknownTarget || !strings.Contains(e.Message, "declared in parts.sysml, not in views.sysml") {
		t.Fatalf("rotor stated to be declared in views.sysml: got %v", err)
	}
	_, err = Apply(m, []Operation{SetLayout("EngineViews::engineView", "", at(5, 6)).DeclaredIn("parts.sysml")})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || !strings.Contains(e.Message, "declared in views.sysml, not in parts.sysml") {
		t.Fatalf("the edited document's view stated to be declared in parts.sysml: got %v", err)
	}
}

// A name two documents declare is ambiguous on its own, and is the one
// declaration a stated document holds when the operation states one; stating a
// document declaring neither names them as declared where they are.
func TestSetLayoutByNameDeclaredInADocumentPicksAmongNamesakes(t *testing.T) {
	m := loadEditableWorkspace(t, "views.sysml", engineViews,
		map[string]string{"parts.sysml": engineParts, "spare.sysml": engineParts})
	_, err := Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "", at(5, 6))})
	if e := editError(t, err); e.Failure != FailureAmbiguousTarget {
		t.Fatalf("rotor, declared by two documents and stated in none: got %v", err)
	}
	for _, doc := range []string{"parts.sysml", "spare.sysml"} {
		res := applyOne(t, m, SetLayout("Machinery::Engine::rotor", "", at(5, 6)).DeclaredIn(doc))
		if got := otherContent(t, res, doc); !strings.Contains(got, "part rotor {\n            @DiagramLayout::Layout { x = 5; y = 6; }") {
			t.Fatalf("rotor stated to be declared in %s was not placed there:\n%s", doc, got)
		}
		if len(res.Others) != 1 || string(res.Content) != engineViews {
			t.Fatalf("placing rotor of %s changed other documents: %v", doc, otherNames(res))
		}
	}
	_, err = Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "", at(5, 6)).DeclaredIn("views.sysml")})
	e := editError(t, err)
	if e.Failure != FailureUnknownTarget || !strings.Contains(e.Message, "parts.sysml") ||
		!strings.Contains(e.Message, "spare.sysml") || !strings.Contains(e.Message, "not in views.sysml") {
		t.Fatalf("rotor stated to be declared in views.sysml, which declares none: got %v", err)
	}
}

// A declaration span in another document follows the bytes earlier operations
// of the same request write before it there, and is refused once one of them
// rewrote the declaration itself.
func TestSetLayoutByDeclarationInAnotherDocumentFollowsEarlierOperations(t *testing.T) {
	m := loadEditableWorkspace(t, "views.sysml", engineViews, map[string]string{"parts.sysml": engineParts})
	requireClean(t, m)
	line := declaredAt(t, engineParts, "connection connect rotor to stator;")
	route := &semantics.Route{Points: []semantics.Waypoint{{X: 1, Y: 2}}}
	res, err := Apply(m, []Operation{
		SetLayout("Machinery::Engine::rotor", "", at(5, 6)),
		SetRouteAt(line, "", route).DeclaredIn("parts.sysml"),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := strings.Replace(engineParts, "        part rotor;\n",
		"        part rotor {\n            @DiagramLayout::Layout { x = 5; y = 6; }\n        }\n", 1)
	want = strings.Replace(want, "        connection connect rotor to stator;\n",
		"        connection connect rotor to stator {\n            @DiagramLayout::Route { points = (1, 2); }\n        }\n", 1)
	if got := otherContent(t, res, "parts.sysml"); got != want {
		t.Fatalf("parts.sysml:\n--- want\n%s\n--- got\n%s", want, got)
	}

	const links = "package Rotors {\n    part def Rotor;\n}\n"
	linked := strings.Replace(engineParts, "part rotor;", "part rotor : Rotors::Rotor;", 1)
	m = loadEditableWorkspace(t, "links.sysml", links, map[string]string{"parts.sysml": linked})
	requireClean(t, m)
	line = declaredAt(t, linked, "part rotor : Rotors::Rotor;")
	_, err = Apply(m, []Operation{
		Delete("Rotors::Rotor", true),
		SetRouteAt(line, "", route).DeclaredIn("parts.sysml"),
	})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || e.OperationIndex != 1 || !strings.Contains(e.Message, "of parts.sysml") {
		t.Fatalf("a declaration the cascade removed: got %v", err)
	}
}

// A Canvas sizes a view in the view's document.
func TestSetCanvasOfViewInAnotherDocument(t *testing.T) {
	m := loadEditableWorkspace(t, "parts.sysml", engineParts, map[string]string{"views.sysml": engineViews})
	requireClean(t, m)
	canvas := &semantics.Canvas{Unit: "px", Width: 1200, Height: 800, HasSize: true}
	res := applyOne(t, m, SetCanvas("EngineViews::engineView", canvas))
	if string(res.Content) != engineParts {
		t.Fatalf("parts.sysml changed:\n%s", res.Content)
	}
	sized := strings.Replace(engineViews,
		"        render asInterconnectionDiagram;\n    }\n",
		"        render asInterconnectionDiagram;\n        @DiagramLayout::Canvas { unit = \"px\"; width = 1200; height = 800; }\n    }\n", 1)
	if got := otherContent(t, res, "views.sysml"); got != sized {
		t.Fatalf("views.sysml:\n--- want\n%s\n--- got\n%s", sized, got)
	}
	m = loadEditableWorkspace(t, "parts.sysml", engineParts, map[string]string{"views.sysml": sized})
	res = applyOne(t, m, SetCanvas("EngineViews::engineView", nil))
	if got := otherContent(t, res, "views.sysml"); got != engineViews {
		t.Fatalf("clearing did not restore views.sysml:\n%s", got)
	}
}

// A view the edited document declares is the one its rendering shows, so an
// operation naming it applies there whatever namesakes other documents declare;
// from a document declaring no such view, the name is ambiguous between them.
func TestSetLayoutInViewPrefersTheEditedDocumentsNamesake(t *testing.T) {
	m := loadEditableWorkspace(t, "views.sysml", engineViews,
		map[string]string{"parts.sysml": engineParts, "copy.sysml": engineViews})
	res := applyOne(t, m, SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6)))
	if !strings.Contains(string(res.Content), rotorInView) || len(res.Others) != 0 {
		t.Fatalf("rotor not placed in the edited document's view alone:\n%s\nothers: %v", res.Content, otherNames(res))
	}
	canvas := &semantics.Canvas{Unit: "px", Width: 1200, Height: 800, HasSize: true}
	res = applyOne(t, m, SetCanvas("EngineViews::engineView", canvas))
	if !strings.Contains(string(res.Content), `@DiagramLayout::Canvas { unit = "px"; width = 1200; height = 800; }`) || len(res.Others) != 0 {
		t.Fatalf("the edited document's view not sized alone:\n%s\nothers: %v", res.Content, otherNames(res))
	}
	res = applyOne(t, m, SetCanvas("EngineViews::engineView", canvas).DeclaredIn("copy.sysml"))
	if got := otherContent(t, res, "copy.sysml"); !strings.Contains(got, "@DiagramLayout::Canvas") || string(res.Content) != engineViews {
		t.Fatalf("the view stated to be declared in copy.sysml not sized there alone:\n%s", got)
	}

	m = loadEditableWorkspace(t, "parts.sysml", engineParts,
		map[string]string{"views.sysml": engineViews, "copy.sysml": engineViews})
	_, err := Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6))})
	if e := editError(t, err); e.Failure != FailureAmbiguousTarget {
		t.Fatalf("a view two other documents declare: got %v", err)
	}
	_, err = Apply(m, []Operation{SetCanvas("EngineViews::engineView", canvas)})
	if e := editError(t, err); e.Failure != FailureAmbiguousTarget {
		t.Fatalf("a Canvas of a view two other documents declare: got %v", err)
	}
}

// A namesake that is no view neither stands in for the one view so named nor
// counts among them: the workspace's sole view is taken, from any document, and
// a name naming no view at all is refused as such.
func TestSetLayoutInViewIgnoresNamesakesThatAreNoView(t *testing.T) {
	partsWithNamesake := engineParts + "package EngineViews {\n    part def engineView;\n}\n"
	m := loadEditableWorkspace(t, "parts.sysml", partsWithNamesake, map[string]string{"views.sysml": engineViews})
	res := applyOne(t, m, SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6)))
	if got := otherContent(t, res, "views.sysml"); !strings.Contains(got, rotorInView) || string(res.Content) != partsWithNamesake {
		t.Fatalf("rotor not placed in the one view so named, in views.sysml alone:\n%s\nedited: %s", got, res.Content)
	}
	canvas := &semantics.Canvas{Unit: "px", Width: 1200, Height: 800, HasSize: true}
	res = applyOne(t, m, SetCanvas("EngineViews::engineView", canvas))
	if got := otherContent(t, res, "views.sysml"); !strings.Contains(got, "@DiagramLayout::Canvas") || string(res.Content) != partsWithNamesake {
		t.Fatalf("the one view so named not sized in views.sysml alone:\n%s\nedited: %s", got, res.Content)
	}

	m = loadEditableWorkspace(t, "parts.sysml", partsWithNamesake, nil)
	_, err := Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6))})
	if e := editError(t, err); e.Failure != FailureNotAView || !strings.Contains(e.Message, "EngineViews::engineView is no view") {
		t.Fatalf("a view name naming a part def alone: got %v", err)
	}
	_, err = Apply(m, []Operation{SetCanvas("EngineViews::engineView", canvas)})
	if e := editError(t, err); e.Failure != FailureNotAView || !strings.Contains(e.Message, "Canvas") {
		t.Fatalf("a Canvas of a part def: got %v", err)
	}
}

// One request may place elements in the edited document and in others; each
// document is rewritten once and the edited one comes first.
func TestSetLayoutReachesSeveralDocumentsInOneRequest(t *testing.T) {
	const own = "package Local {\n    part def Housing {\n        part shell;\n    }\n}\n"
	m := loadEditableWorkspace(t, "local.sysml", own, map[string]string{"parts.sysml": engineParts, "views.sysml": engineViews})
	requireClean(t, m)
	res, err := Apply(m, []Operation{
		SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6)),
		SetLayout("Local::Housing::shell", "", at(1, 2)),
		SetLayout("Machinery::Engine::stator", "", at(3, 4)),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(res.Content), "part shell {\n            @DiagramLayout::Layout { x = 1; y = 2; }\n        }\n") {
		t.Fatalf("local.sysml:\n%s", res.Content)
	}
	if got := otherNames(res); strings.Join(got, ",") != "parts.sysml,views.sysml" {
		t.Fatalf("others = %v, want parts.sysml then views.sysml", got)
	}
	if got := otherContent(t, res, "parts.sysml"); !strings.Contains(got, "part stator {\n            @DiagramLayout::Layout { x = 3; y = 4; }\n        }\n") {
		t.Fatalf("parts.sysml:\n%s", got)
	}
	if got := otherContent(t, res, "views.sysml"); !strings.Contains(got, rotorInView) {
		t.Fatalf("views.sysml:\n%s", got)
	}
}

// A document the model hands out no source for, and a bundled library file, are
// never written: the operation refuses, naming the document.
func TestSetLayoutRefusesDocumentsItMayNotRewrite(t *testing.T) {
	m := loadEditableWorkspace(t, "parts.sysml", engineParts, nil)
	m.Index.AddDocument("views.sysml", parseOnly("views.sysml", engineViews))
	m.Index.ExpandWildcardImports()
	for _, op := range []Operation{
		SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6)),
		SetCanvas("EngineViews::engineView", &semantics.Canvas{Width: 1, Height: 2, HasSize: true}),
	} {
		_, err := Apply(m, []Operation{op})
		e := editError(t, err)
		if e.Failure != FailureReferencedElsewhere || e.OperationIndex != 0 || !strings.Contains(e.Message, "views.sysml") {
			t.Fatalf("%s into an unheld document: got %v", op.Annotation, err)
		}
	}

	m = loadEditableWorkspace(t, "parts.sysml", engineParts, nil)
	_, err := Apply(m, []Operation{SetLayout("Parts::Part", "", at(5, 6))})
	e := editError(t, err)
	if e.Failure != FailureReferencedElsewhere || !strings.Contains(e.Message, "the bundled library file Systems Library/Parts.sysml") {
		t.Fatalf("inline layout of a library declaration: got %v", err)
	}
}

// Validation judges every rewritten document: an annotation whose type name the
// view's document spells as something else refuses the request as a whole, and
// a later operation sees what an earlier one placed in another document.
func TestSetLayoutIntoAnotherDocumentRefusesWhenTheResultIsInvalidThere(t *testing.T) {
	shadowed := strings.Replace(engineViews, "    view engineView {", "    part def DiagramLayout;\n    view engineView {", 1)
	m := loadEditableWorkspace(t, "parts.sysml", engineParts, map[string]string{"views.sysml": shadowed})
	requireClean(t, m)
	_, err := Apply(m, []Operation{
		SetLayout("Machinery::Engine::stator", "", at(1, 2)),
		SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6)),
	})
	e := editError(t, err)
	if e.Failure != FailureResultInvalid || !strings.Contains(e.Message, "in views.sysml") || e.Diagnosed == nil || e.Diagnosed.Name() != "views.sysml" {
		t.Fatalf("placing into a document that spells DiagramLayout as its own: got %v", err)
	}
	if string(m.Source.Bytes()) != engineParts {
		t.Fatal("the model's own source was modified")
	}

	m = loadEditableWorkspace(t, "parts.sysml", engineParts, map[string]string{"views.sysml": engineViews})
	res, err := Apply(m, []Operation{
		SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6)),
		Delete("Machinery::Engine::rotor", true),
	})
	if err != nil {
		t.Fatalf("placing then deleting the element: %v", err)
	}
	if got := otherContent(t, res, "views.sysml"); got != engineViews {
		t.Fatalf("the cascade did not remove the annotation it placed:\n%s", got)
	}
}

func TestSetLayoutRefusals(t *testing.T) {
	cases := []struct {
		name    string
		op      Operation
		failure Failure
		message string
	}{
		{
			name:    "unknown element",
			op:      SetLayout("Plant::Loop::valve", "PlantViews::loopView", at(1, 2)),
			failure: FailureUnknownTarget,
			message: "no element named",
		},
		{
			name:    "unknown view",
			op:      SetLayout("Plant::Loop::pump", "PlantViews::noView", at(1, 2)),
			failure: FailureUnknownTarget,
			message: "no element named",
		},
		{
			name:    "view that is no view",
			op:      SetLayout("Plant::Loop::pump", "Plant::Loop", at(1, 2)),
			failure: FailureNotAView,
			message: "is no view",
		},
		{
			name:    "view not exposing the element",
			op:      SetLayout("Plant::Motor::off", "PlantViews::loopView", at(1, 2)),
			failure: FailureNotExposed,
			message: "does not expose",
		},
		{
			name:    "layout of an edge",
			op:      SetLayout("Plant::Loop::supply", "PlantViews::loopView", at(1, 2)),
			failure: FailureNotDrawn,
			message: "as a node",
		},
		{
			name:    "route of a node",
			op:      SetRoute("Plant::Loop::pump", "", &semantics.Route{Points: []semantics.Waypoint{{X: 1, Y: 2}}}),
			failure: FailureNotDrawn,
			message: "as an edge",
		},
		{
			name:    "canvas of a part",
			op:      SetCanvas("Plant::Loop", &semantics.Canvas{Unit: "px"}),
			failure: FailureNotAView,
			message: "Canvas",
		},
		{
			name:    "empty route",
			op:      SetRoute("Plant::Loop::supply", "", &semantics.Route{}),
			failure: FailureInvalidValue,
			message: "at least one waypoint",
		},
		{
			name:    "clearing what is not there",
			op:      SetLayout("Plant::Loop::pump", "PlantViews::loopView", nil),
			failure: FailureNotAnnotated,
			message: "no Layout in PlantViews::loopView",
		},
		{
			name:    "unknown annotation",
			op:      Operation{Kind: OpSetLayout, Target: "Plant::Loop::pump", Annotation: "DiagramLayout::Color"},
			failure: FailureInvalidValue,
			message: "no DiagramLayout annotation",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := loadContent(t, "plant.sysml", plantModel)
			_, err := Apply(m, []Operation{tc.op})
			e := editError(t, err)
			if e.Failure != tc.failure {
				t.Fatalf("failure %s, want %s: %s", e.Failure, tc.failure, e.Message)
			}
			if !strings.Contains(e.Message, tc.message) {
				t.Fatalf("message %q lacks %q", e.Message, tc.message)
			}
		})
	}
}
