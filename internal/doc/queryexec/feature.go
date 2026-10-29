package queryexec

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ReadFeature reads one feature of a row as a span column's cell does: the
// property is a feature name or member path below the row, read from the
// declared model for an element row and from the object for a session row.
// Absent features answer no values; a feature that cannot be evaluated is an
// unevaluable-feature error.
func ReadFeature(context Context, row Value, property string, options Options) ([]Value, error) {
	if context.Index == nil || context.Resolver == nil || context.Model == nil {
		return nil, &Error{Kind: ErrorInvalidContext}
	}
	budget := options.VisitBudget
	if budget == 0 {
		budget = defaultVisitBudget
	}
	related := context.Related
	if related == nil {
		related = NewRelationshipTables()
	}
	execution := &executor{
		context:           context,
		reader:            query.NewPropertyReader(context.Index, context.Resolver, context.Model),
		bindings:          make(map[string]sequence),
		budget:            &visitBudget{remaining: budget},
		calls:             &visitBudget{remaining: defaultInvocationBudget},
		related:           related,
		derived:           &derivedValues{},
		metadataTypes:     make(map[string]*symbols.Symbol),
		metaclassFeatures: make(map[*symbols.Symbol]map[string]metaclassProperty),
		depthLeft:         defaultInvocationDepth,
	}
	execution.reader.WithElementID(execution.elementID)
	values, _, err := execution.featurePathValues(row, property)
	if err != nil {
		var typed *Error
		if errors.As(err, &typed) {
			return nil, err
		}
		return nil, &Error{Kind: ErrorUnevaluableFeature, Property: property, Target: rowTarget(row), Cause: err}
	}
	return values, nil
}

// featurePathValues reads a property of a row, walking a member path through
// the objects a session row holds, which a declared-model row walks itself. An
// object lacking a member contributes nothing; the path is absent only when no
// object reaches its last member.
func (e *executor) featurePathValues(row Value, property string) ([]Value, bool, error) {
	values, present, err := e.propertyValues(row, property)
	if present || err != nil {
		return values, present, err
	}
	if _, _, isObject := row.Object(); !isObject {
		return nil, false, nil
	}
	segments, ok := parseMemberPath(property)
	if !ok {
		return nil, false, nil
	}
	current := []Value{row}
	for _, segment := range segments[:len(segments)-1] {
		var next []Value
		for _, value := range current {
			members, present, err := e.propertyValues(value, segment)
			if err != nil {
				return nil, false, err
			}
			if present {
				next = append(next, members...)
			}
		}
		current = next
	}
	var out []Value
	reached := false
	for _, value := range current {
		members, present, err := e.propertyValues(value, segments[len(segments)-1])
		if err != nil {
			return nil, false, err
		}
		if present {
			reached = true
			out = append(out, members...)
		}
	}
	return out, reached, nil
}
