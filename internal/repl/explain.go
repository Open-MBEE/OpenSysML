package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// ExplainSolve asks a solver which of an element's conditions conflict, for a
// constraint, requirement or satisfaction. Experimental: SysML v2 defines no
// solving, and the runtime evaluator remains normative.
func (s *Session) ExplainSolve(name string) []SolveReport {
	defer s.enter()()
	return s.explainSolve(name)
}

func (s *Session) explainSolve(name string) []SolveReport {
	queries, bad := s.solveQueries(name)
	if bad != nil {
		return []SolveReport{*bad}
	}
	explained, err := s.solveWith(name, queries, (*solve.Solver).Explain)
	if err != nil {
		return []SolveReport{unavailableReport(name, err.Error())}
	}
	reports := make([]SolveReport, 0, len(queries))
	for i, q := range queries {
		reports = append(reports, explainQueryReport(name, q, explained[i]))
	}
	return reports
}

// explainQueryReport renders the conflict the solver reports about one query,
// or says why there is none to render.
func explainQueryReport(name string, q *solve.Query, explained analysis.Evaluation) SolveReport {
	result, err := explained.Solved, explained.Err
	if err != nil {
		return unavailableReport(name, err.Error())
	}
	subject := solveSubject(q)
	switch result.Status {
	case solve.StatusUnsat:
		// A conflict resting on conditions the evaluator rounds is not an
		// evaluator conflict; one among exact conditions alone still is.
		if q.Rounded() && result.Core.Rounded() {
			return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: []string{
				fmt.Sprintf("? %s is undecided, so there is nothing to explain (%s)", subject, solveDetail(result)),
				"  " + roundedUnsat,
			}}
		}
		return SolveReport{Subject: name, Status: SolveUnsat, Solver: result.Solver,
			Lines: conflictLines(subject, result)}
	case solve.StatusSat:
		return SolveReport{Subject: name, Status: SolveSat, Solver: result.Solver, Lines: []string{
			fmt.Sprintf("✓ %s is satisfiable, so no conditions conflict (%s)", subject, solveDetail(result)),
			fmt.Sprintf("  Use %%check %s for a satisfying assignment.", name),
		}}
	default:
		lines := []string{fmt.Sprintf("? %s is undecided, so there is nothing to explain (%s)",
			subject, solveDetail(result))}
		if reason := solveReason(result, q); reason != "" {
			lines = append(lines, "  "+reason)
		}
		return SolveReport{Subject: name, Status: SolveUnknown, Solver: result.Solver, Lines: lines}
	}
}

// conflictLines renders an unsatisfiable verdict and the conditions that conflict,
// in the order the query asserts them.
func conflictLines(subject string, result *solve.Result) []string {
	core := result.Core
	if core == nil || len(core.Members) == 0 {
		// Explain returns a core with every unsat verdict, or an error instead.
		return []string{fmt.Sprintf("✗ %s is unsatisfiable, but no conflicting conditions were reported (%s)",
			subject, solveDetail(result))}
	}
	lines := []string{
		fmt.Sprintf("✗ %s is unsatisfiable: %s (%s)",
			subject, conflicting(len(core.Members)), solveDetail(result)),
		"  " + minimality(core, len(core.Members)),
	}
	for i, member := range core.Members {
		lines = append(lines, fmt.Sprintf("  %d. %s", i+1, conflictRow(member.From)))
	}
	return lines
}

// minimality says what the reported core is: one every member of was shown to be
// needed, or the solver's own, which need not be.
func minimality(core *solve.Core, members int) string {
	if core.Minimal {
		if members == 1 {
			return "The condition below is the whole conflict: nothing else is needed for it."
		}
		return "Every condition below is needed: dropping any one leaves the rest satisfiable."
	}
	msg := "These conditions conflict, but are not necessarily the smallest such set"
	if core.Note != "" {
		msg += ": " + core.Note
	}
	return msg + "."
}

// conflictRow describes one conflicting condition: its role, how it was written,
// what states it, and where.
func conflictRow(p solve.Provenance) string {
	row := fmt.Sprintf("%s: %s", p.Role, conditionText(p.Condition))
	row += fmt.Sprintf(" — %s %s", p.Kind, p.Element)
	if declarer := declaredBy(p); declarer != "" {
		row += ", declared by " + declarer
	}
	if p.Location != "" {
		row += ", at " + p.Location
	}
	return row
}

// declaredBy names the element that declared an inherited condition, and "" when
// the element already reported states it itself.
func declaredBy(p solve.Provenance) string {
	if p.Declared == nil || p.Declared.Name == "" {
		return ""
	}
	name := p.Declared.Name
	if name == p.Element || strings.HasSuffix(p.Element, "::"+name) {
		return ""
	}
	return name
}

// conditionText renders a condition as written, on one line so a row stays a row.
func conditionText(text string) string {
	return "`" + strings.Join(strings.Fields(text), " ") + "`"
}

// conflicting counts the conditions of a conflict, so a header reads as English.
func conflicting(n int) string {
	if n == 1 {
		return "1 condition conflicts with itself"
	}
	return fmt.Sprintf("%d conditions conflict", n)
}

// doExplain carries out %explain.
func (s *Session) doExplain(name string) ([]string, bool, error) {
	var out []string
	for _, r := range s.explainSolve(name) {
		out = append(out, r.Lines...)
	}
	return out, false, nil
}
