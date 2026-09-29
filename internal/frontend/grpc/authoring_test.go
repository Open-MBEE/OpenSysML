package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"google.golang.org/protobuf/proto"
)

func addMemberOp(owner, kind, name string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddMember{
		AddMember: &pb.AddMemberEdit{Owner: owner, Kind: kind, Name: name},
	}}
}

func addConnectionOp(owner, kind, from, to, name, typ string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddConnection{
		AddConnection: &pb.AddConnectionEdit{
			Owner: owner, Kind: kind, FromEnd: from, ToEnd: to, Name: name, Type: typ,
		},
	}}
}

func addSatisfyOp(owner, requirement, by string, asserted, negated bool) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddSatisfy{
		AddSatisfy: &pb.AddSatisfyEdit{
			Owner: owner, Requirement: requirement, SatisfyingFeature: by,
			IsAsserted: asserted, IsNegated: negated,
		},
	}}
}

func addRequirementConstraintOp(owner, kind, expression, name string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddRequirementConstraint{
		AddRequirementConstraint: &pb.AddRequirementConstraintEdit{
			Owner: owner, Kind: kind, Expression: expression, Name: name,
		},
	}}
}

func addTransitionOp(owner, name, source, target, trigger, guard, effect string, initial bool) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddTransition{
		AddTransition: &pb.AddTransitionEdit{
			Owner: owner, Name: name, Source: source, Target: target,
			Trigger: trigger, Guard: guard, Effect: effect, Initial: initial,
		},
	}}
}

func deleteOp(target string, cascade bool) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_Delete{
		Delete: &pb.DeleteEdit{Target: target, Cascade: cascade},
	}}
}

func moveOp(target, owner string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_Move{
		Move: &pb.MoveEdit{Target: target, Owner: owner},
	}}
}

func TestApplyEditsMove(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, "package P {\n    part def Base;\n    part def Holder;\n    part x : P::Base;\n}\n")

	moved, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{moveOp("P::Base", "P::Holder")},
	})
	if err != nil {
		t.Fatalf("move call failed: %v", err)
	}
	want := "package P {\n    part def Holder {\n        part def Base;\n    }\n    part x : P::Holder::Base;\n}\n"
	if moved.Error != "" || moved.Content != want {
		t.Fatalf("move response = %+v\n%s", moved, moved.Content)
	}

	refused, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{moveOp("P::Holder", "P::Holder")},
	})
	if err != nil {
		t.Fatalf("refused move call failed: %v", err)
	}
	if refused.Content != "" || refused.Failure != pb.EditFailure_EDIT_FAILURE_OWNER_INSIDE_TARGET {
		t.Fatalf("self-move response = %+v", refused)
	}
}

func TestApplyEditsAddMemberAndDelete(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, "package P {\n    part def Base;\n    part x : Base;\n}\n")

	added, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{addMemberOp("P", "part def", "Child")},
	})
	if err != nil {
		t.Fatalf("add call failed: %v", err)
	}
	if added.Error != "" || !strings.Contains(added.Content, "part def Child;") {
		t.Fatalf("add response = %+v\n%s", added, added.Content)
	}

	hash = mustParsedModel(t, srv, added.Content)
	deleted, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{deleteOp("P::Base", true)},
	})
	if err != nil {
		t.Fatalf("delete call failed: %v", err)
	}
	if deleted.Error != "" {
		t.Fatalf("delete refused: %s", deleted.Error)
	}
	if strings.Contains(deleted.Content, "Base") || strings.Contains(deleted.Content, "part x") {
		t.Fatalf("cascade result retained deleted declarations:\n%s", deleted.Content)
	}
}

