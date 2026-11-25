package fmi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// fmi/2 is an additive, newline-delimited command protocol on one runner
// process: init, then step and read from the state reached.
const (
	sessionProtocol = 2
	actionInit      = "init"
	actionStep      = "step"
	actionRead      = "read"
)

// sessionCommand is one init, step or read command. Only its per-action fields
// are set beyond protocol and action.
type sessionCommand struct {
	Protocol   int               `json:"protocol"`
	Action     string            `json:"action"`
	FMU        string            `json:"fmu,omitempty"`
	Interface  string            `json:"interface,omitempty"`
	Experiment *runnerExperiment `json:"experiment,omitempty"`
	Start      map[string]any    `json:"start,omitempty"`
	Time       *float64          `json:"time,omitempty"`
	Outputs    []string          `json:"outputs,omitempty"`
}

// sessionReply is one answer line: the time reached, outputs, or an error.
type sessionReply struct {
	Protocol int                        `json:"protocol"`
	Action   string                     `json:"action"`
	Time     *float64                   `json:"time,omitempty"`
	Outputs  map[string]json.RawMessage `json:"outputs,omitempty"`
	Error    *string                    `json:"error,omitempty"`
}

// SessionConfig bounds a manager's sessions. Zero fields take the defaults
// declared below: idle, lifetime, open count and steps.
type SessionConfig struct {
	// Idle is how long an untouched session stays open. DefaultSessionIdle when zero.
	Idle time.Duration
	// Lifetime is how long any session stays open from its init. DefaultSessionLifetime when zero.
	Lifetime time.Duration
	// MaxSessions bounds how many sessions a manager holds. DefaultMaxSessions when zero.
	MaxSessions int
	// MaxSteps bounds the steps one session may take. DefaultMaxSteps when zero.
	MaxSteps int
}

// Defaults for a manager made with NewSessionManager.
const (
	DefaultSessionIdle     = 60 * time.Second
	DefaultSessionLifetime = 10 * time.Minute
	DefaultMaxSessions     = 16
	DefaultMaxSteps        = 100_000
)

// DefaultSessionConfig is the bound a production manager starts with.
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{Idle: DefaultSessionIdle, Lifetime: DefaultSessionLifetime,
		MaxSessions: DefaultMaxSessions, MaxSteps: DefaultMaxSteps}
}

func (c SessionConfig) withDefaults() SessionConfig {
	d := DefaultSessionConfig()
	if c.Idle > 0 {
		d.Idle = c.Idle
	}
	if c.Lifetime > 0 {
		d.Lifetime = c.Lifetime
	}
	if c.MaxSessions > 0 {
		d.MaxSessions = c.MaxSessions
	}
	if c.MaxSteps > 0 {
		d.MaxSteps = c.MaxSteps
	}
	return d
}

// ErrSession is the typed error every session fault unwraps to.
var ErrSession = errors.New("fmi session failed")

// SessionError reports a fault of one session: its caller, the FMU it holds and
// what broke. Every typed session error unwraps to ErrSession.
type SessionError struct {
	Caller, FMU, Detail string
	Err                 error
}

// Error names the caller, the FMU and the fault.
func (e *SessionError) Error() string {
	text := fmt.Sprintf("fmi session (%s, %s): %s", e.Caller, baseName(e.FMU), e.Detail)
	if e.Err != nil {
		text += ": " + e.Err.Error()
	}
	return text
}

// Is matches ErrSession.
func (e *SessionError) Is(target error) bool { return target == ErrSession }

// Unwrap is the underlying fault.
func (e *SessionError) Unwrap() error { return e.Err }

// ErrSessionExpired is the typed error for a session past its idle or absolute
// bound, or one that has taken its full allowance of steps.
var ErrSessionExpired = errors.New("fmi session expired")

// SessionExpiredError reports why a session can no longer be used.
type SessionExpiredError struct {
	Caller, FMU, Reason string
}

// Error names the caller, the FMU and the bound.
func (e *SessionExpiredError) Error() string {
	return fmt.Sprintf("fmi session (%s, %s): %s", e.Caller, baseName(e.FMU), e.Reason)
}

