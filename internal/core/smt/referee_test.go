package smt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The referee holds the encoding to the interpreter over every conformance action case with
// outcomes: same outcome set, every witness replays, same deadlock verdict; refusals are counted.
// Its fourth check, over the cases with free inputs: a violated witness with inputs, explored
// with those inputs pinned as a caller pins them, reproduces the violation. Its fifth, per
// feature of every case that completes: a sensitive verdict has both witnesses replayed and
// their two values among the case's outcomes; a feature not sensitive has one value there.

// corpusCase is the part of a conformance case's expectation the referee reads.
type corpusCase struct {
	Type          string            `json:"type"`
	Evaluate      string            `json:"evaluate"`
	Outcomes      []json.RawMessage `json:"outcomes"`
	ExploreBudget *struct {
		Runs  *int `json:"runs"`
		Depth *int `json:"depth"`
	} `json:"exploreBudget"`
}

// refereeCases are the action cases of the corpus that list outcomes, by name.
func refereeCases(t *testing.T) map[string]corpusCase {
	t.Helper()
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read %s: %v", conformanceDir, err)
	}
	cases := make(map[string]corpusCase)
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".expected.json")
		if !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(conformanceDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		var c corpusCase
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		if c.Type == "action" && len(c.Outcomes) > 0 {
			cases[name] = c
		}
	}
	if len(cases) == 0 {
		t.Fatal("no action case with outcomes in the corpus")
	}
	return cases
}

// corpusDocument indexes a conformance case over the standard library, which
// the encoding needs to sort its features, and finds the action it evaluates.
func corpusDocument(t *testing.T, name string, c corpusCase) (*document, *symbols.Symbol) {
	t.Helper()
	path := filepath.Join(conformanceDir, name+".sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	idx := libs.NewModelIndex()
	sf := source.New(path, src)
	idx.AddDocument(path, parser.New(sf).ParseFile())
	idx.ExpandWildcardImports()
	var action *symbols.Symbol
	if c.Evaluate != "" {
		action = lookup(t, idx, c.Evaluate)
	} else {
		action = firstAction(idx.DocumentRoot(path))
	}
	if action == nil {
		t.Fatalf("%s: no action to evaluate", name)
	}
	model := &analysis.Model{
		Semantics: func() (*runtime.Model, error) {
			resolver := resolve.New(idx)
			m := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
			m.RegisterSource(sf)
			return m, nil
		},
		Fresh: func(w *analysis.Worker) (*runtime.Context, error) {
			return runtime.NewContext(w.Model, 10000), nil
		},
	}
	return &document{idx: idx, model: model}, action
}

// firstAction is the first action definition or usage declared under scope,
// searched as the conformance test searches when a case names none.
func firstAction(scope *symbols.Scope) *symbols.Symbol {
	find := func(s *symbols.Scope) *symbols.Symbol {
		for _, name := range s.MemberNames() {
			sym, ok := s.LookupLocal(name)
			if !ok {
				continue
			}
			if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == ast.DefAction {
				return sym
			}
			if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsageAction {
				return sym
			}
		}
		return nil
	}
	for _, child := range scope.Children() {
		if sym := find(child); sym != nil {
			return sym
		}
	}
	return find(scope)
}

// refereeTally counts what the referee saw over the corpus.
type refereeTally struct {
	encoded, refused, agreeing, witnesses, replayed int
	// inputs counts the violated witnesses fixing inputs, reproduced those the
	// exploration with the inputs pinned reaches the violation of.
	inputs, reproduced int
	// features counts the features asked about across schedules, sensitive those
	// found sensitive with both witnesses replayed to two of the case's outcomes,
	// insensitive those found not sensitive with one value across its outcomes.
	features, sensitive, insensitive int
	refusals                         []string
}

// log reports the tally as the referee's columns.
func (tally *refereeTally) log(t *testing.T) {
	t.Helper()
	t.Logf("referee: %d cases encoded, %d refused, %d agreeing on outcomes and verdict; %d witnesses, %d replayed; %d witnesses with inputs, %d reproduced under explore; %d features compared across schedules, %d sensitive with both witnesses replayed, %d not sensitive",
		tally.encoded, tally.refused, tally.agreeing, tally.witnesses, tally.replayed, tally.inputs, tally.reproduced, tally.features, tally.sensitive, tally.insensitive)
	for _, refusal := range tally.refusals {
		t.Logf("refused: %s", refusal)
	}
}

// refereeTimeout is how long the referee gives the solver per query: it judges
// faithfulness, not speed, so a slow machine must not turn a verdict undecided.
const refereeTimeout = 5 * time.Minute

// TestRefereeCorpus runs checks 1–3 and 5 over the corpus and reports the counts.
func TestRefereeCorpus(t *testing.T) {
	solver := *requireSolver(t)
	solver.Timeout = refereeTimeout
	cases := refereeCases(t)
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	tally := &refereeTally{}
	for _, name := range names {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			refereeCase(t, &solver, name, c, tally)
		})
	}
	tally.log(t)
	if tally.encoded == 0 {
		t.Fatal("the referee encoded no case")
	}
}

