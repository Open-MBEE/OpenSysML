package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// renderModel declares a view stating no rendering, one exposing nothing, and a
// part def to ask for by mistake.
const renderModel = `package Demo {
    part def Vehicle { part wheel : Wheel; }
    part def Wheel;
    view overview { expose Demo::Vehicle; }
    view parts { expose Demo::Vehicle; render Views::asElementTable; }
    view empty;
}
`

// The rendering is the run's result on stdout, and everything about the run —
// what was loaded, what the rendering could not show — is on stderr.
func TestRenderWritesTheArtifactOnStdout(t *testing.T) {
	binary := buildCLI(t)

	got := runStreams(t, binary, renderModel, "-render", "Demo::overview")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"flowchart TD", "part def Demo::Vehicle"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	if strings.Contains(got.stdout, "✓ package Demo") {
		t.Errorf("the load report landed in the artifact:\n%s", got.stdout)
	}
	if !strings.Contains(got.stderr, "package Demo") {
		t.Errorf("stderr does not say what the load declared:\n%s", got.stderr)
	}
}

func TestRenderTextFormAndOutputFile(t *testing.T) {
	binary := buildCLI(t)

	out := filepath.Join(t.TempDir(), "view.txt")
	got := runStreams(t, binary, renderModel, "-render", "Demo::overview", "-render-form", "text", "-o", out)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stderr, "wrote "+out) {
		t.Errorf("stderr should name the file written, got:\n%s", got.stderr)
	}
	written, err := os.ReadFile(out) // #nosec G304 -- the test wrote this path.
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "Demo::overview - tree rendering") {
		t.Errorf("the rendering is missing from the file:\n%s", written)
	}
	if strings.Contains(string(written), "wrote ") {
		t.Errorf("the file carries a line about the run:\n%s", written)
	}
}

// A tabular view writes a Markdown table by default and aligned columns as text,
// since a table is no Mermaid diagram.
func TestRenderOfATabularView(t *testing.T) {
	binary := buildCLI(t)

	got := runStreams(t, binary, renderModel, "-render", "Demo::parts")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"| Element | Kind | Type | Declared in |", "| Demo::Vehicle | part def |"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	text := runStreams(t, binary, renderModel, "-render", "Demo::parts", "-render-form", "text")
	if !strings.Contains(text.stdout, "Declared in") || !strings.Contains(text.stdout, "\n-----") {
		t.Errorf("the text form is no aligned table:\n%s", text.stdout)
	}
	mermaid := runStreams(t, binary, renderModel, "-render", "Demo::parts", "-render-form", "mermaid")
	if mermaid.status != exitUnevaluable || !strings.Contains(mermaid.stderr, "ask for text or markdown") {
		t.Errorf("Mermaid of a table = %d\n%s", mermaid.status, mermaid.output())
	}
}

// The DOT form is asked for by name, writes a digraph of the same rendering
// into a .dot file, and is refused for a table with the forms a table has.
func TestRenderDotForm(t *testing.T) {
	binary := buildCLI(t)

	got := runStreams(t, binary, renderModel, "-render", "Demo::overview", "-render-form", "dot")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"// view: Demo::overview", "// layout: dot", `digraph "Demo::overview" {`, `label="part def Demo::Vehicle"`, `"n0" -> "n1" [arrowhead=none];`} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	if strings.Contains(got.stdout, "flowchart") {
		t.Errorf("the DOT form is Mermaid:\n%s", got.stdout)
	}

	dir := filepath.Join(t.TempDir(), "rendered")
	got = runStreams(t, binary, renderAllModel, "-render-all", dir, "-render-form", "dot")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{
		"wrote " + filepath.Join(dir, "Demo.treeView.dot") + " (dot, ",
		"wrote " + filepath.Join(dir, "Demo.stateView.dot") + " (dot, ",
		"Demo::tableView: skipped:",
		"not written as dot; ask for text or markdown",
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr is missing %q:\n%s", want, got.stderr)
		}
	}
	state, err := os.ReadFile(filepath.Join(dir, "Demo.stateView.dot"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`digraph "Demo::stateView" {`, "shape=point", "style=rounded"} {
		if !strings.Contains(string(state), want) {
			t.Errorf("state artifact is missing %q:\n%s", want, state)
		}
	}

	table := runStreams(t, binary, renderModel, "-render", "Demo::parts", "-render-form", "dot")
	if table.status != exitUnevaluable || !strings.Contains(table.stderr, "table rendering is not written as dot; ask for text or markdown") {
		t.Errorf("DOT of a table = %d\n%s", table.status, table.output())
	}
}

