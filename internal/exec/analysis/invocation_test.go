package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// entryText is a tool entry with the invocation block given, as a manifest file spells it.
func entryText(invocation string) string {
	return `{"toolName": "Solver", "executable": "solve", "variables": ["mass", "power", "label", "a"], "invocation": ` + invocation + `}`
}

func TestManifestInvocationAcceptedShapes(t *testing.T) {
	cases := map[string]struct {
		text string
		want Invocation
	}{
		"empty block": {`{}`, Invocation{Stdin: StdinSpec{Format: StdinJSON}}},
		"args and env": {`{"args": ["--mass", "{mass}", "{power.value}", "{power.unit}", "{{literal}}", "{toolName}", "{uri}"], "env": {"OMP_NUM_THREADS": "4", "RUN": "{label}"}}`,
			Invocation{Args: []string{"--mass", "{mass}", "{power.value}", "{power.unit}", "{{literal}}", "{toolName}", "{uri}"},
				Env: map[string]string{"OMP_NUM_THREADS": "4", "RUN": "{label}"}, Stdin: StdinSpec{Format: StdinJSON}}},
		"stdin none":     {`{"stdin": "none"}`, Invocation{Stdin: StdinSpec{Format: StdinNone}}},
		"stdin csv":      {`{"stdin": "csv"}`, Invocation{Stdin: StdinSpec{Format: StdinCSV}}},
		"stdin json":     {`{"stdin": "json"}`, Invocation{Stdin: StdinSpec{Format: StdinJSON}}},
		"stdin template": {`{"stdin": {"template": "mass={mass}\n"}}`, Invocation{Stdin: StdinSpec{Format: StdinTemplate, Template: "mass={mass}\n"}}},
		"input file": {`{"args": ["{inputFile}", "{outputDir}/out.csv"], "inputFile": {"format": "csv", "name": "inputs.csv"}}`,
			Invocation{Args: []string{"{inputFile}", "{outputDir}/out.csv"}, Stdin: StdinSpec{Format: StdinJSON}, InputFile: &InputFile{Format: InputCSV, Name: "inputs.csv"}}},
		"json input file": {`{"inputFile": {"format": "json", "name": "in.json"}}`,
			Invocation{Stdin: StdinSpec{Format: StdinJSON}, InputFile: &InputFile{Format: InputJSON, Name: "in.json"}}},
		"output dir alone": {`{"env": {"OUT": "{outputDir}"}}`, Invocation{Env: map[string]string{"OUT": "{outputDir}"}, Stdin: StdinSpec{Format: StdinJSON}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeManifest(t, map[string]string{"solver.json": entryText(tc.text)})
			entries, err := LoadManifest(dir)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if len(entries) != 1 || entries[0].Invocation == nil {
				t.Fatalf("entries %+v, want one with its invocation", entries)
			}
			got := *entries[0].Invocation
			if got.compiled == nil {
				t.Fatal("invocation loaded with its templates unparsed")
			}
			got.compiled = nil
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("invocation %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestManifestInvocationCwd(t *testing.T) {
	dir := writeManifest(t, map[string]string{
		"relative.json": `{"toolName": "Rel", "executable": "solve", "variables": [], "invocation": {"cwd": "work"}}`,
		"absolute.json": `{"toolName": "Abs", "executable": "solve", "variables": [], "invocation": {"cwd": "/opt/solver"}}`,
		"work/.keep":    "",
	})
	entries, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(dir)
	if got := entries[1].Invocation.Cwd; got != filepath.Join(resolved, "work") {
		t.Errorf("relative cwd %s, want it resolved under the manifest directory", got)
	}
	if got := entries[0].Invocation.Cwd; got != filepath.Clean("/opt/solver") {
		t.Errorf("absolute cwd %s, want it kept", got)
	}
}

func TestManifestInvocationRefusedShapes(t *testing.T) {
	cases := map[string]struct {
		text   string
		detail string
	}{
		"reply no outputs":    {`{"toolName": "A", "executable": "a", "variables": [], "reply": {"format": "csv"}}`, `reply.outputs is required under format csv`},
		"unknown key":         {entryText(`{"shell": true}`), `unknown field "shell"`},
		"unknown nested key":  {entryText(`{"inputFile": {"format": "csv", "name": "x", "mode": "0600"}}`), `unknown field "mode"`},
		"stdin unknown name":  {entryText(`{"stdin": "yaml"}`), `stdin "yaml" is not one of json, none and csv`},
		"stdin template key":  {entryText(`{"stdin": {"text": "x"}}`), `unknown field "text"`},
		"stdin no template":   {entryText(`{"stdin": {}}`), `stdin object has no template`},
		"stdin number":        {entryText(`{"stdin": 1}`), `stdin is not one of json, none and csv`},
		"args not strings":    {entryText(`{"args": [1]}`), `cannot unmarshal number`},
		"undeclared variable": {entryText(`{"args": ["{speed}"]}`), `args[0] names {speed}, which variables does not list`},
		"undeclared in env":   {entryText(`{"env": {"X": "{speed.unit}"}}`), `env.X names {speed}`},
		"undeclared in stdin": {entryText(`{"stdin": {"template": "{speed}"}}`), `stdin.template names {speed}`},
		"unclosed brace":      {entryText(`{"args": ["{mass"]}`), `unclosed placeholder`},
		"stray close brace":   {entryText(`{"args": ["mass}"]}`), `stray }`},
		"empty placeholder":   {entryText(`{"args": ["{}"]}`), `malformed placeholder {}`},
		"nested brace":        {entryText(`{"args": ["{{mass}"]}`), `stray }`},
		"spaced placeholder":  {entryText(`{"args": ["{mass }"]}`), `malformed placeholder`},
		"unknown field":       {entryText(`{"args": ["{mass.text}"]}`), `names a field other than value and unit`},
		"reserved with field": {entryText(`{"args": ["{uri.value}"]}`), `{uri.value} takes no field`},
		"inputFile unset":     {entryText(`{"args": ["{inputFile}"]}`), `names {inputFile} but the block has no inputFile`},
		"inputFile format":    {entryText(`{"inputFile": {"format": "xml", "name": "in.xml"}}`), `inputFile format "xml" is not one of json and csv`},
		"inputFile no name":   {entryText(`{"inputFile": {"format": "json", "name": " "}}`), `inputFile name is empty`},
		"inputFile dotdot":    {entryText(`{"inputFile": {"format": "json", "name": ".."}}`), `inputFile name is empty`},
		"inputFile path":      {entryText(`{"inputFile": {"format": "json", "name": "../in.json"}}`), `is not a bare file name`},
		"env empty name":      {entryText(`{"env": {"": "x"}}`), `env has an empty variable name`},
		"env equals in name":  {entryText(`{"env": {"A=B": "x"}}`), `has = or NUL in its name`},
		"cwd escapes":         {entryText(`{"cwd": "../elsewhere"}`), `invocation cwd "../elsewhere" names a path outside the manifest directory`},
		"reserved variable": {`{"toolName": "A", "executable": "a", "variables": ["uri"], "invocation": {}}`,
			`variables names "uri", which an invocation template reserves`},
		"dotted variable": {`{"toolName": "A", "executable": "a", "variables": ["mass", "mass.unit"], "invocation": {}}`,
			`variables names "mass.unit"; with an invocation block a variable name has no period, brace or space`},
		"braced variable": {`{"toolName": "A", "executable": "a", "variables": ["{x}"], "invocation": {}}`,
			`variables names "{x}"; with an invocation block`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeManifest(t, map[string]string{"entry.json": tc.text})
			_, err := LoadManifest(dir)
			var fault *ManifestError
			if !errors.As(err, &fault) || !errors.Is(err, ErrManifest) {
				t.Fatalf("load: %v, want a ManifestError", err)
			}
			if !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("error %q, want it to say %q", err, tc.detail)
			}
		})
	}
}

// An entry built in code, not read from a manifest, has its block checked by NewTool: a sound
// one composes the command, a faulty one refuses every question with the manifest fault.
func TestNewToolChecksAProgrammaticInvocation(t *testing.T) {
	call := &runtime.ToolCall{ToolName: "Echo", Inputs: []runtime.ToolInput{{Variable: "mass"}}}
	question := Question{Kind: Compute, Compute: &ComputeAsk{Call: call}}

	entry := echoEntry(echo(t), &Invocation{Args: []string{"{mass}", "{outputDir}"}})
	if got := entry.Protocol(); got != "argv+json" {
		t.Errorf("protocol of an unchecked block %q, want argv+json", got)
	}
	sound := NewTool(entry)
	if c := sound.Covers(nil, question); !c.Covered {
		t.Fatalf("sound block: %+v, want covered", c)
	}
	out := parseProbe(t).perform(t, registered(t, NewRun(), sound))
	if !strings.HasPrefix(out["argv"], `["1500",`) {
		t.Errorf("argv %s, want the block's arguments rendered", out["argv"])
	}

	for name, inv := range map[string]*Invocation{
		"undeclared variable": {Args: []string{"{speed}"}},
		"relative cwd":        {Cwd: "work"},
		"unknown stdin":       {Stdin: StdinSpec{Format: "yaml"}},
	} {
		faulty := NewTool(echoEntry(echo(t), inv))
		c := faulty.Covers(nil, question)
		if c.Covered || !errors.Is(c.Refusal, ErrManifest) || !strings.Contains(c.Refusal.Error(), "tool:Echo") {
			t.Errorf("%s: %+v, want the refusal a ManifestError naming tool:Echo", name, c)
		}
		if _, err := faulty.Process(); !errors.Is(err, ErrManifest) {
			t.Errorf("%s: process %v, want the ManifestError, so listings show the tool unavailable", name, err)
		}
		if _, err := faulty.Run(context.Background(), nil, question, Budget{}); !errors.Is(err, ErrManifest) {
			t.Errorf("%s: run %v, want the ManifestError", name, err)
		}
	}
}

// A variable named like a reserved placeholder is refused only beside an invocation block:
// an entry without one keeps today's meaning.
func TestManifestWithoutInvocationKeepsReservedVariableNames(t *testing.T) {
	dir := writeManifest(t, map[string]string{"entry.json": `{"toolName": "A", "executable": "a", "variables": ["uri", "toolName"]}`})
	entries, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if entries[0].Invocation != nil || entries[0].Protocol() != "object" {
		t.Errorf("entry %+v, want no invocation and the object protocol", entries[0])
	}
}

func TestToolEntryProtocol(t *testing.T) {
	cases := map[string]string{
		`{}`:                                    "argv+json",
		`{"stdin": "none"}`:                     "argv+none",
		`{"stdin": "csv"}`:                      "argv+csv",
		`{"stdin": {"template": "{toolName}"}}`: "argv+template",
	}
	for text, want := range cases {
		dir := writeManifest(t, map[string]string{"solver.json": entryText(text)})
		entries, err := LoadManifest(dir)
		if err != nil {
			t.Fatalf("load %s: %v", text, err)
		}
		if got := entries[0].Protocol(); got != want {
			t.Errorf("%s: protocol %s, want %s", text, got, want)
		}
		if got := NewTool(entries[0]).(Manifested).Origin().ProtocolText(); got != want {
			t.Errorf("%s: listing protocol %s, want %s", text, got, want)
		}
	}
	if got := NewTool(ToolEntry{ToolName: "A", Executable: "a"}).(Manifested).Origin().ProtocolText(); got != "object" {
		t.Errorf("bare entry lists protocol %s, want object", got)
	}
}

func TestTemplateParsesPlaceholdersAndEscapes(t *testing.T) {
	cases := map[string]template{
		"":                        {},
		"plain":                   {{text: "plain"}},
		"{mass}":                  {{hole: &hole{name: "mass"}}},
		"--m={mass.value}":        {{text: "--m="}, {hole: &hole{name: "mass", field: "value"}}},
		"{mass.unit}":             {{hole: &hole{name: "mass", field: "unit"}}},
		"{{x}}":                   {{text: "{x}"}},
		"{{{mass}}}":              {{text: "{"}, {hole: &hole{name: "mass"}}, {text: "}"}},
		"{toolName}/{uri}":        {{hole: &hole{name: "toolName"}}, {text: "/"}, {hole: &hole{name: "uri"}}},
		"{inputFile} {outputDir}": {{hole: &hole{name: "inputFile"}}, {text: " "}, {hole: &hole{name: "outputDir"}}},
	}
	for text, want := range cases {
		got, err := parseTemplate(text)
		if err != nil {
			t.Errorf("%q: %v", text, err)
			continue
		}
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q parsed as %s, want %s", text, spellTemplate(got), spellTemplate(want))
		}
	}
}

// checked is inv accepted for an entry declaring variables, its templates parsed.
func checked(t *testing.T, inv *Invocation, variables ...string) *Invocation {
	t.Helper()
	entry := ToolEntry{ToolName: "Solver", Executable: "solve", Variables: variables, Invocation: inv}
	if err := checkInvocation(&entry, t.TempDir()); err != nil {
		t.Fatalf("check %+v: %v", inv, err)
	}
	return inv
}

// spellTemplate spells a parsed template for a failure message.
func spellTemplate(t template) string {
	parts := make([]string, 0, len(t))
	for _, s := range t {
		if s.hole != nil {
			parts = append(parts, fmt.Sprintf("hole(%s.%s)", s.hole.name, s.hole.field))
		} else {
			parts = append(parts, fmt.Sprintf("text(%q)", s.text))
		}
	}
	return strings.Join(parts, " ")
}

// A rendered value is one argv entry whatever it holds: a shell's metacharacters, spaces
// and braces pass through as text, and nothing is split or interpreted.
func TestTemplateRendersValuesAsText(t *testing.T) {
	sc := scope{tool: "Solver", uri: "aserv://host/a?x=1 2", inputFile: "/tmp/run/in.json", outputDir: "/tmp/run",
		inputs: map[string]runtime.ToolValue{
			"mass":  {Value: semantics.Value{Kind: semantics.ValReal, Real: 1500.5}, Unit: "kg"},
			"n":     {Value: semantics.Value{Kind: semantics.ValInt, Int: 3}},
			"on":    {Value: semantics.Value{Kind: semantics.ValBool, Bool: true}},
			"label": {Text: "a b; rm -rf / {x} $HOME `id`"},
		}}
	cases := map[string]string{
		"{mass}":                    "1500.5",
		"{mass.value}":              "1500.5",
		"{mass.unit}":               "kg",
		"{n} {on}":                  "3 true",
		"{n.unit}":                  "",
		"{label}":                   "a b; rm -rf / {x} $HOME `id`",
		"--label={label}":           "--label=a b; rm -rf / {x} $HOME `id`",
		"{{{label}}}":               "{a b; rm -rf / {x} $HOME `id`}",
		"{toolName}@{uri}":          "Solver@aserv://host/a?x=1 2",
		"{inputFile}|{outputDir}/o": "/tmp/run/in.json|/tmp/run/o",
	}
	for text, want := range cases {
		tpl, err := parseTemplate(text)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		got, err := tpl.render(sc)
		if err != nil {
			t.Errorf("%q: %v", text, err)
		} else if got != want {
			t.Errorf("%q rendered %q, want %q", text, got, want)
		}
	}
	inv := checked(t, &Invocation{Args: []string{"{label}", "--n", "{n}"}}, "label", "n")
	args, err := inv.renderArgs(sc)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a b; rm -rf / {x} $HOME `id`", "--n", "3"}; !reflect.DeepEqual(args, want) {
		t.Errorf("argv %q, want %q: each template one entry", args, want)
	}
}

