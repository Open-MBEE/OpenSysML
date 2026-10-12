package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// verifyArgumentsModel declares a requirement and a constraint whose conditions
// read `in` parameters, so a check binds them from the request.
const verifyArgumentsModel = `package Demo {
	private import ScalarValues::*;
	part def Thing { attribute v : Integer = 2; }
	part t : Thing;
	requirement def Under {
		subject s : Thing;
		in limit : Integer;
		in slack : Integer = 0;
		require constraint { s.v + slack < limit }
	}
	constraint def Between {
		in low : Integer;
		in high : Integer = 10;
		low < high
	}
}`

func integerArg(n int64) *pb.Value {
	return &pb.Value{Kind: &pb.Value_IntValue{IntValue: n}}
}

func TestVerifyRequirementBindsArguments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyArgumentsModel, "verify-arguments")

	resp, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Under",
		SubjectSymbolId: "Demo::t",
		NamedArguments:  map[string]*pb.Value{"limit": integerArg(5)},
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if resp.Error != "" || !resp.Verdict.Holds {
		t.Fatalf("Under(limit = 5): holds = %v, error %q; want holds", resp.Verdict.GetHolds(), resp.Error)
	}

	failing, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Under",
		SubjectSymbolId: "Demo::t",
		Arguments:       []*pb.Value{integerArg(5), integerArg(4)},
	})
	if err != nil {
		t.Fatalf("VerifyRequirement: %v", err)
	}
	if failing.Error != "" || failing.Verdict.Holds {
		t.Fatalf("Under(5, 4) on t: holds = %v, error %q; want a false verdict", failing.Verdict.GetHolds(), failing.Error)
	}
}

func TestVerifyConstraintBindsArguments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyArgumentsModel, "verify-constraint-arguments")

	resp, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Between",
		Arguments: []*pb.Value{integerArg(3)},
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if resp.Error != "" || !resp.Verdict.Holds {
		t.Fatalf("Between(3): holds = %v, error %q; want holds", resp.Verdict.GetHolds(), resp.Error)
	}

	failing, err := srv.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash:      hash,
		SymbolId:       "Demo::Between",
		Arguments:      []*pb.Value{integerArg(3)},
		NamedArguments: map[string]*pb.Value{"high": integerArg(1)},
	})
	if err != nil {
		t.Fatalf("VerifyConstraint: %v", err)
	}
	if failing.Error != "" || failing.Verdict.Holds {
		t.Fatalf("Between(3, high = 1): holds = %v, error %q; want a false verdict", failing.Verdict.GetHolds(), failing.Error)
	}
}

func TestVerifyArgumentsRefusedWithoutCapabilityOrForProofs(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, verifyArgumentsModel, "verify-arguments-proof")

	_, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        "Demo::Under",
		SubjectSymbolId: "Demo::t",
		Question:        "holds",
		NamedArguments:  map[string]*pb.Value{"limit": integerArg(5)},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "takes no arguments") {
		t.Fatalf("holds with arguments: err = %v, want INVALID_ARGUMENT naming the arguments", err)
	}

	without := mustNewServiceWithout(t, CapabilityVerificationArguments)
	hash = mustVerifyModel(t, without, verifyArgumentsModel, "verify-arguments-without")
	_, err = without.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{
		ModelHash: hash,
		SymbolId:  "Demo::Between",
		Arguments: []*pb.Value{integerArg(3)},
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("arguments without the capability: err = %v, want UNIMPLEMENTED", err)
	}
	plain, err := without.VerifyConstraint(context.Background(), &pb.VerifyConstraintRequest{ModelHash: hash, SymbolId: "Demo::Between"})
	if err != nil || plain.Verdict == nil {
		t.Fatalf("a request without arguments needs no capability: %v", err)
	}
}