func TestApplyEditsAddConnection(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, "package Demo {\n    part def System {\n        part a;\n        part b;\n    }\n}\n")

	added, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{addConnectionOp("Demo::System", "allocation", "a", "b", "alloc1", "")},
	})
	if err != nil {
		t.Fatalf("add connection call failed: %v", err)
	}
	want := "package Demo {\n    part def System {\n        part a;\n        part b;\n        allocation alloc1 allocate a to b;\n    }\n}\n"
	if added.Error != "" || added.Content != want {
		t.Fatalf("add connection response = %+v\n%s\nwant:\n%s", added, added.Content, want)
	}

	hash = mustParsedModel(t, srv, `package Demo {
    item def Fuel;
    part def Tank { port fuelOut : Fuel; }
    part def Engine { port fuelIn : Fuel; }
    part def System {
        part tank : Tank;
        part engine : Engine;
    }
}
`)
	flow, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{addConnectionOp("Demo::System", "flow", "tank.fuelOut", "engine.fuelIn", "", "")},
	})
	if err != nil {
		t.Fatalf("add flow call failed: %v", err)
	}
	if flow.Error != "" || !strings.Contains(flow.Content, "        flow from tank.fuelOut to engine.fuelIn;\n") {
		t.Fatalf("flow response = %+v\n%s", flow, flow.Content)
	}

	hash = mustParsedModel(t, srv, `package Demo {
    port def Plug;
    connection def Cable {
        end a : Plug;
        end b : Plug;
    }
    part def Rig {
        part left { port p : Plug; }
        part right { port p : Plug; }
    }
}
`)
	typed, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{addConnectionOp("Demo::Rig", "connection", "left.p", "right.p", "cable", "Cable")},
	})
	if err != nil {
		t.Fatalf("add typed connection call failed: %v", err)
	}
	if typed.Error != "" || !strings.Contains(typed.Content, "connection cable : Cable connect left.p to right.p;") {
		t.Fatalf("typed connection response = %+v\n%s", typed, typed.Content)
	}
}

func TestApplyEditsAddConnectionRefusals(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		op      *pb.EditOperation
		failure pb.EditFailure
	}{
		{
			name:    "unknown kind",
			source:  "package Demo {\n    part def System {\n        part a;\n        part b;\n    }\n}\n",
			op:      addConnectionOp("Demo::System", "unknown", "a", "b", "", ""),
			failure: pb.EditFailure_EDIT_FAILURE_ILLEGAL_KIND,
		},
		{
			name:    "type on succession",
			source:  "package Demo {\n    action def System {\n        action a;\n        action b;\n    }\n}\n",
			op:      addConnectionOp("Demo::System", "succession", "a", "b", "", "SomeType"),
			failure: pb.EditFailure_EDIT_FAILURE_ILLEGAL_KIND,
		},
		{
			name:    "unresolved end",
			source:  "package Demo {\n    part def System {\n        part a;\n        part b;\n    }\n}\n",
			op:      addConnectionOp("Demo::System", "flow", "missing", "b", "", ""),
			failure: pb.EditFailure_EDIT_FAILURE_RESULT_INVALID,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := mustNewService(t, 10)
			hash := mustParsedModel(t, srv, tc.source)
			resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
				ModelHash: hash, Operations: []*pb.EditOperation{tc.op},
			})
			if err != nil {
				t.Fatalf("ApplyEdits: %v", err)
			}
			if resp.Content != "" || resp.Failure != tc.failure {
				t.Fatalf("response = %+v, want refusal %s", resp, tc.failure)
			}
		})
	}
}

func TestApplyEditsAddConnectionRequiresAuthoring(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityAuthoring)
	ctx := context.Background()
	hash := mustParsedModel(t, srv, "package Demo {\n    part def System {\n        part a;\n        part b;\n    }\n}\n")
	_, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			addConnectionOp("Demo::System", "allocation", "a", "b", "alloc1", ""),
		},
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("ApplyEdits status = %s, want UNIMPLEMENTED: %v", connect.CodeOf(err), err)
	}
}

func TestApplyEditsAddConnectionRequiresConnectionAuthoring(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityConnectionAuthoring)
	ctx := context.Background()
	hash := mustParsedModel(t, srv, "package Demo {\n    part def System {\n        part a;\n        part b;\n    }\n}\n")
	_, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			addConnectionOp("Demo::System", "allocation", "a", "b", "alloc1", ""),
		},
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityConnectionAuthoring) {
		t.Fatalf("ApplyEdits refusal = %v, want UNIMPLEMENTED naming %q", err, CapabilityConnectionAuthoring)
	}

	added, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{addMemberOp("Demo::System", "part", "c")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits add_member: %v", err)
	}
	if added.Error != "" || !strings.Contains(added.Content, "part c") {
		t.Fatalf("add_member response = %+v, want the new part", added)
	}
}

