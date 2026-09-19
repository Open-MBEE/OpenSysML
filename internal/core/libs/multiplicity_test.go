package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestMultiplicityDeclaration(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "exactlyOne",
			input: `package Test {
				multiplicity exactlyOne [1..1] {
					doc /* exactly one */
				}
			}`,
		},
		{
			name: "zeroOrOne",
			input: `package Test {
				multiplicity zeroOrOne [0..1] {
					doc /* zero or one */
				}
			}`,
		},
		{
			name: "no body",
			input: `package Test {
				multiplicity custom [2..5];
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.New(source.New("test.kerml", []byte(tt.input)))
			_ = p.ParseFile()

			if len(p.Diagnostics) > 0 {
				for _, d := range p.Diagnostics {
					t.Errorf("  %s", d.Message)
				}
			}
		})
	}
}

// A ranged library multiplicity is a MultiplicityRange on every load path: the
// cache and the snapshot restore its declaration, which is what the kind reads.
func TestLibraryMultiplicityRangeKeepsItsMetaclassOnEveryPath(t *testing.T) {
	cacheDir := t.TempDir()
	cold := NewLoader(DefaultSource(), &Cache{dir: cacheDir})
	warm := NewLoader(DefaultSource(), &Cache{dir: cacheDir})
	load := func(ld *Loader) func(*testing.T) *symbols.Index {
		return func(t *testing.T) *symbols.Index {
			idx := symbols.NewIndex()
			if err := ld.LoadAll(idx); err != nil {
				t.Fatalf("load the library: %v", err)
			}
			return idx
		}
	}
	paths := []struct {
		name string
		load func(*testing.T) *symbols.Index
	}{
		{"cold", load(cold)},
		{"warm", load(warm)},
		{"no cache", load(NewLoader(DefaultSource(), nil))},
		{"snapshot", func(t *testing.T) *symbols.Index {
			idx, err := SnapshotIndex()
			if err != nil {
				t.Fatalf("load the snapshot: %v", err)
			}
			return idx
		}},
	}
	for _, path := range paths {
		t.Run(path.name, func(t *testing.T) {
			idx := path.load(t)
			for _, fqn := range []string{"Base::exactlyOne", "Base::zeroOrOne", "Base::oneToMany", "Base::zeroToMany"} {
				sym := lookupOne(t, idx, fqn)
				if sym.Kind != symbols.SymbolMultiplicity {
					t.Errorf("%s has kind %v, want multiplicity", fqn, sym.Kind)
				}
				if got := semantics.MultiplicityMetaclassName(sym); got != "MultiplicityRange" {
					t.Errorf("%s classifies as %s, want MultiplicityRange", fqn, got)
				}
			}
		})
	}
	if warm.Hits() == 0 {
		t.Error("the warm load hit no record, so it did not exercise the restored path")
	}
}
