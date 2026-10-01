package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// A signal only a button of the tool's UI prototype posts has no sender in the
// migrated model, so every accept of it — a transition's trigger, an accept
// action — is ledgered as waiting for a message nothing in the model posts, and
// a configuration whose UI is that prototype says a run here takes no input
// from it. A signal a send action sends — itself or a signal specializing it,
// which the accept takes too — or one nothing at all posts, is an ordinary
// accept.
func TestSignalsOnlyTheToolsUIPostsAreLedgered(t *testing.T) {
	r := migrateXMI(t, "ui_stimuli")
	wantClean(t, "t.sysml", r)
	const unsent = "no send action of the document sends 'Start', which only 1 button of the tool's UI prototype posts, which the migration does not write; an accept of it waits for a message nothing in the model posts"
	wantNote(t, r, "_trStart", migrate.Approximated, unsent)
	wantNote(t, r, "_awTr", migrate.Approximated, unsent)
	wantNote(t, r, "_trStop", migrate.Mapped, "")
	wantNote(t, r, "_trReset", migrate.Mapped, "")
	wantNote(t, r, "_trFinish", migrate.Mapped, "")
	wantNote(t, r, "_cfg", migrate.Approximated, "«SimulationConfig» UI = 'Operate the controller' is the tool's UI prototype, through which a user of the tool's run posts signals and reads values; it has no v2 form, so a run here takes no input from it")
	wantLine(t, r.Notation, "UI = Gui::Console")
}
