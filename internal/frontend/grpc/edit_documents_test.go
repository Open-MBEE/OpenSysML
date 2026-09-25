package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const (
	editDocP = "package P {\n    part def Base;\n    part def Keep;\n    part own : Base;\n}\n"
	editDocQ = "package Q {\n    part b : P::Base;\n    part k : P::Keep;\n}\n"
	editDocR = "package R {\n    part c = Q::b;\n    part d : P::Keep;\n}\n"
)

// mustParsedSources parses named inline documents as one model and returns its hash.
func mustParsedSources(t *testing.T, srv *Service, named ...string) string {
	t.Helper()
	parsed, err := srv.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: inlineDocuments(named...)})
	if err != nil {
		t.Fatalf("ParseSources failed: %v", err)
	}
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("fixture has diagnostics: %v", parsed.Diagnostics)
	}
	return parsed.ModelHash
}

func renameOp(target, newName string) *pb.EditOperation {
	return &pb.EditOperation{Operation: &pb.EditOperation_Rename{
		Rename: &pb.RenameEdit{Target: target, NewName: newName},
	}}
}

// mustApplied applies ops to the model's first document and fails the test on
// a call failure or a refusal.
func mustApplied(t *testing.T, srv *Service, hash string, ops ...*pb.EditOperation) *pb.ApplyEditsResponse {
	t.Helper()
	return mustAppliedIn(t, srv, hash, "", ops...)
}

// mustAppliedIn applies ops to the named document of the model and fails the
// test on a call failure or a refusal.
func mustAppliedIn(t *testing.T, srv *Service, hash, document string, ops ...*pb.EditOperation) *pb.ApplyEditsResponse {
	t.Helper()
	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{ModelHash: hash, Document: document, Operations: ops, AcceptDocuments: true})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Error != "" || resp.Failure != pb.EditFailure_EDIT_FAILURE_UNSPECIFIED {
		t.Fatalf("edit refused (%s): %s", resp.Failure, resp.Error)
	}
	return resp
}

// documentNames lists the documents of a response in order.
func documentNames(resp *pb.ApplyEditsResponse) []string {
	names := make([]string, 0, len(resp.Documents))
	for _, doc := range resp.Documents {
		names = append(names, doc.Name)
	}
	return names
}

// documentContent is the edited notation of the named document, or fails.
func documentContent(t *testing.T, resp *pb.ApplyEditsResponse, name string) string {
	t.Helper()
	for _, doc := range resp.Documents {
		if doc.Name == name {
			return doc.Content
		}
	}
	t.Fatalf("documents %v do not include %s", documentNames(resp), name)
	return ""
}

// A single-document model answers in both shapes: content as before, and
// documents with one entry, named as the parse named it, carrying the same notation.
func TestApplyEditsSingleDocumentFillsContentAndDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedModel(t, srv, editModelSource)

	resp := mustApplied(t, srv, hash, setValueOp("Demo::SC::unitMass", "1050.0[SI::kg]"))
	want := strings.Replace(editModelSource, "1000.0[SI::kg]", "1050.0[SI::kg]", 1)
	if resp.Content != want {
		t.Errorf("content =\n%s\nwant\n%s", resp.Content, want)
	}
	if len(resp.Documents) != 1 {
		t.Fatalf("documents = %v, want one", documentNames(resp))
	}
	if resp.Documents[0].Content != resp.Content {
		t.Errorf("documents[0].content differs from content:\n%s", resp.Documents[0].Content)
	}
	if got, want := resp.Documents[0].Name, "<content>"; got != want {
		t.Errorf("documents[0].name = %q, want the parse's name %q", got, want)
	}
	if len(resp.Applied) != 1 || resp.Applied[0].Document != "<content>" {
		t.Errorf("applied = %v, want one edit in <content>", resp.Applied)
	}
}

