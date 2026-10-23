package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/usecase_sequence.xmi: a use case owns an interaction whose lifelines stand
// for the interaction's own properties, typed by an actor and by blocks or untyped, two of
// them unnamed as Cameo leaves a default lifeline's; the interaction is an occurrence def of
// part usages holding event occurrences, the unnamed ones named after their types as any
// unnamed property is, with a message per v1 message, and its sequence diagram a
// SequenceView whose lifeline heads and message lines keep the Cameo layout.
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
		"ref part 'display System' : 'Display System' {",
		"ref part operator2 :> Operator {",
		"message show from image.showSent to 'display System'.showReceived;",
		"message confirm from 'display System'.confirmSent to operator2.confirmReceived;",
		"metadata MigrationMetadata::SynthesizedName about 'display System', operator2;",
		"view 'Acquire Images' : StandardViewDefinitions::SequenceView {",
		"metadata DiagramLayout::Layout about operator { x = 20; y = 20; width = 120; height = 50; }",
		"metadata DiagramLayout::Layout about 'display System' { x = 900; y = 20; width = 140; height = 50; }",
		"metadata DiagramLayout::Route about move { points = (80, 120, 260, 120); }",
		"metadata DiagramLayout::Route about 'new' { points = (450, 240, 630, 240); }",
		"metadata DiagramLayout::Route about confirm { points = (970, 420, 1160, 420); }",
	} {
		wantLine(t, r.Notation, line)
	}
	if l := r.Report.Layout; l == nil || l.PlacementsWritten != 7 || l.RoutesWritten != 6 {
		t.Errorf("layout report = %+v, want 7 lifeline heads placed and 6 message lines routed", l)
	}
	wantNote(t, r, "_seq", migrate.Mapped, "written as an occurrence def of 7 lifeline usages and 6 messages")
	wantNote(t, r, "_lOperator", migrate.Mapped, "the interaction's own 'operator'")
	wantNote(t, r, "_lDisplay", migrate.Mapped, "written as the ref part usage display System holding its 2 event occurrences, the interaction's own")
	wantNote(t, r, "_lOperator2", migrate.Mapped, "written as the ref part usage operator2 holding its 1 event occurrence, the interaction's own")
	wantNote(t, r, "_diag_seq", migrate.Mapped, "written as a view of SequenceView, drawn as a sequence diagram by its definition")
	wantNote(t, r, "_diag_seq", migrate.Mapped, "7 of 7 shown elements positioned, 6 of 6 connectors routed")
	wantNote(t, r, "_mNew", migrate.Approximated, "v2 sequence notation has no creation message")
	wantNote(t, r, "_mLog", migrate.Approximated, "the call names no operation")
}

// The migrated sequence view renders as a sequence diagram: the renderer draws the
// lifelines as the occurrence def's usages and the messages between their events in
// the modelled order, which is the order the interaction's occurrences state.
func TestUseCaseSequenceRendersInModelledOrder(t *testing.T) {
	r := migrateFixtureFile(t, "usecase_sequence")
	s := session(t, r)
	out := meta(t, s, "%render Model::'Acquire Images'::'Acquire Images'::'Acquire Images' text")
	if !strings.Contains(out, "sequence rendering") {
		t.Fatalf("the view is not rendered as a sequence:\n%s", out)
	}
	last := -1
	for _, want := range []string{
		"part operator", "part stage", "part vacuum", "part image", "part log", "part 'display System'", "part operator2",
		"messages:",
		"operator => stage: move",
		"stage => vacuum: pumpDown",
		"vacuum => image: 'new'",
		"image => log: record",
		"image => 'display System': show",
		"'display System' => operator2: confirm",
	} {
		at := strings.Index(out, want)
		if at < 0 {
			t.Errorf("the rendering lacks %q:\n%s", want, out)
			continue
		}
		if at < last {
			t.Errorf("%q is drawn out of order:\n%s", want, out)
		}
		last = at
	}
}
