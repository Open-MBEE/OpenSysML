package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// forkFedCall is a Meter whose Sample forks into a write of last and into a read
// of gain, whose value flows into the write's pin. Its Sweep reads gain once,
// then loops twice through a write of last fed by it.
const forkFedCall = `
    <packagedElement xmi:type="uml:Class" xmi:id="_meter" name="Meter">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_gain" name="gain">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_gain0" value="21"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_last" name="last">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_last0" value="0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_passes" name="passes">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_passes0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_sample" name="Sample">
        <node xmi:type="uml:InitialNode" xmi:id="_mi"/>
        <node xmi:type="uml:ForkNode" xmi:id="_mfork" name="fork"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readGain" name="read gain" structuralFeature="_gain">
          <result xmi:type="uml:OutputPin" xmi:id="_rgOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setLast" name="set last" structuralFeature="_last" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_slVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_mf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_me1" source="_mi" target="_mfork"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_me2" source="_mfork" target="_setLast"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_me3" source="_mfork" target="_readGain"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_mo1" source="_rgOut" target="_slVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_me4" source="_setLast" target="_mf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_sweep" name="Sweep">
        <node xmi:type="uml:InitialNode" xmi:id="_wi"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readGain2" name="read gain" structuralFeature="_gain">
          <result xmi:type="uml:OutputPin" xmi:id="_rg2Out" name="result"/>
        </node>
        <node xmi:type="uml:MergeNode" xmi:id="_wmerge" name="merge"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setLast2" name="set last" structuralFeature="_last" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_sl2Val" name="value"/>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_count" name="count">
          <language>JavaScript</language>
          <body>passes = passes + 1;</body>
        </node>
        <node xmi:type="uml:DecisionNode" xmi:id="_wdec" name="again"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_wf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we1" source="_wi" target="_readGain2"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we2" source="_readGain2" target="_wmerge"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we3" source="_wmerge" target="_setLast2"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_wo1" source="_rg2Out" target="_sl2Val"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we4" source="_setLast2" target="_count"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we5" source="_count" target="_wdec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we6" source="_wdec" target="_wmerge">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_wg1"><language>JavaScript</language><body>passes &lt; 2</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we7" source="_wdec" target="_wf">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_wg2"><language>JavaScript</language><body>passes &gt;= 2</body></guard>
        </edge>
      </ownedBehavior>
    </packagedElement>`

const forkFedCallApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_meter"/>`

