package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// rigInteraction is a rig whose controller part calls an operation on a motor
// nested in a drive part, receives the reply into an attribute, and sends
// signals inside alt, opt, loop and par fragments; a second interaction stands a
// lifeline for a property no part of the rig reaches, a third is a test case.
const rigInteraction = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_goN" name="n">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Signal" xmi:id="_done" name="Done"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_motor" name="Motor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_speed" name="speed">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_speed0" value="0.0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_spin" name="Spin" method="_spinning">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRpm" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spTimes" name="times" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
          <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_spTimes0" value="1"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRes" name="result" direction="return">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_brake" name="Brake">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_brForce" name="force" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_spinning" name="Spinning" specification="_spin">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRpm2" name="rpm" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spTimes2" name="times" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_spRes2" name="result" direction="return">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_apnRpm" name="rpm" parameter="_spRpm2"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_apnRes" name="result" parameter="_spRes2"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set speed" structuralFeature="_speed" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_read" name="read speed" structuralFeature="_speed">
          <result xmi:type="uml:OutputPin" xmi:id="_readOut" name="result"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_ofRpm" source="_apnRpm" target="_setVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_cfSet" source="_set" target="_read"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_ofRes" source="_readOut" target="_apnRes"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_drive" name="Drive">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_dMotor" name="motor" type="_motor" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_ctrl" name="Controller">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_got" name="got">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_got0" value="0.0"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_other" name="Other">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_oMotor" name="motor" type="_motor" aggregation="composite"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_rig" name="Rig">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rCtrl" name="ctrl" type="_ctrl" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_rDrive" name="drive" type="_drive" aggregation="composite"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_mode" name="mode">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_mode0" value="1"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_spinup" name="Spinup">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lc" name="c" represents="_rCtrl"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lm" name="m" represents="_dMotor"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sSpin" covered="_lc" message="_mSpin"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rSpin" covered="_lm" message="_mSpin"/>
        <fragment xmi:type="uml:BehaviorExecutionSpecification" xmi:id="_exec" covered="_lm" start="_rSpin" finish="_sRet"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sRet" covered="_lm" message="_mRet"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rRet" covered="_lc" message="_mRet"/>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_alt" interactionOperator="alt">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_altFast">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_altFastG">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_altFastS"><body>mode == 1</body></specification>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sGo" covered="_lc" message="_mGo"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rGo" covered="_lm" message="_mGo"/>
          </operand>
          <operand xmi:type="uml:InteractionOperand" xmi:id="_altElse">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_altElseG">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_altElseS"><body>else</body></specification>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sBrake" covered="_lc" message="_mBrake"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rBrake" covered="_lm" message="_mBrake"/>
          </operand>
        </fragment>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_loop" interactionOperator="loop">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_loopOp">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_loopG">
              <minint xmi:type="uml:LiteralInteger" xmi:id="_loopMin" value="2"/>
              <maxint xmi:type="uml:LiteralInteger" xmi:id="_loopMax" value="2"/>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sSpin2" covered="_lc" message="_mSpin2"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rSpin2" covered="_lm" message="_mSpin2"/>
          </operand>
        </fragment>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_par" interactionOperator="par">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_par1">
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sGo2" covered="_lc" message="_mGo2"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rGo2" covered="_lm" message="_mGo2"/>
          </operand>
          <operand xmi:type="uml:InteractionOperand" xmi:id="_par2">
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sDone" covered="_lm" message="_mDone"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rDone" covered="_lc" message="_mDone"/>
          </operand>
        </fragment>
        <fragment xmi:type="uml:CombinedFragment" xmi:id="_opt" interactionOperator="opt">
          <operand xmi:type="uml:InteractionOperand" xmi:id="_optOp">
            <guard xmi:type="uml:InteractionConstraint" xmi:id="_optG">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_optS"><body>ctrl.got &gt; 10.0</body></specification>
            </guard>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sNew" covered="_lc" message="_mNew"/>
            <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_rNew" covered="_lm" message="_mNew"/>
          </operand>
        </fragment>
        <message xmi:type="uml:Message" xmi:id="_mSpin" name="spin" messageSort="synchCall" signature="_spin" sendEvent="_sSpin" receiveEvent="_rSpin">
          <argument xmi:type="uml:LiteralReal" xmi:id="_mSpinRpm" value="30.0"/>
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mSpinTimes" name="times" value="2"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_mRet" name="spun" messageSort="reply" signature="_spin" sendEvent="_sRet" receiveEvent="_rRet">
          <argument xmi:type="uml:Expression" xmi:id="_mRetArg" symbol="=">
            <operand xmi:type="uml:LiteralString" xmi:id="_mRetTarget" value="got"/>
            <operand xmi:type="uml:LiteralReal" xmi:id="_mRetValue" value="30.0"/>
          </argument>
        </message>
        <message xmi:type="uml:Message" xmi:id="_mGo" name="go" messageSort="asynchSignal" signature="_go" sendEvent="_sGo" receiveEvent="_rGo">
          <argument xmi:type="uml:LiteralInteger" xmi:id="_mGoN" value="3"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_mBrake" name="brake" messageSort="asynchCall" signature="_brake" sendEvent="_sBrake" receiveEvent="_rBrake">
          <argument xmi:type="uml:LiteralReal" xmi:id="_mBrakeF" value="0.5"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_mSpin2" messageSort="synchCall" signature="_spin" sendEvent="_sSpin2" receiveEvent="_rSpin2">
          <argument xmi:type="uml:LiteralReal" xmi:id="_mSpin2Rpm" value="40.0"/>
        </message>
        <message xmi:type="uml:Message" xmi:id="_mGo2" messageSort="asynchSignal" signature="_go" sendEvent="_sGo2" receiveEvent="_rGo2"/>
        <message xmi:type="uml:Message" xmi:id="_mDone" messageSort="asynchSignal" signature="_done" sendEvent="_sDone" receiveEvent="_rDone"/>
        <message xmi:type="uml:Message" xmi:id="_mNew" name="new" messageSort="createMessage" sendEvent="_sNew" receiveEvent="_rNew"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_astray" name="Astray">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_lo" name="o" represents="_oMotor"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_sO" covered="_lo" message="_mO"/>
        <message xmi:type="uml:Message" xmi:id="_mO" name="go" messageSort="asynchSignal" signature="_go" receiveEvent="_sO"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_trace" name="Spinup at 12.01">
        <ownedRule xmi:type="uml:TimeConstraint" xmi:id="_tc1" constrainedElement="_inv1"/>
        <ownedRule xmi:type="uml:TimeConstraint" xmi:id="_tc2" constrainedElement="_inv2"/>
        <lifeline xmi:type="uml:Lifeline" xmi:id="_trl" name="motor" represents="_dMotor"/>
        <fragment xmi:type="uml:StateInvariant" xmi:id="_inv1" covered="_trl"/>
        <fragment xmi:type="uml:StateInvariant" xmi:id="_inv2" covered="_trl"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Interaction" xmi:id="_tc" name="Spinup Test">
        <lifeline xmi:type="uml:Lifeline" xmi:id="_tlm" name="m" represents="_dMotor"/>
        <fragment xmi:type="uml:MessageOccurrenceSpecification" xmi:id="_tRSpin" covered="_tlm" message="_tmSpin"/>
        <message xmi:type="uml:Message" xmi:id="_tmSpin" name="spin" messageSort="synchCall" signature="_spin" receiveEvent="_tRSpin">
          <argument xmi:type="uml:LiteralReal" xmi:id="_tmSpinRpm" value="12.0"/>
        </message>
      </ownedBehavior>
    </packagedElement>`

const rigApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_motor"/>
  <sysml:Block xmi:id="_s2" base_Class="_drive"/>
  <sysml:Block xmi:id="_s3" base_Class="_ctrl"/>
  <sysml:Block xmi:id="_s4" base_Class="_other"/>
  <sysml:Block xmi:id="_s5" base_Class="_rig"/>
  <sysml:TestCase xmi:id="_s6" base_Behavior="_tc"/>`