// A declared variable the call did not send is the typed runtime error, from every place a
// template is rendered; so is a value the protocol does not carry.
func TestTemplateRefusesAnUnsentInput(t *testing.T) {
	sc := scope{tool: "Solver", inputs: map[string]runtime.ToolValue{
		"inf": {Value: semantics.Value{Kind: semantics.ValReal, Real: math.Inf(1)}},
	}}
	for _, inv := range []*Invocation{
		{Args: []string{"--mass", "{mass}"}},
		{Env: map[string]string{"MASS": "{mass.value}"}},
		{Stdin: StdinSpec{Format: StdinTemplate, Template: "{mass.unit}"}},
		{Args: []string{"{inf}"}},
	} {
		call := &runtime.ToolCall{ToolName: "Solver"}
		for name, v := range sc.inputs {
			call.Inputs = append(call.Inputs, runtime.ToolInput{Variable: name, Value: v})
		}
		entry := ToolEntry{ToolName: "Solver", Variables: []string{"mass", "inf"}, Invocation: checked(t, inv, "mass", "inf")}
		_, err := inv.compose(call, entry, nil)
		var fault *runtime.ToolError
		if !errors.As(err, &fault) || fault.Kind != runtime.ToolUnsentInput || fault.Tool != "Solver" {
			t.Errorf("%+v: %v, want ToolUnsentInput", inv, err)
		}
	}
}

