// Package edit rewrites the source a model was parsed from. An operation names
// an element the way symbols name it, the spans of the parse say which bytes
// carry that element's value or name, and only those bytes are replaced — so
// comments, blank lines and indentation come back byte-identical. The edited
// notation is re-parsed and re-analyzed before it is returned: an edit that
// would make the model unreadable is refused rather than written.
package edit

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// OpKind is which source-preserving change an Operation makes.
type OpKind int

const (
	// OpSetValue sets the value of a feature that already exists.
	OpSetValue OpKind = iota
	// OpRename rewrites the name token of a declaration and the references to it.
	OpRename
	// OpAddMember inserts a declaration into a namespace.
	OpAddMember
	// OpDelete removes a declaration and its owned trivia.
	OpDelete
	// OpAddConnection inserts a connector-like usage joining two features.
	OpAddConnection
	// OpMove re-parents a declaration: the span OpDelete removes is written
	// where OpAddMember inserts, and the references it breaks are respelled.
	OpMove
	// OpSetLayout writes, updates or clears a DiagramLayout annotation.
	OpSetLayout
)

// Operation is one change to make to a model's source.
type Operation struct {
	Kind OpKind
	// Target is the element to edit, by FQN, as symbols name it. An OpSetLayout
	// may give Declaration instead, the span the element is declared at, for an
	// element no qualified name reaches: an unnamed one, or one declared inside
	// an unnamed one. DeclarationDoc names the document the element is declared
	// in — the one the span is of, or the one Target must resolve to a declaration
	// of; empty means the edited document for a span and any for a Target.
	Target         string
	Declaration    source.Span
	DeclarationDoc string
	// Value is the new value in SysML notation, for OpSetValue.
	Value string
	// NewName is the new declared name, for OpRename.
	NewName string
	// Owner is the namespace receiving an OpAddMember or OpAddConnection; empty
	// means the root.
	Owner string
	// Declaration details for OpAddMember and OpAddConnection. MemberName is
	// optional for a connection, which the notation lets be anonymous.
	MemberKind   string
	MemberName   string
	Type         string
	Multiplicity string
	Specializes  []string
	// From and To are the ends of an OpAddConnection, written as the notation
	// references features (`a.p`, `A::b`).
	From    string
	To      string
	Cascade bool
	// NewOwner is the namespace an OpMove moves Target into; empty means the root.
	NewOwner string
	// Annotation is the DiagramLayout metadata an OpSetLayout writes, by FQN
	// (semantics.LayoutFQN, RouteFQN or CanvasFQN). View names the view whose
	// body states it about Target; empty, the annotation is inline on Target
	// and applies in every view.
	Annotation string
	View       string
	// The geometry to write, the one Annotation names; all nil clears the
	// annotation.
	Layout *semantics.Layout
	Route  *semantics.Route
	Canvas *semantics.Canvas
}

// SetValue is an operation setting target's value to the expression value.
func SetValue(target, value string) Operation {
	return Operation{Kind: OpSetValue, Target: target, Value: value}
}

// Rename is an operation rewriting target's declared name to newName.
func Rename(target, newName string) Operation {
	return Operation{Kind: OpRename, Target: target, NewName: newName}
}

// AddMember creates an operation inserting a declaration into owner.
func AddMember(owner, kind, name string) Operation {
	return Operation{Kind: OpAddMember, Owner: owner, MemberKind: kind, MemberName: name}
}

// Delete creates an operation removing target, optionally including referrers.
func Delete(target string, cascade bool) Operation {
	return Operation{Kind: OpDelete, Target: target, Cascade: cascade}
}

// AddConnection creates an operation inserting into owner a connector-like
// usage of kind (`connection`, `flow`, `succession`, …) joining from to to.
// name may be empty for an anonymous connection.
func AddConnection(owner, kind, from, to, name string) Operation {
	return Operation{Kind: OpAddConnection, Owner: owner, MemberKind: kind, From: from, To: to, MemberName: name}
}

// Move is an operation making target a member of newOwner, "" for the root.
func Move(target, newOwner string) Operation {
	return Operation{Kind: OpMove, Target: target, NewOwner: newOwner}
}

