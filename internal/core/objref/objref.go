// Package objref reads the references every surface names a held object by —
// `#<id>`, a declared name, or either followed by the features walked from it
// (`car.wheels[2]`) — and walks such a path through an object's feature values.
// The REPL's commands and the gRPC service's document-query bindings share it,
// so one spelling reaches one object on both.
package objref

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Segment is one segment of an object reference.
type Segment struct {
	Text   string // as written, quotes kept, so a lookup reads the notation
	Name   string // the declaration or feature it names
	Dotted bool   // written after a `.` rather than `::`
	Index  int    // 1-based element of a multi-valued feature, 0 for none
}

// Ref is a parsed object reference.
type Ref struct {
	Text     string
	ID       int64 // the root object's id, 0 when the root is a declared name
	Segments []Segment
}

// RefError reports text that is no object reference at all.
type RefError struct {
	Ref    string
	Detail string
	// Named is the `::`-joined run of names read before a character no unquoted
	// name holds stopped the text: `T::SA` in `T::SA-506`.
	Named string
	// Hint is what Named may have meant, appended to the report.
	Hint string
}

func (e *RefError) Error() string {
	return fmt.Sprintf("%q is not an object reference: %s%s", e.Ref, e.Detail, e.Hint)
}

// IsID reports whether text is an id alone, `#` followed by digits.
func IsID(text string) bool {
	return len(text) > 1 && text[0] == '#' && LeadingDigits(text[1:]) == text[1:]
}

// LeadingDigits returns the run of ASCII digits text starts with.
func LeadingDigits(text string) string {
	i := 0
	for i < len(text) && text[i] >= '0' && text[i] <= '9' {
		i++
	}
	return text[:i]
}

// LooksLikePath reports whether text can only be an object reference — it
// starts with an id or walks a feature with `.` or an index — so a failure to
// resolve it is reported as such rather than tried as a declaration's name.
func LooksLikePath(text string) bool {
	if strings.HasPrefix(text, "#") {
		return true
	}
	inName, escaped := false, false
	for _, r := range text {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '\'':
			inName = !inName
		case !inName && (r == '.' || r == '['):
			return true
		}
	}
	return false
}

// Parse reads an object reference, reporting what makes text none.
func Parse(text string) (Ref, error) {
	ref := Ref{Text: text}
	rest := text
	if rest == "" {
		return ref, &RefError{Ref: text, Detail: "nothing was named"}
	}
	if strings.HasPrefix(rest, "#") {
		digits := LeadingDigits(rest[1:])
		if digits == "" {
			return ref, &RefError{Ref: text, Detail: "an object id is written #<id>, with the number the object was created under"}
		}
		id, err := strconv.ParseInt(digits, 10, 64)
		if err != nil || id <= 0 {
			return ref, &RefError{Ref: text, Detail: fmt.Sprintf("#%s is not an object id (ids count up from 1)", digits)}
		}
		ref.ID = id
		rest = rest[1+len(digits):]
		if rest == "" {
			return ref, nil
		}
		var ok bool
		if _, rest, ok = CutSeparator(rest); !ok {
			return ref, &RefError{Ref: text, Detail: fmt.Sprintf("a feature of #%d is written after . or ::, not %q", id, rest)}
		}
	}
	dotted := false
	for {
		seg, after, err := scanSegment(text, rest)
		if err != nil {
			return ref, err
		}
		seg.Dotted = dotted
		if len(ref.Segments) == 0 && ref.ID == 0 && seg.Index > 0 {
			return ref, &RefError{Ref: text, Detail: fmt.Sprintf("%s takes no index: an index picks an element of a multi-valued feature", seg.Text)}
		}
		ref.Segments = append(ref.Segments, seg)
		if after == "" {
			return ref, nil
		}
		sep, next, ok := CutSeparator(after)
		if !ok {
			err := &RefError{Ref: text, Detail: fmt.Sprintf("%q cannot follow %s: segments are separated by . or ::", after, seg.Text)}
			if ref.ID == 0 {
				err.Named = DeclaredRun(ref.Segments)
			}
			return ref, err
		}
		if next == "" {
			return ref, &RefError{Ref: text, Detail: fmt.Sprintf("it ends in %q with no feature after it", sep)}
		}
		rest, dotted = next, sep == "."
	}
}

