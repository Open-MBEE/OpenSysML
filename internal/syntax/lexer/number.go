package lexer

import "github.com/Open-MBEE/OpenSysML/internal/syntax/source"

// IsDecimalValue reports whether s is exactly one DECIMAL_VALUE token (KerML
// §8.2.2): digits alone, so no sign, point or exponent.
func IsDecimalValue(s string) bool {
	return isNumberToken(s, Decimal)
}

// IsRealValue reports whether s is exactly one REAL_VALUE token (KerML §8.2.2):
// an unsigned number with a fractional part or an exponent, as `1.5`, `.5`, `1e3`.
func IsRealValue(s string) bool {
	return isNumberToken(s, Real)
}

func isNumberToken(s string, kind Kind) bool {
	tok := New(source.New("", []byte(s))).Next()
	return tok.Kind == kind && tok.Span.Len == len(s)
}
