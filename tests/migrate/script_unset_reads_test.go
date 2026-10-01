package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A property declaring no default holds no value until assigned, and a v2 run
// reading it first stops where the tool's script engine reads null and counts
// it as 0; a body reading such a property before it assigns it is reported
// naming the property, a body assigning it first or only writing it is not.
// Every JavaScript body is an approximation already, so the verdicts stay.
func TestScriptReadsOfUnsetPropertiesAreReported(t *testing.T) {
	r := migrateXMI(t, "script_unset_reads")
	for _, line := range []string{
		"assign context.t0 := localClock.currentTime;",
		"assign context.tdiff := t1 - context.t0;",
		"assign context.tdiff := 0;",
		"assign context.tStep := context.tdiff + context.steps;",
		"assign context.steps := context.steps + 1;",
		"assign context.tdiff := context.tdiff + context.steps;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_stamp", migrate.Approximated, "the JavaScript body is translated to v2; the clock variable simtime, named by the configuration Timing, reads the local clock; the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_diffTime", migrate.Approximated, "the JavaScript body is translated to v2; the clock variable simtime, named by the configuration Timing, reads the local clock; t1 is declared nowhere, so it is declared a local attribute of the action: a script's assignment to an undeclared name creates it; tdiffOld is declared nowhere, so it is declared a local attribute of the action: a script's assignment to an undeclared name creates it; context.tdiff and context.t0 hold no initial value and the body reads them before assigning them, so a run reaching the read first stops; the script would read an unset name as null, which its arithmetic takes as 0; the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_reset", migrate.Approximated, "the JavaScript body is translated to v2; the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_count", migrate.Approximated, "the JavaScript body is translated to v2; context.tdiff holds no initial value and the body reads it before assigning it, so a run reaching the read first stops; the script would read an unset name as null, which its arithmetic takes as 0; the JavaScript body is written as v2 assignments")
}
