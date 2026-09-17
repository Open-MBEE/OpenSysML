package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/repl"
)

// session submits migrated notation to a fresh REPL session, failing on any
// error diagnostic: the notation a migration writes must analyse before it runs.
func session(t *testing.T, r *migrate.Result) *repl.Session {
	t.Helper()
	s := repl.NewSession()
	for _, d := range s.Submit(string(r.Notation)).Diagnostics {
		if d.Severity == passes.SeverityError {
			t.Fatalf("migrated notation: %v\n%s", d, r.Notation)
		}
	}
	return s
}

// meta runs one REPL meta command and returns its output.
func meta(t *testing.T, s *repl.Session, line string) string {
	t.Helper()
	out, _, err := s.RunMeta(line)
	if err != nil {
		t.Fatalf("%s: %v", line, err)
	}
	return strings.Join(out, "\n")
}

// wantVerdict asserts a run held, printing its lines otherwise.
func wantVerdict(t *testing.T, v repl.Verdict) {
	t.Helper()
	if !v.Holds() {
		t.Fatalf("%s did not hold:\n%s", v.Subject, strings.Join(v.Lines, "\n"))
	}
}

// missionActivity is an activity in the shape of a workflow-duration analysis:
// an initial node, a bookkeeping opaque action in JavaScript, a fork into two
// called activities each bounded by a duration constraint (a point interval and
// a range), a join, a decision whose two branches carry «Probability», a merge
// and an activity final node. The called activities are owned by the same class.
const missionActivity = `
    <packagedElement xmi:type="uml:Class" xmi:id="_mission" name="Mission">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_tacq" name="Time_Acq_Total">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_acq" name="Acquire">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:OpaqueAction" xmi:id="_stamp" name="stamp">
          <language>JavaScript</language>
          <body>Time_Acq_Total = simtime;</body>
        </node>
        <node xmi:type="uml:ForkNode" xmi:id="_fork"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callA" name="Point" behavior="_point"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callB" name="Focus" behavior="_focus"/>
        <node xmi:type="uml:JoinNode" xmi:id="_join"/>
        <node xmi:type="uml:DecisionNode" xmi:id="_decide"/>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_callC" name="Retry" behavior="_focus"/>
        <node xmi:type="uml:MergeNode" xmi:id="_merge"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_stamp"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_stamp" target="_fork"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_fork" target="_callA"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_fork" target="_callB"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e5" source="_callA" target="_join"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e6" source="_callB" target="_join"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e7" source="_join" target="_decide"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e8" source="_decide" target="_callC">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_g8"><body>Focus lost</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e9" source="_decide" target="_merge">
          <guard xmi:type="uml:OpaqueExpression" xmi:id="_g9"><body>Focus held</body></guard>
        </edge>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e10" source="_callC" target="_merge"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e11" source="_merge" target="_final"/>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_dcA" name="pointing">
          <constrainedElement xmi:idref="_callA"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_diA" min="_dA1" max="_dA2"/>
        </ownedRule>
        <ownedRule xmi:type="uml:DurationConstraint" xmi:id="_dcB" name="focusing">
          <constrainedElement xmi:idref="_callB"/>
          <specification xmi:type="uml:DurationInterval" xmi:id="_diB" min="_dB1" max="_dB2"/>
        </ownedRule>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_point" name="Point Telescope">
        <node xmi:type="uml:InitialNode" xmi:id="_pinit"/>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_pfinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_pe" source="_pinit" target="_pfinal"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_focus" name="Focus Instrument">
        <node xmi:type="uml:InitialNode" xmi:id="_finit"/>
        <node xmi:type="uml:FlowFinalNode" xmi:id="_ffinal"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_fe" source="_finit" target="_ffinal"/>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_teA1">
      <when xmi:type="uml:TimeExpression" xmi:id="_teA1w">
        <expr xmi:type="uml:Duration" xmi:id="_dA1"><expr xmi:type="uml:LiteralString" xmi:id="_lA1" value="3s"/></expr>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_teA2">
      <when xmi:type="uml:TimeExpression" xmi:id="_teA2w">
        <expr xmi:type="uml:Duration" xmi:id="_dA2"><expr xmi:type="uml:LiteralString" xmi:id="_lA2" value="3s"/></expr>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_teB1">
      <when xmi:type="uml:TimeExpression" xmi:id="_teB1w">
        <expr xmi:type="uml:Duration" xmi:id="_dB1"><expr xmi:type="uml:LiteralString" xmi:id="_lB1" value="1s"/></expr>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_teB2">
      <when xmi:type="uml:TimeExpression" xmi:id="_teB2w">
        <expr xmi:type="uml:Duration" xmi:id="_dB2"><expr xmi:type="uml:LiteralString" xmi:id="_lB2" value="8s"/></expr>
      </when>
    </packagedElement>`

const missionApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_mission"/>
  <sysml:Probability xmi:id="_s3" base_ActivityEdge="_e8" probability="0.25"/>
  <sysml:Probability xmi:id="_s4" base_ActivityEdge="_e9" probability="0.75"/>`

// An activity becomes an action def owned by its block, its nodes the action's
// members: the initial node the succession from start, fork and join their v2
// nodes, a point duration interval a fixed wait, a range a uniform draw, the
// «Probability» branches weighted, the JavaScript body a comment, and the
// activity final a terminating action. The result runs under the model seed.
func TestActivityMigratesToAnExecutableActionDef(t *testing.T) {
	r := migrateDocument(t, missionActivity, missionApplications)
	for _, line := range []string{
		"part def Mission {",
		"action def Acquire {",
		"first start then stamp;",
		"fork 'fork';",
		"action wait accept after 3.0 [SI::s];",
		"first wait then Point;",
		"action Point : 'Point Telescope';",
		"action wait2 accept after RandomFunctions::uniform(1.0, 8.0) [SI::s];",
		"join 'join';",
		"decide 'decide';",
		"/* guard not migrated: [Focus lost] — not v2 expression syntax */",
		"first 'decide' then Retry { @Stochastic::Probability { p = 0.25; } }",
		"first 'decide' then 'merge' { @Stochastic::Probability { p = 0.75; } }",
		"merge 'merge';",
		"action final terminate;",
		"* Time_Acq_Total = simtime;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_stamp", migrate.Approximated, "the body is kept as a comment")
	wantNote(t, r, "_e8", migrate.Approximated, "the guard [Focus lost] is kept as a comment and the edge written unguarded")
	wantNote(t, r, "_dcA", migrate.Approximated, "written as a fixed wait of 3.0 s before 'Point'")
	wantNote(t, r, "_dcB", migrate.Approximated, "written as a wait drawn uniformly over [1.0, 8.0] s before 'Focus'")
	wantNote(t, r, "_ffinal", migrate.Mapped, "a flow final ends the token, as done does")
	wantNote(t, r, "_acq", migrate.Mapped, "")

	s := session(t, r)
	meta(t, s, "%seed 1")
	wantVerdict(t, s.RunAction("Mission::Acquire"))
	runs := meta(t, s, "%runs 20 1 Mission::Acquire")
	if !strings.Contains(runs, "20 run(s)") || strings.Contains(runs, "error") {
		t.Errorf("Monte Carlo runs of the migrated activity:\n%s", runs)
	}
}

// probabilityBranches is a decision with three outgoing edges; the
// applications decide how they are weighted.
const probabilityBranches = `
    <packagedElement xmi:type="uml:Activity" xmi:id="_act" name="Choose">
      <node xmi:type="uml:InitialNode" xmi:id="_init"/>
      <node xmi:type="uml:DecisionNode" xmi:id="_decide"/>
      <node xmi:type="uml:OpaqueAction" xmi:id="_a" name="a"/>
      <node xmi:type="uml:OpaqueAction" xmi:id="_b" name="b"/>
      <node xmi:type="uml:OpaqueAction" xmi:id="_c" name="c"/>
      <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_e0" source="_init" target="_decide"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ea" source="_decide" target="_a"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_eb" source="_decide" target="_b"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_ec" source="_decide" target="_c"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_fa" source="_a" target="_final"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_fb" source="_b" target="_final"/>
      <edge xmi:type="uml:ControlFlow" xmi:id="_fc" source="_c" target="_final"/>
    </packagedElement>`

// Probabilities are written only when they can be trusted: an unmarked branch
// takes the remainder to 1, a sum other than 1 over marked branches is scaled,
// and a value outside 0..1 drops every weight with the reason.
func TestProbabilitiesAreWrittenOnlyWhenTheyAreSound(t *testing.T) {
	t.Run("remainder", func(t *testing.T) {
		r := migrateDocument(t, probabilityBranches, `
  <sysml:Probability xmi:id="_p1" base_ActivityEdge="_ea" probability="0.5"/>
  <sysml:Probability xmi:id="_p2" base_ActivityEdge="_eb" probability="0.25"/>`)
		wantLine(t, r.Notation, "first 'decide' then a { @Stochastic::Probability { p = 0.5; } }")
		wantLine(t, r.Notation, "first 'decide' then b { @Stochastic::Probability { p = 0.25; } }")
		wantLine(t, r.Notation, "first 'decide' then c { @Stochastic::Probability { p = 0.25; } }")
		wantNote(t, r, "_ec", migrate.Approximated, "")
		s := session(t, r)
		meta(t, s, "%seed 1")
		wantVerdict(t, s.RunAction("Choose"))
	})
	t.Run("scaled", func(t *testing.T) {
		r := migrateDocument(t, probabilityBranches, `
  <sysml:Probability xmi:id="_p1" base_ActivityEdge="_ea" probability="0.2"/>
  <sysml:Probability xmi:id="_p2" base_ActivityEdge="_eb" probability="0.2"/>
  <sysml:Probability xmi:id="_p3" base_ActivityEdge="_ec" probability="0.4"/>`)
		wantLine(t, r.Notation, "first 'decide' then a { @Stochastic::Probability { p = 0.25; } }")
		wantLine(t, r.Notation, "first 'decide' then c { @Stochastic::Probability { p = 0.5; } }")
		wantNote(t, r, "_ea", migrate.Approximated, "sum to 0.8, not 1: each is scaled by the sum")
	})
	t.Run("out of range", func(t *testing.T) {
		r := migrateDocument(t, probabilityBranches, `
  <sysml:Probability xmi:id="_p1" base_ActivityEdge="_ea" probability="1.5"/>
  <sysml:Probability xmi:id="_p2" base_ActivityEdge="_eb" probability="0.5"/>
  <sysml:Probability xmi:id="_p3" base_ActivityEdge="_ec" probability="0.5"/>`)
		if strings.Contains(string(r.Notation), "p = 1.5") || strings.Contains(string(r.Notation), "p = 0.5") {
			t.Errorf("weights were written from probabilities outside 0..1:\n%s", r.Notation)
		}
		wantNote(t, r, "_ea", migrate.Approximated, "no «Probability» is written on the decision's branches: the probability 1.5 on")
		wantNote(t, r, "_decide", migrate.Approximated, "several branches leave the decision unconditionally, so one is drawn at random with the model seed")
	})
}

// controllerMachine is a block whose classifier behavior is a state machine:
// an initial pseudostate, a state with a do activity that sends a signal, a
// signal-triggered transition with an effect, a time-triggered transition, a
// composite state with a nested region, a submachine state with a connection
// point reference, and a final state.
const controllerMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_ack" name="Ack"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_tick" isRelative="true">
      <when xmi:type="uml:TimeExpression" xmi:id="_tickw">
        <expr xmi:type="uml:LiteralString" xmi:id="_tickl" value="2s"/>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_ctl" name="Controller" classifierBehavior="_sm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_count" name="count">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_count0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Control">
        <region xmi:type="uml:Region" xmi:id="_r0" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_busy" name="Busy">
            <region xmi:type="uml:Region" xmi:id="_r1" name="inner">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_init1"/>
              <subvertex xmi:type="uml:State" xmi:id="_warm" name="Warm"/>
              <subvertex xmi:type="uml:State" xmi:id="_hot" name="Hot"/>
              <transition xmi:type="uml:Transition" xmi:id="_t1i" source="_init1" target="_warm"/>
              <transition xmi:type="uml:Transition" xmi:id="_t1" source="_warm" target="_hot">
                <trigger xmi:type="uml:Trigger" xmi:id="_tr1" event="_tick"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_cool" name="Cool" submachine="_cooling">
            <connection xmi:type="uml:ConnectionPointReference" xmi:id="_cpr"/>
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="_fin"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_t2" name="start" source="_idle" target="_busy">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr2" event="_goEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_eff">
              <language>JavaScript</language>
              <body>count = count + 1;</body>
            </effect>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_t3" source="_busy" target="_cool">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr3" event="_tick"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_t4" source="_cool" target="_fin">
            <trigger xmi:type="uml:Trigger" xmi:id="_tr4" event="_goEv"/>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_cooling" name="Cooling">
        <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_cpEntry" name="in" kind="entryPoint"/>
        <region xmi:type="uml:Region" xmi:id="_r2" name="cool">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init2"/>
          <subvertex xmi:type="uml:State" xmi:id="_fan" name="Fan"/>
          <transition xmi:type="uml:Transition" xmi:id="_t2i" source="_init2" target="_fan"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const controllerApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_ctl"/>`

