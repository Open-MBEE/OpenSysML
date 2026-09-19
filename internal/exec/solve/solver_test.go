package solve

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// scenarioEnv names the environment variable that turns this test binary into a
// fake solver, which is how the driver is tested without a real solver.
const scenarioEnv = "OPENSYSML_TEST_SOLVER_SCENARIO"

// coreEnv names the environment variable holding the reply a fake solver gives to
// `get-unsat-core`, which is how a refused or malformed core is tested.
const coreEnv = "OPENSYSML_TEST_SOLVER_CORE"

// objectivesEnv names the environment variable holding the reply a fake solver
// gives to `get-objectives`, which is how the forms an optimum is reported in are
// tested without a solver.
const objectivesEnv = "OPENSYSML_TEST_SOLVER_OBJECTIVES"

// TestHelperSolverProcess is the fake solver: with scenarioEnv set it plays that
// scenario and exits before the framework prints, so stdout is only SMT-LIB.
func TestHelperSolverProcess(t *testing.T) {
	scenario := os.Getenv(scenarioEnv)
	if scenario == "" {
		t.Skip("not the fake solver child process")
	}
	os.Exit(playScenario(scenario, os.Stdin, os.Stdout, os.Stderr))
}

// playScenario answers the commands a driver sends, as the named scenario does.
func playScenario(scenario string, in *os.File, out, errOut *os.File) int {
	verdicts := map[string]string{
		"sat":     "sat",
		"unsat":   "unsat",
		"unknown": "unknown",
		"garbage": "maybe",
		"error":   `(error "line 3: unknown constant q")`,
	}
	if scenario == "hang" {
		time.Sleep(time.Minute)
		return 0
	}
	if scenario == capabilityScenario {
		return playCapabilities(in, out)
	}
	var labels []string
	checks := 0
	for cmd := range readCommands(in) {
		if label, ok := namedLabel(cmd); ok {
			labels = append(labels, label)
		}
		switch {
		case strings.HasPrefix(cmd, "(check-sat"):
			checks++
			switch scenario {
			case "sat-then-hang":
				// One answer, then silence: an enumeration meets the deadline
				// having already reported a configuration.
				if checks > 1 {
					time.Sleep(time.Minute)
					return 0
				}
				fmt.Fprintln(out, "sat")
				continue
			case "optimal", "optimal-unknown":
				// The query is satisfiable; the checks after it ask whether a
				// better value is feasible, and none is.
				if checks == 1 {
					fmt.Fprintln(out, "sat")
					continue
				}
				if scenario == "optimal-unknown" {
					fmt.Fprintln(out, "unknown")
					continue
				}
				fmt.Fprintln(out, "unsat")
				continue
			case "silent":
				return 0
			case "crash":
				fmt.Fprintln(errOut, "fake solver: segmentation fault")
				return 3
			case "core-needs-first", "core-then-crash":
				// A shrinking round asserts a subset, which is what labels that
				// are not the whole query's show.
				if scenario == "core-then-crash" && !wholeQuery(labels) {
					fmt.Fprintln(errOut, "fake solver: segmentation fault")
					return 3
				}
				// Only the first assertion conflicts, so the solver's own core is
				// larger than it needs to be and reduction has work to do.
				if !slices.Contains(labels, CoreLabel(0)) {
					fmt.Fprintln(out, "sat")
					continue
				}
				fmt.Fprintln(out, "unsat")
				continue
			}
			fmt.Fprintln(out, verdicts[strings.TrimSuffix(scenario, "-exit-1")])
		case strings.HasPrefix(cmd, "(get-unsat-core"):
			if scenario == "core-needs-first" || scenario == "core-then-crash" {
				fmt.Fprintln(out, "("+strings.Join(labels, " ")+")")
				continue
			}
			fmt.Fprintln(out, os.Getenv(coreEnv))
		case strings.HasPrefix(cmd, "(get-objectives"):
			fmt.Fprintln(out, os.Getenv(objectivesEnv))
		case strings.HasPrefix(cmd, "(get-value"):
			fmt.Fprintln(out, os.Getenv("OPENSYSML_TEST_SOLVER_MODEL"))
		case strings.HasPrefix(cmd, "(get-info"):
			fmt.Fprintln(out, "(:reason-unknown \"incomplete arithmetic\")")
		case strings.HasPrefix(cmd, "(exit"):
			if strings.HasSuffix(scenario, "-exit-1") {
				return 1
			}
			return 0
		}
	}
	return 0
}

// namedLabel is the label an assertion was named with, for a command naming one.
func namedLabel(cmd string) (string, bool) {
	_, rest, ok := strings.Cut(cmd, ":named ")
	if !ok {
		return "", false
	}
	return strings.TrimRight(rest, ")"), true
}

