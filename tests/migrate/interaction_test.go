package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/rig_interactions.xmi: an interaction is an occurrence def whose lifelines are part
// usages holding their event occurrences in order, each message a message between two events; a call's
// receiving event performs the operation and the reply carries its result; fragments are comments around
// the messages of their operands in occurrence order; create messages are ordinary messages.
func TestInteractionCallsRepliesAndFragments(t *testing.T) {
	r := migrateFixtureFile(t, "rig_interactions")
	for _, line := range []string{
		"occurrence def Spinup {",
		"ref part c : Controller {",
		"event occurrence spinSent;",
		"then event occurrence spunReceived;",
		"ref part m : Motor {",
		"event occurrence spinReceived {",
		"perform action spinCall ::> Motor::spin { in rpm[1] = 30.0; in times[1] = 2; }",
		"then event occurrence spunSent;",
		"then event occurrence brakeReceived {",
		"perform action brakeCall ::> Motor::brake { in force[1] = 0.5; }",
		"perform action callSpin ::> Motor::spin { in rpm[1] = 40.0; }",
		"message spin from c.spinSent to m.spinReceived;",
		"message spun of result : ScalarValues::Real = m.spinReceived.spinCall.result from m.spunSent to c.spunReceived;",
		"/* alt [mode == 1] */",
		"message go of goPayload : Go = new Go(n = 3) from c.goSent to m.goReceived;",
		"/* else */",
		"message brake from c.brakeSent to m.brakeReceived;",
		"/* end alt */",
		"/* loop (2, 2) */",
		"message callSpin from c.callSpinSent to m.callSpinReceived;",
		"/* par */",
		"message parEnd of go : Go = new Go(n = 4) from c.parEndSent to m.parEndReceived;",
		"/* and */",
		"message parOp1 of Done[1] from m.parOp1Sent to c.parOp1Received;",
		"/* opt [ctrl.got > 10.0] */",
		"message 'new' from c.newSent to m.newReceived;",
		"occurrence spinup : Spinup {",
		"ref part :>> c = ctrl;",
		"ref part :>> m = drive.motor;",
		"/* not migrated: Interaction 'Astray' — the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches */",
		"/* not migrated: Interaction 'Twinned' — the lifeline 'm' stands for 'motor', which Twin reaches as both left.motor and right.motor */",
		"verification def 'Spinup Test' {",
		"subject context : Rig;",
		"ref part m :> context.drive.motor {",
		"perform action spinCall ::> Motor::spin { in rpm[1] = 12.0; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "action spinup {")
	wantNoLine(t, r.Notation, "send new Go")
	wantNote(t, r, "_spinup", migrate.Mapped, "written as an occurrence def of 2 lifeline usages and 8 messages between their event occurrences in occurrence order; its lifelines are bound to the features of Rig they stand for in the occurrence usage spinup")
	wantNote(t, r, "_lm", migrate.Mapped, "written as the ref part usage m holding its 8 event occurrences, bound to drive.motor of Rig where the def occurs")
	wantNote(t, r, "_mSpin", migrate.Approximated, "the call is the message; its receiving event performs spin on m")
	wantNote(t, r, "_mRet", migrate.Approximated, "the reply carries the result result of the call as its payload; the caller's got is not assigned the result result: a sequence states transfers, not assignments")
	wantNote(t, r, "_exec", migrate.Skipped, "the execution spans the events between its start and finish, which the then successions order without it")
	wantNote(t, r, "_alt", migrate.Approximated, "the alt fragment has no form in v2 sequence notation: its operands' messages are written in occurrence order, the operator and guards as comments")
	wantNote(t, r, "_altFast", migrate.Approximated, "the operand's messages are written in occurrence order under the comment alt [mode == 1]")
	wantNote(t, r, "_mBrake", migrate.Approximated, "the call is the message; its receiving event performs brake on m, to completion within the event")
	wantNote(t, r, "_loop", migrate.Approximated, "the loop fragment has no form in v2 sequence notation")
	wantNote(t, r, "_par", migrate.Approximated, "the concurrent operands are written one after the other")
	wantNote(t, r, "_mNew", migrate.Approximated, "the message creates m: v2 sequence notation has no creation message, so it is an ordinary message to the object's first event")
	wantNote(t, r, "_sNew", migrate.Mapped, "written as the event occurrence c.newSent, which the message 'new' leaves")
	wantNote(t, r, "_astray", migrate.Unmapped, "the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches")
	wantNote(t, r, "_twinned", migrate.Unmapped, "the lifeline 'm' stands for 'motor', which Twin reaches as both left.motor and right.motor")
	wantNote(t, r, "_tc", migrate.Mapped, "written as a verification def of 1 lifeline usage and 1 message between their event occurrences in occurrence order, the lifelines bound to the subject's parts")
	wantNote(t, r, "_trace", migrate.Mapped, "written as an occurrence def of 1 lifeline usage and 0 messages")
	wantClean(t, "rig_interactions.sysml", r)
}
