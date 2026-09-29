package source

import "strings"

// CommentBody is the body a REGULAR_COMMENT gives its Comment, Documentation or
// TextualRepresentation (KerML 1.1 §8.2.3.3.2 Note 1): the delimiters come off,
// then the white space after `/*` up to and including the first line
// terminator; each later line loses its initial white space, then a `*`, then
// one space. All other text, line breaks and white space stay as entered. A
// `\r\n` line terminator reads as `\n`.
func CommentBody(raw string) string {
	raw = strings.TrimPrefix(raw, "/*")
	raw = strings.TrimSuffix(raw, "*/")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	lines[0] = strings.TrimLeft(lines[0], commentSpace)
	for i, line := range lines[1:] {
		line = strings.TrimLeft(line, commentSpace)
		line = strings.TrimPrefix(line, "*")
		lines[i+1] = strings.TrimPrefix(line, " ")
	}
	if lines[0] == "" && len(lines) > 1 {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n")
}

// commentSpace is the white space other than line terminators (KerML 1.1 §8.2.2.1).
const commentSpace = " \t\f"

// CommentText writes a REGULAR_COMMENT whose CommentBody is body, its later
// lines opening with the ` * ` margin under indent. False for a body no comment
// carries: `*/` would close it, and `\r` is not a line break it reads back.
func CommentText(body, indent string) (string, bool) {
	if strings.Contains(body, "*/") || strings.Contains(body, "\r") {
		return "", false
	}
	if body == "" {
		return "/* */", true
	}
	lines := strings.Split(body, "\n")
	var b strings.Builder
	b.WriteString("/*")
	if lines[0] != "" && strings.TrimLeft(lines[0], commentSpace) == lines[0] {
		b.WriteString(" " + lines[0])
		lines = lines[1:]
	}
	for i, line := range lines {
		switch {
		case i == len(lines)-1 && line == "":
			b.WriteString("\n" + indent + " ")
		case line == "":
			b.WriteString("\n" + indent + " *")
		default:
			b.WriteString("\n" + indent + " * " + line)
		}
	}
	b.WriteString("*/")
	return b.String(), true
}

// CommentProse reads a comment or note token as display prose: the `/* */`,
// `/** */`, `//* */` or `//` delimiters come off, as does each line's
// indentation and the `*` margin a block comment runs down its left edge. Lines
// keep their breaks.
func CommentProse(raw string) string {
	raw = strings.TrimSpace(raw)
	block := false
	for _, open := range []string{"//*", "/*"} {
		if rest, ok := strings.CutPrefix(raw, open); ok {
			raw, block = trimDocOpener(strings.TrimSuffix(rest, "*/")), true
			break
		}
	}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !block {
			line = strings.TrimPrefix(line, "//")
		}
		lines[i] = strings.TrimSpace(line)
	}
	if block && hasMargin(lines[1:]) {
		for i, line := range lines[1:] {
			lines[i+1] = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// trimDocOpener drops the second `*` of a `/**` opener: one glued to the
// delimiter and followed by nothing or whitespace. A `*` that opens text, as in
// `/* *important* */` or `/**bold**`, is the author's and stays.
func trimDocOpener(body string) string {
	rest, ok := strings.CutPrefix(body, "*")
	if !ok || (rest != "" && strings.IndexByte(" \t\r\n", rest[0]) < 0) {
		return body
	}
	return rest
}

// hasMargin reports whether the continuation lines of a block comment all open
// with a margin `*`; a `*` on only some lines, or one glued to text, is the
// author's (a bullet, emphasis) and stays.
func hasMargin(lines []string) bool {
	margin := false
	for _, line := range lines {
		switch {
		case line == "":
		case line == "*", strings.HasPrefix(line, "* "), strings.HasPrefix(line, "*\t"):
			margin = true
		default:
			return false
		}
	}
	return margin
}
