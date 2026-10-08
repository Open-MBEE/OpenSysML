package repl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureNotebook(name string) string {
	return filepath.Join("..", "..", "..", "tests", "testdata", "notebooks", name)
}

// writeNotebook writes an nbformat 4 notebook whose code cells hold the given
// sources, tagged as given, and returns its path.
func writeNotebook(t *testing.T, path string, cells ...notebookCell) string {
	t.Helper()
	var raw []map[string]any
	for _, c := range cells {
		cell := map[string]any{"cell_type": "code", "metadata": map[string]any{}, "outputs": []any{}, "source": c.source}
		if len(c.tags) > 0 {
			cell["metadata"] = map[string]any{"tags": c.tags}
		}
		raw = append(raw, cell)
	}
	data, err := json.Marshal(map[string]any{
		"cells":    raw,
		"metadata": map[string]any{"kernelspec": map[string]any{"language": "sysml", "name": "sysml"}},
		"nbformat": 4, "nbformat_minor": 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	return writeFile(t, path, string(data))
}

type notebookCell struct {
	source string
	tags   []string
}

func loadMeta(t *testing.T, s *Session, line string) string {
	t.Helper()
	out, _, err := s.RunMeta(line)
	if err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	return strings.Join(out, "\n")
}

func TestLoadingANotebookSubmitsItsCodeCellsInOrder(t *testing.T) {
	s := NewSession()
	out := loadMeta(t, s, "%load "+fixtureNotebook("plain.ipynb"))
	if !strings.Contains(out, "loaded "+fixtureNotebook("plain.ipynb")+": 2 of 2 code cells, 2 declarations") {
		t.Errorf("report:\n%s", out)
	}
	wheels, cars := strings.Index(out, "✓ package Wheels"), strings.Index(out, "✓ package Cars")
	if wheels < 0 || cars < 0 || cars < wheels {
		t.Errorf("want Wheels declared before Cars:\n%s", out)
	}
	if strings.Contains(out, "skipped") {
		t.Errorf("a notebook of declarations alone has nothing to skip:\n%s", out)
	}
	if list := strings.Join(s.List(), "\n"); !strings.Contains(list, "Cars") {
		t.Errorf("the loaded cells are not in the session:\n%s", list)
	}
}

func TestLoadingANotebookSkipsItsCommandsAndExpressionsAndSaysSo(t *testing.T) {
	s := NewSession()
	out := loadMeta(t, s, "%load "+fixtureNotebook("magics.ipynb"))
	for _, want := range []string{
		": 4 of 4 code cells, 2 declarations",
		"skipped 3 % command lines and 4 expression lines: a loaded notebook declares; its commands are not run and its expressions not evaluated",
		"✓ package Demo",
		"✓ package Extra",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "= 17.0") || strings.Contains(out, "= 3") {
		t.Errorf("an expression of the notebook was evaluated:\n%s", out)
	}
	if s.verbosity != VerbosityNormal {
		t.Errorf("the notebook's %%verbosity quiet ran: verbosity is %v", s.verbosity)
	}
	eval, _, err := s.RunMeta("%eval Extra::spare.diameter")
	if err != nil || !strings.Contains(strings.Join(eval, "\n"), "16.0") {
		t.Errorf("the loaded declarations are not usable: %v %v", eval, err)
	}
}

func TestACellTaggedSkipLoadIsPassedOver(t *testing.T) {
	s := NewSession()
	out := loadMeta(t, s, "%load "+fixtureNotebook("tagged.ipynb"))
	if !strings.Contains(out, ": 3 of 4 code cells, 3 declarations") || !strings.Contains(out, "skipped cell 2: tagged skip-load") {
		t.Errorf("report:\n%s", out)
	}
	list := strings.Join(s.List(), "\n")
	for _, want := range []string{"Model", "More", "Untagged"} {
		if !strings.Contains(list, want) {
			t.Errorf("want %s loaded:\n%s", want, list)
		}
	}
	if strings.Contains(list, "Scratch") {
		t.Errorf("the skip-load cell was loaded:\n%s", list)
	}
}

func TestCellsArePickedByPositionOrTag(t *testing.T) {
	for _, tt := range []struct{ line, wantReport string }{
		{"%load " + fixtureNotebook("tagged.ipynb") + " --cells 1,3-4", ": 3 of 4 code cells (--cells 1,3-4), 3 declarations"},
		{"%load --cells=tag:model " + fixtureNotebook("tagged.ipynb"), ": 2 of 4 code cells (--cells tag:model), 2 declarations"},
	} {
		s := NewSession()
		out := loadMeta(t, s, tt.line)
		if !strings.Contains(out, tt.wantReport) {
			t.Errorf("%s:\n%s", tt.line, out)
		}
		list := strings.Join(s.List(), "\n")
		if !strings.Contains(list, "Model") || !strings.Contains(list, "More") || strings.Contains(list, "Scratch") {
			t.Errorf("%s loaded:\n%s", tt.line, list)
		}
	}
	s := NewSession()
	out := loadMeta(t, s, "%load "+fixtureNotebook("tagged.ipynb")+" --cells 1,3-4")
	if strings.Contains(out, "skipped cell") {
		t.Errorf("a cell not picked is not reported skipped:\n%s", out)
	}
	if strings.Contains(out, "loaded 3 files:") {
		t.Errorf("the picked cells of one notebook are not three files:\n%s", out)
	}
	if list := strings.Join(NewSession().List(), "\n"); strings.Contains(list, "Untagged") {
		t.Errorf("cell 4 is not tagged model:\n%s", list)
	}
}

func TestASkipLoadCellIsPassedOverEvenWhenPicked(t *testing.T) {
	s := NewSession()
	out := loadMeta(t, s, "%load "+fixtureNotebook("tagged.ipynb")+" --cells 2")
	if !strings.Contains(out, ": 0 of 4 code cells (--cells 2), 0 declarations") || !strings.Contains(out, "skipped cell 2: tagged skip-load") {
		t.Errorf("report:\n%s", out)
	}
	if len(s.List()) != 0 {
		t.Errorf("loaded %v", s.List())
	}
}

func TestLoadRefusesWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	for _, tt := range []struct{ name, line, want string }{
		{"out of range", "%load " + fixtureNotebook("tagged.ipynb") + " --cells 2,9", "cells 2,9 selected, but the notebook has 4 code cells"},
		{"tag no cell carries", "%load " + fixtureNotebook("tagged.ipynb") + " --cells tag:nope", "no code cell is tagged nope"},
		{"python", "%load " + fixtureNotebook("python.ipynb"), "a python notebook"},
		{"nbformat 3", "%load " + fixtureNotebook("nbformat3.ipynb"), "nbformat 3 is not read"},
		{"malformed", "%load " + fixtureNotebook("malformed.ipynb"), "not a notebook"},
		{"missing", "%load " + filepath.Join(dir, "nope.ipynb"), "cannot read"},
		{"--cells without a notebook", "%load " + filepath.Join(dir, "a.sysml") + " --cells 1", "no .ipynb was named"},
		{"--cells without a selection", "%load " + fixtureNotebook("plain.ipynb") + " --cells", "--cells needs a selection"},
		{"--cells twice", "%load --cells 1 --cells 2 " + fixtureNotebook("plain.ipynb"), "--cells given twice"},
		{"a bad selection", "%load " + fixtureNotebook("plain.ipynb") + " --cells 0", "not a cell position"},
		{"an unknown option", "%load --tags x " + fixtureNotebook("plain.ipynb"), "unknown option --tags"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSession()
			_, _, err := s.RunMeta(tt.line)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%s: err = %v, want %q", tt.line, err, tt.want)
			}
			if len(s.List()) != 0 {
				t.Errorf("%s: a refused load declared %v", tt.line, s.List())
			}
		})
	}
}

