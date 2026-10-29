package grpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestConvertLibraryCapabilities(t *testing.T) {
	srv := mustNewService(t, 10)
	if !srv.Supports(CapabilityConvertLibraryReferences) || !srv.Supports(CapabilityConvertLibraryStubs) {
		t.Fatal("library compatibility capabilities not advertised")
	}
	for _, target := range []string{"sysml", "ttl"} {
		_, err := srv.Convert(context.Background(), &pb.ConvertRequest{
			Source: &pb.ConvertRequest_Content{Content: "package P {}"}, FromFormat: "sysml", ToFormat: target, IncludeLibraryStubs: true,
		})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("target %s: %v", target, err)
		}
	}
}

func TestConvertLibraryTableForContentAndCachedDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	ctx := context.Background()
	const source = `package P { private import SI::*; attribute time = 20 [s]; }`
	direct, err := srv.Convert(ctx, &pb.ConvertRequest{Source: &pb.ConvertRequest_Content{Content: source}, FromFormat: "sysml", ToFormat: "api-json"})
	if err != nil || direct.Error != "" {
		t.Fatalf("convert: %v %v", direct, err)
	}
	if len(direct.LibraryReferences) == 0 {
		t.Fatal("inline conversion has no table")
	}
	parsed, err := srv.ParseSources(ctx, &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{
		{Source: &pb.SourceDocument_Content{Content: source}, Name: "p.sysml"},
		{Source: &pb.SourceDocument_Content{Content: "package Other {}"}, Name: "other.sysml"},
	}})
	if err != nil || parsed.Error != "" {
		t.Fatalf("parse sources: %v %v", parsed, err)
	}
	for _, docs := range [][]string{nil, {"p.sysml"}, {"other.sysml"}} {
		response, err := srv.Convert(ctx, &pb.ConvertRequest{Source: &pb.ConvertRequest_ModelHash{ModelHash: parsed.ModelHash}, ToFormat: "api-json", Documents: docs})
		if err != nil || response.Error != "" {
			t.Fatalf("cached conversion: %v %v", response, err)
		}
		if len(docs) != 0 && docs[0] == "other.sysml" {
			if len(response.LibraryReferences) != 0 {
				t.Fatalf("unreferenced library names leaked: %v", response.LibraryReferences)
			}
		} else if len(response.LibraryReferences) != len(direct.LibraryReferences) {
			t.Fatalf("cached table: %v", response.LibraryReferences)
		}
	}
}
