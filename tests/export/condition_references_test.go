package export_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// conditionReferenceFixtures are the constraint and invariant bodies whose closing
// condition is a bare feature reference, with the notation the graph alone must write.
var conditionReferenceFixtures = []struct {
	name  string
	wants []string
}{
	{"condition_references.sysml", []string{
		"require constraint {\n            structuralIntegrityMaintained\n        }",
		"require constraint {\n            not x\n        }",
		"assume constraint {\n            a\n        }",
		"require constraint <'HLR-R059'> {\n            x\n        }",
		"require constraint <'HLR-R060'> named {\n            a and b\n        }",
		"constraint {\n            a and b\n        }",
		"constraint c1 {\n            x\n        }",
		"assert constraint {\n            x\n        }",
		"assert constraint {\n            not x\n        }",
		"assert not constraint {\n            x\n        }",
		"assert constraint {\n            ok\n        }",
		"assert constraint {\n            n > 0;\n            ok\n        }",
		"constraint inv1 {\n            not ok\n        }",
	}},
	{"invariant_references.kerml", []string{
		"inv {\n            ready\n        }",
		"inv holds {\n            not sealed\n        }",
		"inv {\n            sealed and ready\n        }",
		"inv bounded {\n            level > 0;\n            ready\n        }",
	}},
}

// A closing condition comes back from the graph's FeatureReferenceExpression alone,
// written bare: with a `;` it would declare a feature, and the reference could not be checked.
func TestConditionReferencesComeBackFromTheGraphAlone(t *testing.T) {
	for _, fixture := range conditionReferenceFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			_, turtle := readAndConvertFixture(t, fixture.name)
			fromGraph := string(structuralRoundTrip(t, fixture.name, turtle))
			for _, want := range fixture.wants {
				if !strings.Contains(fromGraph, want) {
					t.Errorf("the notation rebuilt from the graph lacks %q:\n%s", want, fromGraph)
				}
			}
			for _, unwanted := range []string{"structuralIntegrityMaintained;", "ok;", "x;", "a;", "ready;"} {
				if strings.Contains(fromGraph, "\n            "+unwanted) {
					t.Errorf("a bare condition is written as a declaration (%q):\n%s", unwanted, fromGraph)
				}
			}
		})
	}
}

// The notation the mapping alone writes states the model the fixture does: the
// analyser reports the same diagnostics for both.
func TestConditionReferencesRevalidateFromTheGraphAlone(t *testing.T) {
	for _, fixture := range conditionReferenceFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			src, turtle := readAndConvertFixture(t, fixture.name)
			fromGraph, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation from the mapping alone: %v", err)
			}
			written, rebuilt := diagnosticMessages(fixture.name, src), diagnosticMessages(fixture.name, fromGraph)
			if len(written) > 0 {
				t.Fatalf("the fixture should analyse clean:\n%s", strings.Join(written, "\n"))
			}
			if strings.Join(written, "\n") != strings.Join(rebuilt, "\n") {
				t.Errorf("the rebuilt notation validates differently\n--- written ---\n%s\n--- rebuilt ---\n%s\n--- notation ---\n%s",
					strings.Join(written, "\n"), strings.Join(rebuilt, "\n"), fromGraph)
			}
		})
	}
}

// readAndConvertFixture returns a convert fixture's notation and its Turtle.
func readAndConvertFixture(t *testing.T, name string) (src, turtle []byte) {
	t.Helper()
	path := filepath.Join("testdata", "convert", name)
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err = convert.Convert(path, src, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	return src, turtle
}

// diagnosticMessages analyses notation as one document of a workspace and
// returns its diagnostics as sorted severity-and-message lines.
func diagnosticMessages(name string, notation []byte) []string {
	ws := model.NewWorkspace()
	ws.Open(name, notation, 1)
	var lines []string
	for _, d := range ws.Diagnostics(name) {
		lines = append(lines, fmt.Sprintf("%v: %s", d.Severity, d.Message))
	}
	sort.Strings(lines)
	return lines
}

// A bare `assume c;`, `require q.k;` or `assert c;` is the reference form of its
// constraint usage (RequirementConstraintUsage, AssertConstraintUsage): the
// usage owns a ReferenceSubsetting to the feature it names, or to the chain of
// features, rather than stating an expression.
func TestBareConditionMembersOwnAReferenceSubsetting(t *testing.T) {
	// A member named by reference subsetting takes the referenced feature's
	// name, so the features named live outside the bodies naming them.
	src := "package P {\n    constraint def C;\n    constraint c : C;\n    part def Q {\n        constraint k : C;\n    }\n" +
		"    requirement def R {\n        subject q : Q;\n        assume c;\n        require q.k;\n    }\n" +
		"    constraint def D {\n        assert c;\n    }\n}\n"
	if diagnostics := diagnosticMessages("m.sysml", []byte(src)); len(diagnostics) > 0 {
		t.Fatalf("the model should analyse clean:\n%s", strings.Join(diagnostics, "\n"))
	}
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	if strings.Contains(graph, "sysx:condition") {
		t.Errorf("a bare condition member should state no expression:\n%s", graph)
	}
	for _, want := range []string{
		"sysml:referencedFeature elmt:P__c",
		"sysml:referencedFeature <urn:opensysml:expr:P__R___402_pchain0>",
		"sysml:chainingFeature elmt:P__R__q, elmt:P__Q__k",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	if n := strings.Count(graph, "a sysml:ReferenceSubsetting"); n != 3 {
		t.Errorf("each of the three members should own a ReferenceSubsetting, found %d:\n%s", n, graph)
	}
	back, err := convert.Convert("m.ttl", withoutSourceText(t, turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v\n%s", err, turtle)
	}
	if string(back) != src {
		t.Fatalf("the notation changed\n--- want ---\n%s\n--- got ---\n%s", src, back)
	}

	doc, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, e := range elements {
		id, _ := e["@id"].(string)
		byID[id] = e
	}
	ref := func(v any) string {
		id, _ := v.(map[string]any)["@id"].(string)
		return id
	}
	members := 0
	for _, e := range elements {
		if e["@type"] != "RequirementConstraintMembership" && e["@type"] != "AssertConstraintUsage" {
			continue
		}
		usage := e
		if e["@type"] == "RequirementConstraintMembership" {
			usage = byID[ref(e["ownedConstraint"])]
		}
		members++
		subsetting := byID[ref(usage["ownedReferenceSubsetting"])]
		if subsetting == nil || subsetting["@type"] != "ReferenceSubsetting" {
			t.Errorf("%v: ownedReferenceSubsetting %v, want a ReferenceSubsetting in the output", usage["@id"], usage["ownedReferenceSubsetting"])
			continue
		}
		if byID[ref(subsetting["referencedFeature"])] == nil {
			t.Errorf("%v: referencedFeature %v is not in the output", subsetting["@id"], subsetting["referencedFeature"])
		}
	}
	if members != 3 {
		t.Errorf("want three condition members, found %d", members)
	}
	if _, err := convert.Convert("m.json", doc, convert.FormatAPIJSON, convert.FormatSysML); err != nil {
		t.Errorf("the API JSON did not read back: %v", err)
	}
}
