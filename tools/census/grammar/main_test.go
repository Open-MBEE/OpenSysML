package grammar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Progress and the written-file announcements go to the writer the caller
// supplies, so an embedding program captures the whole run.
func TestRunReportsProgressToTheSuppliedWriter(t *testing.T) {
	repo := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("build/pilot-grammars/Toy.xtext", "grammar org.example.Toy\n\nPackage : 'package' declaredName = Name ';' ;\n\nterminal Name : ('a'..'z')+ ;\n")
	write("tests/testdata/model.sysml", "package p;\n")

	var log strings.Builder
	if err := run(repo, "", filepath.Join(repo, "out"), filepath.Join(repo, "baseline.json"), &log); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Toy.xtext: 2 production(s)\n",
		"searched 1 corpus file(s) for ",
		"wrote " + filepath.Join(repo, "out", "grammar-coverage.json") + "\n",
		"wrote " + filepath.Join(repo, "baseline.json") + "\n",
	} {
		if !strings.Contains(log.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, log.String())
		}
	}
}