// wholeQuery reports whether the labels are a query's own, in order, rather than
// the subset a shrinking round asserts.
func wholeQuery(labels []string) bool {
	for i, label := range labels {
		if label != CoreLabel(i) {
			return false
		}
	}
	return true
}

// readCommands yields the driver's input one top-level command at a time, so the
// fake answers incrementally instead of waiting for input sent only once answered.
func readCommands(in *os.File) func(func(string) bool) {
	return func(yield func(string) bool) {
		var current strings.Builder
		depth := 0
		buf := make([]byte, 1)
		for {
			n, err := in.Read(buf)
			if n == 1 {
				c := buf[0]
				switch c {
				case '(':
					depth++
				case ')':
					depth--
				}
				current.WriteByte(c)
				if depth == 0 && c == ')' {
					if !yield(strings.TrimSpace(current.String())) {
						return
					}
					current.Reset()
				}
			}
			if err != nil {
				return
			}
		}
	}
}

// fakeSolver returns a solver that is this test binary playing a scenario.
func fakeSolver(t *testing.T, scenario string) *Solver {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("test executable: %v", err)
	}
	return &Solver{
		Name:    "fake",
		Path:    self,
		Args:    []string{"-test.run=TestHelperSolverProcess"},
		Timeout: 20 * time.Second,
		Env:     []string{scenarioEnv + "=" + scenario},
		// A scenario answers the driver's dialogue rather than a probe, so the
		// capabilities are declared: what a probe reports is tested separately.
		Declared: DeclaredCapabilities("fake", AllCapabilities...),
	}
}

// intQuery is a query over one integer feature, the smallest query with a model.
func intQuery(t *testing.T) *Query {
	t.Helper()
	return constraintQuery(t, `
		package test {
			private import ScalarValues::Integer;
			constraint def C {
				in i : Integer;
				assert constraint { i > 3 }
			}
		}`, "test::C")
}

// TestSolverVerdicts: each of the three verdicts reaches the caller as itself.
func TestSolverVerdicts(t *testing.T) {
	q := intQuery(t)
	cases := []struct {
		scenario string
		want     Status
		reason   string
	}{
		{"sat", StatusSat, ""},
		{"unsat", StatusUnsat, ""},
		{"unknown", StatusUnknown, "incomplete arithmetic"},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			solver := fakeSolver(t, tc.scenario)
			solver.Env = append(solver.Env, "OPENSYSML_TEST_SOLVER_MODEL=((|test::C::i| 4))")
			result, err := solver.Solve(context.Background(), q)
			if err != nil {
				t.Fatalf("solve: %v", err)
			}
			if result.Status != tc.want {
				t.Fatalf("status %s, want %s", result.Status, tc.want)
			}
			if result.TimedOut {
				t.Fatalf("status %s reported as a timeout", result.Status)
			}
			if result.Reason != tc.reason {
				t.Fatalf("reason %q, want %q", result.Reason, tc.reason)
			}
			if tc.want == StatusSat {
				if len(result.Model) != 1 || result.Model[0].Var.Name != "test::C::i" || result.Model[0].Value != "4" {
					t.Fatalf("model %+v, want i = 4", result.Model)
				}
			} else if len(result.Model) != 0 {
				t.Fatalf("%s carries a model: %+v", result.Status, result.Model)
			}
		})
	}
}

