package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestResultParameterIsMarked covers the `return` forms that declare the result
// parameter of a calculation. All of them are out parameters, and only they are
// result parameters: the distinction decides whether the parameter is redefined
// by position or as the result (SysML v2 7.19.2).
func TestResultParameterIsMarked(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		param    string
		isResult bool
	}{
		{"typed result", "return average : Speed;", "average", true},
		{"untyped result", "return average;", "average", true},
		{"result with value", "return average = 1;", "average", true},
		{"out parameter", "out average : Speed;", "average", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P { attribute def Speed; calc def C { in samples : Speed; " + tc.body + " } }"
			p := New(source.New("t.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			usage := findUsageNamed(root, tc.param)
			if usage == nil {
				t.Fatalf("parameter %q not found", tc.param)
			}
			if usage.Direction != ast.DirOut {
				t.Fatalf("direction of %q = %v, want out", tc.param, usage.Direction)
			}
			if usage.IsResult != tc.isResult {
				t.Fatalf("IsResult of %q = %v, want %v", tc.param, usage.IsResult, tc.isResult)
			}
			if samples := findUsageNamed(root, "samples"); samples == nil || samples.IsResult {
				t.Fatalf("an in parameter must not be marked as the result parameter")
			}
		})
	}
}

// TestResultParameterMultiplicityBeforeSpecialization covers a result parameter
// whose multiplicity is written ahead of its specialization part, the order the
// RDF writer spells (`return y[*] subsets A = ();`): the specialization, the
// value and the body are all read, with the multiplicity on the parameter.
func TestResultParameterMultiplicityBeforeSpecialization(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		rels    int
		value   bool
		hasBody bool
	}{
		{"subsets with value", "return y[*] subsets A = ();", 1, true, false},
		{"symbolic with value", "return attribute y[*] :> A = ();", 1, true, false},
		{"redefines", "return y[0..1] redefines A;", 1, false, false},
		{"multiplicity only", "return y[*];", 0, false, false},
		{"multiplicity with body", "return ref y[0..*] { attribute z; }", 0, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P { attribute A; calc def C { " + tc.body + " } }"
			p := New(source.New("t.sysml", []byte(src)))
			root := p.ParseFile()
			if len(p.Diagnostics) != 0 {
				t.Fatalf("parse diagnostics: %v", p.Diagnostics)
			}
			y := findUsageNamed(root, "y")
			if y == nil {
				t.Fatal("result parameter y not found")
			}
			if !y.IsResult || y.Multiplicity == nil {
				t.Fatalf("y: IsResult=%v Multiplicity=%v, want the result with its multiplicity", y.IsResult, y.Multiplicity)
			}
			if len(y.Relationships) != tc.rels {
				t.Fatalf("y has %d relationships, want %d", len(y.Relationships), tc.rels)
			}
			if (y.Value != nil) != tc.value {
				t.Fatalf("y has value %v, want %v", y.Value != nil, tc.value)
			}
			if y.HasBody != tc.hasBody {
				t.Fatalf("y.HasBody = %v, want %v", y.HasBody, tc.hasBody)
			}
		})
	}
}

// findUsageNamed returns the first usage named name in the namespace members of
// the tree rooted at node.
func findUsageNamed(node ast.Node, name string) *ast.Usage {
	var members []ast.Node
	switch v := node.(type) {
	case *ast.Membership:
		return findUsageNamed(v.Member, name)
	case *ast.RootNamespace:
		members = v.Members
	case *ast.Package:
		members = v.Members
	case *ast.Definition:
		members = v.Members
	case *ast.Usage:
		if v.Ident.Name == name {
			return v
		}
		members = v.Members
	default:
		return nil
	}
	for _, member := range members {
		if found := findUsageNamed(member, name); found != nil {
			return found
		}
	}
	return nil
}
