package analysis

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// ToolEnvPassthroughEnv lists, comma-separated, the variables of this process a tool with
// an invocation block is started with besides PATH, HOME, TMPDIR and LANG.
const ToolEnvPassthroughEnv = "OPENSYSML_TOOL_ENV_PASSTHROUGH" // #nosec G101 -- a variable name, not a credential

// ToolKeepEnv, set to 1, keeps every invocation's temporary directory instead of removing
// it once the reply is read.
const ToolKeepEnv = "OPENSYSML_TOOL_KEEP"

// baseToolEnv are the variables of this process every tool with an invocation block is
// started with; the rest of the environment is not inherited.
var baseToolEnv = []string{"PATH", "HOME", "TMPDIR", "LANG"}

// Invocation is a tool entry's `invocation` block: how the process is composed from one
// call's values. Absent, the executable starts as before: no arguments, the JSON request.
type Invocation struct {
	// Args are the argv entries after the executable, each one template.
	Args []string `json:"args,omitempty"`
	// Env are variables added to the minimal base environment, each value one template.
	Env map[string]string `json:"env,omitempty"`
	// Cwd is the working directory: absolute, or relative to the manifest directory and
	// confined to it. Empty inherits this process's.
	Cwd string `json:"cwd,omitempty"`
	// Stdin says what the process reads: the JSON request by default.
	Stdin StdinSpec `json:"stdin"`
	// InputFile, when set, is written into the invocation's temporary directory before the
	// process starts; `{inputFile}` names its path.
	InputFile *InputFile `json:"inputFile,omitempty"`

	// compiled holds the parsed templates once checkInvocation has accepted the block.
	compiled *compiledInvocation
}

// compiledInvocation is the block's templates, parsed at manifest load.
type compiledInvocation struct {
	args      []template
	env       map[string]template
	stdin     template
	outputDir bool
}

// StdinFormat is what a tool with an invocation block reads on standard input.
type StdinFormat string

const (
	// StdinJSON is the protocol's request object, as a tool without the block reads.
	StdinJSON StdinFormat = "json"
	// StdinNone writes nothing.
	StdinNone StdinFormat = "none"
	// StdinCSV writes one header row and one row of the inputs sent.
	StdinCSV StdinFormat = "csv"
	// StdinTemplate writes the manifest's template, rendered for the call.
	StdinTemplate StdinFormat = "template"
)

// StdinSpec is the `stdin` field: one of the format names as a string, or
// `{"template": "..."}` for text composed from the call.
type StdinSpec struct {
	Format   StdinFormat
	Template string
}

// UnmarshalJSON reads the format name or the template object.
func (s *StdinSpec) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		switch StdinFormat(strings.TrimSpace(name)) {
		case StdinJSON, StdinNone, StdinCSV:
			*s = StdinSpec{Format: StdinFormat(strings.TrimSpace(name))}
			return nil
		}
		return fmt.Errorf("stdin %q is not one of json, none and csv, or {\"template\": ...}", name)
	}
	var object struct {
		Template *string `json:"template"`
	}
	if err := decodeOne(data, &object); err != nil {
		return fmt.Errorf("stdin is not one of json, none and csv, or {\"template\": ...}: %v", err)
	}
	if object.Template == nil {
		return errors.New("stdin object has no template")
	}
	*s = StdinSpec{Format: StdinTemplate, Template: *object.Template}
	return nil
}

// MarshalJSON writes the spec as it is read.
func (s StdinSpec) MarshalJSON() ([]byte, error) {
	if s.Format == StdinTemplate {
		return json.Marshal(map[string]string{"template": s.Template})
	}
	if s.Format == "" {
		return json.Marshal(StdinJSON)
	}
	return json.Marshal(s.Format)
}

// InputFormat is what an input file holds.
type InputFormat string

const (
	// InputJSON is the protocol's request object.
	InputJSON InputFormat = "json"
	// InputCSV is one header row and one row of the inputs sent.
	InputCSV InputFormat = "csv"
)

