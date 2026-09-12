package docir

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

const diagramModel = `
	private import Views::*;

	part def Camera;
	part def Recorder;
	connection def DataLink;

	part imagingChain {
		part camera : Camera;
		part recorder : Recorder;
		connection link : DataLink connect camera to recorder;
	}

	state def ObservatoryStates {
		entry; then idle;
		state idle;
		state observing;
		transition first idle then observing;
	}

	view interconnectView {
		expose imagingChain;
		render asInterconnectionDiagram;
	}
`

func diagramDocument(body string) string {
	return diagramModel + `
	part def Report :> Document {
		attribute redefines title = "Report";
` + body + `
	}
`
}

func TestEvaluateDiagramFromDeclaredView(t *testing.T) {
	fixture := loadEvaluationFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines caption = "Imaging chain";
			ref redefines source = interconnectView;
		}
	`))
	document := fixture.mustEvaluate(t, "Report")
	content := document.Content()
	if len(content) != 1 {
		t.Fatalf("content = %d nodes, want 1", len(content))
	}
	diagram := content[0]
	if diagram.Kind() != ContentDiagram || diagram.Name() != "imaging" {
		t.Fatalf("diagram = %s %q", diagram.Kind(), diagram.Name())
	}
	if diagram.Caption() != "Imaging chain" {
		t.Fatalf("caption = %q", diagram.Caption())
	}
	if diagram.Direction() != "" {
		t.Fatalf("direction = %q", diagram.Direction())
	}
	rendering := diagram.Rendering()
	if rendering == nil {
		t.Fatal("rendering is nil")
	}
	if rendering.Kind != view.KindInterconnection {
		t.Fatalf("rendering kind = %q", rendering.Kind)
	}
	if len(rendering.Roots) == 0 {
		t.Fatal("rendering has no roots")
	}
	if !diagram.Origin().Located() || diagram.Origin().Doc != fixtureDoc {
		t.Fatalf("origin = %+v", diagram.Origin())
	}
}

func TestEvaluateDiagramFromElementAndKind(t *testing.T) {
	fixture := loadEvaluationFixture(t, diagramDocument(`
		part states : Diagram {
			attribute redefines kind = "state";
			attribute redefines direction = "LR";
			ref redefines source = ObservatoryStates;
		}
	`))
	document := fixture.mustEvaluate(t, "Report")
	diagram := document.Content()[0]
	if diagram.Direction() != view.DirectionLeftRight {
		t.Fatalf("direction = %q", diagram.Direction())
	}
	rendering := diagram.Rendering()
	if rendering == nil || rendering.Kind != view.KindState {
		t.Fatalf("rendering = %+v", rendering)
	}
}

func TestDiagramRenderingIsDefensivelyCopied(t *testing.T) {
	fixture := loadEvaluationFixture(t, diagramDocument(`
		part imaging : Diagram {
			ref redefines source = interconnectView;
		}
	`))
	document := fixture.mustEvaluate(t, "Report")
	diagram := document.Content()[0]
	first := diagram.Rendering()
	first.Kind = view.KindTable
	first.Roots = nil
	first.Notices = append(first.Notices, "mutated")
	second := diagram.Rendering()
	if second.Kind != view.KindInterconnection {
		t.Fatalf("kind mutated to %q", second.Kind)
	}
	if len(second.Roots) == 0 {
		t.Fatal("roots mutated")
	}
	if len(second.Notices) != 0 {
		t.Fatalf("notices mutated: %v", second.Notices)
	}
}
