package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// CheckEngineName is the name of the engine that searches an action's schedules explicitly.
const CheckEngineName = "check"

// CheckAsk is what a check question asks over the schedules of one action: how the
// action begins, what must hold at every stable state, what may not diverge, and
// where the witnesses go.
type CheckAsk struct {
	// Start begins the action in a context of the check's own; a replay begins it the same way.
	Start runtime.ActionStarter
	// Properties are evaluated at every stable state; one false is a violation.
	Properties []runtime.CheckProperty
	// Diverge names the features whose final values are compared across the
	// complete schedules; none compares the action's own attributes, and the
	// performing object's features when one performs it.
	Diverge []string
	// WitnessDir is where every violation's and divergent value's witness is
	// written, a file per witness; empty writes none.
	WitnessDir string
}

// Checked is a check's report with where its witnesses were written: a path per
// violation and per divergent value, in the report's order; none without a directory.
type Checked struct {
	Report     *runtime.CheckReport
	Violations []string
	Divergent  [][]string
}

// ErrConstruct is the typed refusal of a construct the check engine does not search.
var ErrConstruct = errors.New("construct outside the check engine's search")

// ConstructError names the construct the check engine met and does not search.
type ConstructError struct {
	Engine    string
	Construct string
}

// Error names the engine and the construct.
func (e *ConstructError) Error() string {
	return fmt.Sprintf("%s does not search %s", e.Engine, e.Construct)
}

// Is matches ErrConstruct.
func (e *ConstructError) Is(target error) bool { return target == ErrConstruct }

// checkEngine answers Outcomes and Holds questions over an action by a depth-first
// search of its schedules, one token advancing one node at a time, under static
// partial-order reduction; what it finds within its bounds is bounded, never proved.
type checkEngine struct{}

// NewCheck returns the check engine.
func NewCheck() Engine { return checkEngine{} }

// Name is `check`.
func (checkEngine) Name() string { return CheckEngineName }

// Describe: the search is bounded by depth, states, the plan's clock and the
// executor's budgets; every violation and divergent value is a replayed witness.
func (checkEngine) Describe() Description {
	return Description{
		Questions: []Kind{Outcomes, Holds},
		Bounds:    []string{"depth", "states", "deadline", "steps", "elements", "behaviors"},
		Replays:   true,
		Authority: Bounded,
	}
}

// Covers takes an Outcomes or Holds question over an action's schedules with its
// inputs as written; the constructs the search meets and cannot capture are
// refused as it meets them, as a result not covered naming the construct.
func (e checkEngine) Covers(_ *Model, q Question) Coverage {
	if q.Kind != Outcomes && q.Kind != Holds {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if q.Free.Has(FreeInputs) {
		return refused(&FreedomError{Engine: e.Name(), Free: FreeInputs})
	}
	if !q.Free.Has(FreeSchedule) {
		return refused(&ConstructError{Engine: e.Name(), Construct: "a fixed schedule"})
	}
	if q.Check == nil || q.Check.Start == nil {
		return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "a Check starting an action"})
	}
	return covered
}

// Run searches the action's schedules under the budget's depth and, as its runs,
// states, in a context of the check's own on the plan's first worker; a violation
// is witnessed and a clean search bounded once every witness has replayed on the
// plan's workers. The plan's clock ending is the search stopping, an error, not a
// result. A model that builds no context of a run's own is the typed fault NoRuntimeError.
func (e checkEngine) Run(ctx context.Context, model *Model, q Question, budget Budget) (Result, error) {
	if !model.builds() {
		return Result{}, &NoRuntimeError{Engine: e.Name()}
	}
	fresh := func() (*runtime.Context, error) { return model.NewContextOn(0, budget) }
	started := time.Now()
	limits := runtime.CheckBudget{Depth: budget.Depth, States: budget.Runs}
	options := runtime.CheckOptions{Diverge: q.Check.Diverge, Reduce: true}
	report, err := runtime.CheckAction(ctx, fresh, q.Check.Start, limits, options, q.Check.Properties)
	result := Result{Question: q, Engine: e.Name()}
	switch {
	case errors.Is(err, runtime.ErrSnapshotPausedBody):
		result.Strength = NotCovered
		result.Reason = (&ConstructError{Engine: e.Name(), Construct: "a body paused mid-statement: " + err.Error()}).Error()
		result.Elapsed = time.Since(started)
		return result, nil
	case errors.Is(err, runtime.ErrCheckRefused):
		result.Strength = NotCovered
		result.Reason = (&ConstructError{Engine: e.Name(), Construct: "a move the run makes otherwise: " + err.Error()}).Error()
		result.Elapsed = time.Since(started)
		return result, nil
	case err != nil:
		return Result{}, err
	}
	checked, disagreement, err := e.replayed(ctx, model, q, budget, report)
	if err != nil {
		return Result{}, err
	}
	result.Bounds = checkBounds(report, budget)
	result.Values = []Evaluation{{Name: q.Subject, Checked: checked}}
	result.Elapsed = time.Since(started)
	switch {
	case disagreement != nil:
		result.Strength = NotCovered
		result.Reason = disagreement.Error()
	case report.Verdict == runtime.CheckViolation:
		result.Claim = ClaimViolated
		result.Strength = Witnessed
		first := report.Violations[0]
		result.Reason = first.String()
		result.Witness = &Witness{Schedule: runtime.ReplayPolicy(first.Witness.Choices), Choices: first.Witness.Choices}
	case q.Kind == Holds:
		result.Claim = ClaimHolds
		result.Strength = Bounded
	default:
		result.Claim = ClaimOutcomes
		result.Strength = Bounded
	}
	return result, nil
}

