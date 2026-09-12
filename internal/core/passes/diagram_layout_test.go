package passes

import (
	"strings"
	"testing"
)

// layoutModel wraps decls in a package importing the DiagramLayout and Views
// libraries, so decls start on line 4.
func layoutModel(decls string) string {
	return "package P {\n\tprivate import DiagramLayout::*;\n\tprivate import Views::*;\n" + decls + "}\n"
}

// layoutDiags analyses a model and returns the DiagramLayout diagnostics only.
func layoutDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, d := range w8dDiags(t, src) {
		if strings.HasPrefix(d.Code, "diagram-layout-") {
			out = append(out, d)
		}
	}
	return out
}

// wantLayoutDiag checks one diagnostic's severity, code, source, message and line.
func wantLayoutDiag(t *testing.T, src string, d Diagnostic, severity Severity, code string, line int, message ...string) {
	t.Helper()
	if d.Severity != severity || d.Code != code || d.Source != "constraint" {
		t.Fatalf("got severity %v, code %q, source %q; want %v, %q, constraint: %s",
			d.Severity, d.Code, d.Source, severity, code, d.Message)
	}
	if got := w8dLine(src, d.Span); got != line {
		t.Fatalf("diagnostic on line %d, want %d: %s", got, line, d.Message)
	}
	for _, want := range message {
		if !strings.Contains(d.Message, want) {
			t.Fatalf("message %q lacks %q", d.Message, want)
		}
	}
}

func TestDiagramLayoutWellFormedAnnotationsPass(t *testing.T) {
	src := layoutModel(`	part def Pump;
	part def Tank;
	part def Loop {
		part pump : Pump { @Layout { x = 10; y = 20; } }
		part tank : Tank;
		connection supply connect pump to tank { @Route { points = (100, 20, 150, 60); } }
	}
	state def Machine {
		state off { @Layout { x = 0; y = 0; width = 80; height = 40; collapsed = true; } }
		state on;
		transition off_on first off then on { @Route { points = (40, 40, 80, 80); } }
	}
	view def Diagram;
	view wiring : Diagram {
		render asInterconnectionDiagram;
		expose Loop::*;
		@Canvas { unit = "px"; width = 800; height = 600; }
		metadata Layout about Loop::tank { x = 300; y = 20; }
		metadata Route about Loop::supply { points = (110, 30, 300, 30); }
	}
`)
	for _, d := range w8dDiags(t, src) {
		if d.Severity == SeverityError || strings.HasPrefix(d.Code, "diagram-layout-") {
			t.Fatalf("well-formed annotations drew %v", d)
		}
	}
}

func TestDiagramLayoutOddRoutePointsIsAnError(t *testing.T) {
	src := layoutModel(`	part def Pump;
	part def Tank;
	part def Loop {
		part pump : Pump;
		part tank : Tank;
		connection supply connect pump to tank {
			@Route { points = (100, 20, 150); }
		}
	}
`)
	diags := layoutDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityError, "diagram-layout-value", 10,
		"connection P::Loop::supply: Route binds 3 values", "x, y pairs")
}

// A binding the model cannot evaluate is the metadata annotation check's report at
// the type tier; one it evaluates to something other than the geometry's kind is
// reported here.
func TestDiagramLayoutNonConstantValueIsAnError(t *testing.T) {
	unevaluable := layoutModel(`	part def Pump {
		attribute offset : ScalarValues::Real;
		@Layout { x = offset; y = 20; }
	}
`)
	if got := w8dLines(t, unevaluable, "metadata-value-not-evaluable"); len(got) != 1 || got[0] != 6 {
		t.Fatalf("unevaluable binding reported on lines %v, want [6]", got)
	}
	src := layoutModel(`	part def Pump {
		@Layout { x = 3; y = (1, 2); }
	}
	part def Tank {
		@Layout { x = null; y = 20; collapsed = 1; }
	}
	part def Loop {
		part pump : Pump;
		part tank : Tank;
		connection supply connect pump to tank {
			@Route { points = (0, 0, "a", 1); }
		}
	}
	view def Diagram;
	view diagram : Diagram {
		@Canvas { unit = "px"; width = 640; }
	}
`)
	diags := layoutDiags(t, src)
	if len(diags) != 5 {
		t.Fatalf("got %d diagnostics, want 5: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityError, "diagram-layout-value", 5,
		"part def P::Pump: y of Layout is not a constant number")
	wantLayoutDiag(t, src, diags[1], SeverityError, "diagram-layout-value", 8,
		"part def P::Tank: x of Layout is not a constant number")
	wantLayoutDiag(t, src, diags[2], SeverityError, "diagram-layout-value", 8,
		"part def P::Tank: collapsed of Layout is not a constant boolean")
	wantLayoutDiag(t, src, diags[3], SeverityError, "diagram-layout-value", 14,
		"connection P::Loop::supply: points of Route is not a constant number")
	wantLayoutDiag(t, src, diags[4], SeverityError, "diagram-layout-value", 19,
		"view P::diagram: Canvas binds one of width and height; an extent needs both")
}

func TestDiagramLayoutCanvasOutsideAViewIsAnError(t *testing.T) {
	src := layoutModel(`	part def Pump {
		@Canvas { unit = "px"; }
	}
	view def Diagram;
	view diagram : Diagram {
		@Canvas { width = 400; height = 300; }
	}
`)
	diags := layoutDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityError, "diagram-layout-canvas", 5,
		"Canvas annotates part def P::Pump", "no view")
}

