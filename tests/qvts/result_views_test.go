package qvts_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// resultView is one query answer written as a QVTS::QueryResult view and that
// view rendered: models are loaded, query is the flag and text that runs it.
type resultView struct {
	name   string
	models []string
	query  []string
	view   string
	status int
}

// renderedForms are the renderings recorded for each view, by file extension:
// RenderView's text and Markdown, and every form RenderDocument writes without a converter.
var renderedForms = []struct {
	extension string
	args      []string
}{
	{".txt", []string{"-render", "", "-render-form", "text"}},
	{".view.md", []string{"-render", "", "-render-form", "markdown"}},
	{".md", []string{"-render-document", "", "-doc-form", "markdown"}},
	{".html", []string{"-render-document", "", "-doc-form", "html"}},
	{".adoc", []string{"-render-document", "", "-doc-form", "asciidoc"}},
}

func TestResultViews(t *testing.T) {
	root := repoRoot(t)
	binary := buildCLI(t, root)
	lander := []string{"examples/qvts/lander.sysml", "examples/qvts/queries.sysml"}
	duplicates := []string{"examples/qvts/lander.sysml", "examples/qvts/duplicates.sysml", "examples/qvts/queries.sysml"}
	named := func(name string) []string { return []string{"-run-query", "QVTSDemo::" + name} }
	cases := []resultView{
		{name: "cabin-mass", models: lander, query: named("CabinMass"), view: "CabinMassResult::cabinMass"},
		{name: "empty-selection", models: lander, query: named("EmptySelection"), view: "EmptySelectionResult::emptySelection"},
		{name: "missing-mass-project", models: lander, query: named("MissingMassProject"), view: "MissingMassProjectResult::missingMassProject", status: 2},
		{name: "misspelled-name", models: lander, query: named("MisspelledName"), view: "MisspelledNameResult::misspelledName", status: 2},
		{name: "ambiguous-part", models: duplicates, query: named("AmbiguousPart"), view: "AmbiguousPartResult::ambiguousPart", status: 2},
		{name: "double-mass-value", models: duplicates, query: named("DoubleMassValue"), view: "DoubleMassValueResult::doubleMassValue"},
		{
			name: "inline-mass", models: lander[:1], view: "AnswerResult::answer",
			query: []string{"-run-inline-query", `Project(WhereType(Descendants(Named("Lander::lander")), "PartUsage", true), ("qualifiedName", "mass"))`},
		},
		{
			name: "oslc-cabin", models: lander[:1], view: "AnswerResult::answer",
			query: []string{"-query", `oslc.where=sysml:name="cabin"`},
		},
		{
			name: "oslc-unknown-property", models: lander[:1], view: "AnswerResult::answer",
			query: []string{"-query", `oslc.where=sysml:nme="cabin"`}, status: 2,
		},
		// The view Open-MBEE/qvts writes by hand over its observatory, rendered as the engine's are.
		{name: "heavy-parts", models: []string{"examples/qvts/observatory/observatory.sysml"}, view: "HeavyPartsResult::heavyParts"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			recorded := filepath.Join("examples", "qvts", "expected", "views", c.name+".sysml")
			if c.query == nil {
				recorded = "examples/qvts/observatory/result.sysml"
			} else {
				written := filepath.Join(t.TempDir(), "view.sysml")
				args := append(append(append([]string(nil), c.models...), c.query...), "-as-view", written)
				runCLI(t, binary, root, c.status, args...)
				// The same run writes the same bytes: a view is deterministic for one commit.
				again := filepath.Join(t.TempDir(), "view.sysml")
				runCLI(t, binary, root, c.status, append(args[:len(args)-1:len(args)-1], again)...)
				first, second := readFile(t, written), readFile(t, again)
				if !bytes.Equal(first, second) {
					t.Fatalf("two runs wrote different views\n--- first ---\n%s\n--- second ---\n%s", first, second)
				}
				compareRecording(t, root, recorded, first)
			}
			for _, form := range renderedForms {
				out := filepath.Join(t.TempDir(), "rendered"+filepath.Ext(form.extension))
				args := append(append([]string(nil), c.models...), recorded)
				args = append(args, form.args[0], c.view, form.args[2], form.args[3], "-o", out)
				runCLI(t, binary, root, 0, args...)
				compareRecording(t, root, filepath.Join("examples", "qvts", "expected", "views", c.name+form.extension), readFile(t, out))
			}
		})
	}
}

