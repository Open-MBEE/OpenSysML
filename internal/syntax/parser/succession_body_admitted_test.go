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

// The predicate agrees with the parser for every definition and usage kind: a bodied `then` parses clean
// exactly where SuccessionAdmitsBody says its owner admits one — a case or calculation body as an action's.
func TestSuccessionAdmitsBodyAgreesWithTheParser(t *testing.T) {
	check := func(t *testing.T, label, src string) {
		t.Helper()
		p := New(source.New("p.sysml", []byte(src)))
		edge, owner := firstBodiedThen(p.ParseFile().Members)
		if edge == nil {
			return // the kind's body has no `then` member at all
		}
		if got, want := ast.SuccessionAdmitsBody(owner, edge), len(p.Diagnostics) == 0 && edge.HasBody; got != want {
			t.Errorf("%s: SuccessionAdmitsBody = %v, the parser reads the body clean = %v", label, got, want)
		}
	}
	member := func(kw string) string {
		if kw == "state" || kw == "exhibit" {
			return "state"
		}
		return "item"
	}
	for kw := range definitionKindKeywords {
		m := member(kw)
		check(t, kw+" def", "package P { "+kw+" def X { "+m+" a; then b { doc /* d */ } "+m+" b; } }")
	}
	for kw := range usageKindKeywords {
		m := member(kw)
		check(t, kw+" usage", "package P { part def H { "+kw+" x { "+m+" a; then b { doc /* d */ } "+m+" b; } } }")
	}
}

// firstBodiedThen is the first `then` edge among a definition's or usage's members, with that owner.
func firstBodiedThen(nodes []ast.Node) (*ast.SuccessionEdge, ast.Node) {
	for _, n := range nodes {
		var members []ast.Node
		switch d := unwrapped(n).(type) {
		case *ast.Package:
			return firstBodiedThen(d.Members)
		case *ast.Definition:
			members = d.Members
		case *ast.Usage:
			members = d.Members
		default:
			continue
		}
		for _, m := range members {
			if edge, ok := m.(*ast.SuccessionEdge); ok {
				return edge, unwrapped(n)
			}
		}
		if edge, owner := firstBodiedThen(members); edge != nil {
			return edge, owner
		}
	}
	return nil, nil
}
