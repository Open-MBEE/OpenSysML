package edit

import (
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func (m Model) deleteSplices(i int, op Operation) ([]splice, error) {
	sym, err := m.target(i, op)
	if err != nil {
		return nil, err
	}
	targets, elsewhere := m.cascadeTargets(sym)
	if len(elsewhere) > 0 {
		return nil, referencedElsewhere(i, m.Source.Name(), op.Target, elsewhere)
	}
	if len(targets) > 1 && !op.Cascade {
		referrers := make([]Referrer, 0, len(targets)-1)
		for _, referrer := range targets[1:] {
			referrers = append(referrers, referrer.referrer())
		}
		referring := referring(m.Source.Name(), referrers)
		return nil, &Error{
			Failure:        FailureDeleteReferenced,
			OperationIndex: i,
			Referring:      referring,
			Referrers:      referrers,
			Message: op.Target + " is referenced by " + strings.Join(referring, ", ") +
				"; delete it with cascade to remove those declarations",
		}
	}
	out := make([]splice, 0, len(targets))
	for _, target := range targets {
		in, _ := m.inDocument(target.doc)
		sp := splice{span: in.deleteSpan(target), opIndex: i, target: target.name}
		if target.doc != m.Source.Name() {
			sp.doc = target.doc
		}
		out = append(out, sp)
	}
	return out, nil
}

// deletion is one declaration a delete removes: a symbol's, or one no symbol
// stands for — an import or a filter, which has a span to remove all the same.
type deletion struct {
	node ast.Node
	span source.Span
	// sym is nil for a declaration no symbol stands for.
	sym  *symbols.Symbol
	name string
	// doc is the document declaring it.
	doc string
}

// referrer presents the deletion as a referrer of the target being deleted.
func (d deletion) referrer() Referrer {
	return Referrer{Name: d.name, Document: d.doc}
}

// symbolDeletion is the deletion of sym, declared in doc, or false for a symbol
// declared by no node of its own. An anonymous declaration is named by its
// heading when doc's source is at hand, else by its keyword, each in its namespace.
func (m Model) symbolDeletion(r *resolve.Resolver, doc string, sym *symbols.Symbol) (deletion, bool) {
	if sym == nil || sym.Decl == nil {
		return deletion{}, false
	}
	span := symbolSpan(sym)
	name := notationName(sym)
	if sym.Name == "" {
		name = anonymousName(sym)
		if in, ok := m.inDocument(doc); ok {
			name = in.heading(span)
		}
		name += " in " + m.namespaceName(doc, sym.OwnerScope)
	}
	return deletion{node: sym.Decl, span: span, sym: sym, name: name, doc: doc}, true
}

// anonymousName labels an unnamed declaration by its notation's keyword, or by
// its kind where the notation has none: `an anonymous part`.
func anonymousName(sym *symbols.Symbol) string {
	if kw := ast.Notation(sym.Decl); kw != "" {
		return "an anonymous " + kw
	}
	return "an anonymous " + sym.Kind.String()
}

// symbolSpan is the span of sym's declaration.
func symbolSpan(sym *symbols.Symbol) source.Span {
	if sym.DeclSpan.Len == 0 && sym.Decl != nil {
		return sym.Decl.Span()
	}
	return sym.DeclSpan
}

// encloses reports whether inner lies within outer, the two being equal included.
func encloses(outer, inner source.Span) bool {
	return outer.Offset <= inner.Offset && outer.End() >= inner.End()
}

// covers reports whether sym is declared inside d, d's own declaration included.
func (d deletion) covers(sym *symbols.Symbol) bool {
	return sym != nil && sym.DocName == d.doc && encloses(d.span, symbolSpan(sym))
}

// coveredBy reports whether d lies inside one of set, or is one of them.
func (d deletion) coveredBy(set []deletion) bool {
	for _, other := range set {
		if other.doc == d.doc && encloses(other.span, d.span) {
			return true
		}
	}
	return false
}

// cascadeTargets is sym followed by every declaration removing it would leave
// dangling: referrers of sym or of anything it contains, then their referrers,
// until none remain, in the edited document and in every other the edit may
// rewrite. Referrers in documents it may not rewrite are returned as elsewhere
// instead, and are not followed further.
func (m Model) cascadeTargets(sym *symbols.Symbol) (targets []deletion, elsewhere []Referrer) {
	r, _ := m.resolver()
	first, ok := m.symbolDeletion(r, m.Source.Name(), sym)
	if !ok {
		return nil, nil
	}
	targets = []deletion{first}
	seen := map[ast.Node]bool{first.node: true}
	for frontier := targets; len(frontier) > 0; {
		var next []deletion
		followed, unfollowed := m.referrers(r, targets, frontier)
		for _, referrer := range followed {
			if !seen[referrer.node] {
				seen[referrer.node] = true
				next = append(next, referrer)
			}
		}
		for _, referrer := range unfollowed {
			if !seen[referrer.node] {
				seen[referrer.node] = true
				elsewhere = append(elsewhere, referrer.referrer())
			}
		}
		targets = append(targets, next...)
		frontier = next
	}
	sortReferrers(elsewhere)
	return withoutNested(targets), elsewhere
}

// withoutNested drops every deletion lying inside another's span, which
// removes it already.
func withoutNested(targets []deletion) []deletion {
	out := make([]deletion, 0, len(targets))
	for _, d := range targets {
		nested := false
		for _, other := range targets {
			if other.node != d.node && other.doc == d.doc && encloses(other.span, d.span) {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, d)
		}
	}
	return out
}

// referrers returns the declarations outside every target that refer to one of
// frontier or to a declaration inside one: those of documents the edit may
// rewrite, the edited one first, then those of documents it may not. Each
// document's are sorted by name then position, and same-named declarations are
// distinct referrers.
func (m Model) referrers(r *resolve.Resolver, targets, frontier []deletion) (followed, unfollowed []deletion) {
	for _, doc := range m.workspaceDocuments() {
		found := m.referrersIn(r, doc, targets, frontier)
		if _, ok := m.inDocument(doc); ok {
			followed = append(followed, found...)
		} else {
			unfollowed = append(unfollowed, found...)
		}
	}
	return followed, unfollowed
}

// referrersIn is referrers within the one document doc, whose root is read from
// the index.
func (m Model) referrersIn(r *resolve.Resolver, doc string, targets, frontier []deletion) []deletion {
	root, rootScope, ok := m.documentRoot(doc)
	if !ok {
		return nil
	}
	seen := map[ast.Node]bool{}
	var out []deletion
	for _, ref := range resolve.References(root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for i, part := range ref.QN.Parts {
			seg, ok := r.PartSymbol(ref.QN, i)
			if !ok || isDeclaration(doc, part.Span, seg) || !coveredByAny(seg, frontier) {
				continue
			}
			referrer, ok := m.referrer(r, doc, ref, part.Span.Offset)
			if !ok || seen[referrer.node] || referrer.coveredBy(targets) {
				continue
			}
			seen[referrer.node] = true
			out = append(out, referrer)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].span.Offset < out[j].span.Offset
	})
	return out
}

