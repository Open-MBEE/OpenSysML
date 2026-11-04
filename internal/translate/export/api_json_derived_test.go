package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The element form completes every derived property of the table the graph
// does not state, names them in sysx:completedProperties, and reads them back
// as no statement, so the graph survives the round trip triple for triple.
func TestAPIJSONCompletedDerivedPropertiesRoundTrip(t *testing.T) {
	graph := booleansGraph(t)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for id, element := range elements {
		if element["@type"] == "Namespace" {
			continue
		}
		value, stated := element["isLibraryElement"]
		if !stated || value != false {
			t.Errorf("%s: isLibraryElement is %v, want false (KerML 8.3.2.1.2 deriveElementIsLibraryElement)", id, value)
		}
		names, _ := element[apiJSONCompletedPropertiesKey].([]any)
		if len(names) == 0 || names[0] != "isLibraryElement" {
			t.Errorf("%s: %s = %v does not name isLibraryElement", id, apiJSONCompletedPropertiesKey, names)
		}
	}
	back, err := ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the graph changed through the element form:\nmissing:\n%s\nadded:\n%s",
			strings.Join(missing, "\n"), strings.Join(added, "\n"))
	}
	for _, subject := range back.SubjectsView() {
		if back.HasProperty(subject, rdf.OpenSysML+strings.TrimPrefix(apiJSONCompletedPropertiesKey, "sysx:")) {
			t.Errorf("%s: the marker entered the graph as a fact", subject.Value)
		}
	}
}

// Without the writer's marker a stated derived property is the statement it
// looks like, as an external document's would be.
func TestAPIJSONUnmarkedCompletedPropertyReadsAsStated(t *testing.T) {
	graph := booleansGraph(t)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		delete(element, apiJSONStatedDefaultsKey)
		delete(element, apiJSONCompletedPropertiesKey)
	}
	unmarked, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ReadAPIJSON(unmarked)
	if err != nil {
		t.Fatal(err)
	}
	if !back.HasProperty(rdf.ElementIRIForID("P__D__a"), rdf.SysML+"isLibraryElement") {
		t.Error("unmarked document: the stated isLibraryElement is no triple")
	}
}

// The compact form without derived properties leaves the completed derived
// values out and keeps the list naming them; with them it writes both.
func TestAPIJSONCompactOmitsCompletedProperties(t *testing.T) {
	graph := booleansGraph(t)
	for _, omit := range []bool{false, true} {
		compact, err := WriteAPIJSONCompact(graph, CompactAPIJSONOptions{OmitDerived: omit})
		if err != nil {
			t.Fatal(err)
		}
		text := string(compact)
		stated := strings.Contains(text, `"isLibraryElement":`)
		listed := strings.Contains(text, `"`+apiJSONCompletedPropertiesKey+`":["isLibraryElement"`)
		if stated == omit || !listed || !strings.Contains(text, `"visibility"`) {
			t.Errorf("OmitDerived=%v: isLibraryElement written %v, listed %v", omit, stated, listed)
		}
	}
}

// A sysx:completedProperties list is honoured only for a property the writer
// completes on that metaclass and the element states; anything else is refused
// rather than read as the writer's completion.
func TestAPIJSONCompletedPropertiesListIsChecked(t *testing.T) {
	for _, tc := range []struct{ name, document, want string }{
		{"a declared property", `[{"@type":"PartDefinition","@id":"P","declaredName":"Pump","sysx:completedProperties":["declaredName"]}]`,
			"does not complete"},
		{"another class's property", `[{"@type":"PartDefinition","@id":"P","sysx:completedProperties":["memberElementId"]}]`,
			"does not complete"},
	} {
		if _, err := ReadAPIJSON([]byte(tc.document)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one saying %q", tc.name, err, tc.want)
		}
	}
	back, err := ReadAPIJSON([]byte(`[{"@type":"PartDefinition","@id":"P","declaredName":"Pump","isLibraryElement":false,"sysx:completedProperties":["isLibraryElement"]}]`))
	if err != nil {
		t.Fatal(err)
	}
	p := rdf.ElementIRIForID("P")
	if !back.HasProperty(p, rdf.SysML+"declaredName") || back.HasProperty(p, rdf.SysML+"isLibraryElement") {
		t.Error("a checked list drops the completed property and keeps the declared one")
	}
}

// A membership's memberElementId is its member's elementId as the graph states
// it (KerML 8.3.2.4.3), not the member's "@id" when a document declared another.
func TestAPIJSONMemberElementIdIsTheStatedElementId(t *testing.T) {
	graph := booleansGraph(t)
	member := rdf.ElementIRIForID("P__D")
	triples := make([]rdf.Triple, 0, graph.Len())
	for _, triple := range graph.Triples() {
		if triple.Subject.Equal(member) && triple.Predicate.Value == rdf.SysML+"elementId" {
			triple.Object = rdf.String("declared-D")
		}
		triples = append(triples, triple)
	}
	document, err := WriteAPIJSON(rdf.NewGraphOf(triples, graph.Prefixes))
	if err != nil {
		t.Fatal(err)
	}
	membership := booleansElements(t, document)["P__D_om"]
	for _, key := range []string{"memberElementId", "ownedMemberElementId"} {
		if membership[key] != "declared-D" {
			t.Errorf("%s = %v, want the stated elementId declared-D", key, membership[key])
		}
	}
}

// An OwningMembership's ownedMemberName redefines memberName (KerML 8.3.2.4.8),
// so a minted end's membership states the name the writer derives for both.
func TestAPIJSONOwnedMemberNameIsTheMemberName(t *testing.T) {
	graph := booleansGraph(t)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	named := 0
	for id, element := range booleansElements(t, document) {
		if element["@type"] != "EndFeatureMembership" {
			continue
		}
		named++
		if element["ownedMemberName"] == nil || element["ownedMemberName"] != element["memberName"] {
			t.Errorf("%s: ownedMemberName %v, memberName %v", id, element["ownedMemberName"], element["memberName"])
		}
	}
	if named == 0 {
		t.Fatal("the model mints no connector end membership")
	}
}