// Is matches ErrSessionExpired and ErrSession.
func (e *SessionExpiredError) Is(target error) bool {
	return target == ErrSessionExpired || target == ErrSession
}

// ErrSessionLimit is the typed error for a caller asking for one session too many.
var ErrSessionLimit = errors.New("fmi session limit reached")

// SessionLimitError reports the bound a manager refuses to exceed.
type SessionLimitError struct {
	Caller string
	Limit  int
}

// Error names the caller and the bound.
func (e *SessionLimitError) Error() string {
	return fmt.Sprintf("fmi session for %q: the manager holds %d sessions already", e.Caller, e.Limit)
}

// Is matches ErrSessionLimit and ErrSession.
func (e *SessionLimitError) Is(target error) bool {
	return target == ErrSessionLimit || target == ErrSession
}

// ErrStep is the typed error for a step a session cannot take: one that goes
// backwards, or one past the experiment's stop time.
var ErrStep = errors.New("fmi session refused a step")

// StepError reports the time refused and the time the session is at.
type StepError struct {
	Caller, FMU string
	From, To    float64
	Reason      string
}

// Error names the times and why.
func (e *StepError) Error() string {
	return fmt.Sprintf("fmi session (%s, %s): cannot step from %v to %v: %s", e.Caller, baseName(e.FMU), e.From, e.To, e.Reason)
}

// Is matches ErrStep and ErrSession.
func (e *StepError) Is(target error) bool { return target == ErrStep || target == ErrSession }

// SessionManager holds sessions keyed by caller and init request, so one
// caller's simulation state is never handed to another.
type SessionManager struct {
	look    func() (string, error)
	timeout func() time.Duration
	limit   func() int
	config  SessionConfig

	mu       sync.Mutex
	sessions map[string]*Session
	opening  map[string]*sessionOpening
	closed   bool

	stop chan struct{}
	done chan struct{}
}

// sessionOpening is one in-flight init. A second caller waits on done rather
// than starting a second runner or holding the manager's lock.
type sessionOpening struct {
	done    chan struct{}
	session *Session
	err     error
}

// NewSessionManager returns a manager with the production bounds, finding the
// runner exactly as the one-shot engine does. A nil look reads RunnerEnv.
func NewSessionManager(look func() (string, error)) *SessionManager {
	return NewSessionManagerWith(look, DefaultSessionConfig())
}

// NewSessionManagerWith returns a manager with explicit bounds. Zero fields in
// config take their defaults.
func NewSessionManagerWith(look func() (string, error), config SessionConfig) *SessionManager {
	if look == nil {
		look = lookRunner
	}
	m := &SessionManager{look: look, timeout: analysis.ToolTimeoutFromEnv,
		limit: analysis.OutputLimitFromEnv, config: config.withDefaults(),
		sessions: make(map[string]*Session), opening: make(map[string]*sessionOpening),
		stop: make(chan struct{}), done: make(chan struct{})}
	go m.janitor()
	return m
}

// newSessionManager is the seam tests use to fix the timeout and output bound.
func newSessionManager(look func() (string, error), timeout func() time.Duration, limit func() int, config SessionConfig) *SessionManager {
	if look == nil {
		look = lookRunner
	}
	m := &SessionManager{look: look, timeout: timeout, limit: limit, config: config.withDefaults(),
		sessions: make(map[string]*Session), opening: make(map[string]*sessionOpening),
		stop: make(chan struct{}), done: make(chan struct{})}
	go m.janitor()
	return m
}

// janitor closes sessions that pass a bound even when nobody calls Open again.
func (m *SessionManager) janitor() {
	defer close(m.done)
	interval := m.config.Idle / 4
	if m.config.Lifetime/4 < interval || interval == 0 {
		interval = m.config.Lifetime / 4
	}
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}
	if interval > 5*time.Second {
		interval = 5 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			m.evict(time.Now())
		case <-m.stop:
			return
		}
	}
}

