package queryexec

import (
	"slices"
	"testing"
)

// A cell over a transition compares TransitionFeatureMembership::kind and TriggerInvocationExpression::kind
// with their SysML literal, and a WhereFeature keeps a guard's membership and an `after` trigger's expression by kind.
func TestExecuteReadsTransitionFeatureKindsAsTheLibraryLiteral(t *testing.T) {
	fixture := loadExecutionFixture(t, `
private import ControlFunctions::*;
part def Shutter {
	attribute x : Integer = 1;
	state sm {
		entry; then idle;
		state idle; state open;
		transition opening first idle accept when x > 5 if x > 0 do action click then open;
		transition closing first open accept after 1 [SI::s] then idle;
	}
}
calc def Kinds :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "Guard", cell = { in row : SysML::Systems::TransitionUsage;
				row.ownedMembership->exists { in m : SysML::Systems::TransitionFeatureMembership;
					m.kind == SysML::Systems::TransitionFeatureKind::guard }
			}),
			Column(name = "Effect", cell = { in row : SysML::Systems::TransitionUsage;
				row.ownedMembership->exists { in m : SysML::Systems::TransitionFeatureMembership;
					m.kind == SysML::Systems::TransitionFeatureKind::effect }
			}),
			Column(name = "When", cell = { in row : SysML::Systems::TransitionUsage;
				row.triggerAction.payloadArgument->exists { in e : SysML::Systems::TriggerInvocationExpression;
					e.kind == SysML::Systems::TriggerKind::'when' }
			})
		)
	)
}
calc def Guards :> Query {
	in root : Element;
	Project(
		source = WhereFeature(source = Descendants(source = root, maxDepth = 1, relationships = true),
			feature = "kind", operator = "=", value = "guard", tolerate = true),
		properties = ("name")
	)
}
calc def Afters :> Query {
	in root : Element;
	Project(
		source = WhereFeature(source = Descendants(source = root, maxDepth = 6, relationships = true),
			feature = "kind", operator = "=", value = "after", tolerate = true),
		properties = ("name")
	)
}
`)
	result := quantityRows(t, fixture, "Kinds", "Shutter::sm")
	names := cellTexts(t, result, 0)
	for transition, want := range map[string]bool{"opening": true, "closing": false} {
		at := slices.Index(names, transition)
		if at < 0 {
			t.Fatalf("name cells = %q, want the transition %s", names, transition)
		}
		for _, column := range []string{"Guard", "Effect", "When"} {
			if got := cellBooleans(t, result, column)[at]; !slices.Equal(got, []bool{want}) {
				t.Errorf("%s cell of %s = %v, want [%t]", column, transition, got, want)
			}
		}
	}
	for transition, want := range map[string][2]int{"opening": {1, 0}, "closing": {0, 1}} {
		if got := len(quantityRows(t, fixture, "Guards", "Shutter::sm::"+transition).Rows()); got != want[0] {
			t.Errorf("WhereFeature kind = guard under %s keeps %d rows, want %d", transition, got, want[0])
		}
		if got := len(quantityRows(t, fixture, "Afters", "Shutter::sm::"+transition).Rows()); got != want[1] {
			t.Errorf("WhereFeature kind = after under %s keeps %d rows, want %d", transition, got, want[1])
		}
	}
}
