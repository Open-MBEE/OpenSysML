package solve

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// capabilityScenario is the fake solver scenario that answers capability checks,
// and refuseEnv lists the commands it refuses, which is how an incapable backend
// is tested without hunting for one that lacks a feature.
const (
	capabilityScenario = "capabilities"
	refuseEnv          = "OPENSYSML_TEST_SOLVER_REFUSE"
	unsupportedEnv     = "OPENSYSML_TEST_SOLVER_UNSUPPORTED"
	ackEnv             = "OPENSYSML_TEST_SOLVER_ACK"
)

// playCapabilities answers a capability check as a backend supporting everything
// does, except for the commands refuseEnv names, which it rejects.
func playCapabilities(in, out *os.File) int {
	names := func(env string) func(string) bool {
		listed := strings.Split(os.Getenv(env), ",")
		return func(cmd string) bool {
			for _, name := range listed {
				if name != "" && strings.HasPrefix(cmd, "("+name) {
					return true
				}
			}
			return false
		}
	}
	// A backend refusing with an error reply, and one refusing with SMT-LIB's own
	// `unsupported`: both are refusals rather than failures.
	rejects, declines := names(refuseEnv), names(unsupportedEnv)
	// A backend printing SMT-LIB's per-command acknowledgement even after being
	// told not to: every reply read has to look past it.
	acknowledges := os.Getenv(ackEnv) != ""
	cores, checks := false, 0
	for cmd := range readCommands(in) {
		if acknowledges && !strings.HasPrefix(cmd, "(get-") && !strings.HasPrefix(cmd, "(check-sat") {
			writeLine(out, "success")
		}
		if rejects(cmd) {
			writeLine(out, `(error "unsupported")`)
			continue
		}
		if declines(cmd) {
			writeLine(out, "unsupported")
			continue
		}
		switch {
		case strings.Contains(cmd, ":produce-unsat-cores"):
			cores = true
		case strings.HasPrefix(cmd, "(check-sat"):
			checks++
			// A check asking for a core asserts a conflict; a second check in one
			// script asserts one too, which is what an incremental check tests.
			if cores || checks > 1 {
				writeLine(out, "unsat")
				continue
			}
			writeLine(out, "sat")
		case strings.HasPrefix(cmd, "(get-unsat-core"):
			writeLine(out, "("+CoreLabel(0)+" "+CoreLabel(1)+")")
		case strings.HasPrefix(cmd, "(get-value"):
			writeLine(out, "((x 2))")
		case strings.HasPrefix(cmd, "(get-objectives"):
			writeLine(out, "((x 4))")
		}
	}
	return 0
}

// writeLine answers one line, ignoring a closed pipe as the driver having stopped
// asking.
func writeLine(out *os.File, line string) {
	_, _ = out.WriteString(line + "\n")
}

// capabilitySolver is this test binary probed as a backend, refusing the commands
// named. Each gets its own environment, so each is a distinct cache entry.
func capabilitySolver(t *testing.T, refuse ...string) *Solver {
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
		Env: []string{
			scenarioEnv + "=" + capabilityScenario,
			refuseEnv + "=" + strings.Join(refuse, ","),
		},
	}
}

// A backend answering every check is reported as supporting the whole subset, and
// the answers are remembered rather than probed again per query.
func TestCapabilitiesProbeCapableBackend(t *testing.T) {
	solver := capabilitySolver(t)
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !caps.Probed {
		t.Error("capabilities are reported as declared, want probed")
	}
	for _, capability := range AllCapabilities {
		if !caps.Supports(capability) {
			t.Errorf("%s is unsupported (%s), want supported", capability, caps.Detail(capability))
		}
	}
	if len(caps.Supported()) != len(AllCapabilities) {
		t.Errorf("%d capabilities supported, want %d", len(caps.Supported()), len(AllCapabilities))
	}
	// A second ask answers from the cache: were it probing again it would spend
	// another process per capability.
	again, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe again: %v", err)
	}
	if again.Elapsed > caps.Elapsed {
		t.Errorf("asking again took %s, longer than the %s the probe took: it was not cached", again.Elapsed, caps.Elapsed)
	}
}

