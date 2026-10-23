package export_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// apiJSONModels lists every notation fixture under testdata, recursively, plus
// the interop model, whose element form the inverse contract is checked on.
func apiJSONModels(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.Walk("testdata", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		ext := filepath.Ext(path)
		if info.IsDir() || (ext != ".sysml" && ext != ".kerml") || strings.Contains(path, ".golden.") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out = append(out, filepath.Join("..", "..", "tools", "referee", "flexo", "testdata", "model.sysml"))
	sort.Strings(out)
	return out
}

// tripleSet is a graph's triples as a set; the API element form preserves the
// statements, not the spelling order inside a subject.
func tripleSet(g *rdf.Graph) map[rdf.Triple]bool {
	out := map[rdf.Triple]bool{}
	for _, triple := range g.Triples() {
		out[triple] = true
	}
	return out
}

// The API element form is a serializer over the same graph as Turtle: writing
// it and reading it back must yield the graph's triple set exactly, and writing
// the graph it read must spell the document the same way again.
func TestAPIJSONInverseOverAllFixtures(t *testing.T) {
	refused := 0
	for _, path := range apiJSONModels(t) {
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := convert.SysMLToRDF(path, src)
			var unsupported *export.UnsupportedError
			if errors.As(err, &unsupported) {
				refused++
				t.Skipf("ToRDF refuses this model: %v", err)
			}
			if err != nil {
				t.Fatalf("SysMLToRDF: %v", err)
			}
			document, err := export.WriteAPIJSON(graph)
			if err != nil {
				t.Fatalf("WriteAPIJSON: %v", err)
			}
			reread, err := export.ReadAPIJSON(document)
			if err != nil {
				t.Fatalf("ReadAPIJSON: %v\n%s", err, document)
			}
			// The element form carries no library references: a reader resolves
			// a normative id against the library, so their names are not carried.
			want := tripleSet(graph)
			for triple := range want {
				if export.LibraryReference(graph, triple.Subject) {
					delete(want, triple)
				}
			}
			if !sameTriples(want, tripleSet(reread)) {
				missing := diffTriples(want, tripleSet(reread))
				added := diffTriples(tripleSet(reread), want)
				t.Fatalf("the graph changed through the element form\nmissing: %v\nadded: %v", missing, added)
			}
			again, err := export.WriteAPIJSON(reread)
			if err != nil {
				t.Fatalf("WriteAPIJSON of the read graph: %v", err)
			}
			if !bytes.Equal(document, again) {
				t.Fatalf("the element form was not written the same way again\n--- first ---\n%s\n--- again ---\n%s", document, again)
			}
		})
	}
	if refused == len(apiJSONModels(t)) {
		t.Fatal("ToRDF refused every fixture; the gate checked nothing")
	}
}

// Develop's reader matched the standard route for this set; its compact reader
// did not accept the compact document envelope.
var apiJSONDevelopStandardRouteParityFixtures = []string{
	"../../tools/referee/flexo/testdata/model.sysml",
	"testdata/convert/accept.sysml",
	"testdata/convert/action_nodes.sysml",
	"testdata/convert/anonymous_assertions.sysml",
	"testdata/convert/anonymous_features.kerml",
	"testdata/convert/anonymous_portions.sysml",
	"testdata/convert/body_scopes.sysml",
	"testdata/convert/bounds.sysml",
	"testdata/convert/chain_and_bare_comment.sysml",
	"testdata/convert/chain_scopes.sysml",
	"testdata/convert/composite_multiplicity.kerml",
	"testdata/convert/condition_references.sysml",
	"testdata/convert/conjugation.sysml",
	"testdata/convert/connector_ends.kerml",
	"testdata/convert/constraints.sysml",
	"testdata/convert/decision_branches.sysml",
	"testdata/convert/deps.sysml",
	"testdata/convert/docs.sysml",
	"testdata/convert/docs_before_members.sysml",
	"testdata/convert/end_binding_bodies.sysml",
	"testdata/convert/end_prefix_metadata.sysml",
	"testdata/convert/endpoint_scopes.sysml",
	"testdata/convert/enums.sysml",
	"testdata/convert/expression_body_members.sysml",
	"testdata/convert/expression_body_order.sysml",
	"testdata/convert/feature_chains.sysml",
	"testdata/convert/feature_chains_repeated.sysml",
	"testdata/convert/feature_chains_unresolved.sysml",
	"testdata/convert/feature_prefixes.kerml",
	"testdata/convert/imported_references.sysml",
	"testdata/convert/imports.sysml",
	"testdata/convert/individual_definitions.sysml",
	"testdata/convert/invariant_references.kerml",
	"testdata/convert/irregular_layout.sysml",
	"testdata/convert/kerml.sysml",
	"testdata/convert/kind_keyword_synonyms.sysml",
	"testdata/convert/loop_scopes.sysml",
	"testdata/convert/loops_conditionals.sysml",
	"testdata/convert/metadata_bodies.sysml",
	"testdata/convert/metadata_prefixes.sysml",
	"testdata/convert/modifiers.sysml",
	"testdata/convert/names.sysml",
	"testdata/convert/nested.sysml",
	"testdata/convert/nested_namespace_import.sysml",
	"testdata/convert/parts.sysml",
	"testdata/convert/payload_declaration_bodies.sysml",
	"testdata/convert/quoted_names.sysml",
	"testdata/convert/quoted_succession_ends.sysml",
	"testdata/convert/ref_subsets.sysml",
	"testdata/convert/relations.sysml",
	"testdata/convert/relationship_clause_order.sysml",
	"testdata/convert/requirements.sysml",
	"testdata/convert/result_expressions.sysml",
	"testdata/convert/send_new.sysml",
	"testdata/convert/shadowed_references.sysml",
	"testdata/convert/shadowing_body.sysml",
	"testdata/convert/shadowing_parameters.sysml",
	"testdata/convert/state_machine.sysml",
	"testdata/convert/then_after_members.sysml",
	"testdata/convert/values.sysml",
	"testdata/convert/views_flows_parameters.sysml",
	"testdata/interchange/flow_ends.sysml",
	"testdata/interchange/library_identity.sysml",
	"testdata/interchange/p10_interchange.sysml",
	"testdata/interchange/state_entry.sysml",
	"testdata/interchange/states.sysml",
	"testdata/interchange/successions.sysml",
	"testdata/interchange/transitions.sysml",
}

