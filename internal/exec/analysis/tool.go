package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// ToolEnginePrefix starts the name of every tool engine: `tool:ModelCenter`.
const ToolEnginePrefix = "tool:"

// ToolEngineName is the name of the engine answering for the tool named.
func ToolEngineName(tool string) string { return ToolEnginePrefix + tool }

// ComputeAsk is the invocation a Compute question asks for: one performance of an action
// annotated ToolExecution, as the runtime hands it to the tool.
type ComputeAsk struct {
	Call *runtime.ToolCall
}

// toolEngine answers Compute questions for one manifest entry by running its executable
// once per invocation, with the protocol below over its standard input and output.
type toolEngine struct {
	entry   ToolEntry
	fault   error
	look    func(ToolEntry) (string, error)
	timeout func() time.Duration
	limit   func() int
}

// NewTool returns the `tool:<name>` engine of a manifest entry. It registers whether or not
// the executable is found and refuses through Covers while it is not. An invocation or reply
// block not yet checked by a manifest load is checked here; a faulty one refuses every question.
func NewTool(entry ToolEntry) External {
	e := toolEngine{entry: entry, look: lookExecutable, timeout: toolTimeoutFromEnv, limit: outputLimitFromEnv}
	dir, at := "", e.Name()
	if entry.File != "" {
		dir, at = filepath.Dir(entry.File), entry.File
	}
	if inv := entry.Invocation; inv != nil && inv.compiled == nil {
		checked := *inv
		e.entry.Invocation = &checked
		if err := checkInvocation(&e.entry, dir); err != nil {
			e.fault = &ManifestError{Path: at, Detail: err.Error()}
		}
	}
	if r := entry.Reply; r != nil && r.compiled == nil && e.fault == nil {
		checked := *r
		e.entry.Reply = &checked
		if err := checkReply(&e.entry); err != nil {
			e.fault = &ManifestError{Path: at, Detail: err.Error()}
		}
	}
	return e
}

// Name is `tool:` and the tool's name.
func (e toolEngine) Name() string { return ToolEngineName(e.entry.ToolName) }

// Describe: one process per invocation, whose answer nothing in the build can check.
func (e toolEngine) Describe() Description {
	return Description{
		Questions: []Kind{Compute},
		Process:   fmt.Sprintf("the executable %s of tool '%s'", e.entry.Executable, e.entry.ToolName),
		Bounds:    []string{"tool"},
		Authority: Observed,
	}
}

// Origin is the tool's manifest entry.
func (e toolEngine) Origin() Origin {
	return Origin{Kind: KindTool, Version: e.entry.Version, File: e.entry.File, Command: e.entry.Executable, Exchange: e.entry.Protocol()}
}

// Process names the executable found, with the tool's version, or reports its absence or
// the entry's fault.
func (e toolEngine) Process() (string, error) {
	if e.fault != nil {
		return "", e.fault
	}
	path, err := e.look(e.entry)
	if err != nil {
		return "", &ProcessAbsentError{Engine: e.Name(), Process: e.Describe().Process, Err: err}
	}
	if e.entry.Version != "" {
		return fmt.Sprintf("%s %s at %s", e.entry.ToolName, e.entry.Version, path), nil
	}
	return e.entry.ToolName + " at " + path, nil
}

// Covers takes a Compute question whose call names this tool with variables it accepts,
// when the executable is found.
func (e toolEngine) Covers(_ *Model, q Question) Coverage {
	if q.Kind != Compute {
		return refused(&NotAskedError{Engine: e.Name(), Kind: q.Kind})
	}
	if q.Compute == nil || q.Compute.Call == nil {
		return refused(&MalformedQuestionError{Kind: q.Kind, Missing: "a Compute with a Call"})
	}
	call := q.Compute.Call
	if call.ToolName != e.entry.ToolName {
		return refused(&WrongToolError{Engine: e.Name(), Tool: call.ToolName})
	}
	if unknown := e.unaccepted(call); len(unknown) > 0 {
		return refused(&ToolVariableError{Tool: e.entry.ToolName, Variables: unknown})
	}
	if _, err := e.Process(); err != nil {
		return refused(err)
	}
	return covered
}

// unaccepted is every tool variable the call uses that the manifest entry does not accept.
func (e toolEngine) unaccepted(call *runtime.ToolCall) []string {
	var unknown []string
	seen := make(map[string]bool)
	for _, in := range call.Inputs {
		if !seen[in.Variable] && !e.entry.Accepts(in.Variable) {
			unknown, seen[in.Variable] = append(unknown, in.Variable), true
		}
	}
	for _, out := range call.Outputs {
		if !seen[out.Variable] && !e.entry.Accepts(out.Variable) {
			unknown, seen[out.Variable] = append(unknown, out.Variable), true
		}
	}
	sort.Strings(unknown)
	return unknown
}

