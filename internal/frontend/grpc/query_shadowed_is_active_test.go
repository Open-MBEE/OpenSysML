package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const stringShadowModel = `package Demo {
	private import ScalarValues::*;
	part a { attribute isActive : String = "ready"; }
}`

// A bare name that shadows a Boolean built-in projects and filters as the
// feature's own text; the explicit "@" spelling keeps the built-in's type.
func TestQueryFiltersAndProjectsAStringShadowingIsActive(t *testing.T) {
	_, _, resp := runQueryOnSource(t, stringShadowModel, &pb.Query{
		Scope:  []string{"Demo::a"},
		Select: []string{QueryPropIsActive},
	})
	rows := rowsByID(t, resp)
	if got := rows["Demo::a"].Properties[QueryPropIsActive]; got != "ready" {
		t.Fatalf("isActive of Demo::a = %q, want ready", got)
	}
	_, _, resp = runQueryOnSource(t, stringShadowModel, &pb.Query{
		Where: primitive(QueryPropIsActive, opEqual, false, "ready"),
	})
	rows = rowsByID(t, resp)
	if rows["Demo::a"] == nil {
		t.Fatalf("filter on the string isActive = %v, want Demo::a", resp.Elements)
	}
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: stringShadowModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query: &pb.Query{
			Where: primitive("@isActive", opEqual, false, "ready"),
		},
	})
	assertQueryError(t, err, QueryErrUnparsableValue)
}
