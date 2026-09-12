package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// A model positioning what its views draw: one inline Layout, one stated in the
// view's body, a route, and a canvas.
const layoutModel = `package Plant {
    private import DiagramLayout::*;
    private import Views::*;
    part def Pump;
    part def Tank;
    part def Loop {
        part pump : Pump { @Layout { x = 10; y = 20; width = 100; height = 50; } }
        part tank : Tank;
        connection supply connect pump to tank { @Route { points = (110, 45, 300, 45); } }
    }
    view def Wiring;
    view wiring : Wiring {
        render asInterconnectionDiagram;
        expose Loop::*;
        @Canvas { unit = "px"; width = 800; height = 600; }
        metadata Layout about Loop::tank { x = 300; y = 20; }
    }
}
`

// The prompt reports a DiagramLayout finding as it reports any other: the line
// it was written on, its severity and its message.
func TestSubmittedLayoutFindingsAreReported(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package Plant {
    private import DiagramLayout::*;
    part def Pump {
        @Route { points = (0, 0, 10, 10); }
        @Canvas { width = 400; }
    }
}
`)
	if !hasCode(res.Diagnostics, "diagram-layout-unplaced") || !hasCode(res.Diagnostics, "diagram-layout-canvas") {
		t.Fatalf("want the placement and Canvas findings, got %v", codesOf(res.Diagnostics))
	}
	out := strings.Join(renderDiagnostics(res.Diagnostics, res.Source, res.diagLocation, false), "\n")
	wants(t, out,
		"4:9: warning: Route steers part def Plant::Pump, which no rendering draws as an edge",
		"5:9: error: Canvas annotates part def Plant::Pump, which is no view")
}

// A well-formed layout draws no finding, and %render keeps it visible in both
// the text and the Mermaid form.
func TestRenderKeepsTheLayoutVisible(t *testing.T) {
	s := NewSession()
	res := s.Submit(layoutModel)
	for _, d := range res.Diagnostics {
		if strings.HasPrefix(d.Code, "diagram-layout-") || d.Severity == passes.SeverityError {
			t.Fatalf("the laid-out model drew %v", res.Diagnostics)
		}
	}
	wants(t, run(t, s, "%render Plant::wiring"),
		"part Plant::Loop::pump (Pump) at (10, 20) size 100×50",
		"part Plant::Loop::tank (Tank) at (300, 20)")
	wants(t, run(t, s, "%render Plant::wiring mermaid"),
		"%% canvas: unit=px w=800 h=600",
		"x=10 y=20 w=100 h=50",
		"x=300 y=20",
		"110,45 300,45")
}
