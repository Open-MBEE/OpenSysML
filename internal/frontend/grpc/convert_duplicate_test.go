package grpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A model declaring one name twice in a namespace converts: both members are
// their own elements, the name-conflict warnings are reported, and neither is
// an error.
func TestConvertDuplicateMemberNames(t *testing.T) {
	srv := mustNewService(t, 10)
	const src = `package P {
	private import ScalarValues::*;
	part def A { attribute x : Real; }
	part def A { attribute y : Real; }
}`
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: src},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	var conflicts int
	for _, d := range parsed.Diagnostics {
		if d.Severity != "warning" {
			t.Errorf("diagnostic %q is a %s, want a warning", d.Message, d.Severity)
		}
		if d.Code == "name-conflict" {
			conflicts++
		}
	}
	if conflicts != 2 {
		t.Fatalf("got %d name-conflict warnings, want 2: %v", conflicts, parsed.Diagnostics)
	}
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:     &pb.ConvertRequest_Content{Content: src},
		FromFormat: "sysml",
		ToFormat:   "api-json",
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("the duplicate-name model was refused: %s", resp.Error)
	}
	var elements []map[string]any
	if err := json.Unmarshal([]byte(resp.Content), &elements); err != nil {
		t.Fatalf("the API JSON does not decode: %v\n%s", err, resp.Content)
	}
	var defs []map[string]any
	for _, el := range elements {
		if el["@type"] == "PartDefinition" && el["declaredName"] == "A" {
			defs = append(defs, el)
		}
	}
	if len(defs) != 2 {
		t.Fatalf("got %d `part def A` elements, want 2:\n%s", len(defs), resp.Content)
	}
	if defs[0]["@id"] == defs[1]["@id"] {
		t.Errorf("the two `part def A` share an @id %v", defs[0]["@id"])
	}
	if defs[0]["qualifiedName"] != "P::A" || !strings.HasPrefix(defs[1]["qualifiedName"].(string), "P::@") {
		t.Errorf("qualified names are %v and %v, want P::A and the positional name", defs[0]["qualifiedName"], defs[1]["qualifiedName"])
	}
}
