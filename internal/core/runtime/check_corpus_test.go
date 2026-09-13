package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CheckExpected is a `.check.expected.json` beside a conformance case whose
// oracle entry leaves an ordering open: what a check of every schedule finds,
// derived from the library text.
type CheckExpected struct {
	// Verdict is the check's verdict as CheckVerdict spells it.
	Verdict string `json:"verdict"`
	// Divergent lists every feature the schedule decides with every final value it
	// takes, in canonical order; a feature diverging unlisted fails the case.
	Divergent map[string][]string `json:"divergent,omitempty"`
	// Agreed lists features every schedule leaves with one value, and that value.
	Agreed map[string]string `json:"agreed,omitempty"`
}

// checkRefusedCases are the action cases with an admissible set the check
// refuses with a typed reason: a body paused mid-statement, or a state machine's
// transition due beside the action's token — both later stages' constructs.
var checkRefusedCases = map[string]error{
	"action_explore_performed_and_accept_due_together": ErrSnapshotPausedBody,
	"clock_action_state_due_together":                  ErrCheckRefused,
}

// checkCase is one action conformance case with an admissible set, ready to check and explore.
type checkCase struct {
	name     string
	expected ExpectedOutcome
	model    *exploreModel
	sym      *symbols.Symbol
	run      func(*Context) (Outcome, error)
}

// checkCorpus loads every action case of the conformance corpus whose oracle
// entry leaves an ordering open — those with an admissible set — and every one
// with a check expectation beside it.
func checkCorpus(t *testing.T) []checkCase {
	t.Helper()
	dir := filepath.Join("testdata", "conformance")
	knownFailures := loadKnownFailures(t, dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var cases []checkCase
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".expected.json") || strings.HasSuffix(entry.Name(), ".check.expected.json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".expected.json")
		if knownFailures[name] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var expected ExpectedOutcome
		if err := json.Unmarshal(data, &expected); err != nil {
			t.Fatalf("%s: %v", entry.Name(), err)
		}
		if expected.Type != "action" || (len(expected.Outcomes) == 0 && !hasCheckExpected(name)) {
			continue
		}
		path := filepath.Join(dir, name+".sysml")
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := source.New(path, text)
		p := parser.New(src)
		file := p.ParseFile()
		idx := symbols.NewIndex()
		if expected.Libraries {
			idx = libs.NewModelIndex()
		}
		idx.AddDocument(path, file)
		if expected.Libraries {
			idx.ExpandWildcardImports()
		}
		resolver := resolve.New(idx)
		sem := semantics.NewModel(resolver)
		sem.SetSourceText(source.TextOf(map[string]*source.SourceFile{path: src}, nil))
		model := &exploreModel{idx: idx, model: sem, resolver: resolver, path: path}
		sym := namedOrFoundSymbol(t, idx, expected.Evaluate, idx.DocumentRoot(path), ast.DefAction, ast.UsageAction)
		cases = append(cases, checkCase{name: name, expected: expected, model: model, sym: sym, run: conformanceRun(t, idx, path, expected)})
	}
	if len(cases) == 0 {
		t.Fatal("no action case with an admissible set")
	}
	return cases
}

func hasCheckExpected(name string) bool {
	_, err := os.Stat(filepath.Join("testdata", "conformance", name+".check.expected.json"))
	return err == nil
}

// loadCheckExpected reads the case's `.check.expected.json`; a case leaving an
// ordering open owes one.
func loadCheckExpected(t *testing.T, name string) CheckExpected {
	t.Helper()
	path := filepath.Join("testdata", "conformance", name+".check.expected.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("a case with an admissible set states what a check finds: %v", err)
	}
	var expected CheckExpected
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if expected.Verdict == "" {
		t.Fatalf("%s: no verdict", path)
	}
	return expected
}

// check checks the case's action under the default bounds.
func (c checkCase) check(t *testing.T, opts CheckOptions) (*CheckReport, error) {
	t.Helper()
	return CheckAction(context.Background(), c.model.fresh, starterOf(c.sym), CheckBudget{}, opts, nil)
}

// checked is check with the report owed.
func (c checkCase) checked(t *testing.T, opts CheckOptions) *CheckReport {
	t.Helper()
	report, err := c.check(t, opts)
	if err != nil {
		t.Fatalf("reduce=%v: check: %v", opts.Reduce, err)
	}
	return report
}

// explore explores the case's run to its complete table under the case's budget.
func (c checkCase) explore(t *testing.T) *Exploration {
	t.Helper()
	policy, err := ExplorePolicy(c.expected.ExploreBudget.budget())
	if err != nil {
		t.Fatal(err)
	}
	x, err := Explore(context.Background(), policy, c.model.fresh, c.run)
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	if !x.Complete() {
		t.Fatalf("exploration %s, want complete as the conformance harness has it", x.Status())
	}
	return x
}

