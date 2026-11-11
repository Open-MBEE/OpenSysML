package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A `then` edge takes a UsageBody as an ActionTargetSuccession (SysML.xtext:1698, `then done` included) or a state
// body's TargetTransitionUsage after a state (:1866); none as a member-attached, entry-transition or definition's `then`.
func TestSuccessionAdmitsBody(t *testing.T) {
	const src = `package P {
		action def A { action x; then y; action y; then action z; then done; }
		state def S { entry; then a; state a; then b; state b; do action d; then a; then state c; }
		part def D { part a; then b; part b; }
	}`
	file := New(source.New("p.sysml", []byte(src))).ParseFile()
	pkg := unwrapped(file.Members[0]).(*ast.Package)
	want := map[string][]bool{"A": {true, false, true}, "S": {false, true, false, false}, "D": {false}}
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

// Of a state body's `then` edges, only the one after a state is a TargetTransitionUsage: not an entry
// transition's succession, nor a member-attached `then state c;`.
func TestIsStateTargetTransition(t *testing.T) {
	const src = `package P {
		state def S { entry; then a; state a; then b; state b; then state c; state c { entry; then d; state d; then e; state e; } }
		action def A { action x; then y; action y; }
	}`
	file := New(source.New("p.sysml", []byte(src))).ParseFile()
	pkg := unwrapped(file.Members[0]).(*ast.Package)
	s := unwrapped(pkg.Members[0]).(*ast.Definition)
	var got []bool
	for _, m := range s.Members {
		if edge, ok := m.(*ast.SuccessionEdge); ok {
			got = append(got, ast.IsStateTargetTransition(s, edge))
		}
	}
	if want := []bool{false, true, false}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("S then edges are target transitions = %v, want %v", got, want)
	}
	var c *ast.Usage
	for _, m := range s.Members {
		if u, ok := unwrapped(m).(*ast.Usage); ok && u.Ident.Name == "c" {
			c = u
		}
	}
	got = got[:0]
	for _, m := range c.Members {
		if edge, ok := m.(*ast.SuccessionEdge); ok {
			got = append(got, ast.IsStateTargetTransition(c, edge))
		}
	}
	if want := []bool{false, true}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("c then edges are target transitions = %v, want %v", got, want)
	}
	a := unwrapped(pkg.Members[1]).(*ast.Definition)
	for _, m := range a.Members {
		if edge, ok := m.(*ast.SuccessionEdge); ok && ast.IsStateTargetTransition(a, edge) {
			t.Errorf("an action body's then is a target transition")
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