// evict drops and closes sessions past a bound. It never holds both locks at
// once: Close can take them in the other order.
func (m *SessionManager) evict(now time.Time) {
	m.mu.Lock()
	candidates := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		candidates = append(candidates, s)
	}
	m.mu.Unlock()
	for _, s := range candidates {
		reason := s.expired(now)
		if reason == "" {
			continue
		}
		m.mu.Lock()
		if m.sessions[s.key] == s {
			delete(m.sessions, s.key)
		}
		m.mu.Unlock()
		s.mu.Lock()
		_ = s.expireLocked(reason)
		s.mu.Unlock()
	}
}

// Close ends every session and stops the janitor. It is idempotent.
func (m *SessionManager) Close() error {
	if m == nil || m.stop == nil {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	all := make([]*Session, 0, len(m.sessions))
	for key, s := range m.sessions {
		delete(m.sessions, key)
		all = append(all, s)
	}
	close(m.stop)
	m.mu.Unlock()

	var first error
	for _, s := range all {
		if err := s.closeProcess(); err != nil && first == nil {
			first = err
		}
	}
	<-m.done
	return first
}

// Open returns the live session for caller's request, initializing it once.
// StepVariable is not part of the identity; it selects a later step.
func (m *SessionManager) Open(ctx context.Context, caller string, call *runtime.ToolCall) (s *Session, err error) {
	if m == nil || m.look == nil {
		return nil, &SessionError{Caller: caller, Detail: "the session manager is nil or unconfigured"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if call == nil {
		return nil, &SessionError{Caller: caller, Detail: "no tool call to open a session for"}
	}
	path, err := m.look()
	if err != nil {
		return nil, &analysis.ProcessAbsentError{Engine: analysis.ToolEngineName("fmi"), Process: runnerDescription, Err: err}
	}
	fmu, err := resolveURI(call)
	if err != nil {
		return nil, err
	}
	d, err := Read(fmu)
	if err != nil {
		return nil, err
	}
	if err := checkVariables(call, d); err != nil {
		return nil, err
	}
	fields, _, err := requestFields(call, d, fmu)
	if err != nil {
		return nil, err
	}
	fields.Outputs = nil
	init := sessionCommand{Protocol: sessionProtocol, Action: actionInit, FMU: fields.FMU,
		Interface: fields.Interface, Experiment: &fields.Experiment, Start: fields.Start}
	raw, err := json.Marshal(init)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	key := fmt.Sprintf("%s|%x", caller, sum)

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, &SessionError{Caller: caller, FMU: fmu, Detail: "the session manager is closed"}
	}
	now := time.Now()
	if existing, ok := m.sessions[key]; ok {
		reason := existing.expired(now)
		if reason == "" {
			existing.touch(now)
			m.mu.Unlock()
			return existing, nil
		}
		delete(m.sessions, key)
		existing.mu.Lock()
		_ = existing.expireLocked(reason)
		existing.mu.Unlock()
	}
	if inFlight := m.opening[key]; inFlight != nil {
		m.mu.Unlock()
		select {
		case <-inFlight.done:
			if inFlight.session != nil {
				inFlight.session.touch(time.Now())
			}
			return inFlight.session, inFlight.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if len(m.sessions)+len(m.opening) >= m.config.MaxSessions {
		m.mu.Unlock()
		return nil, &SessionLimitError{Caller: caller, Limit: m.config.MaxSessions}
	}
	inFlight := &sessionOpening{done: make(chan struct{})}
	m.opening[key] = inFlight
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		if s != nil {
			if m.closed {
				_ = s.closeProcess()
				s = nil
				err = &SessionError{Caller: caller, FMU: fmu, Detail: "the session manager is closed"}
			} else {
				m.sessions[key] = s
			}
		}
		inFlight.session, inFlight.err = s, err
		delete(m.opening, key)
		m.mu.Unlock()
		close(inFlight.done)
	}()

	proc, err := startSessionProcess(ctx, path, m.limitOf(), m.timeoutOf())
	if err != nil {
		return nil, m.sessionError(caller, fmu, "the runner did not start", err)
	}
	candidate := &Session{manager: m, key: key, caller: caller, fmu: fmu, d: d, proc: proc,
		startTime: fields.Experiment.StartTime, stopTime: fields.Experiment.StopTime,
		now: fields.Experiment.StartTime, created: now, used: now}
	commandCtx, cancelCommand := m.commandContext(ctx)
	reply, err := proc.command(commandCtx, init, m.limitOf())
	cancelCommand()
	if err != nil {
		_ = proc.close()
		return nil, m.sessionError(caller, fmu, "the runner did not initialize", err)
	}
	reached, err := sessionTime(reply, actionInit)
	if err != nil {
		_ = proc.close()
		return nil, m.sessionError(caller, fmu, "the runner answered init badly", err)
	}
	if !sameTime(reached, candidate.startTime) {
		_ = proc.close()
		return nil, m.sessionError(caller, fmu, fmt.Sprintf("init answered time %v, want %v", reached, candidate.startTime), nil)
	}
	return candidate, nil
}

func (m *SessionManager) sessionError(caller, fmu, detail string, err error) error {
	return &SessionError{Caller: caller, FMU: fmu, Detail: detail, Err: err}
}

// timeoutOf and limitOf let a zero-value manager answer with the production
// defaults instead of panicking.
func (m *SessionManager) timeoutOf() time.Duration {
	if m == nil || m.timeout == nil {
		return analysis.ToolTimeoutFromEnv()
	}
	return m.timeout()
}

func (m *SessionManager) limitOf() int {
	if m == nil || m.limit == nil {
		return analysis.OutputLimitFromEnv()
	}
	return m.limit()
}

// commandContext bounds one command by the manager's timeout even when a
// caller passes a context with no deadline.
func (m *SessionManager) commandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, m.timeoutOf())
}

// Session is one initialized FMU for a caller. Step advances time; Read takes
// values from the state reached.
type Session struct {
	manager *SessionManager
	key     string
	caller  string
	fmu     string
	d       *Description
	proc    *sessionProcess

	startTime, stopTime, now float64
	steps                    int
	created, used            time.Time

	mu            sync.Mutex
	closed        bool
	expiredReason string
}

// expired reports why s is past a bound, "" while usable. Takes s.mu.
func (s *Session) expired(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.manager == nil {
		return "the session is unconfigured"
	}
	if s.closed {
		if s.expiredReason != "" {
			return s.expiredReason
		}
		return "the session was closed"
	}
	if reason := s.timeExpired(now); reason != "" {
		return reason
	}
	if s.manager.config.MaxSteps > 0 && s.steps >= s.manager.config.MaxSteps {
		return fmt.Sprintf("took %d steps", s.steps)
	}
	return ""
}

// timeExpired checks the clock bounds only. The step budget is checked when a
// further step is asked for, so a Read after the last allowed step still works.
func (s *Session) timeExpired(now time.Time) string {
	switch {
	case s.manager.config.Idle > 0 && now.Sub(s.used) >= s.manager.config.Idle:
		return fmt.Sprintf("idle for %s", now.Sub(s.used))
	case s.manager.config.Lifetime > 0 && now.Sub(s.created) >= s.manager.config.Lifetime:
		return fmt.Sprintf("open for %s", now.Sub(s.created))
	}
	return ""
}

// touch records that a caller used the session now.
func (s *Session) touch(now time.Time) {
	s.mu.Lock()
	s.used = now
	s.mu.Unlock()
}

// Caller is the identity the session is isolated for.
func (s *Session) Caller() string { return s.caller }

// FMU is the archive the session initialized.
func (s *Session) FMU() string { return s.fmu }

// Time is the simulation time the session is at now.
func (s *Session) Time() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now
}

