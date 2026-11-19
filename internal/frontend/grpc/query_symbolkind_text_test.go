package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const kindAndTextQueryModel = `package Demo {
	requirement def Mass { doc /* The mass shall be small. */ }
	requirement limit : Mass { doc /* First sentence. */ doc /* Second sentence. */ }
	requirement open;
	part rig { doc /* A rig, which is no requirement. */ }
	state def Lamp {
		entry; then off;
		state off;
		state on;
		transition first off then on;
		action { }
	}
}`

// TestQuerySymbolKindNamesEveryRow verifies symbolKind answers each row what
// GetSymbol's SymbolInfo.kind does, rows identified by position included.
func TestQuerySymbolKindNamesEveryRow(t *testing.T) {
	_, _, resp := runQueryOnSource(t, kindAndTextQueryModel, &pb.Query{
		Scope:  []string{"Demo"},
		Select: []string{QueryPropSymbolKind},
	})
	got := make(map[string]string, len(resp.Elements))
	for _, element := range resp.Elements {
		kind, ok := element.Properties[QueryPropSymbolKind]
		if !ok {
			t.Errorf("%s: symbolKind is absent", element.Id)
		}
		got[element.Id] = kind
	}
	for id, want := range map[string]string{
		"Demo":            "package",
		"Demo::Mass":      "requirementDef",
		"Demo::Mass::@0":  "documentation",
		"Demo::limit":     "requirementUsage",
		"Demo::rig":       "partUsage",
		"Demo::Lamp":      "stateDef",
		"Demo::Lamp::@0":  "actionUsage",
		"Demo::Lamp::@1":  "successionUsage",
		"Demo::Lamp::off": "stateUsage",
		"Demo::Lamp::@4":  "actionUsage",
		"Demo::Lamp::@5":  "actionUsage",
	} {
		if got[id] != want {
			t.Errorf("properties[symbolKind] of %s = %q, want %q", id, got[id], want)
		}
	}
}

// TestQuerySymbolKindSelectsRowsOfThatKind verifies symbolKind constrains as it projects.
func TestQuerySymbolKindSelectsRowsOfThatKind(t *testing.T) {
	_, _, resp := runQueryOnSource(t, kindAndTextQueryModel, &pb.Query{
		Scope: []string{"Demo"},
		Where: primitive(QueryPropSymbolKind, opEqual, false, "stateUsage"),
	})
	var ids []string
	for _, element := range resp.Elements {
		ids = append(ids, element.Id)
	}
	if len(ids) != 2 || ids[0] != "Demo::Lamp::off" || ids[1] != "Demo::Lamp::on" {
		t.Errorf("elements = %v, want Demo::Lamp::off and Demo::Lamp::on", ids)
	}
}

// TestQueryTextReadsRequirementText verifies text answers a requirement's doc
// bodies and is absent of a requirement stating none and of any other element.
func TestQueryTextReadsRequirementText(t *testing.T) {
	_, _, resp := runQueryOnSource(t, kindAndTextQueryModel, &pb.Query{
		Scope:  []string{"Demo"},
		Select: []string{QueryPropText, QueryPropDocumentation},
	})
	texts := map[string]string{}
	documented := map[string]bool{}
	for _, element := range resp.Elements {
		if text, ok := element.Properties[QueryPropText]; ok {
			texts[element.Id] = text
		}
		_, documented[element.Id] = element.Properties[QueryPropDocumentation]
	}
	want := map[string]string{
		"Demo::Mass":  "The mass shall be small.",
		"Demo::limit": "First sentence.",
	}
	if len(texts) != len(want) {
		t.Errorf("text is present on %v, want it on the two requirements stating one", texts)
	}
	for id, text := range want {
		if texts[id] != text {
			t.Errorf("properties[text] of %s = %q, want %q", id, texts[id], text)
		}
	}
	if !documented["Demo::rig"] {
		t.Error("Demo::rig lost its documentation property")
	}
}

// TestQueryStillRefusesAPropertyOutsideTheSet verifies the two additions leave
// every other undeclared name an unknown property.
func TestQueryStillRefusesAPropertyOutsideTheSet(t *testing.T) {
	for _, name := range []string{"symbolkind", "symbol_kind", "requirementText"} {
		_, err := runQuery(t, &pb.Query{Select: []string{name}})
		assertQueryError(t, err, QueryErrUnknownProperty)
	}
}
