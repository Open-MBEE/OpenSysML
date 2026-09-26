package opensysml

import (
	"context"
	"fmt"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Format is a representation a model is written in or read from.
type Format string

// The formats conversion accepts. There are three canonical ones that are
// written, FormatSysML, FormatTTL and FormatAPIJSON, and a Conversion answers by
// those names whichever alias was asked for. RDF and the API's JSON element form,
// in any spelling, are one experimental mapping, which a Conversion reports; so
// is migration from FormatXMI, which is only ever read.
const (
	FormatSysML Format = "sysml"
	FormatTTL   Format = "ttl"
	// FormatKerML and FormatText are aliases of FormatSysML: notation is one
	// format, read and written by the same parser and printer.
	FormatKerML Format = "kerml"
	FormatText  Format = "text"
	// FormatTurtle and FormatRDF are aliases of FormatTTL, the one RDF
	// serialization written.
	FormatTurtle  Format = "turtle"
	FormatRDF     Format = "rdf"
	FormatAPIJSON Format = "api-json"
	// FormatJSON is an alias of FormatAPIJSON, the OMG API's JSON element
	// form of the same graph FormatTTL writes.
	FormatJSON Format = "json"
	// FormatXMI is SysML v1 as UML XMI, an Eclipse UML2 .uml file or a .mdzip
	// archive, migrated to v2 on the way in. Asking to write it is refused.
	FormatXMI Format = "xmi"
)

// ConvertOption configures Convert and ConvertFile.
type ConvertOption func(*convertOptions)

type convertOptions struct {
	from      Format
	tolerated bool
}

// WithFromFormat names the notation to read, for a file whose extension does
// not say and for inline content, which has no extension. Convert refuses it:
// a parse established what a parsed model was written in.
func WithFromFormat(from Format) ConvertOption {
	return func(o *convertOptions) { o.from = from }
}

// WithTolerateSyntaxErrors writes notation back out even where the parser could
// not read all of it, reporting the syntax errors as diagnostics. Notation to
// notation only: every other direction builds a graph, where unreadable
// declarations would go missing silently.
func WithTolerateSyntaxErrors() ConvertOption {
	return func(o *convertOptions) { o.tolerated = true }
}

// Conversion is a model written in another representation.
type Conversion struct {
	// Content is the converted model.
	Content string
	// From and To are the formats used, so a caller that let the source format be
	// inferred learns what it was inferred as.
	From Format
	To   Format
	// Experimental is set when either format is RDF or the API's JSON form, or the
	// source is SysML v1: mappings that may change without a compatibility path.
	Experimental bool
	// ExperimentalNotice says what is experimental about the conversion, empty
	// when it is not.
	ExperimentalNotice string
	// Diagnostics are the syntax errors tolerated, when any were.
	Diagnostics []Diagnostic
}

func (c *client) Convert(ctx context.Context, model *Model, to Format, opts ...ConvertOption) (*Conversion, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	req := convertRequest(to, opts)
	if req.FromFormat != "" {
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "WithFromFormat does not apply to a parsed model: it is read as the notation the parse read",
		}
	}
	req.Source = &pb.ConvertRequest_ModelHash{ModelHash: hash}
	return c.convert(ctx, req)
}

func (c *client) ConvertFile(ctx context.Context, path string, to Format, opts ...ConvertOption) (*Conversion, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	req := convertRequest(to, opts)
	req.Source = &pb.ConvertRequest_FilePath{FilePath: path}
	return c.convert(ctx, req)
}

func (c *client) ConvertSource(
	ctx context.Context,
	content string,
	to Format,
	opts ...ConvertOption,
) (*Conversion, error) {
	if err := c.live(); err != nil {
		return nil, err
	}
	req := convertRequest(to, opts)
	req.Source = &pb.ConvertRequest_Content{Content: content}
	return c.convert(ctx, req)
}