// A backend acknowledging every command, as SMT-LIB's :print-success default
// asks, is read as capable rather than as refusing: the acknowledgements are not
// mistaken for its answers.
func TestCapabilitiesProbeAcknowledgingBackend(t *testing.T) {
	solver := capabilitySolver(t)
	solver.Env = append(solver.Env, ackEnv+"=1")
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	for _, capability := range AllCapabilities {
		if !caps.Supports(capability) {
			t.Errorf("%s is unsupported (%s), want supported", capability, caps.Detail(capability))
		}
	}
	// The dialogue itself reads past the acknowledgements too.
	v := &Var{Name: "x", Sort: Int}
	q := &Query{Kind: "constraint", Element: "Budget", Vars: []*Var{v},
		Assertions: []Assertion{{Term: Binary(OpGt, Bool, VarTerm(v), IntTerm(1))}}}
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solving against an acknowledging backend: %v", err)
	}
	if result.Status != StatusSat {
		t.Errorf("it answered %s, want sat", result.Status)
	}
}

// A backend refusing a command is reported as lacking that capability alone, with
// what it answered, and keeps the ones it does support.
func TestCapabilitiesProbeIncapableBackend(t *testing.T) {
	solver := capabilitySolver(t, "maximize", "get-unsat-core")
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	for _, capability := range []Capability{CapOptimization, CapOptimizationPriority, CapUnsatCores} {
		if caps.Supports(capability) {
			t.Errorf("%s is supported, want refused", capability)
		}
		if !strings.Contains(caps.Detail(capability), "unsupported") {
			t.Errorf("%s was refused with %q, want what the backend answered", capability, caps.Detail(capability))
		}
	}
	for _, capability := range []Capability{CapModels, CapIncremental, CapDatatypes, CapStrings, CapIntegerDivision} {
		if !caps.Supports(capability) {
			t.Errorf("%s is unsupported (%s), want supported", capability, caps.Detail(capability))
		}
	}
}

// A query needing a capability the backend lacks is refused before it is run, by a
// typed error naming the feature, the backend and what was being asked — never a
// verdict and never a quieter question.
func TestUnsupportedCapabilityIsRefused(t *testing.T) {
	solver := capabilitySolver(t, "get-unsat-core")
	q := &Query{
		Kind: "constraint", Element: "Budget",
		Vars:       []*Var{{Name: "x", Sort: Int}},
		Assertions: []Assertion{{Term: Binary(OpGt, Bool, VarTerm(&Var{Name: "x", Sort: Int}), IntTerm(1))}},
	}
	_, err := solver.Explain(context.Background(), q)
	if err == nil {
		t.Fatal("explaining answered, want a refusal")
	}
	if !Unsupported(err) || !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("explaining failed with %v, want an unsupported-capability error", err)
	}
	var unsupported *UnsupportedCapabilityError
	if !errors.As(err, &unsupported) {
		t.Fatalf("explaining failed with %T, want *UnsupportedCapabilityError", err)
	}
	if unsupported.Solver != "fake" || unsupported.Operation != "explaining a conflict" {
		t.Errorf("refusal is about %s %s, want fake explaining a conflict", unsupported.Solver, unsupported.Operation)
	}
	if len(unsupported.Missing) != 1 || unsupported.Missing[0] != CapUnsatCores {
		t.Errorf("refusal names %v, want the unsat-cores capability", unsupported.Missing)
	}
	for _, want := range []string{"fake", "get-unsat-core", "explaining a conflict", SolverEnv} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not mention %q", err, want)
		}
	}
	// The same backend answers a query needing nothing it lacks.
	if _, err := solver.Solve(context.Background(), q); err != nil {
		t.Errorf("solving a query needing no core: %v", err)
	}
}

