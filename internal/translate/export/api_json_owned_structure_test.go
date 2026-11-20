package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func ownedStructureElements(t *testing.T, options APIJSONOptions) ([]byte, []map[string]any) {
	t.Helper()
	graph := duplicateGraph(t, inheritedMembersModel, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, options)
	if err != nil {
		t.Fatalf("WriteAPIJSONWithOptions: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return document, elements
}

// The default document is the complete one less the inherited families and their names in
// sysx:completedProperties: same elements, same order, every other property equal.
func TestAPIJSONOwnedStructureIsTheCompleteFormLessTheInheritedFamilies(t *testing.T) {
	_, owned := ownedStructureElements(t, APIJSONOptions{})
	_, complete := ownedStructureElements(t, APIJSONOptions{InheritedMembers: true})
	if len(owned) != len(complete) || len(owned) == 0 {
		t.Fatalf("%d elements by default, %d with inherited members", len(owned), len(complete))
	}
	dropped := 0
	for i, want := range complete {
		got := owned[i]
		for family := range apiJSONInheritedFamilies {
			if _, ok := got[family]; ok {
				t.Errorf("%v carries %s by default", got["@id"], family)
			}
			if _, ok := want[family]; ok {
				dropped++
				delete(want, family)
			}
		}
		var names []any
		for _, name := range anySlice(want[apiJSONCompletedPropertiesKey]) {
			if !apiJSONInheritedFamilies[name.(string)] {
				names = append(names, name)
			}
		}
		delete(want, apiJSONCompletedPropertiesKey)
		if names != nil {
			want[apiJSONCompletedPropertiesKey] = names
		}
		wantJSON, _ := json.Marshal(want)
		gotJSON, _ := json.Marshal(got)
		if !bytes.Equal(wantJSON, gotJSON) {
			t.Errorf("%v differs beyond the inherited families:\n got %s\nwant %s", got["@id"], gotJSON, wantJSON)
		}
	}
	if dropped == 0 {
		t.Fatal("the complete form carries no inherited family: the case proves nothing")
	}
}

func anySlice(value any) []any {
	items, _ := value.([]any)
	return items
}

// A family the graph itself states is the model's own statement, written either way.
func TestAPIJSONOwnedStructureKeepsAStatedFamily(t *testing.T) {
	graph := duplicateGraph(t, inheritedMembersModel, IDQualifiedName)
	named := map[string]rdf.Term{}
	for _, subject := range graph.Subjects() {
		if name, ok := graph.Lexical(subject, rdf.SysML+"declaredName"); ok && graph.Type(subject) == rdf.SysML+"PartDefinition" {
			named[name] = subject
		}
	}
	car, engine := named["Car"], named["Engine"]
	graph.Add(car, rdf.SysMLTerm("member"), engine)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	element := booleansElements(t, document)["Probe__Car"]
	if got := referenceIDs(t, element, "member"); !slices.Equal(got, []string{"Probe__Engine"}) {
		t.Errorf("Car.member = %v, want the stated Probe__Engine", got)
	}
	if _, ok := element["membership"]; ok {
		t.Error("Car carries membership, which the graph does not state")
	}
}

type failingWriter struct {
	after int
	err   error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.after -= len(p); w.after < 0 {
		return 0, w.err
	}
	return len(p), nil
}

// The streamed writer frames the array as the indented encoding of the whole does, and
// returns the destination's error.
func TestWriteAPIJSONToFramesTheArrayAndReportsTheWriteError(t *testing.T) {
	for name, source := range map[string]string{"several": inheritedMembersModel, "one": "package Lone;"} {
		graph := duplicateGraph(t, source, IDQualifiedName)
		for _, options := range []APIJSONOptions{{}, {InheritedMembers: true}, {IncludeLibraryStubs: true}} {
			elements, err := apiJSONElementsWithOptions(graph, options)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			var compact, want bytes.Buffer
			w := apiJSONWriter{buf: &compact, enc: json.NewEncoder(&compact)}
			compact.WriteByte('[')
			for i, element := range elements {
				if i > 0 {
					compact.WriteByte(',')
				}
				if err := w.object(element); err != nil {
					t.Fatal(err)
				}
			}
			compact.WriteByte(']')
			if err := json.Indent(&want, compact.Bytes(), "", "  "); err != nil {
				t.Fatal(err)
			}
			want.WriteByte('\n')
			var got bytes.Buffer
			if err := WriteAPIJSONTo(&got, graph, options); err != nil {
				t.Fatalf("%s: WriteAPIJSONTo: %v", name, err)
			}
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Errorf("%s %+v: the streamed document differs from the array indented whole", name, options)
			}
			full := errors.New("disk full")
			if err := WriteAPIJSONTo(&failingWriter{after: got.Len() / 2, err: full}, graph, options); !errors.Is(err, full) {
				t.Errorf("%s: a failed write returned %v", name, err)
			}
		}
	}
}

// Legacy stubs follow the document: the owned structure writes a stub for each library element
// its table names and no other, and both tables are the ones written without stubs.
func TestAPIJSONLegacyStubsFollowTheOwnedStructure(t *testing.T) {
	graph := duplicateGraph(t, "package P { private import SI::*; part def V; part car : V { attribute time = 20 [s]; } }", IDUUID)
	sizes := map[bool]int{}
	for _, inherited := range []bool{false, true} {
		tables := map[bool][]APIJSONLibraryReference{}
		stubs := map[string]bool{}
		for _, legacy := range []bool{false, true} {
			document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: inherited, IncludeLibraryStubs: legacy,
				LibraryReferences: func(refs []APIJSONLibraryReference) { tables[legacy] = refs }})
			if err != nil {
				t.Fatal(err)
			}
			var elements []map[string]any
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatal(err)
			}
			for _, element := range elements {
				if element["isLibraryElement"] == true {
					stubs[element["@id"].(string)] = true
				}
			}
		}
		if !reflect.DeepEqual(tables[true], tables[false]) {
			t.Errorf("inherited=%v: the table with stubs %v differs from the one without %v", inherited, tables[true], tables[false])
		}
		named := map[string]bool{}
		for _, ref := range tables[false] {
			named[ref.ID] = true
		}
		if len(named) == 0 || !reflect.DeepEqual(stubs, named) {
			t.Errorf("inherited=%v: %d stubs for a table of %d, want one each", inherited, len(stubs), len(named))
		}
		sizes[inherited] = len(named)
	}
	if sizes[false] >= sizes[true] {
		t.Errorf("the owned structure names %d library elements, the complete form %d; want fewer", sizes[false], sizes[true])
	}
}