// inputCase is one pinned model with free inputs the referee holds to the interpreter.
type inputCase struct {
	name, src, action, condition string
}

// inputCases are the free-input models of layer 4 whose verdict is violated: an
// Integer below the default, an Integer where Natural proves, an enumeration
// constructor, and a real.
func inputCases() []inputCase {
	realSrc := strings.Replace(freeSrc, "attribute u : Natural;", "attribute u : Real;", 1)
	return []inputCase{
		{"integer-input", freeSrc, "test::A", "test::A::positive"},
		{"integer-where-natural-proves", strings.Replace(freeSrc, "attribute u : Natural;", "attribute u : Integer;", 1), "test::A", "test::A::natural"},
		{"enumeration-input", freeSrc, "test::A", "test::A::fast"},
		{"real-input", realSrc, "test::A", "test::A::natural"},
	}
}

// TestRefereeInputs runs check 4 over the free-input models: every violated
// witness fixes its inputs, and exploring with them pinned reproduces the violation.
func TestRefereeInputs(t *testing.T) {
	solver := *requireSolver(t)
	solver.Timeout = refereeTimeout
	e := New(func() (*solve.Solver, error) { return &solver, nil })
	tally := &refereeTally{}
	for _, c := range inputCases() {
		t.Run(c.name, func(t *testing.T) {
			d := indexed(t, c.name+".sysml", c.src)
			q := d.holds(t, c.action, c.condition)
			budget := analysis.Budget{Depth: DefaultMoves, Solver: solver.Timeout}
			verdict := answer(t, e, d, q, budget)
			tally.encoded++
			expect(t, verdict, analysis.ClaimViolated, analysis.Witnessed)
			if len(verdict.Witness.Inputs) == 0 {
				t.Fatalf("the witness fixes no input: %v", verdict.Witness)
			}
			if reproduceViolation(t, d, q.Holds, budget, verdict, tally) {
				tally.agreeing++
			}
		})
	}
	tally.log(t)
	if tally.reproduced != len(inputCases()) {
		t.Errorf("%d of %d violations reproduced under explore", tally.reproduced, len(inputCases()))
	}
}

// refereeCase holds one case to the interpreter, adding to the tally.
func refereeCase(t *testing.T, solver *solve.Solver, name string, c corpusCase, tally *refereeTally) {
	d, action := corpusDocument(t, name, c)
	budget := analysis.Budget{Depth: DefaultMoves, Solver: solver.Timeout}
	encoding, refusal := encodeDocument(t, d, action, budget)
	if refusal != nil {
		tally.refused++
		tally.refusals = append(tally.refusals, name+": "+refusal.Error())
		t.Logf("refused: %v", refusal)
		return
	}
	tally.encoded++
	exploration := explore(t, c.exploreBudget(), d.fresh(budget), action)
	outcomes := compareOutcomes(t, solver, encoding, d, action, budget, exploration)
	tally.witnesses += outcomes.witnesses
	tally.replayed += outcomes.replayed
	ask := startAsk(action)
	agreeing, verdict := refereeVerdict(t, solver, d, ask, budget, exploration)
	if verdict.Claim == analysis.ClaimViolated && verdict.Witness != nil && len(verdict.Witness.Inputs) > 0 {
		agreeing = reproduceViolation(t, d, ask, budget, verdict, tally) && agreeing
	}
	if agreeing {
		tally.agreeing++
	}
	if verdict.Claim == analysis.ClaimHolds {
		r := refereed{d: d, encoding: encoding, ask: ask, budget: budget}
		refereeSensitivity(t, solver, r, oracleOutcomes(t, name, c.Outcomes), tally)
	}
}

