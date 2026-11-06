package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// structureImpliedFeatureTypes returns the types an expression declaring none takes
// from what it implicitly specializes (KerML 1.1 §8.4.4.7): the function it invokes
// (§8.4.4.7.3), then the types of the evaluations feature its metaclass implies.
func (e *Evaluator) structureImpliedFeatureTypes(element Element) ([]Value, bool) {
	if e.model == nil || e.resolver == nil || e.resolver.Index() == nil {
		return nil, true
	}
	metaclass, known := e.options.Structure.Metaclass(element)
	if !known {
		return nil, false
	}
	fqn, expression := e.model.ImpliedExpressionBaseFQN(metaclass)
	if !expression {
		return nil, true
	}
	values, ok := e.structureExpressionFunction(element)
	if !ok {
		return nil, false
	}
	bases := e.resolver.Index().LookupQualified(fqn)
	if len(bases) == 0 {
		// Without the standard library there is no implied base to take types from.
		if !e.resolver.Index().HasLibrary() {
			return values, true
		}
		return nil, false
	}
	base := bases[0]
	if !base.IsFeature() {
		return append(values, elementValue(ElementOf(base))), true
	}
	if !e.completeFeatureTypeInputs(base) {
		return nil, false
	}
	for _, typ := range e.model.FeatureTypeSet(base) {
		if typ == nil {
			return nil, false
		}
		values = append(values, elementValue(ElementOf(typ)))
	}
	return values, true
}

// structureExpressionFunction resolves the function an expression invokes: an
// operator's from its node or the operator a graph-only element states, an
// invocation's or a chain's from the callee the graph states.
func (e *Evaluator) structureExpressionFunction(element Element) ([]Value, bool) {
	var function *symbols.Symbol
	switch node := expressionNodeOf(element).(type) {
	case *ast.OperatorExpr:
		function = e.model.OperatorFunction(node.Operator)
	case *ast.IndexExpr:
		if node.Bracket {
			function = e.model.OperatorFunction(ast.OpIndex)
		}
	case *ast.SequenceExpr:
		function = e.model.OperatorFunctionNamed(",")
	case *ast.FeatureChainExpr:
		function = e.model.OperatorFunctionNamed(".")
	case *ast.SelectExpr:
		function = e.librarySymbol("ControlFunctions::select")
	case *ast.CollectExpr:
		function = e.librarySymbol("ControlFunctions::collect")
	case *ast.TimeEvent:
		if node.Absolute {
			function = e.librarySymbol("Triggers::TriggerAt")
		} else {
			function = e.librarySymbol("Triggers::TriggerAfter")
		}
	case *ast.ChangeEvent:
		function = e.librarySymbol("Triggers::TriggerWhen")
	case nil:
		function = e.statedExpressionFunction(element)
	}
	if function != nil && semantics.IsType(function) {
		return []Value{elementValue(ElementOf(function))}, true
	}
	// A resolved callee is stated as an element; an unresolved one, spelled by name, types nothing.
	targets, ok := e.options.Structure.RelatedElements(element, "function")
	if !ok || len(targets) == 0 {
		return nil, true
	}
	// A library callee is read through its symbol, so its supertypes are known.
	for i, target := range targets {
		if symbol := e.structureElementSymbol(target); symbol != nil {
			targets[i] = ElementOf(symbol)
		}
	}
	functions, ok := e.structureFilterElements(targets, "Function")
	if !ok {
		return nil, false
	}
	return functions, true
}

// statedExpressionFunction resolves the function a graph-only expression invokes by
// what the graph states of it: its operator, or a trigger invocation's kind.
func (e *Evaluator) statedExpressionFunction(element Element) *symbols.Symbol {
	metaclass, known := e.options.Structure.Metaclass(element)
	if !known {
		return nil
	}
	switch metaclass {
	case "FeatureChainExpression":
		return e.model.OperatorFunctionNamed(".")
	case "SelectExpression":
		return e.librarySymbol("ControlFunctions::select")
	case "CollectExpression":
		return e.librarySymbol("ControlFunctions::collect")
	case "OperatorExpression":
		if stated, ok := e.options.Structure.Attribute(element, "operator"); ok && stated.Kind == StringValue && stated.String != "" {
			return e.model.OperatorFunctionNamed(stated.String)
		}
	case "TriggerInvocationExpression":
		if stated, ok := e.options.Structure.Attribute(element, "kind"); ok {
			switch stated.Enum + stated.String {
			case "after":
				return e.librarySymbol("Triggers::TriggerAfter")
			case "at":
				return e.librarySymbol("Triggers::TriggerAt")
			case "when":
				return e.librarySymbol("Triggers::TriggerWhen")
			}
		}
	}
	return nil
}

// librarySymbol is the library element fqn names, or nil without the library.
func (e *Evaluator) librarySymbol(fqn string) *symbols.Symbol {
	if e.resolver == nil || e.resolver.Index() == nil {
		return nil
	}
	for _, symbol := range e.resolver.Index().LookupQualified(fqn) {
		if symbol != nil {
			return symbol
		}
	}
	return nil
}
