package grpc

import (
	"context"
	"errors"
	"sort"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ApplyEdits edits the source a model was parsed from and returns the edited
// notation, so a client can change a model and write it back with its comments
// and layout intact. A model of several documents is edited as one, for a
// request that accepts documents: the operations target the document the
// request names, a rename or cascade delete follows references into every
// other, and the response carries each document the edits rewrote, or none
// when they were refused. A request not accepting documents is refused on such
// a model, since it reads only content. Argument faults fail the call; an edit
// the engine refuses is reported in the response's error, failure kind and
// diagnostics. Without the edit_documents capability the service answers as one
// predating documents: a model of several is refused, a named document is
// refused with UNIMPLEMENTED, and the response carries content alone.
func (s *Service) ApplyEdits(ctx context.Context, req *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error) {
	if err := s.requireCapability(CapabilityApplyEdits); err != nil {
		return nil, err
	}
	if requestsAuthoring(req.Operations) {
		if err := s.requireCapability(CapabilityAuthoring); err != nil {
			return nil, err
		}
	}
	documents := s.capabilities.has(CapabilityEditDocuments)
	if req.Document != "" && !documents {
		return nil, s.requireCapability(CapabilityEditDocuments)
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
	if !documents && len(cached.Documents) != 1 {
		return nil, statusErrorf(connect.CodeFailedPrecondition,
			"the model has %d documents, and this service edits one alone: it lacks the %s capability",
			len(cached.Documents), CapabilityEditDocuments)
	}
	if err := acceptsDocuments(cached, req.AcceptDocuments); err != nil {
		return nil, err
	}
	edited, err := editedDocument(cached, req.Document)
	if err != nil {
		return nil, err
	}

	result, err := edit.Apply(s.editModel(cached, edited), ops)
	if err != nil {
		resp, err := s.editRefusal(err, edited.Source)
		return withoutDocuments(resp, documents), err
	}
	resp := editResultToProto(result, edited.Source.Name(), len(cached.Documents) == 1)
	return withoutDocuments(resp, documents), nil
}

// withoutDocuments strips the fields the edit_documents capability advertises
// from a response of a service withholding it.
func withoutDocuments(resp *pb.ApplyEditsResponse, documents bool) *pb.ApplyEditsResponse {
	if documents || resp == nil {
		return resp
	}
	resp.Documents = nil
	resp.Referrers = nil
	for _, applied := range resp.Applied {
		applied.Document = ""
	}
	return resp
}

// acceptsDocuments refuses a model of several documents for a request that did not say it
// reads the response's documents: it reads content alone, which such a model leaves empty.
func acceptsDocuments(cached *CachedModel, accept bool) error {
	if accept || len(cached.Documents) == 1 {
		return nil
	}
	return statusErrorf(connect.CodeFailedPrecondition,
		"the model has %d documents, and the request reads only content: "+
			"set accept_documents to have each edited document answered in documents", len(cached.Documents))
}

// editedDocument is the document a request's operations target: the one it
// names, or the model's first when it names none.
func editedDocument(cached *CachedModel, name string) (*CachedDocument, error) {
	if name == "" {
		return cached.Primary(), nil
	}
	for _, doc := range cached.Documents {
		if doc.Source.Name() == name {
			return doc, nil
		}
	}
	names := make([]string, 0, len(cached.Documents))
	for _, doc := range cached.Documents {
		names = append(names, doc.Source.Name())
	}
	return nil, statusErrorf(connect.CodeInvalidArgument,
		"document %q is not one of the model's: it has %s", name, strings.Join(names, ", "))
}

// editModel is the cached model as the edit engine reads it: edited is the
// document the operations target, and every other is one a rename or delete
// may rewrite.
func (s *Service) editModel(cached *CachedModel, edited *CachedDocument) edit.Model {
	others := make([]*CachedDocument, 0, len(cached.Documents)-1)
	for _, doc := range cached.Documents {
		if doc != edited {
			others = append(others, doc)
		}
	}
	return edit.Model{
		Source:     edited.Source,
		Root:       edited.Root,
		Index:      cached.Index,
		ParseDiags: edited.ParseDiags,
		SemDiags:   edited.PassesDiags,
		Analysis:   passes.Options{Conformance: cached.Mode},
		// The edited notation is analyzed in an index of its own, over the
		// libraries and every document of the model the edit did not rewrite.
		NewIndex: func() *symbols.Index {
			idx, _ := s.libIndexes.get()
			for _, doc := range others {
				idx.AddDocumentWithKind(doc.Source.Name(), doc.Root, doc.Source.Kind())
			}
			idx.ExpandWildcardImports()
			return idx
		},
		Other: func(name string) (edit.Document, bool) {
			for _, doc := range others {
				if doc.Source.Name() == name {
					return edit.Document{Source: doc.Source, ParseDiags: doc.ParseDiags, SemDiags: doc.PassesDiags}, true
				}
			}
			return edit.Document{}, false
		},
	}
}

// editResultToProto reports the documents the edits rewrote, the edited one
// first when it is among them, then the others in name order. content repeats
// the notation for a single-document model alone, so a client reading it alone
// never writes one document's notation over another's.
func editResultToProto(result *edit.Result, edited string, sole bool) *pb.ApplyEditsResponse {
	resp := &pb.ApplyEditsResponse{}
	if sole || len(result.Applied) > 0 {
		resp.Documents = append(resp.Documents, &pb.EditedDocument{Name: edited, Content: string(result.Content)})
		resp.Applied = append(resp.Applied, appliedToProto(result.Applied, edited)...)
	}
	for _, other := range result.Others {
		resp.Documents = append(resp.Documents, &pb.EditedDocument{Name: other.Name, Content: string(other.Content)})
		resp.Applied = append(resp.Applied, appliedToProto(other.Applied, other.Name)...)
	}
	if sole {
		resp.Content = string(result.Content)
	}
	return resp
}

func requestsAuthoring(operations []*pb.EditOperation) bool {
	for _, operation := range operations {
		switch operation.GetOperation().(type) {
		case *pb.EditOperation_AddMember, *pb.EditOperation_Delete, *pb.EditOperation_Move:
			return true
		}
	}
	return false
}

// editOperations reads the operations a request carries, rejecting a request
// that names none of the forms: an unset operation is a client fault rather
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
		case *pb.EditOperation_Move:
			ops = append(ops, edit.Move(op.Move.GetTarget(), op.Move.GetOwner()))
		default:
			return nil, statusErrorf(connect.CodeInvalidArgument,
				"operation %d must be set_value, rename, add_member, delete or move", i)
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
	referring, referrers := sortedReferrers(refusal)
	resp := &pb.ApplyEditsResponse{
		Error:             refusal.Message,
		Failure:           editFailureToProto(refusal.Failure),
		ReferringElements: referring,
		Referrers:         referrers,
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

// sortedReferrers reports a refusal's referrers in document then name order, the
// legacy spelling of each kept beside it, whichever order the engine found them in.
func sortedReferrers(refusal *edit.Error) ([]string, []*pb.Referrer) {
	if len(refusal.Referring) != len(refusal.Referrers) {
		return refusal.Referring, referrersToProto(refusal.Referrers)
	}
	order := make([]int, len(refusal.Referrers))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := refusal.Referrers[order[i]], refusal.Referrers[order[j]]
		if a.Document != b.Document {
			return a.Document < b.Document
		}
		return a.Name < b.Name
	})
	referring := make([]string, 0, len(order))
	referrers := make([]edit.Referrer, 0, len(order))
	for _, i := range order {
		referring = append(referring, refusal.Referring[i])
		referrers = append(referrers, refusal.Referrers[i])
	}
	return referring, referrersToProto(referrers)
}

// referrersToProto reports each referrer with the document declaring it.
func referrersToProto(referrers []edit.Referrer) []*pb.Referrer {
	out := make([]*pb.Referrer, 0, len(referrers))
	for _, r := range referrers {
		out = append(out, &pb.Referrer{Name: r.Name, Document: r.Document})
	}
	return out
}

// appliedToProto reports what each operation changed in the document named document.
func appliedToProto(applied []edit.Applied, document string) []*pb.AppliedEdit {
	out := make([]*pb.AppliedEdit, 0, len(applied))
	for _, a := range applied {
		out = append(out, &pb.AppliedEdit{
			OperationIndex: int32Clamp(a.OperationIndex),
			Target:         a.Target,
			Offset:         int32Clamp(a.Span.Offset),
			Length:         int32Clamp(a.Span.Len),
			OldText:        a.OldText,
			NewText:        a.NewText,
			Document:       document,
		})
	}
	return out
}

// editFailures maps every refusal kind to its wire value, so a client acts on
// the kind rather than on the message text.
var editFailures = map[edit.Failure]pb.EditFailure{
	edit.FailureNone:                pb.EditFailure_EDIT_FAILURE_UNSPECIFIED,
	edit.FailureNoOperations:        pb.EditFailure_EDIT_FAILURE_NO_OPERATIONS,
	edit.FailureUnknownTarget:       pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET,
	edit.FailureAmbiguousTarget:     pb.EditFailure_EDIT_FAILURE_AMBIGUOUS_TARGET,
	edit.FailureNotValued:           pb.EditFailure_EDIT_FAILURE_NOT_VALUED,
	edit.FailureInvalidValue:        pb.EditFailure_EDIT_FAILURE_INVALID_VALUE,
	edit.FailureInvalidName:         pb.EditFailure_EDIT_FAILURE_INVALID_NAME,
	edit.FailureNotNamed:            pb.EditFailure_EDIT_FAILURE_NOT_NAMED,
	edit.FailureRenameReferenced:    pb.EditFailure_EDIT_FAILURE_RENAME_REFERENCED,
	edit.FailureOverlappingEdits:    pb.EditFailure_EDIT_FAILURE_OVERLAPPING_EDITS,
	edit.FailureResultInvalid:       pb.EditFailure_EDIT_FAILURE_RESULT_INVALID,
	edit.FailureOwnerUnknown:        pb.EditFailure_EDIT_FAILURE_OWNER_UNKNOWN,
	edit.FailureOwnerNotNamespace:   pb.EditFailure_EDIT_FAILURE_OWNER_NOT_NAMESPACE,
	edit.FailureIllegalKind:         pb.EditFailure_EDIT_FAILURE_ILLEGAL_KIND,
	edit.FailureMemberNameTaken:     pb.EditFailure_EDIT_FAILURE_MEMBER_NAME_TAKEN,
	edit.FailureDeleteReferenced:    pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED,
	edit.FailureOwnerInsideTarget:   pb.EditFailure_EDIT_FAILURE_OWNER_INSIDE_TARGET,
	edit.FailureMoveReferenced:      pb.EditFailure_EDIT_FAILURE_MOVE_REFERENCED,
	edit.FailureReferencedElsewhere: pb.EditFailure_EDIT_FAILURE_REFERENCED_ELSEWHERE,
}

func editFailureToProto(f edit.Failure) pb.EditFailure {
	if v, ok := editFailures[f]; ok {
		return v
	}
	return pb.EditFailure_EDIT_FAILURE_UNSPECIFIED
}
