package lexer

import "github.com/Open-MBEE/OpenSysML/internal/syntax/source"

// keywords maps each KerML or SysML keyword to its canonical string, so a
// scanned Keyword token shares that string instead of allocating its own.
var keywords = map[string]string{}

func init() {
	for _, kw := range source.Keywords() {
		keywords[kw] = kw
	}
}

// InvalidEscapes locates the backslash escapes in the quoted token text raw
// (spanning tok) that its terminal does not admit, each span covering the
// backslash and the character after it.
func InvalidEscapes(tok source.Span, raw string) []source.Span {
	return escapeSpans(tok, raw, func(c byte) bool { return !source.IsEscapeChar(c) })
}

// NonstandardEscapes locates the escapes in raw the lexer admits beyond KerML
// §8.2.2.3 Table 4, each span covering the backslash and the letter after it.
func NonstandardEscapes(tok source.Span, raw string) []source.Span {
	return escapeSpans(tok, raw, func(c byte) bool { return source.IsEscapeChar(c) && !source.IsStandardEscapeChar(c) })
}

// escapeSpans locates the backslash escapes of raw whose letter satisfies match.
func escapeSpans(tok source.Span, raw string, match func(byte) bool) []source.Span {
	var spans []source.Span
	for i := 0; i+1 < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		if match(raw[i+1]) {
			spans = append(spans, source.Span{Offset: tok.Offset + i, Len: 2})
		}
		i++
	}
	return spans
}
