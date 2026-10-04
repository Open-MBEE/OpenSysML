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

func buildModelWithStdlibNamedKind(t *testing.T, name string, kind source.Kind, src string) (*Model, *symbols.Scope) {
	t.Helper()
	idx := stdlibIndex(t)
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocumentWithKind(name, root, kind)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
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

// A recorded (frozen-library) symbol declares no AST node, yet `owned*` still
// derives from it: the prefix is its recorded declaration kind.
func TestReflectiveOwnedUsagesOnARecordedDefinition(t *testing.T) {
	src := symbols.NewIndex()
	addTestDoc(t, src, "lib.sysml", `part def Vehicle { port p; part wheel; }
		package SysML { package Systems { part def PortUsage; part def PartUsage; } }`)
	rec, err := symbols.RecordScope(src.DocumentRoot("lib.sysml"), func(*symbols.Symbol) bool { return true }, func(sym *symbols.Symbol) symbols.LibraryFacts {
		return symbols.LibraryFacts{Node: symbols.NodeKindOf(sym.Decl)}
	})
	if err != nil {
		t.Fatalf("RecordScope: %v", err)
	}
	recorded, err := symbols.BuildRecorded(rec, "lib.sysml")
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	idx := symbols.NewIndex()
	idx.AddRecordedDocument("lib.sysml", source.KindSysML, recorded, nil)
	res := resolve.New(idx)
	m := NewModel(res)
	res.SetModel(m)
	vehicle := sym(t, idx.DocumentRoot("lib.sysml"), "Vehicle")
	if !vehicle.Recorded() {
		t.Fatal("vehicle is not a recorded symbol")
	}

	elems, ok := m.reflectiveOwnedUsages(vehicle, "ownedPort")
	if !ok || len(elems) != 1 || elems[0].Name != "p" {
		t.Errorf("ownedPort on recorded Vehicle = %v, %v, want [p]", fqns(elems), ok)
	}
	elems, ok = m.reflectiveOwnedUsages(vehicle, "ownedPart")
	if !ok || len(elems) != 1 || elems[0].Name != "wheel" {
		t.Errorf("ownedPart on recorded Vehicle = %v, %v, want [wheel]", fqns(elems), ok)
	}
}

// `state idle` nested in `state running` is a SubstateMember — still a
// StateUsage the owner's `nested*` properties derive (SysML v2 §8.3), as a
// transition member is a TransitionUsage.
func TestReflectiveNestedUsagesOfMemberForms(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		state machine {
			state running { state idle; }
			transition t first running accept after 1 [SI::s] then running;
		}
	}`)
	running := nestedSym(t, root, "P::machine::running")

	elems, ok := m.reflectiveOwnedUsages(running, "nestedState")
	if !ok || len(elems) != 1 || elems[0].Name != "idle" {
		t.Errorf("nestedState on running = %v, %v, want [idle]", fqns(elems), ok)
	}
	elems, ok = m.reflectiveOwnedUsages(running, "nestedUsage")
	if !ok || len(elems) != 1 || elems[0].Name != "idle" {
		t.Errorf("nestedUsage on running = %v, %v, want [idle]", fqns(elems), ok)
	}

	machine := nestedSym(t, root, "P::machine")
	elems, ok = m.reflectiveOwnedUsages(machine, "nestedTransition")
	if !ok || len(elems) != 1 || elems[0].Name != "t" {
		t.Errorf("nestedTransition on machine = %v, %v, want [t]", fqns(elems), ok)
	}
}

// A `render rendering child;` declares a composite rendering, so it follows
// the rendering rules — `subrenderings` under a rendering, the part rules
// under a view — while `render r;` only references a rendering and subsets
// nothing.
func TestImplicitSubsettingsRenderRendering(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		rendering def AsTree;
		view v {
			render asTree;
			render rendering child : AsTree;
		}
	}`)

	got := m.ImplicitSubsettings(nestedSym(t, root, "P::v::child"))
	if len(got) != 1 || symbols.FQNOf(got[0]) != "Items::Item::subparts" {
		t.Errorf("ImplicitSubsettings(child) = %v, want [Items::Item::subparts]", fqns(got))
	}
	if got := m.ImplicitSubsettings(nestedSym(t, root, "P::v::asTree")); len(got) != 0 {
		t.Errorf("ImplicitSubsettings(asTree) = %v, want none for a render reference", fqns(got))
	}
}

// A `ref step` is a reference, not a composite nested performance, so it must
// not enter the owner's `subperformances`; a composite `step` does.
func TestImplicitSubsettingsRefStep(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		action outer {
			step inner;
			ref step borrowed;
		}
	}`)

	inner := nestedSym(t, root, "P::outer::inner")
	got := m.ImplicitSubsettings(inner)
	if len(got) != 1 || symbols.FQNOf(got[0]) != "Performances::Performance::subperformances" {
		t.Errorf("ImplicitSubsettings(inner) = %v, want [Performances::Performance::subperformances]", fqns(got))
	}
	borrowed := nestedSym(t, root, "P::outer::borrowed")
	if got := m.ImplicitSubsettings(borrowed); len(got) != 0 {
		t.Errorf("ImplicitSubsettings(borrowed) = %v, want none for a ref step", fqns(got))
	}
}