// Step advances the FMU to t. A backwards or past-stop step is a *StepError;
// runner and protocol failures are typed errors.
func (s *Session) Step(ctx context.Context, t float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(ctx); err != nil {
		return err
	}
	if math.IsNaN(t) || math.IsInf(t, 0) {
		return &StepError{Caller: s.caller, FMU: s.fmu, From: s.now, To: t, Reason: "the time is not finite"}
	}
	if t < s.now {
		return &StepError{Caller: s.caller, FMU: s.fmu, From: s.now, To: t, Reason: "the experiment cannot go backwards"}
	}
	if t > s.stopTime {
		return &StepError{Caller: s.caller, FMU: s.fmu, From: s.now, To: t,
			Reason: fmt.Sprintf("the experiment stops at %v", s.stopTime)}
	}
	if s.manager.config.MaxSteps > 0 && s.steps >= s.manager.config.MaxSteps {
		return s.expireLocked(fmt.Sprintf("took %d steps", s.steps))
	}
	at := t
	commandCtx, cancelCommand := s.manager.commandContext(ctx)
	reply, err := s.proc.command(commandCtx, sessionCommand{Protocol: sessionProtocol, Action: actionStep, Time: &at}, s.manager.limitOf())
	cancelCommand()
	if err != nil {
		return &SessionError{Caller: s.caller, FMU: s.fmu, Detail: "the runner did not step", Err: err}
	}
	reached, err := sessionTime(reply, actionStep)
	if err != nil {
		return &SessionError{Caller: s.caller, FMU: s.fmu, Detail: "the runner answered step badly", Err: err}
	}
	if !sameTime(reached, t) {
		return &SessionError{Caller: s.caller, FMU: s.fmu,
			Detail: fmt.Sprintf("step answered time %v, want %v", reached, t)}
	}
	s.now, s.steps = reached, s.steps+1
	s.used = time.Now()
	return nil
}

