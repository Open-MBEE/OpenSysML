package resolve_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// visibilityModel declares one member behind each kind of membership a
// qualified name written from the root namespace can cross.
const visibilityModel = `package M {
	attribute def Real;
	package Lib {
		part def Pump;
		attribute rate : Real;
		alias PumpAlias for Pump;
		alias rateAlias for rate;
		package Sub { part def Valve; attribute volume : Real; }
		private part def SecretDef { part inner; }
		private attribute secretRate : Real;
		part pSecret : SecretDef;
		attribute aSecret :> secretRate;
	}
	package Facade { public import Lib::Pump; public import Lib::rate; }
	package FacadeAll { public import Lib::*; }
	package FacadeDeep { public import Lib::**; }
	package Chain { public import FacadeAll::*; }
	package Hidden { private import Lib::**; part pUse : Valve; attribute aUse :> rate; }
	package HiddenOne { private import Lib::Pump; part pOne : HiddenOne::Pump; }
	package Guarded { protected import Lib::Pump; }
	package GuardedAll { protected import Lib::*; }
	part def Base { protected part def ProtDef; protected attribute protRate : Real; part open; }
	part def Derived :> Base { part pUse : ProtDef; attribute aUse :> protRate; }
	package Outside {
		part bad : Lib::SecretDef; part worse : Base::ProtDef;
		part deep : Hidden::Valve; part one : Hidden::rate; part guarded : Guarded::Pump; part all : GuardedAll::Pump;
	}
}`

// A qualified name resolves from the root through visible memberships (KerML 8.2.3.5):
// public, recursive and chained imports and aliases are crossed, private and protected are not.
func TestResolveGlobalFollowsVisibleMemberships(t *testing.T) {
	r, _, _ := resolvedDoc(t, visibilityModel)
	cases := []struct{ name, want string }{
		{"M::Lib::Pump", "M::Lib::Pump"},
		{"M::Lib::PumpAlias", "M::Lib::Pump"},
		{"M::Lib::rateAlias", "M::Lib::rate"},
		{"M::Facade::Pump", "M::Lib::Pump"},
		{"M::Facade::rate", "M::Lib::rate"},
		{"M::FacadeAll::Pump", "M::Lib::Pump"},
		{"M::FacadeAll::rate", "M::Lib::rate"},
		{"M::FacadeAll::PumpAlias", "M::Lib::Pump"},
		{"M::FacadeAll::Sub::Valve", "M::Lib::Sub::Valve"},
		{"M::FacadeDeep::Valve", "M::Lib::Sub::Valve"},
		{"M::FacadeDeep::volume", "M::Lib::Sub::volume"},
		{"M::Chain::Pump", "M::Lib::Pump"},
		{"M::Derived::open", "M::Base::open"},
		{"M::Facade::Valve", ""},
		{"M::FacadeAll::Valve", ""},
		{"M::Hidden::Pump", ""},
		{"M::Hidden::rate", ""},
		{"M::Hidden::Valve", ""},
		{"M::HiddenOne::Pump", ""},
		{"M::Guarded::Pump", ""},
		{"M::GuardedAll::Pump", ""},
		{"M::Lib::SecretDef", ""},
		{"M::Lib::SecretDef::inner", ""},
		{"M::Lib::secretRate", ""},
		{"M::FacadeAll::SecretDef", ""},
		{"M::FacadeDeep::secretRate", ""},
		{"M::Base::ProtDef", ""},
		{"M::Base::protRate", ""},
		{"M::Derived::ProtDef", ""},
		{"M::Lib::Missing", ""},
		{"Missing::Pump", ""},
		{"M::Lib::", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := ""
		if syms := r.ResolveGlobal(strings.Split(tc.name, "::")); len(syms) == 1 {
			got = symbols.FQNOf(syms[0])
		} else if len(syms) > 1 {
			t.Errorf("%s resolves to %d elements", tc.name, len(syms))
		}
		if got != tc.want {
			t.Errorf("%s resolves to %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The member a name's last segment names is the alias membership where the
// element is not asked for.
func TestResolveGlobalMembersKeepsTheAlias(t *testing.T) {
	r, _, _ := resolvedDoc(t, visibilityModel)
	syms := r.ResolveGlobalMembers([]string{"M", "Lib", "PumpAlias"})
	if len(syms) != 1 || syms[0].Kind != symbols.SymbolAlias || symbols.FQNOf(syms[0]) != "M::Lib::PumpAlias" {
		t.Fatalf("M::Lib::PumpAlias answers %v, want the alias membership", syms)
	}
	if syms = r.ResolveGlobalMembers([]string{"M", "Lib", "PumpAlias", "x"}); len(syms) != 0 {
		t.Errorf("a member of the aliased element that is not declared answers %v", syms)
	}
}

// A reference inside the namespace still names its private members, one in a
// specialization the protected ones, and one outside neither.
func TestResolveGlobalLeavesReferencesAlone(t *testing.T) {
	r, root, scope := resolvedDoc(t, visibilityModel)
	_ = r.ResolveGlobal([]string{"M", "Lib", "SecretDef"})
	for ref, want := range map[string]string{
		"SecretDef":        "M::Lib::SecretDef",
		"secretRate":       "M::Lib::secretRate",
		"ProtDef":          "M::Base::ProtDef",
		"Valve":            "M::Lib::Sub::Valve",
		"protRate":         "M::Base::protRate",
		"HiddenOne::Pump":  "M::Lib::Pump",
		"Lib::SecretDef":   "",
		"Base::ProtDef":    "",
		"Hidden::Valve":    "",
		"Hidden::rate":     "",
		"Guarded::Pump":    "",
		"GuardedAll::Pump": "",
	} {
		if got := bindingOf(t, r, root, scope, ref); got != want {
			t.Errorf("%s binds %q, want %q", ref, got, want)
		}
	}
	if _, errors := diagnosticsOf(r); len(errors) != 6 {
		t.Errorf("got errors %v, want the six references from outside unresolved", errors)
	}
}

// A protected import is read by the namespace that declares it and by a
// specialization of it, by simple name and qualified through the general.
func TestProtectedImportReachesSpecializations(t *testing.T) {
	r, root, scope := resolvedDoc(t, `package M {
	package Lib { part def Pump; part def Valve; }
	part def General { protected import Lib::*; part own : Pump; }
	part def Special :> General { part simple : Valve; part qualified : General::Pump; }
	part def One { protected import Lib::Pump; }
	part def Other :> One { part named : One::Pump; }
}`)
	for ref, want := range map[string]string{
		"Pump":          "M::Lib::Pump",
		"Valve":         "M::Lib::Valve",
		"General::Pump": "M::Lib::Pump",
		"One::Pump":     "M::Lib::Pump",
	} {
		if got := bindingOf(t, r, root, scope, ref); got != want {
			t.Errorf("%s binds %q, want %q", ref, got, want)
		}
	}
	if _, errors := diagnosticsOf(r); len(errors) != 0 {
		t.Errorf("got errors %v, want none", errors)
	}
	for _, name := range [][]string{{"M", "General", "Pump"}, {"M", "One", "Pump"}} {
		if syms := r.ResolveGlobal(name); len(syms) != 0 {
			t.Errorf("%v resolves from the root to %v, want nothing", name, syms)
		}
	}
}