func TestAPIJSONConvertRoutesMatchTurtleOnDevelopPassingFixtures(t *testing.T) {
	expected := make(map[string]struct{}, len(apiJSONDevelopStandardRouteParityFixtures))
	for _, path := range apiJSONDevelopStandardRouteParityFixtures {
		expected[filepath.Clean(path)] = struct{}{}
	}
	for _, path := range apiJSONModels(t) {
		path = filepath.Clean(path)
		if _, ok := expected[path]; !ok {
			continue
		}
		delete(expected, path)
		name, _ := fixtureName(path)
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := cachedSysMLToRDF(path, source)
			if err != nil {
				t.Fatalf("SysMLToRDF: %v", err)
			}
			want := rdf.WriteTurtle(apiJSONGraphWithoutLibraryReferences(graph))
			document, err := cachedAPIJSON(path, source, "standard", "graph")
			if err != nil {
				t.Fatalf("write standard API JSON: %v", err)
			}
			got, err := convert.Convert(path+".json", document, convert.FormatAPIJSON, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("read standard API JSON as Turtle: %v", err)
			}
			if !bytes.Equal(got, want) {
				offset := firstDifferentByte(got, want)
				start := max(0, offset-120)
				gotEnd := min(len(got), offset+220)
				wantEnd := min(len(want), offset+220)
				t.Logf("got near mismatch: %q", got[start:gotEnd])
				t.Logf("want near mismatch: %q", want[start:wantEnd])
				t.Fatalf("standard API JSON route differs from SysML-to-Turtle at byte %d (got %d bytes, want %d)",
					offset, len(got), len(want))
			}
		})
	}
	if len(expected) > 0 {
		missing := make([]string, 0, len(expected))
		for path := range expected {
			missing = append(missing, path)
		}
		sort.Strings(missing)
		t.Fatalf("develop-passing API JSON fixtures are missing: %s", strings.Join(missing, ", "))
	}
}

func firstDifferentByte(got, want []byte) int {
	offset := 0
	for offset < len(got) && offset < len(want) && got[offset] == want[offset] {
		offset++
	}
	return offset
}

func TestAPIJSONWritersNeverEmitNameReferences(t *testing.T) {
	paths, examplesRoot := apiJSONQualifiedNamePaths(t)
	conversions := corpusConversions(t)
	filesWithMappingValues := 0
	convertedFiles := 0
	fixtureFilesWithMappingValues := 0
	convertedFixtures := 0
	exampleFilesWithMappingValues := 0
	convertedExamples := 0
	mappingPropertyEntries := make(map[string]int)
	mappingValueEntries := make(map[string]int)
	examplesRoot = filepath.Clean(examplesRoot)
	var metricsMu sync.Mutex
	t.Cleanup(func() {
		metricsMu.Lock()
		defer metricsMu.Unlock()
		totalMappingValueEntries := 0
		for _, count := range mappingValueEntries {
			totalMappingValueEntries += count
		}
		totalMappingPropertyEntries := 0
		for _, count := range mappingPropertyEntries {
			totalMappingPropertyEntries += count
		}
		t.Logf(
			"mappingValues: %d/%d fixtures and %d/%d examples (%d/%d total); %d property entries by property: %v; %d values by property: %v",
			fixtureFilesWithMappingValues, convertedFixtures, exampleFilesWithMappingValues, convertedExamples,
			filesWithMappingValues, convertedFiles, totalMappingPropertyEntries, mappingPropertyEntries,
			totalMappingValueEntries, mappingValueEntries,
		)
	})
	for _, path := range paths {
		t.Run(filepath.Clean(path), func(t *testing.T) {
			t.Parallel()
			defer conversions.release(path)
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := conversions.sysMLToRDF(path, source)
			if err != nil {
				if strings.HasPrefix(filepath.Clean(path), examplesRoot+string(filepath.Separator)) {
					t.Skipf("example does not convert: %v", err)
				}
				t.Fatal(err)
			}
			var unsupported *export.UnsupportedError
			semantic, err := conversions.apiJSON(path, source, "standard", "semantic")
			if errors.As(err, &unsupported) {
				t.Skipf("semantic API JSON refuses this model: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(semantic, []byte(`"@ref"`)) {
				t.Error("semantic API JSON output contains @ref")
			}
			standard, err := conversions.apiJSON(path, source, "standard", "graph")
			if errors.As(err, &unsupported) {
				t.Skipf("standard API JSON refuses this graph: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			isExample := strings.HasPrefix(filepath.Clean(path), examplesRoot+string(os.PathSeparator))
			var standardElements []map[string]json.RawMessage
			if err := json.Unmarshal(standard, &standardElements); err != nil {
				t.Fatalf("read standard API JSON for provenance counts: %v", err)
			}
			fileHasMappingValues := false
			for _, element := range standardElements {
				raw, ok := element["sysx:mappingValues"]
				if !ok {
					continue
				}
				var properties map[string]json.RawMessage
				if err := json.Unmarshal(raw, &properties); err != nil {
					t.Fatalf("read mappingValues: %v", err)
				}
				fileHasMappingValues = fileHasMappingValues || len(properties) > 0
				for property, rawValues := range properties {
					var values []json.RawMessage
					if err := json.Unmarshal(rawValues, &values); err != nil {
						t.Fatalf("read mappingValues.%s: %v", property, err)
					}
					metricsMu.Lock()
					mappingPropertyEntries[property]++
					mappingValueEntries[property] += len(values)
					metricsMu.Unlock()
				}
			}
			compact, err := conversions.apiJSON(path, source, "compact", "graph")
			if errors.As(err, &unsupported) {
				t.Skipf("compact API JSON refuses this graph: %v", err)
			}
			if err != nil {
				t.Fatal(err)
			}
			semanticCompact, err := conversions.apiJSON(path, source, "compact", "semantic")
			if err != nil {
				t.Fatalf("compact semantic API JSON: %v", err)
			}
			var omissions []export.APIJSONOmission
			recordOmission := func(omission export.APIJSONOmission) {
				omissions = append(omissions, omission)
			}
			if _, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{Omitted: recordOmission}); err != nil {
				t.Fatalf("write standard API JSON for unresolved-value sweep: %v", err)
			}
			if _, err := export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{Omitted: recordOmission}); err != nil {
				t.Fatalf("write compact API JSON for unresolved-value sweep: %v", err)
			}
			for format, document := range map[string][]byte{
				"standard":         standard,
				"compact":          compact,
				"semantic":         semantic,
				"semantic-compact": semanticCompact,
			} {
				if bytes.Contains(document, []byte(`"@ref"`)) {
					t.Errorf("%s output contains @ref", format)
				}
				assertAPIJSONUnresolvedValuesOmitted(t, path, format, document, omissions)
			}
			metricsMu.Lock()
			convertedFiles++
			if isExample {
				convertedExamples++
			} else {
				convertedFixtures++
			}
			if fileHasMappingValues {
				filesWithMappingValues++
				if isExample {
					exampleFilesWithMappingValues++
				} else {
					fixtureFilesWithMappingValues++
				}
			}
			metricsMu.Unlock()
		})
	}
}

func assertAPIJSONUnresolvedValuesOmitted(
	t *testing.T,
	path string,
	format string,
	document []byte,
	omissions []export.APIJSONOmission,
) {
	t.Helper()
	var elements []map[string]json.RawMessage
	if format == "compact" || format == "semantic-compact" {
		var compact struct {
			IDs          []string                     `json:"ids"`
			ElementCount int                          `json:"elementCount"`
			Elements     []map[string]json.RawMessage `json:"elements"`
		}
		if err := json.Unmarshal(document, &compact); err != nil {
			t.Fatalf("%s %s API JSON is invalid: %v", path, format, err)
		}
		if len(compact.Elements) != compact.ElementCount || compact.ElementCount > len(compact.IDs) {
			t.Fatalf("%s %s API JSON element IDs do not match its elements", path, format)
		}
		elements = compact.Elements
		for i := 0; i < compact.ElementCount; i++ {
			id, err := json.Marshal(compact.IDs[i])
			if err != nil {
				t.Fatalf("%s %s API JSON element ID %d: %v", path, format, i, err)
			}
			elements[i]["@id"] = id
		}
	} else if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("%s %s API JSON is invalid: %v", path, format, err)
	}

	unresolved := make(map[string]map[string]bool)
	for _, omission := range omissions {
		if omission.Reason.Category != metamodel.ReasonUnresolvedReference {
			continue
		}
		if unresolved[omission.Subject] == nil {
			unresolved[omission.Subject] = make(map[string]bool)
		}
		unresolved[omission.Subject][omission.Property] = true
	}
	for _, element := range elements {
		var subject, metaclass string
		if err := json.Unmarshal(element["@id"], &subject); err != nil {
			continue
		}
		if err := json.Unmarshal(element["@type"], &metaclass); err != nil {
			continue
		}
		properties := make(map[string]bool)
		if raw, ok := element["sysx:mappingValues"]; ok {
			var mappingValues map[string]json.RawMessage
			if err := json.Unmarshal(raw, &mappingValues); err != nil {
				t.Errorf("%s %s element %s has invalid mappingValues: %v", path, format, subject, err)
				continue
			}
			for property := range mappingValues {
				properties[property] = true
			}
		}
		for property := range unresolved[subject] {
			properties[property] = true
		}
		for property := range properties {
			definition, ok := ontology.PropertyOf(apiJSONTestOntologyMetaclass(metaclass), property)
			if !ok || (!definition.Many && definition.Lower != 1) {
				continue
			}
			value, present := element[property]
			if !present {
				continue
			}
			value = bytes.TrimSpace(value)
			if bytes.Equal(value, []byte("[]")) || bytes.Equal(value, []byte("null")) {
				t.Errorf("%s %s element %s.%s = %s for collection or [1..1] property",
					path, format, subject, property, value)
			}
		}
	}
}

