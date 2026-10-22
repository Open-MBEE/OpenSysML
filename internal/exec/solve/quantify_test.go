package solve

import (
	"errors"
	"strings"
	"testing"
)

// quantifiedQuery translates a constraint of the quantified-variants fixture.
func quantifiedQuery(t *testing.T, element string) (*Query, error) {
	t.Helper()
	ctx, idx := fixtureFile(t, "quantified_variants.sysml")
	sym := symbolNamed(t, idx, "test::carFamily::"+element)
	return Constraint(ctx, sym, sym.OwnerScope)
}

// TestQuantifierOverAVariationPointIsItsSelection: `->forAll` and `->exists`
// over a variation point holding at least one object compare the point's
// selection with the variant, so the query reads the point as a variation.
func TestQuantifierOverAVariationPointIsItsSelection(t *testing.T) {
	for _, element := range []string{"allNarrow", "someWide", "noneWide"} {
		q, err := quantifiedQuery(t, element)
		if err != nil {
			t.Fatalf("%s: %v", element, err)
		}
		names := []string{}
		for _, v := range q.Variations() {
			names = append(names, v.Name)
		}
		if got := strings.Join(names, ","); got != "test::carFamily::engineChoice,test::carFamily::wheelChoice" {
			t.Errorf("%s reads %s, want the engine and the wheel choice", element, got)
		}
	}
}

// TestQuantifierRefusedWithItsReason: a point that may hold no object says
// nothing about its selection, a body comparing the iterate with another
// point's variant is not a selection, and any other invocation stays refused.
func TestQuantifierRefusedWithItsReason(t *testing.T) {
	cases := map[string]string{
		"spareNarrow":   "spareChoice may hold no object, so the quantifier says nothing about the selection",
		"wheelIsEngine": "not a variant of wheelChoice",
		"twoWheels":     "invocation",
	}
	for element, reason := range cases {
		_, err := quantifiedQuery(t, element)
		if !errors.Is(err, ErrNotTranslatable) {
			t.Fatalf("%s: error %v, want a refusal", element, err)
		}
		if !strings.Contains(err.Error(), reason) {
			t.Errorf("%s: %v, want %q in the reason", element, err, reason)
		}
	}
}
