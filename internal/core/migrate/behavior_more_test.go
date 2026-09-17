package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// ovenMachine is a block whose classifier behavior is a state machine with a
// state of two orthogonal regions, entry and exit behaviors, a guarded pair of
// transitions leaving a choice, one guard that is a v2 expression and one that
// is not, a change event on an attribute an operation writes, an internal
// transition, an absolute time event and a transition with two triggers.
const ovenMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_on" name="TurnOn"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_off" name="TurnOff"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_door" name="Door"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_onEv" signal="_on"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_offEv" signal="_off"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_doorEv" signal="_door"/>
    <packagedElement xmi:type="uml:ChangeEvent" xmi:id="_hotEv">
      <changeExpression xmi:type="uml:OpaqueExpression" xmi:id="_hotX"><body>temperature > 200.0</body></changeExpression>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_noon" isRelative="false">
      <when xmi:type="uml:TimeExpression" xmi:id="_noonw">
        <expr xmi:type="uml:LiteralString" xmi:id="_noonl" value="12h"/>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_tick" isRelative="true">
      <when xmi:type="uml:TimeExpression" xmi:id="_tickw">
        <expr xmi:type="uml:LiteralString" xmi:id="_tickl" value="500ms"/>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_oven" name="Oven" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_temp" name="temperature">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_temp0" value="20.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cycles" name="cycles">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_cycles0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_heat" name="heat" method="_heatM"/>
      <ownedBehavior xmi:type="uml:OpaqueBehavior" xmi:id="_heatM" name="heat" specification="_heat">
        <language>JavaScript</language>
        <body>temperature = 250.0;</body>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Baking">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_off_s" name="Off">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dDoor" event="_doorEv"/>
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_offEntry" name="cool">
              <language>JavaScript</language>
              <body>temperature = 20.0;</body>
            </entry>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_on_s" name="On">
            <exit xmi:type="uml:OpaqueBehavior" xmi:id="_onExit" name="count">
              <language>JavaScript</language>
              <body>cycles = cycles + 1;</body>
            </exit>
            <region xmi:type="uml:Region" xmi:id="_rHeat" name="heating">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_initH"/>
              <subvertex xmi:type="uml:State" xmi:id="_warm" name="Warming"/>
              <subvertex xmi:type="uml:State" xmi:id="_hot" name="Hot"/>
              <transition xmi:type="uml:Transition" xmi:id="_tHi" source="_initH" target="_warm"/>
              <transition xmi:type="uml:Transition" xmi:id="_tHot" source="_warm" target="_hot">
                <trigger xmi:type="uml:Trigger" xmi:id="_trHot" event="_hotEv"/>
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="_rLight" name="lighting">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_initL"/>
              <subvertex xmi:type="uml:State" xmi:id="_lit" name="Lit"/>
              <subvertex xmi:type="uml:State" xmi:id="_dark" name="Dark"/>
              <transition xmi:type="uml:Transition" xmi:id="_tLi" source="_initL" target="_lit"/>
              <transition xmi:type="uml:Transition" xmi:id="_tDark" source="_lit" target="_dark">
                <trigger xmi:type="uml:Trigger" xmi:id="_trDark" event="_tick"/>
              </transition>
              <transition xmi:type="uml:Transition" xmi:id="_tLit" source="_dark" target="_lit">
                <trigger xmi:type="uml:Trigger" xmi:id="_trLit" event="_tick"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_pick" kind="choice"/>
          <subvertex xmi:type="uml:State" xmi:id="_rest" name="Resting">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="_dNoon" event="_noon"/>
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="_fin"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_off_s"/>
          <transition xmi:type="uml:Transition" xmi:id="_tOn" source="_off_s" target="_on_s">
            <trigger xmi:type="uml:Trigger" xmi:id="_trOn" event="_onEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tOff" source="_on_s" target="_pick">
            <trigger xmi:type="uml:Trigger" xmi:id="_trOff" event="_offEv"/>
            <trigger xmi:type="uml:Trigger" xmi:id="_trDoor" event="_doorEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tWorn" source="_pick" target="_fin">
            <guard xmi:type="uml:Constraint" xmi:id="_gWorn">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gWornX"><body>cycles >= 3</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tFresh" source="_pick" target="_rest">
            <guard xmi:type="uml:Constraint" xmi:id="_gFresh">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gFreshX"><body>else</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tRested" source="_rest" target="_off_s">
            <trigger xmi:type="uml:Trigger" xmi:id="_trNoon" event="_noon"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tSelf" kind="internal" source="_rest" target="_rest">
            <trigger xmi:type="uml:Trigger" xmi:id="_trSelf" event="_doorEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const ovenApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_oven"/>`

// Two regions of one state become the sub-states of a parallel state, a
// JavaScript entry or exit behavior an entry or exit action of assignments, a
// choice a state left at once by its guarded transitions, a guard that is a v2
// expression an if clause and one that is not a comment, a change event an
// accept when, an internal transition and an absolute time event unmapped, and
// a transition with two triggers two transitions. The result runs: On enters
// both regions, the tick alternates the light, the change event fires when the
// temperature is raised, and Off leaves through the choice on the counter.
func TestStateMachineWithOrthogonalRegionsAndGuards(t *testing.T) {
	r := migrateDocument(t, ovenMachine, ovenApplications)
	for _, line := range []string{
		"state def Baking {",
		"entry; then Off;",
		"state Off {",
		"defer Door;",
		"entry action cool {",
		"assign this.temperature := 20.0;",
		"state On {",
		"exit action count {",
		"assign this.cycles := this.cycles + 1;",
		"entry; then regions;",
		"state regions parallel {",
		"state heating {",
		"entry; then Warming;",
		"transition first Warming accept when this.temperature > 200.0 then Hot;",
		"state lighting {",
		"entry; then Lit;",
		"transition first Lit accept after 0.5 [SI::s] then Dark;",
		"transition first Dark accept after 0.5 [SI::s] then Lit;",
		"transition first regions then done;",
		"state choice;",
		"state Resting;",
		"transition first Off accept TurnOn then On;",
		"transition first On accept TurnOff then choice;",
		"transition first On accept Door then choice;",
		"transition first choice if this.cycles >= 3 then done;",
		"/* guard not migrated: [else] — ",
		"transition first choice then Resting;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "first Resting") {
		t.Errorf("a transition on an absolute time or an internal transition was written:\n%s", r.Notation)
	}
	wantNote(t, r, "_rHeat", migrate.Mapped, "an orthogonal region is written as a sub-state of the parallel state regions")
	wantNote(t, r, "_offEntry", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_onExit", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_pick", migrate.Approximated, "a choice pseudostate is written as a state its guarded transitions leave at once")
	wantNote(t, r, "_gWorn", migrate.Mapped, "")
	wantNote(t, r, "_gFresh", migrate.Approximated, "the guard is kept as a comment and the transition written unguarded")
	wantNote(t, r, "_tOff", migrate.Approximated, "written as 2 transitions, one per trigger")
	wantNote(t, r, "_tRested", migrate.Unmapped, "an absolute time event needs a TimeInstantValue, which no literal writes")
	wantNote(t, r, "_tSelf", migrate.Unmapped, "an internal transition has no v2 form")
	wantNote(t, r, "_hotEv", migrate.Mapped, "written where a trigger refers to it, as accept when this.temperature > 200.0")
	wantNote(t, r, "_noon", migrate.Unmapped, "an absolute time event needs a TimeInstantValue, which no literal writes")
	wantNote(t, r, "_dDoor", migrate.Approximated, "written as defer Door, an OpenSysML extension of the notation that the runtime executes")
	wantNote(t, r, "_doorEv", migrate.Approximated, "written where a trigger refers to it, as defer Door, an OpenSysML extension of the notation")
	wantNote(t, r, "_dNoon", migrate.Unmapped, "only a signal event can be deferred, not a TimeEvent")

	s := session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Baking")
	if out := meta(t, s, "%send TurnOn"); !strings.Contains(out, "transition Off -> On fires on it") {
		t.Errorf("%%send TurnOn: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Warming") || !strings.Contains(out, "Lit") {
		t.Errorf("On did not enter both regions:\n%s", out)
	}
	if out := meta(t, s, "%advance 0.5"); !strings.Contains(out, "Advanced to 0.5") {
		t.Errorf("%%advance 0.5: %s", out)
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Dark") {
		t.Errorf("the tick did not switch the light off:\n%s", out)
	}
	meta(t, s, "%invoke #1 heat")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Hot") {
		t.Errorf("the change event did not fire on the raised temperature:\n%s", out)
	}
	if out := meta(t, s, "%send Door"); !strings.Contains(out, "transition On -> choice fires on it") {
		t.Errorf("%%send Door: %s", out)
	}
	meta(t, s, "%step")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Resting") {
		t.Errorf("the first cycle did not rest through the choice:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : cycles"); !strings.Contains(out, "= 1") {
		t.Errorf("the exit action did not count the cycle: %s", out)
	}

	// A Door sent while Off is deferred there, and taken once On is entered.
	s = session(t, r)
	meta(t, s, "%instantiate Oven")
	meta(t, s, "%state Oven::Baking")
	if out := meta(t, s, "%send Door"); !strings.Contains(out, `Deferred by state machine "Baking" in state Off`) {
		t.Errorf("%%send Door while Off: %s", out)
	}
	meta(t, s, "%send TurnOn")
	for i := 0; i < 4 && !strings.Contains(meta(t, s, "%current"), "Current state: Resting"); i++ {
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Resting") {
		t.Errorf("the deferred Door did not leave On once it was entered:\n%s", out)
	}
}

// pipelineActivity is a package-owned activity taking a parameter, which it
// hands to a called activity's parameter through object flows; the called
// activity doubles it through a function behavior, returns it, and the
// caller writes the result to its out parameter. A partition groups the call.
const pipelineActivity = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_double" name="Double">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_dIn" name="x" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_dOut" name="y" direction="out">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_dInN" name="x" parameter="_dIn"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_dOutN" name="y" parameter="_dOut"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callTwice" name="twice" behavior="_twice">
        <argument xmi:type="uml:InputPin" xmi:id="_twiceIn" name="v"/>
        <result xmi:type="uml:OutputPin" xmi:id="_twiceOut" name="result"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_dOf1" source="_dInN" target="_twiceIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_dOf2" source="_twiceOut" target="_dOutN"/>
    </packagedElement>
    <packagedElement xmi:type="uml:FunctionBehavior" xmi:id="_twice" name="Twice">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_tv" name="v" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_tr" name="result" direction="return">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <body>v * 2.0</body>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_pipe" name="Pipeline">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_pIn" name="seed" direction="in">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_pOut" name="total" direction="out">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <group xmi:type="uml:ActivityPartition" xmi:id="_lane" name="Compute" node="_callD"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_pInN" name="seed" parameter="_pIn"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_pOutN" name="total" parameter="_pOut"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callD" name="double" behavior="_double" inPartition="_lane">
        <argument xmi:type="uml:InputPin" xmi:id="_callDIn" name="x"/>
        <result xmi:type="uml:OutputPin" xmi:id="_callDOut" name="y"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_pOf1" source="_pInN" target="_callDIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_pOf2" source="_callDOut" target="_pOutN"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_run" name="Run">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="_rOut" name="answer" direction="out">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedParameter>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="_rOutN" name="answer" parameter="_rOut"/>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_seedV" name="seed">
        <value xmi:type="uml:LiteralReal" xmi:id="_seedL" value="21.0"/>
        <result xmi:type="uml:OutputPin" xmi:id="_seedOut" name="result"/>
      </node>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callP" name="pipeline" behavior="_pipe">
        <argument xmi:type="uml:InputPin" xmi:id="_callPIn" name="seed"/>
        <result xmi:type="uml:OutputPin" xmi:id="_callPOut" name="total"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_rOf1" source="_seedOut" target="_callPIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_rOf2" source="_callPOut" target="_rOutN"/>
    </packagedElement>`

