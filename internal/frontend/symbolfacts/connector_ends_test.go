package symbolfacts

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// connectorModel connects features by chains and by plain names, through an
// interface with named ends and a connection with unnamed ones, and a flow.
const connectorModel = `
package Ends {
	private import ScalarValues::*;
	port def FuelPort;
	part def Tank { port fuelOut : FuelPort; attribute level : Real; }
	part def Engine { port fuelIn : FuelPort; attribute demand : Real; }
	part def Powertrain { part tank : Tank; part engine : Engine; }
	part vehicle {
		part powertrain : Powertrain;
		part spare : Tank;
		interface fuelLine connect
			supplier ::> powertrain.tank.fuelOut to
			consumer ::> powertrain.engine.fuelIn;
		connect spare to powertrain;
		flow of Real from powertrain.tank.level to powertrain.engine.demand;
	}
}
`

func TestConnectorEndsNameTheFeaturesTheyConnect(t *testing.T) {
	idx, sc := modelWithLibrary(t, connectorModel)
	info := Of(one(t, idx, "Ends::vehicle::fuelLine"), sc)
	want := []*ConnectorEnd{
		{
			EndId: "Ends::vehicle::fuelLine::supplier", Name: "supplier", Declared: "powertrain.tank.fuelOut",
			FeatureId: "Ends::Tank::fuelOut", FeatureKind: "portUsage",
			ChainIds: []string{"Ends::vehicle::powertrain", "Ends::Powertrain::tank", "Ends::Tank::fuelOut"},
		},
		{
			EndId: "Ends::vehicle::fuelLine::consumer", Name: "consumer", Declared: "powertrain.engine.fuelIn",
			FeatureId: "Ends::Engine::fuelIn", FeatureKind: "portUsage",
			ChainIds: []string{"Ends::vehicle::powertrain", "Ends::Powertrain::engine", "Ends::Engine::fuelIn"},
		},
	}
	if !reflect.DeepEqual(info.ConnectorEnds, want) {
		t.Errorf("fuelLine connector ends = %s, want %s", endsText(info.ConnectorEnds), endsText(want))
	}
	// Each end's own facts reference the feature it connects, the chain included.
	for _, end := range want {
		var references *Specialization
		for _, spec := range Of(one(t, idx, end.EndId), sc).Specializations {
			if spec.Kind == "references" {
				references = spec
			}
		}
		wantReference := &Specialization{Kind: "references", Declared: end.Declared, TargetId: end.FeatureId, TargetKind: end.FeatureKind, ChainIds: end.ChainIds}
		if !reflect.DeepEqual(references, wantReference) {
			t.Errorf("%s specializations lack %+v, got %+v", end.EndId, wantReference, references)
		}
	}
}

func TestPlainConnectorEndsAndFlowsReportTheirFeatures(t *testing.T) {
	idx, sc := modelWithLibrary(t, connectorModel)
	var connection, flow *Info
	for _, child := range one(t, idx, "Ends::vehicle").Scope.AllMembers() {
		switch info := Of(child, sc); info.Kind {
		case "connectionUsage":
			connection = info
		case "flowUsage":
			flow = info
		}
	}
	if connection == nil || flow == nil {
		t.Fatalf("vehicle owns connection %v and flow %v, want both", connection != nil, flow != nil)
	}
	if got := connection.ConnectorEnds; len(got) != 2 ||
		got[0].FeatureId != "Ends::vehicle::spare" || got[0].Declared != "spare" || len(got[0].ChainIds) != 0 ||
		got[1].FeatureId != "Ends::vehicle::powertrain" || got[1].Declared != "powertrain" || got[1].EndId == "" {
		t.Errorf("connect spare to powertrain ends = %s", endsText(got))
	}
	// A flow's ends are the ones it inherits from its type; the features its
	// paths name are reported at their positions.
	wantFlow := []*ConnectorEnd{
		{EndId: "Flows::flows::source", Name: "source", Declared: "powertrain.tank.level", FeatureId: "Ends::Tank::level", FeatureKind: "attributeUsage",
			ChainIds: []string{"Ends::vehicle::powertrain", "Ends::Powertrain::tank", "Ends::Tank::level"}},
		{EndId: "Flows::flows::target", Name: "target", Declared: "powertrain.engine.demand", FeatureId: "Ends::Engine::demand", FeatureKind: "attributeUsage",
			ChainIds: []string{"Ends::vehicle::powertrain", "Ends::Powertrain::engine", "Ends::Engine::demand"}},
	}
	if !reflect.DeepEqual(flow.ConnectorEnds, wantFlow) {
		t.Errorf("flow ends = %s, want %s", endsText(flow.ConnectorEnds), endsText(wantFlow))
	}
	if ends := Of(one(t, idx, "Ends::Tank"), sc).ConnectorEnds; len(ends) != 0 {
		t.Errorf("a part definition reports connector ends %s, want none", endsText(ends))
	}
}

// inheritedEndsModel declares a connection's ends in a definition another
// definition and a usage inherit, and an end attached by a qualified name.
const inheritedEndsModel = `
package Inherited {
	port def P;
	part def Thing { port p : P; }
	connection def Base { end a : Thing; end b : Thing; }
	connection def Derived :> Base;
	part x : Thing;
	package Other { part y : Thing; }
	connection link : Base connect x to Other::y;
	interface i connect supplier ::> Other::y.p to consumer ::> x.p;
}
`

