package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func TestFeatureReadOnly(t *testing.T) {
	m, root := buildModel(t, `package P {
	part def Base {
		constant attribute k;
		derived attribute d;
		attribute w;
	}
	part def Sub :> Base {
		attribute :>> k;
		attribute :>> d;
		attribute :>> w;
		attribute s :> k;
		attribute ds :> d;
	}
	action def Gen { in constant attribute limit; }
	action def Spec :> Gen { in attribute cap; }
	part def Cyc {
		attribute a :> b;
		attribute b :> a;
	}
}`)
	tests := []struct {
		path     string
		kind     ReadOnlyKind
		declared string
	}{
		{"P::Base::k", ReadOnlyConstant, "P::Base::k"},
		{"P::Base::d", ReadOnlyDerived, "P::Base::d"},
		{"P::Base::w", Writable, ""},
		{"P::Sub::k", ReadOnlyConstant, "P::Base::k"},
		{"P::Sub::d", ReadOnlyDerived, "P::Base::d"},
		{"P::Sub::w", Writable, ""},
		{"P::Sub::s", ReadOnlyConstant, "P::Base::k"},
		// Subsetting a derived feature makes the subsetting one's values a subset
		// of them, not values the model determines, so it stays writable.
		{"P::Sub::ds", Writable, ""},
		// A parameter implicitly redefines the general behavior's one at its position.
		{"P::Spec::cap", ReadOnlyConstant, "P::Gen::limit"},
		{"P::Cyc::a", Writable, ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := m.FeatureReadOnly(nestedSym(t, root, tt.path))
			if got.Kind != tt.kind {
				t.Fatalf("kind = %v, want %v", got.Kind, tt.kind)
			}
			if tt.declared == "" {
				if got.Declared != nil {
					t.Fatalf("declared = %v, want none", got.Declared.Name)
				}
				return
			}
			if got.Declared != nestedSym(t, root, tt.declared) {
				t.Fatalf("declared by %v, want %s", got.Declared, tt.declared)
			}
		})
	}
}

// An end usage that may vary in time is implicitly constant (KerML 1.0
// §8.3.3.3.4 validateFeatureEndIsConstant; SysML 2.0 §8.4.2.2): no `constant`
// is written, yet the one derivation both reflection and FeatureReadOnly read
// answers constant.
func TestFeatureReadOnlyImplicitlyConstantEnds(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
	part def Thing;
	part def Holder {
		end part e : Thing;
		part notEnd : Thing;
	}
	connection def Link { end part a : Thing; }
	attribute def A { end part x; }
}`)

	reflective := func(t *testing.T, sym *symbols.Symbol, feature string) bool {
		t.Helper()
		got, ok := m.ReflectiveFeatureValue(sym, feature)
		if !ok {
			t.Fatalf("%s.%s is underived", symbols.FQNOf(sym), feature)
		}
		return got.Bool
	}
	check := func(t *testing.T, sym *symbols.Symbol, kind ReadOnlyKind, declared *symbols.Symbol, implied, isConstant, isVariable bool) {
		t.Helper()
		ro := m.FeatureReadOnly(sym)
		if ro.Kind != kind {
			t.Errorf("%s read-only kind = %v, want %v", symbols.FQNOf(sym), ro.Kind, kind)
		}
		if ro.Declared != declared {
			var got string
			if ro.Declared != nil {
				got = symbols.FQNOf(ro.Declared)
			}
			var want string
			if declared != nil {
				want = symbols.FQNOf(declared)
			}
			t.Errorf("%s read-only declared = %s, want %s", symbols.FQNOf(sym), got, want)
		}
		if ro.ImpliedByEnd != implied {
			t.Errorf("%s ImpliedByEnd = %t, want %t", symbols.FQNOf(sym), ro.ImpliedByEnd, implied)
		}
		if got := reflective(t, sym, "isConstant"); got != isConstant {
			t.Errorf("%s reflective isConstant = %t, want %t", symbols.FQNOf(sym), got, isConstant)
		}
		if got := reflective(t, sym, "isVariable"); got != isVariable {
			t.Errorf("%s reflective isVariable = %t, want %t", symbols.FQNOf(sym), got, isVariable)
		}
		// None of these features inherits: own isConstant and the read-only
		// verdict are the one derivation's two readings.
		if constant := ro.Kind == ReadOnlyConstant; constant != isConstant {
			t.Errorf("%s read-only constant = %t, reflective isConstant = %t", symbols.FQNOf(sym), constant, isConstant)
		}
	}
	library := func(t *testing.T, fqn string) *symbols.Symbol {
		t.Helper()
		matches := m.resolver.Index().LookupQualified(fqn)
		if len(matches) != 1 {
			t.Fatalf("%s: %d matching symbols, want 1", fqn, len(matches))
		}
		return matches[0]
	}

	e := nestedSym(t, root, "P::Holder::e")
	notEnd := nestedSym(t, root, "P::Holder::notEnd")
	a := nestedSym(t, root, "P::Link::a")
	x := nestedSym(t, root, "P::A::x")
	check(t, e, ReadOnlyConstant, e, true, true, true)
	check(t, notEnd, Writable, nil, false, false, true)
	check(t, a, ReadOnlyConstant, a, true, true, true)
	check(t, x, Writable, nil, false, false, false)
	for _, fqn := range []string{
		"Connections::BinaryConnection::source", "Connections::BinaryConnection::target",
		"Allocations::Allocation::source", "Allocations::Allocation::target",
	} {
		sym := library(t, fqn)
		check(t, sym, ReadOnlyConstant, sym, true, true, true)
	}

	if got, want := ReadOnlyViolation("e", e, m.FeatureReadOnly(e)),
		"e is constant as an end feature: its value does not change over the lifetime of its featuring occurrence"; got != want {
		t.Errorf("violation text = %q, want %q", got, want)
	}
}