// Run invokes the tool once: the call as one JSON object on its standard input, or as the
// entry's invocation block composes it, its one JSON object on standard output bound to the
// call's outputs. Every failure of the process or the protocol is a runtime.ToolError, which
// fails the performance that asked.
func (e toolEngine) Run(ctx context.Context, _ *Model, q Question, _ Budget) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if q.Compute == nil || q.Compute.Call == nil {
		return Result{}, &MalformedQuestionError{Kind: q.Kind, Missing: "a Compute with a Call"}
	}
	call := q.Compute.Call
	if e.fault != nil {
		return Result{}, e.fault
	}
	path, err := e.look(e.entry)
	if err != nil {
		return Result{}, &ProcessAbsentError{Engine: e.Name(), Process: e.Describe().Process, Err: err}
	}
	request, err := ToolRequestOf(call)
	if err != nil {
		return Result{}, err
	}
	process := &composed{stdin: request}
	if e.entry.Invocation != nil {
		if process, err = e.entry.Invocation.compose(call, e.entry, request); err != nil {
			return Result{}, err
		}
	}
	timeout := e.timeout()
	started := time.Now()
	ex, err := e.invoke(ctx, path, process, timeout, e.outputLimit())
	if err != nil {
		process.remove()
		return Result{}, err
	}
	reply, err := e.read(call, ex, process)
	process.remove()
	if err != nil {
		return Result{}, err
	}
	bound, err := call.Bind(reply)
	if err != nil {
		return Result{}, err
	}
	values := make([]Evaluation, 0, len(bound))
	for name, value := range bound {
		values = append(values, Evaluation{Name: name, Value: value})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	return Result{
		Question: q,
		Engine:   e.Name(),
		Claim:    ClaimValue,
		Strength: Observed,
		Values:   values,
		Reply:    renderReply(reply),
		Bounds:   Bounds{{Name: "tool", Limit: timeout.Milliseconds()}},
		Elapsed:  time.Since(started),
	}, nil
}

// renderReply spells the tool's reply canonically, by variable name, as it was written and
// before binding: two invocations of equal inputs compare by it.
func renderReply(reply map[string]runtime.ToolValue) string {
	parts := make([]string, 0, len(reply))
	for variable, v := range reply {
		part := variable + "="
		if v.Value.Kind == semantics.ValInvalid {
			part += strconv.Quote(v.Text)
		} else {
			part += semantics.FormatConst(v.Value)
		}
		if v.Unit != "" {
			part += " [" + v.Unit + "]"
		}
		parts = append(parts, part)
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// outputLimit is the bound on one reply, OutputLimitEnv's unless the engine was built without it.
func (e toolEngine) outputLimit() int {
	if e.limit == nil {
		return outputLimitFromEnv()
	}
	return e.limit()
}

// execution is what one run of the executable produced: its captured standard streams and
// its exit status.
type execution struct {
	stdout, stderr []byte
	exit           int
}

// invoke runs the executable once as composed, under the timeout, and captures what it
// produced. Each argument is one argv entry as rendered; no shell is involved. A nil env
// inherits this process's, as an entry without an invocation block does. Under an `exitcode`
// reply a real exit status is data, not a process failure; anything else failed is.
func (e toolEngine) invoke(ctx context.Context, path string, process *composed, timeout time.Duration, limit int) (*execution, error) {
	tool := e.entry.ToolName
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(tctx, path, process.args...) // #nosec G204 -- the manifest names the executable; the arguments are values
	cmd.Env, cmd.Dir = process.env, process.dir
	cmd.Stdin = bytes.NewReader(process.stdin)
	stdout, stderr := newBoundedBuffer(limit, cancel), newBoundedBuffer(limit, cancel)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	switch {
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case stdout.Over() || stderr.Over():
		stream := "output"
		if stderr.Over() {
			stream = "error"
		}
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed,
			Detail: fmt.Sprintf("%s wrote more than %d bytes to standard %s (%s)", path, limit, stream, OutputLimitEnv)}
	case errors.Is(tctx.Err(), context.DeadlineExceeded):
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolTimeout,
			Detail: fmt.Sprintf("%s did not answer within %s (%s)", path, timeout, ToolTimeoutEnv)}
	case err != nil:
		var exitErr *exec.ExitError
		if e.entry.Reply != nil && e.entry.Reply.Format == ReplyExitCode && errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
			return &execution{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exit: exitErr.ExitCode()}, nil
		}
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolProcessFailed, Detail: processDetail(path, err, stderr.Bytes())}
	}
	return &execution{stdout: stdout.Bytes(), stderr: stderr.Bytes()}, nil
}

