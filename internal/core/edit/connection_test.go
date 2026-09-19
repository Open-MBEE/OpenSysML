package edit

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const interconnectionFixture = `package Vehicle {
	port def FuelPort;
	part def Tank {
		port fuelOut : FuelPort;
	}
	part def Engine {
		port fuelIn : FuelPort;
	}
	// The assembly the connections are added to.
	part def Assembly {
		part tank : Tank;

		part engine : Engine;
	}
}
`

func TestAddConnectionWritesEachKind(t *testing.T) {
	cases := []struct {
		kind, name, typ, want string
	}{
		{"connection", "fuel", "", "connection fuel connect tank.fuelOut to engine.fuelIn;"},
		{"connection", "", "", "connection connect tank.fuelOut to engine.fuelIn;"},
		{"interface", "fuelLine", "", "interface fuelLine connect tank.fuelOut to engine.fuelIn;"},
		{"allocation", "", "", "allocation allocate tank.fuelOut to engine.fuelIn;"},
		{"binding", "", "", "binding bind tank.fuelOut = engine.fuelIn;"},
		{"flow", "fuelFlow", "", "flow fuelFlow from tank.fuelOut to engine.fuelIn;"},
		{"flow", "", "", "flow from tank.fuelOut to engine.fuelIn;"},
	}
	for _, tc := range cases {
		t.Run(tc.kind+"/"+tc.name, func(t *testing.T) {
			m := loadContent(t, "assembly.sysml", interconnectionFixture)
			requireClean(t, m)
			op := AddConnection("Vehicle::Assembly", tc.kind, "tank.fuelOut", "engine.fuelIn", tc.name)
			op.Type = tc.typ
			res, err := Apply(m, []Operation{op})
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			got := string(res.Content)
			if !strings.Contains(got, "\t\t"+tc.want+"\n\t}\n}\n") {
				t.Fatalf("content does not end the assembly with %q:\n%s", tc.want, got)
			}
			// Everything outside the insertion comes back byte-identical.
			before, after, _ := strings.Cut(got, "\t\t"+tc.want+"\n")
			if before+after != interconnectionFixture {
				t.Fatalf("edit rewrote text outside the insertion:\n%s", got)
			}
		})
	}
}

func TestAddConnectionTypedByDefinition(t *testing.T) {
	m := loadContent(t, "typed.sysml", `package P {
	port def Plug;
	connection def Cable {
		end a : Plug;
		end b : Plug;
	}
	part def Rig {
		part left { port p : Plug; }
		part right { port p : Plug; }
	}
}
`)
	requireClean(t, m)
	op := AddConnection("P::Rig", "connection", "left.p", "right.p", "cable")
	op.Type = "Cable"
	res, err := Apply(m, []Operation{op})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !strings.Contains(string(res.Content), "\t\tconnection cable : Cable connect left.p to right.p;\n") {
		t.Fatalf("typed connection not written:\n%s", res.Content)
	}
}

func TestAddConnectionBehaviorKinds(t *testing.T) {
	m := loadContent(t, "behavior.sysml", `package B {
	action def Brew {
		action grind;
		action pour;
	}
	state def Machine {
		state off;
		state on;
	}
}
`)
	requireClean(t, m)
	res, err := Apply(m, []Operation{
		AddConnection("B::Brew", "succession", "grind", "pour", ""),
		AddConnection("B::Machine", "transition", "off", "on", "powerUp"),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	for _, want := range []string{
		"\t\taction pour;\n\t\tsuccession first grind then pour;\n\t}\n",
		"\t\tstate on;\n\t\ttransition powerUp first off then on;\n\t}\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content does not contain %q:\n%s", want, got)
		}
	}
}

func TestAddConnectionKerMLNotation(t *testing.T) {
	m := loadContent(t, "links.kerml", `package Links {
	classifier Tank {
		feature a;
		feature b;
	}
	classifier Rig {
		feature left : Tank;
		feature right : Tank;
	}
}
`)
	requireClean(t, m)
	res, err := Apply(m, []Operation{
		AddConnection("Links::Rig", "connector", "left.a", "right.b", "link"),
		AddConnection("Links::Rig", "binding", "left.a", "right.a", ""),
		AddConnection("Links::Rig", "succession", "left", "right", ""),
		AddConnection("Links::Rig", "flow", "left.a", "right.a", ""),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(res.Content)
	for _, want := range []string{
		"\t\tconnector link from left.a to right.b;\n",
		"\t\tbinding of left.a = right.a;\n",
		"\t\tsuccession first left then right;\n",
		"\t\tflow from left.a to right.a;\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content does not contain %q:\n%s", want, got)
		}
	}
}

