package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// parseMemberPath reads a member path: two or more `.`-joined names, each a
// basic or a 'quoted name'; anything else is no member path.
func parseMemberPath(property string) ([]string, bool) {
	segments, ok := source.MemberPathSegments(property)
	return segments, ok && len(segments) >= 2
}

// memberPathStep resolves one inner segment on an element: the member it
// names, own members first, else the elements of the metaclass feature it
// names (`type`, `owner`, …), so `type.documentation` reads a definition.
func (e *executor) memberPathStep(current *symbols.Symbol, segment string) []*symbols.Symbol {
	if member, ok := e.context.Model.LookupMember(current, segment); ok && member != nil {
		return []*symbols.Symbol{member}
	}
	if elements, ok := e.context.Model.ReflectiveElements(current, segment); ok {
		return elements
	}
	return nil
}

// memberPathValues walks a member path from the row element; a segment naming
// neither a member nor a metaclass feature makes the path absent on this row.
// The last segment reads a declared feature, else a metaclass feature such as
// documentation. The member the last segment names is returned with the
// values (nil when absent or when it is not a declared member).
func (e *executor) memberPathValues(sym *symbols.Symbol, segments []string) ([]Value, bool, *symbols.Symbol, error) {
	reached := []*symbols.Symbol{sym}
	for _, segment := range segments[:len(segments)-1] {
		var next []*symbols.Symbol
		for _, current := range reached {
			next = append(next, e.memberPathStep(current, segment)...)
		}
		if len(next) == 0 {
			return nil, false, nil, nil
		}
		reached = next
	}
	last := segments[len(segments)-1]
	var values []Value
	var member *symbols.Symbol
	present := false
	for _, current := range reached {
		found, ok, err := e.declaredFeatureValues(current, last)
		if err != nil {
			return nil, false, nil, err
		}
		if ok {
			if m, _ := e.context.Model.LookupMember(current, last); member == nil {
				member = m
			}
		} else {
			found, ok, err = e.reflectiveValues(queryplan.Expression{}, current, last)
			if err != nil {
				return nil, false, nil, err
			}
		}
		if ok {
			present = true
			values = append(values, found...)
		}
	}
	if !present {
		return nil, false, nil, nil
	}
	return values, true, member, nil
}

// memberPathStop walks a member path as memberPathValues does and reports where
// it stopped: the last element reached and the segment that names nothing there.
func (e *executor) memberPathStop(sym *symbols.Symbol, segments []string) (*symbols.Symbol, string) {
	current := sym
	for _, segment := range segments {
		next := e.memberPathStep(current, segment)
		if len(next) == 0 {
			return current, segment
		}
		current = next[0]
	}
	return current, segments[len(segments)-1]
}
