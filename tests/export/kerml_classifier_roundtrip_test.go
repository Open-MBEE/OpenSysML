package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// Each KerML classifier keyword comes back as written from the structural triples alone,
// and converting that notation again gives the first graph's structure back.
func TestKerMLClassifierKeywordsComeBackFromTheGraphAlone(t *testing.T) {
	for _, keyword := range []string{"classifier", "class", "struct", "assoc", "assoc struct", "behavior",
		"predicate", "interaction", "datatype", "function", "metaclass"} {
		t.Run(keyword, func(t *testing.T) {
			src := "package P {\n    " + keyword + " A;\n    " + keyword + " X specializes A;\n}\n"
			turtle, err := convert.Convert("k.kerml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			stripped := withoutTriples(t, turtle, "sysx:sourceText")
			back, err := convert.Convert("k.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v\n%s", err, stripped)
			}
			if string(back) != src {
				t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", src, back)
			}
			second, err := convert.Convert("k.kerml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("second hop to turtle: %v", err)
			}
			if got := withoutTriples(t, second, "sysx:sourceText"); string(got) != string(stripped) {
				t.Errorf("second hop changed the graph:\n--- first ---\n%s--- second ---\n%s", stripped, got)
			}
			if !strings.Contains(string(turtle), "elmt:P__X_sc0\n    a sysml:Subclassification ;") {
				t.Errorf("the specializes is not written as the Subclassification P__X_sc0:\n%s", turtle)
			}
		})
	}
}

// A graph that states no keyword, as one another tool wrote, still comes back under the
// keyword of its metaclass, so converting it again keeps the metaclass.
func TestKerMLClassifierKeywordsComeBackFromTheMetaclassAlone(t *testing.T) {
	for _, keyword := range []string{"classifier", "class", "struct", "assoc", "assoc struct", "behavior",
		"predicate", "interaction", "metaclass"} {
		t.Run(keyword, func(t *testing.T) {
			src := "package P {\n    " + keyword + " A;\n    " + keyword + " X specializes A;\n}\n"
			turtle, err := convert.Convert("k.kerml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			stripped := withoutTriples(t, withoutTriples(t, turtle, "sysx:sourceText"), "sysx:declaredKeyword")
			back, err := convert.Convert("k.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v\n%s", err, stripped)
			}
			if string(back) != src {
				t.Errorf("round trip changed the notation:\n--- want ---\n%s--- got ---\n%s", src, back)
			}
		})
	}
}
