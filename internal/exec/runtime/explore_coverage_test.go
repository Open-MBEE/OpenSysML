package runtime

import (
	"context"
	"slices"
	"testing"
)

// Two orders of one machine reach one outcome, through viaA and through viaB.
// The outcome's witness entered viaA; the coverage still holds viaB, which only
// the other run entered, counting the runs that entered each state and took
// each transition.
func TestExploreCoverageSpansEveryRun(t *testing.T) {
	m := parseExploreModel(t, `package test {
		part def M;
		state def Run {
			entry; then s0;
			state s0;
			transition first s0 then viaA;
			transition first s0 then viaB;
			state viaA;
			transition first viaA then fin;
			state viaB;
			transition first viaB then fin;
			state fin;
		}
	}`)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	machine := oneSymbol(t, m.idx, "test::M")
	stateSym := oneSymbol(t, m.idx, "test::Run")
	x, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
		self, err := ctx.Instantiate(machine)
		if err != nil {
			return Outcome{}, err
		}
		exec, err := newStateExecutor(ctx, stateSym, self)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.initialize(); err != nil {
			return Outcome{}, err
		}
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return Outcome{FinalState: StateVertexName(exec.CurrentState()), ctx: ctx}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 || len(x.Outcomes) != 1 || x.Outcomes[0].Linearizations != 2 {
		t.Fatalf("status %q, %d outcomes, want one outcome reached by both of 2 runs", x.Status(), len(x.Outcomes))
	}
	wantStates := []CoverageEntry{
		{Path: "test::M#1.Run: fin", Declaration: "test::Run::fin", Runs: 2},
		{Path: "test::M#1.Run: s0", Declaration: "test::Run::s0", Runs: 2},
		{Path: "test::M#1.Run: viaA", Declaration: "test::Run::viaA", Runs: 1},
		{Path: "test::M#1.Run: viaB", Declaration: "test::Run::viaB", Runs: 1},
	}
	if !slices.Equal(x.Coverage.States, wantStates) {
		t.Errorf("states entered %+v\nwant %+v", x.Coverage.States, wantStates)
	}
	wantTransitions := []CoverageEntry{
		{Path: "test::M#1.Run: s0 -> viaA", Declaration: "test::Run::s0 -> viaA", Runs: 1},
		{Path: "test::M#1.Run: s0 -> viaB", Declaration: "test::Run::s0 -> viaB", Runs: 1},
		{Path: "test::M#1.Run: viaA -> fin", Declaration: "test::Run::viaA -> fin", Runs: 1},
		{Path: "test::M#1.Run: viaB -> fin", Declaration: "test::Run::viaB -> fin", Runs: 1},
	}
	if !slices.Equal(x.Coverage.Transitions, wantTransitions) {
		t.Errorf("transitions taken %+v\nwant %+v", x.Coverage.Transitions, wantTransitions)
	}
	wantLines := []string{
		"coverage over all runs: 4 states entered, 4 transitions taken",
		"  state test::M#1.Run: fin (test::Run::fin): 2 runs",
		"  state test::M#1.Run: s0 (test::Run::s0): 2 runs",
		"  state test::M#1.Run: viaA (test::Run::viaA): 1 run",
		"  state test::M#1.Run: viaB (test::Run::viaB): 1 run",
		"  transition test::M#1.Run: s0 -> viaA (test::Run::s0 -> viaA): 1 run",
		"  transition test::M#1.Run: s0 -> viaB (test::Run::s0 -> viaB): 1 run",
		"  transition test::M#1.Run: viaA -> fin (test::Run::viaA -> fin): 1 run",
		"  transition test::M#1.Run: viaB -> fin (test::Run::viaB -> fin): 1 run",
	}
	if got := x.Coverage.Lines("run"); !slices.Equal(got, wantLines) {
		t.Errorf("lines %q\nwant %q", got, wantLines)
	}
}

// exploreMachine explores the state machine stateDef running on an instance of
// partDef under the explore policy.
func exploreMachine(t *testing.T, m *exploreModel, partDef, stateDef string) *Exploration {
	t.Helper()
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	machine := oneSymbol(t, m.idx, partDef)
	stateSym := oneSymbol(t, m.idx, stateDef)
	x, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
		self, err := ctx.Instantiate(machine)
		if err != nil {
			return Outcome{}, err
		}
		exec, err := newStateExecutor(ctx, stateSym, self)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.initialize(); err != nil {
			return Outcome{}, err
		}
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return Outcome{FinalState: StateVertexName(exec.CurrentState()), ctx: ctx}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return x
}