// TestRenderSeveralFiles checks that a view declared in one file renders the
// elements its sibling files declare, loaded as one model, on stdout and into
// -o in the form -render-form names.
func TestRenderSeveralFiles(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	types := writeModel(t, dir, "types.sysml", `package Types {
    part def Wheel;
    part def Vehicle { part wheel : Wheel; }
}
`)
	diagrams := writeModel(t, dir, "diagrams.sysml", `package Diagrams {
    private import Types::*;
    view overview { expose Types::Vehicle; }
    view parts { expose Types::*; render Views::asElementTable; }
}
`)

	got := runFiles(t, binary, []string{types, diagrams}, "-render", "Diagrams::overview")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"flowchart TD", "part def Types::Vehicle", "wheel"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout is missing %q:\n%s", want, got.stdout)
		}
	}
	for _, want := range []string{"package Types", "package Diagrams"} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr does not say the load declared %q:\n%s", want, got.stderr)
		}
	}

	out := filepath.Join(dir, "parts.md")
	got = runFiles(t, binary, []string{diagrams, types}, "-render", "Diagrams::parts", "-render-form", "markdown", "-o", out)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if got.stdout != "" {
		t.Errorf("stdout is not empty with -o:\n%s", got.stdout)
	}
	if !strings.Contains(got.stderr, "wrote "+out) {
		t.Errorf("stderr should name the file written, got:\n%s", got.stderr)
	}
	written, err := os.ReadFile(out) // #nosec G304 -- the test wrote this path.
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| Element | Kind | Type | Declared in |", "| Types::Vehicle | part def |", "| Types::Wheel | part def |"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the table is missing %q:\n%s", want, written)
		}
	}

	got = runFiles(t, binary, []string{types, diagrams}, "-render", "#tree", "-render-form", "text")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{"Types", "Diagrams"} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("#tree over two files is missing %q:\n%s", want, got.stdout)
		}
	}
}

func TestRenderReportsWhatItCouldNotDo(t *testing.T) {
	binary := buildCLI(t)

	cases := []struct {
		name   string
		args   []string
		status int
		stderr []string
	}{{
		name:   "a view exposing nothing renders empty and says so",
		args:   []string{"-render", "Demo::empty"},
		status: exitHolds,
		stderr: []string{"note: Demo::empty renders empty"},
	}, {
		name:   "a name that is no view is reported under the command's prefix",
		args:   []string{"-render", "Demo::Vehicle"},
		status: exitUnevaluable,
		stderr: []string{"sysml: ", "not a view"},
	}, {
		name:   "an unknown name is reported",
		args:   []string{"-render", "Demo::Nope"},
		status: exitUnevaluable,
		stderr: []string{"sysml: "},
	}, {
		name:   "a form that is not a form is reported",
		args:   []string{"-render", "Demo::overview", "-render-form", "svg"},
		status: exitUnevaluable,
		stderr: []string{"unknown rendering form \"svg\"", "text, mermaid, markdown, dot"},
	}, {
		name:   "a form without a view to render is reported",
		args:   []string{"-render-form", "text"},
		status: exitUnevaluable,
		stderr: []string{"name the view to render with -render"},
	}, {
		name:   "a form without a view to render is reported even while converting",
		args:   []string{"-convert", "ttl", "-render-form", "mermaid"},
		status: exitUnevaluable,
		stderr: []string{"name the view to render with -render"},
	}, {
		name:   "rendering and converting in one run is refused",
		args:   []string{"-render", "Demo::overview", "-convert", "ttl"},
		status: exitUnevaluable,
		stderr: []string{"ask for one per run"},
	}, {
		name:   "rendering decides nothing, so a check may not be asked for with it",
		args:   []string{"-render", "Demo::overview", "-validate"},
		status: exitUnevaluable,
		stderr: []string{"check it in its own run"},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runStreams(t, binary, renderModel, tc.args...)
			if got.status != tc.status {
				t.Errorf("exit status = %d, want %d\n%s", got.status, tc.status, got.output())
			}
			for _, want := range tc.stderr {
				if !strings.Contains(got.stderr, want) {
					t.Errorf("stderr is missing %q:\n%s", want, got.stderr)
				}
			}
		})
	}
}

