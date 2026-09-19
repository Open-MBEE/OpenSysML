package model

import (
	"strings"
	"testing"
)

// flowPayloadPrelude declares the ends a message connects, so the cases below
// differ only in their `of` clause.
const flowPayloadPrelude = `item def FuelCommand;
	part def Endpoint {
		event occurrence sent;
		event occurrence received;
	}
`

// TestDeclaredFlowPayloadIsAMember covers the payload feature a flow (message)
// declares in its `of` clause: it is a member of the flow, so the `of` name
// itself resolves and later feature chains reach it.
func TestDeclaredFlowPayloadIsAMember(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"declaration in the of clause", `
			message m of fuelCommand : FuelCommand
				from sender.sent to receiver.received;`},
		{"payload reached through the flow", `
			message m of fuelCommand : FuelCommand
				from sender.sent to receiver.received;
			message n of fuelCommand : FuelCommand = m.fuelCommand
				from sender.sent to receiver.received;`},
		{"declaration alongside a flow body", `
			message m of fuelCommand : FuelCommand
				from sender.sent to receiver.received {
				attribute note;
			}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "package P {\n\t" + flowPayloadPrelude + `
			occurrence def I {
				ref part sender : Endpoint;
				ref part receiver : Endpoint;
` + tc.body + "\n\t\t\t}\n\t\t}"
			if got := diagnose(t, "flow_payload", src); len(got) != 0 {
				t.Fatalf("unexpected findings: %s", strings.Join(got, "; "))
			}
		})
	}
}

// TestFlowPayloadReferenceStillResolvesOutward covers the other `of` form: a
// payload that names an existing element resolves in the enclosing scope, and
// an undeclared one is still reported.
func TestFlowPayloadReferenceStillResolvesOutward(t *testing.T) {
	clean := `package P {
		item def Fuel;
		part def Sys {
			part a { out item outFuel : Fuel; }
			part b { in item inFuel : Fuel; }
			flow f of Fuel from a.outFuel to b.inFuel;
		}
	}`
	if got := diagnose(t, "flow_payload_ref", clean); len(got) != 0 {
		t.Fatalf("unexpected findings: %s", strings.Join(got, "; "))
	}

	missing := `package P {
		part def Sys {
			part a { out item outFuel; }
			part b { in item inFuel; }
			flow f of Fuel from a.outFuel to b.inFuel;
		}
	}`
	got := diagnose(t, "flow_payload_missing", missing)
	if len(got) != 1 || !strings.Contains(got[0], "Fuel") {
		t.Fatalf("findings = %v; want one unresolved 'Fuel'", got)
	}
}
