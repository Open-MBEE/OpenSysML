package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestBehaviorSuccessionFeaturingType(t *testing.T) {
	model, root := buildModel(t, `package P {
		part def Robot { perform action move; perform action grip; }
		part r : Robot;
		first r::move then r::grip;
		part part1 { action action1; }
		requirement requirement1;
		first part1::action1 then requirement1;
		part b { action g; action m; }
		first b.g then b.m;
		part p1 { action a; }
		part p2 { action b; }
		first p1::a then p2::b;
	}`)
	pkg := sym(t, root, "P")
	type testCase struct {
		earlier, later string
		want           *symbols.Symbol
		wantOK         bool
		path           []string
	}
	tests := []testCase{
		{earlier: "r::move", later: "r::grip", want: nested(t, pkg.Scope, "Robot"), wantOK: true, path: []string{"move", "grip"}},
		{earlier: "part1::action1", later: "requirement1", want: nested(t, pkg.Scope, "part1"), wantOK: true, path: []string{"action1", "requirement1"}},
		{earlier: "b.g", later: "b.m", wantOK: true, path: []string{"b.g", "b.m"}},
		{earlier: "p1::a", later: "p2::b", wantOK: false, path: []string{"a", "b"}},
	}
	decls := model.DeclaredSuccessions(pkg.Scope, pkg, pkg.Decl.(*ast.Package).Members)
	for _, test := range tests {
		t.Run(test.earlier+" then "+test.later, func(t *testing.T) {
			var found *ActionSuccession
			for i := range decls {
				if successionEndText(decls[i].Decl, 0) == test.earlier &&
					successionEndText(decls[i].Decl, 1) == test.later {
					found = &decls[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("succession %q then %q not found", test.earlier, test.later)
			}
			left := model.SuccessionEndPath(pkg.Scope, pkg, successionSyntaxEnd(found.Decl, 0))
			right := model.SuccessionEndPath(pkg.Scope, pkg, successionSyntaxEnd(found.Decl, 1))
			if got := []string{successionPathText(left), successionPathText(right)}; strings.Join(got, "|") != strings.Join(test.path, "|") {
				t.Fatalf("paths = %v, want %v", got, test.path)
			}
			featuring, ok := model.BehaviorSuccessionFeaturingType(
				pkg, [][]*symbols.Symbol{left, right},
				successionSyntaxEnd(found.Decl, 0), successionSyntaxEnd(found.Decl, 1))
			if ok != test.wantOK || featuring != test.want {
				t.Fatalf("featuring type = %v, %t; want %v, %t", featuring, ok, test.want, test.wantOK)
			}
		})
	}
}

func successionSyntaxEnd(decl ast.Node, index int) ast.Node {
	usage := decl.(*ast.Usage)
	return usage.ConnectorEnds[index]
}

func successionEndText(decl ast.Node, index int) string {
	end := successionSyntaxEnd(decl, index).(*ast.ConnectorEnd)
	return successionNameText(end.AttachedTarget())
}

func successionNameText(node ast.Node) string {
	switch n := node.(type) {
	case *ast.FeatureReference:
		return successionNameText(n.Name)
	case *ast.QualifiedName:
		var out strings.Builder
		for i, part := range n.Parts {
			if i > 0 {
				if part.Chained {
					out.WriteByte('.')
				} else {
					out.WriteString("::")
				}
			}
			out.WriteString(part.Text)
		}
		return out.String()
	case *ast.FeatureChainExpr:
		return successionNameText(n.Operand) + "." + successionNameText(n.Member)
	default:
		return ""
	}
}

func successionPathText(path []*symbols.Symbol) string {
	var names []string
	for _, sym := range path {
		names = append(names, sym.Name)
	}
	return strings.Join(names, ".")
}
