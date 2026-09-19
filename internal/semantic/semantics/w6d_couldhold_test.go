package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// CouldHold decides whether a feature of a type can hold a scalar at all, which
// is what lets a Boolean-valued context report a condition the scalar lattice
// alone leaves unknown.
func TestCouldHoldBoolean(t *testing.T) {
	const src = `package ScalarValues {
		attribute def ScalarValue;
		attribute def Boolean specializes ScalarValue;
		attribute def Number specializes ScalarValue;
		attribute def Integer specializes Number;
	}
	part def Engine;
	attribute def Flag :> ScalarValues::Boolean;
	enum def Level { red; }
	attribute flag : Flag;
	`
	m, root := buildModel(t, src)
	scalars := sym(t, root, "ScalarValues")
	for _, tc := range []struct {
		name string
		sym  *symbols.Symbol
		want bool
	}{
		{"Boolean", sym(t, scalars.Scope, "Boolean"), true},
		{"a specialization of Boolean", sym(t, root, "Flag"), true},
		{"a feature typed by one", sym(t, root, "flag"), true},
		{"a supertype of Boolean", sym(t, scalars.Scope, "ScalarValue"), true},
		{"Integer", sym(t, scalars.Scope, "Integer"), false},
		{"a part definition", sym(t, root, "Engine"), false},
		{"an enumeration", sym(t, root, "Level"), false},
		{"no symbol", nil, true},
	} {
		if got := m.CouldHold(tc.sym, PrimBoolean); got != tc.want {
			t.Errorf("CouldHold(%s, Boolean) = %v, want %v", tc.name, got, tc.want)
		}
	}
	// An unknown target type constrains nothing.
	if !m.CouldHold(sym(t, root, "Engine"), PrimUnknown) {
		t.Error("CouldHold(Engine, unknown) = false, want true")
	}
}
