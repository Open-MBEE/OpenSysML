package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// heaterMachine enters Heating on SetPoint from two states, once with an effect; Heating's
// entry and do behaviors take the signal's properties as parameters. Idle's do parameter gets nothing.
const heaterMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_setPoint" name="SetPoint">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_spLevel" name="level">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_spHold" name="hold">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_spEv" signal="_setPoint"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_heater" name="Heater" classifierBehavior="_hsm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_hlast" name="last">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_hlast0" value="0.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_hsets" name="sets">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_hsets0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_hsm" name="Ctl">
        <region xmi:type="uml:Region" xmi:id="_hr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_hinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_hidle" name="Idle">
            <doActivity xmi:type="uml:OpaqueBehavior" xmi:id="_idleDo">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_idleP" name="why" direction="in" type="_stop"/>
              <language>JavaScript</language>
              <body>sets = sets;</body>
            </doActivity>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_hoff" name="Off"/>
          <subvertex xmi:type="uml:State" xmi:id="_heating" name="Heating">
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_prime">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_primeL" name="target" direction="in">
                <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
              </ownedParameter>
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_primeH" name="keep" direction="in">
                <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
              </ownedParameter>
              <language>JavaScript</language>
              <body>last = target;</body>
            </entry>
            <doActivity xmi:type="uml:Activity" xmi:id="_run" name="Run">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_runL" name="level" direction="in">
                <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
              </ownedParameter>
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_runH" name="hold" direction="in">
                <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
              </ownedParameter>
              <node xmi:type="uml:ActivityParameterNode" xmi:id="_runLN" name="level" parameter="_runL"/>
              <node xmi:type="uml:ActivityParameterNode" xmi:id="_runHN" name="hold" parameter="_runH"/>
              <node xmi:type="uml:InitialNode" xmi:id="_runI"/>
              <node xmi:type="uml:CallBehaviorAction" xmi:id="_warm" name="warm" behavior="_warmB">
                <argument xmi:type="uml:InputPin" xmi:id="_warmL" name="l"/>
                <argument xmi:type="uml:InputPin" xmi:id="_warmH" name="h"/>
              </node>
              <node xmi:type="uml:ActivityFinalNode" xmi:id="_runF"/>
              <edge xmi:type="uml:ControlFlow" xmi:id="_runE1" source="_runI" target="_warm"/>
              <edge xmi:type="uml:ObjectFlow" xmi:id="_runE2" source="_runLN" target="_warmL"/>
              <edge xmi:type="uml:ObjectFlow" xmi:id="_runE3" source="_runHN" target="_warmH"/>
              <edge xmi:type="uml:ControlFlow" xmi:id="_runE4" source="_warm" target="_runF"/>
            </doActivity>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_ht0" source="_hinit" target="_hidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_ht1" source="_hidle" target="_heating">
            <trigger xmi:type="uml:Trigger" xmi:id="_htr1" event="_spEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_ht2" source="_heating" target="_hoff">
            <trigger xmi:type="uml:Trigger" xmi:id="_htr2" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_ht3" source="_hoff" target="_heating">
            <trigger xmi:type="uml:Trigger" xmi:id="_htr3" event="_spEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_heff3">
              <language>JavaScript</language>
              <body>sets = sets + 1;</body>
            </effect>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_ht4" source="_hoff" target="_hidle">
            <trigger xmi:type="uml:Trigger" xmi:id="_htr4" event="_stopEv"/>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_warmB" name="Warm">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_warmBL" name="l" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_warmBH" name="h" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
        </ownedParameter>
      </ownedBehavior>
    </packagedElement>`

const heaterApplications = `
  <sysml:Block xmi:id="_h1" base_Class="_heater"/>`

// Heating's transitions keep the accepted SetPoint in an item of the state def that
// its entry and do parameters bind to; Idle, entered by the initial transition, says why not.
func TestStateBehaviorsTakeTheSignalTheirTransitionsAccept(t *testing.T) {
	r := migrateDocument(t, heaterMachine, heaterApplications)
	for _, line := range []string{
		"state def Ctl {",
		"item setPoint : SetPoint;",
		"transition first Idle accept setPoint2 : SetPoint",
		"assign setPoint := setPoint2;",
		"then Heating;",
		"transition first Off accept setPoint2 : SetPoint",
		"assign this.sets := this.sets + 1;",
		"state Heating {",
		"in target : ScalarValues::Real = setPoint.level;",
		"in keep : ScalarValues::Boolean = setPoint.hold;",
		"assign this.last := target;",
		"do action Run {",
		"in level : ScalarValues::Real = setPoint.level;",
		"in hold : ScalarValues::Boolean = setPoint.hold;",
		"action warm : Warm;",
		"bind warm.l = level;",
		"bind warm.h = hold;",
	} {
		wantLine(t, r.Notation, line)
	}
	if !strings.Contains(string(r.Notation), "then warm;") {
		t.Errorf("no succession reaches the call whose inputs the signal values:\n%s", r.Notation)
	}
	wantNote(t, r, "_heating", migrate.Approximated, "the parameters of its entry and do actions take the attributes of SetPoint, the signal every transition into it accepts and keeps in setPoint")
	wantNote(t, r, "_primeL", migrate.Mapped, "bound to setPoint.level, an attribute of the signal the transitions into the state accept")
	wantNote(t, r, "_runH", migrate.Mapped, "bound to setPoint.hold, an attribute of the signal the transitions into the state accept")
	wantNote(t, r, "_idleP", migrate.Approximated, "the parameter takes no value: a state performs its do action with no arguments; the signal the transitions into the state accept would value them, but the transition from the initial pseudostate (_hinit) enters the state with no signal of its own")

	s := session(t, r)
	meta(t, s, "%instantiate Heater")
	meta(t, s, "%state Heater::Ctl")
	if out := meta(t, s, "%send SetPoint(level=3.5, hold=true)"); !strings.Contains(out, "transition Idle -> Heating fires on it") {
		t.Errorf("%%send SetPoint: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Heating") {
		t.Errorf("the machine is not in Heating:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : last"); !strings.Contains(out, "= 3.5") {
		t.Errorf("the entry action did not read the signal's level: %s", out)
	}
	if out := meta(t, s, "%send Stop()"); !strings.Contains(out, "transition Heating -> Off fires on it") {
		t.Errorf("%%send Stop: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%send SetPoint(level=7.0, hold=false)"); !strings.Contains(out, "transition Off -> Heating fires on it") {
		t.Errorf("%%send SetPoint: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%eval in #1 : last"); !strings.Contains(out, "= 7.0") {
		t.Errorf("the entry action did not read the second signal's level: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : sets"); !strings.Contains(out, "= 1") {
		t.Errorf("the effect before keeping the signal did not run: %s", out)
	}
}
