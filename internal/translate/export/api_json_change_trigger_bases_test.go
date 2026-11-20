package export

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A change trigger's argument wrapper is a FeatureReferenceExpression over the
// condition: like the condition it is typed by Performances::Evaluation, while the
// trigger invocation alone is typed by Triggers::TriggerWhen.
func TestAPIJSONChangeTriggerArgumentIsNotTypedAsTheTrigger(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		attribute ready : ScalarValues::Boolean;
		action def Wait {
			accept when ready;
		}
	}`, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	library := map[string]string{}
	for _, el := range identity.LibraryCatalog(libs.NewModelIndex()).Elements() {
		library[el.FQN] = el.ID
	}
	libraryID := func(fqn string) string {
		id, ok := library[fqn]
		if !ok {
			t.Fatalf("%s is not catalogued", fqn)
		}
		return id
	}
	libraryName := func(id string) string {
		for fqn, candidate := range library {
			if candidate == id {
				return fqn
			}
		}
		return id
	}
	triggers, references := 0, 0
	for _, element := range booleansElements(t, document) {
		metaclass, _ := element["@type"].(string)
		function, _ := element["function"].(map[string]any)
		functionID, _ := function["@id"].(string)
		switch metaclass {
		case "TriggerInvocationExpression":
			triggers++
			if got := referenceIDs(t, element, "type"); !slices.Equal(got, []string{libraryID("Triggers::TriggerWhen")}) {
				t.Errorf("TriggerInvocationExpression.type = %v, want [Triggers::TriggerWhen]", got)
			}
			if functionID != libraryID("Triggers::TriggerWhen") {
				t.Errorf("TriggerInvocationExpression.function = %v, want Triggers::TriggerWhen", element["function"])
			}
		case "FeatureReferenceExpression":
			references++
			if got := referenceIDs(t, element, "type"); !slices.Equal(got, []string{libraryID("Performances::Evaluation")}) {
				t.Errorf("FeatureReferenceExpression %v.type = %v, want [Performances::Evaluation]", element["@id"], libraryName(strings.Join(got, ",")))
			}
			if functionID != libraryID("Performances::Evaluation") {
				t.Errorf("FeatureReferenceExpression %v.function = %s, want Performances::Evaluation", element["@id"], libraryName(functionID))
			}
		}
	}
	if triggers != 1 || references != 2 {
		t.Fatalf("element form has %d TriggerInvocationExpression and %d FeatureReferenceExpression, want 1 and 2 (the argument wrapper and the condition)", triggers, references)
	}
}
