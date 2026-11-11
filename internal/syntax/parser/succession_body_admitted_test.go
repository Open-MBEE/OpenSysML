package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A `then` edge takes a UsageBody where its production is an ActionTargetSuccession — in an action
// body, or in a state body after a state — and none where it is an entry transition's or a definition's.
func TestSuccessionAdmitsBody(t *testing.T) {
	const src = `package P {
		action def A { action x; then y; action y; }
		state def S { entry; then a; state a; then b; state b; }
		part def D { part a; then b; part b; }
	}`
	file := New(source.New("p.sysml", []byte(src))).ParseFile()
	pkg := unwrapped(file.Members[0]).(*ast.Package)
	want := map[string][]bool{"A": {true}, "S": {false, true}, "D": {false}}
	for _, member := range pkg.Members {
		def, ok := unwrapped(member).(*ast.Definition)
		if !ok {
			continue
		}
		var got []bool
		for _, m := range def.Members {
			if edge, ok := m.(*ast.SuccessionEdge); ok {
				got = append(got, ast.SuccessionAdmitsBody(def, edge))
			}
		}
		if len(got) != len(want[def.Ident.Name]) {
			t.Fatalf("%s: %d then edges, want %d", def.Ident.Name, len(got), len(want[def.Ident.Name]))
		}
		for i, w := range want[def.Ident.Name] {
			if got[i] != w {
				t.Errorf("%s then #%d admits a body = %v, want %v", def.Ident.Name, i, got[i], w)
			}
		}
	}
}

// unwrapped is the declaration a Membership wraps, or node itself.
func unwrapped(node ast.Node) ast.Node {
	if m, ok := node.(*ast.Membership); ok {
		return m.Member
	}
	return node
}
