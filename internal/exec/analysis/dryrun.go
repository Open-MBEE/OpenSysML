package analysis

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// DryRun is what one tool call would be given, composed without starting the process.
type DryRun struct {
	Tool, Version, Manifest, Executable, Protocol string
	// Args are the argv entries after the executable, as the invocation block renders
	// them; nil for an entry without one.
	Args []string
	// Env is the environment in NAME=value pairs, sorted; nil when the entry has no
	// invocation block, in which case the whole environment is inherited.
	Env []string
	// Cwd is the working directory; empty inherits this process's.
	Cwd string
	// Stdin says what the process reads; StdinJSON for an entry without the block.
	Stdin StdinFormat
	// StdinData is what standard input carries.
	StdinData []byte
	// InputFile is the file the block's `inputFile` writes, nil when it has none.
	InputFile *DryRunFile
	// OutputDir marks that the entry uses {outputDir}.
	OutputDir bool
	// Inputs are the values the call sent the tool.
	Inputs []runtime.ToolInput
	// Outputs are the bindings the call expects back, sorted by variable.
	Outputs []runtime.ToolOutput
	// Reply is the entry's `reply` block; nil is the object protocol.
	Reply *Reply
	// ReplySource is where the reply is read, rendered like argv: stdout, or
	// file:<template> with {outputDir} spelled <outputDir>.
	ReplySource string
}

// DryRunFile is the input file a dry run would write: its name, format and contents.
type DryRunFile struct {
	Name   string
	Format InputFormat
	Data   []byte
}

// ToolDryRunError fails the performance a dry run reached a tool call in; surfaces
// read the preview from it.
type ToolDryRunError struct {
	// Preview is what the call would have been given.
	Preview DryRun
	// Action is the ToolExecution-annotated action the call was made for.
	Action string
}

// Error says the tool was previewed, not run.
func (e *ToolDryRunError) Error() string {
	return fmt.Sprintf("dry run of tool '%s': the process was not started", e.Preview.Tool)
}

// Is matches ErrToolDryRun.
func (e *ToolDryRunError) Is(target error) bool { return target == runtime.ErrToolDryRun }

// NotAToolEntryError is the dry runner's refusal of a call whose engine answers
// Compute questions but is no tool manifest entry.
type NotAToolEntryError struct {
	Engine string
}

// Error names the engine.
func (e *NotAToolEntryError) Error() string {
	return fmt.Sprintf("engine %s is not a tool manifest entry; nothing to preview", e.Engine)
}

// DryRunner is a runtime.ToolRunner that previews the first tool call the
// selection's engines reach and fails the performance with ToolDryRunError.
func (r *Registry) DryRunner(selection Selection) runtime.ToolRunner {
	return &dryRunner{registry: r, selection: selection}
}

// dryRunner answers every call with the call's preview instead of the tool's run.
type dryRunner struct {
	registry  *Registry
	selection Selection
}

// RunTool composes the call for the entry registered for it, refusing as the real
// runner does — no such engine, a refusal of the entry, an absent executable — and
// answering ToolDryRunError with the preview otherwise.
func (d *dryRunner) RunTool(call *runtime.ToolCall) (runtime.ToolAnswer, error) {
	fqn := symbols.FQNOf(call.Action)
	q := Question{Kind: Compute, Subject: fqn, Compute: &ComputeAsk{Call: call}}
	candidates, err := d.registry.candidates(Compute, d.selection)
	if err != nil {
		if errors.Is(err, ErrNoEngine) {
			return runtime.ToolAnswer{}, &runtime.ToolNotRegisteredError{Tool: call.ToolName}
		}
		return runtime.ToolAnswer{}, err
	}
	var e toolEngine
	own := false
	for _, c := range candidates {
		if te, ok := c.(toolEngine); ok && te.entry.ToolName == call.ToolName {
			e, own = te, true
			break
		}
	}
	if !own {
		if d.selection.Mode == SelectNamed {
			named := candidates[0]
			if cov := named.Covers(nil, q); cov.Refusal != nil {
				return runtime.ToolAnswer{}, cov.Refusal
			}
			return runtime.ToolAnswer{}, &NotAToolEntryError{Engine: named.Name()}
		}
		return runtime.ToolAnswer{}, &runtime.ToolNotRegisteredError{Tool: call.ToolName}
	}
	if cov := e.Covers(nil, q); cov.Refusal != nil {
		return runtime.ToolAnswer{}, cov.Refusal
	}
	path, err := e.look(e.entry)
	if err != nil {
		return runtime.ToolAnswer{}, &ProcessAbsentError{Engine: e.Name(), Process: e.Describe().Process, Err: err}
	}
	preview, err := e.entry.Preview(call, path)
	if err != nil {
		return runtime.ToolAnswer{}, err
	}
	return runtime.ToolAnswer{}, &ToolDryRunError{Preview: preview, Action: fqn}
}

