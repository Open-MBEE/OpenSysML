package grpc

import (
	"context"
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// The scoped property resolves to the owner's text feature. Its value, not an
// unrelated built-in of the same name, decides whether `>` is a valid operand.
const scopedTextFeatureModel = `package Demo {
	private import ScalarValues::*;
	part def Owner {
		attribute annotations : String = "text";
		part probe;
	}
}`

func TestOSLCScopedTextFeatureRejectsOrderingOperand(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: scopedTextFeatureModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	_, err = srv.Query(context.Background(), &pb.QueryRequest{
		ModelHash: parsed.ModelHash,
		OslcQuery: `oslc.where=rdf:type=sysml:PartUsage and sysml:name="probe" and sysml:owner{sysx:annotations>3}`,
	})
	assertQueryError(t, err, QueryErrUnorderedProperty)
}

// Reversing the terms of an AND cannot change whether a shadowed operand is
// valid: the shadow is found in the rows the conjunction started with.
func TestQueryConjunctionOrderDoesNotChangeShadowDeferral(t *testing.T) {
	name := primitive(QueryPropName, opEqual, false, "ready")
	inverseActivity := primitive(QueryPropIsActive, opEqual, true, "ready")
	var forward, backward []string
	_, _, resp := runQueryOnSource(t, mixedActivityFilterModel, &pb.Query{
		Where: composite(pb.CompositeOperator_COMPOSITE_OPERATOR_AND, name, inverseActivity),
	})
	for _, element := range resp.Elements {
		forward = append(forward, element.Id)
	}
	_, _, resp = runQueryOnSource(t, mixedActivityFilterModel, &pb.Query{
		Where: composite(pb.CompositeOperator_COMPOSITE_OPERATOR_AND, inverseActivity, name),
	})
	for _, element := range resp.Elements {
		backward = append(backward, element.Id)
	}
	want := []string{"Demo::Machine::ready"}
	if !slices.Equal(forward, want) || !slices.Equal(backward, want) {
		t.Fatalf("name then inverse = %v, inverse then name = %v, want %v", forward, backward, want)
	}
}
