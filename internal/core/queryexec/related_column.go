package queryexec

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// relatedColumn is one decoded RelatedColumn of a projection: a relationship
// traversal run from each row's declaration, reduced by aggregate.
type relatedColumn struct {
	plan      queryplan.Expression
	kind      string
	direction string
	maxDepth  int64
	aggregate string
}

// relatedColumnOf evaluates a RelatedColumn's arguments once per projection
// and validates them as RelatedElements would, naming the column in failures.
func (e *executor) relatedColumnOf(plan queryplan.Expression) (*relatedColumn, error) {
	column := &relatedColumn{plan: plan, aggregate: queryplan.RelatedAggregateList}
	var err error
	if column.kind, err = e.stringArgument(plan, "relationshipKind"); err != nil {
		return nil, columnScoped(err, plan.Target())
	}
	if column.direction, err = e.stringArgument(plan, "direction"); err != nil {
		return nil, columnScoped(err, plan.Target())
	}
	if column.maxDepth, err = e.integerArgument(plan, "maxDepth"); err != nil {
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
	if err := e.validateRelationship(plan, column.kind, column.direction); err != nil {
		return nil, columnScoped(err, plan.Target())
	}
	return column, nil
}

// evaluateRelatedCell traverses from the row's declaration and reduces the
// reached elements to the column's aggregate: the ordered list, its count, or
// whether it is non-empty.
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
	values, err := e.traverseRelated(related.plan, related.kind, related.direction, related.maxDepth, []*symbols.Symbol{root})
	if err != nil {
		return nil, columnScoped(err, column.name)
	}
	switch related.aggregate {
	case queryplan.RelatedAggregateCount:
		return []Value{IntegerValue(int64(len(values)))}, nil
	case queryplan.RelatedAggregateAny:
		return []Value{BooleanValue(len(values) > 0)}, nil
	default:
		return values, nil
	}
}

// columnScoped names the projected column an execution error arose in.
func columnScoped(err error, column string) error {
	var execution *Error
	if errors.As(err, &execution) && execution.Property == "" {
		execution.Property = column
	}
	return err
}