func convertRequest(to Format, opts []ConvertOption) *pb.ConvertRequest {
	var options convertOptions
	for _, opt := range opts {
		opt(&options)
	}
	return &pb.ConvertRequest{
		FromFormat:           string(options.from),
		ToFormat:             string(to),
		TolerateSyntaxErrors: options.tolerated,
	}
}

func (c *client) convert(ctx context.Context, req *pb.ConvertRequest) (*Conversion, error) {
	resp, err := c.caller.convert(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "Convert", Message: resp.Error, Diagnostics: diagnostics}
	}
	return &Conversion{
		Content:            resp.Content,
		From:               Format(resp.FromFormat),
		To:                 Format(resp.ToFormat),
		Experimental:       resp.Experimental,
		ExperimentalNotice: resp.ExperimentalNotice,
		Diagnostics:        diagnostics,
	}, nil
}

// Edit is one source-preserving change to a model's notation: SetValue, Rename,
// AddMember, AddConnection, Delete or Move. A type switch over them is exhaustive.
type Edit interface {
	isEdit()
}

// SetValue sets the value of a feature that already exists, replacing the
// expression of its `= <expr>` or adding one before the declaration's `;`.
type SetValue struct {
	// Target is the element to edit, named as Symbol.ID names it.
	Target string
	// Value is the new value in SysML notation ("1050.0[SI::kg]", "mass * 2").
	Value string
}

// Rename rewrites a declaration's name token. References are not updated: a
// rename of a referenced element is refused, naming what refers to it.
type Rename struct {
	// Target is the element to rename, named as Symbol.ID names it.
	Target string
	// NewName is the new declared name; it must lex as an identifier.
	NewName string
}

// AddMember inserts a declaration into a namespace or the document root.
type AddMember struct {
	// Owner is the namespace to receive the declaration; empty is the document root.
	Owner string
	// Kind is the written declaration kind, such as "part def" or "class".
	Kind string
	// Name is the declared identifier.
	Name string
	// Type is an optional type target for a usage, written as notation.
	Type string
	// Multiplicity is optional and includes its brackets, such as "[0..*]".
	Multiplicity string
	// Value is an optional value expression, written as notation.
	Value string
	// Specializes are optional specialization targets for a definition.
	Specializes []string
}

// AddConnection inserts a connection-like usage into a namespace or document root.
type AddConnection struct {
	// Owner is the namespace to receive the usage; empty is the document root.
	Owner string
	// Kind is the written connection kind, such as "allocation" or "flow".
	Kind string
	// From is the first feature reference, written as notation.
	From string
	// To is the second feature reference, written as notation.
	To string
	// Name is the optional declared identifier.
	Name string
	// Type is an optional typing target.
	Type string
}

// Delete removes a declaration and the trivia it owns.
type Delete struct {
	// Target is the declaration to remove, by qualified name.
	Target string
	// Cascade also removes the declarations that refer to Target.
	Cascade bool
}

// Move re-parents a declaration into another namespace of the same document,
// carrying its body and owned trivia and respelling the references it breaks.
type Move struct {
	// Target is the declaration to move, by qualified name.
	Target string
	// Owner is the namespace to receive it; empty is the document root.
	Owner string
}

func (SetValue) isEdit()      { /* marker: closed Edit set */ }
func (Rename) isEdit()        { /* marker: closed Edit set */ }
func (AddMember) isEdit()     { /* marker: closed Edit set */ }
func (AddConnection) isEdit() { /* marker: closed Edit set */ }
func (Delete) isEdit()        { /* marker: closed Edit set */ }
func (Move) isEdit()          { /* marker: closed Edit set */ }

// EditFailure says why edits were refused.
type EditFailure int32

