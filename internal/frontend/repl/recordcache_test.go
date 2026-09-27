package repl

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A session loading files whose records the cache holds reports the load as a
// session parsing them does — the same summary, the same diagnostics — and
// holds the files as their records: listing what a file declares does not
// hydrate it.
func TestLoadFilesFromRecordsReportsAsLoaded(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeFile(t, filepath.Join(dir, "a.sysml"), "comment /* the bus */\nimport ScalarValues::*;\npackage A { part def Bus { attribute mass : Real; } }\npart def <'1'> ;\n"),
		writeFile(t, filepath.Join(dir, "b.sysml"), "package B { import A::*; part bus : Bus; part loose : Missing; }\n"),
	}
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	load := func() (*Session, []string) {
		s := NewSession()
		s.SetRecordCache(cache)
		lines, err := s.LoadFilesSummary(paths)
		if err != nil {
			t.Fatal(err)
		}
		return s, append(lines, s.DiagnosticLines()...)
	}
	cold, coldLines := load()
	for _, p := range paths {
		if cold.ws.Recorded(p) {
			t.Fatalf("%s is recorded on the load that wrote its record", p)
		}
	}
	warm, warmLines := load()
	for _, p := range paths {
		if !warm.ws.Recorded(p) {
			t.Errorf("%s is not held as its record on a warm load", p)
		}
	}
	if got, want := strings.Join(warmLines, "\n"), strings.Join(coldLines, "\n"); got != want {
		t.Errorf("a warm load reported:\n%s\nwant, as the cold load did:\n%s", got, want)
	}
	if !strings.Contains(strings.Join(warmLines, "\n"), "✓ comment") || !strings.Contains(strings.Join(warmLines, "\n"), "✓ part def <'1'>") {
		t.Errorf("the summary lacks a top-level member:\n%s", strings.Join(warmLines, "\n"))
	}
	for _, p := range paths {
		if !warm.ws.Recorded(p) {
			t.Fatalf("%s was hydrated by listing the load", p)
		}
	}
}
