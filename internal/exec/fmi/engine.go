package fmi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/hostcap"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// RunnerEnv names the environment variable holding the FMI runner: an executable
// name looked up on PATH or a path, exactly as OPENSYSML_SMT names the solver.
// Setting it is the operator's grant of execution: the runner instantiates an
// FMU's native code, which nothing in the build checks. Unset, the engine
// registers and refuses every question through Covers.
const RunnerEnv = "OPENSYSML_FMI_RUNNER"

// Exchange names the request/reply protocol the engine and the runner speak, as
// an engine listing reports it.
const Exchange = "fmi/1"

// runnerDescription names the process the engine needs, as Describe reports it.
const runnerDescription = "the FMI runner OPENSYSML_FMI_RUNNER names"

// engine answers Compute questions for ToolExecution-annotated actions and
// calcs naming the tool `fmi`, by running the runner once per invocation with
// the request and reply below over its standard input and output.
type engine struct {
	look    func() (string, error)
	timeout func() time.Duration
	limit   func() int

	mu sync.Mutex
	// descriptions are the FMU descriptions the engine read, by path, each
	// re-read only when the file's size or modification time moved.
	descriptions map[string]cached
}

// maxCachedDescriptions bounds the description cache: inserting a 65th clears
// the map rather than let it grow for every FMU a workspace ever touched.
const maxCachedDescriptions = 64

// cached is one description held against the file it was read from.
type cached struct {
	desc *Description
	err  error
	size int64
	mod  time.Time
}

// New returns the `tool:fmi` engine. look finds the runner's executable; nil
// reads RunnerEnv, whose absence is the absence the engine reports.
func New(look func() (string, error)) analysis.Manifested {
	if look == nil {
		look = lookRunner
	}
	return &engine{look: look, timeout: analysis.ToolTimeoutFromEnv, limit: analysis.OutputLimitFromEnv,
		descriptions: make(map[string]cached)}
}

// Name is `tool:fmi`.
func (e *engine) Name() string { return analysis.ToolEngineName("fmi") }

// Describe: one process per invocation, whose answer nothing in the build can check.
func (e *engine) Describe() analysis.Description {
	return analysis.Description{
		Questions: []analysis.Kind{analysis.Compute},
		Process:   runnerDescription,
		Bounds:    []string{"tool"},
		Authority: analysis.Observed,
	}
}

// Origin reports the runner as a tool-kind engine speaking the fmi/1 exchange.
func (e *engine) Origin() analysis.Origin {
	path, _ := e.look()
	return analysis.Origin{Kind: analysis.KindTool, Command: path, Exchange: Exchange}
}

// Process names the runner found, or reports its absence.
func (e *engine) Process() (string, error) {
	path, err := e.look()
	if err != nil {
		return "", &analysis.ProcessAbsentError{Engine: e.Name(), Process: runnerDescription, Err: err}
	}
	return "fmi runner at " + path, nil
}

// lookRunner finds the runner RunnerEnv names: a path as given, a bare name on
// PATH. Unset is the refusal every Covers carries.
func lookRunner() (string, error) {
	runner := strings.TrimSpace(os.Getenv(RunnerEnv))
	if runner == "" {
		return "", fmt.Errorf("%s is not set: the fmi engine runs an FMU's native code through the runner that variable names", RunnerEnv)
	}
	if err := hostcap.CheckSpawn(runner); err != nil {
		return "", err
	}
	path, err := exec.LookPath(runner)
	if err != nil {
		return "", fmt.Errorf("%s names %q, which is not an executable on PATH or a path: %w", RunnerEnv, runner, err)
	}
	return path, nil
}

// ErrVariable is the typed error for a ToolVariable an FMU cannot answer for.
var ErrVariable = errors.New("fmi variable refused")

// VariableError reports a ToolVariable naming no variable of the FMU, or naming
// one of the wrong direction. Detail lists the candidates it could have been.
type VariableError struct {
	FMU      string
	Variable string
	Detail   string
}

// Error names the FMU, the variable and the detail.
func (e *VariableError) Error() string {
	return fmt.Sprintf("fmi: %s: %s (%s)", filepath.Base(e.FMU), e.Detail, e.Variable)
}

// Is matches ErrVariable.
func (e *VariableError) Is(target error) bool { return target == ErrVariable }

// reservedInputs are the variables a call may name beside the FMU's own inputs.
var reservedInputs = map[string]bool{
	"fmi:startTime": true,
	"fmi:stopTime":  true,
	"fmi:stepSize":  true,
	"fmi:tolerance": true,
}

// reservedOutputs are the variables a call may name beside the FMU's own outputs.
var reservedOutputs = map[string]bool{
	"fmi:time": true,
}

