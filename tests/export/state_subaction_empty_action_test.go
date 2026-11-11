package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A bare `entry;` performs an anonymous empty ActionUsage its StateSubactionMembership owns
// (SysML.xtext EmptyActionUsage), as the pilot exports it; both readers write it back as `entry;`.
func TestBareStateSubactionStatesItsEmptyAction(t *testing.T) {
	const model = "package P {\n    state def S {\n        entry;\n        then idle;\n        state idle;\n        do;\n        exit;\n    }\n}\n"
	turtle, err := convert.Convert("s.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	kinds := map[string]bool{}
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.RDFType || triple.Object.Value != rdf.SysML+"StateSubactionMembership" {
			continue
		}
		membership := triple.Subject
		if owner, _ := graph.Object(membership, rdf.SysML+"owner"); owner.Value == "" {
			continue // a library membership the graph names, not one of S's
		}
		kind, _ := graph.Lexical(membership, rdf.SysML+"kind")
		member, ok := graph.Object(membership, rdf.SysML+"memberElement")
		if !ok {
			t.Errorf("the %s membership owns no member", kind)
			continue
		}
		if meta, _ := graph.Object(member, rdf.RDFType); meta.Value != rdf.SysML+"ActionUsage" {
			t.Errorf("the %s member is a %s, want ActionUsage", kind, meta.Value)
		}
		if !graph.BoolValue(member, rdf.SysML+"isComposite") {
			t.Errorf("the %s action is not composite", kind)
		}
		if owning, _ := graph.Object(member, rdf.SysML+"owningMembership"); owning != membership {
			t.Errorf("the %s action's owningMembership = %v, want its subaction membership", kind, owning)
		}
		kinds[kind] = true
	}
	if len(kinds) != 3 {
		t.Fatalf("subaction kinds stated: %v, want entry, do and exit", kinds)
	}
	back, err := convert.Convert("s.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("turtle back: %v", err)
	}
	if string(back) != model {
		t.Errorf("turtle round trip:\n%s", back)
	}
	document, err := convert.Convert("s.sysml", []byte(model), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	if !strings.Contains(string(document), `"StateSubactionMembership"`) {
		t.Fatal("api-json states no StateSubactionMembership")
	}
	back, err = convert.Convert("s.json", document, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json back: %v", err)
	}
	if string(back) != model {
		t.Errorf("api-json round trip:\n%s", back)
	}
}

// An anonymous subaction written with a body, `entry action { }`, keeps its own declaration: the membership is
// the positioned element and both readers write the braced form back, not the bare keyword.
func TestBracedEmptySubactionKeepsItsDeclaration(t *testing.T) {
	const model = "package P {\n    action def A;\n    state def S {\n        entry action { }\n        do action : A;\n        exit;\n    }\n}\n"
	turtle, err := convert.Convert("p.sysml", []byte(model), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), "sysx:positionalName \"P::S::@0\"") {
		t.Fatalf("the braced subaction's membership should take the position:\n%s", turtle)
	}
	for name, format := range map[string]convert.Format{"p.ttl": convert.FormatTurtle, "p.json": convert.FormatAPIJSON} {
		document, err := convert.Convert("p.sysml", []byte(model), convert.FormatSysML, format)
		if err != nil {
			t.Fatalf("to %s: %v", name, err)
		}
		back, err := convert.Convert(name, document, format, convert.FormatSysML)
		if err != nil {
			t.Fatalf("%s back to notation: %v", name, err)
		}
		if string(back) != model {
			t.Fatalf("%s round trip:\n%s", name, back)
		}
	}
}
