package queryexec

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// differSide is one input of Differ with its key and column indexes and its rows by key.
type differSide struct {
	name   string
	rows   sequence
	key    int
	column int
	byKey  map[string]int
}

func (s differSide) cell(row, column int) []Value {
	if row < len(s.rows.cells) && column < len(s.rows.cells[row]) {
		return s.rows.cells[row][column].values
	}
	return nil
}

// evaluateDiffer is the keyed value comparison of two projected row sets: one row
// (key, left column, right column) per key on both sides whose column cells differ.
// A key on one side only is Except's; a key carried twice on one side is a model error.
func (e *executor) evaluateDiffer(expression queryplan.Expression) (sequence, error) {
	left, err := e.rowArgument(expression, "left")
	if err != nil {
		return sequence{}, err
	}
	right, err := e.rowArgument(expression, "right")
	if err != nil {
		return sequence{}, err
	}
	key, err := e.stringArgument(expression, "key")
	if err != nil {
		return sequence{}, err
	}
	column, err := e.stringArgument(expression, "column")
	if err != nil {
		return sequence{}, err
	}
	step := Step{Kind: StepDiffer, Argument: key + ", " + column}
	sides := [2]differSide{{name: "left", rows: left}, {name: "right", rows: right}}
	var findings []Finding
	for i := range sides {
		side := &sides[i]
		for _, name := range []string{key, column} {
			if projectedColumn(side.rows, name) < 0 {
				return sequence{}, e.unknownColumn(expression, step, side.name, name, side.rows.columns)
			}
		}
		side.key = projectedColumn(side.rows, key)
		side.column = projectedColumn(side.rows, column)
		side.byKey = make(map[string]int, len(side.rows.values))
		for row := range side.rows.values {
			cell := side.cell(row, side.key)
			if len(cell) != 1 {
				findings = append(findings, Finding{
					Step: step, Element: rowTarget(side.rows.values[row]), Constraint: ConstraintMultiplicity,
					Observed: fmt.Sprintf("%s key %s holds %d values", side.name, key, len(cell)),
				})
				continue
			}
			text := e.keyText(cell[0])
			if previous, duplicate := side.byKey[text]; duplicate {
				findings = append(findings, Finding{
					Step: step, Element: rowTarget(side.rows.values[row]), Constraint: ConstraintDuplicateKey,
					Observed: fmt.Sprintf("%s key %s = %s is also carried by %s", side.name, key, e.valueText(cell[0]), rowTarget(side.rows.values[previous])),
				})
				continue
			}
			side.byKey[text] = row
		}
	}
	if len(findings) > 0 {
		return sequence{}, &Error{
			Kind:      ErrorDuplicateKey,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Property:  key,
			Actual:    findings[0].Observed,
			Origin:    expression.Origin(),
			Outcome:   &Outcome{Model: &ModelError{Findings: findings}},
		}
	}
	origin := expression.Origin()
	result := sequence{columns: []Column{{name: key, origin: origin}, {name: "left", origin: origin}, {name: "right", origin: origin}}}
	for row, value := range left.values {
		cell := sides[0].cell(row, sides[0].key)
		other, both := sides[1].byKey[e.keyText(cell[0])]
		if !both {
			continue
		}
		leftValues, rightValues := sides[0].cell(row, sides[0].column), sides[1].cell(other, sides[1].column)
		if cellsEqual(leftValues, rightValues) {
			continue
		}
		result.values = append(result.values, value)
		result.cells = append(result.cells, []Cell{
			{values: append([]Value(nil), cell...), origin: left.cells[row][sides[0].key].origin},
			{values: append([]Value(nil), leftValues...), origin: left.cells[row][sides[0].column].origin},
			{values: append([]Value(nil), rightValues...), origin: right.cells[other][sides[1].column].origin},
		})
	}
	return result, nil
}

// cellsEqual is sequence equality of two cells under KerML ==.
func cellsEqual(left, right []Value) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !valuesEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

// keyText identifies a key value: an element by its qualified name, an object by its
// label, a scalar by its kind and spelling — so "1" and 1 are different keys.
func (e *executor) keyText(value Value) string {
	if sym, ok := value.Element(); ok {
		return "element:" + symbols.FQNOf(sym)
	}
	if _, label, ok := value.Object(); ok {
		return "object:" + label
	}
	return string(value.Kind()) + ":" + e.valueText(value)
}

// unknownColumn is Differ's path error: one side projects no column of the name.
func (e *executor) unknownColumn(expression queryplan.Expression, step Step, side, name string, columns []Column) error {
	names := make([]string, len(columns))
	for i, column := range columns {
		names[i] = column.name
	}
	return &Error{
		Kind:       ErrorUnknownColumn,
		Query:      e.definition.Name(),
		Operation:  expression.Operation(),
		Parameter:  side,
		Property:   name,
		Candidates: names,
		Origin:     expression.Origin(),
		Outcome: &Outcome{Path: &PathError{Failures: []Failure{{
			Step:     step,
			Expected: "a column named " + name + " on the " + side + " rows",
			Frontier: Frontier{At: side, Nearest: first(suggest.Nearest(name, names))},
		}}}},
	}
}
