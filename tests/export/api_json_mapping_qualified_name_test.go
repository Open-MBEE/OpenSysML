package export_test

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONMappingNamesPreserveUnresolvedTargetSegments(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		targetName string
	}{
		{
			name:       "perform raw backslash n",
			source:     `package P { part p { perform 'a-b\n'; } }`,
			targetName: `a-b\n`,
		},
		{
			name:       "perform escaped apostrophe",
			source:     `package P { part p { perform 'it\'s'; } }`,
			targetName: `it\'s`,
		},
		{
			name:       "redefinition in externally typed part",
			source:     `package P { part p : External::Vehicle { attribute :>> 'x y'; } }`,
			targetName: "x y",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			graph, err := convert.SysMLToRDF(test.name+".sysml", []byte(test.source))
			if err != nil {
				t.Fatalf("SysMLToRDF: %v", err)
			}

			var subject rdf.Term
			var turtleName string
			var qualifiedNames []string
			for _, candidate := range graph.Subjects() {
				name, ok := graph.Object(candidate, rdf.SysML+"qualifiedName")
				if !ok {
					continue
				}
				qualifiedNames = append(qualifiedNames, name.Value)
				if !strings.HasSuffix(name.Value, "::"+test.targetName) {
					continue
				}
				if subject.Value != "" {
					t.Fatalf("multiple elements have qualifiedName ending in %q", test.targetName)
				}
				subject, turtleName = candidate, name.Value
			}
			if subject.Value == "" {
				t.Fatalf("no Turtle graph qualifiedName ends in %q; got %v", test.targetName, qualifiedNames)
			}
			for _, candidate := range graph.Subjects() {
				if candidate == subject {
					continue
				}
				if name, ok := graph.Lexical(candidate, rdf.SysML+"declaredName"); ok && name == test.targetName {
					t.Fatalf("target %q unexpectedly has a declared element", test.targetName)
				}
			}

			document, err := export.WriteAPIJSON(graph)
			if err != nil {
				t.Fatalf("WriteAPIJSON: %v", err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatalf("decode API JSON: %v", err)
			}
			id, ok := rdf.SubjectID(subject)
			if !ok {
				t.Fatalf("subject %s has no element ID", subject.Value)
			}
			var mappingNameFound bool
			for _, element := range elements {
				var elementID string
				if err := json.Unmarshal(element["@id"], &elementID); err != nil {
					continue
				}
				if elementID != id {
					continue
				}
				var mappingName string
				if err := json.Unmarshal(element["sysx:mappingQualifiedName"], &mappingName); err != nil {
					t.Fatalf("decode mappingQualifiedName: %v", err)
				}
				if mappingName != turtleName {
					t.Errorf("API JSON mappingQualifiedName = %q, want Turtle graph name %q", mappingName, turtleName)
				}
				mappingNameFound = true
				delete(element, "qualifiedName")
				delete(element, "sysx:mappingQualifiedName")
				delete(element, "name")
				break
			}
			if !mappingNameFound {
				t.Fatalf("API JSON has no mapping name for element %q", id)
			}
			document, err = json.Marshal(elements)
			if err != nil {
				t.Fatalf("encode API JSON without spec qualifiedName: %v", err)
			}
			imported, err := export.ReadAPIJSON(document)
			if err != nil {
				t.Fatalf("ReadAPIJSON: %v", err)
			}
			importedName, ok := imported.Object(subject, rdf.SysML+"qualifiedName")
			if !ok || importedName.Value != turtleName {
				t.Errorf("imported mapping name = %q, %t; want Turtle graph name %q", importedName.Value, ok, turtleName)
			}
		})
	}
}