// The form -render writes where none was named is the text form at a terminal,
// read by a person, and the kind's machine-readable form into a file or a pipe.
func TestDefaultRenderFormFollowsTheDestination(t *testing.T) {
	cases := []struct {
		name     string
		kind     view.Kind
		output   string
		terminal bool
		want     view.Form
	}{
		{"a table at a terminal", view.KindTable, "", true, view.FormText},
		{"a table into a pipe", view.KindTable, "", false, view.FormMarkdown},
		{"a table into a file", view.KindTable, "table.md", true, view.FormMarkdown},
		{"a tree at a terminal", view.KindTree, "", true, view.FormText},
		{"a tree into a pipe", view.KindTree, "", false, view.FormMermaid},
	}
	for _, c := range cases {
		if got := defaultRenderForm(c.kind, c.output, c.terminal); got != c.want {
			t.Errorf("%s: default form = %q, want %q", c.name, got, c.want)
		}
	}
}

// The text form is written to fit the terminal, and to no width at all into a
// file, so a saved artifact does not depend on the window it was written from.
func TestArtifactWidthIsUnboundedIntoAFile(t *testing.T) {
	if got := artifactWidth("", 100); got != 100 {
		t.Errorf("width on stdout = %d, want the terminal's 100", got)
	}
	if got := artifactWidth("table.txt", 100); got != view.WidthUnbounded {
		t.Errorf("width into a file = %d, want %d", got, view.WidthUnbounded)
	}
}

const renderAllModel = `package Demo {
    part def Vehicle;
    state def Machine {
        entry; then idle;
        state idle;
    }
    view treeView { expose Demo::Vehicle; }
    view tableView {
        expose Demo::Vehicle;
        render Views::asElementTable;
    }
    view stateView : StandardViewDefinitions::StateTransitionView {
        expose Demo::Machine;
    }
}
`

