package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// acceptModel writes an accept node both ways SysML.xtext AcceptNode allows:
// named through an ActionNodeUsageDeclaration and bare.
const acceptModel = `package Orders {
    item def Order;
    action def Process {
        action receiveOrder accept order : Order;
        accept other : Order;
    }
}
`

func acceptGraph(t *testing.T) (string, *rdf.Graph) {
	t.Helper()
	turtle, err := convert.Convert("orders.sysml", []byte(acceptModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	return string(turtle), g
}

// An accept node is an AcceptActionUsage (SysML.xtext AcceptNode), named or
// not, whose payload is the parameter its payloadParameter names, owned
// through a ParameterMembership (SysML.xtext PayloadParameterMember).
func TestAcceptNodeIsAcceptActionUsage(t *testing.T) {
	_, g := acceptGraph(t)
	for _, tc := range []struct{ node, payload string }{
		{"Orders__Process__receiveOrder", "Orders__Process__receiveOrder__order"},
		{"Orders__Process___401", "Orders__Process___401__other"},
	} {
		node, payload := element(tc.node), element(tc.payload)
		if got := g.Type(node); got != rdf.SysML+"AcceptActionUsage" {
			t.Errorf("%s is a %s, not an AcceptActionUsage", tc.node, got)
		}
		if got, _ := g.Object(node, rdf.SysML+"payloadParameter"); got != payload {
			t.Errorf("%s payloadParameter = %v, want %v", tc.node, got, payload)
		}
		if got, _ := g.Object(node, rdf.SysML+"parameter"); got != payload {
			t.Errorf("%s parameter = %v, want %v", tc.node, got, payload)
		}
		membership, ok := g.Object(payload, rdf.SysML+"owningMembership")
		if !ok {
			t.Fatalf("%s states no owningMembership", tc.payload)
		}
		if got := g.Type(membership); got != rdf.SysML+"ParameterMembership" {
			t.Errorf("%s is owned through a %s, not a ParameterMembership", tc.payload, got)
		}
		if got, _ := g.Object(membership, rdf.SysML+"ownedMemberParameter"); got != payload {
			t.Errorf("the payload's membership ownedMemberParameter = %v, want %v", got, payload)
		}
	}
	if _, stated := g.Object(element("Orders__Process__receiveOrder"), rdf.SysML+"payloadArgument"); stated {
		t.Error("an accept with no `via` and no trigger value states a payloadArgument")
	}
}

// The structure alone writes both forms back as written.
func TestAcceptNodeReadsFromStructureAlone(t *testing.T) {
	turtle, _ := acceptGraph(t)
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range []string{"action receiveOrder accept order : Order;", "accept other : Order;"} {
		if !strings.Contains(back, want) {
			t.Errorf("the graph alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
	if strings.Contains(back, "action accept other") {
		t.Errorf("the bare accept came back with an `action` keyword:\n%s", back)
	}
}

// The API element form types both accept nodes AcceptActionUsage and reads
// back from it with the source text dropped, as from the toolkit's compact
// form, which states neither sysml:isAccept nor payloadParameter.
func TestAcceptNodeAPIJSON(t *testing.T) {
	_, g := acceptGraph(t)
	document, err := export.WriteAPIJSON(g)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	compact := func(raw json.RawMessage) string { return strings.Join(strings.Fields(string(raw)), "") }
	for _, tc := range []struct{ node, payload string }{
		{"Orders__Process__receiveOrder", "Orders__Process__receiveOrder__order"},
		{"Orders__Process___401", "Orders__Process___401__other"},
	} {
		node := apiJSONElement(t, elements, tc.node)
		if got := string(node["@type"]); got != `"AcceptActionUsage"` {
			t.Errorf("%s @type = %s", tc.node, got)
		}
		if got := compact(node["payloadParameter"]); got != `{"@id":"`+tc.payload+`"}` {
			t.Errorf("%s payloadParameter = %s", tc.node, got)
		}
	}
	for i := range elements {
		delete(elements[i], "sysx:sourceText")
		delete(elements[i], "sysx:sourceTail")
	}
	stripped, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("orders.json", stripped, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back from the element form alone: %v", err)
	}
	for _, want := range []string{"action receiveOrder accept order : Order;", "accept other : Order;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the element form alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}

	again, err := convert.Convert("orders.json", toolkitCompact(t, acceptModel), convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("toolkit compact decode: %v", err)
	}
	for _, want := range []string{"action receiveOrder accept order : Order;", "accept other : Order;"} {
		if !strings.Contains(string(again), want) {
			t.Errorf("the toolkit's compact form did not write %q\n--- notation ---\n%s", want, again)
		}
	}
}
