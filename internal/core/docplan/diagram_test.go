package docplan

import (
	"strings"
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

	view textualView {
		expose imagingChain;
		render asTextualNotation;
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

func TestCompileDiagramWithDeclaredView(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines caption = "Imaging chain";
			ref redefines source = interconnectView;
		}
	`))
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
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
	reference := diagram.Diagram()
	if reference == nil {
		t.Fatal("diagram reference is nil")
	}
	declared, ok := reference.View()
	if !ok || declared == nil {
		t.Fatal("reference names no view")
	}
	if reference.Kind() != view.KindInterconnection {
		t.Fatalf("kind = %q", reference.Kind())
	}
	if reference.Direction() != "" {
		t.Fatalf("direction = %q", reference.Direction())
	}
	if !reference.Origin().Located() || reference.Origin().Doc != fixtureDoc {
		t.Fatalf("origin = %+v", reference.Origin())
	}
}

func TestCompileDiagramWithElementAndKind(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part states : Diagram {
			attribute redefines kind = "state";
			attribute redefines direction = "LR";
			ref redefines source = ObservatoryStates;
		}
	`))
	plan := fixture.mustCompile(t, "Report")
	reference := plan.Content()[0].Diagram()
	target, ok := reference.Target()
	if !ok || target == nil {
		t.Fatal("reference names no target element")
	}
	if reference.Kind() != view.KindState {
		t.Fatalf("kind = %q", reference.Kind())
	}
	if reference.Direction() != view.DirectionLeftRight {
		t.Fatalf("direction = %q", reference.Direction())
	}
}

func TestCompileDiagramWithoutSource(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines kind = "tree";
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingViewSource || planning.Content != "Observatory::Report::imaging" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileDiagramWithUnknownSource(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines kind = "tree";
			ref redefines source = NoSuchElement;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorUnknownViewSource || planning.Actual != "NoSuchElement" {
		t.Fatalf("error = %+v", planning)
	}
	if !planning.Origin.Located() || planning.Origin.Doc != fixtureDoc {
		t.Fatalf("origin = %+v", planning.Origin)
	}
}

func TestCompileDiagramRejectsKindOnAView(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines kind = "tree";
			ref redefines source = interconnectView;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorConflictingKind || planning.Actual != "tree" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileDiagramRequiresKindForAnElement(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			ref redefines source = imagingChain;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorMissingDiagramKind {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileDiagramRejectsUnsupportedKind(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines kind = "geometry";
			ref redefines source = imagingChain;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorUnsupportedKind || planning.Actual != "geometry" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileDiagramRejectsAViewOfUnsupportedKind(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			ref redefines source = textualView;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorUnsupportedKind || planning.Err == nil {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileDiagramRejectsInvalidDirection(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines direction = "sideways";
			ref redefines source = interconnectView;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidDirection || planning.Actual != "sideways" {
		t.Fatalf("error = %+v", planning)
	}
}

func TestCompileDiagramWithForm(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines form = "dot";
			ref redefines source = interconnectView;
		}
		part states : Diagram {
			attribute redefines kind = "state";
			attribute redefines form = "mermaid";
			ref redefines source = ObservatoryStates;
		}
		part plain : Diagram {
			ref redefines source = interconnectView;
		}
	`))
	plan := fixture.mustCompile(t, "Report")
	content := plan.Content()
	if len(content) != 3 {
		t.Fatalf("content = %d nodes, want 3", len(content))
	}
	for i, want := range []view.Form{view.FormDot, view.FormMermaid, ""} {
		if got := content[i].Diagram().Form(); got != want {
			t.Errorf("%s form = %q, want %q", content[i].Name(), got, want)
		}
	}
}

func TestCompileDiagramRejectsInvalidForm(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines form = "svg";
			ref redefines source = interconnectView;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidForm || planning.Actual != "svg" {
		t.Fatalf("error = %+v", planning)
	}
	if !strings.Contains(planning.Error(), `"mermaid" or "dot"`) {
		t.Fatalf("message = %q", planning.Error())
	}
}

func TestCompileDiagramRejectsFormTheKindIsNotWrittenAs(t *testing.T) {
	for _, tc := range []struct{ kind, form string }{{"sequence", "dot"}, {"table", "mermaid"}} {
		fixture := loadPlanningFixture(t, diagramDocument(`
			part imaging : Diagram {
				attribute redefines kind = "`+tc.kind+`";
				attribute redefines form = "`+tc.form+`";
				ref redefines source = imagingChain;
			}
		`))
		_, err := fixture.compile(t, "Report")
		planning := planningError(t, err)
		if planning.Kind != ErrorUnsupportedForm || planning.Expected != tc.kind || planning.Actual != tc.form {
			t.Fatalf("%s as %s: error = %+v", tc.kind, tc.form, planning)
		}
	}
}

func TestCompileDiagramRejectsDirectionOnASequence(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines kind = "sequence";
			attribute redefines direction = "LR";
			ref redefines source = imagingChain;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorUnsupportedDirection || planning.Expected != "sequence" {
		t.Fatalf("error = %+v", planning)
	}
}
