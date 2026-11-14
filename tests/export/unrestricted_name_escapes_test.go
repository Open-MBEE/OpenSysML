package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const escapedNamesModel = `package Observatory {
    part def Pt;
    part def Holder {
        part 'it\'s' : Pt;
        part 'a\\b' : Pt;
        part 'q\"d' : Pt;
        part 'bs\bht\tlf\nff\fcr\rend' : Pt;
    }
    part h : Holder {
        part redefines 'it\'s';
    }
}
`

// escapedNames maps each declared name of escapedNamesModel to the qualified name
// §8.3.2.1.2 derives for it and to the elementId the conversion mints from it.
var escapedNames = []struct{ declared, qualified, id string }{
	{`it's`, `Observatory::Holder::'it\'s'`, "Observatory__Holder__it_27s"},
	{`a\b`, `Observatory::Holder::'a\\b'`, "Observatory__Holder__a_5cb"},
	{`q"d`, `Observatory::Holder::'q\"d'`, "Observatory__Holder__q_22d"},
	{"bs\bht\tlf\nff\fcr\rend", `Observatory::Holder::'bs\bht\tlf\nff\fcr\rend'`, "Observatory__Holder__bs_08ht_09lf_0aff_0ccr_0dend"},
}

// The API element of an escaped name carries the name the escapes denote, its
// qualified name re-escaped once (KerML §8.2.2.3, §8.3.2.1.2).
func TestAPIJSONDeclaresTheNameUnrestrictedEscapesDenote(t *testing.T) {
	document, err := convert.Convert("m.sysml", []byte(escapedNamesModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []struct {
		ID            string  `json:"@id"`
		DeclaredName  *string `json:"declaredName"`
		Name          *string `json:"name"`
		QualifiedName *string `json:"qualifiedName"`
	}
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatalf("decode api-json: %v", err)
	}
	for _, want := range escapedNames {
		found := false
		for _, element := range elements {
			if element.ID != want.id {
				continue
			}
			found = true
			if element.DeclaredName == nil || *element.DeclaredName != want.declared {
				t.Errorf("%s: declaredName = %v, want %q", want.id, text(element.DeclaredName), want.declared)
			}
			if element.Name == nil || *element.Name != want.declared {
				t.Errorf("%s: name = %v, want %q", want.id, text(element.Name), want.declared)
			}
			if element.QualifiedName == nil || *element.QualifiedName != want.qualified {
				t.Errorf("%s: qualifiedName = %v, want %q", want.id, text(element.QualifiedName), want.qualified)
			}
		}
		if !found {
			t.Errorf("no element has the id %q", want.id)
		}
	}
}

func text(value *string) string {
	if value == nil {
		return "null"
	}
	return *value
}

// The Turtle of an escaped name declares the denoted name under an IRI minted from it.
func TestTurtleDeclaresTheNameUnrestrictedEscapesDenote(t *testing.T) {
	graph, err := rdf.ParseTurtle(idTurtle(t, escapedNamesModel))
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}
	for _, want := range escapedNames {
		subject := rdf.ElementIRIForID(want.id)
		if got, ok := graph.Lexical(subject, rdf.SysML+"declaredName"); !ok || got != want.declared {
			t.Errorf("%s: sysml:declaredName = %q, %t; want %q", want.id, got, ok, want.declared)
		}
		if got, ok := graph.Lexical(subject, rdf.SysML+"elementId"); !ok || got != want.id {
			t.Errorf("%s: sysml:elementId = %q, %t; want it", want.id, got, ok)
		}
	}
}

// A graph that lost its source text writes each denoted name back escaped, and
// that notation converts to the graph it came from.
func TestEscapedNamesComeBackEscapedFromTheGraphAlone(t *testing.T) {
	first := idTurtle(t, escapedNamesModel)
	for _, form := range []convert.Format{convert.FormatTurtle, convert.FormatAPIJSON} {
		intermediate := withoutSourceText(t, first)
		if form == convert.FormatAPIJSON {
			var err error
			if intermediate, err = convert.Convert("m.ttl", intermediate, convert.FormatTurtle, form); err != nil {
				t.Fatalf("to api-json: %v", err)
			}
		}
		back, err := convert.Convert("m", intermediate, form, convert.FormatSysML)
		if err != nil {
			t.Fatalf("back to notation: %v", err)
		}
		for _, declaration := range []string{
			`part 'it\'s' : Pt;`, `part 'a\\b' : Pt;`, `part 'q"d' : Pt;`,
			`part 'bs\bht\tlf\nff\fcr\rend' : Pt;`, `redefines 'it\'s'`,
		} {
			if !strings.Contains(string(back), declaration) {
				t.Errorf("notation read back lacks %q:\n%s", declaration, back)
			}
		}
		lost, gained := tripleSetDiff(t, withoutSourceText(t, first), withoutSourceText(t, idTurtle(t, string(back))))
		if len(lost)+len(gained) != 0 {
			t.Errorf("the second hop lost %v and gained %v", lost, gained)
		}
	}
}