// An activity's parameters become the action def's parameters, a flow from a
// parameter node into a call's pin the binding of the call's input, a flow
// from the call's result to a parameter node the binding of the out
// parameter, a function behavior a calc def the call evaluates, and a
// partition a comment naming its nodes. A driver feeding the pipeline a
// literal through a value specification action runs and yields the double.
func TestActivityParametersFlowThroughNestedCalls(t *testing.T) {
	r := migrateDocument(t, pipelineActivity, "")
	for _, line := range []string{
		"action def Double {",
		"in x : ScalarValues::Real;",
		"out y : ScalarValues::Real;",
		"calc def Twice {",
		"in v : ScalarValues::Real;",
		"out result : ScalarValues::Real;",
		"v * 2.0",
		"action def Pipeline {",
		"in seed : ScalarValues::Real;",
		"out total : ScalarValues::Real;",
		"action double : Double;",
		"bind double.x = seed;",
		"bind total = double.y;",
		"/* partition 'Compute': double */",
		"action def Run {",
		"out answer : ScalarValues::Real;",
		"action pipeline : Pipeline;",
		"out result = 21.0;",
		"flow seed.result to pipeline.seed;",
		"bind answer = pipeline.total;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_lane", migrate.Approximated, "")
	wantNote(t, r, "_pInN", migrate.Mapped, "")
	wantNote(t, r, "_pOf1", migrate.Mapped, "")
	wantNote(t, r, "_twice", migrate.Mapped, "")
	wantNote(t, r, "_seedV", migrate.Approximated, "no edge leads to the node, so it starts with the activity")
	wantNote(t, r, "_rOf1", migrate.Mapped, "")

	s := session(t, r)
	v := s.RunAction("Run")
	wantVerdict(t, v)
	if out := strings.Join(v.Lines, "\n"); !strings.Contains(out, "answer = 42.0") {
		t.Errorf("the pipeline did not double its seed:\n%s", out)
	}
}

