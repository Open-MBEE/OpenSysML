package export

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

var verbatimEquations = []string{
	"Medium.temperature(state_a)",
	"inStream(port_a.h_outflow)",
	"noEvent(if abs(m_flow) > 1.0 then m_flow else 1.0)",
}

// verbatimEquationNotation is the steam-generator constraint body carried in
// the golden AST and the execution conformance fixtures.
const verbatimEquationNotation = `package ModelicaMapping {
    private import ScalarValues::*;
    metadata def EquationMapping { attribute body : String; }
}
package Steam {
    private import ScalarValues::*;
    private import ModelicaMapping::*;
    part def SteamGenerator {
        attribute state_a;
        attribute m_flow : Real = 2.0;
        port port_a;
        port port_b;
        constraint steamEquations {
            @EquationMapping { body = "Medium.temperature(state_a)"; }
            @EquationMapping { body = "inStream(port_a.h_outflow)"; }
            @EquationMapping { body = "noEvent(if abs(m_flow) > 1.0 then m_flow else 1.0)"; }
            m_flow > 0.0
        }
    }
}
`

// jsonStrings collects every string in a decoded API-JSON document, so a
// comparison is of the value itself and not of an encoder's choice of escaping
// for '<', '>' and '&' in a `body` that carries an equation.
func jsonStrings(v any, into map[string]bool) {
	switch x := v.(type) {
	case string:
		into[x] = true
	case []any:
		for _, e := range x {
			jsonStrings(e, into)
		}
	case map[string]any:
		for _, e := range x {
			jsonStrings(e, into)
		}
	}
}

func assertCarriesEquations(t *testing.T, notation string) {
	t.Helper()
	graph := graphOfNotation(t, notation)

	// The api-json element form states each equation as the value of its binding.
	api, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(api, &decoded); err != nil {
		t.Fatalf("api-json is not one document: %v", err)
	}
	values := map[string]bool{}
	jsonStrings(decoded, values)
	for _, want := range verbatimEquations {
		if !values[want] {
			t.Errorf("api-json does not carry %q verbatim", want)
		}
	}

	// The RDF graph states each equation as a sysml:value literal, unescaped.
	turtle := string(rdf.WriteTurtle(graph))
	for _, want := range verbatimEquations {
		if !strings.Contains(turtle, "sysml:value "+strconv.Quote(want)) {
			t.Errorf("RDF does not carry %q verbatim as a sysml:value literal", want)
		}
		found := false
		for _, tr := range graph.Triples() {
			if tr.Predicate.Value == rdf.SysML+pValue && tr.Object.IsLiteral() && tr.Object.Value == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("RDF graph has no sysml:value literal %q", want)
		}
	}

	// Reading the graph back writes the same equation text into the notation.
	back, err := ToSysML(graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range verbatimEquations {
		if !strings.Contains(string(back), want) {
			t.Errorf("notation read back from the graph lost %q:\n%s", want, back)
		}
	}
}

func TestVerbatimEquationCarriesThroughAPIJSONAndRDF(t *testing.T) {
	assertCarriesEquations(t, verbatimEquationNotation)
}

// A verbatim equation is text, not a SysML expression: quotes, backslashes,
// ampersands, angle brackets, newlines and non-ASCII all survive both writers
// and the graph-to-notation reader unchanged.
func TestVerbatimEquationHostileStringsRoundtrip(t *testing.T) {
	hostile := []string{
		`a = "quoted" \ backslash & <tag> 'single' 100%` + "\n" + `second = °C µ → ≤ ≥`,
		`b = inStream(p) ? noEvent(x) : Medium.temperature(s)`,
	}
	notation := `package M {
    private import ScalarValues::*;
    metadata def EquationMapping { attribute body : String; }
    part def S {
        @EquationMapping { body = ` + strconv.Quote(hostile[0]) + `; }
        @EquationMapping { body = ` + strconv.Quote(hostile[1]) + `; }
    }
}
`
	graph := graphOfNotation(t, notation)

	api, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(api, &decoded); err != nil {
		t.Fatalf("api-json is not one document: %v", err)
	}
	values := map[string]bool{}
	jsonStrings(decoded, values)
	for _, want := range hostile {
		if !values[want] {
			t.Errorf("api-json does not carry %q verbatim", want)
		}
	}
	turtle := string(rdf.WriteTurtle(graph))
	for _, want := range hostile {
		if !strings.Contains(turtle, "sysml:value "+strconv.Quote(want)) {
			t.Errorf("RDF does not carry %q verbatim", want)
		}
	}
	back, err := ToSysML(graph)
	if err != nil {
		t.Fatal(err)
	}
	// The written notation is a model again, not just text that happens to
	// match: compare the values a second export carries, since a newline in a
	// string is written as the two-character escape `\n` in notation.
	file := source.New("roundtrip.sysml", back)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("notation read back from the graph does not parse: %v", p.Diagnostics)
	}
	second, err := ToRDF(file, root)
	if err != nil {
		t.Fatal(err)
	}
	again, err := WriteAPIJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	var decodedAgain any
	if err := json.Unmarshal(again, &decodedAgain); err != nil {
		t.Fatal(err)
	}
	valuesAgain := map[string]bool{}
	jsonStrings(decodedAgain, valuesAgain)
	for _, want := range hostile {
		if !valuesAgain[want] {
			t.Errorf("the value of %q did not survive notation roundtrip", want)
		}
	}
}

// A body naming functions and packages the model never declares is still a
// clean model: the annotation is a verbatim carrier, and the engine does not
// resolve or evaluate its text as an expression.
func TestVerbatimEquationBodyIsNotAnExpression(t *testing.T) {
	file := source.New("verbatim.sysml", []byte(verbatimEquationNotation))
	p := parser.New(file)
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("a verbatim equation body must not be resolved as an expression: %v", p.Diagnostics)
	}
}

// TestVerbatimEquationExpressionRouteKnownLimitation records the language
// surface the annotation route stands in for. Writing the Modelica forms as
// constraint-body expressions — a medium state package call
// `Medium.temperature(state_a)`, a stream value `inStream(port_a.h_outflow)`,
// or a regularising `noEvent(if abs(m_flow) > 1.0 then m_flow else 1.0)` — has
// no parse, evaluation or export form at this pin. The engine's conditional
// expression spells the condition with `?`, not Modelica's `then`, and neither
// `inStream` nor `noEvent` is a builtin. This test is skipped rather than
// asserted green: unskipping it shows the exact gap the annotation carries.
func TestVerbatimEquationExpressionRouteKnownLimitation(t *testing.T) {
	t.Skip("the full Modelica expression form is not implemented; use the verbatim EquationMapping annotation")

	const direct = `package Steam {
    private import ScalarValues::*;
    part def SteamGenerator {
        attribute state_a;
        attribute m_flow : Real = 2.0;
        port port_a;
        port port_b;
        constraint steamEquations {
            Medium.temperature(state_a);
            inStream(port_a.h_outflow);
            noEvent(if abs(m_flow) > 1.0 then m_flow else 1.0)
        }
    }
}
`
	file := source.New("direct.sysml", []byte(direct))
	p := parser.New(file)
	p.ParseFile()
	if len(p.Diagnostics) == 0 {
		t.Fatal("the full expression form became available: replace this skipped test with a conformance case")
	}
}