// The refusals ApplyEdits reports. Every refusal is one of these: an edit is
// never silently dropped.
const (
	EditFailureUnspecified       EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_UNSPECIFIED)
	EditFailureNoOperations      EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_NO_OPERATIONS)
	EditFailureUnknownTarget     EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_UNKNOWN_TARGET)
	EditFailureAmbiguousTarget   EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_AMBIGUOUS_TARGET)
	EditFailureNotValued         EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_NOT_VALUED)
	EditFailureInvalidValue      EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_INVALID_VALUE)
	EditFailureInvalidName       EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_INVALID_NAME)
	EditFailureNotNamed          EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_NOT_NAMED)
	EditFailureRenameReferenced  EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_RENAME_REFERENCED)
	EditFailureOverlappingEdits  EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_OVERLAPPING_EDITS)
	EditFailureResultInvalid     EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_RESULT_INVALID)
	EditFailureOwnerUnknown      EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_OWNER_UNKNOWN)
	EditFailureOwnerNotNamespace EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_OWNER_NOT_NAMESPACE)
	EditFailureIllegalKind       EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_ILLEGAL_KIND)
	EditFailureMemberNameTaken   EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_MEMBER_NAME_TAKEN)
	EditFailureDeleteReferenced  EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_DELETE_REFERENCED)
	EditFailureOwnerInsideTarget EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_OWNER_INSIDE_TARGET)
	EditFailureMoveReferenced    EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_MOVE_REFERENCED)
	// EditFailureReferencedElsewhere is a rename, delete or move of a declaration
	// referred to from a document the edit cannot rewrite, such as a library.
	EditFailureReferencedElsewhere EditFailure = EditFailure(pb.EditFailure_EDIT_FAILURE_REFERENCED_ELSEWHERE)
)

// String names the refusal as the wire enum spells it.
func (f EditFailure) String() string {
	return pb.EditFailure(f).String()
}

// EditResult is a model's source with every edit applied.
type EditResult struct {
	// Content is the edited notation of a single-document model, byte-identical
	// to the source outside the edited spans. Empty for a model of several
	// documents, whose notation is in Documents alone.
	//
	// Deprecated: read Documents, which carries the same notation for a model of
	// one document and every rewritten document of a model of several, from a
	// service advertising the edit_documents capability.
	Content string
	// Documents is the edited notation of every document the edits rewrote: the
	// edited document first when it is among them, then the others in name order.
	// A single-document model's one document is always listed; a document of
	// several the edits left as parsed is not. Empty, with Content filled, from a
	// service without the edit_documents capability, which edits one document alone.
	Documents []EditedDocument
	// Applied says what each edit changed, grouped by document in the order
	// Documents lists them and in request order within a document.
	Applied []AppliedEdit
	// Diagnostics the edited source was found to have, when any.
	Diagnostics []Diagnostic
}

// EditedDocument is the edited notation of one document of a model.
type EditedDocument struct {
	// Name is the document's name as the parse gave it: a File's path, a
	// Document's Name, or the position of an inline document named neither.
	Name string
	// Content is the edited notation, byte-identical to the source outside the
	// edited spans.
	Content string
}

// AppliedEdit is one byte range of the original source an edit replaced.
type AppliedEdit struct {
	// Index is the edit's position in the request, so an answer maps back to its ask.
	Index int
	// Target is the element edited, as the request named it.
	Target string
	// Document is the document the bytes belong to, named as in EditedDocument.
	Document string
	// Offset and Length are the bytes replaced; Length is zero for an insertion.
	Offset int
	Length int
	// OldText is what was there, empty for an insertion; NewText what was written.
	OldText string
	NewText string
}

// Referrer is one declaration referring to the target of a refused rename,
// delete or move, with the document declaring it.
type Referrer struct {
	// Name is the referring declaration, as the notation names it.
	Name string
	// Document is the document declaring it, named as in EditedDocument.
	Document string
}

func (c *client) ApplyEdits(ctx context.Context, model *Model, edits ...Edit) (*EditResult, error) {
	return c.ApplyDocumentEdits(ctx, model, "", edits...)
}