func TestDiagnosticsOfALoadedCellNameTheNotebookAndTheCell(t *testing.T) {
	s := NewSession()
	out := loadMeta(t, s, "%load "+fixtureNotebook("broken.ipynb"))
	if want := fixtureNotebook("broken.ipynb") + " cell 2:2:"; !strings.Contains(out, want) {
		t.Errorf("want a diagnostic at %q in:\n%s", want, out)
	}
}

func TestDiagnosticLinesAreTheCellsOwnDespiteSkippedLines(t *testing.T) {
	dir := t.TempDir()
	nb := writeNotebook(t, filepath.Join(dir, "a.ipynb"),
		notebookCell{source: "package A { }\n"},
		notebookCell{source: "%print A\n1 + 2\npackage B {\n  import Missing::X;\n}\n"},
	)
	s := NewSession()
	out := loadMeta(t, s, "%load "+nb)
	if want := nb + " cell 2:4:"; !strings.Contains(out, want) {
		t.Errorf("want a diagnostic at %q in:\n%s", want, out)
	}
}

func TestReloadingANotebookRedeclaresIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.ipynb")
	writeNotebook(t, path,
		notebookCell{source: "package P { part def Wheel; }\n"},
		notebookCell{source: "package Q { part def Axle; }\n"},
	)
	s := NewSession()
	loadMeta(t, s, "%load "+path)
	writeNotebook(t, path, notebookCell{source: "package P { part def Rim; }\n"})
	out := loadMeta(t, s, "%load "+path)
	if !strings.Contains(out, "✓ package P") {
		t.Errorf("report:\n%s", out)
	}
	list := strings.Join(s.List(), "\n")
	if strings.Contains(list, "Wheel") || strings.Contains(list, "Axle") {
		t.Errorf("a stale declaration survived the reload:\n%s", list)
	}
	if !strings.Contains(list, "Rim") {
		t.Errorf("the reloaded cell is not in the session:\n%s", list)
	}
}

