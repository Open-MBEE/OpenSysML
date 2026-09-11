package analysis

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// writeManifest writes the given files into a manifest directory of the test's own.
func writeManifest(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestManifestReadsOneEntryPerJSONFile(t *testing.T) {
	dir := writeManifest(t, map[string]string{
		"modelcenter.json": `{"toolName": "ModelCenter", "version": "2024.1", "executable": "/opt/mc/bin/mc", "variables": ["deltaT", "a"]}`,
		"local.json":       `{"toolName": "Local", "executable": "bin/local", "variables": []}`,
		"onpath.json":      `{"toolName": "OnPath", "executable": "octave", "variables": ["x"]}`,
		"README.md":        "not an entry",
		"nested/x.json":    `{"toolName": "Nested", "executable": "x", "variables": []}`,
	})
	entries, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(entries) != 3 || entries[0].ToolName != "Local" || entries[1].ToolName != "ModelCenter" || entries[2].ToolName != "OnPath" {
		t.Fatalf("entries %+v, want Local, ModelCenter, OnPath in name order from the top-level .json files", entries)
	}
	if e := entries[1]; e.Version != "2024.1" || e.Executable != "/opt/mc/bin/mc" || len(e.Variables) != 2 || e.File != filepath.Join(dir, "modelcenter.json") {
		t.Errorf("ModelCenter %+v, want its fields as written", e)
	}
	if e := entries[0]; e.Executable != filepath.Join(dir, "bin", "local") {
		t.Errorf("Local's executable %s, want it resolved against the entry's directory", e.Executable)
	}
	if e := entries[2]; e.Executable != "octave" {
		t.Errorf("OnPath's executable %s, want the bare name kept for PATH", e.Executable)
	}
	if !entries[1].Accepts("deltaT") || entries[1].Accepts("b") {
		t.Errorf("ModelCenter accepts %v, want the listed variables alone", entries[1].Variables)
	}
}

func TestManifestFaultsAreTyped(t *testing.T) {
	cases := map[string]struct {
		text   string
		detail string
	}{
		"not JSON":        {`toolName = MC`, "not one JSON object"},
		"two objects":     {`{"toolName":"A","executable":"a","variables":[]} {"toolName":"B","executable":"b","variables":[]}`, "not one JSON object"},
		"unknown field":   {`{"toolName":"A","executable":"a","variables":[],"timeout":"1s"}`, "not one JSON object"},
		"no toolName":     {`{"executable":"a","variables":[]}`, "toolName is empty"},
		"blank toolName":  {`{"toolName":"  ","executable":"a","variables":[]}`, "toolName is empty"},
		"spaced toolName": {`{"toolName":"Model Center","executable":"a","variables":[]}`, "has whitespace"},
		"no executable":   {`{"toolName":"A","variables":["x"]}`, "executable is empty"},
		"no variables":    {`{"toolName":"A","executable":"a"}`, "variables is missing"},
		"empty variable":  {`{"toolName":"A","executable":"a","variables":["x",""]}`, "empty name"},
		"twice variable":  {`{"toolName":"A","executable":"a","variables":["x","x"]}`, `lists "x" twice`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeManifest(t, map[string]string{"entry.json": tc.text})
			_, err := LoadManifest(dir)
			var fault *ManifestError
			if !errors.As(err, &fault) || !errors.Is(err, ErrManifest) {
				t.Fatalf("load: %v, want a ManifestError", err)
			}
			if fault.Path != filepath.Join(dir, "entry.json") || !strings.Contains(fault.Detail, tc.detail) {
				t.Errorf("fault %+v, want the entry named with %q", fault, tc.detail)
			}
			if !strings.HasPrefix(err.Error(), ToolsEnv+": ") {
				t.Errorf("error %q, want it to open with the variable to fix", err)
			}
		})
	}
}

func TestManifestRefusesTwoEntriesForOneTool(t *testing.T) {
	dir := writeManifest(t, map[string]string{
		"a.json": `{"toolName":"MC","executable":"a","variables":[]}`,
		"b.json": `{"toolName":"MC","executable":"b","variables":[]}`,
	})
	_, err := LoadManifest(dir)
	var fault *ManifestError
	if !errors.As(err, &fault) || fault.Path != filepath.Join(dir, "b.json") || !strings.Contains(fault.Detail, `tool "MC" is also the entry`) {
		t.Fatalf("load: %v, want the second entry refused as a duplicate", err)
	}
}

