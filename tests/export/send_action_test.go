package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// sendModel writes a send node both ways SysML.xtext SendNode allows: named
// through an ActionNodeUsageDeclaration and bare.
const sendModel = `package Orders {
    item def Receipt;
    part def Target;
    action def Process {
        in item receipt : Receipt;
        in part target : Target;
        action sendReceipt send receipt to target;
        send receipt to target;
    }
}
`

var sendNodes = []string{"Orders__Process__sendReceipt", "Orders__Process___403"}

func sendGraph(t *testing.T) (string, *rdf.Graph) {
	t.Helper()
	turtle, err := convert.Convert("orders.sysml", []byte(sendModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	g, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatal(err)
	}
	return string(turtle), g
}

// A send node is one SendActionUsage (SysML.xtext SendNode), named or not: the
// named form is not an ActionUsage owning a nameless send.
func TestSendNodeIsOneSendActionUsage(t *testing.T) {
	_, g := sendGraph(t)
	for _, id := range sendNodes {
		node := element(id)
		if got := g.Type(node); got != rdf.SysML+"SendActionUsage" {
			t.Errorf("%s is a %s, not a SendActionUsage", id, got)
		}
		for _, property := range []string{"payload", "receiver"} {
			if !g.HasProperty(node, rdf.OpenSysML+property) {
				t.Errorf("%s states no sysx:%s", id, property)
			}
		}
		for _, owned := range g.Objects(node, rdf.SysML+"ownedMember") {
			if g.Type(owned) == rdf.SysML+"SendActionUsage" {
				t.Errorf("%s owns a SendActionUsage %s of its own", id, owned.Value)
			}
		}
	}
	if got, _ := g.Object(element(sendNodes[0]), rdf.SysML+"declaredName"); got != rdf.String("sendReceipt") {
		t.Errorf("the named send's declaredName = %v", got)
	}
	if g.HasProperty(element(sendNodes[1]), rdf.SysML+"declaredName") {
		t.Error("the bare send declares a name")
	}
}

// The structure alone writes both forms back as written.
func TestSendNodeReadsFromStructureAlone(t *testing.T) {
	turtle, _ := sendGraph(t)
	back := backFromTheGraphAlone(t, turtle)
	for _, want := range []string{"action sendReceipt send receipt to target;", "\n        send receipt to target;"} {
		if !strings.Contains(back, want) {
			t.Errorf("the graph alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
	if strings.Contains(back, "action sendReceipt {") {
		t.Errorf("the named send came back as an action owning a send:\n%s", back)
	}
}

// The API element form types both send nodes SendActionUsage and reads back
// from it with the source text dropped.
func TestSendNodeAPIJSON(t *testing.T) {
	_, g := sendGraph(t)
	document, err := export.WriteAPIJSON(g)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	for _, id := range sendNodes {
		node := apiJSONElement(t, elements, id)
		if got := string(node["@type"]); got != `"SendActionUsage"` {
			t.Errorf("%s @type = %s", id, got)
		}
	}
	if got := string(apiJSONElement(t, elements, sendNodes[0])["declaredName"]); got != `"sendReceipt"` {
		t.Errorf("the named send's declaredName = %s", got)
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
	for _, want := range []string{"action sendReceipt send receipt to target;", "\n        send receipt to target;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the element form alone did not write %q\n--- notation ---\n%s", want, back)
		}
	}
}
