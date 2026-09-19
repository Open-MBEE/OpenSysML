package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetaSaveSysML(t *testing.T) {
	s := NewSession()
	s.Submit("package Demo {\npart def Engine;\n}")
	path := filepath.Join(t.TempDir(), "out.sysml")

	out, quit, err := s.runMeta("%save " + path)
	if err != nil || quit {
		t.Fatalf("%%save: err=%v quit=%v", err, quit)
	}
	if !strings.Contains(strings.Join(out, "\n"), "saved") {
		t.Errorf("expected a confirmation, got %v", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package Demo", "part def Engine;"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved file is missing %q:\n%s", want, data)
		}
	}
}

func TestMetaSaveTurtle(t *testing.T) {
	s := NewSession()
	s.Submit("package Demo { part def Engine; }")
	path := filepath.Join(t.TempDir(), "out.ttl")

	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"@prefix sysml:", "elmt:Demo__Engine", "a sysml:PartDefinition"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved Turtle is missing %q:\n%s", want, data)
		}
	}
}

// TestMetaSaveKeepsEveryDeclaration checks that a save covers the whole session,
// not just the last thing typed.
func TestMetaSaveKeepsEveryDeclaration(t *testing.T) {
	s := NewSession()
	s.Submit("package First { part def A; }")
	s.Submit("package Second { part def B; }")
	path := filepath.Join(t.TempDir(), "out.sysml")

	if _, _, err := s.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package First", "package Second"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved file is missing %q:\n%s", want, data)
		}
	}
}

func TestMetaSaveUsage(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	out, _, err := s.runMeta("%save")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "usage:") {
		t.Errorf("expected usage guidance, got %v", out)
	}
}

func TestMetaSaveEmptySession(t *testing.T) {
	s := NewSession()
	path := filepath.Join(t.TempDir(), "out.sysml")
	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "empty") {
		t.Errorf("expected an empty-session message, got %v", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an empty session should not write a file")
	}
}

func TestMetaSaveUnknownExtension(t *testing.T) {
	s := NewSession()
	s.Submit("package P { }")
	path := filepath.Join(t.TempDir(), "out.json")
	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "cannot tell the format") {
		t.Errorf("expected a format error, got %v", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("an unsupported extension should not write a file")
	}
}

func TestMetaSaveThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "model.sysml")

	first := NewSession()
	first.Submit("package Demo { part def Engine; part def Vehicle { part engine : Engine[1]; } }")
	if _, _, err := first.runMeta("%save " + path); err != nil {
		t.Fatal(err)
	}

	second := NewSession()
	if _, _, err := second.runMeta("%load " + path); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(second.List(), "\n"); !strings.Contains(got, "package Demo") {
		t.Errorf("the saved file did not load back: %v", second.List())
	}
}

// A session the parser could not fully read is still written as notation, with
// its syntax errors reported: the file is the user's own text, and refusing
// would strand work that exists nowhere else.
func TestMetaSaveWithSyntaxErrorWarnsAndWrites(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part x; }")
	s.Submit("part 3x;")
	path := filepath.Join(t.TempDir(), "out.sysml")

	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(out, "\n")
	for _, want := range []string{"warning:", "syntax error(s)", "saved"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in the output, got %v", want, out)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"package P", "part 3x;"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved file is missing %q:\n%s", want, data)
		}
	}
}

// Turtle keeps the refusal: a graph built from a tree the parser recovered
// would be quietly missing declarations.
func TestMetaSaveTurtleWithSyntaxErrorRefuses(t *testing.T) {
	s := NewSession()
	s.Submit("part 3x;")
	path := filepath.Join(t.TempDir(), "out.ttl")

	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if joined := strings.Join(out, "\n"); !strings.Contains(joined, "error:") || !strings.Contains(joined, "syntax error(s)") {
		t.Errorf("expected a refusal, got %v", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a broken session should not write Turtle")
	}
}

func TestMetaSaveExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s := NewSession()
	s.Submit("part b;")

	out, _, err := s.runMeta("%save ~/model.sysml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "model.sysml")); err != nil {
		t.Errorf("~ was not expanded (%v): %v", err, out)
	}
}

