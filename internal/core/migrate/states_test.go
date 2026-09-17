package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// plantMachine is a block whose classifier behavior is a state machine with
// transitions crossing region boundaries in both directions, a junction with an
// else branch, a fork and join around a state of two regions, a shallow
// history, a submachine state entered and left through its entry and exit
// points, an internal transition on a plain state, a local transition, an
// absolute time event, and a signal event no trigger refers to.
const plantMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_go" name="Go"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_stop" name="Stop"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_hold" name="Hold"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_resume" name="Resume"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_next" name="Next"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_enter" name="Enter"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_finish" name="Finish"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_split" name="Split"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_route" name="Route"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_ping" name="Ping"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_bump" name="Bump"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_unused" name="Unused"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_goEv" signal="_go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_stopEv" signal="_stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_holdEv" signal="_hold"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_resumeEv" signal="_resume"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_nextEv" signal="_next"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_enterEv" signal="_enter"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_finishEv" signal="_finish"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_splitEv" signal="_split"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_routeEv" signal="_route"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pingEv" signal="_ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_bumpEv" signal="_bump"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_unusedEv" signal="_unused"/>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_dawn" name="dawn" isRelative="false">
      <when xmi:type="uml:TimeExpression" xmi:id="_dawnw">
        <expr xmi:type="uml:LiteralString" xmi:id="_dawnl" value="6h"/>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:TimeEvent" xmi:id="_never" name="never" isRelative="false">
      <when xmi:type="uml:TimeExpression" xmi:id="_neverw">
        <expr xmi:type="uml:LiteralString" xmi:id="_neverl" value="next Tuesday"/>
      </when>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_plant" name="Plant" classifierBehavior="_line">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_count" name="count">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_count0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_line" name="Line">
        <region xmi:type="uml:Region" xmi:id="_main" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_idle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_work" name="Work">
            <exit xmi:type="uml:OpaqueBehavior" xmi:id="_workExit" name="tally">
              <language>JavaScript</language>
              <body>count = count + 1;</body>
            </exit>
            <region xmi:type="uml:Region" xmi:id="_rw" name="steps">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_initW"/>
              <subvertex xmi:type="uml:State" xmi:id="_prep" name="Prep"/>
              <subvertex xmi:type="uml:State" xmi:id="_run" name="Run"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_hist" name="last" kind="shallowHistory"/>
              <transition xmi:type="uml:Transition" xmi:id="_tWi" source="_initW" target="_prep"/>
              <transition xmi:type="uml:Transition" xmi:id="_tNext" source="_prep" target="_run">
                <trigger xmi:type="uml:Trigger" xmi:id="_trNext" event="_nextEv"/>
              </transition>
              <transition xmi:type="uml:Transition" xmi:id="_tHist" source="_hist" target="_prep"/>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_pause" name="Pause"/>
          <subvertex xmi:type="uml:State" xmi:id="_cellSt" name="Cell" submachine="_cell">
            <connection xmi:type="uml:ConnectionPointReference" xmi:id="_cprIn" entry="_cpIn"/>
            <connection xmi:type="uml:ConnectionPointReference" xmi:id="_cprOut" exit="_cpOut"/>
          </subvertex>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_junc" name="route" kind="junction"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_fork" name="spread" kind="fork"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_join" name="gather" kind="join"/>
          <subvertex xmi:type="uml:State" xmi:id="_both" name="Both">
            <region xmi:type="uml:Region" xmi:id="_ra" name="a">
              <subvertex xmi:type="uml:State" xmi:id="_a1" name="A1"/>
              <subvertex xmi:type="uml:State" xmi:id="_a2" name="A2"/>
              <transition xmi:type="uml:Transition" xmi:id="_tA" source="_a1" target="_a2">
                <trigger xmi:type="uml:Trigger" xmi:id="_trA" event="_nextEv"/>
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="_rb" name="b">
              <subvertex xmi:type="uml:State" xmi:id="_b1" name="B1"/>
              <subvertex xmi:type="uml:State" xmi:id="_b2" name="B2"/>
              <transition xmi:type="uml:Transition" xmi:id="_tB" source="_b1" target="_b2">
                <trigger xmi:type="uml:Trigger" xmi:id="_trB" event="_nextEv"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="_fin"/>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_tGo" source="_idle" target="_work">
            <trigger xmi:type="uml:Trigger" xmi:id="_trGo" event="_goEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tStop" source="_run" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trStop" event="_stopEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tDirect" source="_idle" target="_run">
            <trigger xmi:type="uml:Trigger" xmi:id="_trDirect" event="_resumeEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tHold" source="_work" target="_pause">
            <trigger xmi:type="uml:Trigger" xmi:id="_trHold" event="_holdEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tResume" source="_pause" target="_hist">
            <trigger xmi:type="uml:Trigger" xmi:id="_trResume" event="_resumeEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tEnter" source="_idle" target="_cprIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_trEnter" event="_enterEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tLeave" source="_cprOut" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_tRoute" source="_idle" target="_junc">
            <trigger xmi:type="uml:Trigger" xmi:id="_trRoute" event="_routeEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tBusy" source="_junc" target="_work">
            <guard xmi:type="uml:Constraint" xmi:id="_gBusy">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gBusyX"><body>count &lt; 2</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tSpent" source="_junc" target="_fin">
            <guard xmi:type="uml:Constraint" xmi:id="_gSpent">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="_gSpentX"><body>else</body></specification>
            </guard>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tSplit" source="_idle" target="_fork">
            <trigger xmi:type="uml:Trigger" xmi:id="_trSplit" event="_splitEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tFa" source="_fork" target="_a1"/>
          <transition xmi:type="uml:Transition" xmi:id="_tFb" source="_fork" target="_b1"/>
          <transition xmi:type="uml:Transition" xmi:id="_tJa" source="_a2" target="_join"/>
          <transition xmi:type="uml:Transition" xmi:id="_tJb" source="_b2" target="_join"/>
          <transition xmi:type="uml:Transition" xmi:id="_tJoined" source="_join" target="_idle"/>
          <transition xmi:type="uml:Transition" xmi:id="_tPing" kind="internal" source="_idle" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trPing" event="_pingEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tBump" kind="local" source="_work" target="_prep">
            <trigger xmi:type="uml:Trigger" xmi:id="_trBump" event="_bumpEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tDawn" source="_pause" target="_idle">
            <trigger xmi:type="uml:Trigger" xmi:id="_trDawn" event="_dawn"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tNever" source="_pause" target="_fin">
            <trigger xmi:type="uml:Trigger" xmi:id="_trNever" event="_never"/>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_cell" name="CellMachine">
        <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_cpIn" name="warmStart" kind="entryPoint"/>
        <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_cpOut" name="spent" kind="exitPoint"/>
        <region xmi:type="uml:Region" xmi:id="_rc" name="cell">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_initC"/>
          <subvertex xmi:type="uml:State" xmi:id="_cold" name="Cold"/>
          <subvertex xmi:type="uml:State" xmi:id="_hot" name="Hot"/>
          <transition xmi:type="uml:Transition" xmi:id="_tCi" source="_initC" target="_cold"/>
          <transition xmi:type="uml:Transition" xmi:id="_tWarm" source="_cold" target="_hot">
            <trigger xmi:type="uml:Trigger" xmi:id="_trWarm" event="_nextEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_tIn" source="_cpIn" target="_hot"/>
          <transition xmi:type="uml:Transition" xmi:id="_tOut" source="_hot" target="_cpOut">
            <trigger xmi:type="uml:Trigger" xmi:id="_trOut" event="_finishEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const plantApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_plant"/>`

// A transition whose ends lie in different regions names the nested end by its
// path; a junction and its else guard become a junction pseudostate with an
// unguarded branch; fork and join pseudostates keep their kind; a shallow
// history is a history member; an entry or exit point is a state of the
// submachine's state def that a connection point reference names through the
// submachine state; an internal transition on a plain state is a self
// transition; a local transition is written external and noted; a literal
// absolute time is a TimeInstantValue attribute the transition accepts at, one
// that is not is reported; an event no trigger refers to is skipped.
func TestStateMachineCrossRegionTransitionsAndPseudostates(t *testing.T) {
	r := migrateDocument(t, plantMachine, plantApplications)
	for _, line := range []string{
		"state def Line {",
		"attribute dawn : Time::TimeInstantValue = 21600.0 [SI::s];",
		"entry; then Idle;",
		"state Work {",
		"entry; then Prep;",
		"history last;",
		"transition first last then Prep;",
		"state Cell : CellMachine;",
		"junction route;",
		"fork spread;",
		"join gather;",
		"state Both {",
		"state regions parallel {",
		"transition first Work::Run accept Stop then Idle;",
		"transition first Idle accept Resume then Work::Run;",
		"transition first Pause accept Resume then Work::last;",
		"transition first Idle accept Enter then Cell::warmStart;",
		"transition first Cell::spent then Idle;",
		"transition first route if this.count < 2 then Work;",
		"transition first route then done;",
		"transition first spread then Both::regions::a::A1;",
		"transition first spread then Both::regions::b::B1;",
		"transition first Both::regions::a::A2 then gather;",
		"transition first Both::regions::b::B2 then gather;",
		"transition first gather then Idle;",
		"transition first Idle accept Ping then Idle;",
		"transition first Work accept Bump then Work::Prep;",
		"transition first Pause accept at dawn then Idle;",
		"state def CellMachine {",
		"state warmStart;",
		"state spent;",
		"transition first warmStart then Hot;",
		"transition first Hot accept Finish then spent;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "accept at never") || strings.Contains(string(r.Notation), "accept Unused") {
		t.Errorf("an unresolvable instant or an unreferenced event was written:\n%s", r.Notation)
	}
	wantNote(t, r, "_tStop", migrate.Mapped, "the source 'Run' lies in another region and is named by its path Work::Run")
	wantNote(t, r, "_tDirect", migrate.Mapped, "the target 'Run' lies in another region and is named by its path Work::Run")
	wantNote(t, r, "_junc", migrate.Mapped, "written as a junction pseudostate")
	wantNote(t, r, "_gSpent", migrate.Mapped, "an else guard is written as the unguarded transition out of the junction")
	wantNote(t, r, "_fork", migrate.Mapped, "written as a fork pseudostate")
	wantNote(t, r, "_join", migrate.Mapped, "written as a join pseudostate")
	wantNote(t, r, "_hist", migrate.Mapped, "written as a shallow history")
	wantNote(t, r, "_cpIn", migrate.Mapped, "a transition entering a submachine state through the entry point enters this state")
	wantNote(t, r, "_cpOut", migrate.Mapped, "a transition leaving a submachine state through the exit point leaves this state")
	wantNote(t, r, "_cprIn", migrate.Mapped, "written as the entry point's state in the submachine state, Cell::warmStart")
	wantNote(t, r, "_cprOut", migrate.Mapped, "written as the exit point's state in the submachine state, Cell::spent")
	wantNote(t, r, "_tPing", migrate.Mapped, "Idle has no entry, exit or do behavior and no substates, so re-entering it is not observable")
	wantNote(t, r, "_tBump", migrate.Approximated, "a local transition is written external: the composite state Work exits and re-enters")
	wantNote(t, r, "_tDawn", migrate.Mapped, "")
	wantNote(t, r, "_dawn", migrate.Approximated, "written where a trigger refers to it, as accept at dawn")
	wantNote(t, r, "_tNever", migrate.Unmapped, "the time event's time is not written")
	wantNote(t, r, "_never", migrate.Unmapped, "the time event's time is not written")
	wantNote(t, r, "_unusedEv", migrate.Skipped, "not referenced by any behavior")
	if c := r.Report.Count(); c[migrate.Skipped] < 1 {
		t.Errorf("the skipped count leaves out the unreferenced event: %v", c)
	}

	// Idle leaves into the nested Run and comes back from it.
	s := session(t, r)
	meta(t, s, "%instantiate Plant")
	meta(t, s, "%state Plant::Line #1")
	if out := meta(t, s, "%send Resume"); !strings.Contains(out, "transition Idle -> Run fires on it") {
		t.Errorf("%%send Resume: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Run") {
		t.Errorf("the cross-region transition did not enter Work::Run:\n%s", out)
	}
	if out := meta(t, s, "%send Stop"); !strings.Contains(out, "transition Run -> Idle fires on it") {
		t.Errorf("%%send Stop: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("the cross-region transition did not leave Work for Idle:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : count"); !strings.Contains(out, "= 1") {
		t.Errorf("leaving Work from its nested state did not run its exit action: %s", out)
	}

	// The junction routes to Work while the count is low.
	meta(t, s, "%send Route")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Prep") {
		t.Errorf("the junction did not route to Work while the count is low:\n%s", out)
	}
	meta(t, s, "%send Next")
	meta(t, s, "%step")
	meta(t, s, "%send Stop")
	meta(t, s, "%step")

	// Ping stays in Idle; the shallow history returns to Run after a pause.
	meta(t, s, "%send Ping")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("the internal transition left Idle:\n%s", out)
	}
	meta(t, s, "%send Resume")
	meta(t, s, "%step")
	meta(t, s, "%send Hold")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Pause") {
		t.Errorf("Hold did not pause the work:\n%s", out)
	}
	meta(t, s, "%send Resume")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Run") {
		t.Errorf("the history did not resume the nested Run:\n%s", out)
	}

	// The entry point enters the submachine at Hot; the exit point leaves it.
	meta(t, s, "%send Stop")
	meta(t, s, "%step")
	if out := meta(t, s, "%send Enter"); !strings.Contains(out, "transition Idle -> warmStart fires on it") {
		t.Errorf("%%send Enter: %s", out)
	}
	meta(t, s, "%step")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Hot") {
		t.Errorf("the entry point did not enter the submachine at Hot:\n%s", out)
	}
	meta(t, s, "%send Finish")
	meta(t, s, "%step")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("the exit point did not leave the submachine for Idle:\n%s", out)
	}

	// The fork enters both regions, the join waits for both, and the
	// junction's else branch ends the machine once the count is spent.
	meta(t, s, "%send Split")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "A1") || !strings.Contains(out, "B1") {
		t.Errorf("the fork did not enter both regions:\n%s", out)
	}
	meta(t, s, "%send Next")
	meta(t, s, "%step")
	meta(t, s, "%send Next")
	meta(t, s, "%step")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("the join did not bring the machine back to Idle:\n%s", out)
	}
	meta(t, s, "%send Route")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: done") {
		t.Errorf("the junction's else branch did not end the machine:\n%s", out)
	}
}
