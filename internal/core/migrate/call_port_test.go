package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// The fixture testdata/xmi/ported_calls.xmi is a drive whose port a connector
// joins to its motor's command port; its activities call the motor's operation
// over the drive's port, over the motor's own port on the read part, and over a
// port no connector joins. A call over the caller's port performs the
// operation's usage on the part the caller's connector joins to that port; one
// over the target's own port performs it on the target, the port left
// unwritten; one over a port no connector joins stays in the caller's context
// with the reason. Running the drive's activity spins its motor through both
// calls.
func TestCallOperationOverPortsReachesTheConnectedPart(t *testing.T) {
	r := migrateFixtureFile(t, "ported_calls")
	for _, line := range []string{
		"perform action 'spin over p' ::> motor.spin;",
		"perform action 'spin over cmd' ::> motor.spin;",
		"action 'spin over loose' : Motor::Spin;",
		"flow thirty.result to 'spin over p'.rpm;",
		"flow forty.result to 'spin over cmd'.rpm;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "::> p.") || strings.Contains(string(r.Notation), "::> motor.cmd.") {
		t.Errorf("a call was performed on a port whose type has no operation:\n%s", r.Notation)
	}
	wantNote(t, r, "_callP", migrate.Mapped, "the call performs the usage spin of the part connected to the port p")
	wantNote(t, r, "_callCmdTgt", migrate.Mapped, "the call performs the usage spin of the target this.motor; its port cmd is not written, as a v2 perform names the operation on the object")
	wantNote(t, r, "_callCmd", migrate.Approximated, "several edges lead to the node, which waits for all of them through the join 'join'")
	wantNote(t, r, "_callLoose", migrate.Approximated, "the call runs in the caller's context: no connector of Drive joins its port loose to a part")

	s := session(t, r)
	meta(t, s, "%instantiate Drive")
	meta(t, s, "%action Drive::Run #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : motor.speed"); !strings.Contains(out, "= 40.0") {
		t.Errorf("the calls did not spin the motor: %s", out)
	}
}
