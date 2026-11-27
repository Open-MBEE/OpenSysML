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
	graph, err := convert.ModelGraph(
		[]convert.Input{{Name: "garage.sysml", Data: []byte(interchangeSource)}}, convert.Options{ID: export.IDUUID})
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