// Preview composes the call for the entry without making a temporary directory or
// starting anything: {inputFile} and {outputDir} spell their names in angle brackets.
func (e ToolEntry) Preview(call *runtime.ToolCall, executable string) (DryRun, error) {
	request, err := ToolRequestOf(call)
	if err != nil {
		return DryRun{}, err
	}
	outputs := append([]runtime.ToolOutput(nil), call.Outputs...)
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].Variable < outputs[j].Variable })
	preview := DryRun{
		Tool: e.ToolName, Version: e.Version, Manifest: e.File, Executable: executable,
		Protocol: e.Protocol(), Inputs: call.Inputs, Outputs: outputs, Reply: e.Reply,
		ReplySource: "stdout",
	}
	inv := e.Invocation
	if inv == nil {
		preview.Stdin, preview.StdinData = StdinJSON, request
		return preview, nil
	}
	sc := callScope(call)
	if inv.InputFile != nil || inv.compiled.outputDir {
		sc.outputDir = "<outputDir>"
		preview.OutputDir = inv.compiled.outputDir
		if f := inv.InputFile; f != nil {
			sc.inputFile = "<inputFile>"
			data, err := f.inputData(call, e, request)
			if err != nil {
				return DryRun{}, err
			}
			preview.InputFile = &DryRunFile{Name: f.Name, Format: f.Format, Data: data}
		}
	}
	c := &composed{}
	if err := inv.render(c, sc, call, e, request); err != nil {
		return DryRun{}, err
	}
	preview.Args, preview.Env = c.args, c.env
	preview.Cwd = inv.Cwd
	preview.Stdin, preview.StdinData = inv.Stdin.Format, c.stdin
	if r := e.Reply; r != nil && r.compiled != nil && r.compiled.source != nil {
		source, err := r.compiled.source.render(sc)
		if err != nil {
			return DryRun{}, err
		}
		preview.ReplySource = "file:" + source
	}
	return preview, nil
}

