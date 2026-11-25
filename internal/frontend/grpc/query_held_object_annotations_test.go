package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const heldAnnotationsModel = `package Demo {
	private import ScalarValues::*;
	metadata def Tag { attribute label : String; }
	part device;
	@Tag about device { label = "alpha"; }
}`

// A held object selects the annotations of the declaration it stands for, both
// for projection and for the single value the Query RPC reports.
func TestQueryRPCReadsAnnotationsOfAHeldObject(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: heldAnnotationsModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	hash := parsed.ModelHash
	holdObject(t, srv, hash, "Demo::device")
	iri := "<" + rdf.Element + rdf.EncodeElementID("Demo::device") + ">"
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: hash,
		Query: &pb.Query{
			Where:  primitive(QueryPropType, opIn, false, iri),
			Select: []string{"@annotations"},
		},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(resp.Elements) != 1 {
		t.Fatalf("object rows = %v, want the device alone", resp.Elements)
	}
	if got := resp.Elements[0].Properties["@annotations"]; got != "Demo::Tag::label=alpha" {
		t.Fatalf("annotations of the held device = %q, want Demo::Tag::label=alpha", got)
	}
	resp, err = srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: hash,
		Query: &pb.Query{
			Where:  primitive(QueryPropType, opIn, false, iri),
			Select: []string{"@annotations"},
		},
	})
	if err != nil {
		t.Fatalf("Query with a filter: %v", err)
	}
	found := false
	for _, row := range resp.Elements {
		if row.Properties["@annotations"] == "Demo::Tag::label=alpha" {
			found = true
		}
	}
	if !found {
		t.Fatalf("annotations after a held-object filter = %v", resp.Elements)
	}
}