// A single document parsed through ParseSources under a name of its own is
// listed under that name.
func TestApplyEditsSingleDocumentIsNamedAsParsed(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "demo.sysml", editModelSource)

	resp := mustApplied(t, srv, hash, setValueOp("Demo::SC::unitMass", "1050.0[SI::kg]"))
	if resp.Content == "" {
		t.Error("content empty for a single-document model")
	}
	if got := documentNames(resp); len(got) != 1 || got[0] != "demo.sysml" {
		t.Errorf("documents = %v, want [demo.sysml]", got)
	}
}

// A request not accepting documents reads only content, which a model of several leaves
// empty, so it is refused before anything is edited, whatever document it names.
func TestApplyEditsRefusesAModelOfSeveralUnlessDocumentsAreAccepted(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)

	for _, document := range []string{"", "q.sysml", "missing.sysml"} {
		_, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
			ModelHash: hash, Document: document, Operations: []*pb.EditOperation{renameOp("P::Keep", "Kept")},
		})
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("document %q: err = %v, want FAILED_PRECONDITION", document, err)
		}
		if !strings.Contains(err.Error(), "accept_documents") || !strings.Contains(err.Error(), "2 documents") {
			t.Errorf("document %q: err %q does not name accept_documents and the document count", document, err)
		}
	}

	resp := mustApplied(t, srv, hash, renameOp("P::Keep", "Kept"))
	if strings.Join(documentNames(resp), ",") != "p.sysml,q.sysml" {
		t.Errorf("accepting documents: documents = %v, want p.sysml and q.sysml", documentNames(resp))
	}
}

// A model of one document is edited whether or not the request accepts
// documents, and answers content and documents alike either way.
func TestApplyEditsSingleDocumentIgnoresAcceptDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "demo.sysml", editModelSource)

	var answers []*pb.ApplyEditsResponse
	for _, accept := range []bool{false, true} {
		resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
			ModelHash: hash, AcceptDocuments: accept,
			Operations: []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "1050.0[SI::kg]")},
		})
		if err != nil {
			t.Fatalf("accept_documents=%t: ApplyEdits failed: %v", accept, err)
		}
		if resp.Error != "" || resp.Content == "" || len(resp.Documents) != 1 {
			t.Fatalf("accept_documents=%t: error=%q content=%q documents=%v", accept, resp.Error, resp.Content, documentNames(resp))
		}
		answers = append(answers, resp)
	}
	if answers[0].Content != answers[1].Content || answers[0].Documents[0].Content != answers[1].Documents[0].Content {
		t.Error("a single-document model answered differently with and without accept_documents")
	}
}

// A service withholding edit_documents answers as one predating the fields: a
// model of one document is edited into content alone, with no documents, no
// referrers and no applied document names; a model of several is refused even
// for a request accepting documents.
func TestApplyEditsWithoutEditDocumentsAnswersContentAlone(t *testing.T) {
	srv := mustNewServiceWithout(t, CapabilityEditDocuments)
	ctx := context.Background()

	hash := mustParsedSources(t, srv, "demo.sysml", editModelSource)
	resp, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true,
		Operations: []*pb.EditOperation{setValueOp("Demo::SC::unitMass", "1050.0[SI::kg]")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Error != "" || !strings.Contains(resp.Content, "1050.0[SI::kg]") {
		t.Fatalf("error=%q content=%q, want the edited notation in content", resp.Error, resp.Content)
	}
	if len(resp.Documents) != 0 {
		t.Errorf("documents = %v, want none without the capability", documentNames(resp))
	}
	if len(resp.Applied) != 1 || resp.Applied[0].Document != "" {
		t.Errorf("applied = %v, want one edit naming no document", resp.Applied)
	}

	refused, err := srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: hash, Operations: []*pb.EditOperation{deleteOp("Demo::SC", false)},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if refused.Failure != pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED {
		t.Fatalf("failure = %s (%s), want DELETE_REFERENCED", refused.Failure, refused.Error)
	}
	if len(refused.ReferringElements) == 0 || len(refused.Referrers) != 0 {
		t.Errorf("referring_elements=%v referrers=%v, want the legacy list alone", refused.ReferringElements, refused.Referrers)
	}

	several := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)
	_, err = srv.ApplyEdits(ctx, &pb.ApplyEditsRequest{
		ModelHash: several, AcceptDocuments: true, Operations: []*pb.EditOperation{renameOp("P::Keep", "Kept")},
	})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), CapabilityEditDocuments) {
		t.Errorf("model of several: err = %v, want FAILED_PRECONDITION naming %s", err, CapabilityEditDocuments)
	}
}

