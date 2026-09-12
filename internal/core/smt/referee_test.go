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

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The referee holds the encoding to the interpreter over every conformance action
// case that lists its admissible outcomes, on three checks:
//  1. the outcomes the solver enumerates are the outcomes the exploration reaches;
//  2. every witness the solver gives replays to the outcome it claims, and the
//     replay's trace is the witness's choice lines;
//  3. the engine's verdict on deadlock freedom agrees with the exhaustive exploration.
//
// A case with a construct outside the stage is recorded as refused, not passed.

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
			m := runtime.NewModel(semantics.NewModel(resolver), resolver)
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
	refusals                                        []string
}

// TestRefereeCorpus runs checks 1–3 over the corpus and reports the counts.
func TestRefereeCorpus(t *testing.T) {
	solver := requireSolver(t)
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
			refereeCase(t, solver, name, c, tally)
		})
	}
	t.Logf("referee: %d cases encoded, %d refused, %d agreeing on outcomes and verdict; %d witnesses, %d replayed",
		tally.encoded, tally.refused, tally.agreeing, tally.witnesses, tally.replayed)
	for _, refusal := range tally.refusals {
		t.Logf("refused: %s", refusal)
	}
	if tally.encoded == 0 {
		t.Fatal("the referee encoded no case")
	}
}

// refereeCase holds one case to the interpreter, adding to the tally.
func refereeCase(t *testing.T, solver *solve.Solver, name string, c corpusCase, tally *refereeTally) {
	d, action := corpusDocument(t, name, c)
	q := analysis.Question{Kind: analysis.Holds, Subject: action.Name, Free: analysis.FreeSchedule, Holds: &analysis.HoldsAsk{
		Behavior: action,
		Start: func(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
			return ctx.CreateActionExecutor(action)
		},
	}}
	budget := analysis.Budget{Depth: DefaultMoves}
	fresh := func() (*runtime.Context, error) {
		return d.model.NewContextOn(0, budget)
	}

	ctx, err := fresh()
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	exec, err := ctx.CreateActionExecutor(action)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	encoding, err := Encode(ctx, action, exec.Graph(), DefaultMoves, DefaultUnroll)
	exec.Release()
	if err != nil {
		if refusal, fault := refusalOf(err); fault != nil {
			t.Fatalf("encode: %v", fault)
		} else {
			tally.refused++
			tally.refusals = append(tally.refusals, name+": "+refusal.Error())
			t.Logf("refused: %v", refusal)
			return
		}
	}
	tally.encoded++

	exploration := exploreCase(t, c, fresh, action)
	explored := make(map[string]string)
	for _, o := range exploration.Outcomes {
		if o.Outcome.Err != nil {
			continue
		}
		explored[rootOutputs(o.Outcome)] = runtime.FormatChoices(o.Witness)
	}
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

	// Check 1: the outcome sets agree.
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
	agreeing := true
	for identity, witness := range explored {
		if _, ok := found[identity]; !ok {
			agreeing = false
			t.Errorf("the exploration reached an outcome the solver does not: %s\n  witness: %s", identity, witness)
		}
	}
	for identity := range found {
		if _, ok := explored[identity]; !ok {
			agreeing = false
			t.Errorf("the solver reaches an outcome the exploration does not: %s", identity)
		}
	}

	// Check 2: one witness per outcome replays to it, its trace the witness's choices.
	for _, identity := range sortedKeys(found) {
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
		tally.witnesses++
		if replayOutcome(t, fresh, action, identity, w) {
			tally.replayed++
		}
	}

	// Check 3: the deadlock verdict agrees with the exhaustive exploration.
	engine := New(func() (*solve.Solver, error) { return solver, nil })
	verdict := answer(t, engine, d, q, budget)
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
	if agreeing {
		tally.agreeing++
	}
	t.Logf("%d outcomes explored (%s), %d enumerated; engine: %v/%v", len(explored), exploration.Status(), len(found), verdict.Claim, verdict.Strength)
}

// exploreCase runs the case's exploration under its own budget.
func exploreCase(t *testing.T, c corpusCase, fresh func() (*runtime.Context, error), action *symbols.Symbol) *runtime.Exploration {
	t.Helper()
	budget := runtime.DefaultExploreBudget
	if c.ExploreBudget != nil {
		if c.ExploreBudget.Runs != nil {
			budget.Runs = *c.ExploreBudget.Runs
		}
		if c.ExploreBudget.Depth != nil {
			budget.Depth = *c.ExploreBudget.Depth
		}
	}
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
