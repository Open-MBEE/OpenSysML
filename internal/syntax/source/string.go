package source

import "strings"

// escape is one backslash escape the lexer admits in a STRING_VALUE or UNRESTRICTED_NAME:
// the letter written, the character it names, and whether KerML §8.2.2.3 Table 4 lists it.
type escape struct {
	letter, value byte
	standard      bool
}

// escapes is the one table of admitted escapes: Table 4's seven, and `\r`,
// which only the pilot grammar adds (KerMLExpressions.xtext:562-566).
var escapes = [...]escape{
	{'b', '\b', true},
	{'t', '\t', true},
	{'n', '\n', true},
	{'f', '\f', true},
	{'r', '\r', false},
	{'"', '"', true},
	{'\'', '\'', true},
	{'\\', '\\', true},
}

// escapeByLetter and escapeByValue index escapes by the letter written and by
// the character named; a zero letter marks a byte that is neither.
var escapeByLetter, escapeByValue = func() (byLetter, byValue [256]escape) {
	for _, e := range escapes {
		byLetter[e.letter], byValue[e.value] = e, e
	}
	return byLetter, byValue
}()

// IsEscapeChar reports whether c may follow a backslash in a STRING_VALUE or
// UNRESTRICTED_NAME: Table 4's letters and the pilot grammar's `r`.
func IsEscapeChar(c byte) bool { return escapeByLetter[c].letter != 0 }

// IsStandardEscapeChar reports whether backslash-c is an escape sequence of
// KerML §8.2.2.3 Table 4, which §8.2.2.5 gives STRING_VALUE too.
func IsStandardEscapeChar(c byte) bool { return escapeByLetter[c].standard }

// EscapeLetter is the letter that, after a backslash, names c; ok is false
// for a character no admitted escape names.
func EscapeLetter(c byte) (letter byte, ok bool) {
	e := escapeByValue[c]
	return e.letter, e.letter != 0
}

// AdmittedEscapes spells the escape sequences the lexer admits, in table order.
func AdmittedEscapes() string { return escapeList(false) }

// StandardEscapes spells the escape sequences of KerML §8.2.2.3 Table 4.
func StandardEscapes() string { return escapeList(true) }

// escapeList spells the table's escapes separated by blanks, the standard ones alone on request.
func escapeList(standardOnly bool) string {
	var list strings.Builder
	for _, e := range escapes {
		if standardOnly && !e.standard {
			continue
		}
		if list.Len() > 0 {
			list.WriteByte(' ')
		}
		list.WriteByte('\\')
		list.WriteByte(e.letter)
	}
	return list.String()
}

// StringValue reads the text a STRING_VALUE token spells: the quotes come off
// and the backslash escapes KerML §8.2.2 defines stand for the characters they
// name. A backslash before anything else (rejected by the scanner) stands for
// that character itself, and a token the scanner reported unterminated has
// only its opening quote to come off.
func StringValue(raw string) string {
	if len(raw) >= 1 && (raw[0] == '"' || raw[0] == '\'') {
		if len(raw) >= 2 && raw[len(raw)-1] == raw[0] {
			raw = raw[1 : len(raw)-1]
		} else {
			raw = raw[1:]
		}
	}
	return Unescape(raw)
}

// Unescape reads the backslash escapes KerML §8.2.2 defines as the characters
// they name; a backslash before anything else stands for that character
// itself, and text with no backslash stands as it is.
func Unescape(raw string) string {
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
		if e := escapeByLetter[raw[i]]; e.letter != 0 {
			text.WriteByte(e.value)
		} else {
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
		c := value[i]
		// The other quote is a character of the text, written as it is.
		if e := escapeByValue[c]; e.letter != 0 && (c == quote || (c != '"' && c != '\'')) {
			text.WriteByte('\\')
			text.WriteByte(e.letter)
		} else {
			text.WriteByte(c)
		}
	}
	text.WriteByte(quote)
	return text.String()
}
