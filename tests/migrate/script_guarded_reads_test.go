package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A script assigning a property declaring no default from a name admitting no
// value writes the assignment guarded, so the property stays unset when the name
// holds none: a read after it is reported as reaching the property unset on
// that path; after several guarded assignments the property is unset only when
// every guard fails, and the note names the guards made before the read, not
// one made after it. An unguarded assignment before the read settles the
// property on every path, and the read is not reported.
func TestScriptReadsAfterGuardedAssignmentsAreReported(t *testing.T) {
	r := migrateXMI(t, "script_guarded_reads")
	wantClean(t, "t.sysml", r)
	wantLine(t, r.Notation, "if w->SequenceFunctions::notEmpty() { assign tMax := w; }")
	wantNote(t, r, "_clamp", migrate.Approximated, "tMax must hold a value, so it is assigned only when w, which may hold none, holds one; tMax holds no initial value and is assigned only when w holds one, and the body reads it after, so a run in which w holds none reaches the read unset and stops; the script would read an unset name as null, which its arithmetic takes as 0")
	wantNote(t, r, "_pick", migrate.Approximated, "tMax must hold a value, so it is assigned only when w, which may hold none, holds one; tMax must hold a value, so it is assigned only when v, which may hold none, holds one; tMax holds no initial value and is assigned only when w or v holds one, and the body reads it after, so a run in which neither w nor v holds one reaches the read unset and stops; the script would read an unset name as null, which its arithmetic takes as 0")
	wantNote(t, r, "_early", migrate.Approximated, "tMax must hold a value, so it is assigned only when w, which may hold none, holds one; tMax must hold a value, so it is assigned only when v, which may hold none, holds one; tMax holds no initial value and is assigned only when w holds one, and the body reads it after, so a run in which w holds none reaches the read unset and stops; the script would read an unset name as null, which its arithmetic takes as 0")
	wantNote(t, r, "_settle", migrate.Approximated, "tMax must hold a value, so it is assigned only when w, which may hold none, holds one")
	for _, e := range entriesFor(r, "_settle") {
		if strings.Contains(e.Note, "reaches the read unset") {
			t.Errorf("settle, which assigns tMax on every path before reading it, is noted as reading it unset: %s", e.Note)
		}
	}
}
