package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const inheritedMembersModel = `package Probe {
	part def Vehicle { attribute mass : ScalarValues::Real; }
	part def Car :> Vehicle { part engine : Engine; }
	part def Engine;
	part car : Car;
}`

func inheritedMembersElements(t *testing.T) (*rdf.Graph, map[string]map[string]any) {
	t.Helper()
	graph := duplicateGraph(t, inheritedMembersModel, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	return graph, booleansElements(t, document)
}

func referenceIDs(t *testing.T, element map[string]any, property string) []string {
	t.Helper()
	values, ok := element[property].([]any)
	if !ok {
		t.Fatalf("%s is %v, want an array", property, element[property])
	}
	ids := make([]string, 0, len(values))
	for _, value := range values {
		reference, _ := value.(map[string]any)
		id, _ := reference["@id"].(string)
		ids = append(ids, id)
	}
	return ids
}

// A specialization's inherited memberships are the general's non-private ones and,
// through the implied Parts::Part base, the library's (KerML 8.3.3.1.10).
func TestAPIJSONInheritedMembershipThroughGraphAndLibraryBases(t *testing.T) {
	_, elements := inheritedMembersElements(t)
	car := elements["Probe__Car"]
	inherited := referenceIDs(t, car, "inheritedMembership")
	if !slices.Contains(inherited, "Probe__Vehicle__mass_om") {
		t.Errorf("Car.inheritedMembership = %v lacks Vehicle's mass membership", inherited)
	}
	vehicle := referenceIDs(t, elements["Probe__Vehicle"], "inheritedMembership")
	if len(vehicle) == 0 || slices.Contains(vehicle, "Probe__Vehicle__mass_om") {
		t.Errorf("Vehicle.inheritedMembership = %v, want the Parts::Part members only", vehicle)
	}
	if len(inherited) != len(vehicle)+1 {
		t.Errorf("Car inherits %d memberships, want Vehicle's %d plus its owned mass", len(inherited), len(vehicle))
	}
	features := referenceIDs(t, car, "feature")
	if len(features) < 2 || features[0] != "Probe__Car__engine" || features[1] != "Probe__Vehicle__mass" {
		t.Errorf("Car.feature = %v, want the owned feature first, then the inherited ones", features)
	}
	if got := referenceIDs(t, car, "inheritedFeature"); len(got) == 0 || got[0] != "Probe__Vehicle__mass" {
		t.Errorf("Car.inheritedFeature = %v, want Vehicle's mass first", got)
	}
	members := referenceIDs(t, car, "member")
	memberships := referenceIDs(t, car, "membership")
	if len(members) != len(memberships) || len(memberships) != len(inherited)+1 {
		t.Errorf("Car has %d members, %d memberships, %d inherited: want owned + inherited", len(members), len(memberships), len(inherited))
	}
	usages := referenceIDs(t, car, "usage")
	if !slices.Contains(usages, "Probe__Car__engine") || !slices.Contains(usages, "Probe__Vehicle__mass") {
		t.Errorf("Car.usage = %v lacks the owned and inherited usages", usages)
	}
	completed, _ := car[apiJSONCompletedPropertiesKey].([]any)
	for _, name := range []any{"inheritedMembership", "feature", "featureMembership", "inheritedFeature", "membership", "member", "usage"} {
		if !slices.Contains(completed, name) {
			t.Errorf("%s = %v does not name %s", apiJSONCompletedPropertiesKey, completed, name)
		}
	}
}

// A usage inherits through its type and its implied Parts::parts base alike.
func TestAPIJSONUsageInheritsThroughTypingAndImpliedBase(t *testing.T) {
	graph, elements := inheritedMembersElements(t)
	car := referenceIDs(t, elements["Probe__car"], "inheritedFeature")
	for _, want := range []string{"Probe__Car__engine", "Probe__Vehicle__mass"} {
		if !slices.Contains(car, want) {
			t.Errorf("car.inheritedFeature = %v lacks %s", car, want)
		}
	}
	definition := referenceIDs(t, elements["Probe__Car"], "inheritedFeature")
	if len(car) <= len(definition) {
		t.Errorf("car inherits %d features, want more than Car's %d through Parts::parts", len(car), len(definition))
	}
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the graph changed through the element form:\nmissing: %v\nadded: %v", missing, added)
	}
}

// A redefinition removes the redefined feature from the inherited lists (KerML
// 8.3.3.1.10 removeRedefinedFeatures), whether its target is stated or in a library.
func TestAPIJSONRedefinedFeaturesLeaveTheInheritedLists(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def Plain;
		part def Base { part x; }
		part def Derived :> Base { part y :>> x; }
		part def Timed { part begin :>> start; }
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for _, property := range []string{"inheritedFeature", "inheritedMembership", "feature", "featureMembership", "member", "membership"} {
		for _, id := range referenceIDs(t, elements["Probe__Derived"], property) {
			if id == "Probe__Base__x" || id == "Probe__Base__x_om" {
				t.Errorf("Derived.%s still lists the redefined Base::x", property)
			}
		}
	}
	plain := referenceIDs(t, elements["Probe__Plain"], "inheritedFeature")
	timed := referenceIDs(t, elements["Probe__Timed"], "inheritedFeature")
	if len(timed) != len(plain)-1 {
		t.Errorf("Timed inherits %d features, want Plain's %d less the redefined Parts::Part::start", len(timed), len(plain))
	}
}