// The process environment is the base variables, those passed through by name, and the
// block's, which override; nothing else of this process's environment reaches the tool.
func TestInvocationEnvIsMinimal(t *testing.T) {
	t.Setenv("OPENSYSML_TEST_SECRET", "hidden")
	t.Setenv("OPENSYSML_TEST_SHOWN", "shown")
	t.Setenv("LANG", "C.UTF-8")
	t.Setenv(ToolEnvPassthroughEnv, " OPENSYSML_TEST_SHOWN, OPENSYSML_TEST_UNSET ,")
	inv := checked(t, &Invocation{Env: map[string]string{"RUN": "{label}", "LANG": "en_US"}}, "label")
	env, err := inv.renderEnv(scope{inputs: map[string]runtime.ToolValue{"label": {Text: "r1"}}})
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string, len(env))
	for _, pair := range env {
		name, value, _ := strings.Cut(pair, "=")
		got[name] = value
	}
	if _, leaked := got["OPENSYSML_TEST_SECRET"]; leaked {
		t.Errorf("env %v carries OPENSYSML_TEST_SECRET, which nothing passed through", env)
	}
	if _, leaked := got[ToolEnvPassthroughEnv]; leaked {
		t.Errorf("env %v carries %s itself", env, ToolEnvPassthroughEnv)
	}
	if got["OPENSYSML_TEST_SHOWN"] != "shown" || got["RUN"] != "r1" || got["LANG"] != "en_US" || got["PATH"] != os.Getenv("PATH") {
		t.Errorf("env %v, want the passed-through variable, the rendered one, the override and PATH", env)
	}
	if _, set := got["OPENSYSML_TEST_UNSET"]; set {
		t.Errorf("env %v carries a passed-through variable this process does not hold", env)
	}
	if !sortedPairs(env) {
		t.Errorf("env %v, want it in name order", env)
	}
}

