package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func specializesFQN(m *Model, sym *symbols.Symbol, fqn string) bool {
	for _, general := range append(m.ImplicitBases(sym), m.DirectSupertypes(sym)...) {
		if symbols.FQNOf(general) == fqn {
			return true
		}
	}
	return false
}

// The expression takes Performances::Evaluation through its implied base (KerML 1.1
// §8.4.4.7); the parameters synthesized over its node are plain features and do not.
func TestExpressionImpliedBaseIsNotGivenToItsParameters(t *testing.T) {
	m, root := buildModelWithStdlibNamedKind(t, "t.sysml", source.KindSysML, `package P {
	attribute x = 1 + 2;
}`)
	pkg, _ := root.LookupLocal("P")
	x, _ := pkg.Scope.LookupLocal("x")
	var value *symbols.Symbol
	for _, membership := range m.OwnedMemberships(x) {
		if m.implicitMetaclassName(membership) == "FeatureValue" {
			value = membership.Implicit.Member
		}
	}
	if value == nil {
		t.Fatal("x owns no FeatureValue")
	}
	if !specializesFQN(m, value, "Performances::Evaluation") {
		var names []string
		for _, general := range append(m.ImplicitBases(value), m.DirectSupertypes(value)...) {
			names = append(names, symbols.FQNOf(general))
		}
		t.Errorf("1 + 2 specializes %v, want Performances::Evaluation among them", names)
	}
	parameters := append(m.ExpressionParameters(value), m.ExpressionResult(value))
	if len(parameters) != 3 {
		t.Fatalf("1 + 2 has %d parameters, want two inputs and the result", len(parameters))
	}
	for _, parameter := range parameters {
		if parameter == nil {
			t.Fatal("nil parameter")
		}
		if specializesFQN(m, parameter, "Performances::Evaluation") {
			t.Errorf("parameter %d (role %v) of 1 + 2 specializes Performances::Evaluation", parameter.Expr.Index, parameter.Expr.Role)
		}
	}
}
