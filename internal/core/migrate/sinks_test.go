package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// sinkNodes is a Probe whose Run forks into a write of hits and into a merge no
// edge leaves; a second fork has no edge at all.
const sinkNodes = `
    <packagedElement xmi:type="uml:Class" xmi:id="_probe" name="Probe">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_hits" name="hits">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_hits0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ForkNode" xmi:id="_fork" name="fork"/>
        <node xmi:type="uml:ForkNode" xmi:id="_loose" name="loose"/>
        <node xmi:type="uml:MergeNode" xmi:id="_end" name="end"/>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_one" name="one">
          <value xmi:type="uml:LiteralInteger" xmi:id="_oneV" value="1"/>
          <result xmi:type="uml:OutputPin" xmi:id="_oneOut" name="result"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set hits" structuralFeature="_hits" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_fork"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_fork" target="_one"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_fork" target="_end"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of" source="_oneOut" target="_setVal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_set" target="_final"/>
      </ownedBehavior>
    </packagedElement>`

const sinkNodesApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_probe"/>`

// A control node no edge leaves ends the token led to it, as done does; one no
// edge touches is skipped rather than started with the activity.
func TestControlNodesWithoutOutgoingEdgesEndTheirTokens(t *testing.T) {
	r := migrateDocument(t, sinkNodes, sinkNodesApplications)
	for _, line := range []string{
		"first start then 'fork';",
		"fork 'fork';",
		"first 'fork' then one;",
		"first 'fork' then done;",
		"flow one.result to 'set hits'.value;",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"merge end;", "fork loose;", "then loose;"} {
		wantNoLine(t, r.Notation, line)
	}
	wantNote(t, r, "_end", migrate.Approximated, "no edge leaves the node, so the token it takes ends there, as at done")
	wantNote(t, r, "_loose", migrate.Skipped, "not referenced by any behavior: no edge leads to or leaves the node")
	wantNote(t, r, "_e3", migrate.Mapped, "")
	if diags := errors(t, "t.sysml", r.Notation); len(diags) > 0 {
		t.Errorf("%v", diags)
	}

	s := session(t, r)
	meta(t, s, "%instantiate Probe")
	meta(t, s, "%action Probe::Run #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : hits"); !strings.Contains(out, "= 1") {
		t.Errorf("the write fed by the flow did not run: %s", out)
	}
}
