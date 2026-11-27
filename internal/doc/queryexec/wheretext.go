package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// evaluateWhereText keeps the projected rows one of whose cells, among the
// named columns or every column, prints a text satisfying the comparison.
// Each value of a multi-valued cell is compared on its own.
func (e *executor) evaluateWhereText(expression queryplan.Expression) (sequence, error) {
	source, err := e.rowArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	if len(source.columns) == 0 {
		return sequence{}, e.invalidArgument(expression, "source", "unprojected row set")
	}
	selected, err := e.textColumns(expression, source)
	if err != nil {
		return sequence{}, err
	}
	operator, err := e.stringArgument(expression, "operator")
	if err != nil {
		return sequence{}, err
	}
	expected, err := e.stringArgument(expression, "value")
	if err != nil {
		return sequence{}, err
	}
	comparison, compareErr := newTextComparison(operator, expected)
	if compareErr != nil {
		if compareErr != errComparison {
			return sequence{}, e.invalidArgument(expression, "value", expected)
		}
		return sequence{}, e.operatorError(expression, operator)
	}
	result := filtered(source)
	for i := range source.values {
		if i >= len(source.cells) {
			continue
		}
		matched := false
		for _, column := range selected {
			if column >= len(source.cells[i]) {
				continue
			}
			for _, value := range source.cells[i][column].values {
				match, compareErr := comparison.text(e.valueText(value))
				if compareErr != nil {
					return sequence{}, e.operatorError(expression, operator)
				}
				if match {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			appendSelected(&result, source, i)
		}
	}
	return result, nil
}

// textColumns resolves the `columns` argument to indexes among the source's
// projected columns: every column when the argument is absent or empty.
func (e *executor) textColumns(expression queryplan.Expression, source sequence) ([]int, error) {
	var names []string
	if hasArgument(expression, "columns") {
		var err error
		if names, err = e.stringsArgument(expression, "columns"); err != nil {
			return nil, err
		}
	}
	if len(names) == 0 {
		all := make([]int, len(source.columns))
		for i := range all {
			all[i] = i
		}
		return all, nil
	}
	selected := make([]int, 0, len(names))
	for _, name := range names {
		index := -1
		for i, column := range source.columns {
			if column.name == name {
				index = i
				break
			}
		}
		if index < 0 {
			columns := make([]string, 0, len(source.columns))
			for _, column := range source.columns {
				columns = append(columns, column.name)
			}
			return nil, e.unknownPropertyAmong(expression, name, columns)
		}
		selected = append(selected, index)
	}
	return selected, nil
}

// valueText is the plain text a cell prints for one value: an element by its
// effective name, a number or boolean as written, a quantity with its unit.
func (e *executor) valueText(value Value) string {
	return ValueText(value, func(sym *symbols.Symbol) string {
		if names, _, err := e.propertyValues(value, query.PropertyName); err == nil && len(names) > 0 {
			if name, ok := names[0].String(); ok && name != "" {
				return name
			}
		}
		return e.elementText(sym)
	})
}
