package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestConditionTextWritesNamesAsTheNotationDoes(t *testing.T) {
	name := func(parts ...string) *ast.QualifiedName {
		qn := &ast.QualifiedName{}
		for _, p := range parts {
			qn.Parts = append(qn.Parts, ast.NameSegment{Text: p})
		}
		return qn
	}
	cases := map[ast.Node]string{
		&ast.FeatureReference{Name: name("Cfg Space", "Engine Choices", "4cyl")}:                              "'Cfg Space'::'Engine Choices'::'4cyl'",
		&ast.FeatureReference{Name: name("trans", "man")}:                                                     "trans::man",
		&ast.FeatureChainExpr{Operand: &ast.FeatureReference{Name: name("engine")}, Member: name("auto box")}: "engine.'auto box'",
	}
	for node, want := range cases {
		if got := ConditionText(node); got != want {
			t.Errorf("ConditionText = %q, want %q", got, want)
		}
	}
}
