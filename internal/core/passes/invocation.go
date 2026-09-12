package passes

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// SelectInvocation is the declaration e calls in scope, chosen among every visible
// declaration of its name the call site can run by its arguments' static types; the
// runtime dispatches on it too.
func SelectInvocation(resolver *resolve.Resolver, model *semantics.Model, scope *symbols.Scope, e *ast.InvocationExpr, performs semantics.Performs) *semantics.InvocationSelection {
	silent := exprChecker{resolver: resolver, model: model}
	return silent.selectInvocation(scope, e, silent.argumentTypes(scope, e), performs)
}

// NewArgumentTyper is the checker's argument typing for model to select calls with, so
// a consumer reading a call through the model selects what the checker selects.
func NewArgumentTyper(resolver *resolve.Resolver, model *semantics.Model) semantics.ArgumentTyper {
	return argumentTyper{resolver: resolver, model: model}
}

// argumentTyper is the checker's argument typing as the semantic model consumes
// it, so a call it reads on its own selects what the checker selects.
type argumentTyper struct {
	resolver *resolve.Resolver
	model    *semantics.Model
}

func (t argumentTyper) InvocationArguments(scope *symbols.Scope, e *ast.InvocationExpr) []semantics.Argument {
	silent := exprChecker{resolver: t.resolver, model: t.model}
	return silent.argumentTypes(scope, e).arguments()
}

// argumentTypes are the types of e's arguments: the positional ones, the
// receiver of `x->f(a)` first, and the named ones in the order written.
type argumentTypes struct {
	positional []semantics.Argument
	named      []semantics.Argument
}

// argumentTypes types e's arguments once, so nested errors report once.
func (ec *exprChecker) argumentTypes(scope *symbols.Scope, e *ast.InvocationExpr) argumentTypes {
	args := InvocationArgs(e)
	types := argumentTypes{
		positional: make([]semantics.Argument, len(args)),
		named:      make([]semantics.Argument, len(e.NamedArgs)),
	}
	for i, arg := range args {
		types.positional[i] = ec.argument(scope, arg, nil)
	}
	for i, arg := range e.NamedArgs {
		types.named[i] = ec.argument(scope, arg.Value, arg.Name)
	}
	return types
}

// InvocationArgs returns e's positional arguments, the receiver of `x->f(a)`
// first; the operand of a chain call `x.f(a)` is the calc applied, not an argument.
func InvocationArgs(e *ast.InvocationExpr) []ast.Node {
	if e.Operand == nil || ChainCallee(e) != nil {
		return e.Args
	}
	return append([]ast.Node{e.Operand}, e.Args...)
}

// ChainCallee is the feature chain a call `x.f(a)` applies (KerMLExpressions
// InstantiatedTypeMember → OwnedFeatureChain), nil for `T(a)` and `x->T(a)`.
func ChainCallee(e *ast.InvocationExpr) *ast.FeatureChainExpr {
	if e.Type != nil {
		return nil
	}
	chain, _ := e.Operand.(*ast.FeatureChainExpr)
	return chain
}

// selectInvocation records the declaration e calls given the types of its arguments.
func (ec *exprChecker) selectInvocation(scope *symbols.Scope, e *ast.InvocationExpr, argTypes argumentTypes, performs semantics.Performs) *semantics.InvocationSelection {
	return ec.model.SelectInvocation(scope, e, argTypes.arguments(), performs)
}

// performs is the kind of call site e is: an action performance when it is the
// value of an action usage the checker is reading, else an expression.
func (ec *exprChecker) performs(e *ast.InvocationExpr) semantics.Performs {
	if ec.performed[e] {
		return semantics.PerformsAction
	}
	return semantics.PerformsBehavior
}

// arguments lists the arguments for selection: the positional ones, then the named
// ones that name a parameter.
func (t argumentTypes) arguments() []semantics.Argument {
	typed := make([]semantics.Argument, 0, len(t.positional)+len(t.named))
	typed = append(typed, t.positional...)
	for _, arg := range t.named {
		if arg.Name != nil && len(arg.Name.Parts) > 0 {
			typed = append(typed, arg)
		}
	}
	return typed
}

// argument describes value as an argument: its scalar type, and the declared type
// of the feature it names, or of the result of the call it makes. A collection
// literal binds its elements, so it is typed by the type they have in common.
func (ec *exprChecker) argument(scope *symbols.Scope, value ast.Node, name *ast.QualifiedName) semantics.Argument {
	elements := []ast.Node{value}
	switch v := value.(type) {
	case *ast.SequenceExpr:
		elements = v.Elements
	case *ast.NullExpr:
		elements = nil
	}
	arg := ec.argumentOf(scope, elements)
	arg.Exact = len(elements) > 0 && allSpellOneValue(elements)
	arg.Empty = len(elements) == 0
	arg.Name = name
	return arg
}

// argumentOf types the values an argument writes together: by the scalar type and the declared
// type they share, unknown where they share none.
func (ec *exprChecker) argumentOf(scope *symbols.Scope, elements []ast.Node) semantics.Argument {
	return semantics.Argument{
		Prim: ec.commonElementType(scope, elements),
		Type: ec.commonElementTypeSymbol(scope, elements),
	}
}

// declaredValueType is the declared type of the feature value names, of the result
// of the call it makes, the Evaluation a body `{ … }` is, or the DerivedUnit
// a unit expression `m * s` composes, directly or as the value of an untyped
// feature; nil when none.
func (ec *exprChecker) declaredValueType(scope *symbols.Scope, value ast.Node) *symbols.Symbol {
	if _, ok := value.(*ast.BodyExpr); ok {
		return ec.model.ScalarSymbol(semantics.PrimExpression)
	}
	if e, ok := value.(*ast.OperatorExpr); ok {
		return ec.model.MeasurementRefExprType(scope, e)
	}
	if declared := ec.valueTypeSymbol(scope, value); declared != nil {
		return declared
	}
	if feature := ec.valueFeature(scope, value); feature != nil {
		return ec.model.MeasurementRefFeatureType(feature)
	}
	return ec.invocationResultTypeSymbol(scope, value)
}

// commonElementTypeSymbol is the declared type every element conforms to, nil when one
// has none or they share none.
func (ec *exprChecker) commonElementTypeSymbol(scope *symbols.Scope, elements []ast.Node) *symbols.Symbol {
	var common *symbols.Symbol
	for _, el := range elements {
		elem := ec.declaredValueType(scope, el)
		switch {
		case elem == nil:
			return nil
		case common == nil, ec.model.Conforms(common, elem):
			common = elem
		case !ec.model.Conforms(elem, common):
			return nil
		}
	}
	return common
}

func allSpellOneValue(elements []ast.Node) bool {
	for _, el := range elements {
		if !spellsOneValue(el) {
			return false
		}
	}
	return true
}

// candidateNames lists declarations for a diagnostic by qualified name, in order.
func candidateNames(syms []*symbols.Symbol) string {
	names := make([]string, 0, len(syms))
	for _, sym := range syms {
		names = append(names, symbols.FQNOf(sym))
	}
	return strings.Join(names, ", ")
}
