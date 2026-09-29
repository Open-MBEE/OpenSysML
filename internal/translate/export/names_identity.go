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
	for start := 0; start <= len(qname); {
		if start == len(qname) {
			segments = append(segments, "")
			return segments
		}
		if qname[start] == '\'' {
			for i := start + 1; i < len(qname); i++ {
				if qname[i] == '\\' {
					i++
					continue
				}
				if qname[i] == '\'' {
					end := i + 1
					if end == len(qname) || strings.HasPrefix(qname[end:], "::") {
						segments = append(segments, qname[start:end])
						if end == len(qname) {
							return segments
						}
						start = end + 2
						goto next
					}
					break
				}
			}
		}
		if separator := strings.Index(qname[start:], "::"); separator >= 0 {
			segments = append(segments, qname[start:start+separator])
			start += separator + 2
		} else {
			segments = append(segments, qname[start:])
			return segments
		}
	next:
	}
	return segments
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