// Read answers the named tool variables from the state the last Step reached.
// Values are typed by the same reader the one-shot reply uses.
func (s *Session) Read(ctx context.Context, names []string) (map[string]runtime.ToolValue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.check(ctx); err != nil {
		return nil, err
	}
	modelNames := make([]string, 0, len(names))
	for _, name := range names {
		if reservedOutputs[name] {
			continue
		}
		v, ok := s.d.Variable(name)
		if !ok {
			return nil, &VariableError{FMU: s.fmu, Variable: name, Detail: "has no variable"}
		}
		if !outputDirection(v) {
			return nil, &VariableError{FMU: s.fmu, Variable: name,
				Detail: fmt.Sprintf("variable %q is causality %s, not an output", name, v.Causality)}
		}
		modelNames = append(modelNames, v.Name)
	}
	commandCtx, cancelCommand := s.manager.commandContext(ctx)
	reply, err := s.proc.command(commandCtx, sessionCommand{Protocol: sessionProtocol, Action: actionRead, Outputs: modelNames}, s.manager.limitOf())
	cancelCommand()
	if err != nil {
		return nil, &SessionError{Caller: s.caller, FMU: s.fmu, Detail: "the runner did not read", Err: err}
	}
	read, err := sessionRead(reply)
	if err != nil {
		return nil, &SessionError{Caller: s.caller, FMU: s.fmu, Detail: "the runner answered read badly", Err: err}
	}
	values := make(map[string]runtime.ToolValue, len(names))
	for _, name := range names {
		if reservedOutputs[name] {
			values[name] = runtime.ToolValue{Value: valueReal(s.now)}
			continue
		}
		v, _ := s.d.Variable(name)
		raw, ok := read[v.Name]
		if !ok {
			return nil, &SessionError{Caller: s.caller, FMU: s.fmu,
				Detail: fmt.Sprintf("the runner answered no output %q", v.Name)}
		}
		value, err := outputValue(s.d, v, raw)
		if err != nil {
			return nil, &SessionError{Caller: s.caller, FMU: s.fmu, Detail: err.Error()}
		}
		values[name] = value
	}
	s.used = time.Now()
	return values, nil
}

// check is the common refusal of a Step or Read on a closed or expired session.
// Called with s.mu held.
func (s *Session) check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		if s.expiredReason != "" {
			return &SessionExpiredError{Caller: s.caller, FMU: s.fmu, Reason: s.expiredReason}
		}
		return &SessionError{Caller: s.caller, FMU: s.fmu, Detail: "the session was closed"}
	}
	if s.manager == nil || s.proc == nil {
		return &SessionError{Caller: s.caller, FMU: s.fmu, Detail: "the session is unconfigured"}
	}
	now := time.Now()
	if reason := s.timeExpired(now); reason != "" {
		return s.expireLocked(reason)
	}
	return nil
}