// A fork's branches and a join's segments are transitions taken: the coverage
// holds each branch from the fork and each segment into the join, beside the
// transition into the fork and the one out of the join, which leaves the
// composite state the join's segments exited.
func TestExploreCoverageHoldsForkBranchesAndJoinSegments(t *testing.T) {
	m := parseExploreModel(t, `package test {
		part def M;
		state def Run {
			entry; then ready;
			state ready;
			state working parallel {
				state left {
					entry; then ls;
					state ls;
					state lwork;
				}
				state right {
					entry; then rs;
					state rs;
					state rwork;
				}
			}
			state fin;
			fork split;
			join sync;
			transition first ready then split;
			transition first split then lwork;
			transition first split then rwork;
			transition first lwork then sync;
			transition first rwork then sync;
			transition first sync then fin;
		}
	}`)
	x := exploreMachine(t, m, "test::M", "test::Run")
	if !x.Complete() {
		t.Fatalf("status %q, want complete", x.Status())
	}
	var taken []string
	for _, entry := range x.Coverage.Transitions {
		taken = append(taken, entry.Declaration)
	}
	for _, want := range []string{
		"test::Run::ready -> split",
		"test::Run::split -> working::left::lwork",
		"test::Run::split -> working::right::rwork",
		"test::Run::working::left::lwork -> sync",
		"test::Run::working::right::rwork -> sync",
		"test::Run::working -> fin",
	} {
		if !slices.Contains(taken, want) {
			t.Errorf("transitions taken %q lack %q", taken, want)
		}
	}
}

// Two regions reusing the same local state names take transitions of their own:
// the coverage keys a transition by the paths of its states, as it keys states.
func TestExploreCoverageTellsRegionsReusingNamesApart(t *testing.T) {
	m := parseExploreModel(t, `package test {
		part def M;
		state def Run {
			entry; then working;
			state working parallel {
				state left {
					entry; then s0;
					state s0;
					state s1;
					transition first s0 then s1;
				}
				state right {
					entry; then s0;
					state s0;
					state s1;
					transition first s0 then s1;
				}
			}
		}
	}`)
	x := exploreMachine(t, m, "test::M", "test::Run")
	if !x.Complete() {
		t.Fatalf("status %q, want complete", x.Status())
	}
	var taken []string
	for _, entry := range x.Coverage.Transitions {
		taken = append(taken, entry.Path+" ("+entry.Declaration+")")
	}
	want := []string{
		"test::M#1.Run: working.left.s0 -> working.left.s1 (test::Run::working::left::s0 -> working::left::s1)",
		"test::M#1.Run: working.right.s0 -> working.right.s1 (test::Run::working::right::s0 -> working::right::s1)",
	}
	if !slices.Equal(taken, want) {
		t.Errorf("transitions taken %q\nwant %q", taken, want)
	}
}

// A name that is no basic name is quoted in the declaration, as in any qualified
// name, so a dot inside it does not read as nesting.
func TestExploreCoverageQuotesNamesThatAreNoBasicNames(t *testing.T) {
	m := parseExploreModel(t, `package test {
		part def M;
		state def Run {
			entry; then s0;
			state s0;
			state 'ready.now';
			transition first s0 then 'ready.now';
		}
	}`)
	x := exploreMachine(t, m, "test::M", "test::Run")
	if !x.Complete() {
		t.Fatalf("status %q, want complete", x.Status())
	}
	var lines []string
	for _, entry := range x.Coverage.States {
		lines = append(lines, entry.Path+" ("+entry.Declaration+")")
	}
	for _, entry := range x.Coverage.Transitions {
		lines = append(lines, entry.Path+" ("+entry.Declaration+")")
	}
	want := []string{
		"test::M#1.Run: ready.now (test::Run::'ready.now')",
		"test::M#1.Run: s0 (test::Run::s0)",
		"test::M#1.Run: s0 -> ready.now (test::Run::s0 -> 'ready.now')",
	}
	if !slices.Equal(lines, want) {
		t.Errorf("coverage %q\nwant %q", lines, want)
	}
}
