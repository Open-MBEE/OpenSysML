// Package lexer implements a hand-written scanner for SysML v2 and KerML.
//
// The lexer tokenizes source text into a stream of tokens with full position tracking.
// It handles the ~200 SysML keywords, multi-character operators, string/number/infinity literals,
// comments (both line and block), and whitespace (tracked as leading/trailing trivia).
//
// Key types:
//   - Token: a single lexical unit (kind, text, span, trivia)
//   - TokenKind: categorizes tokens (Keyword, Identifier, Number, String, Operator, etc.)
//   - Lexer: stateful scanner with Next() method
//
// The lexer supports error recovery: invalid characters produce ErrorToken but scanning continues.
// All keywords are case-sensitive and pre-registered in keywords.go.
package lexer
