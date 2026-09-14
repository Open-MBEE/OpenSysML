package fixtures

import (
	"os"
	"path/filepath"
	"reflect"
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