// handshakeInteraction is a block with two parts whose interaction sends two
// signals between lifelines standing for the parts, listed out of occurrence
// order, with a duration constraint; a second interaction carries a call.
const handshakeInteraction = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_req" name="Request">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_reqN" name="n">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_rep" name="Reply"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_node" name="Node"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_net" name="Net">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_a" name="a" type="_node" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b" type="_node" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_hs" name="Handshake">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_la" name="a" represents="_a" coveredBy="_sReq _rRep"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lb" name="b" represents="_b" coveredBy="_rReq _sRep"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sReq" covered="_la" message="_mReq"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rReq" covered="_lb" message="_mReq"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sRep" covered="_lb" message="_mRep"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rRep" covered="_la" message="_mRep"/>
        <message xmi:type="uml:Message" xmi:id="_mRep" name="reply" messageSort="asynchSignal" signature="_rep" sendEvent="_sRep" receiveEvent="_rRep"/>
        <message xmi:type="uml:Message" xmi:id="_mReq" name="request" messageSort="asynchSignal" signature="_req" sendEvent="_sReq" receiveEvent="_rReq">
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mReqArg" value="7"/>
        </message>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_hsDur">
          <constrainedElement xmi:idref="_sReq"/>
          <constrainedElement xmi:idref="_rRep"/>
        </ownedRule>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_rpc" name="RemoteCall">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_la2" name="a" represents="_a" coveredBy="_sCall"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lb2" name="b" represents="_b" coveredBy="_rCall"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sCall" covered="_la2" message="_mCall"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rCall" covered="_lb2" message="_mCall"/>
        <message xmi:type="uml:Message" xmi:id="_mCall" name="call" messageSort="synchCall" sendEvent="_sCall" receiveEvent="_rCall"/>
      </ownedBehavior>
    </packagedElement>`

const handshakeApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_node"/>
  <sysml:Block xmi:id="_s2" base_Class="_net"/>`

