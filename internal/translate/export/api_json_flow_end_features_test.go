package export

import "testing"

// The element form writes a flow's sourceOutputFeature and targetInputFeature as the first
// ownedFeature of its first and second connectorEnd (KerML 8.3.4.9.2 deriveFlowSourceOutputFeature).
func TestAPIJSONWritesAFlowsEndFeatures(t *testing.T) {
	document, err := WriteAPIJSON(graphOfNotation(t, `package P {
    item def Fuel;
    part def Tank { out item fuelOut : Fuel; part inner { out item o : Fuel; } }
    part def Engine { in item fuelIn : Fuel; part inner { in item i : Fuel; } }
    part tank : Tank;
    part engine : Engine;
    flow f of Fuel from tank.fuelOut to engine.fuelIn;
    succession flow s of Fuel from tank.fuelOut to engine.fuelIn;
    flow chained from tank.inner.o to engine.inner.i;
    message msg of Fuel from tank to engine;
}`))
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	reference := func(value any) string {
		object, _ := value.(map[string]any)
		id, _ := object["@id"].(string)
		return id
	}
	checked := 0
	for id, element := range elements {
		if _, isFlow := element["flowEnd"]; !isFlow {
			continue
		}
		ends, _ := element["connectorEnd"].([]any)
		for index, property := range []string{"sourceOutputFeature", "targetInputFeature"} {
			got := reference(element[property])
			if element["name"] == "msg" {
				if got != "" {
					t.Errorf("%s: %s = %s, want none for a message", id, property, got)
				}
				continue
			}
			if len(ends) != 2 {
				t.Fatalf("%s: connectorEnd = %v, want two ends", id, ends)
			}
			owned, _ := elements[reference(ends[index])]["ownedFeature"].([]any)
			if len(owned) == 0 {
				t.Fatalf("%s: connectorEnd %d owns no feature", id, index)
			}
			if want := reference(owned[0]); got != want {
				t.Errorf("%s: %s = %q, want %q, the first ownedFeature of connectorEnd %d", id, property, got, want, index)
			}
			checked++
		}
	}
	if checked != 6 {
		t.Errorf("checked %d end features, want 6 over three flows", checked)
	}
}
