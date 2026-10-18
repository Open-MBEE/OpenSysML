package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A script's assignment to a name nothing in the model bears creates that name,
// as the tool's engine does, so the body is translated with the name declared a
// local attribute of the effect rather than kept whole as a comment. A Java body
// declares every variable, so there the name stays refused, as it does when read
// before it is assigned.
func TestScriptAssignmentsDeclareUndeclaredNames(t *testing.T) {
	r := migrateXMI(t, "script_locals")
	for _, line := range []string{
		"assign context.t0 := localClock.currentTime;",
		"attribute t1 : ScalarValues::Real;",
		"assign t1 := localClock.currentTime;",
		"attribute tdiffOld : ScalarValues::Real;",
		"assign tdiffOld := context.tdiff;",
		"assign context.tdiff := t1 - context.t0;",
		"assign context.tStep := context.tdiff - tdiffOld;",
		"* tdiffOld = tdiff;",
		"* stepsOld = steps;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "attribute stepsOld")
	wantClean(t, "t.sysml", r)
	wantLine(t, r.Notation, `rep language "JavaScript" /* t1 = simtime;`)
	wantLine(t, r.Notation, `rep language "Java" /* tdiffOld = tdiff;`)
	wantLine(t, r.Notation, `rep language "JavaScript" /* steps = steps + stepsOld;`)
	wantNote(t, r, "_diffTime", migrate.Approximated, "the JavaScript body is translated to v2; the clock variable simtime, named by the configuration Timing, reads the local clock; t1 is declared nowhere, so it is declared a local attribute of the action: a script's assignment to an undeclared name creates it; tdiffOld is declared nowhere, so it is declared a local attribute of the action: a script's assignment to an undeclared name creates it; the console print print(…) is left out, as it writes to the tool's console and changes nothing of the model; the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_javaDiff", migrate.Approximated, `the body is kept as a textual representation, which is not executed: the name "tdiffOld" resolves to nothing readable: nothing visible from Executive::Executive Behavior::<Region>::<Transition>::javaDiff is called tdiffOld`)
	wantNote(t, r, "_readFirst", migrate.Approximated, `the body is kept as a textual representation, which is not executed: the name "stepsOld" resolves to nothing readable: nothing visible from Executive::Executive Behavior::<Region>::<Transition>::readFirst is called stepsOld`)
}
