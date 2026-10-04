package migrate_test

import (
	"slices"
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
		"in ref context : Scanner[1];",
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
	if n := strings.Count(string(r.Notation), "in ref context : Scanner[1];"); n != 4 {
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
		"in ref context : Timer[1];",
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
		"in ref context : Host[1];",
		"if n > 0 then hit;",
		"action hit : Hit { in ref :>> context = Relay::context; }",
		"send new Ping() via context.tx;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "in ref context : Controller[1];")
	wantNoLine(t, r.Notation, "context.n")
	wantClean(t, "port-context-guard", r)
}

// localsModel is a block whose activity Compute declares a local named as the
// block's attribute, which the translator refuses since the declaration would
// shadow it, and whose activity Tally reads the attribute into a local.
const localsModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_counter" name="Counter">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_cnt" name="count">` + integerHref + `
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_cnt0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_compute" name="Compute">
        <node xmi:type="uml:InitialNode" xmi:id="_cpi"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_cpop" name="bump">
          <language>JavaScript</language>
          <body>let count = 1; count += 1;</body>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_cpf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_cpe1" source="_cpi" target="_cpop"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_cpe2" source="_cpop" target="_cpf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_tally" name="Tally">
        <node xmi:type="uml:InitialNode" xmi:id="_tli"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_tlop" name="bump">
          <language>JavaScript</language>
          <body>let n = count; count = n + 1;</body>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_tlf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_tle1" source="_tli" target="_tlop"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_tle2" source="_tlop" target="_tlf"/>
      </ownedBehavior>
    </packagedElement>`

// A name a body declares as a local is no read of the owner's feature called
// the same, so a behavior naming its owner's features only so stays a def
// without a context parameter; one reading a feature into a local reads the
// owner, and is written as its usage.
func TestLocalsABodyDeclaresAreNoReadsOfTheOwner(t *testing.T) {
	r := migrateDocument(t, localsModel, `<sysml:Block xmi:id="_b1" base_Class="_counter"/>`)
	for _, line := range []string{
		"action def Compute {",
		`rep language "JavaScript" /* let count = 1; count += 1; */`,
		"action tally {",
		"attribute n : ScalarValues::Integer;",
		"assign count := n + 1;",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"in ref context : Counter[1];", "action compute", "action def Tally"} {
		wantNoLine(t, r.Notation, line)
	}
}

// callerBlocks are two blocks: A, whose activity Inspect assigns A's attribute,
// and B, whose activity Run calls Inspect; B holds the part given, if any.
func callerBlocks(part string) (a, b string) {
	a = `
    <packagedElement xmi:type="uml:Class" xmi:id="_a" name="A">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_status" name="status">` + integerHref + `
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_status0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_inspect" name="Inspect">
        <node xmi:type="uml:InitialNode" xmi:id="_ii"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_mark" name="mark">
          <language>JavaScript</language>
          <body>status = status + 1</body>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_if"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ie1" source="_ii" target="_mark"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ie2" source="_mark" target="_if"/>
      </ownedBehavior>
    </packagedElement>`
	b = `
    <packagedElement xmi:type="uml:Class" xmi:id="_b" name="B">` + part + `
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_call" name="inspect" behavior="_inspect"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_call"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_call" target="_rf"/>
      </ownedBehavior>
    </packagedElement>`
	return a, b
}

const callerApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_a"/>
  <sysml:Block xmi:id="_b2" base_Class="_b"/>`

// sortedLines is the notation's lines, trimmed and sorted, for comparing two
// documents that declare the same members in different orders.
func sortedLines(notation []byte) string {
	lines := strings.Split(string(notation), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

// A def reading its owner takes the owner as its context whether it is reached
// first from its own block or from a caller in another; the caller does not
// take the callee's owner as its own context. When the caller's object holds a
// part that is one, the callee is the part's usage, performed on it.
func TestCalleeContextIsTheSameWhicheverBlockIsReachedFirst(t *testing.T) {
	a, b := callerBlocks("")
	first := migrateDocument(t, a+b, callerApplications)
	second := migrateDocument(t, b+a, callerApplications)
	if sortedLines(first.Notation) != sortedLines(second.Notation) {
		t.Errorf("the blocks migrate differently by order:\n%s\n----\n%s", first.Notation, second.Notation)
	}
	for _, line := range []string{
		"action def Inspect {",
		"in ref context : A[1];",
		"assign context.status := context.status + 1;",
		"action def Run {",
		"which is left unbound: the caller is a B, which is no A and has no part that is one",
	} {
		wantLine(t, first.Notation, line)
	}
	wantNoLine(t, first.Notation, "in ref context : B[1];")

	a, b = callerBlocks(`
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ba" name="a" type="_a" aggregation="composite"/>`)
	first = migrateDocument(t, a+b, callerApplications)
	second = migrateDocument(t, b+a, callerApplications)
	if sortedLines(first.Notation) != sortedLines(second.Notation) {
		t.Errorf("the blocks migrate differently by order:\n%s\n----\n%s", first.Notation, second.Notation)
	}
	for _, line := range []string{
		"part a : A;",
		"action inspect {",
		"assign status := status + 1;",
		"in ref context : B[1];",
		"perform action inspect ::> context.a.inspect;",
	} {
		wantLine(t, first.Notation, line)
	}
	wantNoLine(t, first.Notation, "in ref context : A[1];")
	wantClean(t, "caller-part", first)
}

// An activity of no classifier calling a def that reads its owner takes that
// owner as its own context: v1 runs the def on the caller's object, so the
// caller's is one of the owner, which it performs the callee on.
func TestCallerOfNoClassifierTakesTheCalleesOwnerAsContext(t *testing.T) {
	a, _ := callerBlocks("")
	r := migrateDocument(t, a+`
    <packagedElement xmi:type="uml:Activity" xmi:id="_bench" name="Bench">
      <node xmi:type="uml:InitialNode" xmi:id="_bi"/>
      <node xmi:type="uml:CallBehaviorAction" xmi:id="_callInspect" name="inspect" behavior="_inspect"/>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_bf"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_be1" source="_bi" target="_callInspect"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_be2" source="_callInspect" target="_bf"/>
    </packagedElement>`, `<sysml:Block xmi:id="_b1" base_Class="_a"/>`)
	for _, line := range []string{
		"action inspect {",
		"assign status := status + 1;",
		"action def Bench {",
		"in ref context : A[1];",
		"perform action inspect ::> context.inspect;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantClean(t, "caller-of-no-classifier", r)
}

// A part holding a collection of a block's objects, or a number of them its
// bounds do not tell, is no one object to bind a context to or perform a usage
// on: the caller binds nothing, and says so, rather than binding the collection.
func TestACollectionOfPartsIsNoOneObjectForAContext(t *testing.T) {
	a, b := callerBlocks(`
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ba" name="a" type="_a" aggregation="composite">
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="_bal" value="0"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="_bau" value="*"/>
      </ownedAttribute>`)
	r := migrateDocument(t, a+b, callerApplications)
	for _, line := range []string{
		"part a : A[0..*];",
		"action def Inspect {",
		"in ref context : A[1];",
		"action def Run {",
		"which is left unbound: the caller is a B, which is no A and holds them only as the collection a, no one object of which is chosen",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"in ref context : B[1];", "context.a", "::> a.inspect"} {
		wantNoLine(t, r.Notation, line)
	}
}

// cycleModel is a block whose activities A and B call each other: A reads the
// block's attribute, B reads nothing, and A is a state's do activity, which
// keeps it a def.
const cycleModel = `
    <packagedElement xmi:type="uml:Class" xmi:id="_timer" name="Timer">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_t" name="t">` + realHref + `
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_t0" value="0.0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_a" name="A">
        <node xmi:type="uml:InitialNode" xmi:id="_ai"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_tick" name="tick">
          <language>JavaScript</language>
          <body>t = t + 1.0;</body>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callB" name="b" behavior="_b"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_af"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae1" source="_ai" target="_tick"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae2" source="_tick" target="_callB"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_ae3" source="_callB" target="_af"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_b" name="B">
        <node xmi:type="uml:InitialNode" xmi:id="_bi"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callA" name="a" behavior="_a"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_bf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_be1" source="_bi" target="_callA"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_be2" source="_callA" target="_bf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_life" name="Life">
        <region xmi:type="uml:Region" xmi:id="_lr">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_linit"/>
          <subvertex xmi:type="uml:State" xmi:id="_ticking" name="ticking" doActivity="_a"/>
          <transition xmi:type="uml:Transition" xmi:id="_lt0" source="_linit" target="_ticking"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

// A member of a cycle of calls that reads nothing of the owner itself still
// binds the context a fellow member takes for its reads, so it takes the owner
// as its own context rather than binding the fellow's to its own occurrence.
func TestCycleMembersBindTheContextAFellowMemberReadsThrough(t *testing.T) {
	r := migrateDocument(t, cycleModel, `<sysml:Block xmi:id="_b1" base_Class="_timer"/>`)
	for _, line := range []string{
		"action def A {",
		"assign context.t := context.t + 1.0;",
		"action b : B { in ref :>> context = A::context; }",
		"action def B {",
		"action a : A { in ref :>> context = B::context; }",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNoLine(t, r.Notation, "bind a.context = this;")
	if n := strings.Count(string(r.Notation), "in ref context : Timer[1];"); n != 3 {
		t.Errorf("A, B and Life declare %d context parameters, want 3:\n%s", n, r.Notation)
	}
	wantNote(t, r, "_b", migrate.Mapped, "acts on its owner Timer, which it takes as its parameter context")
	wantNote(t, r, "_callA", migrate.Approximated, "which is bound to B::context")
	wantClean(t, "cycle", r)
}

// lanePortModel is the block Controller, whose activity Probe sets the
// attribute of its part sensor through a swimlane over it and sends a signal
// through the port of the unrelated block Host, whose activity Run calls Probe.
const lanePortModel = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_sensor" name="Sensor">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_v" name="v">` + integerHref + `
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_v0" value="0"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_controller" name="Controller">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_sens" name="sensor" type="_sensor" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_probe" name="Probe">
        <group xmi:type="uml:ActivityPartition" xmi:id="_lane" name="sensor" represents="_sens" node="_set"/>
        <node xmi:type="uml:InitialNode" xmi:id="_pi"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_set" name="set" inPartition="_lane">
          <language>JavaScript</language>
          <body>v = 1;</body>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_psend" name="send ping" signal="_ping" onPort="_tx"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_pf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe1" source="_pi" target="_set"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe2" source="_set" target="_psend"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe3" source="_psend" target="_pf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_host" name="Host">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callProbe" name="probe" behavior="_probe"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_callProbe"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callProbe" target="_rf"/>
      </ownedBehavior>
    </packagedElement>`

const lanePortApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_sensor"/>
  <sysml:Block xmi:id="_b2" base_Class="_controller"/>
  <sysml:Block xmi:id="_b3" base_Class="_host"/>`

// A def reading its owner through a swimlane over one of the owner's parts
// acts on the owner, as one reading the owner's features directly does, and so
// takes the owner as its context over the ports of another block it also goes
// through: the reads are qualified by the context, the send loses its port and
// says so, and a caller in the other block reports the context it cannot bind.
func TestOwnerReadsThroughASwimlaneOutweighAnotherBlocksPorts(t *testing.T) {
	r := migrateDocument(t, lanePortModel, lanePortApplications)
	for _, line := range []string{
		"action def Probe {",
		"in ref context : Controller[1];",
		"assign context.sensor.v := 1;",
		"send new Ping();",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"in ref context : Host[1];", "via context.tx", "context.probe"} {
		wantNoLine(t, r.Notation, line)
	}
	wantNote(t, r, "_probe", migrate.Mapped, "acts on its owner Controller, which it takes as its parameter context")
	wantNote(t, r, "_psend", migrate.Approximated, "the port Host::tx is no port of the object the sender acts on")
	wantNote(t, r, "_callProbe", migrate.Approximated, "which is left unbound: the caller is a Host, which is no Controller and has no part that is one")
	wantClean(t, "lane-port", r)
}

// discardedGuardModel is the block Controller, whose activity Probe decides on
// a JavaScript guard the translator refuses (it calls a function) before it
// sends through the Host's port, and whose activity Watch decides on a guard in
// a language the translator does not read, copied as the v2 syntax it already
// is; both guards name the Controller's attribute status.
const discardedGuardModel = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_controller" name="Controller">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_status" name="status">` + booleanHref + `
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_status0" value="false"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_probe" name="Probe">
        <node xmi:type="uml:InitialNode" xmi:id="_pi"/>
        <node xmi:type="uml:DecisionNode" xmi:id="_pdec" name="decide"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_psend" name="send ping" signal="_ping" onPort="_tx"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_pf" name="done"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe1" source="_pi" target="_pdec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe2" source="_pdec" target="_psend">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_pg"><language>JavaScript</language><body>status &amp;&amp; unknownCall()</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe3" source="_pdec" target="_pf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe4" source="_psend" target="_pf"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_watch" name="Watch">
        <node xmi:type="uml:InitialNode" xmi:id="_wi"/>
        <node xmi:type="uml:DecisionNode" xmi:id="_wdec" name="decide"/>
        <node xmi:type="uml:SendSignalAction" xmi:id="_wsend" name="send ping" signal="_ping" onPort="_tx"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_wf" name="done"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we1" source="_wi" target="_wdec"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we2" source="_wdec" target="_wsend">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_wg"><language>OCL</language><body>status</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we3" source="_wdec" target="_wf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_we4" source="_wsend" target="_wf"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_host" name="Host">
      <ownedAttribute xmi:type="uml:Port" xmi:id="_tx" name="tx" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_run" name="Run">
        <node xmi:type="uml:InitialNode" xmi:id="_ri"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callProbe" name="probe" behavior="_probe"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callWatch" name="watch" behavior="_watch"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_rf"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re1" source="_ri" target="_callProbe"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re2" source="_callProbe" target="_callWatch"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_re3" source="_callWatch" target="_rf"/>
      </ownedBehavior>
    </packagedElement>`

const discardedGuardApplications = `
  <sysml:Block xmi:id="_b1" base_Class="_controller"/>
  <sysml:Block xmi:id="_b2" base_Class="_host"/>`

// A guard the writer keeps as a comment reads nothing: the def sending through
// another block's port keeps that block as its context, its edge written
// unguarded. A guard in a language the translator does not read, copied as
// the v2 syntax it is, reads what it names: that def acts on its owner, and
// the guard spells the attribute through the context.
func TestADiscardedGuardReadsNothingOfTheOwner(t *testing.T) {
	r := migrateDocument(t, discardedGuardModel, discardedGuardApplications)
	for _, line := range []string{
		"action def Probe {",
		"in ref context : Host[1];",
		"send new Ping() via context.tx;",
		"action def Watch {",
		"in ref context : Controller[1];",
		"if context.status then 'send ping';",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"context.status && unknownCall()", "context.probe"} {
		wantNoLine(t, r.Notation, line)
	}
	wantNote(t, r, "_pe2", migrate.Approximated, "is kept as a comment and the edge written unguarded")
	wantNote(t, r, "_probe", migrate.Approximated, "acts on a Host through its ports, which it takes as its parameter context rather than its owner Controller")
	wantNote(t, r, "_watch", migrate.Mapped, "acts on its owner Controller, which it takes as its parameter context")
	wantNote(t, r, "_wsend", migrate.Approximated, "the port Host::tx is no port of the object the sender acts on")
	wantClean(t, "discarded-guard", r)
}