func TestApplyEditsAddTransitionRequiresTransitionAuthoring(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityTransitionAuthoring)
	ctx := context.Background()
	hash := mustParsedModel(t, srv, "state def S { state idle; }\n")
	_, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			addTransitionOp("S", "", "", "idle", "", "", "", true),
		},
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented ||
		!strings.Contains(err.Error(), CapabilityTransitionAuthoring) {
		t.Fatalf("ApplyEdits refusal = %v, want UNIMPLEMENTED naming %q", err, CapabilityTransitionAuthoring)
	}

	added, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash, Operations: []*pb.EditOperation{addMemberOp("S", "state", "toasting")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits add_member: %v", err)
	}
	if added.Error != "" || !strings.Contains(added.Content, "state toasting") {
		t.Fatalf("add_member response = %+v, want the new state", added)
	}
}

func TestApplyEditsNewAuthoringOperationsRequireDedicatedCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		operation  *pb.EditOperation
	}{
		{
			name:       "member modifiers",
			capability: CapabilityMemberModifiers,
			operation: &pb.EditOperation{Operation: &pb.EditOperation_AddMember{
				AddMember: &pb.AddMemberEdit{Owner: "Demo", Kind: "part def", Name: "X", IsAbstract: true},
			}},
		},
		{
			name:       "return member kind",
			capability: CapabilityMemberModifiers,
			operation: &pb.EditOperation{Operation: &pb.EditOperation_AddMember{
				AddMember: &pb.AddMemberEdit{Owner: "Demo", Kind: "return", Name: "result"},
			}},
		},
		{
			name:       "satisfy",
			capability: CapabilitySatisfyAuthoring,
			operation:  addSatisfyOp("Demo::r", "Demo::r", "Demo::t", true, false),
		},
		{
			name:       "requirement constraint",
			capability: CapabilityRequirementConstraintAuthoring,
			operation:  addRequirementConstraintOp("Demo::r", "require", "true", ""),
		},
		{
			name:       "transition",
			capability: CapabilityTransitionAuthoring,
			operation:  addTransitionOp("Demo::S", "", "idle", "idle", "", "", "", false),
		},
		{
			name:       "constraint body",
			capability: CapabilityConstraintBodyAuthoring,
			operation: &pb.EditOperation{Operation: &pb.EditOperation_AddMember{
				AddMember: &pb.AddMemberEdit{
					Owner: "Demo", Kind: "constraint", Name: "c", BodyExpression: "true",
				},
			}},
		},
		{
			name:       "assert constraint",
			capability: CapabilityConstraintBodyAuthoring,
			operation:  addMemberOp("Demo", "assert constraint", "c"),
		},
		{
			name:       "reference assertion",
			capability: CapabilityConstraintBodyAuthoring,
			operation:  addMemberOp("Demo", "assert", "c"),
		},
		{
			name:       "negated reference assertion",
			capability: CapabilityConstraintBodyAuthoring,
			operation:  addMemberOp("Demo", "assert not", "c"),
		},
		{
			name:       "calculation result expression",
			capability: CapabilityConstraintBodyAuthoring,
			operation: &pb.EditOperation{Operation: &pb.EditOperation_AddMember{
				AddMember: &pb.AddMemberEdit{
					Owner: "Demo", Kind: "calc def", Name: "D", BodyExpression: "x * 2",
				},
			}},
		},
		{
			name:       "state behavior kind",
			capability: CapabilityStateActionAuthoring,
			operation:  addMemberOp("Demo", "exhibit state", "shown"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := mustNewServiceWithout(t, tc.capability)
			hash := mustParsedModel(t, srv, "package Demo { requirement r; part t; }\n")
			_, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
				ModelHash: hash, Operations: []*pb.EditOperation{tc.operation},
			})
			if connect.CodeOf(err) != connect.CodeUnimplemented ||
				!strings.Contains(err.Error(), tc.capability) {
				t.Fatalf("ApplyEdits refusal = %v, want UNIMPLEMENTED naming %q", err, tc.capability)
			}
			added, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
				ModelHash: hash,
				Operations: []*pb.EditOperation{
					addMemberOp("Demo", "part", "stillSupported"),
				},
			})
			if err != nil || added.Error != "" || !strings.Contains(added.Content, "part stillSupported") {
				t.Fatalf("ordinary add_member = %+v, %v; want success", added, err)
			}
		})
	}
}

