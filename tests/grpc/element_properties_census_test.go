package grpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// censusWitness is one element of one model whose metaclass carries a
// metamodel property under the defining class a rule or omission names.
type censusWitness struct {
	path, modelHash, id, metaclass string
}

// censusModels are the models the census reads: the examples and the gRPC conformance
// fixtures, which carry the metaclasses the handbook's examples do not reach.
func censusModels(t *testing.T) []string {
	t.Helper()
	paths := agreementExamples(t)
	fixtures, err := filepath.Glob(filepath.Join("testdata", "conformance", "*.*ml"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(fixtures)
	return append(paths, fixtures...)
}

// TestElementPropertiesServeEveryRuleAndExplainEveryOmission walks all of metamodel.Rules()
// and Omitted(): every served rule is answered by some carrier, every omission says why.
func TestElementPropertiesServeEveryRuleAndExplainEveryOmission(t *testing.T) {
	rules, omissions := metamodel.Rules(), metamodel.Omitted()
	if len(rules) == 0 || len(omissions) == 0 {
		t.Fatalf("rules = %d, omissions = %d; want both populated", len(rules), len(omissions))
	}
	srv, err := grpc.NewService(4, "test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// wanted maps "Class::property" to the open rules and omissions; a key is
	// settled by the first element that answers it as expected.
	type want struct {
		class, property, reason string
		served                  bool
	}
	open := map[string]want{}
	for _, rule := range rules {
		key := rule.DefiningClass + "::" + rule.Property
		if _, ok := ontology.PropertyOf(rule.DefiningClass, rule.Property); !ok {
			t.Errorf("served rule %s names no property of the metamodel", key)
			continue
		}
		open[key] = want{class: rule.DefiningClass, property: rule.Property, served: true}
	}
	for _, omission := range omissions {
		key := omission.DefiningClass + "::" + omission.Property
		if _, ok := ontology.PropertyOf(omission.DefiningClass, omission.Property); !ok {
			t.Errorf("omission %s names no property of the metamodel", key)
			continue
		}
		if _, served := open[key]; served {
			t.Errorf("%s is both served and omitted", key)
			continue
		}
		open[key] = want{class: omission.DefiningClass, property: omission.Property, reason: omission.Reason}
	}
	// carried caches, per metaclass, the open keys its elements witness: the
	// metaclass specializes the defining class and declares the property.
	carried := map[string][]string{}
	keysOf := func(metaclass string) []string {
		if keys, ok := carried[metaclass]; ok {
			return keys
		}
		var keys []string
		for key, w := range open {
			if !ontology.IsAncestorOrSelf(metaclass, w.class) {
				continue
			}
			if _, ok := ontology.PropertyOf(metaclass, w.property); ok {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		carried[metaclass] = keys
		return keys
	}
	// seen records, per open key, how its witnesses answered when none settled it.
	seen := map[string][]string{}
	settled := map[string]censusWitness{}
	tried := map[string]int{}
	const triesPerKey = 40

	for _, path := range censusModels(t) {
		if len(settled) == len(open) {
			break
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		exported, err := convert.Convert(filepath.Base(path), data, convert.FormatSysML, convert.FormatAPIJSON)
		if err != nil {
			continue
		}
		var objects []map[string]any
		if err := json.Unmarshal(exported, &objects); err != nil {
			t.Fatalf("%s: api-json: %v", path, err)
		}
		var modelHash string
		for _, object := range objects {
			metaclass, _ := object["@type"].(string)
			id, _ := object["@id"].(string)
			var pending []string
			for _, key := range keysOf(metaclass) {
				if _, done := settled[key]; !done && tried[key] < triesPerKey {
					pending = append(pending, key)
				}
			}
			if len(pending) == 0 {
				continue
			}
			if modelHash == "" {
				absolute, err := filepath.Abs(path)
				if err != nil {
					t.Fatal(err)
				}
				parsed, err := srv.ParseFile(ctx, &pb.ParseFileRequest{Source: &pb.ParseFileRequest_FilePath{FilePath: absolute}})
				if err != nil {
					t.Fatalf("%s: ParseFile: %v", path, err)
				}
				modelHash = parsed.ModelHash
			}
			names := make([]string, 0, len(pending))
			for _, key := range pending {
				names = append(names, open[key].property)
			}
			resp, err := srv.GetElementProperties(ctx, &pb.GetElementPropertiesRequest{ModelHash: modelHash, ElementId: id, Properties: names})
			if err != nil {
				t.Fatalf("%s: GetElementProperties(%s): %v", path, id, err)
			}
			if resp.Error != "" {
				t.Fatalf("%s: GetElementProperties(%s): %s", path, id, resp.Error)
			}
			for i, key := range pending {
				tried[key]++
				answer := resp.Properties[i]
				w := open[key]
				var ok bool
				if w.served {
					ok = answer.Unsupported == ""
				} else {
					ok = strings.Contains(answer.Unsupported, w.reason)
				}
				if ok {
					settled[key] = censusWitness{path: path, modelHash: modelHash, id: id, metaclass: metaclass}
					continue
				}
				seen[key] = append(seen[key], fmt.Sprintf("%s %s (%s): %q", filepath.Base(path), id, metaclass, answer.Unsupported))
			}
		}
	}

	var unsettled []string
	for key := range open {
		if _, done := settled[key]; !done {
			unsettled = append(unsettled, key)
		}
	}
	sort.Strings(unsettled)
	for _, key := range unsettled {
		w := open[key]
		kind := "served"
		if !w.served {
			kind = "omitted (" + w.reason + ")"
		}
		if len(seen[key]) == 0 {
			if reason, absent := elementFormAbsentMetaclasses[w.class]; absent {
				t.Logf("%s (%s): no model element carries it: %s", key, kind, reason)
				continue
			}
			t.Errorf("%s (%s): no model element carries it", key, kind)
			continue
		}
		t.Errorf("%s (%s): no model element is answered as expected; seen:\n  %s", key, kind, strings.Join(seen[key], "\n  "))
	}
	for class, reason := range elementFormAbsentMetaclasses {
		for key, witness := range settled {
			if open[key].class == class {
				t.Errorf("%s is carried by %s (%s) in %s; the element form now states %s elements, so drop it from elementFormAbsentMetaclasses (%s)",
					key, witness.id, witness.metaclass, witness.path, class, reason)
			}
		}
	}
	t.Logf("%d served rules and %d omissions witnessed over %d models", len(rules), len(omissions), len(censusModels(t)))
}

// elementFormAbsentMetaclasses are the relationship metaclasses the API element form states
// as text of their owner rather than as elements; an entry that becomes carried fails the census.
var elementFormAbsentMetaclasses = map[string]string{
	"Unioning":        "a type's `unions` is a text property of the type in the element form",
	"Intersecting":    "a type's `intersects` is a text property of the type in the element form",
	"Differencing":    "a type's `differences` is a text property of the type in the element form",
	"CrossSubsetting": "an end feature's `crosses` is a text property of the feature in the element form",
	"TypeFeaturing":   "a feature's `featured by` is a text property of the feature in the element form",
}
