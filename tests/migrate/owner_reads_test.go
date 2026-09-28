package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// ownerReadsModel is a block whose activities read its features only where the
// writer resolves names — guards, and a swimlane over one of its parts — and
// whose state machine runs them as do activities, which keeps them defs, as
// does the sibling def Run that calls them.
const ownerReadsModel = ownerReadsParts + `
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callCheck" name="check" behavior="_check"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callAcquire" name="acquire" behavior="_acquire"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_callCheck"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callCheck" target="_callAcquire"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re3" source="_callAcquire" target="_rf"/>
      </ownedBehavior>` + ownerReadsBehaviors + `
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_life" name="Life">
        <region xmi:type="uml:Region" xmi:id="_lr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_linit"/>
          <subvertex xmi:type="uml:State" xmi:id="_checking" name="checking" doActivity="_check"/>
          <subvertex xmi:type="uml:State" xmi:id="_acquiring" name="acquiring" doActivity="_acquire"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt0" source="_linit" target="_checking"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt1" source="_checking" target="_acquiring"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

// ownerReadsParts opens the block Scanner, with its features and its part's block.
const ownerReadsParts = `
    <packagedElement xmi:type="uml:Class" xmi:id="_seq" name="Seq">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_seqI" name="i">` + integerHref + `
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_seqI0" value="0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_seqRetries" name="retries">` + integerHref + `
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_seqRetries0" value="3"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_scanner" name="Scanner">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_count" name="count">` + integerHref + `
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_count0" value="0"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_max" name="max">` + integerHref + `
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_max0" value="2"/>
      </ownedAttribute>
      <ownedAttribute xmi:type="uml:Property" xmi:id="_esw" name="esw" type="_seq" aggregation="composite"/>`

// ownerReadsBehaviors are the block's behaviors: Check, whose only owner reads
// are its guards, and Acquire, whose reads all resolve through its swimlane
// over the part esw.
const ownerReadsBehaviors = `
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_check" name="Check">
        <node xmi:type="uml:InitialNode" xmi:id="_ci"/>
        <node xmi:type="uml:DecisionNode" xmi:id="_cdec" name="decide"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_cf" name="done"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_cf2" name="more"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ce1" source="_ci" target="_cdec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ce2" source="_cdec" target="_cf">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_cg1"><language>JavaScript</language><body>count &gt;= max</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ce3" source="_cdec" target="_cf2">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_cg2"><language>JavaScript</language><body>count &lt; max</body></guard>
        </edge>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_acquire" name="Acquire">
        <group xmi:type="uml:ActivityPartition" xmi:id="_lane" name="esw" represents="_esw" node="_bump _adec"/>
        <node xmi:type="uml:InitialNode" xmi:id="_ai"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_bump" name="bump" inPartition="_lane">
          <language>JavaScript</language>
          <body>i += 1;</body>
        </node>
        <node xmi:type="uml:DecisionNode" xmi:id="_adec" name="decide" inPartition="_lane"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_af" name="done"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_af2" name="more"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae1" source="_ai" target="_bump"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae2" source="_bump" target="_adec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae3" source="_adec" target="_af">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_ag1"><language>JavaScript</language><body>i &gt;= retries</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae4" source="_adec" target="_af2">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_ag2"><language>JavaScript</language><body>i &lt; retries</body></guard>
        </edge>
      </ownedBehavior>`

const ownerReadsApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_seq"/>
  <sysml:Block xmi:id="_b2" base_Class="_scanner"/>`

// A def reads its owner wherever the writer resolves a name to one of the
// owner's features — in a guard as much as in an opaque action, and through a
// swimlane over one of the owner's parts as much as directly — so it takes the
// owner as its context parameter and qualifies every such read by it, whichever
// order the defs are reached in.
func TestDefReadingItsOwnerInGuardsAndThroughSwimlanesTakesItAsContext(t *testing.T) {
	r := migrateDocument(t, ownerReadsModel, ownerReadsApplications)
	for _, line := range []string{
		"action def Check {",
		"in ref context : Scanner;",
		"if context.count >= context.max then done2;",
		"if context.count < context.max then more;",
		"action def Acquire {",
		"assign context.esw.i := context.esw.i + 1;",
		"if context.esw.i >= context.esw.retries then done2;",
		"action check : Check { in ref :>> context = Run::context; }",
		"action acquire : Acquire { in ref :>> context = Run::context; }",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"if count >= max", "if i >= retries", "assign esw.i", "assign i :="} {
		wantNoLine(t, r.Notation, line)
	}
	if n := strings.Count(string(r.Notation), "in ref context : Scanner;"); n != 4 {
		t.Errorf("Run, Check, Acquire and Life declare %d context parameters, want 4:\n%s", n, r.Notation)
	}
	for _, id := range []string{"_check", "_acquire"} {
		wantNote(t, r, id, migrate.Approximated, "acts on its owner Scanner, which it takes as its parameter context")
	}
	s := session(t, r)
	meta(t, s, "%instantiate Scanner")
	meta(t, s, "%action Scanner::Run #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the run did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : esw.i"); !strings.Contains(out, "= 1") {
		t.Errorf("the part's counter was not bumped through the context: %s", out)
	}
}

