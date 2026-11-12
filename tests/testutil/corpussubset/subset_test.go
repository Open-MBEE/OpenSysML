package corpussubset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeList(t *testing.T, lines ...string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(ListPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestUnsetIteratesEverything(t *testing.T) {
	t.Setenv(EnvVar, "")
	s := Must(t, t.TempDir())
	if !s.Whole() || !s.Wants("anything.sysml") || !s.WantsPath("examples", "examples/x.sysml") {
		t.Fatalf("unset %s must iterate every file", EnvVar)
	}
	if err := s.Recording("-update-x"); err != nil {
		t.Fatalf("recording over the whole corpus must be allowed: %v", err)
	}
}

func TestConstructCoverIteratesTheList(t *testing.T) {
	root := writeList(t, "# header", "a.sysml", "pilot-corpora/kerml-examples/b.kerml")
	t.Setenv(EnvVar, ConstructCover)
	s := Must(t, root)
	if s.Whole() || s.Name() != ConstructCover {
		t.Fatalf("expected the %s subset, got %q", ConstructCover, s.Name())
	}
	if !s.Wants("a.sysml") || s.Wants("c.sysml") {
		t.Errorf("Wants: a.sysml yes, c.sysml no")
	}
	if !s.WantsPath("../../examples", filepath.FromSlash("../../examples/pilot-corpora/kerml-examples/b.kerml")) {
		t.Errorf("WantsPath must match a listed file under examplesDir")
	}
	if s.WantsPath("../../examples", filepath.FromSlash("../../examples/c.sysml")) {
		t.Errorf("WantsPath must drop an unlisted file under examplesDir")
	}
	if !s.WantsPath("../../examples", filepath.FromSlash("../../tests/testdata/c.sysml")) {
		t.Errorf("WantsPath must keep a file outside examplesDir")
	}
	if got := s.Filter([]string{"a.sysml", "c.sysml"}); len(got) != 1 || got[0] != "a.sysml" {
		t.Errorf("Filter = %v", got)
	}
	if err := s.Recording("-update-x"); err == nil || !strings.Contains(err.Error(), "-update-x") {
		t.Errorf("recording under the subset must be refused naming the flag, got %v", err)
	}
}

func TestUnknownValueFails(t *testing.T) {
	t.Setenv(EnvVar, "construct_cover")
	if _, err := Load(t.TempDir()); err == nil || !strings.Contains(err.Error(), "unknown subset") {
		t.Fatalf("an unknown value must fail loudly, got %v", err)
	}
}

func TestListMustBeSortedAndNonEmpty(t *testing.T) {
	t.Setenv(EnvVar, ConstructCover)
	if _, err := Load(writeList(t, "b.sysml", "a.sysml")); err == nil || !strings.Contains(err.Error(), "not sorted") {
		t.Errorf("an unsorted list must be rejected, got %v", err)
	}
	if _, err := Load(writeList(t, "# only a comment")); err == nil || !strings.Contains(err.Error(), "no files") {
		t.Errorf("an empty list must be rejected, got %v", err)
	}
	if _, err := Load(t.TempDir()); err == nil {
		t.Errorf("a missing list must be rejected")
	}
}

func TestFilterPathsAndWantsAnyFollowTheList(t *testing.T) {
	root := writeList(t, "demo/a.sysml")
	t.Setenv(EnvVar, ConstructCover)
	s := Must(t, root)
	paths := []string{"../../examples/demo/a.sysml", "../../examples/demo/b.sysml", "../../tests/testdata/c.sysml"}
	if got := s.FilterPaths("../../examples", paths); len(got) != 2 || got[1] != paths[2] {
		t.Errorf("FilterPaths = %v, want the listed example and the file outside examples/", got)
	}
	if !s.WantsAny("../../examples", paths[:2]) || s.WantsAny("../../examples", paths[1:2]) || s.WantsAny("../../examples", nil) {
		t.Errorf("WantsAny must follow the listed file alone")
	}
	t.Setenv(EnvVar, "")
	whole := Must(t, root)
	if !whole.WantsAny("../../examples", nil) || len(whole.FilterPaths("../../examples", paths)) != 3 {
		t.Errorf("the whole corpus wants every directory and every path")
	}
}
