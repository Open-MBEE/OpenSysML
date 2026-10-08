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
// accept. A button posting a signal specializing the accepted one counts as
// posting it, and a send action written as a placeholder, which performs
// nothing, is no sender.
func TestSignalsOnlyTheToolsUIPostsAreLedgered(t *testing.T) {
	r := migrateXMI(t, "ui_stimuli")
	wantClean(t, "t.sysml", r)
	const unsent = "no send action of the document sends 'Start', which only 1 button of the tool's UI prototype posts, which the migration does not write; an accept of it waits for a message nothing in the model posts"
	wantNote(t, r, "_trStart", migrate.Approximated, unsent)
	wantNote(t, r, "_awTr", migrate.Approximated, unsent)
	wantNote(t, r, "_trStop", migrate.Mapped, "")
	wantNote(t, r, "_trReset", migrate.Mapped, "")
	wantNote(t, r, "_trFinish", migrate.Mapped, "")
	wantNote(t, r, "_trPause", migrate.Approximated, "no send action of the document sends 'Pause' or a signal specializing it ('Long Pause'), which only 1 button of the tool's UI prototype posts, which the migration does not write; an accept of it waits for a message nothing in the model posts")
	wantNote(t, r, "_sendResume", migrate.Approximated, "the send passes no argument for the attribute delay of Resume, which must hold a value")
	wantNote(t, r, "_trResume", migrate.Approximated, "no send action of the document sends 'Resume', which only 1 button of the tool's UI prototype posts, which the migration does not write; an accept of it waits for a message nothing in the model posts")
	wantNote(t, r, "_cfg", migrate.Approximated, "«SimulationConfig» UI = 'Operate the controller' is the tool's UI prototype, through which a user of the tool's run posts signals and reads values; it has no v2 form, so a run here takes no input from it")
	wantLine(t, r.Notation, "UI = Gui::Console")
}
