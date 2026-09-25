package solve

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SolverEnv names the environment variable that overrides solver discovery with
// the path of an executable speaking SMT-LIB2 on standard input.
const SolverEnv = "OPENSYSML_SMT"

// TimeoutEnv names the environment variable that overrides how long a solver is
// given, as a Go duration ("5s", "500ms").
const TimeoutEnv = "OPENSYSML_SMT_TIMEOUT"

// DefaultTimeout is how long a solver is given to answer one query before the
// verdict becomes `unknown`, in the spirit of the runtime's step budgets.
const DefaultTimeout = 10 * time.Second

// candidates are the solvers looked for on PATH, in order of preference.
var candidates = []string{"z3", "cvc5"}

// Status is a solver's verdict about a query. The three verdicts stay distinct:
// StatusUnknown is never reported as either of the others.
type Status int

const (
	// StatusSat means the solver found an assignment satisfying the assertions.
	StatusSat Status = iota
	// StatusUnsat means no assignment satisfies them.
	StatusUnsat
	// StatusUnknown means the solver did not decide: it timed out, or gave up on
	// arithmetic it cannot decide.
	StatusUnknown
)

// String names the verdict as SMT-LIB writes it.
func (s Status) String() string {
	switch s {
	case StatusSat:
		return "sat"
	case StatusUnsat:
		return "unsat"
	default:
		return "unknown"
	}
}

// Solver is an external SMT solver run as a process: no library is linked in, so
// releases stay pure Go and cross-compile.
type Solver struct {
	// Name is the solver as a message names it, its executable's base name.
	Name string

	// Path is the executable run.
	Path string

	// Args are the arguments that put it in SMT-LIB2 mode on standard input.
	Args []string

	// Timeout is how long it is given to answer one query; zero means
	// DefaultTimeout.
	Timeout time.Duration

	// Env are environment variables added to the process's own environment.
	Env []string

	// CoreBudget is how long shrinking an unsat core is given in total; zero
	// means the OPENSYSML_SMT_CORE_BUDGET override or DefaultCoreBudget.
	CoreBudget time.Duration

	// MaxCoreMembers is the largest core shrinking is attempted over; zero means
	// DefaultMaxCoreMembers.
	MaxCoreMembers int

	// Declared states what this backend supports instead of probing it, which is
	// how a caller that already knows a backend skips the probe; nil probes.
	Declared *Capabilities
}

// Assignment is one variable's value in a satisfying model, rendered in the
// notation's own terms rather than SMT-LIB's.
type Assignment struct {
	// Var is the variable the solver assigned, naming the feature it stands for.
	Var *Var

	// Value renders the value as the notation writes it: a quantity with its
	// unit, an enumeration value by name, a number, a boolean or a string.
	Value string

	// Raw is the solver's own S-expression for the value.
	Raw string

	// Rendered is false when the solver answered with a term the notation has no
	// literal for, such as an algebraic number, and Value repeats Raw.
	Rendered bool
}

// Result is what a solver answered about one query.
type Result struct {
	// Query is the query asked.
	Query *Query

	// Status is the verdict.
	Status Status

	// Solver names the solver that answered.
	Solver string

	// TimedOut reports that the solver ran out of time rather than giving up:
	// Status is StatusUnknown, or StatusSat and Undecided for an enumeration
	// reporting what it had found when the deadline fired.
	TimedOut bool

	// Reason is what the solver said when asked why it answered `unknown`,
	// empty when it said nothing or was not asked.
	Reason string

	// Model is the satisfying assignment, one entry per declared variable, for
	// StatusSat and empty otherwise. A solver may answer with any model of many:
	// it is one witness, not a canonical answer.
	Model []Assignment

	// Solutions are the distinct assignments an enumeration reported, in the
	// order the solver produced them, over the variables enumerated rather than
	// every declared one; nil for a plain solve.
	Solutions [][]Assignment

	// Truncated reports that an enumeration stopped before it had shown there is
	// no further solution, for the reason AtBound or Undecided names.
	Truncated bool

	// AtBound reports that the enumeration stopped because it had reported as
	// many solutions as it was asked for; raising the bound may report more.
	AtBound bool

	// Undecided reports that the enumeration stopped because the solver stopped
	// deciding or ran out of time (TimedOut), whether or not it said why: the
	// solutions found stand, and whether others exist is unknown.
	Undecided bool

	// Optima are what came of asking for each objective's optimum, in the order
	// they are optimized; nil for a query that states none.
	Optima []Optimum

	// Core holds the conflicting assertions for a query Explain found unsat, and
	// is nil for every other verdict; a plain Solve leaves it nil unless its
	// caller attaches the core an Explain of the same query found.
	Core *Core

	// Elapsed is how long the solver took, for an Explain every round of
	// shrinking the core included.
	Elapsed time.Duration
}

