package repl

import (
	"fmt"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// SolveStatus is what a solver answered about an element's conditions. It is
// kept apart from VerdictStatus: satisfiability is not evaluation.
type SolveStatus int

const (
	// SolveSat means an assignment satisfying the conditions exists.
	SolveSat SolveStatus = iota
	// SolveUnsat means no assignment satisfies them.
	SolveUnsat
	// SolveUnknown means the solver did not decide: it timed out, or gave up on
	// arithmetic it cannot decide.
	SolveUnknown
	// SolveUnbounded means the conditions are satisfiable but an objective
	// improves without limit, so it has no optimum.
	SolveUnbounded
	// SolveNoOptimum means the conditions are satisfiable and an objective's
	// optimum was not established: only a bound, or an unverified answer.
	SolveNoOptimum
	// SolveUnavailable means nothing was asked: no solver is installed, the
	// element is outside the translatable subset, or the solver failed.
	SolveUnavailable
)

// String names the status for a report a machine reads.
func (s SolveStatus) String() string {
	switch s {
	case SolveSat:
		return "sat"
	case SolveUnsat:
		return "unsat"
	case SolveUnknown:
		return "unknown"
	case SolveUnbounded:
		return "unbounded"
	case SolveNoOptimum:
		return "no-optimum"
	default:
		return "unavailable"
	}
}

// SolveReport is what a solver answered about one element, with the lines the
// REPL prints for it.
type SolveReport struct {
	// Subject is what was checked, spelled as the caller named it.
	Subject string
	Status  SolveStatus
	Lines   []string
	// Solver names the solver that answered, empty when none did.
	Solver string
	// Plan is how the engines answered; nil for a report made before any was asked.
	Plan *analysis.Plan
}

// Satisfiable reports whether the solver found the conditions satisfiable.
func (r SolveReport) Satisfiable() bool { return r.Status == SolveSat }

// CheckSolve asks a solver whether the named constraint, requirement or
// satisfaction can be satisfied at all. Experimental: SysML v2 defines no solving.
func (s *Session) CheckSolve(name string) []SolveReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkSolve(name)
}

func (s *Session) checkSolve(name string) []SolveReport {
	queries, bad := s.solveQueries(name)
	if bad != nil {
		return []SolveReport{*bad}
	}
	plan, err := s.solveWith(name, queries, (*solve.Solver).Solve)
	return solveReports(name, queries, plan, err, solveQueryReport)
}

// solveQueryReport renders the solver's answer about one query.
func solveQueryReport(name string, q *solve.Query, solved analysis.Evaluation) SolveReport {
	result, err := solved.Solved, solved.Err
	if err != nil {
		return unavailableReport(name, err.Error())
	}
	subject := solveSubject(q)
	switch result.Status {
	case solve.StatusSat:
		lines := []string{fmt.Sprintf("✓ %s is satisfiable (%s)", subject, solveDetail(result))}
		return SolveReport{Subject: name, Status: SolveSat, Solver: result.Solver,
			Lines: append(lines, assignmentLines(result.Model)...)}
	case solve.StatusUnsat:
		if report, rounded := roundedUnsatReport(name, subject, result, q); rounded {
			return report
		}
		return SolveReport{Subject: name, Status: SolveUnsat, Solver: result.Solver, Lines: []string{
			fmt.Sprintf("✗ %s is unsatisfiable (%s)", subject, solveDetail(result)),
		}}
	default:
		lines := []string{fmt.Sprintf("? %s is undecided (%s)", subject, solveDetail(result))}
		if reason := solveReason(result, q); reason != "" {
			lines = append(lines, "  "+reason)
		}
		return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}
	}
}

// roundedUnsat explains an exact-real unsat left undecided: the evaluator
// rounds these conditions in float64, which the exact encoding does not model.
const roundedUnsat = "Reason: no exact-real values satisfy it, but the evaluator rounds these conditions in floating point, which may still accept values"

// roundedClaim explains a solver claim withheld outright: what it would state
// about exact reals does not decide the evaluator's floating-point arithmetic.
const roundedClaim = "Reason: the conditions round in floating point when evaluated, which the exact-real encoding does not decide"

// roundedUnsatReport downgrades an unsat about conditions the evaluator rounds:
// exact-real unsatisfiability does not decide the evaluator's own arithmetic.
func roundedUnsatReport(name, subject string, result *solve.Result, q *solve.Query) (SolveReport, bool) {
	if !q.Rounded() {
		return SolveReport{}, false
	}
	return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: []string{
		fmt.Sprintf("? %s is undecided (%s)", subject, solveDetail(result)),
		"  " + roundedUnsat,
	}}, true
}

// solveSubject names the element a report is about, as a verdict about it would.
func solveSubject(q *solve.Query) string {
	subject := strings.ToUpper(q.Kind[:1]) + q.Kind[1:] + " " + q.Element
	if q.Negated {
		return subject + " (negated)"
	}
	return subject
}

// solveDetail is the solver and the time it took, as the report's suffix.
func solveDetail(result *solve.Result) string {
	return fmt.Sprintf("%s, %s", result.Solver, result.Elapsed.Round(time.Millisecond))
}

