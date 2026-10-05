package grammar

import "strings"

// LocateLiteral returns the byte offset of a literal's first occurrence on a
// 1-based source line, using the same stripping and matching rules as the
// coverage index.
func LocateLiteral(content string, line int, literal string) (int, bool) {
	if line < 1 || literal == "" {
		return 0, false
	}
	stripped := stripModelSource(content)
	start := 0
	for current := 1; current < line; current++ {
		next := strings.IndexByte(stripped[start:], '\n')
		if next < 0 {
			return 0, false
		}
		start += next + 1
	}
	end := strings.IndexByte(stripped[start:], '\n')
	if end < 0 {
		end = len(stripped) - start
	}
	text := stripped[start : start+end]
	if isWordLiteral(literal) {
		for at := 0; at < len(text); {
			if !isIdentStart(text[at]) {
				at++
				continue
			}
			wordStart := at
			for at < len(text) && isIdentPart(text[at]) {
				at++
			}
			if text[wordStart:at] == literal {
				return start + wordStart, true
			}
		}
		return 0, false
	}
	at := strings.Index(text, literal)
	if at < 0 {
		return 0, false
	}
	return start + at, true
}
