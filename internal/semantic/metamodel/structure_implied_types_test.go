package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// An expression declaring no type is typed, through the symbol structure as through the graph
// one, by the evaluations feature its metaclass implies (KerML 1.1 §8.4.4.7): the semantic
// model's empty answer does not stand in for the implied one.
func TestSymbolStructureTypesAnExpressionByItsImpliedEvaluations(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "literal.sysml", `package P {
	attribute x = 1;
}`)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	x := testSymbol(t, testSymbol(t, root, "P").Scope, "x")
	literal := evaluator.model.ValueExpression(x)
	if literal == nil {
		t.Fatal("x has no value expression")
	}
	for _, property := range []struct{ class, name string }{{"Feature", "type"}, {"Expression", "function"}, {"Step", "behavior"}} {
		value, ok := e.Property(structure.ElementFor(literal), property.class, property.name)
		if !ok || !value.Success {
			t.Errorf("%s::%s = %v, %v; want the one implied type", property.class, property.name, value, ok)
			continue
		}
		elements := []Element{value.Element}
		if value.Kind == SequenceValue {
			elements = valueElements(t, value)
		}
		if len(elements) != 1 || elements[0].Symbol == nil {
			t.Errorf("%s::%s answers %d elements, want the one implied type", property.class, property.name, len(elements))
			continue
		}
		if got := symbols.FQNOf(elements[0].Symbol); got != "Performances::LiteralIntegerEvaluation" {
			t.Errorf("%s::%s = %s, want Performances::LiteralIntegerEvaluation", property.class, property.name, got)
		}
	}
}