// InputFile is the `inputFile` field: the format and the bare file name written into the
// invocation's temporary directory.
type InputFile struct {
	Format InputFormat `json:"format"`
	Name   string      `json:"name"`
}

// Protocol names how the tool is spoken to, as an engine listing reports it: `object` for
// one JSON object each way, `argv+<stdin>` for a process composed from an invocation block,
// and for a reply of another format the format alone or `argv+<stdin>/<format>`.
func (e ToolEntry) Protocol() string {
	format := ""
	if e.Reply != nil && e.Reply.Format != "" && e.Reply.Format != ReplyObject {
		format = string(e.Reply.Format)
	}
	if e.Invocation == nil {
		if format != "" {
			return format
		}
		return "object"
	}
	stdin := e.Invocation.Stdin.Format
	if stdin == "" {
		stdin = StdinJSON
	}
	base := "argv+" + string(stdin)
	if format != "" {
		return base + "/" + format
	}
	return base
}

// reserved placeholders of a template, which no variable may be named like.
const (
	holeToolName  = "toolName"
	holeURI       = "uri"
	holeInputFile = "inputFile"
	holeOutputDir = "outputDir"
)

// hole is one placeholder of a template: a reserved name, or a variable with the field of
// its value wanted (`value`, `unit`, or empty for the value as text).
type hole struct {
	name  string
	field string
}

// segment is one run of a template: literal text, or a hole.
type segment struct {
	text string
	hole *hole
}

// template is the parsed text of an argument, an environment value or a stdin template.
type template []segment

// parseTemplate reads text: `{name}`, `{name.value}` and `{name.unit}` are holes, `{{` and
// `}}` spell one brace, and any other brace is malformed.
func parseTemplate(text string) (template, error) {
	var t template
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			t = append(t, segment{text: literal.String()})
			literal.Reset()
		}
	}
	for i := 0; i < len(text); i++ {
		switch c := text[i]; c {
		case '{':
			if i+1 < len(text) && text[i+1] == '{' {
				literal.WriteByte('{')
				i++
				continue
			}
			end := strings.IndexByte(text[i+1:], '}')
			if end < 0 {
				return nil, fmt.Errorf("%q has an unclosed placeholder at %d", text, i)
			}
			inner := text[i+1 : i+1+end]
			if strings.ContainsAny(inner, "{ \t\r\n") || inner == "" {
				return nil, fmt.Errorf("%q has a malformed placeholder {%s}", text, inner)
			}
			h, err := parseHole(inner)
			if err != nil {
				return nil, fmt.Errorf("%q: %v", text, err)
			}
			flush()
			t = append(t, segment{hole: h})
			i += end + 1
		case '}':
			if i+1 < len(text) && text[i+1] == '}' {
				literal.WriteByte('}')
				i++
				continue
			}
			return nil, fmt.Errorf("%q has a stray } at %d; spell one as }}", text, i)
		default:
			literal.WriteByte(c)
		}
	}
	flush()
	return t, nil
}

// parseHole reads a placeholder's inside: a reserved name alone, or a variable name with
// an optional `.value` or `.unit`.
func parseHole(inner string) (*hole, error) {
	name, field, dotted := strings.Cut(inner, ".")
	switch name {
	case holeToolName, holeURI, holeInputFile, holeOutputDir:
		if dotted {
			return nil, fmt.Errorf("placeholder {%s} takes no field", inner)
		}
		return &hole{name: name}, nil
	}
	if dotted && field != "value" && field != "unit" {
		return nil, fmt.Errorf("placeholder {%s} names a field other than value and unit", inner)
	}
	return &hole{name: name, field: field}, nil
}

// variables are the variable names the template's holes name, in order of first use.
func (t template) variables() []string {
	var names []string
	seen := make(map[string]bool)
	for _, s := range t {
		if s.hole != nil && !reservedHole(s.hole.name) && !seen[s.hole.name] {
			names, seen[s.hole.name] = append(names, s.hole.name), true
		}
	}
	return names
}

