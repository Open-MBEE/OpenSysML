package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"slices"
)

// SetSourceText gives the model the notation its documents were written in,
// which Element::documentation bodies are read from. Nil reads no bodies.
func (m *Model) SetSourceText(text source.Lookup) {
	if m != nil {
		m.sourceText = text
	}
}

// SourceText is the notation lookup the model reads documentation from.
func (m *Model) SourceText() source.Lookup {
	if m == nil {
		return nil
	}
	return m.sourceText
}

// SetSourceFile tells the model which file on disk each of its documents' spans
// was read from, for a document joining several files. Nil: source.FileNamed.
func (m *Model) SetSourceFile(locate source.Locate) {
	if m != nil {
		m.sourceFile = locate
	}
}

// SourceFileOf is the file on disk the declaration at origin was read from, ""
// when it came from none.
func (m *Model) SourceFileOf(origin symbols.Origin) string {
	if !origin.Located() {
		return ""
	}
	if m == nil || m.sourceFile == nil {
		return source.FileNamed(origin.Doc, origin.Span)
	}
	return m.sourceFile(origin.Doc, origin.Span)
}

// DocumentationOf reads Element::documentation (KerML 1.1 §8.2.4) as display
// prose (source.CommentProse), one per `doc` the element owns in declaration
// order; Documentation::body itself is exact. A blank or unreadable body is left out.
func (m *Model) DocumentationOf(sym *symbols.Symbol) []string {
	if m == nil || sym == nil || m.sourceText == nil {
		return nil
	}
	var bodies []string
	for _, doc := range m.documentationSymbols(sym) {
		decl, ok := doc.Decl.(*ast.Documentation)
		if !ok {
			continue
		}
		if body := m.documentationProse(doc, decl.BodySpan); body != "" {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

// TextualRepresentationOf reads Element::textualRepresentation (KerML 1.1
// §8.3.2.3): TextualRepresentation::body of each `rep` the element owns, in
// declaration order and exact. A blank or unreadable body is left out.
func (m *Model) TextualRepresentationOf(sym *symbols.Symbol) []string {
	if m == nil || sym == nil || m.sourceText == nil {
		return nil
	}
	var bodies []string
	for _, rep := range m.annotationSymbols(sym, symbols.SymbolTextualRepresentation) {
		decl, ok := rep.Decl.(*ast.TextualRepresentation)
		if !ok {
			continue
		}
		if body := m.commentBody(rep, decl.BodySpan); body != "" {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

// AnnotationBodyOf is Comment::body or TextualRepresentation::body of the
// comment, documentation or textual representation sym declares, exact
// (source.CommentBody) and "" for an empty one; false for any other element
// or when the notation of its document cannot be read.
func (m *Model) AnnotationBodyOf(sym *symbols.Symbol) (string, bool) {
	if m == nil || sym == nil || m.sourceText == nil {
		return "", false
	}
	var span source.Span
	switch decl := sym.Decl.(type) {
	case *ast.Comment:
		span = decl.BodySpan
	case *ast.Documentation:
		span = decl.BodySpan
	case *ast.TextualRepresentation:
		span = decl.BodySpan
	default:
		return "", false
	}
	// A body token is at least `/**/`, so an empty read is a document the lookup lacks.
	raw := m.sourceText(sym.DocName, span)
	if raw == "" {
		return "", false
	}
	return source.CommentBody(raw), true
}

// commentBody is Comment::body of the comment or documentation sym declares
// (source.CommentBody), "" when the notation cannot be read.
func (m *Model) commentBody(sym *symbols.Symbol, span source.Span) string {
	if m.sourceText == nil || span.Len == 0 {
		return ""
	}
	return source.CommentBody(m.sourceText(sym.DocName, span))
}

func (m *Model) documentationProse(sym *symbols.Symbol, span source.Span) string {
	if m.sourceText == nil || span.Len == 0 {
		return ""
	}
	return source.CommentProse(m.sourceText(sym.DocName, span))
}

// documentationSymbols lists the `doc` members sym declares, in order, each once.
func (m *Model) documentationSymbols(sym *symbols.Symbol) []*symbols.Symbol {
	return m.annotationSymbols(sym, symbols.SymbolDocumentation)
}

// annotationSymbols lists the members of one annotating kind sym declares, in
// order, each once; an alias reads its target's.
func (m *Model) annotationSymbols(sym *symbols.Symbol, kind symbols.SymbolKind) []*symbols.Symbol {
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	if sym.Scope == nil {
		return nil
	}
	var found []*symbols.Symbol
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == kind && !slices.Contains(found, member) {
			found = append(found, member)
		}
		return true
	})
	return found
}
