package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A feature typed by a library type inherits that type's memberships through the symbol
// structure — ones its stated type owns, which no implied base (Base::dataValues) supplies.
func TestLibraryTypedFeatureInheritsItsTypesMemberships(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "lib.sysml", `package P {
	metadata def Reading { attribute q : Quantities::ScalarQuantityValue; }
}`)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	syms := e.resolver.Index().LookupQualified("P::Reading::q")
	if len(syms) == 0 {
		t.Fatal("no P::Reading::q")
	}
	feature := structure.ElementFor(syms[0])
	for _, read := range []struct{ metaclass, property string }{
		{"Type", "inheritedMembership"},
		{"Namespace", "membership"},
	} {
		value, ok := e.Property(feature, read.metaclass, read.property)
		if !ok || !value.Success {
			t.Errorf("%s::%s = %v, %v: want it derived", read.metaclass, read.property, value.Success, ok)
			continue
		}
		var stated bool
		for _, v := range value.Values {
			if m := v.Membership; m.Owner != nil && m.Member != nil &&
				symbols.FQNOf(m.Owner) == "Quantities::ScalarQuantityValue" && m.Member.Name == "mRef" {
				stated = true
			}
		}
		if !stated {
			t.Errorf("%s::%s lacks Quantities::ScalarQuantityValue::mRef, the stated type's own feature", read.metaclass, read.property)
		}
	}
}
