package smt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// Layer 5 of the test contract: schedule sensitivity over the conformance corpus,
// every expectation read from the case's oracle outcomes, never from the checker.

// conformance indexes a conformance case's model over the standard library, as
// the surfaces do.
func conformance(t *testing.T, name string) *document {
	t.Helper()
	path := filepath.Join(conformanceDir, name+".sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return indexed(t, path, string(src))
}

// sensitive is the Sensitive question about the behavior fqn names, comparing the
// features named across schedules; none compares every feature of the action.
func (d *document) sensitive(t *testing.T, fqn string, features ...string) analysis.Question {
	t.Helper()
	q := d.holds(t, fqn, "")
	q.Kind = analysis.Sensitive
	q.Holds.Diverge = features
	return q
}

// oracleValue is one typed value of a case's expectation.
type oracleValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// spell renders the value as the interpreter spells a final value, so the two
// compare as text.
func (v oracleValue) spell(t *testing.T) string {
	t.Helper()
	var value runtime.Value
	switch v.Type {
	case "Integer", "Natural", "Positive":
		var n int64
		if err := json.Unmarshal(v.Value, &n); err != nil {
			t.Fatalf("oracle %s %s: %v", v.Type, v.Value, err)
		}
		value = runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	case "Real":
		var f float64
		if err := json.Unmarshal(v.Value, &f); err != nil {
			t.Fatalf("oracle Real %s: %v", v.Value, err)
		}
		value = runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
	case "Boolean":
		var b bool
		if err := json.Unmarshal(v.Value, &b); err != nil {
			t.Fatalf("oracle Boolean %s: %v", v.Value, err)
		}
		value = runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}
	case "String":
		var str string
		if err := json.Unmarshal(v.Value, &str); err != nil {
			t.Fatalf("oracle String %s: %v", v.Value, err)
		}
		value = runtime.NewStringValue(str)
	default:
		t.Fatalf("oracle value of type %s has no spelling here", v.Type)
	}
	return runtime.FormatValue(value)
}

// oracleOutcome is the part of one listed outcome the sensitivity checks read.
type oracleOutcome struct {
	Outputs map[string]oracleValue `json:"outputs"`
}

// oracle is the final values a case's expectation gives each of the action's own
// features, over every outcome it lists.
type oracle map[string]map[string]bool

// oracleOf reads the case's expectation: its outcomes, or the one outputs map of
// a case with one outcome.
func oracleOf(t *testing.T, name string) oracle {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(conformanceDir, name+".expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		Outputs  map[string]oracleValue `json:"outputs"`
		Outcomes []json.RawMessage      `json:"outcomes"`
	}
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(expected.Outcomes) > 0 {
		return oracleOutcomes(t, name, expected.Outcomes)
	}
	one, err := json.Marshal(oracleOutcome{Outputs: expected.Outputs})
	if err != nil {
		t.Fatal(err)
	}
	return oracleOutcomes(t, name, []json.RawMessage{one})
}

// oracleOutcomes collects the values the listed outcomes give each root feature.
func oracleOutcomes(t *testing.T, name string, outcomes []json.RawMessage) oracle {
	t.Helper()
	o := make(oracle)
	for _, raw := range outcomes {
		var outcome oracleOutcome
		if err := json.Unmarshal(raw, &outcome); err != nil {
			t.Fatalf("parse an outcome of %s: %v", name, err)
		}
		for feature, v := range outcome.Outputs {
			if strings.Contains(feature, ".") {
				continue
			}
			if o[feature] == nil {
				o[feature] = make(map[string]bool)
			}
			o[feature][v.spell(t)] = true
		}
	}
	if len(o) == 0 {
		t.Fatalf("%s: the oracle lists no outputs", name)
	}
	return o
}

// values are the final values the oracle gives the feature, sorted.
func (o oracle) values(t *testing.T, feature string) []string {
	t.Helper()
	if o[feature] == nil {
		t.Fatalf("the oracle gives %s no value", feature)
	}
	return sortedKeys(o[feature])
}

