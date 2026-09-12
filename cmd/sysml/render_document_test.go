package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
// on request, tables either way, and refused where it has nothing to act on.
func TestRenderDocumentDiagramForm(t *testing.T) {
	binary := buildCLI(t)
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
	goldens := map[string]string{
		"mermaid": "telescope_report.golden.md",
		"dot":     "telescope_report.dot.golden.md",
	}
	for form, name := range goldens {
		golden, err := os.ReadFile(filepath.Join("..", "..", "internal", "core", "docrender", "testdata", name))
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

	wantReport(t, check(t, binary, documentModel, "-render-document", "Reports::MassReport", "-diagram-form", "svg"),
		2, `unknown diagram form "svg"`, "-diagram-form takes mermaid, dot")
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
	fixture := filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.sysml")
	golden, err := os.ReadFile(filepath.Join("..", "..", "internal", "core", "docrender", "testdata", "telescope_report.golden.md"))
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
