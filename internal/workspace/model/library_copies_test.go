package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// copiedSource lists every file of a library source under each of the given
// prefixes, the way a library path holding several copies of the library does.
type copiedSource struct {
	inner    libs.Source
	prefixes []string
}

func (s copiedSource) List() []string {
	var out []string
	for _, prefix := range s.prefixes {
		for _, name := range s.inner.List() {
			out = append(out, prefix+"/"+name)
		}
	}
	return out
}

func (s copiedSource) Read(name string) ([]byte, error) {
	for _, prefix := range s.prefixes {
		if rest, ok := strings.CutPrefix(name, prefix+"/"); ok {
			return s.inner.Read(rest)
		}
	}
	return s.inner.Read(name)
}

// connectionEndsModel inherits `self` into each connection end through Part
// twice over, which two library copies wired into one another report as a
// duplicate inherited name.
const connectionEndsModel = `package Test {
	part def C {
		connection c {
			end part p1;
			end part p2;
		}
	}
}`

// A workspace reports the same diagnostics whether its library's facts were
// derived from the parsed files or restored from the records the previous load
// wrote, whatever number of copies of the library the source holds.
func TestDiagnosticsAgreeAcrossColdAndWarmLibraryLoads(t *testing.T) {
	for _, tc := range []struct {
		name   string
		copies []string
	}{
		{"one copy", []string{"lib"}},
		{"two copies", []string{"one", "two"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := copiedSource{inner: libs.BundledSource(), prefixes: tc.copies}
			cache, err := libs.NewCacheIn(t.TempDir())
			if err != nil {
				t.Fatalf("cache: %v", err)
			}
			var runs []string
			for _, state := range []string{"cold", "warm"} {
				base := symbols.NewIndex()
				loader := libs.NewLoader(src, cache)
				if err := loader.LoadAll(base); err != nil {
					t.Fatalf("%s load: %v", state, err)
				}
				if state == "warm" && loader.Hits() != len(src.List()) {
					t.Fatalf("warm load restored %d documents, want all %d", loader.Hits(), len(src.List()))
				}
				base.Freeze()
				ws := NewWorkspaceWithIndex(symbols.NewOverlay(base), WithLibrarySource(src))
				const uri = "file:///ws/connection.sysml"
				ws.Open(uri, []byte(connectionEndsModel), 1)
				runs = append(runs, strings.Join(diagnosticMessages(ws, uri), "\n"))
				ws.Close(uri)
			}
			if runs[0] != runs[1] {
				t.Errorf("diagnostics differ between the cold and the warm load:\ncold:\n%s\nwarm:\n%s", runs[0], runs[1])
			}
			if strings.Contains(runs[1], "Duplicate of inherited member name") {
				t.Errorf("warm load reports a duplicate inherited name:\n%s", runs[1])
			}
		})
	}
}