// Discover finds a solver: the OPENSYSML_SMT override first, then z3 and cvc5 on
// PATH. An absent solver is a typed error, never a fabricated verdict.
func Discover() (*Solver, error) {
	if override := strings.TrimSpace(os.Getenv(SolverEnv)); override != "" {
		path, err := exec.LookPath(override)
		if err != nil {
			return nil, &NoSolverError{Override: override}
		}
		return newSolver(path), nil
	}
	for _, name := range candidates {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		return newSolver(path), nil
	}
	return nil, &NoSolverError{Looked: candidates}
}

// newSolver builds a solver for an executable, with the arguments its family
// needs to read a script from standard input.
func newSolver(path string) *Solver {
	name := baseName(path)
	s := &Solver{Name: name, Path: path, Timeout: timeoutFromEnv()}
	switch {
	case strings.Contains(name, "z3"):
		s.Args = []string{"-smt2", "-in"}
	case strings.Contains(name, "cvc5"):
		s.Args = []string{"--lang=smt2", "--incremental"}
	}
	return s
}

// baseName is the executable's file name, without a directory or an extension.
func baseName(path string) string {
	name := path
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimSuffix(strings.TrimSuffix(name, ".exe"), ".EXE")
}

// timeoutFromEnv reads the timeout override, falling back to DefaultTimeout for
// an unset, unparsable or non-positive value.
func timeoutFromEnv() time.Duration {
	text := strings.TrimSpace(os.Getenv(TimeoutEnv))
	if text == "" {
		return DefaultTimeout
	}
	d, err := time.ParseDuration(text)
	if err != nil || d <= 0 {
		return DefaultTimeout
	}
	return d
}

// Solve discovers a solver and asks it about the query.
func Solve(ctx context.Context, q *Query) (*Result, error) {
	solver, err := Discover()
	if err != nil {
		return nil, err
	}
	return solver.Solve(ctx, q)
}

// Solve writes the query's script to the solver and reads its verdict, plus the
// model on `sat`. Failing to obtain a verdict is an error, not `unknown`, and a
// backend lacking a feature the query needs refuses before it is run.
func (s *Solver) Solve(ctx context.Context, q *Query) (*Result, error) {
	if err := s.require(ctx, q, "solving", modelCapabilities(q)...); err != nil {
		return nil, err
	}
	result, err := s.solve(ctx, q, func(sess *session) (*Result, error) { return sess.run(q) })
	if err != nil {
		return nil, err
	}
	// A witness only stands where the evaluator, the normative arithmetic,
	// confirms it; one it rejects — rounding differently, holding no such
	// value — leaves the question undecided rather than answered.
	if result.Status == StatusSat {
		if ok, reason := replayWitness(q, result.Model); !ok {
			result.Status = StatusUnknown
			result.Reason = reason
		}
	}
	return result, nil
}

// modelCapabilities are what reading an assignment back needs, which is nothing
// for a query declaring no variable to report.
func modelCapabilities(q *Query) []Capability {
	if q == nil || len(q.Vars) == 0 {
		return nil
	}
	return []Capability{CapModels}
}

