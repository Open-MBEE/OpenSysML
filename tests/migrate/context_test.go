package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// hostedWait is a package-owned activity accepting Ack through no port, run by
// a Host's activity and as the do behavior of a state of its machine; Ack reaches
// the Host only over a rig connector into its port rx.
const hostedWait = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ack" name="Ack"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ackEv" signal="_ack"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_await" name="Await">
      <node xmi:type="uml:InitialNode" xmi:id="_ai"/>
      <node xmi:type="uml:AcceptEventAction" xmi:id="_take" name="take ack">
        <trigger xmi:type="uml:Trigger" xmi:id="_takeTr" event="_ackEv"/>
      </node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_af"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ae1" source="_ai" target="_take"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ae2" source="_take" target="_af"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sender" name="Sender">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_fire" name="Fire">
        <node xmi:type="uml:InitialNode" xmi:id="_fi"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send ack" signal="_ack" onPort="_tx"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_ff"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe1" source="_fi" target="_send"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe2" source="_send" target="_ff"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_host" name="Host">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_rx" name="rx" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_seen" name="seen">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_seen0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callAwait" name="await" behavior="_await"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
          <value xmi:type="uml:LiteralInteger" xmi:id="_oneV" value="1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set seen" structuralFeature="_seen" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_callAwait"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callAwait" target="_one"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_rof" source="_oneOut" target="_setVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re3" source="_set" target="_rf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_life" name="Life">
        <region xmi:type="uml:Region" xmi:id="_lr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_linit"/>
          <subvertex xmi:type="uml:State" xmi:id="_waiting" name="waiting" doActivity="_await"/>
          <subvertex xmi:type="uml:State" xmi:id="_done" name="done"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt0" source="_linit" target="_waiting"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt1" source="_waiting" target="_done"/>
        </region>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="s" type="_sender" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_h" name="h" type="_host" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_link">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le1" role="_tx" partWithPort="_s"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le2" role="_rx" partWithPort="_h"/>
      </ownedConnector>
    </packagedElement>`

const hostedWaitApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_sender"/>
  <sysml:Block xmi:id="_b2" base_Class="_host"/>
  <sysml:Block xmi:id="_b3" base_Class="_rig"/>`

// An activity no classifier owns, whose accept names no port, acts on the block
// whose behaviors run it when the signal arrives at that block's ports: it takes
// a context parameter, accepts via the port, and each runner binds its object.
func TestUnownedActivityAcceptsViaThePortsOfTheBlocksRunningIt(t *testing.T) {
	r := migrateDocument(t, hostedWait, hostedWaitApplications)
	for _, line := range []string{
		"in ref context : Host;",
		"action 'take ack' accept Ack via context.rx;",
		"action await : Await;",
		"bind await.context = this;",
		"do action : Await { in context = this; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_takeTr", migrate.Approximated, "the signal arrives at the port rx over the document's connectors or declarations, so the action accepts via it; an action accepts through one route, and one sent to the object itself is not taken")
	wantNote(t, r, "_callAwait", migrate.Mapped, "the behavior acts on a Host through its parameter context, which is bound to this")
	wantNote(t, r, "_await", migrate.Approximated, "acts on a Host through its ports, which it takes as its parameter context; also run as the do action of 'waiting'; the behavior acts on a Host through its parameter context, which is bound to this")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%action Sender::Fire #1.s")
	meta(t, s, "%continue")
	meta(t, s, "%action Host::Run #1.h")
	meta(t, s, "%continue")
	if out := meta(t, s, "%eval in #1.h : seen"); !strings.Contains(out, "= 1") {
		t.Errorf("the hosted activity did not take the signal sent to its host's port: %s", out)
	}

	s = session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Host::Life #1.h")
	meta(t, s, "%action Sender::Fire #1.s")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: done2") {
		t.Errorf("the state's referenced do activity did not take the signal sent to its host's port:\n%s", out)
	}
}

