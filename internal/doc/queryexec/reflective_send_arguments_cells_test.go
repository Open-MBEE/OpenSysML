package queryexec

import (
	"slices"
	"testing"
)

// A cell over a send action reads its payloadArgument, senderArgument and receiverArgument,
// and the `in` parameters that hold them (SysML v2.1 §8.3.17.15).
func TestExecuteReadsSendActionArguments(t *testing.T) {
	fixture := loadExecutionFixture(t, `
private import ControlFunctions::*;
item def Alarm;
part def Monitor {
	port alarmPort;
	part display;
	action supervise {
		action notifyVia send new Alarm() via alarmPort;
		action notifyTo send new Alarm() to display;
	}
}
calc def Sends :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "Payload", cell = { in row : SysML::Systems::SendActionUsage;
				row.payloadArgument->exists { in e : SysML::Systems::ConstructorExpression; true }
			}),
			Column(name = "Sender", cell = { in row : SysML::Systems::SendActionUsage;
				row.senderArgument->exists { in e : SysML::Systems::FeatureReferenceExpression; e.referent.declaredName == "alarmPort" }
			}),
			Column(name = "Receiver", cell = { in row : SysML::Systems::SendActionUsage;
				row.receiverArgument->exists { in e : SysML::Systems::FeatureReferenceExpression; e.referent.declaredName == "display" }
			}),
			Column(name = "Parameter", cell = { in row : SysML::Systems::SendActionUsage;
				row.parameter->exists { in f : SysML::Systems::ReferenceUsage;
					f.ownedElement->exists { in e : SysML::Systems::FeatureReferenceExpression; e.referent.declaredName == "display" } }
			})
		)
	)
}
`)
	result := quantityRows(t, fixture, "Sends", "Monitor::supervise")
	names := cellTexts(t, result, 0)
	want := map[string]struct {
		payload, sender, receiver, parameter bool
	}{
		"notifyVia": {true, true, false, false},
		"notifyTo":  {true, false, true, true},
	}
	for send, expect := range want {
		at := slices.Index(names, send)
		if at < 0 {
			t.Fatalf("name cells = %q, want the send %s", names, send)
		}
		for column, value := range map[string]bool{"Payload": expect.payload, "Sender": expect.sender, "Receiver": expect.receiver, "Parameter": expect.parameter} {
			if got := cellBooleans(t, result, column)[at]; !slices.Equal(got, []bool{value}) {
				t.Errorf("%s cell of %s = %v, want [%t]", column, send, got, value)
			}
		}
	}
}
