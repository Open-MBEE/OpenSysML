package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// DocumentationOf is Element::documentation (KerML 1.1 §8.2.4): the prose of
// each `doc` the element owns, in declaration order. A body whose notation the
// model cannot read, or that reads as blank, is left out.
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
		if body := m.commentBody(doc, decl.BodySpan); body != "" {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

// commentBody is Comment::body of the comment or documentation sym declares:
// the prose of its comment token, "" when the notation cannot be read.
func (m *Model) commentBody(sym *symbols.Symbol, span source.Span) string {
	if m.sourceText == nil || span.Len == 0 {
		return ""
	}
	return source.CommentBody(m.sourceText(sym.DocName, span))
}

// documentationSymbols lists the `doc` members sym declares, in order, each once.
func (m *Model) documentationSymbols(sym *symbols.Symbol) []*symbols.Symbol {
	if target, ok := m.resolver.ResolveAliasTarget(sym); ok {
		sym = target
	}
	if sym.Scope == nil {
		return nil
	}
	var docs []*symbols.Symbol
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member.Kind == symbols.SymbolDocumentation && !containsSymbol(docs, member) {
			docs = append(docs, member)
		}
		return true
	})
	return docs
}
