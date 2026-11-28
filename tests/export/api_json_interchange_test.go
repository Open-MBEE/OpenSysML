package export_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

const interchangeSource = `package Garage {
	part def Wheel;
	part def Car {
		part wheel : Wheel[4];
	}
}
`

func interchangeElements(t *testing.T, options export.APIJSONOptions) ([]byte, []map[string]json.RawMessage) {
	t.Helper()
	return interchangeElementsOf(t, interchangeSource, options)
}

func interchangeElementsOf(t *testing.T, source string, options export.APIJSONOptions) ([]byte, []map[string]json.RawMessage) {
	t.Helper()
	graph, err := convert.ModelGraph(
		[]convert.Input{{Name: "garage.sysml", Data: []byte(source)}}, convert.Options{ID: export.IDUUID})
	if err != nil {
		t.Fatal(err)
	}
	document, err := export.WriteAPIJSONWithOptions(graph, options)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	return document, elements
}

// KerML 10.4.1: an element of the interchange form carries its uuid, its metaclass and
// properties of that metaclass, and nothing of the mapping's own.
func TestAPIJSONInterchangeStatesOnlyMetaclassProperties(t *testing.T) {
	t.Parallel()
	uuid := regexp.MustCompile(`^"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"$`)
	_, elements := interchangeElements(t, export.APIJSONOptions{Interchange: true})
	if len(elements) == 0 {
		t.Fatal("no elements")
	}
	for _, element := range elements {
		var metaclass string
		if err := json.Unmarshal(element["@type"], &metaclass); err != nil {
			t.Fatal(err)
		}
		if !uuid.Match(element["@id"]) {
			t.Errorf("%s: @id %s is no uuid", metaclass, element["@id"])
		}
		for key := range element {
			if key == "@id" || key == "@type" {
				continue
			}
			if _, declared := ontology.PropertyOf(metaclass, key); !declared {
				t.Errorf("%s %s states %q, which %s does not declare", metaclass, element["@id"], key, metaclass)
			}
		}
	}
}

// KerML 10.4.5: a derived property is stated even when it has no value, a multi-valued
// one as [] and a single-valued one as null, and the inherited members are stated.
func TestAPIJSONInterchangeStatesEmptyAndInheritedDerivedProperties(t *testing.T) {
	t.Parallel()
	_, elements := interchangeElements(t, export.APIJSONOptions{Interchange: true})
	var wheel, car map[string]json.RawMessage
	for _, element := range elements {
		switch string(element["declaredName"]) {
		case `"Wheel"`:
			wheel = element
		case `"Car"`:
			car = element
		}
	}
	if wheel == nil || car == nil {
		t.Fatal("Wheel or Car is not in the document")
	}
	for property, want := range map[string]string{
		"ownedFeature": "[]", "ownedUsage": "[]", "documentation": "[]", "ownedConjugator": "null", "shortName": "null",
	} {
		if got := string(bytes.TrimSpace(wheel[property])); got != want {
			t.Errorf("Wheel.%s = %q, want %s", property, got, want)
		}
	}
	for _, property := range []string{"feature", "inheritedFeature", "membership", "member", "ownedPart"} {
		value, stated := car[property]
		if !stated || !strings.Contains(string(value), `"@id"`) {
			t.Errorf("Car.%s = %q, want references", property, value)
		}
	}
}

// The option changes nothing unless given: the default form keeps the mapping's keys.
func TestAPIJSONInterchangeLeavesTheDefaultFormAlone(t *testing.T) {
	t.Parallel()
	document, _ := interchangeElements(t, export.APIJSONOptions{})
	if !bytes.Contains(document, []byte(`"sysx:`)) {
		t.Error("the default form lost its sysx: keys")
	}
}

// The interchange form states each body member's derived edges back to the expression
// that owns it; reading one refuses the cycle instead of following it without end.
func TestAPIJSONInterchangeExpressionBodyCycleIsRefused(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"expression_body_members.sysml", "expression_body_order.sysml"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source, err := os.ReadFile(filepath.Join("testdata", "convert", name))
			if err != nil {
				t.Fatal(err)
			}
			document, err := convert.ConvertWith(name, source, convert.FormatSysML, convert.FormatAPIJSON,
				convert.Options{Interchange: true})
			if err != nil {
				t.Fatal(err)
			}
			_, err = convert.Convert("model.json", document, convert.FormatAPIJSON, convert.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected an UnsupportedError, got %v", err)
			}
		})
	}
}

