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
