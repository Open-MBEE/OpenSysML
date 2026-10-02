package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const importModel = `package V {
	private import ScalarValues::*;
	private import ISQ::*;
	private import SI::*;
	part def Engine {
		attribute mass : MassValue default = 175 [kg];
	}
	part vehicle {
		attribute count : Integer = 3;
		part engine : Engine;
	}
}
`

// TestImportConvert sets a declared and an inherited value from a TSV file and
// writes the result with -convert sysml -o, leaving the source model alone.
func TestImportConvert(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := write(t, filepath.Join(dir, "m.sysml"), importModel)
	data := write(t, filepath.Join(dir, "d.tsv"), "element\tcount\tmass [kg]\nV::vehicle\t7\t\nV::vehicle::engine\t\t180\n")
	out := filepath.Join(dir, "out.sysml")

	got := runFiles(t, binary, []string{model}, "-import", data, "-convert", "sysml", "-o", out)
	if got.status != 0 || !strings.Contains(got.stderr, "imported 2 values into 2 elements from "+data) {
		t.Fatalf("status %d, stderr:\n%s", got.status, got.stderr)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"attribute count : Integer = 7;", "attribute :>> mass = 180 [kg];", "default = 175 [kg]"} {
		if !strings.Contains(string(written), want) {
			t.Errorf("the written model lacks %q:\n%s", want, written)
		}
	}
	if src, _ := os.ReadFile(model); string(src) != importModel {
		t.Errorf("-import changed the source model:\n%s", src)
	}
	if check := runFiles(t, binary, []string{out}, "-validate"); check.status != 0 {
		t.Errorf("the imported model does not validate: %s", check.stderr)
	}
}

// TestImportDryRun reports the edits and writes nothing.
func TestImportDryRun(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := write(t, filepath.Join(dir, "m.sysml"), importModel)
	data := write(t, filepath.Join(dir, "d.csv"), "element,count\nV::vehicle,7\n")
	got := runFiles(t, binary, []string{model}, "-import", data, "-import-dry-run")
	if got.status != 0 || !strings.Contains(got.stdout, "would import 1 value into 1 element") ||
		!strings.Contains(got.stdout, "V::vehicle::count = 7") {
		t.Errorf("status %d, stdout:\n%s\nstderr:\n%s", got.status, got.stdout, got.stderr)
	}
	if src, _ := os.ReadFile(model); string(src) != importModel {
		t.Errorf("-import-dry-run changed the model:\n%s", src)
	}
}

// TestImportRefusesBadData writes nothing when a row does not fit the model.
func TestImportRefusesBadData(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := write(t, filepath.Join(dir, "m.sysml"), importModel)
	for name, tc := range map[string]struct{ data, want string }{
		"not an integer": {"element,count\nV::vehicle,7.5\n", "line 2, column count"},
		"wrong unit":     {"element,mass [m]\nV::vehicle::engine,3\n", "line 2, column mass [m]"},
		"no element":     {"element,count\nV::nothing,1\n", "line 2: no element named V::nothing"},
		"no feature":     {"element,speed\nV::vehicle,1\n", "line 2, column speed"},
	} {
		t.Run(name, func(t *testing.T) {
			data := write(t, filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".csv"), tc.data)
			out := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".sysml")
			got := runFiles(t, binary, []string{model}, "-import", data, "-convert", "sysml", "-o", out)
			if got.status == 0 || !strings.Contains(got.stderr, tc.want) {
				t.Errorf("status %d, stderr:\n%s", got.status, got.stderr)
			}
			if _, err := os.Stat(out); err == nil {
				t.Errorf("a refused import wrote %s", out)
			}
		})
	}
}

// TestImportFlagMisuse refuses flag combinations that import nothing.
func TestImportFlagMisuse(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := write(t, filepath.Join(dir, "m.sysml"), importModel)
	data := write(t, filepath.Join(dir, "d.csv"), "element,count\nV::vehicle,7\n")
	for name, tc := range map[string]struct {
		args []string
		want string
	}{
		"no convert":      {[]string{"-import", data}, "-import writes the imported model with -convert sysml -o"},
		"map alone":       {[]string{"-import-map", data, "-validate"}, "accompany -import"},
		"dry run convert": {[]string{"-import", data, "-import-dry-run", "-convert", "sysml"}, "writes nothing"},
		"other shape":     {[]string{"-import", data, "-import-as", "elements", "-convert", "sysml"}, `-import-as "elements" is not supported`},
	} {
		t.Run(name, func(t *testing.T) {
			got := runFiles(t, binary, []string{model}, tc.args...)
			if got.status != 2 || !strings.Contains(got.stderr, tc.want) {
				t.Errorf("status %d, stderr:\n%s", got.status, got.stderr)
			}
		})
	}
}
