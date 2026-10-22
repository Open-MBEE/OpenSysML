package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The member a declaration registered is read from its scope without
// collecting the scope's members: the lookup allocates nothing and answers
// the first registration, as the scan did.
func TestMemberSymbolReadsTheScopeWithoutCollectingIt(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", parser.New(source.New("<test>", []byte(`package test {
		part def P {
			attribute a = 1;
			attribute b = 2;
			attribute :> a = 3;
			part def Nested;
		}
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	p := idx.LookupQualified("test::P")[0]
	scope := p.Scope
	for _, member := range scope.AllMembers() {
		if member.Decl == nil {
			continue
		}
		got := memberSymbol(scope, member.Decl)
		if got == nil || got.Decl != member.Decl {
			t.Fatalf("memberSymbol(%q) = %v, want the member declared there", member.Name, got)
		}
		if allocs := testing.AllocsPerRun(20, func() { memberSymbol(scope, member.Decl) }); allocs != 0 {
			t.Errorf("memberSymbol(%q) allocates %.0f times, want 0", member.Name, allocs)
		}
	}
	if got := memberSymbol(scope, p.Decl); got != nil {
		t.Fatalf("memberSymbol(scope, P's own declaration) = %v, want nil", got)
	}
}
