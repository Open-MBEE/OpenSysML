package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// alarmSignals is a signal hierarchy: Alarm owns code, DetailedAlarm adds level,
// and LoudAlarm redefines code. A panel's activity sends a DetailedAlarm with
// both values and a LoudAlarm with its one, to itself; its state machine reads
// the code of the alarm it accepts. An interaction sends the same alarm.
const alarmSignals = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_alarm" name="Alarm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_code" name="code">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_detailed" name="DetailedAlarm">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen1" general="_alarm"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_level" name="level">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_loud" name="LoudAlarm">
      <generalization xmi:type="uml:Generalization" xmi:id="_gen2" general="_alarm"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_code2" name="code" redefinedProperty="_code">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_detailedEv" signal="_detailed"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_panel" name="Panel" classifierBehavior="_ctl">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_last" name="last">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_last0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_ctl" name="Ctl">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_t1" source="_idle" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr1" event="_detailedEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_eff1">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_p1" name="d" direction="in" type="_detailed"/>
              <language>JavaScript</language>
              <body>last = d.code;</body>
            </effect>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_raise" name="Raise">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
          <value xmi:type="uml:LiteralInteger" xmi:id="_oneV" value="1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_seven" name="seven">
          <value xmi:type="uml:LiteralInteger" xmi:id="_sevenV" value="7"/>
          <result xmi:type="uml:OutputPin" xmi:id="_sevenOut" name="result"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_nine" name="nine">
          <value xmi:type="uml:LiteralInteger" xmi:id="_nineV" value="9"/>
          <result xmi:type="uml:OutputPin" xmi:id="_nineOut" name="result"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send detailed" signal="_detailed">
          <argument xmi:type="uml:InputPin" xmi:id="_sendLevel" name="level"/>
          <argument xmi:type="uml:InputPin" xmi:id="_sendCode" name="code"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_sendLoud" name="send loud" signal="_loud">
          <argument xmi:type="uml:InputPin" xmi:id="_loudCode" name="code"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_one"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_one" target="_seven"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_seven" target="_nine"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_oneOut" target="_sendLevel"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of2" source="_sevenOut" target="_sendCode"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of3" source="_nineOut" target="_loudCode"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_send" target="_sendLoud"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e5" source="_sendLoud" target="_final"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_room" name="Room">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_p" name="p" type="_panel" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_q" name="q" type="_panel" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_drill" name="Drill">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lp" name="p" represents="_p" coveredBy="_sA"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lq" name="q" represents="_q" coveredBy="_rA"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sA" covered="_lp" message="_mA"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rA" covered="_lq" message="_mA"/>
        <message xmi:type="uml:Message" xmi:id="_mA" name="alarm" messageSort="asynchSignal" signature="_detailed" sendEvent="_sA" receiveEvent="_rA">
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mAlevel" value="2"/>
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mAcode" value="8"/>
        </message>
      </ownedBehavior>
    </packagedElement>`

const alarmApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_panel"/>
  <sysml:Block xmi:id="_s2" base_Class="_room"/>`

// A send binds its argument pins to the signal's attributes in constructor
// order, the signal's own first and then the inherited ones, a redefinition
// taking its target's place; an interaction's message arguments bind the same
// way. The result runs: the send builds the signal with the inherited code,
// and the panel's machine reads it.
func TestSignalArgumentsIncludeInheritedAttributes(t *testing.T) {
	r := migrateDocument(t, alarmSignals, alarmApplications)
	for _, line := range []string{
		"item def DetailedAlarm :> Alarm {",
		"send new DetailedAlarm(level, code);",
		"send new LoudAlarm(code);",
		"action alarm send new DetailedAlarm(level = 2, code = 8) to this.q;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "send new DetailedAlarm(level);") || strings.Contains(string(r.Notation), "send new LoudAlarm();") {
		t.Errorf("an argument for an inherited or redefined attribute was dropped:\n%s", r.Notation)
	}
	wantNote(t, r, "_mA", migrate.Mapped, "written as a send to the part q")

	s := session(t, r)
	meta(t, s, "%instantiate Panel")
	meta(t, s, "%state Panel::Ctl")
	meta(t, s, "%action Panel::Raise #1")
	meta(t, s, "%continue")
	if out := meta(t, s, "%events"); !strings.Contains(out, "DetailedAlarm(code=7, level=1)") {
		t.Errorf("the send did not build the signal with the inherited code: %s", out)
	}
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : last"); !strings.Contains(out, "= 7") {
		t.Errorf("the machine did not read the inherited code the send carried: %s", out)
	}
}
