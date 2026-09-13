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

func TestSetLayoutOfElementInAnotherDocumentFromAView(t *testing.T) {
	const parts = "package Machinery {\n    part def Engine {\n        part rotor;\n        part stator;\n        connection connect rotor to stator;\n    }\n}\n"
	const views = "package EngineViews {\n    private import Views::*;\n    private import StandardViewDefinitions::*;\n    view engineView {\n        expose Machinery::Engine;\n        render asInterconnectionDiagram;\n    }\n}\n"
	m := loadWorkspace(t, "views.sysml", views, map[string]string{"parts.sysml": parts})
	requireClean(t, m)
	res, err := Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "EngineViews::engineView", at(5, 6))})
	if err != nil {
		t.Fatalf("view-local layout of a sibling document's element: %v", err)
	}
	if !strings.Contains(string(res.Content), "metadata DiagramLayout::Layout about Machinery::Engine::rotor { x = 5; y = 6; }") {
		t.Fatalf("layout not stated in the view:\n%s", res.Content)
	}
	_, err = Apply(m, []Operation{SetLayout("Machinery::Engine::rotor", "", at(5, 6))})
	if e := editError(t, err); e.Failure != FailureUnknownTarget || !strings.Contains(e.Message, "parts.sysml") {
		t.Fatalf("inline layout of another document's element: got %v", err)
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
