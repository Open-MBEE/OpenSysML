package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// motorNetwork is a net of two motors whose operation Spin sets the speed it is
// given. Turn calls Spin on whatever motor its parameter is handed; Kick reads
// the part m2 and calls Turn with it.
const motorNetwork = `
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rpm" name="rpm">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_rpm0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_spin" name="Spin" method="_spinning">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spinTo" name="to" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_spinning" name="Spinning" specification="_spin">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spinningTo" name="to" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_spinningToN" name="to" parameter="_spinningTo"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setRpm" name="set rpm" structuralFeature="_rpm" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setRpmVal" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_sof" source="_spinningToN" target="_setRpmVal"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_net" name="Net">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_m1" name="m1" type="_motor" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_m2" name="m2" type="_motor" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_turn" name="Turn">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_turnMotor" name="motor" direction="in" type="_motor"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_turnMotorN" name="motor" parameter="_turnMotor"/>
        <node xmi:type="uml:InitialNode" xmi:id="_ti"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_ten" name="ten">
          <value xmi:type="uml:LiteralInteger" xmi:id="_tenV" value="10"/>
          <result xmi:type="uml:OutputPin" xmi:id="_tenOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_call" name="spin" operation="_spin">
          <argument xmi:type="uml:InputPin" xmi:id="_callTo" name="to"/>
          <target xmi:type="uml:InputPin" xmi:id="_callTgt" name="target"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_tf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_te1" source="_ti" target="_ten"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_tof1" source="_tenOut" target="_callTo"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_tof2" source="_turnMotorN" target="_callTgt"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_te2" source="_call" target="_tf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_kick" name="Kick">
        <node xmi:type="uml:InitialNode" xmi:id="_kinit"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readM2" name="read m2" structuralFeature="_m2">
          <result xmi:type="uml:OutputPin" xmi:id="_readM2Out" name="result"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_kcall" name="turn" behavior="_turn">
          <argument xmi:type="uml:InputPin" xmi:id="_kcallIn" name="motor"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_kfinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ke1" source="_kinit" target="_readM2"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_kof1" source="_readM2Out" target="_kcallIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ke2" source="_kcall" target="_kfinal"/>
      </ownedBehavior>
    </packagedElement>`

// createdTarget creates a Motor and calls Spin on it.
const createdTarget = `
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedOperation xmi:type="uml:Operation" xmi:id="_spin" name="Spin"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Activity" xmi:id="_build" name="Build">
      <node xmi:type="uml:InitialNode" xmi:id="_bi"/>
      <node xmi:type="uml:CreateObjectAction" xmi:id="_create" name="create" classifier="_motor">
        <result xmi:type="uml:OutputPin" xmi:id="_createOut" name="result" type="_motor"/>
      </node>
      <node xmi:type="uml:CallOperationAction" xmi:id="_call" name="spin" operation="_spin">
        <target xmi:type="uml:InputPin" xmi:id="_callTgt" name="target"/>
      </node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_bf"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_be1" source="_bi" target="_create"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_bof" source="_createOut" target="_callTgt"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_be2" source="_call" target="_bf"/>
    </packagedElement>`

var motorNetworkApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_motor"/>
  <sysml:Block xmi:id="_s2" base_Class="_net"/>`

// A call whose target pin a flow feeds with an object not read from this is
// written with the pin as a parameter typed by the operation's class, to which
// the operation's context is bound. The result runs Spin on the motor the
// parameter holds, and on no other.
func TestCallOperationRunsOnTheObjectItsTargetPinHolds(t *testing.T) {
	r := migrateDocument(t, motorNetwork, motorNetworkApplications)
	for _, line := range []string{
		"action spin : Motor::Spin { in ref :>> context = target; in 'to'[1]; in target : Motor[1]; }",
		"succession flow ten.result to spin.'to';",
		"bind spin.target = motor;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "Turn::context") {
		t.Errorf("the call was bound to the caller's context:\n%s", r.Notation)
	}
	wantNote(t, r, "_call", migrate.Mapped, "the call runs on the object its target pin target holds")
	wantNote(t, r, "_callTgt", migrate.Mapped, "the target pin is the parameter target, typed by Motor, the class owning the operation")
	for _, id := range []string{"_tof1", "_tof2", "_callTo"} {
		wantNote(t, r, id, migrate.Mapped, "")
	}

	s := session(t, r)
	meta(t, s, "%instantiate Net")
	meta(t, s, "%action Net::kick #1")
	meta(t, s, "%continue")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : m2.rpm"); !strings.Contains(out, "= 10") {
		t.Errorf("the motor the target pin held did not spin: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : m1.rpm"); !strings.Contains(out, "= 0") {
		t.Errorf("a motor the target pin did not hold spun: %s", out)
	}
}

// A call on an object a CreateObjectAction creates keeps its target pin and the
// flow into it, but CreateObjectAction is not migrated: the ledger says the pin
// holds no object, and a run stops at the call rather than running it elsewhere.
func TestCallOperationOnACreatedObjectActsOnAnEmptyPin(t *testing.T) {
	r := migrateDocument(t, createdTarget, motorNetworkApplications)
	for _, line := range []string{
		"action spin : Motor::Spin { in target : Motor[1]; }",
		"flow create.result to spin.target;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_call", migrate.Mapped, "the object is the one 'create' creates, and CreateObjectAction is not migrated, so the pin holds no object and the call acts on an empty pin")
	wantNote(t, r, "_bof", migrate.Approximated, "its source 'create' is not migrated and produces no value")

	s := session(t, r)
	meta(t, s, "%action Build")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "node create produced no value at result") {
		t.Errorf("a run did not stop at the call's empty target pin: %s", out)
	}
}

// startedBehaviors starts the behavior of an object a CreateObjectAction creates,
// then the classifier behavior of the same object.
const startedBehaviors = `
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_build" name="Build">
      <node xmi:type="uml:InitialNode" xmi:id="_bi"/>
      <node xmi:type="uml:CreateObjectAction" xmi:id="_create" name="create" classifier="_motor">
        <result xmi:type="uml:OutputPin" xmi:id="_createOut" name="result" type="_motor"/>
      </node>
      <node xmi:type="uml:ForkNode" xmi:id="_fork" name="copy"/>
      <node xmi:type="uml:StartObjectBehaviorAction" xmi:id="_startObj" name="launch">
        <object xmi:type="uml:InputPin" xmi:id="_startObjIn" name="object"/>
      </node>
      <node xmi:type="uml:StartClassifierBehaviorAction" xmi:id="_startCls" name="launch classifier">
        <object xmi:type="uml:InputPin" xmi:id="_startClsIn" name="object"/>
      </node>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_bf"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_be1" source="_bi" target="_create"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_bof1" source="_createOut" target="_fork"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_bof2" source="_fork" target="_startObjIn"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="_bof3" source="_fork" target="_startClsIn"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_be2" source="_startObj" target="_startCls"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_be3" source="_startCls" target="_bf"/>
    </packagedElement>`

// v2 has no action that starts an object's behavior: an object's exhibited states
// and performed actions start when it does. Either start is a placeholder keeping
// its pin and its place in the flow, reported unmapped with that reason.
func TestStartBehaviorActionsArePlaceholdersKeepingTheirPins(t *testing.T) {
	r := migrateDocument(t, startedBehaviors, motorNetworkApplications)
	for _, line := range []string{
		"action launch {",
		"action 'launch classifier' {",
		"in object[1];",
		"first launch then 'join';",
		"flow create.result to launch.object;",
		"/* not migrated: StartObjectBehaviorAction 'launch'",
		"/* not migrated: StartClassifierBehaviorAction 'launch classifier'",
	} {
		wantLine(t, r.Notation, line)
	}
	why := "a v2 object's exhibited states and performed actions start when the object does, and no v2 action starts them later"
	wantNote(t, r, "_startObj", migrate.Unmapped, "no v2 form for a UML StartObjectBehaviorAction: "+why)
	wantNote(t, r, "_startCls", migrate.Unmapped, "no v2 form for a UML StartClassifierBehaviorAction: "+why)
	session(t, r)
}
