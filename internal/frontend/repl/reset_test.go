package repl

import (
	"strings"
	"testing"
)

// A reset replaces every declaration, so nothing materialized from the old ones
// can be rebound: what goes is reported at the reset rather than silently emptied.
func TestClearReportsWhatItTook(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Demo::Vehicle")
	got := run(t, s, "%clear")
	wants(t, got, "session cleared", "1 instance", "reset")
}

// A later command about an object the reset took explains the loss instead of
// reading as a session that never materialized anything.
func TestCommandsAfterClearExplainTheLoss(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Demo::Vehicle")
	run(t, s, "%clear")
	wants(t, run(t, s, "%instances"), "the session was reset")
	// The declaration went with the reset, so the name itself is unresolved — the
	// loss is what makes that answer make sense.
	wants(t, run(t, s, "%features Demo::Vehicle"), "the session was reset")
}

// A reset ends a debugging session, and the next %step says so rather than
// reporting a session that was never started.
func TestClearEndsDebuggerWithAReason(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	wants(t, run(t, s, "%clear"), `action debugging session for "tally" ended`, "reset")
	wants(t, run(t, s, "%step"), "reset")
}

// A reset with nothing materialized has nothing to report.
func TestClearOfAnEmptySessionIsSilent(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	if got := run(t, s, "%clear"); got != "session cleared" {
		t.Errorf("%%clear reported %q, want the reset alone", got)
	}
}

// Reloading the same text supersedes the declarations but resolves them the same
// way, so an object survives it — with its identity, not as a second object.
func TestReloadKeepsObjectsItStillResolves(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	wants(t, run(t, s, "%instantiate Demo::Vehicle"), "ID: 1")

	out, err := s.LoadPaths([]string{"testdata/vehicle_package.sysml"})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if strings.Contains(joinLines(out), "dropped") {
		t.Errorf("reload dropped the object it still resolves:\n%s", joinLines(out))
	}
	wants(t, run(t, s, "%instances"), "Demo::Vehicle (ID: 1)")
	wants(t, run(t, s, "%features Demo::Vehicle"), "ID: 1", "mass = 1500")
}

// A reload replaces the declaration the debugger is stepping, which ends that
// session as any redeclaration of it does — reported, and with the reason kept for
// the next %step, rather than silently.
func TestReloadEndsDebuggerWithAReason(t *testing.T) {
	s := loadFixture(t, "testdata/action_debug.sysml")
	run(t, s, "%action tally")
	out, err := s.LoadPaths([]string{"testdata/action_debug.sysml"})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	wants(t, joinLines(out), `action debugging session for "tally" ended`)
	wants(t, run(t, s, "%step"), "no active action session", "ended")
}

// A load that does change what an object was materialized against cannot carry it
// over, so the loss is reported and the reason kept for the next command.
func TestLoadThatChangesDeclarationsReportsTheLoss(t *testing.T) {
	s := loadFixture(t, "testdata/vehicle_package.sysml")
	run(t, s, "%instantiate Demo::Vehicle")

	res := s.Submit("package Demo { part def Vehicle { attribute mass = 900.0; } }")
	if !hasNotice(res, "dropped") {
		t.Fatalf("notices = %v, want the dropped object reported", res.Notices)
	}
	wants(t, run(t, s, "%instances"), "dropped", "re-run %instantiate")
}
