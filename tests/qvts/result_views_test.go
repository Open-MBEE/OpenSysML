package qvts_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