// borrowedContext is a package-owned activity Hit sending Ping(a, b) through a
// Host's port tx, fed by one value through a fork into the two argument pins; a
// Controller, which is no Host and holds none, owns Relay, which only calls Hit;
// the Host's Run calls Relay. A Receiver on the rig takes Ping at its port rx.
const borrowedContext = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_pa" name="a">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_pb" name="b">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_hit" name="Hit">
      <node xmi:type="uml:InitialNode" xmi:id="_hi"/>
      <node xmi:type="uml:ValueSpecificationAction" xmi:id="_two" name="two">
        <value xmi:type="uml:LiteralInteger" xmi:id="_twoV" value="2"/>
        <result xmi:type="uml:OutputPin" xmi:id="_twoOut" name="result"/>
      </node>
      <node xmi:type="uml:ForkNode" xmi:id="_split"/>
      <node xmi:type="uml:SendSignalAction" xmi:id="_hsend" name="send ping" signal="_ping" onPort="_tx">
        <argument xmi:type="uml:InputPin" xmi:id="_argA" name="a"/>
        <argument xmi:type="uml:InputPin" xmi:id="_argB" name="b"/>
      </node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_hf"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_he1" source="_hi" target="_two"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_he2" source="_two" target="_hsend"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_ho1" source="_twoOut" target="_split"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_ho2" source="_split" target="_argA"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_ho3" source="_split" target="_argB"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_he3" source="_hsend" target="_hf"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_controller" name="Controller">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_relay" name="Relay">
        <node xmi:type="uml:InitialNode" xmi:id="_yi"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callHit" name="hit" behavior="_hit"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_yf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ye1" source="_yi" target="_callHit"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ye2" source="_callHit" target="_yf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_host" name="Host">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callRelay" name="relay" behavior="_relay"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_callRelay"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callRelay" target="_rf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_receiver" name="Receiver" classifierBehavior="_life">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_rx" name="rx" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_sum" name="sum">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_sum0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_life" name="Life">
        <region xmi:type="uml:Region" xmi:id="_lr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_linit"/>
          <subvertex xmi:type="uml:State" xmi:id="_waiting" name="waiting"/>
          <subvertex xmi:type="uml:State" xmi:id="_done" name="done"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt0" source="_linit" target="_waiting"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt1" source="_waiting" target="_done">
            <trigger xmi:type="uml:Trigger" xmi:id="_ltr1" event="_pingEv" port="_rx"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_leff1">
              <ownedParameter xmi:type="uml:Parameter" xmi:id="_lp1" name="p" direction="in" type="_ping"/>
              <language>JavaScript</language>
              <body>sum = p.a + p.b;</body>
            </effect>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_h" name="h" type="_host" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r" name="r" type="_receiver" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_link">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le1" role="_tx" partWithPort="_h"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le2" role="_rx" partWithPort="_r"/>
      </ownedConnector>
    </packagedElement>`

const borrowedContextApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_controller"/>
  <sysml:Block xmi:id="_b2" base_Class="_host"/>
  <sysml:Block xmi:id="_b3" base_Class="_receiver"/>
  <sysml:Block xmi:id="_b4" base_Class="_rig"/>`

// A classifier's own activity whose every need is another block's ports takes that
// block as its context, as v1 ran it on whichever object called it; the caller
// that is one binds itself. A fork only object flows lead to and leave routes data:
// the flows are written from its source to the pins, and no node waits on it.
func TestOwnedActivityBorrowsTheContextItsCallsNeed(t *testing.T) {
	r := migrateDocument(t, borrowedContext, borrowedContextApplications)
	for _, line := range []string{
		"action def Relay {",
		"in ref context : Host;",
		"action hit : Hit;",
		"bind hit.context = context;",
		"action relay : Controller::Relay;",
		"bind relay.context = this;",
		"send new Ping(a, b) via context.tx;",
		"flow two.result to 'send ping'.a;",
		"flow two.result to 'send ping'.b;",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"fork ", "then 'fork'", "then split"} {
		wantNoLine(t, r.Notation, line)
	}
	wantNote(t, r, "_relay", migrate.Approximated, "acts on a Host through its ports, which it takes as its parameter context rather than its owner Controller, which is no such object and holds no one part that is: v1 ran it on whichever object called it")
	wantNote(t, r, "_callHit", migrate.Mapped, "the behavior acts on a Host through its parameter context, which is bound to context")
	wantNote(t, r, "_callRelay", migrate.Approximated, "the behavior acts on a Host through its parameter context, which is bound to this; the behavior belongs to Controller and runs here in the caller's context")
	wantNote(t, r, "_split", migrate.Approximated, "the node routes data only: the flows through it are written from their sources to the pins it leads to")
	wantNote(t, r, "_ho1", migrate.Approximated, "the flow into (_split) is written from its sources to the pins the node leads to")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Receiver::Life #1.r")
	meta(t, s, "%action Host::Run #1.h")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1.r : sum"); !strings.Contains(out, "= 4") {
		t.Errorf("the borrowed context did not carry the ping to the receiver: %s", out)
	}
}

