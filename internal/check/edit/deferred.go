package edit

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// deferredRefs are the references the operations of one Apply call wrote that
// are judged against the model the whole batch leaves, so each may name a
// declaration a later operation adds. A reference is followed by the byte it
// starts at in the edited document, which the later operations' splices move,
// so it is read under the name a later rename gives it.
type deferredRefs struct {
	pending  []pendingRefs
	anchored []deferredRef
}

// pendingRefs are the references one operation's splice wrote, to be located in
// its text once that text is parsed: region is the splice's text in the edited
// document as the operation leaves it, and locate finds the references in it.
type pendingRefs struct {
	region source.Span
	locate func(after Model, region source.Span) []deferredRef
}

// deferredRef is one reference: the byte it starts at in the edited document and
// the check that judges what is written there in the final model. Nothing being
// written there any more — a later operation rewrote it — leaves the verdict to
// the validation of the whole result.
type deferredRef struct {
	offset int
	check  func(final Model, offset int) error
}

// resolveLater has the references sp — the one splice its operation makes in
// the edited document — writes located once its text is parsed and judged
// against the model the batch leaves.
func (m Model) resolveLater(sp splice, locate func(after Model, region source.Span) []deferredRef) {
	if m.deferred == nil {
		return
	}
	m.deferred.pending = append(m.deferred.pending, pendingRefs{
		region: source.Span{Offset: sp.span.Offset, Len: len(sp.text)}, locate: locate,
	})
}

// rebase moves the anchored references past the splices one operation applied
// to the edited document, named own. A reference a splice rewrites in place —
// its bytes start where the reference does, as a rename's do — stays where it
// is; one a splice covers from before is gone.
func (d *deferredRefs) rebase(own string, splices []splice) {
	if d == nil {
		return
	}
	kept := d.anchored[:0]
	for _, ref := range d.anchored {
		shift, gone := 0, false
		for _, sp := range splices {
			switch {
			case sp.document(own) != own:
			case sp.span.End() <= ref.offset:
				shift += len(sp.text) - sp.span.Len
			case sp.span.Offset == ref.offset:
			case sp.span.Offset < ref.offset:
				gone = true
			}
		}
		if gone {
			continue
		}
		ref.offset += shift
		kept = append(kept, ref)
	}
	d.anchored = kept
}

// locate anchors the pending references in after, the model the operation that
// wrote them leaves, where their text is parsed and their names still stand.
func (d *deferredRefs) locate(after Model) {
	if d == nil {
		return
	}
	pending := d.pending
	d.pending = nil
	for _, p := range pending {
		d.anchored = append(d.anchored, p.locate(after, p.region)...)
	}
}

// settle judges the references, in operation order, against final, the model
// the whole batch leaves; the first refusal is the batch's. References still
// pending were written by the one operation applied since they were, and are
// located in final itself.
func (d *deferredRefs) settle(final Model) error {
	if d == nil || len(d.pending)+len(d.anchored) == 0 {
		return nil
	}
	d.locate(final)
	for _, ref := range d.anchored {
		if err := ref.check(final, ref.offset); err != nil {
			return err
		}
	}
	return nil
}

// inspectWithin visits the nodes of root whose span reaches into region, in
// source order, descending into no node that lies wholly outside it.
func inspectWithin(root ast.Node, region source.Span, visit func(ast.Node)) {
	ast.Inspect(root, func(n ast.Node) bool {
		if _, isRoot := n.(*ast.RootNamespace); !isRoot {
			sp := n.Span()
			if sp.Len > 0 && (sp.End() <= region.Offset || sp.Offset >= region.End()) {
				return false
			}
		}
		visit(n)
		return true
	})
}

// successionEnds are the names a body item writes as the ends of a succession,
// in source order: a `then`, guarded `then` or `else` target, and the node a
// `first` starts at or the succession it opens leads to. An end the notation
// implied — the source a `then <target>` takes from the member before — is a
// member's own name, not a reference.
func successionEnds(n ast.Node) []*ast.QualifiedName {
	var ends []*ast.QualifiedName
	add := func(qn *ast.QualifiedName, implied bool) {
		if qn != nil && !implied && len(qn.Parts) > 0 {
			ends = append(ends, qn)
		}
	}
	switch d := n.(type) {
	case *ast.SuccessionEdge:
		add(d.Source, d.SourceImplied)
		add(d.Target, d.TargetImplied)
	case *ast.ControlFlowEdge:
		add(d.Source, d.SourceImplied)
		add(d.Target, d.TargetImplied)
	case *ast.InitialNode:
		add(d.First, false)
		add(d.Successor, false)
	}
	return ends
}

// successionEndAt is the succession end written at offset in m's document and
// the scope of the body it is written in, or nil where none is.
func (m Model) successionEndAt(offset int) (*ast.QualifiedName, *symbols.Scope) {
	var found *ast.QualifiedName
	inspectWithin(m.Root, source.Span{Offset: offset, Len: 1}, func(n ast.Node) {
		for _, qn := range successionEnds(n) {
			if qn.Span().Offset == offset {
				found = qn
			}
		}
	})
	if found == nil {
		return nil, nil
	}
	return found, m.bodyScopeAt(offset)
}

// childCovering is the child of scope whose declaration covers offset, or nil.
func childCovering(scope *symbols.Scope, offset int) *symbols.Scope {
	for _, child := range scope.Children() {
		node := child.Node()
		if node == nil {
			continue
		}
		if sp := node.Span(); sp.Offset <= offset && offset < sp.End() {
			return child
		}
	}
	return nil
}

// bodyScopeAt is the innermost scope of m's document whose declaration covers
// offset and which is a body a succession end at offset is written in: the
// scope a `first` label or a succession's own body opens is not, as its ends
// are written in the body around it.
func (m Model) bodyScopeAt(offset int) *symbols.Scope {
	scope := m.Index.DocumentRoot(m.Source.Name())
	for {
		next := childCovering(scope, offset)
		if next == nil {
			return scope
		}
		switch next.Node().(type) {
		case *ast.InitialNode, *ast.SuccessionEdge, *ast.ControlFlowEdge:
			return scope
		}
		scope = next
	}
}

// metadataPrefixAt is the metadata prefix written at offset in m's document and
// the declaration it prefixes, or nil where none is.
func (m Model) metadataPrefixAt(offset int) (*symbols.Symbol, *ast.PrefixMetadata) {
	var (
		sym    *symbols.Symbol
		prefix *ast.PrefixMetadata
	)
	scope := m.Index.DocumentRoot(m.Source.Name())
	for scope != nil {
		scope.ForEachMember(func(candidate *symbols.Symbol) bool {
			if sp := candidate.DeclSpan; sp.Offset <= offset && offset < sp.End() {
				prefixes, _, _ := ast.DeclaredMetadata(candidate.Decl)
				for _, p := range prefixes {
					if p != nil && p.Span().Offset == offset {
						sym, prefix = candidate, p
						return false
					}
				}
			}
			return true
		})
		if prefix != nil {
			return sym, prefix
		}
		scope = childCovering(scope, offset)
	}
	return nil, nil
}
