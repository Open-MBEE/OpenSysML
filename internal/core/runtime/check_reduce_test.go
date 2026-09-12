package runtime

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var updateCheckReduction = flag.Bool("update-check-reduction", false, "Rewrite the reduction ratchet of testdata/check")

// The reduction corpus: one small model per dependence clause of the checker's
// static reduction, and one whose branches are independent.
var reductionCorpus = []struct{ file, action string }{
	{"por_shared_write", "race"},
	{"por_guard_read", "gated"},
	{"por_trigger_read", "monitor"},
	{"por_send_accept", "communicator"},
	{"por_join", "gather"},
	{"por_dynamic_target", "dynamic"},
	{"por_independent_branches", "parallel"},
	{"por_alias", "aliased"},
	{"por_address", "addressed"},
}

const reductionExpected = "testdata/check/reduction_expected.txt"

func reductionModel(t *testing.T, file string) *exploreModel {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "check", file+".sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return parseExploreModel(t, string(text))
}

func finalOutcomes(report *CheckReport) []string {
	outcomes := make([]string, len(report.Finals))
	for i, f := range report.Finals {
		outcomes[i] = f.Outcome
	}
	sort.Strings(outcomes)
	return outcomes
}

// Soundness: the reduced search reaches exactly the final states of the full one
// on every clause of the dependence relation, and every final is witnessed.
func TestCheckReductionIsSound(t *testing.T) {
	for _, c := range reductionCorpus {
		t.Run(c.file, func(t *testing.T) {
			m := reductionModel(t, c.file)
			with := checkModel(t, m, c.action, CheckBudget{}, reduced())
			without := checkModel(t, m, c.action, CheckBudget{}, unreduced())
			if with.Verdict == CheckViolation || without.Verdict == CheckViolation {
				t.Fatalf("violations: reduced %v, unreduced %v", with.Violations, without.Violations)
			}
			if len(with.BoundsHit) != 0 || len(without.BoundsHit) != 0 {
				t.Fatalf("bounds hit: reduced %v, unreduced %v", with.BoundsHit, without.BoundsHit)
			}
			got, want := finalOutcomes(with), finalOutcomes(without)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("reduced finals:\n%s\nunreduced finals:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if len(want) < 1 {
				t.Fatal("no final state")
			}
			if with.States > without.States || with.Moves > without.Moves {
				t.Fatalf("reduced search did more: %d states, %d moves against %d, %d",
					with.States, with.Moves, without.States, without.Moves)
			}
			start := starterOf(m.action(t, c.action))
			for _, final := range with.Finals {
				replayWitness(t, m, start, final.Witness, final.Outcome)
			}
		})
	}
}

// Effectiveness: the state and move counts of the reduced search over the
// corpus are pinned as a ratchet; every movement is adjudicated.
func TestCheckReductionRatchet(t *testing.T) {
	got := make(map[string]string, len(reductionCorpus))
	for _, c := range reductionCorpus {
		m := reductionModel(t, c.file)
		with := checkModel(t, m, c.action, CheckBudget{}, reduced())
		without := checkModel(t, m, c.action, CheckBudget{}, unreduced())
		got[c.file] = fmt.Sprintf("%d\t%d\t%d\t%d", with.States, with.Moves, without.States, without.Moves)
	}
	if *updateCheckReduction {
		var b strings.Builder
		b.WriteString("# Reduced states, reduced moves, unreduced states, unreduced moves per model of\n")
		b.WriteString("# the reduction corpus. Regenerate with -update-check-reduction and adjudicate\n")
		b.WriteString("# every movement: a reduced count rising loses reduction, one falling must keep\n")
		b.WriteString("# TestCheckReductionIsSound passing.\n")
		for _, c := range reductionCorpus {
			fmt.Fprintf(&b, "%s\t%s\n", got[c.file], c.file)
		}
		if err := os.WriteFile(reductionExpected, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	content, err := os.ReadFile(reductionExpected)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with -update-check-reduction)", reductionExpected, err)
	}
	want := make(map[string]string)
	for i, line := range strings.Split(string(content), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		at := strings.LastIndex(line, "\t")
		if at < 0 {
			t.Fatalf("%s:%d: want counts, a tab and the model, got %q", reductionExpected, i+1, line)
		}
		for _, n := range strings.Split(line[:at], "\t") {
			if _, err := strconv.Atoi(n); err != nil {
				t.Fatalf("%s:%d: bad count: %v", reductionExpected, i+1, err)
			}
		}
		want[line[at+1:]] = line[:at]
	}
	for _, c := range reductionCorpus {
		if want[c.file] != got[c.file] {
			t.Errorf("%s: counts %q, pinned %q; adjudicate and regenerate with -update-check-reduction",
				c.file, got[c.file], want[c.file])
		}
	}
	if len(want) != len(reductionCorpus) {
		t.Errorf("%s pins %d models, the corpus has %d", reductionExpected, len(want), len(reductionCorpus))
	}
}

// A write that redirects a send's address is dependent on the send: the reduced
// search reaches the message at each node the address may name, as the full one does.
func TestCheckReductionKeepsEveryAddressee(t *testing.T) {
	m := reductionModel(t, "por_address")
	addressed := func(node string) CheckProperty {
		return CheckProperty{Name: node + " unaddressed", Holds: func(ctx *Context, _ *ActionExecutor) (bool, error) {
			for _, msg := range ctx.PendingMessages() {
				if inst, held := ctx.instances[msg.Object]; held && symbolText(inst.Type) == node {
					return false, nil
				}
			}
			return true, nil
		}}
	}
	violated := func(report *CheckReport) []string {
		names := make([]string, 0, len(report.Violations))
		for _, v := range report.Violations {
			names = append(names, v.Name)
		}
		sort.Strings(names)
		return names
	}
	with := checkModel(t, m, "addressed", CheckBudget{}, reduced(), addressed("alpha"), addressed("beta"))
	without := checkModel(t, m, "addressed", CheckBudget{}, unreduced(), addressed("alpha"), addressed("beta"))
	want := []string{"alpha unaddressed", "beta unaddressed"}
	if got := violated(without); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unreduced violations %v, want %v", got, want)
	}
	if got := violated(with); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("reduced violations %v, want %v", got, want)
	}
	if len(with.BoundsHit) != 0 || with.States > without.States {
		t.Fatalf("reduced %s against unreduced %s", with.Status(), without.Status())
	}
}