func TestManifestDirectoryMustBeReadable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none")
	_, err := LoadManifest(missing)
	var fault *ManifestError
	if !errors.As(err, &fault) || fault.Path != missing || fault.Err == nil {
		t.Fatalf("load: %v, want a ManifestError with the directory's fault", err)
	}
	t.Setenv(ToolsEnv, missing)
	if _, err := DefaultFromEnv(); !errors.Is(err, ErrManifest) {
		t.Fatalf("DefaultFromEnv: %v, want the manifest's fault", err)
	}
}

// Unset, the environment adds no tool and the default engines are the build's.
func TestToolsFromEnvUnsetIsNoTool(t *testing.T) {
	t.Setenv(ToolsEnv, "")
	tools, err := ToolsFromEnv()
	if err != nil || len(tools) != 0 {
		t.Fatalf("tools %v, %v; want none", tools, err)
	}
	r, err := DefaultFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := names(r), names(Default()); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("engines %v, want the build's %v", got, want)
	}
}

// Every manifest entry is an engine tool:<name>, listed in name order among the build's
// engines with its process status, and the one engine answering compute.
func TestToolsFromEnvRegisterEachEntry(t *testing.T) {
	present := standin(t)
	absent := filepath.Join(t.TempDir(), "none")
	r := toolRegistry(t, manifestDir(t,
		ToolEntry{ToolName: "Zed", Version: "9", Executable: present, Variables: []string{"x"}},
		ToolEntry{ToolName: "Absent", Executable: absent, Variables: []string{"x"}},
	))
	want := []string{ExploreEngineName, RunEngineName, SolveEngineName, SweepEngineName, "tool:Absent", "tool:Zed"}
	if got := names(r); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("engines %v, want %v", got, want)
	}
	statuses := r.Statuses()
	byEngine := make(map[string]Status)
	for _, s := range statuses {
		byEngine[s.Engine] = s
	}
	if s := byEngine["tool:Zed"]; s.Err != nil || s.Process != "Zed 9 at "+present {
		t.Errorf("tool:Zed status %+v, want its version and path", s)
	}
	var absence *ProcessAbsentError
	if s := byEngine["tool:Absent"]; !errors.As(s.Err, &absence) || !errors.Is(s.Err, ErrToolAbsent) || absence.Engine != "tool:Absent" {
		t.Errorf("tool:Absent status %+v, want ProcessAbsentError wrapping the executable's absence", s)
	}
	for _, e := range r.Engines() {
		d := e.Describe()
		answers := false
		for _, k := range d.Questions {
			answers = answers || k == Compute
		}
		if answers != strings.HasPrefix(e.Name(), ToolEnginePrefix) {
			t.Errorf("%s answers compute: %v; want the tool engines alone", e.Name(), answers)
		}
		if answers && d.Authority != Observed {
			t.Errorf("%s authority %v, want observed", e.Name(), d.Authority)
		}
	}
}

// A second engine for a tool already registered is the registry's typed duplicate error,
// and two registries loaded from one manifest do not see each other's engines.
func TestToolRegistrationIsIsolatedAndUnique(t *testing.T) {
	entry := ToolEntry{ToolName: "MC", Executable: standin(t), Variables: []string{"x"}}
	first := toolRegistry(t, manifestDir(t, entry))
	err := first.Register(NewTool(entry))
	var dup *DuplicateEngineError
	if !errors.As(err, &dup) || dup.Name != "tool:MC" {
		t.Fatalf("second tool:MC: %v, want DuplicateEngineError", err)
	}
	second := registered(t, NewRun())
	if err := second.Register(NewTool(entry)); err != nil {
		t.Fatalf("tool:MC in another registry: %v", err)
	}
	if got := names(first); len(got) != 5 {
		t.Errorf("first registry %v, want the build's four engines and tool:MC", got)
	}
	if got := names(second); strings.Join(got, ",") != RunEngineName+",tool:MC" {
		t.Errorf("second registry %v, want run and tool:MC", got)
	}
}

