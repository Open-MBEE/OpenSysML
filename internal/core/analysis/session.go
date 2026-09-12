package analysis

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis/enginewire"
)

// ErrProtocol is the typed error for an engine that broke the message protocol.
var ErrProtocol = errors.New("engine broke protocol")

// ProtocolError reports a line the host could not take as a message: not JSON, no jsonrpc
// member, no id, a member of the wrong type, a line over the output bound, a witness of the
// wrong kind. The session is ended with it.
type ProtocolError struct {
	Engine string
	Detail string
}

// Error names the engine and what broke.
func (e *ProtocolError) Error() string {
	return fmt.Sprintf("engine %q broke protocol: %s", e.Engine, e.Detail)
}

// Is matches ErrProtocol.
func (e *ProtocolError) Is(target error) bool { return target == ErrProtocol }

// ErrEngineExited is the typed error for an engine process that ended before it answered.
var ErrEngineExited = errors.New("engine process exited")

// EngineExitedError reports the process ending: at start, or during a request. Stderr is
// what it wrote to standard error, within the output bound.
type EngineExitedError struct {
	Engine  string
	Process string
	// Started reports whether the process had answered describe before it ended.
	Started bool
	Err     error
	Stderr  string
}

// Error names the engine, the process and how it ended.
func (e *EngineExitedError) Error() string {
	how := "did not start"
	if e.Started {
		how = "exited during a request"
	}
	detail := exitText(e.Err)
	if e.Stderr != "" {
		detail += ": " + e.Stderr
	}
	return fmt.Sprintf("engine %q at %s %s (%s)", e.Engine, e.Process, how, detail)
}

// Is matches ErrEngineExited.
func (e *EngineExitedError) Is(target error) bool { return target == ErrEngineExited }

// Unwrap is the process error.
func (e *EngineExitedError) Unwrap() error { return e.Err }

// exitText spells how a process ended: `exit 127`, or the error as it is.
func exitText(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() >= 0 {
		return "exit " + strconv.Itoa(exit.ExitCode())
	}
	if err == nil {
		return "exit 0"
	}
	return err.Error()
}

// ErrHandshake is the typed error for a describe answer that disagrees with the manifest.
var ErrHandshake = errors.New("engine describes itself otherwise than its manifest entry")

// HandshakeError reports the field of describe that differs from the manifest entry.
type HandshakeError struct {
	Engine   string
	Field    string
	Manifest string
	Answer   string
}

// Error names the engine, the field and both spellings.
func (e *HandshakeError) Error() string {
	return fmt.Sprintf("engine %q describes its %s as %s; the manifest says %s", e.Engine, e.Field, e.Answer, e.Manifest)
}

// Is matches ErrHandshake.
func (e *HandshakeError) Is(target error) bool { return target == ErrHandshake }

// ErrEngineTimeout is the typed error for a message unanswered within OPENSYSML_TOOL_TIMEOUT.
var ErrEngineTimeout = errors.New("engine did not answer in time")

// EngineTimeoutError reports a request unanswered within the timeout: describe or covers
// within the round-trip bound, or a cancelled run within the grace after cancel.
type EngineTimeoutError struct {
	Engine  string
	Method  string
	Timeout time.Duration
}

// Error names the engine, the request and the bound.
func (e *EngineTimeoutError) Error() string {
	if e.Method == enginewire.MethodRun {
		return fmt.Sprintf("engine %q did not answer cancel within %s (%s); the process was ended", e.Engine, e.Timeout, ToolTimeoutEnv)
	}
	return fmt.Sprintf("engine %q did not answer %s within %s (%s)", e.Engine, e.Method, e.Timeout, ToolTimeoutEnv)
}

// Is matches ErrEngineTimeout.
func (e *EngineTimeoutError) Is(target error) bool { return target == ErrEngineTimeout }

// ErrEngineFault is the typed error for an engine answering a request with an error.
var ErrEngineFault = errors.New("engine answered with an error")

// EngineFaultError is the engine's error answer: its code and message.
type EngineFaultError struct {
	Engine  string
	Method  string
	Code    string
	Message string
}

// Error names the engine, the code and the message.
func (e *EngineFaultError) Error() string {
	return fmt.Sprintf("engine %q answered %s with error %s: %s", e.Engine, e.Method, e.Code, e.Message)
}

