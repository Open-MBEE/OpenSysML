package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// declaredRig sends nothing; its receiver's rx inherits an in flow of Go, rx2 conjugates an
// out flow of Go, aux keeps that out flow unconjugated, and bare is untyped.
const declaredRig = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_goBase" name="GoBase">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_goBaseFlow" name="go" type="_go"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_goIn" name="GoIn">
      <generalization xmi:type="uml:Generalization" xmi:id="_goInGen" general="_goBase"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_goOut" name="GoOut">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_goOutFlow" name="go" type="_go"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_receiver" name="Receiver" classifierBehavior="_rsm">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_rx" name="rx" type="_goIn" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_rx2" name="rx2" type="_goOut" isConjugated="true" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_aux" name="aux" type="_goOut" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_bare" name="bare" aggregation="composite"/>
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
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_box" name="Box">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_cmd" name="cmd" type="_goIn" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r" name="r" type="_receiver" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_delegate">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_de1" role="_cmd"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_de2" role="_rx" partWithPort="_r"/>
      </ownedConnector>
    </packagedElement>`

const declaredRigApplications = `
  <sysml:InterfaceBlock xmi:id="_s1" base_Class="_goBase"/>
  <sysml:InterfaceBlock xmi:id="_s2" base_Class="_goIn"/>
  <sysml:InterfaceBlock xmi:id="_s3" base_Class="_goOut"/>
  <sysml:FlowProperty xmi:id="_s4" base_Property="_goBaseFlow" direction="in"/>
  <sysml:FlowProperty xmi:id="_s5" base_Property="_goOutFlow" direction="out"/>
  <sysml:Block xmi:id="_s6" base_Class="_receiver"/>
  <sysml:Block xmi:id="_s7" base_Class="_box"/>`

// A bench declared beside the migrated model drives the box's boundary port from outside it.
const declaredBench = `
part def Driver {
    port tx;
    action def Poke {
        first start then poke;
        action poke {
            send new Go() via this.tx;
        }
        first poke then final;
        action final terminate;
    }
}
part def Bench {
    part d : Driver;
    part b : Box;
    connect d.tx to b.cmd;
}`

// A trigger naming no port accepts via each port whose declarations let the signal in, not via
// one whose flow goes out; an untyped port is reported unrouted, and a send from outside fires it.
func TestTriggersAcceptViaThePortsDeclaredToCarryTheSignal(t *testing.T) {
	r := migrateDocument(t, declaredRig, declaredRigApplications)
	for _, line := range []string{
		"transition first idle accept Go then got;",
		"transition first idle accept Go via rx then got;",
		"transition first idle accept Go via rx2 then got;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "via aux") || strings.Contains(string(r.Notation), "via bare") {
		t.Errorf("a transition accepts via a port the signal does not arrive at:\n%s", r.Notation)
	}
	wantNote(t, r, "_rt1", migrate.Mapped, "the signal arrives at the ports rx, rx2 over the document's connectors or declarations, so the trigger is also written accepting via each; "+
		"nothing in the document declares or sends a signal to the port bare, so one arriving there is not accepted")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	if res := s.Submit(declaredBench); len(res.Diagnostics) > 0 {
		t.Fatalf("bench: %v", res.Diagnostics)
	}
	meta(t, s, "%instantiate Bench")
	meta(t, s, "%state Receiver::Life #1.b.r")
	if out := meta(t, s, "%action Driver::Poke #1.d"); strings.Contains(out, "error") {
		t.Fatal(out)
	}
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: got") {
		t.Errorf("the receiver's machine did not take the signal sent through the boundary:\n%s", out)
	}
}

// conveyedRig joins a source's untyped port to a receiver's untyped port by a connector
// realizing an item flow that conveys Go to the receiver's port; nothing sends.
const conveyedRig = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_source" name="Source">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_o" name="o" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_receiver" name="Receiver" classifierBehavior="_rsm">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_in1" name="in1" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_in2" name="in2" aggregation="composite"/>
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
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_s" name="s" type="_source" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_r" name="r" type="_receiver" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_link">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le1" role="_o" partWithPort="_s"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_le2" role="_in1" partWithPort="_r"/>
      </ownedConnector>
    </packagedElement>
    <packagedElement xmi:type="uml:InformationFlow" xmi:id="_if" informationSource="_o" informationTarget="_in1" conveyed="_go" realizingConnector="_link"/>`

const conveyedRigApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_source"/>
  <sysml:Block xmi:id="_s2" base_Class="_receiver"/>
  <sysml:Block xmi:id="_s3" base_Class="_rig"/>
  <sysml:ItemFlow xmi:id="_s4" base_InformationFlow="_if"/>`

// An item flow conveying the signal to a port makes the trigger accept via that port too,
// while the receiver's other untyped port is reported as left unrouted.
func TestTriggersAcceptViaThePortsItemFlowsConveyTheSignalTo(t *testing.T) {
	r := migrateDocument(t, conveyedRig, conveyedRigApplications)
	wantLine(t, r.Notation, "transition first idle accept Go then got;")
	wantLine(t, r.Notation, "transition first idle accept Go via in1 then got;")
	if strings.Contains(string(r.Notation), "via in2") {
		t.Errorf("a transition accepts via a port the signal does not arrive at:\n%s", r.Notation)
	}
	wantNote(t, r, "_rt1", migrate.Mapped, "the signal arrives at the port in1 over the document's connectors or declarations, so the trigger is also written accepting via each; "+
		"nothing in the document declares or sends a signal to the port in2, so one arriving there is not accepted")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}