// refereed is one corpus case as check 5 sees it: its document, the encoding of its
// action, the ask starting the action and the budget the queries run under.
type refereed struct {
	d        *document
	encoding *Encoding
	ask      *analysis.HoldsAsk
	budget   analysis.Budget
}

// refereeSensitivity runs check 5 over a case every schedule of which completes: each
// of the action's own features is asked about across schedules; a sensitive answer
// replays both witnesses to two distinct values the case's outcomes list, and a
// feature not sensitive has one value across them.
func refereeSensitivity(t *testing.T, solver *solve.Solver, r refereed, o oracle, tally *refereeTally) {
	t.Helper()
	e := New(func() (*solve.Solver, error) { return solver, nil })
	d, ask, budget := r.d, r.ask, r.budget
	for _, out := range r.encoding.Outputs() {
		feature := out.Name
		if o[feature] == nil {
			t.Errorf("the encoding holds %s, which the case's outcomes do not list", feature)
			continue
		}
		tally.features++
		q := analysis.Question{Kind: analysis.Sensitive, Subject: ask.Behavior.Name, Free: analysis.FreeSchedule,
			Holds: &analysis.HoldsAsk{Behavior: ask.Behavior, Start: ask.Start, Diverge: []string{feature}}}
		verdict := answer(t, e, d, q, budget)
		listed := sortedKeys(o[feature])
		switch verdict.Claim {
		case analysis.ClaimSensitive:
			if verdict.Strength != analysis.Witnessed || verdict.Witness == nil || verdict.Contrast == nil {
				t.Errorf("%s sensitive at %v with witnesses %v, %v", feature, verdict.Strength, verdict.Witness, verdict.Contrast)
				continue
			}
			values := replayedValues(t, d, q, verdict, feature)
			if values[0] == values[1] || !o[feature][values[0]] || !o[feature][values[1]] {
				t.Errorf("%s sensitive between %v, the case's outcomes list %v", feature, values, listed)
				continue
			}
			tally.sensitive++
			t.Logf("%s: sensitive between %v", feature, values)
		case analysis.ClaimHolds:
			if len(listed) != 1 {
				t.Errorf("%s not sensitive (%v: %s), the case's outcomes list %v", feature, verdict.Strength, verdict.Reason, listed)
				continue
			}
			tally.insensitive++
			t.Logf("%s: not sensitive (%v), %s throughout", feature, verdict.Strength, listed[0])
		default:
			t.Errorf("%s: the engine answers %v/%v where the exploration completes: %s", feature, verdict.Claim, verdict.Strength, verdict.Reason)
		}
	}
}

// startAsk asks after the action begun as the interpreter begins it, no condition named.
func startAsk(action *symbols.Symbol) *analysis.HoldsAsk {
	return &analysis.HoldsAsk{
		Behavior: action,
		Start: func(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
			return ctx.CreateActionExecutor(action)
		},
	}
}

// fresh builds one context of the document's model under the budget.
func (d *document) fresh(budget analysis.Budget) func() (*runtime.Context, error) {
	return func() (*runtime.Context, error) { return d.model.NewContextOn(0, budget) }
}

// encodeDocument encodes the action as the engine would, returning the refusal
// of a construct outside the stage, or failing on a fault.
func encodeDocument(t *testing.T, d *document, action *symbols.Symbol, budget analysis.Budget) (*Encoding, error) {
	t.Helper()
	ctx, err := d.fresh(budget)()
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	exec, err := ctx.CreateActionExecutor(action)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer exec.Release()
	encoding, err := Encode(ctx, action, exec.Graph(), exec.Held(), nil, budget.Depth, DefaultUnroll)
	if err != nil {
		refusal, fault := refusalOf(err)
		if fault != nil {
			t.Fatalf("encode: %v", fault)
		}
		return nil, refusal
	}
	return encoding, nil
}

