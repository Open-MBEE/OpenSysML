package grpc

import (
	"context"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
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

func addImportOp(owner, visibility, target string, recursive, all bool, filters []string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddImport{
		AddImport: &pb.AddImportEdit{
			Owner: owner, Visibility: visibility, Target: target,
			IsRecursive: recursive, IsImportAll: all, Filters: filters,
		},
	}}
}

func addDocumentationOp(target, body string, replace bool) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_AddDocumentation{
		AddDocumentation: &pb.AddDocumentationEdit{Target: target, Body: body, Replace: replace},
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

func TestApplyEditsAddImportRequiresImportAuthoring(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityImportAuthoring)
	ctx := context.Background()
	hash := mustParsedModel(t, srv, "package P {\n}\n")
	_, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			addImportOp("P", "", "ScalarValues::*", false, false, nil),
		},
	})
	if connect.CodeOf(err) != connect.CodeUnimplemented ||
		!strings.Contains(err.Error(), CapabilityImportAuthoring) {
		t.Fatalf("ApplyEdits refusal = %v, want UNIMPLEMENTED naming %q", err, CapabilityImportAuthoring)
	}

	added, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash, Operations: []*pb.EditOperation{addMemberOp("P", "part", "a")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits add_member: %v", err)
	}
	if added.Error != "" || !strings.Contains(added.Content, "part a") {
		t.Fatalf("add_member response = %+v, want the new part", added)
	}
}

func TestApplyEditsAddImportWritesAndRefuses(t *testing.T) {
	srv := mustNewService(t, 10)
	ctx := context.Background()
	hash := mustParsedModel(t, srv, "package P {\n}\n")
	added, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			addImportOp("P", "", "ScalarValues::*", false, false, nil),
		},
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if added.Error != "" || !strings.Contains(added.Content, "private import ScalarValues::*;") {
		t.Fatalf("add_import response = %+v, want the import written", added)
	}

	refused, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			addImportOp("P", "", "Nope::*", false, false, nil),
		},
	})
	if err != nil {
		t.Fatalf("ApplyEdits refusal: %v", err)
	}
	if refused.Error == "" || refused.Content != "" {
		t.Fatalf("unresolved import response = %+v, want an in-band refusal", refused)
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
			name:       "import",
			capability: CapabilityImportAuthoring,
			operation:  addImportOp("Demo", "", "ScalarValues::*", false, false, nil),
		},
		{
			name:       "documentation",
			capability: CapabilityDocumentationAuthoring,
			operation:  addDocumentationOp("Demo::t", "A part.", false),
		},
		{
			name:       "member documentation",
			capability: CapabilityDocumentationAuthoring,
			operation: &pb.EditOperation{Operation: &pb.EditOperation_AddMember{
				AddMember: &pb.AddMemberEdit{Owner: "Demo", Kind: "part def", Name: "X", Doc: "A definition."},
			}},
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

func TestApplyEditsDocumentationRoundTrip(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, "package Demo {\n    item def Bread;\n}\n")
	added, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
				Owner: "Demo", Kind: "action def", Name: "ToastBread", Doc: "Toast it.\nEvenly.",
			}}},
			addDocumentationOp("Demo::Bread", "Sliced.", false),
			{Operation: &pb.EditOperation_AddDocumentation{AddDocumentation: &pb.AddDocumentationEdit{
				Target: "Demo", Body: "The demo.", Name: "Summary", Locale: "en",
			}}},
		},
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if added.Error != "" {
		t.Fatalf("edit refused: %s", added.Error)
	}
	want := "package Demo {\n    doc Summary locale \"en\" /* The demo. */\n    item def Bread {\n" +
		"        doc /* Sliced. */\n    }\n    action def ToastBread {\n" +
		"        doc /* Toast it.\n         * Evenly. */\n    }\n}\n"
	if added.Content != want {
		t.Fatalf("content =\n%s\nwant\n%s", added.Content, want)
	}

	hash = mustParsedModel(t, srv, added.Content)
	refused, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{addDocumentationOp("Demo::Bread", "Again.", false)},
	})
	if err != nil {
		t.Fatalf("ApplyEdits: %v", err)
	}
	if refused.Failure != pb.EditFailure_EDIT_FAILURE_MEMBER_NAME_TAKEN {
		t.Fatalf("second documentation = %+v, want MEMBER_NAME_TAKEN", refused)
	}
	replaced, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{addDocumentationOp("Demo::Bread", "Again.", true)},
	})
	if err != nil || replaced.Error != "" || !strings.Contains(replaced.Content, "doc /* Again. */") ||
		strings.Contains(replaced.Content, "Sliced.") {
		t.Fatalf("replaced documentation = %+v, %v", replaced, err)
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
		CapabilityMemberModifiers, CapabilityTransitionAuthoring, CapabilityInlineLanguage,
		CapabilityImportAuthoring, CapabilityDocumentationAuthoring,
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
