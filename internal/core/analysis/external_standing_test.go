package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// The schedules of the racing action's three outcomes, as check spells its witnesses:
// the writer chosen last is the one whose value x ends with. raceUnfollow names a second
// move the run cannot take, 2@a having acted already.
const (
	raceEndsOne   = "step 3: 3@b first of 2@a, 3@b, 4@c; step 4: 4@c first of 2@a, 4@c"
	raceEndsOneCB = "step 3: 4@c first of 2@a, 3@b, 4@c; step 4: 3@b first of 2@a, 3@b"
	raceEndsTwo   = "step 3: 2@a first of 2@a, 3@b, 4@c; step 4: 4@c first of 3@b, 4@c"
	raceEndsThree = "step 3: 2@a first of 2@a, 3@b, 4@c; step 4: 3@b first of 3@b, 4@c"
	raceUnfollow  = "step 3: 2@a first of 2@a, 3@b; step 4: 4@c first of 2@a, 4@c"
)

// endsAtOne is the property that the racing action, once complete, left x as 1: false at
// the end of a run whose last writer is another, true at every state before.
func endsAtOne() runtime.CheckProperty {
	return runtime.CheckProperty{Name: "x", Holds: func(_ *runtime.Context, exec *runtime.ActionExecutor) (bool, error) {
		return exec.State() != runtime.StateCompleted || exec.Results()["x"].Const.Int == 1, nil
	}}
}

// standinRegistry is the default registry with the stand-in registered as an engine
// answering every check and solve kind, its runs answering result.
func standinRegistry(t *testing.T, result string, witness WitnessKind) *Registry {
	t.Helper()
	t.Setenv(engineStandinDescribe, `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds","sensitive","outcomes","satisfiable"]}`)
	return standinRegistryEntry(t, result, witness, func(*EngineEntry) {})
}

// standinRegistryEntry is standinRegistry with the entry edited before registration; the
// caller sets the describe the edited entry matches.
func standinRegistryEntry(t *testing.T, result string, witness WitnessKind, edit func(*EngineEntry)) *Registry {
	t.Helper()
	t.Setenv(engineStandinResult, result)
	entry := standinEntry(t)
	entry.Answers = []Kind{Holds, Sensitive, Outcomes, Satisfiable}
	entry.Witness = witness
	edit(&entry)
	r := Default()
	if err := r.Register(NewEngine(entry)); err != nil {
		t.Fatalf("register: %v", err)
	}
	return r
}

// standinAnswers puts q to the stand-in alone and returns its result.
func standinAnswers(t *testing.T, r *Registry, model *Model, q Question, budget Budget) Result {
	t.Helper()
	plan, err := r.AnswerWith(context.Background(), model, q, budget, Only("standin"))
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Engine != "standin" {
		t.Fatalf("steps %+v, want the stand-in alone", plan.Steps)
	}
	return plan.Result
}

// notCovered asserts a result is not covered with the claim kept and the reason spelt.
func notCovered(t *testing.T, result Result, want ...string) {
	t.Helper()
	if result.Claim != ClaimNone || result.Strength != NotCovered || result.Witness != nil || result.Executions != nil {
		t.Fatalf("result %+v, want not covered without evidence", result)
	}
	for _, w := range want {
		if !strings.Contains(result.Reason, w) {
			t.Fatalf("reason %q, want it to say %q", result.Reason, w)
		}
	}
}

func schedules(schedules ...string) string {
	quoted := make([]string, len(schedules))
	for i, s := range schedules {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// A violated claim is witnessed only after its schedule replays and the property
// evaluates false at the run's end.
func TestExternalViolationStandsAfterReplay(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	r := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`}}`, WitnessSchedule)
	result := standinAnswers(t, r, f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}}), Budget{})
	if result.Engine != "standin" || result.Claim != ClaimViolated || result.Strength != Witnessed || result.Witness == nil {
		t.Fatalf("result %+v, want the stand-in's violation witnessed", result)
	}
	if replay, ok := result.Witness.Schedule.Replay(); !ok || runtime.FormatChoices(replay) != raceEndsThree {
		t.Fatalf("witness %s, want the engine's schedule as a replay", result.Witness.Schedule)
	}
	if !strings.Contains(result.Reason, "at its end") || !strings.Contains(result.Reason, "`x` evaluates false") || !strings.Contains(result.Reason, "replayed") {
		t.Fatalf("reason %q, want the violation placed at the end and replayed", result.Reason)
	}
}

