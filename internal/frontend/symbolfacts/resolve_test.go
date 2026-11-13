package symbolfacts

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const resolveModel = `
package Vis {
	package Lib {
		part def Pump;
		alias PumpAlias for Pump;
		package Sub { part def Valve; }
		private part def Secret { part inner; part; }
		part held : Secret;
	}
	package Facade { public import Lib::Pump; }
	package FacadeDeep { public import Lib::**; }
	package Hidden { private import Lib::*; }
	part def Base { protected part guarded; }
}
`

func fqns(idx *symbols.Index, syms []*symbols.Symbol) string {
	out := ""
	for _, sym := range syms {
		out += idx.GetFQN(sym) + ";"
	}
	return out
}

// A name is resolved through visible memberships to the element it names, and
// an id the service reported denotes its element whatever its visibility.
func TestResolveAndLookupTellANameFromAReportedID(t *testing.T) {
	idx, sc := modelWithLibrary(t, resolveModel)
	cases := []struct{ id, resolve, member, lookup string }{
		{"Vis::Lib::Pump", "Vis::Lib::Pump;", "Vis::Lib::Pump;", "Vis::Lib::Pump;"},
		{"Vis::Lib::PumpAlias", "Vis::Lib::Pump;", "Vis::Lib::PumpAlias;", "Vis::Lib::PumpAlias;"},
		{"Vis::Facade::Pump", "Vis::Lib::Pump;", "Vis::Lib::Pump;", "Vis::Lib::Pump;"},
		{"Vis::FacadeDeep::Valve", "Vis::Lib::Sub::Valve;", "Vis::Lib::Sub::Valve;", "Vis::Lib::Sub::Valve;"},
		{"Vis::Hidden::Pump", "", "", ""},
		{"Vis::Lib::Secret", "", "", "Vis::Lib::Secret;"},
		{"Vis::Base::guarded", "", "", "Vis::Base::guarded;"},
		{"Vis::Lib::Secret::inner", "Vis::Lib::Secret::inner;", "Vis::Lib::Secret::inner;", "Vis::Lib::Secret::inner;"},
		{"ScalarValues::Real", "ScalarValues::Real;", "ScalarValues::Real;", "ScalarValues::Real;"},
		{"Vis::Nope", "", "", ""},
	}
	for _, tc := range cases {
		if got := fqns(idx, sc.Resolve(tc.id)); got != tc.resolve {
			t.Errorf("Resolve(%s) = %q, want %q", tc.id, got, tc.resolve)
		}
		if got := fqns(idx, sc.ResolveMember(tc.id)); got != tc.member {
			t.Errorf("ResolveMember(%s) = %q, want %q", tc.id, got, tc.member)
		}
		if got := fqns(idx, sc.Lookup(tc.id)); got != tc.lookup {
			t.Errorf("Lookup(%s) = %q, want %q", tc.id, got, tc.lookup)
		}
	}
}

// The positional id of an unnamed member, which no name reaches, denotes it
// under either reading.
func TestResolveKeepsPositionalIDs(t *testing.T) {
	idx, sc := modelWithLibrary(t, resolveModel)
	secret := LookupNamed(idx, "Vis::Lib::Secret")
	if len(secret) != 1 {
		t.Fatalf("Vis::Lib::Secret answers %d symbols", len(secret))
	}
	var unnamed *symbols.Symbol
	for _, child := range secret[0].Scope.AllMembers() {
		if child.Name == "" {
			unnamed = child
		}
	}
	if unnamed == nil {
		t.Fatal("the unnamed part is not indexed")
	}
	id := sc.Identity(unnamed)
	for name, got := range map[string][]*symbols.Symbol{"Resolve": sc.Resolve(id), "Lookup": sc.Lookup(id)} {
		if len(got) != 1 || got[0] != unnamed {
			t.Errorf("%s(%q) = %v, want the unnamed part", name, id, got)
		}
	}
}

// A filtered import re-exports only the members its filter keeps (KerML 8.2.3.5), and the
// declared name of a public member of a private namespace still denotes that member.
func TestResolveHonoursImportFiltersAndDeclaredNames(t *testing.T) {
	idx, sc := modelWithLibrary(t, `
package F {
	metadata def Safety;
	package Lib {
		part def Safe { @Safety; }
		part def Plain;
		private part def Secret { part inner; alias innerAlias for inner; }
	}
	package Only { public import Lib::*[@Safety]; }
	package Deep { public import Lib::**[@Safety]; }
}
`)
	cases := []struct{ id, resolve, member string }{
		{"F::Only::Safe", "F::Lib::Safe;", "F::Lib::Safe;"},
		{"F::Only::Plain", "", ""},
		{"F::Deep::Safe", "F::Lib::Safe;", "F::Lib::Safe;"},
		{"F::Deep::Plain", "", ""},
		{"F::Lib::Secret::inner", "F::Lib::Secret::inner;", "F::Lib::Secret::inner;"},
		{"F::Lib::Secret::innerAlias", "F::Lib::Secret::inner;", "F::Lib::Secret::innerAlias;"},
	}
	for _, tc := range cases {
		if got := fqns(idx, sc.Resolve(tc.id)); got != tc.resolve {
			t.Errorf("Resolve(%s) = %q, want %q", tc.id, got, tc.resolve)
		}
		if got := fqns(idx, sc.ResolveMember(tc.id)); got != tc.member {
			t.Errorf("ResolveMember(%s) = %q, want %q", tc.id, got, tc.member)
		}
	}
}
