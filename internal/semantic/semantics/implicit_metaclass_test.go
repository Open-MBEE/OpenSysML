package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A graph element's metaclass alone names its implied kind base, for definitions
// (SysML 8.3.6.2) as for usages, so a type with no stated general still inherits.
func TestImpliedBaseFQNsForMetaclassCoversDefinitions(t *testing.T) {
	model := &Model{}
	for metaclass, want := range map[string]string{
		"PartDefinition":   "Parts::Part",
		"PartUsage":        "Parts::parts",
		"ActionDefinition": "Actions::Action",
		"PortDefinition":   "Ports::Port",
	} {
		if got := model.ImpliedBaseFQNsForMetaclass(metaclass, source.KindSysML); !slices.Equal(got, []string{want}) {
			t.Errorf("ImpliedBaseFQNsForMetaclass(%s) = %v, want [%s]", metaclass, got, want)
		}
	}
	if got := model.ImpliedBaseFQNsForMetaclass("Package", source.KindSysML); got != nil {
		t.Errorf("ImpliedBaseFQNsForMetaclass(Package) = %v, want none", got)
	}
}

// An action node's metaclass names its kind's action base (SysML v2 §8.2.2.17)
// and SuccessionFlowUsage the base a `succession flow` subsets (§8.3.15.7).
func TestImpliedBaseFQNsForMetaclassCoversActionNodesAndSuccessionFlows(t *testing.T) {
	model := &Model{}
	for metaclass, want := range map[string]string{
		"AcceptActionUsage":     "Actions::actions",
		"AssignmentActionUsage": "Actions::actions",
		"IfActionUsage":         "Actions::actions",
		"WhileLoopActionUsage":  "Actions::actions",
		"ForLoopActionUsage":    "Actions::actions",
		"TerminateActionUsage":  "Actions::actions",
		"FlowUsage":             "Flows::flows",
		"SuccessionFlowUsage":   "Flows::successionFlows",
	} {
		if got := model.ImpliedBaseFQNsForMetaclass(metaclass, source.KindSysML); !slices.Equal(got, []string{want}) {
			t.Errorf("ImpliedBaseFQNsForMetaclass(%s) = %v, want [%s]", metaclass, got, want)
		}
	}
}
