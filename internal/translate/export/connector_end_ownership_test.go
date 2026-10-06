package export

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A connection usage owns the unnamed ends its `connect` clause writes
// (SysML v2 8.2.2.13.1), so the API JSON lists them under ownedMember as it does
// under connectorEnd and ownedEndFeature, and each end names the usage as owner.
func TestAPIJSONConnectorEndsAreOwnedMembers(t *testing.T) {
	const model = `package Reflect {
    port def P;
    part def Source { port y : P; }
    part def Sink { port u : P; }
    connection def C { end source[1] : P; end target[1] : P; }
    part def Asm {
        part s : Source;
        part k : Sink;
        connection c1 : C connect [1] s.y to [1] k.u;
    }
}
`
	file := source.New("reflect.sysml", []byte(model))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	data, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var objects []map[string]any
	if err := json.Unmarshal(data, &objects); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]any, len(objects))
	var connection map[string]any
	for _, object := range objects {
		byID[object["@id"].(string)] = object
		if object["@type"] == "ConnectionUsage" {
			connection = object
		}
	}
	if connection == nil {
		t.Fatal("the export has no ConnectionUsage")
	}
	ids := func(key string) []string {
		refs, _ := connection[key].([]any)
		out := make([]string, 0, len(refs))
		for _, ref := range refs {
			out = append(out, ref.(map[string]any)["@id"].(string))
		}
		return out
	}
	ends := ids("connectorEnd")
	if len(ends) != 2 {
		t.Fatalf("connectorEnd = %v, want two ends", ends)
	}
	for _, key := range []string{"ownedEndFeature", "ownedFeature", "ownedMember"} {
		if got := ids(key); !slices.Equal(got, ends) {
			t.Errorf("%s = %v, want the connector ends %v", key, got, ends)
		}
	}
	for _, id := range ends {
		end := byID[id]
		if end["@type"] != "ReferenceUsage" {
			t.Errorf("end %s is a %v, want a ReferenceUsage", id, end["@type"])
		}
		if owner, _ := end["owner"].(map[string]any); owner["@id"] != connection["@id"] {
			t.Errorf("end %s has owner %v, want the connection %v", id, owner, connection["@id"])
		}
	}
	// The ownership the export states is the writer's own, and reads back.
	if _, err := ToSysML(graph); err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
}
