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

// The compact form without derived properties leaves the completed ones and
// their marker out; with them it writes both, as the element array does.
func TestAPIJSONCompactOmitsCompletedProperties(t *testing.T) {
	graph := booleansGraph(t)
	for _, omit := range []bool{false, true} {
		compact, err := WriteAPIJSONCompact(graph, CompactAPIJSONOptions{OmitDerived: omit})
		if err != nil {
			t.Fatal(err)
		}
		text := string(compact)
		stated := strings.Contains(text, `"isLibraryElement"`)
		marked := strings.Contains(text, `"`+apiJSONCompletedPropertiesKey+`"`)
		if stated == omit || marked == omit {
			t.Errorf("OmitDerived=%v: isLibraryElement written %v, marker written %v", omit, stated, marked)
		}
	}
}
