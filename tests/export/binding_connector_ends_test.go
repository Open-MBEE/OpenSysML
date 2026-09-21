package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A binding's two ends are standard owned connector-end features.
func TestBindingConnectorEndsAreStatedLikeSuccessionEnds(t *testing.T) {
	src := "package P {\n    part def Car {\n        attribute a : Integer;\n        attribute b : Integer;\n        bind e3 ::> a = b;\n        succession first s1 ::> a then s2 ::> b;\n    }\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, want := range []string{
		"elmt:P__Car___402\n    a sysml:BindingConnectorAsUsage ;",
		"sysml:connectorEnd expr:P__Car___402_pend0, expr:P__Car___402_pend1 ;",
		"sysx:endForm \"equals\" ;",
		"sysx:declaredKeyword \"bind\" ;",
		"expr:P__Car___402_pend0\n    a sysml:ReferenceUsage ;",
		"sysml:elementId \"P__Car___402_pend0\" ;",
		"sysml:isEnd \"true\"^^xsd:boolean ;",
		"sysml:references elmt:P__Car__a",
		"sysml:declaredName \"e3\" ;",
		"expr:P__Car___402_pend1\n    a sysml:ReferenceUsage ;",
		"sysml:elementId \"P__Car___402_pend1\" ;",
		"sysml:references elmt:P__Car__b",
		"sysml:declaredName \"s1\" ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q\n%s", want, graph)
		}
	}
	for _, legacy := range []string{"sysx:relatedFeature", "sysx:endIndex", "sysx:endRole", "sysx:endName"} {
		if strings.Contains(graph, legacy) {
			t.Errorf("the standard end mapping emitted retired property %s\n%s", legacy, graph)
		}
	}
	back := backFromTheGraphAlone(t, graph)
	if !strings.Contains(back, "bind e3 ::> a = b;") {
		t.Errorf("the named end should come back as written\n%s", back)
	}
}

// KerML writes binding ends after `of` with either ReferencesKeyword; the graph
// keeps the spelling (sysx:endReferencesKeyword) so each end comes back as written.
func TestKerMLBindingConnectorEndsCarryTheRoundTripWithoutSourceText(t *testing.T) {
	src := `package Corpus {
	class A { feature x; }
	class B { feature y; }
	class Ctx {
		feature a : A;
		feature b : B;
		feature p : A;
		feature q : B;
		binding a = b;
		binding of a = b;
		binding ab of a = b;
		binding all of a = b;
		binding [1] a = b;
		binding of e1 ::> a = e2 ::> b;
		binding named of e1 references a = e2 references b;
		binding of [1] e1 ::> a.x = [0..1] e2 references b.y;
		binding p.x = q.y;
		binding of Corpus::Ctx::a = b;
	}
}
`
	turtle, err := convert.Convert("corpus.kerml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, want := range []string{
		"sysml:declaredName \"e1\" ;",
		"sysml:declaredName \"e2\" ;",
		"sysx:endReferencesKeyword \"references\"",
		"sysml:declaredName \"named\" ;",
		"sysml:isAll \"true\"^^xsd:boolean ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should carry %q\n%s", want, graph)
		}
	}
	for _, legacy := range []string{"sysx:relatedFeature", "sysx:endIndex", "sysx:endRole", "sysx:endName"} {
		if strings.Contains(graph, legacy) {
			t.Errorf("the standard end mapping emitted retired property %s\n%s", legacy, graph)
		}
	}
	backBytes, err := convert.Convert("corpus.kerml", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("from turtle: %v", err)
	}
	back := string(backBytes)
	for _, want := range []string{
		"binding a = b;",
		"binding of a = b;",
		"binding ab of a = b;",
		"binding all of a = b;",
		"binding [1] a = b;",
		"binding of e1 ::> a = e2 ::> b;",
		"binding named of e1 references a = e2 references b;",
		"binding of [1] e1 ::> a.x = [0..1] e2 references b.y;",
		"binding p.x = q.y;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should read %q\n%s", want, back)
		}
	}
	// The qualified end names its referent from the graph alone.
	if n := strings.Count(back, "binding of a = b;"); n != 1 {
		t.Errorf("the qualified end should come back as its referent, got %d plain `binding of a = b;`\n%s", n, back)
	}
}

// A binding end the notation cannot write is refused by name — an end named
// twice, an end named but relating no feature, a binding relating no ends —
// rather than dropped or reported as a syntax error.
func TestBindingEndsWithoutANotationAreRefused(t *testing.T) {
	src := "package P {\n    part def Car {\n        attribute a : Integer;\n        attribute b : Integer;\n        bind e3 ::> a = b;\n    }\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	stripped := string(withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:sourceTail"))
	const end0 = "expr:P__Car___402_pend0"
	cases := []struct {
		name string
		edit func(string) string
		want []string
	}{
		{
			name: "two names",
			edit: func(g string) string {
				return strings.Replace(g, "sysml:declaredName \"e3\" ;", "sysml:declaredName \"e3\", \"e4\" ;", 1)
			},
			want: []string{end0, "more than one end name"},
		},
		{
			name: "named without a feature",
			edit: func(g string) string {
				return strings.Replace(g, "sysml:references elmt:P__Car__a", "sysml:references elmt:missing", 1)
			},
			want: []string{"reference"},
		},
		{
			name: "unknown references keyword",
			edit: func(g string) string {
				return strings.Replace(g, "sysml:references elmt:P__Car__a", "sysml:references elmt:P__Car__a ;\n    sysx:endReferencesKeyword \"subsets\"", 1)
			},
			want: []string{"ReferencesKeyword", "\"subsets\""},
		},
		{
			name: "missing standard end",
			edit: func(g string) string {
				return strings.Replace(g, "sysml:references elmt:P__Car__a", "sysml:references elmt:missing", 1)
			},
			want: []string{"reference"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edited := tc.edit(stripped)
			if edited == stripped {
				t.Fatalf("the edit changed nothing:\n%s", stripped)
			}
			_, err := convert.Convert("m.ttl", []byte(edited), convert.FormatTurtle, convert.FormatSysML)
			var unsupported *export.UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an *export.UnsupportedError naming the binding end, got %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal should name %q: %v", want, err)
				}
			}
		})
	}
}
