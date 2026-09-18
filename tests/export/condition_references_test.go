package export_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
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
			fromGraph, err := export.Convert("m.ttl", withoutSourceText(t, turtle), export.FormatTurtle, export.FormatSysML)
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
	turtle, err = export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
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
