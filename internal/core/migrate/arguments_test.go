package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// unarguedCalls is a block whose activity calls three behaviors without argument
// pins: one requiring its parameter, one whose parameter admits no value, and one
// whose parameter has a default; a fourth call names an operation without arguments,
// and a fifth passes the first call's result, which never holds a value.
const unarguedCalls = `
    <packagedElement xmi:type="uml:Class" xmi:id="_ctl" name="Ctl">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_seen" name="seen">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_seen0" value="0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_op" name="Tune" method="_tuning">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_opGain" name="gain" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_tuning" name="Tuning" specification="_op">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tGain" name="gain" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_needs" name="Needs">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_needsIn" name="image" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_needsOut" name="found" direction="out">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_admits" name="Admits">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_admitsIn" name="image" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
          <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_admitsLo" value="0"/>
          <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_admitsHi" value="1"/>
        </ownedParameter>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_defaults" name="Defaults">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_defaultsIn" name="image" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
          <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_defaultsV" value="3"/>
        </ownedParameter>
        <node xmi:type="uml:InitialNode" xmi:id="_dInit"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_dSet" name="set seen" structuralFeature="_seen" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_dSetVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_dApn" name="image" parameter="_defaultsIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_dE1" source="_dInit" target="_dSet"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_dOf" source="_dApn" target="_dSetVal"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callNeeds" name="find" behavior="_needs">
          <result xmi:type="uml:OutputPin" xmi:id="_callNeedsOut" name="found"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callAgain" name="refind" behavior="_needs">
          <argument xmi:type="uml:InputPin" xmi:id="_callAgainIn" name="image"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callAdmits" name="peek" behavior="_admits"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callDefaults" name="settle" behavior="_defaults"/>
        <node xmi:type="uml:CallOperationAction" xmi:id="_callOp" name="tune" operation="_op"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_callNeeds"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_callNeeds" target="_callAgain"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_callNeedsOut" target="_callAgainIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2b" source="_callAgain" target="_callAdmits"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_callAdmits" target="_callDefaults"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_callDefaults" target="_callOp"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e5" source="_callOp" target="_final"/>
      </ownedBehavior>
    </packagedElement>`

const unarguedApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_ctl"/>`

// A call passing no argument for a parameter the callee requires stands in for
// the call and is reported, and so does one whose argument only such a call
// produces; one whose parameter admits no value or has a default is written as
// the typed call, and the activity runs through all of them.
func TestCallsWithoutRequiredArgumentsAreReported(t *testing.T) {
	r := migrateDocument(t, unarguedCalls, unarguedApplications)
	for _, line := range []string{
		"action find {",
		"/* not migrated: CallBehaviorAction 'find' — the call passes no argument for the parameter image of Ctl::Needs, which must hold a value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing */",
		"action refind {",
		"/* not migrated: CallBehaviorAction 'refind' — the pin 'image' it passes for the parameter image of Ctl::Needs, which must hold a value, receives none: 'find', which feeds it, produces no value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing */",
		"action peek : Admits;",
		"action settle : Defaults;",
		"action tune {",
		"/* not migrated: CallOperationAction 'tune' — the call passes no argument for the parameter gain of Ctl::Tune, which must hold a value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing */",
		"first find then refind;",
		"first refind then peek;",
		"first peek then settle;",
		"first settle then tune;",
		"first tune then final;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "action find : Needs;") || strings.Contains(string(r.Notation), "action tune : Ctl::Tune;") {
		t.Errorf("a call lacking a required argument was written as the typed call:\n%s", r.Notation)
	}
	wantNote(t, r, "_callNeeds", migrate.Approximated, "the call passes no argument for the parameter image of Ctl::Needs, which must hold a value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing")
	wantNote(t, r, "_callOp", migrate.Approximated, "the call passes no argument for the parameter gain of Ctl::Tune, which must hold a value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing")
	wantNote(t, r, "_callAgain", migrate.Approximated, "the pin 'image' it passes for the parameter image of Ctl::Needs, which must hold a value, receives none: 'find', which feeds it, produces no value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing")
	wantNote(t, r, "_callAdmits", migrate.Mapped, "")
	wantNote(t, r, "_callDefaults", migrate.Mapped, "")

	s := session(t, r)
	meta(t, s, "%instantiate Ctl")
	meta(t, s, "%action Ctl::Run #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : seen"); !strings.Contains(out, "= 3") {
		t.Errorf("the call with a defaulted parameter did not run its body: %s", out)
	}
}

