package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// PropertyValuation projects what a row feature's own valuation binds (KerML
// Feature::valuation): the value its declaration writes, or a redefinition
// binds, evaluated where it is an expression. A row that is not a feature, or
// a feature bound to nothing, reads as absent.
const PropertyValuation = "valuation"

// valuationValues reads the values the row feature's own FeatureValue binds, the
// derived reader evaluating a value the constant folder cannot spell.
func (e *executor) valuationValues(sym *symbols.Symbol) ([]Value, error) {
	values, present, err := e.context.Model.OwnFeatureValues(sym)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	result := make([]Value, 0, len(values))
	for _, value := range values {
		converted, ok := e.filterValue(value, sym)
		if !ok {
			switch value.Kind {
			case symbols.FilterValueEmpty:
				continue
			case symbols.FilterValueUnknown:
				return e.derivedValuation(sym)
			}
			return nil, e.featureError(queryplan.Expression{}, PropertyValuation, ElementValue(sym))
		}
		result = append(result, converted)
	}
	return result, nil
}

// derivedValuation evaluates the row feature as a member of its owner, the way
// a property naming it on the owner's row would.
func (e *executor) derivedValuation(sym *symbols.Symbol) ([]Value, error) {
	owner := sym.Owner()
	if owner == nil || sym.Name == "" {
		return nil, nil
	}
	return e.derivedFeatureValues(owner, sym.Name)
}
