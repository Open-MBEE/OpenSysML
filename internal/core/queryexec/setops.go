package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// setKey is a row's identity for set operations: an element or object as
// rowKey has it, a verdict by its assertion and the carrier it was checked on.
type setKey struct {
	row  rowKey
	kind ValueKind
	path string
}

func setKeyOf(row Value) setKey {
	verdict, ok := row.Verdict()
	if !ok {
		return setKey{row: keyOfRow(row), kind: row.Kind()}
	}
	key := setKey{
		row:  rowKey{element: symbols.KeyOf(verdict.Assertion())},
		kind: ValueVerdict,
	}
	if carrier, held := verdict.Carrier(); held {
		key.row.object = carrier.ID
	} else {
		key.path = verdict.Path()
	}
	return key
}

// evaluateExcept keeps the source rows whose identity does not occur among
// the exclude rows, in source order with its projected columns.
func (e *executor) evaluateExcept(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	exclude, err := e.rowArgument(expression, "exclude")
	if err != nil {
		return sequence{}, err
	}
	excluded := make(map[setKey]struct{}, len(exclude.values))
	for _, value := range exclude.values {
		excluded[setKeyOf(value)] = struct{}{}
	}
	result := filtered(source)
	for i, value := range source.values {
		if _, ok := excluded[setKeyOf(value)]; ok {
			continue
		}
		appendSelected(&result, source, i)
	}
	return result, nil
}

// evaluateUnion emits every source row, then each row of other not yet emitted;
// both inputs must be unprojected or project the same columns.
func (e *executor) evaluateUnion(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	other, err := e.rowArgument(expression, "other")
	if err != nil {
		return sequence{}, err
	}
	if !sameColumns(source.columns, other.columns) {
		actual := "unprojected row set"
		if len(other.columns) > 0 {
			actual = "projected row set"
		}
		return sequence{}, e.invalidArgument(expression, "other", actual)
	}
	seen := make(map[setKey]struct{}, len(source.values)+len(other.values))
	result := filtered(source)
	for _, input := range []sequence{source, other} {
		for i, value := range input.values {
			key := setKeyOf(value)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			appendSelected(&result, input, i)
		}
	}
	return result, nil
}

func sameColumns(a, b []Column) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].name != b[i].name {
			return false
		}
	}
	return true
}