func TestAPIJSONUnresolvedSingleObjectPropertyUsesOntologyLowerBound(t *testing.T) {
	type testCase struct {
		subject   rdf.Term
		metaclass string
		property  string
		value     string
	}
	cases := []testCase{
		{
			subject:   rdf.ElementIRIForID("feature-typing"),
			metaclass: "FeatureTyping",
			property:  "type",
			value:     "Ext::LV",
		},
		{
			subject:   rdf.ElementIRIForID("element-owner"),
			metaclass: "Element",
			property:  "owner",
			value:     "Ext::Owner",
		},
	}
	graphTriples := make([]rdf.Triple, 0, len(cases)*2)
	for _, tc := range cases {
		property, ok := ontology.PropertyOf(tc.metaclass, tc.property)
		if !ok || property.Kind != ontology.ObjectProperty || property.Many {
			t.Fatalf("%s.%s is not a single-valued object property", tc.metaclass, tc.property)
		}
		graphTriples = append(graphTriples,
			rdf.Triple{
				Subject: tc.subject, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(tc.metaclass),
			},
			rdf.Triple{
				Subject: tc.subject, Predicate: rdf.SysMLTerm(tc.property), Object: rdf.String(tc.value),
			},
		)
	}
	if required, _ := ontology.PropertyOf("FeatureTyping", "type"); required.Lower != 1 {
		t.Fatalf("FeatureTyping.type lower bound = %d, want 1", required.Lower)
	}
	if optional, _ := ontology.PropertyOf("Element", "owner"); optional.Lower != 0 {
		t.Fatalf("Element.owner lower bound = %d, want 0", optional.Lower)
	}
	graph := rdf.NewGraphOf(graphTriples, nil)
	var omissions []export.APIJSONOmission
	var diagnostics []string
	document, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{
		Omitted: func(omission export.APIJSONOmission) {
			omissions = append(omissions, omission)
		},
		Diagnostic: func(message string) {
			diagnostics = append(diagnostics, message)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(document, []byte(`"@ref"`)) {
		t.Fatalf("API JSON contains @ref:\n%s", document)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]any, len(elements))
	for _, element := range elements {
		if id, ok := element["@id"].(string); ok {
			byID[id] = element
		}
	}
	for _, tc := range cases {
		property, _ := ontology.PropertyOf(tc.metaclass, tc.property)
		element := byID[rdf.LocalName(tc.subject.Value)]
		value, present := element[tc.property]
		if property.Lower == 0 && (!present || value != nil) {
			t.Errorf("%s.%s = (%v, %t), want null at lower bound 0", tc.metaclass, tc.property, value, present)
		}
		if property.Lower > 0 && present {
			t.Errorf("%s.%s = %v, want omission at lower bound %d", tc.metaclass, tc.property, value, property.Lower)
		}
		mapping, ok := element["sysx:mappingValues"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no mappingValues sidecar:\n%s", tc.property, document)
		}
		values, ok := mapping[tc.property].([]any)
		if !ok || len(values) != 1 || values[0] != tc.value {
			t.Errorf("mappingValues.%s = %v, want [%q]", tc.property, mapping[tc.property], tc.value)
		}
	}
	if len(omissions) != len(cases) {
		t.Fatalf("unresolved omission diagnostics = %#v, want %d", omissions, len(cases))
	}
	if len(diagnostics) != len(cases) {
		t.Fatalf("diagnostics = %v, want %d unresolved-reference messages", diagnostics, len(cases))
	}
	omitted := make(map[string]export.APIJSONOmission, len(omissions))
	for _, omission := range omissions {
		omitted[omission.Subject+"."+omission.Property] = omission
	}
	for _, tc := range cases {
		key := rdf.LocalName(tc.subject.Value) + "." + tc.property
		omission, ok := omitted[key]
		if !ok || omission.Metaclass != tc.metaclass ||
			omission.Reason.Category != metamodel.ReasonUnresolvedReference ||
			omission.Reason.Detail != tc.value {
			t.Errorf("omission[%s] = %#v, want %s %q", key, omission, tc.metaclass, tc.value)
		}
		foundDiagnostic := false
		for _, diagnostic := range diagnostics {
			if strings.Contains(diagnostic, tc.metaclass+"."+tc.property+": "+tc.value) {
				foundDiagnostic = true
				break
			}
		}
		if !foundDiagnostic {
			t.Errorf("diagnostics = %v, want unresolved %s.%s message", diagnostics, tc.metaclass, tc.property)
		}
	}
	reread, err := export.ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		if !reread.Has(rdf.Triple{
			Subject: tc.subject, Predicate: rdf.SysMLTerm(tc.property), Object: rdf.String(tc.value),
		}) {
			t.Errorf("mappingValues did not restore %s.%s", tc.metaclass, tc.property)
		}
	}
	plain, err := export.ReadAPIJSON(apiJSONWithoutSysx(t, document))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		if len(plain.Objects(tc.subject, rdf.SysML+tc.property)) != 0 {
			t.Errorf("reading %s.%s without sysx metadata invented a reference", tc.metaclass, tc.property)
		}
	}
}

