package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An expression owns one `in` parameter per operand, each binding its operand
// through a FeatureValue, and the `out` result parameter when its evaluation
// yields a value (KerML 8.4.4.9), as the conversion writes them.
func TestExpressionOwnsItsParametersAndResult(t *testing.T) {
	m, root := buildModelWithStdlib(t, `
		package P {
			part def D {
				attribute a : ScalarValues::Real;
				attribute b : ScalarValues::Real;
				attribute x = a + b * 2;
				attribute y = a as ScalarValues::Integer;
				attribute z = 1;
			}
		}
	`)
	pkg, _ := root.LookupLocal("P")
	d := member(t, m, pkg, "D")
	names := func(syms []*symbols.Symbol) []string {
		var out []string
		for _, sym := range syms {
			if mc := m.MetaclassOf(sym); mc != nil {
				out = append(out, mc.Name)
			} else {
				out = append(out, "?")
			}
		}
		return out
	}
	equal := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	x := m.ValueExpression(member(t, m, d, "x"))
	if x == nil {
		t.Fatal("x: no value expression")
	}
	params := m.ExpressionParameters(x)
	if got := names(params); !equal(got, []string{"Feature", "Feature"}) {
		t.Fatalf("x parameters: %v", got)
	}
	if again := m.ExpressionParameters(x); len(again) != 2 || again[0] != params[0] || again[1] != params[1] {
		t.Error("x parameters are not stable across reads")
	}
	result := m.ExpressionResult(x)
	if result == nil || m.MetaclassOf(result).Name != "Feature" {
		t.Fatal("x: no result parameter")
	}
	for i, want := range []ast.FeatureDirection{ast.DirIn, ast.DirIn} {
		if got, ok := ReflectiveDirection(params[i]); !ok || got != want {
			t.Errorf("in%d direction: %v %v", i, got, ok)
		}
	}
	if got, ok := ReflectiveDirection(result); !ok || got != ast.DirOut {
		t.Errorf("out direction: %v %v", got, ok)
	}
	if got, ok := ReflectiveDirection(x); !ok || got != ast.DirNone {
		t.Errorf("expression direction: %v %v", got, ok)
	}
	if got := names(m.OwnedMemberships(x)); !equal(got, []string{"ParameterMembership", "ParameterMembership", "ReturnParameterMembership"}) {
		t.Errorf("x owned memberships: %v", got)
	}
	for feature, want := range map[string][]*symbols.Symbol{
		"input":           params,
		"parameter":       {params[0], params[1], result},
		"ownedFeature":    {params[0], params[1], result},
		"directedFeature": {params[0], params[1], result},
		"ownedElement":    {params[0], params[1], result},
		"output":          {result},
		"result":          {result},
	} {
		got, ok := m.ReflectiveElements(x, feature)
		if !ok || len(got) != len(want) {
			t.Errorf("x.%s: %v (%v), want %d", feature, names(got), ok, len(want))
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("x.%s[%d] is not the synthesized part", feature, i)
			}
		}
	}
	args := m.ExpressionArguments(x)
	if got := names(args); !equal(got, []string{"FeatureReferenceExpression", "OperatorExpression"}) {
		t.Fatalf("x arguments: %v", got)
	}
	if got, ok := m.ReflectiveElements(x, "argument"); !ok || len(got) != 2 || got[0] != args[0] || got[1] != args[1] {
		t.Error("x.argument is not the operands")
	}
	for i := range args {
		if got, ok := m.ReflectiveElements(args[i], "owner"); !ok || len(got) != 1 || got[0] != params[i] {
			t.Errorf("a%d.owner is not in%d", i, i)
		}
		if got := m.ValueExpression(params[i]); got != args[i] {
			t.Errorf("in%d binds another value", i)
		}
		fv := m.OwnedMemberships(params[i])
		if got := names(fv); !equal(got, []string{"FeatureValue"}) {
			t.Errorf("in%d owned memberships: %v", i, got)
			continue
		}
		if got, ok := m.ReflectiveElements(fv[0], "value"); !ok || len(got) != 1 || got[0] != args[i] {
			t.Errorf("in%d FeatureValue.value is not a%d", i, i)
		}
		if got, ok := m.ReflectiveElements(fv[0], "featureWithValue"); !ok || len(got) != 1 || got[0] != params[i] {
			t.Errorf("in%d FeatureValue.featureWithValue is not in%d", i, i)
		}
		if got, ok := m.ReflectiveElements(args[i], "owningMembership"); !ok || len(got) != 1 || got[0] != fv[0] {
			t.Errorf("a%d.owningMembership is not its FeatureValue", i)
		}
	}
	if got, ok := m.ReflectiveElements(params[0], "owner"); !ok || len(got) != 1 || got[0] != x {
		t.Error("in0.owner is not the expression")
	}
	if got, ok := m.ReflectiveElements(result, "owner"); !ok || len(got) != 1 || got[0] != x {
		t.Error("out.owner is not the expression")
	}
	// The nested operand `b * 2` owns parts of its own; its literal owns none.
	nested := args[1]
	if got := names(m.ExpressionParameters(nested)); !equal(got, []string{"Feature", "Feature"}) {
		t.Errorf("b*2 parameters: %v", got)
	}
	if m.ExpressionResult(nested) == nil {
		t.Error("b*2: no result parameter")
	}
	literal := m.ExpressionArguments(nested)[1]
	if m.MetaclassOf(literal).Name != "LiteralInteger" || len(m.ExpressionParameters(literal)) != 0 ||
		m.ExpressionResult(literal) != nil || len(m.OwnedMemberships(literal)) != 0 {
		t.Error("the literal 2 owns parts")
	}
	if got, ok := m.ReflectiveElements(literal, "ownedElement"); !ok || len(got) != 0 {
		t.Errorf("literal ownedElement: %v", names(got))
	}

	// `a as T` owns a parameter for its operand and one for the type
	// argument, unbound, after it; and its result.
	y := m.ValueExpression(member(t, m, d, "y"))
	yParams := m.ExpressionParameters(y)
	if got := names(yParams); !equal(got, []string{"Feature", "Feature"}) {
		t.Fatalf("y parameters: %v", got)
	}
	if got := names(m.ExpressionArguments(y)); !equal(got, []string{"FeatureReferenceExpression"}) {
		t.Errorf("y arguments: %v", got)
	}
	if yParams[1].Expr.Index != 1 || m.ValueExpression(yParams[1]) != nil || len(m.OwnedMemberships(yParams[1])) != 0 {
		t.Error("y's type argument parameter binds a value")
	}
	if got := names(m.OwnedMemberships(y)); !equal(got, []string{"ParameterMembership", "ParameterMembership", "ReturnParameterMembership"}) {
		t.Errorf("y owned memberships: %v", got)
	}

	// A literal value owns nothing.
	z := m.ValueExpression(member(t, m, d, "z"))
	if len(m.ExpressionParameters(z)) != 0 || m.ExpressionResult(z) != nil || len(m.OwnedMemberships(z)) != 0 {
		t.Error("the literal value of z owns parts")
	}
}
