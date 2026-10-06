package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
)

// The service reports the lints on by default and no opt-in one.
func TestParseFileLeavesOptInLintsOut(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: `package P {
	private import ScalarValues::*;
	attribute x : Real = 0.1;
	state def M { entry; then a; state a; state b; transition first a when Pnig then b; }
}`},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	var signal bool
	for _, d := range parsed.Diagnostics {
		switch d.GetCode() {
		case passes.CodeRoundedRealLiteral:
			t.Errorf("an opt-in lint was reported: %s", d.GetMessage())
		case passes.CodeUndeclaredSignal:
			signal = true
		}
	}
	if !signal {
		t.Fatalf("want the on-by-default lint reported: %v", parsed.Diagnostics)
	}
}
