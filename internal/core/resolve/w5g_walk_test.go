package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A symbol carrying a declaration but no scope must not panic the walk: the
// superseded searchInheritedFeature dereferenced Scope.Parent() behind a
// Decl-only nil check.
func TestW5GFeatureWalkToleratesDeclWithoutScope(t *testing.T) {
	r := New(symbols.NewIndex())
	sym := &symbols.Symbol{Name: "X", Decl: &ast.Definition{}}
	if _, ok := r.featureOf(sym, "anything", newFeatureWalk(nil)); ok {
		t.Fatal("a scope-less symbol declares no members to find")
	}
}
