package grpc_test

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// conjugatedPortTypingModel types a port by the conjugate of Cmd, so cmdIn owns a
// ConjugatedPortTyping of the implied Cmd::'~Cmd', which owns a PortConjugation of Cmd.
const conjugatedPortTypingModel = `package P {
    port def Cmd;
    part x { port cmdIn : ~Cmd; }
}`

// apiJSONElementOfType is the one element of the metaclass in an API-JSON export.
func apiJSONElementOfType(t *testing.T, exported []byte, metaclass string) map[string]any {
	t.Helper()
	var objects []map[string]any
	if err := json.Unmarshal(exported, &objects); err != nil {
		t.Fatalf("api-json: %v", err)
	}
	var found []map[string]any
	for _, object := range objects {
		if object["@type"] == metaclass {
			found = append(found, object)
		}
	}
	if len(found) != 1 {
		t.Fatalf("api-json exports %d %s elements, want 1", len(found), metaclass)
	}
	return found[0]
}

// The export and GetElementProperties agree that a ConjugatedPortTyping's conjugatedPortDefinition is the element
// its type names (SysML v2 8.3.12.3) and its portDefinition the conjugate's original, as is the PortConjugation's.
func TestGRPCConjugatedPortTypingDefinitions(t *testing.T) {
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: conjugatedPortTypingModel}, Language: "sysml"})
	if err != nil || parsed.GetError() != "" {
		t.Fatalf("ParseFile: %v %s", err, parsed.GetError())
	}
	exported, err := convert.Convert("conjugated.sysml", []byte(conjugatedPortTypingModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	conjugate := apiJSONElementOfType(t, exported, "ConjugatedPortDefinition")
	typing := apiJSONElementOfType(t, exported, "ConjugatedPortTyping")
	conjugation := apiJSONElementOfType(t, exported, "PortConjugation")
	const original, conjugateName = "P::Cmd", "P::Cmd::'~Cmd'"
	if name := conjugate["qualifiedName"]; name != conjugateName {
		t.Fatalf("ConjugatedPortDefinition qualifiedName = %v, want %s", name, conjugateName)
	}
	for _, tc := range []struct {
		element  map[string]any
		property string
		want     string // the qualified name of the one element answered
	}{
		{typing, "conjugatedPortDefinition", conjugateName},
		{typing, "type", conjugateName},
		{typing, "portDefinition", original},
		{conjugation, "originalPortDefinition", original},
	} {
		metaclass, _ := tc.element["@type"].(string)
		id, _ := tc.element["@id"].(string)
		wantID, _ := apiJSONElementOfType(t, exported, map[string]string{original: "PortDefinition", conjugateName: "ConjugatedPortDefinition"}[tc.want])["@id"].(string)
		got := apiJSONReferenceIDs(tc.element[tc.property])
		if !slices.Equal(got, []string{wantID}) {
			t.Errorf("api-json %s.%s = %v, want [%s] (%s)", metaclass, tc.property, got, wantID, tc.want)
		}
		if ids := elementPropertyIDs(t, srv, parsed.ModelHash, id, tc.property); !slices.Equal(ids, []string{tc.want}) {
			t.Errorf("GetElementProperties(%s).%s = %v, want [%s]", metaclass, tc.property, ids, tc.want)
		}
	}
}

// apiJSONReferenceIDs are the "@id"s an API-JSON property value references.
func apiJSONReferenceIDs(value any) []string {
	switch v := value.(type) {
	case map[string]any:
		id, _ := v["@id"].(string)
		return []string{id}
	case []any:
		var ids []string
		for _, member := range v {
			ids = append(ids, apiJSONReferenceIDs(member)...)
		}
		return ids
	}
	return nil
}
