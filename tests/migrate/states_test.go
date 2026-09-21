package migrate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/plant_states.xmi: cross-region transitions, junction, fork/join, histories, entry/exit
// points, internal and local transitions and absolute time events are written; an orphan event is skipped.
func TestStateMachineCrossRegionTransitionsAndPseudostates(t *testing.T) {
	r := migrateFixtureFile(t, "plant_states")
	for _, line := range []string{
		"state def Line {",
		"attribute dawn : Time::TimeInstantValue = 21600.0 [SI::s];",
		"entry; then Idle;",
		"state Work {",
		"entry; then Prep;",
		"history last;",
		"deep history deepest;",
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
	wantNote(t, r, "_deep", migrate.Mapped, "written as a deep history, which re-enters the innermost states active")
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
	c := r.Report.Count()
	if c[migrate.Skipped] < 1 {
		t.Errorf("the skipped count leaves out the unreferenced event: %v", c)
	}
	if n := r.Report.Unreferenced(); n != 1 {
		t.Errorf("unreferenced = %d, want the one event no trigger refers to", n)
	}
	summary := r.Report.Summary()
	want := fmt.Sprintf("migrated %d element(s): %d mapped, %d approximated, %d unmapped (%d skipped as profile, library or notation-only content, 1 as model elements nothing refers to)",
		len(r.Report.Entries)-c[migrate.Skipped], c[migrate.Mapped], c[migrate.Approximated], c[migrate.Unmapped], c[migrate.Skipped]-1)
	if summary != want {
		t.Errorf("summary\n got %s\nwant %s", summary, want)
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

// testdata/xmi/station_points.xmi: entry and exit points owned by composite states — on a
// nested state, on a state with orthogonal regions, beside a default initial pseudostate
// and a shallow history — are written as junctions, a fork and a join of their state; an
// entry point leading straight to an exit point of its state is refused. Every entry, exit
// and effect behavior appends a two-digit code to trace, so a run pins the UML order:
// 11 Work entry, 12 Work exit, 13 Prep entry, 14 Run entry, 15 Run exit, 17 Fast entry,
// 21 Deep→Fast, 22 Fast→Out, 23 Start→Run, 24 Run→Leave, 25 Out→Prep, 31 Sync entry,
// 32 Sync exit, 33/34 A1 entry/exit, 35/36 B1 entry/exit, 37 A1→Gather, 38 B1→Gather,
// 41 Idle→Start, 42 Idle→Deep, 43 Leave→Idle, 44 Work→Idle, 46 Gather→Idle.
func TestCompositeStateConnectionPointsKeepTheUMLOrder(t *testing.T) {
	r := migrateFixtureFile(t, "station_points")
	for _, line := range []string{
		"state Work {",
		"junction Start;",
		"junction Leave;",
		"junction Deep;",
		"junction Out;",
		"history H;",
		"transition first Work::Run::Deep",
		"transition first Fast accept Back",
		"then Work::Run::Out;",
		"transition first Work::Start",
		"transition first Run accept Finish",
		"then Work::Leave;",
		"transition first Work::Run::Out",
		"fork Both;",
		"join Gather;",
		"transition first Sync::Both then A1;",
		"transition first Sync::Both then B1;",
		"then Sync::Gather;",
		"then Work::Start;",
		"transition first Idle accept Enter then Work;",
		"then Work::Run::Deep;",
		"transition first Idle accept Resume then Work::H;",
		"transition first Work::Leave",
		"transition first Idle accept Split then Sync::Both;",
		"transition first Sync::Gather",
		"/* not migrated: Pseudostate 'Through' — (_tThrough) leads from the entry point straight to the exit point 'Leave' of the same state, crossing it without settling in it; the runtime would then run neither its entry nor its exit behavior */",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "state Start") || strings.Contains(string(r.Notation), "then Through") {
		t.Errorf("a connection point was written as a state, or a refused one was named:\n%s", r.Notation)
	}
	wantNote(t, r, "_start", migrate.Mapped, "written as a junction of its state; a transition entering through it runs the state's entry behavior, then the transition leaving the junction")
	wantNote(t, r, "_leave", migrate.Mapped, "written as a junction of its state; a transition leaving through it runs the transition into the junction, the state's exit behavior, then the transition leaving it")
	wantNote(t, r, "_deep", migrate.Mapped, "written as a junction of its state")
	wantNote(t, r, "_out", migrate.Mapped, "written as a junction of its state")
	wantNote(t, r, "_plain", migrate.Mapped, "no transition leaves the entry point, so entering through it enters 'Work' by its default entry; a transition to it is written to the state")
	wantNote(t, r, "_tEnter", migrate.Mapped, "written to Work: no transition leaves the entry point 'Plain'")
	wantNote(t, r, "_both", migrate.Mapped, "written as a fork of its state, whose branches start its regions; a transition entering through it runs the state's entry behavior, then the branches")
	wantNote(t, r, "_gather", migrate.Mapped, "written as a join of its state, which its regions leave through together; the transitions into the join run, then the state's exit behavior, then the transition leaving it")
	wantNote(t, r, "_tGo", migrate.Mapped, "named by its path Work::Start")
	wantNote(t, r, "_tDive", migrate.Mapped, "named by its path Work::Run::Deep")
	wantNote(t, r, "_tBothA", migrate.Mapped, "named by its path Sync::Both")
	wantNote(t, r, "_tAg", migrate.Mapped, "named by its path Sync::Gather")
	wantNote(t, r, "_hist", migrate.Mapped, "written as a shallow history")
	wantNote(t, r, "_through", migrate.Unmapped, "leads from the entry point straight to the exit point 'Leave' of the same state, crossing it without settling in it; the runtime would then run neither its entry nor its exit behavior")
	wantNote(t, r, "_tThrough", migrate.Unmapped, "the source 'Through' has no v2 form")
	wantNote(t, r, "_tSkip", migrate.Unmapped, "the target 'Through' has no v2 form")

	s := session(t, r)
	trace := func(object, want string) {
		t.Helper()
		if out := meta(t, s, "%eval in "+object+" : trace"); strings.TrimSpace(out[strings.LastIndex(out, "=")+1:]) != want {
			t.Errorf("trace of %s: want %s, got\n%s", object, want, out)
		}
	}
	current := func(want string) {
		t.Helper()
		if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: "+want) {
			t.Errorf("want the current state %s:\n%s", want, out)
		}
	}
	station := func() string {
		t.Helper()
		out := meta(t, s, "%instantiate Station")
		_, id, ok := strings.Cut(out, "ID: ")
		if !ok {
			t.Fatalf("%%instantiate Station: %s", out)
		}
		id, _, _ = strings.Cut(id, "\n")
		object := "#" + strings.TrimSpace(id)
		meta(t, s, "%state Station::Cycle "+object)
		return object
	}
	drive := func(signal, fires string) {
		t.Helper()
		if out := meta(t, s, "%send "+signal); !strings.Contains(out, "transition "+fires+" fires on it") {
			t.Errorf("%%send %s: %s", signal, out)
		}
		meta(t, s, "%step")
	}

	// Through the entry point Start: the effect into it, Work's entry, the
	// transition out of it, then Run and its default Slow. Out through Leave
	// from the nested Run: Run's exit, the transition into it, Work's exit,
	// then the transition out of it.
	one := station()
	drive("Go", "Idle -> Start")
	current("Slow")
	trace(one, "41112314")
	drive("Finish", "Run -> Leave")
	current("Idle")
	trace(one, "4111231415241243")
	meta(t, s, "%stop")

	// Into the nested Run through its own entry point Deep: Work's entry, Run's
	// entry, the transition out of Deep, then Fast rather than the default Slow.
	// Out of Run through its exit point Out: the transition into it, Run's exit,
	// then the transition out of it into Prep, still inside Work.
	two := station()
	drive("Dive", "Idle -> Deep")
	current("Fast")
	trace(two, "4211142117")
	drive("Back", "Fast -> Out")
	current("Prep")
	trace(two, "421114211722152513")
	meta(t, s, "%stop")

	// An entry point no transition leaves enters Work by its default Prep. After
	// Work is left from Run, the shallow history beside the entry points brings
	// Run back, with its default Slow.
	three := station()
	drive("Enter", "Idle -> Work")
	current("Prep")
	trace(three, "1113")
	drive("Next", "Prep -> Run")
	drive("Stop", "Work -> Idle")
	trace(three, "111314151244")
	drive("Resume", "Idle -> H")
	current("Slow")
	trace(three, "1113141512441114")
	meta(t, s, "%stop")

	// The entry point Both of the orthogonal Sync starts both regions at once
	// after Sync's entry; each region then leaves through the exit point Gather,
	// which joins them: both exits and effects, Sync's exit, then the transition out.
	four := station()
	drive("Split", "Idle -> Both")
	current("A1 | B1")
	trace(four, "313335")
	meta(t, s, "%step")
	current("Idle")
	trace(four, "313335343736383246")
	meta(t, s, "%stop")

	// Entered plainly, Sync's regions start at A0 and B0 and reach Gather the same way.
	five := station()
	drive("Pair", "Idle -> Sync")
	current("A0 | B0")
	drive("Bump", "A0 -> A1 and transition B0 -> B1")
	meta(t, s, "%step")
	current("Idle")
	trace(five, "313335343736383246")
}

// gateMachine has an empty region beside the one holding its states, and transitions
// between Idle and the nested Busy::Inner across nesting levels.
const gateMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_open" name="Open"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_shut" name="Shut"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_openEv" signal="_open"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_shutEv" signal="_shut"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_gate" name="Gate" classifierBehavior="_gsm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_gsm" name="Latch">
        <region xmi:type="uml:Region" xmi:id="_gUnused" name="Unused"/>
        <region xmi:type="uml:Region" xmi:id="_gMain" name="Main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_ginit"/>
          <subvertex xmi:type="uml:State" xmi:id="_gidle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_gbusy" name="Busy">
            <region xmi:type="uml:Region" xmi:id="_gbr">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_gbinit"/>
              <subvertex xmi:type="uml:State" xmi:id="_ginner" name="Inner"/>
              <transition xmi:type="uml:Transition" xmi:id="_gbt0" source="_gbinit" target="_ginner"/>
              <transition xmi:type="uml:Transition" xmi:id="_gtOut" source="_ginner" target="_gidle">
                <trigger xmi:type="uml:Trigger" xmi:id="_gtrOut" event="_shutEv"/>
              </transition>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_gt0" source="_ginit" target="_gidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_gtIn" source="_gidle" target="_ginner">
            <trigger xmi:type="uml:Trigger" xmi:id="_gtrIn" event="_openEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const gateApplications = `
  <sysml:Block xmi:id="_g1" base_Class="_gate"/>`

// An empty region is skipped when the machine's vertices are named as when they are
// written, so the remaining region's states are inline and a transition across nesting
// levels names its far end by a path that exists.
func TestEmptyRegionLeavesNoPhantomPath(t *testing.T) {
	r := migrateDocument(t, gateMachine, gateApplications)
	for _, line := range []string{
		"state def Latch {",
		"entry; then Idle;",
		"state Busy {",
		"entry; then Inner;",
		"transition first Idle accept Open then Busy::Inner;",
		"transition first Inner accept Shut then Idle;",
	} {
		wantLine(t, r.Notation, line)
	}
	if strings.Contains(string(r.Notation), "regions") {
		t.Errorf("a parallel state was named for a machine with one populated region:\n%s", r.Notation)
	}
	wantNote(t, r, "_gUnused", migrate.Skipped, "the region holds no vertex, so nothing enters it and no state is written for it")
	wantNote(t, r, "_gtIn", migrate.Mapped, "the target 'Inner' lies in another region and is named by its path Busy::Inner")

	s := session(t, r)
	meta(t, s, "%instantiate Gate")
	meta(t, s, "%state Gate::Latch")
	if out := meta(t, s, "%send Open"); !strings.Contains(out, "transition Idle -> Inner fires on it") {
		t.Errorf("%%send Open: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Inner") {
		t.Errorf("the transition did not enter Busy::Inner:\n%s", out)
	}
	if out := meta(t, s, "%send Shut"); !strings.Contains(out, "transition Inner -> Idle fires on it") {
		t.Errorf("%%send Shut: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("the transition did not leave Busy for Idle:\n%s", out)
	}
}

// clashMachine owns attributes named like its fork, join, shallow and deep history
// pseudostates, and an attribute named like the base name an anonymous fork takes.
const clashMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_cgo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_cgoEv" signal="_cgo"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_press" name="Press" classifierBehavior="_csm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_csm" name="Cycle">
        <ownedAttribute xmi:type="uml:Property" xmi:id="_aSpread" name="spread">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_aGather" name="gather">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_aCheck" name="checkpoint">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_aDeep" name="deepest">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
        <ownedAttribute xmi:type="uml:Property" xmi:id="_aFork" name="fork">
          <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
        <region xmi:type="uml:Region" xmi:id="_cMain" name="Main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_cinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_cidle" name="Idle"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_cfork" name="spread" kind="fork"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_cjoin" name="gather" kind="join"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_cfork2" kind="fork"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_chist" name="checkpoint" kind="shallowHistory"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_cdeep" name="deepest" kind="deepHistory"/>
          <subvertex xmi:type="uml:State" xmi:id="_cboth" name="Both">
            <region xmi:type="uml:Region" xmi:id="_cra" name="a">
              <subvertex xmi:type="uml:State" xmi:id="_ca1" name="A1"/>
            </region>
            <region xmi:type="uml:Region" xmi:id="_crb" name="b">
              <subvertex xmi:type="uml:State" xmi:id="_cb1" name="B1"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_ct0" source="_cinit" target="_cidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctGo" source="_cidle" target="_cfork">
            <trigger xmi:type="uml:Trigger" xmi:id="_ctrGo" event="_cgoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_ctA" source="_cfork" target="_ca1"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctB" source="_cfork" target="_cb1"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctJa" source="_ca1" target="_cjoin"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctJb" source="_cb1" target="_cjoin"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctBack" source="_cjoin" target="_chist"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctHist" source="_chist" target="_cfork2"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctHa" source="_cfork2" target="_ca1"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctHb" source="_cfork2" target="_cb1"/>
          <transition xmi:type="uml:Transition" xmi:id="_ctDeep" source="_cdeep" target="_cidle"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const clashApplications = `
  <sysml:Block xmi:id="_c1" base_Class="_press"/>`

// A fork, join or history pseudostate named like another member of the body it
// is written in is renamed as a state would be, so the state def has distinct members.
func TestPseudostatesNamedLikeMembersAreDistinguished(t *testing.T) {
	r := migrateDocument(t, clashMachine, clashApplications)
	for _, line := range []string{
		"attribute spread : ScalarValues::Integer;",
		"attribute gather : ScalarValues::Integer;",
		"attribute checkpoint : ScalarValues::Integer;",
		"attribute deepest : ScalarValues::Integer;",
		"attribute 'fork' : ScalarValues::Integer;",
		"fork 'spread 2';",
		"join 'gather 2';",
		"fork fork2;",
		"history 'checkpoint 2';",
		"deep history 'deepest 2';",
		"transition first Idle accept Go then 'spread 2';",
		"transition first 'spread 2' then Both::regions::a::A1;",
		"transition first 'gather 2' then 'checkpoint 2';",
		"transition first 'checkpoint 2' then fork2;",
		"transition first 'deepest 2' then Idle;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_cfork", migrate.Approximated, "written as spread 2 since a sibling is also named spread")
	wantNote(t, r, "_cjoin", migrate.Approximated, "written as gather 2 since a sibling is also named gather")
	wantNote(t, r, "_chist", migrate.Approximated, "written as checkpoint 2 since a sibling is also named checkpoint")
	wantNote(t, r, "_cdeep", migrate.Approximated, "written as deepest 2 since a sibling is also named deepest")

	// The renamed fork enters both regions; the join, history and anonymous
	// fork bring the machine round to them again.
	s := session(t, r)
	meta(t, s, "%instantiate Press")
	meta(t, s, "%state Press::Cycle")
	meta(t, s, "%send Go")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "A1") || !strings.Contains(out, "B1") {
		t.Errorf("the renamed fork did not enter both regions:\n%s", out)
	}
	meta(t, s, "%step")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "A1") || !strings.Contains(out, "B1") {
		t.Errorf("the join, history and second fork did not re-enter both regions:\n%s", out)
	}
}

// internalMachine has an internal transition written with no target, one whose target is
// another state, and one leaving a choice pseudostate.
const internalMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_iping" name="Ping"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ipingEv" signal="_iping"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_ijump" name="Jump"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ijumpEv" signal="_ijump"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_igo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_igoEv" signal="_igo"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_counter" name="Counter" classifierBehavior="_ism">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_ipings" name="pings">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="_ipings0" value="0"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_ism" name="Counting">
        <region xmi:type="uml:Region" xmi:id="_ir" name="Main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_iinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_iidle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ibusy" name="Busy"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_ipick" kind="choice"/>
          <transition xmi:type="uml:Transition" xmi:id="_it0" source="_iinit" target="_iidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_itPing" kind="internal" source="_iidle">
            <trigger xmi:type="uml:Trigger" xmi:id="_itrPing" event="_ipingEv"/>
            <effect xmi:type="uml:OpaqueBehavior" xmi:id="_ieff">
              <language>JavaScript</language>
              <body>pings = pings + 1;</body>
            </effect>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_itJump" kind="internal" source="_iidle" target="_ibusy">
            <trigger xmi:type="uml:Trigger" xmi:id="_itrJump" event="_ijumpEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_itGo" source="_iidle" target="_ipick">
            <trigger xmi:type="uml:Trigger" xmi:id="_itrGo" event="_igoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_itPick" kind="internal" source="_ipick">
            <trigger xmi:type="uml:Trigger" xmi:id="_itrPick" event="_ipingEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_itOut" source="_ipick" target="_ibusy"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const internalApplications = `
  <sysml:Block xmi:id="_i1" base_Class="_counter"/>`

// An internal transition with no target stays in its source and is written as the self
// transition, effect included; one naming another target, or leaving a pseudostate, is refused.
func TestTargetlessInternalTransitionsStayInTheirSource(t *testing.T) {
	r := migrateDocument(t, internalMachine, internalApplications)
	for _, line := range []string{
		"transition first Idle accept Ping",
		"do action {",
		"assign this.pings := this.pings + 1;",
		"then Idle;",
		"/* not migrated: Transition (_itJump) — an internal transition targets 'Busy', not its source 'Idle'; whether it stays or moves cannot be told */",
		"/* not migrated: Transition (_itPick) — the source (_ipick) is a Pseudostate, and only a state has an internal transition */",
		"transition first choice then Busy;",
	} {
		wantLine(t, r.Notation, line)
	}
	wantNote(t, r, "_itPing", migrate.Mapped, "an internal transition is written as a self transition; Idle has no entry, exit or do behavior and no substates, so re-entering it is not observable")
	wantNote(t, r, "_itJump", migrate.Unmapped, "an internal transition targets 'Busy', not its source 'Idle'")
	wantNote(t, r, "_itPick", migrate.Unmapped, "only a state has an internal transition")

	s := session(t, r)
	meta(t, s, "%instantiate Counter")
	meta(t, s, "%state Counter::Counting")
	for i := 0; i < 2; i++ {
		if out := meta(t, s, "%send Ping"); !strings.Contains(out, "transition Idle -> Idle fires on it") {
			t.Errorf("%%send Ping: %s", out)
		}
		meta(t, s, "%step")
	}
	if out := meta(t, s, "%features #1"); !strings.Contains(out, "pings = 2") {
		t.Errorf("the internal transition's effect did not run twice:\n%s", out)
	}
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("the internal transition left Idle:\n%s", out)
	}
}

const emptyEffectMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ego" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_egoEv" signal="_ego"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_eclass" name="Blank" classifierBehavior="_esm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_esm" name="Blanking">
        <region xmi:type="uml:Region" xmi:id="_er" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_einit"/>
          <subvertex xmi:type="uml:State" xmi:id="_eidle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_ebusy" name="Busy"/>
          <transition xmi:type="uml:Transition" xmi:id="_et0" source="_einit" target="_eidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_etGo" source="_eidle" target="_ebusy">
            <trigger xmi:type="uml:Trigger" xmi:id="_etrGo" event="_egoEv"/>
            <effect xmi:type="uml:Activity" xmi:id="_eeff" name="effect"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const emptyEffectApplications = `
  <sysml:Block xmi:id="_e1" base_Class="_eclass"/>`

// A transition whose effect activity has no nodes keeps its braces, `do action effect { }`,
// so the `then` clause that follows still belongs to the transition; the result parses and runs.
func TestEmptyTransitionEffectKeepsItsBraces(t *testing.T) {
	r := migrateDocument(t, emptyEffectMachine, emptyEffectApplications)
	wantLine(t, r.Notation, "do action effect { }\n")
	wantLine(t, r.Notation, "then Busy;")
	if strings.Contains(string(r.Notation), "do action effect;") {
		t.Errorf("an empty effect ended the transition clause:\n%s", r.Notation)
	}
	s := session(t, r)
	meta(t, s, "%instantiate Blank")
	meta(t, s, "%state Blank::Blanking")
	if out := meta(t, s, "%send Go"); !strings.Contains(out, "transition Idle -> Busy fires on it") {
		t.Errorf("%%send Go: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Busy") {
		t.Errorf("the transition with the empty effect did not fire:\n%s", out)
	}
}

// joinShapesMachine has three orthogonal states whose exit point several regions reach, each
// also reached in a way a join cannot take: twice from one region, from outside the state,
// and from a junction.
const joinShapesMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_jgo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_jgoEv" signal="_jgo"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_jclass" name="Rig" classifierBehavior="_jsm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_jsm" name="Rigging">
        <region xmi:type="uml:Region" xmi:id="_jr" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_jinit"/>
          <subvertex xmi:type="uml:State" xmi:id="_jidle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_jtwice" name="Twice">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_jxTwice" name="out" kind="exitPoint"/>
            <region xmi:type="uml:Region" xmi:id="_jta" name="a">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_jtaInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_jta1" name="A1"/>
              <subvertex xmi:type="uml:State" xmi:id="_jta2" name="A2"/>
              <transition xmi:type="uml:Transition" xmi:id="_jtaT0" source="_jtaInit" target="_jta1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jtaT1" source="_jta1" target="_jxTwice">
                <trigger xmi:type="uml:Trigger" xmi:id="_jtaTr1" event="_jgoEv"/>
              </transition>
              <transition xmi:type="uml:Transition" xmi:id="_jtaT2" source="_jta2" target="_jxTwice"/>
            </region>
            <region xmi:type="uml:Region" xmi:id="_jtb" name="b">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_jtbInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_jtb1" name="B1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jtbT0" source="_jtbInit" target="_jtb1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jtbT1" source="_jtb1" target="_jxTwice">
                <trigger xmi:type="uml:Trigger" xmi:id="_jtbTr1" event="_jgoEv"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_jouter" name="Outer">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_jxOuter" name="out" kind="exitPoint"/>
            <region xmi:type="uml:Region" xmi:id="_joa" name="a">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_joaInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_joa1" name="A1"/>
              <transition xmi:type="uml:Transition" xmi:id="_joaT0" source="_joaInit" target="_joa1"/>
              <transition xmi:type="uml:Transition" xmi:id="_joaT1" source="_joa1" target="_jxOuter">
                <trigger xmi:type="uml:Trigger" xmi:id="_joaTr1" event="_jgoEv"/>
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="_job" name="b">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_jobInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_job1" name="B1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jobT0" source="_jobInit" target="_job1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jobT1" source="_job1" target="_jxOuter">
                <trigger xmi:type="uml:Trigger" xmi:id="_jobTr1" event="_jgoEv"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_jpseudo" name="Pseudo">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_jxPseudo" name="out" kind="exitPoint"/>
            <region xmi:type="uml:Region" xmi:id="_jpa" name="a">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_jpaInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_jpa1" name="A1"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_jpaJ" name="j" kind="junction"/>
              <transition xmi:type="uml:Transition" xmi:id="_jpaT0" source="_jpaInit" target="_jpa1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jpaT1" source="_jpa1" target="_jpaJ">
                <trigger xmi:type="uml:Trigger" xmi:id="_jpaTr1" event="_jgoEv"/>
              </transition>
              <transition xmi:type="uml:Transition" xmi:id="_jpaT2" source="_jpaJ" target="_jxPseudo"/>
            </region>
            <region xmi:type="uml:Region" xmi:id="_jpb" name="b">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_jpbInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_jpb1" name="B1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jpbT0" source="_jpbInit" target="_jpb1"/>
              <transition xmi:type="uml:Transition" xmi:id="_jpbT1" source="_jpb1" target="_jxPseudo">
                <trigger xmi:type="uml:Trigger" xmi:id="_jpbTr1" event="_jgoEv"/>
              </transition>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_jt0" source="_jinit" target="_jidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_jtIn" source="_jidle" target="_jxOuter">
            <trigger xmi:type="uml:Trigger" xmi:id="_jtrIn" event="_jgoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_jtTwice" source="_jxTwice" target="_jidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_jtOuter" source="_jxOuter" target="_jidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_jtPseudo" source="_jxPseudo" target="_jidle"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const joinShapesApplications = `
  <sysml:Block xmi:id="_j1" base_Class="_jclass"/>`

// An exit point several regions reach is a join; one also reached twice from one region or from
// a pseudostate is refused with that shape named, one reached from outside its state with that
// transition named, and their transitions with them.
func TestExitPointJoinShapesAreRefusedPrecisely(t *testing.T) {
	r := migrateDocument(t, joinShapesMachine, joinShapesApplications)
	if strings.Contains(string(r.Notation), "join out;") {
		t.Errorf("a refused exit point was written as a join:\n%s", r.Notation)
	}
	wantNote(t, r, "_jxTwice", migrate.Unmapped, "as through a join, but two of its incoming transitions leave the same region")
	wantNote(t, r, "_jxOuter", migrate.Unmapped, "(_jtIn) leads to the exit point from outside the state, from 'Idle'")
	wantNote(t, r, "_jxPseudo", migrate.Unmapped, "as through a join, but (_jpaT2) leaves 'j', a Pseudostate rather than a state")
	for _, id := range []string{"_jtaT1", "_jtaT2", "_jtbT1", "_jtTwice", "_joaT1", "_jobT1", "_jtIn", "_jtOuter", "_jpaT2", "_jpbT1", "_jtPseudo"} {
		wantNote(t, r, id, migrate.Unmapped, "has no v2 form")
	}
}

// entryShapesMachine has five composite states whose entry point leaves by a route a junction
// cannot take: out of the state, on into the state's history, to no target, back to the state,
// and on a trigger.
const entryShapesMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ego" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_egoEv" signal="_ego"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_eclass" name="Rig" classifierBehavior="_esm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_esm" name="Rigging">
        <region xmi:type="uml:Region" xmi:id="_er" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_einit"/>
          <subvertex xmi:type="uml:State" xmi:id="_eidle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_eaway" name="Away">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_eaIn" name="in" kind="entryPoint"/>
            <region xmi:type="uml:Region" xmi:id="_ear" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_earInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_ear1" name="A1"/>
              <transition xmi:type="uml:Transition" xmi:id="_earT0" source="_earInit" target="_ear1"/>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_eback" name="Back">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_ebIn" name="in" kind="entryPoint"/>
            <region xmi:type="uml:Region" xmi:id="_ebr" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_ebrInit"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_ebH" name="H" kind="shallowHistory"/>
              <subvertex xmi:type="uml:State" xmi:id="_ebr1" name="B1"/>
              <transition xmi:type="uml:Transition" xmi:id="_ebrT0" source="_ebrInit" target="_ebr1"/>
              <transition xmi:type="uml:Transition" xmi:id="_ebrT1" source="_ebIn" target="_ebH"/>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_ebare" name="Bare">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_ecIn" name="in" kind="entryPoint"/>
            <region xmi:type="uml:Region" xmi:id="_ecr" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_ecrInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_ecr1" name="C1"/>
              <transition xmi:type="uml:Transition" xmi:id="_ecrT0" source="_ecrInit" target="_ecr1"/>
              <transition xmi:type="uml:Transition" xmi:id="_ecrT1" source="_ecIn" target="_egone"/>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_eself" name="Self">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_edIn" name="in" kind="entryPoint"/>
            <region xmi:type="uml:Region" xmi:id="_edr" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_edrInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_edr1" name="D1"/>
              <transition xmi:type="uml:Transition" xmi:id="_edrT0" source="_edrInit" target="_edr1"/>
              <transition xmi:type="uml:Transition" xmi:id="_edrT1" kind="local" source="_edIn" target="_eself"/>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_ewait" name="Wait">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_eeIn" name="in" kind="entryPoint"/>
            <region xmi:type="uml:Region" xmi:id="_eer" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_eerInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_eer1" name="E1"/>
              <subvertex xmi:type="uml:State" xmi:id="_eer2" name="E2"/>
              <transition xmi:type="uml:Transition" xmi:id="_eerT0" source="_eerInit" target="_eer1"/>
              <transition xmi:type="uml:Transition" xmi:id="_eerT1" source="_eeIn" target="_eer2">
                <trigger xmi:type="uml:Trigger" xmi:id="_eerTr1" event="_egoEv"/>
              </transition>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_et0" source="_einit" target="_eidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_etWait" source="_eidle" target="_eeIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_etrWait" event="_egoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_etSelf" source="_eidle" target="_edIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_etrSelf" event="_egoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_etAway" source="_eidle" target="_eaIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_etrAway" event="_egoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_etOut" source="_eaIn" target="_eidle"/>
          <transition xmi:type="uml:Transition" xmi:id="_etBack" source="_eidle" target="_ebIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_etrBack" event="_egoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_etBare" source="_eidle" target="_ecIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_etrBare" event="_egoEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const entryShapesApplications = `
  <sysml:Block xmi:id="_e1" base_Class="_eclass"/>`

// An entry point whose route leaves the state, runs on into a history, reaches no target, leads
// back to the state or waits on a trigger is refused with that route named, the transitions
// through it with it, and no junction is written.
func TestEntryPointRoutesAreRefusedPrecisely(t *testing.T) {
	r := migrateDocument(t, entryShapesMachine, entryShapesApplications)
	if strings.Contains(string(r.Notation), "junction in;") {
		t.Errorf("a refused entry point was written as a junction:\n%s", r.Notation)
	}
	wantNote(t, r, "_eaIn", migrate.Unmapped, "(_etOut) leads from the entry point out of the state, to 'Idle'")
	wantNote(t, r, "_ebIn", migrate.Unmapped, "(_ebrT1) leads from the entry point on into the history pseudostate 'H', which the runtime does not follow from a junction")
	wantNote(t, r, "_ecIn", migrate.Unmapped, "(_ecrT1) leads from the entry point to no target")
	wantNote(t, r, "_edIn", migrate.Unmapped, "(_edrT1) leads from the entry point back to the state itself, which v1 enters by its default entry while the runtime would leave and re-enter it")
	wantNote(t, r, "_eeIn", migrate.Unmapped, "(_eerT1) leads from the entry point with a trigger, which no transition out of a pseudostate takes; the runtime would follow it without waiting for the event")
	for _, id := range []string{"_etAway", "_etOut", "_etBack", "_etBare", "_etSelf", "_edrT1", "_etWait", "_eerT1"} {
		wantNote(t, r, id, migrate.Unmapped, "has no v2 form")
	}
	wantNote(t, r, "_ebrT1", migrate.Unmapped, "does not follow a transition from a entryPoint pseudostate on into the history pseudostate")
	wantNote(t, r, "_ecrT1", migrate.Unmapped, "lacks an end")
}

// guardedEntryMachine has a composite state whose entry point leaves by a guarded transition,
// and whose entry behavior falsifies that guard.
const guardedEntryMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ggo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ggoEv" signal="_ggo"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_gclass" name="Rig" classifierBehavior="_gsm">
      <ownedAttribute xmi:type="uml:Property" xmi:id="_garmed" name="armed">
        <type xmi:type="uml:PrimitiveType" href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Boolean"/>
        <defaultValue xmi:type="uml:LiteralBoolean" xmi:id="_garmed0" value="true"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_gsm" name="Main">
        <region xmi:type="uml:Region" xmi:id="_gr" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_gInit"/>
          <subvertex xmi:type="uml:State" xmi:id="_gIdle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_gWork" name="Work">
            <entry xmi:type="uml:OpaqueBehavior" xmi:id="_gWorkEntry">
              <language>JavaScript</language>
              <body>armed = false;</body>
            </entry>
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_gIn" name="arm" kind="entryPoint"/>
            <region xmi:type="uml:Region" xmi:id="_gwr" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_gwInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_gw1" name="W1"/>
              <subvertex xmi:type="uml:State" xmi:id="_gw2" name="W2"/>
              <transition xmi:type="uml:Transition" xmi:id="_gwT0" source="_gwInit" target="_gw1"/>
              <transition xmi:type="uml:Transition" xmi:id="_gwT1" source="_gIn" target="_gw2">
                <guard xmi:type="uml:Constraint" xmi:id="_gGuard">
                  <specification xmi:type="uml:OpaqueExpression" xmi:id="_gGuardSpec">
                    <language>JavaScript</language>
                    <body>armed</body>
                  </specification>
                </guard>
              </transition>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_gT0" source="_gInit" target="_gIdle"/>
          <transition xmi:type="uml:Transition" xmi:id="_gT1" source="_gIdle" target="_gIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_gTr1" event="_ggoEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const guardedEntryApplications = `
  <sysml:Block xmi:id="_g1" base_Class="_gclass"/>`

// A guard on the transition leaving an entry point is kept: UML evaluates it with the compound
// transition's other guards before the transition fires, as the runtime does at a junction, so
// the owning state's entry behavior falsifying it does not turn the route.
func TestGuardedEntryPointRouteIsKept(t *testing.T) {
	r := migrateDocument(t, guardedEntryMachine, guardedEntryApplications)
	for _, line := range []string{"junction arm;", "transition first Work::arm if this.armed then W2;", "transition first Idle accept Go then Work::arm;"} {
		if !strings.Contains(string(r.Notation), line) {
			t.Errorf("missing %q in:\n%s", line, r.Notation)
		}
	}
	wantNote(t, r, "_gIn", migrate.Mapped, "written as a junction of its state")
	wantNote(t, r, "_gGuard", migrate.Mapped, "")
	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Rig::Main #1")
	if out := meta(t, s, "%send Go"); !strings.Contains(out, "transition Idle -> arm fires on it") {
		t.Errorf("%%send Go: %s", out)
	}
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: W2") {
		t.Errorf("the guarded entry route was not taken on the guard's value before Work's entry:\n%s", out)
	}
	if out := meta(t, s, "%eval in #1 : armed"); !strings.Contains(out, "= false") {
		t.Errorf("Work's entry behavior did not run: %s", out)
	}
}

// nestedMachines has a machine Front whose submachine state enters Outer through a connection
// point reference, so Outer is named before it is written, and a machine Inner nested in Outer
// whose composite state Par has an entry point forking into its two regions.
const nestedMachines = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ngo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ngoEv" signal="_ngo"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_nclass" name="Rig" classifierBehavior="_nfront">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_nfront" name="Front">
        <region xmi:type="uml:Region" xmi:id="_nfr" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_nfInit"/>
          <subvertex xmi:type="uml:State" xmi:id="_nfIdle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_nfSub" name="Sub" submachine="_nouter">
            <connection xmi:type="uml:ConnectionPointReference" xmi:id="_nfRef" name="viaStart" entry="_noStart"/>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_nfT0" source="_nfInit" target="_nfIdle"/>
          <transition xmi:type="uml:Transition" xmi:id="_nfT1" source="_nfIdle" target="_nfRef">
            <trigger xmi:type="uml:Trigger" xmi:id="_nfTr1" event="_ngoEv"/>
          </transition>
        </region>
      </ownedBehavior>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_nouter" name="Outer">
        <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_noStart" name="start" kind="entryPoint"/>
        <nestedClassifier xmi:type="uml:StateMachine" xmi:id="_ninner" name="Inner">
          <region xmi:type="uml:Region" xmi:id="_nir" name="main">
            <subvertex xmi:type="uml:Pseudostate" xmi:id="_niInit"/>
            <subvertex xmi:type="uml:State" xmi:id="_niIdle" name="Idle"/>
            <subvertex xmi:type="uml:State" xmi:id="_niPar" name="Par">
              <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_niIn" name="split" kind="entryPoint"/>
              <region xmi:type="uml:Region" xmi:id="_nia" name="a">
                <subvertex xmi:type="uml:Pseudostate" xmi:id="_niaInit"/>
                <subvertex xmi:type="uml:State" xmi:id="_nia1" name="A1"/>
                <subvertex xmi:type="uml:State" xmi:id="_nia2" name="A2"/>
                <transition xmi:type="uml:Transition" xmi:id="_niaT0" source="_niaInit" target="_nia1"/>
                <transition xmi:type="uml:Transition" xmi:id="_niaT1" source="_niIn" target="_nia2"/>
              </region>
              <region xmi:type="uml:Region" xmi:id="_nib" name="b">
                <subvertex xmi:type="uml:Pseudostate" xmi:id="_nibInit"/>
                <subvertex xmi:type="uml:State" xmi:id="_nib1" name="B1"/>
                <subvertex xmi:type="uml:State" xmi:id="_nib2" name="B2"/>
                <transition xmi:type="uml:Transition" xmi:id="_nibT0" source="_nibInit" target="_nib1"/>
                <transition xmi:type="uml:Transition" xmi:id="_nibT1" source="_niIn" target="_nib2"/>
              </region>
            </subvertex>
            <transition xmi:type="uml:Transition" xmi:id="_niT0" source="_niInit" target="_niIdle"/>
            <transition xmi:type="uml:Transition" xmi:id="_niT1" source="_niIdle" target="_niIn">
              <trigger xmi:type="uml:Trigger" xmi:id="_niTr1" event="_ngoEv"/>
            </transition>
          </region>
        </nestedClassifier>
        <region xmi:type="uml:Region" xmi:id="_nor" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_noInit"/>
          <subvertex xmi:type="uml:State" xmi:id="_noWait" name="Wait"/>
          <subvertex xmi:type="uml:State" xmi:id="_noRun" name="Run" submachine="_ninner"/>
          <transition xmi:type="uml:Transition" xmi:id="_noT0" source="_noInit" target="_noWait"/>
          <transition xmi:type="uml:Transition" xmi:id="_noT1" source="_noStart" target="_noRun"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const nestedMachinesApplications = `
  <sysml:Block xmi:id="_n1" base_Class="_nclass"/>`

// A machine nested in another is indexed once however early the outer one is named, so the
// entry point of its composite state still forks into two regions rather than four transitions.
func TestNestedMachineTransitionsAreIndexedOnce(t *testing.T) {
	r := migrateDocument(t, nestedMachines, nestedMachinesApplications)
	for _, line := range []string{
		"fork split;",
		"transition first Par::split then A2;",
		"transition first Par::split then B2;",
		"transition first Idle accept Go then Par::split;",
	} {
		if !strings.Contains(string(r.Notation), line) {
			t.Errorf("missing %q in:\n%s", line, r.Notation)
		}
	}
	wantNote(t, r, "_niIn", migrate.Mapped, "written as a fork of its state")
}

// regionListedPoints has an orthogonal state Sync whose entry point Both and exit point Gather a
// tool listed among the vertices of its regions rather than as its connection points.
const regionListedPoints = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_rgo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_rgoEv" signal="_rgo"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_rclass" name="Rig" classifierBehavior="_rsm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_rsm" name="Main">
        <region xmi:type="uml:Region" xmi:id="_rr" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_rInit"/>
          <subvertex xmi:type="uml:State" xmi:id="_rIdle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_rSync" name="Sync">
            <region xmi:type="uml:Region" xmi:id="_ra" name="a">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_rBoth" name="Both" kind="entryPoint"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_raInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_ra1" name="A1"/>
              <subvertex xmi:type="uml:State" xmi:id="_ra2" name="A2"/>
              <transition xmi:type="uml:Transition" xmi:id="_raT0" source="_raInit" target="_ra1"/>
              <transition xmi:type="uml:Transition" xmi:id="_raT1" source="_rBoth" target="_ra2"/>
              <transition xmi:type="uml:Transition" xmi:id="_raT2" source="_ra2" target="_rGather"/>
            </region>
            <region xmi:type="uml:Region" xmi:id="_rb" name="b">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_rGather" name="Gather" kind="exitPoint"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_rbInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_rb1" name="B1"/>
              <subvertex xmi:type="uml:State" xmi:id="_rb2" name="B2"/>
              <transition xmi:type="uml:Transition" xmi:id="_rbT0" source="_rbInit" target="_rb1"/>
              <transition xmi:type="uml:Transition" xmi:id="_rbT1" source="_rBoth" target="_rb2"/>
              <transition xmi:type="uml:Transition" xmi:id="_rbT2" source="_rb2" target="_rGather"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_rT0" source="_rInit" target="_rIdle"/>
          <transition xmi:type="uml:Transition" xmi:id="_rT1" source="_rIdle" target="_rBoth">
            <trigger xmi:type="uml:Trigger" xmi:id="_rTr1" event="_rgoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_rT2" source="_rGather" target="_rIdle"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const regionListedPointsApplications = `
  <sysml:Block xmi:id="_r1" base_Class="_rclass"/>`

// A connection point a tool lists in a region of an orthogonal state is written in the state's
// body, and every transition through it names it there, not in the region it was listed in.
func TestRegionListedPointsArePathedFromTheirState(t *testing.T) {
	r := migrateDocument(t, regionListedPoints, regionListedPointsApplications)
	for _, line := range []string{
		"fork Both;",
		"join Gather;",
		"transition first Idle accept Go then Sync::Both;",
		"transition first Sync::Both then A2;",
		"transition first Sync::Both then B2;",
		"transition first A2 then Sync::Gather;",
		"transition first B2 then Sync::Gather;",
		"transition first Sync::Gather then Idle;",
	} {
		if !strings.Contains(string(r.Notation), line) {
			t.Errorf("missing %q in:\n%s", line, r.Notation)
		}
	}
	if strings.Contains(string(r.Notation), "regions::") {
		t.Errorf("a connection point is named through the region it was listed in:\n%s", r.Notation)
	}
	wantNote(t, r, "_rBoth", migrate.Mapped, "written as a fork of its state")
	wantNote(t, r, "_rGather", migrate.Mapped, "written as a join of its state")
	session(t, r)
}

// pointOnlyRegion has a state Sync with one region of its own, a, and a second, b, in which a
// tool listed nothing but Sync's entry point Both and exit point Gather and the transition
// leaving Both.
const pointOnlyRegion = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_pgo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pgoEv" signal="_pgo"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_pstop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_pstopEv" signal="_pstop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_pclass" name="Rig" classifierBehavior="_psm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_psm" name="Main">
        <region xmi:type="uml:Region" xmi:id="_pr" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_pInit"/>
          <subvertex xmi:type="uml:State" xmi:id="_pIdle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_pSync" name="Sync">
            <region xmi:type="uml:Region" xmi:id="_pa" name="a">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_paInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_pa1" name="A1"/>
              <subvertex xmi:type="uml:State" xmi:id="_pa2" name="A2"/>
              <transition xmi:type="uml:Transition" xmi:id="_paT0" source="_paInit" target="_pa1"/>
              <transition xmi:type="uml:Transition" xmi:id="_paT2" source="_pa2" target="_pGather">
                <trigger xmi:type="uml:Trigger" xmi:id="_paTr2" event="_pstopEv"/>
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="_pb" name="b">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_pBoth" name="Both" kind="entryPoint"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_pGather" name="Gather" kind="exitPoint"/>
              <transition xmi:type="uml:Transition" xmi:id="_pbT1" source="_pBoth" target="_pa2"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_pT0" source="_pInit" target="_pIdle"/>
          <transition xmi:type="uml:Transition" xmi:id="_pT1" source="_pIdle" target="_pBoth">
            <trigger xmi:type="uml:Trigger" xmi:id="_pTr1" event="_pgoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_pT2" source="_pGather" target="_pIdle"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const pointOnlyRegionApplications = `
  <sysml:Block xmi:id="_p1" base_Class="_pclass"/>`

// A region listing only its state's connection points holds nothing to enter: it is skipped, so the
// state has one region and its points are junctions, not a fork and a join into a parallel state
// with an empty branch; the transitions the region holds are written in the state's body.
func TestPointOnlyRegionIsNotAParallelBranch(t *testing.T) {
	r := migrateDocument(t, pointOnlyRegion, pointOnlyRegionApplications)
	for _, line := range []string{
		"junction Both;",
		"junction Gather;",
		"transition first Idle accept Go then Sync::Both;",
		"transition first Sync::Both then A2;",
		"transition first A2 accept Stop then Sync::Gather;",
		"transition first Sync::Gather then Idle;",
	} {
		if !strings.Contains(string(r.Notation), line) {
			t.Errorf("missing %q in:\n%s", line, r.Notation)
		}
	}
	for _, bad := range []string{"parallel", "regions::", "fork ", "join "} {
		if strings.Contains(string(r.Notation), bad) {
			t.Errorf("the point-only region was written as a parallel branch (%q):\n%s", bad, r.Notation)
		}
	}
	wantNote(t, r, "_pb", migrate.Skipped, "the region lists only connection points of its state, which are written in the state's body")
	wantNote(t, r, "_pBoth", migrate.Mapped, "written as a junction of its state")
	wantNote(t, r, "_pGather", migrate.Mapped, "written as a junction of its state")
	wantNote(t, r, "_pbT1", migrate.Mapped, "")
	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Rig::Main #1")
	meta(t, s, "%send Go")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: A2") {
		t.Errorf("entering through Both did not reach A2:\n%s", out)
	}
	meta(t, s, "%send Stop")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("leaving through Gather did not reach Idle:\n%s", out)
	}
}

// noDefaultEntryMachine has a composite state Work whose region holds a state but no initial
// pseudostate, and an entry point in that no transition leaves.
const noDefaultEntryMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_dgo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_dgoEv" signal="_dgo"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="_dstop" name="Stop"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_dstopEv" signal="_dstop"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_dclass" name="Rig" classifierBehavior="_dsm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_dsm" name="Main">
        <region xmi:type="uml:Region" xmi:id="_dr" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_dInit"/>
          <subvertex xmi:type="uml:State" xmi:id="_dIdle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_dWork" name="Work">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_dIn" name="via" kind="entryPoint"/>
            <region xmi:type="uml:Region" xmi:id="_dwr" name="r">
              <subvertex xmi:type="uml:State" xmi:id="_dw1" name="W1"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_dT0" source="_dInit" target="_dIdle"/>
          <transition xmi:type="uml:Transition" xmi:id="_dT1" source="_dIdle" target="_dIn">
            <trigger xmi:type="uml:Trigger" xmi:id="_dTr1" event="_dgoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_dT2" source="_dWork" target="_dIdle">
            <trigger xmi:type="uml:Trigger" xmi:id="_dTr2" event="_dstopEv"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>`

const noDefaultEntryApplications = `
  <sysml:Block xmi:id="_d1" base_Class="_dclass"/>`

// An entry point no transition leaves enters its state by the state's default entry, which, with no
// initial pseudostate in the region, enters the state and leaves the region inactive in v1 and in
// the runtime alike; the transition is written to the state and the report says so.
func TestDefaultEntryPointOnOwnerWithoutInitialEntersTheState(t *testing.T) {
	r := migrateDocument(t, noDefaultEntryMachine, noDefaultEntryApplications)
	for _, line := range []string{
		"transition first Idle accept Go then Work;",
		"transition first Work accept Stop then Idle;",
		"the region has no initial pseudostate: nothing enters it",
	} {
		if !strings.Contains(string(r.Notation), line) {
			t.Errorf("missing %q in:\n%s", line, r.Notation)
		}
	}
	if strings.Contains(string(r.Notation), "junction via;") {
		t.Errorf("an entry point no transition leaves was written as a junction:\n%s", r.Notation)
	}
	wantNote(t, r, "_dIn", migrate.Mapped, "no initial pseudostate starts the region 'r', which v1 too leaves inactive on entering the state")
	wantNote(t, r, "_dT1", migrate.Mapped, "written to Work: no transition leaves the entry point 'via'")
	s := session(t, r)
	meta(t, s, "%instantiate Rig")
	meta(t, s, "%state Rig::Main #1")
	meta(t, s, "%send Go")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Work") || strings.Contains(out, "W1") {
		t.Errorf("entering Work by its default entry did not leave its region inactive:\n%s", out)
	}
	meta(t, s, "%send Stop")
	meta(t, s, "%step")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Idle") {
		t.Errorf("Work was not left on Stop:\n%s", out)
	}
}

