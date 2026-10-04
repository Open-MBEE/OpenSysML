package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/variables.xmi, activities Reducing and Raising: a reduce folds
// its collection by the reducer's v2 function; a raise exception ends the
// activity as an activity final does.
func TestReduceRaiseFixture(t *testing.T) {
	r := migrateFixtureFile(t, "variables")
	wantClean(t, "variables.xmi", r)

	// A library primitive binary function and a model calc of two in
	// parameters and one return both reduce; an activity does not.
	wantLine(t, r.Notation, "out result : ScalarValues::Integer[1] = collection->ControlFunctions::reduce {in x; in y; IntegerFunctions::'+'(x, y)};")
	wantNote(t, r, "_reduce", migrate.Mapped, "")
	wantLine(t, r.Notation, "out result : ScalarValues::Integer[1] = collection->ControlFunctions::reduce {in x; in y; Sum(x, y)};")
	wantNote(t, r, "_reduceModel", migrate.Mapped, "")
	wantNote(t, r, "_reduceNone", migrate.Unmapped, "not a calc or function def, so it has no v2 function")

	// A raise exception writes a sibling terminate after it; its own outgoing
	// edge carries no token.
	wantNote(t, r, "_raise", migrate.Approximated, "v2 has no exceptions: raising one ends the enclosing activity, as an activity final does; the exception value is not passed to a caller")
	wantLine(t, r.Notation, "action 'terminate' terminate;")
	wantLine(t, r.Notation, "first raise then 'terminate';")
	wantNote(t, r, "_ra2", migrate.Unmapped, "the action raises an exception, so no token leaves it")
}
