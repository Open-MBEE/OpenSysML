package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ApplyEdits edits the source a model was parsed from and returns the edited
// notation, so a client can change a model and write it back with its comments
// and layout intact. Argument faults fail the call; an edit the engine refuses
// is reported in the response's error, failure kind and diagnostics.
func (s *Service) ApplyEdits(ctx context.Context, req *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error) {
	if err := s.requireCapability(CapabilityApplyEdits); err != nil {
		return nil, err
	}
	if requestsAuthoring(req.Operations) {
		if err := s.requireCapability(CapabilityAuthoring); err != nil {
			return nil, err
		}
	}
	if req.ModelHash == "" {
		return nil, statusError(connect.CodeInvalidArgument, "model_hash is required")
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound,
			"model %s is no longer cached: parse it again before editing it", req.ModelHash)
	}

	ops, err := editOperations(req.Operations)
	if err != nil {
		return nil, err
	}

	// Edits rewrite one document's own notation, so a model of several is refused
	// rather than edited in one of them.
	doc, err := cached.SoleDocument()
	if err != nil {
		return nil, err
	}

	model := edit.Model{
		Source:     doc.Source,
		Root:       doc.Root,
		Index:      cached.Index,
		ParseDiags: doc.ParseDiags,
		SemDiags:   doc.PassesDiags,
		// The edited notation is analyzed in an index of its own: the model's own
		// index already holds the document under this name, and the libraries are
		// what the new source has to resolve against.
		NewIndex: func() *symbols.Index {
			idx, _ := s.libIndexes.get()
			return idx
		},
	}
	result, err := edit.Apply(model, ops)
	if err != nil {
		return s.editRefusal(err, doc.Source)
	}
	return &pb.ApplyEditsResponse{
		Content: string(result.Content),
		Applied: appliedToProto(result.Applied),
	}, nil
}

func requestsAuthoring(operations []*pb.EditOperation) bool {
	for _, operation := range operations {
		switch operation.GetOperation().(type) {
		case *pb.EditOperation_AddMember, *pb.EditOperation_Delete:
			return true
		}
	}
	return false
}

// editOperations reads the operations a request carries, rejecting a request
// that names none of the two forms: an unset operation is a client fault rather
// than a refused edit.
func editOperations(pbOps []*pb.EditOperation) ([]edit.Operation, error) {
	ops := make([]edit.Operation, 0, len(pbOps))
	for i, pbOp := range pbOps {
		switch op := pbOp.GetOperation().(type) {
		case *pb.EditOperation_SetValue:
			ops = append(ops, edit.SetValue(op.SetValue.GetTarget(), op.SetValue.GetValue()))
		case *pb.EditOperation_Rename:
			ops = append(ops, edit.Rename(op.Rename.GetTarget(), op.Rename.GetNewName()))
		case *pb.EditOperation_AddMember:
			add := op.AddMember
			member := edit.AddMember(add.GetOwner(), add.GetKind(), add.GetName())
			member.Type = add.GetType()
			member.Multiplicity = add.GetMultiplicity()
			member.Value = add.GetValue()
			member.Specializes = append([]string(nil), add.GetSpecializes()...)
			ops = append(ops, member)
		case *pb.EditOperation_Delete:
			del := op.Delete
			ops = append(ops, edit.Delete(del.GetTarget(), del.GetCascade()))
		default:
			return nil, statusErrorf(connect.CodeInvalidArgument,
				"operation %d must be set_value, rename, add_member or delete", i)
		}
	}
	return ops, nil
}

// editRefusal reports a refused edit as a response rather than a call failure:
// the request was well formed, and the answer is why the model was not edited.
func (s *Service) editRefusal(err error, sf *source.SourceFile) (*pb.ApplyEditsResponse, error) {
	var refusal *edit.Error
	if !errors.As(err, &refusal) {
		return nil, statusErrorf(connect.CodeInternal, "apply edits: %v", err)
	}
	resp := &pb.ApplyEditsResponse{
		Error:             refusal.Message,
		Failure:           editFailureToProto(refusal.Failure),
		ReferringElements: refusal.Referring,
	}
	// Diagnostic spans are offsets into what was diagnosed: the new value's text
	// or the edited notation, not the model as the client has it.
	diagnosed := refusal.Diagnosed
	if diagnosed == nil {
		diagnosed = sf
	}
	for _, diag := range refusal.Diagnostics {
		resp.Diagnostics = append(resp.Diagnostics, DiagnosticToProto(diag, diagnosed))
	}
	s.filterDiagnosticCapabilities(resp.Diagnostics)
	return resp, nil
}

// appliedToProto reports what each operation changed.
func appliedToProto(applied []edit.Applied) []*pb.AppliedEdit {
	out := make([]*pb.AppliedEdit, 0, len(applied))
	for _, a := range applied {
		out = append(out, &pb.AppliedEdit{
			OperationIndex: int32Clamp(a.OperationIndex),
			Target:         a.Target,
			Offset:         int32Clamp(a.Span.Offset),
			Length:         int32Clamp(a.Span.Len),
			OldText:        a.OldText,
			NewText:        a.NewText,
		})
	}
	return out
}

// editFailures maps every refusal kind to its wire value, so a client acts on
// the kind rather than on the message text.
var editFailures = map[edit.Failure]pb.EditFailure{
	edit.FailureNone:              pb.EditFailure_EDIT_FAILURE_UNSPECIFIED,
	edit.FailureNoOperations:      pb.EditFailure_EDIT_FAILURE_NO_OPERATIONS,
	edit.FailureUnknownTarget:     pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET,
	edit.FailureAmbiguousTarget:   pb.EditFailure_EDIT_FAILURE_AMBIGUOUS_TARGET,
	edit.FailureNotValued:         pb.EditFailure_EDIT_FAILURE_NOT_VALUED,
	edit.FailureInvalidValue:      pb.EditFailure_EDIT_FAILURE_INVALID_VALUE,
	edit.FailureInvalidName:       pb.EditFailure_EDIT_FAILURE_INVALID_NAME,
	edit.FailureNotNamed:          pb.EditFailure_EDIT_FAILURE_NOT_NAMED,
	edit.FailureRenameReferenced:  pb.EditFailure_EDIT_FAILURE_RENAME_REFERENCED,
	edit.FailureOverlappingEdits:  pb.EditFailure_EDIT_FAILURE_OVERLAPPING_EDITS,
	edit.FailureResultInvalid:     pb.EditFailure_EDIT_FAILURE_RESULT_INVALID,
	edit.FailureOwnerUnknown:      pb.EditFailure_EDIT_FAILURE_OWNER_UNKNOWN,
	edit.FailureOwnerNotNamespace: pb.EditFailure_EDIT_FAILURE_OWNER_NOT_NAMESPACE,
	edit.FailureIllegalKind:       pb.EditFailure_EDIT_FAILURE_ILLEGAL_KIND,
	edit.FailureMemberNameTaken:   pb.EditFailure_EDIT_FAILURE_MEMBER_NAME_TAKEN,
	edit.FailureDeleteReferenced:  pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED,
}

func editFailureToProto(f edit.Failure) pb.EditFailure {
	if v, ok := editFailures[f]; ok {
		return v
	}
	return pb.EditFailure_EDIT_FAILURE_UNSPECIFIED
}
