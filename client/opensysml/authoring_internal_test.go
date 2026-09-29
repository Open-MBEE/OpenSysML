package opensysml

import (
	"context"
	"errors"
	"slices"
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
			name:         "documentation operation",
			operation:    AddDocumentation{Target: "Demo::r", Body: "A requirement."},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityDocumentationAuthoring,
		},
		{
			name:         "member documentation",
			operation:    AddMember{Owner: "Demo", Kind: "part def", Name: "X", Doc: "A definition."},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityDocumentationAuthoring,
		},
		{
			name:         "comment operation",
			operation:    AddComment{Owner: "Demo", Body: "A note."},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityCommentAuthoring,
		},
		{
			name:         "note operation",
			operation:    AddNote{Target: "Demo::r", Text: "A note."},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityCommentAuthoring,
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
		{
			name:         "sequence operation",
			operation:    AddSequence{Owner: "Demo::A", Keyword: "then", Ref: "done"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilitySequenceAuthoring,
		},
		{
			name: "action-body statement operation",
			operation: AddSequence{
				Owner: "Demo::A", Keyword: "then", MemberKind: "assign",
				Target: "x", Value: "1",
			},
			capabilities: []string{
				CapabilityApplyEdits, CapabilityAuthoring, CapabilitySequenceAuthoring,
			},
			missing: CapabilityActionBodyStatementAuthoring,
		},
		{
			name: "source-end multiplicity",
			operation: AddSequence{
				Owner: "Demo::A", Keyword: "then", Ref: "done", Multiplicity: "[1]",
			},
			capabilities: []string{
				CapabilityApplyEdits, CapabilityAuthoring, CapabilitySequenceAuthoring,
			},
			missing: CapabilityActionBodyStatementAuthoring,
		},
		{
			name:         "sequence requires authoring",
			operation:    AddSequence{Owner: "Demo::A", Keyword: "first", Ref: "start"},
			capabilities: []string{CapabilityApplyEdits, CapabilitySequenceAuthoring},
			missing:      CapabilityAuthoring,
		},
		{
			name: "constraint body",
			operation: AddMember{
				Owner: "Demo", Kind: "constraint", Name: "c", BodyExpression: "true",
			},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityConstraintBodyAuthoring,
		},
		{
			name:         "assert constraint",
			operation:    AddMember{Owner: "Demo", Kind: "assert constraint", Name: "c"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityConstraintBodyAuthoring,
		},
		{
			name:         "reference assertion",
			operation:    AddMember{Owner: "Demo", Kind: "assert", Name: "c"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityConstraintBodyAuthoring,
		},
		{
			name:         "negated reference assertion",
			operation:    AddMember{Owner: "Demo", Kind: "assert not", Name: "c"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityConstraintBodyAuthoring,
		},
		{
			name: "calculation result expression",
			operation: AddMember{
				Owner: "Demo", Kind: "calc def", Name: "D", BodyExpression: "x * 2",
			},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityConstraintBodyAuthoring,
		},
		{
			name:         "state behavior kind",
			operation:    AddMember{Owner: "Demo::S", Kind: "do action", Name: "run"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityStateActionAuthoring,
		},
		{
			name:         "import operation",
			operation:    AddImport{Owner: "Demo", Target: "ScalarValues::*"},
			capabilities: []string{CapabilityApplyEdits, CapabilityAuthoring},
			missing:      CapabilityImportAuthoring,
		},
		{
			name:         "import requires authoring",
			operation:    AddImport{Owner: "Demo", Target: "ScalarValues::*"},
			capabilities: []string{CapabilityApplyEdits, CapabilityImportAuthoring},
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
		BodyExpression:   "x > 1", Doc: "Checks the bound.",
	})
	if err != nil {
		t.Fatal(err)
	}
	member := memberOperation.GetAddMember()
	if member == nil || !member.GetIsAbstract() || !member.GetIsDefault() ||
		member.GetDirection() != "in" || len(member.GetRedefines()) != 1 ||
		member.GetRedefines()[0] != "Demo::old" ||
		len(member.GetMetadataPrefixes()) != 1 ||
		member.GetMetadataPrefixes()[0] != "Demo::Safety" ||
		member.GetBodyExpression() != "x > 1" ||
		member.GetDoc() != "Checks the bound." {
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
	documentedMember, err := editToProto(AddMember{Owner: "Demo", Kind: "part def", Name: "X", Doc: "Text."})
	if err != nil {
		t.Fatal(err)
	}
	if got := documentedMember.GetAddMember(); got == nil || got.GetDoc() != "Text." {
		t.Fatalf("AddMember doc mapping = %+v", got)
	}
	documentationOperation, err := editToProto(AddDocumentation{
		Target: "Demo::X", Body: "Text.", Name: "Summary", Locale: "en", Replace: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := documentationOperation.GetAddDocumentation(); got == nil ||
		got.GetTarget() != "Demo::X" || got.GetBody() != "Text." || got.GetName() != "Summary" ||
		got.GetLocale() != "en" || !got.GetReplace() {
		t.Fatalf("AddDocumentation mapping = %+v", got)
	}
	commentOperation, err := editToProto(AddComment{
		Owner: "Demo", Body: " Two\nlines ", Name: "Why", About: []string{"Demo::X", "Demo"}, Locale: "en",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := commentOperation.GetAddComment(); got == nil ||
		got.GetOwner() != "Demo" || got.GetBody() != " Two\nlines " || got.GetName() != "Why" ||
		!slices.Equal(got.GetAbout(), []string{"Demo::X", "Demo"}) || got.GetLocale() != "en" {
		t.Fatalf("AddComment mapping = %+v", got)
	}
	noteOperation, err := editToProto(AddNote{Target: "Demo::X", Text: "DimensionOneValue"})
	if err != nil {
		t.Fatal(err)
	}
	if got := noteOperation.GetAddNote(); got == nil ||
		got.GetTarget() != "Demo::X" || got.GetText() != "DimensionOneValue" {
		t.Fatalf("AddNote mapping = %+v", got)
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
	sequenceOperation, err := editToProto(AddSequence{
		Owner: "Demo::A", Keyword: "then", MemberKind: "action",
		MemberName: "b", Type: "B", After: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := sequenceOperation.GetAddSequence(); got == nil ||
		got.GetOwner() != "Demo::A" || got.GetKeyword() != "then" ||
		got.GetMemberKind() != "action" || got.GetMemberName() != "b" ||
		got.GetType() != "B" || got.GetAfter() != "a" || got.GetRef() != "" {
		t.Fatalf("AddSequence mapping = %+v", got)
	}
	refSequenceOperation, err := editToProto(AddSequence{
		Owner: "Demo::A", Keyword: "first", Ref: "start",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := refSequenceOperation.GetAddSequence(); got == nil ||
		got.GetKeyword() != "first" || got.GetRef() != "start" {
		t.Fatalf("AddSequence ref mapping = %+v", got)
	}
	nestedOperation, err := editToProto(AddSequence{
		Owner: "Demo::A", Keyword: "then", MemberKind: "if",
		Condition: "ready", Multiplicity: "[0..1]",
		Body: []AddSequence{{
			MemberKind: "if", Condition: "nested",
			Body: []AddSequence{{
				MemberKind: "send", Value: "payload", Target: "receiver",
			}},
		}},
		Else: []AddSequence{{Keyword: "else", Ref: "done"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := nestedOperation.GetAddSequence(); got == nil ||
		got.GetCondition() != "ready" || got.GetMultiplicity() != "[0..1]" ||
		len(got.GetBody()) != 1 || got.GetBody()[0].GetMemberKind() != "if" ||
		got.GetBody()[0].GetCondition() != "nested" ||
		len(got.GetBody()[0].GetBody()) != 1 ||
		got.GetBody()[0].GetBody()[0].GetMemberKind() != "send" ||
		got.GetBody()[0].GetBody()[0].GetValue() != "payload" ||
		got.GetBody()[0].GetBody()[0].GetTarget() != "receiver" ||
		len(got.GetElseBody()) != 1 || got.GetElseBody()[0].GetKeyword() != "else" {
		t.Fatalf("nested AddSequence mapping = %+v", got)
	}
	importOperation, err := editToProto(AddImport{
		Owner: "Demo", Visibility: "public", Target: "ScalarValues::*",
		Recursive: true, All: true, Filters: []string{"@Safety", "@Approved"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := importOperation.GetAddImport(); got == nil ||
		got.GetOwner() != "Demo" || got.GetVisibility() != "public" ||
		got.GetTarget() != "ScalarValues::*" || !got.GetIsRecursive() ||
		!got.GetIsImportAll() || len(got.GetFilters()) != 2 ||
		got.GetFilters()[0] != "@Safety" || got.GetFilters()[1] != "@Approved" {
		t.Fatalf("AddImport mapping = %+v", got)
	}
}