// A rename in a model of several documents rewrites the declaration and every
// reference, and the response lists each rewritten document by its parse name,
// the model's first document first and the others in name order. content stays
// empty: it is one document's notation, and the model has several.
func TestApplyEditsRenameCrossesDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "r.sysml", editDocR, "q.sysml", editDocQ)

	resp := mustApplied(t, srv, hash, renameOp("P::Keep", "Kept"))
	if resp.Content != "" {
		t.Errorf("content = %q, want empty for a model of several documents", resp.Content)
	}
	if strings.Join(documentNames(resp), ",") != "p.sysml,q.sysml,r.sysml" {
		t.Errorf("documents = %v, want p.sysml first, then q.sysml and r.sysml", documentNames(resp))
	}
	if got, want := documentContent(t, resp, "p.sysml"), strings.Replace(editDocP, "part def Keep;", "part def Kept;", 1); got != want {
		t.Errorf("p.sysml =\n%s\nwant\n%s", got, want)
	}
	if got, want := documentContent(t, resp, "q.sysml"), strings.Replace(editDocQ, "P::Keep", "P::Kept", 1); got != want {
		t.Errorf("q.sysml =\n%s\nwant\n%s", got, want)
	}
	if got, want := documentContent(t, resp, "r.sysml"), strings.Replace(editDocR, "P::Keep", "P::Kept", 1); got != want {
		t.Errorf("r.sysml =\n%s\nwant\n%s", got, want)
	}
	if len(resp.Applied) != 3 {
		t.Fatalf("applied %d edits, want 3 (declaration and two references)", len(resp.Applied))
	}
	for i, doc := range []string{"p.sysml", "q.sysml", "r.sysml"} {
		if resp.Applied[i].Document != doc {
			t.Errorf("applied[%d].document = %q, want %q", i, resp.Applied[i].Document, doc)
		}
	}
}

// An edit that touches one document of several returns that document alone,
// with content empty: a document the edits left as parsed is not listed. The
// request names the document its operations target.
func TestApplyEditsListsChangedDocumentsOnly(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)

	resp := mustAppliedIn(t, srv, hash, "q.sysml", renameOp("Q::k", "kept"))
	if resp.Content != "" {
		t.Errorf("content = %q, want empty for a model of several documents", resp.Content)
	}
	if got := documentNames(resp); len(got) != 1 || got[0] != "q.sysml" {
		t.Fatalf("documents = %v, want [q.sysml]", got)
	}
	if got, want := resp.Documents[0].Content, strings.Replace(editDocQ, "part k :", "part kept :", 1); got != want {
		t.Errorf("q.sysml =\n%s\nwant\n%s", got, want)
	}
	if len(resp.Applied) != 1 || resp.Applied[0].Document != "q.sysml" {
		t.Errorf("applied = %v, want one edit in q.sysml", resp.Applied)
	}
}

// A cascade delete removes the target and its referrers in every document,
// their referrers in turn included, and returns each rewritten document.
func TestApplyEditsCascadeDeleteCrossesDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ, "r.sysml", editDocR)

	resp := mustApplied(t, srv, hash, deleteOp("P::Base", true))
	if strings.Join(documentNames(resp), ",") != "p.sysml,q.sysml,r.sysml" {
		t.Fatalf("documents = %v, want all three", documentNames(resp))
	}
	if got, want := documentContent(t, resp, "p.sysml"), "package P {\n    part def Keep;\n}\n"; got != want {
		t.Errorf("p.sysml =\n%s\nwant\n%s", got, want)
	}
	if got, want := documentContent(t, resp, "q.sysml"), "package Q {\n    part k : P::Keep;\n}\n"; got != want {
		t.Errorf("q.sysml =\n%s\nwant\n%s", got, want)
	}
	if got, want := documentContent(t, resp, "r.sysml"), "package R {\n    part d : P::Keep;\n}\n"; got != want {
		t.Errorf("r.sysml =\n%s\nwant\n%s", got, want)
	}
}

