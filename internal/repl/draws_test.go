package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// %draws shows and sets how runs resolve their RandomFunctions draws; under min,
// max or average a %runs needs no seed and every run agrees, under random the
// seed is required, and a word that is no policy is refused.
func TestDrawsFixesTheRunsWithoutASeed(t *testing.T) {
	s := runsSession(t)
	wants(t, run(t, s, "%draws"), "draws: random")
	wants(t, run(t, s, "%runs 3 MC::acquire clock"), "the seed may be left out only under a fixed %draws policy")
	wants(t, run(t, s, "%draws max"), "draws: max")
	if s.Draws() != runtime.DrawMax {
		t.Errorf("Draws() = %s, want max", s.Draws())
	}
	got := sweepTable(run(t, s, "%runs 3 MC::acquire clock"))
	wants(t, got, "runs MC::acquire — 3 run(s), no seed",
		"1   | 80.0 [s] | <time>", "2   | 80.0 [s] | <time>", "3   | 80.0 [s] | <time>",
		"clock: 3 run(s), min 80.0 [s], mean 80.0 [s], max 80.0 [s]")
	if strings.Contains(got, "seed") && !strings.Contains(got, "no seed") {
		t.Errorf("a seedless table names a seed:\n%s", got)
	}
	// A seed given under a fixed policy still seeds the weighted decision.
	wants(t, sweepTable(run(t, s, "%runs 2 7 MC::acquire clock")), "runs MC::acquire — 2 run(s), seed 7", "80.0 [s]")
	wants(t, run(t, s, "%draws average"), "draws: average")
	wants(t, sweepTable(run(t, s, "%runs 2 MC::acquire clock tries")), "40.5 [s]", "| 2     |")
	wants(t, run(t, s, "%draws min"), "draws: min")
	wants(t, sweepTable(run(t, s, "%runs 1 MC::acquire clock tries")), "1.0 [s]", "| 1     |")
	wants(t, run(t, s, "%draws fastest"), `"fastest" is not one of random, min, max, average`)
	wants(t, run(t, s, "%draws random"), "draws: random")
	wants(t, run(t, s, "%runs 3 MC::acquire clock"), "the seed may be left out only under a fixed %draws policy")
}

// SetDraws fixes the draws of a run started at the prompt too: the debugger's
// run of a drawing action needs no seed under max and draws each maximum.
func TestSetDrawsAppliesToTheDebugger(t *testing.T) {
	s := runsSession(t)
	s.SetDraws(runtime.DrawMax)
	wants(t, run(t, s, "%action MC::acquire"), "Started action executor")
	wants(t, run(t, s, "%continue"), "Action completed", "tries = 3")
	s.SetDraws(runtime.DrawRandom)
	wants(t, run(t, s, "%action MC::acquire"), "modeled randomness needs a seed")
}
