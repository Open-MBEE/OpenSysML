package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// A `for` visits a sequence in the order the expression that built it produced,
// and a set in its canonical order, numbers ascending.
func TestForElementsOrder(t *testing.T) {
	set := NewSet()
	for _, n := range []int64{30, 4, 100, 4} {
		set.Add(integerValue(n))
	}

	cases := map[string]struct {
		value Value
		want  []int64
	}{
		"a sequence keeps its own order":   {sequenceOf([]Value{integerValue(3), integerValue(1), integerValue(2)}), []int64{3, 1, 2}},
		"an empty sequence visits nothing": {sequenceOf(nil), nil},
		"a set visits canonically":         {NewSetValue(set), []int64{4, 30, 100}},
		"an empty set visits nothing":      {NewSetValue(NewSet()), nil},
		"null visits nothing":              {nullValue(), nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			elements, err := forElements(tc.value)
			if err != nil {
				t.Fatalf("forElements: %v", err)
			}
			got := intsOf(t, sequenceOf(elements))
			if !equalInts(got, tc.want) {
				t.Errorf("visited %v, want %v", got, tc.want)
			}
		})
	}
}

// A `for` input that is not a collection is reported with the kind it has, and
// is never read as the one-element collection elementsOf coerces it to.
func TestForElementsRejectsANonCollection(t *testing.T) {
	cases := map[string]Value{
		"an integer":       integerValue(7),
		"a real":           Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 2.5}},
		"a boolean":        boolValue(true),
		"a string":         NewStringValue("xs"),
		"an expression":    Value{Kind: ValExpr},
		"an invalid value": Value{Kind: ValInvalid},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			elements, err := forElements(value)
			if err == nil {
				t.Fatalf("forElements visited %v, want a typed error", elements)
			}
			if !errors.Is(err, ErrTypeMismatch) {
				t.Errorf("forElements failed with %v, want ErrTypeMismatch", err)
			}
		})
	}
}

// elementsOf keeps coercing a scalar to the one-element collection KerML reads
// it as: only `for` is strict, so the general readers are unchanged.
func TestElementsOfStillCoercesAScalar(t *testing.T) {
	if got := len(elementsOf(integerValue(7))); got != 1 {
		t.Errorf("elementsOf(7) holds %d elements, want 1", got)
	}
}
