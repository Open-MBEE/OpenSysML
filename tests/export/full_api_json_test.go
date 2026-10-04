package export_test

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

var updateFullAPIJSON = flag.Bool("update-full-api-json", false, "rewrite the full API JSON goldens")
var updateFullImportParity = flag.Bool("update-full-import-parity", false, "rewrite full API JSON import parity outcomes")
var updateAnnexAFullOutcomes = flag.Bool("update-annex-a-outcomes", false, "rewrite Annex A full API JSON outcome counts")

func TestFullAPIJSONGoldens(t *testing.T) {
	for _, name := range []string{"names", "enums", "nested", "parts"} {
		t.Run(name, func(t *testing.T) {
			sourcePath := filepath.Join("testdata", "convert", name+".sysml")
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			full, err := convert.ConvertWith(sourcePath, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
				APIJSON: export.APIJSONFull,
			})
			if err != nil {
				t.Fatalf("full API JSON: %v", err)
			}
			goldenPath := filepath.Join("testdata", "convert", name+".full.golden.json")
			if *updateFullAPIJSON {
				if err := os.WriteFile(goldenPath, full, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("%v (run with -update-full-api-json to create it)", err)
			}
			if !bytes.Equal(full, want) {
				t.Errorf("%s differs\n--- want ---\n%s\n--- got ---\n%s", goldenPath, want, full)
			}
		})
	}
}

func TestFullAndCompactAPIJSONImportParityForConvertFixtures(t *testing.T) {
	actual := make(map[string]string)
	for _, path := range modelFiles(t) {
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			actual[name] = fullImportParityVerdict(path, t)
		})
	}
	if *updateFullImportParity {
		var contents strings.Builder
		contents.WriteString("# Full-form API JSON import uses the element-form path shared with toolkit full JSON; movements must be adjudicated.\n")
		for _, path := range modelFiles(t) {
			name, _ := fixtureName(path)
			fmt.Fprintf(&contents, "%s\t%s\n", name, actual[name])
		}
		if err := os.WriteFile(filepath.Join("testdata", "full-api-json-import-parity.txt"), []byte(contents.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := readFullImportParity(t)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range modelFiles(t) {
		name, _ := fixtureName(path)
		got, ok := actual[name]
		if !ok {
			t.Errorf("%s: no parity verdict was recorded", name)
			continue
		}
		if expected, ok := want[name]; !ok {
			t.Errorf("%s: missing from full API JSON parity ratchet", name)
		} else if expected != got {
			t.Errorf("%s: full API JSON parity moved: want %q, got %q", name, expected, got)
		}
	}
	for name := range want {
		if _, ok := actual[name]; !ok {
			t.Errorf("%s: stale entry in full API JSON parity ratchet", name)
		}
	}
}

func fullImportParityVerdict(path string, t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		return parityError("source", err)
	}
	compact, err := convert.Convert(path, source, convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		return parityError("compact export", err)
	}
	full, err := convert.ConvertWith(path, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		return parityError("full export", err)
	}
	compactGraph, err := export.ReadAPIJSON(compact)
	if err != nil {
		return parityError("compact read", err)
	}
	fullGraph, err := export.ReadAPIJSON(full)
	if err != nil {
		return parityError("full read", err)
	}
	compactNotation, err := export.ToSysML(compactGraph)
	if err != nil {
		return parityError("compact import", err)
	}
	fullNotation, err := export.ToSysML(fullGraph)
	if err != nil {
		return parityError("full import", err)
	}
	if !bytes.Equal(compactNotation, fullNotation) {
		t.Logf("imported notation differs (%d compact bytes, %d full bytes)", len(compactNotation), len(fullNotation))
	}
	compactTurtle, err := convert.Convert(path, compactNotation, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		return parityError("compact re-export", err)
	}
	fullTurtle, err := convert.Convert(path, fullNotation, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		return parityError("full re-export", err)
	}
	compactRDF, err := rdf.ParseTurtle(compactTurtle)
	if err != nil {
		return parityError("compact Turtle parse", err)
	}
	fullRDF, err := rdf.ParseTurtle(fullTurtle)
	if err != nil {
		return parityError("full Turtle parse", err)
	}
	// Full-form import resolves library references through the stub's library qualified name, so re-exported notation may spell a reference canonically.
	compactTriples, fullTriples := tripleSet(withoutNotationFidelity(compactRDF)), tripleSet(withoutNotationFidelity(fullRDF))
	if sameTriples(compactTriples, fullTriples) {
		return "pass"
	}
	differingPredicates := make(map[string]bool)
	for _, triple := range diffTriples(compactTriples, fullTriples) {
		differingPredicates[rdf.LocalName(triple.Predicate.Value)] = true
	}
	for _, triple := range diffTriples(fullTriples, compactTriples) {
		differingPredicates[rdf.LocalName(triple.Predicate.Value)] = true
	}
	predicates := make([]string, 0, len(differingPredicates))
	for predicate := range differingPredicates {
		predicates = append(predicates, predicate)
	}
	sort.Strings(predicates)
	return "structural " + strings.Join(predicates, ",")
}

func parityError(stage string, err error) string {
	var unsupported *export.UnsupportedError
	if errors.As(err, &unsupported) {
		return fmt.Sprintf("error %s %T %s", stage, unsupported, stableParityWhat(unsupported.What))
	}
	return fmt.Sprintf("error %s %T %s", stage, err, stableParityWhat(err.Error()))
}

func stableParityWhat(what string) string {
	for _, prefix := range []string{"http://", "https://", "urn:"} {
		for {
			match := strings.Index(what, prefix)
			if match < 0 {
				break
			}
			start := match
			if start > 0 && what[start-1] == '<' {
				start--
			}
			end := match
			for end < len(what) && !strings.ContainsRune(" \t\r\n,;)]}", rune(what[end])) {
				end++
			}
			what = what[:start] + "<IRI>" + what[end:]
		}
	}
	fields := strings.Fields(what)
	for i, field := range fields {
		name := strings.Trim(field, "<>[](){}.,;:")
		if strings.Contains(name, "::") || strings.HasPrefix(name, "@") {
			fields[i] = "<name>"
			continue
		}
		if strings.HasPrefix(field, "line") || strings.HasPrefix(field, "column") {
			fields[i] = "<position>"
			if i+1 < len(fields) {
				fields[i+1] = ""
			}
		}
	}
	what = strings.Join(strings.Fields(strings.Join(fields, " ")), " ")
	if len(what) > 100 {
		what = what[:100]
	}
	return what
}

func readFullImportParity(t *testing.T) (map[string]string, error) {
	t.Helper()
	path := filepath.Join("testdata", "full-api-json-import-parity.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	verdicts := make(map[string]string)
	for lineNumber, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
			return nil, fmt.Errorf("%s:%d: invalid parity outcome", path, lineNumber+1)
		}
		if _, exists := verdicts[fields[0]]; exists {
			return nil, fmt.Errorf("%s:%d: duplicate fixture %q", path, lineNumber+1, fields[0])
		}
		verdicts[fields[0]] = fields[1]
	}
	return verdicts, nil
}