const interchangeDerivedSource = `package Derived {
	private import ScalarValues::*;
	// a note of the source layout
	metadata def Reviewed;
	part def Engine {
		attribute mass : Integer;
		attribute doubled = mass * 2;
		attribute fixed = 2 + 3;
		constraint light { mass < 5 }
		assert light;
		action run { action step; }
		perform run;
		@Reviewed;
	}
	action def Drive { action steer; }
	calc def Twice { in x : Integer; calc inner : Twice; x * 2 }
	package Selected {
		public import Derived::*;
		filter @Reviewed;
	}
}
`

// interchangeTyped returns the elements of one metaclass, in document order.
func interchangeTyped(t *testing.T, elements []map[string]json.RawMessage, metaclass string) []map[string]json.RawMessage {
	t.Helper()
	var typed []map[string]json.RawMessage
	for _, element := range elements {
		if string(element["@type"]) == `"`+metaclass+`"` {
			typed = append(typed, element)
		}
	}
	if len(typed) == 0 {
		t.Fatalf("the document holds no %s", metaclass)
	}
	return typed
}

// interchangeNamed maps each element's @id to its declaredName, "" for an unnamed one.
func interchangeNamed(t *testing.T, elements []map[string]json.RawMessage) map[string]string {
	t.Helper()
	names := map[string]string{}
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		var name *string
		if raw, stated := element["declaredName"]; stated {
			if err := json.Unmarshal(raw, &name); err != nil {
				t.Fatal(err)
			}
		}
		if name != nil {
			names[id] = *name
		} else {
			names[id] = ""
		}
	}
	return names
}

// interchangeReferences reads a property as the names of the document's elements it
// references; a reference out of the document reads as "<library>".
func interchangeReferences(t *testing.T, names map[string]string, raw json.RawMessage) []string {
	t.Helper()
	type reference struct {
		ID string `json:"@id"`
	}
	var many []reference
	if err := json.Unmarshal(raw, &many); err != nil {
		var one *reference
		if err := json.Unmarshal(raw, &one); err != nil {
			t.Fatalf("%s is no reference: %v", raw, err)
		}
		if one != nil {
			many = []reference{*one}
		}
	}
	var out []string
	for _, ref := range many {
		name, held := names[ref.ID]
		if !held {
			name = "<library>"
		}
		out = append(out, name)
	}
	return out
}

// KerML 8.3.4.7.3: an Expression of the interchange form states whether it is model-level
// evaluable: a literal and an operator over literals are, a reference to a feature of a type is not.
func TestAPIJSONInterchangeStatesModelLevelEvaluability(t *testing.T) {
	t.Parallel()
	_, elements := interchangeElementsOf(t, interchangeDerivedSource, export.APIJSONOptions{Interchange: true})
	for _, literal := range interchangeTyped(t, elements, "LiteralInteger") {
		if got := string(literal["isModelLevelEvaluable"]); got != "true" {
			t.Errorf("LiteralInteger.isModelLevelEvaluable = %q, want true", got)
		}
	}
	for _, reference := range interchangeTyped(t, elements, "FeatureReferenceExpression") {
		if got := string(reference["isModelLevelEvaluable"]); got != "false" {
			t.Errorf("FeatureReferenceExpression.isModelLevelEvaluable = %q, want false for a feature of a type", got)
		}
	}
	evaluable := map[string]int{}
	for _, operator := range interchangeTyped(t, elements, "OperatorExpression") {
		var name string
		if err := json.Unmarshal(operator["operator"], &name); err != nil {
			t.Fatal(err)
		}
		if string(operator["instantiatedType"]) == "" || string(operator["instantiatedType"]) == "null" {
			t.Errorf("OperatorExpression %s states no instantiatedType", name)
		}
		evaluable[name+" "+string(operator["isModelLevelEvaluable"])]++
	}
	want := map[string]int{"+ true": 1, "* false": 2, "< false": 1, "@ true": 1}
	if len(evaluable) != len(want) {
		t.Fatalf("OperatorExpression evaluability = %v, want %v", evaluable, want)
	}
	for key, count := range want {
		if evaluable[key] != count {
			t.Errorf("OperatorExpression evaluability = %v, want %v", evaluable, want)
		}
	}
	for _, calculation := range interchangeTyped(t, elements, "CalculationDefinition") {
		if got := string(calculation["isModelLevelEvaluable"]); got != "false" {
			t.Errorf("CalculationDefinition.isModelLevelEvaluable = %q, want false outside the Kernel Function Library", got)
		}
	}
}

