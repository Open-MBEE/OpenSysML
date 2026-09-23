package source

import (
	"reflect"
	"testing"
)

func TestQualifiedNameSegments(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"A::B", []string{"A", "B"}},
		{"'Sep::Pkg'::x", []string{"Sep::Pkg", "x"}},
		{`'a\'b'::c`, []string{`a\'b`, "c"}},
		{"x", []string{"x"}},
	} {
		got, ok := QualifiedNameSegments(tc.text)
		if !ok {
			t.Errorf("QualifiedNameSegments(%q) failed", tc.text)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("QualifiedNameSegments(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
	if _, ok := QualifiedNameSegments("A::"); ok {
		t.Error("QualifiedNameSegments(\"A::\") succeeded, want a failure")
	}
}

// A typing's references each end in the name a diagram heads the type by; a
// conjugation is kept, and text that is not a reference list stands as it is.
func TestReferenceEndNames(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"Pump", "Pump"},
		{"Plant::Pumps::Pump", "Pump"},
		{"$::ISQ::LengthValue", "LengthValue"},
		{"~Ports::FuelPort", "~FuelPort"},
		{"Plant::Pumps::Pump, ~Ports::FuelPort", "Pump, ~FuelPort"},
		{"'Sep::Pkg'::'x::y'", "'x::y'"},
		{"vehicle.engine.Cylinder", "Cylinder"},
		{"Plant::pump.'fuel in'", "'fuel in'"},
		{"", ""},
		{"T<U>", "T<U>"},
		{"A::", "A::"},
		{"A,B", "A,B"},
	} {
		if got := ReferenceEndNames(tc.text); got != tc.want {
			t.Errorf("ReferenceEndNames(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}
