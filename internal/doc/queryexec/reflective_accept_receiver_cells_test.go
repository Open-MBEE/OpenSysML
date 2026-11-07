package queryexec

import (
	"slices"
	"testing"
)

// A cell over a transition reads its `via` trigger's receiverArgument and the receiver parameter it values,
// each the FeatureReferenceExpression to the port.
func TestExecuteReadsViaTriggerReceiverArgument(t *testing.T) {
	fixture := loadExecutionFixture(t, `
private import ControlFunctions::*;
item def Sig;
port def P { in item s : Sig; }
part def Shutter {
	port p : P;
	state sm {
		entry; then idle;
		state idle; state open;
		transition opening first idle accept Sig via p then open;
		transition closing first open accept Sig then idle;
	}
}
calc def Receivers :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "Via", cell = { in row : SysML::Systems::TransitionUsage;
				row.triggerAction->exists { in a : SysML::Systems::AcceptActionUsage;
					a.receiverArgument->exists { in e : SysML::Systems::FeatureReferenceExpression; e.referent.declaredName == "p" } }
			}),
			Column(name = "Parameter", cell = { in row : SysML::Systems::TransitionUsage;
				row.triggerAction->exists { in a : SysML::Systems::AcceptActionUsage;
					a.parameter->exists { in f : SysML::Systems::ReferenceUsage;
						f.ownedElement->exists { in e : SysML::Systems::FeatureReferenceExpression; e.referent.declaredName == "p" } } }
			})
		)
	)
}
`)
	result := quantityRows(t, fixture, "Receivers", "Shutter::sm")
	names := cellTexts(t, result, 0)
	for transition, want := range map[string]bool{"opening": true, "closing": false} {
		at := slices.Index(names, transition)
		if at < 0 {
			t.Fatalf("name cells = %q, want the transition %s", names, transition)
		}
		for _, column := range []string{"Via", "Parameter"} {
			if got := cellBooleans(t, result, column)[at]; !slices.Equal(got, []bool{want}) {
				t.Errorf("%s cell of %s = %v, want [%t]", column, transition, got, want)
			}
		}
	}
}
