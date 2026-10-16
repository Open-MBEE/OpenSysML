package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A name a run reads on every step is resolved once per scope: the second
// lookup of the same name, found or not, comes from the model's memo and
// allocates nothing, and answers as the resolver did.
func TestNameLookupIsMemoizedPerScope(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(`package test {
		private import ScalarValues::*;
		part def Base { attribute inherited : Integer = 1; }
		part def P :> Base { attribute own : Integer = 2; }
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(typedModel(semantics.NewModel(resolver), resolver), 1_000_000)
	scope := idx.LookupQualified("test::P")[0].Scope

	for _, name := range []string{"own", "inherited", "absent"} {
		want, wantOK := resolver.LookupName(scope, name)
		got, ok := ctx.lookupName(scope, name)
		if got != want || ok != wantOK {
			t.Fatalf("lookupName(%s) = %v, %v; resolver says %v, %v", name, got, ok, want, wantOK)
		}
		if allocs := testing.AllocsPerRun(50, func() {
			if again, againOK := ctx.lookupName(scope, name); again != got || againOK != ok {
				t.Errorf("lookupName(%s) again = %v, %v; first was %v, %v", name, again, againOK, got, ok)
			}
		}); allocs != 0 {
			t.Errorf("a repeated lookupName(%s) allocates %.0f objects, want 0", name, allocs)
		}
	}
}