func TestRenderAllWritesOneMachineArtifactPerView(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	got := runStreams(t, binary, renderAllModel, "-render-all", dir)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if got.stdout != "" {
		t.Errorf("stdout is not empty:\n%s", got.stdout)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, file := range files {
		names = append(names, file.Name())
	}
	want := []string{"Demo.stateView.mmd", "Demo.tableView.md", "Demo.treeView.mmd"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("files = %v, want %v", names, want)
	}
	for _, name := range want {
		if !strings.Contains(got.stderr, "wrote "+filepath.Join(dir, name)) {
			t.Errorf("stderr does not list %s:\n%s", name, got.stderr)
		}
	}
	state, err := os.ReadFile(filepath.Join(dir, "Demo.stateView.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), "stateDiagram-v2") {
		t.Errorf("state artifact is not Mermaid:\n%s", state)
	}
	table, err := os.ReadFile(filepath.Join(dir, "Demo.tableView.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(table), "| Element | Kind | Type | Declared in |") {
		t.Errorf("table artifact is not Markdown:\n%s", table)
	}
}

func TestRenderAllForcedTextUsesTxtAndUnboundedWidth(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "text")
	got := runStreams(t, binary, renderAllModel, "-render-all", dir, "-render-form", "text")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("wrote %d files, want 3", len(files))
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".txt" {
			t.Errorf("forced text file = %s", file.Name())
		}
	}
}

func TestRenderAllSkipsUnsupportedKindsAndWrongForcedForms(t *testing.T) {
	binary := buildCLI(t)
	model := `package Demo {
    part def Vehicle;
    view treeView { expose Demo::Vehicle; }
    view textView {
        expose Demo::Vehicle;
        render Views::asTextualNotation;
    }
    view tableView {
        expose Demo::Vehicle;
        render Views::asElementTable;
    }
}`
	dir := filepath.Join(t.TempDir(), "rendered")
	got := runStreams(t, binary, model, "-render-all", dir, "-render-form", "mermaid")
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	for _, want := range []string{
		"Demo::textView: skipped: textual rendering",
		"Demo::tableView: skipped:",
		"ask for text or markdown",
		"wrote " + filepath.Join(dir, "Demo.treeView.mmd"),
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr is missing %q:\n%s", want, got.stderr)
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "Demo.treeView.mmd" {
		t.Errorf("files = %v, want only the tree rendering", files)
	}

	onlyUnsupported := `package Demo {
    part def Vehicle;
    view textView {
        expose Demo::Vehicle;
        render Views::asTextualNotation;
    }
}`
	emptyDir := filepath.Join(t.TempDir(), "unsupported")
	got = runStreams(t, binary, onlyUnsupported, "-render-all", emptyDir)
	if got.status != exitHolds {
		t.Fatalf("only unsupported view status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	files, err = os.ReadDir(emptyDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 || !strings.Contains(got.stderr, "Demo::textView: skipped:") {
		t.Errorf("only unsupported view wrote %v or was not reported:\n%s", files, got.stderr)
	}
}

func TestRenderAllWithoutViewsIsUnevaluable(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	got := runStreams(t, binary, "package Demo { part def Vehicle; }", "-render-all", dir)
	if got.status != exitUnevaluable {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitUnevaluable, got.output())
	}
	if !strings.Contains(got.stderr, "declares no views") {
		t.Errorf("stderr does not say the model has no views:\n%s", got.stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("destination exists after no-view failure: %v", err)
	}
}

func TestRenderAllStopsWhenAnalysisFails(t *testing.T) {
	binary := buildCLI(t)
	dir := filepath.Join(t.TempDir(), "rendered")
	got := runStreams(t, binary, brokenModel, "-render-all", dir)
	if got.status != exitUnevaluable {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitUnevaluable, got.output())
	}
	if !strings.Contains(got.stderr, "did not analyse cleanly; nothing was rendered") {
		t.Errorf("stderr does not explain the rendering failure:\n%s", got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout is not empty:\n%s", got.stdout)
	}
}

func TestRenderAllPrefixesRenderingNoticesWithTheirView(t *testing.T) {
	binary := buildCLI(t)
	model := `package Demo {
    part def Vehicle;
    view actions : StandardViewDefinitions::ActionFlowView {
        expose Demo::Vehicle;
    }
}`
	dir := filepath.Join(t.TempDir(), "rendered")
	got := runStreams(t, binary, model, "-render-all", dir)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	if !strings.Contains(got.stderr, "Demo::actions: note:") ||
		!strings.Contains(got.stderr, "Demo::Vehicle") {
		t.Errorf("stderr does not prefix the rendering notice:\n%s", got.stderr)
	}
	if strings.Contains(got.stderr, "\nnote:") {
		t.Errorf("stderr contains an unprefixed rendering notice:\n%s", got.stderr)
	}
}

func TestRenderAllMutualExclusions(t *testing.T) {
	binary := buildCLI(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"render", []string{"-render-all", "out", "-render", "Demo::treeView"}, "mutually exclusive"},
		{"output", []string{"-render-all", "out", "-o", "one.mmd"}, "cannot be combined with -output"},
		{"convert", []string{"-render-all", "out", "-convert", "ttl"}, "ask for one per run"},
		{"check", []string{"-render-all", "out", "-validate"}, "check it in its own run"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runStreams(t, binary, renderAllModel, tc.args...)
			if got.status != exitUnevaluable {
				t.Errorf("exit status = %d, want %d\n%s", got.status, exitUnevaluable, got.output())
			}
			if !strings.Contains(got.stderr, tc.want) {
				t.Errorf("stderr is missing %q:\n%s", tc.want, got.stderr)
			}
		})
	}
}

func TestRenderFilenameStaysInsideTheDestination(t *testing.T) {
	if _, err := renderFilename("Package::../../outside", view.FormMermaid); err == nil {
		t.Fatal("a view name containing a path was accepted as a rendering filename")
	}
}

// The OOSEM view definitions carry their filter and rendering; a usage only
// exposes, and the inherited filter keeps everything else out of the artifact.
func TestRenderAllOOSEMViews(t *testing.T) {
	binary := buildCLI(t)
	const model = `package Demo {
    private import OOSEM::*;
    package Needs {
        #stakeholderNeed requirement coverage;
        #systemRequirement requirement revisit;
        #logical part def Sensing;
        #logical part sensing : Sensing;
        part def Plain;
    }
    view requirements : RequirementsView { expose Needs::*; }
    view architecture : LogicalArchitectureView { expose Needs::*; }
}
`
	dir := filepath.Join(t.TempDir(), "rendered")
	got := runStreams(t, binary, model, "-render-all", dir)
	if got.status != exitHolds {
		t.Fatalf("exit status = %d, want %d\n%s", got.status, exitHolds, got.output())
	}
	table, err := os.ReadFile(filepath.Join(dir, "Demo.requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| Element | Kind |", "coverage", "revisit"} {
		if !strings.Contains(string(table), want) {
			t.Errorf("requirements table is missing %q:\n%s", want, table)
		}
	}
	for _, unwanted := range []string{"Plain", "sensing"} {
		if strings.Contains(string(table), unwanted) {
			t.Errorf("requirements table leaks %q past the view's filter:\n%s", unwanted, table)
		}
	}
	tree, err := os.ReadFile(filepath.Join(dir, "Demo.architecture.mmd"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"flowchart TD", "Sensing", "sensing"} {
		if !strings.Contains(string(tree), want) {
			t.Errorf("logical tree is missing %q:\n%s", want, tree)
		}
	}
	for _, unwanted := range []string{"Plain", "coverage"} {
		if strings.Contains(string(tree), unwanted) {
			t.Errorf("logical tree leaks %q past the view's filter:\n%s", unwanted, tree)
		}
	}
}
