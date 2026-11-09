// Package naming derives the qualified names the export, the query API and the
// symbol facts share: a declared name per segment, `@N` for an unnamed member.
package naming

import (
	"fmt"
	"strconv"
	"strings"
)

// Segment is the identity segment of a declared name: quoted when it would
// otherwise read as an unnamed member's `@N` address or as several segments.
func Segment(name string) string {
	if strings.HasPrefix(name, "@") || strings.Contains(name, "::") || strings.HasPrefix(name, "'") {
		var segment strings.Builder
		segment.Grow(len(name) + 2)
		segment.WriteByte('\'')
		for i := 0; i < len(name); i++ {
			if name[i] == '\\' || name[i] == '\'' {
				segment.WriteByte('\\')
			}
			segment.WriteByte(name[i])
		}
		segment.WriteByte('\'')
		return segment.String()
	}
	return name
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

// Qualify builds the qualified name of a member; an unnamed declaration is
// addressed by its position in its owner, which keeps every element identifiable.
func Qualify(owner, name string, index int) string {
	if name == "" {
		name = fmt.Sprintf("@%d", index)
	} else {
		name = Segment(name)
	}
	if owner == "" {
		return name
	}
	return owner + "::" + name
}
