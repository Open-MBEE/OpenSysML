package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	modeledit "github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// MethodApplyModelEdit turns diagram actions into a WorkspaceEdit the client
// applies to the document: the server rewrites nothing itself.
const MethodApplyModelEdit = "opensysml/applyModelEdit"

// The operation kinds a modelEditOperation names.
const (
	EditSetValue      = "setValue"
	EditRename        = "rename"
	EditAddMember     = "addMember"
	EditAddConnection = "addConnection"
	EditDelete        = "delete"
)

// applyModelEditParams asks for the operations to be applied to the document as
// the client holds it, at the version the client last sent.
type applyModelEditParams struct {
	TextDocument protocol.TextDocumentIdentifier `json:"textDocument"`
	Version      int                             `json:"version"`
	Operations   []modelEditOperation            `json:"operations"`
}

// modelEditOperation is one modeledit.Operation on the wire. Kind selects the
// operation; the other fields are read as that operation reads them. Elements
// are named by qualified name, as a rendering's nodes report them.
type modelEditOperation struct {
	Kind         string   `json:"kind"`
	Target       string   `json:"target,omitempty"`
	Value        string   `json:"value,omitempty"`
	NewName      string   `json:"newName,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	MemberKind   string   `json:"memberKind,omitempty"`
	Name         string   `json:"name,omitempty"`
	Type         string   `json:"type,omitempty"`
	Multiplicity string   `json:"multiplicity,omitempty"`
	Specializes  []string `json:"specializes,omitempty"`
	From         string   `json:"from,omitempty"`
	To           string   `json:"to,omitempty"`
	Cascade      bool     `json:"cascade,omitempty"`
}

// applyModelEditResult is exactly one of: an edit to apply, the refusals that
// kept the model as it was, or a stale version. Version is the document version
// the answer was made at, which tells a stale client how far behind it was.
type applyModelEditResult struct {
	Edit    *protocol.WorkspaceEdit `json:"edit,omitempty"`
	Refused []modelEditRefusal      `json:"refused,omitempty"`
	Stale   bool                    `json:"stale,omitempty"`
	Version int                     `json:"version"`
}

// modelEditRefusal is why an operation was not applied. Operation is its index
// in the request, or -1 when the edited model as a whole was refused.
type modelEditRefusal struct {
	Operation   int                   `json:"operation"`
	Failure     string                `json:"failure"`
	Message     string                `json:"message"`
	Diagnostics []protocol.Diagnostic `json:"diagnostics,omitempty"`
	Referring   []string              `json:"referring,omitempty"`
}

// editPalette lists the declarations a diagram of one rendering kind offers to
// add, in the document's language; Typed are the Members that take a type.
type editPalette struct {
	Members     []string `json:"members"`
	Connections []string `json:"connections"`
	Typed       []string `json:"typed"`
}

// modelEditHandler dispatches opensysml/applyModelEdit and passes everything
// else on.
func (s *Server) modelEditHandler(inner jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		if req.Method() != MethodApplyModelEdit {
			return inner(ctx, reply, req)
		}
		var params applyModelEditParams
		if err := json.Unmarshal(req.Params(), &params); err != nil {
			return reply(ctx, nil, fmt.Errorf("%s: %w", jsonrpc2.ErrParse, err))
		}
		result, err := s.ApplyModelEdit(&params)
		if err != nil {
			return reply(ctx, nil, err)
		}
		return reply(ctx, result, nil)
	}
}

// ApplyModelEdit answers opensysml/applyModelEdit: the edits that make the
// document say what the operations ask, or why it cannot. The document is read
// at the version the client named; any other version is reported stale, since
// an edit computed against text the client no longer has would land wrong.
func (s *Server) ApplyModelEdit(params *applyModelEditParams) (*applyModelEditResult, error) {
	name := uriToName(params.TextDocument.URI)
	doc := s.ws.Document(name)
	if doc == nil {
		return nil, fmt.Errorf("%s: no such document", name)
	}
	if doc.Version != params.Version {
		return &applyModelEditResult{Stale: true, Version: doc.Version}, nil
	}
	ops := make([]modeledit.Operation, 0, len(params.Operations))
	for i, op := range params.Operations {
		converted, err := op.operation()
		if err != nil {
			return nil, fmt.Errorf("%s: operation %d: %w", jsonrpc2.ErrInvalidParams, i, err)
		}
		ops = append(ops, converted)
	}
	result, version, ok, err := s.ws.ApplyEdit(name, ops)
	if !ok {
		return nil, fmt.Errorf("%s: no such document", name)
	}
	if version != params.Version {
		return &applyModelEditResult{Stale: true, Version: version}, nil
	}
	if err != nil {
		var refusal *modeledit.Error
		if !errors.As(err, &refusal) {
			return nil, err
		}
		return &applyModelEditResult{Refused: []modelEditRefusal{s.refusal(refusal, doc.Content)}, Version: version}, nil
	}
	if version > math.MaxInt32 {
		return nil, fmt.Errorf("%s: document version %d exceeds int32", name, version)
	}
	v := int32(version) // #nosec G115 -- bounds checked above; the client sent it as int32.
	return &applyModelEditResult{
		Version: version,
		Edit: &protocol.WorkspaceEdit{
			DocumentChanges: []protocol.TextDocumentEdit{{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					TextDocumentIdentifier: params.TextDocument,
					Version:                &v,
				},
				Edits: textEdits(doc.Content, result.Content),
			}},
		},
	}, nil
}

// operation reads the wire operation as the edit operation it names.
func (op modelEditOperation) operation() (modeledit.Operation, error) {
	switch op.Kind {
	case EditSetValue:
		return modeledit.SetValue(op.Target, op.Value), nil
	case EditRename:
		return modeledit.Rename(op.Target, op.NewName), nil
	case EditAddMember:
		out := modeledit.AddMember(op.Owner, op.MemberKind, op.Name)
		out.Type, out.Multiplicity, out.Value, out.Specializes = op.Type, op.Multiplicity, op.Value, op.Specializes
		return out, nil
	case EditAddConnection:
		out := modeledit.AddConnection(op.Owner, op.MemberKind, op.From, op.To, op.Name)
		out.Type = op.Type
		return out, nil
	case EditDelete:
		return modeledit.Delete(op.Target, op.Cascade), nil
	}
	return modeledit.Operation{}, fmt.Errorf("kind %q is none of %s", op.Kind,
		strings.Join([]string{EditSetValue, EditRename, EditAddMember, EditAddConnection, EditDelete}, ", "))
}

// refusal reports an edit refusal to the client. Diagnostics of the edited
// notation are located in it, not in the document the client holds, so they are
// carried as messages with their ranges in the text that was refused.
func (s *Server) refusal(e *modeledit.Error, content []byte) modelEditRefusal {
	out := modelEditRefusal{
		Operation: e.OperationIndex,
		Failure:   e.Failure.String(),
		Message:   e.Message,
		Referring: e.Referring,
	}
	diagnosed := content
	if e.Diagnosed != nil {
		diagnosed = e.Diagnosed.Bytes()
	}
	for _, d := range e.Diagnostics {
		out.Diagnostics = append(out.Diagnostics, protocol.Diagnostic{
			Range:    spanToRange(diagnosed, d.Span),
			Severity: protocol.DiagnosticSeverity(int(d.Severity) + 1),
			Message:  d.Message,
			Code:     d.Code,
			Source:   d.Source,
		})
	}
	return out
}

// textEdits returns the edits turning content into edited, one per run of
// changed lines, so an insertion into a body is one edit at the body's end and
// a rename is one edit per line it touched.
func textEdits(content, edited []byte) []protocol.TextEdit {
	oldLines, oldOffsets := cutLines(content)
	newLines, _ := cutLines(edited)
	table := newLineTable(content, oldOffsets)
	ids := map[string]int{}
	intern := func(lines []string) []int {
		out := make([]int, len(lines))
		for i, line := range lines {
			id, ok := ids[line]
			if !ok {
				id = len(ids)
				ids[line] = id
			}
			out[i] = id
		}
		return out
	}
	edits := []protocol.TextEdit{}
	for _, h := range diffLines(intern(oldLines), intern(newLines)) {
		start, end := oldOffsets[h.oldStart], oldOffsets[h.oldEnd]
		text := strings.Join(newLines[h.newStart:h.newEnd], "")
		if h.oldEnd-h.oldStart == 1 && h.newEnd-h.newStart == 1 {
			from, to, replacement, _ := trimCommon(oldLines[h.oldStart], newLines[h.newStart])
			start, end, text = start+from, start+to, replacement
		}
		edits = append(edits, protocol.TextEdit{
			Range:   protocol.Range{Start: table.position(start), End: table.position(end)},
			NewText: text,
		})
	}
	return edits
}

// palette is what a diagram of kind offers to add in lang: the member kinds a
// rendering of that kind draws, and the connection kinds it draws as edges.
func palette(kind view.Kind, lang source.Kind) *editPalette {
	members, connections := modeledit.MemberKinds(lang), modeledit.ConnectionKinds(lang)
	keep := func(list []string, want func(string) bool) []string {
		out := []string{}
		for _, item := range list {
			if want(item) {
				out = append(out, item)
			}
		}
		return out
	}
	anyOf := func(names ...string) func(string) bool {
		return func(item string) bool {
			for _, name := range names {
				if item == name {
					return true
				}
			}
			return false
		}
	}
	var p *editPalette
	switch kind {
	case view.KindInterconnection:
		p = &editPalette{
			Members:     keep(members, anyOf("part", "port", "item", "attribute", "feature")),
			Connections: keep(connections, anyOf("connection", "interface", "flow", "binding", "allocation", "connector")),
		}
	case view.KindState:
		p = &editPalette{
			Members:     keep(members, anyOf("state")),
			Connections: keep(connections, anyOf("transition", "succession")),
		}
	case view.KindAction, view.KindSequence:
		p = &editPalette{
			Members:     keep(members, anyOf("action", "fork", "join", "merge", "decide", "step", "item")),
			Connections: keep(connections, anyOf("succession", "flow")),
		}
	case view.KindTree, view.KindTable:
		p = &editPalette{Members: members, Connections: connections}
	default:
		return nil
	}
	p.Typed = keep(p.Members, modeledit.MemberKindTyped)
	return p
}

// nodeFQN names the declaration a rendering node was built from, as an edit
// targets it, or "" for a node with no named declaration in the document.
func nodeFQN(scope *symbols.Scope, o view.Origin) string {
	if scope == nil || !o.Located() {
		return ""
	}
	sym := symbolAtOffset(scope, o.Span.Offset)
	if sym == nil || sym.Name == "" || sym.DeclSpan.Offset != o.Span.Offset {
		return ""
	}
	return symbols.FQNOf(sym)
}
