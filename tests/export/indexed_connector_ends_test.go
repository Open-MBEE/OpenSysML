package export_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
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

// Derived endpoints name the feature the end reference-subsets, as reflection
// derives it; the selector is on the end reached by the connectorEnd links.
func TestIndexedConnectionKeepsStandardEndpointsAndCarriesTheSelector(t *testing.T) {
	const src = `part def C {
		port p[3];
		port q;
		connect p#(1) to q;
	}`
	doc, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, el := range elements {
		id, _ := el["@id"].(string)
		byID[id] = el
	}
	var conn map[string]any
	for _, el := range elements {
		if el["@type"] == "ConnectionUsage" {
			conn = el
			break
		}
	}
	if conn == nil {
		t.Fatalf("no ConnectionUsage in the API JSON\n%s", doc)
	}
	ends, _ := conn["connectorEnd"].([]any)
	source, ok := conn["sourceFeature"].(map[string]any)
	if !ok || len(ends) == 0 {
		t.Fatalf("connection states no ends or sourceFeature\n%s", doc)
	}
	if source["@id"] != "C__p" {
		t.Errorf("sourceFeature = %v, want the whole port C__p", source["@id"])
	}
	// The end feature the ordered connectorEnd links reach is where the
	// selector lives, beside the ReferenceSubsetting of the whole port.
	end := byID[ends[0].(map[string]any)["@id"].(string)]
	if end["isEnd"] != true || end["sysx:sourceText"] != "p#(1)" {
		t.Errorf("first end = %v, want the end written p#(1)", end)
	}
	index, ok := end["sysx:endElement"].(map[string]any)
	if !ok {
		t.Fatalf("the first end states no sysx:endElement\n%s", doc)
	}
	expr := byID[index["@id"].(string)]
	if expr["sysx:sourceText"] != "1" {
		t.Errorf("index expression = %v, want 1", expr["sysx:sourceText"])
	}
	subsetting, _ := end["ownedReferenceSubsetting"].(map[string]any)
	referenced, _ := byID[subsetting["@id"].(string)]["referencedFeature"].(map[string]any)
	if referenced["@id"] != source["@id"] {
		t.Errorf("the end reference-subsets %v, want the same port as sourceFeature %v", referenced["@id"], source["@id"])
	}

	// A plain end's sourceFeature still points straight at the feature: the
	// selector is what distinguishes the two.
	const plain = `part def D { port a; port b; connect a to b; }`
	plainDoc, err := convert.Convert("m.sysml", []byte(plain), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("plain model to api-json: %v", err)
	}
	if strings.Contains(string(plainDoc), "p#(1)") {
		t.Errorf("a model with no indexed end should not state p#(1)")
	}
	plainByID := map[string]map[string]any{}
	for _, el := range elementsOf(t, plainDoc) {
		id, _ := el["@id"].(string)
		plainByID[id] = el
	}
	plainConnections := 0
	for _, el := range plainByID {
		if el["@type"] != "ConnectionUsage" {
			continue
		}
		plainConnections++
		sf, _ := el["sourceFeature"].(map[string]any)
		target := plainByID[sf["@id"].(string)]
		if _, indexed := target["sysx:endElement"]; indexed {
			t.Errorf("a plain connection's sourceFeature carries an index\n%s", plainDoc)
		}
		if target["@type"] != "PortUsage" || target["declaredName"] != "a" {
			t.Errorf("a plain connection's sourceFeature = %v, want the port a", target)
		}
	}
	if plainConnections != 1 {
		t.Errorf("plain model has %d connections, want 1\n%s", plainConnections, plainDoc)
	}

	// The written form stays byte-identical through both element forms.
	back, err := convert.Convert("m.json", doc, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json did not read back: %v", err)
	}
	if !strings.Contains(string(back), "connect p#(1) to q;") {
		t.Errorf("the API JSON should read back with the indexed end\n%s", back)
	}
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(turtle), "sysml:sourceFeature elmt:"+source["@id"].(string)+" ;") {
		t.Errorf("the Turtle sourceFeature should name the referenced port\n%s", turtle)
	}
	if !strings.Contains(string(turtle), "sysml:connectorEnd expr:"+ends[0].(map[string]any)["@id"].(string)) {
		t.Errorf("the Turtle connectorEnd should reach the indexed end\n%s", turtle)
	}
	fromTurtle, err := convert.Convert("m.ttl", turtle, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("turtle did not read back: %v", err)
	}
	if !strings.Contains(string(fromTurtle), "connect p#(1) to q;") {
		t.Errorf("the Turtle should read back with the indexed end\n%s", fromTurtle)
	}
}

