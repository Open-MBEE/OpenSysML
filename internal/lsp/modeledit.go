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
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// MethodApplyModelEdit turns diagram actions into a WorkspaceEdit the client
// applies to the document and to every other document the edit reached: the
// server rewrites nothing itself.
const MethodApplyModelEdit = "opensysml/applyModelEdit"

// The operation kinds a modelEditOperation names.
const (
	EditSetValue      = "setValue"
	EditRename        = "rename"
	EditAddMember     = "addMember"
	EditAddConnection = "addConnection"
	EditDelete        = "delete"
	EditMove          = "move"
	EditSetLayout     = "setLayout"
	EditSetRoute      = "setRoute"
	EditSetCanvas     = "setCanvas"
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
//
// The DiagramLayout kinds place what a rendering draws: setLayout writes the
// Layout of the node Target, setRoute the Route of the edge Target, setCanvas
// the Canvas of the view Target. A setLayout or setRoute may give Declaration
// instead of Target, the range a rendering reports for a node or edge no
// qualified name reaches. DeclaredIn names the document declaring the target —
// the document the range is one of, the one Target must be declared in — or is
// empty for the requested one; Digest is the origin's digest of that document's
// text, and a target of another document is refused as stale when that text has
// changed since it was rendered. View names the view whose body states a
// Layout or Route, so it applies in that view alone; left empty, the annotation
// goes inline into Target's declaration and applies in every view. A setLayout
// with no Layout, a setRoute with no or an empty Route and a setCanvas with no
// Canvas clear the annotation.
type modelEditOperation struct {
	Kind         string               `json:"kind"`
	Target       string               `json:"target,omitempty"`
	Declaration  *protocol.Range      `json:"declaration,omitempty"`
	DeclaredIn   protocol.DocumentURI `json:"declaredIn,omitempty"`
	Digest       string               `json:"digest,omitempty"`
	Value        string               `json:"value,omitempty"`
	NewName      string               `json:"newName,omitempty"`
	Owner        string               `json:"owner,omitempty"`
	MemberKind   string               `json:"memberKind,omitempty"`
	Name         string               `json:"name,omitempty"`
	Type         string               `json:"type,omitempty"`
	Multiplicity string               `json:"multiplicity,omitempty"`
	Specializes  []string             `json:"specializes,omitempty"`
	From         string               `json:"from,omitempty"`
	To           string               `json:"to,omitempty"`
	Cascade      bool                 `json:"cascade,omitempty"`
	View         string               `json:"view,omitempty"`
	Layout       *modelEditLayout     `json:"layout,omitempty"`
	Route        []renderPoint        `json:"route,omitempty"`
	Canvas       *renderCanvas        `json:"canvas,omitempty"`
}

// modelEditLayout is a node's geometry as setLayout writes it, in the units
// opensysml/render reports: pixels, y down. Width and Height are written both
// or neither; Collapsed is written only when set.
type modelEditLayout struct {
	X         float64  `json:"x"`
	Y         float64  `json:"y"`
	Width     *float64 `json:"width,omitempty"`
	Height    *float64 `json:"height,omitempty"`
	Collapsed bool     `json:"collapsed,omitempty"`
}

// applyModelEditResult is exactly one of: an edit to apply, the refusals that
// kept the model as it was, or a stale version. Version is the document version
// the answer was made at, which tells a stale client how far behind it was. The
// edit holds one versioned TextDocumentEdit per document it rewrites, the
// requested document first when it is among them; the others carry the versions
// the server holds, null for one it read from disk, so a client refuses to apply
// them to text that has moved on rather than land them wrong.
type applyModelEditResult struct {
	Edit    *protocol.WorkspaceEdit `json:"edit,omitempty"`
	Refused []modelEditRefusal      `json:"refused,omitempty"`
	Stale   bool                    `json:"stale,omitempty"`
	Version int                     `json:"version"`
}

// modelEditRefusal is why an operation was not applied. Operation is its index
// in the request, or -1 when the edited model as a whole was refused. Referring
// names the declarations that refer to a target whose delete or rename was
// refused, qualified by document when that is another; Referrers tells each
// name from its document, so a client can list them by file.
type modelEditRefusal struct {
	Operation   int                   `json:"operation"`
	Failure     string                `json:"failure"`
	Message     string                `json:"message"`
	Diagnostics []protocol.Diagnostic `json:"diagnostics,omitempty"`
	Referring   []string              `json:"referring,omitempty"`
	Referrers   []modelEditReferrer   `json:"referrers,omitempty"`
}

// modelEditReferrer is one referring declaration and the document declaring it.
type modelEditReferrer struct {
	Name string               `json:"name"`
	URI  protocol.DocumentURI `json:"uri"`
}

// editPalette lists the declarations a diagram of one rendering kind offers to
// add, in the document's language; Typed are the Members that take a type.
// Owners lists, for each Member only some bodies offer (`subject`) and for the
// notation of each drawn declaration that is such a member, the nodes whose
// declaration offers it; a kind absent from Owners goes into any node.
type editPalette struct {
	Members     []string            `json:"members"`
	Connections []string            `json:"connections"`
	Typed       []string            `json:"typed"`
	Owners      map[string][]string `json:"owners,omitempty"`
}

// declaredNode is a drawn node the document declares, for admission once every
// confined kind of the rendering is known.
type declaredNode struct {
	id   string
	decl ast.Node
}

// confine lists kind in Owners when only some bodies offer it, so a move of a
// node declared with kind learns where it may go.
func (p *editPalette) confine(kind string) {
	if _, listed := p.Owners[kind]; listed || !modeledit.MemberKindOwnerBound(kind) {
		return
	}
	if p.Owners == nil {
		p.Owners = map[string][]string{}
	}
	p.Owners[kind] = []string{}
}

// admit records that the node with id, declared by decl, may own the members
// only some bodies offer.
func (p *editPalette) admit(id string, decl ast.Node) {
	for kind, ids := range p.Owners {
		if modeledit.MemberKindAdmittedBy(decl, kind) {
			p.Owners[kind] = append(ids, id)
		}
	}
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
// an edit computed against text the client no longer has would land wrong. The
// other documents a rename, a delete or a layout operation reaches are read at
// the versions the server holds, which their edits carry; a document read for a
// target's declaration and left as it was carries its version with no edits, so
// that the client refuses the edit once that document has moved on too.
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
	var read []*model.Document
	for i, op := range params.Operations {
		converted, declaring, err := s.operation(doc, op)
		var stale *model.StaleError
		if errors.As(err, &stale) {
			return &applyModelEditResult{Stale: true, Version: doc.Version}, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: operation %d: %w", jsonrpc2.ErrInvalidParams, i, err)
		}
		ops = append(ops, converted)
		if declaring != nil {
			read = append(read, declaring)
		}
	}
	result, version, ok, err := s.ws.ApplyEdit(name, ops, read)
	if !ok {
		return nil, fmt.Errorf("%s: no such document", name)
	}
	var stale *model.StaleError
	if version != params.Version || errors.As(err, &stale) {
		return &applyModelEditResult{Stale: true, Version: version}, nil
	}
	if err != nil {
		var refusal *modeledit.Error
		if !errors.As(err, &refusal) {
			return nil, err
		}
		return &applyModelEditResult{Refused: []modelEditRefusal{s.refusal(refusal, doc.Content)}, Version: version}, nil
	}
	changes := make([]protocol.TextDocumentEdit, 0, len(result.Documents))
	for _, edited := range result.Documents {
		change, err := documentChange(edited)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return &applyModelEditResult{
		Version: version,
		Edit:    &protocol.WorkspaceEdit{DocumentChanges: changes},
	}, nil
}

// documentChange is the versioned edit turning one document into its rewrite,
// computed from the content the rewrite was made of; no edits when the two are
// the same. A document read from disk has no client version, which the null
// version says.
func documentChange(edited model.DocumentEdit) (protocol.TextDocumentEdit, error) {
	change := protocol.TextDocumentEdit{
		TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: nameToURI(edited.Name)},
		},
		Edits: textEdits(edited.Original, edited.Content),
	}
	if !edited.Open {
		return change, nil
	}
	if edited.Version > math.MaxInt32 {
		return protocol.TextDocumentEdit{}, fmt.Errorf("%s: document version %d exceeds int32", edited.Name, edited.Version)
	}
	v := int32(edited.Version) // #nosec G115 -- bounds checked above; the client sent it as int32.
	change.TextDocument.Version = &v
	return change, nil
}