func TestAPIJSONMappingQualifiedNamesAreExact(t *testing.T) {
	paths, exampleRoot := apiJSONQualifiedNamePaths(t)
	conversions := corpusConversions(t)

	fixtures, fixtureKeys := 0, 0
	examples, exampleKeys, skippedExamples := 0, 0, 0
	var countsMu sync.Mutex
	t.Cleanup(func() {
		countsMu.Lock()
		defer countsMu.Unlock()
		t.Logf("convert fixtures: %d models, %d mapping-name keys; examples: %d converting models, %d mapping-name keys, %d skipped",
			fixtures, fixtureKeys, examples, exampleKeys, skippedExamples)
	})
	for _, path := range paths {
		path := path
		isExample := strings.HasPrefix(path, exampleRoot+string(filepath.Separator))
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			t.Parallel()
			defer conversions.release(path)
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			original, err := conversions.sysMLToRDF(path, source)
			if err != nil {
				if isExample {
					countsMu.Lock()
					skippedExamples++
					countsMu.Unlock()
					t.Logf("skipping non-converting example: %v", err)
					return
				}
				t.Fatalf("convert fixture: %v", err)
			}
			document, err := conversions.apiJSON(path, source, "standard", "graph")
			if err != nil {
				t.Fatalf("WriteAPIJSON: %v", err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(document, &elements); err != nil {
				t.Fatalf("decode API JSON: %v", err)
			}
			keyed := 0
			for _, element := range elements {
				if _, ok := element["sysx:mappingQualifiedName"]; ok {
					keyed++
					delete(element, "qualifiedName")
				}
			}
			document, err = json.Marshal(elements)
			if err != nil {
				t.Fatalf("encode API JSON without spec qualifiedName: %v", err)
			}
			got, err := export.ReadAPIJSON(document)
			if err != nil {
				t.Fatalf("ReadAPIJSON: %v", err)
			}
			assertAPIJSONGraphsEqual(t, apiJSONGraphWithoutLibraryReferences(original), got)

			if isExample {
				countsMu.Lock()
				examples++
				exampleKeys += keyed
				countsMu.Unlock()
			} else {
				countsMu.Lock()
				fixtures++
				fixtureKeys += keyed
				countsMu.Unlock()
			}
		})
	}
}

func apiJSONQualifiedNamePaths(t *testing.T) ([]string, string) {
	t.Helper()
	paths := modelFiles(t)
	exampleRoot := filepath.Join("..", "..", "examples")
	if err := filepath.WalkDir(exampleRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".sysml" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths, exampleRoot
}

func apiJSONGraphWithoutLibraryReferences(graph *rdf.Graph) *rdf.Graph {
	triples := make([]rdf.Triple, 0, graph.Len())
	for _, triple := range graph.Triples() {
		if !export.LibraryReference(graph, triple.Subject) {
			triples = append(triples, triple)
		}
	}
	return rdf.NewGraphOf(triples, graph.Prefixes)
}

func assertAPIJSONGraphsEqual(t *testing.T, want, got *rdf.Graph) {
	t.Helper()
	wantTriples := make(map[rdf.Triple]bool, want.Len())
	for _, triple := range want.Triples() {
		wantTriples[apiJSONComparableTriple(triple)] = true
	}
	gotTriples := make(map[rdf.Triple]bool, got.Len())
	for _, triple := range got.Triples() {
		gotTriples[apiJSONComparableTriple(triple)] = true
	}
	var differences []string
	for triple := range wantTriples {
		if !gotTriples[triple] {
			differences = append(differences, "missing "+string(rdf.WriteTurtle(rdf.NewGraphOf([]rdf.Triple{triple}, nil))))
		}
	}
	for triple := range gotTriples {
		if !wantTriples[triple] {
			differences = append(differences, "unexpected "+string(rdf.WriteTurtle(rdf.NewGraphOf([]rdf.Triple{triple}, nil))))
		}
	}
	if len(differences) > 0 {
		sort.Strings(differences)
		t.Errorf("API-JSON round trip differs by %d triples:\n%s", len(differences), strings.Join(differences, "\n"))
	}
}

func apiJSONComparableTriple(triple rdf.Triple) rdf.Triple {
	if triple.Object.Datatype == rdf.XSD+"decimal" {
		if value, ok := new(big.Rat).SetString(triple.Object.Value); ok {
			triple.Object.Value = value.RatString()
		}
	}
	return triple
}
