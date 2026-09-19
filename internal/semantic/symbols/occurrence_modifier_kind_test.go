package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func buildSysML(t *testing.T, src string) *Scope {
	t.Helper()
	p := parser.New(source.New("p.sysml", []byte(src)))
	root := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("parse error: %s", d.Message)
	}
	return Build(root)
}

// An occurrence modifier with no kind keyword after it decides the symbol kind:
// `individual i` is an individual usage, not the plain feature it was classified as.
func TestOccurrenceModifierDecidesSymbolKind(t *testing.T) {
	scope := buildSysML(t, "package P { individual i : V; snapshot s : F; timeslice ts : F; "+
		"individual part ip : V; ref r : Integer; attribute a : Integer; }")
	pkg, ok := scope.LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	want := map[string]SymbolKind{
		"i":  SymbolIndividualUsage,
		"s":  SymbolOccurrenceUsage,
		"ts": SymbolOccurrenceUsage,
		"ip": SymbolPartUsage,
		"r":  SymbolAttributeUsage,
		"a":  SymbolAttributeUsage,
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

// A parameter may omit its declaration (SysML.xtext Usage: UsageDeclaration?), and
// is then registered as an anonymous member under the kind its prefix states.
func TestAnonymousParameterBuildsSymbol(t *testing.T) {
	scope := buildSysML(t, "action def A { in snapshot ; in event ; in item ; }")
	act, ok := scope.LookupLocal("A")
	if !ok {
		t.Fatal("action def A not indexed")
	}
	anon := act.Scope.AnonymousMembers()
	if len(anon) != 3 {
		t.Fatalf("anonymous members = %d, want 3", len(anon))
	}
	want := []SymbolKind{SymbolOccurrenceUsage, SymbolOccurrenceUsage, SymbolItemUsage}
	for i, kind := range want {
		if anon[i].Kind != kind {
			t.Errorf("kind of anonymous member %d = %v, want %v", i, anon[i].Kind, kind)
		}
		if anon[i].Decl == nil {
			t.Errorf("anonymous member %d has no declaration", i)
		}
	}
}