// SetLayout is an operation placing target in view — inline on target when
// view is empty — or clearing its position when layout is nil.
func SetLayout(target, view string, layout *semantics.Layout) Operation {
	return Operation{Kind: OpSetLayout, Target: target, View: view, Annotation: semantics.LayoutFQN, Layout: layout}
}

// SetLayoutAt is SetLayout of the element declared at decl in the document,
// which is how an element no qualified name reaches is placed.
func SetLayoutAt(decl source.Span, view string, layout *semantics.Layout) Operation {
	return Operation{Kind: OpSetLayout, Declaration: decl, View: view, Annotation: semantics.LayoutFQN, Layout: layout}
}

// DeclaredIn is op with its element declared in the named document: its
// Declaration a span of that document's text, its Target a name declared there.
func (op Operation) DeclaredIn(doc string) Operation {
	op.DeclarationDoc = doc
	return op
}

// SetRoute is an operation steering the edge target through route's waypoints in
// view — inline on target when view is empty — or clearing them when route is nil.
func SetRoute(target, view string, route *semantics.Route) Operation {
	return Operation{Kind: OpSetLayout, Target: target, View: view, Annotation: semantics.RouteFQN, Route: route}
}

// SetRouteAt is SetRoute of the edge declared at decl in the document, which is
// how an unnamed transition or succession is steered.
func SetRouteAt(decl source.Span, view string, route *semantics.Route) Operation {
	return Operation{Kind: OpSetLayout, Declaration: decl, View: view, Annotation: semantics.RouteFQN, Route: route}
}

// SetCanvas is an operation sizing the drawing surface of view, or clearing its
// size when canvas is nil.
func SetCanvas(view string, canvas *semantics.Canvas) Operation {
	return Operation{Kind: OpSetLayout, Target: view, Annotation: semantics.CanvasFQN, Canvas: canvas}
}

// Model is a parsed model to edit: the source that was read, its parse, and the
// index it was analyzed in.
type Model struct {
	Source *source.SourceFile
	Root   *ast.RootNamespace
	Index  *symbols.Index
	// ParseDiags and SemDiags are what the original was found to have, so a
	// refusal reports the errors an edit introduced and not ones it inherited.
	ParseDiags []parser.Diagnostic
	SemDiags   []passes.Diagnostic
	// NewIndex hands out an index carrying the libraries the model was analyzed
	// against and every other document of Index, but none under Source's name,
	// for analyzing the edited notation. Nil checks syntax alone.
	NewIndex func() *symbols.Index
	// Analysis is the options the edited notation is judged under, the same the
	// original's SemDiags came from; the zero value is the default mode.
	Analysis passes.Options
	// Other hands out another document of Index whose notation the edit may
	// rewrite when a rename or delete reaches a reference in it, or false for one
	// it may not, which the edit then refuses to follow. Nil rewrites Source alone.
	Other func(name string) (Document, bool)
	// reindex is the one index an Apply call analyzes in, set by Apply.
	reindex *reindexer
}

// Document is the source of another document of a Model's index, as Index was
// built from it, with what its original was found to have.
type Document struct {
	Source     *source.SourceFile
	ParseDiags []parser.Diagnostic
	SemDiags   []passes.Diagnostic
}

// inDocument is m read as the document named name: m itself for the edited one,
// else the other document as the edit may rewrite it, or false for one it may not.
func (m Model) inDocument(name string) (Model, bool) {
	if name == m.Source.Name() {
		return m, true
	}
	if m.Other == nil {
		return Model{}, false
	}
	doc, ok := m.Other(name)
	if !ok || doc.Source == nil {
		return Model{}, false
	}
	root, _, ok := m.documentRoot(name)
	if !ok {
		return Model{}, false
	}
	return Model{
		Source: doc.Source, Root: root, Index: m.Index,
		ParseDiags: doc.ParseDiags, SemDiags: doc.SemDiags,
		NewIndex: m.NewIndex, Analysis: m.Analysis, Other: m.Other, reindex: m.reindex,
	}, true
}

// reindexer holds the one index an Apply call reads its intermediate and final
// notation in: adding a document the index already holds drops the previous
// contributions first, so reuse leaves what a fresh build would.
type reindexer struct {
	newIndex func() *symbols.Index
	idx      *symbols.Index
}