func TestAddConnectionIntoBodylessOwnerAndRoot(t *testing.T) {
	m := loadContent(t, "bodyless.sysml", "part def Pair;\npart a;\npart b;\n")
	requireClean(t, m)
	res, err := Apply(m, []Operation{
		AddConnection("Pair", "connection", "a", "b", ""),
		AddConnection("", "connection", "a", "b", "top"),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "part def Pair {\n    connection connect a to b;\n}\npart a;\npart b;\nconnection top connect a to b;\n"
	if got := string(res.Content); got != want {
		t.Fatalf("content:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddConnectionAfterAddedMembersInOneRequest(t *testing.T) {
	m := loadContent(t, "batch.sysml", "package P {\n\tpart def Rig;\n}\n")
	requireClean(t, m)
	res, err := Apply(m, []Operation{
		AddMember("P::Rig", "part", "left"),
		AddMember("P::Rig", "part", "right"),
		AddConnection("P::Rig", "connection", "left", "right", ""),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "package P {\n\tpart def Rig {\n\t\tpart left;\n\t\tpart right;\n\t\tconnection connect left to right;\n\t}\n}\n"
	if got := string(res.Content); got != want {
		t.Fatalf("content:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddConnectionRefusals(t *testing.T) {
	m := loadContent(t, "assembly.sysml", interconnectionFixture)
	requireClean(t, m)
	owner := "Vehicle::Assembly"
	cases := []struct {
		name string
		op   Operation
		want Failure
	}{
		{"unknown kind", AddConnection(owner, "wire", "tank.fuelOut", "engine.fuelIn", ""), FailureIllegalKind},
		{"kerml kind in sysml", AddConnection(owner, "connector", "tank.fuelOut", "engine.fuelIn", ""), FailureIllegalKind},
		{"untyped kind with type", func() Operation {
			op := AddConnection(owner, "binding", "tank.fuelOut", "engine.fuelIn", "")
			op.Type = "Cable"
			return op
		}(), FailureIllegalKind},
		{"empty from", AddConnection(owner, "connection", "", "engine.fuelIn", ""), FailureInvalidName},
		{"end is an expression", AddConnection(owner, "connection", "tank.fuelOut", "1 + 2", ""), FailureInvalidName},
		{"end has a trailing dot", AddConnection(owner, "connection", "tank.", "engine.fuelIn", ""), FailureInvalidName},
		{"keyword name", AddConnection(owner, "connection", "tank.fuelOut", "engine.fuelIn", "part"), FailureInvalidName},
		{"name taken", AddConnection(owner, "connection", "tank.fuelOut", "engine.fuelIn", "tank"), FailureMemberNameTaken},
		{"unknown owner", AddConnection("Vehicle::Nowhere", "connection", "tank.fuelOut", "engine.fuelIn", ""), FailureOwnerUnknown},
		{"library owner", AddConnection("ISQ", "connection", "tank.fuelOut", "engine.fuelIn", ""), FailureOwnerUnknown},
		{"unresolved end", AddConnection(owner, "connection", "tank.fuelOut", "engine.exhaust", ""), FailureResultInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := addFailure(t, m, tc.op, tc.want)
			if e.Message == "" {
				t.Fatal("refusal carries no message")
			}
		})
	}
}

func TestAddConnectionDuplicateNamesInOneRequestRefuse(t *testing.T) {
	m := loadContent(t, "assembly.sysml", interconnectionFixture)
	_, err := Apply(m, []Operation{
		AddConnection("Vehicle::Assembly", "connection", "tank.fuelOut", "engine.fuelIn", "fuel"),
		AddMember("Vehicle::Assembly", "part", "fuel"),
	})
	if e := editError(t, err); e.Failure != FailureMemberNameTaken {
		t.Fatalf("failure = %s (%s), want %s", e.Failure, e.Message, FailureMemberNameTaken)
	}
	// Anonymous connections never collide with each other.
	res, err := Apply(m, []Operation{
		AddConnection("Vehicle::Assembly", "connection", "tank.fuelOut", "engine.fuelIn", ""),
		AddConnection("Vehicle::Assembly", "flow", "tank.fuelOut", "engine.fuelIn", ""),
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := strings.Count(string(res.Content), "tank.fuelOut to engine.fuelIn;"); got != 2 {
		t.Fatalf("wrote %d connections, want 2:\n%s", got, res.Content)
	}
}

func TestKindListings(t *testing.T) {
	sysml := strings.Join(ConnectionKinds(source.KindSysML), " ")
	if sysml != "allocation binding connection flow interface succession transition" {
		t.Fatalf("SysML connection kinds = %q", sysml)
	}
	kerml := strings.Join(ConnectionKinds(source.KindKerML), " ")
	if kerml != "binding connector flow succession" {
		t.Fatalf("KerML connection kinds = %q", kerml)
	}
	members := MemberKinds(source.KindSysML)
	for _, want := range []string{"part def", "port", "state", "action", "fork"} {
		if !contains(members, want) {
			t.Fatalf("SysML member kinds %v lack %q", members, want)
		}
	}
	if contains(members, "class") || contains(MemberKinds(source.KindKerML), "part") {
		t.Fatal("member kinds leak across languages")
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
