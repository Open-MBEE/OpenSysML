package fixtures

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCaseNameTellsACaseFromItsCheckExpectation(t *testing.T) {
	for fileName, want := range map[string]string{
		"calc_add.expected.json":       "calc_add",
		"calc_add.check.expected.json": "",
		"calc_add.sysml":               "",
		"calc_add.trace.golden":        "",
		"known_failures.txt":           "",
	} {
		name, ok := CaseName(fileName)
		if ok != (want != "") || name != want {
			t.Errorf("CaseName(%q) = %q, %v; want %q", fileName, name, ok, want)
		}
		if IsCase(fileName) != ok {
			t.Errorf("IsCase(%q) disagrees with CaseName", fileName)
		}
	}
}

func TestCasesListsTheDirectoryInNameOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"state_b.expected.json", "state_b.sysml", "state_b.check.expected.json",
		"action_a.expected.json", "action_a.sysml", "action_a.trace.golden",
		"README.md", "known_failures.txt",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested.expected.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases, err := Cases(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"action_a", "state_b"}; !reflect.DeepEqual(cases, want) {
		t.Fatalf("cases: %v, want %v", cases, want)
	}
}

func TestKnownFailuresSkipsCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	if failures, err := KnownFailures(dir); err != nil || len(failures) != 0 {
		t.Fatalf("absent file: %v, %v; want an empty set", failures, err)
	}
	text := "# comment\n\n  state_b  \naction_a\n"
	if err := os.WriteFile(filepath.Join(dir, "known_failures.txt"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	failures, err := KnownFailures(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]bool{"state_b": true, "action_a": true}; !reflect.DeepEqual(failures, want) {
		t.Fatalf("failures: %v, want %v", failures, want)
	}
}

// TestTracesListsWhatTheTraceHarnessSchedules mirrors TestExecutionTrace: a
// known failure's goldens are passed over, an opted-in case owns its policy
// goldens, and one no gate would read is an error naming it.
func TestTracesListsWhatTheTraceHarnessSchedules(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"calc_a", "calc_b", "state_a", "state_b"} {
		write(name+".sysml", "package P;\n")
		write(name+".expected.json", "{}\n")
	}
	write("calc_a.expected.json", `{"outcomes": [{}, {}]}`+"\n")
	write("calc_a.trace.golden", "")
	write("calc_a.declared.trace.golden", "")
	write("calc_a.seed-1.trace.golden", "")
	write("state_a.expected.json", `{"trace": true, "outcomes": [{}]}`+"\n")
	write("state_a.declared.trace.golden", "")
	write("state_b.trace.golden", "")
	write("known_failures.txt", "state_b\n")

	traces, err := Traces(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Trace{
		{Case: "calc_a", Policy: "declared"},
		{Case: "calc_a", Policy: "seed:1"},
		{Case: "calc_a"},
		{Case: "state_a", Policy: "declared"},
	}
	if !reflect.DeepEqual(traces, want) {
		t.Fatalf("traces: %v, want %v", traces, want)
	}

	for name, text := range map[string]string{
		"calc_a.typo.trace.golden":     "",
		"calc_b.declared.trace.golden": "",
		"ghost.trace.golden":           "",
	} {
		write(name, text)
		_, err := Traces(dir)
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("%s: err = %v, want one naming the file", name, err)
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	write("state_a.expected.json", `{"outcomes": [{}]}`+"\n")
	if _, err := Traces(dir); err == nil || !strings.Contains(err.Error(), "state_a.declared.trace.golden") {
		t.Fatalf("policy golden of a case owning no default golden: err = %v", err)
	}
}
