package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A project's lib/ is not scanned as the workspace's own files; what it holds
// reaches the index only through the project's usages (walkWorkspace).
func TestWalkFolderPassesOverTheProjectLibraryDirectory(t *testing.T) {
	folder := t.TempDir()
	files := map[string]string{
		".project.json":                      `{"name": "demo", "publisher": "example", "version": "0.1.0", "usage": [{"resource": "pkg:registry/acme/gears@1.0.0"}]}`,
		"main.sysml":                         "package Main;\n",
		"lib/acme-gears_1.0.0/.project.json": `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`,
		"lib/acme-gears_1.0.0/Gears.kerml":   "package Gears;\n",
		"lib/acme-gears_2.0.0/.project.json": `{"name": "Gears", "publisher": "acme", "version": "2.0.0"}`,
		"lib/acme-gears_2.0.0/Gears.kerml":   "package Gears;\n",
		"plain/lib/Other.sysml":              "package Other;\n",
	}
	for name, content := range files {
		path := filepath.Join(folder, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	paths, _ := walkFolder(folder, -1, nil)
	var got []string
	for _, p := range paths {
		rel, _ := filepath.Rel(folder, p)
		got = append(got, filepath.ToSlash(rel))
	}
	if want := "main.sysml plain/lib/Other.sysml"; strings.Join(got, " ") != want {
		t.Errorf("walkFolder = %v, want %q (lib/ beside .project.json skipped, a lib/ under no project walked)", got, want)
	}
}