// The second end keeps its referenced feature on targetFeature too, while the
// ordered connectorEnd links carry both selectors; the head reads back as written.
func TestIndexedConnectionKeepsTheReferencedFeatureOnTargetFeature(t *testing.T) {
	const src = `part def C {
		port p[3];
		port q[3];
		connect p#(1) to q#(2);
	}`
	doc, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	elements := elementsOf(t, doc)
	byID := map[string]map[string]any{}
	for _, el := range elements {
		id, _ := el["@id"].(string)
		byID[id] = el
	}
	var conn map[string]any
	for _, el := range elements {
		if el["@type"] == "ConnectionUsage" {
			conn = el
			break
		}
	}
	if conn == nil {
		t.Fatalf("no ConnectionUsage\n%s", doc)
	}
	ends, _ := conn["connectorEnd"].([]any)
	if len(ends) != 2 {
		t.Fatalf("connectorEnd = %v, want two ends", ends)
	}
	targets, _ := conn["targetFeature"].([]any)
	if len(targets) != 1 {
		t.Fatalf("targetFeature = %v, want one indexed end", targets)
	}
	got, _ := targets[0].(map[string]any)["@id"].(string)
	if got != "C__q" {
		t.Errorf("targetFeature = %v, want the whole port C__q", got)
	}
	end := byID[ends[1].(map[string]any)["@id"].(string)]
	if end["sysx:sourceText"] != "q#(2)" {
		t.Errorf("second end = %v, want q#(2)", end)
	}
	index, _ := end["sysx:endElement"].(map[string]any)
	if expr := byID[index["@id"].(string)]; expr["sysx:sourceText"] != "2" {
		t.Errorf("target index expression = %v, want 2", expr["sysx:sourceText"])
	}
	back, err := convert.Convert("m.json", doc, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("api-json did not read back: %v", err)
	}
	if !strings.Contains(string(back), "connect p#(1) to q#(2);") {
		t.Errorf("the API JSON should read back with both indexed ends\n%s", back)
	}
}

// A derived connector property naming an undefined expression IRI must be
// refused; the narrow end-feature exception is only for graph-defined ends.
func TestIndexedConnectionRefusesAnUndefinedEndReference(t *testing.T) {
	const src = `part def C { port p[3]; port q; connect p#(1) to q; }`
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	bad := strings.Replace(string(turtle), "sysml:sourceFeature elmt:C__p ;",
		"sysml:sourceFeature expr:MISSING_END ;", 1)
	if bad == string(turtle) {
		t.Fatalf("the sourceFeature triple was not found\n%s", turtle)
	}
	_, err = convert.Convert("m.ttl", []byte(bad), convert.FormatTurtle, convert.FormatSysML)
	var unsupported *export.UnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("want a typed refusal for the undefined end, got %v", err)
	}
	if !strings.Contains(err.Error(), "MISSING_END") {
		t.Errorf("the refusal should name the undefined end: %v", err)
	}
}

func elementsOf(t *testing.T, doc []byte) []map[string]any {
	t.Helper()
	var elements []map[string]any
	if err := json.Unmarshal(doc, &elements); err != nil {
		t.Fatal(err)
	}
	return elements
}
