package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// documentModel declares a document over the query model's part tree, so
// -render-document decides both ways: Markdown written, and a failure that
// gates a build.
const documentModel = queryModel + `package Reports {
	private import DocumentQueries::*;
	private import Observatory::*;

	part def MassReport :> Document {
		attribute redefines title = "Telescope Mass Report";

		part intro : Paragraph {
			attribute redefines text = "Mass rollup for the telescope assembly.";
		}

		part breakdown : Section {
			attribute redefines title = "Heavy Subsystems";

			part masses : Table {
				attribute redefines caption = "Heavy subsystems by mass";
				calc rows : HeavySubsystems {
					in root = telescope;
				}
			}
		}
	}
}
`

// unboundModel declares a document whose table query lacks a required binding,
// which analysis reports and the render run then refuses.
const unboundModel = queryModel + `package Reports {
	private import DocumentQueries::*;
	private import Observatory::*;

	part def UnboundReport :> Document {
		attribute redefines title = "Unbound Report";

		part masses : Table {
			calc rows : HeavySubsystems;
		}
	}
}
`

// TestRenderDocumentFlag checks the scripted surface of document rendering:
// Markdown on stdout, an artifact written to -o, and a documented failure for
// a document that could not be rendered.
func TestRenderDocumentFlag(t *testing.T) {
	binary := buildCLI(t)

	got := check(t, binary, documentModel, "-render-document", "Reports::MassReport")
	wantReport(t, got, 0,
		"# Telescope Mass Report",
		"Mass rollup for the telescope assembly.",
		"## Heavy Subsystems",
		"<!-- caption -->\n*Heavy subsystems by mass*",
		"| name | mass |",
		"| --- | --- |",
		"| mount | 15 |")
	if strings.Contains(got.stderr, "# Telescope Mass Report") {
		t.Errorf("the artifact belongs on stdout, not stderr:\n%s", got.stderr)
	}

	// An unknown document, a non-document, and a document whose query lacks a
	// binding are all runs that could not be carried out.
	wantReport(t, check(t, binary, documentModel, "-render-document", "NoSuchDocument"),
		2, "NoSuchDocument")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Observatory::HeavySubsystems"),
		2, "not a document", "DocumentQueries::Document")
	wantReport(t, check(t, binary, unboundModel, "-render-document", "Reports::UnboundReport"),
		2, "does not bind required parameter root")
}

// TestRenderDocumentDiagramForm checks -diagram-form: Mermaid by default, DOT
// or PlantUML on request, tables either way, and refused where it has nothing
// to act on.
func TestRenderDocumentDiagramForm(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.sysml")
	goldens := map[string]string{
		"mermaid":  "telescope_report.golden.md",
		"dot":      "telescope_report.dot.golden.md",
		"plantuml": "telescope_report.plantuml.golden.md",
	}
	for form, name := range goldens {
		golden, err := os.ReadFile(filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-diagram-form", form)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("-diagram-form %s: %v", form, err)
		}
		if string(out) != string(golden) {
			t.Errorf("-diagram-form %s differs from %s:\n%s", form, name, out)
		}
	}
	if dot, err := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-diagram-form", "dot").Output(); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(dot), "```dot\n") || strings.Contains(string(dot), "```mermaid") || !strings.Contains(string(dot), "| name | mass |") {
		t.Errorf("dot rendering does not write DOT diagrams next to pipe tables:\n%s", dot)
	}
	if puml, err := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-diagram-form", "plantuml").Output(); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(puml), "```plantuml\n@startuml\n") || strings.Contains(string(puml), "```mermaid") || !strings.Contains(string(puml), "| name | mass |") {
		t.Errorf("plantuml rendering does not write PlantUML diagrams next to pipe tables:\n%s", puml)
	}

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-diagram-form", "svg"),
		2, `unknown diagram form "svg"`, "-diagram-form takes mermaid, dot, plantuml")
	wantReport(t, check(t, binary, documentModel, "-diagram-form", "dot"),
		2, "-diagram-form", "apply to -render-document and -render-documents")
	wantReport(t, check(t, binary, documentModel, "-render", "SomeView", "-diagram-form", "dot"),
		2, "-diagram-form", "apply to -render-document and -render-documents")
	wantReport(t, check(t, binary, documentModel, "-render", "SomeView", "-render-form", "dot"),
		2, "SomeView")
}

