package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// portedCalls is a drive whose port a connector joins to its motor's command
// port; its activities call the motor's operation over the drive's port, over the
// motor's own port on the read part, and over a port no connector joins.
const portedCalls = `
    <packagedElement xmi:type="uml:Interface" xmi:id="_cmdIf" name="MotorCmd"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_speed" name="speed">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_speed0" value="0.0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_cmd" name="cmd" type="_cmdIf" aggregation="composite"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_spin" name="Spin" method="_spinning">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spinRpm" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_spinning" name="Spinning" specification="_spin">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRpm" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_spApn" name="rpm" parameter="_spRpm"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setSpeed" name="set speed" structuralFeature="_speed" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setSpeedVal" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_spOf" source="_spApn" target="_setSpeedVal"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_drive" name="Drive">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_p" name="p" type="_cmdIf" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Port" xmi:id="_loose" name="loose" type="_cmdIf" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_motorPart" name="motor" type="_motor" aggregation="composite"/>
      <ownedConnector xmi:type="uml:Connector" xmi:id="_c">
        <end xmi:type="uml:ConnectorEnd" xmi:id="_c1" role="_p"/>
        <end xmi:type="uml:ConnectorEnd" xmi:id="_c2" role="_cmd" partWithPort="_motorPart"/>
      </ownedConnector>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_thirty" name="thirty">
          <value xmi:type="uml:LiteralReal" xmi:id="_thirtyV" value="30.0"/>
          <result xmi:type="uml:OutputPin" xmi:id="_thirtyOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_callP" name="spin over p" operation="_spin" onPort="_p">
          <argument xmi:type="uml:InputPin" xmi:id="_callPRpm" name="rpm"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readMotor" name="read motor" structuralFeature="_motorPart">
          <result xmi:type="uml:OutputPin" xmi:id="_readMotorOut" name="result"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_forty" name="forty">
          <value xmi:type="uml:LiteralReal" xmi:id="_fortyV" value="40.0"/>
          <result xmi:type="uml:OutputPin" xmi:id="_fortyOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_callCmd" name="spin over cmd" operation="_spin" onPort="_cmd">
          <target xmi:type="uml:InputPin" xmi:id="_callCmdTgt" name="target"/>
          <argument xmi:type="uml:InputPin" xmi:id="_callCmdRpm" name="rpm"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_thirty"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_thirtyOut" target="_callPRpm"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_callP" target="_readMotor"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_readMotor" target="_forty"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of2" source="_readMotorOut" target="_callCmdTgt"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of3" source="_fortyOut" target="_callCmdRpm"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_callCmd" target="_final"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_stray" name="Stray">
        <node xmi:type="uml:CallOperationAction" xmi:id="_callLoose" name="spin over loose" operation="_spin" onPort="_loose"/>
      </ownedBehavior>
    </packagedElement>`

const portedCallsApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_motor"/>
  <sysml:Block xmi:id="_s2" base_Class="_drive"/>`

// A call over the caller's port performs the operation's usage on the part the
// caller's connector joins to that port; one over the target's own port performs
// it on the target, the port left unwritten; one over a port no connector joins
// stays in the caller's context with the reason. Running the drive's activity
// spins its motor through both calls.
func TestCallOperationOverPortsReachesTheConnectedPart(t *testing.T) {
	r := migrateDocument(t, portedCalls, portedCallsApplications)
	for _, line := range []string{
		"perform action 'spin over p' ::> motor.spin;",
		"perform action 'spin over cmd' ::> motor.spin;",
		"action 'spin over loose' : Motor::Spin;",
		"flow thirty.result to 'spin over p'.rpm;",
		"flow forty.result to 'spin over cmd'.rpm;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "::> p.") || strings.Contains(string(r.Notation), "::> motor.cmd.") {
		t.Errorf("a call was performed on a port whose type has no operation:\n%s", r.Notation)
	}
	wantNote(t, r, "_callP", migrate.Mapped, "the call performs the usage spin of the part connected to the port p")
	wantNote(t, r, "_callCmdTgt", migrate.Mapped, "the call performs the usage spin of the target this.motor; its port cmd is not written, as a v2 perform names the operation on the object")
	wantNote(t, r, "_callCmd", migrate.Approximated, "several edges lead to the node, which waits for all of them through the join 'join'")
	wantNote(t, r, "_callLoose", migrate.Approximated, "the call runs in the caller's context: no connector of Drive joins its port loose to a part")

	s := session(t, r)
	meta(t, s, "%instantiate Drive")
	meta(t, s, "%action Drive::Run #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : motor.speed"); !strings.Contains(out, "= 40.0") {
		t.Errorf("the calls did not spin the motor: %s", out)
	}
}
