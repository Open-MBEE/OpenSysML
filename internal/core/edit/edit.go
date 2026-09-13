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
)

// Operation is one change to make to a model's source.
type Operation struct {
	Kind OpKind
	// Target is the element to edit, by FQN, as symbols name it.
	Target string
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
	// against and no document of its own, for analyzing the edited notation.
	// Nil checks syntax alone.
	NewIndex func() *symbols.Index
	// Analysis is the options the edited notation is judged under, the same the
	// original's SemDiags came from; the zero value is the default mode.
	Analysis passes.Options
	// reindex is the one index an Apply call analyzes in, set by Apply.
	reindex *reindexer
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

// Result is the edited notation and what each operation changed.
type Result struct {
	Content []byte
	Applied []Applied
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
	content := append([]byte(nil), m.Source.Bytes()...)
	applied := make([]Applied, 0, len(ops))
	for i, op := range ops {
		splices, err := current.splicesFor(i, op)
		if err != nil {
			return nil, err
		}
		if err := checkOverlap(splices); err != nil {
			return nil, err
		}
		next := current.splice(splices)
		for _, sp := range splices {
			applied = append(applied, Applied{
				OperationIndex: sp.opIndex,
				Target:         sp.target,
				Span:           sp.span,
				OldText:        current.Source.Text(sp.span),
				NewText:        sp.text,
			})
		}
		content = next
		current, err = reparseModel(m, content)
		if err != nil {
			return nil, err
		}
	}
	if err := m.validate(content); err != nil {
		return nil, err
	}
	return &Result{Content: content, Applied: applied}, nil
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
	if err := checkOverlap(splices); err != nil {
		return nil, err
	}
	content := m.splice(splices)
	if err := m.validate(content); err != nil {
		return nil, err
	}
	applied := make([]Applied, len(splices))
	for i, sp := range splices {
		applied[i] = Applied{
			OperationIndex: sp.opIndex,
			Target:         sp.target,
			Span:           sp.span,
			OldText:        m.Source.Text(sp.span),
			NewText:        sp.text,
		}
	}
	return &Result{Content: content, Applied: applied}, nil
}

// reparseModel is the state the later operations locate their targets in: it is
// parsed and indexed, and not analyzed — an edit is judged by the original's
// diagnostics and the returned notation's, which validate takes.
func reparseModel(base Model, content []byte) (Model, error) {
	sf := source.NewWithKind(base.Source.Name(), content, base.Source.Kind())
	p := parser.New(sf)
	root := p.ParseFile()
	idx := base.reindex.analyzedIn(sf.Name(), root, sf.Kind())
	return Model{
		Source: sf, Root: root, Index: idx,
		ParseDiags: p.Diagnostics,
		NewIndex:   base.NewIndex, Analysis: base.Analysis, reindex: base.reindex,
	}, nil
}

// splice is one byte range of the original source to replace with text.
type splice struct {
	span    source.Span
	text    string
	opIndex int
	target  string
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
