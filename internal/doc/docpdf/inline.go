package docpdf

import (
	"html"
	"strings"
)

// This file renders docrender's inline dialect to HTML: escaped prose,
// emphasis and strong spans, code spans, inline links, reference links to
// in-document anchors, and dollar-delimited inline math. Anything malformed
// stays literal text.

// inlineHTML renders one line of the inline dialect as HTML element content,
// each inline formula shown as math typesets it.
func inlineHTML(text string, math formulas) string {
	var b strings.Builder
	for _, seg := range splitMath(text) {
		if seg.math {
			b.WriteString(`<span class="math">` + math.typeset(formula{Source: seg.text}) + "</span>")
			continue
		}
		b.WriteString(proseHTML(seg.text))
	}
	return b.String()
}

// segment is one stretch of an inline line: prose in the dialect, or the
// LaTeX source of one inline formula.
type segment struct {
	text string
	math bool
}

// splitMath cuts a line at its inline formulas. A dollar opens one unless
// escaped or inside a code span or link; it runs to the next unescaped
// dollar, and a dollar with no closing one stays prose.
func splitMath(text string) []segment {
	var segs []segment
	start := 0
	for i := 0; i < len(text); {
		switch text[i] {
		case '\\':
			i += 2
		case '`':
			fence := backtickRun(text, i)
			if _, next, ok := codeSpanBody(text, i+fence, fence); ok {
				i = next
				continue
			}
			i += fence
		case '[':
			if _, _, next, ok := linkAt(text, i); ok {
				i = next
				continue
			}
			i++
		case '$':
			source, next, ok := mathAt(text, i)
			if !ok {
				i++
				continue
			}
			if i > start {
				segs = append(segs, segment{text: text[start:i]})
			}
			segs = append(segs, segment{text: source, math: true})
			start, i = next, next
		default:
			i++
		}
	}
	if start < len(text) || len(segs) == 0 {
		segs = append(segs, segment{text: text[start:]})
	}
	return segs
}

// mathAt parses an inline formula opened by the dollar at i, returning its
// LaTeX source and the index after the closing dollar. The source keeps its
// backslash escapes, which are LaTeX's own, and may not be empty.
func mathAt(text string, i int) (source string, next int, ok bool) {
	for j := i + 1; j < len(text); j++ {
		switch text[j] {
		case '\\':
			j++
		case '$':
			if j == i+1 {
				return "", 0, false
			}
			return text[i+1 : j], j + 1, true
		}
	}
	return "", 0, false
}

// proseHTML renders one stretch of the inline dialect holding no math.
func proseHTML(text string) string {
	var b strings.Builder
	var literal strings.Builder
	flush := func() {
		b.WriteString(literalHTML(literal.String()))
		literal.Reset()
	}
	for i := 0; i < len(text); {
		switch text[i] {
		case '\\':
			literal.WriteByte(text[i])
			if i+1 < len(text) {
				literal.WriteByte(text[i+1])
				i += 2
				continue
			}
			i++
		case '`':
			fence := backtickRun(text, i)
			body, next, ok := codeSpanBody(text, i+fence, fence)
			if !ok {
				literal.WriteString(text[i : i+fence])
				i += fence
				continue
			}
			flush()
			b.WriteString("<code>" + html.EscapeString(body) + "</code>")
			i = next
		case '*':
			marker := "*"
			if strings.HasPrefix(text[i:], "**") {
				marker = "**"
			}
			end := closingMarker(text, i+len(marker), marker)
			if end < 0 {
				literal.WriteString(marker)
				i += len(marker)
				continue
			}
			flush()
			tag := "em"
			if marker == "**" {
				tag = "strong"
			}
			b.WriteString("<" + tag + ">" + literalHTML(text[i+len(marker):end]) + "</" + tag + ">")
			i = end + len(marker)
		case '[':
			label, href, next, ok := linkAt(text, i)
			if !ok {
				literal.WriteByte(text[i])
				i++
				continue
			}
			flush()
			b.WriteString(`<a href="` + html.EscapeString(href) + `">` + literalHTML(label) + "</a>")
			i = next
		default:
			literal.WriteByte(text[i])
			i++
		}
	}
	flush()
	return b.String()
}

// literalHTML unescapes a stretch of escaped prose and escapes it for HTML.
func literalHTML(text string) string {
	return html.EscapeString(unescape(text))
}

// backtickRun counts the consecutive backticks starting at i.
func backtickRun(text string, i int) int {
	n := 0
	for i+n < len(text) && text[i+n] == '`' {
		n++
	}
	return n
}

// codeSpanBody finds the closing fence of exactly the opening length and
// returns the span's content — with the single space of padding stripped
// from each end when both are present, undoing what codeSpan added — and
// the index after the closing fence.
func codeSpanBody(text string, start, fence int) (string, int, bool) {
	for j := start; j < len(text); j++ {
		if text[j] != '`' {
			continue
		}
		run := backtickRun(text, j)
		if run == fence {
			body := text[start:j]
			if len(body) >= 2 && body[0] == ' ' && body[len(body)-1] == ' ' {
				body = body[1 : len(body)-1]
			}
			return body, j + run, true
		}
		j += run - 1
	}
	return "", 0, false
}

// closingMarker finds the next unescaped emphasis marker from start, or -1.
func closingMarker(text string, start int, marker string) int {
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '*':
			if strings.HasPrefix(text[i:], marker) {
				return i
			}
		}
	}
	return -1
}

// linkAt parses an inline link at i: "[label](<destination>)" for a URL with
// the pointy-bracket escapes undone, or "[label](#anchor)" for an
// in-document reference kept as the fragment href.
func linkAt(text string, i int) (label, href string, next int, ok bool) {
	end := closingBracket(text, i+1)
	if end < 0 || end+1 >= len(text) || text[end+1] != '(' {
		return "", "", 0, false
	}
	label = text[i+1 : end]
	rest := text[end+2:]
	if strings.HasPrefix(rest, "<") {
		dest, after, closed := destinationBody(rest[1:])
		if !closed || after >= len(rest) || rest[after] != ')' {
			return "", "", 0, false
		}
		return label, dest, end + 2 + after + 1, true
	}
	if strings.HasPrefix(rest, "#") {
		closeAt := strings.IndexByte(rest, ')')
		if closeAt < 0 {
			return "", "", 0, false
		}
		return label, rest[:closeAt], end + 2 + closeAt + 1, true
	}
	return "", "", 0, false
}

// closingBracket finds the unescaped "]" ending a link label, or -1.
func closingBracket(text string, start int) int {
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case ']':
			return i
		}
	}
	return -1
}

// destinationBody reads a pointy-bracket destination after its opening "<",
// undoing the backslash escapes of backslashes and angle brackets, and
// returns the index just past the closing ">" relative to that "<".
func destinationBody(text string) (string, int, bool) {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\\':
			if i+1 < len(text) {
				b.WriteByte(text[i+1])
				i++
				continue
			}
			b.WriteByte('\\')
		case '>':
			return b.String(), i + 2, true
		default:
			b.WriteByte(text[i])
		}
	}
	return "", 0, false
}