// The usages that reference another state it under the property their metaclass
// declares, and a definition states the steps and calculations it features.
func TestAPIJSONInterchangeStatesDerivedReferences(t *testing.T) {
	t.Parallel()
	_, elements := interchangeElementsOf(t, interchangeDerivedSource, export.APIJSONOptions{Interchange: true})
	names := interchangeNamed(t, elements)
	single := func(metaclass, property, want string) {
		t.Helper()
		for _, element := range interchangeTyped(t, elements, metaclass) {
			got := interchangeReferences(t, names, element[property])
			if len(got) != 1 || got[0] != want {
				t.Errorf("%s.%s = %v, want %s", metaclass, property, got, want)
			}
		}
	}
	single("PerformActionUsage", "performedAction", "run")
	single("PerformActionUsage", "eventOccurrence", "run")
	single("AssertConstraintUsage", "assertedConstraint", "light")
	single("MetadataUsage", "metaclass", "Reviewed")
	contains := func(metaclass, name, property, want string) {
		t.Helper()
		for _, element := range interchangeTyped(t, elements, metaclass) {
			if string(element["declaredName"]) != `"`+name+`"` {
				continue
			}
			for _, got := range interchangeReferences(t, names, element[property]) {
				if got == want {
					return
				}
			}
			t.Errorf("%s %s.%s = %s, want a reference to %s", metaclass, name, property, element[property], want)
			return
		}
		t.Errorf("the document holds no %s %s", metaclass, name)
	}
	contains("ActionDefinition", "Drive", "action", "steer")
	contains("CalculationDefinition", "Twice", "calculation", "inner")
	for _, element := range interchangeTyped(t, elements, "Package") {
		if string(element["declaredName"]) != `"Selected"` {
			continue
		}
		var conditions []json.RawMessage
		if err := json.Unmarshal(element["filterCondition"], &conditions); err != nil || len(conditions) != 1 {
			t.Errorf("Package.filterCondition = %s, want its one filter expression", element["filterCondition"])
		}
	}
	for _, constraint := range interchangeTyped(t, elements, "ConstraintUsage") {
		if _, stated := constraint["predicate"]; !stated {
			t.Error("ConstraintUsage states no predicate")
		}
	}
	for _, metadata := range interchangeTyped(t, elements, "MetadataUsage") {
		if _, stated := metadata["owningAnnotatingRelationship"]; !stated {
			t.Error("MetadataUsage states no owningAnnotatingRelationship")
		}
	}
}

// The normative SysML.json declares neither InvocationExpression::operand nor a note of
// the source layout: the interchange form states neither, and the default form keeps its notes.
func TestAPIJSONInterchangeLeavesOutWhatTheSchemaDoesNotDeclare(t *testing.T) {
	t.Parallel()
	hasNote := func(elements []map[string]json.RawMessage) bool {
		for _, element := range elements {
			if string(element["@type"]) == `"sysx:Note"` {
				return true
			}
		}
		return false
	}
	_, elements := interchangeElementsOf(t, interchangeDerivedSource, export.APIJSONOptions{Interchange: true})
	if hasNote(elements) {
		t.Error("the interchange form writes a sysx:Note")
	}
	for _, operator := range interchangeTyped(t, elements, "OperatorExpression") {
		if _, stated := operator["operand"]; stated {
			t.Error("an OperatorExpression of the interchange form states operand")
		}
		if _, stated := operator["argument"]; !stated {
			t.Error("an OperatorExpression of the interchange form states no argument")
		}
	}
	if _, elements := interchangeElementsOf(t, interchangeDerivedSource, export.APIJSONOptions{}); !hasNote(elements) {
		t.Error("the default form lost its sysx:Note")
	}
}