func sortedPairs(env []string) bool {
	for i := 1; i < len(env); i++ {
		if env[i-1] > env[i] {
			return false
		}
	}
	return true
}

// The CSV forms carry the inputs sent in the entry's variable order, a unit column after a
// measured one, quoted as CSV needs.
func TestInvocationCSV(t *testing.T) {
	entry := ToolEntry{ToolName: "Solver", Variables: []string{"mass", "label", "power", "a"}}
	call := &runtime.ToolCall{ToolName: "Solver", Inputs: []runtime.ToolInput{
		{Variable: "power", Value: runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValInt, Int: 2}, Unit: "kW"}},
		{Variable: "label", Value: runtime.ToolValue{Text: `run "1", b`}},
		{Variable: "mass", Value: runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: 1500}}},
	}}
	got, err := csvBytes(call, entry)
	if err != nil {
		t.Fatal(err)
	}
	want := "mass,label,power,power.unit\n1500,\"run \"\"1\"\", b\",2,kW\n"
	if string(got) != want {
		t.Errorf("csv\n%s\nwant\n%s", got, want)
	}
}

// echoDriver performs Echo, a tool-computed action, with fixed inputs and adopts its
// string outputs: what the echo stand-in saw of its command.
const echoDriver = `package Probe {
	private import AnalysisTooling::*;
	private import ScalarValues::*;
	private import ISQ::*;

	action def Echo {
		metadata ToolExecution {
			toolName = "Echo";
			uri = "echo://host/a?x=1 2";
		}
		in mass : MassValue        { @ToolVariable { name = "mass"; } }
		in power : PowerValue      { @ToolVariable { name = "power"; } }
		in label : String          { @ToolVariable { name = "label"; } }
		in count : Integer         { @ToolVariable { name = "count"; } }
		out argv : String          { @ToolVariable { name = "argv"; } }
		out env : String           { @ToolVariable { name = "env"; } }
		out cwd : String           { @ToolVariable { name = "cwd"; } }
		out stdin : String         { @ToolVariable { name = "stdin"; } }
		out input : String         { @ToolVariable { name = "input"; } }
		out outDir : String        { @ToolVariable { name = "outDir"; } }
	}

	action def Once {
		out argv : String;
		out env : String;
		out cwd : String;
		out stdin : String;
		out input : String;
		out outDir : String;
		action step : Echo {
			in mass = 1500 [SI::kg];
			in power = 2 [SI::kW];
			in label = "a b; rm -rf / {x}";
			in count = 3;
		}
		bind argv = step.argv;
		bind env = step.env;
		bind cwd = step.cwd;
		bind stdin = step.stdin;
		bind input = step.input;
		bind outDir = step.outDir;
	}
}`