func TestAPIJSONMappingValuesRestoreExactObjectsAndStripToIRI(t *testing.T) {
	subject := rdf.ElementIRIForID("feature-typing")
	iriSubject := rdf.ElementIRIForID("feature-typing-iri")
	real := rdf.ElementIRIForID("14c0aa22-5489-59b5-b438-ded26e83ba31")
	graph := rdf.NewGraphOf([]rdf.Triple{
		{Subject: subject, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("FeatureTyping")},
		{Subject: subject, Predicate: rdf.SysMLTerm("type"), Object: rdf.String("Real")},
		{Subject: subject, Predicate: rdf.SysMLTerm("general"), Object: rdf.String("~ServiceDiscovery")},
		{Subject: iriSubject, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("FeatureTyping")},
		{Subject: iriSubject, Predicate: rdf.SysMLTerm("type"), Object: real},
	}, nil)
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	var mappingValues map[string][]any
	foundIRIReference := false
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			continue
		}
		switch id {
		case "feature-typing":
			if err := json.Unmarshal(element["sysx:mappingValues"], &mappingValues); err != nil {
				t.Fatalf("read mappingValues: %v", err)
			}
		case "feature-typing-iri":
			_, foundIRIReference = element["sysx:mappingValues"]
		}
	}
	if len(mappingValues["type"]) != 1 || mappingValues["type"][0] != "Real" {
		t.Fatalf("mappingValues[type] = %v, want [Real]", mappingValues["type"])
	}
	if len(mappingValues["general"]) != 1 || mappingValues["general"][0] != "~ServiceDiscovery" {
		t.Fatalf("mappingValues[general] = %v, want [~ServiceDiscovery]", mappingValues["general"])
	}
	if foundIRIReference {
		t.Fatal("an IRI-form graph reference gained stated-name provenance")
	}
	restored, err := export.ReadAPIJSON(document)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Has(rdf.Triple{
		Subject: subject, Predicate: rdf.SysMLTerm("type"), Object: rdf.String("Real"),
	}) {
		t.Fatal("mappingValues did not restore the source literal")
	}
	if !restored.Has(rdf.Triple{
		Subject: subject, Predicate: rdf.SysMLTerm("general"), Object: rdf.String("~ServiceDiscovery"),
	}) {
		t.Fatal("mappingValues did not restore an expression-like source string as a plain literal")
	}
	if objects := restored.Objects(subject, rdf.SysML+"type"); len(objects) != 1 || objects[0] != rdf.String("Real") {
		t.Fatalf("mappingValues did not replace the spec reference: %v", objects)
	}
	if !restored.Has(rdf.Triple{
		Subject: iriSubject, Predicate: rdf.SysMLTerm("type"), Object: real,
	}) {
		t.Fatal("an IRI-form graph reference changed on read")
	}
	for _, triple := range restored.Triples() {
		if triple.Predicate.Value == "sysx:mappingValues" {
			t.Fatal("mappingValues was retained as an RDF triple")
		}
	}
	plain := apiJSONWithoutSysx(t, document)
	iriForm, err := export.ReadAPIJSON(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !iriForm.Has(rdf.Triple{
		Subject: subject, Predicate: rdf.SysMLTerm("type"),
		Object: real,
	}) {
		t.Fatal("plain API JSON did not retain the reference as an IRI")
	}
	compact, err := export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	compactRestored, err := export.ReadAPIJSON(compact)
	if err != nil {
		t.Fatalf("ReadAPIJSON compact form: %v", err)
	}
	if !compactRestored.Has(rdf.Triple{
		Subject: subject, Predicate: rdf.SysMLTerm("type"), Object: rdf.String("Real"),
	}) {
		t.Fatal("compact mappingValues did not restore the source literal")
	}
	compactPlain, err := export.ReadAPIJSON(apiJSONWithoutSysx(t, compact))
	if err != nil {
		t.Fatalf("ReadAPIJSON compact form without sysx keys: %v", err)
	}
	if !compactPlain.Has(rdf.Triple{
		Subject: subject, Predicate: rdf.SysMLTerm("type"), Object: real,
	}) {
		t.Fatal("compact API JSON without sysx metadata did not retain the reference as an IRI")
	}
}

func TestAPIJSONMappingValuesRestoreNumericRange(t *testing.T) {
	document := []byte(`[{
		"@type": "LiteralRational",
		"@id": "literal-rational",
		"value": 0.1,
		"sysx:mappingValues": {"value": ["1/10"]}
	}]`)
	graph, err := export.ReadAPIJSON(document)
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	if !graph.Has(rdf.Triple{
		Subject:   rdf.IRI(rdf.Expression + "literal-rational"),
		Predicate: rdf.SysMLTerm("value"),
		Object:    rdf.TypedLiteral("1/10", rdf.XSD+"decimal"),
	}) {
		t.Fatalf("mappingValues did not restore the numeric property's exact datatype and lexical value: %v", graph.Triples())
	}
}

func apiJSONWithoutSysx(t *testing.T, document []byte) []byte {
	t.Helper()
	var scrub func(any) any
	scrub = func(value any) any {
		switch value := value.(type) {
		case map[string]any:
			result := make(map[string]any, len(value))
			for key, member := range value {
				if strings.HasPrefix(key, "sysx:") {
					continue
				}
				result[key] = scrub(member)
			}
			return result
		case []any:
			result := make([]any, len(value))
			for i, member := range value {
				result[i] = scrub(member)
			}
			return result
		default:
			return value
		}
	}
	var decoded any
	if err := json.Unmarshal(document, &decoded); err != nil {
		t.Fatal(err)
	}
	plain, err := json.Marshal(scrub(decoded))
	if err != nil {
		t.Fatal(err)
	}
	return plain
}

