package model

import (
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestResolveQualifiedInDocConcurrent exercises the LSP read path from several
// goroutines over one shared AST. Resolution used to write each resolved symbol
// back into the qualified name's segments, so two concurrent go-to-definition
// requests over the same reference raced; resolution results now live in the
// resolver's side table. Run with -race.
func TestResolveQualifiedInDocConcurrent(t *testing.T) {
	ws := NewWorkspace()
	name := "race.sysml"
	ws.Open(name, []byte(`package P { package Q { part def R; } part def S :> Q::R; }`), 1)

	doc := ws.Document(name)
	if doc == nil || doc.Scope == nil {
		t.Fatal("document/scope missing")
	}
	// The reference the readers share: the `Q::R` target of S's specialization,
	// resolved from the scope of P.
	scope, qn := specializationTarget(t, ws, "P", "S")

	const readers = 8
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sym, ok := ws.ResolveQualifiedInDoc(name, scope, qn); !ok || sym.Name != "R" {
				t.Errorf("ResolveQualifiedInDoc = %v, %v; want symbol R", sym, ok)
			}
		}()
	}
	wg.Wait()
}

// specializationTarget returns the qualified name a nested definition
// specializes, e.g. `Q::R` for `part def S :> Q::R` declared in package pkg,
// along with the scope it is written in.
func specializationTarget(t *testing.T, ws *Workspace, pkg, def string) (*symbols.Scope, *ast.QualifiedName) {
	t.Helper()
	pkgSyms := ws.LookupQualified(pkg)
	if len(pkgSyms) != 1 || pkgSyms[0].Scope == nil {
		t.Fatalf("package %s not indexed", pkg)
	}
	sym, ok := pkgSyms[0].Scope.LookupLocal(def)
	if !ok {
		t.Fatalf("%s::%s not found", pkg, def)
	}
	d, ok := sym.Decl.(*ast.Definition)
	if !ok || len(d.Relationships) == 0 {
		t.Fatalf("%s has no specialization relationship", def)
	}
	qn, ok := d.Relationships[0].Target.(*ast.QualifiedName)
	if !ok {
		t.Fatalf("unexpected relationship target %T", d.Relationships[0].Target)
	}
	return pkgSyms[0].Scope, qn
}