// echoVariables are the echo stand-in's tool variables, inputs then outputs.
var echoVariables = []string{"mass", "power", "label", "count", "argv", "env", "cwd", "stdin", "input", "outDir"}

// echoRequest is the protocol's request object for Once, as the JSON forms carry it.
const echoRequest = `{"toolName":"Echo","uri":"echo://host/a?x=1 2","inputs":{"count":{"value":3},"label":{"value":"a b; rm -rf / {x}"},"mass":{"value":1500,"unit":"kg"},"power":{"value":2,"unit":"kW"}}}`

// echoCSV is the CSV form of the same inputs, in the entry's variable order.
const echoCSV = "mass,mass.unit,power,power.unit,label,count\n1500,kg,2,kW,a b; rm -rf / {x},3\n"

var (
	echoOnce sync.Once
	echoPath string
	echoErr  error
)

// echo builds the echo stand-in once per test binary and returns its path.
func echo(t *testing.T) string {
	t.Helper()
	echoOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolecho")
		if err != nil {
			echoErr = err
			return
		}
		echoPath = filepath.Join(dir, "toolecho")
		build := exec.Command("go", gobuild.Args(echoPath)...)
		build.Dir = filepath.Join("testdata", "toolecho")
		if out, err := build.CombinedOutput(); err != nil {
			echoErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if echoErr != nil {
		t.Fatalf("building the echo stand-in: %v", echoErr)
	}
	return echoPath
}

// probe is the echo driver indexed over the standard libraries.
type probe struct {
	idx *symbols.Index
	pkg *symbols.Scope
}

func parseProbe(t *testing.T) *probe {
	t.Helper()
	idx := libs.NewModelIndex()
	p := parser.New(source.New("probe.sysml", []byte(echoDriver)))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse probe: %v", p.Diagnostics)
	}
	idx.AddDocument("probe.sysml", file)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("probe.sysml").LookupLocal("Probe")
	if !ok || pkg.Scope == nil {
		t.Fatal("probe package not indexed")
	}
	return &probe{idx: idx, pkg: pkg.Scope}
}

