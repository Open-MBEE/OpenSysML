package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A `perform tag(3);` statement is a performed call, as `action call = tag(3);`
// is, so its name is collected with the invocation and the action context.
func TestPerformStatementReferenceIsAPerformedCall(t *testing.T) {
	const src = `package Lib {
	action def tag { in x; }
}
package App {
	private import Lib::*;
	action def Outer {
		action call = tag(3);
	}
}`
	p := parser.New(source.New("app.sysml", []byte(src)))
	parsed := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	root := &ast.RootNamespace{NodeBase: parsed.NodeBase}
	for _, m := range parsed.Members {
		root.Members = append(root.Members, withPerformStatement(m))
	}
	idx := symbols.NewIndexFromDoc("app.sysml", root)
	idx.ExpandWildcardImports()

	var performed []resolve.Reference
	for _, ref := range resolve.References(root, idx.DocumentRoot("app.sysml")) {
		if nameText(ref.QN) == "tag" {
			performed = append(performed, ref)
		}
	}
	if len(performed) != 1 {
		t.Fatalf("`tag` is referenced %d times, want once", len(performed))
	}
	if ref := performed[0]; ref.Invocation == nil || !ref.Performed {
		t.Errorf("`perform tag(3)` collected as %v, want the invocation marked performed", kindsOf(performed))
	}
}

// withPerformStatement rebuilds n with the usage `action call = …` replaced by
// a `perform …;` statement of its value.
func withPerformStatement(n ast.Node) ast.Node {
	switch d := n.(type) {
	case *ast.Membership:
		m := *d
		m.Member = withPerformStatement(d.Member)
		return &m
	case *ast.Package:
		pkg := *d
		pkg.Members = nil
		for _, m := range d.Members {
			pkg.Members = append(pkg.Members, withPerformStatement(m))
		}
		return &pkg
	case *ast.Definition:
		def := *d
		def.Members = nil
		for _, m := range d.Members {
			def.Members = append(def.Members, withPerformStatement(m))
		}
		return &def
	case *ast.Usage:
		if inv := d.PerformedInvocation(); inv != nil && d.Ident.Name == "call" {
			return &ast.PerformActionNode{NodeBase: d.NodeBase, ActionRef: inv}
		}
	}
	return n
}