// expireLocked closes the process and answers the typed expiry. Called with
// s.mu held; the manager's map entry is removed by its janitor.
func (s *Session) expireLocked(reason string) error {
	s.closed = true
	s.expiredReason = reason
	proc := s.proc
	s.proc = nil
	if proc != nil {
		_ = proc.close()
	}
	return &SessionExpiredError{Caller: s.caller, FMU: s.fmu, Reason: reason}
}

// Close ends the session and frees its process. It is idempotent. The
// manager's lock is taken before the session's, the order Open and evict use.
func (s *Session) Close() error {
	if s.manager != nil {
		s.manager.mu.Lock()
		if s.manager.sessions[s.key] == s {
			delete(s.manager.sessions, s.key)
		}
		s.manager.mu.Unlock()
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	proc := s.proc
	s.proc = nil
	s.mu.Unlock()
	if proc != nil {
		return proc.close()
	}
	return nil
}

// closeProcess ends the process without taking the manager's lock, for the
// janitor which already removed the entry.
func (s *Session) closeProcess() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	proc := s.proc
	s.proc = nil
	s.mu.Unlock()
	if proc != nil {
		return proc.close()
	}
	return nil
}

// sessionTime reads the time an init or step answered, refusing a reply that
// names no finite time or carries the runner's error.
func sessionTime(reply []byte, action string) (float64, error) {
	r, err := decodeSessionReply(reply)
	if err != nil {
		return 0, err
	}
	if err := checkAction(r, action); err != nil {
		return 0, err
	}
	if r.Error != nil {
		return 0, &RunnerError{Message: *r.Error}
	}
	if r.Time == nil || math.IsNaN(*r.Time) || math.IsInf(*r.Time, 0) {
		return 0, fmt.Errorf("no finite time in the %s answer", action)
	}
	return *r.Time, nil
}

// sessionRead reads the outputs a read answered.
func sessionRead(reply []byte) (map[string]json.RawMessage, error) {
	r, err := decodeSessionReply(reply)
	if err != nil {
		return nil, err
	}
	if err := checkAction(r, actionRead); err != nil {
		return nil, err
	}
	if r.Error != nil {
		return nil, &RunnerError{Message: *r.Error}
	}
	return r.Outputs, nil
}

// decodeSessionReply checks the protocol and the action of one answer line.
func decodeSessionReply(reply []byte) (sessionReply, error) {
	var r sessionReply
	if err := json.Unmarshal(bytes.TrimSpace(reply), &r); err != nil {
		return r, fmt.Errorf("the runner wrote no JSON answer: %w", err)
	}
	if r.Protocol != sessionProtocol {
		return r, fmt.Errorf("the runner spoke protocol %d; a session speaks %d", r.Protocol, sessionProtocol)
	}
	return r, nil
}

// checkAction refuses an answer that names a different command than the one
// asked; a runner that leaves action out still answers by position.
func checkAction(r sessionReply, action string) error {
	if r.Action != "" && r.Action != action {
		return fmt.Errorf("the runner answered %q to %q", r.Action, action)
	}
	return nil
}

