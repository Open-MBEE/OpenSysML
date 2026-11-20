package metamodel

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A change trigger's argument wrapper shares the trigger's node but reflects as a
// FeatureReferenceExpression: the symbol-backed structure types it, like the condition,
// by Performances::Evaluation, and only the trigger invocation by Triggers::TriggerWhen.
func TestSymbolStructureChangeTriggerArgumentIsNotTypedAsTheTrigger(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "change-trigger.sysml", `package Probe {
    attribute ready : ScalarValues::Boolean;
    action def Wait {
        accept when ready;
    }
}`)
	structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
	e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	roots := e.resolver.Index().LookupQualified("Probe")
	if len(roots) != 1 {
		t.Fatalf("Probe resolves to %d symbols", len(roots))
	}
	typeNames := func(element Element, metaclass, name string) []string {
		value, ok := e.Property(element, metaclass, name)
		if !ok || !value.Success {
			t.Fatalf("%s::%s = %#v, %t: want it derived", metaclass, name, value, ok)
		}
		var names []string
		values := value.Values
		if value.Kind == ElementValue {
			values = []Value{value}
		}
		for _, typ := range values {
			if typ.Kind != ElementValue || typ.Element.Symbol == nil {
				t.Fatalf("%s::%s value %#v is no symbol element", metaclass, name, typ)
			}
			names = append(names, typ.Element.Symbol.Name)
		}
		return names
	}
	triggers, references := 0, 0
	var changeEvent *ast.ChangeEvent
	seen := map[*symbols.Symbol]bool{}
	var visit func(sym *symbols.Symbol)
	visit = func(sym *symbols.Symbol) {
		if sym == nil || seen[sym] {
			return
		}
		seen[sym] = true
		element := structure.ElementFor(sym)
		switch metaclass, _ := structure.Metaclass(element); metaclass {
		case "TriggerInvocationExpression":
			triggers++
			changeEvent, _ = sym.Decl.(*ast.ChangeEvent)
			if got := typeNames(element, "Feature", "type"); len(got) != 1 || got[0] != "TriggerWhen" {
				t.Errorf("TriggerInvocationExpression Feature::type = %v, want [TriggerWhen]", got)
			}
			if got := typeNames(element, "Expression", "function"); len(got) != 1 || got[0] != "TriggerWhen" {
				t.Errorf("TriggerInvocationExpression Expression::function = %v, want [TriggerWhen]", got)
			}
		case "FeatureReferenceExpression":
			references++
			if got := typeNames(element, "Feature", "type"); len(got) != 1 || got[0] != "Evaluation" {
				t.Errorf("FeatureReferenceExpression (role %v) Feature::type = %v, want [Evaluation]", sym.Expr.Role, got)
			}
			if got := typeNames(element, "Expression", "function"); len(got) != 1 || got[0] != "Evaluation" {
				t.Errorf("FeatureReferenceExpression (role %v) Expression::function = %v, want [Evaluation]", sym.Expr.Role, got)
			}
		}
		for _, rel := range e.model.OwnedRelationships(sym) {
			visit(rel)
			if related, ok := e.model.ReflectiveElements(rel, "ownedRelatedElement"); ok {
				for _, owned := range related {
					visit(owned)
				}
			}
		}
	}
	visit(roots[0])
	if triggers != 1 || references != 2 || changeEvent == nil {
		t.Fatalf("walked %d TriggerInvocationExpression and %d FeatureReferenceExpression, want 1 and 2 (the argument wrapper and the condition)", triggers, references)
	}

	// A structure handing out the wrapper with the trigger's node still reflects it as the
	// FeatureReferenceExpression it is: the node alone does not make it the trigger.
	trigger := Element{Node: changeEvent, Container: roots[0], Aspect: "change-trigger"}
	wrapper := Element{Node: changeEvent, Container: roots[0], Aspect: "change-wrapper"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			trigger.Key(): "TriggerInvocationExpression",
			wrapper.Key(): "FeatureReferenceExpression",
		},
	}
	impliedNames := func(element Element) []string {
		values, ok := evaluator.structureImpliedFeatureTypes(element)
		if !ok {
			t.Fatalf("implied types of %s are undetermined", element.Aspect)
		}
		var names []string
		for _, value := range values {
			if value.Kind != ElementValue || value.Element.Symbol == nil {
				t.Fatalf("implied type %#v of %s is no symbol element", value, element.Aspect)
			}
			names = append(names, value.Element.Symbol.Name)
		}
		return names
	}
	if got := impliedNames(trigger); !slices.Equal(got, []string{"TriggerWhen", "Evaluation"}) {
		t.Errorf("trigger's implied types = %v, want [TriggerWhen Evaluation]", got)
	}
	if got := impliedNames(wrapper); !slices.Equal(got, []string{"Evaluation"}) {
		t.Errorf("argument wrapper's implied types = %v, want [Evaluation]", got)
	}
}
