package docplan

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
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
	if reference.Direction() != "" || reference.Palette() != "" {
		t.Fatalf("direction = %q, palette = %q", reference.Direction(), reference.Palette())
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

func TestCompileDiagramWithPalette(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines palette = "okabe-ito";
			ref redefines source = interconnectView;
		}
	`))
	plan := fixture.mustCompile(t, "Report")
	reference := plan.Content()[0].Diagram()
	if reference.Palette() != view.PaletteOkabeIto {
		t.Fatalf("palette = %q", reference.Palette())
	}
	if reference.Direction() != "" {
		t.Fatalf("direction = %q", reference.Direction())
	}
}

func TestCompileDiagramRejectsInvalidPalette(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines palette = "rainbow";
			ref redefines source = interconnectView;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorInvalidPalette || planning.Actual != "rainbow" {
		t.Fatalf("error = %+v", planning)
	}
	want := `document Observatory::Report diagram Observatory::Report::imaging palette must be one of okabe-ito, tol-bright, tol-muted, tol-light, brewer-set2, brewer-dark2, viridis, cividis, got "rainbow"`
	if planning.Error() != want {
		t.Fatalf("error = %q, want %q", planning.Error(), want)
	}
}

func TestCompileDiagramRejectsPaletteOnATable(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines kind = "table";
			attribute redefines palette = "viridis";
			ref redefines source = imagingChain;
		}
	`))
	_, err := fixture.compile(t, "Report")
	planning := planningError(t, err)
	if planning.Kind != ErrorUnsupportedPalette || planning.Expected != "table" || planning.Actual != "viridis" {
		t.Fatalf("error = %+v", planning)
	}
	want := `document Observatory::Report diagram Observatory::Report::imaging states palette "viridis", but a table rendering has no DOT or PlantUML form to fill`
	if planning.Error() != want {
		t.Fatalf("error = %q, want %q", planning.Error(), want)
	}
}

func TestCompileDiagramAcceptsPaletteOnASequence(t *testing.T) {
	fixture := loadPlanningFixture(t, diagramDocument(`
		part imaging : Diagram {
			attribute redefines kind = "sequence";
			attribute redefines palette = "viridis";
			ref redefines source = imagingChain;
		}
	`))
	plan := fixture.mustCompile(t, "Report")
	reference := plan.Content()[0].Diagram()
	if reference.Kind() != view.KindSequence || reference.Palette() != "viridis" {
		t.Fatalf("kind = %q, palette = %q", reference.Kind(), reference.Palette())
	}
}
