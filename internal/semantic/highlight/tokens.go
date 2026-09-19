package highlight

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// SegmentResolver answers what the names of a document refer to. Without one,
// only lexical and declared names are classified.
type SegmentResolver interface {
	// SegmentSymbols resolves one reference and returns the symbol each segment
	// of its qualified name denotes, nil where a segment did not resolve.
	SegmentSymbols(ref resolve.Reference) []*symbols.Symbol
}

// Tokens classifies a document into semantic tokens ordered by position and free
// of overlap, from the lexer, the symbol table and the resolver.
func Tokens(content []byte, root *ast.RootNamespace, scope *symbols.Scope, res SegmentResolver) []Token {
	var (
		semantic []Token
		lexical  = lexicalTokens(content)
	)
	semantic = append(semantic, declarationTokens(scope)...)
	semantic = append(semantic, referenceTokens(root, scope, res)...)
	return merge(semantic, lexical)
}

// lexicalTokens classifies keywords, comments and literals, read from the same
// lexer the parser uses rather than recognized again.
func lexicalTokens(content []byte) []Token {
	if len(content) == 0 {
		return nil
	}
	lx := lexer.New(source.New("", content))
	var out []Token
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return out
		}
		var class Class
		switch tok.Kind {
		case lexer.Keyword:
			class = ClassKeyword
		case lexer.SLNote, lexer.MLNote, lexer.RegularComment:
			class = ClassComment
		case lexer.String:
			class = ClassString
		case lexer.Decimal, lexer.Real:
			class = ClassNumber
		default:
			continue
		}
		if sp := trimEOL(content, tok.Span); sp.Len > 0 {
			out = append(out, Token{Span: sp, Class: class})
		}
	}
}

// trimEOL drops the line terminator a line comment's span ends with, which is
// not part of what is highlighted.
func trimEOL(content []byte, sp source.Span) source.Span {
	for sp.Len > 0 {
		switch content[sp.End()-1] {
		case '\n', '\r':
			sp.Len--
		default:
			return sp
		}
	}
	return sp
}

// declarationTokens classifies the declared name of every symbol in a document's
// scope tree.
func declarationTokens(scope *symbols.Scope) []Token {
	if scope == nil {
		return nil
	}
	var out []Token
	var walk func(*symbols.Scope)
	walk = func(s *symbols.Scope) {
		for _, sym := range s.AllMembers() {
			// An anonymous member has no name; a borrowed one is highlighted at
			// the reference it came from.
			if sym == nil || sym.EffectiveName() || sym.NameSpan.Len == 0 {
				continue
			}
			class, mods := classify(sym)
			out = append(out, Token{Span: sym.NameSpan, Class: class, Modifiers: mods})
		}
		for _, child := range s.Children() {
			walk(child)
		}
	}
	walk(scope)
	return out
}

// referenceTokens classifies each segment of every reference as what it denotes.
// An unresolved segment yields no token; it carries a diagnostic instead.
func referenceTokens(root *ast.RootNamespace, scope *symbols.Scope, res SegmentResolver) []Token {
	if res == nil {
		return nil
	}
	var out []Token
	for _, ref := range resolve.References(root, scope) {
		if ref.QN == nil {
			continue
		}
		syms := res.SegmentSymbols(ref)
		for i, part := range ref.QN.Parts {
			if i >= len(syms) || syms[i] == nil || part.Span.Len == 0 {
				continue
			}
			class, mods := referenceClass(syms[i])
			out = append(out, Token{Span: part.Span, Class: class, Modifiers: mods})
		}
	}
	return out
}

// merge orders tokens by position and drops overlaps, semantics winning: a name
// spelled as an unrestricted name or a keyword is highlighted for what it means.
func merge(semantic, lexical []Token) []Token {
	type entry struct {
		tok      Token
		semantic bool
	}
	all := make([]entry, 0, len(semantic)+len(lexical))
	for _, t := range semantic {
		all = append(all, entry{tok: t, semantic: true})
	}
	for _, t := range lexical {
		all = append(all, entry{tok: t})
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i].tok.Span, all[j].tok.Span
		switch {
		case a.Offset != b.Offset:
			return a.Offset < b.Offset
		case all[i].semantic != all[j].semantic:
			return all[i].semantic
		default:
			return a.Len > b.Len
		}
	})
	out := make([]Token, 0, len(all))
	end := 0
	for _, e := range all {
		if e.tok.Span.Offset < end {
			continue
		}
		out = append(out, e.tok)
		end = e.tok.Span.End()
	}
	return out
}
