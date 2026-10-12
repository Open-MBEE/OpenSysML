package opensysml

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// argumentCaller records the verification requests it is sent.
type argumentCaller struct {
	oldCaller
	constraint  *pb.VerifyConstraintRequest
	requirement *pb.VerifyRequirementRequest
}

func (a *argumentCaller) verifyConstraint(_ context.Context, req *pb.VerifyConstraintRequest) (*pb.VerifyConstraintResponse, error) {
	a.constraint = req
	return &pb.VerifyConstraintResponse{Verdict: &pb.Verdict{Kind: "constraint", Holds: true}}, nil
}

func (a *argumentCaller) verifyRequirement(_ context.Context, req *pb.VerifyRequirementRequest) (*pb.VerifyRequirementResponse, error) {
	a.requirement = req
	return &pb.VerifyRequirementResponse{Verdict: &pb.Verdict{Kind: "requirement", Holds: true}}, nil
}

func TestVerifyArgumentsAreSentAsValues(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	caller := &argumentCaller{oldCaller: oldCaller{t: t, capabilities: []string{
		CapabilityVerification, CapabilityFeatureValues, CapabilityVerificationArguments,
	}}}
	c := &client{caller: caller}

	out, err := c.VerifyRequirement(ctx, model, "Demo::Under", Against("Demo::t"),
		VerifyArguments(Int(5)), VerifyArgument("slack", Int(1)))
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if !out.Verdict.Holds {
		t.Error("holds = false, want true")
	}
	req := caller.requirement
	if req.SubjectSymbolId != "Demo::t" || len(req.Arguments) != 1 || req.Arguments[0].GetIntValue() != 5 {
		t.Errorf("requirement request = %v, want subject Demo::t and argument 5", req)
	}
	if req.NamedArguments["slack"].GetIntValue() != 1 {
		t.Errorf("named arguments = %v, want slack = 1", req.NamedArguments)
	}

	if _, err := c.VerifyConstraint(ctx, model, "Demo::Between", VerifyArgument("low", Int(3))); err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if got := caller.constraint; len(got.Arguments) != 0 || got.NamedArguments["low"].GetIntValue() != 3 {
		t.Errorf("constraint request = %v, want only low = 3", got)
	}
}

func TestVerifyArgumentsAreNotSentWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	for name, old := range map[string]*oldCaller{
		"predates verification_arguments": {t: t, capabilities: []string{CapabilityVerification, CapabilityFeatureValues}},
		"predates GetServerInfo":          {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.VerifyConstraint(ctx, model, "C", VerifyArguments(Int(3)))
			wantUnimplemented(t, "VerifyConstraint", err)
			_, err = c.VerifyRequirement(ctx, model, "R", VerifyArgument("limit", Int(5)))
			wantUnimplemented(t, "VerifyRequirement", err)
		})
	}
}

func TestSatisfactionAndObjectsTakeNoArguments(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	c := &client{caller: &oldCaller{t: t, capabilities: []string{
		CapabilityVerification, CapabilityFeatureValues, CapabilityVerificationArguments,
	}}}
	_, err := c.VerifySatisfaction(ctx, model, "", VerifyArguments(Int(1)))
	if status, ok := err.(*StatusError); !ok || status.Code != CodeInvalidArgument {
		t.Errorf("VerifySatisfaction with arguments: err = %v, want INVALID_ARGUMENT", err)
	}
	_, err = c.ValidateInstance(ctx, model, "p", VerifyArgument("x", Int(1)))
	if status, ok := err.(*StatusError); !ok || status.Code != CodeInvalidArgument {
		t.Errorf("ValidateInstance with arguments: err = %v, want INVALID_ARGUMENT", err)
	}
}
