package symbolfacts

import (
	"testing"
)

// compositionModel pairs each composite usage with its reference counterpart:
// `part`/`ref part`, `item`/`ref item`, an attribute, a port owned by a part
// (referential) and one nested in a port, and an action's directed parameters.
const compositionModel = `
package RefParts {
	part def Engine;
	part def Person;
	part def Car {
		part engine : Engine;
		ref part driver : Person;
		item cargo;
		ref item manifest;
		attribute mass : ScalarValues::Real;
		port p { port inner; }
		action drive { in speed : ScalarValues::Real; out odometer : ScalarValues::Real; }
		connection link connect engine to driver;
	}
}
`

func boolFact(t *testing.T, fact *bool, want bool, what string) {
	t.Helper()
	if fact == nil {
		t.Errorf("%s is unset, want %t", what, want)
		return
	}
	if *fact != want {
		t.Errorf("%s = %t, want %t", what, *fact, want)
	}
}

// Of reports Feature::isComposite and Usage::isReference as the semantic model
// derives them, as typed facts and as the metadata flags `composite` and
// `reference` (each present only when it holds, as `abstract` is), so a client
// tells `part` from `ref part` without the notation.
func TestOfReportsCompositeAndReference(t *testing.T) {
	idx, sc := modelWithLibrary(t, compositionModel)
	cases := map[string]bool{
		"RefParts::Car::engine":          true,
		"RefParts::Car::driver":          false,
		"RefParts::Car::cargo":           true,
		"RefParts::Car::manifest":        false,
		"RefParts::Car::mass":            false,
		"RefParts::Car::p":               false,
		"RefParts::Car::p::inner":        true,
		"RefParts::Car::drive":           true,
		"RefParts::Car::drive::speed":    false,
		"RefParts::Car::drive::odometer": false,
		"RefParts::Car::link":            true,
	}
	for fqn, composite := range cases {
		syms := idx.LookupQualified(fqn)
		if len(syms) != 1 {
			t.Fatalf("lookup %s: %d symbols", fqn, len(syms))
		}
		info := Of(syms[0], sc)
		boolFact(t, info.IsComposite, composite, fqn+" IsComposite")
		boolFact(t, info.IsReference, !composite, fqn+" IsReference")
		if got := info.Metadata["composite"]; got != flag(composite) {
			t.Errorf("%s metadata composite = %q, want %q", fqn, got, flag(composite))
		}
		if got := info.Metadata["reference"]; got != flag(!composite) {
			t.Errorf("%s metadata reference = %q, want %q", fqn, got, flag(!composite))
		}
	}
}

// A definition is not a feature: both facts stay unset and neither metadata
// flag is written.
func TestOfLeavesCompositionUnsetForADefinition(t *testing.T) {
	idx, sc := modelWithLibrary(t, compositionModel)
	info := Of(idx.LookupQualified("RefParts::Car")[0], sc)
	if info.IsComposite != nil || info.IsReference != nil {
		t.Errorf("RefParts::Car composite/reference = %v/%v, want unset", info.IsComposite, info.IsReference)
	}
	for _, key := range []string{"composite", "reference"} {
		if value, ok := info.Metadata[key]; ok {
			t.Errorf("RefParts::Car metadata %s = %q, want absent", key, value)
		}
	}
}

// flag is the metadata value of a flag that holds, or its absence.
func flag(holds bool) string {
	if holds {
		return "true"
	}
	return ""
}