func TestApplyEditsNewOperationsRoundTrip(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, `package Demo {
    requirement def R { subject x : Real; }
    requirement r : R;
    part t;
}
`)
	added, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			addSatisfyOp("Demo::r", "Demo::r", "Demo::t", true, false),
			addRequirementConstraintOp("Demo::r", "require", "true", "valid"),
		},
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if added.Error != "" {
		t.Fatalf("edit refused: %s", added.Error)
	}
	for _, want := range []string{
		"assert satisfy Demo::r by Demo::t;",
		"require constraint valid { true }",
	} {
		if !strings.Contains(added.Content, want) {
			t.Errorf("content missing %q:\n%s", want, added.Content)
		}
	}
}

func TestApplyEditsReferenceAssertionsAndResultBodiesRoundTrip(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, `package Demo {
    constraint c;
}
`)
	ops := []*pb.EditOperation{
		addMemberOp("Demo", "assert", "c"),
		addMemberOp("Demo", "assert not", "c"),
		addMemberOp("Demo", "assert", "c"),
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo", Kind: "calc def", Name: "Double", BodyExpression: "3 * 2",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo", Kind: "case def", Name: "Example", BodyExpression: "1",
		}}},
	}
	added, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, Operations: ops,
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if added.Error != "" {
		t.Fatalf("edit refused: %s\n%s", added.Error, added.Content)
	}
	for _, want := range []string{
		"assert c;", "assert not c;", "calc def Double { 3 * 2 }",
		"case def Example { 1 }",
	} {
		if !strings.Contains(added.Content, want) {
			t.Errorf("edited content missing %q:\n%s", want, added.Content)
		}
	}
	if got := strings.Count(added.Content, "assert c;"); got != 2 {
		t.Errorf("assert c count = %d, want 2:\n%s", got, added.Content)
	}
	if reparsed := mustParsedModel(t, srv, added.Content); reparsed == "" {
		t.Fatal("edited notation did not parse")
	}
}

func TestApplyEditsConstraintBodiesAndStateBehaviorRoundTrip(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, `package Demo {
    constraint def ConstraintType;
    action def GenerateHeat;
    state def Cycle {
        state heating;
    }
    part def Toaster {
        state cycle : Cycle;
    }
    part toaster : Toaster;
    part def Host;
}
`)
	ops := []*pb.EditOperation{
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo", Kind: "constraint def", Name: "C", BodyExpression: "true",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo", Kind: "constraint", Name: "c", BodyExpression: "true",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo", Kind: "assert constraint", Name: "positive", BodyExpression: "true",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo", Kind: "assert not constraint", Name: "negative", BodyExpression: "false",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo", Kind: "assert constraint", Type: "ConstraintType",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::Host", Kind: "exhibit state", Name: "shown", Type: "Cycle",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::Host", Kind: "exhibit", Name: "toaster.cycle",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::Cycle", Kind: "entry action", Name: "entryWork", Type: "GenerateHeat",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::Cycle", Kind: "do action", Name: "doWork", Type: "GenerateHeat",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::Cycle", Kind: "exit action", Name: "exitWork", Type: "GenerateHeat",
		}}},
	}
	added, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, Operations: ops,
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if added.Error != "" {
		t.Fatalf("edit refused: %s\n%s", added.Error, added.Content)
	}
	for _, want := range []string{
		"constraint def C { true }",
		"constraint c { true }",
		"assert constraint positive { true }",
		"assert not constraint negative { false }",
		"assert constraint : ConstraintType;",
		"exhibit state shown : Cycle;",
		"exhibit toaster.cycle;",
		"entry action entryWork : GenerateHeat;",
		"do action doWork : GenerateHeat;",
		"exit action exitWork : GenerateHeat;",
	} {
		if !strings.Contains(added.Content, want) {
			t.Errorf("edited content missing %q:\n%s", want, added.Content)
		}
	}
	if reparsed := mustParsedModel(t, srv, added.Content); reparsed == "" {
		t.Fatal("edited notation did not parse")
	}
}