// Covers refuses, typed, everything but a compute of this tool over variables it accepts
// while its executable is present.
func TestToolEngineCoversItsOwnComputationsOnly(t *testing.T) {
	entry := ToolEntry{ToolName: "MC", Executable: standin(t), Variables: []string{"x", "y"}}
	e := NewTool(entry)
	if c := e.Covers(nil, Question{Kind: Evaluate}); c.Covered || !errors.Is(c.Refusal, ErrNotAsked) {
		t.Errorf("evaluate: %+v, want NotAskedError", c)
	}
	if c := e.Covers(nil, Question{Kind: Compute}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Errorf("compute without a call: %+v, want MalformedQuestionError", c)
	}
	other := &runtime.ToolCall{ToolName: "Other"}
	var wrong *WrongToolError
	if c := e.Covers(nil, Question{Kind: Compute, Compute: &ComputeAsk{Call: other}}); c.Covered || !errors.As(c.Refusal, &wrong) || wrong.Tool != "Other" {
		t.Errorf("another tool's call: %+v, want WrongToolError", c)
	}
	unknown := &runtime.ToolCall{ToolName: "MC", Inputs: []runtime.ToolInput{{Variable: "x"}, {Variable: "z"}}, Outputs: []runtime.ToolOutput{{Variable: "w"}}}
	var variable *ToolVariableError
	if c := e.Covers(nil, Question{Kind: Compute, Compute: &ComputeAsk{Call: unknown}}); c.Covered || !errors.As(c.Refusal, &variable) || strings.Join(variable.Variables, ",") != "w,z" {
		t.Errorf("unaccepted variables: %+v, want ToolVariableError naming w and z", c)
	}
	ok := &runtime.ToolCall{ToolName: "MC", Inputs: []runtime.ToolInput{{Variable: "x"}}, Outputs: []runtime.ToolOutput{{Variable: "y"}}}
	if c := e.Covers(nil, Question{Kind: Compute, Compute: &ComputeAsk{Call: ok}}); !c.Covered {
		t.Errorf("its own call: %+v, want covered", c)
	}
	gone := NewTool(ToolEntry{ToolName: "MC", Executable: filepath.Join(t.TempDir(), "none"), Variables: []string{"x", "y"}})
	if c := gone.Covers(nil, Question{Kind: Compute, Compute: &ComputeAsk{Call: ok}}); c.Covered || !errors.Is(c.Refusal, ErrProcessAbsent) || !errors.Is(c.Refusal, ErrToolAbsent) {
		t.Errorf("absent executable: %+v, want the absence as refusal", c)
	}
}

func TestToolTimeoutFromEnv(t *testing.T) {
	for text, want := range map[string]time.Duration{"": DefaultToolTimeout, "soon": DefaultToolTimeout, "-1s": DefaultToolTimeout, "0": DefaultToolTimeout, "250ms": 250 * time.Millisecond, " 2m ": 2 * time.Minute} {
		t.Setenv(ToolTimeoutEnv, text)
		if got := toolTimeoutFromEnv(); got != want {
			t.Errorf("%s=%q: %v, want %v", ToolTimeoutEnv, text, got, want)
		}
	}
	if DefaultToolTimeout != 10*time.Second {
		t.Errorf("default %v, want the solver's 10s", DefaultToolTimeout)
	}
}