// comparedOutcomes is what checks 1 and 2 found over one action.
type comparedOutcomes struct {
	explored, found     []string
	agreeing            bool
	witnesses, replayed int
}

// compareOutcomes runs checks 1 and 2: the outcomes the solver enumerates are
// the completed outcomes the exploration reached, and one witness per outcome
// replays to it with the witness's own trace.
func compareOutcomes(t *testing.T, solver *solve.Solver, encoding *Encoding, d *document, action *symbols.Symbol, budget analysis.Budget, exploration *runtime.Exploration) comparedOutcomes {
	t.Helper()
	fresh := d.fresh(budget)
	explored := make(map[string]string)
	for _, o := range exploration.Outcomes {
		if o.Outcome.Err == nil {
			explored[rootOutputs(o.Outcome)] = runtime.FormatChoices(o.Witness)
		}
	}

	completion := encoding.Completion()
	enumerated, err := solver.Enumerate(context.Background(), completion, encoding.OutputVars(), len(explored)+1)
	if err != nil {
		t.Fatalf("enumerate completions: %v", err)
	}
	if enumerated.Truncated {
		t.Fatalf("the enumeration of completions stopped early: at bound %v, undecided %v", enumerated.AtBound, enumerated.Undecided)
	}
	found := make(map[string][]solve.Assignment)
	for _, values := range enumerated.Solutions {
		outputs, err := encoding.DecodeOutputs(values)
		if err != nil {
			t.Fatalf("decode outputs: %v", err)
		}
		found[spellOutputs(outputs)] = values
	}
	out := comparedOutcomes{explored: sortedKeys(explored), found: sortedKeys(found), agreeing: true}
	for identity, witness := range explored {
		if _, ok := found[identity]; !ok {
			out.agreeing = false
			t.Errorf("the exploration reached an outcome the solver does not: %s\n  witness: %s", identity, witness)
		}
	}
	for identity := range found {
		if _, ok := explored[identity]; !ok {
			out.agreeing = false
			t.Errorf("the solver reaches an outcome the exploration does not: %s", identity)
		}
	}

	for _, identity := range out.found {
		fixed, err := encoding.Fix(completion, found[identity])
		if err != nil {
			t.Fatalf("fix outcome: %v", err)
		}
		result, err := solver.Solve(context.Background(), fixed)
		if err != nil {
			t.Fatalf("solve fixed outcome: %v", err)
		}
		if result.Status != solve.StatusSat {
			t.Errorf("the outcome %s enumerated is not satisfiable on its own: %v", identity, result.Status)
			continue
		}
		w, err := encoding.Decode(result)
		if err != nil {
			t.Fatalf("decode witness: %v", err)
		}
		out.witnesses++
		if replayOutcome(t, fresh, action, identity, w) {
			out.replayed++
		}
	}
	t.Logf("%d outcomes explored (%s), %d enumerated, %d of %d witnesses replayed", len(explored), exploration.Status(), len(found), out.replayed, out.witnesses)
	return out
}

