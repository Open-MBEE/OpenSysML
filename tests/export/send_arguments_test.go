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

// sendBodiesModel writes sends whose body declares a member of their own (SysML.xtext
// SendNode ends in an ActionBody), named and not.
const sendBodiesModel = `package P {
    item def Alarm;
    part def Monitor {
        part display;
        action supervise {
            action delayed send new Alarm() to display { in delay : Integer = 50; }
            send new Alarm() to display { in pause : Integer = 5; }
        }
    }
}
`

// TestSendBodiesBelongToTheSend: a send with a body is one SendActionUsage owning its
// three parameters and then the body's member, as the symbol table classifies it, and
// the graph alone reads the body back.
func TestSendBodiesBelongToTheSend(t *testing.T) {
	byID := apiJSONByID(t, sendBodiesModel)
	var sends []map[string]any
	for _, el := range byID {
		if el["@type"] == "SendActionUsage" {
			sends = append(sends, el)
		}
	}
	if len(sends) != 2 {
		t.Fatalf("%d SendActionUsages, want the named and the unnamed send", len(sends))
	}
	for _, send := range sends {
		members, _ := send["ownedMember"].([]any)
		if len(members) != 4 {
			t.Fatalf("send %v owns %d members, want three parameters and the body's", send["declaredName"], len(members))
		}
		last := byID[members[3].(map[string]any)["@id"].(string)]
		if name, _ := last["declaredName"].(string); name != "delay" && name != "pause" {
			t.Errorf("send %v's last owned member is %v, want the body's parameter", send["declaredName"], last["declaredName"])
		}
		if params, _ := send["parameter"].([]any); len(params) != 3 {
			t.Errorf("send %v parameter = %v, want the payload, sender and receiver", send["declaredName"], params)
		}
	}
	turtle, err := convert.Convert("m.sysml", []byte(sendBodiesModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("m.ttl", withoutTriples(t, turtle, "sysx:sourceText"), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := normalizeNotation(string(back)), normalizeNotation(sendBodiesModel); got != want {
		t.Errorf("the graph alone changed the model:\n%s\nwant:\n%s", got, want)
	}
}

// TestSendArgumentsReadFromTheirParameters: a send's ordered parameters carry its
// arguments when the graph states no payloadArgument, senderArgument or
// receiverArgument, and a stated one naming another value than its parameter's
// is refused rather than one of them dropped.
func TestSendArgumentsReadFromTheirParameters(t *testing.T) {
	turtle, err := convert.Convert("m.sysml", []byte(sendShapesModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	graph := withoutTriples(t, turtle, "sysx:sourceText")
	for _, property := range []string{"sysml:payloadArgument", "sysml:senderArgument", "sysml:receiverArgument"} {
		graph = withoutTriples(t, graph, property)
	}
	back, err := convert.Convert("m.ttl", graph, convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("without the argument links: %v", err)
	}
	if got, want := normalizeNotation(string(back)), normalizeNotation(sendShapesModel); got != want {
		t.Errorf("the parameters alone changed the model:\n%s\nwant:\n%s", got, want)
	}
	stripped := string(withoutTriples(t, turtle, "sysx:sourceText"))
	from := "sysml:receiverArgument expr:P__Monitor__supervise__notifyTo___402_pvalue"
	if !strings.Contains(stripped, from) {
		t.Fatalf("the graph no longer states %q:\n%s", from, stripped)
	}
	conflicting := strings.Replace(stripped, from, "sysml:receiverArgument expr:P__Monitor__supervise__notifyVia___401_pvalue", 1)
	if _, err := convert.Convert("m.ttl", []byte(conflicting), convert.FormatTurtle, convert.FormatSysML); err == nil || !strings.Contains(err.Error(), "is not the value of its parameter") {
		t.Errorf("a receiverArgument that is not the receiver parameter's value should be refused, got %v", err)
	}
}

// sendBodyParameterModel declares a send's parameters in its body where the head
// writes no payload: the body's `in :>> payload` is the body's, not a head parameter.
const sendBodyParameterModel = `package SendBodyParameters {
    item def Alarm;
    part def Display;
    action def Monitor {
        attribute s : Alarm;
        part display : Display;
        action snd send { in redefines payload = s; }
        action notify send { in payload; in sender = display; }
    }
}
`

func TestSendBodyRedefinedParametersRoundTrip(t *testing.T) {
	turtle, err := convert.Convert("m.sysml", []byte(sendBodyParameterModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	want := normalizeNotation(sendBodyParameterModel)
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
