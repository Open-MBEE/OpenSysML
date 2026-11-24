package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// countingSymbolClient counts the per-element lookups a consumer would make to
// read metadata the slow way, so a query that needs none is observable.
type countingSymbolClient struct {
	srv   *Service
	calls int
}

func (c *countingSymbolClient) GetSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	c.calls++
	return c.srv.GetSymbol(ctx, req)
}

const metadataSurfacesModel = `package Demo {
	metadata def Tag { attribute label : String; }
	part a;
	part b;
	@Tag about a { label = "alpha"; }
	state def Machine {
		entry; then ready;
		state ready;
		state idle;
	}
}`

// The two new derived properties are selectable through the Query RPC without
// a schema change: one annotated element answers its binding, an unannotated
// one omits the property, and a state usage answers its Boolean activity.
func TestQuerySelectsAnnotationAndActivityProperties(t *testing.T) {
	_, _, resp := runQueryOnSource(t, metadataSurfacesModel, &pb.Query{
		Scope:  []string{"Demo"},
		Select: []string{QueryPropAnnotations, QueryPropIsActive},
	})
	rows := rowsByID(t, resp)
	if got := rows["Demo::a"].Properties[QueryPropAnnotations]; got != "Demo::Tag::label=alpha" {
		t.Fatalf("annotations of Demo::a = %q, want Demo::Tag::label=alpha", got)
	}
	if _, ok := rows["Demo::b"].Properties[QueryPropAnnotations]; ok {
		t.Fatalf("unannotated Demo::b answered annotations = %q", rows["Demo::b"].Properties[QueryPropAnnotations])
	}
	activity, found := rows["Demo::Machine::ready"].Properties[QueryPropIsActive]
	if !found || activity != "false" {
		t.Fatalf("isActive of a state without a session = %q (present %v), want false", activity, found)
	}
	if _, found := rows["Demo::a"].Properties[QueryPropIsActive]; found {
		t.Fatal("isActive was answered on a part, which is no state usage")
	}
}

// Reading the annotations of several annotated elements in one query makes no
// per-element symbol calls, however many rows the answer carries.
func TestQueryReadsAnnotationsWithNoPerElementSymbolLookups(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: metadataSurfacesModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	client := &countingSymbolClient{srv: srv}
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		Query:     &pb.Query{Scope: []string{"Demo"}, Select: []string{QueryPropAnnotations}},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	answered := 0
	for _, row := range resp.Elements {
		if _, ok := row.Properties[QueryPropAnnotations]; ok {
			answered++
		}
	}
	if answered == 0 {
		t.Fatal("no row answered an annotation value")
	}
	if client.calls != 0 {
		t.Fatalf("GetSymbol called %d times while reading annotations for %d rows", client.calls, answered)
	}
}

// Both properties filter rows as they project them.
func TestQueryFiltersByAnnotationAndActivityProperties(t *testing.T) {
	_, _, resp := runQueryOnSource(t, metadataSurfacesModel, &pb.Query{
		Where: primitive(QueryPropAnnotations,
			pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, false, "Demo::Tag::label=alpha"),
	})
	rows := rowsByID(t, resp)
	if len(rows) != 1 || rows["Demo::a"] == nil {
		t.Fatalf("rows filtered by annotations = %v, want Demo::a alone", rows)
	}
	_, _, resp = runQueryOnSource(t, metadataSurfacesModel, &pb.Query{
		Where: primitive(QueryPropIsActive,
			pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL, false, "false"),
	})
	rows = rowsByID(t, resp)
	if len(rows) != 2 || rows["Demo::Machine::ready"] == nil || rows["Demo::Machine::idle"] == nil {
		t.Fatalf("rows filtered by isActive = %v, want the two state usages", rows)
	}
}