// read parses the execution's reply as the entry's reply block says, reading the file a
// `file:` source names before the invocation's directory is removed.
func (e toolEngine) read(call *runtime.ToolCall, ex *execution, process *composed) (map[string]runtime.ToolValue, error) {
	tool := e.entry.ToolName
	r := e.entry.Reply
	if r == nil || r.Format == "" || r.Format == ReplyObject {
		source, err := e.replySource(ex, process)
		if err != nil {
			return nil, err
		}
		return ToolReplyOf(tool, source)
	}
	wanted := make(map[string]bool, len(call.Outputs))
	for _, out := range call.Outputs {
		wanted[out.Variable] = true
	}
	if r.Format == ReplyExitCode {
		return r.readExitCode(ex, wanted), nil
	}
	source, err := e.replySource(ex, process)
	if err != nil {
		return nil, err
	}
	if r.Format == ReplyJSON {
		return r.readJSON(e.entry, source, wanted)
	}
	if r.Format == ReplyCSV {
		return r.readCSV(e.entry, source, wanted)
	}
	return r.readLines(e.entry, source, wanted)
}

// replySource is the reply's bytes: standard output, or the file a `file:` source names —
// confined to {outputDir}, a regular file the tool wrote, within the output bound.
func (e toolEngine) replySource(ex *execution, process *composed) ([]byte, error) {
	tool := e.entry.ToolName
	r := e.entry.Reply
	if r == nil || r.compiled == nil || r.compiled.source == nil {
		return ex.stdout, nil
	}
	outputDir := filepath.Clean(process.tempDir)
	path, err := r.compiled.source.render(scope{tool: tool, outputDir: outputDir})
	if err != nil {
		return nil, err
	}
	path = filepath.Clean(path)
	if !within(outputDir, path) {
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed,
			Detail: fmt.Sprintf("reply source %s escapes {outputDir}", path)}
	}
	rel, err := filepath.Rel(outputDir, path)
	if err != nil {
		rel = path
	}
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: "wrote no file " + rel}
	case err != nil:
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: "cannot read " + rel + ": " + err.Error()}
	case !info.Mode().IsRegular():
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: rel + " is not a regular file"}
	}
	// Symlinks among the path's ancestors may not lead out of {outputDir} either.
	real, err := filepath.EvalSymlinks(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: "wrote no file " + rel}
	case err != nil:
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: "cannot read " + rel + ": " + err.Error()}
	}
	realDir, err := filepath.EvalSymlinks(outputDir)
	if err != nil {
		realDir = outputDir
	}
	if !within(realDir, real) {
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed,
			Detail: fmt.Sprintf("reply source %s escapes {outputDir}", path)}
	}
	f, err := os.Open(path) // #nosec G304 -- a file under the invocation's own directory
	if err != nil {
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: "cannot read " + rel + ": " + err.Error()}
	}
	defer f.Close() // the read below is the only use; a close error cannot lose data
	limit := e.outputLimit()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: "cannot read " + rel + ": " + err.Error()}
	}
	if len(data) > limit {
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed,
			Detail: fmt.Sprintf("wrote more than %d bytes to %s (%s)", limit, rel, OutputLimitEnv)}
	}
	return data, nil
}

// processDetail spells a failed process: how it exited and what it wrote to standard error.
func processDetail(path string, err error, stderr []byte) string {
	detail := path + ": " + err.Error()
	if text := strings.TrimSpace(string(stderr)); text != "" {
		detail += ": " + text
	}
	return detail
}

// toolRequest is the JSON object one invocation writes to the tool.
type toolRequest struct {
	ToolName string                   `json:"toolName"`
	URI      string                   `json:"uri"`
	Inputs   map[string]protocolValue `json:"inputs"`
}

// toolReply is the JSON object the tool writes back: outputs, or an error. Both are kept
// raw so a member written as null is told apart from one not written.
type toolReply struct {
	Outputs json.RawMessage `json:"outputs"`
	Error   json.RawMessage `json:"error"`
}

// jsonNull reports a raw member written as the literal null.
func jsonNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// protocolValue is one value on the wire: a JSON number, boolean or string, and for a
// quantity the unit expression it is measured in.
type protocolValue struct {
	Value json.RawMessage `json:"value"`
	Unit  string          `json:"unit,omitempty"`
}

