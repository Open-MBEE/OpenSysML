package grpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestOSLCQueryText(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: queryModel},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		OslcQuery: `oslc.where=rdf:type="PartUsage"&oslc.select=sysml:name`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Elements) != 3 || resp.Elements[0].Id != "Demo::vehicle" {
		t.Fatalf("elements = %#v", resp.Elements)
	}
	if resp.Elements[0].Properties[QueryPropName] != "vehicle" {
		t.Fatalf("properties = %#v", resp.Elements[0].Properties)
	}
}

func TestOSLCQueryMutuallyExclusive(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: queryModel},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{},
		OslcQuery: `sysml:type=PartUsage`,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("error = %v, want INVALID_ARGUMENT", err)
	}
}