func TestFullAPIJSONOwnedProjectionsPreserveRelationshipOrder(t *testing.T) {
	projections := make(map[string]bool)
	for _, property := range ontology.Properties() {
		if strings.HasPrefix(property.Name, "owned") &&
			propertyIsOwnedRelationshipProjection(property, make(map[string]bool)) {
			projections[property.DefiningClass+"::"+property.Name] = true
		}
	}
	if len(projections) == 0 {
		t.Fatal("ontology has no ownedRelationship filter projections")
	}
	for _, path := range modelFiles(t) {
		fixture, _ := fixtureName(path)
		t.Run(fixture, func(t *testing.T) {
			elements := fullAPIJSONElements(t, fixture)
			for _, element := range elements {
				metaclass, _ := element["@type"].(string)
				ownedIDs, ok := jsonReferenceIDs(element["ownedRelationship"])
				if !ok {
					t.Errorf("%s.%s has malformed ownedRelationship", fixture, element["@id"])
					continue
				}
				for name, raw := range element {
					property, ok := ontology.PropertyOf(metaclass, name)
					if !ok || !projections[property.DefiningClass+"::"+property.Name] {
						continue
					}
					projectedIDs, ok := jsonReferenceIDs(raw)
					if !ok {
						t.Errorf("%s.%s.%s has malformed reference projection", fixture, element["@id"], name)
						continue
					}
					position := 0
					for _, projectedID := range projectedIDs {
						for position < len(ownedIDs) && ownedIDs[position] != projectedID {
							position++
						}
						if position == len(ownedIDs) {
							t.Errorf("%s.%s.%s is not an order-preserving subsequence of ownedRelationship: %v not in %v",
								fixture, element["@id"], name, projectedIDs, ownedIDs)
							break
						}
						position++
					}
				}
			}
		})
	}
}

func TestFullAPIJSONUniqueListsHaveNoDuplicateMembers(t *testing.T) {
	settings := umlCollectionSettings(t)
	chainingFeature, ok := settings["Feature::chainingFeature"]
	if !ok || !chainingFeature.ordered || chainingFeature.unique {
		t.Fatal("SysML.uml does not declare Feature::chainingFeature as ordered and nonunique")
	}
	for _, path := range modelFiles(t) {
		fixture, _ := fixtureName(path)
		t.Run(fixture, func(t *testing.T) {
			for _, element := range fullAPIJSONElements(t, fixture) {
				metaclass, _ := element["@type"].(string)
				for name, raw := range element {
					property, ok := ontology.PropertyOf(metaclass, name)
					if !ok || !property.Many || propertyIsNonUnique(property, settings, make(map[string]bool)) {
						continue
					}
					items, isArray := raw.([]any)
					if !isArray {
						continue
					}
					seen := make(map[string]bool, len(items))
					for _, item := range items {
						key := ""
						if id, ok := jsonReferenceID(item); ok {
							key = "ref:" + id
						} else {
							encoded, err := json.Marshal(item)
							if err != nil {
								t.Fatal(err)
							}
							key = string(encoded)
						}
						if seen[key] {
							t.Errorf("%s.%s.%s contains duplicate member %s", fixture, element["@id"], name, key)
							break
						}
						seen[key] = true
					}
				}
			}
		})
	}
	elements := fullAPIJSONElements(t, "feature_chains_repeated")
	chain := fullJSONElementByID(t, elements, "P__c_pend0_pchain")
	chainIDs, ok := jsonReferenceIDs(chain["chainingFeature"])
	if !ok || len(chainIDs) != 3 || chainIDs[1] != "P__Node__next" || chainIDs[2] != "P__Node__next" {
		t.Fatalf("nonunique Feature::chainingFeature = %v, want the repeated next feature preserved", chain["chainingFeature"])
	}
}

type umlMultiplicitySettings struct {
	ordered bool
	unique  bool
}

func propertyIsOwnedRelationshipProjection(property ontology.Property, seen map[string]bool) bool {
	name := property.DefiningClass + "::" + property.Name
	if name == "Element::ownedRelationship" {
		return true
	}
	if seen[name] {
		return false
	}
	seen[name] = true
	for _, parent := range append(append([]string(nil), property.Subsets...), property.Redefines...) {
		parts := strings.SplitN(parent, "::", 2)
		if len(parts) != 2 {
			continue
		}
		for _, candidate := range ontology.LookupProperty(parts[1]) {
			if candidate.DefiningClass == parts[0] && propertyIsOwnedRelationshipProjection(candidate, seen) {
				return true
			}
		}
	}
	return false
}

func propertyIsNonUnique(property ontology.Property, settings map[string]umlMultiplicitySettings, seen map[string]bool) bool {
	name := property.QualifiedName()
	if value, ok := settings[name]; ok && !value.unique {
		return true
	}
	if seen[name] {
		return false
	}
	seen[name] = true
	for _, parent := range append(append([]string(nil), property.Subsets...), property.Redefines...) {
		parts := strings.SplitN(parent, "::", 2)
		if len(parts) != 2 {
			continue
		}
		for _, candidate := range ontology.LookupProperty(parts[1]) {
			if candidate.DefiningClass == parts[0] && propertyIsNonUnique(candidate, settings, seen) {
				return true
			}
		}
	}
	return false
}