// An interaction whose messages are all signals sent to the block's parts
// becomes a scenario action def of sends in occurrence order, its arguments
// the signal's attribute values and its duration constraint reported; one
// carrying a call is unmapped with the message named. The scenario runs.
func TestInteractionMigratesToAScenarioOfSends(t *testing.T) {
	r := migrateDocument(t, handshakeInteraction, handshakeApplications)
	for _, line := range []string{
		"action def Handshake {",
		"action request send new Request(n = 7) to this.b;",
		"first start then request;",
		"action reply send new Reply() to this.a;",
		"first request then reply;",
		"first reply then done;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "action def RemoteCall {") {
		t.Errorf("an interaction carrying a call was written as a scenario:\n%s", r.Notation)
	}
	wantNote(t, r, "_hs", migrate.Approximated, "written as a scenario of 2 sends, one per message in occurrence order")
	wantNote(t, r, "_mReq", migrate.Mapped, "written as a send to the part b")
	wantNote(t, r, "_la", migrate.Mapped, "the lifeline stands for the part the sends address")
	wantNote(t, r, "_hsDur", migrate.Unmapped, "a DurationConstraint on an interaction has no form in a scenario of sends")
	wantNote(t, r, "_rpc", migrate.Unmapped, "is a synchCall message; only a signal send has a v2 form")

	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%action Net::Handshake #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the scenario did not run to completion:\n%s", out)
	}
}

