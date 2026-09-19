package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// editModelSource carries comments and blank lines an edit must leave alone.
const editModelSource = `package Demo {
    // The mass of one unit, measured on the bench.
    part def SC {
        attribute unitMass : ISQ::MassValue = 1000.0[SI::kg];

        attribute margin : ISQ::MassValue;
    }

    part sc : SC;
}
`

// mustParsedModel parses notation and returns the hash the service cached it
// under, which is how an edit names the model to edit.
func mustParsedModel(t *testing.T, srv *Service, content string) string {
	t.Helper()
	parsed, err := srv.ParseFile(context.Background(),
		&pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: content}})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	return parsed.ModelHash
}

func setValueOp(target, value string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_SetValue{
		SetValue: &pb.SetValueEdit{Target: target, Value: value},
	}}
}

// TestApplyEditsSetValuePreservesSource verifies an edited value comes back with
// every other byte of the notation, comments and blank lines included, intact.
func TestApplyEditsSetValuePreservesSource(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, editModelSource)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  hash,
		Operations: []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "1050.0[SI::kg]")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("edit refused: %s", resp.Error)
	}
	want := strings.Replace(editModelSource, "1000.0[SI::kg]", "1050.0[SI::kg]", 1)
	if resp.Content != want {
		t.Errorf("content =\n%s\nwant\n%s", resp.Content, want)
	}
	if len(resp.Applied) != 1 {
		t.Fatalf("applied %d edits, want 1", len(resp.Applied))
	}
	applied := resp.Applied[0]
	if applied.Target != "Demo::SC::unitMass" || applied.OldText != "1000.0[SI::kg]" ||
		applied.NewText != "1050.0[SI::kg]" {
		t.Errorf("applied edit = %+v, want the unitMass value replaced", applied)
	}
	if int(applied.Offset) != strings.Index(editModelSource, "1000.0[SI::kg]") {
		t.Errorf("applied offset = %d, want the value's own span", applied.Offset)
	}
}

// TestApplyEditsAddsValueAndRenames verifies the other two shapes over the wire:
// a value added to a feature that had none, and a rename of an unreferenced
// declaration.
func TestApplyEditsAddsValueAndRenames(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, editModelSource)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{
			setValueOp("Demo::SC::margin", "50.0[SI::kg]"),
			{Operation: &pb.EditOperation_Rename{
				Rename: &pb.RenameEdit{Target: "Demo::SC::unitMass", NewName: "unitWeight"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("edit refused: %s", resp.Error)
	}
	if !strings.Contains(resp.Content, "attribute margin : ISQ::MassValue = 50.0[SI::kg];") {
		t.Errorf("value not added before the semicolon:\n%s", resp.Content)
	}
	if !strings.Contains(resp.Content, "attribute unitWeight : ISQ::MassValue") {
		t.Errorf("declaration not renamed:\n%s", resp.Content)
	}
	if !strings.Contains(resp.Content, "// The mass of one unit, measured on the bench.") {
		t.Errorf("comment lost:\n%s", resp.Content)
	}
	if len(resp.Applied) != 2 {
		t.Errorf("applied %d edits, want 2", len(resp.Applied))
	}
}

// TestApplyEditsRefusalIsAResponse verifies a refused edit answers with its
// failure kind and no content, rather than failing the call.
func TestApplyEditsRefusalIsAResponse(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, editModelSource)

	cases := []struct {
		name    string
		ops     []*pb.EditOperation
		failure pb.EditFailure
	}{
		{
			name:    "unknown target",
			ops:     []*pb.EditOperation{setValueOp("Demo::SC::nothing", "1")},
			failure: pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET,
		},
		{
			name:    "target carries no value",
			ops:     []*pb.EditOperation{setValueOp("Demo::SC", "1")},
			failure: pb.EditFailure_EDIT_FAILURE_NOT_VALUED,
		},
		{
			name:    "value does not parse",
			ops:     []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "1050.0[")},
			failure: pb.EditFailure_EDIT_FAILURE_INVALID_VALUE,
		},
		{
			name:    "value does not resolve",
			ops:     []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "nosuchFeature")},
			failure: pb.EditFailure_EDIT_FAILURE_RESULT_INVALID,
		},
		{
			name: "overlapping edits",
			ops: []*pb.EditOperation{
				setValueOp("Demo::SC::unitMass", "1"),
				setValueOp("Demo::SC::unitMass", "2"),
			},
			failure: pb.EditFailure_EDIT_FAILURE_OVERLAPPING_EDITS,
		},
		{
			name:    "no operations",
			ops:     nil,
			failure: pb.EditFailure_EDIT_FAILURE_NO_OPERATIONS,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := srv.ApplyEdits(context.Background(),
				&pb.ApplyEditsRequest{ModelHash: hash, Operations: tc.ops})
			if err != nil {
				t.Fatalf("ApplyEdits failed the call: %v", err)
			}
			if resp.Failure != tc.failure {
				t.Errorf("failure = %s, want %s: %s", resp.Failure, tc.failure, resp.Error)
			}
			if resp.Error == "" {
				t.Error("refusal carries no message")
			}
			if resp.Content != "" {
				t.Errorf("refusal returned content:\n%s", resp.Content)
			}
		})
	}
}