func (p *probe) context() *runtime.Context {
	resolver := resolve.New(p.idx)
	model := runtime.NewModel(passes.NewTypedModel(resolver), resolver)
	model.SetExpressionParser(parser.ParseOneExpression)
	return runtime.NewContext(model, fixtureSteps)
}

// perform performs Probe::Once against the registry and returns its string outputs.
func (p *probe) perform(t *testing.T, r *Registry) map[string]string {
	t.Helper()
	action, ok := p.pkg.LookupLocal("Once")
	if !ok {
		t.Fatal("Once not indexed")
	}
	out, _, err := Perform(context.Background(), r, request(Held(p.context()), "Probe::Once", runtime.DefaultSchedulePolicy),
		func(rctx *runtime.Context) (map[string]runtime.Value, error) { return rctx.ExecuteAction(action) },
		func(out map[string]runtime.Value, err error) Answer {
			if err != nil {
				return Answer{Err: err}
			}
			return Answer{Claim: ClaimValue, Values: ValuesOf(out)}
		})
	if err != nil {
		t.Fatalf("perform: %v", err)
	}
	texts := make(map[string]string)
	for _, name := range echoVariables[4:] {
		v, ok := out[name]
		if !ok || v.Kind != runtime.ValString {
			t.Fatalf("%s = %s, want a string output; got %v", name, runtime.FormatValue(v), out)
		}
		texts[name] = v.Str()
	}
	return texts
}