func sameTriples(a, b map[rdf.Triple]bool) bool {
	return len(diffTriples(a, b)) == 0 && len(diffTriples(b, a)) == 0
}

func diffTriples(a, b map[rdf.Triple]bool) []rdf.Triple {
	var out []rdf.Triple
	for triple := range a {
		if !b[triple] {
			out = append(out, triple)
		}
	}
	return out
}

// apiJSONDocument converts the interop model to the element form and decodes it
// into objects for shape assertions.
func apiJSONDocument(t *testing.T) []map[string]json.RawMessage {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "tools", "referee", "flexo", "testdata", "model.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := convert.SysMLToRDF("model.sysml", src)
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the document is not a JSON array: %v\n%s", err, document)
	}
	return elements
}

func apiJSONElement(t *testing.T, elements []map[string]json.RawMessage, id string) map[string]json.RawMessage {
	t.Helper()
	for _, element := range elements {
		var elementID string
		if err := json.Unmarshal(element["@id"], &elementID); err == nil && elementID == id {
			return element
		}
	}
	t.Fatalf("no element object with @id %q", id)
	return nil
}

// The interop model's element objects carry the shape the API serves: keywords,
// scalar properties, {"@id": …} references and arrays for collections.
func TestAPIJSONShapeOnTheInteropModel(t *testing.T) {
	elements := apiJSONDocument(t)
	pkg := apiJSONElement(t, elements, "Interop")
	var typ string
	if err := json.Unmarshal(pkg["@type"], &typ); err != nil || typ != "Package" {
		t.Errorf(`Interop "@type" = %s, want "Package"`, pkg["@type"])
	}
	for _, key := range []string{"elementId", "qualifiedName", "declaredName"} {
		var value string
		if err := json.Unmarshal(pkg[key], &value); err != nil || value == "" {
			t.Errorf("Interop %s = %s, want a string", key, pkg[key])
		}
	}
	var members []map[string]string
	if err := json.Unmarshal(pkg["ownedMember"], &members); err != nil || len(members) == 0 {
		t.Fatalf("Interop ownedMember = %s, want an array of references", pkg["ownedMember"])
	}
	for i, member := range members {
		if len(member) != 1 || member["@id"] == "" {
			t.Errorf("ownedMember[%d] = %v, want a {\"@id\": …} reference", i, member)
		}
	}
	component := apiJSONElement(t, elements, "Interop__Component")
	var owner map[string]string
	if err := json.Unmarshal(component["owner"], &owner); err != nil || owner["@id"] != "Interop" {
		t.Errorf("Interop__Component owner = %s, want a single reference object, not an array", component["owner"])
	}
	var index json.Number
	if err := json.Unmarshal(pkg["sysx:memberIndex"], &index); err != nil {
		t.Errorf("Interop sysx:memberIndex = %s, want a JSON number", pkg["sysx:memberIndex"])
	}
	var text string
	if err := json.Unmarshal(pkg["sysx:sourceText"], &text); err != nil || !strings.Contains(text, "package Interop {") {
		t.Errorf("Interop sysx:sourceText = %s, want the notation string", pkg["sysx:sourceText"])
	}
	node := apiJSONElement(t, elements, "Interop__Component__mass_pvalue")
	if err := json.Unmarshal(node["@type"], &typ); err != nil || typ != "LiteralRational" {
		t.Errorf(`mass expression "@type" = %s, want "LiteralRational"`, node["@type"])
	}
	var value json.Number
	if err := json.Unmarshal(node["value"], &value); err != nil {
		t.Errorf("mass expression value = %s, want a JSON number", node["value"])
	}
	apiJSONElement(t, elements, "Interop__Component__mass_pvalue_om")
	for _, element := range elements {
		for key := range element {
			if strings.HasPrefix(key, "json:") {
				t.Errorf("element %s carries a json: key", element["@id"])
			}
		}
	}
}

// The reader reports malformed documents and objects that are not elements.
func TestReadAPIJSONRejectsNonElements(t *testing.T) {
	for name, data := range map[string]string{
		"malformed":                        `{`,
		"top-level string":                 `"hello"`,
		"missing @type":                    `[{"@id": "X"}]`,
		"missing @id":                      `[{"@type": "Package"}]`,
		"empty @id":                        `[{"@type": "Package", "@id": ""}]`,
		"duplicate @id":                    `[{"@type": "Package", "@id": "X"}, {"@type": "PartUsage", "@id": "X"}]`,
		"repeated @id key":                 `[{"@type": "Package", "@id": "A", "@id": "B"}]`,
		"repeated @type key":               `[{"@type": "Package", "@type": "PartUsage", "@id": "X"}]`,
		"repeated member key":              `[{"@type": "Package", "@id": "X", "name": "a", "name": "b"}]`,
		"repeated array member":            `[{"@type": "Package", "@id": "X", "ownedMember": [{"@id": "A"}, {"@id": "A"}]}]`,
		"repeated key in a reference":      `[{"@type": "Package", "@id": "X", "ownedMember": {"@id": "A", "@id": "B"}}]`,
		"repeated key in an array element": `[{"@type": "Package", "@id": "X", "ownedMember": [{"@id": "A", "@id": "B"}]}]`,
		"unknown @key":                     `[{"@type": "Package", "@id": "X", "@foo": 1}]`,
		"object without @id":               `[{"@type": "Package", "@id": "X", "ownedMember": {"@type": "Y"}}]`,
		"object with unknown key":          `[{"@type": "Package", "@id": "X", "ownedMember": {"@foo": "x"}}]`,
		"object with @id and @ref":         `[{"@type": "Package", "@id": "X", "ownedMember": {"@id": "A", "@ref": "b"}}]`,
		"nested array":                     `[{"@type": "Package", "@id": "X", "ownedMember": [[{"@id": "Y"}]]}]`,
		"null member":                      `[{"@type": "Package", "@id": "X", "ownedMember": [null]}]`,
		"prefixed key":                     `[{"@type": "Package", "@id": "X", "sysml:name": "n"}]`,
		"empty sysx: @type":                `{"@type": "sysx:", "@id": "X"}`,
		"empty sysx: key":                  `{"@type": "Package", "@id": "X", "sysx:": true}`,
		"empty scope qualifier":            `{"@type": "Package", "@id": ":X"}`,
		"equivalent ids":                   `[{"@type": "Package", "@id": "X"}, {"@type": "Package", "@id": ":X"}]`,
		"trailing JSON":                    `[{"@type": "Package", "@id": "X"}] 42`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := export.ReadAPIJSON([]byte(data)); err == nil {
				t.Errorf("ReadAPIJSON(%s) succeeded, want an error", data)
			}
		})
	}
}

