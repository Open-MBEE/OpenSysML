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
	byKey  map[keyBucket][]int
}

// keyBucket is the comparable coarsening of a key value under which two values equal by
// valuesEqual share a bucket: rows by identity, numbers by value, quantities by base magnitude.
type keyBucket struct {
	kind     string
	row      setKey
	position int
	number   float64
	text     string
	boolean  bool
}

func keyBucketOf(value Value) keyBucket {
	if value.isRow() {
		return keyBucket{kind: "row", row: setKeyOf(value)}
	}
	if connector, position, ok := value.ConnectorEnd(); ok {
		return keyBucket{kind: "connectorEnd", row: setKey{row: rowKey{element: symbols.KeyOf(connector)}}, position: position}
	}
	if arithmeticKind(value.Kind()) {
		return keyBucket{kind: "number", number: realOperand(value)}
	}
	if quantity, ok := value.Quantity(); ok {
		return keyBucket{kind: "quantity", number: quantity.BaseMagnitude()}
	}
	if text, ok := value.String(); ok {
		return keyBucket{kind: "string", text: text}
	}
	if boolean, ok := value.Boolean(); ok {
		return keyBucket{kind: "boolean", boolean: boolean}
	}
	return keyBucket{kind: string(value.Kind())}
}

// find is the row of the side whose key equals value under valuesEqual, if any.
func (s differSide) find(value Value) (int, bool) {
	for _, row := range s.byKey[keyBucketOf(value)] {
		if valuesEqual(s.cell(row, s.key)[0], value) {
			return row, true
		}
	}
	return 0, false
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
	if key == "left" || key == "right" {
		// The result names its side columns left and right; a key so named would be ambiguous.
		return sequence{}, e.invalidArgument(expression, "key", key)
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
		side.byKey = make(map[keyBucket][]int, len(side.rows.values))
		for row := range side.rows.values {
			cell := side.cell(row, side.key)
			if len(cell) != 1 {
				findings = append(findings, Finding{
					Step: step, Element: rowTarget(side.rows.values[row]), Constraint: ConstraintMultiplicity,
					Observed: fmt.Sprintf("%s key %s holds %d values", side.name, key, len(cell)),
				})
				continue
			}
			if previous, duplicate := side.find(cell[0]); duplicate {
				findings = append(findings, Finding{
					Step: step, Element: rowTarget(side.rows.values[row]), Constraint: ConstraintDuplicateKey,
					Observed: fmt.Sprintf("%s key %s = %s is also carried by %s", side.name, key, e.valueText(cell[0]), rowTarget(side.rows.values[previous])),
				})
				continue
			}
			bucket := keyBucketOf(cell[0])
			side.byKey[bucket] = append(side.byKey[bucket], row)
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
	for row := range left.values {
		cell := sides[0].cell(row, sides[0].key)
		other, both := sides[1].find(cell[0])
		if !both {
			continue
		}
		leftValues, rightValues := sides[0].cell(row, sides[0].column), sides[1].cell(other, sides[1].column)
		if cellsEqual(leftValues, rightValues) {
			continue
		}
		// appendSelected keeps the left row's Tree depth; its cells are then the three of Differ.
		appendSelected(&result, left, row)
		result.cells[len(result.values)-1] = []Cell{
			{values: append([]Value(nil), cell...), origin: left.cells[row][sides[0].key].origin},
			{values: append([]Value(nil), leftValues...), origin: left.cells[row][sides[0].column].origin},
			{values: append([]Value(nil), rightValues...), origin: right.cells[other][sides[1].column].origin},
		}
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
