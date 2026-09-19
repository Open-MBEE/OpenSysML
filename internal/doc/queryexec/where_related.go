package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// evaluateWhereRelated keeps the source elements with a reachable related element
// (exists = true) or without one (exists = false).
func (e *executor) evaluateWhereRelated(expression queryplan.Expression) (sequence, error) {
	source, err := e.elementArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	walk, err := e.relationshipArguments(expression)
	if err != nil {
		return sequence{}, err
	}
	exists := true
	if hasArgument(expression, "exists") {
		if exists, err = e.booleanArgument(expression, "exists"); err != nil {
			return sequence{}, err
		}
	}
	result := filtered(source)
	for i, value := range source.values {
		sym, _ := value.Element()
		related, err := e.hasRelated(expression, walk, sym)
		if err != nil {
			return sequence{}, err
		}
		if related == exists {
			appendSelected(&result, source, i)
		}
	}
	return result, nil
}

// hasRelated reports whether any element is reachable from sym over the walk,
// stopping at the first one found.
func (e *executor) hasRelated(expression queryplan.Expression, walk relationshipWalk, sym *symbols.Symbol) (bool, error) {
	found := false
	err := e.traverseRelated(expression, walk, []*symbols.Symbol{sym}, func(*symbols.Symbol) bool {
		found = true
		return false
	})
	return found, err
}