func fullAPIJSONElements(t *testing.T, fixture string) []map[string]any {
	t.Helper()
	path := convertFixturePath(t, fixture)
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	full, err := convert.ConvertWith(path, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	return elements
}

func convertFixturePath(t *testing.T, fixture string) string {
	t.Helper()
	for _, candidate := range modelFiles(t) {
		name, _ := fixtureName(candidate)
		if name == fixture {
			return candidate
		}
	}
	t.Fatalf("no convert fixture named %q", fixture)
	return ""
}

func fullJSONElementByID(t *testing.T, elements []map[string]any, id string) map[string]any {
	t.Helper()
	for _, element := range elements {
		if element["@id"] == id {
			return element
		}
	}
	t.Fatalf("full API JSON has no element %q", id)
	return nil
}

func jsonReferenceIDs(value any) ([]string, bool) {
	switch value := value.(type) {
	case nil:
		return nil, true
	case []any:
		ids := make([]string, 0, len(value))
		for _, item := range value {
			id, ok := jsonReferenceID(item)
			if !ok {
				return nil, false
			}
			ids = append(ids, id)
		}
		return ids, true
	case map[string]any:
		id, ok := jsonReferenceID(value)
		if !ok {
			return nil, false
		}
		return []string{id}, true
	default:
		return nil, false
	}
}

func jsonReferenceID(value any) (string, bool) {
	reference, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	id, ok := reference["@id"].(string)
	return id, ok
}

func umlCollectionSettings(t *testing.T) map[string]umlMultiplicitySettings {
	t.Helper()
	settings := make(map[string]umlMultiplicitySettings)
	for _, path := range []string{
		filepath.Join("..", "..", "build", "pilot-metamodel", "KerML_only.uml"),
		filepath.Join("..", "..", "build", "pilot-metamodel", "SysML.uml"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		type frame struct{ class string }
		var stack []frame
		decoder := xml.NewDecoder(bytes.NewReader(data))
		for {
			token, err := decoder.Token()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				t.Fatalf("parse %s: %v", path, err)
			}
			switch token := token.(type) {
			case xml.StartElement:
				parent := ""
				if len(stack) > 0 {
					parent = stack[len(stack)-1].class
				}
				class := parent
				name, elementType, ordered, unique := "", "", "", ""
				for _, attribute := range token.Attr {
					switch attribute.Name.Local {
					case "name":
						name = attribute.Value
					case "type":
						elementType = attribute.Value
					case "isOrdered":
						ordered = attribute.Value
					case "isUnique":
						unique = attribute.Value
					}
				}
				if token.Name.Local == "packagedElement" && strings.HasSuffix(elementType, ":Class") {
					class = name
				}
				if (token.Name.Local == "ownedAttribute" || token.Name.Local == "ownedEnd") &&
					class != "" && name != "" {
					settings[class+"::"+name] = umlMultiplicitySettings{
						ordered: ordered == "true",
						unique:  unique != "false",
					}
				}
				stack = append(stack, frame{class: class})
			case xml.EndElement:
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	return settings
}

func withoutNotationFidelity(graph *rdf.Graph) *rdf.Graph {
	out := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if export.IsSourceRangeProperty(triple.Predicate.Value) {
			continue
		}
		switch triple.Predicate.Value {
		case rdf.OpenSysML + "sourceText",
			rdf.OpenSysML + "sourceTail",
			rdf.OpenSysML + "sourceLanguage",
			rdf.OpenSysML + "sourceDocument",
			rdf.OpenSysML + "sourceMultiplicityBeforeThen":
			continue
		}
		out.AddTriple(triple)
	}
	return out
}

func TestFullAPIJSONHasLibraryStatusForEverySubject(t *testing.T) {
	inputs := make([]string, 0, 5)
	for _, name := range []string{"names", "enums", "nested", "parts"} {
		inputs = append(inputs, filepath.Join("testdata", "convert", name+".sysml"))
	}
	if annex := annexAModelPath(t); annex != "" {
		inputs = append(inputs, annex)
	}
	for _, path := range inputs {
		t.Run(filepath.Base(path), func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			full, err := convert.ConvertWith(path, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
				APIJSON: export.APIJSONFull,
			})
			if err != nil {
				t.Fatalf("full API JSON: %v", err)
			}
			var elements []map[string]json.RawMessage
			if err := json.Unmarshal(full, &elements); err != nil {
				t.Fatal(err)
			}
			var missing []string
			for _, element := range elements {
				if _, typed := element["@type"]; !typed {
					continue
				}
				if _, present := element["isLibraryElement"]; present {
					continue
				}
				var id string
				_ = json.Unmarshal(element["@id"], &id)
				var metaclass string
				_ = json.Unmarshal(element["@type"], &metaclass)
				missing = append(missing, metaclass+":"+id)
			}
			if len(missing) > 0 {
				t.Errorf("SysML subjects without semantic handles: %v", missing)
			}
		})
	}
}

func TestFullAPIJSONAnnexAFeatureTargetsAreNotNull(t *testing.T) {
	path := annexAModelPath(t)
	if path == "" {
		t.Skip("Annex A corpus is not available")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	full, err := convert.ConvertWith(path, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatalf("full API JSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	features := 0
	for _, element := range elements {
		value, ok := element["featureTarget"]
		if !ok {
			continue
		}
		features++
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			t.Errorf("%s has a null featureTarget", element["@id"])
		}
	}
	if features == 0 {
		t.Fatal("Annex A full API JSON has no featureTarget values")
	}
}

func TestFullAPIJSONLibraryStubUsesCatalogNameAndOmitsContainment(t *testing.T) {
	full, err := convert.ConvertWith("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatalf("full API JSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		var qualifiedName string
		if err := json.Unmarshal(element["qualifiedName"], &qualifiedName); err != nil {
			t.Fatal(err)
		}
		if qualifiedName != "ISQBase::MassValue" {
			continue
		}
		var name string
		if err := json.Unmarshal(element["name"], &name); err != nil {
			t.Fatal(err)
		}
		if name != "MassValue" {
			t.Errorf("library stub name = %q, want MassValue", name)
		}
		if shortName, ok := element["shortName"]; !ok || string(shortName) != "null" {
			t.Errorf("library stub shortName = %s, want null from its absent declared short name", shortName)
		}
		var library bool
		if err := json.Unmarshal(element["isLibraryElement"], &library); err != nil {
			t.Fatal(err)
		}
		if !library {
			t.Error("library stub lost isLibraryElement")
		}
		for key := range element {
			if strings.HasPrefix(key, "owned") || strings.Contains(key, "owning") || key == "owner" {
				t.Errorf("library stub states containment property %q: %s", key, element[key])
			}
		}
		if _, ok := element["elementId"]; !ok {
			t.Error("library stub lost elementId")
		}
		return
	}
	t.Fatal("full API JSON omitted the ISQBase::MassValue library stub")
}

func annexAModelPath(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("OPENSYSML_ANNEX_A"); path != "" {
		return path
	}
	path := filepath.Join("..", "..", "examples", "pilot-corpora", "sysml-examples",
		"Vehicle Example", "SysML v2 Spec Annex A SimpleVehicleModel.sysml")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func TestFullAPIJSONAnnexAOutcomes(t *testing.T) {
	path := annexAModelPath(t)
	if path == "" {
		if os.Getenv("OPENSYSML_REQUIRE_PILOT_CORPORA") == "1" {
			t.Fatal("Annex A corpus is required but not available")
		}
		t.Skip("Annex A corpus is not available")
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	full, err := convert.ConvertWith(path, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatalf("full API JSON: %v", err)
	}
	outPath := filepath.Join(t.TempDir(), "annex-a-full-api.json")
	if configured := os.Getenv("OPENSYSML_ANNEX_A_JSON_OUT"); configured != "" {
		outPath = configured
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(outPath, full, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("full API JSON written to %s", outPath)
	counts, err := classifyAnnexAOutcomes(full)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Annex A full API JSON outcomes: elements=%d reads=%d Value=%d Empty=%d NotSupplied=%d UnresolvedReference=%d MalformedValue=%d",
		counts.elements, counts.reads, counts.values, counts.empty, counts.notSupplied, counts.unresolved, counts.malformed)
	if counts.malformed != 0 {
		t.Fatalf("Annex A full API JSON has %d MalformedValue outcomes; this assertion cannot be updated", counts.malformed)
	}
	pinPath := filepath.Join("testdata", "annex-a-full-outcomes.txt")
	actual := []byte(strings.Join(counts.lines(), "\n") + "\n")
	if *updateAnnexAFullOutcomes {
		if err := os.WriteFile(pinPath, actual, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(pinPath)
	if err != nil {
		t.Fatalf("%v (run with -update-annex-a-outcomes to create it)", err)
	}
	if !bytes.Equal(actual, want) {
		t.Errorf("Annex A full API JSON outcomes moved:\n--- want ---\n%s--- got ---\n%s", want, actual)
	}
}

type annexAOutcomeCounts struct {
	elements              int
	reads                 int
	values                int
	empty                 int
	notSupplied           int
	unresolved            int
	malformed             int
	notSuppliedByProperty map[string]int
}

func classifyAnnexAOutcomes(data []byte) (annexAOutcomeCounts, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var elements []map[string]any
	if err := decoder.Decode(&elements); err != nil {
		return annexAOutcomeCounts{}, err
	}
	byID := make(map[string]map[string]any, len(elements))
	modelElements := make([]map[string]any, 0, len(elements))
	hasFalseLibraryFlag := false
	for _, element := range elements {
		id, ok := element["@id"].(string)
		if !ok || id == "" {
			return annexAOutcomeCounts{}, fmt.Errorf("element lacks a string @id")
		}
		if _, exists := byID[id]; exists {
			return annexAOutcomeCounts{}, fmt.Errorf("duplicate element @id %q", id)
		}
		byID[id] = element
		if element["isLibraryElement"] == false {
			hasFalseLibraryFlag = true
		}
	}
	for _, element := range elements {
		if hasFalseLibraryFlag && element["isLibraryElement"] == true {
			continue
		}
		modelElements = append(modelElements, element)
	}
	counts := annexAOutcomeCounts{elements: len(modelElements), notSuppliedByProperty: make(map[string]int)}
	for _, element := range modelElements {
		metaclass, ok := element["@type"].(string)
		if !ok {
			return annexAOutcomeCounts{}, fmt.Errorf("element %v lacks a string @type", element["@id"])
		}
		names := make(map[string]bool)
		for _, property := range ontology.Properties() {
			if ontology.IsAncestorOrSelf(metaclass, property.DefiningClass) {
				names[property.Name] = true
			}
		}
		for name := range names {
			property, ok := ontology.PropertyOf(metaclass, name)
			if !ok {
				continue
			}
			outcome := classifyAnnexProperty(property, element[name], hasMapKey(element, name), byID)
			for range annexAPropertyReadMultiplicity(property.Name) {
				counts.reads++
				switch outcome {
				case "Value":
					counts.values++
				case "Empty":
					counts.empty++
				case "NotSupplied":
					counts.notSupplied++
					counts.notSuppliedByProperty[property.QualifiedName()]++
				case "UnresolvedReference":
					counts.unresolved++
				case "MalformedValue":
					counts.malformed++
				}
			}
		}
	}
	return counts, nil
}

func annexAPropertyReadMultiplicity(name string) int {
	if name != annexAPropertySnakeName(name) && !annexAPythonKeyword(name) {
		return 2
	}
	return 1
}

func annexAPropertySnakeName(name string) string {
	runes := []rune(name)
	var result strings.Builder
	for i, current := range runes {
		if i > 0 && unicode.IsUpper(current) {
			previous := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) ||
				(unicode.IsUpper(previous) && nextIsLower) {
				result.WriteByte('_')
			}
		}
		result.WriteRune(unicode.ToLower(current))
	}
	snake := result.String()
	if annexAPythonKeyword(snake) {
		return snake + "_"
	}
	return snake
}

func annexAPythonKeyword(name string) bool {
	switch name {
	case "False", "None", "True", "and", "as", "assert", "async", "await", "break",
		"class", "continue", "def", "del", "elif", "else", "except", "finally",
		"for", "from", "global", "if", "import", "in", "is", "lambda", "nonlocal",
		"not", "or", "pass", "raise", "return", "try", "while", "with", "yield":
		return true
	default:
		return false
	}
}

func hasMapKey(element map[string]any, key string) bool {
	_, ok := element[key]
	return ok
}

func classifyAnnexProperty(property ontology.Property, raw any, present bool, elements map[string]map[string]any) string {
	if !present {
		return "NotSupplied"
	}
	if property.Many {
		if raw == nil {
			return "Empty"
		}
		items, ok := raw.([]any)
		if !ok {
			return "MalformedValue"
		}
		if len(items) == 0 {
			return "Empty"
		}
		for _, item := range items {
			outcome := classifyAnnexValue(property, item, elements)
			if outcome != "Value" {
				return outcome
			}
		}
		return "Value"
	}
	if raw == nil {
		return "Empty"
	}
	if raw == "" && property.Range == rdf.XSD+"string" {
		return "Empty"
	}
	return classifyAnnexValue(property, raw, elements)
}

func classifyAnnexValue(property ontology.Property, raw any, elements map[string]map[string]any) string {
	if reference, ok := raw.(map[string]any); ok {
		if ref, exists := reference["@ref"]; exists {
			if _, ok := ref.(string); !ok {
				return "MalformedValue"
			}
			return "UnresolvedReference"
		}
		id, exists := reference["@id"]
		if !exists {
			return "MalformedValue"
		}
		elementID, ok := id.(string)
		if !ok {
			return "MalformedValue"
		}
		target, exists := elements[elementID]
		if !exists {
			return "UnresolvedReference"
		}
		expected := strings.TrimPrefix(property.Range, rdf.SysML)
		if _, known := ontology.LookupClass(expected); known {
			targetClass, _ := target["@type"].(string)
			if !ontology.IsAncestorOrSelf(targetClass, expected) {
				return "MalformedValue"
			}
		}
		return "Value"
	}
	if property.Kind != ontology.DatatypeProperty {
		return "MalformedValue"
	}
	switch property.Range {
	case rdf.XSD + "boolean":
		if _, ok := raw.(bool); !ok {
			return "MalformedValue"
		}
		return "Value"
	case rdf.XSD + "string":
		if _, ok := raw.(string); !ok {
			return "MalformedValue"
		}
		return "Value"
	case rdf.XSD + "integer", rdf.XSD + "int", rdf.XSD + "long", rdf.XSD + "short", rdf.XSD + "byte":
		number, ok := raw.(json.Number)
		if !ok {
			return "MalformedValue"
		}
		if _, ok := new(big.Int).SetString(string(number), 10); !ok {
			return "MalformedValue"
		}
		return "Value"
	case rdf.XSD + "double", rdf.XSD + "decimal", rdf.XSD + "float", rdf.OWL + "real":
		number, ok := raw.(json.Number)
		if !ok {
			return "MalformedValue"
		}
		if _, err := strconv.ParseFloat(string(number), 64); err != nil {
			return "MalformedValue"
		}
		return "Value"
	default:
		name := strings.TrimPrefix(property.Range, rdf.SysML)
		enumeration, ok := ontology.LookupEnumeration(name)
		value, stringOK := raw.(string)
		if !ok || !stringOK {
			return "MalformedValue"
		}
		for _, literal := range enumeration.Literals {
			if literal == value {
				return "Value"
			}
		}
		return "MalformedValue"
	}
}

func (counts annexAOutcomeCounts) lines() []string {
	lines := []string{
		fmt.Sprintf("elements\t%d", counts.elements),
		fmt.Sprintf("reads\t%d", counts.reads),
		fmt.Sprintf("Value\t%d", counts.values),
		fmt.Sprintf("Empty\t%d", counts.empty),
		fmt.Sprintf("NotSupplied\t%d", counts.notSupplied),
		fmt.Sprintf("UnresolvedReference\t%d", counts.unresolved),
		fmt.Sprintf("MalformedValue\t%d", counts.malformed),
	}
	properties := make([]string, 0, len(counts.notSuppliedByProperty))
	for property := range counts.notSuppliedByProperty {
		properties = append(properties, property)
	}
	sort.Strings(properties)
	for _, property := range properties {
		lines = append(lines, fmt.Sprintf("NotSupplied:%s\t%d", property, counts.notSuppliedByProperty[property]))
	}
	return lines
}

func TestFullAPIJSONKeepsCompactGraphValues(t *testing.T) {
	const source = `package P {
		part def Base { attribute size : Integer; }
		part def Derived specializes Base { attribute mass : Integer; }
		part vehicle : Derived;
	}`
	compact, err := convert.Convert("m.sysml", []byte(source), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	full, err := convert.ConvertWith("m.sysml", []byte(source), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	var compactElements, fullElements []map[string]json.RawMessage
	if err := json.Unmarshal(compact, &compactElements); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(full, &fullElements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]json.RawMessage, len(fullElements))
	for _, element := range fullElements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		byID[id] = element
	}
	for _, element := range compactElements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		fullElement := byID[id]
		for key, value := range element {
			var metaclass string
			if err := json.Unmarshal(element["@type"], &metaclass); err != nil {
				t.Fatalf("%s has invalid @type: %v", id, err)
			}
			if property, ok := ontology.PropertyOf(metaclass, key); ok && property.Derived {
				continue
			}
			if !bytes.Equal(value, fullElement[key]) {
				t.Errorf("%s (%s)[%q] changed: compact %s, full %s", id, metaclass, key, value, fullElement[key])
			}
		}
	}
}

func TestFullAPIJSONFeatureTypeIncludesASTOwnedTyping(t *testing.T) {
	assertFullFeatureTyping(t, "connector_ends", "ConnectorEnds__Vehicle__eng", "ConnectorEnds__Engine")
}

func TestFullAPIJSONFeatureTypeIncludesASTOnlyEndTyping(t *testing.T) {
	assertFullFeatureTyping(t, "end_prefix_metadata", "EndPrefixMetadata__Derivation__effect", "EndPrefixMetadata__Req1_5f1")
}

func TestFullAPIJSONFeatureTypeIncludesAnonymousFeatureTyping(t *testing.T) {
	assertFullFeatureTyping(t, "irregular_layout", "IrregularLayout__Speed___401", "14c0aa22-5489-59b5-b438-ded26e83ba31")
}

func TestFullAPIJSONFeatureTypeIncludesAnonymousElementTyping(t *testing.T) {
	assertFullFeatureTyping(t, "anonymous_features", "AnonymousFeatures__v", "AnonymousFeatures__Part")
}

func TestFullAPIJSONFeatureTypeIncludesFeaturePrefixTyping(t *testing.T) {
	assertFullFeatureTyping(t, "feature_prefixes", "FeaturePrefixes__B__k", "FeaturePrefixes__A")
}

func TestFullAPIJSONFeatureTypeIncludesLibraryTyping(t *testing.T) {
	assertFullFeatureTyping(t, "irregular_layout", "IrregularLayout__Speed__m", "14c0aa22-5489-59b5-b438-ded26e83ba31")
}

func TestFullAPIJSONFeatureTypeIsNotEmptyWhenDerivationInputsExist(t *testing.T) {
	for _, path := range modelFiles(t) {
		fixture, _ := fixtureName(path)
		t.Run(fixture, func(t *testing.T) {
			for _, element := range fullAPIJSONElements(t, fixture) {
				metaclass, ok := element["@type"].(string)
				if !ok || !ontology.IsAncestorOrSelf(metaclass, "Feature") {
					continue
				}
				hasInputs := false
				for _, property := range []string{"ownedTyping", "ownedSubsetting", "chainingFeature"} {
					ids, ok := jsonReferenceIDs(element[property])
					if ok && len(ids) > 0 {
						hasInputs = true
						break
					}
				}
				if !hasInputs {
					continue
				}
				if raw, exists := element["type"]; exists {
					if raw == nil {
						t.Errorf("%s.%s has derivation inputs but Feature::type is null", fixture, element["@id"])
					} else if types, ok := raw.([]any); ok && len(types) == 0 {
						t.Errorf("%s.%s has derivation inputs but Feature::type is empty", fixture, element["@id"])
					}
				}
			}
		})
	}
}

func TestFullAPIJSONFeatureTypeIncludesEveryOwnedTypingTarget(t *testing.T) {
	for _, path := range modelFiles(t) {
		fixture, _ := fixtureName(path)
		t.Run(fixture, func(t *testing.T) {
			elements := fullAPIJSONElements(t, fixture)
			byID := make(map[string]map[string]any, len(elements))
			for _, element := range elements {
				if id, ok := element["@id"].(string); ok {
					byID[id] = element
				}
			}
			for _, element := range elements {
				metaclass, ok := element["@type"].(string)
				if !ok || !ontology.IsAncestorOrSelf(metaclass, "Feature") {
					continue
				}
				typingIDs, ok := jsonReferenceIDs(element["ownedTyping"])
				if !ok || len(typingIDs) == 0 {
					continue
				}
				expected := make(map[string]bool, len(typingIDs))
				allKnown := true
				for _, typingID := range typingIDs {
					typing := byID[typingID]
					if typing == nil {
						allKnown = false
						continue
					}
					typeIDs, ok := jsonReferenceIDs(typing["type"])
					if !ok || len(typeIDs) != 1 {
						allKnown = false
						continue
					}
					expected[typeIDs[0]] = true
				}
				raw, present := element["type"]
				if !present {
					continue
				}
				typeIDs, ok := jsonReferenceIDs(raw)
				if !ok {
					t.Errorf("%s.type is not a sequence of references: %v", element["@id"], raw)
					continue
				}
				if !allKnown {
					t.Errorf("%s.type is stated although an ownedTyping target cannot be computed", element["@id"])
					continue
				}
				actual := make(map[string]bool, len(typeIDs))
				for _, typeID := range typeIDs {
					actual[typeID] = true
				}
				for typeID := range expected {
					if !actual[typeID] {
						t.Errorf("%s.type omits ownedTyping type %s: %v", element["@id"], typeID, typeIDs)
					}
				}
			}
		})
	}
}

func TestFullAPIJSONFeatureTypeIncludesSubsettingAndChainingInputs(t *testing.T) {
	for _, path := range modelFiles(t) {
		fixture, _ := fixtureName(path)
		t.Run(fixture, func(t *testing.T) {
			elements := fullAPIJSONElements(t, fixture)
			byID := make(map[string]map[string]any, len(elements))
			for _, element := range elements {
				if id, ok := element["@id"].(string); ok {
					byID[id] = element
				}
			}
			for _, element := range elements {
				metaclass, ok := element["@type"].(string)
				if !ok || !ontology.IsAncestorOrSelf(metaclass, "Feature") {
					continue
				}
				sources := make([]string, 0)
				inputsKnown := true
				addTypes := func(featureID string) {
					feature := byID[featureID]
					if feature == nil {
						inputsKnown = false
						return
					}
					types, ok := jsonReferenceIDs(feature["type"])
					if !ok || len(types) == 0 {
						inputsKnown = false
						return
					}
					sources = append(sources, types...)
				}
				typingIDs, _ := jsonReferenceIDs(element["ownedTyping"])
				for _, typingID := range typingIDs {
					typing := byID[typingID]
					if typing == nil {
						inputsKnown = false
						continue
					}
					typeIDs, ok := jsonReferenceIDs(typing["type"])
					if !ok || len(typeIDs) == 0 {
						inputsKnown = false
						continue
					}
					sources = append(sources, typeIDs...)
				}
				subsettingIDs, _ := jsonReferenceIDs(element["ownedSubsetting"])
				for _, subsettingID := range subsettingIDs {
					subsetting := byID[subsettingID]
					if subsetting == nil {
						inputsKnown = false
						continue
					}
					targetIDs := make([]string, 0)
					for _, property := range []string{"redefinedFeature", "referencedFeature", "subsettedFeature"} {
						ids, ok := jsonReferenceIDs(subsetting[property])
						if ok {
							targetIDs = append(targetIDs, ids...)
						}
					}
					if len(targetIDs) == 0 {
						inputsKnown = false
						continue
					}
					for _, targetID := range targetIDs {
						addTypes(targetID)
					}
				}
				chainingIDs, _ := jsonReferenceIDs(element["chainingFeature"])
				if len(chainingIDs) > 0 {
					addTypes(chainingIDs[len(chainingIDs)-1])
				}
				raw, present := element["type"]
				if !inputsKnown {
					if present {
						t.Errorf("%s.%s states Feature::type while a typing input is unavailable", fixture, element["@id"])
					}
					continue
				}
				if len(sources) == 0 {
					continue
				}
				if !present {
					continue
				}
				typeIDs, ok := jsonReferenceIDs(raw)
				if !ok || len(typeIDs) == 0 {
					t.Errorf("%s.%s omits Feature::type despite computable typing inputs %v", fixture, element["@id"], sources)
					continue
				}
				actual := make(map[string]bool, len(typeIDs))
				for _, typeID := range typeIDs {
					actual[typeID] = true
				}
				derivationKnown := true
				for _, sourceID := range sources {
					specializes, known := fullJSONTypeSpecializes(sourceID, typeIDs, byID)
					if actual[sourceID] {
						continue
					}
					if !known {
						derivationKnown = false
						continue
					}
					if specializes {
						continue
					}
					t.Errorf("%s.%s.type omits input type %s: got %v", fixture, element["@id"], sourceID, typeIDs)
				}
				if !derivationKnown {
					t.Errorf("%s.%s states Feature::type while type redundancy is unknown", fixture, element["@id"])
				}
			}
		})
	}
}

func fullJSONTypeSpecializes(sourceID string, actualTypeIDs []string, byID map[string]map[string]any) (bool, bool) {
	known := true
	for _, actualID := range actualTypeIDs {
		seen := make(map[string]bool)
		stack := []string{actualID}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if current == sourceID {
				return true, true
			}
			if seen[current] {
				continue
			}
			seen[current] = true
			element := byID[current]
			if element == nil {
				known = false
				continue
			}
			rawRelationships, present := element["ownedSpecialization"]
			if !present {
				known = false
				continue
			}
			relationships, ok := jsonReferenceIDs(rawRelationships)
			if !ok {
				known = false
				continue
			}
			for _, relationshipID := range relationships {
				relationship := byID[relationshipID]
				if relationship == nil {
					known = false
					continue
				}
				rawGeneral, present := relationship["general"]
				if !present {
					known = false
					continue
				}
				generalIDs, ok := jsonReferenceIDs(rawGeneral)
				if !ok || len(generalIDs) == 0 {
					known = false
					continue
				}
				stack = append(stack, generalIDs...)
			}
		}
	}
	return false, known
}

func TestFullAPIJSONCommentAnnotationDerivations(t *testing.T) {
	for _, test := range []struct {
		fixture  string
		id       string
		target   string
		hasAbout bool
	}{
		{fixture: "chain_and_bare_comment", id: "Chains__Named", target: "Chains__V", hasAbout: true},
		{fixture: "chain_and_bare_comment", id: "Chains___400", target: "Chains"},
		{fixture: "docs", id: "Documented__Wheel___400", target: "Documented__Wheel"},
		{fixture: "docs", id: "Documented___402", target: "Documented__Wheel", hasAbout: true},
	} {
		t.Run(test.id, func(t *testing.T) {
			elements := fullAPIJSONElements(t, test.fixture)
			element := fullJSONElementByID(t, elements, test.id)
			ids, ok := jsonReferenceIDs(element["annotatedElement"])
			if !ok || len(ids) != 1 || ids[0] != test.target {
				t.Fatalf("%s.annotatedElement = %v, want [%s]", test.id, element["annotatedElement"], test.target)
			}
			annotation, hasAnnotation := element["annotation"]
			owned, hasOwned := element["ownedAnnotatingRelationship"]
			if test.hasAbout {
				if hasAnnotation || hasOwned {
					t.Errorf("%s unexpectedly serializes annotation relationships: annotation=%v ownedAnnotatingRelationship=%v", test.id, annotation, owned)
				}
				return
			}
			if !hasAnnotation || !hasOwned {
				t.Errorf("%s omits empty annotation relationships: annotation present=%t ownedAnnotatingRelationship present=%t", test.id, hasAnnotation, hasOwned)
			}
			annotationIDs, annotationOK := jsonReferenceIDs(annotation)
			if !annotationOK || len(annotationIDs) != 0 {
				t.Errorf("%s.annotation = %v, want an empty sequence", test.id, annotation)
			}
			ownedIDs, ownedOK := jsonReferenceIDs(owned)
			if !ownedOK || len(ownedIDs) != 0 {
				t.Errorf("%s.ownedAnnotatingRelationship = %v, want an empty sequence", test.id, owned)
			}
		})
	}
}

func TestFullAPIJSONCommentAboutTargetsPreserveOrder(t *testing.T) {
	full, err := convert.ConvertWith("annotation.sysml", []byte(`package P {
		part def A;
		part def B;
		comment C about B, A /* comment */
	}`), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	comment := fullJSONElementByID(t, elements, "P__C")
	ids, ok := jsonReferenceIDs(comment["annotatedElement"])
	if !ok || len(ids) != 2 || ids[0] != "P__B" || ids[1] != "P__A" {
		t.Fatalf("Comment.annotatedElement = %v, want [P__B P__A]", comment["annotatedElement"])
	}
	if _, ok := comment["annotation"]; ok {
		t.Errorf("Comment.annotation is present, want unsupported")
	}
	if _, ok := comment["ownedAnnotatingRelationship"]; ok {
		t.Errorf("Comment.ownedAnnotatingRelationship is present, want unsupported")
	}
}

func TestFullAPIJSONPrefixMetadataAnnotationDerivations(t *testing.T) {
	elements := fullAPIJSONElements(t, "metadata_prefixes")
	annotation := fullJSONElementByID(t, elements, "Prefixed___409___400_an")
	for _, property := range []string{"annotatingElement", "ownedAnnotatingElement"} {
		ids, ok := jsonReferenceIDs(annotation[property])
		if !ok || len(ids) != 1 || ids[0] != "Prefixed___409___400" {
			t.Errorf("Annotation.%s = %v, want [Prefixed___409___400]", property, annotation[property])
		}
	}
	if ids, ok := jsonReferenceIDs(annotation["owningAnnotatingElement"]); !ok || len(ids) != 0 {
		t.Errorf("Annotation.owningAnnotatingElement = %v, want null", annotation["owningAnnotatingElement"])
	}
	usage := fullJSONElementByID(t, elements, "Prefixed___409___400")
	annotationIDs, ok := jsonReferenceIDs(usage["annotation"])
	if !ok || len(annotationIDs) != 1 || annotationIDs[0] != "Prefixed___409___400_an" {
		t.Errorf("MetadataUsage.annotation = %v, want [Prefixed___409___400_an]", usage["annotation"])
	}
	if ids, ok := jsonReferenceIDs(usage["ownedAnnotatingRelationship"]); !ok || len(ids) != 0 {
		t.Errorf("MetadataUsage.ownedAnnotatingRelationship = %v, want an empty sequence", usage["ownedAnnotatingRelationship"])
	}
	annotatedIDs, ok := jsonReferenceIDs(usage["annotatedElement"])
	if !ok || len(annotatedIDs) != 1 || annotatedIDs[0] != "Prefixed___409" {
		t.Errorf("MetadataUsage.annotatedElement = %v, want [Prefixed___409]", usage["annotatedElement"])
	}
	relatedIDs, ok := jsonReferenceIDs(annotation["ownedRelatedElement"])
	if !ok || len(relatedIDs) != 1 || relatedIDs[0] != "Prefixed___409___400" {
		t.Errorf("Annotation.ownedRelatedElement = %v, want [Prefixed___409___400]", annotation["ownedRelatedElement"])
	}
}

func assertFullFeatureTyping(t *testing.T, fixture, featureID, typeID string) {
	t.Helper()
	path := convertFixturePath(t, fixture)
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	full, err := convert.ConvertWith(path, source, convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]json.RawMessage, len(elements))
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err == nil {
			byID[id] = element
		}
	}
	feature := byID[featureID]
	if feature == nil {
		t.Fatalf("full API JSON has no feature %s", featureID)
	}
	var typValues []map[string]string
	if err := json.Unmarshal(feature["type"], &typValues); err != nil {
		t.Fatalf("decode Feature::type: %v", err)
	}
	for _, typ := range typValues {
		if typ["@id"] == typeID {
			return
		}
	}
	var ownedTyping []map[string]string
	if err := json.Unmarshal(feature["ownedTyping"], &ownedTyping); err != nil {
		t.Fatalf("decode Feature::ownedTyping: %v", err)
	}
	t.Fatalf("Feature::type for %s = %v, want typing target %s (ownedTyping=%v)", featureID, typValues, typeID, ownedTyping)
}

func TestFullAPIJSONUsesGraphMembershipIDs(t *testing.T) {
	const source = `package P {
		part def Base { part size; }
		part def Derived specializes Base { part mass; }
	}`
	full, err := convert.ConvertWith("m.sysml", []byte(source), convert.FormatSysML, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(full, &elements); err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]bool, len(elements))
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		ids[id] = true
	}
	for _, element := range elements {
		var qualifiedName string
		_ = json.Unmarshal(element["qualifiedName"], &qualifiedName)
		if qualifiedName != "P" {
			continue
		}
		var memberships []map[string]string
		if err := json.Unmarshal(element["membership"], &memberships); err != nil {
			t.Fatal(err)
		}
		if len(memberships) == 0 {
			t.Fatal("package P has no membership references")
		}
		for _, membership := range memberships {
			if !ids[membership["@id"]] {
				t.Errorf("membership %q has no graph element", membership["@id"])
			}
		}
		return
	}
	t.Fatal("full API JSON omitted package P")
}

func TestFullAPIJSONRejectsGraphInputs(t *testing.T) {
	_, err := convert.ConvertWith("m.ttl", []byte("@prefix sysml: <https://www.omg.org/spec/SysML/> ."), convert.FormatTurtle, convert.FormatAPIJSON, convert.Options{
		APIJSON: export.APIJSONFull,
	})
	if err == nil || err.Error() != "full API JSON form requires SysML or KerML input; graph inputs are not supported" {
		t.Fatalf("full form on a graph input = %v, want a clear unsupported-input error", err)
	}
}

func TestFullAPIJSONModelUsesSharedDocumentModel(t *testing.T) {
	out, err := convert.ConvertModel([]convert.Input{
		{Name: "types.sysml", Data: []byte(`package Types { part def Vehicle; }`)},
		{Name: "uses.sysml", Data: []byte(`package Uses { part car : Types::Vehicle; }`)},
	}, convert.FormatAPIJSON, convert.Options{APIJSON: export.APIJSONFull})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, element := range elements {
		var name string
		_ = json.Unmarshal(element["name"], &name)
		if name != "car" {
			continue
		}
		var types []map[string]string
		if err := json.Unmarshal(element["type"], &types); err != nil {
			t.Fatal(err)
		}
		if len(types) == 0 || types[0]["@id"] == "" {
			t.Error("car.type has no reference id")
		}
		found = true
	}
	if !found {
		t.Fatal("full API JSON omitted the car usage")
	}
}
