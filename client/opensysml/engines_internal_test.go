package opensysml

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func (o *oldCaller) verifyConstraint(context.Context, *pb.VerifyConstraintRequest) (*pb.VerifyConstraintResponse, error) {
	o.t.Fatal("an engine was sent to a service without engines")
	return nil, nil
}

func (o *oldCaller) verifyRequirement(context.Context, *pb.VerifyRequirementRequest) (*pb.VerifyRequirementResponse, error) {
	o.t.Fatal("an engine was sent to a service without engines")
	return nil, nil
}

func (o *oldCaller) verifySatisfaction(context.Context, *pb.VerifySatisfactionRequest) (*pb.VerifySatisfactionResponse, error) {
	o.t.Fatal("an engine was sent to a service without engines")
	return nil, nil
}

// A named engine is refused before it leaves the client when the service lacks
// the engines capability, since such a service would answer with whichever
// engine it chose: whether it predates the capability or GetServerInfo itself.
func TestANamedEngineIsNotSentWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	for name, old := range map[string]*oldCaller{
		"predates engines":       {t: t, capabilities: []string{CapabilityVerification, CapabilityFeatureValues, CapabilitySchedule}},
		"predates GetServerInfo": {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.VerifyConstraint(ctx, model, "C", WithEngine("run"))
			wantUnimplemented(t, "VerifyConstraint", err)
			_, err = c.VerifyRequirement(ctx, model, "R", WithEngine("run"))
			wantUnimplemented(t, "VerifyRequirement", err)
			_, err = c.VerifySatisfaction(ctx, model, "", WithEngine(EngineAll))
			wantUnimplemented(t, "VerifySatisfaction", err)
			_, err = c.RunAnalysis(ctx, model, "an", Engine("run"))
			wantUnimplemented(t, "RunAnalysis", err)
			_, err = c.Calculate(ctx, model, "f", CalcArguments(Int(1)), CalcEngine("run"))
			wantUnimplemented(t, "Calculate", err)
		})
	}
}

// Leaving the engine unset, or naming auto, sends no engine at all, so an
// older service answers exactly as it did before.
func TestAutoSendsNoEngine(t *testing.T) {
	for _, engine := range []string{"", EngineAuto} {
		if got := engineField(engine); got != "" {
			t.Errorf("engineField(%q) = %q, want empty", engine, got)
		}
	}
	if got := engineField(EngineAll); got != "all" {
		t.Errorf("engineField(all) = %q, want all", got)
	}
}
