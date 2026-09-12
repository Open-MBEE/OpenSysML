package lexer

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// IsEscapeChar reports whether c may follow a backslash in a STRING_VALUE or
// UNRESTRICTED_NAME (KerMLExpressions.xtext:562-566): b t n f r " ' \.
func IsEscapeChar(c byte) bool {
	switch c {
	case 'b', 't', 'n', 'f', 'r', '"', '\'', '\\':
		return true
	}
	return false
}

// InvalidEscapes locates the backslash escapes in the quoted token text raw
// (spanning tok) that its terminal does not admit, each span covering the
// backslash and the character after it.
func InvalidEscapes(tok source.Span, raw string) []source.Span {
	var spans []source.Span
	for i := 0; i+1 < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		if !IsEscapeChar(raw[i+1]) {
			spans = append(spans, source.Span{Offset: tok.Offset + i, Len: 2})
		}
		i++
	}
	return spans
}

// StringValue reads the text a STRING_VALUE token spells: the quotes come off
// and the backslash escapes KerML §8.2.2 defines stand for the characters they
// name. A backslash before anything else (rejected by the scanner) stands for
// that character itself.
func StringValue(raw string) string {
	if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '\'') && raw[len(raw)-1] == raw[0] {
		raw = raw[1 : len(raw)-1]
	}
	if !strings.ContainsRune(raw, '\\') {
		return raw
	}
	var text strings.Builder
	text.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' || i+1 == len(raw) {
			text.WriteByte(raw[i])
			continue
		}
		i++
		switch raw[i] {
		case 'b':
			text.WriteByte('\b')
		case 't':
			text.WriteByte('\t')
		case 'n':
			text.WriteByte('\n')
		case 'f':
			text.WriteByte('\f')
		case 'r':
			text.WriteByte('\r')
		default:
			text.WriteByte(raw[i])
		}
	}
	return text.String()
}

// StringText writes a value as the double-quoted STRING_VALUE token that
// StringValue reads back to it, escaping the quote, the backslash and the
// control characters KerML §8.2.2 names; everything else stands as it is.
func StringText(value string) string { return quotedText(value, '"') }

// UnrestrictedNameText writes raw text as the quoted unrestricted name whose
// value is that text, escaping the quote, the backslash and the control characters.
func UnrestrictedNameText(value string) string { return quotedText(value, '\'') }

// quotedText writes value between quote characters, escaping so it lexes as one token.
func quotedText(value string, quote byte) string {
	var text strings.Builder
	text.Grow(len(value) + 2)
	text.WriteByte(quote)
	for i := 0; i < len(value); i++ {
		switch c := value[i]; c {
		case quote, '\\':
			text.WriteByte('\\')
			text.WriteByte(c)
		case '\b':
			text.WriteString(`\b`)
		case '\t':
			text.WriteString(`\t`)
		case '\n':
			text.WriteString(`\n`)
		case '\f':
			text.WriteString(`\f`)
		case '\r':
			text.WriteString(`\r`)
		default:
			text.WriteByte(c)
		}
	}
	text.WriteByte(quote)
	return text.String()
}