// Is matches ErrEngineFault.
func (e *EngineFaultError) Is(target error) bool { return target == ErrEngineFault }

// ErrSessionEnded is the typed error for a request to an engine whose session in this plan
// has already ended.
var ErrSessionEnded = errors.New("engine session ended")

// SessionEndedError reports a request after the session ended, with why it ended.
type SessionEndedError struct {
	Engine string
	Err    error
}

// Error names the engine and the end.
func (e *SessionEndedError) Error() string {
	return fmt.Sprintf("engine %q: its session ended earlier in this plan: %v", e.Engine, e.Err)
}

// Is matches ErrSessionEnded.
func (e *SessionEndedError) Is(target error) bool { return target == ErrSessionEnded }

// Unwrap is why the session ended.
func (e *SessionEndedError) Unwrap() error { return e.Err }

// ProgressReport is one coalesced progress notification of an external engine's run.
type ProgressReport struct {
	Engine   string
	Progress enginewire.ProgressParams
}

// Reporter takes the progress reports a plan's external engines send, at most one per
// ProgressInterval per open run; the surface that installs one prints them.
type Reporter func(ProgressReport)

// ProgressInterval is the least time between two reports of one run: an engine that sends a
// notification per state costs the pipe, not the report.
const ProgressInterval = 250 * time.Millisecond

type reporterKey struct{}

// WithReporter installs the reporter a plan's external engines report progress to.
func WithReporter(ctx context.Context, report Reporter) context.Context {
	return context.WithValue(ctx, reporterKey{}, report)
}

// ReporterFrom is the reporter WithReporter installed, nil when none was.
func ReporterFrom(ctx context.Context) Reporter {
	if report, ok := ctx.Value(reporterKey{}).(Reporter); ok {
		return report
	}
	return nil
}

// progressSink keeps the latest progress of one run and reports it at most once per
// ProgressInterval, never under the session's lock.
type progressSink struct {
	engine string
	report Reporter

	mu       sync.Mutex
	latest   enginewire.ProgressParams
	seen     bool
	reported time.Time
	timer    *time.Timer
	closed   bool
}

// newProgressSink is a sink reporting one run's progress to report, keeping it when nil.
func newProgressSink(engine string, report Reporter) *progressSink {
	return &progressSink{engine: engine, report: report}
}

// take keeps p as the latest and reports it now, or schedules the report when the last
// one was within the interval.
func (k *progressSink) take(p enginewire.ProgressParams) {
	k.mu.Lock()
	k.latest, k.seen = p, true
	if k.report == nil || k.closed || k.timer != nil {
		k.mu.Unlock()
		return
	}
	wait := ProgressInterval - time.Since(k.reported)
	if wait > 0 {
		k.timer = time.AfterFunc(wait, k.flush)
		k.mu.Unlock()
		return
	}
	k.reported = time.Now()
	k.mu.Unlock()
	k.report(ProgressReport{Engine: k.engine, Progress: p})
}

// flush reports the latest once the interval has passed, unless close delivered it first.
func (k *progressSink) flush() {
	k.mu.Lock()
	if k.timer == nil {
		k.mu.Unlock()
		return
	}
	k.timer = nil
	p := k.latest
	k.reported = time.Now()
	k.mu.Unlock()
	k.report(ProgressReport{Engine: k.engine, Progress: p})
}

// close stops reporting, delivering a report still scheduled; the latest stays for the
// reason of a run that ends unanswered.
func (k *progressSink) close() {
	k.mu.Lock()
	k.closed = true
	pending := k.timer != nil
	if pending {
		k.timer.Stop()
		k.timer = nil
	}
	p := k.latest
	k.mu.Unlock()
	if pending {
		k.report(ProgressReport{Engine: k.engine, Progress: p})
	}
}

// last is the latest progress the run reported, and whether it reported any.
func (k *progressSink) last() (enginewire.ProgressParams, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.latest, k.seen
}

