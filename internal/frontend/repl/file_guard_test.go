package repl

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAFileGuardIsAskedAboutEveryFileLoadedOrSaved(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	if err := os.WriteFile(model, []byte("package M { part def P; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("refused by the guard")
	var asked []string
	s := NewSession()
	s.SetFileGuard(func(path string) error {
		asked = append(asked, path)
		if strings.HasSuffix(path, "model.sysml") || strings.HasSuffix(path, "out.sysml") {
			return refused
		}
		return nil
	})
	if _, err := s.LoadPathsReport([]string{dir}); !errors.Is(err, refused) {
		t.Errorf("loading a directory holding a refused file = %v; want the guard's refusal", err)
	}
	if _, err := s.LoadFile(model); !errors.Is(err, refused) {
		t.Errorf("loading a refused file = %v; want the guard's refusal", err)
	}
	s.Submit("package S { part def Q; }")
	lines, written, _ := s.Save(filepath.Join(dir, "out.sysml"))
	if written || len(lines) == 0 || !strings.Contains(lines[0], "refused by the guard") {
		t.Errorf("saving to a refused path = %v, %v; want the refusal and no write", lines, written)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.sysml")); !os.IsNotExist(err) {
		t.Errorf("a refused save wrote the file: %v", err)
	}
	if len(asked) < 3 {
		t.Errorf("the guard was asked %d times (%v); want every file", len(asked), asked)
	}
	s.SetFileGuard(nil)
	if _, err := s.LoadFile(model); err != nil {
		t.Errorf("with the guard lifted the load failed: %v", err)
	}
}

func TestTheFileGuardCoversImportsDependenciesAndLibraries(t *testing.T) {
	dir, lib, outside := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(path, text string) {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "main.sysml"), "package Main { import Beside::*; part def P; }")
	write(filepath.Join(dir, "beside.sysml"), "package Beside { part def B; }")
	write(filepath.Join(lib, "lib.sysml"), "package Lib { part def L; }")
	write(filepath.Join(outside, "elsewhere.sysml"), "package Elsewhere { part def E; }")
	write(filepath.Join(dir, "rows.csv"), "name\nx\n")

	var asked []string
	guard := func(path string) error {
		asked = append(asked, path)
		if strings.HasSuffix(path, "beside.sysml") || strings.HasSuffix(path, "rows.csv") || strings.HasSuffix(path, "linked.sysml") {
			return errors.New("refused by the guard")
		}
		return nil
	}
	s := NewSession()
	s.SetLibraryRoots([]string{lib})
	s.SetFileGuard(guard)

	// The dependency found beside the named file is asked about before it is read, and left out when refused.
	if _, err := s.LoadPathsReport([]string{filepath.Join(dir, "main.sysml")}); err != nil {
		t.Fatalf("loading main.sysml = %v", err)
	}
	if !slices.ContainsFunc(asked, func(p string) bool { return strings.HasSuffix(p, "beside.sysml") }) {
		t.Errorf("the guard was not asked about the dependency; asked %v", asked)
	}
	if _, err := s.EvalExpr("Beside::B"); err == nil {
		t.Error("the refused dependency was loaded")
	}

	// Files under a library root load without the guard; a link under it to a file elsewhere does not.
	asked = nil
	if _, err := s.LoadPathsReport([]string{filepath.Join(lib, "lib.sysml")}); err != nil || len(asked) != 0 {
		t.Errorf("loading a library file = %v, asked %v; want no refusal and no question", err, asked)
	}
	if err := os.Symlink(filepath.Join(outside, "elsewhere.sysml"), filepath.Join(lib, "linked.sysml")); err != nil {
		t.Skip(err)
	}
	if _, err := s.LoadPathsReport([]string{filepath.Join(lib, "linked.sysml")}); err == nil || !strings.Contains(err.Error(), "refused by the guard") {
		t.Errorf("loading a link planted under the library root = %v; want the guard's refusal", err)
	}

	// %import reads its data and mapping files through the guard too.
	if v := s.importData(filepath.Join(dir, "rows.csv"), ImportOptions{}); v.Status != VerdictFails || len(v.Lines) == 0 || !strings.Contains(v.Lines[0], "refused by the guard") {
		t.Errorf("%%import of a refused file = %+v; want the guard's refusal", v)
	}
}
