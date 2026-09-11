package analysis

import (
	"context"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// Kind is what a question asks of its subject.
type Kind int

const (
	// Evaluate asks for one execution under stated bindings and a stated
	// scheduling policy: a calc, a case, a behavior, or a condition on a subject.
	Evaluate Kind = iota
	// Outcomes asks what a behavior can do over every schedule.
	Outcomes
	// Holds asks whether a condition holds over a behavior or a case, with the
	// schedule and/or the unbound inputs free within stated bounds.
	Holds
	// Sensitive asks whether a behavior's result depends on its schedule.
	Sensitive
	// Satisfiable asks whether some assignment to the unbound features meets a
	// set of conditions.
	Satisfiable
	// Sweep asks a calc or case once per row of a domain the rows enumerate or sample.
	Sweep
	// Compute asks an external tool once, as an action annotated ToolExecution names it.
	Compute
)

// String names the kind as a plan spells it.
func (k Kind) String() string {
	switch k {
	case Evaluate:
		return "evaluate"
	case Outcomes:
		return "outcomes"
	case Holds:
		return "holds"
	case Sensitive:
		return "sensitive"
	case Satisfiable:
		return "satisfiable"
	case Sweep:
		return "sweep"
	case Compute:
		return "compute"
	}
	return "unknown"
}

// Freedom is what a question leaves free; a set of the flags below.
type Freedom int

const (
	// FreeNothing fixes every choice: one execution, or one per row.
	FreeNothing Freedom = 0
	// FreeSchedule leaves the scheduling of concurrent behavior open.
	FreeSchedule Freedom = 1 << iota
	// FreeInputs leaves unbound inputs open.
	FreeInputs
)

// Has reports whether the freedom includes f.
func (f Freedom) Has(flag Freedom) bool { return f&flag != 0 }

// String names what is free: `nothing`, `the schedule`, `the inputs`, or both.
func (f Freedom) String() string {
	var parts []string
	if f.Has(FreeSchedule) {
		parts = append(parts, "the schedule")
	}
	if f.Has(FreeInputs) {
		parts = append(parts, "the inputs")
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, " and ")
}

// Question is what is asked, independently of who answers it: a subject, what is asked of
// it, what is fixed and what is free, and the ask for its kind saying how one unit is made.
type Question struct {
	Kind Kind
	// Subject names the element asked about, as the surface spelled it.
	Subject string
	// Schedule is the scheduling policy the question states: the one an
	// evaluation runs under, or the exploring one whose budget bounds outcomes.
	Schedule runtime.SchedulePolicy
	// Free is what the question leaves open.
	Free Freedom
	// Perform makes the one execution an Evaluate question asks for.
	Perform Performance
	// Linearize makes one run of the behavior an Outcomes question asks about,
	// in a context of the run's own.
	Linearize Linearization
	// Sweep is the domain and the row of a Sweep question.
	Sweep *SweepAsk
	// Solve is the queries of a Satisfiable question and how each is asked.
	Solve *SolveAsk
	// Compute is the tool invocation a Compute question asks for.
	Compute *ComputeAsk
}

// Performance makes one execution in the given context and reports what it established.
// An error is a fault, not a failed execution, which is an Answer claiming nothing.
type Performance func(*runtime.Context) (Answer, error)

// Linearization runs the behavior once in a fresh context under the context's
// schedule, in the shape runtime.Explore drives.
type Linearization func(*runtime.Context) (runtime.Outcome, error)

// Answer is what one performance established: the claim it supports, the
// values it produced, and why when it claims nothing.
type Answer struct {
	Claim Claim
	// Reason says why nothing is claimed when no error does.
	Reason string
	// Err is the failure that ended the execution: a typed runtime error, whose
	// budget the engine reports as a bound reached.
	Err    error
	Values []Evaluation
}

// SweepAsk is the plan of a sweep and the run of one row of it.
type SweepAsk struct {
	Plan runtime.SweepPlan
	Row  runtime.SweepRun
}

// SolveAsk is the queries a satisfiability question puts to a solver, one per
// condition set, and how each is asked.
type SolveAsk struct {
	Queries []*solve.Query
	Ask     Asking
}

// Asking puts one query to the solver in a method expression's shape. An error is that
// query's failure, not the question's; nil for both is a query withheld, which stays uncovered.
type Asking func(*solve.Solver, context.Context, *solve.Query) (*solve.Result, error)
