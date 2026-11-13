package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// An annotating element owns one Annotation per element its `about` names, after its
// typing and ahead of its body, as KerML 8.3.2.3.2 (ownedAnnotatingRelationship) and the
// pilot state it; the collapsed annotatedElement stays beside them.
func TestAboutAnnotationsAreOwnedByTheAnnotatingElement(t *testing.T) {
	src := "package P {\n\tmetadata def Heritage { attribute level : ScalarValues::Integer; }\n\tpart def Telescope;\n\tpart telescope : Telescope;\n\tpart camera : Telescope;\n\t@Heritage about telescope, camera { level = 2; }\n\tcomment about telescope, camera /* both */\n\tcomment note about camera /* one */\n\tmetadata m : Heritage about camera;\n}"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for owner, targets := range map[string][]string{
		"P___404": {"P__telescope", "P__camera"},
		"P___405": {"P__telescope", "P__camera"},
		"P__note": {"P__camera"},
		"P__m":    {"P__camera"},
	} {
		for i, target := range targets {
			annotation := "elmt:" + owner + "_an" + string(rune('0'+i))
			for _, want := range []string{
				annotation + "\n    a sysml:Annotation ;",
				annotation + "\n    a sysml:Annotation ;\n    sysml:elementId \"" + owner + "_an" + string(rune('0'+i)) + "\"",
				"sysml:annotatingElement elmt:" + owner,
				"sysml:annotatedElement elmt:" + target,
				"sysml:owningRelatedElement elmt:" + owner,
				"sysml:ownedAnnotatingRelationship elmt:" + owner + "_an0",
			} {
				if !strings.Contains(graph, want) {
					t.Errorf("the graph does not state %q:\n%s", want, graph)
				}
			}
		}
	}
	// The metadata usage's typing, its annotations, then its body member.
	if want := "sysml:ownedRelationship elmt:P___404_ft0, elmt:P___404_an0, elmt:P___404_an1, elmt:P___404__level_om ;"; !strings.Contains(graph, want) {
		t.Errorf("the graph does not own the head ahead of the body %q:\n%s", want, graph)
	}
	if want := "sysml:ownedRelationship elmt:P___405_an0, elmt:P___405_an1 ;"; !strings.Contains(graph, want) {
		t.Errorf("the comment does not own its annotations %q:\n%s", want, graph)
	}
	back, err := convert.Convert("m.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{"@Heritage about telescope, camera {", "comment about telescope, camera", "comment note about camera", "metadata m : Heritage about camera;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation does not come back with %q:\n%s", want, back)
		}
	}
}

// A repeated `about` name states one Annotation per name on the API notation round trip too.
func TestRepeatedAboutNamesRoundTrip(t *testing.T) {
	src := "package P {\n\tpart x;\n\tcomment about x, x /* twice */\n}"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if want := "sysml:ownedRelationship elmt:P___401_an0, elmt:P___401_an1 ;"; !strings.Contains(string(turtle), want) {
		t.Errorf("the comment does not own two Annotations %q:\n%s", want, turtle)
	}
	back, err := convert.Convert("m.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "comment about x, x") {
		t.Errorf("the notation does not come back with both names:\n%s", back)
	}
}