// A call message performs the operation's usage on the object its lifeline
// stands for, reached through the part tree, with its arguments bound by
// position and by name; the reply assigns the call's result to the attribute it
// names; alt, opt, loop and par fragments become if, if, for and fork; a create
// message is reported, as is a timing trace of state invariants. The scenario validates, runs, and leaves the motor
// spinning at the last rpm and the controller holding the first.
func TestInteractionCallsRepliesAndFragments(t *testing.T) {
	r := migrateDocument(t, rigInteraction, rigApplications)
	for _, line := range []string{
		"action def Spinup {",
		"perform action spin : Motor::Spin ::> drive.motor.spin { in rpm = 30.0; in times = 2; }",
		"first start then spin;",
		"action spun {",
		"assign this.ctrl.got := spin.result;",
		"first spin then spun;",
		"action alt {",
		"if this.mode == 1 {",
		"action altOp1 {",
		"action go send new Go(n = 3) to this.drive.motor;",
		"else {",
		"action altOp2 {",
		"perform action brake : Motor::Brake ::> drive.motor.brake { in force = 0.5; }",
		"first spun then alt;",
		"action 'loop' {",
		"for i in 1..2 {",
		"action loopOp1 {",
		"perform action callSpin : Motor::Spin ::> drive.motor.spin { in rpm = 40.0; }",
		"first alt then 'loop';",
		"fork par;",
		"first 'loop' then par;",
		"action parOp1 {",
		"action sendGo send new Go() to this.drive.motor;",
		"first par then parOp1;",
		"first parOp1 then parEnd;",
		"action parOp2 {",
		"action sendDone send new Done() to this.ctrl;",
		"join parEnd;",
		"action opt {",
		"if this.ctrl.got > 10.0 {",
		"first parEnd then opt;",
		"first opt then done;",
		"/* not migrated: Interaction 'Astray' — the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches */",
		"verification def 'Spinup Test' {",
		"subject context : Rig;",
		"perform action spin : Motor::Spin ::> context.drive.motor.spin { in rpm = 12.0; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_spinup", migrate.Approximated, "written as a scenario of 8 steps, one per message in occurrence order")
	wantNote(t, r, "_lm", migrate.Mapped, "the lifeline stands for this.drive.motor, which the steps address")
	wantNote(t, r, "_mSpin", migrate.Mapped, "written as a call of Spin on this.drive.motor")
	wantNote(t, r, "_mRet", migrate.Approximated, "the value 30.0 the reply states for result is the operation's own result, which the call computes")
	wantNote(t, r, "_exec", migrate.Skipped, "the execution spans the steps between its occurrences")
	wantNote(t, r, "_alt", migrate.Mapped, "written as the action alt, a if over the operands")
	wantNote(t, r, "_altFast", migrate.Mapped, "its guard is the condition [this.mode == 1]")
	wantNote(t, r, "_altElseG", migrate.Mapped, "the guard is the operand's condition")
	wantNote(t, r, "_mBrake", migrate.Approximated, "the asynchronous call is performed to completion before the next step")
	wantNote(t, r, "_loop", migrate.Mapped, "written as the action 'loop', a for over the operands")
	wantNote(t, r, "_par", migrate.Mapped, "written as the action par, a fork over the operands")
	wantNote(t, r, "_mNew", migrate.Unmapped, "the message creates this.drive.motor, a part that exists for as long as its owner does")
	wantNote(t, r, "_sNew", migrate.Unmapped, "the occurrence belongs to the message 'new', which is not written")
	wantNote(t, r, "_astray", migrate.Unmapped, "the lifeline 'o' stands for 'motor' of Other, which no part of Rig reaches")
	wantNote(t, r, "_tc", migrate.Approximated, "written as a scenario of 1 steps")
	wantNote(t, r, "_trace", migrate.Unmapped, "the interaction has no message: it records 2 state invariant(s) under 2 time constraint(s), a timing trace, which no scenario step performs")

	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%action Rig::Spinup #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "completed") {
		t.Errorf("the scenario did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : drive.motor.speed"); !strings.Contains(out, "= 40.0") {
		t.Errorf("the loop's calls did not spin the motor to 40.0: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : ctrl.got"); !strings.Contains(out, "= 30.0") {
		t.Errorf("the reply did not store the call's result: %s", out)
	}
}