// A Canvas about a view sizes it only from inside its body: one stated in the
// enclosing package, or in the body of another view, is an error.
func TestDiagramLayoutCanvasAboutAViewFromOutsideItsBodyIsAnError(t *testing.T) {
	src := layoutModel(`	view def Diagram;
	view diagram : Diagram {
		metadata Canvas about diagram { width = 400; height = 300; }
	}
	metadata Canvas about diagram { width = 800; height = 600; }
	view other : Diagram {
		metadata Canvas about diagram { width = 1200; height = 900; }
	}
`)
	diags := layoutDiags(t, src)
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityError, "diagram-layout-canvas", 8,
		"Canvas about view P::diagram is stated outside its body")
	wantLayoutDiag(t, src, diags[1], SeverityError, "diagram-layout-canvas", 10,
		"Canvas about view P::diagram is stated outside its body")
}

// An annotation applying in every view is judged against every rendering kind:
// a tree draws a package, so a Layout on one is placed; nothing draws a
// dependency, and nothing draws a part def as an edge.
func TestDiagramLayoutUnplaceableInlineAnnotationsWarn(t *testing.T) {
	src := layoutModel(`	package Parts {
		@Layout { x = 0; y = 0; }
	}
	part def Pump {
		@Route { points = (0, 0, 10, 10); }
	}
	part def Tank {
		@Layout { x = 1; y = 1; }
	}
	dependency feeds from Pump to Tank;
	metadata Layout about feeds { x = 2; y = 2; }
`)
	diags := layoutDiags(t, src)
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityWarning, "diagram-layout-unplaced", 8,
		"Route steers part def P::Pump", "no rendering draws as an edge")
	wantLayoutDiag(t, src, diags[1], SeverityWarning, "diagram-layout-unplaced", 14,
		"Layout positions dependency P::feeds", "no rendering draws as a node")
}

func TestDiagramLayoutViewLocalAnnotationsJudgedByTheViewsRendering(t *testing.T) {
	src := layoutModel(`	part def Pump;
	part def Tank;
	part def Loop {
		part pump : Pump;
		part tank : Tank;
		connection supply connect pump to tank;
	}
	view def Diagram;
	view wiring : Diagram {
		render asInterconnectionDiagram;
		expose Loop::*;
		metadata Layout about Loop::supply { x = 0; y = 0; }
		metadata Route about Loop::pump { points = (0, 0, 1, 1); }
		metadata Layout about Loop::pump { x = 5; y = 5; }
	}
	view outline : Diagram {
		render asTreeDiagram;
		expose Loop::*;
		metadata Layout about Loop::supply { x = 0; y = 0; }
	}
`)
	diags := layoutDiags(t, src)
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityWarning, "diagram-layout-unplaced", 15,
		"Layout positions connection P::Loop::supply", "interconnection rendering of view P::wiring does not draw as a node")
	wantLayoutDiag(t, src, diags[1], SeverityWarning, "diagram-layout-unplaced", 16,
		"Route steers part P::Loop::pump", "interconnection rendering of view P::wiring does not draw as an edge")
}

