package export

import (
	"strconv"
	"strings"
)

func identitySegment(name string) string {
	if positionalSegment(name) || strings.Contains(name, "::") {
		return "'" + name + "'"
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
				return segment[1:i]
			}
			return segment
		}
	}
	return segment
}

func positionalSegment(segment string) bool {
	if len(segment) < 2 || segment[0] != '@' {
		return false
	}
	value, err := strconv.Atoi(segment[1:])
	return err == nil && value >= 0 && strconv.Itoa(value) == segment[1:]
}