// A schedule under which the property holds earns nothing: the engine's claim is kept in
// the not-covered reason.
func TestExternalViolationRefusedWhenThePropertyHolds(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	r := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne)+`}}`, WitnessSchedule)
	result := standinAnswers(t, r, f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}}), Budget{})
	notCovered(t, result, `engine "standin" reports a violation, witnessed`, "its schedule replays and `x` holds at its end")
}

// The claim is evaluated at the move the witness names: a property false after the first
// move and restored by the run's end is violated at move 1, and holds at the end.
func TestExternalViolationIsJudgedAtTheMoveNamed(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	notOne := runtime.CheckProperty{Name: "x", Holds: func(_ *runtime.Context, exec *runtime.ActionExecutor) (bool, error) {
		return exec.Results()["x"].Const.Int != 1, nil
	}}
	ask := &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{notOne}}
	at1 := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`,"at":1}}`, WitnessSchedule)
	result := standinAnswers(t, at1, f.building(), checkQuestion(t, f, Holds, ask), Budget{})
	if result.Claim != ClaimViolated || result.Strength != Witnessed || !strings.Contains(result.Reason, "at move 1") {
		t.Fatalf("result %+v, want the violation witnessed at move 1", result)
	}
	atEnd := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, atEnd, f.building(), checkQuestion(t, f, Holds, ask), Budget{}), "`x` holds at its end")
	at9 := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`,"at":9}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, at9, f.building(), checkQuestion(t, f, Holds, ask), Budget{}), "does not replay")
}

// A schedule the run cannot follow, or one that does not read, is a not-covered result.
func TestExternalWitnessThatDoesNotReplay(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}})
	unfollowed := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceUnfollow)+`}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, unfollowed, f.building(), q, Budget{}), `engine "standin" reports a violation, witnessed`, "its witness does not replay")
	unreadable := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":["not a schedule"]}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, unreadable, f.building(), q, Budget{}), "its witness does not read as a schedule")
}

// A witness of the wrong kind or count is a protocol break, the run not covered.
func TestExternalWitnessShapeIsChecked(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}})
	cases := map[string]struct {
		result  string
		witness WitnessKind
		want    string
	}{
		"no witness":          {`{"claim":"violated","strength":"witnessed"}`, WitnessSchedule, "with no witness"},
		"witness on holds":    {`{"claim":"holds","strength":"bounded","witness":{"schedules":` + schedules(raceEndsOne) + `}}`, WitnessSchedule, "which only violated, sensitive and satisfiable take"},
		"two schedules":       {`{"claim":"violated","strength":"witnessed","witness":{"schedules":` + schedules(raceEndsOne, raceEndsTwo) + `}}`, WitnessSchedule, "one schedule, not 2"},
		"assignment declared": {`{"claim":"violated","strength":"witnessed","witness":{"schedules":` + schedules(raceEndsOne) + `}}`, WitnessAssignment, "declares assignment witnesses"},
		"assignment given":    {`{"claim":"violated","strength":"witnessed","witness":{"inputs":[{"name":"x","value":1}]}}`, WitnessSchedule, "is an assignment"},
		"unknown claim":       {`{"claim":"maybe","strength":"witnessed"}`, WitnessSchedule, "no claim"},
		"unknown strength":    {`{"claim":"violated","strength":"sure"}`, WitnessSchedule, "no strength"},
		"inconsistent":        {`{"claim":"holds","strength":"witnessed"}`, WitnessSchedule, "holds cannot be witnessed"},
		"two per execution":   {`{"claim":"holds","strength":"bounded","executions":[{"schedules":` + schedules(raceEndsOne, raceEndsTwo) + `}]}`, WitnessSchedule, "execution 1 is 2 schedules"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := standinRegistry(t, c.result, c.witness)
			notCovered(t, standinAnswers(t, r, f.building(), q, Budget{}), c.want)
		})
	}
}

