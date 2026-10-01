package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// An unresolved dotted callee is written collapsed with the dot it was
// spelled with, so the collapsed function and the owned chain agree and the
// graph reads back to the same notation.
func TestUnresolvedChainedCalleeRoundTrips(t *testing.T) {
	file := source.New("n.sysml", []byte("package P {\n    attribute a = service::foo(1);\n}\n"))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	chained := false
	ast.Inspect(root, func(n ast.Node) bool {
		if inv, ok := n.(*ast.InvocationExpr); ok && inv.Type != nil && len(inv.Type.Parts) == 2 {
			inv.Type.Parts[1].Chained = true
			chained = true
		}
		return true
	})
	if !chained {
		t.Fatal("fixture has no two-segment callee")
	}
	graph, err := ToRDF(file, root)
	if err != nil {
		t.Fatal(err)
	}
	var function rdf.Term
	for _, subject := range graph.Subjects() {
		if f, ok := graph.Object(subject, rdf.SysML+pFunction); ok {
			function = f
		}
	}
	if !function.IsLiteral() || function.Value != "service.foo" {
		t.Fatalf("collapsed function = %v, want the literal service.foo", function)
	}
	structural := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.OpenSysML+xSourceText {
			structural.AddTriple(triple)
		}
	}
	back, err := ToSysML(structural)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(back), "service.foo(1)") {
		t.Fatalf("notation lost the chained callee:\n%s", back)
	}
}