func TestReloadingPickedCellsKeepsTheOthers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.ipynb")
	writeNotebook(t, path,
		notebookCell{source: "package P { part def Wheel; }\n"},
		notebookCell{source: "package Q { part def Axle; }\n"},
	)
	s := NewSession()
	loadMeta(t, s, "%load "+path)
	writeNotebook(t, path,
		notebookCell{source: "package P { part def Rim; }\n"},
		notebookCell{source: "package Q { part def Hub; }\n"},
	)
	loadMeta(t, s, "%load "+path+" --cells 1")
	list := strings.Join(s.List(), "\n")
	if !strings.Contains(list, "Rim") || !strings.Contains(list, "Axle") || strings.Contains(list, "Wheel") || strings.Contains(list, "Hub") {
		t.Errorf("want cell 1 refreshed and cell 2 as it was:\n%s", list)
	}
}

func TestAPickedCellLoadedForNothingReplacesWhatItDeclared(t *testing.T) {
	for _, tt := range []struct {
		name string
		cell notebookCell
	}{
		{"an expression now", notebookCell{source: "1 + 2\n"}},
		{"tagged skip-load now", notebookCell{source: "package P { part def Old; }\n", tags: []string{"skip-load"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.ipynb")
			writeNotebook(t, path, notebookCell{source: "package P { part def Old; }\n"}, notebookCell{source: "package Q { part def Kept; }\n"})
			s := NewSession()
			loadMeta(t, s, "%load "+path+" --cells 1")
			writeNotebook(t, path, tt.cell, notebookCell{source: "package Q { part def Kept; }\n"})
			loadMeta(t, s, "%load "+path+" --cells 1")
			list := strings.Join(s.List(), "\n")
			if strings.Contains(list, "Old") {
				t.Errorf("a stale declaration survived:\n%s", list)
			}
		})
	}
}

func TestAPickedCellReloadedByAnotherSpellingReplacesItself(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "real.ipynb")
	alias := filepath.Join(dir, "alias.ipynb")
	if err := os.Symlink(path, alias); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	writeNotebook(t, path, notebookCell{source: "package P { part def Old; }\n"})
	s := NewSession()
	loadMeta(t, s, "%load "+path+" --cells 1")
	writeNotebook(t, path, notebookCell{source: "package P { part def New; }\n"})
	loadMeta(t, s, "%load "+alias+" --cells 1")
	list := strings.Join(s.List(), "\n")
	if strings.Contains(list, "Old") || !strings.Contains(list, "New") || strings.Count(list, "package P") != 1 {
		t.Errorf("want the cell replaced once:\n%s", list)
	}
}

func TestATypedDeclarationStillReplacesALoadedCell(t *testing.T) {
	s := NewSession()
	loadMeta(t, s, "%load "+fixtureNotebook("plain.ipynb"))
	s.Submit("package Cars { part def Truck; }")
	list := strings.Join(s.List(), "\n")
	if strings.Contains(list, "wheels") || !strings.Contains(list, "Truck") {
		t.Errorf("want the typed Cars to replace the loaded one:\n%s", list)
	}
}

func TestANotebookDeclaringNothingReplacesWhatItDeclared(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.ipynb")
	writeNotebook(t, path, notebookCell{source: "package P { part def Wheel; }\n"})
	s := NewSession()
	loadMeta(t, s, "%load "+path)
	writeNotebook(t, path, notebookCell{source: "1 + 2\n"})
	out := loadMeta(t, s, "%load "+path)
	if !strings.Contains(out, ": 1 of 1 code cells, 0 declarations") {
		t.Errorf("report:\n%s", out)
	}
	if list := s.List(); len(list) != 0 {
		t.Errorf("a stale declaration survived: %v", list)
	}
}

func TestNotebooksAreLoadedBesideModelFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib { part def Base; }\n")
	nb := writeNotebook(t, filepath.Join(dir, "model.ipynb"), notebookCell{source: "package M { part def Derived; }\n"})
	writeFile(t, filepath.Join(dir, "README.md"), "not loaded\n")

	s := NewSession()
	out := loadMeta(t, s, "%load "+dir)
	if !strings.Contains(out, "loaded 2 files:") || !strings.Contains(out, "  "+nb) || strings.Contains(out, "cell 1\n") {
		t.Errorf("want the notebook listed once among the files:\n%s", out)
	}
	if !strings.Contains(out, "✓ package Lib") || !strings.Contains(out, "✓ package M") {
		t.Errorf("report:\n%s", out)
	}

	s = NewSession()
	out = loadMeta(t, s, "%load "+filepath.Join(dir, "*.ipynb"))
	if !strings.Contains(out, "✓ package M") || strings.Contains(out, "✓ package Lib") {
		t.Errorf("a pattern picks notebooks:\n%s", out)
	}
}

func TestANotebookPullsInTheSiblingsItImports(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib { part def Base; }\n")
	nb := writeNotebook(t, filepath.Join(dir, "model.ipynb"), notebookCell{source: "package M {\n  private import Lib::*;\n  part def Derived :> Base;\n}\n"})
	s := NewSession()
	out := loadMeta(t, s, "%load "+nb)
	if !strings.Contains(out, "✓ package Lib") || !strings.Contains(out, "✓ package M") || strings.Contains(out, "error") {
		t.Errorf("want Lib loaded as a dependency:\n%s", out)
	}
}

func TestAQuotedNotebookPathLoads(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "with space")
	nb := writeNotebook(t, filepath.Join(dir, "a.ipynb"), notebookCell{source: "package P { }\n"}, notebookCell{source: "package Q { }\n"})
	s := NewSession()
	out := loadMeta(t, s, `%load "`+nb+`" --cells "2"`)
	if !strings.Contains(out, "✓ package Q") || strings.Contains(out, "✓ package P") {
		t.Errorf("report:\n%s", out)
	}
}

func TestPrintingALoadedCellsDeclarationWorks(t *testing.T) {
	s := NewSession()
	loadMeta(t, s, "%load "+fixtureNotebook("plain.ipynb"))
	out, _, err := s.RunMeta("%print Cars")
	if err != nil || !strings.Contains(strings.Join(out, "\n"), "part wheels : Wheel[4];") {
		t.Errorf("%%print Cars = %v, %v", out, err)
	}
}

func TestLoadPathsReportCarriesTheNotebookReport(t *testing.T) {
	s := NewSession()
	rep, err := s.LoadPathsReport([]string{fixtureNotebook("magics.ipynb")})
	if err != nil {
		t.Fatal(err)
	}
	loaded := strings.Join(rep.Loaded, "\n")
	if !strings.Contains(loaded, "4 of 4 code cells") || !strings.Contains(loaded, "skipped 3 % command lines") {
		t.Errorf("Loaded = %v", rep.Loaded)
	}
	if rep.Errors {
		t.Errorf("Found = %v", rep.Found)
	}
}

func TestLoadFilesSummaryReadsNotebooks(t *testing.T) {
	s := NewSession()
	lines, err := s.LoadFilesSummary([]string{fixtureNotebook("plain.ipynb")})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "2 of 2 code cells") || !strings.Contains(joined, "Wheels") || !strings.Contains(joined, "Cars") {
		t.Errorf("summary:\n%s", joined)
	}
}

func TestHelpDescribesNotebookLoading(t *testing.T) {
	out, _, err := NewSession().RunMeta("%help")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "--cells") || !strings.Contains(joined, ".ipynb") {
		t.Errorf("%%help does not describe notebook loading:\n%s", joined)
	}
}

func TestANotebookInTheWorkingDirectoryLoadsByName(t *testing.T) {
	dir := t.TempDir()
	writeNotebook(t, filepath.Join(dir, "a.ipynb"), notebookCell{source: "package P { }\n"})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(wd) }()
	s := NewSession()
	out := loadMeta(t, s, "%load a.ipynb")
	if !strings.Contains(out, "loaded a.ipynb: 1 of 1 code cells, 1 declaration") {
		t.Errorf("report:\n%s", out)
	}
}
