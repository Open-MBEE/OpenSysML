package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// portedRig sends Go from a sender's port over a rig connector and a box's
// delegation to a receiver; its Named machine's trigger also names a foreign port.
const portedRig = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_sender" name="Sender" classifierBehavior="_fire">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_fire" name="Fire">
        <node xmi:type="uml:InitialNode" xmi:id="_fi"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send go" signal="_go" onPort="_tx"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_ff"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe1" source="_fi" target="_send"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe2" source="_send" target="_ff"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_receiver" name="Receiver" classifierBehavior="_rsm">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_rx" name="rx" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_aux" name="aux" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_rsm" name="Life">
        <region xmi:type="uml:Region" xmi:id="_rr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_rinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_got" name="got"/>
          <transition xmi:type="uml:Transition" xmi:id="_rt0" source="_rinit" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_rt1" source="_idle" target="_got">
            <trigger xmi:type="uml:Trigger" xmi:id="_rtr1" event="_goEv"/>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_nsm" name="Named">
        <region xmi:type="uml:Region" xmi:id="_nr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_ninit"/>
          <subvertex xmi:type="uml:State" xmi:id="_nidle" name="idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ngot" name="got"/>
          <transition xmi:type="uml:Transition" xmi:id="_nt0" source="_ninit" target="_nidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_nt1" source="_nidle" target="_ngot">
            <trigger xmi:type="uml:Trigger" xmi:id="_ntr1" event="_goEv" port="_rx _tx"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_box" name="Box">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_cmd" name="cmd" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r" name="r" type="_receiver" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_delegate">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_de1" role="_cmd"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_de2" role="_rx" partWithPort="_r"/>
      </ownedConnector>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="s" type="_sender" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_b" name="b" type="_box" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_link">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le1" role="_tx" partWithPort="_s"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le2" role="_cmd" partWithPort="_b"/>
      </ownedConnector>
    </packagedElement>`

const portedRigApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_sender"/>
  <sysml:Block xmi:id="_s2" base_Class="_receiver"/>
  <sysml:Block xmi:id="_s3" base_Class="_box"/>
  <sysml:Block xmi:id="_s4" base_Class="_rig"/>`

// A trigger naming no port also accepts via each port the connectors carry the
// signal to; one naming ports keeps the owner's and reports the rest. The result runs.
func TestTriggersAcceptViaThePortsTheSignalArrivesAt(t *testing.T) {
	r := migrateDocument(t, portedRig, portedRigApplications)
	for _, line := range []string{
		"transition first idle accept Go then got;",
		"transition first idle accept Go via rx then got;",
	} {
		wantLine(t, r.Notation, line)
	}
	if got := strings.Count(string(r.Notation), "accept Go via rx then got;"); got != 2 {
		t.Errorf("accept via rx written %d times, want one per machine:\n%s", got, r.Notation)
	}
	if strings.Contains(string(r.Notation), "via aux") || strings.Contains(string(r.Notation), "via tx then") {
		t.Errorf("a transition accepts via a port the signal does not arrive at:\n%s", r.Notation)
	}
	wantNote(t, r, "_rt1", migrate.Mapped, "the signal arrives at the port rx over the document's connectors or declarations, so the trigger is also written accepting via each")
	wantNote(t, r, "_ntr1", migrate.Approximated, "the trigger's port Sender::tx is no port of the behavior's owner")
	wantNote(t, r, "_nt1", migrate.Mapped, "the trigger accepts via the port rx it names")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Receiver::Life #1.b.r")
	meta(t, s, "%action Sender::Fire #1.s")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: got") {
		t.Errorf("the receiver's machine did not take the signal sent through the rig:\n%s", out)
	}
}