// hostlessCaller is a Console, no Host and holding none, whose Drive calls Relay
// and whose machine runs Hit as a state's do behavior, then sets `ran`.
const hostlessCaller = `
    <packagedElement xmi:type="uml:Class" xmi:id="_console" name="Console">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ran" name="ran">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_ran0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_drive" name="Drive">
        <node xmi:type="uml:InitialNode" xmi:id="_di"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callRelay2" name="relay" behavior="_relay"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
          <value xmi:type="uml:LiteralInteger" xmi:id="_oneV" value="1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set ran" structuralFeature="_ran" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_df"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_de1" source="_di" target="_callRelay2"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_de2" source="_callRelay2" target="_one"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_dof" source="_oneOut" target="_setVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_de3" source="_set" target="_df"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_clife" name="Life">
        <region xmi:type="uml:Region" xmi:id="_clr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_clinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_hitting" name="hitting" doActivity="_hit"/>
          <subvertex xmi:type="uml:State" xmi:id="_cdone" name="done"/>
          <transition xmi:type="uml:Transition" xmi:id="_clt0" source="_clinit" target="_hitting"/>
          <transition xmi:type="uml:Transition" xmi:id="_clt1" source="_hitting" target="_cdone"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const hostlessCallerApplications = `
  <sysml:Block xmi:id="_b5" base_Class="_console"/>`

// A call from an object that is not, and holds no part that is, what the callee
// acts on is written as a placeholder that passes the token on, and a state's do
// behavior in that position is not run: v1 ran both on an object lacking the ports.
func TestCallsFromObjectsLackingTheCalleesContextAreNotPerformed(t *testing.T) {
	r := migrateDocument(t, borrowedContext+hostlessCaller, borrowedContextApplications+hostlessCallerApplications)
	why := "the behavior acts on a Host through its parameter context, which is left unbound: the caller is a Console, which is no Host and has no part that is one"
	for _, line := range []string{
		"action relay {",
		"/* not migrated: CallBehaviorAction 'relay' — " + why + "; v1 runs Controller::Relay on the caller's object, which lacks the ports it goes through, so the action carries the token and performs nothing */",
		"/* do action Hit is not run: " + why + " */",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "action relay : Controller::Relay;\n        first relay then one;\n        action one")
	wantNote(t, r, "_callRelay2", migrate.Approximated, why+"; v1 runs Controller::Relay on the caller's object, which lacks the ports it goes through, so the action carries the token and performs nothing")
	wantNote(t, r, "_hitting", migrate.Approximated, "its do action Hit is not run: "+why)
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Console")
	meta(t, s, "%action Console::Drive #1")
	meta(t, s, "%continue")
	if out := meta(t, s, "%eval in #1 : ran"); !strings.Contains(out, "= 1") {
		t.Errorf("the placeholder did not pass the token on: %s", out)
	}
	s = session(t, r)
	meta(t, s, "%instantiate Console")
	meta(t, s, "%state Console::Life #1")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: done") {
		t.Errorf("the machine did not pass the state whose do behavior is not run:\n%s", out)
	}
}

// callCycle is a Controller-owned Knock sending Ping through a Host's port tx and
// then calling the package-owned Again, which decides whether to call Knock once
// more (it never does); a Host runs Knock, and a Receiver counts the pings.
const callCycleKnocker = `
    <packagedElement xmi:type="uml:Class" xmi:id="_controller" name="Controller">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_knock" name="Knock">
        <node xmi:type="uml:InitialNode" xmi:id="_ki"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_ksend" name="send ping" signal="_ping" onPort="_tx"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callAgain" name="again" behavior="_again"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_kf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ke1" source="_ki" target="_ksend"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ke2" source="_ksend" target="_callAgain"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ke3" source="_callAgain" target="_kf"/>
      </ownedBehavior>
    </packagedElement>`

const callCycleAgain = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_again" name="Again">
      <node xmi:type="uml:InitialNode" xmi:id="_ai"/>
      <node xmi:type="uml:DecisionNode" xmi:id="_more"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callKnock" name="knock" behavior="_knock"/>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_af"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ae1" source="_ai" target="_more"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ae2" source="_more" target="_callKnock">
        <guard xmi:type="uml:OpaqueExpression" xmi:id="_gMore"><body>false</body></guard>
      </edge>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ae3" source="_more" target="_af">
        <guard xmi:type="uml:OpaqueExpression" xmi:id="_gDone"><body>else</body></guard>
      </edge>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ae4" source="_callKnock" target="_af"/>
    </packagedElement>`