func TestConnectorEndsIncludeTheOnesInherited(t *testing.T) {
	idx, sc := modelWithLibrary(t, inheritedEndsModel)
	want := []*ConnectorEnd{
		{EndId: "Inherited::Base::a", Name: "a"},
		{EndId: "Inherited::Base::b", Name: "b"},
	}
	if got := Of(one(t, idx, "Inherited::Derived"), sc).ConnectorEnds; !reflect.DeepEqual(got, want) {
		t.Errorf("Derived :> Base ends = %s, want %s", endsText(got), endsText(want))
	}
	// A usage's own clause ends come first, and redefine the inherited ones.
	got := Of(one(t, idx, "Inherited::link"), sc).ConnectorEnds
	if len(got) != 2 || got[0].EndId == "" || got[1].EndId == "" ||
		got[0].FeatureId != "Inherited::x" || got[0].Declared != "x" ||
		got[1].FeatureId != "Inherited::Other::y" || got[1].Declared != "Other::y" {
		t.Errorf("link ends = %s", endsText(got))
	}
}

func TestExplicitEndReferencesKeepTheirSpelling(t *testing.T) {
	idx, sc := modelWithLibrary(t, inheritedEndsModel)
	got := Of(one(t, idx, "Inherited::i"), sc).ConnectorEnds
	if len(got) != 2 || got[0].Declared != "Other::y.p" || got[0].FeatureId != "Inherited::Thing::p" ||
		!reflect.DeepEqual(got[0].ChainIds, []string{"Inherited::Other::y", "Inherited::Thing::p"}) ||
		got[1].Declared != "x.p" {
		t.Errorf("interface ends = %s", endsText(got))
	}
}

func TestAnonymousConnectionsWithUnnamedEndsReportTheirEnds(t *testing.T) {
	base, err := libs.SnapshotIndex()
	if err != nil {
		base, _ = libs.SharedLibrary()
	}
	idx := symbols.NewOverlay(base)
	sc := NewContext(idx)
	var connections int
	var anonymous []*symbols.Symbol
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		anonymous = append(anonymous, scope.AnonymousMembers()...)
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(one(t, idx, "Flows").Scope)
	for _, member := range anonymous {
		if len(sc.Semantics.EndFeatures(member)) != 2 {
			continue
		}
		connections++
		ends := Of(member, sc).ConnectorEnds
		if len(ends) != 2 || ends[0].FeatureId == "" || ends[1].FeatureId == "" {
			t.Errorf("%s ends = %s, want two connected features", idx.GetFQN(member), endsText(ends))
		}
	}
	if connections == 0 {
		t.Fatal("the Flows library declares anonymous connections, found none")
	}
}

// attachmentsModel types a flow and a connection by definitions whose ends
// reference features of their own.
const attachmentsModel = `
package Attach {
	private import ScalarValues::*;
	attribute oldSource : Real; attribute oldTarget : Real;
	attribute newSource : Real; attribute newTarget : Real;
	flow def F { end source references oldSource; end target references oldTarget; }
	flow f : F from newSource to newTarget;
	connection def C { end a references oldSource; end b references oldTarget; }
	connection c : C;
}
`

func TestOwnAttachmentsOutrankInheritedEndReferences(t *testing.T) {
	idx, sc := modelWithLibrary(t, attachmentsModel)
	want := []*ConnectorEnd{
		{EndId: "Attach::F::source", Name: "source", Declared: "newSource", FeatureId: "Attach::newSource", FeatureKind: "attributeUsage"},
		{EndId: "Attach::F::target", Name: "target", Declared: "newTarget", FeatureId: "Attach::newTarget", FeatureKind: "attributeUsage"},
	}
	if got := Of(one(t, idx, "Attach::f"), sc).ConnectorEnds; !reflect.DeepEqual(got, want) {
		t.Errorf("flow f : F from newSource to newTarget ends = %s, want %s", endsText(got), endsText(want))
	}
	// A connector attaching nothing of its own connects what its inherited ends reference.
	want = []*ConnectorEnd{
		{EndId: "Attach::C::a", Name: "a", FeatureId: "Attach::oldSource", FeatureKind: "attributeUsage"},
		{EndId: "Attach::C::b", Name: "b", FeatureId: "Attach::oldTarget", FeatureKind: "attributeUsage"},
	}
	if got := Of(one(t, idx, "Attach::c"), sc).ConnectorEnds; !reflect.DeepEqual(got, want) {
		t.Errorf("connection c : C ends = %s, want %s", endsText(got), endsText(want))
	}
}

// one is the single symbol id names.
func one(t *testing.T, idx *symbols.Index, id string) *symbols.Symbol {
	t.Helper()
	syms := LookupNamed(idx, id)
	if len(syms) != 1 {
		t.Fatalf("%s names %d symbols, want one", id, len(syms))
	}
	return syms[0]
}

func endsText(ends []*ConnectorEnd) string {
	text := "["
	for i, end := range ends {
		if i > 0 {
			text += ", "
		}
		text += "{" + end.EndId + " " + end.Name + " " + end.Declared + " -> " + end.FeatureId + " (" + end.FeatureKind + ") " + reflect.ValueOf(end.ChainIds).String() + "}"
	}
	return text + "]"
}