// A missing parent directory is named, which a bare open(2) failure does not do.
func TestMetaSaveMissingParentDirectory(t *testing.T) {
	s := NewSession()
	s.Submit("part b;")
	dir := filepath.Join(t.TempDir(), "absent")

	_, _, err := s.runMeta("%save " + filepath.Join(dir, "model.sysml"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), dir) || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error does not name the missing directory: %v", err)
	}
}

// The prompt has no -from/-to, so it may not suggest them.
func TestMetaSaveNoExtensionAdvisesTheFileName(t *testing.T) {
	s := NewSession()
	s.Submit("part b;")
	path := filepath.Join(t.TempDir(), "model")

	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "-from") || strings.Contains(joined, "-to") {
		t.Errorf("the REPL suggested command-line flags: %v", out)
	}
	if !strings.Contains(joined, "extension") {
		t.Errorf("expected guidance about the file name, got %v", out)
	}
}

func TestMetaSaveDirectoryPath(t *testing.T) {
	s := NewSession()
	s.Submit("part b;")
	dir := t.TempDir()

	out, _, err := s.runMeta("%save " + dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "is a directory") {
		t.Errorf("expected a directory-specific message, got %v", out)
	}
}

// An overwrite is allowed — the user named the file — but it is stated, so a
// replaced model is never silent.
func TestMetaSaveReportsOverwrite(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part x; }")
	path := filepath.Join(t.TempDir(), "out.sysml")

	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(out, "\n"), "replaced") {
		t.Errorf("a first save replaced nothing: %v", out)
	}
	out, _, err = s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "replaced the existing file") {
		t.Errorf("expected the overwrite to be stated, got %v", out)
	}
}

// TestMetaSaveTurtleIsMarkedExperimental checks a .ttl save reports the RDF
// mapping's status, that a refused one reports it alongside the error, and that
// a notation save does not.
func TestMetaSaveTurtleIsMarkedExperimental(t *testing.T) {
	dir := t.TempDir()

	s := NewSession()
	s.Submit("package Demo { part def Engine; }")
	out, _, err := s.runMeta("%save " + filepath.Join(dir, "out.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "RDF conversion is experimental") {
		t.Errorf("expected an experimental notice, got %v", out)
	}

	out, _, err = s.runMeta("%save " + filepath.Join(dir, "out.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(out, "\n"), "experimental") {
		t.Errorf("a notation save is stable, but was marked: %v", out)
	}

	refusing := NewSession()
	refusing.Submit("package P { part def Seat; part seat : Seat; part seat : Seat; }")
	out, _, err = refusing.runMeta("%save " + filepath.Join(dir, "refused.ttl"))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "error:") {
		t.Fatalf("expected the mapping to refuse the duplicate declaration, got %v", out)
	}
	if !strings.Contains(joined, "RDF conversion is experimental") {
		t.Errorf("a refusal is the experimental behavior, but was not marked: %v", out)
	}
}

// TestMetaSaveHelp checks the command is discoverable.
func TestMetaSaveHelp(t *testing.T) {
	s := NewSession()
	out, _, err := s.runMeta("%help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "%save") {
		t.Error("%help should list the save command")
	}
}

// The work-loss case the tolerant save exists for: an unterminated comment is
// exactly what a user types before reaching for %save.
func TestMetaSaveUnterminatedComment(t *testing.T) {
	s := NewSession()
	s.Submit("part def A;")
	s.Submit("/* oops")
	path := filepath.Join(t.TempDir(), "out.sysml")

	out, _, err := s.runMeta("%save " + path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "saved") {
		t.Fatalf("expected the work to be saved, got %v", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"part def A;", "/* oops"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("saved file is missing %q:\n%s", want, data)
		}
	}
}