// wiredValue is a protocolValue as the tool wrote it, the unit kept raw so one written as
// null or as no text is told apart from one omitted.
type wiredValue struct {
	Value json.RawMessage `json:"value"`
	Unit  json.RawMessage `json:"unit"`
}

// ToolRequestOf is the JSON object the protocol writes to the tool for one call: the
// toolName and uri passed through, the inputs keyed by ToolVariable name. Its bytes are
// the same for equal inputs, so two invocations compare by them.
func ToolRequestOf(call *runtime.ToolCall) ([]byte, error) {
	request := toolRequest{ToolName: call.ToolName, URI: call.URI, Inputs: make(map[string]protocolValue, len(call.Inputs))}
	for _, in := range call.Inputs {
		value, err := encodeValue(in.Value)
		if err != nil {
			return nil, &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolUnsentInput,
				Detail: fmt.Sprintf("%s (%s): %v", in.Variable, in.Parameter, err)}
		}
		request.Inputs[in.Variable] = protocolValue{Value: value, Unit: in.Value.Unit}
	}
	return json.Marshal(request)
}

// encodeValue is one runtime value as JSON: an integer, a finite real, a truth, or a string.
func encodeValue(v runtime.ToolValue) (json.RawMessage, error) {
	switch v.Value.Kind {
	case semantics.ValInt:
		return json.RawMessage(strconv.FormatInt(v.Value.Int, 10)), nil
	case semantics.ValReal:
		if math.IsInf(v.Value.Real, 0) || math.IsNaN(v.Value.Real) {
			return nil, fmt.Errorf("%v is not a JSON number", v.Value.Real)
		}
		return json.RawMessage(strconv.FormatFloat(v.Value.Real, 'g', -1, 64)), nil
	case semantics.ValBool:
		return json.RawMessage(strconv.FormatBool(v.Value.Bool)), nil
	case semantics.ValInvalid:
		return json.Marshal(v.Text)
	}
	return nil, fmt.Errorf("%s is not a JSON value", semantics.FormatConst(v.Value))
}

// ToolReplyOf reads the tool's standard output as the protocol's one JSON object: the
// outputs keyed by ToolVariable name, or the tool's own error as a ToolError.
func ToolReplyOf(tool string, stdout []byte) (map[string]runtime.ToolValue, error) {
	malformed := func(detail string, err error) error {
		if err != nil {
			detail += ": " + err.Error()
		}
		return &runtime.ToolError{Tool: tool, Kind: runtime.ToolMalformed, Detail: detail}
	}
	if len(bytes.TrimSpace(stdout)) == 0 {
		return nil, malformed("the tool wrote nothing to standard output", nil)
	}
	var reply toolReply
	if err := decodeOne(stdout, &reply); err != nil {
		return nil, malformed("standard output is not one JSON object of outputs or error", err)
	}
	if path, twice := repeatedKey(stdout); twice {
		return nil, malformed("the reply names "+path+" twice", nil)
	}
	if jsonNull(reply.Outputs) {
		reply.Outputs = nil
	}
	switch {
	case reply.Error != nil && reply.Outputs != nil:
		return nil, malformed("the reply carries both outputs and an error", nil)
	case jsonNull(reply.Error):
		return nil, malformed("the reply's error is null, not a message", nil)
	case reply.Error != nil:
		var message string
		if err := json.Unmarshal(reply.Error, &message); err != nil {
			return nil, malformed("the reply's error is not a message", err)
		}
		return nil, &runtime.ToolError{Tool: tool, Kind: runtime.ToolRefused, Detail: message}
	case reply.Outputs == nil:
		return nil, malformed("the reply carries neither outputs nor an error", nil)
	}
	var wired map[string]wiredValue
	if err := decodeOne(reply.Outputs, &wired); err != nil {
		return nil, malformed("outputs is not an object of values", err)
	}
	outputs := make(map[string]runtime.ToolValue, len(wired))
	for name, raw := range wired {
		value, err := decodeValue(raw)
		if err != nil {
			return nil, malformed(name+": "+err.Error(), nil)
		}
		outputs[name] = value
	}
	return outputs, nil
}

