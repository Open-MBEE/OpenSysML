package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A send or assignment node declared with a name is one element of the
// statement's metaclass carrying that name (SysML.xtext SendNode,
// AssignmentNode), as the unnamed statement is: a succession or reference to
// the name reaches the send itself, not an action wrapping it.
func TestNamedActionNodeIsTheStatement(t *testing.T) {
	src := `package Orders {
    item def Receipt;
    part def Target;
    action def Process {
        in item receipt : Receipt;
        in part target : Target;
        attribute count = 0;
        action sendReceipt send receipt to target;
        send receipt to target;
        action recount assign count := 1;
        first sendReceipt then recount;
    }
}
`
	out, err := convert.Convert("orders.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, el := range elements {
		byID[el["@id"].(string)] = el
	}
	for id, metaclass := range map[string]string{
		"Orders__Process__sendReceipt": "SendActionUsage",
		"Orders__Process__recount":     "AssignmentActionUsage",
	} {
		el, ok := byID[id]
		if !ok {
			t.Fatalf("%s is not written", id)
		}
		if el["@type"] != metaclass {
			t.Errorf("%s is a %v, want %s", id, el["@type"], metaclass)
		}
		for _, other := range elements {
			if owner, _ := other["owner"].(map[string]any); owner != nil && owner["@id"] == id && other["@type"] == metaclass {
				t.Errorf("%s owns another %s, %v", id, metaclass, other["@id"])
			}
		}
	}
	sends := 0
	for _, el := range elements {
		if el["@type"] == "SendActionUsage" {
			sends++
		}
	}
	if sends != 2 {
		t.Errorf("%d SendActionUsages, want one per send", sends)
	}
}

// A node whose declaration says more than its visibility and name keeps the
// action it is declared as, whose head writes those clauses back: the typing
// of `action a : Act assign …` and the prefix of `#Tag action s send …`
// survive a hop through the graph alone.
func TestRicherActionNodeKeepsItsDeclaration(t *testing.T) {
	src := "package P {\n    part x;\n    part y;\n    action def Act;\n    metadata def Tag;\n    action def A {\n        attribute v = 0;\n        action a : Act assign v := 1;\n        #Tag action s send x to y;\n    }\n}\n"
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	back, err := convert.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, head := range []string{"action a : Act", "#Tag action s"} {
		if !strings.Contains(string(back), head) {
			t.Errorf("the head %q did not come back:\n%s", head, back)
		}
	}
}
