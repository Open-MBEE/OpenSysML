package export_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

func TestAPIJSONUnnamedUsageWritesOwnedAbsences(t *testing.T) {
	path := filepath.Join("testdata", "convert", "anonymous_features.kerml")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := cachedSysMLToRDF(path, source)
	if err != nil {
		t.Fatal(err)
	}
	var unnamedID string
	for _, subject := range graph.Subjects() {
		metaclass := rdf.LocalName(graph.Type(subject))
		if metaclass != "Feature" || graph.HasProperty(subject, rdf.SysML+"declaredName") {
			continue
		}
		unnamedID, _ = rdf.SubjectID(subject)
		if unnamedID != "" {
			break
		}
	}
	if unnamedID == "" {
		t.Fatal("anonymous_features fixture has no unnamed Feature")
	}
	document, err := cachedAPIJSON(path, source, "standard", "graph")
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil || id != unnamedID {
			continue
		}
		for key, want := range map[string]string{
			"declaredName":      "null",
			"declaredShortName": "null",
			"aliasIds":          "[]",
		} {
			if got := string(element[key]); got != want {
				t.Errorf("%s = %s, want %s", key, got, want)
			}
		}
		return
	}
	t.Fatalf("API JSON has no unnamed Feature with id %q", unnamedID)
}

func TestAPIJSONKeysStayWithinMetaclassPropertySets(t *testing.T) {
	for _, path := range modelFiles(t) {
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			document, err := cachedAPIJSON(path, source, "standard", "graph")
			if err != nil {
				t.Fatal(err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatal(err)
			}
			for _, element := range elements {
				var metaclass string
				if err := json.Unmarshal(element["@type"], &metaclass); err != nil {
					t.Fatal(err)
				}
				metaclass = apiJSONTestOntologyMetaclass(metaclass)
				for key, value := range element {
					if strings.HasPrefix(key, "@") || strings.HasPrefix(key, "sysx:") {
						continue
					}
					value = bytes.TrimSpace(value)
					if !bytes.Equal(value, []byte("null")) && !bytes.Equal(value, []byte("[]")) {
						continue
					}
					if _, ok := ontology.PropertyOf(metaclass, key); !ok {
						t.Errorf("%s element has non-ontology key %q", metaclass, key)
					}
				}
			}
		})
	}
}

func TestAPIJSONPropertiesWithPositiveLowerBoundsAreNeverNullForConvertFixtures(t *testing.T) {
	for _, path := range modelFiles(t) {
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			document, err := cachedAPIJSON(path, source, "standard", "graph")
			if err != nil {
				t.Fatal(err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatal(err)
			}
			for _, element := range elements {
				var metaclass string
				if err := json.Unmarshal(element["@type"], &metaclass); err != nil {
					t.Fatal(err)
				}
				metaclass = apiJSONTestOntologyMetaclass(metaclass)
				for key, value := range element {
					if !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
						continue
					}
					property, ok := ontology.PropertyOf(metaclass, key)
					if ok && property.Lower >= 1 {
						t.Errorf("%s.%s has lower bound %d but was written as null", metaclass, key, property.Lower)
					}
				}
			}
		})
	}
}

func TestAPIJSONWriteReadWriteIsByteIdenticalForConvertFixtures(t *testing.T) {
	for _, path := range modelFiles(t) {
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := cachedSysMLToRDF(path, source)
			if err != nil {
				t.Fatal(err)
			}
			for _, form := range []struct {
				name  string
				write func(*rdf.Graph) ([]byte, error)
			}{
				{name: "standard", write: func(*rdf.Graph) ([]byte, error) {
					return cachedAPIJSON(path, source, "standard", "graph")
				}},
				{name: "compact", write: func(*rdf.Graph) ([]byte, error) {
					return cachedAPIJSON(path, source, "compact", "graph")
				}},
			} {
				t.Run(form.name, func(t *testing.T) {
					first, err := form.write(graph)
					if err != nil {
						t.Fatal(err)
					}
					reread, err := export.ReadAPIJSON(first)
					if err != nil {
						t.Fatalf("ReadAPIJSON: %v", err)
					}
					again, err := writeAPIJSONForm(reread, form.name)
					if err != nil {
						t.Fatalf("write after read: %v", err)
					}
					if !bytes.Equal(first, again) {
						t.Fatalf("API JSON changed after read/write:\n--- first ---\n%s\n--- again ---\n%s", first, again)
					}
				})
			}
		})
	}
}

func apiJSONTestOntologyMetaclass(metaclass string) string {
	switch metaclass {
	case "AssociationUsage":
		return "Association"
	case "BehaviorUsage":
		return "Behavior"
	case "BooleanUsage":
		return "BooleanExpression"
	case "ClassUsage":
		return "Class"
	case "IndividualDefinition":
		return "OccurrenceDefinition"
	case "IndividualUsage":
		return "OccurrenceUsage"
	case "InteractionUsage":
		return "Interaction"
	case "PredicateUsage":
		return "Predicate"
	case "StructureUsage":
		return "Structure"
	case "sysx:Pseudostate":
		return "StateUsage"
	default:
		return metaclass
	}
}
