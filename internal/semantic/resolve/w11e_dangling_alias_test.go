package resolve

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// An alias names an existing element rather than being one (KerML 8.2.3.2), so a
// reference through an alias whose target is unresolved is unresolved too.
func TestW11EReferenceThroughDanglingAliasIsUnresolved(t *testing.T) {
	src := "package P { alias aliass for Nowhere::nothing; classifier Try { feature f : aliass; } }"
	root := parser.New(source.New("a.kerml", []byte(src))).ParseFile()
	idx := symbols.NewIndexFromDoc("a.kerml", root)
	r := New(idx)
	r.ResolveDocument("a.kerml", root)

	var forAliass int
	for _, d := range r.Diagnostics {
		if strings.Contains(d.Message, "aliass") {
			forAliass++
			if strings.Contains(d.Message, "did you mean") {
				t.Errorf("a dangling alias is not a spelling to suggest: %q", d.Message)
			}
		}
	}
	if forAliass != 1 {
		t.Fatalf("expected the reference to aliass to be unresolved once, got %v", r.Diagnostics)
	}
}

// A cyclic alias does name an element — another alias — so a reference to it
// resolves and is diagnosed by the type rules instead.
func TestW11EReferenceThroughCyclicAliasStillResolves(t *testing.T) {
	src := "package P { alias A for B; alias B for A; }"
	idx := indexOf(t, map[string]string{"a.kerml": src})
	r := New(idx)
	scope := scopeOf(t, idx.DocumentRoot("a.kerml"), "P")
	sym, ok := r.ResolveQualified(scope, qn(false, "A"))
	if !ok || sym == nil || sym.Kind != symbols.SymbolAlias {
		t.Fatalf("cyclic alias A = %v (ok=%v), want the alias itself", sym, ok)
	}
}
