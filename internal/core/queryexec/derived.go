package queryexec

import (
	"errors"

	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// derivedValues holds the runtime reader behind derived attribute values, made
// on first use and shared by every executor of one execution.
type derivedValues struct {
	reader *runtime.DeclaredReader
}

func (d *derivedValues) get(context Context) *runtime.DeclaredReader {
	if d.reader == nil {
		d.reader = runtime.NewDeclaredReader(context.Model, context.Resolver)
	}
	return d.reader
}

// derivedFeatureValues evaluates a feature the constant folder could not, as
// seen from the row's element. An unbound leaf reads as absent; anything else
// the runtime cannot evaluate is a typed error carrying the runtime's reason.
func (e *executor) derivedFeatureValues(sym *symbols.Symbol, property string) ([]Value, error) {
	value, err := e.derived.get(e.context).Read(sym, property)
	if err != nil {
		var noValue *runtime.NoValueError
		if errors.As(err, &noValue) && !semantics.IsParameter(noValue.Symbol) {
			return nil, nil
		}
		return nil, e.unevaluable(queryplan.Expression{}, property, sym, err)
	}
	return e.cellValues(value, property, sym)
}

// cellValues converts a runtime value to the values of one cell: collections
// flatten, and a result no cell can hold — an object, an infinity — is a
// typed error.
func (e *executor) cellValues(value runtime.Value, property string, sym *symbols.Symbol) ([]Value, error) {
	origin := ElementValue(sym).Origin()
	if lit := value.EnumerationLiteral(); lit != nil {
		return []Value{valueAt(StringValue(symbols.FQNOf(lit)), origin)}, nil
	}
	var elements []runtime.Value
	switch value.Kind {
	case runtime.ValNull:
		return nil, nil
	case runtime.ValSequence:
		elements = value.Sequence().Elements()
	case runtime.ValSet:
		elements = value.Set().Elements()
	case runtime.ValConst:
		converted, ok := constValue(value.Const)
		if !ok {
			return nil, e.unevaluable(queryplan.Expression{}, property, sym, notAValue(value))
		}
		return []Value{valueAt(converted, origin)}, nil
	case runtime.ValString:
		return []Value{valueAt(StringValue(value.Str()), origin)}, nil
	case runtime.ValQuantity:
		return []Value{valueAt(QuantityValue(*value.Quantity()), origin)}, nil
	default:
		return nil, e.unevaluable(queryplan.Expression{}, property, sym, notAValue(value))
	}
	var result []Value
	for _, element := range elements {
		values, err := e.cellValues(element, property, sym)
		if err != nil {
			return nil, err
		}
		result = append(result, values...)
	}
	return result, nil
}

// constValue converts a folded constant; an infinity has no query value.
func constValue(value semantics.Value) (Value, bool) {
	switch value.Kind {
	case semantics.ValBool:
		return BooleanValue(value.Bool), true
	case semantics.ValInt:
		return IntegerValue(value.Int), true
	case semantics.ValReal:
		return RealValue(value.Real), true
	default:
		return Value{}, false
	}
}

// errNotAValue reports a runtime result no cell can hold: an object, a
// deferred expression, an infinity.
type errNotAValue struct{ kind string }

func (e *errNotAValue) Error() string { return "evaluates to " + e.kind + ", not a value" }

func notAValue(value runtime.Value) error {
	kind := value.Kind.String()
	if value.Kind == runtime.ValConst && value.Const.Kind == semantics.ValInfinity {
		kind = "an infinity"
	}
	return &errNotAValue{kind: kind}
}
