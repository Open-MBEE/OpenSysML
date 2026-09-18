package export_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// withoutSourceLayout strips the lines a graph carries for layout alone, keeping
// sysx:sourceLanguage: the grammar a root is written under is a fact the mapping reads.
func withoutSourceLayout(t *testing.T, turtle []byte) []byte {
	t.Helper()
	return withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail")
}

// mappingAloneRoundTrip converts notation to Turtle, strips the layout lines, writes
// the notation the mapping alone spells, and checks that notation states the same graph.
func mappingAloneRoundTrip(t *testing.T, name string, src []byte) string {
	t.Helper()
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	first, err := export.Convert(name, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	stripped := withoutSourceLayout(t, first)
	back, err := export.Convert(stem+".ttl", stripped, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation from the mapping alone: %v", err)
	}
	again, err := export.Convert(name, back, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again from the mapping alone: %v\n--- notation ---\n%s", err, back)
	}
	if lost, gained := tripleSetDiff(t, stripped, withoutSourceLayout(t, again)); len(lost)+len(gained) > 0 {
		t.Errorf("the mapping alone changed the graph\n--- notation ---\n%s\n--- lost ---\n%s\n--- gained ---\n%s",
			back, strings.Join(lost, "\n"), strings.Join(gained, "\n"))
	}
	return string(back)
}

func checkSpelling(t *testing.T, notation string, want, forbade []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(notation, w) {
			t.Errorf("the mapping alone should spell %q\n--- notation ---\n%s", w, notation)
		}
	}
	for _, f := range forbade {
		if strings.Contains(notation, f) {
			t.Errorf("the mapping alone should not spell %q\n--- notation ---\n%s", f, notation)
		}
	}
}

// TestFeaturePrefixesSurviveTheMappingAlone pins that isConstant and isPortion come
// back in the grammar of the root that states them: KerML's `const` and `portion`,
// SysML's `constant`, with the source lines stripped so only the predicates carry them.
func TestFeaturePrefixesSurviveTheMappingAlone(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "convert", "feature_prefixes.kerml"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name          string
		src           string
		want, forbade []string
	}{{
		name: "const.kerml",
		src: `package Prefixes {
    class A;
    class B {
        const feature k : A;
        var const feature vk : A;
    }
}
`,
		want:    []string{"const feature k : A;", "const var feature vk : A;"},
		forbade: []string{"constant"},
	}, {
		name: "portion.kerml",
		src: `package FeatureMods {
    class A;
    class B {
        portion feature p : A;
        var portion feature pv : A;
    }
}
`,
		want:    []string{"portion feature p : A;", "portion var feature pv : A;"},
		forbade: []string{"composite"},
	}, {
		// The stdlib's shapes (Occurrences.kerml): a portion with `all`, an
		// anonymous portion redefinition, a portion bound to a value.
		name: "feature_prefixes.kerml",
		src:  string(fixture),
		want: []string{
			"const feature k : A;",
			"portion feature p : A;",
			"portion feature all slices : A[1..*] subsets p;",
			"portion feature redefines p[1];",
			"portion redefines pv = p;",
			"derived const feature dk : A;",
		},
		forbade: []string{"constant", "composite"},
	}, {
		name: "constant.sysml",
		src: `package Prefixes {
    attribute def A;
    part def B {
        constant attribute k : A;
        derived constant attribute dk : A;
    }
}
`,
		want:    []string{"constant attribute k : A;", "derived constant attribute dk : A;"},
		forbade: []string{"const attribute", "portion"},
	}, {
		// A SysML portion is `snapshot`/`timeslice`; its isPortion writes no prefix.
		name: "portions.sysml",
		src: `package Portions {
    occurrence def Car;
    occurrence car : Car {
        snapshot start : Car;
        timeslice middle : Car;
    }
}
`,
		want:    []string{"snapshot start : Car;", "timeslice middle : Car;"},
		forbade: []string{"portion ", "composite"},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkSpelling(t, mappingAloneRoundTrip(t, tc.name, []byte(tc.src)), tc.want, tc.forbade)
		})
	}
}

