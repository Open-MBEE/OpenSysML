package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"strings"
)

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

func mappingQualifiedNameFromSpec(qname string) string {
	global := strings.HasPrefix(qname, "$::")
	if global {
		qname = strings.TrimPrefix(qname, "$::")
	}
	segments := naming.Segments(qname)
	for i, segment := range segments {
		segments[i] = naming.Segment(identityName(segment))
	}
	out := strings.Join(segments, "::")
	if global {
		return "$::" + out
	}
	return out
}