// ProgressText spells progress as a report prints it: `runs 120, depth 17, steps 3: text`.
func ProgressText(p enginewire.ProgressParams) string {
	var parts []string
	if p.Runs != 0 {
		parts = append(parts, fmt.Sprintf("runs %d", p.Runs))
	}
	if p.Depth != 0 {
		parts = append(parts, fmt.Sprintf("depth %d", p.Depth))
	}
	if p.Steps != 0 {
		parts = append(parts, fmt.Sprintf("steps %d", p.Steps))
	}
	text := strings.Join(parts, ", ")
	switch {
	case text != "" && p.Text != "":
		return text + ": " + p.Text
	case p.Text != "":
		return p.Text
	}
	return text
}

// session is one external engine process: JSON-RPC lines on its pipes, requests matched to
// answers by id, one reader for every answer and notification. It ends at the first break
// of the protocol, when the process ends, or when the plan closes it.
type session struct {
	entry   EngineEntry
	limit   int
	timeout time.Duration
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stderr  *boundedBuffer
	writeMu sync.Mutex

	mu       sync.Mutex
	nextID   int64
	pending  map[int64]chan enginewire.Message
	progress map[int64]*progressSink
	// open counts the requests sent and not yet answered.
	open int
	// ended is closed when the reader is done; failure is why, nil for a clean end.
	ended   chan struct{}
	failure error
	// waited is the process's exit once Wait returned.
	waited chan struct{}
	exit   error

	described enginewire.Description
}

// startSession spawns the entry's command, never through PATH, and takes the engine's
// description, checked against the entry field by field. A process that does not start,
// does not describe itself within the timeout or describes itself otherwise is ended.
func startSession(entry EngineEntry, limit int, timeout time.Duration) (*session, error) {
	if err := entry.Present(); err != nil {
		return nil, err
	}
	s := &session{entry: entry, limit: limit, timeout: timeout,
		pending: make(map[int64]chan enginewire.Message), progress: make(map[int64]*progressSink),
		ended: make(chan struct{}), waited: make(chan struct{})}
	s.cmd = exec.Command(entry.Executable, entry.Command[1:]...) // #nosec G204 -- the manifest names the command
	s.cmd.Dir = entry.Dir
	ownProcessGroup(s.cmd)
	s.stderr = newBoundedBuffer(limit, nil)
	s.cmd.Stderr = s.stderr
	stdin, err := s.cmd.StdinPipe()
	if err != nil {
		return nil, s.notStarted(err)
	}
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		return nil, s.notStarted(err)
	}
	s.stdin = stdin
	if err := s.cmd.Start(); err != nil {
		return nil, s.notStarted(err)
	}
	go s.wait()
	go s.read(stdout)
	if err := s.describe(); err != nil {
		s.end(err)
		return nil, err
	}
	return s, nil
}

// notStarted is the error of a process that never ran.
func (s *session) notStarted(err error) error {
	return &EngineExitedError{Engine: s.entry.Name, Process: s.entry.Executable, Err: err}
}

// wait collects the process's exit.
func (s *session) wait() {
	s.exit = s.cmd.Wait()
	close(s.waited)
}

// describe takes the engine's description within the timeout and checks it.
func (s *session) describe() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	raw, err := s.call(ctx, enginewire.MethodDescribe, enginewire.DescribeParams{Protocols: ProtocolVersions}, nil)
	if err != nil {
		var exited *EngineExitedError
		if errors.As(err, &exited) {
			exited.Started = false
		}
		return err
	}
	var d enginewire.Description
	if err := decodeOne(raw, &d); err != nil {
		return s.broke("the describe result is not a description: " + err.Error())
	}
	if err := s.entry.check(d); err != nil {
		return err
	}
	s.described = d
	return nil
}

