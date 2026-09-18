package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// testdata/xmi/rig_interactions.xmi: call and reply messages perform the operation on the lifeline's
// object and assign its result; alt/opt/loop/par become if/if/for/fork; create messages are reported.
func TestInteractionCallsRepliesAndFragments(t *testing.T) {
	r := migrateFixtureFile(t, "rig_interactions")
	for _, line := range []string{
		"action def Spinup {",
		"perform action spin : Motor::Spin ::> drive.motor.spin { in rpm = 30.0; in times = 2; }",
		"first start then spin;",
		"action spun {",
		"assign this.ctrl.got := spin.result;",
		"first spin then spun;",
		"action alt {",
		"if this.mode == 1 {",
		"action altOp1 {",
		"action go send new Go(n = 3) to this.drive.motor;",
		"else {",
		"action altOp2 {",
		"perform action brake : Motor::Brake ::> drive.motor.brake { in force = 0.5; }",
		"first spun then alt;",
		"action 'loop' {",
		"for i in 1..2 {",
		"action loopOp1 {",
		"perform action callSpin : Motor::Spin ::> drive.motor.spin { in rpm = 40.0; }",
		"first alt then 'loop';",
		"fork par;",
		"first 'loop' then par;",
		"action parOp1 {",
		"action sendGo send new Go() to this.drive.motor;",
		"first par then parOp1;",
		"first parOp1 then parEnd;",
		"action parOp2 {",
		"action sendDone send new Done() to this.ctrl;",
		"join parEnd;",
		"action opt {",
		"if this.ctrl.got > 10.0 {",
		"first parEnd then opt;",
		"first opt then done;",
		"/* not migrated: Interaction 'Astray' — the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches */",
		"verification def 'Spinup Test' {",
		"subject context : Rig;",
		"perform action spin : Motor::Spin ::> context.drive.motor.spin { in rpm = 12.0; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_spinup", migrate.Approximated, "written as a scenario of 8 steps, one per message in occurrence order")
	wantNote(t, r, "_lm", migrate.Mapped, "the lifeline stands for this.drive.motor, which the steps address")
	wantNote(t, r, "_mSpin", migrate.Mapped, "written as a call of Spin on this.drive.motor")
	wantNote(t, r, "_mRet", migrate.Approximated, "the value 30.0 the reply states for result is the operation's own result, which the call computes")
	wantNote(t, r, "_exec", migrate.Skipped, "the execution spans the steps between its occurrences")
	wantNote(t, r, "_alt", migrate.Mapped, "written as the action alt, an if over the operands")
	wantNote(t, r, "_altFast", migrate.Mapped, "its guard is the condition [this.mode == 1]")
	wantNote(t, r, "_altElseG", migrate.Mapped, "the guard is the operand's condition")
	wantNote(t, r, "_mBrake", migrate.Approximated, "the asynchronous call is performed to completion before the next step")
	wantNote(t, r, "_loop", migrate.Mapped, "written as the action 'loop', a for over the operands")
	wantNote(t, r, "_par", migrate.Mapped, "written as the action par, a fork over the operands")
	wantNote(t, r, "_mNew", migrate.Unmapped, "the message creates this.drive.motor, a part that exists for as long as its owner does")
	wantNote(t, r, "_sNew", migrate.Unmapped, "the occurrence belongs to the message 'new', which is not written")
	wantNote(t, r, "_astray", migrate.Unmapped, "the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches")
	wantNote(t, r, "_tc", migrate.Approximated, "written as a scenario of 1 step")
	wantNote(t, r, "_trace", migrate.Unmapped, "the interaction has no message: it records 2 state invariant(s) under 2 time constraint(s), a timing trace, which no scenario step performs")

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%action Rig::Spinup #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the scenario did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : drive.motor.speed"); !strings.Contains(out, "= 40.0") {
		t.Errorf("the loop's calls did not spin the motor to 40.0: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : ctrl.got"); !strings.Contains(out, "= 30.0") {
		t.Errorf("the reply did not store the call's result: %s", out)
	}
}
