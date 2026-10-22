package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/usecase_sequence.xmi: a use case owns an interaction whose lifelines stand
// for the interaction's own properties, typed by an actor and by blocks or untyped; the
// interaction is an occurrence def of part usages holding event occurrences, with a message
// per v1 message, and its sequence diagram a SequenceView whose lifeline heads and message
// lines keep the Cameo layout.
func TestGoldenUseCaseSequence(t *testing.T) {
	r := migrateStream(t, "usecase_sequence")
	checkGolden(t, "testdata/xmi/usecase_sequence.golden.sysml", r.Notation)
	for _, d := range errors(t, "usecase_sequence.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
	for _, line := range []string{
		"occurrence def 'Acquire Images' {",
		"ref part operator :> Operator {",
		"ref part stage : 'Sample Stage' {",
		"part log {",
		"event occurrence moveSent;",
		"perform action moveCall ::> 'Sample Stage'::move { in 'to'[1] = 2.5; }",
		"message move from operator.moveSent to stage.moveReceived;",
		"message pumpDown of pumpDownPayload : PumpDown = new PumpDown(target = 0.1) from stage.pumpDownSent to vacuum.pumpDownReceived;",
		"message 'new' from vacuum.newSent to image.newReceived;",
		"message record from image.recordSent to log.recordReceived;",
		"view 'Acquire Images' : StandardViewDefinitions::SequenceView {",
		"metadata DiagramLayout::Layout about operator { x = 20; y = 20; width = 120; height = 50; }",
		"metadata DiagramLayout::Route about move { points = (80, 120, 260, 120); }",
		"metadata DiagramLayout::Route about 'new' { points = (450, 240, 630, 240); }",
	} {
		wantLine(t, r.Notation, line)
	}
	if l := r.Report.Layout; l == nil || l.PlacementsWritten != 5 || l.RoutesWritten != 4 {
		t.Errorf("layout report = %+v, want 5 lifeline heads placed and 4 message lines routed", l)
	}
	wantNote(t, r, "_seq", migrate.Mapped, "written as an occurrence def of 5 lifeline usages and 4 messages")
	wantNote(t, r, "_lOperator", migrate.Mapped, "the interaction's own 'operator'")
	wantNote(t, r, "_mNew", migrate.Approximated, "v2 sequence notation has no creation message")
	wantNote(t, r, "_mLog", migrate.Approximated, "the call names no operation")
}
