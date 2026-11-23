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

// memberPathStep resolves one inner segment on an element: the member it names, own members
// first, else the elements of the metaclass feature it names; known is false where it names neither.
func (e *executor) memberPathStep(current *symbols.Symbol, segment string) (elements []*symbols.Symbol, known bool) {
	if member, ok := e.context.Model.LookupMember(current, segment); ok && member != nil {
		return []*symbols.Symbol{member}, true
	}
	return e.context.Model.ReflectiveElements(current, segment)
}

// memberOverflow names a declared member a path reached holding more values than its multiplicity admits.
type memberOverflow struct {
	member *symbols.Symbol
	count  int
}

// memberPathValues walks a member path from the row element. A segment naming neither a member nor a metaclass
// feature makes the path absent, one holding no element an empty cell. The first overfull member is reported.
func (e *executor) memberPathValues(sym *symbols.Symbol, segments []string) ([]Value, bool, *memberOverflow, error) {
	reached := []*symbols.Symbol{sym}
	for _, segment := range segments[:len(segments)-1] {
		var next []*symbols.Symbol
		known := false
		for _, current := range reached {
			elements, ok := e.memberPathStep(current, segment)
			known = known || ok
			next = append(next, elements...)
		}
		if len(next) == 0 {
			return nil, known, nil, nil
		}
		reached = next
	}
	last := segments[len(segments)-1]
	var values []Value
	var overflow *memberOverflow
	present := false
	for _, current := range reached {
		found, ok, err := e.declaredFeatureValues(current, last)
		if err != nil {
			return nil, false, nil, err
		}
		if ok {
			if m, _ := e.context.Model.LookupMember(current, last); m != nil && overflow == nil && e.exceedsMultiplicity(m, len(found)) {
				overflow = &memberOverflow{member: m, count: len(found)}
			}
		} else {
			found, ok, err = e.metaclassSegmentValues(current, last)
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
	return values, true, overflow, nil
}

// exceedsMultiplicity reports whether count values are more than member's governing multiplicity admits.
func (e *executor) exceedsMultiplicity(member *symbols.Symbol, count int) bool {
	rng := e.context.Model.GoverningMultiplicityOf(member)
	return rng.Upper.Known && !rng.Upper.Infinite && int64(count) > rng.Upper.Value
}

// metaclassSegmentValues reads a last segment with no declared value: the metaclass property a
// valueless member yields to, else, where no member bears the name, the metaclass feature itself.
func (e *executor) metaclassSegmentValues(current *symbols.Symbol, last string) ([]Value, bool, error) {
	if e.valuelessMemberYieldsMetaclassProperty(current, last) {
		return e.metaclassPropertyValues(current, last)
	}
	if member, ok := e.context.Model.LookupMember(current, last); ok && member != nil {
		return nil, false, nil
	}
	return e.reflectiveValues(queryplan.Expression{}, current, last)
}

// memberPathStop walks a member path as memberPathValues does and reports where
// it stopped: the last element reached and the segment that names nothing there.
func (e *executor) memberPathStop(sym *symbols.Symbol, segments []string) (*symbols.Symbol, string) {
	current := sym
	for _, segment := range segments {
		next, _ := e.memberPathStep(current, segment)
		if len(next) == 0 {
			return current, segment
		}
		current = next[0]
	}
	return current, segments[len(segments)-1]
}
