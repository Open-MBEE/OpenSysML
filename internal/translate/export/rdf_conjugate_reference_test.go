package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// conjugateReferenceSource names the implied conjugate `P::'~P'` four ways: a conjugated
// typing, a conjugated typing by the conjugate itself, which conjugates nothing further,
// an ordinary typing and an alias (SysML v2 8.3.12.3).
const conjugateReferenceSource = `package Q {
    port def P;
    part def Box {
        port a : ~P;
        port x : ~P::'~P';
        ref c : P::'~P';
        alias Conj for P::'~P';
    }
}
`

// TestConjugateReferencesKeepTheirForm checks the graph links every spelling of
// the conjugate to the one element and the notation read back from the graph
// alone keeps each form: only the ConjugatedPortTyping is written `~P`.
func TestConjugateReferencesKeepTheirForm(t *testing.T) {
	graph := normativeGraph(t, conjugateReferenceSource)
	conjugate := rdf.ElementIRIForID("Q__P_conjugated")
	for _, subject := range []string{"Q__Box__a", "Q__Box__x", "Q__Box__c"} {
		if got, ok := graph.Object(rdf.ElementIRIForID(subject), rdf.SysML+relationshipProperty[ast.RelTyping]); !ok || got != conjugate {
			t.Errorf("%s sysml:type = %v, %v; want %v", subject, got, ok, conjugate)
		}
	}
	if got, ok := graph.Object(rdf.ElementIRIForID("Q__Box__Conj"), rdf.SysML+pMemberElement); !ok || got != conjugate {
		t.Errorf("Conj memberElement = %v, %v; want %v", got, ok, conjugate)
	}

	notation, err := ToSysML(withoutSourceLayout(graph))
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, want := range []string{"port a : ~P;", "port x : ~P;", "ref c : P::'~P';", "alias Conj for P::'~P';"} {
		if !strings.Contains(string(notation), want) {
			t.Errorf("notation lacks %q:\n%s", want, notation)
		}
	}
	root := parser.New(source.New("again.sysml", notation)).ParseFile()
	typings := map[string]bool{}
	ast.Inspect(root, func(n ast.Node) bool {
		if usage, ok := n.(*ast.Usage); ok {
			for _, rel := range usage.Relationships {
				if rel.Kind == ast.RelTyping {
					typings[usage.Ident.Name] = rel.Conjugated
				}
			}
		}
		return true
	})
	if !typings["a"] || !typings["x"] || typings["c"] {
		t.Errorf("conjugated typings after read-back: %v; want a and x only", typings)
	}
}