// operation reads a wire operation of doc as the edit operation it names, its
// target declared in the document DeclaredIn names: a Declaration a range of
// that document's text, a Target a name it declares. Another document than doc
// is read at the text the operation's digest names, and returned as the
// snapshot the target was read in, so that the edit can be pinned to it; a
// digest of other text is a *model.StaleError, since the range may fall on, or
// the name reach, another declaration now.
func (s *Server) operation(doc *model.Document, op modelEditOperation) (modeledit.Operation, *model.Document, error) {
	if op.DeclaredIn == "" {
		converted, err := op.operation(doc.Content)
		return converted, nil, err
	}
	name := uriToName(op.DeclaredIn)
	if name == doc.Name {
		converted, err := op.operation(doc.Content)
		if err != nil {
			return modeledit.Operation{}, nil, err
		}
		return converted.DeclaredIn(name), nil, nil
	}
	if op.Digest == "" {
		return modeledit.Operation{}, nil, fmt.Errorf("a target declared in %s, which declaredIn names, needs the digest of the text it was rendered from", op.DeclaredIn)
	}
	// A library document is never rewritten, so there is no snapshot to pin.
	pinned := s.ws.Document(name)
	declaring := pinned
	if declaring == nil {
		declaring = s.ws.LibraryDocument(name)
	}
	if declaring == nil {
		return modeledit.Operation{}, nil, fmt.Errorf("%s, which declaredIn names, is no document the server holds", op.DeclaredIn)
	}
	if declaring.Digest() != op.Digest {
		return modeledit.Operation{}, nil, &model.StaleError{Name: name}
	}
	converted, err := op.operation(declaring.Content)
	if err != nil {
		return modeledit.Operation{}, nil, err
	}
	return converted.DeclaredIn(name), pinned, nil
}

