package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestEvaluateTransitionFeatureKindIsTheLibraryLiteral: TransitionFeatureMembership::kind and
// TriggerInvocationExpression::kind travel as their SysML literal over Evaluate.
func TestEvaluateTransitionFeatureKindIsTheLibraryLiteral(t *testing.T) {
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: `package D {
    private import ScalarValues::*;
    part def M {
        attribute x : Integer = 1;
        state sm { entry; then a; state a; state b; transition t first a accept when x > 5 if x > 0 then b; }
    }
}`},
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
	transition := "(M::sm::t meta SysML::TransitionUsage)"
	for expression, want := range map[string][2]string{
		transition + ".guardExpression.owningMembership.kind": {"SysML::Systems::TransitionFeatureKind::guard", "SysML::Systems::TransitionFeatureKind"},
		transition + ".triggerAction.payloadArgument.kind":    {"SysML::Systems::TriggerKind::when", "SysML::Systems::TriggerKind"},
	} {
		value := evaluate(expression)
		if elements := value.GetSequence().GetElements(); len(elements) == 1 {
			value = elements[0]
		}
		lit := value.GetEnumLiteral()
		if lit == nil || lit.LiteralId != want[0] || lit.EnumerationId != want[1] {
			t.Errorf("%s = %v, want the literal %s of %s", expression, value, want[0], want[1])
		}
	}
	if got := evaluate(transition + ".guardExpression.owningMembership.kind == SysML::Systems::TransitionFeatureKind::guard"); !got.GetBoolValue() {
		t.Errorf("kind == TransitionFeatureKind::guard = %v, want true", got)
	}
}
