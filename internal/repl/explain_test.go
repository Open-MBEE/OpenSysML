package repl

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// conflictFixture is a session over the elements whose conditions conflict.
func conflictFixture(t *testing.T) *Session {
	t.Helper()
	return loadFixture(t, "testdata/explain_conflicts.sysml")
}

// The expected output of %explain over a two-condition contradiction, which is
// what the command promises: a verdict, what minimality was established, and each
// conflicting condition as written, with its role, element and location.
func TestExplainReportsTheConflictingConditions(t *testing.T) {
	requireSolver(t)
	s := conflictFixture(t)
	got := solverAgnostic(run(t, s, "%explain Contradictory"))
	want := strings.Join([]string{
		"✗ Constraint Contradictory is unsatisfiable: 2 conditions conflict (SOLVER)",
		"  Every condition below is needed: dropping any one leaves the rest satisfiable.",
		"  1. required condition: `i > 8` — constraint Contradictory, at <repl>:10:29",
		"  2. required condition: `i < 3` — constraint Contradictory, at <repl>:11:29",
		"  standing: unsatisfiable (SOLVER)",
	}, "\n")
	if got != want {
		t.Errorf("%%explain printed:\n%s\nwant:\n%s", got, want)
	}
	if reports := s.ExplainSolve("Contradictory"); len(reports) != 1 || reports[0].Status != SolveUnsat {
		t.Fatalf("ExplainSolve answered %v, want one unsat report", reports)
	}
}

// An exact-real conflict over conditions the evaluator rounds is no evaluator
// conflict: %explain reports it undecided rather than rendering the core.
func TestExplainLeavesRoundedUnsatUndecided(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, `
		package Explain {
			private import ScalarValues::Integer;
			constraint def HalfUlp {
				in a : Integer;
				assert constraint { a == 9007199254740993 }
				assert constraint { a / 2 == 4503599627370496.0 }
			}
		}`)
	got := run(t, s, "%explain HalfUlp")
	wants(t, got, "? Constraint HalfUlp is undecided, so there is nothing to explain",
		"rounds these conditions in floating point")
	rejects(t, got, "unsatisfiable")
}

// A conflict a condition only has with the domain its parameter was declared
// with names that domain rather than dropping it for not being a condition.
func TestExplainReportsADomainConflict(t *testing.T) {
	requireSolver(t)
	got := run(t, conflictFixture(t), "%explain NatBound")
	wants(t, got, "✗ Constraint NatBound is unsatisfiable",
		"declared domain: `a Natural is not negative`",
		"required condition: `n < 0`")
	rejects(t, got, "error:")
}

// A divisor guard is a real conflict: the conditions say the divisor is zero,
// which the guard the translation states for division refuses.
func TestExplainReportsADivisorGuardConflict(t *testing.T) {
	requireSolver(t)
	got := run(t, conflictFixture(t), "%explain ZeroDivisor")
	wants(t, got, "✗ Constraint ZeroDivisor is unsatisfiable",
		"well-definedness:", "required condition: `b == 0`")
}

// An inherited condition names the supertype that declared it, so a conflict in
// a specialized requirement can be traced to where each condition was written.
func TestExplainNamesTheDeclaringSupertype(t *testing.T) {
	requireSolver(t)
	got := run(t, conflictFixture(t), "%explain Derived")
	wants(t, got, "✗ Requirement Derived is unsatisfiable",
		"`x > 10` — requirement Derived, declared by Base",
		"`x < 1` — requirement Derived")
}

// A negated element asserts that its conditions do not all hold, so denying a
// tautology is itself the conflict.
func TestExplainReportsADeniedElement(t *testing.T) {
	requireSolver(t)
	got := run(t, conflictFixture(t), "%explain Conflicts::rig::always")
	wants(t, got, "1 condition conflicts with itself",
		"The condition below is the whole conflict: nothing else is needed for it.",
		"denied conditions: `not (i == i)`")
}

// There is nothing to explain about a satisfiable element, so no core is printed
// and %check is where the assignment is.
func TestExplainReportsNoConflictWhenSatisfiable(t *testing.T) {
	requireSolver(t)
	s := conflictFixture(t)
	got := solverAgnostic(run(t, s, "%explain Satisfiable"))
	want := strings.Join([]string{
		"✓ Constraint Satisfiable is satisfiable, so no conditions conflict (SOLVER)",
		"  Use %check Satisfiable for a satisfying assignment.",
		"  standing: satisfiable (SOLVER)",
	}, "\n")
	if got != want {
		t.Errorf("%%explain printed:\n%s\nwant:\n%s", got, want)
	}
	if reports := s.ExplainSolve("Satisfiable"); reports[0].Status != SolveSat {
		t.Errorf("status is %s, want sat", reports[0].Status)
	}
}