// A view-local annotation is judged by what the view's rendering draws, not by
// what its kind could draw: a part and a connection nested in an exposed part
// are drawn; a part the view does not expose, and a connection declared beside
// the exposed parts rather than in or among them, are not.
func TestDiagramLayoutViewLocalAnnotationsJudgedByWhatTheViewExposes(t *testing.T) {
	src := layoutModel(`	part def Loop {
		part pump {
			part impeller;
			connection shaft connect impeller to impeller;
		}
		part tank;
		connection supply connect pump to tank;
	}
	part def Spare { part valve; }
	view def Diagram;
	view wiring : Diagram {
		render asInterconnectionDiagram;
		expose Loop::pump;
		expose Loop::tank;
		metadata Layout about Loop::pump::impeller { x = 1; y = 1; }
		metadata Route about Loop::pump::shaft { points = (0, 0, 1, 1); }
		metadata Route about Loop::supply { points = (0, 0, 1, 1); }
		metadata Layout about Spare::valve { x = 2; y = 2; }
	}
`)
	diags := layoutDiags(t, src)
	if len(diags) != 2 {
		t.Fatalf("got %d diagnostics, want 2: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityWarning, "diagram-layout-unplaced", 20,
		"Route steers connection P::Loop::supply", "interconnection rendering of view P::wiring does not draw as an edge")
	wantLayoutDiag(t, src, diags[1], SeverityWarning, "diagram-layout-unplaced", 21,
		"Layout positions part P::Spare::valve", "interconnection rendering of view P::wiring does not draw as a node")
}

// The states and transitions of an exposed state usage are drawn from its
// definition, so annotations about them are placed.
func TestDiagramLayoutViewLocalAnnotationsFollowInheritedBehavior(t *testing.T) {
	src := layoutModel(`	state def Machine {
		entry; then off;
		state off;
		state on;
		transition off_on first off then on;
	}
	state machine : Machine;
	view def Diagram;
	view lifecycle : Diagram {
		render asStateDiagram;
		expose machine;
		metadata Layout about Machine::off { x = 1; y = 1; }
		metadata Route about Machine::off_on { points = (0, 0, 1, 1); }
	}
`)
	if diags := layoutDiags(t, src); len(diags) != 0 {
		t.Fatalf("got %d diagnostics, want none: %v", len(diags), diags)
	}
}

func TestDiagramLayoutDuplicateViewLocalAnnotationWarnsOnTheSecond(t *testing.T) {
	src := layoutModel(`	part def Pump;
	part def Loop {
		part pump : Pump;
	}
	view def Diagram;
	view a : Diagram {
		render asInterconnectionDiagram;
		expose Loop::*;
		metadata Layout about Loop::pump { x = 1; y = 1; }
		metadata Layout about Loop::pump { x = 2; y = 2; }
	}
	view b : Diagram {
		render asInterconnectionDiagram;
		expose Loop::*;
		metadata Layout about Loop::pump { x = 3; y = 3; }
	}
`)
	diags := layoutDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	wantLayoutDiag(t, src, diags[0], SeverityWarning, "diagram-layout-duplicate", 13,
		"Layout about part P::Loop::pump is already stated in view P::a", "first stated applies")
}

func TestDiagramLayoutReportsOnlyTheStatingDocument(t *testing.T) {
	srcA := `package PA {
	private import DiagramLayout::*;
	part def Pump;
	part def Loop {
		part pump : Pump;
	}
}
`
	srcB := `package PB {
	private import DiagramLayout::*;
	private import Views::*;
	view def Diagram;
	view a : Diagram {
		render asInterconnectionDiagram;
		expose PA::Loop::*;
		metadata Route about PA::Loop::pump { points = (0, 0, 1); }
	}
}
`
	diagsA, diagsB := identityDiagsAcross(t, srcA, srcB)
	if n := len(only(diagsA, "diagram-layout-value")) + len(only(diagsA, "diagram-layout-unplaced")); n != 0 {
		t.Fatalf("document A states no annotation yet drew %d diagnostics: %v", n, diagsA)
	}
	if got := len(only(diagsB, "diagram-layout-value")); got != 1 {
		t.Fatalf("got %d value diagnostics in document B, want 1: %v", got, diagsB)
	}
	if got := len(only(diagsB, "diagram-layout-unplaced")); got != 1 {
		t.Fatalf("got %d placement diagnostics in document B, want 1: %v", got, diagsB)
	}
}
