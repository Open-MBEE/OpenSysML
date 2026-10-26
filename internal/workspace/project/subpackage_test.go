package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A fragment on a repository IRI names a project directory within the tagged
// tree, so the usage names that project — by the fragment's last segment,
// under the repository's owner — not the repository's own project.
func TestUsesMatchesASubpathUsageToTheProjectTheFragmentNames(t *testing.T) {
	widgets := &Interchange{Publisher: "acme", Name: "widgets", Version: "1.0.0"}
	gears := &Interchange{Publisher: "acme", Name: "gears", Version: "1.0.0"}
	for _, resource := range []string{
		"pkg:git/git.example.org/acme/widgets#packages/gears",
		"pkg:git/git.example.org/acme/widgets@1.0.0#packages/gears",
		"pkg:hg/hg.example.org/acme/widgets#gears/",
		"https://git.example.org/acme/widgets.git?ref=main#packages/gears",
		"git@git.example.org:acme/widgets.git#packages/Gears",
	} {
		if !gears.Uses(Usage{Resource: resource}) || widgets.Uses(Usage{Resource: resource}) {
			t.Errorf("%s: gears %v, widgets %v; want the sub-package alone", resource, gears.Uses(Usage{Resource: resource}), widgets.Uses(Usage{Resource: resource}))
		}
	}
	for _, resource := range []string{"pkg:git/git.example.org/acme/widgets", "pkg:git/git.example.org/acme/widgets#", "pkg:sysand/acme/widgets.kpar#packages/gears"} {
		if !widgets.Uses(Usage{Resource: resource}) || gears.Uses(Usage{Resource: resource}) {
			t.Errorf("%s: widgets %v, gears %v; want the repository's project alone", resource, widgets.Uses(Usage{Resource: resource}), gears.Uses(Usage{Resource: resource}))
		}
	}
}

// nestedFixture writes a project using Widgets and, as a sub-package within
// the same repository, Gears. Widgets, installed under .sysand/lib, carries
// its own copy of Gears under packages/gears, as the tagged tree it was
// installed from does; Gears is installed beside it. The model imports both.
func nestedFixture(t *testing.T) (proj, widgets, gears string) {
	t.Helper()
	proj = writeProject(t, filepath.Join(t.TempDir(), "proj"), `{"name": "app", "publisher": "example", "version": "0.1.0", "usage": [
  {"resource": "pkg:git/git.example.org/acme/widgets@1.0.0"},
  {"resource": "pkg:git/git.example.org/acme/widgets#packages/gears", "versionConstraint": "1.0.0"}]}`)
	writeModel(t, filepath.Join(proj, "App.sysml"), "package App {\n    private import Widgets::*;\n    private import Gears::*;\n    part f : Frame;\n    part g : Gear;\n}\n")
	lib := filepath.Join(proj, ".sysand", "lib")
	widgets = writeProject(t, filepath.Join(lib, "acme-widgets_1.0.0"), `{"name": "widgets", "publisher": "acme", "version": "1.0.0", "usage": [{"resource": "pkg:git/git.example.org/acme/widgets#packages/gears"}]}`)
	writeModel(t, filepath.Join(widgets, "Widgets.sysml"), "package Widgets {\n    private import Gears::*;\n    part def Frame {\n        part g : Gear;\n    }\n}\n")
	nested := writeProject(t, filepath.Join(widgets, "packages", "gears"), `{"name": "gears", "publisher": "acme", "version": "1.0.0"}`)
	writeModel(t, filepath.Join(nested, "Gears.sysml"), "package Gears {\n    part def Gear;\n}\n")
	gears = writeProject(t, filepath.Join(lib, "acme-gears_1.0.0"), `{"name": "gears", "publisher": "acme", "version": "1.0.0"}`)
	writeModel(t, filepath.Join(gears, "Gears.sysml"), "package Gears {\n    part def Gear;\n}\n")
	return proj, widgets, gears
}

// A project nested in an installed project is a project of its own, read
// through its own installation: the installed project's files stop at it, so
// a sub-package installed beside its parent loads once, through the usage
// that names it, whether the project directory or its model file is named.
func TestInstalledProjectFilesStopAtAProjectNestedInIt(t *testing.T) {
	proj, widgets, gears := nestedFixture(t)
	app := filepath.Join(proj, "App.sysml")
	widgetsFile := filepath.Join(widgets, "Widgets.sysml")
	gearsFile := filepath.Join(gears, "Gears.sysml")

	installed, err := ReadInterchange(widgets)
	if err != nil {
		t.Fatal(err)
	}
	if got := installed.Files(); !reflect.DeepEqual(got, []string{widgetsFile}) {
		t.Errorf("Files() = %v, want Widgets.sysml alone: packages/gears is a project of its own", got)
	}
	got, err := Expand([]string{proj})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{app, widgetsFile, gearsFile}; !reflect.DeepEqual(got, want) {
		t.Errorf("Expand = %v, want %v", got, want)
	}
	if got, want := Dependencies([]string{app}, nil), []string{gearsFile, widgetsFile}; !reflect.DeepEqual(got, want) {
		t.Errorf("Dependencies of the model = %v, want %v", got, want)
	}
	if got := (Discovery{}).NewWorkspaceIndex(proj).Declaring([]string{"Gears"}, nil); !reflect.DeepEqual(got["Gears"], []string{gearsFile}) {
		t.Errorf("workspace index declares Gears in %v, want %v", got["Gears"], gearsFile)
	}

	// Without its own installation the nested project is not read at all:
	// the import it would answer stays unresolved rather than being answered
	// by a copy no usage selected.
	if err := os.RemoveAll(gears); err != nil {
		t.Fatal(err)
	}
	if got, err := Expand([]string{proj}); err != nil || !reflect.DeepEqual(got, []string{app, widgetsFile}) {
		t.Errorf("Expand without the sub-package installed = %v, %v; want the model and Widgets alone", got, err)
	}
	if got := Dependencies([]string{app}, nil); !reflect.DeepEqual(got, []string{widgetsFile}) {
		t.Errorf("Dependencies without the sub-package installed = %v, want Widgets alone", got)
	}
}
