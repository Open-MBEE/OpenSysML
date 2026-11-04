package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestEvaluateReflectiveDirectionIsTheLibraryLiteral: Feature::direction travels as the
// FeatureDirectionKind literal of the reflective KerML model; a meta cast's result is a sequence.
func TestEvaluateReflectiveDirectionIsTheLibraryLiteral(t *testing.T) {
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: `package D { action def A { in x; } }`},
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	evaluate := func(expression string) *pb.Value {
		resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
			ModelHash: parseResp.ModelHash, Expression: expression, ContextSymbolId: "D",
		})
		if err != nil || resp.Error != "" {
			t.Fatalf("Evaluate(%s): %v %s", expression, err, resp.GetError())
		}
		return resp.Result
	}
	direction := evaluate("(A::x meta KerML::Feature).direction")
	if elements := direction.GetSequence().GetElements(); len(elements) == 1 {
		direction = elements[0]
	}
	lit := direction.GetEnumLiteral()
	if lit == nil {
		t.Fatalf("direction = %v, want an enum_literal", direction)
	}
	if lit.LiteralId != "KerML::Core::FeatureDirectionKind::in" || lit.EnumerationId != "KerML::Core::FeatureDirectionKind" {
		t.Errorf("direction = %q of %q, want KerML::Core::FeatureDirectionKind::in", lit.LiteralId, lit.EnumerationId)
	}
	if got := evaluate("(A::x meta KerML::Feature).direction == KerML::Core::FeatureDirectionKind::'in'"); !got.GetBoolValue() {
		t.Errorf("direction == FeatureDirectionKind::'in' = %v, want true", got)
	}
}