// inputModel leaves n unbound and binds limit; add moves limit up by one.
const inputModel = `package test {
	private import ScalarValues::*;
	action open {
		attribute n : Natural;
		attribute limit : Integer = 5;
		first start;
		action add { assign limit := limit + 1; }
		done;
		succession first start then add;
		succession first add then done;
	}
}`

// inputQuestion asks Holds of open with its inputs free: the unbound n and those named.
func inputQuestion(t *testing.T, f *fixture, property runtime.CheckProperty, named ...string) Question {
	t.Helper()
	a := f.checked(t, "open")
	q := questionOf(t, "test::open", Holds, &CheckAsk{Start: a.start, Properties: []runtime.CheckProperty{property}})
	q.Free |= FreeInputs
	q.Holds = &HoldsAsk{Behavior: a.sym, Start: a.start, Inputs: named}
	return q
}

// The inputs of a schedule witness are fixed before the replay's first move, on the
// features the question leaves free — the unbound and the named — and the result lists
// them; the engine sees those features listed on the question it was put.
func TestExternalWitnessInputsAreReplayed(t *testing.T) {
	f := parseModel(t, inputModel)
	a := f.checked(t, "open")
	dir := t.TempDir()
	t.Setenv(engineStandinWire, dir)
	violated := func(witness string) string {
		return `{"claim":"violated","strength":"witnessed","witness":{"schedules":["no choice points"],"inputs":` + witness + `}}`
	}
	t.Run("an unbound input", func(t *testing.T) {
		r := standinRegistry(t, violated(`[{"name":"n","value":7}]`), WitnessSchedule)
		result := standinAnswers(t, r, f.building(), inputQuestion(t, f, a.atMost("n", 5)), Budget{})
		if result.Claim != ClaimViolated || result.Strength != Witnessed || !strings.Contains(result.Reason, "`n` evaluates false there") {
			t.Fatalf("result %+v, want the violation witnessed under n = 7", result)
		}
		if len(result.Inputs) != 1 || result.Inputs[0].String() != "n = 7" || result.Inputs[0].Type != "Natural" {
			t.Fatalf("inputs %v, want n = 7 of type Natural", result.Inputs)
		}
		if result.Witness == nil || len(result.Witness.Inputs) != 1 || result.Witness.Inputs[0].String() != "input n = 7" {
			t.Fatalf("witness %+v, want its input kept", result.Witness)
		}
	})
	t.Run("a named input written as notation", func(t *testing.T) {
		r := standinRegistry(t, violated(`[{"name":"n","value":1},{"name":"limit","value":"2 * 4"}]`), WitnessSchedule)
		result := standinAnswers(t, r, f.building(), inputQuestion(t, f, a.atMost("limit", 8), "limit"), Budget{})
		if result.Claim != ClaimViolated || result.Strength != Witnessed {
			t.Fatalf("result %+v, want the violation witnessed under limit = 8 + 1", result)
		}
		if len(result.Inputs) != 2 || result.Inputs[1].String() != "limit = 2 * 4" {
			t.Fatalf("inputs %v, want n and limit as the witness spelt them", result.Inputs)
		}
	})
	t.Run("a bound input the question does not free", func(t *testing.T) {
		r := standinRegistry(t, violated(`[{"name":"limit","value":9}]`), WitnessSchedule)
		result := standinAnswers(t, r, f.building(), inputQuestion(t, f, a.atMost("limit", 8)), Budget{})
		notCovered(t, result, "the input limit, which the question does not leave free")
	})
	t.Run("an input the run cannot read", func(t *testing.T) {
		r := standinRegistry(t, violated(`[{"name":"n","value":"nothing"}]`), WitnessSchedule)
		result := standinAnswers(t, r, f.building(), inputQuestion(t, f, a.atMost("n", 5)), Budget{})
		notCovered(t, result, "its witness does not replay", "witness input refused: n")
	})
	t.Run("an input given twice", func(t *testing.T) {
		r := standinRegistry(t, violated(`[{"name":"n","value":1},{"name":"n","value":2}]`), WitnessSchedule)
		notCovered(t, standinAnswers(t, r, f.building(), inputQuestion(t, f, a.atMost("n", 5)), Budget{}), "gives n twice")
	})
	listed := 0
	for _, line := range captured(t, dir) {
		if line.host && strings.Contains(line.text, `"inputs":[{"name":"n","type":"Natural"}`) {
			listed++
		}
	}
	if listed == 0 {
		t.Fatal("no run listed n as a free input of the question")
	}
}