// A check the backend leaves undecided refuses nothing: the query runs and the
// backend's own answer, or its failure, is reported rather than a guess at what
// it supports.
func TestUndeterminedCapabilityDoesNotRefuse(t *testing.T) {
	solver := capabilitySolver(t)
	// The scenario answers nothing at all, so no check settles either way.
	solver.Env = []string{scenarioEnv + "=silent"}
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	for _, capability := range AllCapabilities {
		if caps.Supports(capability) || caps.Refuses(capability) {
			t.Errorf("%s was settled by a backend that answered nothing", capability)
		}
		if !caps.Undetermined(capability) || caps.Detail(capability) == "" {
			t.Errorf("%s is not reported as undetermined with what happened", capability)
		}
	}
	q := &Query{Kind: "constraint", Element: "Silent",
		Vars:       []*Var{{Name: "x", Sort: Int}},
		Assertions: []Assertion{{Term: Binary(OpGt, Bool, VarTerm(&Var{Name: "x", Sort: Int}), IntTerm(1))}}}
	_, err = solver.Solve(context.Background(), q)
	if err == nil {
		t.Fatal("a silent backend answered")
	}
	if Unsupported(err) {
		t.Errorf("a silent backend was reported as lacking a capability: %v", err)
	}
	if !errors.Is(err, ErrSolverProcess) {
		t.Errorf("a silent backend failed with %v, want a process error", err)
	}
}

// A backend answering something SMT-LIB does not define is not answering as a
// solver: that is a process failure, not a claim about what it supports.
func TestUnreadableReplyIsAProcessFailure(t *testing.T) {
	solver := capabilitySolver(t)
	// The scenario answers `maybe` to every check-sat.
	solver.Env = []string{scenarioEnv + "=garbage"}
	if _, err := solver.Capabilities(context.Background()); err == nil {
		t.Fatal("probing a backend answering `maybe` succeeded")
	} else {
		if Unsupported(err) {
			t.Errorf("an unreadable reply is reported as a missing capability: %v", err)
		}
		if !errors.Is(err, ErrSolverProcess) {
			t.Errorf("probe failed with %v, want a process error", err)
		}
		for _, want := range []string{"fake", "maybe", "SMT-LIB response"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("failure %q does not mention %q", err, want)
			}
		}
	}
	q := &Query{Kind: "constraint", Element: "Budget",
		Vars:       []*Var{{Name: "x", Sort: Int}},
		Assertions: []Assertion{{Term: Binary(OpGt, Bool, VarTerm(&Var{Name: "x", Sort: Int}), IntTerm(1))}}}
	_, err := solver.Solve(context.Background(), q)
	if Unsupported(err) || !errors.Is(err, ErrSolverProcess) {
		t.Errorf("solving against it failed with %v, want a process error", err)
	}
}

// The other side of that line: `unsupported`, which SMT-LIB defines as declining
// a command, is a refusal of the capability rather than a failure.
func TestUnsupportedReplyIsARefusal(t *testing.T) {
	solver := capabilitySolver(t)
	solver.Env = append(solver.Env, unsupportedEnv+"=get-unsat-core")
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !caps.Refuses(CapUnsatCores) {
		t.Errorf("unsat-cores is %v after `unsupported`, want refused", caps.Detail(CapUnsatCores))
	}
	if !strings.Contains(caps.Detail(CapUnsatCores), "unsupported") {
		t.Errorf("refusal detail is %q, want what the backend answered", caps.Detail(CapUnsatCores))
	}
	if !caps.Supports(CapModels) {
		t.Errorf("models is unsupported (%s), want supported", caps.Detail(CapModels))
	}
}

