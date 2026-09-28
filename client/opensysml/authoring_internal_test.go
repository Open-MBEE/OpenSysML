package opensysml

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func (o *oldCaller) applyEdits(context.Context, *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error) {
	o.t.Fatal("an edit was sent without its required capability")
	return nil, nil
}

// A document name is refused before it leaves the client when the service lacks
// edit_documents, since such a service would ignore the name and edit its sole
// document instead: whether it predates the capability or GetServerInfo itself.
func TestADocumentNameIsNotSentWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	for name, old := range map[string]*oldCaller{
		"predates edit_documents": {t: t, capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring}},
		"predates GetServerInfo":  {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.ApplyDocumentEdits(ctx, model, "typo.sysml", Rename{Target: "P::x", NewName: "y"})
			wantUnimplemented(t, "ApplyDocumentEdits", err)
		})
	}
}

func TestAddConnectionIsNotSentWithoutItsCapabilities(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	tests := map[string]struct {
		capabilities []string
		missing      string
	}{
		"predates connection_authoring": {
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityConnectionAuthoring,
		},
		"predates authoring": {
			capabilities: []string{CapabilityApplyEdits, CapabilityConnectionAuthoring},
			missing:      CapabilityAuthoring,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			old := &oldCaller{t: t, capabilities: test.capabilities}
			c := &client{caller: old}
			_, err := c.ApplyEdits(ctx, model, AddConnection{
				Owner: "Demo::System", Kind: "allocation", From: "a", To: "b",
			})
			wantUnimplemented(t, "ApplyEdits AddConnection", err)
			var status *StatusError
			if !errors.As(err, &status) || !strings.Contains(status.Message, test.missing) {
				t.Errorf("ApplyEdits error = %v, want missing capability %q", err, test.missing)
			}
		})
	}
}

