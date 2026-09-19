package edit

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// renameSplices are the byte ranges a rename rewrites: the declaration's name
// token and every reference to it in this source and in every other document
// the edit may rewrite. A rename that would capture or shadow another name, or
// that a document the edit may not rewrite refers to, is refused.
func (m Model) renameSplices(i int, op Operation, sym *symbols.Symbol) ([]splice, error) {
	ident, ok := symbols.DeclIdent(sym.Decl)
	if !ok || ident.Name == "" || ident.NameSpan.Len == 0 {
		return nil, &Error{
			Failure:        FailureNotNamed,
			OperationIndex: i,
			Message: fmt.Sprintf("%s declares no name of its own to rename"+
				" (a shorthand redefinition names the feature it redefines)", op.Target),
		}
	}
	if err := checkName(i, op.NewName); err != nil {
		return nil, err
	}
	r, sem := m.resolver()
	occurrences, elsewhere := m.renameOccurrences(r, sym, ident)
	if len(elsewhere) > 0 {
		return nil, referencedElsewhere(i, m.Source.Name(), op.Target, elsewhere)
	}
	checked := make([]RenameOccurrence, 0, len(occurrences))
	for _, occ := range occurrences {
		checked = append(checked, occ.RenameOccurrence)
	}
	if c := CheckRename(r, sem, sym, ident.Name, op.NewName, checked); c != nil {
		e := &Error{Failure: FailureInvalidName, OperationIndex: i, Message: c.Error()}
		if c.Site != "" {
			e.Referrers = []Referrer{{Name: c.Site, Document: occurrences[c.Occurrence].doc}}
			e.Referring = referring(m.Source.Name(), e.Referrers)
		}
		return nil, e
	}
	out := make([]splice, 0, len(occurrences)+1)
	out = append(out, splice{span: ident.NameSpan, text: op.NewName, opIndex: i, target: op.Target})
	for _, occ := range occurrences {
		sp := splice{span: occ.Span(), text: op.NewName, opIndex: i, target: op.Target}
		if occ.doc != m.Source.Name() {
			sp.doc = occ.doc
		}
		out = append(out, sp)
	}
	return out, nil
}

// occurrence is a reference a rename rewrites and the document it is written in.
type occurrence struct {
	RenameOccurrence
	doc string
}

// renameOccurrences returns every reference spelling sym's declared name in the
// documents the edit may rewrite, the edited one first and each in source order,
// and names the declarations of the documents it may not rewrite that spell it.
// A reference written with the short name is left alone: the rename does not
// change the short name. The LSP applies the same rule through
// Workspace.NameReferencesTo.
func (m Model) renameOccurrences(r *resolve.Resolver, sym *symbols.Symbol, ident ast.Identification) ([]occurrence, []Referrer) {
	var out []occurrence
	var elsewhere []Referrer
	for _, doc := range m.workspaceDocuments() {
		root, rootScope, ok := m.documentRoot(doc)
		if !ok {
			continue
		}
		_, rewritable := m.inDocument(doc)
		seen := map[int]bool{}
		seenReferrers := map[ast.Node]bool{}
		var found []occurrence
		for _, ref := range resolve.References(root, rootScope) {
			if ref.QN == nil {
				continue
			}
			r.ResolveReference(ref)
			for part, segment := range ref.QN.Parts {
				if isDeclaration(doc, segment.Span, sym) || segment.Text != ident.Name ||
					seen[segment.Span.Offset] {
					continue
				}
				// A segment written as an alias name reads the alias membership, so
				// renaming the alias rewrites it and renaming the target does not.
				seg, ok := r.PartName(ref.QN, part)
				if !ok || !symbols.SameElement(seg, sym) {
					continue
				}
				seen[segment.Span.Offset] = true
				if rewritable {
					found = append(found, occurrence{RenameOccurrence: RenameOccurrence{Ref: ref, Part: part}, doc: doc})
					continue
				}
				referrer, ok := m.referrer(r, doc, ref, segment.Span.Offset)
				if ok && !seenReferrers[referrer.node] {
					seenReferrers[referrer.node] = true
					elsewhere = append(elsewhere, referrer.referrer())
				}
			}
		}
		sort.Slice(found, func(a, b int) bool { return found[a].Span().Offset < found[b].Span().Offset })
		out = append(out, found...)
	}
	sortReferrers(elsewhere)
	return out, elsewhere
}
