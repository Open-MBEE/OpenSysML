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

// Two owners declare the same name with different kinds: the ordering term
// must accept the numeric feature whichever side of the conjunction it is on.
const declaredShadowedAnnotationsModel = `package Demo {
	private import ScalarValues::*;
	part def NumericOwner {
		attribute annotations : Integer = 5;
		part probe;
	}
	part def TextOwner {
		attribute annotations : String = "text";
		part other;
	}
}`

func TestOSLCConjunctionOrderDoesNotChangeDeclaredFeatureOperand(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: declaredShadowedAnnotationsModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, where := range []string{
		`rdf:type=sysml:PartUsage and sysml:name="probe" and sysml:owner{sysx:annotations>3}`,
		`rdf:type=sysml:PartUsage and sysml:owner{sysx:annotations>3} and sysml:name="probe"`,
	} {
		rows := rowsByID(t, oslcQuery(t, srv, parsed.ModelHash, "oslc.where="+where))
		if len(rows) != 1 || rows["Demo::NumericOwner::probe"] == nil {
			t.Fatalf("%s = %v, want the numeric owner's probe alone", where, rows)
		}
	}
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