// check compares a description with the manifest entry, field by field: name, version,
// protocol and answers always; the other fields when the engine states them.
func (e EngineEntry) check(d enginewire.Description) error {
	mismatch := func(field, manifest, answer string) error {
		return &HandshakeError{Engine: e.Name, Field: field, Manifest: manifest, Answer: answer}
	}
	if d.Name != e.Name {
		return mismatch("name", strconv.Quote(e.Name), strconv.Quote(d.Name))
	}
	if d.Version != e.Version {
		return mismatch("version", strconv.Quote(e.Version), strconv.Quote(d.Version))
	}
	if d.Protocol != e.Protocol {
		return mismatch("protocol", strconv.Itoa(e.Protocol), strconv.Itoa(d.Protocol))
	}
	answers := make([]string, len(e.Answers))
	for i, k := range e.Answers {
		answers[i] = k.String()
	}
	if !sameSet(answers, d.Answers) {
		return mismatch("answers", spellList(answers), spellList(d.Answers))
	}
	if d.Subjects != nil && !sameSet(e.Subjects, d.Subjects) {
		return mismatch("subjects", spellList(e.Subjects), spellList(d.Subjects))
	}
	if d.Bounds != nil && !sameSet(e.Bounds, d.Bounds) {
		return mismatch("bounds", spellList(e.Bounds), spellList(d.Bounds))
	}
	if d.Witness != "" && d.Witness != string(e.Witness) {
		return mismatch("witness", string(e.Witness), d.Witness)
	}
	if d.Model != nil {
		forms := make([]string, len(e.Model))
		for i, f := range e.Model {
			forms[i] = string(f)
		}
		if !sameSet(forms, d.Model) {
			return mismatch("model", spellList(forms), spellList(d.Model))
		}
	}
	if d.Authority != "" && d.Authority != e.Authority.String() {
		return mismatch("authority", e.Authority.String(), d.Authority)
	}
	if d.Concurrent != nil && *d.Concurrent != e.Concurrent {
		return mismatch("concurrent", strconv.FormatBool(e.Concurrent), strconv.FormatBool(*d.Concurrent))
	}
	return nil
}

// sameSet reports whether two lists name the same strings, in any order.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		if seen[x] == 0 {
			return false
		}
		seen[x]--
	}
	return true
}

// spellList spells a list as the manifest would.
func spellList(list []string) string {
	quoted := make([]string, len(list))
	for i, x := range list {
		quoted[i] = strconv.Quote(x)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// broke is the protocol error that ends the session.
func (s *session) broke(detail string) error {
	return &ProtocolError{Engine: s.entry.Name, Detail: detail}
}

// call sends one request and waits for its answer until ctx is done. A run whose ctx ends
// first is sent cancel and given the timeout's grace to answer; a run that does not answer
// within it ends the process. A run's progress goes to sink, coalesced.
func (s *session) call(ctx context.Context, method string, params any, sink *progressSink) (json.RawMessage, error) {
	id, reply, err := s.send(method, params, sink)
	if err != nil {
		return nil, err
	}
	defer s.forget(id)
	if sink != nil {
		defer sink.close()
	}
	select {
	case msg := <-reply:
		return s.answer(method, msg)
	case <-s.ended:
		return nil, s.unanswered(sink)
	case <-ctx.Done():
	}
	if method != enginewire.MethodRun {
		s.end(&EngineTimeoutError{Engine: s.entry.Name, Method: method, Timeout: s.timeout})
		return nil, s.failed()
	}
	if err := s.notify(enginewire.MethodCancel, enginewire.CancelParams{ID: id}); err != nil {
		return nil, s.unanswered(sink, s.unwritable(err))
	}
	grace := time.NewTimer(s.timeout)
	defer grace.Stop()
	select {
	case msg := <-reply:
		return s.answer(method, msg)
	case <-s.ended:
		return nil, s.unanswered(sink)
	case <-grace.C:
		s.end(&EngineTimeoutError{Engine: s.entry.Name, Method: method, Timeout: s.timeout})
		return nil, s.unanswered(sink)
	}
}

// unanswered is the error of a run that ended without an answer, keeping the last progress
// it reported; err is the end's, s.failed() when none is given.
func (s *session) unanswered(sink *progressSink, err ...error) error {
	end := s.failed()
	if len(err) > 0 {
		end = err[0]
	}
	if sink == nil {
		return end
	}
	last, seen := sink.last()
	if !seen {
		return end
	}
	return &UnansweredRunError{Engine: s.entry.Name, Progress: last, Err: end}
}

// answer reads one response: its result, or its error as the engine's fault.
func (s *session) answer(method string, msg enginewire.Message) (json.RawMessage, error) {
	if msg.Error != nil {
		return nil, &EngineFaultError{Engine: s.entry.Name, Method: method, Code: msg.Error.Code, Message: msg.Error.Message}
	}
	return msg.Result, nil
}

// send writes one request under a fresh id and registers where its answer goes.
func (s *session) send(method string, params any, sink *progressSink) (int64, chan enginewire.Message, error) {
	s.mu.Lock()
	if s.failure != nil || s.isEnded() {
		s.mu.Unlock()
		return 0, nil, s.failed()
	}
	s.nextID++
	id := s.nextID
	reply := make(chan enginewire.Message, 1)
	s.pending[id] = reply
	if sink != nil {
		s.progress[id] = sink
	}
	s.open++
	s.mu.Unlock()
	if err := s.write(enginewire.Message{JSONRPC: enginewire.JSONRPC, ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method}, params); err != nil {
		s.forget(id)
		return 0, nil, s.unwritable(err)
	}
	return id, reply, nil
}

// unwritable is the error of a message the pipe did not take: the process's end, as the
// reader reports it once it sees the pipe close, else the write's own error.
func (s *session) unwritable(err error) error {
	select {
	case <-s.ended:
	case <-time.After(s.timeout):
		s.end(&EngineExitedError{Engine: s.entry.Name, Process: s.entry.Executable, Started: true,
			Err: err, Stderr: strings.TrimSpace(string(s.stderr.Bytes()))})
	}
	return s.failed()
}

// forget drops a request's registration once it is answered or abandoned.
func (s *session) forget(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[id]; ok {
		delete(s.pending, id)
		delete(s.progress, id)
		s.open--
	}
}

// notify writes one notification.
func (s *session) notify(method string, params any) error {
	return s.write(enginewire.Message{JSONRPC: enginewire.JSONRPC, Method: method}, params)
}

// write frames one message as a line. Params are encoded apart so a nil stays absent.
func (s *session) write(msg enginewire.Message, params any) error {
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return err
		}
		msg.Params = raw
	}
	line, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = s.stdin.Write(append(line, '\n'))
	return err
}

