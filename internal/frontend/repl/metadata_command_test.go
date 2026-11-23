package repl

import (
	"strings"
	"testing"
)

const metadataCommandModel = `
package Elsewhere {
    metadata P::Tag about P::car { who = "elsewhere"; }
}
package P {
    private import ScalarValues::*;
    metadata def Tag { attribute who : String default = "nobody"; }
    metadata def Deep { attribute inner : Inner; }
    attribute def Inner { attribute a : Integer = 5; }
    part def Base { @Tag { who = "base"; } }
    part def Engine :> Base { @Deep; }
    part def Wheel;
    part def Car {
        part engine : Engine[1] { @Tag; }
        part wheels : Wheel[2];
    }
    part car : Car { @Tag { who = "car"; } }
    part plain : Wheel;
}
`

// metadataLines runs %metadata and answers its output lines.
func metadataLines(t *testing.T, s *Session, target string) []string {
	t.Helper()
	out, _, err := s.RunMeta("%metadata " + target)
	if err != nil {
		t.Fatalf("%%metadata %s: %v", target, err)
	}
	return out
}

func wantLines(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestMetadataListsAnElementsAnnotationsWithTheirValues(t *testing.T) {
	s := NewSession()
	submitModel(t, s, metadataCommandModel)

	got := metadataLines(t, s, "P::car")
	if len(got) != 5 || got[0] != "Element: P::car (partUsage)" ||
		!strings.HasPrefix(got[1], "  @P::Tag (stated with `about` in ") ||
		got[2] != `    who = "elsewhere"` || got[3] != "  @P::Tag" || got[4] != `    who = "car"` {
		t.Errorf("a usage lists the `about` annotation and its own, in textual order:\n%s", strings.Join(got, "\n"))
	}
	wantLines(t, metadataLines(t, s, "P::Base"), []string{
		"Element: P::Base (partDef)",
		"  @P::Tag",
		`    who = "base"`,
	})
	// A value the body leaves to the metadata type is marked as its default,
	// and an object a feature holds is listed beneath it.
	wantLines(t, metadataLines(t, s, "P::Car::engine"), []string{
		"Element: P::Car::engine (partUsage)",
		"  @P::Tag",
		`    who = "nobody" (default)`,
	})
	got = metadataLines(t, s, "P::Engine")
	if len(got) != 4 || got[1] != "  @P::Deep" || !strings.HasPrefix(got[2], "    inner = Instance(ID: ") || got[3] != "      a = 5" {
		t.Errorf("a structured value is expanded:\n%s", strings.Join(got, "\n"))
	}
}

func TestMetadataGroupsAnObjectsAnnotationsByTheElementAnnotated(t *testing.T) {
	s := NewSession()
	submitModel(t, s, metadataCommandModel)
	metaOK(t, s, "%instantiate P::car")

	got := metadataLines(t, s, "P::car")
	if len(got) != 6 || !strings.HasPrefix(got[0], "Object: P::car (ID: ") ||
		got[1] != "Annotations of usage P::car, which the object is an instance of:" ||
		got[5] != `    who = "car"` {
		t.Errorf("an instantiated usage is answered as its object:\n%s", strings.Join(got, "\n"))
	}

	// A nested object is of the usage holding it and of that usage's types.
	engine := metadataLines(t, s, "car.engine")
	if len(engine) < 1 || !strings.HasPrefix(engine[0], "Object: P::car.engine (ID: ") {
		t.Fatalf("car.engine:\n%s", strings.Join(engine, "\n"))
	}
	want := []string{
		"Annotations of usage P::Car::engine, which the object is an instance of:",
		"  @P::Tag",
		`    who = "nobody" (default)`,
		"Annotations of definition P::Engine, which the object is an instance of:",
		"  @P::Deep",
	}
	wantLines(t, engine[1:6], want)
	wantLines(t, engine[len(engine)-3:], []string{
		"Annotations of definition P::Base, which P::Engine specializes:",
		"  @P::Tag",
		`    who = "base"`,
	})

	// The object's id and an indexed path reach the same listing.
	id := strings.TrimSuffix(strings.TrimPrefix(engine[0], "Object: P::car.engine (ID: "), ")")
	wantLines(t, metadataLines(t, s, "#"+id)[1:], engine[1:])
	wantLines(t, metadataLines(t, s, "car.wheels[1]")[1:], []string{
		"Annotations of usage P::Car::wheels, which the object is an instance of:",
		"  (no annotations)",
		"Annotations of definition P::Wheel, which the object is an instance of:",
		"  (no annotations)",
	})
}

func TestMetadataTellsNoAnnotationsFromAnUnresolvedTarget(t *testing.T) {
	s := NewSession()
	submitModel(t, s, metadataCommandModel)

	wantLines(t, metadataLines(t, s, "P::plain"), []string{
		"Element: P::plain (partUsage)",
		"  (no annotations)",
	})
	for target, want := range map[string]string{
		"P::nope":    "error: unresolved reference: P::nope",
		"#99":        "error: no object #99 in this session",
		"plain.hub":  `error: no instance of "P::plain"`,
		"nothing.at": "error: unresolved reference: nothing",
	} {
		got := metadataLines(t, s, target)
		if len(got) == 0 || !strings.HasPrefix(got[0], want) {
			t.Errorf("%%metadata %s = %q, want %q", target, got, want)
		}
	}
	wantLines(t, mustMeta(t, s, "%metadata"), []string{metadataUsage})
	wantLines(t, mustMeta(t, s, "%metadata P::car json"), []string{metadataUsage})
}

// mustMeta runs a command line and answers its output lines.
func mustMeta(t *testing.T, s *Session, line string) []string {
	t.Helper()
	out, _, err := s.RunMeta(line)
	if err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	return out
}

func TestMetadataReportsAValueThatCannotBeEvaluated(t *testing.T) {
	s := NewSession()
	submitModel(t, s, `
package Bad {
    private import ScalarValues::*;
    metadata def M { attribute n : Integer; }
    part def Q {
        @M { n = 1 / 0; }
        @M { n = 4; }
    }
}
`)
	wantLines(t, metadataLines(t, s, "Bad::Q"), []string{
		"Element: Bad::Q (partDef)",
		"  @Bad::M",
		"    <error: metadata Bad::M: n: division by zero>",
		"  @Bad::M",
		"    n = 4",
	})
	if len(s.materializeFailures) != 1 {
		t.Errorf("the failed value is %d findings about the model, want 1", len(s.materializeFailures))
	}
}

func TestMetadataListsTheFeaturesOfALibraryMetadataType(t *testing.T) {
	s := NewSession()
	submitModel(t, s, `
package L {
    private import ModelingMetadata::*;
    part def A { @Rationale { text = "because"; } }
}
`)
	got := metadataLines(t, s, "L::A")
	if len(got) != 4 || got[1] != "  @ModelingMetadata::Rationale" || got[2] != `    text = "because"` ||
		got[3] != "    explanation = []" {
		t.Errorf("a library metadata type's own features are listed, an item's are not:\n%s", strings.Join(got, "\n"))
	}
}
