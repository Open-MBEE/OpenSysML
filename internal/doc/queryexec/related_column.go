package queryexec

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// relatedColumn is one decoded RelatedColumn of a projection: a relationship
// traversal run from each row's declaration, kept to targets when given and
// reduced by aggregate.
type relatedColumn struct {
	plan      queryplan.Expression
	walk      relationshipWalk
	aggregate string
	targets   map[symbols.ElementKey]struct{}
}

// keeps reports whether a reached element counts for the column.
func (c *relatedColumn) keeps(sym *symbols.Symbol) bool {
	if c.targets == nil {
		return true
	}
	_, ok := c.targets[symbols.KeyOf(sym)]
	return ok
}

// relatedColumnOf evaluates a RelatedColumn's arguments once per projection
// and validates them as RelatedElements would, naming the column in failures.
func (e *executor) relatedColumnOf(plan queryplan.Expression) (*relatedColumn, error) {
	column := &relatedColumn{plan: plan, aggregate: queryplan.RelatedAggregateList}
	var err error
	if column.walk, err = e.relationshipArguments(plan); err != nil {
		return nil, columnScoped(err, plan.Target())
	}
	if hasArgument(plan, "aggregate") {
		if column.aggregate, err = e.stringArgument(plan, "aggregate"); err != nil {
			return nil, columnScoped(err, plan.Target())
		}
	}
	if !queryplan.RelatedAggregateSupported(column.aggregate) {
		return nil, columnScoped(e.invalidArgument(plan, "aggregate", column.aggregate), plan.Target())
	}
	if hasArgument(plan, "targets") {
		targets, err := e.elementArgument(plan, "targets")
		if err != nil {
			return nil, columnScoped(err, plan.Target())
		}
		column.targets = make(map[symbols.ElementKey]struct{}, len(targets.values))
		for _, value := range targets.values {
			sym, _ := value.Element()
			column.targets[symbols.KeyOf(sym)] = struct{}{}
		}
	}
	return column, nil
}

// evaluateRelatedCell traverses from the row's declaration to the column's
// aggregate: the ordered list, its count, or whether any exists (stopping at the first).
func (e *executor) evaluateRelatedCell(column computedColumn, row Value) ([]Value, error) {
	related := column.related
	root := row.Declaration()
	if root == nil {
		return nil, &Error{
			Kind:      ErrorUndeclaredRow,
			Query:     e.definition.Name(),
			Operation: related.plan.Operation(),
			Property:  column.name,
			Target:    rowTarget(row),
			Origin:    related.plan.Origin(),
		}
	}
	if related.aggregate == queryplan.RelatedAggregateAny {
		found := false
		err := e.traverseRelated(related.plan, related.walk, []*symbols.Symbol{root}, func(neighbor *symbols.Symbol) bool {
			found = related.keeps(neighbor)
			return !found
		})
		if err != nil {
			return nil, columnScoped(err, column.name)
		}
		return []Value{BooleanValue(found)}, nil
	}
	var values []Value
	err := e.traverseRelated(related.plan, related.walk, []*symbols.Symbol{root}, func(neighbor *symbols.Symbol) bool {
		if related.keeps(neighbor) {
			values = append(values, ElementValue(neighbor))
		}
		return true
	})
	if err != nil {
		return nil, columnScoped(err, column.name)
	}
	if related.aggregate == queryplan.RelatedAggregateCount {
		return []Value{IntegerValue(int64(len(values)))}, nil
	}
	return values, nil
}

// columnScoped names the projected column an execution error arose in.
func columnScoped(err error, column string) error {
	var execution *Error
	if errors.As(err, &execution) && execution.Property == "" {
		execution.Property = column
	}
	return err
}
