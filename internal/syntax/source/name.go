package source

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

// MemberPathOf writes the segments of a member path, outermost first, joined
// by '.', each quoted on its own like a qualified name segment.
func MemberPathOf(names []string) string {
	segments := make([]string, len(names))
	for i, name := range names {
		segments[i] = NameText(name)
	}
	return strings.Join(segments, ".")
}

// MemberPathSegments reads a member path — `.`-joined names, each a basic or
// a 'quoted name' — back into its names, quotes dropped and escapes kept;
// false for malformed text. A bare name is a one-segment path.
func MemberPathSegments(text string) ([]string, bool) {
	var names []string
	for {
		name, rest, ok := readBasicOrQuotedName(text)
		if !ok {
			return nil, false
		}
		names = append(names, name)
		if rest == "" {
			return names, true
		}
		if !strings.HasPrefix(rest, ".") || rest == "." {
			return nil, false
		}
		text = rest[1:]
	}
}

// ReferenceEndNames rewrites a typing's references — `, ` apart, each a
// qualified name or feature chain, `$::` led or `~` conjugated — by the name
// each ends in, `~` kept; text that does not read as such is returned as it is.
func ReferenceEndNames(text string) string {
	var ends []string
	rest := text
	for {
		conjugated := strings.HasPrefix(rest, "~")
		rest = strings.TrimPrefix(strings.TrimPrefix(rest, "~"), "$::")
		var end string
		for {
			name, after, ok := readBasicOrQuotedName(rest)
			if !ok {
				return text
			}
			end, rest = name, after
			if strings.HasPrefix(rest, "::") {
				rest = rest[2:]
			} else if strings.HasPrefix(rest, ".") {
				rest = rest[1:]
			} else {
				break
			}
		}
		end = NameText(end)
		if conjugated {
			end = "~" + end
		}
		ends = append(ends, end)
		if rest == "" {
			return strings.Join(ends, ", ")
		}
		if !strings.HasPrefix(rest, ", ") {
			return text
		}
		rest = rest[2:]
	}
}

// ReferenceQualifiedNames reads a typing's references — `, ` apart, `$::` led or
// `~` conjugated — into each one's names; a feature chain, or text that does not
// read as references, contributes none.
func ReferenceQualifiedNames(text string) [][]string {
	var refs [][]string
	rest := text
	for {
		rest = strings.TrimPrefix(strings.TrimPrefix(rest, "~"), "$::")
		var names []string
		chain := false
		for {
			name, after, ok := readBasicOrQuotedName(rest)
			if !ok {
				return nil
			}
			names, rest = append(names, name), after
			if strings.HasPrefix(rest, "::") {
				rest = rest[2:]
			} else if strings.HasPrefix(rest, ".") {
				rest, chain = rest[1:], true
			} else {
				break
			}
		}
		if !chain {
			refs = append(refs, names)
		}
		if rest == "" {
			return refs
		}
		if !strings.HasPrefix(rest, ", ") {
			return nil
		}
		rest = rest[2:]
	}
}

// readName reads one name off the front of text: a quoted one up to its closing
// quote, else a bare one up to `::`.
func readName(text string) (name, rest string, ok bool) {
	if text == "" || text[0] == '\'' {
		return readQuotedName(text)
	}
	end := strings.Index(text, "::")
	if end < 0 {
		end = len(text)
	}
	name = text[:end]
	return name, text[end:], name != "" && !strings.Contains(name, "'")
}

// readBasicOrQuotedName reads one name off the front of text: a quoted one up to
// its closing quote, else a basic name up to the first byte that cannot continue it.
func readBasicOrQuotedName(text string) (name, rest string, ok bool) {
	if text == "" || text[0] == '\'' {
		return readQuotedName(text)
	}
	end := 0
	for end < len(text) && IsIdentCont(text[end]) {
		end++
	}
	name = text[:end]
	return name, text[end:], IsIdentifier(name)
}

// readQuotedName reads a quoted name off the front of text, escapes kept.
func readQuotedName(text string) (name, rest string, ok bool) {
	if text == "" || text[0] != '\'' {
		return "", "", false
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