// A delete without cascade of a declaration referred to from another document
// is refused, naming the referrers with their documents, and returns no notation.
func TestApplyEditsRefusalNamesReferrersInOtherDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Operations: []*pb.EditOperation{deleteOp("P::Base", false)},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED {
		t.Fatalf("failure = %s (%s), want DELETE_REFERENCED", resp.Failure, resp.Error)
	}
	if resp.Content != "" || len(resp.Documents) != 0 || len(resp.Applied) != 0 {
		t.Errorf("a refusal returned notation: content=%q documents=%v applied=%v",
			resp.Content, documentNames(resp), resp.Applied)
	}
	if strings.Join(resp.ReferringElements, ",") != "P::own,Q::b (q.sysml)" {
		t.Errorf("referring_elements = %v, want P::own and Q::b (q.sysml)", resp.ReferringElements)
	}
	want := []*pb.Referrer{{Name: "P::own", Document: "p.sysml"}, {Name: "Q::b", Document: "q.sysml"}}
	if len(resp.Referrers) != len(want) {
		t.Fatalf("referrers = %v, want %v", resp.Referrers, want)
	}
	for i := range want {
		if resp.Referrers[i].Name != want[i].Name || resp.Referrers[i].Document != want[i].Document {
			t.Errorf("referrers[%d] = %v, want %v", i, resp.Referrers[i], want[i])
		}
	}
}

// Referrers are answered in document then name order, referring_elements
// beside them, when the engine found the edited document's own first.
func TestApplyEditsRefusalOrdersReferrersByDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "z.sysml", editDocP, "a.sysml", editDocQ)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Operations: []*pb.EditOperation{deleteOp("P::Base", false)},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED {
		t.Fatalf("failure = %s (%s), want DELETE_REFERENCED", resp.Failure, resp.Error)
	}
	if strings.Join(resp.ReferringElements, ",") != "Q::b (a.sysml),P::own" {
		t.Errorf("referring_elements = %v, want Q::b (a.sysml) then P::own", resp.ReferringElements)
	}
	want := []*pb.Referrer{{Name: "Q::b", Document: "a.sysml"}, {Name: "P::own", Document: "z.sysml"}}
	if len(resp.Referrers) != len(want) {
		t.Fatalf("referrers = %v, want %v", resp.Referrers, want)
	}
	for i := range want {
		if resp.Referrers[i].Name != want[i].Name || resp.Referrers[i].Document != want[i].Document {
			t.Errorf("referrers[%d] = %v, want %v", i, resp.Referrers[i], want[i])
		}
	}
}

// A move respells references in its own document only, so one referred to from
// another document is refused as REFERENCED_ELSEWHERE, naming the referrer.
func TestApplyEditsMoveReferredToFromAnotherDocumentIsRefused(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Operations: []*pb.EditOperation{
			addMemberOp("P", "package", "Inner"),
			moveOp("P::Keep", "P::Inner"),
		},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_REFERENCED_ELSEWHERE {
		t.Fatalf("failure = %s (%s), want REFERENCED_ELSEWHERE", resp.Failure, resp.Error)
	}
	if resp.Content != "" || len(resp.Documents) != 0 || len(resp.Applied) != 0 {
		t.Errorf("a refusal returned notation: content=%q documents=%v applied=%v",
			resp.Content, documentNames(resp), resp.Applied)
	}
	if len(resp.Referrers) != 1 || resp.Referrers[0].Name != "Q::k" || resp.Referrers[0].Document != "q.sysml" {
		t.Errorf("referrers = %v, want Q::k in q.sysml", resp.Referrers)
	}
}