func TestNewAuthoringOperationsAreNotSentWithoutTheirCapabilities(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	tests := []struct {
		name         string
		operation    Edit
		capabilities []string
		missing      string
	}{
		{
			name: "abstract member",
			operation: AddMember{
				Owner: "Demo", Kind: "part def", Name: "X", IsAbstract: true,
			},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityMemberModifiers,
		},
		{
			name:         "ref member kind",
			operation:    AddMember{Owner: "Demo", Kind: "ref", Name: "x"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityMemberModifiers,
		},
		{
			name:         "return member kind",
			operation:    AddMember{Owner: "Demo", Kind: "return", Name: "result"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityMemberModifiers,
		},
		{
			name:         "satisfy operation",
			operation:    AddSatisfy{Owner: "Demo::r", Requirement: "r"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilitySatisfyAuthoring,
		},
		{
			name: "requirement constraint operation",
			operation: AddRequirementConstraint{
				Owner: "Demo::r", Kind: "require", Expression: "true",
			},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityRequirementConstraintAuthoring,
		},
		{
			name:         "transition operation",
			operation:    AddTransition{Owner: "Demo::S", Source: "idle", Target: "toasting"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityTransitionAuthoring,
		},
		{
			name:         "transition requires authoring",
			operation:    AddTransition{Owner: "Demo::S", Source: "idle", Target: "toasting"},
			capabilities: []string{CapabilityApplyEdits, CapabilityTransitionAuthoring},
			missing:      CapabilityAuthoring,
		},
		{
			name:         "verify operation",
			operation:    AddVerify{Owner: "Demo::Case", Requirement: "Demo::r"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityVerificationObjectiveAuthoring,
		},
		{
			name: "anonymous objective",
			operation: AddMember{
				Owner: "Demo::Case", Kind: "objective",
			},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityVerificationObjectiveAuthoring,
		},
		{
			name:         "metadata operation",
			operation:    AddMetadata{Owner: "Demo", MetadataType: "Demo::M"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityMetadataAuthoring,
		},
		{
			name: "metadata prefix",
			operation: AddMember{
				Owner: "Demo", Kind: "part def", Name: "P",
				MetadataPrefixes: []string{"Demo::M"},
			},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityMetadataAuthoring,
		},
		{
			name:         "verify requires authoring",
			operation:    AddVerify{Owner: "Demo::Case", Requirement: "Demo::r"},
			capabilities: []string{CapabilityApplyEdits, CapabilityVerificationObjectiveAuthoring},
			missing:      CapabilityAuthoring,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			old := &oldCaller{t: t, capabilities: test.capabilities}
			c := &client{caller: old}
			_, err := c.ApplyEdits(ctx, model, test.operation)
			wantUnimplemented(t, "ApplyEdits", err)
			var status *StatusError
			if !errors.As(err, &status) || !strings.Contains(status.Message, test.missing) {
				t.Fatalf("ApplyEdits error = %v, want missing capability %q", err, test.missing)
			}
		})
	}
}

func TestNewAuthoringOperationsMapToProto(t *testing.T) {
	memberOperation, err := editToProto(AddMember{
		Owner: "Demo", Kind: "attribute", Name: "x", IsAbstract: true,
		Redefines: []string{"Demo::old"}, IsDefault: true, Direction: "in",
		MetadataPrefixes: []string{"Demo::Safety"},
	})
	if err != nil {
		t.Fatal(err)
	}
	member := memberOperation.GetAddMember()
	if member == nil || !member.GetIsAbstract() || !member.GetIsDefault() ||
		member.GetDirection() != "in" || len(member.GetRedefines()) != 1 ||
		member.GetRedefines()[0] != "Demo::old" ||
		len(member.GetMetadataPrefixes()) != 1 ||
		member.GetMetadataPrefixes()[0] != "Demo::Safety" {
		t.Fatalf("AddMember mapping = %+v", member)
	}

	satisfyOperation, err := editToProto(AddSatisfy{
		Owner: "Demo::r", Requirement: "Demo::r", By: "Demo::t",
		Asserted: true, Negated: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := satisfyOperation.GetAddSatisfy(); got == nil ||
		got.GetOwner() != "Demo::r" || got.GetRequirement() != "Demo::r" ||
		got.GetSatisfyingFeature() != "Demo::t" || !got.GetIsAsserted() || !got.GetIsNegated() {
		t.Fatalf("AddSatisfy mapping = %+v", got)
	}

	constraintOperation, err := editToProto(AddRequirementConstraint{
		Owner: "Demo::r", Kind: "assume", Expression: "true", Name: "valid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := constraintOperation.GetAddRequirementConstraint(); got == nil ||
		got.GetOwner() != "Demo::r" || got.GetKind() != "assume" ||
		got.GetExpression() != "true" || got.GetName() != "valid" {
		t.Fatalf("AddRequirementConstraint mapping = %+v", got)
	}

	transitionOperation, err := editToProto(AddTransition{
		Owner: "Demo::S", Name: "go", Source: "idle", Target: "toasting",
		Trigger: "CycleStart", Guard: "ready", Effect: "action cool",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := transitionOperation.GetAddTransition(); got == nil ||
		got.GetOwner() != "Demo::S" || got.GetName() != "go" ||
		got.GetSource() != "idle" || got.GetTarget() != "toasting" ||
		got.GetTrigger() != "CycleStart" || got.GetGuard() != "ready" ||
		got.GetEffect() != "action cool" || got.GetInitial() {
		t.Fatalf("AddTransition mapping = %+v", got)
	}
	entryOperation, err := editToProto(AddEntryTransition("Demo::S", "idle"))
	if err != nil {
		t.Fatal(err)
	}
	if got := entryOperation.GetAddTransition(); got == nil ||
		got.GetOwner() != "Demo::S" || got.GetTarget() != "idle" || !got.GetInitial() {
		t.Fatalf("AddEntryTransition mapping = %+v", got)
	}

	verifyOperation, err := editToProto(AddVerify{
		Owner: "Demo::Case", Requirement: "Demo::r",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := verifyOperation.GetAddVerify(); got == nil ||
		got.GetOwner() != "Demo::Case" || got.GetRequirement() != "Demo::r" {
		t.Fatalf("AddVerify mapping = %+v", got)
	}

	metadataOperation, err := editToProto(AddMetadata{
		Owner: "Demo::Case", MetadataType: "Demo::M", Name: "m",
		About:     []string{"Demo::x", "Demo::y"},
		Values:    []MetadataValue{{Feature: "kind", Value: "Kind::test"}},
		Shorthand: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := metadataOperation.GetAddMetadata(); got == nil ||
		got.GetOwner() != "Demo::Case" || got.GetMetadataType() != "Demo::M" ||
		got.GetName() != "m" || len(got.GetAbout()) != 2 ||
		got.GetAbout()[0] != "Demo::x" || !got.GetShorthand() ||
		len(got.GetValues()) != 1 || got.GetValues()[0].GetFeature() != "kind" ||
		got.GetValues()[0].GetValue() != "Kind::test" {
		t.Fatalf("AddMetadata mapping = %+v", got)
	}
}
