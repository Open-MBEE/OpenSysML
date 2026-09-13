package lexer

import "strings"

// NameText writes a name as the notation spells it: a name that is not a basic
// name — one holding a space or punctuation, one spelling a keyword, an empty
// one — gets the quotes of an unrestricted name back (KerML §8.2.2). The text
// is quoted as it stands, since a name parsed from the notation still carries
// the escapes it was written with, so quoting is the exact inverse of parsing.
func NameText(name string) string {
	if IsIdentifier(name) && !IsKeyword(name) {
		return name
	}
	return "'" + name + "'"
}

// QualifiedNameText writes a qualified name segment by segment, since each
// segment is a name of its own and is quoted on its own. An empty name is
// written as it stands.
func QualifiedNameText(fqn string) string {
	if fqn == "" {
		return fqn
	}
	return QualifiedNameOf(strings.Split(fqn, "::"))
}

// QualifiedNameOf writes names, outermost first, as one qualified name, each
// quoted on its own; a name may hold `::`, so the text is read back exactly.
func QualifiedNameOf(names []string) string {
	segments := make([]string, len(names))
	for i, name := range names {
		segments[i] = NameText(name)
	}
	return strings.Join(segments, "::")
}

// QualifiedNameSegments reads a qualified name back into its names, `'x::y'` one
// and `x::y` two, quotes dropped and escapes kept; false for malformed text.
func QualifiedNameSegments(text string) ([]string, bool) {
	var names []string
	for {
		name, rest, ok := readName(text)
		if !ok {
			return nil, false
		}
		names = append(names, name)
		if rest == "" {
			return names, true
		}
		if !strings.HasPrefix(rest, "::") {
			return nil, false
		}
		text = rest[2:]
	}
}

// readName reads one name off the front of text: a quoted one up to its closing
// quote, else a bare one up to `::`.
func readName(text string) (name, rest string, ok bool) {
	if text == "" {
		return "", "", false
	}
	if text[0] != '\'' {
		end := strings.Index(text, "::")
		if end < 0 {
			end = len(text)
		}
		name = text[:end]
		return name, text[end:], name != "" && !strings.Contains(name, "'")
	}
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '\'':
			return text[1:i], text[i+1:], i > 1
		}
	}
	return "", "", false
}