// refereeVerdict runs check 3: the engine's verdict on the action agrees with
// what the exhaustive exploration reached. The verdict is returned for check 4.
func refereeVerdict(t *testing.T, solver *solve.Solver, d *document, ask *analysis.HoldsAsk, budget analysis.Budget, exploration *runtime.Exploration) (bool, analysis.Result) {
	t.Helper()
	deadlocks, failures := 0, 0
	for _, o := range exploration.Outcomes {
		switch {
		case o.Outcome.Err == nil:
		case errors.Is(o.Outcome.Err, runtime.ErrActionDeadlock):
			deadlocks++
		default:
			failures++
		}
	}
	q := analysis.Question{Kind: analysis.Holds, Subject: ask.Behavior.Name, Free: analysis.FreeSchedule, Holds: ask}
	verdict := answer(t, New(func() (*solve.Solver, error) { return solver, nil }), d, q, budget)
	agreeing := true
	switch {
	case deadlocks > 0:
		if verdict.Claim != analysis.ClaimViolated || !errors.Is(verdict.Values[0].Err, runtime.ErrActionDeadlock) {
			agreeing = false
			t.Errorf("the exploration deadlocks but the engine answers %v/%v: %s", verdict.Claim, verdict.Strength, verdict.Reason)
		}
	case failures > 0:
		if verdict.Claim != analysis.ClaimViolated {
			agreeing = false
			t.Errorf("the exploration fails but the engine answers %v/%v: %s", verdict.Claim, verdict.Strength, verdict.Reason)
		}
	default:
		if verdict.Claim != analysis.ClaimHolds {
			agreeing = false
			t.Errorf("the exploration completes every run but the engine answers %v/%v: %s", verdict.Claim, verdict.Strength, verdict.Reason)
		}
		if verdict.Strength == analysis.Proved && !exploration.Complete() {
			t.Logf("proved by the engine; the exploration was %s", exploration.Status())
		}
	}
	t.Logf("engine: %v/%v", verdict.Claim, verdict.Strength)
	return agreeing, verdict
}

// reproduceViolation runs check 4: the action is explored over every schedule
// with the witness's inputs pinned as a caller pins them, and some run reaches
// the violation the witness claims: the condition false, or the failure named.
func reproduceViolation(t *testing.T, d *document, ask *analysis.HoldsAsk, budget analysis.Budget, verdict analysis.Result, tally *refereeTally) bool {
	t.Helper()
	tally.inputs++
	claimed := verdict.Values[0].Err
	policy, err := runtime.ExplorePolicy(runtime.DefaultExploreBudget)
	if err != nil {
		t.Fatalf("explore policy: %v", err)
	}
	exploration, err := runtime.Explore(context.Background(), policy, d.fresh(budget), func(ctx *runtime.Context) (runtime.Outcome, error) {
		pinned, err := pinInputs(ctx, verdict.Witness.Inputs)
		if err != nil {
			return runtime.Outcome{}, err
		}
		exec, err := ctx.CreateActionExecutorWithInputs(ask.Behavior, nil, pinned)
		if err != nil {
			return runtime.Outcome{}, err
		}
		defer exec.Release()
		for {
			for _, condition := range ask.Conditions {
				if ok, err := exec.Holds(condition, ask.Scope); err != nil {
					return runtime.Outcome{}, err
				} else if !ok {
					return runtime.Outcome{}, fmt.Errorf("%s neither holds nor is violated", condition.Name)
				}
			}
			if exec.State() == runtime.StateCompleted {
				return ctx.ActionOutcome(exec.Results()), nil
			}
			if err := exec.Step(); err != nil {
				return runtime.Outcome{}, err
			}
		}
	})
	if err != nil {
		t.Fatalf("explore with the inputs pinned: %v", err)
	}
	for _, o := range exploration.Outcomes {
		if sameViolation(o.Outcome.Err, claimed) {
			tally.reproduced++
			t.Logf("reproduced under explore with %v pinned: %v", verdict.Witness.Inputs, o.Outcome.Err)
			return true
		}
	}
	t.Errorf("exploring with %v pinned reaches no run ending as the witness claims (%v); %d outcomes, %s",
		verdict.Witness.Inputs, claimed, len(exploration.Outcomes), exploration.Status())
	return false
}

// pinInputs evaluates the values a witness spells, in the context the run is about
// to begin in, as a caller's inputs are evaluated.
func pinInputs(ctx *runtime.Context, inputs []runtime.InputTaken) (map[string]runtime.Value, error) {
	pinned := make(map[string]runtime.Value, len(inputs))
	for _, in := range inputs {
		p := parser.New(source.New("<witness>", []byte(in.Written)))
		expr := p.ParseExpression()
		if expr == nil || len(p.Diagnostics) > 0 {
			return nil, fmt.Errorf("input %s = %q is not an expression", in.Feature, in.Written)
		}
		value, err := ctx.Eval(expr)
		if err != nil {
			return nil, fmt.Errorf("input %s = %q: %w", in.Feature, in.Written, err)
		}
		pinned[in.Feature] = value
	}
	return pinned, nil
}

