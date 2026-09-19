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