// TestSolverProcessFailures: a solver that crashes, refuses the script, answers
// nonsense or exits non-zero is an error rather than `unknown`.
func TestSolverProcessFailures(t *testing.T) {
	q := intQuery(t)
	cases := []struct {
		scenario string
		want     string // substring of the error's message
	}{
		{"crash", "stopped without answering"},
		{"silent", "stopped without answering"},
		{"garbage", "rather than sat, unsat or unknown"},
		{"error", "rejected the script"},
		{"sat-exit-1", "exited with"},
	}
	for _, tc := range cases {
		t.Run(tc.scenario, func(t *testing.T) {
			solver := fakeSolver(t, tc.scenario)
			solver.Env = append(solver.Env, "OPENSYSML_TEST_SOLVER_MODEL=((|test::C::i| 4))")
			result, err := solver.Solve(context.Background(), q)
			if err == nil {
				t.Fatalf("solve answered %s, want a process failure", result.Status)
			}
			if !errors.Is(err, ErrSolverProcess) {
				t.Fatalf("error %v is not an ErrSolverProcess", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

// TestSolverBadModel: a model naming something else, or leaving a variable out,
// is a process failure rather than a partial answer.
func TestSolverBadModel(t *testing.T) {
	q := intQuery(t)
	cases := []struct {
		name  string
		model string
		want  string
	}{
		{"unknown variable", "((|test::C::k| 4))", "which the query does not declare"},
		{"missing variable", "()", "left out the variable test::C::i"},
		{"not a pair", "((|test::C::i|))", "rather than a variable and a value"},
		{"rejected", `(error "no model is available")`, "would not report a model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			solver := fakeSolver(t, "sat")
			solver.Env = append(solver.Env, "OPENSYSML_TEST_SOLVER_MODEL="+tc.model)
			if _, err := solver.Solve(context.Background(), q); err == nil {
				t.Fatal("solve accepted the model, want a process failure")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.want)
			}
		})
	}
}

// TestSolverTimeout: a solver that never answers becomes `unknown`, marked as a
// timeout, rather than an error or a fabricated verdict.
func TestSolverTimeout(t *testing.T) {
	solver := fakeSolver(t, "hang")
	solver.Timeout = 200 * time.Millisecond
	result, err := solver.Solve(context.Background(), intQuery(t))
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if result.Status != StatusUnknown || !result.TimedOut {
		t.Fatalf("status %s (timed out %v), want unknown after a timeout", result.Status, result.TimedOut)
	}
	if !strings.Contains(result.Reason, "ran out of time") {
		t.Fatalf("reason %q does not say the solver ran out of time", result.Reason)
	}
}

// TestSolverCancellation: the caller's own cancellation is its error, not a
// verdict of the solver's.
func TestSolverCancellation(t *testing.T) {
	solver := fakeSolver(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	defer cancel()
	if _, err := solver.Solve(ctx, intQuery(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("error %v, want the caller's cancellation", err)
	}
}

// TestDiscovery: the override is honoured, then PATH, and an absent solver is a
// typed error naming what to install.
func TestDiscovery(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv(SolverEnv, "")

	if _, err := Discover(); !errors.Is(err, ErrNoSolver) {
		t.Fatalf("error %v, want ErrNoSolver", err)
	} else {
		for _, name := range candidates {
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("error %q does not name %s", err.Error(), name)
			}
		}
	}

	cvc5 := writeExecutable(t, dir, "cvc5")
	solver, err := Discover()
	if err != nil {
		t.Fatalf("discover cvc5: %v", err)
	}
	if solver.Path != cvc5 || solver.Name != "cvc5" {
		t.Fatalf("discovered %s at %s, want cvc5 at %s", solver.Name, solver.Path, cvc5)
	}
	if len(solver.Args) == 0 {
		t.Fatal("cvc5 was given no arguments, so it would not read SMT-LIB2 from standard input")
	}

	z3 := writeExecutable(t, dir, "z3")
	if solver, err = Discover(); err != nil {
		t.Fatalf("discover z3: %v", err)
	} else if solver.Path != z3 {
		t.Fatalf("discovered %s, want z3 preferred over cvc5", solver.Path)
	}

	other := writeExecutable(t, t.TempDir(), "my-solver")
	t.Setenv(SolverEnv, other)
	if solver, err = Discover(); err != nil {
		t.Fatalf("discover the override: %v", err)
	} else if solver.Path != other {
		t.Fatalf("discovered %s, want the override %s", solver.Path, other)
	}

	t.Setenv(SolverEnv, filepath.Join(dir, "absent"))
	var missing *NoSolverError
	if _, err = Discover(); !errors.As(err, &missing) {
		t.Fatalf("error %v, want a NoSolverError", err)
	} else if missing.Override == "" {
		t.Fatalf("error %q does not report that the override is what is missing", err.Error())
	}
}

// TestTimeoutFromEnv: the timeout override is read, and a nonsensical one falls
// back to the default rather than leaving a solver unbounded.
func TestTimeoutFromEnv(t *testing.T) {
	cases := map[string]time.Duration{
		"":       DefaultTimeout,
		"250ms":  250 * time.Millisecond,
		"1m":     time.Minute,
		"-1s":    DefaultTimeout,
		"0s":     DefaultTimeout,
		"lots":   DefaultTimeout,
		" 500ms": 500 * time.Millisecond,
	}
	for text, want := range cases {
		t.Setenv(TimeoutEnv, text)
		if got := timeoutFromEnv(); got != want {
			t.Fatalf("%s=%q gave %s, want %s", TimeoutEnv, text, got, want)
		}
	}
}

// executableSuffix is what a PATH lookup expects an executable to be named with.
var executableSuffix = map[string]string{"windows": ".exe"}[runtime.GOOS]

// writeExecutable puts an executable file in dir, for a PATH lookup to find.
func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	if executableSuffix != "" {
		name += executableSuffix
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
