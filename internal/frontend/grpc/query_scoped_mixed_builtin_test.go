package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const scopedMixedBuiltinModel = `package Demo {
	private import ScalarValues::*;
	private import DocumentQueries::*;
	metadata def Tag { attribute label : String; }
	part def NumericOwner {
		attribute annotations : Integer = 5;
		part probe;
	}
	part def AnnotatedOwner {
		part probe;
	}
	@Tag about AnnotatedOwner { label = "text"; }
}`

// A scoped filter compares the final property of the related element: one
// owner's numeric feature matches, the other's annotation text is skipped.
func TestOSLCScopedFilterSkipsIncompatibleBuiltinRows(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: scopedMixedBuiltinModel},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp := oslcQuery(t, srv, parsed.ModelHash,
		`oslc.where=rdf:type=sysml:PartUsage and sysml:name="probe" and sysml:owner{sysx:annotations>3}`)
	rows := rowsByID(t, resp)
	if len(rows) != 1 || rows["Demo::NumericOwner::probe"] == nil {
		t.Fatalf("scoped annotations>3 = %v, want the numeric owner's probe alone", resp.Elements)
	}
}