// replayedValues re-runs both witnesses of a sensitivity through the interpreter to
// their end, as the engine did before claiming it, and reads the feature's final
// value from each; a witness the interpreter cannot follow fails the test.
func replayedValues(t *testing.T, d *document, q analysis.Question, result analysis.Result, feature string) []string {
	t.Helper()
	if result.Witness == nil || result.Contrast == nil {
		t.Fatalf("a sensitivity with witnesses %v and %v, want two", result.Witness, result.Contrast)
	}
	budget := analysis.Budget{Depth: DefaultMoves}
	var values []string
	for i, w := range []*analysis.Witness{result.Witness, result.Contrast} {
		replayed, err := runtime.ReplaySchedule(context.Background(), d.fresh(budget), runtime.ActionStarter(q.Holds.Start),
			runtime.Witness{Inputs: w.Inputs, Choices: w.Choices}, runtime.ScheduleEnd)
		if err != nil {
			t.Fatalf("witness %s does not replay: %v\n  %s", CopyNames[i], err, runtime.FormatChoices(w.Choices))
		}
		if replayed.Err != nil || replayed.Exec.State() != runtime.StateCompleted {
			t.Fatalf("witness %s replays to %v in state %v, want a completion", CopyNames[i], replayed.Err, replayed.Exec.State())
		}
		value, err := replayed.FinalValue(feature)
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// expectSensitive checks a sensitivity claimed over the feature: witnessed, its two
// witnesses replaying to the two values the oracle lists, and its reason spelling
// both values and the parting of the two schedules.
func expectSensitive(t *testing.T, d *document, q analysis.Question, result analysis.Result, feature string, o oracle) {
	t.Helper()
	expect(t, result, analysis.ClaimSensitive, analysis.Witnessed)
	want := o.values(t, feature)
	if len(want) != 2 {
		t.Fatalf("the oracle gives %s %d values, want two for a sensitive feature", feature, len(want))
	}
	if got := replayedValues(t, d, q, result, feature); !slices.Equal(got, want) {
		t.Errorf("the witnesses replay %s to %v, the oracle lists %v", feature, got, want)
	}
	for _, value := range want {
		if !strings.Contains(result.Reason, value) {
			t.Errorf("reason %q does not spell the value %s", result.Reason, value)
		}
	}
	if !strings.Contains(result.Reason, feature+" ends as ") || !strings.Contains(result.Reason, "the schedules part at step ") {
		t.Errorf("reason %q does not spell the pair", result.Reason)
	}
	if a, b := runtime.FormatChoices(result.Witness.Choices), runtime.FormatChoices(result.Contrast.Choices); a == b {
		t.Errorf("the two witnesses make the same choices: %s", a)
	}
}

// expectInsensitive checks a feature proved not sensitive: the oracle gives it one
// value, and the engine's answer is the proved negative with no witness.
func expectInsensitive(t *testing.T, result analysis.Result, feature string, o oracle) {
	t.Helper()
	if want := o.values(t, feature); len(want) != 1 {
		t.Fatalf("the oracle gives %s %d values, want one for a feature that is not sensitive", feature, len(want))
	}
	expect(t, result, analysis.ClaimHolds, analysis.Proved)
	if result.Witness != nil || result.Contrast != nil {
		t.Errorf("a proved negative with witnesses %v, %v", result.Witness, result.Contrast)
	}
	if at := slices.IndexFunc(result.Values, func(v analysis.Evaluation) bool { return v.Name == feature }); at < 0 || result.Values[at].Solved == nil {
		t.Errorf("the answer lists no solver result for %s: %+v", feature, result.Values)
	}
}

// TestSensitivityOfForkBranchesWritingOneFeature: the two branches write x, so x is
// sensitive with both witnesses replaying to the oracle's two values; leftRan and
// rightRan end true on every schedule, so neither is, at proof.
func TestSensitivityOfForkBranchesWritingOneFeature(t *testing.T) {
	const name = "action_fork_branches_write_one_feature"
	e := engine(t)
	d, o := conformance(t, name), oracleOf(t, name)
	budget := analysis.Budget{Depth: 8}

	q := d.sensitive(t, "test::clash", "x")
	result := answer(t, e, d, q, budget)
	expectSensitive(t, d, q, result, "x", o)
	if result.Witness.Written != "" || result.Contrast.Written != "" {
		t.Errorf("witnesses written to %q, %q with no directory asked", result.Witness.Written, result.Contrast.Written)
	}

	for _, feature := range []string{"leftRan", "rightRan"} {
		result := answer(t, e, d, d.sensitive(t, "test::clash", feature), budget)
		expectInsensitive(t, result, feature, o)
	}

	// Every feature at once: the sensitive one decides, the others are listed as solved.
	q = d.sensitive(t, "test::clash", "leftRan", "x", "rightRan")
	result = answer(t, e, d, q, budget)
	expectSensitive(t, d, q, result, "x", o)
	var listed []string
	for _, v := range result.Values {
		listed = append(listed, v.Name)
	}
	if !slices.Equal(listed, []string{"leftRan", "x"}) {
		t.Errorf("the answer lists %v, want the features asked up to the sensitive one", listed)
	}

	// No feature named: every feature of the action is compared.
	q = d.sensitive(t, "test::clash")
	expectSensitive(t, d, q, answer(t, e, d, q, budget), "x", o)
}

// TestSensitivityWritesBothWitnesses: with a directory named, each run of the pair is
// written under the copy's suffix, with the trace the interpreter left, and each
// replays through the `replay:` policy to its own value.
func TestSensitivityWritesBothWitnesses(t *testing.T) {
	const name = "action_fork_branches_write_one_feature"
	e := engine(t)
	d, o := conformance(t, name), oracleOf(t, name)
	q := d.sensitive(t, "test::clash", "x")
	q.Holds.WitnessDir = t.TempDir()
	result := answer(t, e, d, q, analysis.Budget{Depth: 8})
	expectSensitive(t, d, q, result, "x", o)
	var values []string
	for i, w := range []*analysis.Witness{result.Witness, result.Contrast} {
		want := filepath.Join(q.Holds.WitnessDir, analysis.SensitivityFile(q.Subject, "", "x", CopyNames[i]))
		if w.Written != want {
			t.Fatalf("witness %s written to %q, want %q", CopyNames[i], w.Written, want)
		}
		data, err := os.ReadFile(w.Written)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "\n\n") || !strings.Contains(string(data), "step 1: token 1@split") {
			t.Errorf("witness %s carries no trace:\n%s", CopyNames[i], data)
		}
		file, err := runtime.ParseWitness(string(data))
		if err != nil {
			t.Fatalf("witness %s does not read back: %v", CopyNames[i], err)
		}
		replayed, err := runtime.ReplayAction(context.Background(), d.fresh(analysis.Budget{Depth: 8}), runtime.ActionStarter(q.Holds.Start), file, nil)
		if err != nil {
			t.Fatalf("witness %s does not replay: %v", CopyNames[i], err)
		}
		value, err := replayed.FinalValue("x")
		if err != nil {
			t.Fatal(err)
		}
		values = append(values, value)
	}
	sort.Strings(values)
	if want := o.values(t, "x"); !slices.Equal(values, want) {
		t.Errorf("the written witnesses replay to %v, the oracle lists %v", values, want)
	}
}

// TestSensitivityShortOfCompletionIsBounded: at a depth short of the fork's
// completion no two completed schedules differ, but the bound cuts a live one, so
// the answer is the bounded negative, never a proof.
func TestSensitivityShortOfCompletionIsBounded(t *testing.T) {
	const name = "action_fork_branches_write_one_feature"
	e := engine(t)
	d := conformance(t, name)
	result := answer(t, e, d, d.sensitive(t, "test::clash", "x"), analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimHolds, analysis.Bounded)
	if want := fmt.Sprintf(NoSensitivityWithin, 3) + ": a schedule is still live after move 3 ("; !strings.HasPrefix(result.Reason, want) {
		t.Errorf("reason %q, want it to begin %q", result.Reason, want)
	}
	if result.Witness != nil || result.Contrast != nil {
		t.Errorf("a bounded negative with witnesses %v, %v", result.Witness, result.Contrast)
	}
	if bound(t, result, "moves").Reached != true {
		t.Errorf("the moves bound is not reported reached: %+v", result.Bounds)
	}
}

// TestSensitivityOfJoinWaitingForSlowestBranch: every schedule counts three arrivals
// before the join lets `after` read them, so `arrived` is not sensitive, at proof.
func TestSensitivityOfJoinWaitingForSlowestBranch(t *testing.T) {
	const name = "action_join_waits_for_slowest_branch"
	e := engine(t)
	d, o := conformance(t, name), oracleOf(t, name)
	for _, feature := range []string{"arrived", "seen"} {
		result := answer(t, e, d, d.sensitive(t, "test::gather", feature), analysis.Budget{Depth: 12})
		expectInsensitive(t, result, feature, o)
	}
}

// TestSensitivityUnderDeadlockIsTheDeadlock: a join no schedule passes is reported as
// the deadlock, the feature having no final value, never as not sensitive.
func TestSensitivityUnderDeadlockIsTheDeadlock(t *testing.T) {
	e := engine(t)
	d := indexed(t, "stuck.sysml", `package test {
	private import ScalarValues::*;
	action def Stuck {
		attribute x : Integer = 0;
		first start;
		action stranded { assign x := 1; }
		join j;
		done;
		succession first start then j;
		succession first stranded then j;
		succession first j then done;
	}
}`)
	result := answer(t, e, d, d.sensitive(t, "test::Stuck", "x"), analysis.Budget{Depth: 6})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	if len(result.Values) != 1 || !errors.Is(result.Values[0].Err, runtime.ErrActionDeadlock) {
		t.Fatalf("the deadlock is not reported: %+v", result.Values)
	}
	if result.Contrast != nil {
		t.Errorf("a deadlock with a second witness: %v", result.Contrast)
	}
}

// TestSensitivityRefusesTheClockAndPausedFlows: the case whose branches wait on the
// clock, one inside a performed action, is outside this encoding; the answer is the
// typed refusal naming the first construct met, the performed action, not a verdict.
func TestSensitivityRefusesTheClockAndPausedFlows(t *testing.T) {
	const name = "action_explore_performed_and_accept_due_together"
	e := engine(t)
	d := conformance(t, name)
	result := answer(t, e, d, d.sensitive(t, "test::wake", "x"), analysis.Budget{Depth: 8})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(result.Reason, ErrNotEncoded.Error()) || !strings.Contains(result.Reason, "node performed: action invocation") {
		t.Errorf("reason %q does not name the construct and its node", result.Reason)
	}
	if result.Witness != nil || len(result.Values) != 0 {
		t.Errorf("a refusal with witness %v and values %+v", result.Witness, result.Values)
	}
}

// TestSensitivityRefusesTheListWithANestedFeature: a feature a node's performance
// holds is refused beside the action's own, which is still asked; the negative over
// the list is not covered, naming the refused feature, never a proof over both.
func TestSensitivityRefusesTheListWithANestedFeature(t *testing.T) {
	const src = `package test {
    private import ScalarValues::*;
    action A {
        attribute y : Integer = 0;
        first start;
        action inner { in n : Integer = 1; assign y := n; }
        done;
        succession first start then inner;
        succession first inner then done;
    }
}`
	e := engine(t)
	d := indexed(t, "nested.sysml", src)
	result := answer(t, e, d, d.sensitive(t, "test::A", "y", "inner.n"), analysis.Budget{Depth: 4})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if want := "inner.n: " + nestedReason; result.Reason != want {
		t.Errorf("reason %q, want %q", result.Reason, want)
	}
	if len(result.Values) != 2 || result.Values[0].Name != "y" || result.Values[0].Solved == nil || result.Values[0].Solved.Status != solve.StatusUnsat {
		t.Errorf("the action's feature is not listed as asked: %+v", result.Values)
	}
	if len(result.Values) != 2 || result.Values[1].Name != "inner.n" || result.Values[1].Err == nil {
		t.Errorf("the nested feature is not listed as refused: %+v", result.Values)
	}
}

// TestSensitivityRefusesAPairTheInterpreterRefutes: a diverging pair whose value the
// interpreter does not reproduce is not covered, with the disagreement, and no
// witness is written; the interpreter's run is the authority.
func TestSensitivityRefusesAPairTheInterpreterRefutes(t *testing.T) {
	const name = "action_fork_branches_write_one_feature"
	e := engine(t)
	d := conformance(t, name)
	q := d.sensitive(t, "test::clash", "x")
	q.Holds.WitnessDir = t.TempDir()
	solver, err := e.discover()
	if err != nil {
		t.Fatal(err)
	}
	r := &run{engine: e, model: d.model, q: q, budget: analysis.Budget{Depth: 8}, solver: solver,
		timeout: solve.DefaultTimeout, moves: 8, unroll: DefaultUnroll, started: time.Now()}
	if refusal, err := r.encode(); err != nil || refusal != nil {
		t.Fatalf("encode: %v %v", refusal, err)
	}
	out := r.encoding.Outputs()[slices.IndexFunc(r.encoding.Outputs(), func(o Output) bool { return o.Name == "x" })]
	query, err := r.encoding.Sensitivity(out)
	if err != nil {
		t.Fatal(err)
	}
	result, err := solver.Solve(context.Background(), query)
	if err != nil || result.Status != solve.StatusSat {
		t.Fatalf("the two-copy query answered %v, %v", result.Status, err)
	}
	pair, err := r.encoding.DecodePair(result, out)
	if err != nil {
		t.Fatal(err)
	}
	// The solver's copy B leaves x as it does; a pair claiming 7 there is refuted.
	pair.Values[1] = "7"
	answer, err := r.sensitive(context.Background(), pair, nil)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, answer, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(answer.Reason, "the solver claims x ends as 7 under schedule B") || !strings.Contains(answer.Reason, "the interpreter leaves it as ") {
		t.Errorf("reason %q does not report the disagreement", answer.Reason)
	}
	if answer.Witness == nil || answer.Contrast == nil || answer.Contrast.Written != "" {
		t.Errorf("witnesses %+v, %+v: want both reported, the refuted one with no file", answer.Witness, answer.Contrast)
	}
	files, err := os.ReadDir(q.Holds.WitnessDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("%d file(s) written, want copy A's alone", len(files))
	}
}

// TestSensitivityUndecidedFeatureDoesNotHideALaterOne: a solver that does not decide
// leftRan's query still gets asked x's, whose pair decides the question; with x not
// asked, the answer is the undecided query, not covered, never a negative.
func TestSensitivityUndecidedFeatureDoesNotHideALaterOne(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("no sh to stand in for a solver: %v", err)
	}
	const name = "action_fork_branches_write_one_feature"
	real := requireSolver(t)
	// The script reads up to check-sat, answers unknown to leftRan's query by the
	// comment its assertion carries, and hands any other dialogue to the real solver.
	script := `buf=$(mktemp); trap 'rm -f "$buf"' EXIT
while IFS= read -r line; do printf '%s\n' "$line" >> "$buf"; case "$line" in *"(check-sat)"*) break;; esac; done
if grep -q "different values of leftRan" "$buf"; then
  echo unknown
  while IFS= read -r line; do case "$line" in *reason-unknown*) echo '(:reason-unknown "declined")';; esac; done
  exit 0
fi
{ cat "$buf"; cat; } | exec "$@"`
	args := append([]string{"-c", script, "declining", real.Path}, real.Args...)
	declining := &solve.Solver{Name: "declining", Path: "sh", Args: args, Declared: real.Declared}
	e := New(func() (*solve.Solver, error) { return declining, nil })
	d, o := conformance(t, name), oracleOf(t, name)
	budget := analysis.Budget{Depth: 8}

	q := d.sensitive(t, "test::clash", "leftRan", "x")
	result := answer(t, e, d, q, budget)
	expectSensitive(t, d, q, result, "x", o)
	if len(result.Values) != 2 || result.Values[0].Name != "leftRan" || result.Values[0].Solved == nil || result.Values[0].Solved.Status != solve.StatusUnknown {
		t.Errorf("the undecided feature is not listed before the sensitive one: %+v", result.Values)
	}

	result = answer(t, e, d, d.sensitive(t, "test::clash", "leftRan", "rightRan"), budget)
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if want := "the solver did not decide whether two schedules end with different values of leftRan: declined"; result.Reason != want {
		t.Errorf("reason %q, want %q", result.Reason, want)
	}
	if len(result.Values) != 2 || result.Values[1].Name != "rightRan" || result.Values[1].Solved == nil || result.Values[1].Solved.Status != solve.StatusUnsat {
		t.Errorf("the features asked after the undecided one are not listed as solved: %+v", result.Values)
	}
}