// Lines spells the preview as the surfaces print it, one element per line.
func (d DryRun) Lines() []string {
	tool := d.Tool
	if d.Version != "" {
		tool += " " + d.Version
	}
	lines := []string{"tool: " + tool, "protocol: " + d.Protocol}
	if d.Manifest != "" {
		lines = append(lines, "manifest: "+d.Manifest)
	}
	lines = append(lines, "executable: "+d.Executable)
	if len(d.Args) == 0 {
		lines = append(lines, "argv: (none)")
	} else {
		lines = append(lines, "argv:")
		for _, arg := range d.Args {
			lines = append(lines, "  "+strconv.Quote(arg))
		}
	}
	if d.Env == nil {
		lines = append(lines, "env: inherited from this process")
	} else {
		lines = append(lines, "env:")
		for _, pair := range d.Env {
			lines = append(lines, "  "+pair)
		}
	}
	if d.Cwd == "" {
		lines = append(lines, "cwd: inherited")
	} else {
		lines = append(lines, "cwd: "+d.Cwd)
	}
	if d.Stdin == StdinNone {
		lines = append(lines, "stdin: none")
	} else {
		lines = append(lines, "stdin: "+string(d.Stdin))
		lines = appendData(lines, d.StdinData)
	}
	if f := d.InputFile; f != nil {
		lines = append(lines, fmt.Sprintf("input file: <inputFile> (%s, named %s)", f.Format, f.Name))
		lines = appendData(lines, f.Data)
	}
	if d.OutputDir {
		lines = append(lines, "output dir: <outputDir>")
	}
	if len(d.Inputs) == 0 {
		lines = append(lines, "inputs: (none)")
	} else {
		lines = append(lines, "inputs:")
		for _, in := range d.Inputs {
			text, err := valueText(d.Tool, in.Variable, in.Value)
			if err != nil {
				text = "?"
			}
			line := "  " + in.Variable + " = " + text
			if in.Value.Unit != "" {
				line += " [" + in.Value.Unit + "]"
			}
			if in.Parameter != in.Variable {
				line += "  (parameter " + in.Parameter + ")"
			}
			lines = append(lines, line)
		}
	}
	if len(d.Outputs) == 0 {
		lines = append(lines, "outputs: (none)")
	} else {
		lines = append(lines, "outputs:")
		for _, out := range d.Outputs {
			line := "  " + out.Variable
			if out.Parameter != out.Variable {
				line += "  (parameter " + out.Parameter + ")"
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, d.replyLines()...)
	return append(lines, "the process was not started")
}

// appendData appends data, one element per line it holds, indented two spaces.
func appendData(lines []string, data []byte) []string {
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		lines = append(lines, "  "+line)
	}
	return lines
}

// replyLines is the `reply:` line and the per-output selector lines under it.
func (d DryRun) replyLines() []string {
	r := d.Reply
	if r == nil || r.Format == "" || r.Format == ReplyObject {
		line := "reply: object"
		if source := d.ReplySource; source != "" && source != "stdout" {
			line += " from " + source
		}
		return []string{line + " — the protocol's JSON object, one key per output variable"}
	}
	source := d.ReplySource
	if source == "" {
		source = "stdout"
	}
	head := "reply: " + string(r.Format) + " from " + source
	switch r.Format {
	case ReplyCSV:
		if r.Header == nil || *r.Header {
			head += ", header"
		}
		delimiter := r.Delimiter
		if delimiter == "" {
			delimiter = ","
		}
		head += fmt.Sprintf(", delimiter %q", delimiter)
	case ReplyLines:
		if r.Regex != "" {
			head += fmt.Sprintf(", regex %q", r.Regex)
		}
	}
	lines := []string{head}
	for _, out := range d.Outputs {
		o := r.Outputs[out.Variable]
		if o == nil {
			continue
		}
		line := "  " + out.Variable + ": " + replySelector(r, out.Variable, o)
		lines = append(lines, line)
	}
	return lines
}

// replySelector spells how one output is read from the reply, under the format.
func replySelector(r *Reply, variable string, o *ReplyOutput) string {
	var parts []string
	switch r.Format {
	case ReplyJSON:
		if o.Path != nil {
			parts = append(parts, fmt.Sprintf("path %q", *o.Path))
		}
		if o.UnitPath != "" {
			parts = append(parts, fmt.Sprintf("unitPath %q", o.UnitPath))
		}
	case ReplyCSV:
		if o.Column != nil {
			if o.Column.ByIndex {
				parts = append(parts, fmt.Sprintf("column %d", o.Column.Index))
			} else {
				parts = append(parts, fmt.Sprintf("column %q", o.Column.Name))
			}
		}
		row := "last"
		if o.Row != nil {
			row = o.Row.String()
		}
		parts = append(parts, "row "+row)
		if o.UnitColumn != nil {
			if o.UnitColumn.ByIndex {
				parts = append(parts, fmt.Sprintf("unitColumn %d", o.UnitColumn.Index))
			} else {
				parts = append(parts, fmt.Sprintf("unitColumn %q", o.UnitColumn.Name))
			}
		}
	case ReplyLines:
		if r.Regex != "" {
			parts = append(parts, fmt.Sprintf("regex group %q", variable))
		} else {
			key := o.Key
			if key == "" {
				key = variable
			}
			parts = append(parts, fmt.Sprintf("key %q", key))
		}
	case ReplyExitCode:
		parts = append(parts, "exit status")
		success := r.Success
		if len(success) == 0 {
			success = []int{0}
		}
		parts = append(parts, fmt.Sprintf("success %v", success))
	}
	typeOf := o.Type
	if typeOf == "" {
		if r.Format == ReplyExitCode {
			typeOf = TypeBoolean
		} else {
			typeOf = TypeNumber
		}
	}
	parts = append(parts, "type "+string(typeOf))
	if o.Unit != "" {
		parts = append(parts, "unit "+o.Unit)
	}
	return strings.Join(parts, ", ")
}