// A schedule witness with inputs on a question that leaves no input free is refused
// naming the input.
func TestExternalWitnessInputsNeedFreeInputs(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	r := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`,"inputs":[{"name":"x","value":0}]}}`, WitnessSchedule)
	result := standinAnswers(t, r, f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}}), Budget{})
	notCovered(t, result, "the input x, which the question does not leave free")
}

// An engine's own account of the initial state and its assumptions reach the result as
// the symbolic engine's do.
func TestExternalResultCarriesInputsAndAssumptions(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	r := standinRegistry(t, `{"claim":"holds","strength":"bounded","inputs":[{"name":"x","type":"Integer","sort":"Int","domain":">= 0","free":true}],"assumptions":["test::race::positive"]}`, WitnessSchedule)
	result := standinAnswers(t, r, f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}}), Budget{})
	notCovered(t, result, "engine \"standin\" reports holds, bounded")
	if len(result.Inputs) != 1 || result.Inputs[0].String() != "x : Integer free in >= 0" || len(result.Assumptions) != 1 || result.Assumptions[0] != "test::race::positive" {
		t.Fatalf("inputs %v assumptions %v, want the engine's account kept", result.Inputs, result.Assumptions)
	}
}

// An entry declaring no witness cannot have its existential claims stood.
func TestExternalEntryWithoutWitnessesEarnsNoExistential(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	r := standinRegistry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`}}`, WitnessNone)
	result := standinAnswers(t, r, f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}}), Budget{})
	notCovered(t, result, "produces no replayable witness")
}

// Sensitivity stands on two replaying schedules under which the feature ends differently,
// the second as the contrast; equal values, or a feature the run has not, earn nothing.
func TestExternalSensitivityNeedsTwoDivergingSchedules(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Sensitive, &CheckAsk{Start: race.start})
	diverge := standinRegistry(t, `{"claim":"sensitive","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne, raceEndsThree)+`,"feature":"x"}}`, WitnessSchedule)
	result := standinAnswers(t, diverge, f.building(), q, Budget{})
	if result.Claim != ClaimSensitive || result.Strength != Witnessed || result.Witness == nil || result.Contrast == nil {
		t.Fatalf("result %+v, want sensitivity witnessed with its contrast", result)
	}
	if result.Reason != `x ends as 1 or 3 (engine "standin", both replayed)` {
		t.Fatalf("reason %q", result.Reason)
	}
	first, _ := result.Witness.Schedule.Replay()
	second, _ := result.Contrast.Schedule.Replay()
	if runtime.FormatChoices(first) != raceEndsOne || runtime.FormatChoices(second) != raceEndsThree {
		t.Fatalf("witnesses %s / %s, want the engine's two schedules", result.Witness.Schedule, result.Contrast.Schedule)
	}
	equal := standinRegistry(t, `{"claim":"sensitive","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne, raceEndsOneCB)+`,"feature":"x"}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, equal, f.building(), q, Budget{}), `engine "standin" reports sensitive, witnessed`, "both schedules replay and `x` is 1 under each")
	unknown := standinRegistry(t, `{"claim":"sensitive","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne, raceEndsThree)+`,"feature":"w"}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, unknown, f.building(), q, Budget{}), "`w` is no feature of the run")
	one := standinRegistry(t, `{"claim":"sensitive","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne)+`,"feature":"x"}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, one, f.building(), q, Budget{}), "two schedules, not 1")
	unnamed := standinRegistry(t, `{"claim":"sensitive","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne, raceEndsThree)+`}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, unnamed, f.building(), q, Budget{}), "names the feature that diverges")
	broken := standinRegistry(t, `{"claim":"sensitive","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne, raceUnfollow)+`,"feature":"x"}}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, broken, f.building(), q, Budget{}), "schedule 2: its witness does not replay")
}