// Each JSON value form lands as the triple the mapping states it as.
func TestReadAPIJSONValueForms(t *testing.T) {
	document := `[{
		"@type": "Package", "@id": "P",
		"integer": 42, "decimal": 0.0, "double": 1.5e3, "tenth": 1e-1, "flag": true,
		"absent": null, "empty": [], "declaredName": "P",
		"specializes": "A < B", "ownedMember": [{"@id": "Q"}, {"@id": "R"}],
		"sysx:sourceText": "package P {}",
		"sysx:indexes": [0, 1]
	}]`
	graph, err := export.ReadAPIJSON([]byte(document))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	subject := rdf.ElementIRIForID("P")
	for _, want := range []rdf.Triple{
		{Subject: subject, Predicate: rdf.SysMLTerm("integer"), Object: rdf.TypedLiteral("42", rdf.XSD+"integer")},
		{Subject: subject, Predicate: rdf.SysMLTerm("decimal"), Object: rdf.TypedLiteral("0.0", rdf.XSD+"decimal")},
		{Subject: subject, Predicate: rdf.SysMLTerm("double"), Object: rdf.TypedLiteral("1.5e3", rdf.XSD+"double")},
		{Subject: subject, Predicate: rdf.SysMLTerm("tenth"), Object: rdf.TypedLiteral("0.1", rdf.XSD+"decimal")},
		{Subject: subject, Predicate: rdf.SysMLTerm("flag"), Object: rdf.Bool(true)},
		{Subject: subject, Predicate: rdf.SysMLTerm("declaredName"), Object: rdf.String("P")},
		{Subject: subject, Predicate: rdf.SysMLTerm("specializes"), Object: rdf.TypedLiteral("A < B", rdf.OpenSysML+"Expression")},
		{Subject: subject, Predicate: rdf.SysMLTerm("ownedMember"), Object: rdf.ElementIRIForID("Q")},
		{Subject: subject, Predicate: rdf.SysMLTerm("ownedMember"), Object: rdf.ElementIRIForID("R")},
		{Subject: subject, Predicate: rdf.OpenSysMLTerm("sourceText"), Object: rdf.String("package P {}")},
		{Subject: subject, Predicate: rdf.OpenSysMLTerm("indexes"), Object: rdf.TypedLiteral("0", rdf.XSD+"integer")},
		{Subject: subject, Predicate: rdf.OpenSysMLTerm("indexes"), Object: rdf.TypedLiteral("1", rdf.XSD+"integer")},
	} {
		if !graph.Has(want) {
			t.Errorf("missing triple %s %s %s", want.Subject, want.Predicate, want.Object)
		}
	}
	for _, absent := range []string{"absent", "empty"} {
		if graph.HasProperty(subject, rdf.SysML+absent) {
			t.Errorf("%s was dropped in the document but stated in the graph", absent)
		}
	}
	if graph.HasProperty(subject, rdf.AnnotationJSON+"indexes") {
		t.Error("a sysx: array was annotated; only a sysml: collection carries the annotation")
	}
	annotation, ok := graph.Object(subject, rdf.AnnotationJSON+"ownedMember")
	if !ok {
		t.Fatal("a sysml: array did not state its json: annotation")
	}
	want, err := rdf.CollectionJSON(subject, graph.Objects(subject, rdf.SysML+"ownedMember"))
	if err != nil {
		t.Fatal(err)
	}
	if annotation.Value != want {
		t.Errorf("json:ownedMember = %s, want %s", annotation.Value, want)
	}
}

// A scoped @id writes a scoped element IRI, which does not collide with the
// bare id it spells.
func TestReadAPIJSONScopedAndSingle(t *testing.T) {
	graph, err := export.ReadAPIJSON([]byte(`[{"@type": "Package", "@id": "org.proj:X"}, {"@type": "PartUsage", "@id": "X"}]`))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	for _, subject := range []rdf.Term{rdf.ScopedElementIRIForID("org.proj", "X"), rdf.ElementIRIForID("X")} {
		if len(graph.Predicates(subject)) == 0 {
			t.Errorf("%s is not a subject:\n%s", subject.Value, rdf.WriteTurtle(graph))
		}
	}
	if !graph.Has(rdf.Triple{Subject: rdf.ScopedElementIRIForID("org.proj", "X"), Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("Package")}) {
		t.Errorf("the scoped element is not a Package:\n%s", rdf.WriteTurtle(graph))
	}
}

// An id in the expression grammar names an expr: node when its metaclass is
// one the mapping mints under a declaration or its parent is an expr: node; a
// membership follows the node it owns, and every other shape is an element.
func TestReadAPIJSONExpressionClassification(t *testing.T) {
	document := `[
		{"@type": "OwningMembership", "@id": "X_pvalue_pa0_om", "memberElement": {"@id": "X_pvalue_pa0"}},
		{"@type": "LiteralInteger", "@id": "X_pvalue_pa0", "value": 3},
		{"@type": "AttributeUsage", "@id": "X", "value": {"@id": "X_pvalue"}},
		{"@type": "LiteralRational", "@id": "X_pvalue", "value": 0.0},
		{"@type": "OwningMembership", "@id": "X_pvalue_om", "memberElement": {"@id": "X_pvalue"}},
		{"@type": "AttributeUsage", "@id": "X_pvalue_pm0"},
		{"@type": "FeatureValue", "@id": "X_pvalue_pm0_om"},
		{"@type": "LiteralRational", "@id": "Y_pvalue", "qualifiedName": "Y::pvalue"},
		{"@type": "PartUsage", "@id": "A_pfoo"},
		{"@type": "PartUsage", "@id": "X_pbar"},
		{"@type": "OwningMembership", "@id": "X_pbar_om", "memberElement": {"@id": "X_pbar"}},
		{"@type": "ConnectionUsage", "@id": "C", "qualifiedName": "C"},
		{"@type": "ReferenceSubsetting", "@id": "C_pend0_prs"},
		{"@type": "ReferenceUsage", "@id": "C_pend0"},
		{"@type": "AttributeUsage", "@id": "W", "value": {"@id": "X_p2"}},
		{"@type": "LiteralRational", "@id": "X_p2", "qualifiedName": null, "value": 1.0}
	]`
	graph, err := export.ReadAPIJSON([]byte(document))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	for _, expr := range []string{"X_pvalue", "X_pvalue_om", "X_pvalue_pa0", "X_pvalue_pa0_om", "X_pvalue_pm0", "X_pvalue_pm0_om", "C_pend0", "C_pend0_prs", "X_p2"} {
		subject := rdf.IRI(rdf.Expression + expr)
		if len(graph.Predicates(subject)) == 0 {
			t.Errorf("%q is not an expression-namespace subject:\n%s", expr, rdf.WriteTurtle(graph))
		}
	}
	for _, element := range []string{"Y_pvalue", "A_pfoo", "X_pbar", "X_pbar_om"} {
		subject := rdf.ElementIRIForID(element)
		if len(graph.Predicates(subject)) == 0 {
			t.Errorf("%q is not an element-namespace subject:\n%s", element, rdf.WriteTurtle(graph))
		}
	}
	owner := rdf.ElementIRIForID("X")
	value, ok := graph.Object(owner, rdf.SysML+"value")
	if !ok || value.Value != rdf.Expression+"X_pvalue" {
		t.Errorf("X's reference to X_pvalue = %v, want the expression IRI", value)
	}
	w := rdf.ElementIRIForID("W")
	value, ok = graph.Object(w, rdf.SysML+"value")
	if !ok || value.Value != rdf.Expression+"X_p2" {
		t.Errorf("W's reference to X_p2 = %v, want the expression IRI", value)
	}
}