// workspaceDocuments names the documents a reference may be written in: the
// edited one first, then the others in name order — those the model names, or
// else the index's unmarked documents.
func (m Model) workspaceDocuments() []string {
	own := m.Source.Name()
	out := []string{own}
	others := m.Documents
	if others == nil {
		others = m.Index.WorkspaceDocuments()
	} else {
		others = slices.Sorted(slices.Values(others))
	}
	for _, doc := range others {
		if doc != own {
			out = append(out, doc)
		}
	}
	return out
}

// documentRoot is the parsed root of doc and its scope, as the index holds them.
func (m Model) documentRoot(doc string) (*ast.RootNamespace, *symbols.Scope, bool) {
	rootScope := m.Index.DocumentRoot(doc)
	if doc == m.Source.Name() {
		return m.Root, rootScope, rootScope != nil
	}
	if rootScope == nil {
		return nil, nil, false
	}
	root, ok := rootScope.Node().(*ast.RootNamespace)
	return root, rootScope, ok
}

// isDeclaration reports whether the segment at span in doc is sym's own name
// token; an equal span in another document is a reference.
func isDeclaration(doc string, span source.Span, sym *symbols.Symbol) bool {
	return sym.DocName == doc && span == sym.NameSpan
}

// coveredByAny reports whether sym is declared inside one of set.
func coveredByAny(sym *symbols.Symbol, set []deletion) bool {
	for _, d := range set {
		if d.covers(sym) {
			return true
		}
	}
	return false
}

// referrer is the declaration of doc that a reference at offset is written in:
// the import or filter making it, which no symbol stands for, else the innermost
// symbol whose declaration covers the offset.
func (m Model) referrer(r *resolve.Resolver, doc string, ref resolve.Reference, offset int) (deletion, bool) {
	switch member := ref.Member.(type) {
	case *ast.Import:
		return deletion{node: member, span: member.Span(), doc: doc,
			name: importName(member) + " in " + m.namespaceName(doc, ref.Scope)}, true
	case *ast.FilterMember:
		return deletion{node: member, span: member.Span(), doc: doc,
			name: "the filter in " + m.namespaceName(doc, ref.Scope)}, true
	}
	return m.symbolDeletion(r, doc, m.symbolContaining(doc, offset))
}

// importName spells an import as its notation does, `import P::Base` or
// `import P::*`.
func importName(imp *ast.Import) string {
	parts := make([]string, 0, len(imp.Imported.Parts)+1)
	for _, part := range imp.Imported.Parts {
		parts = append(parts, part.Text)
	}
	if imp.Kind == ast.ImportNamespace {
		if imp.IsRecursive {
			parts = append(parts, "**")
		} else {
			parts = append(parts, "*")
		}
	}
	kw := "import"
	if imp.IsExpose {
		kw = "expose"
	}
	return kw + " " + strings.Join(parts, "::")
}

