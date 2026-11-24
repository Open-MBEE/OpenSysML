package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The reflective memos (statement nodes and operands, body results, filter memberships and
// packages, root namespaces) are journaled with the document
// that declares them, so an edit drops every entry rather than keeping the old tree alive.
func TestReflectiveMemosFollowTheDeclaringDocument(t *testing.T) {
	const doc = "reflective.sysml"
	m, _, replace := trackedModel(t, symbols.NewIndex(), doc)
	src := `package Q { part q; }
	package P {
		import Q::*[true];
		filter true;
		calc c { 1 + 2 }
		action a { action a1; first start then a1; then assign x := 1; then if true { action b1; } then for i in (1, 2) { action c1; } then done; attribute x; }
	}`
	read := func(scope *symbols.Scope) {
		p := sym(t, scope, "P")
		for _, rel := range m.OwnedRelationships(p) {
			if rel.Implicit == nil {
				continue
			}
			if _, imported := rel.Implicit.Decl.(*ast.Import); imported {
				pkgs, _ := m.ReflectiveElements(rel, "ownedRelatedElement")
				for _, pkg := range pkgs {
					m.filterPackagePartsOf(pkg)
				}
			}
		}
		m.OwnedMemberships(sym(t, p.Scope, "c"))
		a := sym(t, p.Scope, "a")
		m.OwnedMemberships(a)
		for _, statement := range m.statementNodes(a) {
			m.OwnedMemberships(statement)
		}
		m.RootNamespace(doc)
	}
	read(replace(src))
	sizes := func() map[string]int {
		return map[string]int{
			"statementSymbols":        len(m.statementSymbols),
			"statementOwners":         len(m.statementOwners),
			"statementScopes":         len(m.statementScopes),
			"statementOperandSymbols": len(m.statementOperandSymbols),
			"synthesized":             len(m.synthesized),
			"filterPackages":          len(m.filterPackages),
			"filterPackageImports":    len(m.filterPackageImports),
			"filterPackageMembers":    len(m.filterPackageMembers),
			"rootNamespaces":          len(m.rootNamespaces),
		}
	}
	for table, n := range sizes() {
		if n == 0 {
			t.Errorf("reading the document left %s empty: the test exercises nothing there", table)
		}
	}
	replace(src)
	for table, n := range sizes() {
		if n != 0 {
			t.Errorf("%d %s entries survive the edit, want none", n, table)
		}
	}
}