// analyzedIn returns the index holding root as the document named name,
// building the call's index on first use.
func (r *reindexer) analyzedIn(name string, root *ast.RootNamespace, kind source.Kind) *symbols.Index {
	if r.idx == nil {
		if r.newIndex != nil {
			r.idx = r.newIndex()
		} else {
			r.idx = symbols.NewIndex()
		}
	}
	r.idx.AddDocumentWithKind(name, root, kind)
	return r.idx
}

// Applied describes one replacement. Batch spans use the original source;
// sequential spans use the intermediate source seen by that operation.
type Applied struct {
	OperationIndex int
	Target         string
	// Span is the range replaced; Len is 0 for an insertion.
	Span    source.Span
	OldText string
	NewText string
}

// Result is the edited notation and what each operation changed. Content and
// Applied are the edited document's; Others are the other documents a rename or
// delete followed a reference into, in name order, and none when it reached none.
type Result struct {
	Content []byte
	Applied []Applied
	Others  []DocumentResult
}

// DocumentResult is the edited notation of one other document and what changed in it.
type DocumentResult struct {
	Name    string
	Content []byte
	Applied []Applied
}

// rewrite is one document's notation as the operations so far have left it.
type rewrite struct {
	content []byte
	applied []Applied
}

// rewrites is every document the operations have rewritten, by name.
type rewrites map[string]*rewrite

// result presents the rewrites as the edited document's, named own, then the others.
func (rw rewrites) result(own string) *Result {
	out := &Result{}
	for _, name := range rw.names(own) {
		r := rw[name]
		if name == own {
			out.Content, out.Applied = r.content, r.applied
			continue
		}
		out.Others = append(out.Others, DocumentResult{Name: name, Content: r.content, Applied: r.applied})
	}
	return out
}