// TestRenderDocumentCommittedFixture renders the renderer's committed fixture
// through the binary's full analysis, matching the committed golden Markdown.
func TestRenderDocumentCommittedFixture(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.sysml")
	golden, err := os.ReadFile(filepath.Join("..", "..", "internal", "doc", "docrender", "testdata", "telescope_report.golden.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "report.md")
	cmd := exec.Command(binary, fixture, "-render-document", "Observatory::MassReport", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(golden) {
		t.Errorf("rendered fixture differs from the committed golden:\n%s", written)
	}
}

// TestRenderDocumentOutputFile checks that -o writes the Markdown artifact to
// the named file rather than stdout.
func TestRenderDocumentOutputFile(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte(documentModel), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.md")

	cmd := exec.Command(binary, model, "-render-document", "Reports::MassReport", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.HasPrefix(string(written), "# Telescope Mass Report\n") {
		t.Errorf("artifact does not open with the title heading:\n%s", written)
	}
}

// TestRenderDocumentSeveralFiles checks that a document declared in one file
// renders against the elements its sibling files declare, loaded as one model.
func TestRenderDocumentSeveralFiles(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	subject := filepath.Join(dir, "subject.sysml")
	report := filepath.Join(dir, "report.sysml")
	if err := os.WriteFile(subject, []byte(queryModel), 0o644); err != nil {
		t.Fatal(err)
	}
	document := strings.TrimPrefix(documentModel, queryModel)
	if err := os.WriteFile(report, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "report.md")

	cmd := exec.Command(binary, subject, report, "-render-document", "Reports::MassReport", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Telescope Mass Report", "| mount | 15 |"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the rendered document does not contain %q:\n%s", want, written)
		}
	}
}

// TestRenderDocumentFlagConflicts checks that -render-document refuses runs
// asking for something else too.
func TestRenderDocumentFlagConflicts(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-json"),
		2, "-render-document writes a document, not JSON")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-render", "SomeView"),
		2, "ask for one per run")
	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-constraint", "C"),
		2, "check it in its own run")
}

// objectDocumentModel declares a document whose table is bound to a part the
// run may hold an object of, and one over the objects the run holds.
const objectDocumentModel = objectQueryModel + `package Reports {
	private import DocumentQueries::*;
	private import Garage::*;

	part def CarReport :> Document {
		attribute redefines title = "Car Report";

		part parts : Table {
			attribute redefines caption = "Parts of the car";
			calc rows : Parts {
				in root = car;
			}
		}

		part wheels : Table {
			attribute redefines caption = "Wheels held";
			calc rows : Wheels;
		}
	}
}
`

// TestRenderDocumentOverObjects checks that -instantiate is the one check a
// render run takes: a table bound to a part renders the object the run holds
// under that name, by path, and Objects tables fill from what it holds.
func TestRenderDocumentOverObjects(t *testing.T) {
	binary := buildCLI(t)

	wantReport(t, check(t, binary, objectDocumentModel, "-render-document", "Reports::CarReport"),
		0, "# Car Report", "*Parts of the car*", "| name | pressure |", "*Wheels held*")
	declared := check(t, binary, objectDocumentModel, "-render-document", "Reports::CarReport")
	if strings.Contains(declared.stdout, "wheels[1]") {
		t.Errorf("a run holding no object rendered one:\n%s", declared.stdout)
	}

	got := check(t, binary, objectDocumentModel, "-instantiate", "Garage::car", "-render-document", "Reports::CarReport")
	wantReport(t, got, 0,
		"# Car Report",
		"| name | pressure |",
		`| wheels\[1\] | 30 |`,
		`| wheels\[2\] | 30 |`,
		"*Wheels held*",
		"| pressure |",
		"| 30 |")
	if !strings.Contains(got.stderr, "Created instance") {
		t.Errorf("the object created belongs on stderr:\n%s", got.stderr)
	}

	html := check(t, binary, objectDocumentModel, "-instantiate", "Garage::car", "-render-document", "Reports::CarReport", "-doc-form", "html")
	wantReport(t, html, 0,
		`<tr class="sysml-row" data-object="#2" data-element="Garage::Car::wheels" data-element-kind="partUsage">`)

	wantReport(t, check(t, binary, objectDocumentModel, "-instantiate", "Garage::car", "-validate", "-render-document", "Reports::CarReport"),
		2, "decides nothing about the model")
	wantReport(t, check(t, binary, objectDocumentModel, "-instantiate", "Garage::NoSuchPart", "-render-document", "Reports::CarReport"),
		2, "NoSuchPart")
}

// TestRenderDocumentOverObjectsUnderRunBounds checks that the run bounds the
// environment sets are read before -instantiate materializes anything: a bad
// value is refused at startup, and a small one bounds the object made.
func TestRenderDocumentOverObjectsUnderRunBounds(t *testing.T) {
	binary := buildCLI(t)
	render := []string{"-instantiate", "Garage::car", "-render-document", "Reports::CarReport"}

	wantReport(t, checkEnv(t, binary, objectDocumentModel, []string{runtime.MaxStepsEnvVar + "=none"}, render...),
		2, "OPENSYSML_MAX_STEPS=\"none\" is not an integer")
	wantReport(t, checkEnv(t, binary, objectDocumentModel, []string{runtime.MaxElementsEnvVar + "=1"}, render...),
		2, "collection element limit exceeded")
	wantReport(t, checkEnv(t, binary, objectDocumentModel, []string{runtime.MaxElementsEnvVar + "=1"}, "-render-document", "Reports::CarReport"),
		0, "# Car Report")
}
