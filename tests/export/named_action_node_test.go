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

// A node whose declaration says more than its name is still one element of its node's metaclass
// (SysML.xtext ActionNodePrefix), on Turtle and the API's JSON, and its head comes back from the graph alone.
func TestRicherActionNodeIsOneElementOfItsMetaclass(t *testing.T) {
	src := `package P {
    part def PD { attribute w; }
    part p : PD;
    part x;
    part y;
    attribute def Sig;
    action def Act;
    action base : Act;
    metadata def Tag;
    action def A {
        action typedAssign : Act assign p.w := 1;
        action multAssign[1] assign p.w := 1;
        action subsetAssign :> base assign p.w := 1;
        action redefAssign :>> base assign p.w := 1;
        #Tag action taggedAssign[1] : Act assign p.w := 1;
        abstract action abstractAssign assign p.w := 1;
        action boundAssign : Act = base assign p.w := 1;
        action typedIf : Act if true { assign p.w := 1; } else { assign p.w := 2; }
        action typedWhile : Act while true { assign p.w := 1; }
        action typedFor : Act for i in (1, 2) { assign p.w := i; }
        action typedSend : Act send Sig() via p to x;
        #Tag action taggedSend send x to y;
    }
}
`
	nodes := map[string]string{
		"typedAssign":    "AssignmentActionUsage",
		"multAssign":     "AssignmentActionUsage",
		"subsetAssign":   "AssignmentActionUsage",
		"redefAssign":    "AssignmentActionUsage",
		"taggedAssign":   "AssignmentActionUsage",
		"abstractAssign": "AssignmentActionUsage",
		"boundAssign":    "AssignmentActionUsage",
		"typedIf":        "IfActionUsage",
		"typedWhile":     "WhileLoopActionUsage",
		"typedFor":       "ForLoopActionUsage",
		"typedSend":      "SendActionUsage",
		"taggedSend":     "SendActionUsage",
	}
	out, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, el := range elements {
		id, _ := el["@id"].(string)
		name, isNode := strings.CutPrefix(id, "P__A__")
		if metaclass, ok := nodes[name]; isNode && ok {
			found[name] = true
			if el["@type"] != metaclass {
				t.Errorf("%s is a %v, want %s", name, el["@type"], metaclass)
			}
		}
		owner, _ := el["owner"].(map[string]any)
		if owner == nil {
			continue
		}
		ownerID, _ := owner["@id"].(string)
		if name, ok := strings.CutPrefix(ownerID, "P__A__"); ok && nodes[name] != "" && el["@type"] == nodes[name] {
			t.Errorf("%s owns a second %s, %s", name, nodes[name], id)
		}
	}
	for name := range nodes {
		if !found[name] {
			t.Errorf("%s is not written", name)
		}
	}
	for _, typed := range []string{"typedAssign", "typedIf", "typedWhile", "typedFor", "typedSend"} {
		for _, el := range elements {
			if el["@id"] != "P__A__"+typed {
				continue
			}
			types, _ := el["type"].([]any)
			if len(types) != 1 || types[0].(map[string]any)["@id"] != "P__Act" {
				t.Errorf("%s is typed by %v, want P::Act", typed, el["type"])
			}
		}
	}

	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for name, metaclass := range nodes {
		if !strings.Contains(string(turtle), "elmt:P__A__"+name+"\n    a sysml:"+metaclass+" ;") {
			t.Errorf("the Turtle does not state %s as a %s", name, metaclass)
		}
	}
	stripped := withoutTriples(t, turtle, "sysx:sourceText")
	back, err := convert.Convert("m.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, head := range []string{
		"action typedAssign : Act assign p.w := 1;",
		"action multAssign[1] assign p.w := 1;",
		"action subsetAssign subsets base assign p.w := 1;",
		"action redefAssign redefines base assign p.w := 1;",
		"#Tag action taggedAssign[1] : Act assign p.w := 1;",
		"abstract action abstractAssign assign p.w := 1;",
		"action boundAssign : Act = base assign p.w := 1;",
		"action typedIf : Act if true {",
		"action typedWhile : Act while true {",
		"action typedFor : Act for i in (1, 2) {",
		"action typedSend : Act send Sig() via p to x;",
		"#Tag action taggedSend send x to y;",
	} {
		if !strings.Contains(string(back), head) {
			t.Errorf("the head %q did not come back:\n%s", head, back)
		}
	}
	again, err := convert.Convert("back.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	settled, err := convert.Convert("again.ttl", withoutTriples(t, again, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation again: %v", err)
	}
	if string(settled) != string(back) {
		t.Errorf("the notation written back is not a fixed point:\n%s\nthen\n%s", back, settled)
	}
}

