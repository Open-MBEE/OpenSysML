package export

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

const booleansModel = `package P {
    part def D {
        var attribute v : ScalarValues::Real;
        attribute n : ScalarValues::Real nonunique;
        part a : D;
        ref part b : D;
        attribute e : ScalarValues::Boolean = false;
    }
}
`

func booleansGraph(t *testing.T) *rdf.Graph {
	t.Helper()
	file := source.New("b.sysml", []byte(booleansModel))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func booleansElements(t *testing.T, document []byte) map[string]map[string]any {
	t.Helper()
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]any, len(elements))
	for _, element := range elements {
		byID[element["@id"].(string)] = element
	}
	return byID
}

// tripleDiff lists the triples of want absent from got and of got absent from want.
func tripleDiff(want, got *rdf.Graph) (missing, added []string) {
	// A library element the graph references is a bare reference in the element form.
	lines := func(graph *rdf.Graph) map[string]bool {
		set := make(map[string]bool, graph.Len())
		for _, triple := range graph.Triples() {
			if LibraryReference(graph, triple.Subject) {
				continue
			}
			set[triple.Subject.String()+" "+triple.Predicate.String()+" "+triple.Object.String()] = true
		}
		return set
	}
	wantSet, gotSet := lines(want), lines(got)
	for line := range wantSet {
		if !gotSet[line] {
			missing = append(missing, line)
		}
	}
	for line := range gotSet {
		if !wantSet[line] {
			added = append(added, line)
		}
	}
	sort.Strings(missing)
	sort.Strings(added)
	return missing, added
}

// Every non-derived boolean of an element's metaclass is stated, true or false,
// at the value the graph implies; derived ones only as the graph states them.
func TestAPIJSONStatesEveryNonDerivedBoolean(t *testing.T) {
	document, err := WriteAPIJSON(booleansGraph(t))
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for id, element := range elements {
		metaclass := apiJSONOwnedAbsenceMetaclass(element["@type"].(string))
		if metaclass == "" {
			continue
		}
		for _, property := range apiJSONOwnedAbsenceProperties(metaclass) {
			if property.Derived || property.Range != rdf.XSD+"boolean" {
				continue
			}
			if _, ok := element[property.Name].(bool); !ok {
				t.Errorf("%s (%s) does not state %s: %v", id, metaclass, property.Name, element[property.Name])
			}
		}
		if element["isImpliedIncluded"] != false {
			t.Errorf("%s states isImpliedIncluded %v, want false", id, element["isImpliedIncluded"])
		}
	}
	for id, want := range map[string]map[string]any{
		"P__D__a":           {"isComposite": true, "isUnique": true, "isVariable": false, "isAbstract": false, "isReference": nil},
		"P__D__b":           {"isComposite": false, "isReference": true},
		"P__D__v":           {"isVariable": true, "mayTimeVary": nil},
		"P__D__n":           {"isUnique": false},
		"P__D":              {"isAbstract": false, "isVariation": false, "isSufficient": false, "isIndividual": false},
		"P__D__a_om":        {"isImplied": false},
		"P__D__e_pvalue_om": {"isDefault": false, "isInitial": false, "isImplied": false},
		"P_ns":              {apiJSONStatedDefaultsKey: true},
		"P":                 {apiJSONStatedDefaultsKey: nil},
		"P__D__e_pvalue":    {"value": false, "isComposite": false},
	} {
		element, ok := elements[id]
		if !ok {
			t.Errorf("no element %s in %s", id, strings.Join(sortedKeys(elements), " "))
			continue
		}
		for key, value := range want {
			if got := element[key]; got != value {
				t.Errorf("%s.%s = %v, want %v", id, key, got, value)
			}
		}
	}
}

