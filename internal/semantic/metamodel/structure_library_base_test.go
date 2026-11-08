package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A feature typed by a library type inherits that type's memberships through the
// symbol structure, as one typed by a type of its own model does.
func TestLibraryTypedFeatureInheritsItsTypesMemberships(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "lib.sysml", `package P {
	private import ScalarValues::*;
	metadata def Safety { attribute isMandatory : Boolean; }
}`)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	syms := e.resolver.Index().LookupQualified("P::Safety::isMandatory")
	if len(syms) == 0 {
		t.Fatal("no P::Safety::isMandatory")
	}
	feature := structure.ElementFor(syms[0])
	for _, property := range []string{"inheritedMembership", "membership"} {
		metaclass := "Type"
		if property == "membership" {
			metaclass = "Namespace"
		}
		value, ok := e.Property(feature, metaclass, property)
		if !ok || !value.Success || len(value.Values) == 0 {
			t.Errorf("%s::%s = %v, %v: want the memberships Boolean and Base::dataValues give it", metaclass, property, value.Success, ok)
		}
	}
}
