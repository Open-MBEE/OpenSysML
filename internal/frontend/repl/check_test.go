package repl

import (
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
)

// solverRequiredEnv makes an absent solver a failure rather than a skip, so CI
// runs these checks instead of quietly passing without a solver.
const solverRequiredEnv = "OPENSYSML_REQUIRE_SMT"

// requireSolver skips a solver-dependent check when no solver is installed,
// unless OPENSYSML_REQUIRE_SMT says one must be.
func requireSolver(t *testing.T) {
	t.Helper()
	if _, err := solve.Discover(); err != nil {
		if os.Getenv(solverRequiredEnv) != "" {
			t.Fatalf("%s is set but no solver was found: %v", solverRequiredEnv, err)
		}
		t.Skipf("no SMT solver installed: %v", err)
	}
}

// checkSession is a session holding one model, ready for %check.
func checkSession(t *testing.T, src string) *Session {
	t.Helper()
	s := NewSession()
	if res := s.Submit(src); len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	return s
}

const checkModel = `
package Check {
	private import ScalarValues::Integer;

	constraint def Satisfiable {
		in i : Integer;
		assert constraint { i > 3 and i < 8 }
	}

	constraint def Contradictory {
		in i : Integer;
		assert constraint { i > 8 and i < 3 }
	}

	requirement def Ranged {
		subject i : Integer;
		require constraint { i >= 0 }
	}

	constraint def Exponential {
		in i : Integer;
		assert constraint { i ** 2 == 4 }
	}
}
`

func TestCheckReportsSatisfiableWithAnAssignment(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, checkModel)
	got := run(t, s, "%check Satisfiable")
	wants(t, got, "✓ Constraint Satisfiable is satisfiable", "Check::Satisfiable::i = ")
	rejects(t, got, "error:")

	reports := s.CheckSolve("Satisfiable")
	if len(reports) != 1 || reports[0].Status != SolveSat {
		t.Fatalf("CheckSolve answered %v, want one sat report", reports)
	}
	if !reports[0].Satisfiable() || reports[0].Solver == "" {
		t.Errorf("report %+v does not name the solver that answered", reports[0])
	}
}

func TestCheckReportsUnsatisfiable(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, checkModel)
	got := run(t, s, "%check Contradictory")
	wants(t, got, "✗ Constraint Contradictory is unsatisfiable")
	rejects(t, got, "error:", "=")

	if reports := s.CheckSolve("Contradictory"); reports[0].Status != SolveUnsat {
		t.Errorf("status is %s, want unsat", reports[0].Status)
	}
}

// An exact-real unsat about conditions the evaluator rounds is reported
// undecided: the evaluator's float64 arithmetic may still accept values.
func TestCheckLeavesRoundedUnsatUndecided(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, `
		package Check {
			private import ScalarValues::Integer;
			constraint def HalfUlp {
				in a : Integer;
				assert constraint { a == 9007199254740993 }
				assert constraint { a / 2 == 4503599627370496.0 }
			}
		}`)
	got := run(t, s, "%check HalfUlp")
	wants(t, got, "? Constraint HalfUlp is undecided", "rounds these conditions in floating point")
	rejects(t, got, "unsatisfiable")

	if reports := s.CheckSolve("HalfUlp"); reports[0].Status != SolveUnknown {
		t.Errorf("status is %s, want unknown", reports[0].Status)
	}
}

func TestCheckSolvesARequirement(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, checkModel)
	got := run(t, s, "%check Ranged")
	wants(t, got, "✓ Requirement Ranged is satisfiable")
}

// A verdict about satisfiability is not a verdict about evaluation: %check
// answers `sat` where %constraint cannot evaluate the unbound parameter at all.
func TestCheckIsDistinctFromConstraintEvaluation(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, checkModel)
	if v := s.CheckConstraint("Satisfiable"); v.Status == VerdictHolds {
		t.Fatalf("expected %%constraint not to hold on an unbound parameter, got %v", v.Lines)
	}
	if reports := s.CheckSolve("Satisfiable"); reports[0].Status != SolveSat {
		t.Errorf("status is %s, want sat", reports[0].Status)
	}
}