// outsideExitMachine has two composite states with one exit point each: Work's is reached from
// Idle, outside Work, besides from within; Self's by a local transition of Self itself.
const outsideExitMachine = `
    <packagedElement xmi:type="uml:Signal" xmi:id="_ogo" name="Go"/>
    <packagedElement xmi:type="uml:SignalEvent" xmi:id="_ogoEv" signal="_ogo"/>
    <packagedElement xmi:type="uml:Class" xmi:id="_oclass" name="Rig" classifierBehavior="_osm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_osm" name="Main">
        <region xmi:type="uml:Region" xmi:id="_or" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_oInit"/>
          <subvertex xmi:type="uml:State" xmi:id="_oIdle" name="Idle"/>
          <subvertex xmi:type="uml:State" xmi:id="_oWork" name="Work">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_oOut" name="leave" kind="exitPoint"/>
            <region xmi:type="uml:Region" xmi:id="_owr" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_owInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_ow1" name="W1"/>
              <transition xmi:type="uml:Transition" xmi:id="_owT0" source="_owInit" target="_ow1"/>
              <transition xmi:type="uml:Transition" xmi:id="_owT1" source="_ow1" target="_oOut">
                <trigger xmi:type="uml:Trigger" xmi:id="_owTr1" event="_ogoEv"/>
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="_oSelf" name="Self">
            <connectionPoint xmi:type="uml:Pseudostate" xmi:id="_oSOut" name="leave" kind="exitPoint"/>
            <region xmi:type="uml:Region" xmi:id="_osr" name="r">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_osInit"/>
              <subvertex xmi:type="uml:State" xmi:id="_os1" name="S1"/>
              <transition xmi:type="uml:Transition" xmi:id="_osT0" source="_osInit" target="_os1"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_oT0" source="_oInit" target="_oIdle"/>
          <transition xmi:type="uml:Transition" xmi:id="_oT1" source="_oIdle" target="_oOut">
            <trigger xmi:type="uml:Trigger" xmi:id="_oTr1" event="_ogoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_oT2" source="_oOut" target="_oIdle"/>
          <transition xmi:type="uml:Transition" xmi:id="_oT3" source="_oSelf" target="_oSOut" kind="local">
            <trigger xmi:type="uml:Trigger" xmi:id="_oTr3" event="_ogoEv"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="_oT4" source="_oSOut" target="_oIdle"/>
        </region>
      </ownedBehavior>
    </packagedElement>`