// sameTime compares a reached time with the one asked, allowing only for the
// rounding a JSON number can carry.
func sameTime(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

// outputDirection reports whether a variable may be read from a session.
func outputDirection(v Variable) bool {
	switch v.Causality {
	case CausalityOutput, CausalityCalculatedParameter, CausalityLocal:
		return true
	}
	return false
}

// ToolRunner routes calls binding StepVariable through caller's session.
// Every other call goes to delegate unchanged, keeping fmi/1 byte-identical.
func (m *SessionManager) ToolRunner(caller string, delegate runtime.ToolRunner) runtime.ToolRunner {
	return &sessionToolRunner{manager: m, caller: caller, delegate: delegate}
}

type sessionToolRunner struct {
	manager  *SessionManager
	caller   string
	delegate runtime.ToolRunner
}

// RunTool steps the caller's session to the call's time and binds the values
// read there. Calls without StepVariable keep the delegate's behavior.
func (r *sessionToolRunner) RunTool(call *runtime.ToolCall) (runtime.ToolAnswer, error) {
	t, stepped, err := stepTimeOf(call)
	if err != nil {
		return runtime.ToolAnswer{}, err
	}
	if call.ToolName != "fmi" || !stepped {
		if r.delegate == nil {
			return runtime.ToolAnswer{}, &runtime.ToolNotRegisteredError{Tool: call.ToolName}
		}
		return r.delegate.RunTool(call)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.manager.timeoutOf())
	defer cancel()
	s, err := r.manager.Open(ctx, r.caller, call)
	if err != nil {
		return runtime.ToolAnswer{}, err
	}
	names := make([]string, 0, len(call.Outputs))
	for _, out := range call.Outputs {
		names = append(names, out.Variable)
	}
	if err := s.Step(ctx, t); err != nil {
		return runtime.ToolAnswer{}, err
	}
	values, err := s.Read(ctx, names)
	if err != nil {
		return runtime.ToolAnswer{}, err
	}
	bound, err := call.Bind(values)
	if err != nil {
		return runtime.ToolAnswer{}, err
	}
	return runtime.ToolAnswer{Outputs: bound}, nil
}

// SessionRequiredError refuses a stepped call reaching the one-shot engine,
// which could only answer at stopTime.
type SessionRequiredError struct{ Tool string }

// Error names the reserved input and the missing surface.
func (e *SessionRequiredError) Error() string {
	return fmt.Sprintf("%s: %s needs a session tool runner; the one-shot engine reads outputs at fmi:stopTime", e.Tool, StepVariable)
}

// Is matches ErrSession.
func (e *SessionRequiredError) Is(target error) bool { return target == ErrSession }

// namesStep reports whether a call asks for a session step.
func namesStep(call *runtime.ToolCall) bool {
	if call == nil {
		return false
	}
	for _, in := range call.Inputs {
		if in.Variable == StepVariable {
			return true
		}
	}
	return false
}

// ErrSessionSeries is the typed error for a stepped call that also asks for a
// trajectory.
var ErrSessionSeries = errors.New("fmi session does not answer series")

// SessionSeriesError refuses a step combined with series variables: a session
// answers scalars per step, a trajectory is the one-shot runner's answer.
type SessionSeriesError struct{ Tool string }

// Error names the reserved input and the conflicting series variables.
func (e *SessionSeriesError) Error() string {
	return fmt.Sprintf("%s: %s cannot be combined with fmi:samples, fmi:sampleInterval or fmi:series:* outputs", e.Tool, StepVariable)
}

// Is matches ErrSessionSeries and ErrSession.
func (e *SessionSeriesError) Is(target error) bool {
	return target == ErrSessionSeries || target == ErrSession
}

// namesSeries reports whether a call asks for any trajectory input or output.
func namesSeries(call *runtime.ToolCall) bool {
	if call == nil {
		return false
	}
	for _, in := range call.Inputs {
		if in.Variable == seriesSamplesInput || in.Variable == seriesIntervalInput {
			return true
		}
	}
	for _, out := range call.Outputs {
		if isSeriesOutput(out.Variable) {
			return true
		}
	}
	return false
}

// stepTimeOf finds the reserved step time of a call, if it names one.
func stepTimeOf(call *runtime.ToolCall) (float64, bool, error) {
	if call == nil {
		return 0, false, nil
	}
	for _, in := range call.Inputs {
		if in.Variable == StepVariable {
			n, err := reservedNumber(call, in, "s")
			return n, true, err
		}
	}
	return 0, false, nil
}

// sessionProcess is one long-lived runner process. Commands are serialized: a
// session speaks exactly one request and waits for its answer before the next.
type sessionProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *analysis.BoundedBuffer
	limit  int
	grace  time.Duration
	cancel context.CancelFunc

	mu       sync.Mutex
	dead     chan struct{}
	waitErr  error
	closeOne sync.Once
}

