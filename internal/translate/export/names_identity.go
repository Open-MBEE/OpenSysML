package export

import (
	"strconv"
	"strings"
)

func identitySegment(name string) string {
	if positionalSegment(name) || strings.Contains(name, "::") || strings.HasPrefix(name, "'") {
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

func identitySegments(qname string) []string {
	if qname == "" {
		return []string{""}
	}
	var segments []string
	for start := 0; ; {
		if start == len(qname) {
			return append(segments, "")
		}
		if end, ok := quotedSegmentEnd(qname, start); ok {
			segments = append(segments, qname[start:end])
			if end == len(qname) {
				return segments
			}
			start = end + 2
			continue
		}
		separator := strings.Index(qname[start:], "::")
		if separator < 0 {
			return append(segments, qname[start:])
		}
		segments = append(segments, qname[start:start+separator])
		start += separator + 2
	}
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

func identityName(segment string) string {
	if len(segment) < 2 || segment[0] != '\'' {
		return segment
	}
	for i := 1; i < len(segment); i++ {
		if segment[i] == '\\' {
			i++
			continue
		}
		if segment[i] == '\'' {
			if i == len(segment)-1 {
				var name strings.Builder
				for j := 1; j < i; j++ {
					if segment[j] == '\\' && j+1 < i {
						j++
					}
					name.WriteByte(segment[j])
				}
				return name.String()
			}
			return segment
		}
	}
	return segment
}

func parserQualifiedName(qname string) string {
	global := strings.HasPrefix(qname, "$::")
	if global {
		qname = strings.TrimPrefix(qname, "$::")
	}
	segments := identitySegments(qname)
	for i, segment := range segments {
		segments[i] = identitySegment(escapeName(identityName(segment)))
	}
	out := strings.Join(segments, "::")
	if global {
		return "$::" + out
	}
	return out
}

func positionalSegment(segment string) bool {
	if len(segment) < 2 || segment[0] != '@' {
		return false
	}
	value, err := strconv.Atoi(segment[1:])
	return err == nil && value >= 0 && strconv.Itoa(value) == segment[1:]
}
