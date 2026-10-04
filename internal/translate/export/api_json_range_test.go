package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

func TestFullAPIJSONReferenceRangesAndMetaclasses(t *testing.T) {
	fixtureRoot := filepath.Join("..", "..", "..", "..", "tests", "export", "testdata", "convert")
	goldens, err := filepath.Glob(filepath.Join(fixtureRoot, "*.full.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range goldens {
		t.Run("golden/"+filepath.Base(path), func(t *testing.T) {
			document, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sourcePath := strings.TrimSuffix(path, ".full.golden.json") + ".sysml"
			_, resolver := fullAPIJSONForRangeTest(t, sourcePath)
			assertFullAPIJSONRanges(t, path, document, resolver)
		})
	}

	fixtures, err := filepath.Glob(filepath.Join(fixtureRoot, "*.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range fixtures {
		t.Run("conversion/"+filepath.Base(path), func(t *testing.T) {
			document, resolver := fullAPIJSONForRangeTest(t, path)
			assertFullAPIJSONRanges(t, path, document, resolver)
		})
	}
}

func fullAPIJSONForRangeTest(t *testing.T, path string) ([]byte, *resolve.Resolver) {
	t.Helper()
	input, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file := source.New(path, input)
	parse := parser.New(file)
	root := parse.ParseFile()
	if len(parse.Diagnostics) != 0 {
		t.Fatalf("%s: parse diagnostics: %v", path, parse.Diagnostics)
	}
	graph, resolver, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatalf("%s: RDF conversion: %v", path, err)
	}
	if len(encoders) == 0 {
		t.Fatalf("%s: no model encoder", path)
	}
	subjects := buildSemanticSubjects(graph, resolver, encoders)
	evaluator := metamodel.New(resolver, encoders[0].ids.model, metamodel.Options{
		Structure: newGraphStructure(graph, subjects),
	})
	document, _, err := writeFullAPIJSON(graph, evaluator, subjects)
	if err != nil {
		t.Fatalf("%s: full API JSON: %v", path, err)
	}
	return document, resolver
}

func assertFullAPIJSONRanges(t *testing.T, path string, document []byte, resolver *resolve.Resolver) {
	t.Helper()
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("%s: decode full API JSON: %v", path, err)
	}
	classes := make(map[string]string, len(elements))
	var violations []string
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil || id == "" {
			violations = append(violations, "subject has no valid @id")
			continue
		}
		rawType, typed := element["@type"]
		if !typed {
			continue
		}
		if _, hasHandle := element["isLibraryElement"]; !hasHandle {
			violations = append(violations, fmt.Sprintf("%s is SysML-typed but has no semantic handle", id))
		}
		var metaclass string
		if err := json.Unmarshal(rawType, &metaclass); err != nil || metaclass == "" {
			violations = append(violations, fmt.Sprintf("%s has an invalid @type", id))
			continue
		}
		if !knownSysMLMetaclass(metaclass) {
			violations = append(violations, fmt.Sprintf("%s has unknown SysML metaclass %s", id, metaclass))
			continue
		}
		classes[id] = metaclass
	}
	for id, metaclass := range libraryMetaclasses(resolver) {
		if _, exists := classes[id]; !exists {
			classes[id] = metaclass
		}
	}

	for _, element := range elements {
		var id, metaclass string
		_ = json.Unmarshal(element["@id"], &id)
		_ = json.Unmarshal(element["@type"], &metaclass)
		if metaclass == "" {
			continue
		}
		for property, raw := range element {
			if strings.HasPrefix(property, "@") {
				continue
			}
			definition, ok := ontology.PropertyOf(metaclass, property)
			if !ok || definition.Kind != ontology.ObjectProperty {
				continue
			}
			expected := strings.TrimPrefix(definition.Range, rdf.SysML)
			if expected == definition.Range {
				continue
			}
			for _, target := range fullAPIJSONReferences(raw) {
				actual, ok := classes[target]
				if !ok {
					violations = append(violations, fmt.Sprintf("%s[%s] references %s without a typed subject", id, property, target))
					continue
				}
				if !ontology.IsAncestorOrSelf(actual, expected) {
					violations = append(violations, fmt.Sprintf("%s[%s] references %s:%s outside range %s", id, property, actual, target, expected))
				}
			}
		}
	}
	if len(violations) != 0 {
		t.Errorf("%s: %d reference-range or metaclass violations:\n%s", path, len(violations), strings.Join(violations, "\n"))
	}
}

func libraryMetaclasses(resolver *resolve.Resolver) map[string]string {
	classes := make(map[string]string)
	if resolver == nil || resolver.Index() == nil {
		return classes
	}
	for _, element := range identity.LibraryCatalog(resolver.Index()).Elements() {
		if element.ID != "" {
			if metaclass := declaredMetaclass(element.Symbol.Decl); metaclass != "" {
				classes[element.ID] = metaclass
			}
		}
		if element.OwningMembershipID != "" {
			classes[element.OwningMembershipID] = "OwningMembership"
		}
	}
	return classes
}

func knownSysMLMetaclass(name string) bool {
	for _, class := range ontology.Classes() {
		if class.Name == name {
			return true
		}
	}
	return false
}

func fullAPIJSONReferences(raw json.RawMessage) []string {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	var references []string
	var visit func(any)
	visit = func(current any) {
		switch current := current.(type) {
		case []any:
			for _, item := range current {
				visit(item)
			}
		case map[string]any:
			if id, ok := current["@id"].(string); ok {
				references = append(references, id)
				return
			}
			for _, item := range current {
				visit(item)
			}
		}
	}
	visit(value)
	return references
}
