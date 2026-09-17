package migrate_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// The fixture testdata/xmi/plant_states.xmi is a block whose classifier
// behavior is a state machine with transitions crossing region boundaries in
// both directions, a junction with an else branch, a fork and join around a
// state of two regions, a shallow and a deep history, a submachine state
// entered and left through its entry and exit points, an internal transition on
// a plain state, a local transition, an absolute time event, and a signal event
// no trigger refers to. A transition whose ends lie in different regions names
// the nested end by its path; a junction and its else guard become a junction
// pseudostate with an unguarded branch; fork and join pseudostates keep their
// kind; a shallow or deep history is a history member; an entry or exit point
// is a state of the submachine's state def that a connection point reference
// names through the submachine state; an internal transition on a plain state
// is a self transition; a local transition is written external and noted; a
// literal absolute time is a TimeInstantValue attribute the transition accepts
// at, one that is not is reported; an event no trigger refers to is skipped.
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