// A real's JSON spelling is its XSD lexical, respelled only where JSON cannot
// carry it; the exponent letter is kept so the Turtle round trip does not move.
func TestWriteAPIJSONRealLexicals(t *testing.T) {
	subject := rdf.ElementIRIForID("X")
	for _, test := range []struct {
		lexical  string
		datatype string
		want     string
	}{
		{"1.5e3", rdf.XSD + "double", "1.5e3"},
		{"8.0E-9", rdf.XSD + "double", "8.0E-9"},
		{"2.E3", rdf.XSD + "double", "2.0E3"},
		{".5", rdf.XSD + "decimal", "0.5"},
		{"-.25", rdf.XSD + "decimal", "-0.25"},
		{"+2.5", rdf.XSD + "decimal", "2.5"},
		{"5.", rdf.XSD + "decimal", "5.0"},
		{"0.0", rdf.XSD + "decimal", "0.0"},
	} {
		lexical, want := test.lexical, test.want
		g := rdf.NewGraph()
		g.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Package"))
		g.Add(subject, rdf.SysMLTerm("value"), rdf.TypedLiteral(lexical, test.datatype))
		out, err := export.WriteAPIJSON(g)
		if err != nil {
			t.Fatalf("WriteAPIJSON(%q): %v", lexical, err)
		}
		if !strings.Contains(string(out), `"value": `+want) {
			t.Errorf("WriteAPIJSON(%q) wrote %s, want the number %s", lexical, out, want)
		}
	}
}

// The writer refuses what the element form cannot say rather than dropping it.
func TestWriteAPIJSONRefuses(t *testing.T) {
	element := rdf.ElementIRIForID("X")
	typed := func(objects ...rdf.Term) *rdf.Graph {
		g := rdf.NewGraph()
		for _, object := range objects {
			g.Add(element, rdf.IRI(rdf.RDFType), object)
		}
		return g
	}
	for name, build := range map[string]func() *rdf.Graph{
		"two rdf:types": func() *rdf.Graph {
			return typed(rdf.SysMLTerm("Package"), rdf.SysMLTerm("PartUsage"))
		},
		"no rdf:type": func() *rdf.Graph {
			g := typed()
			g.Add(element, rdf.SysMLTerm("declaredName"), rdf.String("x"))
			return g
		},
		"foreign predicate": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.IRI("http://example.com/prop"), rdf.String("x"))
			return g
		},
		"language tag": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("declaredName"), rdf.Term{Kind: rdf.TermLiteral, Value: "x", Lang: "en"})
			return g
		},
		"foreign subject": func() *rdf.Graph {
			g := rdf.NewGraph()
			other := rdf.IRI("http://example.com/subject")
			g.Add(other, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Package"))
			return g
		},
		"collection without annotation": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("declaredName"), rdf.String("a"))
			g.Add(element, rdf.SysMLTerm("declaredName"), rdf.String("b"))
			return g
		},
		"xsd:float": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("1.5", rdf.XSD+"float"))
			return g
		},
		"xsd:int": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("7", rdf.XSD+"int"))
			return g
		},
		"xsd:boolean spelled 1": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("1", rdf.XSD+"boolean"))
			return g
		},
		"xsd:double without exponent": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("1.5", rdf.XSD+"double"))
			return g
		},
		"expression text on a sysx: key": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.OpenSysMLTerm("sourceText"), rdf.TypedLiteral("a + b", rdf.OpenSysML+"Expression"))
			return g
		},
		"collection in a non-round-trippable datatype": func() *rdf.Graph {
			g := typed(rdf.SysMLTerm("Package"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("1.5", rdf.XSD+"float"))
			g.Add(element, rdf.SysMLTerm("value"), rdf.TypedLiteral("2.5", rdf.XSD+"float"))
			g.Add(element, rdf.IRI(rdf.AnnotationJSON+"value"), rdf.String("[1.5,2.5]"))
			return g
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := export.WriteAPIJSON(build())
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Errorf("WriteAPIJSON = %v, want an UnsupportedError", err)
			}
		})
	}
}

// A property the metamodel declares unbounded is an array even with one
// member — the shape the API serves and other readers require — while a
// single-valued property stays an object.
func TestWriteAPIJSONManyIsAlwaysAnArray(t *testing.T) {
	text := []byte("package P { part def V; }")
	graph, err := convert.SysMLToRDF("p.sysml", text)
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the document is not a JSON array: %v\n%s", err, document)
	}
	pkg := apiJSONElement(t, elements, "P")
	for _, key := range []string{"ownedRelationship", "ownedMember", "ownedMembership"} {
		var members []json.RawMessage
		if err := json.Unmarshal(pkg[key], &members); err != nil || len(members) != 1 {
			t.Errorf("P %s = %s, want a one-member array", key, pkg[key])
		}
	}
	var usage map[string]json.RawMessage
	for _, element := range elements {
		var typ string
		if err := json.Unmarshal(element["@type"], &typ); err == nil && typ == "PartDefinition" {
			usage = element
		}
	}
	if usage == nil {
		t.Fatal("no PartDefinition element")
	}
	var owner map[string]string
	if err := json.Unmarshal(usage["owningRelationship"], &owner); err != nil || owner["@id"] == "" {
		t.Errorf("owningRelationship = %s, want a single reference object", usage["owningRelationship"])
	}
}

// A one-member array states the triple without a json: annotation — the same
// graph the Turtle form reads, where the annotation marks two members or more.
func TestReadAPIJSONOneMemberArray(t *testing.T) {
	graph, err := export.ReadAPIJSON([]byte(`[{"@type": "Package", "@id": "P", "ownedMember": [{"@id": "Q"}]}]`))
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v", err)
	}
	subject := rdf.ElementIRIForID("P")
	if !graph.Has(rdf.Triple{Subject: subject, Predicate: rdf.SysMLTerm("ownedMember"), Object: rdf.ElementIRIForID("Q")}) {
		t.Error("the member triple is missing")
	}
	if graph.HasProperty(subject, rdf.AnnotationJSON+"ownedMember") {
		t.Error("a one-member array stated a json: annotation")
	}
}

