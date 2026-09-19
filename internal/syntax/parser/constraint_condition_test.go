package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A constraint body that declares parameters is read by parseCalcBody, which
// tells a condition from a named `assert constraint { … }` usage. A condition
// whose expression starts with a keyword (`true`, `null`, `if`, …) is still a
// condition and must not be taken for a declaration.
func TestParameterisedConstraintKeywordConditions(t *testing.T) {
	for _, code := range []string{
		`part def P { constraint c { in x : Real; true } }`,
		`part def P { constraint c { in x : Real; not false } }`,
		`part def P { constraint c { in x : Real; if x > 0 ? true else false } }`,
		`part def P { constraint c { in x : Real; null != x } }`,
	} {
		t.Run(code, func(t *testing.T) {
			p := New(source.New("test.sysml", []byte(code)))
			file := p.ParseFile()
			for _, d := range p.Diagnostics {
				t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
			}
			if n := countConstraintMembers(file.Members); n != 1 {
				t.Errorf("got %d constraint conditions, want 1", n)
			}
		})
	}
}

// The same condition parses in a body without parameters, which parseConstraintBody reads.
func TestConstraintKeywordConditionsWithoutParameters(t *testing.T) {
	const code = `part def P { constraint c { true } }`

	p := New(source.New("test.sysml", []byte(code)))
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("offset %d: %s", d.Span.Offset, d.Message)
	}
	if n := countConstraintMembers(file.Members); n != 1 {
		t.Errorf("got %d constraint conditions, want 1", n)
	}
}

func countConstraintMembers(nodes []ast.Node) int {
	total := 0
	for _, n := range nodes {
		switch v := n.(type) {
		case *ast.ConstraintMember:
			total++
		case *ast.Membership:
			total += countConstraintMembers([]ast.Node{v.Member})
		case *ast.Package:
			total += countConstraintMembers(v.Members)
		case *ast.Definition:
			total += countConstraintMembers(v.Members)
		case *ast.Usage:
			total += countConstraintMembers(v.Members)
		}
	}
	return total
}