// TestApplyEditsRenameRewritesReferences verifies a rename over the wire
// rewrites the references to the renamed element, not the declaration alone.
func TestApplyEditsRenameRewritesReferences(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, `package Demo {
    part def SC {
        attribute unitMass : ISQ::MassValue = 1000.0[SI::kg];
        attribute total : ISQ::MassValue = unitMass;
    }
}
`)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{{Operation: &pb.EditOperation_Rename{
			Rename: &pb.RenameEdit{Target: "Demo::SC::unitMass", NewName: "unitWeight"},
		}}},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_UNSPECIFIED {
		t.Fatalf("failure = %s: %s", resp.Failure, resp.Error)
	}
	for _, want := range []string{
		"attribute unitWeight : ISQ::MassValue = 1000.0[SI::kg];",
		"attribute total : ISQ::MassValue = unitWeight;",
	} {
		if !strings.Contains(resp.Content, want) {
			t.Fatalf("edited content missing %q:\n%s", want, resp.Content)
		}
	}
	if len(resp.Applied) != 2 {
		t.Fatalf("applied %d edits, want 2 (declaration and reference)", len(resp.Applied))
	}
}

// A rename through the edit API follows the batch edit layer's rule: a reference
// written with the element's short name still resolves, so it is left as written.
func TestApplyEditsRenameLeavesShortNameReferences(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, "package P {\n\tpart def <O> Old;\n\tpart a : O;\n\tpart b : Old;\n}\n")

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{{Operation: &pb.EditOperation_Rename{
			Rename: &pb.RenameEdit{Target: "P::Old", NewName: "Fresh"},
		}}},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_UNSPECIFIED {
		t.Fatalf("failure = %s: %s", resp.Failure, resp.Error)
	}
	if want := "package P {\n\tpart def <O> Fresh;\n\tpart a : O;\n\tpart b : Fresh;\n}\n"; resp.Content != want {
		t.Fatalf("edited content:\n%s\nwant:\n%s", resp.Content, want)
	}
	if len(resp.Applied) != 2 {
		t.Fatalf("applied %d edits, want 2 (declaration and long-name reference)", len(resp.Applied))
	}
}

// TestApplyEditsUncachedModelIsNotFound verifies an evicted model is named as
// such, the way convert names it, rather than edited as empty notation.
func TestApplyEditsUncachedModelIsNotFound(t *testing.T) {
	srv := mustNewService(t, 10)

	_, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash:  "nosuchmodel",
		Operations: []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "1")},
	})
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("err = %v, want NotFound", err)
	}
}

// TestApplyEditsRequiresModelHash verifies an argument fault fails the call.
func TestApplyEditsRequiresModelHash(t *testing.T) {
	srv := mustNewService(t, 10)

	_, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		Operations: []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "1")},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("err = %v, want InvalidArgument", err)
	}
}

// TestApplyEditsCapabilityIsReported verifies a client can negotiate the RPC
// before calling it.
func TestApplyEditsCapabilityIsReported(t *testing.T) {
	srv := mustNewService(t, 10)

	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatalf("GetServerInfo failed: %v", err)
	}
	for _, c := range info.Capabilities {
		if c == CapabilityApplyEdits {
			return
		}
	}
	t.Errorf("capabilities %v do not include %s", info.Capabilities, CapabilityApplyEdits)
}
