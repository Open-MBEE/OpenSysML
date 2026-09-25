package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// sensorMachine is a block whose state machine has an initial transition
// carrying a trigger and a guard, a state whose only content is an invariant,
// a submachine state with entry and exit behaviors, a signal-triggered
// transition whose effect takes the signal through a parameter named unlike
// the signal beside one the signal cannot fill, and one whose effect takes it
// through an untyped parameter. The submachine's state runs a do activity whose
// parameter feeds a call, which a state passes no value to.
const sensorMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_reading" name="Reading">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rval" name="value">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_readEv" signal="_reading"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_sensor" name="Sensor" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_last" name="last">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_last0" value="0.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_count" name="count">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_count0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Ctl">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle">
            <stateInvariant xmi:type="uml:Constraint" xmi:id="_inv">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_invX"><body>last >= 0.0</body></specification>
            </stateInvariant>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_cool" name="Cool" submachine="_cooling">
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_coolEntry">
              <language>JavaScript</language>
              <body>count = count + 10;</body>
            </entry>
            <exit xmi:type="uml:OpaqueBehavior" xmi:id="_coolExit">
              <language>JavaScript</language>
              <body>count = count + 100;</body>
            </exit>
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="_fin"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr0" event="_readEv"/>
            <guard xmi:type="uml:Constraint" xmi:id="_g0">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_g0X"><body>count > 0</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_t1" source="_idle" target="_cool">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr1" event="_readEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_eff1">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_p1" name="r" direction="in" type="_reading"/>
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_p2" name="n" direction="in">
                <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
              </ownedParameter>
              <language>JavaScript</language>
              <body>last = r.value;</body>
            </effect>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_t2" source="_cool" target="_fin">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr2" event="_readEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_eff2">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_p3" name="reading" direction="in"/>
              <language>JavaScript</language>
              <body>count = count + 1;</body>
            </effect>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_cooling" name="Cooling">
        <region xmi:type="uml:Region" xmi:id="_r1">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init1"/>
          <subvertex xmi:type="uml:State" xmi:id="_fan" name="Fan">
            <doActivity xmi:type="uml:Activity" xmi:id="_spin" name="Spin">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_speed" name="speed" direction="in">
                <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
              </ownedParameter>
              <node xmi:type="uml:ActivityParameterNode" xmi:id="_speedN" name="speed" parameter="_speed"/>
              <node xmi:type="uml:InitialNode" xmi:id="_spinI"/>
              <node xmi:type="uml:CallBehaviorAction" xmi:id="_take" name="take" behavior="_sample">
                <argument xmi:type="uml:InputPin" xmi:id="_takeIn" name="v"/>
              </node>
              <node xmi:type="uml:ActivityFinalNode" xmi:id="_spinF"/>
              <edge xmi:type="uml:ControlFlow" xmi:id="_spinE1" source="_spinI" target="_take"/>
              <edge xmi:type="uml:ObjectFlow" xmi:id="_spinE2" source="_speedN" target="_takeIn"/>
              <edge xmi:type="uml:ControlFlow" xmi:id="_spinE3" source="_take" target="_spinF"/>
            </doActivity>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_t1i" source="_init1" target="_fan"/>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_sample" name="Sample">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_sampleV" name="v" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedBehavior>
    </packagedElement>`

const sensorApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_sensor"/>`