// A named if, while or for node is one element of its node's metaclass (SysML v2
// §8.2.2.17) and a `succession flow` a SuccessionFlowUsage (§8.2.2.14); each round-trips.
func TestNamedControlNodesAndSuccessionFlowKeepTheirMetaclass(t *testing.T) {
	src := `package P {
    private import ScalarValues::*;
    item def Fuel;
    part def Tank { out item fuelOut : Fuel; }
    part def Engine { in item fuelIn : Fuel; }
    action def A {
        attribute count : Integer;
        action ifIt if count > 0 { action body1; }
        action whileIt while count < 3 { action body2; }
        action forIt for i : Integer in (1, 2, 3) { action body3; }
    }
    part def Vehicle {
        part tank : Tank;
        part engine : Engine;
        flow f of Fuel from tank.fuelOut to engine.fuelIn;
        succession flow sf of Fuel from tank.fuelOut to engine.fuelIn;
    }
}
`
	out, err := convert.Convert("nodes.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
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
		"P__A__ifIt":     "IfActionUsage",
		"P__A__whileIt":  "WhileLoopActionUsage",
		"P__A__forIt":    "ForLoopActionUsage",
		"P__Vehicle__f":  "FlowUsage",
		"P__Vehicle__sf": "SuccessionFlowUsage",
	} {
		el, ok := byID[id]
		if !ok {
			t.Fatalf("%s is not written", id)
		}
		if el["@type"] != metaclass {
			t.Errorf("%s is a %v, want %s", id, el["@type"], metaclass)
		}
	}
	ttl, err := convert.Convert("nodes.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("nodes.ttl", ttl, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"action ifIt if count > 0",
		"action whileIt while count < 3",
		"action forIt for i : Integer in (1, 2, 3)",
		"succession flow sf of Fuel from tank.fuelOut to engine.fuelIn;",
		"flow f of Fuel from tank.fuelOut to engine.fuelIn;",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("round trip lacks %q:\n%s", want, back)
		}
	}
}

