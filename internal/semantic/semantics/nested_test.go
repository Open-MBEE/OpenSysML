package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// buildModelWithStdlib resolves src against the standard-library index, so the
// features ImplicitSubsettings names resolve to real library elements.
func buildModelWithStdlib(t *testing.T, src string) (*Model, *symbols.Scope) {
	t.Helper()
	idx := stdlibIndex(t)
	name := "t.sysml"
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.ResolveDocument(name, root)
	return m, idx.DocumentRoot(name)
}

// nestedSym resolves the symbol at a `::`-separated path under the root scope.
func nestedSym(t *testing.T, root *symbols.Scope, path string) *symbols.Symbol {
	t.Helper()
	var sym *symbols.Symbol
	scope := root
	for _, part := range splitPath(path) {
		s, ok := scope.LookupLocal(part)
		if !ok {
			t.Fatalf("symbol %q not found along %q", part, path)
		}
		sym = s
		scope = s.Scope
	}
	return sym
}

func splitPath(path string) []string {
	var out []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == ':' {
			out = append(out, path[start:i])
			i++
			start = i + 1
		}
	}
	return append(out, path[start:])
}

func TestImplicitSubsettings(t *testing.T) {
	src := `package P {
		part def Wheel;
		part vehicle {
			part wheels : Wheel[4];
			ref part spare : Wheel;
			part def NestedDef;
			item cargo;
			port fuelIn;
			port def FuelPort { port inner; }
			port outer : FuelPort;
			action drive { action steer; }
			perform action cruise;
			state engineState { state idle; }
			exhibit state parked;
			calc compute { calc inner; }
			requirement r1 { requirement r2; constraint c; }
			constraint c0;
			interface link connect fuelIn to outer;
			occurrence trip;
			snapshot occurrence snap;
			variant part optional : Wheel;
		}
		part atTop;
	}`
	m, root := buildModelWithStdlib(t, src)

	cases := []struct {
		path string
		want string
	}{
		{"P::vehicle::wheels", "Items::Item::subparts"},
		{"P::vehicle::spare", ""}, // a reference subsets nothing implicitly
		{"P::atTop", ""},          // a package is no owner type
		{"P::vehicle::cargo", "Items::Item::subitems"},
		{"P::vehicle::fuelIn", "Parts::Part::ownedPorts"},
		{"P::vehicle::drive::steer", "Actions::Action::subactions"},
		{"P::vehicle::drive", "Parts::Part::ownedActions"},
		{"P::vehicle::cruise", "Parts::Part::performedActions"},
		{"P::vehicle::engineState::idle", "States::StateAction::substates"},
		{"P::vehicle::engineState", "Parts::Part::ownedStates"},
		{"P::vehicle::parked", "Parts::Part::exhibitedStates"},
		{"P::vehicle::compute", "Parts::Part::ownedActions"},
		{"P::vehicle::compute::inner", "Calculations::Calculation::subcalculations"},
		{"P::vehicle::r1::r2", "Requirements::RequirementCheck::subrequirements"},
		{"P::vehicle::r1::c", "Occurrences::Occurrence::suboccurrences"},
		{"P::vehicle::c0", "Items::Item::checkedConstraints"},
		{"P::vehicle::link", "Items::Item::subparts"},
		{"P::vehicle::trip", "Occurrences::Occurrence::suboccurrences"},
		{"P::vehicle::snap", "Occurrences::Occurrence::snapshots"},
		{"P::vehicle::optional", ""}, // a variant subsets nothing implicitly
	}
	for _, tc := range cases {
		sym := nestedSym(t, root, tc.path)
		got := m.ImplicitSubsettings(sym)
		if tc.want == "" {
			if len(got) != 0 {
				t.Errorf("ImplicitSubsettings(%s) = %v, want none", tc.path, got)
			}
			continue
		}
		if len(got) != 1 || symbols.FQNOf(got[0]) != tc.want {
			t.Errorf("ImplicitSubsettings(%s) = %v, want [%s]", tc.path, fqns(got), tc.want)
		}
	}
}

func fqns(syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, s := range syms {
		out = append(out, symbols.FQNOf(s))
	}
	return out
}

// TestImplicitSubsettingsPortInPort covers `Ports::Port::subports`: a composite
// port nested in a port subsets it.
func TestImplicitSubsettingsPortInPort(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		port def FuelPort { port inner; }
	}`)
	got := m.ImplicitSubsettings(nestedSym(t, root, "P::FuelPort::inner"))
	if len(got) != 1 || symbols.FQNOf(got[0]) != "Ports::Port::subports" {
		t.Fatalf("ImplicitSubsettings(inner) = %v, want [Ports::Port::subports]", fqns(got))
	}
}

// TestImplicitSubsettingsParameters covers the `in item` parameter of an item:
// a parameter is not a subitem.
func TestImplicitSubsettingsParameters(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		action def Drive { in item fuel; }
	}`)
	got := m.ImplicitSubsettings(nestedSym(t, root, "P::Drive::fuel"))
	if len(got) != 0 {
		t.Fatalf("ImplicitSubsettings(fuel) = %v, want none for a parameter", fqns(got))
	}
}

// TestImplicitSubsettingsTimeslice covers the portion rule: a timeslice in an
// occurrence subsets its `timeSlices`.
func TestImplicitSubsettingsTimeslice(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		occurrence def Trip { timeslice occurrence mid; }
	}`)
	got := m.ImplicitSubsettings(nestedSym(t, root, "P::Trip::mid"))
	if len(got) != 1 || symbols.FQNOf(got[0]) != "Occurrences::Occurrence::timeSlices" {
		t.Fatalf("ImplicitSubsettings(mid) = %v, want [Occurrences::Occurrence::timeSlices]", fqns(got))
	}
}

// TestReflectiveOwnedUsagesRejects covers the two shapes the derivation refuses:
// a suffix no metaclass bears, and `owned*` asked of a usage.
func TestReflectiveOwnedUsagesRejects(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		part def Vehicle { part wheel; }
		part car : Vehicle { part seat; }
	}`)
	def := nestedSym(t, root, "P::Vehicle")
	usage := nestedSym(t, root, "P::car")

	if _, ok := m.reflectiveOwnedUsages(usage, "nestedFoo"); ok {
		t.Errorf("nestedFoo derived, want unsupported")
	}
	if _, ok := m.reflectiveOwnedUsages(usage, "ownedPart"); ok {
		t.Errorf("ownedPart on a usage derived, want unsupported")
	}
	if _, ok := m.reflectiveOwnedUsages(def, "nestedPart"); ok {
		t.Errorf("nestedPart on a definition derived, want unsupported")
	}
	elems, ok := m.reflectiveOwnedUsages(usage, "nestedPart")
	if !ok || len(elems) != 1 || elems[0].Name != "seat" {
		t.Errorf("nestedPart on car = %v, %v, want [seat]", fqns(elems), ok)
	}
	elems, ok = m.reflectiveOwnedUsages(def, "ownedPart")
	if !ok || len(elems) != 1 || elems[0].Name != "wheel" {
		t.Errorf("ownedPart on Vehicle = %v, %v, want [wheel]", fqns(elems), ok)
	}
}