// Every case with an admissible set is checked against its `.check.expected.json`,
// reduced and unreduced alike: the verdict, every divergent feature with its values,
// every agreed feature with its one value; a refused case is refused as it says.
func TestCheckConformanceOracles(t *testing.T) {
	for _, c := range checkCorpus(t) {
		t.Run(c.name, func(t *testing.T) {
			if want, refused := checkRefusedCases[c.name]; refused {
				_, err := c.check(t, reduced())
				if !errors.Is(err, want) {
					t.Fatalf("check = %v, want the refusal %v", err, want)
				}
				if hasCheckExpected(c.name) {
					t.Fatal("a refused case states no check expectation")
				}
				return
			}
			want := loadCheckExpected(t, c.name)
			for _, opts := range []CheckOptions{reduced(), unreduced()} {
				report := c.checked(t, opts)
				if report.Verdict.String() != want.Verdict {
					t.Fatalf("reduce=%v: verdict %s, want %s (violations %v)", opts.Reduce, report.Status(), want.Verdict, report.Violations)
				}
				for _, d := range report.Divergent {
					if _, listed := want.Divergent[d.Feature]; !listed {
						t.Fatalf("reduce=%v: %s diverges over %v, which the expectation does not list", opts.Reduce, d.Feature, divergentValues(report, d.Feature))
					}
				}
				for feature, values := range want.Divergent {
					if got := divergentValues(report, feature); strings.Join(got, "\x00") != strings.Join(values, "\x00") {
						t.Fatalf("reduce=%v: %s diverges over %v, want %v", opts.Reduce, feature, got, values)
					}
				}
				for feature, value := range want.Agreed {
					for _, final := range report.Finals {
						got, held := final.Values[feature]
						if !held || got != value {
							t.Fatalf("reduce=%v: %s = %q at %s, want %q on every schedule", opts.Reduce, feature, got, final.Outcome, value)
						}
					}
				}
			}
		})
	}
}

// Every case's final states are exactly the outcomes of explore's complete table,
// reduced and unreduced: explore is the referee of the check.
func TestCheckAgreesWithExploreOverTheConformanceCorpus(t *testing.T) {
	for _, c := range checkCorpus(t) {
		if _, refused := checkRefusedCases[c.name]; refused {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			want := explored(t, c.explore(t))
			for _, opts := range []CheckOptions{reduced(), unreduced()} {
				report := c.checked(t, opts)
				if len(report.Violations) != 0 || len(report.BoundsHit) != 0 {
					t.Fatalf("reduce=%v: %s, violations %v, want a clean complete search", opts.Reduce, report.Status(), report.Violations)
				}
				got := finalOutcomes(report)
				if strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Fatalf("reduce=%v: check finals\n%s\nexplore outcomes\n%s", opts.Reduce, strings.Join(got, "\n"), strings.Join(want, "\n"))
				}
			}
		})
	}
}

// explored is the outcome set an exploration reached, sorted as the check sorts its finals.
func explored(t *testing.T, x *Exploration) []string {
	t.Helper()
	out := make([]string, 0, len(x.Outcomes))
	for _, o := range x.Outcomes {
		if o.Outcome.Err != nil {
			t.Fatalf("exploration reached an error: %v", o.Outcome.Err)
		}
		out = append(out, o.Outcome.String())
	}
	sort.Strings(out)
	return out
}

// Every witness the check writes for the corpus replays to the state it claims:
// a final's run completes with its outcome, a divergent value's run leaves the
// feature with that value, and the replayed trace equals the witness's.
func TestCheckWitnessesReplayOverTheConformanceCorpus(t *testing.T) {
	for _, c := range checkCorpus(t) {
		if _, refused := checkRefusedCases[c.name]; refused {
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			report := c.checked(t, reduced())
			start := starterOf(c.sym)
			witnessed := 0
			for _, final := range report.Finals {
				r := replayWitness(t, c.model, start, final.Witness, final.Outcome)
				if r.Exec.State() != StateCompleted {
					t.Fatalf("%s: replay ends %s, want completed", final.Outcome, r.Exec.State())
				}
				if got := r.Ctx.ActionOutcome(r.Exec.Results()).String(); !strings.HasPrefix(final.Outcome, got) {
					t.Fatalf("replay reaches %s, want %s", got, final.Outcome)
				}
				witnessed++
			}
			for _, d := range report.Divergent {
				for _, v := range d.Values {
					r := replayWitness(t, c.model, start, v.Witness, d.Feature+" = "+v.Value)
					if got := replayedValue(r, d.Feature); got != v.Value {
						t.Fatalf("replay leaves %s = %s, want %s", d.Feature, got, v.Value)
					}
					witnessed++
				}
			}
			if witnessed == 0 {
				t.Fatal("no witness to replay")
			}
		})
	}
}

// replayedValue is the value a replayed run left the action's feature with.
func replayedValue(r *Replayed, feature string) string {
	for _, out := range r.Ctx.ActionOutcome(r.Exec.Results()).RenderedOutputs() {
		if out.Name == feature {
			return out.Text
		}
	}
	return UnsetText
}
