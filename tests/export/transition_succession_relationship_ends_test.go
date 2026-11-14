package export_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// The succession a transition owns states its relationship ends as the pilot derives them — the
// transition's source and target — for a `transition`, a state body's `then x;` and a guarded target.
func TestTransitionSuccessionStatesItsRelationshipEnds(t *testing.T) {
	const model = `package P {
    state def S {
        state idle;
        then running;
        state running;
        transition go first running then busy;
        state busy;
        transition if true then idle;
    }
}`
	turtle, err := convert.Convert("p.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	want := map[string][2]string{
		"P__S__go_succession": {"P__S__running", "P__S__busy"},
	}
	transitions := 0
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.SysML+"succession" {
			continue
		}
		transitions++
		transition, succession := triple.Subject, triple.Object
		source, _ := graph.Object(transition, rdf.SysML+"source")
		target, _ := graph.Object(transition, rdf.SysML+"target")
		if !source.IsIRI() || !target.IsIRI() {
			t.Errorf("%s states source %v, target %v, want both", transition.Value, source, target)
			continue
		}
		if w, ok := want[rdf.LocalName(succession.Value)]; ok {
			if rdf.LocalName(source.Value) != w[0] || rdf.LocalName(target.Value) != w[1] {
				t.Errorf("%s leaves %s for %s, want %s for %s", transition.Value, source.Value, target.Value, w[0], w[1])
			}
		}
		for property, wantTerm := range map[string]rdf.Term{
			"sourceFeature": source, "source": source, "targetFeature": target, "target": target,
		} {
			if got, ok := graph.Object(succession, rdf.SysML+property); !ok || got != wantTerm {
				t.Errorf("%s %s = %v (%t), want %v", succession.Value, property, got, ok, wantTerm)
			}
		}
		for _, property := range []string{"relatedFeature", "relatedElement"} {
			got := graph.Objects(succession, rdf.SysML+property)
			if len(got) != 2 || got[0] != source || got[1] != target {
				t.Errorf("%s %s = %v, want [%v %v]", succession.Value, property, got, source, target)
			}
		}
	}
	if transitions != 3 {
		t.Errorf("%d transitions own a succession, want 3", transitions)
	}
}

// A `then x;` after an unnamed state redefinition beside the state it is named for leaves the
// redefinition, the member before it by position; a guarded `if g then x;` after a state leaves that state.
func TestTransitionAfterUnnamedRedefinitionLeavesTheRedefinition(t *testing.T) {
	const src = "package P {\n    state def Base {\n        state walk;\n    }\n    state def A specializes Base {\n        state walk;\n" +
		"        state redefines walk;\n        then busy;\n        state busy;\n        if true then walk;\n    }\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v\n%s", err, turtle)
	}
	if string(back) != src {
		t.Fatalf("the notation changed\n--- want ---\n%s--- got ---\n%s", src, back)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	var redefinition, busy rdf.Term
	for _, triple := range graph.Triples() {
		switch {
		case triple.Predicate.Value == rdf.SysML+"ownedRedefinition":
			redefinition = triple.Subject
		case triple.Predicate.Value == rdf.SysML+"declaredName" && triple.Object.Value == "busy":
			busy = triple.Subject
		}
	}
	if !redefinition.IsIRI() || !busy.IsIRI() {
		t.Fatalf("redefinition %v, busy %v\n%s", redefinition, busy, turtle)
	}
	transitions := 0
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.SysML+"succession" {
			continue
		}
		transitions++
		want := redefinition
		if target, _ := graph.Object(triple.Subject, rdf.SysML+"target"); target != busy {
			want = busy
		}
		for _, subject := range []rdf.Term{triple.Subject, triple.Object} {
			if got, ok := graph.Object(subject, rdf.SysML+"source"); !ok || got != want {
				t.Errorf("%s source = %v (%t), want %s", subject.Value, got, ok, want.Value)
			}
		}
	}
	if transitions != 2 {
		t.Errorf("%d transitions own a succession, want 2", transitions)
	}
}
