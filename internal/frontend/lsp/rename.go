package lsp

import (
	"context"
	"fmt"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// renameTarget is the name under the rename cursor: the symbol it belongs to,
// which of that symbol's names it is (long or short) and where the declaration
// states that name.
type renameTarget struct {
	sym      *symbols.Symbol
	name     string
	declSpan source.Span
}

// PrepareRename reports the range the client should offer for editing, and
// rejects positions that name nothing renameable before the user types.
func (s *Server) PrepareRename(ctx context.Context, params *protocol.PrepareRenameParams) (*protocol.Range, error) {
	name := uriToName(params.TextDocument.URI)
	_, span, err := s.renameTargetAt(name, params.Position)
	if err != nil {
		return nil, err
	}
	rng := spanToRange(s.ws.Document(name).Content, span)
	return &rng, nil
}

// Rename renames the name under the cursor — a declaration's long or short name —
// where the declaration states it and at every reference across the workspace's
// documents written with that name, including the qualifier positions of
// multi-segment names (`A::B` when renaming A). A reference written with the
// element's other name still resolves afterwards, so it is left as written.
func (s *Server) Rename(ctx context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	name := uriToName(params.TextDocument.URI)
	target, _, err := s.renameTargetAt(name, params.Position)
	if err != nil {
		return nil, err
	}
	if err := validateNewName(params.NewName); err != nil {
		return nil, err
	}
	conflict, err := s.ws.RenameConflict(target.sym, target.name, params.NewName)
	if err != nil {
		return nil, err
	}
	if conflict != nil {
		return nil, conflict
	}

	changes := map[protocol.DocumentURI][]protocol.TextEdit{}
	// One name occurrence is edited once however many times it is collected: a
	// shorthand redefinition (`part redefines x;`) is both the declaration and a
	// reference at the same span, and clients reject overlapping edits.
	edited := map[protocol.DocumentURI]map[source.Span]bool{}
	posOf := map[string]positions{}
	addEdit := func(docName string, content []byte, span source.Span) {
		uri := nameToURI(docName)
		if edited[uri] == nil {
			edited[uri] = map[source.Span]bool{}
		}
		if edited[uri][span] {
			return
		}
		edited[uri][span] = true
		pos, ok := posOf[docName]
		if !ok {
			pos = positionsFor(content)
			posOf[docName] = pos
		}
		changes[uri] = append(changes[uri], protocol.TextEdit{
			Range:   pos.rangeOf(span),
			NewText: params.NewName,
		})
	}

	// The declaration itself.
	declDoc := s.ws.Document(target.sym.DocName)
	addEdit(target.sym.DocName, declDoc.Content, target.declSpan)

	// Every segment, in every document, that writes this name of target: an alias
	// use is rewritten by renaming the alias and not by renaming its target.
	refs, err := s.ws.NameReferencesTo(target.sym, target.name)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		addEdit(ref.Doc, ref.Content, ref.Span)
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

// renameTargetAt returns the name at pos and the span of that name as written
// there (a declaration's long or short identifier, or one segment of a reference).
func (s *Server) renameTargetAt(name string, pos protocol.Position) (renameTarget, source.Span, error) {
	doc := s.ws.Document(name)
	if doc == nil || doc.Scope == nil {
		return renameTarget{}, source.Span{}, fmt.Errorf("no document %q", name)
	}
	offset := positionToOffset(doc.Content, pos)

	// On a reference: rename the symbol the containing segment denotes, so
	// renaming from the `A` of `A::B` renames A, not B.
	if ref := refAtOffset(collectRefs(doc.AST, doc.Scope), offset); ref != nil {
		segs := s.ws.ResolveReferenceNameSegmentsInDoc(name, *ref)
		for i, part := range ref.QN.Parts {
			if offset < part.Span.Offset || offset >= part.Span.End() {
				continue
			}
			if i < len(segs) && segs[i] != nil {
				return s.renameable(segs[i], part.Text, part.Span)
			}
			if i == len(ref.QN.Parts)-1 && len(s.ws.AmbiguousInvocationInDoc(name, *ref)) > 0 {
				return renameTarget{}, source.Span{}, fmt.Errorf("cannot rename %q: the call is ambiguous between several overloads", part.Text)
			}
			return renameTarget{}, source.Span{}, fmt.Errorf("cannot rename %q: unresolved", part.Text)
		}
	}

	// On a declaration: the cursor must be on a declared identifier itself — the
	// name or the short name between its angle brackets — not merely somewhere
	// inside the declaration's body.
	if sym := symbolAtOffset(doc.Scope, offset); sym != nil {
		if sp := sym.NameSpan; sp.Len > 0 && offset >= sp.Offset && offset < sp.End() {
			return s.renameable(sym, sym.Name, sp)
		}
		if sp := shortNameSpan(sym); sp.Len > 0 && offset >= sp.Offset && offset < sp.End() {
			return s.renameable(sym, sym.ShortName, sp)
		}
	}
	return renameTarget{}, source.Span{}, fmt.Errorf("no renameable name at this position")
}

// renameable rejects symbols whose declaration this server cannot edit, and
// otherwise pairs the name written at span with where sym's declaration states it.
func (s *Server) renameable(sym *symbols.Symbol, name string, span source.Span) (renameTarget, source.Span, error) {
	if sym.NameSpan.Len == 0 {
		return renameTarget{}, source.Span{}, fmt.Errorf("cannot rename %q: no declared name", sym.Name)
	}
	if s.ws.Document(sym.DocName) == nil {
		// Standard-library and other out-of-workspace declarations: renaming
		// the references alone would break the model.
		return renameTarget{}, source.Span{}, fmt.Errorf("cannot rename %q: declared outside the workspace", sym.Name)
	}
	if sym.Recorded() {
		return renameTarget{}, source.Span{}, symbols.NeedsTree(sym, "renaming "+sym.Name)
	}
	target := renameTarget{sym: sym, name: name, declSpan: sym.NameSpan}
	if name != sym.Name {
		if sp := shortNameSpan(sym); name == sym.ShortName && sp.Len > 0 {
			target.declSpan = sp
		} else {
			return renameTarget{}, source.Span{}, fmt.Errorf("cannot rename %q: not a name %q declares", name, sym.Name)
		}
	}
	return target, span, nil
}

// shortNameSpan is where sym's declaration states its short name (`<s>`), or an
// empty span when it states none.
func shortNameSpan(sym *symbols.Symbol) source.Span {
	id, ok := symbols.DeclIdent(sym.Decl)
	if !ok || id.ShortName == "" {
		return source.Span{}
	}
	return id.ShortNameSpan
}

// validateNewName rejects names that would not lex as the identifier they are
// meant to replace.
func validateNewName(name string) error {
	if name == "" {
		return fmt.Errorf("new name is empty")
	}
	if source.IsKeyword(name) {
		return fmt.Errorf("%q is a keyword", name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		ok := c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && c >= '0' && c <= '9')
		if !ok {
			return fmt.Errorf("%q is not a valid identifier", name)
		}
	}
	return nil
}