// A universal claim with executions is observed on their count once every one replays
// and the claim holds at every settled state of each.
func TestExternalExecutionsAreObservedAfterReplay(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}})
	r := standinRegistry(t, `{"claim":"holds","strength":"bounded","executions":[{"schedules":`+schedules(raceEndsOne)+`},{"schedules":`+schedules(raceEndsTwo)+`},{"schedules":`+schedules(raceEndsThree)+`}]}`, WitnessSchedule)
	for _, jobs := range []int{1, 8} {
		result := standinAnswers(t, r, f.building(), q, Budget{Jobs: jobs})
		if result.Claim != ClaimHolds || result.Strength != Observed || len(result.Executions) != 3 {
			t.Fatalf("jobs %d: result %+v, want holds observed on 3 executions", jobs, result)
		}
		if result.Reason != `observed on 3 executions chosen by engine "standin", each replayed` {
			t.Fatalf("reason %q", result.Reason)
		}
		for i, want := range []string{raceEndsOne, raceEndsTwo, raceEndsThree} {
			if runtime.FormatChoices(result.Executions[i].Choices) != want {
				t.Fatalf("jobs %d: execution %d is %s, want %s", jobs, i+1, result.Executions[i].Schedule, want)
			}
		}
	}
}

// An execution on which the claim fails at any settled state, or that does not replay,
// makes the claim not covered naming the execution and the move.
func TestExternalExecutionsFailingTheClaimEarnNothing(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	atEnd := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{endsAtOne()}})
	failing := standinRegistry(t, `{"claim":"holds","strength":"bounded","executions":[{"schedules":`+schedules(raceEndsOne)+`},{"schedules":`+schedules(raceEndsThree)+`}]}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, failing, f.building(), atEnd, Budget{Jobs: 4}), `engine "standin" reports holds, bounded`, "execution 2: replays, and after 2 moves `x` evaluates false")
	passing := standinRegistry(t, `{"claim":"holds","strength":"bounded","executions":[{"schedules":`+schedules(raceEndsOne)+`},{"schedules":`+schedules(raceEndsOneCB)+`}]}`, WitnessSchedule)
	if result := standinAnswers(t, passing, f.building(), atEnd, Budget{Jobs: 4}); result.Strength != Observed || len(result.Executions) != 2 {
		t.Fatalf("result %+v, want holds observed on the 2 executions ending at 1", result)
	}
	midway := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}})
	visited := standinRegistry(t, `{"claim":"holds","strength":"bounded","executions":[{"schedules":`+schedules(raceEndsOne)+`}]}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, visited, f.building(), midway, Budget{}), "execution 1: replays, and after 2 moves `x` evaluates false")
	unfollowed := standinRegistry(t, `{"claim":"holds","strength":"bounded","executions":[{"schedules":`+schedules(raceUnfollow)+`}]}`, WitnessSchedule)
	notCovered(t, standinAnswers(t, unfollowed, f.building(), midway, Budget{}), "execution 1: does not replay")
}

