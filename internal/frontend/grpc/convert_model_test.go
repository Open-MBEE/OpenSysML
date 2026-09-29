package grpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Convert of a model parsed from several documents writes them as one graph:
// a reference from one document to an element another declares links it
// (#653). A notation target, which would have to be one file, is refused.
func TestConvertModelOfSeveralDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	defer srv.Close()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{
		Documents: inlineDocuments(
			"lib.sysml", "package Lib { part def Engine; }\n",
			"app.sysml", "package App { private import Lib::*; part e : Engine; }\n"),
	})
	if err != nil {
		t.Fatalf("ParseSources: %v", err)
	}
	resp, err := srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash},
		ToFormat: "api-json",
	})
	if err != nil || resp.Error != "" {
		t.Fatalf("Convert: %v %s", err, resp.GetError())
	}
	var elements []map[string]any
	if err := json.Unmarshal([]byte(resp.Content), &elements); err != nil {
		t.Fatal(err)
	}
	var engine string
	var typed any
	for _, e := range elements {
		if e["qualifiedName"] == "Lib::Engine" {
			engine, _ = e["@id"].(string)
		}
		if e["@type"] == "FeatureTyping" {
			typed = e["type"]
		}
	}
	if ref, _ := typed.(map[string]any); engine == "" || ref["@id"] != engine {
		t.Errorf("e is typed by %v, want the @id of Lib::Engine %q", typed, engine)
	}

	_, err = srv.Convert(context.Background(), &pb.ConvertRequest{
		Source:   &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash},
		ToFormat: "sysml",
	})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "one document") {
		t.Errorf("a notation target for two documents: err = %v, want a FAILED_PRECONDITION naming the one-document limit", err)
	}
}
