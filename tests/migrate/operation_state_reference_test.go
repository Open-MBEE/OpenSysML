package migrate_test

import (
	"strings"
	"testing"
)

// doorOpenedByItsState is a block whose state runs the method of one of its
// operations as the state's entry, and whose activity Run calls the operation
// on the object a ReadSelfAction feeds to the target pin.
const doorOpenedByItsState = `
    <packagedElement xmi:type="uml:Class" xmi:id="_door" name="Door">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_angle" name="angle">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_angle0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_open" name="Open" method="_openM"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_openM" name="OpenMethod" specification="_open">
        <node xmi:type="uml:InitialNode" xmi:id="_oi"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_ninety" name="ninety">
          <value xmi:type="uml:LiteralInteger" xmi:id="_ninetyV" value="90"/>
          <result xmi:type="uml:OutputPin" xmi:id="_ninetyOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setA" name="set angle" structuralFeature="_angle" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setAVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_of"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe1" source="_oi" target="_ninety"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_oof" source="_ninetyOut" target="_setAVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe2" source="_setA" target="_of"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:ReadSelfAction" xmi:id="_self" name="self">
          <result xmi:type="uml:OutputPin" xmi:id="_selfOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_callOpen" name="open it" operation="_open">
          <target xmi:type="uml:InputPin" xmi:id="_callOpenT" name="target"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_self"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_rof" source="_selfOut" target="_callOpenT"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callOpen" target="_rf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_doorLife" name="DoorLife">
        <region xmi:type="uml:Region" xmi:id="_dlr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_dlinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_opening" name="opening" entry="_openM"/>
          <transition xmi:type="uml:Transition" xmi:id="_dlt0" source="_dlinit" target="_opening"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const doorOpenedByItsStateApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_door"/>`

// An operation whose method a state runs keeps its def: the state's entry is
// typed by it and binds the machine's context, since a usage of the block is
// out of the state's reach and no action is typed by a usage.
func TestOperationRunByAStateKeepsItsDef(t *testing.T) {
	r := migrateDocument(t, doorOpenedByItsState, doorOpenedByItsStateApplications)
	for _, line := range []string{
		"action def Open {",
		"in ref context : Door[1];",
		"assign context.angle := value;",
		"entry action : Open { in ref :>> context = DoorLife::context; }",
		"action open : Open { in ref :>> context = this; }",
		"perform action 'open it' ::> open;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "entry action : open;")
	wantClean(t, "t.sysml", r)

	s := session(t, r)
	meta(t, s, "%instantiate Door")
	meta(t, s, "%state Door::DoorLife #1")
	meta(t, s, "%advance 0")
	if out := meta(t, s, "%eval in #1 : angle"); !strings.Contains(out, "= 90") {
		t.Errorf("the state's entry did not run the operation's method on the object: %s", out)
	}

	s = session(t, r)
	meta(t, s, "%instantiate Door")
	meta(t, s, "%action Door::run #1")
	meta(t, s, "%continue")
	if out := meta(t, s, "%eval in #1 : angle"); !strings.Contains(out, "= 90") {
		t.Errorf("the call on the target pin's object did not run the operation on it: %s", out)
	}
}
