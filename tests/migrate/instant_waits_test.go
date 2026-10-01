package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A v2 entry action or transition effect is performed whole at the instant it
// is triggered, so a v1 effect or entry whose activity waits for the clock — by
// a duration constraint on one of its nodes, a call of an activity with one,
// or an accept of a time event — is ledgered as an approximation naming the
// wait: a run stops there. A do activity may wait, and gets no such note.
func TestInstantBehaviorsWaitingForTheClockAreLedgered(t *testing.T) {
	r := migrateXMI(t, "instant_waits")
	wantClean(t, "t.sysml", r)
	wantNote(t, r, "_sendAck", migrate.Approximated, "it waits for the clock (the duration constraint 'dtOn' on 'Turn On' in 'Send Ack On'), which a v2 transition effect, performed whole at the instant it is triggered, may not: a run stops at the wait")
	wantNote(t, r, "_on", migrate.Approximated, "its entry action Stimulus::Warm Up: it waits for the clock (the accept of a time event 'warmed' in 'Warm Up'), which a v2 entry action, performed whole at the instant it is triggered, may not: a run stops at the wait")
	wantNote(t, r, "_blink", migrate.Approximated, "also run as the do action of 'On'")
	for _, e := range entriesFor(r, "_blink") {
		if strings.Contains(e.Note, "waits for the clock") {
			t.Errorf("the do action Blink is noted as waiting for the clock: %s", e.Note)
		}
	}
}
