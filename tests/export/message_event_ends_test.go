package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// A `message m from a to b;` owns each end through a ParameterMembership owning
// an EventOccurrenceUsage (SysML.xtext MessageEventMember, MessageEvent), not an
// EndFeatureMembership owning a ReferenceUsage — and the event usages stay parts
// of the flow's head, never members of their own. A `flow` keeps its connector
// ends, and a message without ends comes back as `message` from its keyword.
const messageEndsModel = `package M {
    item def Ping;
    part a {
        part p;
        part x;
    }
    part b {
        part p;
        part x;
    }
    occurrence def Talk {
        message m from a to b;
        message from a.p to b.p;
        message m2 of Ping from a to b;
        message m3;
        flow f from a.x to b.x;
    }
}
`

func TestMessageEndsAreEventOccurrenceUsages(t *testing.T) {
	turtle, back := graphOnlyRoundTrip(t, "m.sysml", []byte(messageEndsModel))
	graph := string(turtle)
	for _, want := range []string{
		"expr:M__Talk__m_pend0\n    a sysml:EventOccurrenceUsage ;",
		"expr:M__Talk__m_pend1\n    a sysml:EventOccurrenceUsage ;",
		"expr:M__Talk__m_pend0_om\n    a sysml:ParameterMembership ;",
		"expr:M__Talk__m_pend1_om\n    a sysml:ParameterMembership ;",
		"expr:M__Talk__m_pend0_prs\n    a sysml:ReferenceSubsetting ;",
		"sysml:ownedMemberParameter expr:M__Talk__m_pend0 .",
		"sysml:parameter expr:M__Talk__m_pend0, expr:M__Talk__m_pend1 ;",
		// A chained end is an event usage the same way, through the same membership.
		"expr:M__Talk___401_pend0\n    a sysml:EventOccurrenceUsage ;",
		"expr:M__Talk___401_pend0_om\n    a sysml:ParameterMembership ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("the graph should state %q:\n%s", want, graph)
		}
	}
	if strings.Contains(graph, "expr:M__Talk__m_pend0\n    a sysml:ReferenceUsage") ||
		strings.Contains(graph, "expr:M__Talk__m_pend0_om\n    a sysml:EndFeatureMembership") {
		t.Errorf("a message end is no ReferenceUsage end feature:\n%s", graph)
	}
	// The `flow` keeps its connector ends: ReferenceUsage ends under
	// EndFeatureMemberships, unchanged by the message shape.
	for _, want := range []string{
		"expr:M__Talk__f_pend0\n    a sysml:ReferenceUsage ;",
		"expr:M__Talk__f_pend0_om\n    a sysml:EndFeatureMembership ;",
	} {
		if !strings.Contains(graph, want) {
			t.Errorf("a flow end should stay a ReferenceUsage end feature %q:\n%s", want, graph)
		}
	}
	notation := string(back)
	for _, want := range []string{
		"message m from a to b;\n",
		"message from a.p to b.p;\n",
		"message m2 of Ping from a to b;\n",
		// A message without ends states no keyword the graph carries, so it
		// reads back as `flow` — the same as before the ends' metaclass fix.
		"flow m3;\n",
		"flow f from a.x to b.x;\n",
	} {
		if !strings.Contains(notation, want) {
			t.Errorf("the notation should contain %q:\n%s", want, notation)
		}
	}
	if strings.Contains(notation, "flow m ") || strings.Contains(notation, "flow m2") ||
		strings.Contains(notation, "flow from a.p") {
		t.Errorf("a message with ends came back as a flow:\n%s", notation)
	}
}

// The API JSON reader must classify the event-usage ends into the expression
// namespace — parts of the flow's head — while a declared `event occurrence x;`
// stays an element of its own; and the JSON carries the same metaclasses.
func TestMessageEndsInAPIJSON(t *testing.T) {
	document, err := convert.Convert("m.sysml", []byte(messageEndsModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to API JSON: %v", err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	typeOf := func(id string) string {
		for _, element := range elements {
			var elementID string
			if err := json.Unmarshal(element["@id"], &elementID); err == nil && elementID == id {
				var typ string
				_ = json.Unmarshal(element["@type"], &typ)
				return typ
			}
		}
		return ""
	}
	for id, want := range map[string]string{
		"M__Talk__m_pend0":    "EventOccurrenceUsage",
		"M__Talk__m_pend1":    "EventOccurrenceUsage",
		"M__Talk__m_pend0_om": "ParameterMembership",
		"M__Talk__m_pend1_om": "ParameterMembership",
		"M__Talk__f_pend0":    "ReferenceUsage",
		"M__Talk__f_pend0_om": "EndFeatureMembership",
	} {
		if typ := typeOf(id); typ != want {
			t.Errorf("API JSON @type of %s = %q, want %q", id, typ, want)
		}
	}
	back, err := convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("API JSON back to notation: %v", err)
	}
	for _, want := range []string{"message m from a to b;\n", "message from a.p to b.p;\n", "flow f from a.x to b.x;\n"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("the notation should contain %q:\n%s", want, back)
		}
	}
	// The ends come back through the head alone: none may be written as members.
	if strings.Contains(string(back), "event occurrence") {
		t.Errorf("a message end was written back as a member:\n%s", back)
	}
}