// Operations across documents are one batch: a refused operation leaves every
// document unedited, including one another operation of the batch had rewritten.
func TestApplyEditsAcrossDocumentsIsAtomic(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Operations: []*pb.EditOperation{
			renameOp("P::Keep", "Kept"),
			deleteOp("P::Base", false),
		},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED {
		t.Fatalf("failure = %s (%s), want DELETE_REFERENCED", resp.Failure, resp.Error)
	}
	if resp.Content != "" || len(resp.Documents) != 0 || len(resp.Applied) != 0 {
		t.Errorf("a refused batch returned notation: content=%q documents=%v applied=%v",
			resp.Content, documentNames(resp), resp.Applied)
	}
}

// The rename is judged against every document of the model: one whose names
// would capture the new name in another document is refused, naming the site
// there, and nothing is rewritten anywhere.
func TestApplyEditsRefusesARenameAnotherDocumentWouldCapture(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv,
		"p.sysml", "package P {\n    part def Old;\n}\n",
		"q.sysml", "package Q {\n    private import P::*;\n    part def Fresh;\n    part a : Old;\n}\n",
	)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Operations: []*pb.EditOperation{renameOp("P::Old", "Fresh")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_INVALID_NAME {
		t.Fatalf("failure = %s (%s), want INVALID_NAME", resp.Failure, resp.Error)
	}
	if !strings.Contains(resp.Error, "Fresh") {
		t.Errorf("error %q does not name the conflict", resp.Error)
	}
	if strings.Join(resp.ReferringElements, ",") != "Q (q.sysml)" {
		t.Errorf("referring_elements = %v, want Q (q.sysml)", resp.ReferringElements)
	}
	if len(resp.Referrers) != 1 || resp.Referrers[0].Name != "Q" || resp.Referrers[0].Document != "q.sysml" {
		t.Errorf("referrers = %v, want Q in q.sysml", resp.Referrers)
	}
	if resp.Content != "" || len(resp.Documents) != 0 {
		t.Errorf("a refusal returned notation: content=%q documents=%v", resp.Content, documentNames(resp))
	}
}

// A move refused for a reference it cannot respell names the referring
// declaration of the unedited document in both the legacy and structured fields.
func TestApplyEditsNamesTheReferrerAMoveCannotRespell(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "move.sysml",
		"package P {\n    part def Base;\n    part def H {\n        part b : Base;\n    }\n"+
			"    part def Other;\n    part h : H;\n    part c : Base = h.b;\n}\n")

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Operations: []*pb.EditOperation{moveOp("P::H::b", "P::Other")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_MOVE_REFERENCED {
		t.Fatalf("failure = %s (%s), want MOVE_REFERENCED", resp.Failure, resp.Error)
	}
	if strings.Join(resp.ReferringElements, ",") != "P::c" {
		t.Errorf("referring_elements = %v, want P::c", resp.ReferringElements)
	}
	if len(resp.Referrers) != 1 || resp.Referrers[0].Name != "P::c" || resp.Referrers[0].Document != "move.sysml" {
		t.Errorf("referrers = %v, want P::c in move.sysml", resp.Referrers)
	}
	if resp.Content != "" || len(resp.Documents) != 0 || len(resp.Applied) != 0 {
		t.Errorf("a refusal returned notation: content=%q documents=%v", resp.Content, documentNames(resp))
	}
}

// A request naming a document the model does not have is a call failure, not
// a refused edit, and it lists the documents the model has.
func TestApplyEditsRejectsAnUnknownDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)

	_, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Document: "r.sysml", Operations: []*pb.EditOperation{renameOp("P::Keep", "Kept")},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("err = %v, want INVALID_ARGUMENT", err)
	}
	if !strings.Contains(err.Error(), "p.sysml, q.sysml") {
		t.Errorf("err %q does not list the model's documents", err)
	}
}

