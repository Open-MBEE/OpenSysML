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
	})
	if err != nil {
		t.Fatal(err)
	}
	member := memberOperation.GetAddMember()
	if member == nil || !member.GetIsAbstract() || !member.GetIsDefault() ||
		member.GetDirection() != "in" || len(member.GetRedefines()) != 1 ||
		member.GetRedefines()[0] != "Demo::old" {
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
}
