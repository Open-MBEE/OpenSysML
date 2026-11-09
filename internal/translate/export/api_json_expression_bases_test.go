package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// An expression subsets the evaluations of its kind (KerML 8.4.4.7, 8.4.4.8): the element
// form types it by its function or evaluation kind and inherits from it, graph or notation.
func TestAPIJSONExpressionsInheritThroughTheirImpliedBase(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def V {
			attribute x : ScalarValues::Integer = 1 + 2;
			attribute n = null;
			attribute s = x#(1);
		}
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	byType := map[string]map[string]any{}
	for _, element := range elements {
		if metaclass, _ := element["@type"].(string); metaclass != "" {
			byType[metaclass] = element
		}
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
	for metaclass, wantType := range map[string]string{
		"OperatorExpression": "DataFunctions::+",
		"LiteralInteger":     "Performances::LiteralIntegerEvaluation",
		"NullExpression":     "Performances::NullEvaluation",
		"IndexExpression":    "BaseFunctions::#",
	} {
		element := byType[metaclass]
		if element == nil {
			t.Fatalf("no %s in the element form", metaclass)
		}
		if got := referenceIDs(t, element, "type"); !slices.Equal(got, []string{libraryID(wantType)}) {
			t.Errorf("%s.type = %v, want [%s]", metaclass, got, wantType)
		}
		function, _ := element["function"].(map[string]any)
		if id, _ := function["@id"].(string); id != libraryID(wantType) {
			t.Errorf("%s.function = %v, want %s", metaclass, element["function"], wantType)
		}
		if inherited := referenceIDs(t, element, "inheritedMembership"); len(inherited) == 0 {
			t.Errorf("%s.inheritedMembership is empty, want Evaluation's members through %s", metaclass, wantType)
		}
	}
}