// names lists the rewritten documents, own first and the rest in name order.
func (rw rewrites) names(own string) []string {
	out := make([]string, 0, len(rw))
	for name := range rw {
		if name != own {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return append([]string{own}, out...)
}

// unedited is the rewrites before any operation: the edited document as read.
func unedited(m Model) rewrites {
	return rewrites{m.Source.Name(): &rewrite{content: append([]byte(nil), m.Source.Bytes()...)}}
}

// Apply applies every operation to m's source, or none of them, and returns the
// edited notation. Every refusal is an *Error naming its kind.
func Apply(m Model, ops []Operation) (*Result, error) {
	if m.Source == nil || m.Root == nil || m.Index == nil {
		return nil, &Error{Failure: FailureResultInvalid, Message: "no parsed model to edit"}
	}
	if len(ops) == 0 {
		return nil, &Error{Failure: FailureNoOperations, Message: "no edit operations requested"}
	}
	m.reindex = &reindexer{newIndex: m.NewIndex}
	if !needsSequential(ops) {
		return applyBatch(m, ops)
	}

	current := m
	edited := unedited(m)
	ops = append([]Operation(nil), ops...)
	for i, op := range ops {
		splices, err := current.splicesFor(i, op)
		if err != nil {
			return nil, err
		}
		if err := current.rewrite(edited, splices); err != nil {
			return nil, err
		}
		if err := current.rebaseDeclarations(ops[i+1:], i+1, splices); err != nil {
			return nil, err
		}
		current = reparseModel(m, edited)
		if err := current.relocateDeclarations(ops[i+1:], i+1); err != nil {
			return nil, err
		}
	}
	if err := m.validate(edited); err != nil {
		return nil, err
	}
	return edited.result(m.Source.Name()), nil
}

// needsSequential reports whether one operation may see another's work: a name
// an earlier one declares, renames or removes, or bytes it already rewrote.
// Values sit apart from one another and move no name, so only a request of
// nothing but set-value is proven independent.
func needsSequential(ops []Operation) bool {
	if len(ops) == 1 {
		return false
	}
	for _, op := range ops {
		if op.Kind != OpSetValue {
			return true
		}
	}
	return false
}

func applyBatch(m Model, ops []Operation) (*Result, error) {
	splices := make([]splice, 0, len(ops))
	for i, op := range ops {
		next, err := m.splicesFor(i, op)
		if err != nil {
			return nil, err
		}
		splices = append(splices, next...)
	}
	edited := unedited(m)
	if err := m.rewrite(edited, splices); err != nil {
		return nil, err
	}
	if err := m.validate(edited); err != nil {
		return nil, err
	}
	return edited.result(m.Source.Name()), nil
}

// rewrite applies splices to the documents they address, the edited one and the
// others, adding each result to out; overlapping splices in one document refuse.
func (m Model) rewrite(out rewrites, splices []splice) error {
	for _, name := range spliceDocuments(m.Source.Name(), splices) {
		doc, ok := m.inDocument(name)
		if !ok {
			return &Error{Failure: FailureReferencedElsewhere, OperationIndex: -1,
				Message: "the edit reaches " + name + ", which it cannot rewrite"}
		}
		var own []splice
		for _, sp := range splices {
			if sp.document(m.Source.Name()) == name {
				own = append(own, sp)
			}
		}
		if err := checkOverlap(own); err != nil {
			return err
		}
		r := out[name]
		if r == nil {
			r = &rewrite{}
			out[name] = r
		}
		r.content = doc.splice(own)
		for _, sp := range own {
			r.applied = append(r.applied, Applied{
				OperationIndex: sp.opIndex,
				Target:         sp.target,
				Span:           sp.span,
				OldText:        doc.Source.Text(sp.span),
				NewText:        sp.text,
			})
		}
	}
	return nil
}

// spliceDocuments names the documents splices address, own first and the rest in
// name order.
func spliceDocuments(own string, splices []splice) []string {
	seen := map[string]bool{}
	var others []string
	for _, sp := range splices {
		name := sp.document(own)
		if !seen[name] {
			seen[name] = true
			if name != own {
				others = append(others, name)
			}
		}
	}
	sort.Strings(others)
	if seen[own] {
		return append([]string{own}, others...)
	}
	return others
}

// reparseModel is the state the later operations locate their targets in: every
// rewritten document parsed and indexed, and not analyzed — an edit is judged by
// the original's diagnostics and the returned notation's, which validate takes.
func reparseModel(base Model, edited rewrites) Model {
	others := map[string]Document{}
	for _, name := range edited.names(base.Source.Name())[1:] {
		original, _ := base.Other(name)
		sf := source.NewWithKind(name, edited[name].content, original.Source.Kind())
		p := parser.New(sf)
		base.reindex.analyzedIn(name, p.ParseFile(), sf.Kind())
		others[name] = Document{Source: sf, ParseDiags: p.Diagnostics}
	}
	sf := source.NewWithKind(base.Source.Name(), edited[base.Source.Name()].content, base.Source.Kind())
	p := parser.New(sf)
	root := p.ParseFile()
	idx := base.reindex.analyzedIn(sf.Name(), root, sf.Kind())
	other := base.Other
	if len(others) > 0 {
		other = func(name string) (Document, bool) {
			if doc, ok := others[name]; ok {
				return doc, true
			}
			return base.Other(name)
		}
	}
	return Model{
		Source: sf, Root: root, Index: idx,
		ParseDiags: p.Diagnostics,
		NewIndex:   base.NewIndex, Analysis: base.Analysis, Other: other, reindex: base.reindex,
	}
}

// splice is one byte range of a document's source to replace with text.
type splice struct {
	span    source.Span
	text    string
	opIndex int
	target  string
	// doc names the other document the span is in; empty for the edited one.
	doc string
}

// document names the document the splice rewrites, own being the edited one.
func (sp splice) document(own string) string {
	if sp.doc == "" {
		return own
	}
	return sp.doc
}

// splicesFor turns one operation into the byte ranges it rewrites. A rename, a
// cascading delete and a move reach more than one span; every other operation
// reaches one.
func (m Model) splicesFor(i int, op Operation) ([]splice, error) {
	if op.Kind == OpMove {
		return m.moveSplices(i, op)
	}
	if op.Kind == OpAddMember {
		sp, err := m.addMemberSplice(i, op)
		if err != nil {
			return nil, err
		}
		return []splice{sp}, nil
	}
	if op.Kind == OpAddConnection {
		sp, err := m.addConnectionSplice(i, op)
		if err != nil {
			return nil, err
		}
		return []splice{sp}, nil
	}
	if op.Kind == OpSetLayout {
		return m.layoutSplices(i, op)
	}
	if op.Declaration.Len > 0 {
		return nil, &Error{Failure: FailureInvalidValue, OperationIndex: i,
			Message: "only a layout operation reaches its element by declaration; name the target"}
	}
	if op.Kind == OpDelete {
		deletes, err := m.deleteSplices(i, op)
		if err != nil {
			return nil, err
		}
		if len(deletes) == 0 {
			return nil, &Error{Failure: FailureResultInvalid, OperationIndex: i,
				Message: "delete selected no declaration"}
		}
		return deletes, nil
	}
	sym, err := m.target(i, op)
	if err != nil {
		return nil, err
	}
	switch op.Kind {
	case OpSetValue:
		sp, err := m.valueSplice(i, op, sym)
		if err != nil {
			return nil, err
		}
		return []splice{sp}, nil
	case OpRename:
		return m.renameSplices(i, op, sym)
	default:
		return nil, &Error{
			Failure:        FailureResultInvalid,
			OperationIndex: i,
			Message:        fmt.Sprintf("unknown edit operation kind %d", op.Kind),
		}
	}
}

// splice rewrites the source, applying the ranges right-to-left so that each
// offset still names the bytes the parse of the original found there.
func (m Model) splice(splices []splice) []byte {
	ordered := make([]splice, len(splices))
	copy(ordered, splices)
	sort.SliceStable(ordered, func(a, b int) bool {
		if ordered[a].span.Offset != ordered[b].span.Offset {
			return ordered[a].span.Offset > ordered[b].span.Offset
		}
		// At one insertion point, apply later requests first so the result
		// retains request order.
		return ordered[a].opIndex > ordered[b].opIndex
	})

	src := m.Source.Bytes()
	out := make([]byte, len(src))
	copy(out, src)
	for _, sp := range ordered {
		edited := make([]byte, 0, len(out)-sp.span.Len+len(sp.text))
		edited = append(edited, out[:sp.span.Offset]...)
		edited = append(edited, sp.text...)
		edited = append(edited, out[sp.span.End():]...)
		out = edited
	}
	return out
}

// rebaseDeclarations moves the start of each later operation's Declaration,
// numbered from first, past the bytes splices insert or remove before it. A
// declaration a replacement covers is gone, so the operation is refused.
func (m Model) rebaseDeclarations(later []Operation, first int, splices []splice) error {
	for j := range later {
		decl := later[j].Declaration
		if decl.Len == 0 {
			continue
		}
		shift := 0
		for _, sp := range splices {
			switch {
			case sp.document(m.Source.Name()) != m.declarationDoc(later[j]):
				// Another document's bytes.
			case sp.span.End() <= decl.Offset:
				shift += len(sp.text) - sp.span.Len
			case sp.span.Offset <= decl.Offset:
				return m.declarationGone(first+j, later[j])
			}
		}
		later[j].Declaration.Offset += shift
	}
	return nil
}

// relocateDeclarations reads each later operation's Declaration afresh from the
// reparsed source, where its extent may have changed but its start has not.
func (m Model) relocateDeclarations(later []Operation, first int) error {
	for j := range later {
		decl := later[j].Declaration
		if decl.Len == 0 {
			continue
		}
		root, err := m.declarationRoot(first+j, later[j])
		if err != nil {
			return err
		}
		sym := root.DeclaredFrom(decl.Offset)
		if sym == nil {
			return m.declarationGone(first+j, later[j])
		}
		later[j].Declaration = sym.DeclSpan
	}
	return nil
}

func (m Model) declarationGone(i int, op Operation) error {
	return &Error{Failure: FailureUnknownTarget, OperationIndex: i,
		Message: fmt.Sprintf("an earlier operation rewrote the declaration at %s; nothing is declared there now", m.at(op))}
}

// checkOverlap refuses edits covering the same non-empty source bytes.
func checkOverlap(splices []splice) error {
	ordered := make([]splice, len(splices))
	copy(ordered, splices)
	sort.SliceStable(ordered, func(a, b int) bool {
		return ordered[a].span.Offset < ordered[b].span.Offset
	})
	for i := 1; i < len(ordered); i++ {
		prev, cur := ordered[i-1], ordered[i]
		if cur.span.Offset < prev.span.End() && (cur.span.Len > 0 || prev.span.Len > 0) {
			return &Error{
				Failure:        FailureOverlappingEdits,
				OperationIndex: cur.opIndex,
				Message: fmt.Sprintf("edits to %s and %s cover the same source bytes",
					prev.target, cur.target),
			}
		}
	}
	return nil
}
