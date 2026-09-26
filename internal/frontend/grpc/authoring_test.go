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
	for _, capability := range []string{CapabilityAuthoring, CapabilityInlineLanguage} {
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