// dryOutputs is a Cache whose Fetch gives its out parameter only what an opaque
// action computes, and whose Run passes Fetch's result to Use, which requires it,
// and sends it in a Fresh, whose attribute must hold a value.
const dryOutputs = `
    <packagedElement xmi:type="uml:Class" xmi:id="_cache" name="Cache">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_seen" name="seen">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_seen0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_fetch" name="Fetch">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_fetchOut" name="image" direction="out">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
        <node xmi:type="uml:InitialNode" xmi:id="_fInit"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_fCompute" name="compute">
          <language>JavaScript</language>
          <body>var t = java.lang.System.currentTimeMillis();</body>
          <outputValue xmi:type="uml:OutputPin" xmi:id="_fComputeOut" name="t"/>
        </node>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_fApn" name="image" parameter="_fetchOut"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fE1" source="_fInit" target="_fCompute"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_fOf" source="_fComputeOut" target="_fApn"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_use" name="Use">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_useIn" name="image" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
        <node xmi:type="uml:InitialNode" xmi:id="_uInit"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_uSet" name="set seen" structuralFeature="_seen" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_uSetVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_uApn" name="image" parameter="_useIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_uE1" source="_uInit" target="_uSet"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_uOf" source="_uApn" target="_uSetVal"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callFetch" name="fetch" behavior="_fetch">
          <result xmi:type="uml:OutputPin" xmi:id="_callFetchOut" name="image"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callUse" name="apply" behavior="_use">
          <argument xmi:type="uml:InputPin" xmi:id="_callUseIn" name="image"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_notify" name="notify" signal="_fresh">
          <argument xmi:type="uml:InputPin" xmi:id="_notifyIn" name="image"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_callFetch"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_callFetch" target="_callUse"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_callFetchOut" target="_callUseIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_callUse" target="_notify"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of2" source="_callFetchOut" target="_notifyIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_notify" target="_final"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_fresh" name="Fresh">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_freshImage" name="image">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>`

const dryOutputsApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_cache"/>`

