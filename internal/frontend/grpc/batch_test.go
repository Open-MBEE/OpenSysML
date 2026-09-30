package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const forwardBatchSource = `package P {
    action def A {
        first start;
        action a;
    }
    part def Vehicle;
}
`

// One ApplyEdits batch is resolved as a whole: a reference an operation writes
// may name a declaration a later operation of the same batch adds, and the
// operations succeed in either order.
func TestApplyEditsResolvesReferencesAgainstTheWholeBatch(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, forwardBatchSource)
	tests := []struct {
		name string
		ops  []*pb.EditOperation
		want []string
	}{
		{
			name: "typed action then its action def",
			ops: []*pb.EditOperation{
				addSequenceOp("P::A", "then", "", "action", "g", "GenerateHeat", ""),
				addMemberOp("P", "action def", "GenerateHeat"),
			},
			want: []string{"then action g : GenerateHeat;", "action def GenerateHeat;"},
		},
		{
			name: "typed part then its part def",
			ops: []*pb.EditOperation{
				{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
					Owner: "P", Kind: "part", Name: "engine", Type: "Engine",
				}}},
				addMemberOp("P", "part def", "Engine"),
			},
			want: []string{"part engine : Engine;", "part def Engine;"},
		},
		{
			name: "typed attribute then its attribute def",
			ops: []*pb.EditOperation{
				{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
					Owner: "P::Vehicle", Kind: "attribute", Name: "mass", Type: "Mass",
				}}},
				addMemberOp("P", "attribute def", "Mass"),
			},
			want: []string{"attribute mass : Mass;", "attribute def Mass;"},
		},
		{
			name: "succession then the member it reaches",
			ops: []*pb.EditOperation{
				addSequenceOp("P::A", "then", "g", "", "", "", ""),
				addMemberOp("P::A", "action", "g"),
			},
			want: []string{"then g;", "action g;"},
		},
		{
			name: "metadata prefix then its metadata def",
			ops: []*pb.EditOperation{
				addMetadataPrefixOp("P::Vehicle", "Marker"),
				addMemberOp("P", "metadata def", "Marker"),
			},
			want: []string{"#Marker part def Vehicle;", "metadata def Marker;"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, ops := range [][]*pb.EditOperation{tt.ops, {tt.ops[1], tt.ops[0]}} {
				resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
					ModelHash: hash, Operations: ops,
				})
				if err != nil {
					t.Fatalf("ApplyEdits: %v", err)
				}
				if resp.Error != "" {
					t.Fatalf("ApplyEdits refused: %s (%s)", resp.Error, resp.Failure)
				}
				for _, want := range tt.want {
					if !strings.Contains(resp.Content, want) {
						t.Fatalf("result lacks %q:\n%s", want, resp.Content)
					}
				}
			}
		})
	}
}

// A reference nothing in the batch declares, and a name an earlier operation
// of the batch already took, are refused as they are outside a batch.
func TestApplyEditsRefusesReferencesUnresolvedAfterTheWholeBatch(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, forwardBatchSource)
	tests := []struct {
		name    string
		ops     []*pb.EditOperation
		failure pb.EditFailure
		message string
	}{
		{
			name: "typed action",
			ops: []*pb.EditOperation{
				addSequenceOp("P::A", "then", "", "action", "g", "Nowhere", ""),
				addMemberOp("P", "action def", "GenerateHeat"),
			},
			failure: pb.EditFailure_EDIT_FAILURE_RESULT_INVALID,
			message: "unresolved reference: Nowhere",
		},
		{
			name: "succession",
			ops: []*pb.EditOperation{
				addSequenceOp("P::A", "then", "missing", "", "", "", ""),
				addMemberOp("P::A", "action", "g"),
			},
			failure: pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET,
			message: `sequence node "missing" resolves to nothing visible from "P::A"`,
		},
		{
			name: "metadata prefix",
			ops: []*pb.EditOperation{
				addMetadataPrefixOp("P::Vehicle", "Missing"),
				addMemberOp("P", "metadata def", "Marker"),
			},
			failure: pb.EditFailure_EDIT_FAILURE_INVALID_VALUE,
			message: `metadata type "Missing" does not resolve from P::Vehicle`,
		},
		{
			name: "name an earlier operation took",
			ops: []*pb.EditOperation{
				addMemberOp("P", "action def", "GenerateHeat"),
				addMemberOp("P", "action def", "GenerateHeat"),
			},
			failure: pb.EditFailure_EDIT_FAILURE_MEMBER_NAME_TAKEN,
			message: `P already declares "GenerateHeat"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
				ModelHash: hash, Operations: tt.ops,
			})
			if err != nil {
				t.Fatalf("ApplyEdits: %v", err)
			}
			if resp.Content != "" || resp.Failure != tt.failure {
				t.Fatalf("response = %+v, want %s", resp, tt.failure)
			}
			if !strings.Contains(resp.Error, tt.message) {
				t.Fatalf("error = %q, want it to contain %q", resp.Error, tt.message)
			}
		})
	}
}