// startSessionProcess spawns a runner that will speak the session protocol. Its
// standard error is bounded and an overrun ends it, as with the one-shot tool.
func startSessionProcess(ctx context.Context, runner string, limit int, timeout time.Duration) (*sessionProcess, error) {
	// The process outlives the opening call, but a command deadline, a read
	// error, a protocol fault or Close ends it.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(runCtx, runner) // #nosec G204 -- the runner is the operator's grant
	list, err := runnerEnv()
	if err != nil {
		cancel()
		return nil, err
	}
	cmd.Env = list
	stderr := analysis.NewBoundedBuffer(limit, cancel)
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cancel()
		return nil, err
	}
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		cancel()
		return nil, err
	}
	p := &sessionProcess{cmd: cmd, stdin: stdin, stdout: bufio.NewReaderSize(stdout, 64<<10),
		stderr: stderr, limit: limit, grace: timeout, cancel: cancel, dead: make(chan struct{})}
	go func() {
		p.waitErr = cmd.Wait()
		close(p.dead)
	}()
	return p, nil
}

// command writes one line and reads its one answer. A deadline, output
// overrun or runner exit ends the process with a typed fault.
func (p *sessionProcess) command(ctx context.Context, cmd sessionCommand, limit int) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.dead:
		return nil, p.failure()
	default:
	}
	raw, err := json.Marshal(cmd)
	if err != nil {
		return nil, err
	}
	if _, err := p.stdin.Write(append(raw, '\n')); err != nil {
		return nil, p.failureOr(err)
	}
	type read struct {
		line []byte
		err  error
	}
	answer := make(chan read, 1)
	go func() {
		line, err := readBoundedLine(p.stdout, limit)
		answer <- read{line: line, err: err}
	}()
	select {
	case got := <-answer:
		if got.err != nil {
			// A process that exited while the read waited has a richer fault
			// than the pipe's EOF: its exit status and bounded standard error.
			select {
			case <-p.dead:
			case <-time.After(200 * time.Millisecond):
			}
			select {
			case <-p.dead:
				return nil, p.failure()
			default:
			}
			p.cancel()
			return nil, got.err
		}
		return got.line, nil
	case <-ctx.Done():
		p.cancel()
		return nil, ctx.Err()
	case <-p.dead:
		return nil, p.failure()
	}
}

// failure is why the runner process ended, with its bounded standard error.
func (p *sessionProcess) failure() error {
	select {
	case <-p.dead:
	default:
		<-p.dead
	}
	detail := analysis.ProcessDetail(p.cmd.Path, p.waitErr, p.stderr.Bytes())
	return &runtime.ToolError{Tool: "fmi", Kind: runtime.ToolProcessFailed, Detail: detail}
}

func (p *sessionProcess) failureOr(err error) error {
	if p.stderr.Over() {
		return &runtime.ToolError{Tool: "fmi", Kind: runtime.ToolMalformed,
			Detail: fmt.Sprintf("the session runner wrote more than %d bytes to standard error (%s)", p.limit, analysis.OutputLimitEnv)}
	}
	return err
}

// close closes the runner's input and lets it exit, ending it if it does not.
func (p *sessionProcess) close() error {
	var err error
	p.closeOne.Do(func() {
		_ = p.stdin.Close()
		select {
		case <-p.dead:
		case <-time.After(p.grace):
			p.cancel()
			<-p.dead
		}
		if p.waitErr != nil {
			err = p.failure()
		}
	})
	return err
}

// readBoundedLine reads one newline-ended answer of at most limit bytes. The
// last answer may end at EOF.
func readBoundedLine(r *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		line = append(line, part...)
		if len(line) > limit+1 {
			return nil, fmt.Errorf("the runner wrote a line over %d bytes (%s)", limit, analysis.OutputLimitEnv)
		}
		switch {
		case err == nil:
			return bytes.TrimSpace(line), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(line) > 0:
			return bytes.TrimSpace(line), nil
		default:
			return nil, err
		}
	}
}

// valueReal is a session time as the reply carries it: a plain Real, with no
// unit — fmi:time is the experiment's own clock, not a measured quantity.
func valueReal(f float64) semantics.Value { return semantics.Value{Kind: semantics.ValReal, Real: f} }

// baseName keeps an error's file name short and stable across paths.
func baseName(path string) string {
	if path == "" {
		return "no FMU"
	}
	return filepath.Base(path)
}
