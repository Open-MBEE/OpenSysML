package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func buildKerML(t *testing.T, src string) *Scope {
	t.Helper()
	p := parser.New(source.New("p.kerml", []byte(src)))
	root := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	return Build(root)
}

// A textual representation is a member of the element it represents (KerML
// §7.2.5), so its declared name — and its short name — are defined in the
// owning namespace, while the anonymous form is an anonymous member of it.
func TestTextualRepresentationIsAdoptedByItsOwner(t *testing.T) {
	scope := buildKerML(t, `package P {
		class C {
			rep <ocl> inOCL language "ocl" /* self.x > 0.0 */
			language "alf" /* x = 1; */
		}
	}`)
	pkg, ok := scope.LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	cls, ok := pkg.Scope.LookupLocal("C")
	if !ok {
		t.Fatal("class C not indexed")
	}
	for _, name := range []string{"inOCL", "ocl"} {
		sym, ok := cls.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("%s not indexed", name)
			continue
		}
		if sym.Kind != SymbolTextualRepresentation {
			t.Errorf("kind of %s = %v, want %v", name, sym.Kind, SymbolTextualRepresentation)
		}
	}
	anon := cls.Scope.AnonymousMembers()
	if len(anon) != 1 {
		t.Fatalf("anonymous members = %d, want 1", len(anon))
	}
	if anon[0].Kind != SymbolTextualRepresentation {
		t.Errorf("kind of the anonymous member = %v, want %v", anon[0].Kind, SymbolTextualRepresentation)
	}
}
