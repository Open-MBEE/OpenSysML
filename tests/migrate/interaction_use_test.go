package migrate_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/interaction_use.xmi: an interaction use is an occurrence usage of the referenced
// interaction's def at its place among the messages, ordered by successions from the covered
// lifelines' events before it and to those after it, its arguments bound to the attributes the
// referenced interaction's parameters are written as; a message through an actual gate reaches the
// usage; a state invariant is an event occurrence of its lifeline asserting the condition. A use of
// nothing or of the interaction itself is refused, and so is an invariant whose condition is prose
// or reads the interaction's context.
func TestInteractionUsesAndStateInvariants(t *testing.T) {
	r := migrateFixtureFile(t, "interaction_use")
	for _, line := range []string{
		"occurrence def 'Spin Up' {",
		"attribute rpm : ScalarValues::Real;",
		"perform action spinCall ::> Motor::spin { in rpm[1] = 'Spin Up'::rpm; }",
		"message spin from c.spinSent to m.spinReceived;",
		"occurrence def Run {",
		"event occurrence at30 {",
		"assert constraint { speed == 30.0 }",
		"then event occurrence slowReceived {",
		"perform action slow ::> Motor::spin { in rpm[1] = 5.0; }",
		"occurrence warm : 'Spin Up' { attribute :>> rpm = 30.0; }",
		"succession first warm then m.at30;",
		"succession first warm then c.slowSent;",
		"message slow from c.slowSent to m.slowReceived;",
		"occurrence def Overrun {",
		"succession first c.spinSent then 'spin Up';",
		"succession first m.spinReceived then 'spin Up';",
		"occurrence 'spin Up' : 'Spin Up' { attribute :>> rpm = 5.0; }",
		"occurrence def Relay {",
		"event occurrence at20 {",
		"assert constraint { speed == 20.0 }",
		"message cmd from c.cmdSent to kick;",
		"occurrence kick : Drive::Kick;",
		"succession first kick then m.at20;",
		"occurrence run : Run {",
		"ref occurrence :>> m = drive.motor;",
		"/* not migrated: Interaction 'Astray' — the interaction use (_aUse) refers to no interaction (1 refersTo reference(s) resolve to nothing in the document (_gone)) */",
		"/* not migrated: Interaction 'Again' — the interaction use (_gUse) refers to 'Again', which is not migrated: the interaction refers to itself through an interaction use */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "perform action warm")
	wantNote(t, r, "_suRpm", migrate.Approximated, "written as the attribute rpm of the occurrence def, which declares no parameter; the occurrence usages of the def bind it")
	wantNote(t, r, "_runUse", migrate.Mapped, "written as the occurrence usage warm of 'Spin Up'")
	wantNote(t, r, "_oUse", migrate.Mapped, "written as the occurrence usage 'spin Up' of 'Spin Up'; ordered after c.spinSent, m.spinReceived and before the covered lifelines' next events by successions")
	wantNote(t, r, "_ryUse", migrate.Mapped, "written as the occurrence usage kick of 'Kick'")
	wantNote(t, r, "_ryCmd", migrate.Approximated, "the arguments it carries into the gate are those the message 'kickSpin' of 'Kick' states")
	wantNote(t, r, "_kickIn", migrate.Mapped, "the gate is the interaction's edge: a message leaving or entering it states no end there, as v2 sequence notation has no gate")
	wantNote(t, r, "_runAt30", migrate.Mapped, "written as the event occurrence m.at30, which asserts the condition where the invariant holds")
	wantNote(t, r, "_runAt30C", migrate.Mapped, "the constraint is the condition asserted at the event occurrence m.at30")
	wantNote(t, r, "_suFast", migrate.Unmapped, "the state invariant's condition [speed > limit] is not written: names limit, which nothing visible from Motor is called")
	wantNote(t, r, "_runSlowed", migrate.Unmapped, "reads the interaction's context, which its occurrence def has no access to; only the covered lifeline's object m is readable")
	wantNote(t, r, "_runProse", migrate.Unmapped, "the state invariant's condition [{English} the motor runs quietly] is not written")
	wantNote(t, r, "_run", migrate.Mapped, "written as an occurrence def of 2 lifeline usages and 1 message between their event occurrences in occurrence order")
	wantNote(t, r, "_again", migrate.Unmapped, "the interaction refers to itself through an interaction use")
	wantNote(t, r, "_astray", migrate.Unmapped, "refers to no interaction")
	wantClean(t, "interaction_use.sysml", r)
}

// A message received on the actual gate of an interaction use another interaction
// owns refuses its own interaction rather than writing a gate it cannot reach.
func TestMessageThroughAnotherInteractionsGate(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/interaction_use.xmi")
	if err != nil {
		t.Fatal(err)
	}
	foreign := bytes.Replace(data, []byte(`sendEvent="_oS" receiveEvent="_oR"`), []byte(`sendEvent="_oS" receiveEvent="_ryGate"`), 1)
	if bytes.Equal(foreign, data) {
		t.Fatal("the fixture no longer has the message to redirect")
	}
	r, err := migrate.Migrate("interaction_use.xmi", foreign)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantNote(t, r, "_overrun", migrate.Unmapped, "is received on the gate 'cmd' of 'kick', which is not an interaction use of the interaction")
	wantNote(t, r, "_ryUse", migrate.Mapped, "written as the occurrence usage kick of 'Kick'")
	for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// A message entering an interaction use's gate is carried by the performed scenario, so one
// sent with another message between it and the use is refused rather than reordered.
func TestGateMessageSentBeforeAnInterveningMessage(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/interaction_use.xmi")
	if err != nil {
		t.Fatal(err)
	}
	send := `<fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ryS" covered="_rylC" message="_ryCmd"/>`
	between := bytes.Replace(data, []byte(send), []byte(send+`
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ryPS" covered="_rylC" message="_ryPre"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ryPR" covered="_rylM" message="_ryPre"/>`), 1)
	carried := `<argument xmi:type="uml:LiteralReal" xmi:id="_ryCmdRpm" value="20.0"/>
        </message>`
	between = bytes.Replace(between, []byte(carried), []byte(carried+`
        <message xmi:type="uml:Message" xmi:id="_ryPre" name="pre" messageSort="synchCall" signature="_spin" sendEvent="_ryPS" receiveEvent="_ryPR">
          <argument xmi:type="uml:LiteralReal" xmi:id="_ryPreRpm" value="5.0"/>
        </message>`), 1)
	if !bytes.Contains(between, []byte(`xmi:id="_ryPS"`)) || !bytes.Contains(between, []byte(`xmi:id="_ryPre"`)) {
		t.Fatal("the fixture no longer has the relay to put a message into")
	}
	r, err := migrate.Migrate("interaction_use.xmi", between)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantNote(t, r, "_relay", migrate.Unmapped, "the message 'cmd' enters the interaction use 'kick' through its gate but is not sent just before it")
	for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// A state invariant on a lifeline reads that lifeline's object: a name the object's type owns
// but that has no v2 declaration is refused rather than read on the context's feature of that name.
func TestLifelineInvariantDoesNotReadTheContextsFeature(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/interaction_use.xmi")
	if err != nil {
		t.Fatal(err)
	}
	edits := []struct{ old, new string }{
		{`<packagedElement xmi:type="uml:Class" xmi:id="_ctrl" name="Controller"/>`, `<packagedElement xmi:type="uml:Class" xmi:id="_ctrl" name="Controller"/>
    <packagedElement xmi:type="uml:Profile" xmi:id="_kit" name="StandardProfile">
      <packagedElement xmi:type="uml:Class" xmi:id="_spare" name="Spare">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_spareLimit" name="limit">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedAttribute>
      </packagedElement>
    </packagedElement>`},
		{`<ownedAttribute xmi:type="uml:Property" xmi:id="_rCtrl" name="ctrl" type="_ctrl" aggregation="composite"/>`, `<ownedAttribute xmi:type="uml:Property" xmi:id="_rCtrl" name="ctrl" type="_ctrl" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rAux" name="aux" type="_spare" aggregation="composite"/>`},
		{`<lifeline xmi:type="uml:Lifeline" xmi:id="_rlM" name="m" represents="_dMotor"/>`, `<lifeline xmi:type="uml:Lifeline" xmi:id="_rlM" name="m" represents="_dMotor"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_rlA" name="a" represents="_rAux"/>`},
		{`xmi:id="_runAt30" name="at30" covered="_rlM"`, `xmi:id="_runAt30" name="at30" covered="_rlA"`},
		{`<body>speed == 30.0</body>`, `<body>limit == 10.0</body>`},
	}
	for _, e := range edits {
		next := bytes.Replace(data, []byte(e.old), []byte(e.new), 1)
		if bytes.Equal(next, data) {
			t.Fatalf("the fixture no longer has %s", e.old)
		}
		data = next
	}
	r, err := migrate.Migrate("interaction_use.xmi", data)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantNote(t, r, "_runAt30", migrate.Unmapped, "the state invariant's condition [limit == 10.0] reads 'limit' of the lifeline's object aux, which has no v2 declaration")
	if bytes.Contains(r.Notation, []byte("assert constraint at30")) {
		t.Errorf("the invariant is written as an assertion on the context's limit:\n%s", r.Notation)
	}
	for _, d := range errors(t, "interaction_use.sysml", r.Notation) {
		t.Errorf("%v", d)
	}
}

// A seq fragment's operands are written inline, so a gate message ending one still
// enters the interaction use that follows the fragment directly.
func TestGateMessageEndingASeqOperand(t *testing.T) {
	data, err := os.ReadFile("testdata/xmi/interaction_use.xmi")
	if err != nil {
		t.Fatal(err)
	}
	send := `<fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ryS" covered="_rylC" message="_ryCmd"/>`
	wrapped := bytes.Replace(data, []byte(send), []byte(`<fragment xmi:type="uml:CombinedFragment" xmi:id="_rySeq" interactionOperator="seq" covered="_rylC">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_rySeqOp">
            `+send+`
          </operand>
        </fragment>`), 1)
	if bytes.Equal(wrapped, data) {
		t.Fatal("the fixture no longer has the relay's gate message")
	}
	r, err := migrate.Migrate("interaction_use.xmi", wrapped)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	wantNote(t, r, "_ryCmd", migrate.Approximated, "the arguments it carries into the gate are those the message 'kickSpin' of 'Kick' states")
	if !bytes.Contains(r.Notation, []byte("message cmd from c.cmdSent to kick;")) {
		t.Errorf("the relay's message does not reach kick:\n%s", r.Notation)
	}
	wantClean(t, "interaction_use.sysml", r)
}