// A transition's effect takes the accepted signal through its parameter: the
// accept clause names the signal, and the parameter typed by it, or the sole
// untyped one, is bound to that name, while one of another type is noted as
// taking no value. A submachine state keeps its entry and exit actions under
// its type, a state with only an invariant keeps it as a comment, and an
// initial transition's dropped trigger and guard each get a report entry. The
// result runs: the effect reads the signal's value, the submachine state's
// entry and exit actions count, and the machine completes. A do activity's call
// fed only by a parameter the state passes no value to never fires, so no
// succession reaches it and the do activity completes instead of failing.
func TestTransitionEffectsTakeTheAcceptedSignal(t *testing.T) {
	r := migrateDocument(t, sensorMachine, sensorApplications)
	for _, line := range []string{
		"state def Ctl {",
		"entry; then Idle;",
		"state Idle {",
		"/* not migrated: Constraint (_inv) — a state invariant has no v2 form; [last >= 0.0] is kept as a comment */",
		"state Cool : Cooling {",
		"assign this.count := this.count + 10;",
		"assign this.count := this.count + 100;",
		"transition first Idle accept reading : Reading",
		"in r : Reading = reading;",
		"in n : ScalarValues::Integer;",
		"assign this.last := r.value;",
		"then Cool;",
		"transition first Cool accept reading2 : Reading",
		"in reading = reading2;",
		"assign this.count := this.count + 1;",
		"then done;",
		"do action Spin {",
		"in speed : ScalarValues::Real;",
		"action take : Sample;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "then take;") || strings.Contains(string(r.Notation), "first take then") {
		t.Errorf("a succession reaches or leaves the call whose input takes no value:\n%s", r.Notation)
	}
	if strings.Contains(string(r.Notation), "state Idle;") || strings.Contains(string(r.Notation), "state Cool : Cooling;") {
		t.Errorf("a state lost its invariant or its actions:\n%s", r.Notation)
	}
	wantNote(t, r, "_t0", migrate.Approximated, "an initial transition takes no trigger; its triggers are dropped; an initial transition takes no guard; [count > 0] is dropped")
	wantNote(t, r, "_tr0", migrate.Unmapped, "a trigger of an initial transition, which takes none, is dropped")
	wantNote(t, r, "_g0", migrate.Unmapped, "a guard of an initial transition, which takes none, is dropped")
	wantNote(t, r, "_readEv", migrate.Mapped, "written where a trigger refers to it, as accept reading : Reading")
	wantNote(t, r, "_idle", migrate.Mapped, "")
	wantNote(t, r, "_inv", migrate.Unmapped, "a state invariant has no v2 form; [last >= 0.0] is kept as a comment")
	wantNote(t, r, "_cool", migrate.Mapped, "")
	wantNote(t, r, "_coolEntry", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_coolExit", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_p1", migrate.Mapped, "bound to reading, the signal the transition accepts")
	wantNote(t, r, "_p2", migrate.Approximated, "the parameter takes no value: the transition passes only the accepted Reading, which is no Integer")
	wantNote(t, r, "_p3", migrate.Mapped, "bound to reading2, the signal the transition accepts")
	wantNote(t, r, "_speed", migrate.Approximated, "the parameter takes no value: a state performs its do action with no arguments; the signal the transitions into the state accept would value them, but the transition from the initial pseudostate (_init1) enters the state with no signal of its own")
	wantNote(t, r, "_spinE1", migrate.Unmapped, "the edge leads to 'take', which never fires: no value reaches its input pin 'v'")
	wantNote(t, r, "_spinE3", migrate.Unmapped, "the edge leaves 'take', which never fires, so no token travels it")
	starved := false
	for _, e := range entriesFor(r, "_take") {
		starved = starved || e.Verdict == migrate.Approximated && strings.Contains(e.Note, "the action never fires: its input pin 'v' must hold a value, but only parameters taking no value flow into it")
	}
	if !starved {
		t.Errorf("entries for _take = %+v, want one noting the call never fires", entriesFor(r, "_take"))
	}

	s := session(t, r)
	meta(t, s, "%instantiate Sensor")
	meta(t, s, "%state Sensor::Ctl")
	if out := meta(t, s, "%send Reading(value=4.5)"); !strings.Contains(out, "transition Idle -> Cool fires on it") {
		t.Errorf("%%send Reading: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Fan") || !strings.Contains(out, "0. Cool") {
		t.Errorf("the machine is not in Cool::Fan:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : last"); !strings.Contains(out, "= 4.5") {
		t.Errorf("the effect did not read the signal's value: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : count"); !strings.Contains(out, "= 10") {
		t.Errorf("the submachine state's entry action did not run: %s", out)
	}
	if out := meta(t, s, "%send Reading(value=1.0)"); !strings.Contains(out, "transition Cool -> done fires on it") {
		t.Errorf("%%send Reading: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: done") || !strings.Contains(out, "Execution state: Completed") {
		t.Errorf("the machine did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : count"); !strings.Contains(out, "= 111") {
		t.Errorf("the submachine state's exit action and the effect did not count: %s", out)
	}
}

// bumpActivity is an activity with an inout parameter it increments through a
// function behavior, and a driver calling it whose pins carry the parameter
// in and out again.
const bumpActivity = `
    <packagedElement xmi:type="uml:FunctionBehavior" xmi:id="_inc" name="Inc">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_iv" name="v" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_ir" name="result" direction="return">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedParameter>
      <body>v + 1</body>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_bump" name="Bump">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_bn" name="n" direction="inout">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_bnIn" name="n" parameter="_bn"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_bnOut" name="n" parameter="_bn"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callInc" name="inc" behavior="_inc">
        <argument xmi:type="uml:InputPin" xmi:id="_incIn" name="v"/>
        <result xmi:type="uml:OutputPin" xmi:id="_incOut" name="result"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_bOf1" source="_bnIn" target="_incIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_bOf2" source="_incOut" target="_bnOut"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_run" name="Run">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_rOut" name="answer" direction="out">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_rOutN" name="answer" parameter="_rOut"/>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_seedV" name="seed">
        <value xmi:type="uml:LiteralInteger" xmi:id="_seedL" value="5"/>
        <result xmi:type="uml:OutputPin" xmi:id="_seedOut" name="result"/>
      </node>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callB" name="bump" behavior="_bump">
        <argument xmi:type="uml:InputPin" xmi:id="_callBIn" name="n"/>
        <result xmi:type="uml:OutputPin" xmi:id="_callBOut" name="n"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_rOf1" source="_seedOut" target="_callBIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_rOf2" source="_callBOut" target="_rOutN"/>
    </packagedElement>`

// An inout parameter stands behind both a call's input pin and its output pin:
// the flow into the call binds the parameter and the flow out of it reads the
// parameter back, so the driver gets the incremented value.
func TestInoutParameterFlowsInAndOutOfACall(t *testing.T) {
	r := migrateDocument(t, bumpActivity, "")
	for _, line := range []string{
		"action def Bump {",
		"inout n : ScalarValues::Integer;",
		"bind inc.v = n;",
		"bind n = inc.result;",
		"action def Run {",
		"action bump : Bump;",
		"flow seed.result to bump.n;",
		"bind answer = bump.n;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_callBIn", migrate.Mapped, "the pin stands for the parameter n of the definition")
	wantNote(t, r, "_callBOut", migrate.Mapped, "the pin stands for the parameter n of the definition")
	wantNote(t, r, "_rOf2", migrate.Mapped, "")

	s := session(t, r)
	v := s.RunAction("Run")
	wantVerdict(t, v)
	if out := strings.Join(v.Lines, "\n"); !strings.Contains(out, "answer = 6") {
		t.Errorf("the call did not hand its inout parameter back:\n%s", out)
	}
}

// twoFeeds is an activity whose call takes its input from either of two
// values, each by an object flow of its own into the same pin; a third edge
// repeats the first.
const twoFeeds = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_show" name="Show">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_sv" name="v" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_svN" name="v" parameter="_sv"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_pick" name="Pick">
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
        <value xmi:type="uml:LiteralInteger" xmi:id="_oneL" value="1"/>
        <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
      </node>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_two" name="two">
        <value xmi:type="uml:LiteralInteger" xmi:id="_twoL" value="2"/>
        <result xmi:type="uml:OutputPin" xmi:id="_twoOut" name="result"/>
      </node>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callShow" name="show" behavior="_show">
        <argument xmi:type="uml:InputPin" xmi:id="_showIn" name="v"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_pOf1" source="_oneOut" target="_showIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_pOf2" source="_twoOut" target="_showIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_pOf3" source="_oneOut" target="_showIn"/>
    </packagedElement>`

// Each object flow writes the flows from its own source, so two edges into one
// pin write one flow each rather than both twice, and an edge repeating another
// is reported rather than written again.
func TestFlowsIntoOnePinAreWrittenOnceEach(t *testing.T) {
	r := migrateDocument(t, twoFeeds, "")
	for _, line := range []string{"flow one.result to show.v;", "flow two.result to show.v;"} {
		if n := strings.Count(string(r.Notation), line); n != 1 {
			t.Errorf("%q written %d times, want once:\n%s", line, n, r.Notation)
		}
	}
	wantNote(t, r, "_pOf1", migrate.Mapped, "")
	wantNote(t, r, "_pOf2", migrate.Mapped, "")
	wantNote(t, r, "_pOf3", migrate.Mapped, "the flow from one.result to show.v is written once, though several edges carry it")
}

// pointOperation is a block whose operation Point(in azimuth) has an activity
// method whose parameter is called angle, plus an operation whose method takes
// a second parameter the operation lacks and a first one of another direction.
const pointOperation = `
    <packagedElement xmi:type="uml:Class" xmi:id="_scope" name="Scope">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_az" name="azimuth">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_az0" value="0.0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_point" name="Point" method="_pointing">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_pAz" name="azimuth" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_pointing" name="Pointing" specification="_point">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_pAngle" name="angle" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_apn" name="angle" parameter="_pAngle"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set azimuth" structuralFeature="_az" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of0" source="_apn" target="_setVal"/>
      </ownedBehavior>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_tilt" name="Tilt" method="_tilting">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tBy" name="amount" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_tilting" name="Tilting" specification="_tilt">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tOut" name="was" direction="out">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tExtra" name="extra" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_aim" name="Aim">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_ninety" name="ninety">
          <value xmi:type="uml:LiteralReal" xmi:id="_ninetyV" value="90.0"/>
          <result xmi:type="uml:OutputPin" xmi:id="_ninetyOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_call" name="point" operation="_point">
          <argument xmi:type="uml:InputPin" xmi:id="_callAz" name="azimuth"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_ninety"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of2" source="_ninetyOut" target="_callAz"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_call" target="_final"/>
      </ownedBehavior>
    </packagedElement>`

const pointApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_scope"/>`

// A method's parameter stands for the operation's at the same position, whatever
// it is called: the action def declares the operation's signature alone, the
// method's parameter nodes name the operation's parameter, and a call binds it.
// A method parameter matching none of the operation's is declared and reported.
func TestMethodParametersStandForTheOperationsByPosition(t *testing.T) {
	r := migrateDocument(t, pointOperation, pointApplications)
	for _, line := range []string{
		"action def Point {",
		"in azimuth : ScalarValues::Real;",
		"bind 'set azimuth'.value = azimuth;",
		"action def Tilt {",
		"in amount : ScalarValues::Real;",
		"out was : ScalarValues::Real;",
		"in extra : ScalarValues::Real;",
		"flow ninety.result to point.azimuth;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "angle") {
		t.Errorf("the method's own parameter name was written:\n%s", r.Notation)
	}
	wantNote(t, r, "_pAngle", migrate.Mapped, "stands for the operation's parameter azimuth at the same position, which is written once")
	wantNote(t, r, "_tOut", migrate.Approximated, "the method's parameter matches none of the operation's by position, direction and type; it is declared after them, and a call binds it there")
	wantNote(t, r, "_tExtra", migrate.Approximated, "the method's parameter matches none of the operation's by position, direction and type; it is declared after them, and a call binds it there")

	s := session(t, r)
	meta(t, s, "%instantiate Scope")
	meta(t, s, "%action Scope::Aim #1")
	meta(t, s, "%continue")
	if out := meta(t, s, "%features #1"); !strings.Contains(out, "azimuth = 90.0") {
		t.Errorf("the call did not reach the method's body through the operation's parameter:\n%s", out)
	}
}

// methodInputs is a block whose operation Tilt(in amount) has a method taking a
// second required input the operation lacks, called twice from an activity — once
// with the operation's argument alone, once with both — and twice from
// interactions of a block owning it, with the same two argument lists.
const methodInputs = `
    <packagedElement xmi:type="uml:Class" xmi:id="_tilter" name="Tilter">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_reach" name="reach">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_reach0" value="0.0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_tilt" name="Tilt" method="_tilting">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tAmount" name="amount" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_tilting" name="Tilting" specification="_tilt">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tAngle" name="angle" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tExtra" name="extra" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_tApn" name="extra" parameter="_tExtra"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_tSet" name="set reach" structuralFeature="_reach" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_tSetVal" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_tOf" source="_tApn" target="_tSetVal"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_aim" name="Aim">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_ninety" name="ninety">
          <value xmi:type="uml:LiteralReal" xmi:id="_ninetyV" value="90.0"/>
          <result xmi:type="uml:OutputPin" xmi:id="_ninetyOut" name="result"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_seven" name="seven">
          <value xmi:type="uml:LiteralReal" xmi:id="_sevenV" value="7.0"/>
          <result xmi:type="uml:OutputPin" xmi:id="_sevenOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_short" name="short" operation="_tilt">
          <argument xmi:type="uml:InputPin" xmi:id="_shortAmount" name="amount"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_full" name="full" operation="_tilt">
          <argument xmi:type="uml:InputPin" xmi:id="_fullAmount" name="amount"/>
          <argument xmi:type="uml:InputPin" xmi:id="_fullExtra" name="extra"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_ninety"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_ninetyOut" target="_shortAmount"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_short" target="_seven"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of2" source="_sevenOut" target="_fullAmount"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of3" source="_sevenOut" target="_fullExtra"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_full" target="_final"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_operator" name="Operator"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rigOp" name="op" type="_operator" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rigTilter" name="tilter" type="_tilter" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_brief" name="Brief">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_blo" name="o" represents="_rigOp"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_blt" name="t" represents="_rigTilter"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_bs" covered="_blo" message="_bm"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_br" covered="_blt" message="_bm"/>
        <message xmi:type="uml:Message" xmi:id="_bm" name="tilt" messageSort="synchCall" signature="_tilt" sendEvent="_bs" receiveEvent="_br">
          <argument xmi:type="uml:LiteralReal" xmi:id="_bmAmount" value="90.0"/>
        </message>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_whole" name="Whole">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_wlo" name="o" represents="_rigOp"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_wlt" name="t" represents="_rigTilter"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_ws" covered="_wlo" message="_wm"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_wr" covered="_wlt" message="_wm"/>
        <message xmi:type="uml:Message" xmi:id="_wm" name="tilt" messageSort="synchCall" signature="_tilt" sendEvent="_ws" receiveEvent="_wr">
          <argument xmi:type="uml:LiteralReal" xmi:id="_wmAmount" value="90.0"/>
          <argument xmi:type="uml:LiteralReal" xmi:id="_wmExtra" value="7.0"/>
        </message>
      </ownedBehavior>
    </packagedElement>`

const methodInputsApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_tilter"/>
  <sysml:Block xmi:id="_s2" base_Class="_operator"/>
  <sysml:Block xmi:id="_s3" base_Class="_rig"/>`

// A call binds the signature the action def declares — the operation's parameters
// and the method's extra ones — so a call supplying the operation's alone is
// refused for the method's required input, and one supplying both runs the method.
func TestCallsBindTheMethodsExtraParameters(t *testing.T) {
	r := migrateDocument(t, methodInputs, methodInputsApplications)
	for _, line := range []string{
		"action def Tilt {",
		"in amount : ScalarValues::Real;",
		"in extra : ScalarValues::Real[0..1];",
		"bind 'set reach'.value = extra;",
		"action short : Tilt;",
		"action full : Tilt;",
		"flow ninety.result to short.amount;",
		"flow seven.result to full.amount;",
		"flow seven.result to full.extra;",
		"/* not migrated: Interaction 'Brief' — the message 'tilt' binds no argument to the parameter extra of Tilt, which must hold a value */",
		"perform action tilt : Tilter::Tilt ::> tilter.tilt { in amount = 90.0; in extra = 7.0; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "not migrated: CallOperationAction 'short'")
	wantNote(t, r, "_short", migrate.Approximated, "the call passes no argument for the parameter extra of Tilter::Tilt; v1 runs the callee without the value, so the parameter is declared admitting none")
	wantNote(t, r, "_tExtra", migrate.Approximated, "it is declared admitting no value: the call 'short' in Tilter::Aim passes no argument for it, and v1 runs the callee without one")
	wantNote(t, r, "_full", migrate.Mapped, "")
	wantNote(t, r, "_brief", migrate.Unmapped, "the message 'tilt' binds no argument to the parameter extra of Tilt, which must hold a value")
	wantNote(t, r, "_wm", migrate.Mapped, "")

	s := session(t, r)
	meta(t, s, "%instantiate Tilter")
	meta(t, s, "%action Tilter::Aim #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
	if out := meta(t, s, "%features #1"); !strings.Contains(out, "reach = 7.0") {
		t.Errorf("the full call did not reach the method's body through its extra parameter:\n%s", out)
	}
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%action Rig::Whole #2")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the interaction did not complete:\n%s", out)
	}
	if out := meta(t, s, "%features #2"); !strings.Contains(out, "reach = 7.0") {
		t.Errorf("the message did not reach the method's body through its extra parameter:\n%s", out)
	}
}
