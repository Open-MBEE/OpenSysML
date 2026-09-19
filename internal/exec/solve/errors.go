package solve

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ErrNotTranslatable is returned when a condition uses a construct outside the
// translatable subset. A query that cannot encode one conjunct fails as a whole:
// a partial script would answer sat or unsat about conditions it does not hold.
var ErrNotTranslatable = errors.New("not translatable for solving")

// ErrNoConditions is returned for an element that states no condition to
// translate, as evaluating it reports the same.
var ErrNoConditions = errors.New("states no condition")

// ErrNoObjective is returned for an analysis case that states no objective, so
// there is nothing to optimize: what it permits is still a satisfiability
// question, which checking it plainly answers.
var ErrNoObjective = errors.New("states no objective")

// ErrNotOptimizable is returned for an objective that cannot be optimized as
// written: it states no direction, no value, or a value an optimizer cannot
// improve. It is a refusal about the objective rather than about the subset a
// condition is translated in.
var ErrNotOptimizable = errors.New("not optimizable")

// ErrNoOptimization is returned when the solver found does not implement
// optimization. `(maximize …)` is a z3 extension rather than SMT-LIB, so this is
// reported rather than degraded to a plain satisfiability check.
var ErrNoOptimization = errors.New("the SMT solver does not implement optimization")

// ErrNoOptimum is returned when a solver answered sat but did not report the
// optimum readably. No optimum is invented in its place.
var ErrNoOptimum = errors.New("the SMT solver did not report the optimum")

// NoObjectiveError says which analysis case states no objective. It unwraps to
// ErrNoObjective.
type NoObjectiveError struct {
	// Element names the analysis case asked about.
	Element string
}

// Error reports that the case states no objective.
func (e *NoObjectiveError) Error() string {
	return fmt.Sprintf("analysis %s: %s", e.Element, ErrNoObjective)
}

// Unwrap returns ErrNoObjective.
func (e *NoObjectiveError) Unwrap() error { return ErrNoObjective }

// ObjectiveError says which objective cannot be optimized, why, and where it was
// written. It unwraps to ErrNotOptimizable.
type ObjectiveError struct {
	// Objective names the objective as the model writes it.
	Objective string

	// Reason says what about it cannot be optimized.
	Reason string

	// Remedy says what the model would have to state instead, empty when there
	// is nothing to suggest.
	Remedy string

	// Element is the analysis case stating the objective.
	Element string

	// File is the document it was written in, empty when unknown.
	File string

	// Span is where in File it was written.
	Span source.Span

	// Location renders File and Span as `file:line:col`, empty when unknown.
	Location string
}

// Error reports the refusal, naming the objective and where it was written.
func (e *ObjectiveError) Error() string {
	msg := fmt.Sprintf("%s: %s %s", e.Element, e.Objective, ErrNotOptimizable)
	if e.Reason != "" {
		msg += ": it " + e.Reason
	}
	if e.Remedy != "" {
		msg += " (" + e.Remedy + ")"
	}
	if e.Location != "" {
		msg += " at " + e.Location
	}
	return msg
}

// Unwrap returns ErrNotOptimizable.
func (e *ObjectiveError) Unwrap() error { return ErrNotOptimizable }

// NoOptimizationError names the solver that does not implement optimization and
// what to run instead. It unwraps to ErrNoOptimization.
type NoOptimizationError struct {
	// Solver is the executable that was run.
	Solver string

	// Detail says how it turned out not to implement optimization.
	Detail string

	// Cause is the capability refusal this was settled by, when one settled it.
	Cause error
}

// Error reports that the solver cannot optimize, naming what can.
func (e *NoOptimizationError) Error() string {
	msg := fmt.Sprintf("%s: %s", ErrNoOptimization, e.Solver)
	if e.Detail != "" {
		msg += " " + e.Detail
	}
	return msg + fmt.Sprintf("; install z3 or set %s to it", SolverEnv)
}

// Unwrap returns ErrNoOptimization, and the capability refusal behind it when the
// capability model settled it.
func (e *NoOptimizationError) Unwrap() []error {
	if e.Cause != nil {
		return []error{ErrNoOptimization, e.Cause}
	}
	return []error{ErrNoOptimization}
}

// OptimumError says which solver would not report an optimum it had found, and
// how. It unwraps to both ErrNoOptimum and ErrSolverProcess, as the solver did
// not answer what it was asked.
type OptimumError struct {
	// Solver is the executable that was run.
	Solver string

	// Objective names the objective it was asked about.
	Objective string

	// Detail says what it answered instead of an optimum.
	Detail string

	// Stderr is what the solver wrote on standard error, trimmed.
	Stderr string
}