// Covers takes a Compute question whose call names the fmi tool, the runner is
// present and the call's variables resolve to the FMU's in the right direction.
func (e *engine) Covers(_ *analysis.Model, q analysis.Question) analysis.Coverage {
	if q.Kind != analysis.Compute {
		return analysis.Coverage{Refusal: &analysis.NotAskedError{Engine: e.Name(), Kind: q.Kind}}
	}
	if q.Compute == nil || q.Compute.Call == nil {
		return analysis.Coverage{Refusal: &analysis.MalformedQuestionError{Kind: q.Kind, Missing: "a Compute with a Call"}}
	}
	call := q.Compute.Call
	if call.ToolName != "fmi" {
		return analysis.Coverage{Refusal: &analysis.WrongToolError{Engine: e.Name(), Tool: call.ToolName}}
	}
	if _, err := e.Process(); err != nil {
		return analysis.Coverage{Refusal: err}
	}
	d, _, err := e.resolve(call)
	if err != nil {
		return analysis.Coverage{Refusal: err}
	}
	if err := checkVariables(call, d); err != nil {
		return analysis.Coverage{Refusal: err}
	}
	return analysis.Coverage{Covered: true}
}

// resolve finds the FMU the call's uri names and reads its description.
func (e *engine) resolve(call *runtime.ToolCall) (*Description, string, error) {
	path, err := resolveURI(call)
	if err != nil {
		return nil, "", err
	}
	d, err := e.describe(path)
	if err != nil {
		return nil, "", err
	}
	return d, path, nil
}

// resolveURI reads the call's uri: `file://<abs>`, `file:<path>` or a bare
// path. A relative path resolves against the directory of the document
// declaring the action or calc; a declaration with no file refuses.
func resolveURI(call *runtime.ToolCall) (string, error) {
	uri := call.URI
	if u, err := url.Parse(uri); err == nil && u.Scheme == "file" && u.Host == "" && strings.HasPrefix(uri, "file://") {
		uri = u.Path
	} else if rest, ok := strings.CutPrefix(uri, "file:"); ok {
		uri = rest
	} else if u != nil && u.Scheme != "" {
		return "", fmt.Errorf("fmi: the FMU uri %q names scheme %s; a file or a path is needed", call.URI, u.Scheme)
	}
	if !filepath.IsAbs(uri) {
		doc := call.Action.DocName
		if !source.IsFile(doc) {
			return "", fmt.Errorf("fmi: the FMU uri is relative and the declaration has no file")
		}
		uri = filepath.Join(source.Dir(doc), uri)
	}
	return filepath.Clean(uri), nil
}

