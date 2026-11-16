package export

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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

// escapedSegmentName is the name a qualifiedName segment spells (KerML
// §8.3.2.1.2 escapedName): a quoted one with its §8.2.2 escapes read.
func escapedSegmentName(segment string) string {
	if len(segment) < 2 || segment[0] != '\'' || segment[len(segment)-1] != '\'' {
		return segment
	}
	return source.Unescape(segment[1 : len(segment)-1])
}

// mappingQualifiedNameFromSpec is the address a qualifiedName stands for.
func mappingQualifiedNameFromSpec(qname string) string {
	global := strings.HasPrefix(qname, "$::")
	if global {
		qname = strings.TrimPrefix(qname, "$::")
	}
	segments := naming.Segments(qname)
	for i, segment := range segments {
		segments[i] = naming.Segment(escapedSegmentName(segment))
	}
	out := strings.Join(segments, "::")
	if global {
		return "$::" + out
	}
	return out
}