// requireEditDocuments refuses to send a document name to a service without
// the edit_documents capability, which would ignore it and edit its sole document.
func (c *client) requireEditDocuments(ctx context.Context, document string) error {
	if document == "" {
		return nil
	}
	info, err := c.serverInfo(ctx)
	if err != nil {
		return err
	}
	if !info.Has(CapabilityEditDocuments) {
		return &StatusError{
			Code:    CodeUnimplemented,
			Message: fmt.Sprintf("capability %q is unavailable", CapabilityEditDocuments),
		}
	}
	return nil
}

func (c *client) ApplyDocumentEdits(ctx context.Context, model *Model, document string, edits ...Edit) (*EditResult, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireEditDocuments(ctx, document); err != nil {
		return nil, err
	}
	// This client reads Documents, so a model of several documents may be edited.
	req := &pb.ApplyEditsRequest{ModelHash: hash, Document: document, AcceptDocuments: true}
	for _, edit := range edits {
		operation, err := editToProto(edit)
		if err != nil {
			return nil, err
		}
		req.Operations = append(req.Operations, operation)
	}
	resp, err := c.caller.applyEdits(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &EditError{
			FailureError: FailureError{Op: "ApplyEdits", Message: resp.Error, Diagnostics: diagnostics},
			Failure:      EditFailure(resp.Failure),
			Referring:    append([]string(nil), resp.ReferringElements...),
			Referrers:    referrersFromProto(resp.Referrers),
		}
	}
	result := &EditResult{Content: resp.Content, Diagnostics: diagnostics}
	for _, doc := range resp.Documents {
		result.Documents = append(result.Documents, EditedDocument{Name: doc.Name, Content: doc.Content})
	}
	for _, applied := range resp.Applied {
		result.Applied = append(result.Applied, AppliedEdit{
			Index:    int(applied.OperationIndex),
			Target:   applied.Target,
			Document: applied.Document,
			Offset:   int(applied.Offset),
			Length:   int(applied.Length),
			OldText:  applied.OldText,
			NewText:  applied.NewText,
		})
	}
	return result, nil
}

func referrersFromProto(referrers []*pb.Referrer) []Referrer {
	if len(referrers) == 0 {
		return nil
	}
	out := make([]Referrer, 0, len(referrers))
	for _, r := range referrers {
		out = append(out, Referrer{Name: r.Name, Document: r.Document})
	}
	return out
}

func editToProto(edit Edit) (*pb.EditOperation, error) {
	switch operation := edit.(type) {
	case SetValue:
		return &pb.EditOperation{Operation: &pb.EditOperation_SetValue{SetValue: &pb.SetValueEdit{
			Target: operation.Target,
			Value:  operation.Value,
		}}}, nil
	case Rename:
		return &pb.EditOperation{Operation: &pb.EditOperation_Rename{Rename: &pb.RenameEdit{
			Target:  operation.Target,
			NewName: operation.NewName,
		}}}, nil
	case AddMember:
		return &pb.EditOperation{Operation: &pb.EditOperation_AddMember{AddMember: &pb.AddMemberEdit{
			Owner:        operation.Owner,
			Kind:         operation.Kind,
			Name:         operation.Name,
			Type:         operation.Type,
			Multiplicity: operation.Multiplicity,
			Value:        operation.Value,
			Specializes:  append([]string(nil), operation.Specializes...),
		}}}, nil
	case AddConnection:
		return &pb.EditOperation{Operation: &pb.EditOperation_AddConnection{
			AddConnection: &pb.AddConnectionEdit{
				Owner: operation.Owner, Kind: operation.Kind, FromEnd: operation.From,
				ToEnd: operation.To, Name: operation.Name, Type: operation.Type,
			},
		}}, nil
	case Delete:
		return &pb.EditOperation{Operation: &pb.EditOperation_Delete{Delete: &pb.DeleteEdit{
			Target:  operation.Target,
			Cascade: operation.Cascade,
		}}}, nil
	case Move:
		return &pb.EditOperation{Operation: &pb.EditOperation_Move{Move: &pb.MoveEdit{
			Target: operation.Target,
			Owner:  operation.Owner,
		}}}, nil
	default:
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "unknown edit kind"}
	}
}