func TestApplyEditsNewConstraintAndStateKindsRefuseIllegalCases(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, `package Demo {
    enum def Values { enum one; }
    part def P;
}`)
	for _, operation := range []*pb.EditOperation{
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::Values", Kind: "assert constraint", Name: "bad", BodyExpression: "true",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::P", Kind: "part", Name: "bad", BodyExpression: "true",
		}}},
		{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner: "Demo::P", Kind: "do action", Name: "bad",
		}}},
	} {
		response, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
			ModelHash: hash, Operations: []*pb.EditOperation{operation},
		})
		if err != nil {
			t.Fatalf("ApplyEdits transport: %v", err)
		}
		if response.Content != "" || response.Failure != pb.EditFailure_EDIT_FAILURE_ILLEGAL_KIND {
			t.Errorf("ApplyEdits refusal = %+v, want illegal-kind with no content", response)
		}
	}
}

func TestApplyEditsConstraintRuntimeMatchesParsedTarget(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	source := `package P {
    private import ScalarValues::*;
    attribute x : Real = 3.0;
}`
	hash := mustParsedModel(t, srv, source)
	edited, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
				Owner: "P", Kind: "assert constraint", Name: "holding", BodyExpression: "x == 3.0",
			}}},
			{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
				Owner: "P", Kind: "assert not constraint", Name: "negated", BodyExpression: "x < 0.0",
			}}},
			{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
				Owner: "P", Kind: "assert constraint", Name: "violated", BodyExpression: "x < 0.0",
			}}},
		},
	})
	if err != nil || edited.Error != "" {
		t.Fatalf("ApplyEdits: response=%+v, err=%v", edited, err)
	}
	target := `package P {
    private import ScalarValues::*;
    attribute x : Real = 3.0;
    assert constraint holding { x == 3.0 }
    assert not constraint negated { x < 0.0 }
    assert constraint violated { x < 0.0 }
}`
	editedHash := mustParsedModel(t, srv, edited.Content)
	targetHash := mustParsedModel(t, srv, target)
	for _, tc := range []struct {
		symbol string
		holds  bool
	}{
		{"P::holding", true},
		{"P::negated", true},
		{"P::violated", false},
	} {
		editedVerdict, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{
			ModelHash: editedHash, SymbolId: tc.symbol,
		})
		if err != nil || editedVerdict.Error != "" {
			t.Fatalf("edited VerifyConstraint(%s): response=%+v err=%v", tc.symbol, editedVerdict, err)
		}
		targetVerdict, err := srv.VerifyConstraint(ctx, &pb.VerifyConstraintRequest{
			ModelHash: targetHash, SymbolId: tc.symbol,
		})
		if err != nil || targetVerdict.Error != "" {
			t.Fatalf("target VerifyConstraint(%s): response=%+v err=%v", tc.symbol, targetVerdict, err)
		}
		if editedVerdict.Verdict.Holds != tc.holds ||
			editedVerdict.Verdict.Holds != targetVerdict.Verdict.Holds ||
			editedVerdict.Verdict.Condition != targetVerdict.Verdict.Condition {
			t.Errorf("%s edited verdict=%+v target=%+v, want holds=%t",
				tc.symbol, editedVerdict.Verdict, targetVerdict.Verdict, tc.holds)
		}
	}
}

