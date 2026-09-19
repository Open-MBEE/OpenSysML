package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// testdata/xmi/heater_receptions.xmi: a reception with a method accepts its signal and performs the
// method with the payload bound, its optional and defaulted parameters left unbound; one without a
// method, or whose method requires a value the signal lacks, only accepts; unmigratable methods
// and signals are refused.
func TestReceptionsAcceptAndPerformTheirMethod(t *testing.T) {
	r := migrateFixtureFile(t, "heater_receptions")
	for _, line := range []string{
		"action def SetLevel {",
		"first start then receive;",
		"action receive accept setLevel : Signals::SetLevel;",
		"first receive then run;",
		"action run : 'Apply Level' { in value = setLevel.value; }",
		"first run then done;",
		"action setLevel : SetLevel;",
		"action def Stop {",
		"action receive accept stop : Signals::Stop;",
		"first receive then done;",
		"action stop : Stop;",
		"action def Reset {",
		"action receive accept reset : Signals::Reset;",
		"action reset : Reset;",
		"action def Boost {",
		"action receive accept boost : Signals::Boost;",
		"action boost : Boost;",
		"in slack : ScalarValues::Real[0..1];",
		"comment /* reception 'Away' */",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, bound := range []string{"in gain =", "in slack =", ": Boosting"} {
		if strings.Contains(string(r.Notation), bound) {
			t.Errorf("%q was written, though no signal attribute supplies the method's parameter:\n%s", bound, r.Notation)
		}
	}
	wantNote(t, r, "_rcvSet", migrate.Mapped, "written as an action def accepting SetLevel and performing its method Heater::Apply Level, which its owner's usage setLevel runs")
	wantNote(t, r, "_rpValue", migrate.Mapped, "stands for the signal's attribute value, which the accepted payload carries")
	wantNote(t, r, "_rpExtra", migrate.Unmapped, "the parameter extra matches no attribute of the signal")
	wantNote(t, r, "_rcvStop", migrate.Approximated, "the reception has no method, so it only accepts the signal")
	wantNote(t, r, "_rcvReset", migrate.Approximated, "the method Heater::Resetting has no action def to perform; the reception only accepts the signal")
	wantNote(t, r, "_rcvBoost", migrate.Approximated, "the method Heater::Boosting's parameter amount must hold a value that no attribute of the signal supplies; the reception only accepts the signal")
	wantNote(t, r, "_rcvAway", migrate.Unmapped, "signal")
	wantNote(t, r, "_apply", migrate.Mapped, "")

	s := session(t, r)
	meta(t, s, "%instantiate Heater")
	meta(t, s, "%action Heater::SetLevel #1")
	meta(t, s, "%step")
	if out := meta(t, s, "%step"); !strings.Contains(out, "State: Waiting") {
		t.Errorf("the reception is not waiting at its accept:\n%s", out)
	}
	if out := meta(t, s, "%send Signals::SetLevel(value=3.5)"); !strings.Contains(out, "Sent SetLevel") {
		t.Errorf("%%send SetLevel: %s", out)
	}
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the reception did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : level"); !strings.Contains(out, "= 3.5") {
		t.Errorf("the method did not store the signal's value: %s", out)
	}
	meta(t, s, "%action Heater::Boost #1")
	meta(t, s, "%step")
	if out := meta(t, s, "%step"); !strings.Contains(out, "State: Waiting") {
		t.Errorf("the Boost reception is not waiting at its accept:\n%s", out)
	}
	if out := meta(t, s, "%send Signals::Boost()"); !strings.Contains(out, "Sent Boost") {
		t.Errorf("%%send Boost: %s", out)
	}
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the Boost reception did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : level"); !strings.Contains(out, "= 3.5") {
		t.Errorf("the refused method ran: %s", out)
	}
}