// operation reads the wire operation as the edit operation it names; content
// is the document a Declaration range is a range of.
func (op modelEditOperation) operation(content []byte) (modeledit.Operation, error) {
	if op.Declaration != nil && op.Kind != EditSetLayout && op.Kind != EditSetRoute {
		return modeledit.Operation{}, fmt.Errorf("a declaration stands in for the target of a %s or %s alone", EditSetLayout, EditSetRoute)
	}
	if op.Declaration != nil && op.Target != "" {
		return modeledit.Operation{}, errors.New("an operation targets its element by name or by declaration, not both")
	}
	if op.DeclaredIn != "" && op.Kind != EditSetLayout && op.Kind != EditSetRoute {
		return modeledit.Operation{}, fmt.Errorf("declaredIn names the document declaring the target of a %s or %s alone", EditSetLayout, EditSetRoute)
	}
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
	case EditMove:
		return modeledit.Move(op.Target, op.Owner), nil
	case EditSetLayout:
		layout, err := op.Layout.layout()
		if err != nil {
			return modeledit.Operation{}, err
		}
		if op.Declaration != nil {
			return modeledit.SetLayoutAt(rangeToSpan(content, *op.Declaration), op.View, layout), nil
		}
		return modeledit.SetLayout(op.Target, op.View, layout), nil
	case EditSetRoute:
		var route *semantics.Route
		if len(op.Route) > 0 {
			route = &semantics.Route{Points: make([]semantics.Waypoint, len(op.Route))}
			for i, p := range op.Route {
				route.Points[i] = semantics.Waypoint{X: p.X, Y: p.Y}
			}
		}
		if op.Declaration != nil {
			return modeledit.SetRouteAt(rangeToSpan(content, *op.Declaration), op.View, route), nil
		}
		return modeledit.SetRoute(op.Target, op.View, route), nil
	case EditSetCanvas:
		canvas, err := op.Canvas.canvas()
		if err != nil {
			return modeledit.Operation{}, err
		}
		return modeledit.SetCanvas(op.Target, canvas), nil
	}
	return modeledit.Operation{}, fmt.Errorf("kind %q is none of %s", op.Kind,
		strings.Join([]string{EditSetValue, EditRename, EditAddMember, EditAddConnection, EditDelete, EditMove, EditSetLayout, EditSetRoute, EditSetCanvas}, ", "))
}

