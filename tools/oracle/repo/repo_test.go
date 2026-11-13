package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRootSkipsTheToolsModule: from inside the tools module the root is the
// product module's directory, not the nested go.mod on the way up.
func TestRootSkipsTheToolsModule(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if root != want {
		t.Fatalf("Root() = %s, want %s", root, want)
	}
	if _, err := os.Stat(filepath.Join(root, "tools", "go.mod")); err != nil {
		t.Fatalf("the root does not hold the tools module: %v", err)
	}
}

func TestRootFromRejectsATreeWithoutTheModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, err := RootFrom(nested); err == nil {
		t.Fatal("a tree declaring another module was taken for the repository")
	}
}

// TestChooseAnchorsARelativeRepositoryAtTheRoot: from inside the tools module,
// `-repo .` is the product repository and `-repo ../x` its sibling, never tools/.
func TestChooseAnchorsARelativeRepositoryAtTheRoot(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(string(filepath.Separator), "elsewhere", "checkout")
	cases := map[string]string{
		"":           root,
		".":          root,
		"../sibling": filepath.Join(filepath.Dir(root), "sibling"),
		abs:          abs,
	}
	for given, want := range cases {
		got, err := Choose(given)
		if err != nil {
			t.Fatalf("Choose(%q): %v", given, err)
		}
		if got != want {
			t.Errorf("Choose(%q) = %q, want %q", given, got, want)
		}
	}
}

func TestNeedsRootForAnyEmptyOrRelativePath(t *testing.T) {
	abs := filepath.Join(string(filepath.Separator), "elsewhere", "out")
	if NeedsRoot(abs, abs) {
		t.Errorf("NeedsRoot(%q, %q) = true, want false", abs, abs)
	}
	for _, given := range []string{"", "build/out", "../sibling"} {
		if !NeedsRoot(abs, given) {
			t.Errorf("NeedsRoot(%q, %q) = false, want true", abs, given)
		}
	}
}

func TestResolveAnchorsRelativePathsAtTheRoot(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	abs := filepath.Join(string(filepath.Separator), "elsewhere", "out")
	cases := map[string]string{
		"":                      "",
		abs:                     abs,
		"build/out":             filepath.Join(root, "build", "out"),
		"../sibling/out":        filepath.Join(filepath.Dir(root), "sibling", "out"),
		"docs/project/pin.json": filepath.Join(root, "docs", "project", "pin.json"),
	}
	for given, want := range cases {
		if got := Resolve(root, given); got != want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", root, given, got, want)
		}
	}
}