func runCLI(t *testing.T, binary, root string, status int, args ...string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	got := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		got = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v\n%s", args, err, output)
	}
	if got != status {
		t.Fatalf("sysml %v: exit = %d, want %d\n%s", args, got, status, output)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func compareRecording(t *testing.T, root, path string, got []byte) {
	t.Helper()
	path = filepath.Join(root, path)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want := readFile(t, path)
	if !bytes.Equal(got, want) {
		t.Errorf("recording differs from %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// TestResultViewsOfOneRunAreDistinct runs two inline queries and one named twice into one
// file: each view after the first of a name is numbered, and each renders by its own name.
func TestResultViewsOfOneRunAreDistinct(t *testing.T) {
	root := repoRoot(t)
	binary := buildCLI(t, root)
	lander := []string{"examples/qvts/lander.sysml", "examples/qvts/queries.sysml"}
	written := filepath.Join(t.TempDir(), "views.sysml")
	inline := `WhereType(Descendants(Named("Lander::lander")), "PartUsage", true)`
	args := append(append([]string(nil), lander...),
		"-run-inline-query", inline, "-run-inline-query", inline,
		"-run-query", "QVTSDemo::CabinMass", "-run-query", "QVTSDemo::CabinMass", "-as-view", written)
	runCLI(t, binary, root, 0, args...)
	text := readFile(t, written)
	views := []string{"AnswerResult::answer", "Answer2Result::answer2", "CabinMassResult::cabinMass", "CabinMass2Result::cabinMass2"}
	for _, view := range views {
		pkg, _, _ := strings.Cut(view, "::")
		if got := bytes.Count(text, []byte("package "+pkg+" {")); got != 1 {
			t.Errorf("package %s is declared %d times, want once\n%s", pkg, got, text)
		}
		out := filepath.Join(t.TempDir(), "rendered.txt")
		runCLI(t, binary, root, 0, append(append([]string(nil), lander...), written, "-render", view, "-render-form", "text", "-o", out)...)
	}
}

// A view specializing a written result view states the run its supertype binds.
func TestResultViewHeaderIsInherited(t *testing.T) {
	root := repoRoot(t)
	binary := buildCLI(t, root)
	lander := []string{"examples/qvts/lander.sysml", "examples/qvts/queries.sysml"}
	dir := t.TempDir()
	written := filepath.Join(dir, "views.sysml")
	runCLI(t, binary, root, 0, append(append([]string(nil), lander...), "-run-query", "QVTSDemo::CabinMass", "-as-view", written)...)
	derived := filepath.Join(dir, "derived.sysml")
	if err := os.WriteFile(derived, []byte("package Kept {\n\tview kept :> CabinMassResult::cabinMass;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	header := func(view string) string {
		out := filepath.Join(t.TempDir(), "rendered.txt")
		runCLI(t, binary, root, 0, append(append([]string(nil), lander...), written, derived, "-render", view, "-render-form", "text", "-o", out)...)
		_, rest, found := strings.Cut(string(readFile(t, out)), "\nquery: ")
		if !found {
			t.Fatalf("%s renders no header", view)
		}
		return rest
	}
	want, got := header("CabinMassResult::cabinMass"), header("Kept::kept")
	if got != want || !strings.Contains(got, "outcome: established") {
		t.Errorf("Kept::kept states\n%s\nwant its supertype's\n%s", got, want)
	}
}