// repeatedKey is the first key an object anywhere in a JSON document spells twice, as the
// dotted path to it, which a struct or map decode would hide by keeping the last spelling.
// Only well-formed JSON is walked; anything else is left to the decoder to report.
func repeatedKey(document []byte) (string, bool) {
	dec := json.NewDecoder(bytes.NewReader(document))
	// One frame per open object or array; only an object's frame has seen keys.
	type frame struct {
		seen  map[string]bool
		inKey bool
	}
	var path []string
	var open []*frame
	valueDone := func() {
		if top := len(open) - 1; top >= 0 && open[top].inKey {
			open[top].inKey = false
			path = path[:len(path)-1]
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return "", false
		}
		switch tok {
		case json.Delim('{'):
			open = append(open, &frame{seen: make(map[string]bool)})
			continue
		case json.Delim('['):
			open = append(open, &frame{})
			continue
		case json.Delim('}'), json.Delim(']'):
			open = open[:len(open)-1]
			valueDone()
			continue
		}
		top := len(open) - 1
		if top >= 0 && open[top].seen != nil && !open[top].inKey {
			key, _ := tok.(string)
			if open[top].seen[key] {
				return strings.Join(append(path, key), "."), true
			}
			open[top].seen[key], open[top].inKey = true, true
			path = append(path, key)
			continue
		}
		valueDone()
	}
}

// decodeValue reads one wire value: a JSON number as an Integer when it is one and fits,
// else a finite Real; a boolean as a truth; a string as text. Only a number carries a unit,
// which when written is a string of text.
func decodeValue(raw wiredValue) (runtime.ToolValue, error) {
	if len(raw.Value) == 0 {
		return runtime.ToolValue{}, errors.New("no value")
	}
	unit, err := decodeUnit(raw.Unit)
	if err != nil {
		return runtime.ToolValue{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw.Value))
	dec.UseNumber()
	var decoded any
	if err := dec.Decode(&decoded); err != nil {
		return runtime.ToolValue{}, err
	}
	out := runtime.ToolValue{Unit: unit}
	switch v := decoded.(type) {
	case json.Number:
		if i, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			out.Value = semantics.Value{Kind: semantics.ValInt, Int: i}
			return out, nil
		}
		f, err := strconv.ParseFloat(v.String(), 64)
		if err != nil || math.IsInf(f, 0) {
			return runtime.ToolValue{}, fmt.Errorf("%s is not a finite number", v.String())
		}
		out.Value = semantics.Value{Kind: semantics.ValReal, Real: f}
	case bool:
		if out.Unit != "" {
			return runtime.ToolValue{}, fmt.Errorf("a boolean has no unit, got %q", out.Unit)
		}
		out.Value = semantics.Value{Kind: semantics.ValBool, Bool: v}
	case string:
		if out.Unit != "" {
			return runtime.ToolValue{}, fmt.Errorf("a string has no unit, got %q", out.Unit)
		}
		out.Text = v
	default:
		return runtime.ToolValue{}, fmt.Errorf("%s is not a number, boolean or string", strings.TrimSpace(string(raw.Value)))
	}
	return out, nil
}

// decodeUnit reads a wire value's unit: none when omitted, else a string of unit expression
// text; null or another kind of value is malformed.
func decodeUnit(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	if jsonNull(raw) {
		return "", errors.New("unit is null, not a unit expression")
	}
	var unit string
	if err := json.Unmarshal(raw, &unit); err != nil {
		return "", fmt.Errorf("unit %s is not a unit expression", strings.TrimSpace(string(raw)))
	}
	if strings.TrimSpace(unit) == "" {
		return "", errors.New("unit is empty")
	}
	return strings.TrimSpace(unit), nil
}

// ErrWrongTool is the typed error a tool engine refuses with for a call naming another tool.
var ErrWrongTool = errors.New("call names another tool")

// WrongToolError reports a Compute question put to the engine of a tool it does not name.
type WrongToolError struct {
	Engine string
	Tool   string
}

// Error names the engine and the tool the call named.
func (e *WrongToolError) Error() string {
	return fmt.Sprintf("%s does not answer for tool '%s'", e.Engine, e.Tool)
}

// Is matches ErrWrongTool.
func (e *WrongToolError) Is(target error) bool { return target == ErrWrongTool }

// ErrToolVariable is the typed error for a call using tool variables the manifest entry does not accept.
var ErrToolVariable = errors.New("tool does not accept the variable")

// ToolVariableError reports the tool variables a call uses that the tool's entry does not list.
type ToolVariableError struct {
	Tool      string
	Variables []string
}

// Error names the tool and the variables.
func (e *ToolVariableError) Error() string {
	return fmt.Sprintf("tool '%s' does not accept %s; its manifest entry lists the variables it does",
		e.Tool, strings.Join(e.Variables, ", "))
}

// Is matches ErrToolVariable.
func (e *ToolVariableError) Is(target error) bool { return target == ErrToolVariable }
