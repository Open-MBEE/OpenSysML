package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The symbol structure states the effective Feature::isComposite, not the declaration's flag, and
// Usage::isReference negates it (SysML v2.1 §8.3.6.4 deriveUsageIsReference).
func TestSymbolStructureStatesEffectiveIsComposite(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "composite.sysml", `package P {
    part def Engine;
    action def Run;
    part def Car {
        part engine : Engine;
        ref part spare : Engine;
        attribute mass : ScalarValues::Real;
        action run : Run;
        perform action drive : Run;
    }
}`)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	for name, composite := range map[string]bool{
		"P::Car::engine": true,
		"P::Car::spare":  false,
		"P::Car::mass":   false,
		"P::Car::run":    true,
		"P::Car::drive":  false,
	} {
		syms := e.resolver.Index().LookupQualified(name)
		if len(syms) != 1 {
			t.Fatalf("%s resolves to %d symbols", name, len(syms))
		}
		element := structure.ElementFor(syms[0])
		if stated, ok := structure.Attribute(element, "isComposite"); !ok || stated.Kind != BooleanValue || stated.Boolean != composite {
			t.Errorf("%s: structure states isComposite %#v, %t; want %t", name, stated, ok, composite)
		}
		if got, ok := e.Property(element, "Feature", "isComposite"); !ok || got.Kind != BooleanValue || got.Boolean != composite {
			t.Errorf("%s: Feature::isComposite = %#v, %t; want %t", name, got, ok, composite)
		}
		if got, ok := e.Property(element, "Usage", "isReference"); !ok || got.Kind != BooleanValue || got.Boolean == composite {
			t.Errorf("%s: Usage::isReference = %#v, %t; want %t", name, got, ok, !composite)
		}
	}
}
