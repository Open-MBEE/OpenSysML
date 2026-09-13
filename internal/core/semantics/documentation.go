package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
	return lexer.CommentBody(m.sourceText(sym.DocName, span))
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
