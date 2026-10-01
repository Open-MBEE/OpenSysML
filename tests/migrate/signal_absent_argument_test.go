package migrate_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// absentSignalArguments is a tracker whose activity Aim sends its parameter
// target as the target of a Point signal, whose target attribute declares no
// multiplicity (1..1 in UML), and as its note attribute, declared 0..1. Session
// calls Aim passing a result a behavior-less action computes, so the parameter
// may hold no value, and so may the pins the send passes.
const absentSignalArguments = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_point" name="Point">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ptTarget" name="target">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ptNote" name="note">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_ptNoteLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_ptNoteHi" value="1"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_tracker" name="Tracker">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_aim" name="Aim">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_aimTarget" name="target" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_aimApn" name="target" parameter="_aimTarget"/>
        <node xmi:type="uml:InitialNode" xmi:id="_aInit"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send point" signal="_point" onPort="_tx">
          <argument xmi:type="uml:InputPin" xmi:id="_sendTarget" name="target"/>
          <argument xmi:type="uml:InputPin" xmi:id="_sendNote" name="note"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_aFinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_aE1" source="_aInit" target="_send"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_aE2" source="_send" target="_aFinal"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_aOf1" source="_aimApn" target="_sendTarget"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_aOf2" source="_aimApn" target="_sendNote"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_session" name="Session">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_locate" name="locate">
          <result xmi:type="uml:OutputPin" xmi:id="_locateOut" name="where">
            <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          </result>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_aimCall" name="aim" behavior="_aim">
          <argument xmi:type="uml:InputPin" xmi:id="_aimCallTarget" name="target"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_locate"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_locate" target="_aimCall"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_aimCall" target="_final"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_locateOut" target="_aimCallTarget"/>
      </ownedBehavior>
    </packagedElement>`

const absentSignalArgumentApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_tracker"/>`

// A send whose argument pin may hold no value binds it to the signal's attribute
// all the same, and the ledger says what happens when the pin holds none: an
// attribute declaring no multiplicity (1..1) cannot hold none, so a run reaching
// the send with none stops there, where v1 ran on (UML enforces no slot's
// multiplicity). An attribute declared 0..1 holds none, and its argument is not noted.
func TestSendOfAbsentArgumentToRequiredAttributeIsLedgered(t *testing.T) {
	r := migrateDocument(t, absentSignalArguments, absentSignalArgumentApplications)
	wantNote(t, r, "_aimTarget", migrate.Approximated, "it is declared admitting no value: the pin 'target' the call 'aim' in Tracker::Session passes for it receives none: 'locate', which feeds it, produces no value, and v1 runs the callee without one")
	wantNote(t, r, "_sendTarget", migrate.Approximated, "it is declared admitting no value: the parameter target of Tracker::Aim, which feeds it, admits no value")
	wantNote(t, r, "_send", migrate.Approximated, "the argument pin target admits no value, which the signal's target, declared holding one, cannot: a run reaching the send with none stops at it")
	wantLine(t, r.Notation, "send new Point(target, note) via tx;")
}
