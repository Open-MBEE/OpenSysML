package symbolfacts

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	// A flow's ends are its type's; the features its paths name are reported.
	wantFlow := []*ConnectorEnd{
		{Name: "source", Declared: "powertrain.tank.level", FeatureId: "Ends::Tank::level", FeatureKind: "attributeUsage",
			ChainIds: []string{"Ends::vehicle::powertrain", "Ends::Powertrain::tank", "Ends::Tank::level"}},
		{Name: "target", Declared: "powertrain.engine.demand", FeatureId: "Ends::Engine::demand", FeatureKind: "attributeUsage",
			ChainIds: []string{"Ends::vehicle::powertrain", "Ends::Powertrain::engine", "Ends::Engine::demand"}},
	}
	if !reflect.DeepEqual(flow.ConnectorEnds, wantFlow) {
		t.Errorf("flow ends = %s, want %s", endsText(flow.ConnectorEnds), endsText(wantFlow))
	}
	if ends := Of(one(t, idx, "Ends::Tank"), sc).ConnectorEnds; len(ends) != 0 {
		t.Errorf("a part definition reports connector ends %s, want none", endsText(ends))
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