// A surface's held context is a model the engine is sent: a constraint check put to it runs,
// its answer stood as any. A check question replays witnesses in contexts of a run's own,
// which a held context alone does not build: refused before the process starts.
func TestExternalEngineRunsOverAHeldContext(t *testing.T) {
	f := parseFixture(t)
	t.Setenv(engineStandinDescribe, `{"name":"standin","version":"1.0.0","protocol":1,"answers":["evaluate","holds"]}`)
	r := standinRegistryEntry(t, `{"claim":"holds","strength":"proved"}`, WitnessSchedule,
		func(e *EngineEntry) { e.Answers = []Kind{Evaluate, Holds} })
	ctx := f.context(t)
	q := Question{Kind: Evaluate, Subject: "test::Tank::low", Schedule: ctx.Schedule(),
		Perform: func(*runtime.Context) (Answer, error) { return Answer{Claim: ClaimHolds}, nil }}
	result := standinAnswers(t, r, Held(ctx), q, Budget{})
	notCovered(t, result, `engine "standin" reports holds`, "proved", "referee-record stage")

	race := f.checked(t, "race")
	check := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}})
	plan, err := r.AnswerWith(context.Background(), Held(ctx), check, Budget{}, Only("standin"))
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if refused := plan.Refused(); !errors.Is(refused, ErrNoRuntime) {
		t.Fatalf("refused %v, want ErrNoRuntime for a witness with no run context to replay in", refused)
	}
}

// A universal claim without executions is not covered with the claim kept, naming the
// stage that admits engines; a proof is no stronger than an unadmitted bound.
func TestExternalUniversalClaimsWithoutExecutionsAreNotCovered(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}})
	for _, strength := range []string{"bounded", "proved"} {
		r := standinRegistry(t, `{"claim":"holds","strength":"`+strength+`","bounds":[{"name":"depth","limit":40,"reached":false}]}`, WitnessSchedule)
		result := standinAnswers(t, r, f.building(), q, Budget{})
		notCovered(t, result, `engine "standin" reports holds`, strength, "no executions to replay", "referee-record stage")
		if len(result.Bounds) != 1 || result.Bounds[0].Name != "depth" || result.Bounds[0].Limit != 40 {
			t.Fatalf("bounds %+v, want the engine's kept", result.Bounds)
		}
	}
}

// A `none` claim passes as the engine's own not-covered answer, its reason appended.
func TestExternalNoneIsTheEnginesOwnRefusal(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	r := standinRegistry(t, `{"claim":"none","strength":"not covered","reason":"out of my depth"}`, WitnessSchedule)
	result := standinAnswers(t, r, f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start}), Budget{})
	notCovered(t, result, `engine "standin" reports no claim: out of my depth`)
}