func sortedKeys(elements map[string]map[string]any) []string {
	keys := make([]string, 0, len(elements))
	for key := range elements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// A document carrying the marker reads back as the graph that was written: the
// completed booleans state no triple, while isComposite and a literal's value do.
func TestAPIJSONStatedDefaultsReadAsNoStatement(t *testing.T) {
	graph := booleansGraph(t)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the graph changed through the element form:\nmissing:\n%s\nadded:\n%s",
			strings.Join(missing, "\n"), strings.Join(added, "\n"))
	}
	for _, check := range []struct {
		id, property string
		value        bool
	}{{"P__D__a", "isComposite", true}, {"P__D__b", "isComposite", false}, {"P__D__e_pvalue", "value", false}} {
		subject := rdf.ElementIRIForID(check.id)
		if check.property == "value" {
			subject = rdf.IRI(rdf.Expression + check.id)
		}
		if !back.HasProperty(subject, rdf.SysML+check.property) {
			t.Errorf("%s states no %s after the round trip", check.id, check.property)
		}
	}
	if back.HasProperty(rdf.ElementIRIForID("P__D__a"), rdf.SysML+"isImpliedIncluded") {
		t.Error("isImpliedIncluded entered the graph")
	}
}

// Without the marker every stated boolean is a triple, as it always was, and a
// document that omits its false flags reads as the graph that wrote it.
func TestAPIJSONUnmarkedDocumentReadsAsBefore(t *testing.T) {
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
	}
	unmarked, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ReadAPIJSON(unmarked)
	if err != nil {
		t.Fatal(err)
	}
	a := rdf.ElementIRIForID("P__D__a")
	for _, property := range []string{"isAbstract", "isImpliedIncluded", "isUnique"} {
		if !back.HasProperty(a, rdf.SysML+property) {
			t.Errorf("unmarked document: stated %s is no triple", property)
		}
	}

	for _, element := range elements {
		metaclass := apiJSONOwnedAbsenceMetaclass(element["@type"].(string))
		for key, value := range element {
			flag, isBool := value.(bool)
			property, known := ontology.PropertyOf(metaclass, key)
			// An older export states a flag only when the graph does: never a default, never isVariable.
			if isBool && known && !apiJSONGraphStatedBooleans[key] &&
				(flag == (property.Default == "true") || key == "isVariable") {
				delete(element, key)
			}
		}
	}
	compact, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err = ReadAPIJSON(compact)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the compact unmarked document reads differently:\nmissing:\n%s\nadded:\n%s",
			strings.Join(missing, "\n"), strings.Join(added, "\n"))
	}
}

// A boolean the graph states at the value the writer would imply anyway is
// listed in sysx:explicitDefaults and comes back as the statement it is.
func TestAPIJSONExplicitDefaultsSurviveTheRoundTrip(t *testing.T) {
	graph := booleansGraph(t)
	explicit := rdf.ElementIRIForID("P__D")
	graph.Add(explicit, rdf.SysMLTerm("isAbstract"), rdf.Bool(false))
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	element := booleansElements(t, document)["P__D"]
	if listed, _ := element[apiJSONExplicitDefaultsKey].([]any); len(listed) != 1 || listed[0] != "isAbstract" {
		t.Errorf("%s = %v, want [isAbstract]", apiJSONExplicitDefaultsKey, element[apiJSONExplicitDefaultsKey])
	}
	back, err := ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the explicit default did not survive:\nmissing:\n%s\nadded:\n%s",
			strings.Join(missing, "\n"), strings.Join(added, "\n"))
	}
}

// A graph that is already an external writer's element form (it states
// isImpliedIncluded) is written as it is: nothing completed, no marker, and it
// reads back unchanged.
func TestAPIJSONExternalFormGraphIsWrittenAsItIs(t *testing.T) {
	data, err := os.ReadFile("../../../tests/export/testdata/interchange/flow_ends.toolkit.full.json")
	if err != nil {
		t.Skip(err)
	}
	graph, err := ReadAPIJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	for id, element := range booleansElements(t, document) {
		for _, key := range []string{apiJSONStatedDefaultsKey, apiJSONExplicitDefaultsKey} {
			if _, ok := element[key]; ok {
				t.Errorf("%s carries %s", id, key)
			}
		}
	}
	back, err := ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the external form changed through the writer:\nmissing:\n%s\nadded:\n%s",
			strings.Join(missing, "\n"), strings.Join(added, "\n"))
	}
}
