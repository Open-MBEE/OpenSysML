package main

import (
	"fmt"
	"strings"
)

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	// tokLiteral is a single-quoted keyword or punctuation literal, unquoted and
	// unescaped.
	tokLiteral
	// tokString is a double-quoted string, as used by grammar imports.
	tokString
	tokPunct
)

type token struct {
	kind tokenKind
	text string
	line int
}

// multiPunct are the multi-character operators of Xtext's own notation, longest
// first so that "::" is not scanned as two ":".
var multiPunct = []string{"::", "+=", "?=", "->", "=>", ".."}

// scanXtext tokenises an Xtext grammar, dropping comments and whitespace.
func scanXtext(src string) ([]token, error) {
	var toks []token
	line := 1
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			line++
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			end := strings.Index(src[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated comment", line)
			}
			line += strings.Count(src[i:i+2+end+2], "\n")
			i += 2 + end + 2
		case c == '\'' || c == '"':
			text, width, err := scanQuoted(src[i:], c)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			kind := tokLiteral
			if c == '"' {
				kind = tokString
			}
			toks = append(toks, token{kind: kind, text: text, line: line})
			line += strings.Count(src[i:i+width], "\n")
			i += width
		case isIdentStart(c):
			j := i
			for j < len(src) && isIdentPart(src[j]) {
				j++
			}
			toks = append(toks, token{kind: tokIdent, text: src[i:j], line: line})
			i = j
		default:
			if op := matchMultiPunct(src[i:]); op != "" {
				toks = append(toks, token{kind: tokPunct, text: op, line: line})
				i += len(op)
				continue
			}
			toks = append(toks, token{kind: tokPunct, text: string(c), line: line})
			i++
		}
	}
	return append(toks, token{kind: tokEOF, line: line}), nil
}

func matchMultiPunct(s string) string {
	for _, op := range multiPunct {
		if strings.HasPrefix(s, op) {
			return op
		}
	}
	return ""
}

// scanQuoted reads a quoted run starting at s[0], returning the unescaped
// content and how many bytes it occupied.
func scanQuoted(s string, quote byte) (string, int, error) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 >= len(s) {
				return "", 0, fmt.Errorf("unterminated escape")
			}
			i++
			b.WriteByte(unescape(s[i]))
		case quote:
			return b.String(), i + 1, nil
		default:
			b.WriteByte(s[i])
		}
	}
	return "", 0, fmt.Errorf("unterminated %c-quoted text", quote)
}

func unescape(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 't':
		return '\t'
	case 'r':
		return '\r'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	}
	return c
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}