func TestCheckReportsAnUntranslatableElement(t *testing.T) {
	s := checkSession(t, checkModel)
	got := run(t, s, "%check Exponential")
	wants(t, got, "error:", "not translatable for solving")

	if reports := s.CheckSolve("Exponential"); reports[0].Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", reports[0].Status)
	}
}

func TestCheckRejectsAnElementStatingNothingToSolve(t *testing.T) {
	s := checkSession(t, "package Check { part def P; }")
	got := run(t, s, "%check P")
	wants(t, got, "error:")
}

func TestCheckReportsAnUnknownName(t *testing.T) {
	s := checkSession(t, checkModel)
	wants(t, run(t, s, "%check Nope"), "error:")
}

func TestCheckWithoutAName(t *testing.T) {
	s := checkSession(t, checkModel)
	wants(t, run(t, s, "%check"), "usage: %check <name>")
}

// An absent solver is a typed error naming what to install, never a fabricated
// verdict, which the prompt reports as it reports any command it cannot carry out.
func TestCheckReportsAnAbsentSolver(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(solve.SolverEnv, "")
	s := checkSession(t, checkModel)
	got := run(t, s, "%check Satisfiable")
	wants(t, got, "error:", "z3")
	rejects(t, got, "satisfiable", "unsatisfiable")

	if reports := s.CheckSolve("Satisfiable"); reports[0].Status != SolveUnavailable {
		t.Errorf("status is %s, want unavailable", reports[0].Status)
	}
}

// A solver that answers nothing usable is a process failure, distinguished from
// `unknown`: no verdict was reached at all.
func TestCheckReportsASolverProcessFailure(t *testing.T) {
	dir := t.TempDir()
	script := dir + "/mute"
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := checkSession(t, checkModel)
	got := run(t, s, "%check Satisfiable")
	wants(t, got, "error:", "mute")
	rejects(t, got, "unknown", "is satisfiable")
}

// An undecided answer stays undecided: it is reported as neither satisfiable
// nor unsatisfiable, with the reason the solver gave.
func TestCheckReportsAnUndecidedAnswer(t *testing.T) {
	script := t.TempDir() + "/undecided"
	// Both replies are written up front, then the script drains the query with
	// shell builtins alone so it outlives every write the driver makes.
	fake := "#!/bin/sh\nprintf 'unknown\\n(:reason-unknown \"incomplete\")\\n'\nwhile read -r line; do :; done\n"
	if err := os.WriteFile(script, []byte(fake), 0o700); err != nil { // #nosec G306 -- a test's own executable
		t.Fatalf("write fake solver: %v", err)
	}
	t.Setenv(solve.SolverEnv, script)
	s := checkSession(t, checkModel)
	got := run(t, s, "%check Satisfiable")
	wants(t, got, "? Constraint Satisfiable is undecided", "Reason: incomplete")
	rejects(t, got, "is satisfiable", "is unsatisfiable", "error:")

	if reports := s.CheckSolve("Satisfiable"); reports[0].Status != SolveUnknown {
		t.Errorf("status is %s, want unknown: %s", reports[0].Status, strings.Join(reports[0].Lines, "\n"))
	}
}

func TestCheckIsListedInHelpAndCompletion(t *testing.T) {
	help := strings.Join(helpText(), "\n")
	wants(t, help, "%check <name>")
	found := false
	for _, name := range metaCommands() {
		if name == "%check" {
			found = true
		}
	}
	if !found {
		t.Error("the check command is not dispatched")
	}
}

// A %check leaves an action debugging session running: it declares nothing.
func TestCheckKeepsADebuggingSession(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, checkModel+`
package Debug {
	action def Walk {
		first start;
		then action step1;
		then done;
	}
}`)
	wants(t, run(t, s, "%action Debug::Walk"), "Walk")
	run(t, s, "%check Satisfiable")
	rejects(t, run(t, s, "%tokens"), "no active")
}