// An operation targeting a declaration of a document other than the one the
// request names is refused as an unknown target, naming that document.
func TestApplyEditsRefusesATargetOfAnotherDocument(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv, "p.sysml", editDocP, "q.sysml", editDocQ)

	resp, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Operations: []*pb.EditOperation{renameOp("Q::k", "kept")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if resp.Failure != pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET || !strings.Contains(resp.Error, "q.sysml") {
		t.Errorf("failure = %s (%s), want UNKNOWN_TARGET naming q.sysml", resp.Failure, resp.Error)
	}
	if resp.Content != "" || len(resp.Documents) != 0 {
		t.Errorf("a refusal returned notation: content=%q documents=%v", resp.Content, documentNames(resp))
	}
}

// The edited notation is validated in the whole model: a value naming a
// declaration of another document resolves, so the edit is not refused.
func TestApplyEditsValidatesAgainstTheOtherDocuments(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv,
		"lib.sysml", "package Lib {\n\tattribute basePower = 150;\n\tpart def Engine;\n}\n",
		"top.sysml", "package Top {\n\tprivate import Lib::*;\n\tpart def Car {\n\t\tpart motor : Engine;\n\t\tattribute rating = 1;\n\t}\n}\n",
	)

	resp := mustAppliedIn(t, srv, hash, "top.sysml", setValueOp("Top::Car::rating", "Lib::basePower * 2"))
	if resp.Content != "" {
		t.Errorf("content = %q, want empty for a model of several documents", resp.Content)
	}
	if got := documentNames(resp); len(got) != 1 || got[0] != "top.sysml" {
		t.Fatalf("documents = %v, want [top.sysml]", got)
	}
	if !strings.Contains(resp.Documents[0].Content, "attribute rating = Lib::basePower * 2;") {
		t.Errorf("value not set:\n%s", resp.Documents[0].Content)
	}

	refused, err := srv.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash, AcceptDocuments: true, Document: "top.sysml",
		Operations: []*pb.EditOperation{setValueOp("Top::Car::rating", "Lib::peakPower")},
	})
	if err != nil {
		t.Fatalf("ApplyEdits failed: %v", err)
	}
	if refused.Failure != pb.EditFailure_EDIT_FAILURE_RESULT_INVALID {
		t.Errorf("failure = %s (%s), want RESULT_INVALID for a name the model lacks", refused.Failure, refused.Error)
	}
	if refused.Content != "" || len(refused.Documents) != 0 {
		t.Errorf("a refusal returned notation: content=%q documents=%v", refused.Content, documentNames(refused))
	}
}

// Renaming a package every other document reaches through wildcard imports
// rewrites the imports and leaves the names they surface resolving: the edited
// documents are judged with their rewritten imports, chained re-exports included.
func TestApplyEditsRenamesAWildcardImportedPackage(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParsedSources(t, srv,
		"p.sysml", "package P {\n    package Inner {\n        part def Old;\n    }\n}\n",
		"m.sysml", "package M {\n    public import P::**;\n}\n",
		"q.sysml", "package Q {\n    private import M::*;\n    part a : Old;\n    part b : P::Inner::Old;\n}\n",
	)

	resp := mustApplied(t, srv, hash, renameOp("P", "Fresh"))
	if strings.Join(documentNames(resp), ",") != "p.sysml,m.sysml,q.sysml" {
		t.Fatalf("documents = %v, want p.sysml first, then m.sysml and q.sysml", documentNames(resp))
	}
	if got, want := documentContent(t, resp, "m.sysml"), "package M {\n    public import Fresh::**;\n}\n"; got != want {
		t.Errorf("m.sysml =\n%s\nwant\n%s", got, want)
	}
	if got, want := documentContent(t, resp, "q.sysml"), "package Q {\n    private import M::*;\n    part a : Old;\n    part b : Fresh::Inner::Old;\n}\n"; got != want {
		t.Errorf("q.sysml =\n%s\nwant\n%s", got, want)
	}
}