// layout reads the wire geometry as the edit layer writes it; nil clears.
func (l *modelEditLayout) layout() (*semantics.Layout, error) {
	if l == nil {
		return nil, nil
	}
	if (l.Width == nil) != (l.Height == nil) {
		return nil, errors.New("a layout sizes its node with both width and height or with neither")
	}
	out := &semantics.Layout{X: l.X, Y: l.Y, Collapsed: l.Collapsed}
	if l.Width != nil {
		out.Width, out.Height, out.HasSize = *l.Width, *l.Height, true
	}
	return out, nil
}

// canvas reads the wire canvas as the edit layer writes it; nil clears.
func (c *renderCanvas) canvas() (*semantics.Canvas, error) {
	if c == nil {
		return nil, nil
	}
	if (c.Width == nil) != (c.Height == nil) {
		return nil, errors.New("a canvas sizes the drawing surface with both width and height or with neither")
	}
	out := &semantics.Canvas{Unit: c.Unit}
	if c.Width != nil {
		out.Width, out.Height, out.HasSize = *c.Width, *c.Height, true
	}
	return out, nil
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
	for _, r := range e.Referrers {
		out.Referrers = append(out.Referrers, modelEditReferrer{Name: r.Name, URI: nameToURI(r.Document)})
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

// palette is what a diagram of kind offers to add in lang: the member and
// connection kinds it draws. A table draws rows, not nodes, so it offers none.
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
	case view.KindTree:
		p = &editPalette{Members: members, Connections: connections}
	default:
		return nil
	}
	p.Typed = keep(p.Members, modeledit.MemberKindTyped)
	for _, kind := range keep(p.Members, modeledit.MemberKindOwnerBound) {
		if p.Owners == nil {
			p.Owners = map[string][]string{}
		}
		p.Owners[kind] = []string{}
	}
	return p
}

// nodeOwners lists the namespaces declaring sym, nearest first, as an edit names
// them; false when sym or a namespace declaring it is unnamed, so no qualified
// name reaches it.
func nodeOwners(sym *symbols.Symbol) ([]renderOwner, bool) {
	if sym.Name == "" {
		return nil, false
	}
	owners := []renderOwner{}
	for scope := sym.OwnerScope; scope != nil && scope.Owner() != nil; scope = scope.Owner().OwnerScope {
		owner := scope.Owner()
		if owner.Name == "" {
			return nil, false
		}
		owners = append(owners, renderOwner{FQN: notationName(owner), Feature: owner.IsFeature()})
	}
	return owners, true
}

// notationName spells sym's qualified name as the notation does, each name
// quoted on its own, which is how opensysml/applyModelEdit reads a target.
func notationName(sym *symbols.Symbol) string {
	return source.QualifiedNameOf(symbols.NameChain(sym))
}

// nodeSymbol is the declaration a rendering node or edge was built from, named
// or not, in doc, the document declaring it; nil for no document, or for a
// lowering sequenced without a declaration of its own.
func nodeSymbol(doc *model.Document, o view.Origin) *symbols.Symbol {
	if doc == nil || doc.Scope == nil {
		return nil
	}
	return doc.Scope.DeclaredAt(o.Span)
}
