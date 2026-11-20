package export

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// impliedIncludedGraph is booleansGraph with isImpliedIncluded stated of its
// root elements, and the mapping's marker beside it when marked.
func impliedIncludedGraph(t *testing.T, marked bool) *rdf.Graph {
	t.Helper()
	graph := booleansGraph(t)
	roots := 0
	for _, subject := range graph.Subjects() {
		if !apiJSONRootSubject(graph, subject) {
			continue
		}
		roots++
		graph.Add(subject, rdf.SysMLTerm(pIsImpliedIncluded), rdf.Bool(true))
		if marked {
			graph.Add(subject, rdf.OpenSysMLTerm(xStatedDefaults), rdf.Bool(true))
		}
	}
	if roots == 0 {
		t.Fatal("the graph has no root element")
	}
	return graph
}

// A graph is an external writer's element form when it states
// isImpliedIncluded and the mapping's marker nowhere.
func TestExternalElementFormIsTheFlagWithoutTheMarker(t *testing.T) {
	element, other := rdf.ElementIRIForID("a"), rdf.ElementIRIForID("b")
	flag, marker := rdf.SysMLTerm(pIsImpliedIncluded), rdf.OpenSysMLTerm(xStatedDefaults)
	for _, test := range []struct {
		name     string
		triples  []rdf.Triple
		external bool
	}{
		{name: "neither"},
		{name: "flag", triples: []rdf.Triple{{Subject: element, Predicate: flag, Object: rdf.Bool(true)}}, external: true},
		{name: "false flag", triples: []rdf.Triple{{Subject: element, Predicate: flag, Object: rdf.Bool(false)}}, external: true},
		{name: "marker", triples: []rdf.Triple{{Subject: element, Predicate: marker, Object: rdf.Bool(true)}}},
		{name: "flag and marker", triples: []rdf.Triple{
			{Subject: element, Predicate: flag, Object: rdf.Bool(true)},
			{Subject: element, Predicate: marker, Object: rdf.Bool(true)},
		}},
		{name: "marker on another element", triples: []rdf.Triple{
			{Subject: element, Predicate: flag, Object: rdf.Bool(true)},
			{Subject: other, Predicate: marker, Object: rdf.Bool(true)},
		}},
		{name: "false marker", triples: []rdf.Triple{
			{Subject: element, Predicate: flag, Object: rdf.Bool(true)},
			{Subject: element, Predicate: marker, Object: rdf.Bool(false)},
		}, external: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			graph := rdf.NewGraph()
			for _, triple := range test.triples {
				graph.AddTriple(triple)
			}
			if got := externalElementForm(graph); got != test.external {
				t.Errorf("externalElementForm = %v, want %v", got, test.external)
			}
		})
	}
}

// A graph stating isImpliedIncluded beside the marker decodes as the graph
// without the flag does; without the marker it is an external writer's form.
func TestMarkedGraphStatingImpliedIncludedDecodesAsThisMapping(t *testing.T) {
	want, err := ToSysML(booleansGraph(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ToSysML(impliedIncludedGraph(t, true))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the marked graph decodes differently:\n%s\nwant:\n%s", got, want)
	}
	if control, err := ToSysML(impliedIncludedGraph(t, false)); err == nil && bytes.Equal(control, want) {
		t.Error("the unmarked graph decodes as the marked one: the marker decides nothing")
	}
}

// The element form of such a graph is the mapping's own, completed and marked,
// and reads back to the graph written; so does its Turtle.
func TestMarkedGraphStatingImpliedIncludedRoundTripsTheElementForm(t *testing.T) {
	graph := impliedIncludedGraph(t, true)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	marked, flagged := 0, 0
	for _, element := range booleansElements(t, document) {
		if element[apiJSONStatedDefaultsKey] == true {
			marked++
		}
		if element[pIsImpliedIncluded] == true {
			flagged++
		}
	}
	if marked == 0 || flagged == 0 {
		t.Fatalf("the document has %d marked and %d flagged elements, want both", marked, flagged)
	}
	back, err := ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if missing, added := tripleDiff(graph, back); len(missing)+len(added) > 0 {
		t.Errorf("the graph changed through the element form:\nmissing:\n%s\nadded:\n%s",
			strings.Join(missing, "\n"), strings.Join(added, "\n"))
	}
	again, err := WriteAPIJSON(back)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, document) {
		t.Error("the element form is not a fixed point of read and write")
	}
	want, err := ToSysML(booleansGraph(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := ToSysML(back)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the graph read back decodes differently:\n%s\nwant:\n%s", got, want)
	}
	reread, err := rdf.ParseTurtle(rdf.WriteTurtle(graph))
	if err != nil {
		t.Fatal(err)
	}
	if got, err = ToSysML(reread); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the Turtle read back decodes differently:\n%s\nwant:\n%s", got, want)
	}
}
