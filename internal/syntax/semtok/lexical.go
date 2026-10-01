// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package semtok

import (
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Lexical classifies keywords, comments and literals, read from the same lexer
// the parser uses rather than recognized again, in a file of the given kind:
// the tokens a document carries with no symbol table at all.
func Lexical(content []byte, kind source.Kind) []Token {
	if len(content) == 0 {
		return nil
	}
	lx := lexer.New(source.NewWithKind("", content, kind))
	var out []Token
	for {
		tok := lx.Next()
		if tok.Kind == lexer.EOF {
			return out
		}
		var class Class
		switch tok.Kind {
		case lexer.Keyword:
			if !parser.Reserves(kind, tok.KeywordID) {
				continue
			}
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