func TestApplyEditsStateActionExecutionMatchesParsedTarget(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 10)
	source := `package P {
    action def Work {
        out result : ScalarValues::Real = 2.0;
    }
    state def Machine {
        entry; then active;
        state active;
    }
}`
	hash := mustParsedModel(t, srv, source)
	edited, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{{Operation: &pb.EditOperation_AddMember{
			AddMember: &pb.AddMemberEdit{
				Owner: "P::Machine::active", Kind: "do action", Name: "work", Type: "Work",
			},
		}}},
	})
	if err != nil || edited.Error != "" {
		t.Fatalf("ApplyEdits: response=%+v err=%v", edited, err)
	}
	target := `package P {
    action def Work {
        out result : ScalarValues::Real = 2.0;
    }
    state def Machine {
        entry; then active;
        state active {
            do action work : Work;
        }
    }
}`
	editedHash := mustParsedModel(t, srv, edited.Content)
	targetHash := mustParsedModel(t, srv, target)
	editedRun, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
		ModelHash: editedHash, StateMachineSymbolId: "P::Machine",
	})
	if err != nil || editedRun.Error != "" {
		t.Fatalf("edited ExecuteState: response=%+v err=%v", editedRun, err)
	}
	targetRun, err := srv.ExecuteState(ctx, &pb.ExecuteStateRequest{
		ModelHash: targetHash, StateMachineSymbolId: "P::Machine",
	})
	if err != nil || targetRun.Error != "" {
		t.Fatalf("target ExecuteState: response=%+v err=%v", targetRun, err)
	}
	if !proto.Equal(editedRun, targetRun) {
		t.Fatalf("edited state run=%+v target=%+v", editedRun, targetRun)
	}
}

func TestApplyEditsNewFailureEnumsAreMapped(t *testing.T) {
	tests := []struct {
		failure edit.Failure
		want    pb.EditFailure
	}{
		{edit.FailureOwnerUnknown, pb.EditFailure_EDIT_FAILURE_OWNER_UNKNOWN},
		{edit.FailureOwnerNotNamespace, pb.EditFailure_EDIT_FAILURE_OWNER_NOT_NAMESPACE},
		{edit.FailureIllegalKind, pb.EditFailure_EDIT_FAILURE_ILLEGAL_KIND},
		{edit.FailureMemberNameTaken, pb.EditFailure_EDIT_FAILURE_MEMBER_NAME_TAKEN},
		{edit.FailureDeleteReferenced, pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED},
		{edit.FailureOwnerInsideTarget, pb.EditFailure_EDIT_FAILURE_OWNER_INSIDE_TARGET},
		{edit.FailureMoveReferenced, pb.EditFailure_EDIT_FAILURE_MOVE_REFERENCED},
	}
	for _, tc := range tests {
		if got := editFailureToProto(tc.failure); got != tc.want {
			t.Errorf("%s maps to %s, want %s", tc.failure, got, tc.want)
		}
	}
}

func TestGetServerInfoAuthoringCapabilities(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	for _, capability := range []string{
		CapabilityAuthoring, CapabilityConnectionAuthoring,
		CapabilitySatisfyAuthoring, CapabilityRequirementConstraintAuthoring,
		CapabilityMemberModifiers, CapabilityTransitionAuthoring,
		CapabilityConstraintBodyAuthoring, CapabilityStateActionAuthoring,
		CapabilityInlineLanguage,
	} {
		if !slices.Contains(info.Capabilities, capability) {
			t.Errorf("capabilities = %v, want %q", info.Capabilities, capability)
		}
	}
}

func TestParseFileInlineKerMLLanguage(t *testing.T) {
	srv := mustNewService(t, 10)
	content := "namespace N;"
	sysml, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: content},
	})
	if err != nil {
		t.Fatalf("SysML ParseFile: %v", err)
	}
	kerml, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:   &pb.ParseFileRequest_Content{Content: content},
		Language: "kerml",
	})
	if err != nil {
		t.Fatalf("KerML ParseFile: %v", err)
	}
	if len(kerml.Diagnostics) >= len(sysml.Diagnostics) {
		t.Fatalf("KerML diagnostics = %d, SysML diagnostics = %d; content was not interpreted as KerML",
			len(kerml.Diagnostics), len(sysml.Diagnostics))
	}
}
