package repl

import (
	"path/filepath"
	"strings"
	"testing"
)

// Two loaded files that open the same package declare two packages of that
// name, not one shared package. The load says so, rather than leaving the user
// with an unresolved reference and no reason for it.
func TestLoadingTwoFilesThatOpenOnePackageSaysSo(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, filepath.Join(dir, "a.sysml"), "package P {\n    part def Wheel;\n}\n")
	b := writeFile(t, filepath.Join(dir, "b.sysml"), "package P {\n    part def Axle;\n}\n")
	s := NewSession()
	if _, err := s.LoadFile(a); err != nil {
		t.Fatal(err)
	}
	lines, err := s.LoadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "opened by more than one loaded file") {
		t.Errorf("load printed no note about the reopened package: %v", lines)
	}
	// Both declarations stay: neither file loses what it declared.
	for _, name := range []string{"P::Wheel", "P::Axle"} {
		if _, _, err := s.lookupSymbol(name); err != nil {
			t.Errorf("%s did not resolve after both files: %v", name, err)
		}
	}
}

// One file opening one package is no reopening, so nothing is said about it.
func TestLoadingDistinctPackagesSaysNothingAboutReopening(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, filepath.Join(dir, "a.sysml"), "package P {\n    part def Wheel;\n}\n")
	b := writeFile(t, filepath.Join(dir, "b.sysml"), "package Q {\n    part def Axle;\n}\n")
	s := NewSession()
	s.LoadFile(a)
	lines, err := s.LoadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(lines, "\n"), "opened by more than one") {
		t.Errorf("unrelated packages should say nothing: %v", lines)
	}
}

// Re-reading the same file replaces what it declared before, which is no
// reopening either.
func TestReloadingOneFileSaysNothingAboutReopening(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, filepath.Join(dir, "a.sysml"), "package P {\n    part def Wheel;\n}\n")
	s := NewSession()
	s.LoadFile(a)
	writeFile(t, a, "package P {\n    part def Wheel;\n    part def Axle;\n}\n")
	lines, err := s.LoadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(lines, "\n"), "opened by more than one") {
		t.Errorf("re-reading a file should say nothing: %v", lines)
	}
	if _, _, err := s.lookupSymbol("P::Axle"); err != nil {
		t.Errorf("the re-read file's new member did not resolve: %v", err)
	}
}

// Re-typing a package at the prompt still contributes to the package already in
// the buffer: the interactive rule is unchanged.
func TestRetypingAPackageStillMergesInteractively(t *testing.T) {
	s := NewSession()
	s.Submit("package P {\n    part def Wheel;\n}")
	res := s.Submit("package P {\n    part def Axle;\n}")
	if !strings.Contains(strings.Join(res.Notices, "\n"), "added to the existing") {
		t.Errorf("notices = %v, want the merge note", res.Notices)
	}
	for _, name := range []string{"P::Wheel", "P::Axle"} {
		if _, _, err := s.lookupSymbol(name); err != nil {
			t.Errorf("%s did not resolve after the merge: %v", name, err)
		}
	}
}