// sameViolation reports whether a run ended as the witness claims: in the
// violation of the same condition, or in a deadlock or failure alike.
func sameViolation(got, claimed error) bool {
	var violation, claimedViolation *runtime.ViolationError
	switch {
	case got == nil || claimed == nil:
		return false
	case errors.As(claimed, &claimedViolation):
		return errors.As(got, &violation) && violation.Element == claimedViolation.Element
	case errors.Is(claimed, runtime.ErrActionDeadlock):
		return errors.Is(got, runtime.ErrActionDeadlock)
	default:
		return !errors.As(got, &violation) && !errors.Is(got, runtime.ErrActionDeadlock)
	}
}

// exploreBudget is the budget a case's exploration runs under.
func (c corpusCase) exploreBudget() runtime.ExploreBudget {
	budget := runtime.DefaultExploreBudget
	if c.ExploreBudget != nil {
		if c.ExploreBudget.Runs != nil {
			budget.Runs = *c.ExploreBudget.Runs
		}
		if c.ExploreBudget.Depth != nil {
			budget.Depth = *c.ExploreBudget.Depth
		}
	}
	return budget
}

// explore runs every linearization of the action under the budget.
func explore(t *testing.T, budget runtime.ExploreBudget, fresh func() (*runtime.Context, error), action *symbols.Symbol) *runtime.Exploration {
	t.Helper()
	policy, err := runtime.ExplorePolicy(budget)
	if err != nil {
		t.Fatalf("explore policy: %v", err)
	}
	exploration, err := runtime.Explore(context.Background(), policy, fresh, func(ctx *runtime.Context) (runtime.Outcome, error) {
		outputs, err := ctx.ExecuteAction(action)
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	})
	if err != nil {
		t.Fatalf("explore: %v", err)
	}
	return exploration
}

// replayOutcome replays w through the interpreter and checks it completes with
// the outcome named, having followed the witness whole and noted its choices.
func replayOutcome(t *testing.T, fresh func() (*runtime.Context, error), action *symbols.Symbol, identity string, w *Witness) bool {
	t.Helper()
	ctx, err := fresh()
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	if err := ctx.SetSchedule(runtime.ReplayPolicy(w.Choices)); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	outputs, err := ctx.ExecuteAction(action)
	if err != nil {
		t.Errorf("the witness of %s does not replay: %v\n  witness: %s", identity, err, runtime.FormatChoices(w.Choices))
		return false
	}
	if left := ctx.Unfollowed(); left != nil {
		t.Errorf("the witness of %s was not followed whole: %v", identity, left)
		return false
	}
	if got := rootOutputs(ctx.ActionOutcome(outputs)); got != identity {
		t.Errorf("the witness of %s replays to %s\n  witness: %s", identity, got, runtime.FormatChoices(w.Choices))
		return false
	}
	var trace []string
	for _, c := range ctx.Choices() {
		trace = append(trace, c.Choice().String())
	}
	var claimed []string
	for _, c := range w.Choices {
		claimed = append(claimed, c.String())
	}
	if !slices.Equal(trace, claimed) {
		t.Errorf("the replay of %s traces\n  %s\nbut the witness says\n  %s", identity, strings.Join(trace, "\n  "), strings.Join(claimed, "\n  "))
		return false
	}
	return true
}

// rootOutputs spells the action's own attributes of an outcome, as spellOutputs
// spells the solver's, so the two compare as text.
func rootOutputs(o runtime.Outcome) string {
	outputs := make(map[string]string)
	for name, value := range o.Outputs {
		if strings.Contains(name, ".") {
			continue
		}
		outputs[name] = runtime.FormatValue(value)
	}
	return spellOutputs(outputs)
}

// spellOutputs renders outputs in name order as one identity.
func spellOutputs(outputs map[string]string) string {
	if len(outputs) == 0 {
		return "no outputs"
	}
	parts := make([]string, 0, len(outputs))
	for _, name := range sortedKeys(outputs) {
		parts = append(parts, fmt.Sprintf("%s = %s", name, outputs[name]))
	}
	return strings.Join(parts, "; ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