// read takes every line the engine writes until the pipe ends or a line breaks the protocol.
func (s *session) read(stdout io.Reader) {
	r := bufio.NewReaderSize(stdout, 64<<10)
	for {
		line, err := readLine(r, s.limit)
		switch {
		case errors.Is(err, errLineTooLong):
			s.end(s.broke(fmt.Sprintf("wrote a line over %d bytes (%s)", s.limit, OutputLimitEnv)))
			return
		case err != nil:
			s.end(s.exited(err))
			return
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if err := s.take(line); err != nil {
			s.end(err)
			return
		}
	}
}

// exited is the error of a process whose pipe ended: how it exited, with its standard error.
func (s *session) exited(readErr error) error {
	err := readErr
	if errors.Is(readErr, io.EOF) {
		err = nil
	}
	select {
	case <-s.waited:
		if s.exit != nil {
			err = s.exit
		}
	case <-time.After(s.timeout):
	}
	return &EngineExitedError{Engine: s.entry.Name, Process: s.entry.Executable, Started: true,
		Err: err, Stderr: strings.TrimSpace(string(s.stderr.Bytes()))}
}

// errLineTooLong is a line over the output bound.
var errLineTooLong = errors.New("line over the output bound")

// readLine reads one newline-ended line of at most limit bytes; the last line may end at EOF.
func readLine(r *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		line = append(line, part...)
		if len(line) > limit+1 {
			return nil, errLineTooLong
		}
		switch {
		case err == nil:
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(line) > 0:
			return line, nil
		default:
			return nil, err
		}
	}
}

// take reads one line as a message: an answer to a pending request, or a progress
// notification of an open run. Anything else breaks the protocol.
func (s *session) take(line []byte) error {
	var msg enginewire.Message
	if err := decodeOne(line, &msg); err != nil {
		return s.broke("a line is not a JSON-RPC message: " + err.Error())
	}
	if msg.JSONRPC != enginewire.JSONRPC {
		return s.broke(fmt.Sprintf("a message carries jsonrpc %q, not %q", msg.JSONRPC, enginewire.JSONRPC))
	}
	if msg.Method != "" {
		return s.notification(msg)
	}
	if len(msg.ID) == 0 || string(msg.ID) == "null" {
		return s.broke("a response carries no id")
	}
	id, err := strconv.ParseInt(string(msg.ID), 10, 64)
	if err != nil {
		return s.broke(fmt.Sprintf("a response carries id %s, not the number of a request", msg.ID))
	}
	if (msg.Result == nil) == (msg.Error == nil) {
		return s.broke(fmt.Sprintf("the response to %d carries neither result nor error, or both", id))
	}
	if msg.Error != nil && msg.Error.Code == "" {
		return s.broke(fmt.Sprintf("the error answering %d carries no code", id))
	}
	s.mu.Lock()
	reply, ok := s.pending[id]
	s.mu.Unlock()
	if !ok {
		return s.broke(fmt.Sprintf("a response answers %d, which is no open request", id))
	}
	reply <- msg
	return nil
}

