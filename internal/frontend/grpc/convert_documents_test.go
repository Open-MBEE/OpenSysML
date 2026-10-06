package grpc

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// documentsModel parses a model of three documents, one importing another.
func documentsModel(t *testing.T, srv *Service) string {
	t.Helper()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"lib.sysml", "package Lib { part def Engine { attribute rpm : ScalarValues::Integer; } }\n",
			"app.sysml", "package App { private import Lib::*; part e : Engine { attribute :>> rpm = 3; } }\n",
			"fleet.sysml", "package Fleet { part spare :> App::e; }\n"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	return parsed.ModelHash
}

// convertElements converts a model to API JSON with only documents written,
// and keys each element by its @id.
func convertElements(t *testing.T, srv *Service, hash string, documents ...string) map[string]string {
	t.Helper()
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:    &pb.ConvertRequest_ModelHash{ModelHash: hash},
		ToFormat:  "api-json",
		Documents: documents,
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("Convert %v: %v %s", documents, err, resp.GetError())
	}
	var elements []map[string]any
	if err := json.Unmarshal([]byte(resp.Content), &elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]string{}
	for _, e := range elements {
		text, _ := json.Marshal(e)
		byID[e["@id"].(string)] = string(text)
	}
	return byID
}

// documents writes only the elements the named documents declare, and the
// conversions of each document alone are the whole model's, element by element.
func TestConvertDocumentsWritesTheNamedOnes(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	whole := convertElements(t, srv, hash)
	app := convertElements(t, srv, hash, "app.sysml")
	if _, ok := app["Lib__Engine"]; ok {
		t.Error("Lib::Engine is written though only app.sysml is named")
	}
	if _, ok := app["App__e"]; !ok {
		t.Errorf("App::e is not written: %v", app)
	}
	parts := map[string]string{}
	for _, document := range []string{"lib.sysml", "app.sysml", "fleet.sysml"} {
		for id, element := range convertElements(t, srv, hash, document) {
			if _, taken := parts[id]; taken {
				t.Errorf("%s writes %s, which another document also wrote", document, id)
			}
			parts[id] = element
		}
	}
	if len(parts) != len(whole) {
		t.Errorf("the documents written one at a time write %d elements, the whole model %d", len(parts), len(whole))
	}
	for id, element := range whole {
		if parts[id] != element {
			t.Errorf("%s: the whole model writes\n%s\nits document alone writes\n%s", id, element, parts[id])
		}
	}
	if both := convertElements(t, srv, hash, "lib.sysml", "app.sysml", "fleet.sysml"); len(both) != len(whole) {
		t.Errorf("naming every document writes %d elements, the whole model %d", len(both), len(whole))
	}
}

// The capability is the newest, appended after every one before it.
func TestConvertDocumentsCapabilityIsAdvertised(t *testing.T) {
	if all := Capabilities(); all[len(all)-1] != CapabilityConvertDocuments {
		t.Errorf("capabilities %v do not end with %q, the newest", all, CapabilityConvertDocuments)
	}
}

// documents is refused where it names nothing the model holds.
func TestConvertDocumentsRefusals(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	hash := documentsModel(t, srv)
	for name, req := range map[string]*pb.ConvertRequest{
		"unknown document": {Source: &pb.ConvertRequest_ModelHash{ModelHash: hash}, ToFormat: "api-json", Documents: []string{"other.sysml"}},
		"content":          {Source: &pb.ConvertRequest_Content{Content: "package P;"}, FromFormat: "sysml", ToFormat: "api-json", Documents: []string{"p.sysml"}},
	} {
		if _, err := srv.Convert(context.Background(), req); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: err = %v, want INVALID_ARGUMENT", name, err)
		}
	}
}

// A model of one document named by documents converts whole, as without it.
func TestConvertDocumentsOfAModelOfOneDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments("lib.sysml", "package Lib { part def Engine; }\n"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	named, whole := convertElements(t, srv, parsed.ModelHash, "lib.sysml"), convertElements(t, srv, parsed.ModelHash)
	if len(named) == 0 || len(named) != len(whole) {
		t.Errorf("naming the one document writes %d elements, the whole model %d", len(named), len(whole))
	}
}