// A classifier-behavior state machine becomes a state def its block exhibits:
// the initial pseudostate the region's entry, a nested region the body of its
// state, a submachine state a usage typed by the submachine's state def, a
// signal event an accept of the signal, a relative time event an accept after,
// a JavaScript effect an assignment and the final state done. The result runs
// in the state debugger: the signal fires the transition, time moves through
// the composite and submachine states, and the machine completes.
func TestStateMachineMigratesToAnExecutableStateDef(t *testing.T) {
	r := migrateDocument(t, controllerMachine, controllerApplications)
	for _, line := range []string{
		"state def Control {",
		"entry; then Idle;",
		"state Busy {",
		"entry; then Warm;",
		"transition first Warm accept after 2.0 [SI::s] then Hot;",
		"state Cool : Cooling;",
		"transition start2 first Idle accept go : Go",
		"assign this.count := this.count + 1;",
		"then Busy;",
		"transition first Busy accept after 2.0 [SI::s] then Cool;",
		"transition first Cool accept Go then done;",
		"state def Cooling {",
		"exhibit state control : Control;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_sm", migrate.Approximated, "the classifier behavior is run by every object of Controller as its usage control")
	wantNote(t, r, "_cool", migrate.Mapped, "")
	wantNote(t, r, "_cpr", migrate.Unmapped, "a connection point reference has no v2 form")
	wantNote(t, r, "_cpEntry", migrate.Unmapped, "an entry point has no v2 form")
	wantNote(t, r, "_eff", migrate.Approximated, "the JavaScript body is written as v2 assignments")
	wantNote(t, r, "_tick", migrate.Mapped, "written where a trigger refers to it, as accept after 2.0 [SI::s]")

	s := session(t, r)
	meta(t, s, "%instantiate Controller")
	meta(t, s, "%state Controller::Control")
	if out := meta(t, s, "%send Go"); !strings.Contains(out, "transition start2 fires on it") {
		t.Errorf("%%send Go: %s", out)
	}
	if out := meta(t, s, "%advance 2"); !strings.Contains(out, "Advanced to 2.0") {
		t.Errorf("%%advance 2: %s", out)
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Fan") || !strings.Contains(out, "0. Cool") {
		t.Errorf("after 2 s the machine is not in Cool::Fan:\n%s", out)
	}
	if out := meta(t, s, "%send Go"); !strings.Contains(out, "transition Cool -> done fires on it") {
		t.Errorf("%%send Go: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: done") || !strings.Contains(out, "Execution state: Completed") {
		t.Errorf("the machine did not complete:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : count"); !strings.Contains(out, "= 1") {
		t.Errorf("the effect did not count the Go: %s", out)
	}
}

// stationActivity is a block with a part, whose activity reads the part, sends
// it a signal, waits for a reply, calls an operation with a value, and feeds a
// pin from a JavaScript action that migrates to nothing; the part's type owns
// the operation with an activity method, a reception, and a function behavior.
const stationActivity = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_ack" name="Ack"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ackEv" signal="_ack"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_tel" name="Telescope">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_az" name="azimuth">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        <defaultValue xmi:type="uml:LiteralReal" xmi:id="_az0" value="0.0"/>
      </ownedAttribute>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_point" name="Point" method="_pointing">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_pAz" name="az" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
      </ownedOperation>
      <ownedOperation xmi:type="uml:Operation" xmi:id="_park" name="Park"/>
      <ownedReception xmi:type="uml:Reception" xmi:id="_rcv" name="Go" signal="_go"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_pointing" name="Pointing" specification="_point">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_pAz2" name="az" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_apn" name="az" parameter="_pAz2"/>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="_set" name="set azimuth" structuralFeature="_az" isReplaceAll="true">
          <value xmi:type="uml:InputPin" xmi:id="_setVal" name="value"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of0" source="_apn" target="_setVal"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:FunctionBehavior" xmi:id="_twice" name="Twice">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tx" name="x" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_tr" name="result" direction="return">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <body>x * 2.0</body>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_station" name="Station">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_telPart" name="tel" type="_tel" aggregation="composite"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_observe" name="Observe">
        <node xmi:type="uml:InitialNode" xmi:id="_init"/>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readTel" name="read tel" structuralFeature="_telPart">
          <result xmi:type="uml:OutputPin" xmi:id="_readTelOut" name="result"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="_send" name="send Go" signal="_go">
          <target xmi:type="uml:InputPin" xmi:id="_sendTgt" name="target"/>
        </node>
        <node xmi:type="uml:AcceptEventAction" xmi:id="_accept" name="wait Ack">
          <trigger xmi:type="uml:Trigger" xmi:id="_trig" event="_ackEv"/>
        </node>
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="_ninety" name="ninety">
          <value xmi:type="uml:LiteralReal" xmi:id="_ninetyV" value="90.0"/>
          <result xmi:type="uml:OutputPin" xmi:id="_ninetyOut" name="result"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="_call" name="point" operation="_point">
          <target xmi:type="uml:InputPin" xmi:id="_callTgt" name="target"/>
          <argument xmi:type="uml:InputPin" xmi:id="_callAz" name="az"/>
        </node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="_readTel2" name="read tel again" structuralFeature="_telPart">
          <result xmi:type="uml:OutputPin" xmi:id="_readTel2Out" name="result"/>
        </node>
        <node xmi:type="uml:OpaqueAction" xmi:id="_js" name="compute">
          <language>JavaScript</language>
          <body>var t = java.lang.System.currentTimeMillis();</body>
          <outputValue xmi:type="uml:OutputPin" xmi:id="_jsOut" name="t"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="_log" name="log" behavior="_logging">
          <argument xmi:type="uml:InputPin" xmi:id="_logIn" name="t"/>
        </node>
        <node xmi:type="uml:ActivityFinalNode" xmi:id="_final"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e1" source="_init" target="_readTel"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of1" source="_readTelOut" target="_sendTgt"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e2" source="_send" target="_accept"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3" source="_accept" target="_ninety"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e3b" source="_accept" target="_readTel2"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of2" source="_ninetyOut" target="_callAz"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of3" source="_readTel2Out" target="_callTgt"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e4" source="_call" target="_js"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e5" source="_js" target="_log"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="_of4" source="_jsOut" target="_logIn"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="_e6" source="_log" target="_final"/>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="_logging" name="Logging">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="_logT" name="t" direction="in">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Real"/>
        </ownedParameter>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="_logApn" name="t" parameter="_logT"/>
      </ownedBehavior>
    </packagedElement>`

const stationApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_tel"/>
  <sysml:Block xmi:id="_s2" base_Class="_station"/>`

// The reading of a part feeds the send's target through a flow, the accept
// waits for the signal, an operation becomes an action def whose activity
// method is its body and an action usage of its owner, a call on the read part
// performs that usage on it, a value specification an out result, a function
// behavior a calc def, a reception a comment, and a flow from an unmigrated
// JavaScript action a comment naming the input that receives nothing. The
// result runs up to the accept, which a sent Ack releases; the call then
// points the station's telescope, not the station.
func TestActivityWithSendAcceptAndOperationCalls(t *testing.T) {
	r := migrateDocument(t, stationActivity, stationApplications)
	for _, line := range []string{
		"action def Point {",
		"in az : ScalarValues::Real;",
		"action 'set azimuth' {",
		"assign this.azimuth := value;",
		"bind 'set azimuth'.value = az;",
		"abstract action def Park;",
		"comment /* reception 'Go' accepts $::Go */",
		"calc def Twice {",
		"x * 2.0",
		"out result = this.tel;",
		"send new Go() to this.tel;",
		"action 'wait Ack' accept Ack;",
		"out result = 90.0;",
		"action point : Point;",
		"action park : Park;",
		"perform action point ::> tel.point;",
		"* var t = java.lang.System.currentTimeMillis();",
		"action log : Logging;",
		"flow 'read tel'.result to 'send Go'.target;",
		"flow ninety.result to point.az;",
		"/* flow compute.t to log.t not written: 'compute' is not migrated and produces no value */",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "bind log.t") || strings.Contains(string(r.Notation), "flow compute.t to log.t;") {
		t.Errorf("a flow from an action that produces no value was written:\n%s", r.Notation)
	}
	wantNote(t, r, "_pointing", migrate.Mapped, "written as the body of the operation Telescope::Point, whose method it is")
	wantNote(t, r, "_park", migrate.Mapped, "")
	wantNote(t, r, "_twice", migrate.Mapped, "")
	wantNote(t, r, "_tr", migrate.Approximated, "the return parameter is written as an out parameter")
	wantNote(t, r, "_rcv", migrate.Approximated, "a reception names the signal its owner accepts")
	wantNote(t, r, "_js", migrate.Approximated, "the body is kept as a comment")
	wantNote(t, r, "_log", migrate.Approximated, "its input log.t receives no value, since 'compute' is not migrated")
	wantNote(t, r, "_point", migrate.Mapped, "its owner's usage point performs it")
	wantNote(t, r, "_callTgt", migrate.Mapped, "the call performs the usage point of the target this.tel")
	wantNote(t, r, "_call", migrate.Approximated, "several edges lead to the node, which waits for all of them through the join 'join'")

	s := session(t, r)
	meta(t, s, "%instantiate Station")
	meta(t, s, "%action Station::Observe #1")
	if out := meta(t, s, "%continue"); !strings.Contains(out, "waiting since step 4 for a message of type Ack") {
		t.Errorf("the action did not wait at the accept:\n%s", out)
	}
	if out := meta(t, s, "%tokens"); !strings.Contains(out, "send Go.target = Instance(ID: 2)") {
		t.Errorf("the read part did not reach the send's target:\n%s", out)
	}
	if out := meta(t, s, "%send Ack"); !strings.Contains(out, "waiting at the accept of action wait Ack") {
		t.Errorf("%%send Ack: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%tokens"); !strings.Contains(out, "Token 1 @ fork") {
		t.Errorf("the accept did not release the token:\n%s", out)
	}
	if out := meta(t, s, "%continue"); !strings.Contains(out, "action Logging: input parameter t is bound by no argument") {
		t.Errorf("the run did not stop at the input the unmigrated action leaves unbound:\n%s", out)
	}
	if out := meta(t, s, "%features #1.tel"); !strings.Contains(out, "azimuth = 90.0") {
		t.Errorf("the call did not point the telescope:\n%s", out)
	}
}