// uses reports whether the template has a hole of the reserved name.
func (t template) uses(name string) bool {
	for _, s := range t {
		if s.hole != nil && s.hole.name == name {
			return true
		}
	}
	return false
}

// reservedHole reports whether name is a placeholder of the call rather than a variable.
func reservedHole(name string) bool {
	switch name {
	case holeToolName, holeURI, holeInputFile, holeOutputDir:
		return true
	}
	return false
}

// scope is what one invocation's templates render from.
type scope struct {
	tool      string
	uri       string
	inputFile string
	outputDir string
	inputs    map[string]runtime.ToolValue
}

// render spells the template for the scope. A variable the call sent no value for, or one
// whose value the protocol does not carry, is ToolUnsentInput.
func (t template) render(sc scope) (string, error) {
	var out strings.Builder
	for _, s := range t {
		if s.hole == nil {
			out.WriteString(s.text)
			continue
		}
		text, err := sc.fill(s.hole)
		if err != nil {
			return "", err
		}
		out.WriteString(text)
	}
	return out.String(), nil
}

// fill is the text of one hole.
func (sc scope) fill(h *hole) (string, error) {
	switch h.name {
	case holeToolName:
		return sc.tool, nil
	case holeURI:
		return sc.uri, nil
	case holeInputFile:
		return sc.inputFile, nil
	case holeOutputDir:
		return sc.outputDir, nil
	}
	v, sent := sc.inputs[h.name]
	if !sent {
		return "", &runtime.ToolError{Tool: sc.tool, Kind: runtime.ToolUnsentInput,
			Detail: fmt.Sprintf("the invocation names {%s} but the call sent no value for %s", h.name, h.name)}
	}
	if h.field == "unit" {
		return v.Unit, nil
	}
	return valueText(sc.tool, h.name, v)
}

// valueText is a value as one argument spells it: the JSON number or truth, or a string's text.
func valueText(tool, variable string, v runtime.ToolValue) (string, error) {
	if v.Value.Kind == semantics.ValInvalid {
		return v.Text, nil
	}
	encoded, err := encodeValue(v)
	if err != nil {
		return "", &runtime.ToolError{Tool: tool, Kind: runtime.ToolUnsentInput, Detail: fmt.Sprintf("%s: %v", variable, err)}
	}
	return string(encoded), nil
}

