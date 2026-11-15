// Package naming derives the qualified names the export, the query API and the
// symbol facts share: a declared name per segment, `@N` for an unnamed member.
package naming

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Segment is the identity segment of a declared name (identity.Segment).
func Segment(name string) string {
	return identity.Segment(name)
}

// QuotedSegment is a declared name as a quoted identity segment
// (identity.QuotedSegment), which Unquote reads back.
func QuotedSegment(name string) string {
	return identity.QuotedSegment(name)
}

// Unquote is the declared name an identity segment stands for: Segment's quoting
// and escapes removed, a bare or positional segment as it is.
func Unquote(segment string) string {
	if len(segment) < 2 || segment[0] != '\'' || segment[len(segment)-1] != '\'' {
		return segment
	}
	var name strings.Builder
	name.Grow(len(segment) - 2)
	for i := 1; i < len(segment)-1; i++ {
		if segment[i] == '\\' && i+1 < len(segment)-1 {
			i++
		}
		name.WriteByte(segment[i])
	}
	return name.String()
}

// Escaped spells a qualified identity as Element::qualifiedName reads (KerML 1.1 §8.3.2.1.2
// escapedName): a basic name bare, another name single-quoted, a positional `@N` as it is.
func Escaped(qname string) string {
	segments := Segments(qname)
	for i, segment := range segments {
		if PositionalSegment(segment) {
			continue
		}
		segments[i] = identity.EscapeName(Unquote(segment))
	}
	return strings.Join(segments, "::")
}

// Segments splits a qualified identity at the `::` outside its quoted segments.
func Segments(qname string) []string {
	var segments []string
	for {
		segment, rest, more := CutSegment(qname)
		segments = append(segments, segment)
		if !more {
			return segments
		}
		qname = rest
	}
}

// CutSegment splits the first segment of a qualified identity off the rest.
func CutSegment(qname string) (segment, rest string, more bool) {
	if qname != "" {
		if end, ok := quotedSegmentEnd(qname, 0); ok {
			if end == len(qname) {
				return qname, "", false
			}
			return qname[:end], qname[end+2:], true
		}
	}
	return strings.Cut(qname, "::")
}

// quotedSegmentEnd is where the quoted segment opening at start closes, when
// its closing quote ends the segment there.
func quotedSegmentEnd(qname string, start int) (int, bool) {
	if qname[start] != '\'' {
		return 0, false
	}
	for i := start + 1; i < len(qname); i++ {
		switch qname[i] {
		case '\\':
			i++
		case '\'':
			end := i + 1
			return end, end == len(qname) || strings.HasPrefix(qname[end:], "::")
		}
	}
	return 0, false
}

// PositionalSegment reports a canonical `@N` segment: a non-negative decimal
// without sign or leading zeros.
func PositionalSegment(segment string) bool {
	if len(segment) < 2 || segment[0] != '@' {
		return false
	}
	value, err := strconv.Atoi(segment[1:])
	return err == nil && value >= 0 && strconv.Itoa(value) == segment[1:]
}

// IsPositionalIdentity reports whether a name contains a canonical `@N` segment.
func IsPositionalIdentity(name string) bool {
	for _, segment := range Segments(name) {
		if PositionalSegment(segment) {
			return true
		}
	}
	return false
}

// Qualify builds the qualified name of a member under the name it has, which the
// empty unrestricted name is too; one without a name is addressed as Positional.
func Qualify(owner string, name Name, index int) string {
	if !name.Named {
		return Positional(owner, index)
	}
	return join(owner, Segment(name.Text))
}

// Positional addresses an unnamed declaration by its position in its owner,
// which keeps every element identifiable.
func Positional(owner string, index int) string {
	return join(owner, fmt.Sprintf("@%d", index))
}

func join(owner, segment string) string {
	if owner == "" {
		return segment
	}
	return owner + "::" + segment
}

// Name is a name a declaration may have: Text is the empty unrestricted name when it is
// empty and Named, and no name at all when not Named (KerML §8.2.2.3).
type Name struct {
	Text  string
	Named bool
}

// NameOf is the name text states: none when it is empty.
func NameOf(text string) Name { return Name{Text: text, Named: text != ""} }

// identName is the name an identification declares.
func identName(id ast.Identification) Name {
	return Name{Text: id.Name, Named: id.HasName()}
}

// spannedName is a name stated beside the span it was written at.
func spannedName(text string, span source.Span) Name {
	return Name{Text: text, Named: text != "" || span.Len > 0}
}