// describe is the description of the FMU at path, re-read only when the file
// moved.
func (e *engine) describe(path string) (*Description, error) {
	info, statErr := os.Stat(path)
	if statErr != nil {
		return nil, statErr
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if c, ok := e.descriptions[path]; ok && c.size == info.Size() && c.mod.Equal(info.ModTime()) {
		return c.desc, c.err
	}
	d, err := Read(path)
	if len(e.descriptions) >= maxCachedDescriptions {
		clear(e.descriptions)
	}
	e.descriptions[path] = cached{desc: d, err: err, size: info.Size(), mod: info.ModTime()}
	return d, err
}

// checkVariables refuses a call naming a variable the FMU does not hold or one
// of the wrong direction.
func checkVariables(call *runtime.ToolCall, d *Description) error {
	fmu := call.URI
	for _, in := range call.Inputs {
		if reservedInputs[in.Variable] {
			continue
		}
		v, ok := d.Variable(in.Variable)
		if !ok {
			return &VariableError{FMU: fmu, Variable: in.Variable,
				Detail: fmt.Sprintf("has no variable %q; its inputs are %s", in.Variable, candidates(d, true))}
		}
		switch v.Causality {
		case CausalityInput, CausalityParameter, CausalityStructuralParameter:
		default:
			return &VariableError{FMU: fmu, Variable: in.Variable,
				Detail: fmt.Sprintf("variable %q is causality %s, not an input", in.Variable, v.Causality)}
		}
	}
	for _, out := range call.Outputs {
		if reservedOutputs[out.Variable] {
			continue
		}
		v, ok := d.Variable(out.Variable)
		if !ok {
			return &VariableError{FMU: fmu, Variable: out.Variable,
				Detail: fmt.Sprintf("has no variable %q; its outputs are %s", out.Variable, candidates(d, false))}
		}
		switch v.Causality {
		case CausalityOutput, CausalityCalculatedParameter, CausalityLocal:
		default:
			return &VariableError{FMU: fmu, Variable: out.Variable,
				Detail: fmt.Sprintf("variable %q is causality %s, not an output", out.Variable, v.Causality)}
		}
	}
	return nil
}

// candidates lists up to ten variable names of the direction asked.
func candidates(d *Description, inputs bool) string {
	var vars []Variable
	if inputs {
		vars = d.Inputs()
	} else {
		vars = d.Outputs()
	}
	names := make([]string, 0, len(vars))
	for i, v := range vars {
		if i == 10 {
			names = append(names, "…")
			break
		}
		names = append(names, v.Name)
	}
	if len(names) == 0 {
		return "none"
	}
	sort.Strings(names[:min(len(names), 10)])
	return strings.Join(names, ", ")
}

// Run invokes the runner once: the request as one JSON object on its standard
// input, the reply's outputs bound to the call's. Every failure of the process
// or the protocol is a runtime.ToolError or a typed refusal of the package's own.
func (e *engine) Run(ctx context.Context, _ *analysis.Model, q analysis.Question, _ analysis.Budget) (analysis.Result, error) {
	if err := ctx.Err(); err != nil {
		return analysis.Result{}, err
	}
	if q.Compute == nil || q.Compute.Call == nil {
		return analysis.Result{}, &analysis.MalformedQuestionError{Kind: q.Kind, Missing: "a Compute with a Call"}
	}
	call := q.Compute.Call
	path, err := e.look()
	if err != nil {
		return analysis.Result{}, &analysis.ProcessAbsentError{Engine: e.Name(), Process: runnerDescription, Err: err}
	}
	d, fmu, err := e.resolve(call)
	if err != nil {
		return analysis.Result{}, err
	}
	if err := checkVariables(call, d); err != nil {
		return analysis.Result{}, err
	}
	request, err := requestOf(call, d, fmu)
	if err != nil {
		return analysis.Result{}, err
	}
	use := &analysis.ToolUse{Tool: "fmi", Executable: path, Stdin: request}
	failed := func(err error) (analysis.Result, error) {
		use.Failed = err.Error()
		q.Compute.Ran(*use)
		return analysis.Result{}, err
	}
	timeout := e.timeout()
	started := time.Now()
	ex, err := e.invoke(ctx, path, request, timeout)
	if err != nil {
		return failed(err)
	}
	reply, err := replyOf(call, d, ex.stdout, ex.stderr)
	if err != nil {
		return failed(err)
	}
	bound, err := call.Bind(reply)
	if err != nil {
		return failed(err)
	}
	values := make([]analysis.Evaluation, 0, len(bound))
	for name, value := range bound {
		values = append(values, analysis.Evaluation{Name: name, Value: value})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	q.Compute.Ran(*use)
	return analysis.Result{
		Question: q,
		Engine:   e.Name(),
		Claim:    analysis.ClaimValue,
		Strength: analysis.Observed,
		Values:   values,
		Reply:    analysis.RenderReply(reply),
		Bounds:   analysis.Bounds{{Name: "tool", Limit: timeout.Milliseconds()}},
		Elapsed:  time.Since(started),
	}, nil
}

// execution is what one run of the runner produced.
type execution struct {
	stdout, stderr []byte
}

// invoke runs the runner once under the timeout, its standard streams bounded.
func (e *engine) invoke(ctx context.Context, runner string, request []byte, timeout time.Duration) (*execution, error) {
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(tctx, runner) // #nosec G204 -- the runner is the operator's grant
	env, err := analysis.ToolEnvNames()
	if err != nil {
		return nil, &runtime.ToolError{Tool: "fmi", Kind: runtime.ToolProcessFailed, Detail: err.Error()}
	}
	list := make([]string, 0, len(env))
	for _, name := range env {
		if value, set := os.LookupEnv(name); set {
			list = append(list, name+"="+value)
		}
	}
	cmd.Env = list
	cmd.Stdin = strings.NewReader(string(request))
	stdout, stderr := analysis.NewBoundedBuffer(e.limit(), cancel), analysis.NewBoundedBuffer(e.limit(), cancel)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = time.Second
	runErr := cmd.Run()
	switch {
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case stdout.Over() || stderr.Over():
		stream := "output"
		if stderr.Over() {
			stream = "error"
		}
		return nil, &runtime.ToolError{Tool: "fmi", Kind: runtime.ToolMalformed,
			Detail: fmt.Sprintf("%s wrote more than %d bytes to standard %s (%s)", runner, e.limit(), stream, analysis.OutputLimitEnv)}
	case errors.Is(tctx.Err(), context.DeadlineExceeded):
		return nil, &runtime.ToolError{Tool: "fmi", Kind: runtime.ToolTimeout,
			Detail: fmt.Sprintf("%s did not answer within %s (%s)", runner, timeout, analysis.ToolTimeoutEnv)}
	case runErr != nil:
		return nil, &runtime.ToolError{Tool: "fmi", Kind: runtime.ToolProcessFailed, Detail: analysis.ProcessDetail(runner, runErr, stderr.Bytes())}
	}
	return &execution{stdout: stdout.Bytes(), stderr: stderr.Bytes()}, nil
}

// RunnerError is a reply carrying the runner's own error member.
type RunnerError struct {
	Message string
	Stderr  string
}

// Error names the runner's message, then what it wrote to standard error.
func (e *RunnerError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("fmi: the runner failed: %s: %s", e.Message, e.Stderr)
	}
	return fmt.Sprintf("fmi: the runner failed: %s", e.Message)
}
