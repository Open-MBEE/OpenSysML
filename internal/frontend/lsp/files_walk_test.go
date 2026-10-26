package lsp

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// The walk over an installed project stops at a project nested in it; the
// walk over a workspace folder does not.
func TestWalkFolderStopsWhereAsked(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		project.ProjectFile:                     `{"name": "widgets", "version": "1.0.0"}`,
		"Widgets.sysml":                         "package Widgets;\n",
		"packages/gears/" + project.ProjectFile: `{"name": "gears", "version": "1.0.0"}`,
		"packages/gears/Gears.sysml":            "package Gears;\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	widgets := filepath.Join(dir, "Widgets.sysml")
	gears := filepath.Join(dir, "packages", "gears", "Gears.sysml")
	if got, _ := walkFolder(dir, -1, nil); !reflect.DeepEqual(got, []string{widgets, gears}) {
		t.Errorf("walkFolder = %v, want both files", got)
	}
	if got, _ := walkFolder(dir, -1, project.IsProjectDir); !reflect.DeepEqual(got, []string{widgets}) {
		t.Errorf("walkFolder stopping at projects = %v, want Widgets.sysml alone", got)
	}
}
