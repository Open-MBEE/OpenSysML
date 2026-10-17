package export_test

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONMappingQualifiedNamesAreExact(t *testing.T) {
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

	fixtures, fixtureKeys := 0, 0
	examples, exampleKeys, skippedExamples := 0, 0, 0
	for _, path := range paths {
		path := path
		isExample := strings.HasPrefix(path, exampleRoot+string(filepath.Separator))
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			original, err := convert.SysMLToRDF(path, source)
			if err != nil {
				if isExample {
					skippedExamples++
					t.Logf("skipping non-converting example: %v", err)
					return
				}
				t.Fatalf("convert fixture: %v", err)
			}
			document, err := export.WriteAPIJSON(original)
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
			assertAPIJSONGraphsEqual(t, original, got)

			if isExample {
				examples++
				exampleKeys += keyed
			} else {
				fixtures++
				fixtureKeys += keyed
			}
		})
	}
	t.Logf("convert fixtures: %d models, %d mapping-name keys; examples: %d converting models, %d mapping-name keys, %d skipped", fixtures, fixtureKeys, examples, exampleKeys, skippedExamples)
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