// solveReason explains an undecided answer: what the solver said, and that the
// query left the arithmetic it decides when it said nothing.
func solveReason(result *solve.Result, q *solve.Query) string {
	switch {
	case result.Reason != "":
		return "Reason: " + result.Reason
	case q.Nonlinear:
		return "Reason: the query uses nonlinear arithmetic"
	default:
		return ""
	}
}

// assignmentLines renders a satisfying model, one indented line per variable.
func assignmentLines(model []solve.Assignment) []string {
	out := make([]string, 0, len(model))
	for _, a := range model {
		line := fmt.Sprintf("  %s = %s", notationName(a.Var.Name), a.Value)
		if !a.Rendered {
			line += "  (as the solver wrote it)"
		}
		out = append(out, line)
	}
	return out
}

// unavailableReport reports a check that was never made, phrased as the prompt
// reports any command it cannot carry out.
func unavailableReport(subject, msg string) SolveReport {
	return SolveReport{Subject: subject, Status: SolveUnavailable, Lines: []string{"error: " + msg}}
}

// solveQueries translates what the named element states, leaving every value
// free. Its second result is non-nil for a check that cannot be made at all.
func (s *Session) solveQueries(name string) ([]*solve.Query, *SolveReport) {
	queries, _, bad := s.solveQueriesWith(name, nil)
	return queries, bad
}

// pinner supplies the values a query about subject fixes, given the resolved
// element; subject is the element the query is translated from, which for a
// satisfaction is the requirement rather than the element asserting it.
type pinner func(target checkTarget, subject *symbols.Symbol) ([]solve.Pin, []solve.Unfixed, *SolveReport)

// solveQueriesWith translates what the named element states, fixing the values
// the pinner supplies. Its second result names the features whose value could not
// be read, which stay free.
func (s *Session) solveQueriesWith(name string, pins pinner) ([]*solve.Query, []solve.Unfixed, *SolveReport) {
	target, bad := s.resolveCheckTarget(name)
	if bad != nil {
		report := unavailableReport(name, strings.TrimPrefix(bad.Lines[0], "error: "))
		return nil, nil, &report
	}
	fixed, unfixed, prob := s.fixedFor(target, target.sym, pins)
	if prob != nil {
		return nil, nil, prob
	}
	switch {
	case runtime.RequireConstraint(target.sym) == nil:
		q, err := solve.ConstraintWith(target.ctx, target.sym, target.scope, fixed)
		return oneQuery(name, q, unfixed, err)
	case runtime.RequireRequirement(target.sym) == nil:
		q, err := solve.RequirementWith(target.ctx, target.sym, target.scope, fixed)
		return oneQuery(name, q, unfixed, err)
	}
	if a, err := target.ctx.SatisfyAssertionOf(target.sym); err == nil {
		fixed, unfixed, prob := s.fixedFor(target, a.Symbol, pins)
		if prob != nil {
			return nil, nil, prob
		}
		q, qerr := solve.SatisfactionWith(target.ctx, a, fixed)
		return oneQuery(name, q, unfixed, qerr)
	}
	return s.satisfactionQueries(name, target, pins)
}

// fixedFor asks the pinner for the values a query about subject fixes, and fixes
// none when there is no pinner.
func (s *Session) fixedFor(
	target checkTarget,
	subject *symbols.Symbol,
	pins pinner,
) ([]solve.Pin, []solve.Unfixed, *SolveReport) {
	if pins == nil {
		return nil, nil, nil
	}
	return pins(target, subject)
}

// satisfactionQueries translates every satisfaction assertion an element states,
// which is how an anonymous `assert satisfy` is reached.
func (s *Session) satisfactionQueries(
	name string,
	target checkTarget,
	pins pinner,
) ([]*solve.Query, []solve.Unfixed, *SolveReport) {
	if target.sym.Scope == nil {
		report := unavailableReport(name, fmt.Sprintf("%s is not a constraint, requirement or satisfaction assertion", name))
		return nil, nil, &report
	}
	assertions := target.ctx.SatisfyAssertionsIn(target.sym.Scope)
	if len(assertions) == 0 {
		report := unavailableReport(name, fmt.Sprintf("no satisfaction assertion in %s", target.fqn))
		return nil, nil, &report
	}
	queries := make([]*solve.Query, 0, len(assertions))
	var unfixed []solve.Unfixed
	for _, a := range assertions {
		fixed, notRead, prob := s.fixedFor(target, a.Symbol, pins)
		if prob != nil {
			return nil, nil, prob
		}
		unfixed = append(unfixed, notRead...)
		q, err := solve.SatisfactionWith(target.ctx, a, fixed)
		if err != nil {
			report := unavailableReport(name, err.Error())
			return nil, nil, &report
		}
		queries = append(queries, q)
	}
	return queries, unfixed, nil
}

// oneQuery is a single translated query, or the report explaining why the
// element could not be translated.
func oneQuery(name string, q *solve.Query, unfixed []solve.Unfixed, err error) ([]*solve.Query, []solve.Unfixed, *SolveReport) {
	if err != nil {
		report := unavailableReport(name, err.Error())
		return nil, nil, &report
	}
	return []*solve.Query{q}, unfixed, nil
}

// doCheck carries out %check.
func (s *Session) doCheck(name string) ([]string, bool, error) {
	var out []string
	for _, r := range s.checkSolve(name) {
		out = append(out, r.Lines...)
	}
	return out, false, nil
}