// A satisfiable claim stands on an assignment the evaluator confirms against every query;
// one that fails a query, or leaves a variable unbound, earns nothing.
func TestExternalAssignmentIsConfirmed(t *testing.T) {
	f := parseFixture(t)
	solving := func(queries ...*solve.Query) Question {
		return Question{Kind: Satisfiable, Subject: "test::Tank", Free: FreeInputs, Solve: &SolveAsk{Queries: queries, Ask: (*solve.Solver).Solve}}
	}
	sat := standinRegistry(t, `{"claim":"satisfiable","strength":"witnessed","witness":{"inputs":[{"name":"test::C::i","value":3}]}}`, WitnessAssignment)
	result := standinAnswers(t, sat, f.building(), solving(intQuery("C", 2, 5)), Budget{})
	if result.Claim != ClaimSatisfiable || result.Strength != Witnessed {
		t.Fatalf("result %+v, want satisfiable witnessed", result)
	}
	if len(result.Values) != 1 || result.Values[0].Solved == nil || result.Values[0].Solved.Status != solve.StatusSat || result.Values[0].Solved.Solver != "engine standin" {
		t.Fatalf("values %+v, want the confirmed assignment as a sat result", result.Values)
	}
	unsat := standinRegistry(t, `{"claim":"satisfiable","strength":"witnessed","witness":{"inputs":[{"name":"test::C::i","value":7}]}}`, WitnessAssignment)
	notCovered(t, standinAnswers(t, unsat, f.building(), solving(intQuery("C", 2, 5)), Budget{}), `engine "standin" reports satisfiable`, "at its assignment")
	unbound := standinRegistry(t, `{"claim":"satisfiable","strength":"witnessed","witness":{"inputs":[{"name":"test::C::j","value":3}]}}`, WitnessAssignment)
	notCovered(t, standinAnswers(t, unbound, f.building(), solving(intQuery("C", 2, 5)), Budget{}), "gives test::C::i no value")
	twice := standinRegistry(t, `{"claim":"satisfiable","strength":"witnessed","witness":{"inputs":[{"name":"test::C::i","value":3},{"name":"test::C::i","value":4}]}}`, WitnessAssignment)
	notCovered(t, standinAnswers(t, twice, f.building(), solving(intQuery("C", 2, 5)), Budget{}), "gives test::C::i twice")
	schedule := standinRegistry(t, `{"claim":"satisfiable","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsOne)+`}}`, WitnessAssignment)
	notCovered(t, standinAnswers(t, schedule, f.building(), solving(intQuery("C", 2, 5)), Budget{}), "declares assignment witnesses and the result's is 1 schedule")
}

// Under auto the stand-in is reached only after every built-in refused; under all its
// unearned `holds` beside check's witnessed violation is a disagreement the interpreter
// resolves in the built-in's favor.
func TestExternalEngineInAutoAndAll(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}})
	r := standinRegistry(t, `{"claim":"holds","strength":"proved"}`, WitnessSchedule)
	auto := answered(t, r, f.building(), q, Budget{})
	if auto.Result.Engine != CheckEngineName || auto.Result.Claim != ClaimViolated {
		t.Fatalf("auto result %+v, want check's violation", auto.Result)
	}
	for _, step := range auto.Steps {
		if step.Engine == "standin" {
			t.Fatalf("auto ran the stand-in after a built-in answered: %+v", auto.Steps)
		}
	}
	all, err := r.AnswerWith(context.Background(), f.building(), q, Budget{}, All())
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if all.Result.Claim != ClaimViolated || all.Result.Strength != Witnessed {
		t.Fatalf("all result %+v, want the witnessed violation to stand", all.Result)
	}
	var standin *Step
	for i := range all.Steps {
		if all.Steps[i].Engine == "standin" {
			standin = &all.Steps[i]
		}
	}
	if standin == nil || standin.Result == nil || standin.Result.Strength != NotCovered {
		t.Fatalf("all steps %+v, want the stand-in run and not covered", all.Steps)
	}
	if len(all.Disagreements) != 0 {
		t.Fatalf("disagreements %+v, want none: an unearned claim contradicts nothing", all.Disagreements)
	}
}

// Under all, an external claim that stands beside a built-in's contradicting one is the
// disagreement the composition records, resolved in the witnessed claim's favor.
func TestExternalStoodClaimDisagreesWithBuiltIn(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{endsAtOne()}})
	r := standinRegistry(t, `{"claim":"holds","strength":"bounded","executions":[{"schedules":`+schedules(raceEndsOne)+`}]}`, WitnessSchedule)
	all, err := r.AnswerWith(context.Background(), f.building(), q, Budget{}, All())
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	if all.Result.Claim != ClaimViolated || all.Result.Strength != Witnessed || all.Result.Engine != CheckEngineName {
		t.Fatalf("result %+v, want check's witnessed violation over the observed holds", all.Result)
	}
	if len(all.Disagreements) != 1 {
		t.Fatalf("disagreements %+v, want the stand-in's observed holds against check", all.Disagreements)
	}
}

// Under -jobs 8 and all, the plan's standing and every step's result are the same whether the
// engine declares concurrent true or false; only the timings differ.
func TestExternalPlanIsTheSameUnderEitherConcurrency(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}})
	result := `{"claim":"holds","strength":"bounded","bounds":[{"name":"depth","limit":8}],"executions":[{"schedules":` + schedules(raceEndsOne) + `},{"schedules":` + schedules(raceEndsThree) + `}]}`
	var plans [2]Plan
	for i, concurrent := range []bool{true, false} {
		t.Setenv(engineStandinDescribe, `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds","sensitive","outcomes","satisfiable"]}`)
		r := standinRegistryEntry(t, result, WitnessSchedule, func(e *EngineEntry) { e.Concurrent = concurrent })
		plan, err := r.AnswerWith(context.Background(), f.building(), q, Budget{Depth: 8, Jobs: 8}, All())
		if err != nil {
			t.Fatalf("concurrent %v: %v", concurrent, err)
		}
		plans[i] = plan
	}
	if plans[0].Standing() != plans[1].Standing() {
		t.Fatalf("standing differs by concurrency:\n%s\n%s", plans[0].Standing(), plans[1].Standing())
	}
	if len(plans[0].Steps) != len(plans[1].Steps) {
		t.Fatalf("steps %d and %d", len(plans[0].Steps), len(plans[1].Steps))
	}
	observed := false
	for i := range plans[0].Steps {
		a, b := plans[0].Steps[i], plans[1].Steps[i]
		if a.Engine != b.Engine || a.Standing() != b.Standing() {
			t.Fatalf("step %d differs: %s / %s", i, a.Standing(), b.Standing())
		}
		if a.Result != nil && b.Result != nil && !sameAnswer(*a.Result, *b.Result) {
			t.Fatalf("step %d result differs:\n%+v\n%+v", i, *a.Result, *b.Result)
		}
		if a.Engine == "standin" && a.Result != nil && a.Result.Strength == Observed && strings.Contains(a.Result.Reason, "2 executions") {
			observed = true
		}
	}
	if !observed {
		t.Fatalf("steps %+v, want the stand-in's holds observed on 2 executions", plans[0].Steps)
	}
	if !sameAnswer(plans[0].Result, plans[1].Result) {
		t.Fatalf("results differ:\n%+v\n%+v", plans[0].Result, plans[1].Result)
	}
}