// notification takes the one notification the engine sends, progress, for an open run.
func (s *session) notification(msg enginewire.Message) error {
	if len(msg.ID) != 0 {
		return s.broke(fmt.Sprintf("the engine sent a %s request; the host takes no requests", msg.Method))
	}
	if msg.Method != enginewire.MethodProgress {
		return s.broke(fmt.Sprintf("the engine sent a %s notification; the host takes progress alone", msg.Method))
	}
	var p enginewire.ProgressParams
	if err := decodeOne(msg.Params, &p); err != nil {
		return s.broke("progress params are malformed: " + err.Error())
	}
	s.mu.Lock()
	sink, open := s.progress[p.ID]
	_, pending := s.pending[p.ID]
	s.mu.Unlock()
	if !pending {
		return s.broke(fmt.Sprintf("progress names run %d, which is no open request", p.ID))
	}
	if open {
		sink.take(p)
	}
	return nil
}

// ErrUnansweredRun is the typed error for a run that ended without an answer after
// reporting progress.
var ErrUnansweredRun = errors.New("run ended unanswered")

// UnansweredRunError is why a run ended unanswered, with the last progress it reported.
type UnansweredRunError struct {
	Engine   string
	Progress enginewire.ProgressParams
	Err      error
}

// Error names the end and the last progress.
func (e *UnansweredRunError) Error() string {
	return fmt.Sprintf("%v; its last progress was %s", e.Err, ProgressText(e.Progress))
}

// Is matches ErrUnansweredRun.
func (e *UnansweredRunError) Is(target error) bool { return target == ErrUnansweredRun }

// Unwrap is the end itself.
func (e *UnansweredRunError) Unwrap() error { return e.Err }

// end records why the session ended, once, and ends the process and its children at once.
func (s *session) end(err error) {
	if s.finish(err) {
		s.kill(0)
	}
}

// finish records why the session ended; false when it had ended already.
func (s *session) finish(err error) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isEnded() {
		return false
	}
	s.failure = err
	close(s.ended)
	return true
}

// isEnded reports whether ended is closed; called under mu.
func (s *session) isEnded() bool {
	select {
	case <-s.ended:
		return true
	default:
		return false
	}
}

// failed is why the session ended, as every request after that is refused.
func (s *session) failed() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure == nil {
		return &SessionEndedError{Engine: s.entry.Name, Err: io.EOF}
	}
	return s.failure
}

// kill closes the engine's standard input, gives it grace to end, then ends it and its
// children and waits for it.
func (s *session) kill(grace time.Duration) {
	_ = s.stdin.Close()
	if grace > 0 {
		select {
		case <-s.waited:
			return
		case <-time.After(grace):
		}
	}
	killProcessGroup(s.cmd)
	<-s.waited
}

// close ends the session at the plan's end: standard input is closed, and a process still
// running after the timeout is ended with its children.
func (s *session) close() {
	if s.finish(&SessionEndedError{Engine: s.entry.Name, Err: errPlanEnded}) {
		s.kill(s.timeout)
	}
}

// errPlanEnded is why a session closed with its plan.
var errPlanEnded = errors.New("the plan ended")

// alive reports whether the session can still take a request.
func (s *session) alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure == nil && !s.isEnded()
}

// enginePool is one plan's processes of one external engine: a single session every request
// shares when the entry is concurrent, else one session per open request, idle ones reused.
// The first session to end by fault ends the engine for the plan; close ends them all.
type enginePool struct {
	entry   EngineEntry
	limit   int
	timeout time.Duration
	start   func(EngineEntry, int, time.Duration) (*session, error)

	mu      sync.Mutex
	shared  *session
	idle    []*session
	all     []*session
	failure error
	closed  bool
	// starting is closed once the shared process's start has been attempted; requests
	// arriving during it wait rather than start another.
	starting chan struct{}
}

