package repl

import (
	"strings"
	"testing"
)

// An action a subtype inherits is named by the subtype's qualified name too, and
// runs on the object as its own feature: the assignment reaches the object.
func TestActionByTheSubtypesQualifiedNameRunsTheInheritedAction(t *testing.T) {
	s := loadFixture(t, "testdata/inherited_action.sysml")
	run(t, s, "%instantiate p")
	wants(t, run(t, s, "%features p"), "runs = 0")

	wants(t, run(t, s, "%action BigPlant::fill p"), `Started action executor for "BigPlant::fill"`)
	wants(t, run(t, s, "%continue"), "Action completed")
	wants(t, run(t, s, "%features p"), "runs = 1")
}

// A redefinition in the subtype is the member the subtype's qualified name
// denotes, so it is the one that runs: its own step, beside the step it
// inherits from the action it redefines, not the inherited action alone.
func TestActionByTheSubtypesQualifiedNameRunsItsRedefinition(t *testing.T) {
	s := loadFixture(t, "testdata/inherited_action.sysml")
	run(t, s, "%instantiate r")

	wants(t, run(t, s, "%action RedefPlant::fill r"), `Started action executor for "RedefPlant::fill"`)
	wants(t, run(t, s, "%continue"), "Action completed")
	wants(t, run(t, s, "%features r"), "runs = 11")
}

// Inheritance is transitive: the grandparent's action is named by the
// grandchild's qualified name.
func TestActionByTheGrandchildsQualifiedNameRunsTheInheritedAction(t *testing.T) {
	s := loadFixture(t, "testdata/inherited_action.sysml")
	run(t, s, "%instantiate h")

	wants(t, run(t, s, "%action HugePlant::fill h"), `Started action executor for "HugePlant::fill"`)
	wants(t, run(t, s, "%continue"), "Action completed")
	wants(t, run(t, s, "%features h"), "runs = 1")
}

// The other commands naming a behavior resolve the inherited spelling too.
func TestGraphsBySubtypesQualifiedNameFindsTheInheritedAction(t *testing.T) {
	s := loadFixture(t, "testdata/inherited_action.sysml")
	wants(t, run(t, s, "%graphs BigPlant::fill"), `"name":"Plant::fill"`, `"kind":"actionUsage"`)
}

// A name that resolves to nothing still fails as before, with the nearest
// member suggested when the qualifier is a declaration.
func TestActionByAnUnknownQualifiedNameStillErrorsWithSuggestions(t *testing.T) {
	s := loadFixture(t, "testdata/inherited_action.sysml")
	run(t, s, "%instantiate p")

	wants(t, run(t, s, "%action Plant::fil p"), "error: unresolved reference: Plant::fil", "did you mean Plant::fill?")
	got := run(t, s, "%action BigPlant::fil p")
	wants(t, got, "error: unresolved reference: BigPlant::fil")
	wants(t, run(t, s, "%action Nope::fill p"), "error: unresolved reference: Nope::fill")
	if strings.Contains(run(t, s, "%features p"), "runs = 1") {
		t.Errorf("a miss ran the action")
	}
}