// sameAnswer is whether two results agree on their standing, claim, bounds, witnesses and
// executions; timings and the evaluations' internal pointers are left out.
func sameAnswer(a, b Result) bool {
	return a.Standing() == b.Standing() && a.Claim == b.Claim && a.Bounds.String() == b.Bounds.String() &&
		fmt.Sprint(a.Witness) == fmt.Sprint(b.Witness) && fmt.Sprint(a.Contrast) == fmt.Sprint(b.Contrast) &&
		fmt.Sprint(a.Executions) == fmt.Sprint(b.Executions) && len(a.Values) == len(b.Values)
}

// A manifest's subjects are declaration kinds: an engine for actions answers about
// `test::race`, an action, and refuses a part or a name the model does not declare, the
// refusal naming the kind found.
func TestExternalSubjectsAreDeclarationKinds(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	t.Setenv(engineStandinDescribe, `{"name":"standin","version":"1.0.0","protocol":1,"answers":["holds","sensitive","outcomes","satisfiable"],"subjects":["action"]}`)
	r := standinRegistryEntry(t, `{"claim":"violated","strength":"witnessed","witness":{"schedules":`+schedules(raceEndsThree)+`}}`, WitnessSchedule,
		func(e *EngineEntry) { e.Subjects = []string{"action"} })
	ask := &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}}
	if result := standinAnswers(t, r, f.building(), checkQuestion(t, f, Holds, ask), Budget{}); result.Strength != Witnessed {
		t.Fatalf("standing %s, want witnessed on the action", result.Standing())
	}
	for subject, want := range map[string]string{
		"test::Tank":  `answers for ["action"], not the part test::Tank`,
		"test::stray": `answers for ["action"], and test::stray is not a declaration of the model`,
	} {
		plan, err := r.AnswerWith(context.Background(), f.building(), questionOf(t, subject, Holds, ask), Budget{}, Only("standin"))
		if err != nil {
			t.Fatalf("%s: answer: %v", subject, err)
		}
		var refusal *SubjectError
		if refused := plan.Refused(); refused == nil || !errors.As(refused, &refusal) || !strings.Contains(refused.Error(), want) {
			t.Fatalf("%s: refused %v, want a SubjectError %q", subject, refused, want)
		}
	}
}

// Outside a plan, a surface's model holds no session: the request is the typed refusal.
func TestExternalRunNeedsAPlan(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	r := standinRegistry(t, `{"claim":"holds","strength":"bounded"}`, WitnessSchedule)
	e := r.engines["standin"]
	result, err := e.Run(context.Background(), f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start}), Budget{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	notCovered(t, result, errNoPlan.Error())
}