// The timeout the operator set bounds a capability check too, rather than each
// check spending the longer default before answering nothing.
func TestProbeTimeoutFollowsSolverTimeout(t *testing.T) {
	solver := capabilitySolver(t)
	solver.Timeout = 2 * time.Second
	if got := solver.probeTimeout(); got != 2*time.Second {
		t.Errorf("a check is given %s, want the solver's own %s", got, solver.Timeout)
	}
	solver.Timeout = time.Hour
	if got := solver.probeTimeout(); got != ProbeTimeout {
		t.Errorf("a check is given %s, want no more than %s", got, ProbeTimeout)
	}
	// A backend that never answers is given the shorter timeout, and says so.
	hanging := capabilitySolver(t)
	hanging.Timeout = time.Second
	hanging.Env = []string{scenarioEnv + "=hang"}
	caps, err := hanging.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !caps.Undetermined(CapModels) {
		t.Fatalf("a hanging backend settled %s: %s", CapModels, caps.Detail(CapModels))
	}
	if !strings.Contains(caps.Detail(CapModels), hanging.Timeout.String()) {
		t.Errorf("the check reports %q, want the %s it waited", caps.Detail(CapModels), hanging.Timeout)
	}
	if caps.Elapsed > 30*time.Second {
		t.Errorf("the checks took %s, longer than the timeout should allow", caps.Elapsed)
	}
}

// A backend that cannot be run at all is a process failure, not a report that it
// lacks a feature: nothing was learned about what it supports.
func TestCapabilitiesMissingBackend(t *testing.T) {
	solver := &Solver{Name: "nowhere", Path: "/nonexistent/no-such-solver", Timeout: time.Second}
	_, err := solver.Capabilities(context.Background())
	if err == nil {
		t.Fatal("probing a solver that is not there answered, want a failure")
	}
	if Unsupported(err) {
		t.Errorf("probing a missing solver reported an unsupported capability: %v", err)
	}
	if !errors.Is(err, ErrSolverProcess) {
		t.Errorf("probing a missing solver failed with %v, want a process error", err)
	}
}

// Declared capabilities answer without a process, so a caller that knows its
// backend pays for no probe.
func TestDeclaredCapabilitiesSkipProbing(t *testing.T) {
	solver := &Solver{Name: "declared", Path: "/nonexistent/no-such-solver", Timeout: time.Second}
	solver.Declared = DeclaredCapabilities("declared", CapModels)
	caps, err := solver.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("declared capabilities: %v", err)
	}
	if caps.Probed {
		t.Error("declared capabilities are reported as probed")
	}
	if !caps.Supports(CapModels) || caps.Supports(CapUnsatCores) {
		t.Errorf("declared capabilities are %v, want models alone", caps.Supported())
	}
	if caps.Detail(CapUnsatCores) == "" {
		t.Error("a capability declared absent says nothing about why")
	}
}

// What a query needs is read from the query: its sorts, its arithmetic and the
// logic it must set, and nothing an operation adds of its own.
func TestQueryRequires(t *testing.T) {
	intVar := &Var{Name: "i", Sort: Int}
	realVar := &Var{Name: "r", Sort: Real}
	cases := []struct {
		name  string
		query *Query
		want  []Capability
	}{
		{"plain integers need no capability beyond SMT-LIB",
			&Query{Vars: []*Var{intVar}, Assertions: []Assertion{{Term: Binary(OpGt, Bool, VarTerm(intVar), IntTerm(1))}}}, nil},
		{"integer division needs the Ints theory's div",
			&Query{Vars: []*Var{intVar}, Assertions: []Assertion{
				{Term: Binary(OpEq, Bool, Binary(OpIntDiv, Int, VarTerm(intVar), IntTerm(2)), IntTerm(3))}}},
			[]Capability{CapIntegerDivision}},
		{"mixed arithmetic needs the mixed logics",
			&Query{Vars: []*Var{intVar, realVar}, Assertions: []Assertion{
				{Term: Binary(OpLe, Bool, ToReal(VarTerm(intVar)), VarTerm(realVar))}}},
			[]Capability{CapMixedArith}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.query.Requires()
			if len(got) != len(c.want) {
				t.Fatalf("the query needs %v, want %v", got, c.want)
			}
			for i, capability := range c.want {
				if got[i] != capability {
					t.Errorf("the query needs %v, want %v", got, c.want)
				}
			}
		})
	}
}