// echoEntry is the manifest entry the probe's ToolExecution resolves to, answered by the
// echo stand-in under the invocation block.
func echoEntry(executable string, inv *Invocation) ToolEntry {
	return ToolEntry{ToolName: "Echo", Executable: executable, Variables: echoVariables, Invocation: inv}
}

// envOf reads the environment the stand-in echoed.
func envOf(text string) map[string]string {
	env := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		if name, value, ok := strings.Cut(line, "="); ok {
			env[name] = value
		}
	}
	return env
}

// The stand-in is started as the block composes: each argument one argv entry, a shell's
// metacharacters and spaces in a value included; the minimal environment with the block's
// variables rendered; the manifest-relative working directory; the CSV inputs on stdin; the
// JSON input file written and named; and the output directory made, then removed.
func TestEchoStandInSeesTheComposedCommand(t *testing.T) {
	t.Setenv("OPENSYSML_TEST_SECRET", "hidden")
	t.Setenv(ToolEnvPassthroughEnv, "GOCOVERDIR")
	inv := &Invocation{
		Args: []string{"solve.py", "--mass", "{mass}", "--power", "{power.value}", "--unit", "{power.unit}", "--label", "{label}",
			"--n", "{count}", "{{literal}}", "{toolName}", "{uri}", "{inputFile}", "{outputDir}/result.csv"},
		Env:       map[string]string{"TOOL_ECHO_INPUT": "{inputFile}", "TOOL_ECHO_OUTDIR": "{outputDir}", "RUN": "{label} of {toolName}"},
		Cwd:       "work",
		Stdin:     StdinSpec{Format: StdinCSV},
		InputFile: &InputFile{Format: InputJSON, Name: "inputs.json"},
	}
	dir := manifestDir(t, echoEntry(echo(t), inv))
	if err := os.Mkdir(filepath.Join(dir, "work"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := parseProbe(t).perform(t, toolRegistry(t, dir))

	var argv []string
	if err := json.Unmarshal([]byte(out["argv"]), &argv); err != nil {
		t.Fatalf("argv %q: %v", out["argv"], err)
	}
	outDir := out["outDir"]
	if outDir == "" || !strings.HasPrefix(filepath.Base(outDir), "opensysml-tool-") {
		t.Fatalf("output directory %q, want one made for the invocation", outDir)
	}
	wantArgv := []string{"solve.py", "--mass", "1500", "--power", "2", "--unit", "kW", "--label", "a b; rm -rf / {x}",
		"--n", "3", "{literal}", "Echo", "echo://host/a?x=1 2", filepath.Join(outDir, "inputs.json"), filepath.Join(outDir, "result.csv")}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Errorf("argv\n%q\nwant\n%q", argv, wantArgv)
	}
	env := envOf(out["env"])
	if _, leaked := env["OPENSYSML_TEST_SECRET"]; leaked {
		t.Errorf("the stand-in saw OPENSYSML_TEST_SECRET; the parent environment is not inherited")
	}
	if env["RUN"] != "a b; rm -rf / {x} of Echo" || env["TOOL_ECHO_OUTDIR"] != outDir || env["PATH"] != os.Getenv("PATH") {
		t.Errorf("env %v, want the block's variables rendered over PATH", env)
	}
	if want, _ := filepath.EvalSymlinks(filepath.Join(dir, "work")); out["cwd"] != want {
		t.Errorf("cwd %q, want %q", out["cwd"], want)
	}
	if out["stdin"] != echoCSV {
		t.Errorf("stdin\n%q\nwant\n%q", out["stdin"], echoCSV)
	}
	if strings.TrimSpace(out["input"]) != echoRequest {
		t.Errorf("input file\n%s\nwant\n%s", out["input"], echoRequest)
	}
	if _, err := os.Stat(outDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat %s: %v, want the directory removed once the reply is read", outDir, err)
	}
}

// Each stdin form reaches the stand-in as specified; the JSON one is the request an entry
// without the block writes, byte for byte.
func TestEchoStandInReadsEachStdinForm(t *testing.T) {
	cases := map[string]struct {
		stdin StdinSpec
		want  string
	}{
		"json":     {StdinSpec{Format: StdinJSON}, echoRequest},
		"none":     {StdinSpec{Format: StdinNone}, ""},
		"csv":      {StdinSpec{Format: StdinCSV}, echoCSV},
		"template": {StdinSpec{Format: StdinTemplate, Template: "m={mass} [{mass.unit}]\n{{{label}}}\n"}, "m=1500 [kg]\n{a b; rm -rf / {x}}\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			inv := &Invocation{Args: []string{"{count}"}, Stdin: tc.stdin}
			out := parseProbe(t).perform(t, toolRegistry(t, manifestDir(t, echoEntry(echo(t), inv))))
			if out["stdin"] != tc.want {
				t.Errorf("stdin\n%q\nwant\n%q", out["stdin"], tc.want)
			}
			if out["argv"] != `["3"]` || out["input"] != "" || out["outDir"] != "" {
				t.Errorf("argv %s, input %q, outDir %q: want the one argument and no files", out["argv"], out["input"], out["outDir"])
			}
			if out["cwd"] == "" {
				t.Error("cwd empty, want this process's inherited")
			}
		})
	}
}

