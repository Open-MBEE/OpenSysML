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
