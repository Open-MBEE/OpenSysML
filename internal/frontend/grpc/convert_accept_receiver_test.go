package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestEvaluateViaTriggerReceiverArgument: an `accept … via p` accept's receiverArgument reaches p over Evaluate.
func TestEvaluateViaTriggerReceiverArgument(t *testing.T) {
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: `package D {
    item def Sig;
    port def P { in item s : Sig; }
    part def M {
        port p : P;
        state sm { entry; then a; state a; state b; transition t first a accept Sig via p then b; }
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
	accept := "(M::sm::t meta SysML::TransitionUsage).triggerAction"
	name := evaluate(accept + ".receiverArgument.referent.declaredName")
	if elements := name.GetSequence().GetElements(); len(elements) == 1 {
		name = elements[0]
	}
	if got := name.GetStringValue(); got != "p" {
		t.Errorf("receiverArgument.referent.declaredName = %v, want p", name)
	}
	if got := evaluate(accept + ".receiverArgument istype SysML::FeatureReferenceExpression"); !got.GetBoolValue() {
		t.Errorf("receiverArgument istype FeatureReferenceExpression = %v, want true", got)
	}
	if got := evaluate("SequenceFunctions::size(" + accept + ".ownedFeature)"); got.GetIntValue() != 2 {
		t.Errorf("accept ownedFeature size = %v, want 2", got)
	}
}