// newEnginePool is the pool of an entry's processes under the output bound and the timeout.
func newEnginePool(entry EngineEntry, limit int, timeout time.Duration) *enginePool {
	return &enginePool{entry: entry, limit: limit, timeout: timeout, start: startSession}
}

// acquire is a session with no other request open on it when the entry is not concurrent,
// the shared one otherwise; a new process is started when none serves. Give it back with put.
func (p *enginePool) acquire() (*session, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, &SessionEndedError{Engine: p.entry.Name, Err: errPlanEnded}
	}
	if p.failure != nil {
		p.mu.Unlock()
		return nil, &SessionEndedError{Engine: p.entry.Name, Err: p.failure}
	}
	if p.entry.Concurrent {
		return p.acquireShared()
	}
	for len(p.idle) > 0 {
		s := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		if s.alive() {
			p.mu.Unlock()
			return s, nil
		}
	}
	p.mu.Unlock()
	s, err := p.start(p.entry, p.limit, p.timeout)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		p.failure = err
		return nil, err
	}
	if p.closed {
		go s.close()
		return nil, &SessionEndedError{Engine: p.entry.Name, Err: errPlanEnded}
	}
	p.all = append(p.all, s)
	return s, nil
}

// acquireShared is the plan's one process of a concurrent entry, started by the first request
// while the others wait for it. Called with mu held, which it releases.
func (p *enginePool) acquireShared() (*session, error) {
	if p.shared != nil {
		s := p.shared
		p.mu.Unlock()
		return s, nil
	}
	if p.starting != nil {
		starting := p.starting
		p.mu.Unlock()
		<-starting
		p.mu.Lock()
		defer p.mu.Unlock()
		switch {
		case p.closed:
			return nil, &SessionEndedError{Engine: p.entry.Name, Err: errPlanEnded}
		case p.failure != nil:
			return nil, &SessionEndedError{Engine: p.entry.Name, Err: p.failure}
		}
		return p.shared, nil
	}
	p.starting = make(chan struct{})
	p.mu.Unlock()
	s, err := p.start(p.entry, p.limit, p.timeout)
	p.mu.Lock()
	defer p.mu.Unlock()
	defer close(p.starting)
	if err != nil {
		p.failure = err
		return nil, err
	}
	if p.closed {
		go s.close()
		return nil, &SessionEndedError{Engine: p.entry.Name, Err: errPlanEnded}
	}
	p.all = append(p.all, s)
	p.shared = s
	return s, nil
}

// put gives a session back once its request is answered: an ended one records the fault for
// the plan, a live nonconcurrent one waits idle for the next request.
func (p *enginePool) put(s *session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !s.alive() {
		if p.failure == nil && !p.closed {
			p.failure = s.failed()
		}
		return
	}
	if !p.entry.Concurrent {
		p.idle = append(p.idle, s)
	}
}

// close ends every process of the pool with the plan.
func (p *enginePool) close() {
	p.mu.Lock()
	p.closed = true
	all := p.all
	p.all, p.idle, p.shared = nil, nil, nil
	p.mu.Unlock()
	var wg sync.WaitGroup
	for _, s := range all {
		wg.Add(1)
		go func(s *session) {
			defer wg.Done()
			s.close()
		}(s)
	}
	wg.Wait()
}

// processes counts the sessions the pool has started for the plan.
func (p *enginePool) processes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.all)
}

// sessionPlan holds one plan's pools, one per external engine, shared by every fleet of the
// plan and closed with it.
type sessionPlan struct {
	mu     sync.Mutex
	pools  map[string]*enginePool
	closed bool
}

// pool is the plan's pool for an engine, made on first use by make.
func (sp *sessionPlan) pool(name string, make func() *enginePool) (*enginePool, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.closed {
		return nil, &SessionEndedError{Engine: name, Err: errPlanEnded}
	}
	if sp.pools == nil {
		sp.pools = map[string]*enginePool{}
	}
	p, ok := sp.pools[name]
	if !ok {
		p = make()
		sp.pools[name] = p
	}
	return p, nil
}

// close ends every engine's processes of the plan.
func (sp *sessionPlan) close() {
	sp.mu.Lock()
	sp.closed = true
	pools := sp.pools
	sp.pools = nil
	sp.mu.Unlock()
	for _, p := range pools {
		p.close()
	}
}