// A CSV input file carries the CSV form; OPENSYSML_TOOL_KEEP=1 leaves the directory for inspection.
func TestEchoStandInKeepsTheDirectoryWhenAsked(t *testing.T) {
	t.Setenv(ToolKeepEnv, "1")
	inv := &Invocation{Env: map[string]string{"TOOL_ECHO_INPUT": "{inputFile}", "TOOL_ECHO_OUTDIR": "{outputDir}"},
		Stdin: StdinSpec{Format: StdinNone}, InputFile: &InputFile{Format: InputCSV, Name: "inputs.csv"}}
	out := parseProbe(t).perform(t, toolRegistry(t, manifestDir(t, echoEntry(echo(t), inv))))
	outDir := out["outDir"]
	if outDir == "" {
		t.Fatal("no output directory")
	}
	defer os.RemoveAll(outDir)
	if out["input"] != echoCSV {
		t.Errorf("input file\n%q\nwant\n%q", out["input"], echoCSV)
	}
	result, err := os.ReadFile(filepath.Join(outDir, "result.txt"))
	if err != nil || string(result) != "done\n" {
		t.Errorf("read the stand-in's result: %q, %v; want the directory kept with what it wrote", result, err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "inputs.csv")); err != nil {
		t.Errorf("input file: %v, want it kept", err)
	}
}

// The stand-in's bounds still hold under the block: a working directory the process cannot
// enter fails it as a process failure, with the executable and the fault named.
func TestEchoStandInFailsInAMissingDirectory(t *testing.T) {
	inv := &Invocation{Cwd: filepath.Join(t.TempDir(), "gone")}
	dir := manifestDir(t, echoEntry(echo(t), inv))
	p := parseProbe(t)
	action, _ := p.pkg.LookupLocal("Once")
	_, _, err := Perform(context.Background(), toolRegistry(t, dir), request(Held(p.context()), "Probe::Once", runtime.DefaultSchedulePolicy),
		func(rctx *runtime.Context) (map[string]runtime.Value, error) { return rctx.ExecuteAction(action) },
		func(out map[string]runtime.Value, err error) Answer { return Answer{Err: err} })
	var fault *runtime.ToolError
	if !errors.As(err, &fault) || fault.Kind != runtime.ToolProcessFailed {
		t.Fatalf("perform: %v, want ToolProcessFailed", err)
	}
}