// A call's result that its callee gives no value is not flowed on, and a call or
// send requiring that value stands in for itself; the caller still runs through.
func TestResultsTheCalleeNeverProducesAreNotFlowedOn(t *testing.T) {
	r := migrateDocument(t, dryOutputs, dryOutputsApplications)
	for _, line := range []string{
		"action fetch : Fetch;",
		"action apply {",
		"/* not migrated: CallBehaviorAction 'apply' — the pin 'image' it passes for the parameter image of Cache::Use, which must hold a value, receives none: 'fetch', which feeds it, produces no value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing */",
		"/* flow fetch.image to apply.image not written: nothing in the called Cache::Fetch gives its parameter image a value */",
		"first fetch then apply;",
		"first apply then notify;",
		"action notify {",
		"/* not migrated: SendSignalAction 'notify' — the pin 'image' it passes for the attribute image of Fresh, which must hold a value, receives none: 'fetch', which feeds it, produces no value; v1 sends the signal without it, which v2 does not admit, so the action carries the token and performs nothing */",
		"first notify then final;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "flow fetch.image to apply.image;")
	wantNoLine(t, r.Notation, "send new Fresh(image);")
	wantNote(t, r, "_notify", migrate.Approximated, "the pin 'image' it passes for the attribute image of Fresh, which must hold a value, receives none: 'fetch', which feeds it, produces no value; v1 sends the signal without it, which v2 does not admit, so the action carries the token and performs nothing")
	wantNote(t, r, "_callUse", migrate.Approximated, "the pin 'image' it passes for the parameter image of Cache::Use, which must hold a value, receives none: 'fetch', which feeds it, produces no value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing")
	wantNote(t, r, "_of1", migrate.Approximated, "the flow is kept as a comment: nothing in the called Cache::Fetch gives its parameter image a value, so none reaches 'image'")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Cache")
	meta(t, s, "%action Cache::Run #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
}

// omittedSignalArguments is a Siren whose Alert carries a code and a level, both
// required, a tag that admits no value and a flag with a default. Its Raise sends
// an Alert with a pin for the code only, then one with pins for code and level;
// a Hall's Drill sends the same alert to a siren with one argument, then with two.
const omittedSignalArguments = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_alert" name="Alert">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_aCode" name="code">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_aLevel" name="level">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_aTag" name="tag">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_aTagLo" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_aTagHi" value="1"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_aFlag" name="flag">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_aFlag0" value="0"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_siren" name="Siren">
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_raise" name="Raise">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
          <value xmi:type="uml:LiteralInteger" xmi:id="_oneV" value="1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_two" name="two">
          <value xmi:type="uml:LiteralInteger" xmi:id="_twoV" value="2"/>
          <result xmi:type="uml:OutputPin" xmi:id="_twoOut" name="result"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_three" name="three">
          <value xmi:type="uml:LiteralInteger" xmi:id="_threeV" value="3"/>
          <result xmi:type="uml:OutputPin" xmi:id="_threeOut" name="result"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_sendShort" name="warn" signal="_alert">
          <argument xmi:type="uml:InputPin" xmi:id="_shortCode" name="code"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_sendFull" name="alarm" signal="_alert">
          <argument xmi:type="uml:InputPin" xmi:id="_fullCode" name="code"/>
          <argument xmi:type="uml:InputPin" xmi:id="_fullLevel" name="level"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_one"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_oneOut" target="_shortCode"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_sendShort" target="_two"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_two" target="_three"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of2" source="_twoOut" target="_fullCode"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of3" source="_threeOut" target="_fullLevel"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_sendFull" target="_final"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_hall" name="Hall">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_hs" name="s" type="_siren" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_short" name="Short">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_ls1" name="s" represents="_hs"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rShort" covered="_ls1" message="_mShort"/>
        <message xmi:type="uml:Message" xmi:id="_mShort" name="warn" messageSort="asynchSignal" signature="_alert" receiveEvent="_rShort">
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mShortCode" value="1"/>
        </message>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_full" name="Full">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_ls2" name="s" represents="_hs"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rFull" covered="_ls2" message="_mFull"/>
        <message xmi:type="uml:Message" xmi:id="_mFull" name="alarm" messageSort="asynchSignal" signature="_alert" receiveEvent="_rFull">
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mFullLevel" name="level" value="2"/>
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mFullCode" name="code" value="1"/>
        </message>
      </ownedBehavior>
    </packagedElement>`

const omittedSignalApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_siren"/>
  <sysml:Block xmi:id="_b2" base_Class="_hall"/>`

// A send with no pin for an attribute the signal requires stands in for itself and
// is reported, as is a scenario whose message binds no argument to one; an attribute
// that admits no value or has a default needs none. The sends that bind both run.
func TestSendsOmittingRequiredSignalAttributesAreReported(t *testing.T) {
	r := migrateDocument(t, omittedSignalArguments, omittedSignalApplications)
	for _, line := range []string{
		"action warn {",
		"/* not migrated: SendSignalAction 'warn' — the send passes no argument for the attribute level of Alert, which must hold a value; v1 sends the signal without it, which v2 does not admit, so the action carries the token and performs nothing */",
		"send new Alert(code, level);",
		"/* not migrated: Interaction 'Short' — the message 'warn' binds no argument to the attribute level of Alert, which must hold a value */",
		"action alarm send new Alert(level = 2, code = 1) to this.s;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "send new Alert(code);")
	wantNote(t, r, "_sendShort", migrate.Approximated, "the send passes no argument for the attribute level of Alert, which must hold a value; v1 sends the signal without it, which v2 does not admit, so the action carries the token and performs nothing")
	wantNote(t, r, "_short", migrate.Unmapped, "the message 'warn' binds no argument to the attribute level of Alert, which must hold a value")
	wantNote(t, r, "_mFull", migrate.Mapped, "written as a send to this.s")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Siren")
	meta(t, s, "%action Siren::Raise #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "ompleted") {
		t.Errorf("the run did not complete:\n%s", out)
	}
}

// unwrittenValue is a Sky whose Aim requires coordinates, and whose Run calls it
// on a value action holding the string "0", which is no value of the structured
// Coords the action's result is typed by; the value action follows the call.
const unwrittenValue = `
    <packagedElement xmi:type="uml:DataType" xmi:id="_coords" name="Coords">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ra" name="ra">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_sky" name="Sky">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_aimed" name="aimed">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_aimed0" value="false"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_aim" name="Aim">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_aimIn" name="target" direction="in" type="_coords"/>
        <node xmi:type="uml:InitialNode" xmi:id="_aInit"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_aSet" name="set aimed" structuralFeature="_aimed" isReplaceAll="true">
          <value xmi:type="uml:ValuePin" xmi:id="_aSetVal" name="value">
            <value xmi:type="uml:LiteralBoolean" xmi:id="_aSetTrue" value="true"/>
          </value>
        </node>
        <edge xmi:type="uml:ControlFlow" xmi:id="_aE1" source="_aInit" target="_aSet"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callAim" name="aim" behavior="_aim">
          <argument xmi:type="uml:InputPin" xmi:id="_callAimIn" name="target"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_zero" name="zero">
          <result xmi:type="uml:OutputPin" xmi:id="_zeroOut" name="result" type="_coords"/>
          <value xmi:type="uml:LiteralString" xmi:id="_zeroVal" value="0"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_zero"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_zero" target="_callAim"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of" source="_zeroOut" target="_callAimIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_callAim" target="_final"/>
      </ownedBehavior>
    </packagedElement>`

const unwrittenValueApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_sky"/>
  <sysml:ValueType xmi:id="_vt1" base_DataType="_coords"/>`

// A value action whose value is not written produces none, so a call requiring
// that value stands in for itself, whichever of the two the model lists first.
func TestUnwrittenValuesAreNotPassedToCalls(t *testing.T) {
	r := migrateDocument(t, unwrittenValue, unwrittenValueApplications)
	for _, line := range []string{
		"action zero {",
		"/* not migrated: ValueSpecificationAction 'zero' — the value 0 is not written: the literal \"0\" is not a value of Coords, which has no scalar base */",
		"action aim {",
		"/* not migrated: CallBehaviorAction 'aim' — the pin 'target' it passes for the parameter target of Sky::Aim, which must hold a value, receives none: 'zero', which feeds it, produces no value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing */",
		"/* flow zero.result to aim.target not written: 'zero' is not migrated and produces no value */",
		"first zero then aim;",
		"first aim then final;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "action aim : Aim;")
	wantNoLine(t, r.Notation, "flow zero.result to aim.target;")
	wantNote(t, r, "_zero", migrate.Approximated, "the value 0 is not written: the literal \"0\" is not a value of Coords, which has no scalar base")
	wantNote(t, r, "_callAim", migrate.Approximated, "the pin 'target' it passes for the parameter target of Sky::Aim, which must hold a value, receives none: 'zero', which feeds it, produces no value; v1 runs the callee without it, which v2 does not admit, so the action carries the token and performs nothing")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Sky")
	meta(t, s, "%action Sky::Run #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
}