const outsideExitApplications = `
  <sysml:Block xmi:id="_o1" base_Class="_oclass"/>`

// An exit point one transition reaches from outside its state is refused with that transition named
// however few transitions reach it: written as a junction, the runtime would enter the state to leave
// it, running entry and exit behaviors v1 never runs. One a local transition of the state itself
// reaches is a junction, the transition approximated as the local rule says.
func TestExitPointReachedFromOutsideIsRefused(t *testing.T) {
	r := migrateDocument(t, outsideExitMachine, outsideExitApplications)
	if n := strings.Count(string(r.Notation), "junction leave;"); n != 1 {
		t.Errorf("want Self's exit point alone written as a junction, got %d:\n%s", n, r.Notation)
	}
	for _, line := range []string{"transition first Self accept Go then Self::leave;", "transition first Self::leave then Idle;"} {
		if !strings.Contains(string(r.Notation), line) {
			t.Errorf("missing %q in:\n%s", line, r.Notation)
		}
	}
	wantNote(t, r, "_oOut", migrate.Unmapped, "(_oT1) leads to the exit point from outside the state, from 'Idle'; v1 never enters the state, while the runtime would enter and leave it")
	for _, id := range []string{"_owT1", "_oT1", "_oT2"} {
		wantNote(t, r, id, migrate.Unmapped, "has no v2 form")
	}
	wantNote(t, r, "_oSOut", migrate.Mapped, "written as a junction of its state")
	wantNote(t, r, "_oT3", migrate.Approximated, "a local transition is written external: the composite state Self exits and re-enters")
	wantNote(t, r, "_oT4", migrate.Mapped, "")
}
