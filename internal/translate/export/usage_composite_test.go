package export

import (
	"encoding/json"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestFullAPIJSONDerivesEffectiveCompositeStateUsages(t *testing.T) {
	const model = `package P {
		part def Vehicle {
			state machine {
				state running {
					state idle;
				}
			}
		}
		action def Flow {
			fork f;
		}
	}`
	file := source.New("usage-composite.sysml", []byte(model))
	parse := parser.New(file)
	root := parse.ParseFile()
	if len(parse.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", parse.Diagnostics)
	}
	graph, err := ToRDFWith(file, root, IDUUID)
	if err != nil {
		t.Fatalf("ToRDFWith: %v", err)
	}
	for _, name := range []string{"P::Vehicle::machine::running::idle"} {
		var subject rdf.Term
		found := false
		for _, candidate := range graph.Subjects() {
			if qualifiedName, ok := graph.Lexical(candidate, rdf.SysML+"qualifiedName"); ok && qualifiedName == name {
				subject = candidate
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no RDF subject for %s", name)
		}
		if composite, ok := graph.Object(subject, rdf.SysML+"isComposite"); ok {
			t.Errorf("%s has mapping-stated isComposite %v; the state mapping should leave it unstated", name, composite)
		}
	}
	var fork rdf.Term
	for _, candidate := range graph.Subjects() {
		if qualifiedName, ok := graph.Lexical(candidate, rdf.SysML+"qualifiedName"); ok && qualifiedName == "P::Flow::f" {
			fork = candidate
			break
		}
	}
	if fork.Value == "" {
		t.Fatal("no RDF subject for P::Flow::f")
	}
	composite, ok := graph.Object(fork, rdf.SysML+"isComposite")
	if !ok || composite.Kind != rdf.TermLiteral || composite.Value != "true" {
		t.Errorf("control node isComposite = %v (present %t), want mapping-stated true", composite, ok)
	}
	if reference, ok := graph.Object(fork, rdf.SysML+"isReference"); ok {
		t.Errorf("control node isReference = %v, want absent from Turtle", reference)
	}
	full, err := ModelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDUUID, APIJSONFull)
	if err != nil {
		t.Fatalf("ModelToAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"P::Vehicle::machine::running::idle", "P::Flow::f"} {
		found := false
		for _, element := range elements {
			var qualifiedName string
			if json.Unmarshal(element["qualifiedName"], &qualifiedName) != nil || qualifiedName != name {
				continue
			}
			found = true
			var composite, reference bool
			if err := json.Unmarshal(element["isComposite"], &composite); err != nil || !composite {
				t.Errorf("%s full isComposite = %s, want true", name, element["isComposite"])
			}
			if err := json.Unmarshal(element["isReference"], &reference); err != nil || reference {
				t.Errorf("%s full isReference = %s, want false", name, element["isReference"])
			}
		}
		if !found {
			t.Errorf("full API JSON has no element qualifiedName %s", name)
		}
	}
}
