package grpc

import (
	"slices"
	"testing"
)

// A flow's sourceOutputFeature and targetInputFeature are the features its two ends own, which
// redefine the attached features (KerML 8.3.4.9.2 deriveFlowSourceOutputFeature, deriveFlowTargetInputFeature).
func TestElementPropertiesReadAFlowsEndFeatures(t *testing.T) {
	r := parseForElementProperties(t, `package P {
	item def Fuel;
	item def Sig;
	part def Tank { out item fuelOut : Fuel; part inner { out item o : Fuel; } }
	part def Engine { in item fuelIn : Fuel; part inner { in item i : Fuel; } }
	part def Vehicle {
		part tank : Tank;
		part engine : Engine;
		flow f of Fuel from tank.fuelOut to engine.fuelIn;
		succession flow s of Fuel from tank.fuelOut to engine.fuelIn;
		flow chained from tank.inner.o to engine.inner.i;
		message msg of Sig from tank to engine;
	}
}`)
	for _, tc := range []struct{ flow, property, want, redefined string }{
		{"P::Vehicle::f", "sourceOutputFeature", "P::Vehicle::f::source::fuelOut", "P::Tank::fuelOut"},
		{"P::Vehicle::f", "targetInputFeature", "P::Vehicle::f::target::fuelIn", "P::Engine::fuelIn"},
		{"P::Vehicle::s", "sourceOutputFeature", "P::Vehicle::s::source::fuelOut", "P::Tank::fuelOut"},
		{"P::Vehicle::s", "targetInputFeature", "P::Vehicle::s::target::fuelIn", "P::Engine::fuelIn"},
		{"P::Vehicle::chained", "sourceOutputFeature", "P::Vehicle::chained::source::o", "P::Tank::inner::o"},
		{"P::Vehicle::chained", "targetInputFeature", "P::Vehicle::chained::target::i", "P::Engine::inner::i"},
	} {
		got := r.property(tc.flow, tc.property)
		if names := elementNames(got.Values); !slices.Equal(names, []string{tc.want}) {
			t.Errorf("%s.%s = %v, want %s", tc.flow, tc.property, names, tc.want)
			continue
		}
		feature := got.Values[0].GetElement().GetId()
		end := map[string]int{"sourceOutputFeature": 0, "targetInputFeature": 1}[tc.property]
		ends := r.property(tc.flow, "connectorEnd").Values
		if owned := propertyElementIDs(r.property(ends[end].GetElement().GetId(), "ownedFeature").Values); len(owned) == 0 || owned[0] != feature {
			t.Errorf("%s.%s = %s, want the first feature its end owns %v", tc.flow, tc.property, feature, owned)
		}
		var redefined []string
		for _, redefinition := range r.property(feature, "ownedRedefinition").Values {
			redefined = append(redefined, elementNames(r.property(redefinition.GetElement().GetId(), "redefinedFeature").Values)...)
		}
		if !slices.Equal(redefined, []string{tc.redefined}) {
			t.Errorf("%s.%s redefines %v, want %s", tc.flow, tc.property, redefined, tc.redefined)
		}
	}
	for _, property := range []string{"sourceOutputFeature", "targetInputFeature"} {
		if got := r.property("P::Vehicle::msg", property); got.Unsupported != "" || len(got.Values) != 0 {
			t.Errorf("msg.%s = %v (%s), want none", property, elementNames(got.Values), got.Unsupported)
		}
	}
}
