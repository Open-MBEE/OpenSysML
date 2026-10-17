package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// operatorFunctionPackages are the Kernel Function Library packages an operator
// names its function in, searched in this order (KerML 8.3.4.8.3).
var operatorFunctionPackages = []string{"BaseFunctions", "DataFunctions", "ControlFunctions"}

// OperatorFunction is the library function an operator expression invokes:
// the first of BaseFunctions, DataFunctions and ControlFunctions declaring the
// operator's name; nil when the loaded library declares none.
func (m *Model) OperatorFunction(op ast.OperatorKind) *symbols.Symbol {
	if m == nil || op == ast.OpInvalid {
		return nil
	}
	name := operatorFunctionName(op)
	for _, pkg := range operatorFunctionPackages {
		if fn := m.libSymbol(pkg + "::" + name); fn != nil {
			return fn
		}
	}
	return nil
}

func operatorFunctionName(op ast.OperatorKind) string {
	if op == ast.OpIndex {
		return "["
	}
	return op.String()
}

// OperatorFunctionNamed is the library function the Kernel Function Library
// associates with an operator spelling.
func (m *Model) OperatorFunctionNamed(name string) *symbols.Symbol {
	if m == nil || name == "" {
		return nil
	}
	for _, pkg := range operatorFunctionPackages {
		if fn := m.libSymbol(pkg + "::" + name); fn != nil {
			return fn
		}
	}
	return nil
}

// OperatorInputNamedAt is the function input bound at an operator operand position.
func (m *Model) OperatorInputNamedAt(name string, index int) *symbols.Symbol {
	if m == nil || index < 0 {
		return nil
	}
	inputs := m.InputParametersOf(m.OperatorFunctionNamed(name))
	if index >= len(inputs) {
		return nil
	}
	return inputs[index]
}

// OperatorInputAt is the function input bound to an operator operand position.
func (m *Model) OperatorInputAt(op ast.OperatorKind, index int) *symbols.Symbol {
	if op == ast.OpInvalid {
		return nil
	}
	return m.OperatorInputNamedAt(operatorFunctionName(op), index)
}

// InputParametersOf is a behavior's `in` and `inout` parameters, inherited and
// declared, in the order arguments bind them.
func (m *Model) InputParametersOf(behavior *symbols.Symbol) []*symbols.Symbol {
	var params []*symbols.Symbol
	for _, p := range m.BehaviorParametersOf(behavior) {
		if p.IsResult || p.Symbol == nil || (p.Direction != ast.DirIn && p.Direction != ast.DirInOut) {
			continue
		}
		params = append(params, p.Symbol)
	}
	return params
}
