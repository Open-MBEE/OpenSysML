package edit

import (
	"fmt"
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/rename"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// renameSplices are the byte ranges a rename rewrites: the declaration's name
// token and every reference to it in this source. A rename that would capture or
// shadow another name is refused.
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
	if err := m.refuseReferencedElsewhere(i, op, writesName(sym, ident.Name)); err != nil {
		return nil, err
	}
	r, sem := m.resolver()
	occurrences := m.renameOccurrences(r, sym, ident)
	if c := rename.Check(r, sem, sym, ident.Name, op.NewName, occurrences); c != nil {
		e := &Error{Failure: FailureInvalidName, OperationIndex: i, Message: c.Error()}
		if c.Site != "" {
			e.Referring = []string{c.Site}
		}
		return nil, e
	}
	out := make([]splice, 0, len(occurrences)+1)
	out = append(out, splice{span: ident.NameSpan, text: op.NewName, opIndex: i, target: op.Target})
	for _, occ := range occurrences {
		out = append(out, splice{span: occ.Span(), text: op.NewName, opIndex: i, target: op.Target})
	}
	return out, nil
}

// writesName reports a reference segment written as name and reading sym by it,
// which renaming that name rewrites; one written as an alias or the short name
// still resolves and is left alone.
func writesName(sym *symbols.Symbol, name string) referenceTest {
	return func(r *resolve.Resolver, ref resolve.Reference, part int) bool {
		if ref.QN.Parts[part].Text != name {
			return false
		}
		seg, ok := r.PartName(ref.QN, part)
		return ok && symbols.SameElement(seg, sym)
	}
}

// renameOccurrences returns every reference spelling sym's declared name, in
// source order. A reference written with the short name is left alone: the
// rename does not change the short name. The LSP applies the same rule through
// Workspace.NameReferencesTo.
func (m Model) renameOccurrences(r *resolve.Resolver, sym *symbols.Symbol, ident ast.Identification) []rename.Occurrence {
	rootScope := m.Index.DocumentRoot(m.Source.Name())
	if rootScope == nil {
		return nil
	}
	seen := map[int]bool{}
	var out []rename.Occurrence
	for _, ref := range resolve.References(m.Root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for part, segment := range ref.QN.Parts {
			if segment.Span == ident.NameSpan || segment.Text != ident.Name ||
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
			out = append(out, rename.Occurrence{Ref: ref, Part: part})
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Span().Offset < out[b].Span().Offset })
	return out
}
