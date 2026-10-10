package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// TestGRPCRobustnessVerifyArguments binds a requirement's parameters badly: a
// name no parameter has, a parameter left unbound, more positional values than
// parameters, and a value of the wrong type each leave the verdict undecided,
// naming the fault, rather than failing the call or deciding anything.
func TestGRPCRobustnessVerifyArguments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyArgumentsModel, "verify-arguments-robustness")

	for name, req := range map[string]*pb.VerifyRequirementRequest{
		"unknown parameter": {ModelHash: hash, SymbolId: "Demo::Under", SubjectSymbolId: "Demo::t", NamedArguments: map[string]*pb.Value{"limit": integerArg(5), "bound": integerArg(1)}},
		"unbound parameter": {ModelHash: hash, SymbolId: "Demo::Under", SubjectSymbolId: "Demo::t"},
		"too many":          {ModelHash: hash, SymbolId: "Demo::Under", SubjectSymbolId: "Demo::t", Arguments: []*pb.Value{integerArg(5), integerArg(0), integerArg(1)}},
		"wrong type":        {ModelHash: hash, SymbolId: "Demo::Under", SubjectSymbolId: "Demo::t", NamedArguments: map[string]*pb.Value{"limit": {Kind: &pb.Value_StringValue{StringValue: "five"}}}},
	} {
		resp, err := srv.VerifyRequirement(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: VerifyRequirement: %v", name, err)
		}
		if resp.Verdict == nil || resp.Verdict.Error == "" {
			t.Errorf("%s: want an undecided verdict, got %v (error %q)", name, resp.Verdict, resp.Error)
		}
	}
}
