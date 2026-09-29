package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

const flowPayloadModel = `package P {
    attribute def Prio;
    item def Fuel;
    part def A { out item i : Fuel; }
    part def B { in item j : Fuel; }
    part def Ctx {
        part x : A;
        part y : B;
        flow of Fuel from x.i to y.j;
        flow of Fuel[1] from x.i to y.j;
        flow f of fuel : Fuel from x.i to y.j {
            attribute w : Prio;
        }
        flow m of cmd : Prio from x.i to y.j;
        flow of cmd : Prio = m.cmd from x.i to y.j;
    }
}
`

// A flow's `of` clause is a PayloadFeature the flow owns through a
// FeatureMembership (SysML-textual-bnf FlowPayloadFeatureMember): typed by T
// for `of T`, and the declared feature for `of p : T`, which `m.cmd` reaches.
// Every form comes back from the graph alone as written.
func TestFlowPayloadIsAPayloadFeature(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "p.sysml", []byte(flowPayloadModel))
	graph := string(turtle)
	for _, want := range []string{
		"elmt:P__Ctx___402___400\n    a sysml:PayloadFeature ;",
		"elmt:P__Ctx___402___400_om\n    a sysml:FeatureMembership ;",
		"elmt:P__Ctx___402___400_ft0\n    a sysml:FeatureTyping ;",
		"elmt:P__Ctx__f__fuel\n    a sysml:PayloadFeature ;",
		"elmt:P__Ctx__f__fuel_om\n    a sysml:FeatureMembership ;",
		"sysml:ownedMemberFeature elmt:P__Ctx__f__fuel ;",
		"elmt:P__Ctx__m__cmd\n    a sysml:PayloadFeature ;",
		"sysml:targetFeature elmt:P__Ctx__m__cmd ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	if strings.Contains(graph, "sysx:payload") {
		t.Errorf("a flow's payload is still the expression sysx:payload:\n%s", graph)
	}
	notation := string(back)
	for _, want := range []string{
		"flow of Fuel from x.i to y.j;\n",
		"flow of Fuel[1] from x.i to y.j;\n",
		"flow f of fuel : Fuel from x.i to y.j {\n",
		"flow m of cmd : Prio from x.i to y.j;\n",
		"flow of cmd : Prio = m.cmd from x.i to y.j;\n",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "attribute fuel") || strings.Contains(notation, "attribute cmd") {
		t.Errorf("a payload came back as a body member:\n%s", notation)
	}
}

// The API's JSON form carries the payload feature under its FeatureMembership,
// and reads back to the same notation.
func TestFlowPayloadInAPIJSON(t *testing.T) {
	doc, err := convert.Convert("p.sysml", []byte(flowPayloadModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	for _, want := range []string{
		`"@type": "PayloadFeature"`,
		`"ownedMemberFeature": {`,
	} {
		if !strings.Contains(string(doc), want) {
			t.Errorf("the API JSON should contain %s", want)
		}
	}
	back, err := convert.Convert("p.json", doc, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back from api-json: %v", err)
	}
	for _, want := range []string{
		"flow of Fuel[1] from x.i to y.j;",
		"flow of cmd : Prio = m.cmd from x.i to y.j;",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation should contain %q:\n%s", want, back)
		}
	}
}
