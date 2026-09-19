package fixtures

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatLibraryCensusRoundTripsThroughRead(t *testing.T) {
	root := t.TempDir()
	writeLibraryCensus(t, root, libraryCensusFixture)
	census, err := ReadLibraryCensus(root)
	if err != nil {
		t.Fatalf("read census: %v", err)
	}
	formatted, err := FormatLibraryCensus(census)
	if err != nil {
		t.Fatalf("format census: %v", err)
	}
	writeLibraryCensus(t, root, string(formatted))
	again, err := ReadLibraryCensus(root)
	if err != nil {
		t.Fatalf("read formatted census: %v", err)
	}
	formattedAgain, err := FormatLibraryCensus(again)
	if err != nil {
		t.Fatalf("format census again: %v", err)
	}
	if string(formattedAgain) != string(formatted) {
		t.Fatalf("census format is not stable:\n%s\n---\n%s", formatted, formattedAgain)
	}
}

// TestLibraryCensusValidateRejectsUnbalancedVerdicts keeps the figures a census
// of the declarations: every declaration verdicted once, nothing else verdicted.
func TestLibraryCensusValidateRejectsUnbalancedVerdicts(t *testing.T) {
	valid := func() LibraryCensus {
		return LibraryCensus{Command: "c", Packages: []LibraryPackage{{
			Name: "P", Path: "p.sysml", Declarations: []string{"P::a", "P::b", "P::c"},
			Evaluated: []string{"P::a"},
			Refused:   []LibraryRefusal{{Declaration: "P::b", Error: "ErrNoValue", Message: "no value"}},
			Wrong:     []LibraryMismatch{{Declaration: "P::c", Mismatch: "value = 1, want 2"}},
		}}}
	}
	if err := valid().Validate(); err != nil {
		t.Fatalf("valid census: %v", err)
	}
	for name, mutate := range map[string]func(*LibraryCensus){
		"no command":            func(c *LibraryCensus) { c.Command = "" },
		"no packages":           func(c *LibraryCensus) { c.Packages = nil },
		"unverdicted":           func(c *LibraryCensus) { c.Packages[0].Evaluated = nil },
		"verdicted twice":       func(c *LibraryCensus) { c.Packages[0].Evaluated = []string{"P::a", "P::b"} },
		"verdict of a stranger": func(c *LibraryCensus) { c.Packages[0].Evaluated = []string{"P::a", "P::z"} },
		"untyped refusal":       func(c *LibraryCensus) { c.Packages[0].Refused[0].Error = "" },
		"silent refusal":        func(c *LibraryCensus) { c.Packages[0].Refused[0].Message = "" },
		"unexplained mismatch":  func(c *LibraryCensus) { c.Packages[0].Wrong[0].Mismatch = "" },
	} {
		t.Run(name, func(t *testing.T) {
			census := valid()
			mutate(&census)
			if err := census.Validate(); err == nil {
				t.Fatal("want a validation error")
			}
			if _, err := FormatLibraryCensus(census); err == nil {
				t.Fatal("want the format to refuse an invalid census")
			}
		})
	}
}

// libraryCensusFixture is a census of one package with a verdict of each kind
// and one package making no callable declaration.
const libraryCensusFixture = `{"command":"go test -run TestFixtureCensus ./x","packages":[` +
	`{"name":"Alpha","path":"a.sysml","declarations":["Alpha::a","Alpha::b","Alpha::c"],"evaluated":["Alpha::a"],` +
	`"refused":[{"declaration":"Alpha::b","error":"ErrNotAFunction","message":"not a function: b"}],` +
	`"wrong":[{"declaration":"Alpha::c","mismatch":"value = 2, want 3"}]},` +
	`{"name":"Empty","path":"e.sysml","declarations":[],"evaluated":[],"refused":[],"wrong":[]}]}`

func writeLibraryCensus(t *testing.T, root, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(LibraryCensusPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", LibraryCensusPath, err)
	}
}