// loggingMachine is a state machine whose state names its entry behavior, its
// exit behavior and a nested state alike, as UML allows and v2 does not.
const loggingMachine = `
    <packagedElement xmi:type="uml:Class" xmi:id="_logger" name="Logger" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_count" name="count">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_count0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Logging">
        <region xmi:type="uml:Region" xmi:id="_r0">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_on_s" name="On">
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_onEntry" name="log">
              <language>JavaScript</language>
              <body>count = count + 1;</body>
            </entry>
            <exit xmi:type="uml:OpaqueBehavior" xmi:id="_onExit" name="log">
              <language>JavaScript</language>
              <body>count = count + 10;</body>
            </exit>
            <region xmi:type="uml:Region" xmi:id="_rIn">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_initIn"/>
              <subvertex xmi:type="uml:State" xmi:id="_log_s" name="log"/>
              <transition xmi:type="uml:Transition" xmi:id="_tIn" source="_initIn" target="_log_s"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_on_s"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const loggingApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_logger"/>`

// A state's entry and exit behaviors and the states of its one region are
// members of one v2 body, so those sharing a name are told apart the way any
// clashing members are; the region's entry follows the state's own entry action.
func TestStateBehaviorsSharingANameAreDistinguished(t *testing.T) {
	r := migrateDocument(t, loggingMachine, loggingApplications)
	for _, line := range []string{
		"entry action log {",
		"exit action 'log 2' {",
		"then 'log 3';",
		"state 'log 3';",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "entry; then 'log 3'") {
		t.Errorf("the region's entry is written as a second entry action:\n%s", r.Notation)
	}
	wantNote(t, r, "_onExit", migrate.Approximated, "written as v2 assignments")
	if es := entriesFor(r, "_onExit"); len(es) == 1 && es[0].Target != "Logger::Logging::On::'log 2'" {
		t.Errorf("the exit behavior's report target is %q, not the name written", es[0].Target)
	}
	if es := entriesFor(r, "_log_s"); len(es) != 1 || es[0].Target != "'log 3'" {
		t.Errorf("the nested state's report entries are %+v, want one naming 'log 3'", es)
	}
	s := session(t, r)
	meta(t, s, "%instantiate Logger")
	meta(t, s, "%state Logger::Logging")
	meta(t, s, "%step")
	if out := meta(t, s, "%eval in #1 : count"); !strings.Contains(out, "= 1") {
		t.Errorf("the entry action did not run: %s", out)
	}
}