// A graph written before the value parameter was states an assignment's value as sysml:value
// on the assignment: it is the operand after `:=`, not a binding in the node's declaration.
func TestAssignmentNodeValueStatedOnTheNodeIsItsOperand(t *testing.T) {
	const src = `package P {
	part def PD { attribute w; }
	part p : PD;
	action def Act;
	action def A {
		action kinded : Act assign p.w := 1;
		action named assign p.w := 2;
		assign p.w := 3;
	}
}
`
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	older := strings.ReplaceAll(string(withoutTriples(t, turtle, "sysx:sourceText")), "sysx:assignedValue ", "sysml:value ")
	if strings.Count(older, "sysml:value ") < 3 {
		t.Fatalf("the graph does not state the three values on their assignments:\n%s", older)
	}
	back, err := convert.Convert("m.ttl", []byte(older), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, statement := range []string{
		"        action kinded : Act assign p.w := 1;\n",
		"        action named assign p.w := 2;\n",
		"        assign p.w := 3;\n",
	} {
		if !strings.Contains(string(back), statement) {
			t.Errorf("the statement %q did not come back:\n%s", statement, back)
		}
	}
}

// A node taking no body of its own (an assignment, a conditional, a loop) whose id an ElementId annotation
// declares comes back from the graph with that annotation about it, in the body it is a member of.
func TestBodilessActionNodeKeepsItsDeclaredID(t *testing.T) {
	const id = `metadata : IdentityMetadata::ElementId about n { id = "11111111-2222-3333-4444-555555555555"; }`
	for _, tc := range []struct{ name, node, metaclass string }{
		{"typed assignment", "action n : Act assign p.w := 1;", "AssignmentActionUsage"},
		{"typed conditional", "action n : Act if true {\n            action x;\n        }", "IfActionUsage"},
		{"typed while loop", "action n : Act while true {\n            action x;\n        }", "WhileLoopActionUsage"},
		{"typed for loop", "action n : Act for i in (1, 2) {\n            action x;\n        }", "ForLoopActionUsage"},
		{"multiplicity assignment", "action n[1] assign p.w := 1;", "AssignmentActionUsage"},
		{"name-only assignment", "action n assign p.w := 1;", "AssignmentActionUsage"},
		{"name-only conditional", "action n if true {\n            action x;\n        }", "IfActionUsage"},
		{"name-only while loop", "action n while true {\n            action x;\n        }", "WhileLoopActionUsage"},
		{"typed send", "action n : Act send new Sig() to p;", "SendActionUsage"},
		{"name-only send", "action n send new Sig() to p;", "SendActionUsage"},
		{"typed send via a port", "action n : Act send new Sig() via p.pt;", "SendActionUsage"},
		{"typed send via and to", "action n : Act send new Sig() via p.pt to p;", "SendActionUsage"},
		{"name-only send via a port", "action n send new Sig() via p.pt;", "SendActionUsage"},
		{"typed send with a body", "action n : Act send new Sig() to p {\n            action inner;\n        }", "SendActionUsage"},
		{"in a then chain", "first start;\n        then action n : Act assign p.w := 1;\n        then done;", "AssignmentActionUsage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P {\n    @IdentityMetadata::ProjectRef { projectId = \"proj-1\"; }\n" +
				"    item def Sig;\n    port def PT;\n    part def PD { attribute w; port pt : PT; }\n    part p : PD;\n    action def Act;\n    action def A {\n        " +
				tc.node + "\n        " + id + "\n    }\n}\n"
			turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			stated := "<urn:sysmlv2:element:11111111-2222-3333-4444-555555555555>\n    a sysml:" + tc.metaclass + " ;"
			if !strings.Contains(string(turtle), stated) {
				t.Fatalf("the graph does not state the node as one %s with its declared id:\n%s", tc.metaclass, turtle)
			}
			stripped := withoutTriples(t, turtle, "sysx:sourceText")
			back, err := convert.Convert("m.ttl", stripped, convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			for _, want := range []string{"        " + id + "\n", "        " + tc.node + "\n"} {
				if !strings.Contains(string(back), want) {
					t.Errorf("the notation does not write %q:\n%s", want, back)
				}
			}
			again, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("the notation written back does not convert: %v\n%s", err, back)
			}
			if !strings.Contains(string(again), stated) {
				t.Errorf("the node lost its metaclass or declared id on the way back:\n%s", again)
			}
			second, err := convert.Convert("m.ttl", withoutTriples(t, again, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("second pass: %v", err)
			}
			if string(second) != string(back) {
				t.Errorf("the notation is not a fixed point:\n%s\nthen\n%s", back, second)
			}
		})
	}
}

// A member of a named conditional's branch is named through the statement the node is written as,
// which has a place in the name and no element, so a reference to it is written back and checked.
func TestReferenceInsideANamedConditionalConvertsBack(t *testing.T) {
	for _, tc := range []struct{ name, node string }{
		{"typed", "action n : Act if true {\n            action k;\n            action j subsets k;\n        }"},
		{"name-only", "action n if true {\n            action k;\n            action j subsets k;\n        }"},
		{"typed with an else", "action n : Act if true {\n            action k;\n        } else {\n            action k;\n            action j subsets k;\n        }"},
		{"typed, an annotation about a member node", "action n : Act if true {\n            assign p.w := 1;\n            action k assign p.w := 2;\n" +
			"            metadata IdentityMetadata::ElementId about k {\n                id = \"11111111-2222-3333-4444-555555555555\";\n            }\n        }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P {\n    part def PD { attribute w; }\n    part p : PD;\n    action def Act;\n    action def A {\n        " + tc.node + "\n    }\n}\n"
			turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("to turtle: %v", err)
			}
			back, err := convert.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("back to notation: %v", err)
			}
			if !strings.Contains(string(back), "        "+tc.node+"\n") {
				t.Errorf("the notation does not write the node as declared:\n%s", back)
			}
			again, err := convert.Convert("m.sysml", back, convert.FormatSysML, convert.FormatTurtle)
			if err != nil {
				t.Fatalf("the notation written back does not convert: %v\n%s", err, back)
			}
			second, err := convert.Convert("m.ttl", withoutTriples(t, again, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
			if err != nil {
				t.Fatalf("second pass: %v", err)
			}
			if string(second) != string(back) {
				t.Errorf("the notation is not a fixed point:\n%s\nthen\n%s", back, second)
			}
		})
	}
}
