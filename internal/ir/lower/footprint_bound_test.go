package lower

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// attributeExpression parses src and returns the value expression of the named
// attribute of part def T, with the scope it is read in.
func attributeExpression(t *testing.T, src, name string) (*symbols.Scope, ast.Node) {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	doc := idx.DocumentRoot("test.sysml")
	for _, member := range root.Members {
		def, ok := unwrapMembership(member).(*ast.Definition)
		if !ok || def.Ident.Name != "T" {
			continue
		}
		scope := doc.ChildFor(def)
		if scope == nil {
			t.Fatal("scope for T missing")
		}
		for _, m := range def.Members {
			if u, ok := unwrapMembership(m).(*ast.Usage); ok && u.Ident.Name == name {
				return scope, u.Value
			}
		}
	}
	t.Fatalf("attribute %s of T missing", name)
	return nil, nil
}

// A set of mutually referencing expression-valued features reads each binding once,
// in time linear in the references, not in the paths through them (#1929).
func TestExpressionFootprintReadsEachBoundFeatureOnce(t *testing.T) {
	const n = 24
	var b strings.Builder
	b.WriteString("part def T {\n")
	for i := 1; i <= n; i++ {
		expr := "false"
		for j := n; j >= 1; j-- {
			expr = fmt.Sprintf("(if p%d == %d ? a%d else %s)", i, j, j, expr)
		}
		fmt.Fprintf(&b, "  attribute p%d : Integer default 0;\n", i)
		fmt.Fprintf(&b, "  attribute a%d : Boolean = %s;\n", i, expr)
	}
	b.WriteString("}\n")
	scope, expr := attributeExpression(t, b.String(), "a1")

	fp := ExpressionFootprint(nil, nil, scope, expr)

	got := placeNames(fp.Reads)
	slices.Sort(got)
	var want []string
	for i := 1; i <= n; i++ {
		want = append(want, fmt.Sprintf("a%d", i), fmt.Sprintf("p%d", i))
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("reads = %v, want %v", got, want)
	}
	if fp.Dynamic {
		t.Fatalf("footprint is dynamic: %s", fp)
	}
}

// A cyclic binding stops where it returns to a feature already read.
func TestExpressionFootprintStopsAtACyclicBinding(t *testing.T) {
	scope, expr := attributeExpression(t, `
		part def T {
			attribute x : Integer default 1;
			attribute a : Integer = b + x;
			attribute b : Integer = a * 2;
		}
	`, "a")
	fp := ExpressionFootprint(nil, nil, scope, expr)
	got := placeNames(fp.Reads)
	slices.Sort(got)
	if want := []string{"a", "b", "x"}; !slices.Equal(got, want) {
		t.Fatalf("reads = %v, want %v", got, want)
	}
}