// The request is one JSON object: the tool and URI as annotated, the inputs keyed by
// ToolVariable name, a quantity with its unit and a bare number, truth or text without.
func TestToolRequestCarriesTheCallUninterpreted(t *testing.T) {
	call := &runtime.ToolCall{ToolName: "Model Center/2", URI: "aserv://host/Vehicle/Equation1?x=1 2",
		Inputs: []runtime.ToolInput{
			{Variable: "v0", Value: runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValInt, Int: 36}, Unit: "km/h"}},
			{Variable: "C_D", Value: runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValReal, Real: 0.3}}},
			{Variable: "on", Value: runtime.ToolValue{Value: semantics.Value{Kind: semantics.ValBool, Bool: true}}},
			{Variable: "label", Value: runtime.ToolValue{Text: "run 1"}},
		}}
	got, err := ToolRequestOf(call)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"toolName":"Model Center/2","uri":"aserv://host/Vehicle/Equation1?x=1 2","inputs":{"C_D":{"value":0.3},"label":{"value":"run 1"},"on":{"value":true},"v0":{"value":36,"unit":"km/h"}}}`
	if strings.TrimSpace(string(got)) != want {
		t.Errorf("request\n%s\nwant\n%s", got, want)
	}
}

func TestToolReplyIsOneObjectOfOutputsOrAnError(t *testing.T) {
	outputs, err := ToolReplyOf("MC", []byte(` {"outputs":{"a":{"value":3.5,"unit":"m/s**2"},"n":{"value":2},"ok":{"value":false},"s":{"value":"done"}}} `))
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if a := outputs["a"]; a.Value.Kind != semantics.ValReal || a.Value.Real != 3.5 || a.Unit != "m/s**2" {
		t.Errorf("a = %+v, want 3.5 m/s**2", a)
	}
	if n := outputs["n"]; n.Value.Kind != semantics.ValInt || n.Value.Int != 2 || n.Unit != "" {
		t.Errorf("n = %+v, want the integer 2", n)
	}
	if ok := outputs["ok"]; ok.Value.Kind != semantics.ValBool || ok.Value.Bool {
		t.Errorf("ok = %+v, want false", ok)
	}
	if s := outputs["s"]; s.Value.Kind != semantics.ValInvalid || s.Text != "done" {
		t.Errorf("s = %+v, want the text done", s)
	}

	cases := map[string]struct {
		stdout string
		kind   runtime.ToolErrorKind
		detail string
	}{
		"nothing":            {"", runtime.ToolMalformed, "wrote nothing"},
		"prose":              {"a = 3", runtime.ToolMalformed, "not one JSON object"},
		"two objects":        {`{"outputs":{}} {"outputs":{}}`, runtime.ToolMalformed, "not one JSON object"},
		"unknown field":      {`{"outputs":{},"log":"x"}`, runtime.ToolMalformed, "not one JSON object"},
		"neither":            {`{}`, runtime.ToolMalformed, "neither outputs nor an error"},
		"null outputs":       {`{"outputs":null}`, runtime.ToolMalformed, "neither outputs nor an error"},
		"both":               {`{"outputs":{},"error":"x"}`, runtime.ToolMalformed, "both outputs and an error"},
		"null error beside":  {`{"outputs":{"a":{"value":2}},"error":null}`, runtime.ToolMalformed, "both outputs and an error"},
		"null error":         {`{"error":null}`, runtime.ToolMalformed, "error is null"},
		"error not text":     {`{"error":{"code":3}}`, runtime.ToolMalformed, "error is not a message"},
		"tool error":         {`{"error":"did not converge"}`, runtime.ToolRefused, "did not converge"},
		"outputs not object": {`{"outputs":[1]}`, runtime.ToolMalformed, "not an object of values"},
		"misspelt unit":      {`{"outputs":{"a":{"value":2,"units":"kg"}}}`, runtime.ToolMalformed, `unknown field "units"`},
		"extra member":       {`{"outputs":{"a":{"value":2,"note":"x"}}}`, runtime.ToolMalformed, `unknown field "note"`},
		"repeated key":       {`{"outputs":{"a":{"value":1},"a":{"value":2}}}`, runtime.ToolMalformed, "names outputs.a twice"},
		"repeated outputs":   {`{"outputs":{"a":{"value":1}},"outputs":{"a":{"value":2}}}`, runtime.ToolMalformed, "names outputs twice"},
		"repeated error":     {`{"error":"x","error":"y"}`, runtime.ToolMalformed, "names error twice"},
		"repeated value":     {`{"outputs":{"a":{"value":1,"value":2}}}`, runtime.ToolMalformed, "names outputs.a.value twice"},
		"repeated unit":      {`{"outputs":{"a":{"value":1,"unit":"m","unit":"s"}}}`, runtime.ToolMalformed, "names outputs.a.unit twice"},
		"repeated past list": {`{"outputs":{"a":{"value":[1,2]},"a":{"value":3}}}`, runtime.ToolMalformed, "names outputs.a twice"},
		"no value":           {`{"outputs":{"a":{"unit":"m"}}}`, runtime.ToolMalformed, "no value"},
		"object value":       {`{"outputs":{"a":{"value":{"x":1}}}}`, runtime.ToolMalformed, "not a number, boolean or string"},
		"huge number":        {`{"outputs":{"a":{"value":1e999}}}`, runtime.ToolMalformed, "not a finite number"},
		"measured truth":     {`{"outputs":{"a":{"value":true,"unit":"m"}}}`, runtime.ToolMalformed, "boolean has no unit"},
		"measured text":      {`{"outputs":{"a":{"value":"x","unit":"m"}}}`, runtime.ToolMalformed, "string has no unit"},
		"null unit":          {`{"outputs":{"a":{"value":4,"unit":null}}}`, runtime.ToolMalformed, "unit is null"},
		"numeric unit":       {`{"outputs":{"a":{"value":4,"unit":7}}}`, runtime.ToolMalformed, "unit 7 is not a unit expression"},
		"blank unit":         {`{"outputs":{"a":{"value":4,"unit":" "}}}`, runtime.ToolMalformed, "unit is empty"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ToolReplyOf("MC", []byte(tc.stdout))
			var fault *runtime.ToolError
			if !errors.As(err, &fault) || !errors.Is(err, runtime.ErrTool) || fault.Kind != tc.kind || fault.Tool != "MC" {
				t.Fatalf("reply: %v, want a ToolError of kind %s", err, tc.kind)
			}
			if !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("error %q does not carry %q", err, tc.detail)
			}
		})
	}
}