// DeclaredRun is the `::`-joined registered names of segments that may all name
// a declaration — none reached through `.` or an index — or "" when one is not.
func DeclaredRun(segments []Segment) string {
	names := make([]string, len(segments))
	for i, seg := range segments {
		if seg.Dotted || seg.Index > 0 {
			return ""
		}
		names[i] = seg.Name
	}
	return strings.Join(names, "::")
}

// Head is the count of leading segments that may name a declaration: the run
// before the first `.` or index, since a segment after `.` is only ever a feature.
func Head(segments []Segment) int {
	for i, seg := range segments {
		if seg.Index > 0 || seg.Dotted {
			return i
		}
	}
	return len(segments)
}

// JoinTyped spells segments as the qualified name they were written as.
func JoinTyped(segments []Segment) string {
	texts := make([]string, len(segments))
	for i, seg := range segments {
		texts[i] = seg.Text
	}
	return strings.Join(texts, "::")
}

// IsNamespace reports whether sym is a package or namespace, which holds
// members but is never an object.
func IsNamespace(sym *symbols.Symbol) bool {
	return sym != nil && (sym.Kind == symbols.SymbolPackage || sym.Kind == symbols.SymbolNamespace)
}

// NamespaceKind names what a namespace symbol is for a message.
func NamespaceKind(sym *symbols.Symbol) string {
	if sym.Kind == symbols.SymbolPackage {
		return "package"
	}
	return "namespace"
}

// CutSeparator splits the segment separator text starts with from what follows.
func CutSeparator(text string) (sep, rest string, ok bool) {
	switch {
	case strings.HasPrefix(text, "::"):
		return "::", text[2:], true
	case strings.HasPrefix(text, "."):
		return ".", text[1:], true
	}
	return "", text, false
}

// scanSegment reads one segment — a name, quoted or not, and an optional
// index — from the front of rest; ref is the whole reference, for reporting.
func scanSegment(ref, rest string) (Segment, string, error) {
	var seg Segment
	end := 0
	if strings.HasPrefix(rest, "'") {
		escaped := false
		for i, r := range rest[1:] {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '\'':
				end = i + 2
			}
			if end > 0 {
				break
			}
		}
		if end == 0 {
			return seg, "", &RefError{Ref: ref, Detail: fmt.Sprintf("the quoted name %s is not closed", rest)}
		}
		names, ok := source.QualifiedNameSegments(rest[:end])
		if !ok || len(names) != 1 {
			return seg, "", &RefError{Ref: ref, Detail: fmt.Sprintf("%s is not a name", rest[:end])}
		}
		seg.Text, seg.Name = rest[:end], names[0]
	} else {
		for end < len(rest) {
			r, size := utf8.DecodeRuneInString(rest[end:])
			if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				break
			}
			end += size
		}
		if end == 0 {
			return seg, "", &RefError{Ref: ref, Detail: fmt.Sprintf("a name was expected at %q", rest)}
		}
		seg.Text, seg.Name = rest[:end], rest[:end]
	}
	rest = rest[end:]
	if !strings.HasPrefix(rest, "[") {
		return seg, rest, nil
	}
	closeAt := strings.IndexByte(rest, ']')
	if closeAt < 0 {
		return seg, "", &RefError{Ref: ref, Detail: fmt.Sprintf("the index after %s is not closed with ]", seg.Text)}
	}
	digits := rest[1:closeAt]
	index, err := strconv.Atoi(digits)
	if digits == "" || LeadingDigits(digits) != digits || err != nil || index < 1 {
		return seg, "", &RefError{Ref: ref, Detail: fmt.Sprintf("%s[%s] is not an index: elements are counted from 1", seg.Text, digits)}
	}
	seg.Index = index
	seg.Text += rest[:closeAt+1]
	return seg, rest[closeAt+1:], nil
}
