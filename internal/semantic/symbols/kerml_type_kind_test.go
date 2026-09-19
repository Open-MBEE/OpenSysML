package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The parser records KerML type declarations as usages, so they are classified
// from their usage kind rather than left unclassified.
func TestKerMLTypeDeclarationsAreClassified(t *testing.T) {
	src := "package P { class C; classifier D; struct S; assoc A; behavior B; " +
		"interaction I; predicate Q; step s; datatype T; function F; }"
	root := Build(parser.New(source.New("p.kerml", []byte(src))).ParseFile())
	pkg, ok := root.LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	want := map[string]SymbolKind{
		"C": SymbolKerMLType, "D": SymbolKerMLType, "S": SymbolKerMLType,
		"A": SymbolKerMLType, "B": SymbolKerMLType, "I": SymbolKerMLType,
		"Q": SymbolKerMLType, "s": SymbolActionUsage,
		// A `datatype` is a definition even with no specialization to name it one.
		"T": SymbolAttributeDef,
		// A `function` declares a Function, a Behavior specialization: the calc
		// definition of the kernel layer, which may type a step or an action.
		"F": SymbolCalcDef,
	}
	for name, kind := range want {
		sym, ok := pkg.Scope.LookupLocal(name)
		if !ok {
			t.Errorf("%s not indexed", name)
			continue
		}
		if sym.Kind != kind {
			t.Errorf("kind of %s = %v, want %v", name, sym.Kind, kind)
		}
	}
}
