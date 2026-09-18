package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Prose and quoted names are not notation, so they must not become evidence.
func TestStripModelSource(t *testing.T) {
	const src = "part def A;\n" +
		"// comment about part def\n" +
		"doc /* an about of doc */\n" +
		"attribute x = \"about of\";\n" +
		"attribute 'end of it' = 1;\n"
	stripped := stripModelSource(src)
	if want, got := len(strings.Split(src, "\n")), len(strings.Split(stripped, "\n")); got != want {
		t.Fatalf("line count = %d, want %d preserved", got, want)
	}
	for _, keyword := range []string{"comment", "about", "of", "end"} {
		if strings.Contains(stripped, keyword) {
			t.Errorf("%q survived stripping:\n%s", keyword, stripped)
		}
	}
	for _, keep := range []string{"part def A;", "doc", "attribute x =", "= 1;"} {
		if !strings.Contains(stripped, keep) {
			t.Errorf("%q was stripped:\n%s", keep, stripped)
		}
	}
}

func TestStripModelSourceUnterminated(t *testing.T) {
	for name, src := range map[string]string{
		"comment": "part def A;\n/* unterminated\npart def B;\n",
		"string":  "attribute x = \"unterminated\npart def B;\n",
	} {
		if got := stripModelSource(src); len(strings.Split(got, "\n")) != len(strings.Split(src, "\n")) {
			t.Errorf("%s: line structure changed: %q", name, got)
		}
	}
}

// A keyword must match a whole identifier: 'to' is not evidence for "total".
func TestBuildLiteralIndexWordBoundaries(t *testing.T) {
	repo := t.TempDir()
	write(t, filepath.Join(repo, "models", "a.sysml"), "attribute total : Real;\nitem b to c;\nx <= y;\n")

	lits, err := newLitTable([]string{"to", "item", "<", "<=", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	idx, err := buildLiteralIndex(repo, []corpusRoot{{Name: "models", Dir: "models"}}, lits)
	if err != nil {
		t.Fatal(err)
	}
	file := idx.Files()[0]
	if len(idx.Files()) != 1 || file.path != "models/a.sysml" || file.root != "models" {
		t.Fatalf("files = %+v", idx.Files())
	}
	if got := file.lines["to"]; got != 2 {
		t.Errorf("'to' first seen on line %d, want 2 (not inside \"total\")", got)
	}
	if !file.has("item") || !file.has("<=") {
		t.Errorf("literals = %v", file.lines)
	}
	// Punctuation is a substring match, so "<" is credited to "<=".
	if got := file.lines["<"]; got != 3 {
		t.Errorf("'<' first seen on line %d, want 3", got)
	}
	if file.has("missing") || idx.Present("missing") {
		t.Errorf("absent literal reported present")
	}
	if got := idx.Roots(); len(got) != 1 || got[0].Files != 1 || got[0].Lines != 4 {
		t.Errorf("roots = %+v", got)
	}
}

// A root that has not been downloaded is reported as empty rather than failing,
// and files of other roots nested under it are left to those roots.
func TestBuildLiteralIndexRoots(t *testing.T) {
	repo := t.TempDir()
	write(t, filepath.Join(repo, "examples", "top.sysml"), "part def A;\n")
	write(t, filepath.Join(repo, "examples", "corpora", "nested.sysml"), "port def P;\n")
	write(t, filepath.Join(repo, "examples", "notes.md"), "port def P;\n")

	lits, err := newLitTable([]string{"part", "port"})
	if err != nil {
		t.Fatal(err)
	}
	idx, err := buildLiteralIndex(repo, []corpusRoot{
		{Name: "absent", Dir: "no/such/dir"},
		{Name: "examples", Dir: "examples", Skip: []string{"corpora"}},
	}, lits)
	if err != nil {
		t.Fatal(err)
	}
	roots := idx.Roots()
	if len(roots) != 2 || roots[0].Files != 0 || roots[1].Files != 1 {
		t.Fatalf("roots = %+v, want the absent root empty and the skip honoured", roots)
	}
	if idx.Present("port") {
		t.Errorf("skipped directory and non-model files were searched")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
