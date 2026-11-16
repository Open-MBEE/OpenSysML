package identity

import "strings"

// Segment is the identity segment of a declared name: quoted when it is empty or
// would otherwise read as an unnamed member's `@N` address or as several segments.
func Segment(name string) string {
	if name == "" || strings.HasPrefix(name, "@") || strings.Contains(name, "::") || strings.HasPrefix(name, "'") {
		return QuotedSegment(name)
	}
	return name
}

// QuotedSegment is a declared name as a quoted identity segment, its quote and
// backslash escaped, which Unquote reads back.
func QuotedSegment(name string) string {
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

// QualifiedSegments joins declared names, outermost first, as the qualified
// identity a derived element id encodes.
func QualifiedSegments(names []string) string {
	segments := make([]string, len(names))
	for i, name := range names {
		segments[i] = Segment(name)
	}
	return strings.Join(segments, "::")
}