// Error reports the failure, naming the solver and what it answered.
func (e *OptimumError) Error() string {
	msg := fmt.Sprintf("%s: %s answered sat but %s", ErrNoOptimum, e.Solver, e.Detail)
	if e.Objective != "" {
		msg += " for " + e.Objective
	}
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// Unwrap returns both kinds this failure is, so either is testable with
// errors.Is.
func (e *OptimumError) Unwrap() []error { return []error{ErrNoOptimum, ErrSolverProcess} }

// ErrNoSolver is returned when no SMT solver could be found to run a query.
// Solving is optional, so its absence is reported rather than passed over.
var ErrNoSolver = errors.New("no SMT solver found")

// ErrSolverProcess is returned when a solver was found but did not answer: it
// crashed, failed, or replied unintelligibly. Never a verdict, not even `unknown`.
var ErrSolverProcess = errors.New("the SMT solver did not answer")

// ErrNoCore is returned when a solver answered unsat but would not report which
// assertions conflict, or named assertions the query never asserted. No core is
// invented in its place.
var ErrNoCore = errors.New("the SMT solver did not report an unsat core")

// ErrUnsupportedCapability is returned when the backend found lacks a feature the
// query needs. It is a refusal, not a verdict: nothing is degraded and no answer
// is guessed from a script the backend would not accept.
var ErrUnsupportedCapability = errors.New("the SMT solver does not support a feature this query needs")

// UnsupportedCapabilityError names the backend, the features it lacks, and what
// was being asked of it. It unwraps to ErrUnsupportedCapability.
type UnsupportedCapabilityError struct {
	// Solver is the backend that lacks the capability.
	Solver string

	// Operation is what was being asked of it: "solving", "explaining a
	// conflict", "enumerating configurations".
	Operation string

	// Missing are the capabilities it lacks, the first being the one reported.
	Missing []Capability

	// Detail is what it answered when probed for the first missing capability.
	Detail string
}

// Error names the missing feature, the backend, and how to get a backend that has
// it, since the choice of solver is the operator's.
func (e *UnsupportedCapabilityError) Error() string {
	msg := fmt.Sprintf("%s: %s does not support %s, which %s needs",
		ErrUnsupportedCapability, e.Solver, e.feature(), e.Operation)
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	return msg + fmt.Sprintf("; install a solver that supports it or set %s to one", SolverEnv)
}

// feature names the missing capabilities, the first with what SMT-LIB feature it
// is and the rest by name.
func (e *UnsupportedCapabilityError) feature() string {
	if len(e.Missing) == 0 {
		return "a feature it was not asked about"
	}
	out := e.Missing[0].Feature() + " (" + e.Missing[0].String() + ")"
	for _, capability := range e.Missing[1:] {
		out += ", nor " + capability.String()
	}
	return out
}

// Unwrap returns ErrUnsupportedCapability.
func (e *UnsupportedCapabilityError) Unwrap() error { return ErrUnsupportedCapability }

// CoreError says which solver would not explain its unsat verdict and how. It
// unwraps to both ErrNoCore and ErrSolverProcess, as the solver did not answer
// what it was asked.
type CoreError struct {
	// Solver is the executable that was run.
	Solver string

	// Detail says what it answered instead of a core.
	Detail string

	// Stderr is what the solver wrote on standard error, trimmed.
	Stderr string
}

// Error reports the failure, naming the solver and what it answered.
func (e *CoreError) Error() string {
	msg := fmt.Sprintf("%s: %s answered unsat but %s", ErrNoCore, e.Solver, e.Detail)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// Unwrap returns both kinds this failure is, so either is testable with
// errors.Is.
func (e *CoreError) Unwrap() []error { return []error{ErrNoCore, ErrSolverProcess} }

// NoSolverError names the candidates looked for and what to install. It unwraps
// to ErrNoSolver.
type NoSolverError struct {
	// Override is the value of the OPENSYSML_SMT override, empty when unset.
	Override string

	// Looked lists the executables looked for on PATH, in order.
	Looked []string
}

// Error reports that no solver was found, naming what would satisfy the search.
func (e *NoSolverError) Error() string {
	if e.Override != "" {
		return fmt.Sprintf("%s: %s names %q, which is not an executable file",
			ErrNoSolver, SolverEnv, e.Override)
	}
	return fmt.Sprintf("%s: install z3 (`apt install z3`, `brew install z3`) or cvc5, "+
		"or set %s to a solver executable; looked for %v on PATH",
		ErrNoSolver, SolverEnv, e.Looked)
}

// Unwrap returns ErrNoSolver.
func (e *NoSolverError) Unwrap() error { return ErrNoSolver }

// SolverProcessError says which solver failed, at which step, and what it wrote
// on standard error. It unwraps to ErrSolverProcess.
type SolverProcessError struct {
	// Solver is the executable that was run.
	Solver string

	// Stage names what was being done: "start", "write", "check-sat",
	// "get-value", "capability check" or "exit".
	Stage string

	// Detail says what went wrong.
	Detail string

	// Stderr is what the solver wrote on standard error, trimmed.
	Stderr string

	// Err is the underlying error, nil when the failure was the reply itself.
	Err error
}

// Error reports the failure, naming the solver and the step it failed at.
func (e *SolverProcessError) Error() string {
	msg := fmt.Sprintf("%s: %s failed at %s: %s", ErrSolverProcess, e.Solver, e.Stage, e.Detail)
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

// Unwrap returns ErrSolverProcess, and the underlying error when there is one,
// so both are testable with errors.Is.
func (e *SolverProcessError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrSolverProcess}
	}
	return []error{ErrSolverProcess, e.Err}
}

// NotTranslatableError says which construct refused, why, and where it was
// written. It unwraps to ErrNotTranslatable.
type NotTranslatableError struct {
	// Construct names the construct that refused, as the notation writes it.
	Construct string

	// Reason says why it is outside the subset.
	Reason string

	// Element is the element whose condition was being translated.
	Element string

	// Condition is the condition as written, empty when the refusal is about a
	// declaration rather than a condition.
	Condition string

	// File is the document the construct was written in, empty when unknown.
	File string

	// Span is where in File it was written.
	Span source.Span

	// Location renders File and Span as `file:line:col`, empty when unknown.
	Location string
}

// Error reports the refusal, naming the construct, the condition it appeared in,
// and where it was written.
func (e *NotTranslatableError) Error() string {
	msg := fmt.Sprintf("%s: %s %s", e.Element, e.Construct, ErrNotTranslatable)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.Condition != "" {
		msg += fmt.Sprintf(" (in %s)", e.Condition)
	}
	if e.Location != "" {
		msg += " at " + e.Location
	}
	return msg
}

// Unwrap returns ErrNotTranslatable, so a caller tests the kind of failure
// rather than its text.
func (e *NotTranslatableError) Unwrap() error { return ErrNotTranslatable }
