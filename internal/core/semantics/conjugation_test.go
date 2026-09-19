package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// portSrc declares a directed port definition and ordinary and conjugated
// usages of it, the shape SysML v2 §7.12.3 defines conjugation over.
const portSrc = `package P {
	item def Command;
	item def Telemetry;
	item def Sync;
	port def CommunicationPort {
		in item cmd : Command;
		out item tlm : Telemetry;
		inout item sync : Sync;
	}
	part def Ground { port p : CommunicationPort; }
	part def Space { port p : ~CommunicationPort; }
}`

// directions returns the effective direction of each named feature of a port.
func directions(m *Model, port *symbols.Symbol) map[string]ast.FeatureDirection {
	out := make(map[string]ast.FeatureDirection)
	for _, f := range m.PortFeatures(port) {
		if f.Name != "" {
			out[f.Name] = f.Direction
		}
	}
	return out
}

// TestConjugationReversesDirections covers SysML v2 §7.12.3: the features of a
// conjugated port definition have conjugate directions — in and out reversed,
// inout unchanged.
func TestConjugationReversesDirections(t *testing.T) {
	m, root := buildModel(t, portSrc)
	pkg := sym(t, root, "P")
	ground := member(t, m, pkg, "Ground")
	space := member(t, m, pkg, "Space")
	groundPort := member(t, m, ground, "p")
	spacePort := member(t, m, space, "p")

	if m.IsConjugated(groundPort) {
		t.Errorf("Ground::p reported conjugated")
	}
	if !m.IsConjugated(spacePort) {
		t.Errorf("Space::p not reported conjugated")
	}

	want := map[string]ast.FeatureDirection{
		"cmd":  ast.DirIn,
		"tlm":  ast.DirOut,
		"sync": ast.DirInOut,
	}
	got := directions(m, groundPort)
	for name, dir := range want {
		if got[name] != dir {
			t.Errorf("Ground::p feature %s direction = %v, want %v", name, got[name], dir)
		}
	}

	wantConj := map[string]ast.FeatureDirection{
		"cmd":  ast.DirOut,
		"tlm":  ast.DirIn,
		"sync": ast.DirInOut,
	}
	gotConj := directions(m, spacePort)
	for name, dir := range wantConj {
		if gotConj[name] != dir {
			t.Errorf("Space::p feature %s direction = %v, want %v", name, gotConj[name], dir)
		}
	}
}

// TestDoubleConjugationRestoresDirections covers the composition of conjugation
// along a typing chain (§7.12.3): conjugating an already conjugated port
// restores the original directions.
func TestDoubleConjugationRestoresDirections(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def CommunicationPort {
			in item cmd;
			out item tlm;
		}
		part def X {
			port pc : ~CommunicationPort;
			port pcc : ~pc;
		}
	}`)
	x := member(t, m, sym(t, root, "P"), "X")
	pc := member(t, m, x, "pc")
	pcc := member(t, m, x, "pcc")

	if !m.IsConjugated(pc) {
		t.Errorf("pc not reported conjugated")
	}
	if m.IsConjugated(pcc) {
		t.Errorf("pcc reported conjugated: conjugating a conjugate is the original")
	}
	got := directions(m, pcc)
	if got["cmd"] != ast.DirIn || got["tlm"] != ast.DirOut {
		t.Errorf("pcc directions = %v, want cmd in / tlm out", got)
	}
}

// TestConjugationTerminatesOnSpecializationCycle: a cyclic specialization is
// diagnosable input, so the parity walk must terminate rather than recurse.
func TestConjugationTerminatesOnSpecializationCycle(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def A :> B;
		port def B :> A;
		part def X { port a : ~A; }
	}`)
	pkg := sym(t, root, "P")
	for _, name := range []string{"A", "B"} {
		if m.IsConjugated(member(t, m, pkg, name)) {
			t.Errorf("%s reported conjugated", name)
		}
	}
	x := member(t, m, pkg, "X")
	if !m.IsConjugated(member(t, m, x, "a")) {
		t.Errorf("X::a not reported conjugated")
	}
}

