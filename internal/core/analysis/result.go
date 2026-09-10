package analysis

import (
	"fmt"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// Claim is what a result asserts about its question.
type Claim int

const (
	// ClaimNone asserts nothing; the result's Reason says why.
	ClaimNone Claim = iota
	// ClaimHolds asserts the condition holds for what the question left free.
	ClaimHolds
	// ClaimViolated asserts an execution violates the condition.
	ClaimViolated
	// ClaimSensitive asserts the behavior's result depends on its schedule.
	ClaimSensitive
	// ClaimValue asserts the values in Values are what the subject produces.
	ClaimValue
	// ClaimTable asserts the rows in Values are the domain's, one per row.
	ClaimTable
	// ClaimOutcomes asserts the outcomes in Values are what the behavior can do.
	ClaimOutcomes
	// ClaimSatisfiable asserts an assignment in Values meets the conditions.
	ClaimSatisfiable
	// ClaimUnsatisfiable asserts no assignment meets the conditions.
	ClaimUnsatisfiable
	// ClaimUnbounded asserts an objective has no optimum: better values exist without limit.
	ClaimUnbounded
)

// String names the claim as a plan spells it.
func (c Claim) String() string {
	switch c {
	case ClaimNone:
		return "none"
	case ClaimHolds:
		return "holds"
	case ClaimViolated:
		return "violated"
	case ClaimSensitive:
		return "sensitive"
	case ClaimValue:
		return "value"
	case ClaimTable:
		return "table"
	case ClaimOutcomes:
		return "outcomes"
	case ClaimSatisfiable:
		return "satisfiable"
	case ClaimUnsatisfiable:
		return "unsatisfiable"
	case ClaimUnbounded:
		return "unbounded"
	}
	return "unknown"
}

// Strength is how a claim is supported, ranked so a greater Strength is never weaker
// evidence: Proved > Bounded > Witnessed (the one strength of an existential) > Observed > NotCovered.
type Strength int

const (
	// NotCovered claims nothing: a refusal, a solver unknown, a budget reached
	// without completing, a witness that did not replay.
	NotCovered Strength = iota
	// Observed means the claim held on the concrete executions that were run,
	// which fixed something the question left free.
	Observed
	// Witnessed means a concrete execution exhibits the claim and the
	// interpreter has replayed it.
	Witnessed
	// Bounded means the claim holds for everything free within the stated
	// bounds, which the result names.
	Bounded
	// Proved means the claim holds for everything the question left free, with
	// no bound reached.
	Proved
)

// String names the strength as a report prints it.
func (s Strength) String() string {
	switch s {
	case NotCovered:
		return "not covered"
	case Observed:
		return "observed"
	case Witnessed:
		return "witnessed"
	case Bounded:
		return "bounded"
	case Proved:
		return "proved"
	}
	return "unknown"
}

// Bound is one limit a run took, and whether the run reached it.
type Bound struct {
	// Name is the bound's, as the budget spells it: runs, depth, steps,
	// elements, solver, configurations.
	Name string
	// Limit is the value the run was held to: a count, or milliseconds for a time.
	Limit int64
	// Reached reports the run stopped at the limit rather than finishing within it.
	Reached bool
}

// String renders the bound as `name=limit`, a reached one marked `(reached)`.
func (b Bound) String() string {
	text := fmt.Sprintf("%s=%d", b.Name, b.Limit)
	if b.Reached {
		text += " (reached)"
	}
	return text
}

// Bounds is every bound a run took, in the order the budget names them.
type Bounds []Bound

// Reached reports whether any bound stopped the run.
func (b Bounds) Reached() bool {
	for _, bound := range b {
		if bound.Reached {
			return true
		}
	}
	return false
}

// Limit is the limit of the named bound, and whether the run took it at all.
func (b Bounds) Limit(name string) (int64, bool) {
	for _, bound := range b {
		if bound.Name == name {
			return bound.Limit, true
		}
	}
	return 0, false
}

// String renders the bounds as a report qualifies a claim by them: `runs=1024 depth=64`.
func (b Bounds) String() string {
	parts := make([]string, len(b))
	for i, bound := range b {
		parts[i] = bound.String()
	}
	return strings.Join(parts, " ")
}

// Witness is an execution the interpreter replays to exhibit a claim: the
// policy it ran under and the choices it took.
type Witness struct {
	Schedule runtime.SchedulePolicy
	Choices  []runtime.ChoiceTaken
}

// Evaluation is one thing the question asked for, as the engine established it: exactly
// one of Value, Row, Explored and Solved is set, or Err when this unit failed.
type Evaluation struct {
	// Name is the feature, output, row or query the evaluation is of.
	Name string
	// Value is a feature value or output.
	Value runtime.Value
	// Row is one row of a sweep.
	Row *runtime.SweepRow
	// Explored is the outcome set an exploration reached.
	Explored *runtime.Exploration
	// Solved is a solver's answer to one query.
	Solved *solve.Result
	// Err is what failed this unit.
	Err error
}

// Result is what one engine answered to one question.
type Result struct {
	Question Question
	// Engine names who answered, and its version where it has one.
	Engine string
	// Claim is what is asserted.
	Claim Claim
	// Strength is how the claim is supported.
	Strength Strength
	// Bounds is every bound the run took, and which were reached.
	Bounds Bounds
	// Witness is a schedule the interpreter replays, when the claim has one.
	Witness *Witness
	// Reason says why, when nothing is claimed: the construct, the unknown,
	// the budget, the disagreement.
	Reason string
	// Values are the feature values, outputs, rows or answers the question asked for.
	Values  []Evaluation
	Elapsed time.Duration
	// Workers is how many workers the plan built for the engine's runs, and Warming the
	// time building them took; a plan in the surface's own context builds none.
	Workers int
	Warming time.Duration
}

// Covered reports whether the result claims anything.
func (r Result) Covered() bool { return r.Strength != NotCovered }

// Table is the rows of a Sweep result as runtime.RunSweep tables them.
func (r Result) Table() runtime.SweepTable {
	var plan runtime.SweepPlan
	if r.Question.Sweep != nil {
		plan = r.Question.Sweep.Plan
	}
	table := runtime.NewSweepTable(r.Question.Subject, plan)
	for _, v := range r.Values {
		if v.Row != nil {
			table.Rows = append(table.Rows, *v.Row)
		}
	}
	return table
}

// Budget is what a run may spend; a zero field is the engine's own default. Runs, Depth and
// Solver are applied by the engines in their own units, Deadline by Registry.Answer to the
// plan's context; Steps and Memory name the context's.
type Budget struct {
	// Deadline is the wall clock for the whole plan, met as context.DeadlineExceeded; zero means none.
	Deadline time.Time
	// Jobs is how many runs may go concurrently; zero means one.
	Jobs int
	// Runs is how many runs an exploration, rows a sweep or queries a solver
	// may make; the unit is the engine's.
	Runs int
	// Depth is how many moves one run may resolve.
	Depth int
	// Steps is the step budget of one run, as OPENSYSML_MAX_STEPS names it.
	Steps int
	// Solver is the time one query may take, as OPENSYSML_SMT_TIMEOUT names it.
	Solver time.Duration
	// Memory is the elements one run may hold, as OPENSYSML_MAX_ELEMENTS names it.
	Memory int
}

// BudgetOf is the budget a question of kind has under limits and policy, its Runs in the
// kind's own unit: an exploring policy's runs for outcomes, the sweep runs for a sweep.
func BudgetOf(limits runtime.Budgets, policy runtime.SchedulePolicy, kind Kind) Budget {
	budget := Budget{Steps: int(limits.MaxSteps), Memory: int(limits.MaxElements)}
	exploring, ok := policy.Exploration()
	if ok {
		budget.Depth = exploring.Depth
	}
	switch kind {
	case Outcomes:
		if ok {
			budget.Runs = exploring.Runs
		}
	case Sweep:
		budget.Runs = int(limits.MaxSweepRuns)
	}
	return budget
}

// Exploration is the outcome set an Outcomes result reached, nil for any other result.
func (r Result) Exploration() *runtime.Exploration {
	for _, v := range r.Values {
		if v.Explored != nil {
			return v.Explored
		}
	}
	return nil
}
