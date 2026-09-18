package export_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
)

// A binding's two ends are connector ends (SysML.xtext:1000): each is a
// sysx:relatedFeature node that names itself with sysx:endName the way a
// succession's or connector's does, and the graph states no sysml:value.
func TestBindingConnectorEndsAreStatedLikeSuccessionEnds(t *testing.T) {
	src := "package P {\n    part def Car {\n        attribute a : Integer;\n        attribute b : Integer;\n        bind e3 ::> a = b;\n        succession first s1 ::> a then s2 ::> b;\n    }\n}\n"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, want := range []string{
		"elmt:P__Car___402\n    a sysml:BindingConnectorAsUsage ;",
		"sysx:relatedFeature expr:P__Car___402_pend0, expr:P__Car___402_pend1 ;\n    sysx:endForm \"equals\" ;\n    sysx:declaredKeyword \"bind\" ;",
		"expr:P__Car___402_pend0\n    a sysml:FeatureReferenceExpression ;\n    sysx:sourceText \"a\" ;\n    sysml:elementId \"P__Car___402_pend0\" ;\n    sysml:referent elmt:P__Car__a ;\n    sysx:endIndex \"0\"^^xsd:integer ;\n    sysx:endName \"e3\" .",
		"expr:P__Car___402_pend1\n    a sysml:FeatureReferenceExpression ;\n    sysx:sourceText \"b\" ;\n    sysml:elementId \"P__Car___402_pend1\" ;\n    sysml:referent elmt:P__Car__b ;\n    sysx:endIndex \"1\"^^xsd:integer .",
		"sysx:endIndex \"0\"^^xsd:integer ;\n    sysx:endName \"s1\" .",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q\n%s", want, graph)
		}
	}
	for _, legacy := range []string{"sysml:value expr:", "sysml:references", "sysml:declaredName \"e3\"", "_pvalue"} {
		if strings.Contains(graph, legacy) {
			t.Errorf("a binding end is neither the connector's name nor its value (%s)\n%s", legacy, graph)
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
	turtle, err := export.Convert("corpus.kerml", []byte(src), export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, want := range []string{
		"sysx:endIndex \"0\"^^xsd:integer ;\n    sysx:endName \"e1\" .",
		"sysx:endIndex \"1\"^^xsd:integer ;\n    sysx:endName \"e2\" .",
		"sysx:endName \"e1\" ;\n    sysx:endReferencesKeyword \"references\" .",
		"sysx:endName \"e2\" ;\n    sysx:endReferencesKeyword \"references\" .",
		"sysml:declaredName \"named\" ;",
		"sysml:isAll \"true\"^^xsd:boolean ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should carry %q\n%s", want, graph)
		}
	}
	for _, name := range []string{"e1", "e2", "a", "p"} {
		if strings.Contains(graph, "sysml:declaredName \""+name+"\" ;\n    sysx:relatedFeature") {
			t.Errorf("a binding took its end %s as its name\n%s", name, graph)
		}
	}
	if strings.Contains(graph, "sysml:value expr:") {
		t.Errorf("a binding end is not the connector's value\n%s", graph)
	}
	back := string(structuralRoundTrip(t, "corpus.kerml", turtle))
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
	// The qualified end names its referent, written from the graph alone by its
	// shortest name as every reference is; the referent triple is the same.
	if n := strings.Count(back, "binding of a = b;"); n != 2 {
		t.Errorf("the qualified end should come back as its referent, got %d plain `binding of a = b;`\n%s", n, back)
	}
}

// A binding end the notation cannot write is refused by name — an end named
// twice, an end named but relating no feature, a binding relating no ends —
// rather than dropped or reported as a syntax error.
func TestBindingEndsWithoutANotationAreRefused(t *testing.T) {
	src := "package P {\n    part def Car {\n        attribute a : Integer;\n        attribute b : Integer;\n        bind e3 ::> a = b;\n    }\n}\n"
	turtle, err := export.Convert("m.sysml", []byte(src), export.FormatSysML, export.FormatTurtle)
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
				return strings.Replace(g, "sysx:endName \"e3\" .", "sysx:endName \"e3\", \"e4\" .", 1)
			},
			want: []string{end0, "sysx:endName twice", "\"e3\" and \"e4\""},
		},
		{
			name: "named without a feature",
			edit: func(g string) string {
				g = strings.Replace(g, "    a sysml:FeatureReferenceExpression ;\n    sysml:elementId \"P__Car___402_pend0\" ;\n    sysml:referent elmt:P__Car__a ;\n", "", 1)
				return g
			},
			want: []string{end0, "relates no feature"},
		},
		{
			name: "unknown references keyword",
			edit: func(g string) string {
				return strings.Replace(g, "sysx:endName \"e3\" .", "sysx:endName \"e3\" ;\n    sysx:endReferencesKeyword \"subsets\" .", 1)
			},
			want: []string{end0, "sysx:endReferencesKeyword", "\"subsets\""},
		},
		{
			name: "no related ends",
			edit: func(g string) string {
				g = strings.Replace(g, "sysx:relatedFeature expr:P__Car___402_pend0, expr:P__Car___402_pend1 ;\n", "", 1)
				return g
			},
			want: []string{"P__Car___402", "sysx:relatedFeature"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			edited := tc.edit(stripped)
			if edited == stripped {
				t.Fatalf("the edit changed nothing:\n%s", stripped)
			}
			_, err := export.Convert("m.ttl", []byte(edited), export.FormatTurtle, export.FormatSysML)
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