// TestConjugatedPortConformance covers §7.12.2: a conjugated port usage
// conforms to a usage of the original port definition, and two ports conform
// when each feature of one matches a feature of the other — conforming types
// and conjugate directions.
func TestConjugatedPortConformance(t *testing.T) {
	m, root := buildModel(t, portSrc+`
	package Q {
		part def Other { port p : P::CommunicationPort; }
	}`)
	pkg := sym(t, root, "P")
	def := member(t, m, pkg, "CommunicationPort")
	groundPort := member(t, m, member(t, m, pkg, "Ground"), "p")
	spacePort := member(t, m, member(t, m, pkg, "Space"), "p")
	otherPort := member(t, m, member(t, m, sym(t, root, "Q"), "Other"), "p")

	// A conjugated port usage still conforms to the original definition.
	if !m.Conforms(spacePort, def) {
		t.Errorf("Space::p does not conform to CommunicationPort")
	}
	if !m.PortsConform(groundPort, spacePort) {
		t.Errorf("a port and its conjugate do not conform")
	}
	if !m.PortsConform(spacePort, groundPort) {
		t.Errorf("conformance of a port and its conjugate is not symmetric")
	}
	// Two ports typed by the same directed definition do not conform: their
	// directed features are equal, not conjugate.
	if m.PortsConform(groundPort, otherPort) {
		t.Errorf("two like-typed directed ports reported conforming")
	}
}

// TestInterfaceEndConjugation covers the interface-end shape of §8.2.2.14: the
// ends an interface definition declares carry their own conjugation, and the
// ends of a connect clause typed by that definition take it by implicit
// redefinition (§7.13.2).
func TestInterfaceEndConjugation(t *testing.T) {
	m, root := buildModel(t, `package P {
		port def CommunicationPort {
			in item cmd;
			out item tlm;
		}
		interface def CommunicationInterface {
			end groundPort : CommunicationPort;
			end spacePort : ~CommunicationPort;
		}
		part def Station { port comm : CommunicationPort; }
		part def Craft { port comm : ~CommunicationPort; }
		part context {
			part station : Station;
			part craft : Craft;
			interface : CommunicationInterface connect
				g references station.comm to
				s references craft.comm;
		}
	}`)
	pkg := sym(t, root, "P")
	def := member(t, m, pkg, "CommunicationInterface")
	groundEnd := member(t, m, def, "groundPort")
	spaceEnd := member(t, m, def, "spacePort")

	if m.IsConjugated(groundEnd) {
		t.Errorf("groundPort reported conjugated")
	}
	if !m.IsConjugated(spaceEnd) {
		t.Errorf("spacePort not reported conjugated")
	}
	if got := directions(m, spaceEnd); got["cmd"] != ast.DirOut || got["tlm"] != ast.DirIn {
		t.Errorf("spacePort directions = %v, want cmd out / tlm in", got)
	}
	if !m.PortsConform(groundEnd, spaceEnd) {
		t.Errorf("interface ends do not conform")
	}

	// The connect clause has as many ends as the interface definition declares,
	// and each takes the type of the end it implicitly redefines.
	ctx := member(t, m, pkg, "context")
	conn := connector(t, ctx.Scope)
	if got, want := m.ConnectorEndCount(def), 2; got != want {
		t.Errorf("interface def end count = %d, want %d", got, want)
	}
	if got, want := len(m.endsOf(conn)), 2; got != want {
		t.Fatalf("connect end count = %d, want %d", got, want)
	}
	for i, end := range m.endsOf(conn) {
		redefined := m.implicitEndRedefinitions(end)
		if len(redefined) == 0 {
			t.Errorf("end %d redefines nothing", i)
			continue
		}
		want := groundEnd
		if i == 1 {
			want = spaceEnd
		}
		if redefined[0] != want {
			t.Errorf("end %d redefines %v, want %v", i, redefined[0].Name, want.Name)
		}
	}
}

// TestConjugationOnRedefiningPort covers a port that both refines an inherited
// feature and states a conjugated type: the type states the conjugation, so the
// redefinition declared before it must not hide it (§7.12.3).
func TestConjugationOnRedefiningPort(t *testing.T) {
	m, root := buildModel(t, `package P {
	item def Command;
	item def Telemetry;
	port def CommunicationPort {
		in item cmd : Command;
		out item tlm : Telemetry;
	}
	part def Ground { port link : CommunicationPort; }
	part def Space :> Ground { port :>> link : ~CommunicationPort; }
}`)
	pkg := sym(t, root, "P")
	space := member(t, m, pkg, "Space")
	port := member(t, m, space, "link")

	if !m.IsConjugated(port) {
		t.Errorf("Space::link not reported conjugated")
	}
	if got := directions(m, port); got["cmd"] != ast.DirOut || got["tlm"] != ast.DirIn {
		t.Errorf("Space::link directions = %v, want cmd out / tlm in", got)
	}
}
