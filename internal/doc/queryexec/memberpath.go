package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseMemberPath reads a member path: two or more `.`-joined names, each a
// basic or a 'quoted name'; anything else is no member path.
func parseMemberPath(property string) ([]string, bool) {
	segments, ok := source.MemberPathSegments(property)
	return segments, ok && len(segments) >= 2
}

// memberPathValues walks a member path from the row element, own members
// first; a segment naming no member makes the path absent on this row. The
// member the last segment names is returned with the values (nil when absent).
func (e *executor) memberPathValues(sym *symbols.Symbol, segments []string) ([]Value, bool, *symbols.Symbol, error) {
	current := sym
	for _, segment := range segments[:len(segments)-1] {
		member, ok := e.context.Model.LookupMember(current, segment)
		if !ok || member == nil {
			return nil, false, nil, nil
		}
		current = member
	}
	values, present, err := e.declaredFeatureValues(current, segments[len(segments)-1])
	if !present || err != nil {
		return values, present, nil, err
	}
	member, _ := e.context.Model.LookupMember(current, segments[len(segments)-1])
	return values, true, member, nil
}
