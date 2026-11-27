package export_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

const (
	normativeSchemaPath     = "../../build/omg-json-schema/SysML.json"
	normativeSchemaEnv      = "OPENSYSML_REQUIRE_OMG_JSON_SCHEMA"
	normativeSchemaBaseline = "testdata/api_json_normative_schema_baseline.json"
	normativeSchemaResource = "https://www.omg.org/spec/SysML/20250201/SysML.json"
)

var recordNormativeSchema = flag.Bool("update-normative-schema", false,
	"re-record the api-json normative-schema baseline; refused when a figure grew over the same elements")

// normativeSchemaGap is what the api-json element form of the convert fixtures lacks
// against the normative SysML.json, counted per element occurrence.
type normativeSchemaGap struct {
	Fixtures        int `json:"fixtures"`
	Elements        int `json:"elements"`
	InvalidElements int `json:"invalidElements"`
	// MissingRequired counts the elements without each property their metaclass requires.
	MissingRequired map[string]int `json:"missingRequired"`
	// NonSchemaKeys counts the elements carrying each key their metaclass does not define.
	NonSchemaKeys map[string]int `json:"nonSchemaKeys"`
	// InvalidValues counts, per property and schema keyword, the values the schema refuses.
	InvalidValues map[string]int `json:"invalidValues"`
	// UnknownMetaclasses counts the elements of each @type the schema does not define.
	UnknownMetaclasses map[string]int `json:"unknownMetaclasses"`
}

func newNormativeSchemaGap() *normativeSchemaGap {
	return &normativeSchemaGap{
		MissingRequired: map[string]int{}, NonSchemaKeys: map[string]int{},
		InvalidValues: map[string]int{}, UnknownMetaclasses: map[string]int{},
	}
}

// cells flattens the gap into the figures the ratchet compares; the fixture and
// element totals describe the measured input and are not ratcheted.
func (g *normativeSchemaGap) cells() map[string]int {
	cells := map[string]int{"invalidElements": g.InvalidElements}
	for group, counts := range map[string]map[string]int{
		"missingRequired": g.MissingRequired, "nonSchemaKeys": g.NonSchemaKeys,
		"invalidValues": g.InvalidValues, "unknownMetaclasses": g.UnknownMetaclasses,
	} {
		for name, count := range counts {
			cells[group+"/"+name] = count
		}
	}
	return cells
}

// normativeSchemaMovement lists the cells that differ between two gaps, sorted,
// split into the ones that grew and the ones that shrank.
func normativeSchemaMovement(recorded, measured *normativeSchemaGap) (grew, shrank []string) {
	before, after := recorded.cells(), measured.cells()
	names := make(map[string]bool, len(before)+len(after))
	for name := range before {
		names[name] = true
	}
	for name := range after {
		names[name] = true
	}
	for name := range names {
		line := fmt.Sprintf("%s: %d -> %d", name, before[name], after[name])
		switch {
		case after[name] > before[name]:
			grew = append(grew, line)
		case after[name] < before[name]:
			shrank = append(shrank, line)
		}
	}
	sort.Strings(grew)
	sort.Strings(shrank)
	return grew, shrank
}

// normativeSchemas compiles, per metaclass, the object schema SysML.json gives an element of
// exactly that @type: the definition, or its anyOf branch that is not a specialization.
type normativeSchemas struct {
	compiler *jsonschema.Compiler
	pointers map[string]string
	compiled map[string]*jsonschema.Schema
}

