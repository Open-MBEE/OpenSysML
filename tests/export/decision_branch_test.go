package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

const decisionBranches = `package P {
    action def A {
        attribute level = 0;
        action check;
        decide pick;
            if level > 1 then go;
            else stop;
        action go;
        action stop;
    }
}
`

// A decision's branches come back from the graph alone, as written.
func TestDecisionBranchesComeBackFromTheGraphAlone(t *testing.T) {
	turtle, err := convert.Convert("m.sysml", []byte(decisionBranches), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := convert.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, branch := range []string{"if level > 1 then go;", "else stop;"} {
		if !strings.Contains(string(back), branch) {
			t.Errorf("the branch %q did not come back:\n%s", branch, back)
		}
	}
}

// A branch the notation cannot state as the graph does is refused rather than
// written as another branch: one whose source is not the member before it,
// which `if … then` reads its source from; an `else` owning a body; and one
// with neither a guard nor `else`, which `then` alone would write as a
// succession.
func TestDecisionBranchesTheNotationCannotStateAreRefused(t *testing.T) {
	turtle, err := convert.Convert("m.sysml", []byte(decisionBranches), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(withoutTriples(t, turtle, "sysx:sourceText"))
	for name, edit := range map[string]func(string) string{
		"another source": func(g string) string {
			return strings.Replace(g, "sysml:source elmt:P__A__pick ;", "sysml:source elmt:P__A__check ;", 1)
		},
		"another source stated as sourceFeature alone": func(g string) string {
			return strings.Replace(g, "sysml:source elmt:P__A__pick ;", "sysml:sourceFeature elmt:P__A__check ;", 1)
		},
		"two sources, the member before it first": func(g string) string {
			return strings.Replace(g, "sysml:source elmt:P__A__pick ;", "sysml:source elmt:P__A__pick, elmt:P__A__check ;", 1)
		},
		"two sourceFeature sources, the member before it first": func(g string) string {
			return strings.Replace(g, "sysml:source elmt:P__A__pick ;", "sysml:sourceFeature elmt:P__A__pick, elmt:P__A__check ;", 1)
		},
		"a sourceFeature spelled as a literal": func(g string) string {
			return strings.Replace(g, "sysml:source elmt:P__A__pick ;", "sysml:source elmt:P__A__pick ;\n    sysml:sourceFeature \"urn:sysmlv2:element:P__A__pick\" ;", 1)
		},
		"an else with a body": func(g string) string {
			return strings.Replace(g, `sysx:isElse "true"^^xsd:boolean ;`, `sysx:isElse "true"^^xsd:boolean ;`+"\n    sysx:hasBody \"true\"^^xsd:boolean ;", 1)
		},
		"a trigger": func(g string) string {
			return strings.Replace(g, `sysx:transitionSyntax "target" ;`, `sysx:transitionSyntax "target" ;`+"\n    sysx:trigger \"accept Go\" ;", 1)
		},
		"neither guard nor else": func(g string) string {
			return strings.Replace(g, `sysx:isElse "true"^^xsd:boolean ;`, "", 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			edited := edit(graph)
			if edited == graph {
				t.Fatal("the edit changed nothing")
			}
			back, err := convert.Convert("m.ttl", []byte(edited), convert.FormatTurtle, convert.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Errorf("converted to\n%s\nwant it refused, got error %v", back, err)
			}
		})
	}
}