// TestPairIsTheRelationTwiceOver: copy B declares every variable of the relation but
// state 0's under the prefix, asserts every assertion again over them with copy B's
// provenance, pins in B what A pins, and carries the query's sorts and theory flags.
func TestPairIsTheRelationTwiceOver(t *testing.T) {
	d := indexed(t, "loops.sysml", loopsSrc)
	for _, name := range []string{"test::Rounds", "test::Meet", "test::Divides"} {
		encoding, refusal := encodeDocument(t, d, lookup(t, d.idx, name), analysis.Budget{Depth: 8})
		if refusal != nil {
			t.Fatalf("%s refused: %v", name, refusal)
		}
		p, err := encoding.Pair()
		if err != nil {
			t.Fatal(err)
		}
		shared := make(map[string]bool)
		for _, v := range encoding.stateVars(encoding.States[0]) {
			shared[v.Name] = true
		}
		declared := make(map[string]*solve.Var, len(p.Query.Vars))
		for _, v := range p.Query.Vars {
			declared[v.Name] = v
		}
		renamed := 0
		for _, v := range encoding.Query.Vars {
			b, ok := p.Renamed[v.Name]
			if shared[v.Name] {
				if ok {
					t.Errorf("%s: shared %s is renamed", name, v.Name)
				}
				continue
			}
			renamed++
			if !ok || b.Name != CopyPrefix+v.Name || !reflect.DeepEqual(b.Sort, v.Sort) || b.Symbol != v.Symbol {
				t.Errorf("%s: %s copies as %+v", name, v.Name, b)
			}
			if declared[b.Name] != b {
				t.Errorf("%s: copy B's %s is not declared", name, b.Name)
			}
		}
		if renamed == 0 || len(p.Query.Vars) != len(encoding.Query.Vars)+renamed {
			t.Errorf("%s: %d variables, %d renamed, %d declared by the pair", name, len(encoding.Query.Vars), renamed, len(p.Query.Vars))
		}
		n := len(encoding.Query.Assertions)
		if len(p.Query.Assertions) != 2*n {
			t.Fatalf("%s: %d assertions in the pair over %d", name, len(p.Query.Assertions), n)
		}
		for i, a := range encoding.Query.Assertions {
			b := p.Query.Assertions[n+i]
			if b.From.Condition != "copy B: "+a.From.Condition || b.From.Kind != a.From.Kind || b.From.Role != a.From.Role {
				t.Errorf("%s: assertion %d copies with provenance %+v", name, i, b.From)
			}
			if !reflect.DeepEqual(b.Term, p.copy(a.Term)) {
				t.Errorf("%s: assertion %d is not the substitution of copy A's", name, i)
			}
			for _, v := range readVars(b.Term, nil) {
				if !shared[v.Name] && !strings.HasPrefix(v.Name, CopyPrefix) {
					t.Errorf("%s: copy B's assertion %d reads copy A's %s", name, i, v.Name)
				}
			}
		}
		if len(p.Query.Pinned) != 2*len(encoding.Query.Pinned) {
			t.Errorf("%s: %d values pinned in the pair over %d", name, len(p.Query.Pinned), len(encoding.Query.Pinned))
		}
		for i, pin := range encoding.Query.Pinned {
			b := p.Query.Pinned[len(encoding.Query.Pinned)+i]
			if b.Var != p.Renamed[pin.Var.Name] || b.Value != pin.Value || b.Source != pin.Source {
				t.Errorf("%s: pin of %s copies as %+v", name, pin.Var.Name, b)
			}
		}
		if !reflect.DeepEqual(p.Query.Sorts, encoding.Query.Sorts) || p.Query.Nonlinear != encoding.Query.Nonlinear ||
			p.Query.IntegerDivision != encoding.Query.IntegerDivision || p.Query.Element != encoding.Query.Element {
			t.Errorf("%s: the pair does not carry the query's sorts and flags", name)
		}
	}
}

// readVars collects the variables a term reads, parents first.
func readVars(t *solve.Term, into []*solve.Var) []*solve.Var {
	if t == nil {
		return into
	}
	if t.Var != nil {
		into = append(into, t.Var)
	}
	for _, arg := range t.Args {
		into = readVars(arg, into)
	}
	return into
}
