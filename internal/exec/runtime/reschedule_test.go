package runtime

import (
	"errors"
	"fmt"
	"testing"
)

// rescheduleSource is an object whose machine offers two transitions on one
// signal, so which fires is a choice: the first declared under `reverse` and
// `declared`, a draw under a seed.
const rescheduleSource = `
	attribute def Go;
	part def Chooser {
		exhibit state pick {
			entry; then start;
			state start;
			state first;
			state second;
			transition to_first first start accept Go then first;
			transition to_second first start accept Go then second;
			transition back_first first first accept Go then start;
			transition back_second first second accept Go then start;
		}
	}
	part chooser : Chooser;
`

// rescheduleRun instantiates the chooser and answers a function posting Go and
// advancing the clock, which reports the state the machine is then in.
func rescheduleRun(t *testing.T) (*Context, func() string) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "chooser.sysml", parseAndBuild(t, rescheduleSource))
	root := idx.DocumentRoot("chooser.sysml")
	chooser, err := ctx.Instantiate(resolveSymbol(t, root, "chooser"))
	if err != nil {
		t.Fatalf("Instantiate(chooser): %v", err)
	}
	machines := chooser.ExhibitedStates()
	if len(machines) != 1 || machines[0].State == nil {
		t.Fatalf("chooser exhibits %d machines; want pick", len(machines))
	}
	goSym := resolveSymbol(t, root, "Go")
	return ctx, func() string {
		t.Helper()
		msg, err := ctx.SignalMessage(goSym, nil, chooser)
		if err != nil {
			t.Fatalf("SignalMessage(Go): %v", err)
		}
		ctx.PostMessage(msg)
		if _, err := ctx.Advance(1); err != nil {
			t.Fatalf("Advance: %v", err)
		}
		return activeStateNames(machines[0].State)
	}
}

// seedGoingSecond is a seed whose first `draws` draws each take the machine to
// `second`.
func seedGoingSecond(t *testing.T, draws int) SchedulePolicy {
	t.Helper()
seeds:
	for seed := 0; seed < 256; seed++ {
		policy := mustPolicy(t, fmt.Sprintf("seed:%d", seed))
		ctx, fireGo := rescheduleRun(t)
		if err := ctx.Reschedule(policy); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < draws; i++ {
			if fireGo() != "second" {
				continue seeds
			}
			fireGo()
		}
		return policy
	}
	t.Fatalf("no seed under 256 draws to_second %d times running", draws)
	return SchedulePolicy{}
}

// Reschedule reaches the machine an object runs and the clock driving it: from
// the next step on they choose under the new policy, where they stand kept,
// while SetSchedule leaves them under the policy they started with.
func TestRescheduleReachesTheDrivenRuns(t *testing.T) {
	seeded := seedGoingSecond(t, 1)
	ctx, fireGo := rescheduleRun(t)
	if got := fireGo(); got != "first" {
		t.Fatalf("Go under the default reverse policy went to %s, want first", got)
	}
	if got := fireGo(); got != "start" {
		t.Fatalf("Go back went to %s, want start", got)
	}
	if err := ctx.SetSchedule(seeded); err != nil {
		t.Fatal(err)
	}
	if got := fireGo(); got != "first" {
		t.Fatalf("Go after SetSchedule(%s) went to %s, want first: a run under way keeps its policy", seeded, got)
	}
	if got := fireGo(); got != "start" {
		t.Fatalf("Go back went to %s, want start", got)
	}
	if err := ctx.Reschedule(seeded); err != nil {
		t.Fatal(err)
	}
	if got := fireGo(); got != "second" {
		t.Fatalf("Go after Reschedule(%s) went to %s, want second", seeded, got)
	}
	if got := fireGo(); got != "start" {
		t.Fatalf("Go back went to %s, want start", got)
	}
	if err := ctx.Reschedule(mustPolicy(t, "declared")); err != nil {
		t.Fatal(err)
	}
	if got := fireGo(); got != "first" {
		t.Fatalf("Go after Reschedule(declared) went to %s, want first", got)
	}
	if got := ctx.Clock().Now(); got != 7 {
		t.Errorf("clock after seven advances = %v, want 7: rescheduling keeps the clock", got)
	}
	if got := len(ctx.Choices()); got != 4 {
		t.Errorf("choices after four turns out of start = %d, want 4: rescheduling keeps the choices so far", got)
	}
}

// Restoring a snapshot taken before a Reschedule puts the runs under way back
// under the schedulers they had, where they had them, so the turns after it go
// as they would have.
func TestRescheduleIsUndoneWithTheSnapshot(t *testing.T) {
	seeded := seedGoingSecond(t, 2)
	ctx, fireGo := rescheduleRun(t)
	if err := ctx.Reschedule(seeded); err != nil {
		t.Fatal(err)
	}
	if got := fireGo(); got != "second" {
		t.Fatalf("Go under %s went to %s, want second", seeded, got)
	}
	if got := fireGo(); got != "start" {
		t.Fatalf("Go back went to %s, want start", got)
	}
	snapshot, err := ctx.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.Reschedule(mustPolicy(t, "declared")); err != nil {
		t.Fatal(err)
	}
	if got := fireGo(); got != "first" {
		t.Fatalf("Go after Reschedule(declared) went to %s, want first", got)
	}
	snapshot.Restore()
	if got := fireGo(); got != "second" {
		t.Errorf("Go after restoring the snapshot went to %s, want second: the second draw of %s", got, seeded)
	}
}

// A seed set by Reschedule starts its draws over, so a turn under it goes where
// a run started under that seed goes first.
func TestRescheduleStartsASeedOver(t *testing.T) {
	first := make(map[int]string)
	for seed := 0; seed < 8; seed++ {
		ctx, fireGo := rescheduleRun(t)
		if err := ctx.Reschedule(mustPolicy(t, fmt.Sprintf("seed:%d", seed))); err != nil {
			t.Fatal(err)
		}
		first[seed] = fireGo()
	}
	ctx, fireGo := rescheduleRun(t)
	fireGo()
	for seed := 0; seed < 8; seed++ {
		if got := fireGo(); got != "start" {
			t.Fatalf("Go back went to %s, want start", got)
		}
		if err := ctx.Reschedule(mustPolicy(t, fmt.Sprintf("seed:%d", seed))); err != nil {
			t.Fatal(err)
		}
		if got := fireGo(); got != first[seed] {
			t.Errorf("seed:%d set after earlier turns went to %s; a run started under it goes to %s", seed, got, first[seed])
		}
	}
}

// Reschedule refuses an exploration as SetSchedule does, and refuses to change a
// run from inside one of its steps.
func TestRescheduleRefusals(t *testing.T) {
	ctx, _ := rescheduleRun(t)
	if err := ctx.Reschedule(mustPolicy(t, "explore")); !errors.Is(err, ErrExploreUndriven) {
		t.Errorf("Reschedule(explore) = %v, want ErrExploreUndriven", err)
	}
	leave := ctx.beginRun()
	err := ctx.Reschedule(mustPolicy(t, "declared"))
	leave()
	if !errors.Is(err, ErrRescheduleMidRun) {
		t.Errorf("Reschedule inside a run = %v, want ErrRescheduleMidRun", err)
	}
}
