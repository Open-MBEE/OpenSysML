package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

const indexedEndsSource = `package F3Idx {
    private import ScalarValues::*;
    port def P { attribute value : Real default 0.0; }
    part def Source { port y : P[2]; }
    part def Sink { port u : P; }
    connection def C { end source[1] : P; end target[1] : P; }
    part def Asm {
        part s : Source;
        part k : Sink;
        attribute i : Integer = 2;
        connection : C connect [1] s.y#(1) to [1] k.u;
        interface s.y#(i + 0) to k.u;
        flow s.y#(2) to k.u;
        bind s.y#(i) = k.u;
    }
}
`

// An indexed end (an OpenSysML extension) reference-subsets the whole feature
// as a standard end does and states the index as a sysx:endElement expression
// node, which alone carries the `#( … )` back to notation.
func TestIndexedConnectorEndsCarryTheirIndexAsSysxEndElement(t *testing.T) {
	turtle, err := convert.Convert("m.sysml", []byte(indexedEndsSource), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph := string(turtle)
	for _, want := range []string{
		"sysx:endElement expr:F3Idx__Asm___403_pend0_pelement ;",
		"expr:F3Idx__Asm___403_pend0_pelement\n    a sysml:LiteralInteger ;",
		"sysml:chainingFeature elmt:F3Idx__Asm__s, elmt:F3Idx__Source__y ;",
		"expr:F3Idx__Asm___404_pend0_pelement\n    a sysml:OperatorExpression ;",
		"sysx:endElement expr:F3Idx__Asm___405_pend0_pelement ;",
		"sysx:endElement expr:F3Idx__Asm___406_pend0_pelement ;",
		"expr:F3Idx__Asm___406_pend0_pelement\n    a sysml:FeatureReferenceExpression ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q\n%s", want, graph)
		}
	}
	if n := strings.Count(graph, "sysx:endElement "); n != 4 {
		t.Errorf("sysx:endElement stated %d times, want once per indexed end (4)\n%s", n, graph)
	}
	for _, legacy := range []string{"sysx:relatedFeature", "sysx:endIndex", "sysx:endRole", "sysx:endName"} {
		if strings.Contains(graph, legacy) {
			t.Errorf("the standard end mapping emitted retired property %s\n%s", legacy, graph)
		}
	}

	back := backFromTheGraphAlone(t, graph)
	for _, want := range []string{
		"connection : C connect [1] s.y#(1) to [1] k.u;",
		"interface s.y#(i + 0) to k.u;",
		"flow s.y#(2) to k.u;",
		"bind s.y#(i) = k.u;",
	} {
		if !strings.Contains(back, want) {
			t.Errorf("the notation should come back with %q\n%s", want, back)
		}
	}

	// Without sysx:endElement the graph states ends attaching whole features,
	// and that is what comes back: the predicate carries the index.
	whole := withoutTriples(t, withoutTriples(t, withoutTriples(t, turtle, "sysx:endElement"), "sysx:sourceText"), "sysx:sourceTail")
	plain, err := convert.Convert("m.ttl", whole, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back without sysx:endElement: %v", err)
	}
	if strings.Contains(string(plain), "#(") {
		t.Errorf("without sysx:endElement no end should be indexed\n%s", plain)
	}
	for _, want := range []string{
		"connection : C connect [1] s.y to [1] k.u;",
		"flow s.y to k.u;",
		"bind s.y = k.u;",
	} {
		if !strings.Contains(string(plain), want) {
			t.Errorf("without sysx:endElement the end should attach the whole feature, %q\n%s", want, plain)
		}
	}
}

// The API JSON element form keys the index under sysx:endElement on the end
// and reads back to the same notation.
func TestIndexedConnectorEndsRoundTripThroughAPIJSON(t *testing.T) {
	doc, err := convert.Convert("m.sysml", []byte(indexedEndsSource), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatal(err)
	}
	indexed := 0
	for _, e := range elements {
		element, ok := e["sysx:endElement"].(map[string]any)
		if !ok {
			continue
		}
		indexed++
		if e["isEnd"] != true {
			t.Errorf("%v carries sysx:endElement but is no end (%v)", e["@id"], e["@type"])
		}
		if _, ok := element["@id"].(string); !ok {
			t.Errorf("%v: sysx:endElement %v should reference an expression element", e["@id"], element)
		}
	}
	if indexed != 4 {
		t.Errorf("found %d ends with sysx:endElement, want 4\n%s", indexed, doc)
	}
	back, err := convert.Convert("m.json", doc, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("the API JSON did not read back: %v", err)
	}
	for _, want := range []string{"s.y#(1)", "s.y#(i + 0)", "s.y#(2)", "s.y#(i)"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the API JSON should read back with %q\n%s", want, back)
		}
	}
}