// TestPortionFlagIsCarriedOrRefused corrupts the portion facts: the decoder refuses a
// graph whose isPortion the root's grammar cannot spell rather than respelling it.
func TestPortionFlagIsCarriedOrRefused(t *testing.T) {
	kerml := []byte(`package FeatureMods {
    class A;
    class B {
        portion feature p : A;
    }
}
`)
	sysml := []byte(`package Prefixes {
    attribute def A;
    part def B {
        constant attribute k : A;
    }
}
`)
	kermlGraph, err := export.Convert("m.kerml", kerml, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	sysmlGraph, err := export.Convert("m.sysml", sysml, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	portion := "    sysml:isComposite \"true\"^^xsd:boolean ;\n    sysml:isPortion \"true\"^^xsd:boolean ;"
	constant := "    sysml:isConstant \"true\"^^xsd:boolean ;"
	if !strings.Contains(string(kermlGraph), portion) {
		t.Fatalf("a KerML portion should state isComposite and isPortion:\n%s", kermlGraph)
	}
	cases := []struct {
		name     string
		graph    []byte
		old, new string
		want     []string
	}{{
		name: "sysml_root_without_portion_kind", graph: sysmlGraph,
		old: constant, new: constant + "\n    sysml:isPortion \"true\"^^xsd:boolean ;",
		want: []string{"the portion <urn:sysmlv2:element:Prefixes__B__k>", "no sysml:portionKind", "only as `snapshot` or `timeslice`"},
	}, {
		name: "sysml_root_composite_portion", graph: sysmlGraph,
		old: constant, new: constant + "\n    sysml:isComposite \"true\"^^xsd:boolean ;\n    sysml:isPortion \"true\"^^xsd:boolean ;",
		want: []string{"the portion <urn:sysmlv2:element:Prefixes__B__k>", "no sysml:portionKind"},
	}, {
		name: "kerml_portion_not_composite", graph: kermlGraph,
		old: portion, new: "    sysml:isPortion \"true\"^^xsd:boolean ;",
		want: []string{"the portion <urn:sysmlv2:element:FeatureMods__B__p>", "without sysml:isComposite", "a portion is composite"},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			turtle := editTurtle(t, withoutSourceLayout(t, tc.graph), tc.old, tc.new)
			back, err := export.Convert("m.ttl", turtle, export.FormatTurtle, export.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("expected an UnsupportedError, got %v\n--- notation ---\n%s", err, back)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected %q in error:\n%s", want, err)
				}
			}
		})
	}
	// A graph stating a portion kind alone, as graphs written before isPortion was
	// exported do, reads back by its kind; re-exported, the implied flag is all it gains.
	t.Run("sysml_portion_kind_without_flag", func(t *testing.T) {
		src := []byte("package Portions {\n    occurrence def Car;\n    occurrence car : Car {\n        snapshot start : Car;\n    }\n}\n")
		graph, err := export.Convert("m.sysml", src, export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatal(err)
		}
		current := withoutSourceLayout(t, graph)
		legacy := withoutTriples(t, current, "sysml:isPortion")
		back, err := export.Convert("m.ttl", legacy, export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("back to notation: %v", err)
		}
		checkSpelling(t, string(back), []string{"snapshot start : Car;"}, []string{"portion ", "composite"})
		again, err := export.Convert("m.sysml", back, export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatal(err)
		}
		if lost, gained := tripleSetDiff(t, current, withoutSourceLayout(t, again)); len(lost)+len(gained) > 0 {
			t.Errorf("a legacy portion graph should re-export as the current one\n--- lost ---\n%s\n--- gained ---\n%s",
				strings.Join(lost, "\n"), strings.Join(gained, "\n"))
		}
	})
	// A KerML graph whose root states no language reads as SysML, and the
	// spelling follows: the language triple is what selects `const`.
	t.Run("kerml_root_without_language", func(t *testing.T) {
		src := []byte("package Prefixes {\n    class A;\n    class B {\n        const feature k : A;\n    }\n}\n")
		graph, err := export.Convert("m.kerml", src, export.FormatSysML, export.FormatTurtle)
		if err != nil {
			t.Fatal(err)
		}
		back, err := export.Convert("m.ttl", withoutSourceText(t, graph), export.FormatTurtle, export.FormatSysML)
		if err != nil {
			t.Fatalf("back to notation: %v", err)
		}
		checkSpelling(t, string(back), []string{"constant feature k : A;"}, []string{"const feature"})
	})
}