func loadNormativeSchemas(file []byte) (*normativeSchemas, error) {
	var document struct {
		Defs map[string]struct {
			Properties json.RawMessage `json:"properties"`
			AnyOf      []struct {
				Properties json.RawMessage `json:"properties"`
			} `json:"anyOf"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(file, &document); err != nil {
		return nil, err
	}
	schemas := &normativeSchemas{pointers: map[string]string{}, compiled: map[string]*jsonschema.Schema{}}
	for name, definition := range document.Defs {
		if definition.Properties != nil {
			schemas.pointers[name] = "#/$defs/" + name
			continue
		}
		for i, branch := range definition.AnyOf {
			if branch.Properties != nil {
				schemas.pointers[name] = fmt.Sprintf("#/$defs/%s/anyOf/%d", name, i)
				break
			}
		}
	}
	parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(file))
	if err != nil {
		return nil, err
	}
	schemas.compiler = jsonschema.NewCompiler()
	schemas.compiler.AssertFormat()
	if err := schemas.compiler.AddResource(normativeSchemaResource, parsed); err != nil {
		return nil, err
	}
	return schemas, nil
}

func (s *normativeSchemas) of(metaclass string) (*jsonschema.Schema, bool, error) {
	if schema, ok := s.compiled[metaclass]; ok {
		return schema, true, nil
	}
	pointer, ok := s.pointers[metaclass]
	if !ok {
		return nil, false, nil
	}
	schema, err := s.compiler.Compile(normativeSchemaResource + pointer)
	if err != nil {
		return nil, false, err
	}
	s.compiled[metaclass] = schema
	return schema, true, nil
}

// record validates one element and adds what the schema refuses to the gap.
func (g *normativeSchemaGap) record(schemas *normativeSchemas, element any) error {
	g.Elements++
	object, _ := element.(map[string]any)
	metaclass, _ := object["@type"].(string)
	schema, known, err := schemas.of(metaclass)
	if err != nil {
		return err
	}
	if !known {
		g.UnknownMetaclasses[metaclass]++
		g.InvalidElements++
		return nil
	}
	err = schema.Validate(element)
	if err == nil {
		return nil
	}
	var invalid *jsonschema.ValidationError
	if !errors.As(err, &invalid) {
		return err
	}
	g.InvalidElements++
	values := map[string]bool{}
	var walk func(*jsonschema.ValidationError)
	walk = func(cause *jsonschema.ValidationError) {
		if len(cause.InstanceLocation) == 0 {
			switch refused := cause.ErrorKind.(type) {
			case *kind.Required:
				for _, property := range refused.Missing {
					g.MissingRequired[property]++
				}
			case *kind.AdditionalProperties:
				for _, key := range refused.Properties {
					g.NonSchemaKeys[key]++
				}
			}
		} else if len(cause.Causes) == 0 {
			keywords := cause.ErrorKind.KeywordPath()
			values[cause.InstanceLocation[0]+" "+strings.Join(keywords, "/")] = true
		}
		for _, nested := range cause.Causes {
			walk(nested)
		}
	}
	walk(invalid)
	for value := range values {
		g.InvalidValues[value]++
	}
	return nil
}

// TestAPIJSONNormativeSchema measures the api-json element form of every convert fixture against
// the normative SysML.json (KerML 10.4.1, 10.4.5) and holds the gap, not yet zero, to a baseline.
func TestAPIJSONNormativeSchema(t *testing.T) {
	file, err := os.ReadFile(normativeSchemaPath)
	if err != nil {
		message := "normative SysML.json absent; run ./scripts/download-omg-json-schema.sh"
		if os.Getenv(normativeSchemaEnv) != "" {
			t.Fatalf("%s (%s is set): %v", message, normativeSchemaEnv, err)
		}
		t.Skip(message)
	}
	schemas, err := loadNormativeSchemas(file)
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := filepath.Glob(filepath.Join("testdata", "convert", "*.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	measured := newNormativeSchemaGap()
	for _, path := range fixtures {
		if strings.Contains(filepath.Base(path), ".golden.") {
			continue
		}
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		document, err := cachedAPIJSON(path, source, "standard", "graph")
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		parsed, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		elements, ok := parsed.([]any)
		if !ok {
			t.Fatalf("%s: the element form is not an array (KerML 10.4.6)", path)
		}
		measured.Fixtures++
		for _, element := range elements {
			if err := measured.record(schemas, element); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
		}
	}
	if measured.Elements == 0 {
		t.Fatal("no element measured")
	}
	t.Logf("normative schema: %d of %d elements invalid over %d fixtures; %d required-property absences, %d non-schema keys",
		measured.InvalidElements, measured.Elements, measured.Fixtures,
		sumCounts(measured.MissingRequired), sumCounts(measured.NonSchemaKeys))

	recorded := newNormativeSchemaGap()
	baseline, err := os.ReadFile(normativeSchemaBaseline)
	switch {
	case err == nil:
		if err := json.Unmarshal(baseline, recorded); err != nil {
			t.Fatal(err)
		}
	case !*recordNormativeSchema || !os.IsNotExist(err):
		t.Fatal(err)
	}
	grew, shrank := normativeSchemaMovement(recorded, measured)
	if *recordNormativeSchema {
		// Over the same elements a figure may only shrink; a fixture added or resized moves them with the input.
		sameInput := recorded.Fixtures == measured.Fixtures && recorded.Elements == measured.Elements
		if baseline != nil && sameInput && len(grew) > 0 {
			t.Fatalf("refusing to record a larger gap over the same elements; the baseline may only shrink:\n%s", strings.Join(grew, "\n"))
		}
		encoded, err := json.MarshalIndent(measured, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(normativeSchemaBaseline, append(encoded, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if len(grew) > 0 {
		t.Errorf("the element form moved away from the normative schema:\n%s", strings.Join(grew, "\n"))
	}
	if len(shrank) > 0 {
		t.Errorf("the element form moved toward the normative schema; record it with -update-normative-schema:\n%s",
			strings.Join(shrank, "\n"))
	}
	if recorded.Fixtures != measured.Fixtures || recorded.Elements != measured.Elements {
		t.Errorf("the baseline measured %d elements over %d fixtures, this run %d over %d; re-record with -update-normative-schema",
			recorded.Elements, recorded.Fixtures, measured.Elements, measured.Fixtures)
	}
}

func sumCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}
