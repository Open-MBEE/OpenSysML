package edit

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func (m Model) deleteSplices(i int, op Operation) ([]splice, error) {
	sym, err := m.target(i, op)
	if err != nil {
		return nil, err
	}
	targets := m.cascadeTargets(sym)
	if err := m.refuseReferencedElsewhere(i, op, m.reachesDeletion(targets)); err != nil {
		return nil, err
	}
	if len(targets) > 1 && !op.Cascade {
		referring := make([]string, 0, len(targets)-1)
		for _, referrer := range targets[1:] {
			referring = append(referring, referrer.name)
		}
		return nil, &Error{
			Failure:        FailureDeleteReferenced,
			OperationIndex: i,
			Referring:      referring,
			Message: op.Target + " is referenced by " + strings.Join(referring, ", ") +
				"; delete it with cascade to remove those declarations",
		}
	}
	out := make([]splice, 0, len(targets))
	for _, target := range targets {
		out = append(out, splice{span: m.deleteSpan(target), opIndex: i, target: target.name})
	}
	return out, nil
}

// reachesDeletion reports a reference segment that reaches one of targets or a
// declaration inside one; own names the document the targets are declared in.
func (m Model) reachesDeletion(targets []deletion) referenceTest {
	own := m.Source.Name()
	return func(r *resolve.Resolver, ref resolve.Reference, part int) bool {
		seg, ok := r.PartSymbol(ref.QN, part)
		return ok && coveredByAny(own, seg, targets)
	}
}

// deletion is one declaration of this document a delete removes: a symbol's,
// or one no symbol stands for — an import or a filter, which has a span to
// remove all the same.
type deletion struct {
	node ast.Node
	span source.Span
	// sym is nil for a declaration no symbol stands for.
	sym  *symbols.Symbol
	name string
}

// symbolDeletion is the deletion of sym, declared in doc, or false for a symbol
// declared by no node of its own. An anonymous declaration is named by its
// heading when doc is this document, else by its keyword, each in its namespace.
func (m Model) symbolDeletion(r *resolve.Resolver, doc string, sym *symbols.Symbol) (deletion, bool) {
	if sym == nil || sym.Decl == nil {
		return deletion{}, false
	}
	span := symbolSpan(sym)
	name := notationName(sym)
	if sym.Name == "" {
		name = anonymousName(sym)
		if doc == m.Source.Name() {
			name = m.heading(span)
		}
		name += " in " + m.namespaceName(doc, sym.OwnerScope)
	}
	return deletion{node: sym.Decl, span: span, sym: sym, name: name}, true
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
func (d deletion) covers(doc string, sym *symbols.Symbol) bool {
	return sym != nil && sym.DocName == doc && encloses(d.span, symbolSpan(sym))
}

// coveredBy reports whether d lies inside one of set, or is one of them.
func (d deletion) coveredBy(set []deletion) bool {
	for _, other := range set {
		if encloses(other.span, d.span) {
			return true
		}
	}
	return false
}

// cascadeTargets is sym followed by every declaration removing it would leave
// dangling: referrers of sym or of anything it contains, then their referrers,
// until none remain.
func (m Model) cascadeTargets(sym *symbols.Symbol) []deletion {
	r, _ := m.resolver()
	first, ok := m.symbolDeletion(r, m.Source.Name(), sym)
	if !ok {
		return nil
	}
	targets := []deletion{first}
	seen := map[ast.Node]bool{first.node: true}
	for frontier := targets; len(frontier) > 0; {
		var next []deletion
		for _, referrer := range m.referrers(r, targets, frontier) {
			if !seen[referrer.node] {
				seen[referrer.node] = true
				next = append(next, referrer)
			}
		}
		targets = append(targets, next...)
		frontier = next
	}
	return withoutNested(targets)
}

// withoutNested drops every deletion lying inside another's span, which
// removes it already.
func withoutNested(targets []deletion) []deletion {
	out := make([]deletion, 0, len(targets))
	for _, d := range targets {
		nested := false
		for _, other := range targets {
			if other.node != d.node && encloses(other.span, d.span) {
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

// referrers returns the declarations of this document outside every target that
// refer to one of frontier or to a declaration inside one, sorted by name then
// position. Same-named declarations are distinct referrers.
func (m Model) referrers(r *resolve.Resolver, targets, frontier []deletion) []deletion {
	doc := m.Source.Name()
	rootScope := m.Index.DocumentRoot(doc)
	if rootScope == nil {
		return nil
	}
	seen := map[ast.Node]bool{}
	var out []deletion
	for _, ref := range resolve.References(m.Root, rootScope) {
		if ref.QN == nil {
			continue
		}
		r.ResolveReference(ref)
		for i, part := range ref.QN.Parts {
			seg, ok := r.PartSymbol(ref.QN, i)
			if !ok || part.Span == seg.NameSpan || !coveredByAny(doc, seg, frontier) {
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

// coveredByAny reports whether sym is declared inside one of set.
func coveredByAny(doc string, sym *symbols.Symbol, set []deletion) bool {
	for _, d := range set {
		if d.covers(doc, sym) {
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
		return deletion{node: member, span: member.Span(),
			name: importName(member) + " in " + m.namespaceName(doc, ref.Scope)}, true
	case *ast.FilterMember:
		return deletion{node: member, span: member.Span(),
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

// referenceTest reports whether segment part of a resolved ref concerns an edit.
type referenceTest func(r *resolve.Resolver, ref resolve.Reference, part int) bool

// refuseReferencedElsewhere is the refusal of operation i when a declaration of
// another workspace document writes a reference concerns finds: an edit rewrites
// one document, so those references could not follow. Nil when none does.
func (m Model) refuseReferencedElsewhere(i int, op Operation, concerns referenceTest) error {
	elsewhere := m.referredFromElsewhere(concerns)
	if len(elsewhere) == 0 {
		return nil
	}
	return &Error{
		Failure:        FailureReferencedElsewhere,
		OperationIndex: i,
		Referring:      elsewhere,
		Message: op.Target + " is referenced from other documents by " +
			strings.Join(elsewhere, ", ") + "; an edit rewrites one document, so " +
			"change those references first",
	}
}

// referredFromElsewhere names the declarations of the other workspace documents
// writing a reference concerns finds, each with its document, in document then
// name order.
func (m Model) referredFromElsewhere(concerns referenceTest) []string {
	own := m.Source.Name()
	r, _ := m.resolver()
	var out []string
	for _, doc := range m.Index.WorkspaceDocuments() {
		if doc == own {
			continue
		}
		rootScope := m.Index.DocumentRoot(doc)
		if rootScope == nil {
			continue
		}
		root, ok := rootScope.Node().(*ast.RootNamespace)
		if !ok {
			continue
		}
		seen := map[ast.Node]bool{}
		var names []string
		for _, ref := range resolve.References(root, rootScope) {
			if ref.QN == nil {
				continue
			}
			r.ResolveReference(ref)
			for i, part := range ref.QN.Parts {
				if !concerns(r, ref, i) {
					continue
				}
				referrer, ok := m.referrer(r, doc, ref, part.Span.Offset)
				if !ok || seen[referrer.node] {
					continue
				}
				seen[referrer.node] = true
				names = append(names, referrer.name+" ("+doc+")")
			}
		}
		sort.Strings(names)
		out = append(out, names...)
	}
	return out
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
	// A declaration on its own line owns that line, including its newline.
	if onlyWhitespace(content[lineStart:start]) && onlyWhitespace(content[end:lineEnd]) {
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

func onlyWhitespace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}