// solve runs one dialogue in a fresh solver process, turning the run's own
// deadline into an `unknown` verdict. Every question asked of a solver, a
// verdict or a core, goes through it.
func (s *Solver) solve(ctx context.Context, q *Query, dialogue func(*session) (*Result, error)) (*Result, error) {
	if q == nil {
		return nil, fmt.Errorf("solve: no query to solve")
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	session, err := s.start(runCtx)
	if err != nil {
		return nil, err
	}
	defer session.close()

	started := time.Now()
	result, err := dialogue(session)
	if err != nil {
		if timedOut, terr := session.timedOut(ctx, runCtx); timedOut {
			if terr != nil {
				return nil, terr
			}
			reason := "the solver ran out of time after " + timeout.String()
			// A dialogue reporting many answers keeps those it reported before
			// the deadline; whether any others exist is undecided.
			if found := foundBeforeDeadline(err); found != nil {
				found.Query, found.Solver, found.Elapsed = q, s.Name, time.Since(started)
				found.TimedOut, found.Reason = true, reason
				return found, nil
			}
			return &Result{
				Query: q, Status: StatusUnknown, Solver: s.Name,
				TimedOut: true, Reason: reason,
				Elapsed: time.Since(started),
			}, nil
		}
		return nil, err
	}
	result.Query = q
	result.Solver = s.Name
	result.Elapsed = time.Since(started)
	if err := session.finish(); err != nil {
		return nil, err
	}
	return result, nil
}

// session is one solver process and the pipes talking to it.
type session struct {
	solver *Solver
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	out    *bufio.Reader
	stderr *syncBuffer
	waited bool
}

// start launches the solver with its pipes attached.
func (s *Solver) start(ctx context.Context) (*session, error) {
	cmd := exec.CommandContext(ctx, s.Path, s.Args...) // #nosec G204 -- the path is the operator's own solver choice
	if len(s.Env) > 0 {
		cmd.Env = append(os.Environ(), s.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, s.processError("start", "its standard input could not be opened", "", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, s.processError("start", "its standard output could not be opened", "", err)
	}
	stderr := &syncBuffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, s.processError("start", "it could not be run", "", err)
	}
	// SMT-LIB acknowledges every command by default, so every read looks past those
	// acknowledgements: an option silencing them would itself be answered, and a
	// backend declining it would have that answer read as its verdict.
	return &session{solver: s, cmd: cmd, stdin: stdin, out: bufio.NewReader(stdout), stderr: stderr}, nil
}

// run holds the dialogue: the script, `check-sat`, and then the model or the
// reason the solver gives for not deciding.
func (s *session) run(q *Query) (*Result, error) {
	// A solver only answers get-value when models are on, and the option must
	// precede the script's own set-logic.
	if err := s.send("(set-option :produce-models true)\n" + Script(q)); err != nil {
		return nil, err
	}
	status, err := s.verdict()
	if err != nil {
		return nil, err
	}
	result := &Result{Status: status}
	switch status {
	case StatusSat:
		model, err := s.values(q.Vars)
		if err != nil {
			return nil, err
		}
		result.Model = model
	case StatusUnknown:
		result.Reason = s.reasonUnknown()
	}
	return result, nil
}

// send writes to the solver, reporting a closed pipe as a process failure since
// it means the solver stopped reading.
func (s *session) send(text string) error {
	if _, err := io.WriteString(s.stdin, text); err != nil {
		return s.solver.processError("write", "it stopped reading the script", s.stderrText(), err)
	}
	return nil
}

// The SMT-LIB commands whose replies this dialogue reads.
const (
	cmdCheckSat = "check-sat"
	cmdGetValue = "get-value"
)

// verdict reads the answer to `check-sat`.
func (s *session) verdict() (Status, error) {
	reply, err := s.read(cmdCheckSat)
	if err != nil {
		return StatusUnknown, err
	}
	if msg, ok := reply.isError(); ok {
		return StatusUnknown, s.solver.processError(cmdCheckSat, "it rejected the script: "+msg, s.stderrText(), nil)
	}
	switch reply.Atom {
	case "sat":
		return StatusSat, nil
	case "unsat":
		return StatusUnsat, nil
	case "unknown":
		return StatusUnknown, nil
	}
	return StatusUnknown, s.solver.processError(cmdCheckSat,
		"it answered "+quoteReply(reply)+" rather than sat, unsat or unknown", s.stderrText(), nil)
}

// values asks for the value of each variable given and renders each one in the
// notation's own terms.
func (s *session) values(vars []*Var) ([]Assignment, error) {
	if len(vars) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(vars))
	for _, v := range vars {
		names = append(names, smtSymbol(v.Name))
	}
	if err := s.send("(get-value (" + strings.Join(names, " ") + "))\n"); err != nil {
		return nil, err
	}
	reply, err := s.read(cmdGetValue)
	if err != nil {
		return nil, err
	}
	if msg, ok := reply.isError(); ok {
		return nil, s.solver.processError(cmdGetValue, "it would not report a model: "+msg, s.stderrText(), nil)
	}
	values, err := s.pairs(reply, vars)
	if err != nil {
		return nil, err
	}
	out := make([]Assignment, 0, len(vars))
	for _, v := range vars {
		value, ok := values[v.Name]
		if !ok {
			return nil, s.solver.processError(cmdGetValue,
				"its model left out the variable "+v.Name, s.stderrText(), nil)
		}
		out = append(out, assign(v, value))
	}
	return out, nil
}

// pairs reads the `((var value) …)` reply into a value per variable name.
func (s *session) pairs(reply sexpr, vars []*Var) (map[string]sexpr, error) {
	if !reply.IsList {
		return nil, s.solver.processError(cmdGetValue,
			"its model is not a list of assignments: "+quoteReply(reply), s.stderrText(), nil)
	}
	declared := make(map[string]bool, len(vars))
	for _, v := range vars {
		declared[v.Name] = true
	}
	out := make(map[string]sexpr, len(reply.List))
	for _, pair := range reply.List {
		if !pair.IsList || len(pair.List) != 2 || pair.List[0].IsList {
			return nil, s.solver.processError(cmdGetValue,
				"its model holds "+quoteReply(pair)+" rather than a variable and a value", s.stderrText(), nil)
		}
		name := smtName(pair.List[0].Atom)
		if !declared[name] {
			return nil, s.solver.processError(cmdGetValue,
				"its model names "+name+", which the query does not declare", s.stderrText(), nil)
		}
		out[name] = pair.List[1]
	}
	return out, nil
}

// reasonUnknown asks the solver why it did not decide, tolerating a solver that
// declines: the verdict stands either way.
func (s *session) reasonUnknown() string {
	if err := s.send("(get-info :reason-unknown)\n"); err != nil {
		return ""
	}
	reply, err := s.read("get-info")
	if err != nil {
		return ""
	}
	if _, isErr := reply.isError(); isErr {
		return ""
	}
	if reply.IsList && len(reply.List) == 2 {
		return strings.TrimSpace(reply.List[1].Atom)
	}
	return ""
}

// read reads one reply, reporting a solver that stopped as a process failure.
func (s *session) read(stage string) (sexpr, error) {
	reply, err := readSexpr(s.out)
	// A backend that acknowledges commands anyway is not answering yet.
	for err == nil && reply.Atom == "success" {
		reply, err = readSexpr(s.out)
	}
	if err == nil {
		return reply, nil
	}
	if errors.Is(err, errEOF) {
		return sexpr{}, s.solver.processError(stage, "it stopped without answering", s.stderrText(), nil)
	}
	return sexpr{}, s.solver.processError(stage, "its reply could not be read", s.stderrText(), err)
}

// finish ends the dialogue and waits for the solver, whose failure exit is a
// process failure even after a verdict was read.
func (s *session) finish() error {
	if err := s.send("(exit)\n"); err != nil {
		return err
	}
	if err := s.stdin.Close(); err != nil {
		return s.solver.processError("exit", "its standard input could not be closed", s.stderrText(), err)
	}
	s.waited = true
	if err := s.cmd.Wait(); err != nil {
		return s.solver.processError("exit", "it exited with "+err.Error(), s.stderrText(), nil)
	}
	return nil
}

// timedOut reports whether the run's own deadline elapsed rather than the caller
// cancelling, returning the caller's error in that case.
func (s *session) timedOut(caller, run context.Context) (bool, error) {
	if err := caller.Err(); err != nil {
		return true, err
	}
	return errors.Is(run.Err(), context.DeadlineExceeded), nil
}

// close releases the process, killing a solver still running.
func (s *session) close() {
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.waited {
		return
	}
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.waited = true
	_ = s.cmd.Wait()
}

// syncBuffer collects the solver's standard error, which exec's own copying
// goroutine writes while a reply is read here.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write appends to the buffer under its lock.
func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String is what the solver has written so far.
func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// stderrText is what the solver wrote on standard error, on one line.
func (s *session) stderrText() string {
	return comment(s.stderr.String())
}

// processError builds a failure of the solver process itself, which is never a
// verdict.
func (s *Solver) processError(stage, detail, stderr string, err error) error {
	return &SolverProcessError{Solver: s.Name, Stage: stage, Detail: detail, Stderr: stderr, Err: err}
}

// quoteReply renders a reply for a message about it.
func quoteReply(reply sexpr) string { return "`" + comment(reply.String()) + "`" }