// %explain explains a satisfaction the same way it explains a constraint.
func TestExplainSolvesASatisfaction(t *testing.T) {
	requireSolver(t)
	s := loadFixture(t, "testdata/satisfy_landing.sysml")
	wants(t, run(t, s, "%explain touchdown"), "touchdown")
}

// An element outside the translatable subset is a typed error naming why, never
// a fabricated conflict.
func TestExplainReportsAnUntranslatableElement(t *testing.T) {
	s := checkSession(t, checkModel)
	got := run(t, s, "%explain Exponential")
	wants(t, got, "error:", "not translatable for solving")
	if reports := s.ExplainSolve("Exponential"); reports[0].Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", reports[0].Status)
	}
}

func TestExplainReportsAnUnknownName(t *testing.T) {
	wants(t, run(t, conflictFixture(t), "%explain Nope"), "error:")
}

func TestExplainWithoutAName(t *testing.T) {
	wants(t, run(t, conflictFixture(t), "%explain"), "usage: %explain <name>")
}

// An absent solver is a typed error naming what to install, and no verdict.
func TestExplainReportsAnAbsentSolver(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(solve.SolverEnv, "")
	s := conflictFixture(t)
	got := run(t, s, "%explain Contradictory")
	wants(t, got, "error:", "z3")
	rejects(t, got, "conflict", "unsatisfiable")
	if reports := s.ExplainSolve("Contradictory"); reports[0].Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", reports[0].Status)
	}
}

// A solver that answers nothing usable is a process failure, not an explanation.
func TestExplainReportsASolverProcessFailure(t *testing.T) {
	script := t.TempDir() + "/mute"
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	got := run(t, conflictFixture(t), "%explain Contradictory")
	wants(t, got, "error:", "mute")
	rejects(t, got, "conflict", "unknown")
}

// An undecided answer explains nothing, and says so with the solver's reason
// rather than reporting a conflict the solver did not find.
func TestExplainReportsAnUndecidedAnswer(t *testing.T) {
	script := t.TempDir() + "/undecided"
	fake := "#!/bin/sh\nprintf 'unknown\\n(:reason-unknown \"incomplete\")\\n'\nwhile read -r line; do :; done\n"
	if err := os.WriteFile(script, []byte(fake), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := conflictFixture(t)
	got := run(t, s, "%explain Contradictory")
	wants(t, got, "? Constraint Contradictory is undecided", "Reason: incomplete")
	rejects(t, got, "conflict", "unsatisfiable", "error:")
	if reports := s.ExplainSolve("Contradictory"); reports[0].Status != SolveUnknown {
		t.Errorf("status is %s, want unknown", reports[0].Status)
	}
}

func TestExplainIsListedInHelpAndCompletion(t *testing.T) {
	wants(t, strings.Join(helpText(), "\n"), "%explain <name>")
	found := false
	for _, name := range metaCommands() {
		if name == "%explain" {
			found = true
		}
	}
	if !found {
		t.Error("the explain command is not dispatched")
	}
}

// An %explain leaves an action debugging session running: it declares nothing.
func TestExplainKeepsADebuggingSession(t *testing.T) {
	requireSolver(t)
	s := loadFixture(t, "testdata/action_debug.sysml")
	before := run(t, s, "%action Debug::tally")
	if strings.Contains(before, "error:") {
		t.Fatalf("starting the debugger failed: %s", before)
	}
	s.Submit(conflictSource(t))
	run(t, s, "%explain Contradictory")
	rejects(t, run(t, s, "%tokens"), "no active")
}

// conflictSource is the conflicting model as text, for adding to a session that
// already holds another model.
func conflictSource(t *testing.T) string {
	t.Helper()
	text, err := os.ReadFile("testdata/explain_conflicts.sysml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(text)
}

// solverAgnostic replaces the solver name and timing in a verdict, so expected
// output does not depend on which solver answered or how fast.
func solverAgnostic(out string) string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if i := strings.LastIndex(line, " ("); i >= 0 && strings.HasSuffix(line, ")") {
			line = line[:i] + " (SOLVER)"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
