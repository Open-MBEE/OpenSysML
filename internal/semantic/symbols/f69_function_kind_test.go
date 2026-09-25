package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// F69: a KerML `function` declares a Function, a Behavior specialization, so it
// is a definition and may type a step or an action — as
// `OccurrenceFunctions::destroy` does in the Kernel Function Library.
func TestF69FunctionIsADefinition(t *testing.T) {
	src := "package OccurrenceFunctions { function destroy { in occ; return : Occurrence; } " +
		"calc c; behavior b { step s : destroy; } }"
	root := Build(parser.New(source.New("p.kerml", []byte(src))).ParseFile())
	pkg, ok := root.LookupLocal("OccurrenceFunctions")
	if !ok {
		t.Fatal("package OccurrenceFunctions not indexed")
	}
	for name, want := range map[string]SymbolKind{
		"destroy": SymbolCalcDef,
		// A `calc` still declares a usage: only `function` names a definition.
		"c": SymbolCalcUsage,
	} {
		sym, ok := pkg.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("%s not indexed", name)
		}
		if sym.Kind != want {
			t.Errorf("kind of %s = %v, want %v", name, sym.Kind, want)
		}
	}
}

// F60: a `satisfy requirement` usage gets its own kind rather than falling
// through to unknown, so a reference to one can be classified.
func TestF60SatisfyUsageIsClassified(t *testing.T) {
	src := "package P { requirement def Req; part def S; part s : S; " +
		"part ctx { satisfy requirement req : Req by s; } }"
	root := Build(parser.New(source.New("p.sysml", []byte(src))).ParseFile())
	pkg, ok := root.LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	ctx, ok := pkg.Scope.LookupLocal("ctx")
	if !ok || ctx.Scope == nil {
		t.Fatal("part ctx not indexed")
	}
	sym, ok := ctx.Scope.LookupLocal("req")
	if !ok {
		t.Fatal("satisfy usage req not indexed")
	}
	if sym.Kind != SymbolSatisfyRequirementUsage {
		t.Errorf("kind of req = %v, want %v", sym.Kind, SymbolSatisfyRequirementUsage)
	}
}