const callCycleRig = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_host" name="Host">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callKnock0" name="knock" behavior="_knock"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_callKnock0"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callKnock0" target="_rf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_receiver" name="Receiver" classifierBehavior="_life">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_rx" name="rx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_life" name="Life">
        <region xmi:type="uml:Region" xmi:id="_lr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_linit"/>
          <subvertex xmi:type="uml:State" xmi:id="_waiting" name="waiting"/>
          <subvertex xmi:type="uml:State" xmi:id="_done" name="done"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt0" source="_linit" target="_waiting"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt1" source="_waiting" target="_done">
            <trigger xmi:type="uml:Trigger" xmi:id="_ltr1" event="_pingEv" port="_rx"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_h" name="h" type="_host" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r" name="r" type="_receiver" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_link">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le1" role="_tx" partWithPort="_h"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le2" role="_rx" partWithPort="_r"/>
      </ownedConnector>
    </packagedElement>`

const callCycleApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_controller"/>
  <sysml:Block xmi:id="_b2" base_Class="_host"/>
  <sysml:Block xmi:id="_b3" base_Class="_receiver"/>
  <sysml:Block xmi:id="_b4" base_Class="_rig"/>`

// Activities calling each other in a cycle act on the one object whose ports any
// of them names: both take the Host as their context and pass it on to each other,
// whichever of them the document writes first.
func TestActivitiesCallingEachOtherShareTheContextTheCycleNeeds(t *testing.T) {
	for name, doc := range map[string]string{
		"owned first":   callCycleKnocker + callCycleAgain + callCycleRig,
		"unowned first": callCycleAgain + callCycleKnocker + callCycleRig,
	} {
		t.Run(name, func(t *testing.T) {
			r := migrateDocument(t, doc, callCycleApplications)
			for _, line := range []string{
				"action def Knock {",
				"in ref context : Host;",
				"send new Ping() via context.tx;",
				"action again : Again;",
				"bind again.context = context;",
				"action def Again {",
				"action knock : Controller::Knock;",
				"bind knock.context = context;",
				"bind knock.context = this;",
			} {
				wantLine(t, r.Notation, line)
			}
			wantNoLine(t, r.Notation, "via this.tx")
			wantNoLine(t, r.Notation, "in ref context : Controller;")
			if n := strings.Count(string(r.Notation), "in ref context : Host;"); n != 2 {
				t.Errorf("the Host context parameter is declared %d times, want 2 (Knock and Again):\n%s", n, r.Notation)
			}
			wantNote(t, r, "_knock", migrate.Approximated, "acts on a Host through its ports, which it takes as its parameter context rather than its owner Controller, which is no such object and holds no one part that is: v1 ran it on whichever object called it")
			wantNote(t, r, "_again", migrate.Mapped, "acts on a Host through its ports, which it takes as its parameter context")
			wantNote(t, r, "_callAgain", migrate.Mapped, "the behavior acts on a Host through its parameter context, which is bound to context")
			wantNote(t, r, "_callKnock", migrate.Approximated, "the behavior acts on a Host through its parameter context, which is bound to context; the behavior belongs to Controller and runs here in the caller's context")
			if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
				t.Errorf("%v", diags)
			}

			s := session(t, r)
			meta(t, s, "%instantiate Rig")
			meta(t, s, "%state Receiver::Life #1.r")
			meta(t, s, "%action Host::Run #1.h")
			meta(t, s, "%continue")
			meta(t, s, "%advance 0")
			if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: done") {
				t.Errorf("the ping sent through the cycle's context did not reach the receiver:\n%s", out)
			}
		})
	}
}
