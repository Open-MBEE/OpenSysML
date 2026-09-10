package repl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// OptimizeSolve asks a solver for the best values an analysis case's objectives
// admit, over the conditions the case requires or assumes. Experimental: SysML v2
// defines no solving, and the runtime evaluator remains normative.
func (s *Session) OptimizeSolve(name string) []SolveReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.optimizeSolve(name)
}

func (s *Session) optimizeSolve(name string) []SolveReport {
	target, bad := s.resolveCheckTarget(name)
	if bad != nil {
		return []SolveReport{unavailableReport(name, strings.TrimPrefix(bad.Lines[0], "error: "))}
	}
	if err := runtime.RequireAnalysis(target.sym); err != nil {
		return []SolveReport{unavailableReport(name, err.Error())}
	}
	query, err := solve.Analysis(target.ctx, target.sym, target.scope)
	if err != nil {
		return []SolveReport{unavailableReport(name, err.Error())}
	}
	plan, err := s.solveWith(name, []*solve.Query{query}, optimizeExact)
	return solveReports(name, []*solve.Query{query}, plan, err, optimizeQueryReport)
}

// optimizeExact asks the solver for the query's optima, withholding a query the evaluator
// rounds: its optimum would be about exact reals, not the evaluator's arithmetic.
func optimizeExact(solver *solve.Solver, ctx context.Context, q *solve.Query) (*solve.Result, error) {
	if q.Rounded() {
		return nil, nil
	}
	return solver.Optimize(ctx, q)
}

// optimizeQueryReport renders the query's optima, or says why there is no
// optimum to render.
func optimizeQueryReport(name string, q *solve.Query, optimized analysis.Evaluation) SolveReport {
	result, err := optimized.Solved, optimized.Err
	if result == nil && err == nil {
		return SolveReport{Subject: name, Status: SolveUnknown, Lines: []string{
			fmt.Sprintf("? %s is undecided, so no optimum was reported", solveSubject(q)),
			"  " + roundedClaim,
		}}
	}
	if err != nil {
		return unavailableReport(name, err.Error())
	}
	subject := solveSubject(q)
	switch result.Status {
	case solve.StatusUnsat:
		return SolveReport{Subject: name, Status: SolveUnsat, Solver: result.Solver, Lines: []string{
			fmt.Sprintf("✗ %s has no values satisfying its conditions, so no objective can be met (%s)",
				subject, solveDetail(result)),
		}}
	case solve.StatusSat:
		status := optimizeStatus(result.Optima)
		lines := []string{optimizeHeader(status, subject, result)}
		for _, optimum := range result.Optima {
			lines = append(lines, optimumLines(optimum)...)
		}
		return SolveReport{Subject: name, Status: status, Solver: result.Solver,
			Lines: append(lines, assignmentLines(result.Model)...)}
	default:
		lines := []string{fmt.Sprintf("? %s is undecided, so no optimum was reported (%s)",
			subject, solveDetail(result))}
		if reason := solveReason(result, q); reason != "" {
			lines = append(lines, "  "+reason)
		}
		return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}
	}
}

// optimizeStatus is what the whole command answers: every objective optimized,
// or the weakest answer among them, since a report is no better than that.
func optimizeStatus(optima []solve.Optimum) SolveStatus {
	status := SolveSat
	for _, optimum := range optima {
		switch optimum.Status {
		case solve.OptimumAttained:
		case solve.OptimumUnbounded:
			return SolveUnbounded
		default:
			status = SolveNoOptimum
		}
	}
	return status
}

// optimizeHeader states what came of the command, one line, as a check's does.
func optimizeHeader(status SolveStatus, subject string, result *solve.Result) string {
	detail := solveDetail(result)
	switch status {
	case SolveUnbounded:
		return fmt.Sprintf("! %s has no optimum: an objective improves without limit (%s)", subject, detail)
	case SolveNoOptimum:
		return fmt.Sprintf("! %s is satisfiable, but its optimum was not established (%s)", subject, detail)
	default:
		return fmt.Sprintf("✓ %s is optimized (%s)", subject, detail)
	}
}

// optimumLines renders one objective's answer: the optimum when there is one,
// and otherwise what there is instead, never a value standing in for it.
func optimumLines(optimum solve.Optimum) []string {
	head := "  " + objectiveHeading(optimum.Objective)
	switch optimum.Status {
	case solve.OptimumAttained:
		return []string{head + ": " + optimum.Value}
	case solve.OptimumBounded:
		return append([]string{fmt.Sprintf("%s: no %s value, approaching %s without attaining it",
			head, extremeWord(optimum.Objective.Direction), optimum.Bound)}, feasibleLine(optimum)...)
	case solve.OptimumUnbounded:
		return append([]string{fmt.Sprintf("%s: no %s value — %s",
			head, extremeWord(optimum.Objective.Direction), optimum.Detail)}, feasibleLine(optimum)...)
	default:
		return append([]string{fmt.Sprintf("%s: no optimum reported — %s", head, optimum.Detail)},
			feasibleLine(optimum)...)
	}
}

// feasibleLine reports the value the solver's assignment attains, which is a
// value the conditions permit rather than an optimum.
func feasibleLine(optimum solve.Optimum) []string {
	if optimum.Feasible == "" {
		return nil
	}
	return []string{"    the assignment below attains " + optimum.Feasible}
}

// objectiveHeading names an objective as it is declared: which way it is improved,
// what it is called, and the value it states.
func objectiveHeading(obj solve.Objective) string {
	heading := obj.Direction.String() + " " + obj.Name
	if obj.Expression != "" {
		heading += " = " + conditionText(obj.Expression)
	}
	return heading
}

// extremeWord names the value an objective's direction asks for.
func extremeWord(dir solve.Direction) string {
	if dir == solve.Maximize {
		return "greatest"
	}
	return "least"
}

// doOptimize carries out %optimize.
func (s *Session) doOptimize(name string) ([]string, bool, error) {
	var out []string
	for _, r := range s.optimizeSolve(name) {
		out = append(out, r.Lines...)
	}
	return out, false, nil
}