// checkBounds is every bound the search took and which it reached: depth and states
// as the budget set them (0 unbounded) and the executor's budgets; the plan's clock
// is the framework's, a search it stops being cancelled rather than bounded.
func checkBounds(report *runtime.CheckReport, budget Budget) Bounds {
	hit := func(name string) bool { return slices.Contains(report.BoundsHit, name) }
	return Bounds{
		{Name: "depth", Limit: int64(budget.Depth), Reached: hit("depth")},
		{Name: "states", Limit: int64(budget.Runs), Reached: hit("states")},
		{Name: "steps", Limit: report.Limits.MaxActionSteps, Reached: hit("steps")},
		{Name: "elements", Limit: report.Limits.MaxElements, Reached: hit("elements")},
	}
}

// replayWitness is one witness the report carries, replayed and written.
type replayWitness struct {
	what    string
	witness runtime.Witness
	file    string
	path    *string
}

// replayed re-runs every violation's and divergent value's witness on the plan's workers,
// writing each to the question's witness directory; the first witness whose replay
// leaves another state is the disagreement, which leaves the report not covered.
func (e checkEngine) replayed(ctx context.Context, model *Model, q Question, budget Budget, report *runtime.CheckReport) (*Checked, error, error) {
	checked := &Checked{Report: report}
	var witnesses []*replayWitness
	if q.Check.WitnessDir != "" {
		checked.Violations = make([]string, len(report.Violations))
		checked.Divergent = make([][]string, len(report.Divergent))
	}
	prefix := witnessPrefix(q.Subject)
	for i, v := range report.Violations {
		w := &replayWitness{what: "violation " + fmt.Sprint(i+1) + " (" + v.String() + ")", witness: v.Witness, file: fmt.Sprintf("%sviolation-%d.witness", prefix, i+1)}
		if checked.Violations != nil {
			w.path = &checked.Violations[i]
		}
		witnesses = append(witnesses, w)
	}
	for i, d := range report.Divergent {
		if checked.Divergent != nil {
			checked.Divergent[i] = make([]string, len(d.Values))
		}
		for j, value := range d.Values {
			w := &replayWitness{what: fmt.Sprintf("%s = %s", d.Feature, value.Value), witness: value.Witness,
				file: fmt.Sprintf("%s%s-%d.witness", prefix, fileToken(d.Feature), j+1)}
			if checked.Divergent != nil {
				w.path = &checked.Divergent[i][j]
			}
			witnesses = append(witnesses, w)
		}
	}
	if q.Check.WitnessDir != "" && len(witnesses) > 0 {
		if err := os.MkdirAll(q.Check.WitnessDir, 0o755); err != nil {
			return nil, nil, err
		}
	}
	jobs := max(budget.Jobs, 1)
	errs := make([]error, len(witnesses))
	var wg sync.WaitGroup
	for job := 0; job < jobs && job < len(witnesses); job++ {
		wg.Add(1)
		go func(job int) {
			defer wg.Done()
			fresh := func() (*runtime.Context, error) { return model.NewContextOn(job, budget) }
			for i := job; i < len(witnesses); i += jobs {
				errs[i] = e.replayOne(ctx, fresh, q, witnesses[i])
			}
		}(job)
	}
	wg.Wait()
	var disagreement error
	for i, err := range errs {
		switch {
		case err == nil:
		case errors.Is(err, runtime.ErrReplayDisagrees):
			if disagreement == nil {
				disagreement = fmt.Errorf("replay of %s: %w", witnesses[i].what, err)
			}
		default:
			return nil, nil, err
		}
	}
	return checked, disagreement, nil
}

// replayOne writes the witness, when a directory is named, and replays it.
func (e checkEngine) replayOne(ctx context.Context, fresh func() (*runtime.Context, error), q Question, w *replayWitness) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.path != nil {
		*w.path = filepath.Join(q.Check.WitnessDir, w.file)
		if err := os.WriteFile(*w.path, []byte(w.witness.String()), 0o644); err != nil {
			return err
		}
	}
	_, err := runtime.ReplayAction(fresh, q.Check.Start, w.witness)
	return err
}

// witnessPrefix is the subject as a file name begins, `test.race.`; empty for no subject.
func witnessPrefix(subject string) string {
	if subject == "" {
		return ""
	}
	return fileToken(strings.ReplaceAll(subject, "::", ".")) + "."
}

// fileToken keeps letters, digits, `_`, `-` and `.` of a name, replacing the rest with `_`.
func fileToken(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