// A model whose collections each hold one member makes the same graph through
// the element form as through Turtle — neither direction invents or drops a
// statement.
func TestAPIJSONRoundTripSingleMemberCollections(t *testing.T) {
	text := []byte("package P {\n\tpart def V;\n\tpart car : V;\n}")
	graph, err := convert.SysMLToRDF("p.sysml", text)
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	reread, err := export.ReadAPIJSON(document)
	if err != nil {
		t.Fatalf("ReadAPIJSON: %v\n%s", err, document)
	}
	if !sameTriples(tripleSet(graph), tripleSet(reread)) {
		t.Fatalf("the graph changed through the element form\nmissing: %v\nadded: %v",
			diffTriples(tripleSet(graph), tripleSet(reread)), diffTriples(tripleSet(reread), tripleSet(graph)))
	}
}

// A chain with an unresolved member is omitted and reports typed reasons.
func TestAPIJSONAnnotationMembers(t *testing.T) {
	graph, err := convert.SysMLToRDF("p.sysml", []byte("package P { part def V; }"))
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	subject := rdf.ElementIRI("P::V")
	link := rdf.ElementIRI("P::X")
	graph.Add(subject, rdf.SysMLTerm("chainingFeature"), link)
	graph.Add(subject, rdf.SysMLTerm("chainingFeature"), rdf.String("b"))
	graph.Add(subject, rdf.SysMLTerm("chainingFeature"), rdf.String("a"))
	graph.Add(subject, rdf.AnnotationJSONTerm("chainingFeature"), rdf.String(`[{"@id":"P__X"},"b","a"]`))
	chain := rdf.ElementIRI("P::C")
	graph.Add(chain, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Feature"))
	graph.Add(chain, rdf.SysMLTerm("chainingFeature"), link)
	graph.Add(chain, rdf.SysMLTerm("chainingFeature"), rdf.String("b"))
	graph.Add(chain, rdf.SysMLTerm("chainingFeature"), rdf.String("a"))
	graph.Add(chain, rdf.AnnotationJSONTerm("chainingFeature"), rdf.String(`[{"@id":"P__X"},"b","a"]`))
	graph.Add(link, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Feature"))
	var omissions []export.APIJSONOmission
	document, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{
		Omitted: func(omission export.APIJSONOmission) { omissions = append(omissions, omission) },
	})
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the document is not a JSON array: %v\n%s", err, document)
	}
	for _, id := range []string{"P__V", "P__C"} {
		element := apiJSONElement(t, elements, id)
		if _, present := element["chainingFeature"]; present {
			t.Errorf("%s keeps unresolved chainingFeature: %s", id, element["chainingFeature"])
		}
		var mappingValues map[string]json.RawMessage
		if err := json.Unmarshal(element["sysx:mappingValues"], &mappingValues); err != nil {
			t.Fatalf("%s mappingValues: %v", id, err)
		}
		var links []any
		if err := json.Unmarshal(mappingValues["chainingFeature"], &links); err != nil {
			t.Fatalf("%s mappingValues.chainingFeature: %v", id, err)
		}
		first, isReference := any(nil), false
		if len(links) > 0 {
			first, isReference = links[0].(map[string]any)
		}
		if len(links) != 3 || !isReference || first.(map[string]any)["@id"] != "P__X" ||
			links[1] != "b" || links[2] != "a" {
			t.Errorf("%s mappingValues.chainingFeature = %v, want [P__X, b, a]", id, links)
		}
	}
	if strings.Contains(string(document), `"@ref"`) {
		t.Errorf("API JSON contains @ref:\n%s", document)
	}
	reasons := map[string]bool{}
	for _, omission := range omissions {
		if omission.Reason.Category == metamodel.ReasonUnresolvedReference {
			reasons[omission.Reason.Detail] = true
		}
	}
	if !reasons["a"] || !reasons["b"] {
		t.Errorf("omissions = %v, want unresolved reasons for a and b", omissions)
	}

	other := rdf.ElementIRI("P::W")
	graph.Add(other, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("PartUsage"))
	graph.Add(other, rdf.SysMLTerm("aliasIds"), rdf.String("a"))
	graph.Add(other, rdf.SysMLTerm("aliasIds"), rdf.String("b"))
	graph.Add(other, rdf.AnnotationJSONTerm("aliasIds"), rdf.String(`["a","a","b"]`))
	_, err = export.WriteAPIJSON(graph)
	if err == nil || !strings.Contains(err.Error(), "again at") {
		t.Errorf("WriteAPIJSON with a repeated member = %v, want a refusal naming it", err)
	}
}

// A member-attached `then done;` is one SuccessionAsUsage whose target end's
// ReferenceSubsetting reaches the library's Actions::Action::done element —
// the shape `succession first x then done;` writes — not a Membership the end
// would reference, which ReferenceSubsetting cannot target.
func TestAPIJSONThenDoneTargetsLibraryDone(t *testing.T) {
	graph, err := convert.SysMLToRDF("p.sysml", []byte(
		"package P { action def Step; action def A { first start; then action a : Step; then done; } }"))
	if err != nil {
		t.Fatalf("SysMLToRDF: %v", err)
	}
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("the document is not a JSON array: %v\n%s", err, document)
	}
	const doneID = "0cdc3cd3-b06c-5c32-beda-0cf4ba164a64"
	idRef := func(raw json.RawMessage) string {
		var ref map[string]string
		if err := json.Unmarshal(raw, &ref); err != nil {
			return ""
		}
		return ref["@id"]
	}
	successions, doneMemberships, doneEnds := 0, 0, 0
	for _, element := range elements {
		var typ string
		if err := json.Unmarshal(element["@type"], &typ); err != nil {
			continue
		}
		switch typ {
		case "SuccessionAsUsage":
			successions++
		case "Membership":
			if idRef(element["memberElement"]) == doneID {
				doneMemberships++
			}
		case "ReferenceSubsetting":
			if idRef(element["referencedFeature"]) == doneID {
				doneEnds++
			}
		}
	}
	if successions != 2 {
		t.Errorf("want the two `then` successions alone, found %d SuccessionAsUsage", successions)
	}
	if doneMemberships != 0 {
		t.Errorf("`then done;` should state no Membership of the library's done, found %d", doneMemberships)
	}
	if doneEnds != 1 {
		t.Errorf("the `then done` end should subset the library's done feature exactly once, found %d", doneEnds)
	}
}
