package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
)

// evaluateNamed resolves each qualified name, in argument order, to the one
// element it denotes, as WhereType resolves a type name; a name denoting no
// single element is an unknown-element error.
func (e *executor) evaluateNamed(expression queryplan.Expression) (sequence, error) {
	names, err := e.stringsArgument(expression, "qualifiedName")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	for _, name := range names {
		element, ok := e.context.Resolver.ResolveAliasTarget(e.resolveClassification(name))
		if !ok || element == nil {
			return sequence{}, &Error{
				Kind:      ErrorUnknownElement,
				Query:     e.definition.Name(),
				Operation: expression.Operation(),
				Actual:    name,
				Origin:    expression.Origin(),
			}
		}
		result.values = append(result.values, valueAt(ElementValue(element), expression.Origin()))
	}
	return result, nil
}
