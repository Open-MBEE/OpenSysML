package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseMemberPath reads a member path: two or more `.`-joined names, each a
// basic or a 'quoted name'. Anything else — a bare name, malformed text —
// is no member path.
func parseMemberPath(property string) ([]string, bool) {
	segments, ok := source.MemberPathSegments(property)
	return segments, ok && len(segments) >= 2
}

// memberPathString writes a member path back in canonical form, quoting each
// segment that is no basic name; it names a path in trackers and errors.
func memberPathString(segments []string) string {
	return source.MemberPathOf(segments)
}

// memberPathValues walks a member path from the row element: each segment but
// the last names a member of the element reached so far — the row's own
// members before inherited ones — and the last reads its declared feature
// values, so a member without a value yields an empty cell. A segment naming
// no member means the path is absent on this row.
func (e *executor) memberPathValues(sym *symbols.Symbol, segments []string) ([]Value, bool, error) {
	current := sym
	for _, segment := range segments[:len(segments)-1] {
		member, ok := e.context.Model.LookupMember(current, segment)
		if !ok || member == nil {
			return nil, false, nil
		}
		current = member
	}
	return e.declaredFeatureValues(current, segments[len(segments)-1])
}
