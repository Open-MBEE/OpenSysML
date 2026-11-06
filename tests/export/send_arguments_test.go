package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// sendShapesModel writes a send in every form the grammar gives it (SysML.xtext
// SendNode): `via`, `to`, both, no payload, unnamed, in a state's entry and as a
// transition's effect.
const sendShapesModel = `package P {
    item def Alarm;
    part def Monitor {
        port alarmPort;
        part display;
        action supervise {
            action notifyVia send new Alarm() via alarmPort;
            action notifyTo send new Alarm() to display;
            action both send new Alarm() via alarmPort to display;
            action noPayload send via alarmPort;
            send new Alarm() to display;
        }
        state sm {
            entry send new Alarm() to display;
            state idle;
            transition first idle do send new Alarm() via alarmPort then idle;
        }
    }
}
`

func apiJSONByID(t *testing.T, src string) map[string]map[string]any {
	t.Helper()
	out, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
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
	return byID
}

// TestSendActionOwnsItsArgumentsAsParameters: a send owns its payload, sender and
// receiver as `in` ReferenceUsages under ParameterMemberships, each valued by its
// expression, which the send names as payloadArgument, senderArgument and
// receiverArgument (SysML v2 8.3.17.15); a `to` alone leaves an empty sender
// parameter between the payload and the receiver, as the pilot writes it.
func TestSendActionOwnsItsArgumentsAsParameters(t *testing.T) {
	byID := apiJSONByID(t, sendShapesModel)
	sends := map[string]map[string]any{}
	for id, el := range byID {
		if el["@type"] == "SendActionUsage" {
			sends[id] = el
		}
	}
	if len(sends) != 7 {
		t.Fatalf("%d SendActionUsages, want 7: %v", len(sends), sends)
	}
	cases := map[string]struct {
		params                     int
		payload, sender, receiver  string
		valued                     []bool
		senderEmpty, payloadAbsent bool
	}{
		"P__Monitor__supervise__notifyVia": {params: 2, payload: "ConstructorExpression", sender: "FeatureReferenceExpression", valued: []bool{true, true}},
		"P__Monitor__supervise__notifyTo":  {params: 3, payload: "ConstructorExpression", receiver: "FeatureReferenceExpression", valued: []bool{true, false, true}},
		"P__Monitor__supervise__both":      {params: 3, payload: "ConstructorExpression", sender: "FeatureReferenceExpression", receiver: "FeatureReferenceExpression", valued: []bool{true, true, true}},
		"P__Monitor__supervise__noPayload": {params: 2, sender: "FeatureReferenceExpression", valued: []bool{false, true}},
		"P__Monitor__supervise___404":      {params: 3, payload: "ConstructorExpression", receiver: "FeatureReferenceExpression", valued: []bool{true, false, true}},
	}
	for id, want := range cases {
		send := sends[id]
		if send == nil {
			t.Errorf("no SendActionUsage %s among %v", id, sends)
			continue
		}
		params, _ := send["parameter"].([]any)
		owned, _ := send["ownedMember"].([]any)
		if len(params) != want.params || len(owned) != want.params {
			t.Errorf("%s parameter %v, ownedMember %v; want %d parameters", id, params, owned, want.params)
			continue
		}
		for i, ref := range params {
			param := byID[ref.(map[string]any)["@id"].(string)]
			if param["@type"] != "ReferenceUsage" || param["direction"] != "in" || param["isReference"] != true {
				t.Errorf("%s parameter %d = %v, want an `in` ReferenceUsage", id, i, param)
			}
			membership := byID[param["owningRelationship"].(map[string]any)["@id"].(string)]
			if membership["@type"] != "ParameterMembership" {
				t.Errorf("%s parameter %d is owned through a %v, want a ParameterMembership", id, i, membership["@type"])
			}
			value, _ := param["value"].(map[string]any)
			if (value != nil) != want.valued[i] {
				t.Errorf("%s parameter %d value = %v, want valued %t", id, i, value, want.valued[i])
			}
		}
		for property, metaclass := range map[string]string{"payloadArgument": want.payload, "senderArgument": want.sender, "receiverArgument": want.receiver} {
			argument, stated := send[property].(map[string]any)
			if metaclass == "" {
				if stated {
					t.Errorf("%s states %s = %v, want none", id, property, argument)
				}
				continue
			}
			if !stated {
				t.Errorf("%s states no %s, want a %s", id, property, metaclass)
				continue
			}
			if got := byID[argument["@id"].(string)]["@type"]; got != metaclass {
				t.Errorf("%s %s is a %v, want a %s", id, property, got, metaclass)
			}
			position := map[string]int{"payloadArgument": 0, "senderArgument": 1, "receiverArgument": 2}[property]
			param := byID[params[position].(map[string]any)["@id"].(string)]
			if value, _ := param["value"].(map[string]any); value == nil || value["@id"] != argument["@id"] {
				t.Errorf("%s %s is not the value of parameter %d: %v", id, property, position, param["value"])
			}
		}
	}
}

// TestSendShapesRoundTripThroughTurtle: every send shape comes back from its
// Turtle, and from the graph alone once the source text is removed; the send's
// arguments travel as its parameters, not as private expression properties.
func TestSendShapesRoundTripThroughTurtle(t *testing.T) {
	turtle, err := convert.Convert("m.sysml", []byte(sendShapesModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"sysx:receiver", "sysx:isVia", "sysx:payload"} {
		if strings.Contains(string(turtle), private) {
			t.Errorf("the Turtle still writes %s for a send:\n%s", private, turtle)
		}
	}
	for _, property := range []string{"sysml:payloadArgument", "sysml:senderArgument", "sysml:receiverArgument"} {
		if !strings.Contains(string(turtle), property) {
			t.Errorf("the Turtle writes no %s", property)
		}
	}
	want := normalizeNotation(sendShapesModel)
	for name, graph := range map[string][]byte{"with text": turtle, "graph only": withoutTriples(t, turtle, "sysx:sourceText")} {
		back, err := convert.Convert("m.ttl", graph, convert.FormatTurtle, convert.FormatSysML)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := normalizeNotation(string(back)); got != want {
			t.Errorf("%s: round trip changed the model:\n%s\nwant:\n%s", name, got, want)
		}
	}
}

// normalizeNotation folds the layout of a notation text so that two writings of
// one model compare equal.
func normalizeNotation(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