// An action a control flow starts and an object flow feeds waits for both, as
// its pin did in v1, when the value's one producer runs on every pass; inside a
// loop the producer is not on, the flow keeps carrying its value only.
func TestActionsWaitForTheValuesFlowingIntoThem(t *testing.T) {
	r := migrateDocument(t, forkFedCall, forkFedCallApplications)
	for _, line := range []string{
		"first 'read gain' then 'join';",
		"first 'fork' then 'join';",
		"first 'join' then 'set last';",
		"flow 'read gain'.result to 'set last'.value;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "first 'read gain' then 'set last';")
	wantNote(t, r, "_mo1", migrate.Mapped, "the action waits for the value as well as for the control flow into it, as its pin did")
	wantNote(t, r, "_wo1", migrate.Approximated, "the flow carries its value only: the control flow into 'set last' starts the action, so the action does not wait for the value on each pass; the action lies on a loop that leaves 'read gain' out, so waiting would starve its later passes")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Meter")
	meta(t, s, "%action Meter::Sample #1")
	meta(t, s, "%continue")
	if out := meta(t, s, "%eval in #1 : last"); !strings.Contains(out, "= 21") {
		t.Errorf("the call did not wait for the value flowing into it: %s", out)
	}
}

// forkLoopedCall is a Meter whose Poll loops twice through a fork that reads gain
// and writes last from the value read, so each pass runs the producer again.
const forkLoopedCall = `
    <packagedElement xmi:type="uml:Class" xmi:id="_meter" name="Meter">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_gain" name="gain">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_gain0" value="21"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_last" name="last">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_last0" value="0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_passes" name="passes">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_passes0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_poll" name="Poll">
        <node xmi:type="uml:InitialNode" xmi:id="_pi"/>
        <node xmi:type="uml:MergeNode" xmi:id="_pmerge" name="merge"/>
        <node xmi:type="uml:ForkNode" xmi:id="_pfork" name="fork"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readGain" name="read gain" structuralFeature="_gain">
          <result xmi:type="uml:OutputPin" xmi:id="_rgOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setLast" name="set last" structuralFeature="_last" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_slVal" name="value"/>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_count" name="count">
          <language>JavaScript</language>
          <body>passes = passes + 1;</body>
        </node>
        <node xmi:type="uml:DecisionNode" xmi:id="_pdec" name="again"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_pf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe1" source="_pi" target="_pmerge"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe2" source="_pmerge" target="_pfork"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe3" source="_pfork" target="_readGain"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe4" source="_pfork" target="_setLast"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_po1" source="_rgOut" target="_slVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe5" source="_setLast" target="_count"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe6" source="_count" target="_pdec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe7" source="_pdec" target="_pmerge">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_pg1"><language>JavaScript</language><body>passes &lt; 2</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe8" source="_pdec" target="_pf">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_pg2"><language>JavaScript</language><body>passes &gt;= 2</body></guard>
        </edge>
      </ownedBehavior>
    </packagedElement>`

// Inside a loop whose every pass runs the producer again, as a fork on the loop
// starts it, the fed action waits for the value on each pass.
func TestActionsWaitForProducersEachPassOfALoopRuns(t *testing.T) {
	r := migrateDocument(t, forkLoopedCall, forkFedCallApplications)
	for _, line := range []string{
		"first 'read gain' then 'join';",
		"first 'fork' then 'join';",
		"first 'join' then 'set last';",
		"flow 'read gain'.result to 'set last'.value;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_po1", migrate.Mapped, "the action waits for the value as well as for the control flow into it, as its pin did")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Meter")
	meta(t, s, "%action Meter::Poll #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the loop did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : last"); !strings.Contains(out, "= 21") {
		t.Errorf("the call did not wait for the value flowing into it: %s", out)
	}
	if out := meta(t, s, "%eval in #1 : passes"); !strings.Contains(out, "= 2") {
		t.Errorf("the loop did not run twice: %s", out)
	}
}

// bufferFedCall is a Meter whose Drain writes last from a central buffer nothing
// fills, then reads gain, so the write's pin is fed by a flow carrying nothing.
const bufferFedCall = `
    <packagedElement xmi:type="uml:Class" xmi:id="_meter" name="Meter">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_gain" name="gain">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_gain0" value="21"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_last" name="last">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_last0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_drain" name="Drain">
        <node xmi:type="uml:InitialNode" xmi:id="_di"/>
        <node xmi:type="uml:CentralBufferNode" xmi:id="_dbuf" name="pending"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_setLast" name="set last" structuralFeature="_last" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_slVal" name="value"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readGain" name="read gain" structuralFeature="_gain">
          <result xmi:type="uml:OutputPin" xmi:id="_rgOut" name="result"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_df"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_de1" source="_di" target="_setLast"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_do1" source="_dbuf" target="_slVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_de2" source="_setLast" target="_readGain"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_de3" source="_readGain" target="_df"/>
      </ownedBehavior>
    </packagedElement>`

// An action whose required pin is fed only by an object flow that traces to no
// pin or parameter never fires, as it never did in v1: no succession reaches or
// leaves it, the flow is reported as carrying nothing, and the result validates.
func TestActionsFedByProducerlessFlowsNeverFire(t *testing.T) {
	r := migrateDocument(t, bufferFedCall, forkFedCallApplications)
	for _, line := range []string{
		"first 'set last' then 'read gain';",
		"first start then 'set last';",
		"flow 'pending' to 'set last'.value;",
		"flow to 'set last'.value;",
	} {
		wantNoLine(t, r.Notation, line)
	}
	wantNote(t, r, "_do1", migrate.Unmapped, "nothing the flow carries comes from a pin or parameter")
	wantNote(t, r, "_de1", migrate.Unmapped, "the edge leads to 'set last', which never fires: no value reaches its input pin 'value'")
	wantNote(t, r, "_de2", migrate.Unmapped, "the edge leaves 'set last', which never fires, so no token travels it")
	starved := false
	for _, e := range entriesFor(r, "_setLast") {
		starved = starved || e.Verdict == migrate.Approximated && strings.Contains(e.Note, "the action never fires: its input pin 'value' must hold a value, but the object flows into it trace to no pin or parameter that produces a value")
	}
	if !starved {
		t.Errorf("entries for _setLast = %+v, want one noting the action never fires", entriesFor(r, "_setLast"))
	}
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}
}