// heading is the notation of the declaration at span up to its body or `;`,
// on one line: `part : Base` for an anonymous usage.
func (m Model) heading(span source.Span) string {
	end := span.Offset
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF && tok.Span.Offset < span.End(); tok = lx.Next() {
		if tok.Span.Offset < span.Offset || tok.IsTrivia() {
			continue
		}
		if tok.Kind == lexer.LBrace || tok.Kind == lexer.Semicolon {
			break
		}
		end = tok.Span.End()
	}
	return strings.Join(strings.Fields(m.Source.Text(source.Span{Offset: span.Offset, Len: end - span.Offset})), " ")
}

// namespaceName names the namespace declaring scope's members: its qualified
// name, or the document for the root.
func (m Model) namespaceName(doc string, scope *symbols.Scope) string {
	for s := scope; s != nil; s = s.Parent() {
		if owner := s.Owner(); owner != nil && owner.Name != "" {
			return notationName(owner)
		}
	}
	return docLabel(doc)
}

// symbolContaining is the innermost symbol of doc, anonymous ones included,
// whose declaration covers offset.
func (m Model) symbolContaining(doc string, offset int) *symbols.Symbol {
	var found *symbols.Symbol
	var visit func(scope *symbols.Scope)
	visit = func(scope *symbols.Scope) {
		scope.ForEachMember(func(candidate *symbols.Symbol) bool {
			if candidate.Decl == nil || candidate.DocName != doc {
				return true
			}
			span := symbolSpan(candidate)
			if span.Len == 0 || offset < span.Offset || offset >= span.End() {
				return true
			}
			if found == nil || span.Len < symbolSpan(found).Len {
				found = candidate
			}
			return true
		})
		for _, child := range scope.Children() {
			visit(child)
		}
	}
	if root := m.Index.DocumentRoot(doc); root != nil {
		visit(root)
	}
	return found
}

func (m Model) deleteSpan(d deletion) source.Span {
	span := d.span
	content := m.Source.Bytes()
	start := span.Offset
	end := m.declarationEnd(d)
	if end <= start || end > len(content) {
		end = span.End()
	}
	lineStart := start
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	lineEnd := end
	for lineEnd < len(content) && content[lineEnd] != '\n' {
		lineEnd++
	}
	// A declaration on its own line owns that line, including its newline and
	// a line comment written after it.
	if onlyWhitespace(content[lineStart:start]) && trailingComment(content[end:lineEnd]) {
		if lineEnd < len(content) {
			lineEnd++
		}
		start = lineStart
		end = lineEnd
	}
	// Include contiguous leading comment lines, but stop at a blank line so a
	// neighboring declaration's comment ownership is never consumed.
	for {
		cursor := start - 1
		if cursor < 0 {
			break
		}
		if content[cursor] == '\n' {
			cursor--
		}
		if cursor < 0 {
			break
		}
		prevLineEnd := cursor + 1
		prevLineStart := prevLineEnd
		for prevLineStart > 0 && content[prevLineStart-1] != '\n' {
			prevLineStart--
		}
		line := strings.TrimSpace(string(content[prevLineStart:prevLineEnd]))
		if line == "" {
			// A blank line immediately before a leading comment belongs to
			// that comment block and is removed with it.
			start = prevLineStart
			break
		}
		if len(line) < 2 || (!strings.HasPrefix(line, "//") &&
			!strings.HasPrefix(line, "/*") && !strings.HasPrefix(line, "*")) {
			break
		}
		start = prevLineStart
	}
	return source.Span{Offset: start, Len: end - start}
}

// declarationEnd is where d's notation ends: its terminating `;` or closing
// brace, or the last token of a declaration written without one, as a comment.
func (m Model) declarationEnd(d deletion) int {
	start := d.span.Offset
	if d.sym != nil && d.sym.NameSpan.End() > start {
		start = d.sym.NameSpan.End()
	}
	depth, last := 0, 0
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF && tok.Span.Offset < d.span.End(); tok = lx.Next() {
		if tok.Span.End() <= start {
			continue
		}
		switch tok.Kind {
		case lexer.LBrace:
			depth++
		case lexer.RBrace:
			if depth > 0 {
				depth--
				if depth == 0 {
					return tok.Span.End()
				}
			}
		case lexer.Semicolon:
			if depth == 0 {
				return tok.Span.End()
			}
		}
		if !tok.IsTrivia() {
			last = tok.Span.End()
		}
	}
	return last
}

// trailingComment reports whether b, the rest of a declaration's line, holds
// nothing but a `//` comment.
func trailingComment(b []byte) bool {
	rest := strings.TrimLeft(string(b), " \t\r")
	return rest == "" || strings.HasPrefix(rest, "//")
}

func onlyWhitespace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}
