package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// Usage::isReference is a required Boolean derived as not isComposite (SysML
// v2.1 §8.3.6.4 deriveUsageIsReference): every usage states it, a composite
// one as false, and it negates the isComposite the same element states.
func TestAPIJSONStatesIsReferenceOnEveryUsage(t *testing.T) {
	for _, test := range []struct {
		name, model string
		want        map[string]bool
	}{
		{
			name: "r.sysml",
			model: `package M {
    part def Engine;
    part def Person;
    part def Car {
        part engine : Engine;
        ref part driver : Person;
        attribute mass : ScalarValues::Real;
        variation part power : Engine {
            variant part four;
        }
        action start { in key : Person; }
    }
}
`,
			want: map[string]bool{
				"M__Car__engine":      false,
				"M__Car__driver":      true,
				"M__Car__mass":        true,
				"M__Car__power":       false,
				"M__Car__power__four": false,
				"M__Car__start":       false,
				"M__Car__start__key":  true,
			},
		},
		{
			// A KerML feature is written as a usage; the isComposite its
			// declaration states is the one isReference negates.
			name: "r.kerml",
			model: `package K {
    class A {
        feature f : A;
        composite feature g : A;
    }
}
`,
			want: map[string]bool{"K__A__f": true, "K__A__g": false},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := source.New(test.name, []byte(test.model))
			graph, err := ToRDF(file, parser.New(file).ParseFile())
			if err != nil {
				t.Fatal(err)
			}
			document, err := WriteAPIJSON(graph)
			if err != nil {
				t.Fatal(err)
			}
			elements := booleansElements(t, document)
			for id, element := range elements {
				metaclass, _ := element["@type"].(string)
				if !ontology.IsAncestorOrSelf(metaclass, "Usage") {
					continue
				}
				reference, stated := element["isReference"].(bool)
				if !stated {
					t.Errorf("%s (%s) does not state isReference: %v", id, metaclass, element["isReference"])
					continue
				}
				if composite, _ := element["isComposite"].(bool); reference == composite {
					t.Errorf("%s (%s) states isReference %v beside isComposite %v", id, metaclass, reference, composite)
				}
			}
			for id, want := range test.want {
				element, ok := elements[id]
				if !ok {
					t.Errorf("no element %s", id)
					continue
				}
				if element["isReference"] != want || element["isComposite"] != !want {
					t.Errorf("%s: isReference %v, isComposite %v; want %v, %v", id, element["isReference"], element["isComposite"], want, !want)
				}
			}
		})
	}
}
