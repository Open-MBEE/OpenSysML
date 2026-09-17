package runtime

import (
	"path/filepath"
	"strings"
	"testing"
)

var earlyRaceLongTailPath = filepath.Join("testdata", "conformance", "action_explore_early_race_long_tail.sysml")

// The plan varies every choice point of the first run once, earliest first, before it varies
// any twice: an open choice met early, followed by many closed ones, is varied by the second
// run whatever the tails below it fan out to, on one job as on eight.
func TestExploreVariesEveryChoiceOfTheFirstRunFirst(t *testing.T) {
	idx, path := indexFile(t, earlyRaceLongTailPath)
	run := actionRun(t, idx, path, "race")
	fresh := newExploreWorkers(idx).fresh

	complete := exploreBoth(t, DefaultExploreBudget, fresh, run)
	if !complete.Complete() || complete.Runs != 20 || len(complete.Outcomes) != 2 {
		t.Fatalf("status %q with %d outcomes, want complete after the 20 orders of the two branches", complete.Status(), len(complete.Outcomes))
	}
	first, other := complete.Outcomes[1], complete.Outcomes[0]
	if first.WitnessRun != 1 {
		first, other = other, first
	}
	if first.WitnessRun != 1 || first.Outcome.String() != "p = 2; q = 2; x = 2" {
		t.Fatalf("first run reached %s as run %d, want x = 2 as run 1", first.Outcome, first.WitnessRun)
	}
	if len(first.Witness) < 2 || first.Witness[0].Alternatives != 2 {
		t.Fatalf("first run's witness %s, want the write order then the tails", FormatChoices(first.Witness))
	}
	if other.WitnessRun != 2 || other.Outcome.String() != "p = 2; q = 2; x = 1" {
		t.Fatalf("%s reached first by run %d, want x = 1 by run 2, the first run's earliest choice varied", other.Outcome, other.WitnessRun)
	}
	for _, o := range complete.Outcomes {
		if o.Linearizations != 10 {
			t.Errorf("%s reached by %d linearizations, want 10", o.Outcome, o.Linearizations)
		}
	}

	enough := exploreBoth(t, ExploreBudget{Runs: 2, Depth: 64}, fresh, run)
	if enough.Complete() || len(enough.Outcomes) != 2 || strings.Join(enough.BudgetsHit, ",") != "runs" {
		t.Fatalf("status %q with %d outcomes under 2 runs, want both values at the runs cut", enough.Status(), len(enough.Outcomes))
	}
	short := exploreBoth(t, ExploreBudget{Runs: 1, Depth: 64}, fresh, run)
	if short.Complete() || len(short.Outcomes) != 1 {
		t.Fatalf("status %q with %d outcomes under 1 run, want the first run's value alone", short.Status(), len(short.Outcomes))
	}
}