// A block's behavior that reads the block's features only in its guards, or
// only through a swimlane over one of its parts, is written as a usage of the
// block like one whose actions read them: its body runs on the block's object,
// and reads them bare.
func TestBehaviorReadingItsOwnerOnlyInGuardsOrThroughSwimlanesIsItsUsage(t *testing.T) {
	r := migrateDocument(t, ownerReadsParts+ownerReadsBehaviors+`
    </packagedElement>`, ownerReadsApplications)
	for _, line := range []string{
		"action check {",
		"if count >= max then done2;",
		"action acquire {",
		"assign esw.i := esw.i + 1;",
		"if esw.i >= esw.retries then done2;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "context")
	for _, id := range []string{"_check", "_acquire"} {
		wantNote(t, r, id, migrate.Mapped, "written as an action usage of Scanner, which a call on an object performs")
	}
	s := session(t, r)
	meta(t, s, "%instantiate Scanner")
	meta(t, s, "%action Scanner::acquire #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the usage did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : esw.i"); !strings.Contains(out, "= 1") {
		t.Errorf("the part's counter was not bumped by the usage: %s", out)
	}
}

// nestedDefModel is a block whose activity assigns the block's attribute, so it
// is a usage of the block, and nests an activity of its own that reads the same
// attribute, which the usage calls.
const nestedDefModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_timer" name="Timer">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_t" name="t">` + realHref + `
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_t0" value="0.0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_outer" name="Outer">
        <ownedBehavior xmi:type="uml:Activity" xmi:id="_inner" name="Inner">
          <node xmi:type="uml:InitialNode" xmi:id="_ii"/>
          <node xmi:type="uml:OpaqueAction" xmi:id="_tick" name="tick">
            <language>JavaScript</language>
            <body>t = t + 1.0;</body>
          </node>
          <node xmi:type="uml:ActivityFinalNode" xmi:id="_if"/>
          <edge xmi:type="uml:ControlFlow" xmi:id="_ie1" source="_ii" target="_tick"/>
          <edge xmi:type="uml:ControlFlow" xmi:id="_ie2" source="_tick" target="_if"/>
        </ownedBehavior>
        <node xmi:type="uml:InitialNode" xmi:id="_oi"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_reset" name="reset">
          <language>JavaScript</language>
          <body>t = 1.0;</body>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callInner" name="inner" behavior="_inner"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_of"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe1" source="_oi" target="_reset"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe2" source="_reset" target="_callInner"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_oe3" source="_callInner" target="_of"/>
      </ownedBehavior>
    </packagedElement>`

const nestedDefApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_timer"/>`

// A behavior written as a usage of its block reads the block's features bare,
// since its body runs on the block's object; a def nested in that usage is its
// own occurrence, so it takes the block as its context parameter and reads
// through it, and the usage binds itself when it calls the def.
func TestDefNestedInAUsageTakesTheUsagesBlockAsContext(t *testing.T) {
	r := migrateDocument(t, nestedDefModel, nestedDefApplications)
	for _, line := range []string{
		"action outer {",
		"assign t := 1.0;",
		"action def Inner {",
		"in ref context : Timer;",
		"assign context.t := context.t + 1.0;",
		"action inner : Inner;",
		"bind inner.context = this;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "assign t := t + 1.0;")
	wantNoLine(t, r.Notation, "assign context.t := 1.0;")
	wantNote(t, r, "_inner", migrate.Mapped, "acts on its owner Timer, which it takes as its parameter context")
	s := session(t, r)
	meta(t, s, "%instantiate Timer")
	meta(t, s, "%action Timer::outer #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "Completed") {
		t.Errorf("the usage did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : t"); !strings.Contains(out, "= 2.0") {
		t.Errorf("the nested def did not tick the attribute through its context: %s", out)
	}
}

// portContextGuard is borrowedContext with Relay deciding on its own parameter
// before it calls through the Host's port.
const portContextGuard = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Activity" xmi:id="_hit" name="Hit">
      <node xmi:type="uml:InitialNode" xmi:id="_hi"/>
      <node xmi:type="uml:SendSignalAction" xmi:id="_hsend" name="send ping" signal="_ping" onPort="_tx"/>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_hf"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_he1" source="_hi" target="_hsend"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_he2" source="_hsend" target="_hf"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_controller" name="Controller">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_limit" name="limit">` + integerHref + `</ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_relay" name="Relay">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_n" name="n" direction="in">` + integerHref + `</ownedParameter>
        <node xmi:type="uml:InitialNode" xmi:id="_yi"/>
        <node xmi:type="uml:DecisionNode" xmi:id="_ydec" name="decide"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callHit" name="hit" behavior="_hit"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_yf" name="done"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ye1" source="_yi" target="_ydec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ye2" source="_ydec" target="_callHit">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_yg1"><language>JavaScript</language><body>n &gt; 0</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ye3" source="_ydec" target="_yf">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_yg2"><language>JavaScript</language><body>n &lt;= 0</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ye4" source="_callHit" target="_yf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_host" name="Host">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callRelay" name="relay" behavior="_relay"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_callRelay"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callRelay" target="_rf"/>
      </ownedBehavior>
    </packagedElement>`

const portContextGuardApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_controller"/>
  <sysml:Block xmi:id="_b2" base_Class="_host"/>`

// A def whose calls bind through another block's ports keeps that block as its
// context when its guards read only its own parameters: a name of the def's
// own is no read of its owner.
func TestGuardsOnTheDefsOwnParametersKeepItsPortContext(t *testing.T) {
	r := migrateDocument(t, portContextGuard, portContextGuardApplications)
	for _, line := range []string{
		"action def Relay {",
		"in ref context : Host;",
		"if n > 0 then hit;",
		"action hit : Hit { in ref :>> context = Relay::context; }",
		"send new Ping() via context.tx;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "in ref context : Controller;")
	wantNoLine(t, r.Notation, "context.n")
	wantClean(t, "port-context-guard", r)
}
