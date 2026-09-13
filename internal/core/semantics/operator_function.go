package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
	name := op.String()
	for _, pkg := range operatorFunctionPackages {
		if fn := m.libSymbol(pkg + "::" + name); fn != nil {
			return fn
		}
	}
	return nil
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
