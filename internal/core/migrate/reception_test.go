package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// heaterReceptions is a block with a reception whose method stores the signal's
// attribute, a reception without a method, one whose method is a state machine,
// one whose signal is not in the document, and a reception parameter that
// matches no attribute of its signal.
const heaterReceptions = `
    <packagedElement xmi:type="uml:Package" xmi:id="_sigs" name="Signals">
      <packagedElement xmi:type="uml:Signal" xmi:id="_setLevel" name="SetLevel">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_slValue" name="value">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedAttribute>
      </packagedElement>
      <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
      <packagedElement xmi:type="uml:Signal" xmi:id="_reset" name="Reset"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_heater" name="Heater">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_level" name="level">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_level0" value="0.0"/>
      </ownedAttribute>
      <ownedReception xmi:type="uml:Reception" xmi:id="_rcvSet" name="SetLevel" signal="_setLevel" method="_apply">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_rpValue" name="value" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_rpExtra" name="extra" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedReception>
      <ownedReception xmi:type="uml:Reception" xmi:id="_rcvStop" name="Stop" signal="_stop"/>
      <ownedReception xmi:type="uml:Reception" xmi:id="_rcvReset" name="Reset" signal="_reset" method="_resetting"/>
      <ownedReception xmi:type="uml:Reception" xmi:id="_rcvAway" name="Away" signal="_elsewhere"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_apply" name="Apply Level" specification="_rcvSet">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_apValue" name="value" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_apGain" name="gain" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
          <defaultValue xmi:type="uml:LiteralReal" xmi:id="_apGain0" value="1.0"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_apnValue" name="value" parameter="_apValue"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set level" structuralFeature="_level" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of" source="_apnValue" target="_setVal"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_resetting" name="Resetting">
        <region xmi:type="uml:Region" xmi:id="_rr">
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const heaterApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_heater"/>`

// A reception with a method becomes an action def of the block that accepts the
// signal and performs the method, its in parameters bound to the payload's
// attributes of the same name; one without a method only accepts; a state
// machine method and a signal outside the document are refused with the reason.
// Performing the usage on an object of the block and sending it the signal
// runs the method on that object.
func TestReceptionsAcceptAndPerformTheirMethod(t *testing.T) {
	r := migrateDocument(t, heaterReceptions, heaterApplications)
	for _, line := range []string{
		"action def SetLevel {",
		"first start then receive;",
		"action receive accept setLevel : Signals::SetLevel;",
		"first receive then run;",
		"action run : 'Apply Level' { in value = setLevel.value; }",
		"first run then done;",
		"action setLevel : SetLevel;",
		"action def Stop {",
		"action receive accept stop : Signals::Stop;",
		"first receive then done;",
		"action stop : Stop;",
		"action def Reset {",
		"action receive accept reset : Signals::Reset;",
		"action reset : Reset;",
		"comment /* reception 'Away' */",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "in gain =") {
		t.Errorf("the method's parameter gain, which no signal attribute names, was bound:\n%s", r.Notation)
	}
	wantNote(t, r, "_rcvSet", migrate.Mapped, "written as an action def accepting SetLevel and performing its method Heater::Apply Level, which its owner's usage setLevel runs")
	wantNote(t, r, "_rpValue", migrate.Mapped, "stands for the signal's attribute value, which the accepted payload carries")
	wantNote(t, r, "_rpExtra", migrate.Unmapped, "the parameter extra matches no attribute of the signal")
	wantNote(t, r, "_rcvStop", migrate.Approximated, "the reception has no method, so it only accepts the signal")
	wantNote(t, r, "_rcvReset", migrate.Approximated, "the method Heater::Resetting has no action def to perform; the reception only accepts the signal")
	wantNote(t, r, "_rcvAway", migrate.Unmapped, "signal")
	wantNote(t, r, "_apply", migrate.Mapped, "")

	s := session(t, r)
	meta(t, s, "%instantiate Heater")
	meta(t, s, "%action Heater::SetLevel #1")
	meta(t, s, "%step")
	if out := meta(t, s, "%step"); !strings.Contains(out, "State: Waiting") {
		t.Errorf("the reception is not waiting at its accept:\n%s", out)
	}
	if out := meta(t, s, "%send Signals::SetLevel(value=3.5)"); !strings.Contains(out, "Sent SetLevel") {
		t.Errorf("%%send SetLevel: %s", out)
	}
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the reception did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : level"); !strings.Contains(out, "= 3.5") {
		t.Errorf("the method did not store the signal's value: %s", out)
	}
}