// checkInvocation validates an entry's block and parses its templates: each names declared
// variables only, `{inputFile}` needs an inputFile, and cwd is confined to dir like the
// executable; with no dir (an entry not read from a file) cwd must be absolute.
func checkInvocation(entry *ToolEntry, dir string) error {
	inv := entry.Invocation
	for _, v := range entry.Variables {
		switch {
		case reservedHole(v):
			return fmt.Errorf("variables names %q, which an invocation template reserves for the call", v)
		case strings.ContainsAny(v, ".{} \t\r\n"):
			return fmt.Errorf("variables names %q; with an invocation block a variable name has no period, brace or space", v)
		}
	}
	switch inv.Stdin.Format {
	case "":
		inv.Stdin.Format = StdinJSON
	case StdinJSON, StdinNone, StdinCSV, StdinTemplate:
	default:
		return fmt.Errorf("stdin %q is not one of json, none, csv and template", inv.Stdin.Format)
	}
	for name := range inv.Env {
		switch {
		case strings.TrimSpace(name) == "":
			return errors.New("env has an empty variable name")
		case strings.ContainsAny(name, "=\x00"):
			return fmt.Errorf("env variable %q has = or NUL in its name", name)
		}
	}
	compiled := &compiledInvocation{env: make(map[string]template, len(inv.Env))}
	parse := func(where, text string) (template, error) {
		t, err := parseTemplate(text)
		if err != nil {
			return nil, fmt.Errorf("invocation %s: %v", where, err)
		}
		for _, v := range t.variables() {
			if !entry.Accepts(v) {
				return nil, fmt.Errorf("invocation %s names {%s}, which variables does not list", where, v)
			}
		}
		if t.uses(holeInputFile) && inv.InputFile == nil {
			return nil, fmt.Errorf("invocation %s names {inputFile} but the block has no inputFile", where)
		}
		compiled.outputDir = compiled.outputDir || t.uses(holeOutputDir)
		return t, nil
	}
	for i, arg := range inv.Args {
		t, err := parse(fmt.Sprintf("args[%d]", i), arg)
		if err != nil {
			return err
		}
		compiled.args = append(compiled.args, t)
	}
	envNames := make([]string, 0, len(inv.Env))
	for name := range inv.Env {
		envNames = append(envNames, name)
	}
	sort.Strings(envNames)
	for _, name := range envNames {
		t, err := parse("env."+name, inv.Env[name])
		if err != nil {
			return err
		}
		compiled.env[name] = t
	}
	if inv.Stdin.Format == StdinTemplate {
		t, err := parse("stdin.template", inv.Stdin.Template)
		if err != nil {
			return err
		}
		compiled.stdin = t
	}
	if f := inv.InputFile; f != nil {
		f.Name = strings.TrimSpace(f.Name)
		switch {
		case f.Format != InputJSON && f.Format != InputCSV:
			return fmt.Errorf("inputFile format %q is not one of json and csv", f.Format)
		case f.Name == "" || f.Name == "." || f.Name == "..":
			return errors.New("inputFile name is empty")
		case f.Name != filepath.Base(f.Name) || strings.ContainsAny(f.Name, `/\`+"\x00"):
			return fmt.Errorf("inputFile name %q is not a bare file name", f.Name)
		}
	}
	inv.Cwd = strings.TrimSpace(inv.Cwd)
	if inv.Cwd != "" {
		if dir == "" && !filepath.IsAbs(inv.Cwd) {
			return fmt.Errorf("invocation cwd %q is relative, but the entry has no manifest directory to confine it to", inv.Cwd)
		}
		resolved, err := confinedPath(dir, inv.Cwd)
		if err != nil {
			return fmt.Errorf("invocation cwd %v", err)
		}
		inv.Cwd = resolved
	}
	inv.compiled = compiled
	return nil
}

// composed is one invocation's process as the block composes it, and the temporary
// directory made for it, if any.
type composed struct {
	args    []string
	env     []string
	dir     string
	stdin   []byte
	tempDir string
}

// compose renders the accepted block for one call: argv, environment, working directory and
// standard input, with the input file and output directory in a fresh temporary directory.
func (inv *Invocation) compose(call *runtime.ToolCall, entry ToolEntry, request []byte) (*composed, error) {
	sc := scope{tool: call.ToolName, uri: call.URI, inputs: make(map[string]runtime.ToolValue, len(call.Inputs))}
	for _, in := range call.Inputs {
		sc.inputs[in.Variable] = in.Value
	}
	c := &composed{dir: inv.Cwd}
	if inv.InputFile != nil || inv.compiled.outputDir {
		dir, err := os.MkdirTemp("", "opensysml-tool-")
		if err != nil {
			return nil, &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolProcessFailed,
				Detail: "cannot make the invocation's temporary directory: " + err.Error()}
		}
		c.tempDir, sc.outputDir = dir, dir
		if f := inv.InputFile; f != nil {
			sc.inputFile = filepath.Join(dir, f.Name)
			data := request
			if f.Format == InputCSV {
				if data, err = csvBytes(call, entry); err != nil {
					c.remove()
					return nil, err
				}
			}
			if err := os.WriteFile(sc.inputFile, data, 0o600); err != nil {
				c.remove()
				return nil, &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolProcessFailed,
					Detail: "cannot write the input file: " + err.Error()}
			}
		}
	}
	var err error
	if c.args, err = inv.renderArgs(sc); err != nil {
		c.remove()
		return nil, err
	}
	if c.env, err = inv.renderEnv(sc); err != nil {
		c.remove()
		return nil, err
	}
	if c.stdin, err = inv.renderStdin(sc, call, entry, request); err != nil {
		c.remove()
		return nil, err
	}
	return c, nil
}

// remove deletes the temporary directory unless ToolKeepEnv keeps it.
func (c *composed) remove() {
	if c.tempDir == "" || keepToolDirs() {
		return
	}
	_ = os.RemoveAll(c.tempDir)
}

// keepToolDirs reads ToolKeepEnv: 1, true, yes or on keeps every invocation's directory.
func keepToolDirs() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(ToolKeepEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// renderArgs is argv after the executable: each template one entry, never split further.
func (inv *Invocation) renderArgs(sc scope) ([]string, error) {
	args := make([]string, 0, len(inv.compiled.args))
	for _, t := range inv.compiled.args {
		arg, err := t.render(sc)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
	}
	return args, nil
}

// renderEnv is the process environment: the base variables and those ToolEnvPassthroughEnv
// lists, as this process holds them, then the block's, which override; in name order.
func (inv *Invocation) renderEnv(sc scope) ([]string, error) {
	env := make(map[string]string)
	names := append([]string(nil), baseToolEnv...)
	for _, name := range strings.Split(os.Getenv(ToolEnvPassthroughEnv), ",") {
		if name = strings.TrimSpace(name); name != "" {
			if strings.ContainsAny(name, "=\x00") {
				return nil, &runtime.ToolError{Tool: sc.tool, Kind: runtime.ToolProcessFailed,
					Detail: fmt.Sprintf("%s lists %q, which is not an environment variable name", ToolEnvPassthroughEnv, name)}
			}
			names = append(names, name)
		}
	}
	for _, name := range names {
		if value, set := os.LookupEnv(name); set {
			env[name] = value
		}
	}
	for name, t := range inv.compiled.env {
		value, err := t.render(sc)
		if err != nil {
			return nil, err
		}
		env[name] = value
	}
	keys := make([]string, 0, len(env))
	for name := range env {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, name := range keys {
		pairs = append(pairs, name+"="+env[name])
	}
	return pairs, nil
}

// renderStdin is what the process reads: the request, nothing, the CSV row, or the template.
func (inv *Invocation) renderStdin(sc scope, call *runtime.ToolCall, entry ToolEntry, request []byte) ([]byte, error) {
	switch inv.Stdin.Format {
	case StdinNone:
		return nil, nil
	case StdinCSV:
		return csvBytes(call, entry)
	case StdinTemplate:
		text, err := inv.compiled.stdin.render(sc)
		if err != nil {
			return nil, err
		}
		return []byte(text), nil
	}
	return request, nil
}

// csvBytes is one header row and one row of the inputs the call sent, in the order the
// entry lists its variables: a `<name>` column each, and `<name>.unit` after one measured.
func csvBytes(call *runtime.ToolCall, entry ToolEntry) ([]byte, error) {
	inputs := make(map[string]runtime.ToolValue, len(call.Inputs))
	for _, in := range call.Inputs {
		inputs[in.Variable] = in.Value
	}
	var header, row []string
	for _, name := range entry.Variables {
		v, sent := inputs[name]
		if !sent {
			continue
		}
		text, err := valueText(call.ToolName, name, v)
		if err != nil {
			return nil, err
		}
		header, row = append(header, name), append(row, text)
		if v.Unit != "" {
			header, row = append(header, name+".unit"), append(row, v.Unit)
		}
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.WriteAll([][]string{header, row}); err != nil {
		return nil, &runtime.ToolError{Tool: call.ToolName, Kind: runtime.ToolProcessFailed, Detail: "cannot write the CSV input: " + err.Error()}
	}
	return buf.Bytes(), nil
}
