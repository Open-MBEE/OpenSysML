package analysis

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// engineJSON is a well-formed engine entry whose command is the given executable.
func engineJSON(name, command string) string {
	return `{"kind":"engine","name":"` + name + `","version":"1.0","command":["` + command + `","--serve"],` +
		`"protocol":1,"answers":["holds","outcomes"],"subjects":["action"],"model":["sources","graphs:1"],` +
		`"bounds":["depth","steps"],"witness":"schedule","authority":"bounded"}`
}

// writeExecutable puts an executable script at path, making its directories.
func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// One entry of each kind loads, in name order, with the fields the manifest fixes.
func TestManifestReadsEveryKind(t *testing.T) {
	dir := writeManifest(t, map[string]string{
		"spin.json":     engineJSON("spin-bridge", "/opt/spin-bridge/bin/spin-bridge"),
		"priority.json": `{"kind":"policy","name":"priority","version":"0.3","command":["./priority-policy"],"protocol":1}`,
		"lhs.json":      `{"kind":"sampler","name":"lhs","command":["lhs-sampler"],"protocol":1,"concurrent":false}`,
		"wasm.json":     `{"kind":"engine","name":"wasm","module":"engine.wasm","protocol":1,"answers":["holds"],"authority":"observed"}`,
		"remote.json":   `{"kind":"engine","name":"remote","transport":"grpc","address":"engines.example:7000","protocol":1,"answers":["holds"],"model":["rdf"],"authority":"proved"}`,
		"mc.json":       `{"toolName":"MC","executable":"mc","variables":["x"]}`,
	})
	m, err := ReadManifest(dir, EnginesEnv)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(m.Tools) != 1 || m.Tools[0].ToolName != "MC" {
		t.Errorf("tools %+v, want the one tool entry beside the engines", m.Tools)
	}
	var names []string
	for _, e := range m.Engines {
		names = append(names, string(e.Kind)+" "+e.Name)
	}
	if got := strings.Join(names, ", "); got != "sampler lhs, policy priority, engine remote, engine spin-bridge, engine wasm" {
		t.Fatalf("engines %s, want every kind in name order", got)
	}
	spin := m.Engines[3]
	if spin.Version != "1.0" || spin.Executable != "/opt/spin-bridge/bin/spin-bridge" || spin.Transport != TransportStdio ||
		spin.Protocol != 1 || len(spin.Answers) != 2 || spin.Answers[0] != Holds || spin.Witness != WitnessSchedule ||
		spin.Authority != Bounded || !spin.Concurrent || len(spin.Model) != 2 || spin.Model[1] != GraphsForm(1) {
		t.Errorf("spin-bridge %+v, want its fields as written with an absolute command kept", spin)
	}
	if spin.Served() != nil || spin.File != filepath.Join(dir, "spin.json") || spin.Dir != dir {
		t.Errorf("spin-bridge served %v from %s, want it served from its file", spin.Served(), spin.File)
	}
	if v, ok := spin.Model[1].GraphsVersion(); !ok || v != 1 {
		t.Errorf("graphs:1 version %d %v", v, ok)
	}
	priority := m.Engines[1]
	if priority.Executable != filepath.Join(dir, "priority-policy") || priority.Answers != nil {
		t.Errorf("priority %+v, want its relative command confined to the manifest directory and no question fields", priority)
	}
	lhs := m.Engines[0]
	if lhs.Executable != filepath.Join(dir, "lhs-sampler") || lhs.Concurrent {
		t.Errorf("lhs %+v, want a bare command name resolved in the manifest directory, never on PATH", lhs)
	}
	wasm := m.Engines[4]
	if wasm.Module != filepath.Join(dir, "engine.wasm") || wasm.Executable != "" || wasm.Program() != wasm.Module {
		t.Errorf("wasm %+v, want its module confined and the program the module", wasm)
	}
	for _, tc := range []struct {
		entry  EngineEntry
		reason string
	}{
		{priority, "strategies stage"},
		{lhs, "strategies stage"},
		{wasm, "WebAssembly stage"},
		{m.Engines[2], "grpc transport"},
	} {
		err := tc.entry.Served()
		var notServed *NotServedError
		if !errors.As(err, &notServed) || !errors.Is(err, ErrNotServed) || notServed.Name != tc.entry.Name || !strings.Contains(notServed.Reason, tc.reason) {
			t.Errorf("%s served: %v, want a NotServedError naming the %s", tc.entry.Name, err, tc.reason)
		}
	}
	rdf := EngineEntry{Kind: KindEngine, Name: "r", Transport: TransportStdio, Model: []ModelForm{FormSources, FormRDF}}
	if err := rdf.Served(); err == nil || !strings.Contains(err.Error(), "rdf model form is the rdf stage") {
		t.Errorf("rdf served: %v, want the rdf form refused naming its stage", err)
	}
	if forms := m.Engines[2].Forms(); len(forms) != 2 || forms[0] != FormSources || forms[1] != FormRDF {
		t.Errorf("forms %v, want sources first then what was asked", forms)
	}
}

// Every malformed entry is one ManifestError naming the file and the field.
func TestEngineEntryFaultsAreTyped(t *testing.T) {
	base := `"name":"e","command":["e"],"protocol":1,"answers":["holds"],"authority":"bounded"`
	cases := map[string]struct {
		text   string
		detail string
	}{
		"unknown kind":        {`{"kind":"strategy",` + base + `}`, `kind "strategy" is not one of`},
		"kind not a string":   {`{"kind":1,` + base + `}`, "kind is not a string"},
		"unknown field":       {`{"kind":"engine","timeout":"1s",` + base + `}`, "not one JSON object of the engine entry's fields"},
		"two objects":         {`{"kind":"engine",` + base + `} {"kind":"engine",` + base + `}`, "not one JSON object"},
		"no name":             {`{"kind":"engine","command":["e"],"protocol":1,"answers":["holds"],"authority":"bounded"}`, "name is empty"},
		"spaced name":         {`{"kind":"engine","name":"spin bridge","command":["e"],"protocol":1,"answers":["holds"],"authority":"bounded"}`, "has whitespace"},
		"tool-form name":      {`{"kind":"engine","name":"tool:MC","command":["e"],"protocol":1,"answers":["holds"],"authority":"bounded"}`, "tool engines' form"},
		"selection name":      {`{"kind":"engine","name":"auto","command":["e"],"protocol":1,"answers":["holds"],"authority":"bounded"}`, "is a selection"},
		"admit":               {`{"kind":"engine","admit":"bounded",` + base + `}`, "referee-record stage"},
		"no command":          {`{"kind":"engine","name":"e","protocol":1,"answers":["holds"],"authority":"bounded"}`, "command is empty"},
		"empty executable":    {`{"kind":"engine","name":"e","command":[" "],"protocol":1,"answers":["holds"],"authority":"bounded"}`, "executable is empty"},
		"command and module":  {`{"kind":"engine","module":"e.wasm",` + base + `}`, "alternatives"},
		"stdio with address":  {`{"kind":"engine","address":"host:1",` + base + `}`, "address is for the grpc transport"},
		"grpc no address":     {`{"kind":"engine","name":"e","transport":"grpc","protocol":1,"answers":["holds"],"authority":"bounded"}`, "needs an address"},
		"grpc with command":   {`{"kind":"engine","transport":"grpc","address":"host:1",` + base + `}`, "in place of a command"},
		"unknown transport":   {`{"kind":"engine","transport":"pipe",` + base + `}`, `transport "pipe"`},
		"no protocol":         {`{"kind":"engine","name":"e","command":["e"],"answers":["holds"],"authority":"bounded"}`, "protocol is missing"},
		"protocol string":     {`{"kind":"engine","name":"e","command":["e"],"protocol":"1","answers":["holds"],"authority":"bounded"}`, "not a positive integer"},
		"protocol unserved":   {`{"kind":"engine","name":"e","command":["e"],"protocol":2,"answers":["holds"],"authority":"bounded"}`, "protocol 2 is not served; this build serves 1"},
		"policy with answers": {`{"kind":"policy","name":"p","command":["p"],"protocol":1,"answers":["holds"]}`, "a policy entry has no question fields"},
		"no answers":          {`{"kind":"engine","name":"e","command":["e"],"protocol":1,"authority":"bounded"}`, "answers is empty"},
		"unknown answer":      {`{"kind":"engine","name":"e","command":["e"],"protocol":1,"answers":["guesses"],"authority":"bounded"}`, `answers names "guesses"`},
		"answer twice":        {`{"kind":"engine","name":"e","command":["e"],"protocol":1,"answers":["holds","holds"],"authority":"bounded"}`, `lists "holds" twice`},
		"empty subject":       {`{"kind":"engine","subjects":[""],` + base + `}`, "subjects has an empty name"},
		"unknown bound":       {`{"kind":"engine","bounds":["fuel"],` + base + `}`, `bounds names "fuel"`},
		"unknown witness":     {`{"kind":"engine","witness":"trace",` + base + `}`, `witness "trace"`},
		"unknown form":        {`{"kind":"engine","model":["xmi"],` + base + `}`, `model names "xmi"`},
		"graphs unserved":     {`{"kind":"engine","model":["graphs:2"],` + base + `}`, "graphs:2 is not served; this build serves graphs:1"},
		"no authority":        {`{"kind":"engine","name":"e","command":["e"],"protocol":1,"answers":["holds"]}`, `authority ""`},
		"not-covered auth":    {`{"kind":"engine","name":"e","command":["e"],"protocol":1,"answers":["holds"],"authority":"not covered"}`, `authority "not covered"`},
		"concurrent string":   {`{"kind":"engine","concurrent":"yes",` + base + `}`, "not one JSON object"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeManifest(t, map[string]string{"entry.json": tc.text})
			_, err := ReadManifest(dir, EnginesEnv)
			var fault *ManifestError
			if !errors.As(err, &fault) || !errors.Is(err, ErrManifest) {
				t.Fatalf("read: %v, want a ManifestError", err)
			}
			if fault.Path != filepath.Join(dir, "entry.json") || !strings.Contains(fault.Detail, tc.detail) {
				t.Errorf("fault %+v, want the entry named with %q", fault, tc.detail)
			}
			if !strings.HasPrefix(err.Error(), EnginesEnv+": ") {
				t.Errorf("error %q, want it to open with the variable to fix", err)
			}
		})
	}
}

// A name registered by two entries of the engine kinds is refused at the second, whatever
// the kinds; a tool and an engine share a directory without sharing a namespace.
func TestManifestRefusesTwoEntriesForOneEngineName(t *testing.T) {
	dir := writeManifest(t, map[string]string{
		"a.json": `{"kind":"engine","name":"same","command":["a"],"protocol":1,"answers":["holds"],"authority":"bounded"}`,
		"b.json": `{"kind":"policy","name":"same","command":["b"],"protocol":1}`,
		"c.json": `{"toolName":"same","executable":"c","variables":[]}`,
	})
	_, err := ReadManifest(dir, EnginesEnv)
	var fault *ManifestError
	if !errors.As(err, &fault) || fault.Path != filepath.Join(dir, "b.json") || !strings.Contains(fault.Detail, `policy "same" is also the entry`) {
		t.Fatalf("read: %v, want the second entry refused as a duplicate", err)
	}
	if err := os.Remove(filepath.Join(dir, "b.json")); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir, EnginesEnv)
	if err != nil || len(m.Tools) != 1 || len(m.Engines) != 1 {
		t.Fatalf("read: %+v, %v; want the tool and the engine of one name both kept", m, err)
	}
}

// A relative command is confined to the manifest directory through every link: `..`, a link
// inside pointing outside, and a link among the ancestors of a file not yet there are refused.
func TestEngineCommandIsConfinedToTheManifestDirectory(t *testing.T) {
	outside := t.TempDir()
	writeExecutable(t, filepath.Join(outside, "elsewhere"))
	dir := writeManifest(t, nil)
	writeExecutable(t, filepath.Join(dir, "bin", "inside"))
	if err := os.Symlink(filepath.Join(outside, "elsewhere"), filepath.Join(dir, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "lib")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("bin", "inside"), filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	refused := map[string]string{
		"dot-dot":       `../` + filepath.Base(outside) + `/elsewhere`,
		"nested dotdot": `bin/../../` + filepath.Base(outside) + `/elsewhere`,
		"link outside":  `escape`,
		"link ancestor": `lib/not-yet-there`,
	}
	for name, command := range refused {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, "entry.json")
			if err := os.WriteFile(path, []byte(engineJSON("e", command)), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := ReadManifest(dir, EnginesEnv)
			var fault *ManifestError
			if !errors.As(err, &fault) || fault.Path != path || !strings.Contains(fault.Detail, "outside the manifest directory") {
				t.Fatalf("read: %v, want %q refused as outside the manifest directory", err, command)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}
	kept := map[string]string{
		"relative": "bin/inside",
		"dotted":   "./bin/../bin/inside",
		"bare":     "alias",
	}
	for name, command := range kept {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, "entry.json")
			if err := os.WriteFile(path, []byte(engineJSON("e", command)), 0o644); err != nil {
				t.Fatal(err)
			}
			m, err := ReadManifest(dir, EnginesEnv)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if want := filepath.Join(dir, "bin", "inside"); m.Engines[0].Executable != want {
				t.Errorf("%q resolved to %s, want %s through its links", command, m.Engines[0].Executable, want)
			}
			if err := m.Engines[0].Present(); err != nil {
				t.Errorf("present: %v", err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A tool's relative executable follows the same confinement; a bare name stays for PATH.
func TestToolExecutableIsConfinedToTheManifestDirectory(t *testing.T) {
	outside := t.TempDir()
	writeExecutable(t, filepath.Join(outside, "elsewhere"))
	dir := writeManifest(t, map[string]string{
		"mc.json": `{"toolName":"MC","executable":"../` + filepath.Base(outside) + `/elsewhere","variables":[]}`,
	})
	_, err := LoadManifest(dir)
	var fault *ManifestError
	if !errors.As(err, &fault) || !strings.Contains(fault.Detail, "executable") || !strings.Contains(fault.Detail, "outside the manifest directory") {
		t.Fatalf("load: %v, want the executable refused as outside the manifest directory", err)
	}
	if err := os.Symlink(filepath.Join(outside, "elsewhere"), filepath.Join(dir, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mc.json"), []byte(`{"toolName":"MC","executable":"./escape","variables":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(dir); !errors.As(err, &fault) || !strings.Contains(fault.Detail, "outside the manifest directory") {
		t.Fatalf("load: %v, want a link pointing outside refused", err)
	}
}

// An entry's program is checked present without running it: absent, a directory, or not
// executable are each a ProcessAbsentError; an absent program is still listed.
func TestEngineEntryPresentChecksTheProgramWithoutRunning(t *testing.T) {
	dir := writeManifest(t, map[string]string{
		"gone.json": engineJSON("gone", "bin/gone"),
		"dir.json":  engineJSON("dir", "bin"),
		"flat.json": engineJSON("flat", "bin/flat"),
		"wasm.json": `{"kind":"engine","name":"wasm","module":"engine.wasm","protocol":1,"answers":["holds"],"authority":"observed"}`,
	})
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "flat"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "engine.wasm"), []byte("\x00asm"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ReadManifest(dir, EnginesEnv)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := map[string]string{"gone": "no such file", "dir": "not a regular file", "flat": "not executable", "wasm": ""}
	for _, e := range m.Engines {
		err := e.Present()
		var absent *ProcessAbsentError
		switch reason := want[e.Name]; {
		case reason == "":
			if err != nil {
				t.Errorf("%s present: %v, want a module that need not be executable", e.Name, err)
			}
		case !errors.As(err, &absent) || absent.Engine != e.Name || absent.Process != e.Program() || !strings.Contains(err.Error(), reason):
			t.Errorf("%s present: %v, want a ProcessAbsentError with %q", e.Name, err, reason)
		}
	}
}

// A manifest directory under a workspace is not read, followed through its links.
func TestManifestUnderAWorkspaceIsNotRead(t *testing.T) {
	workspace := t.TempDir()
	dir := filepath.Join(workspace, "engines")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "e.json"), []byte(engineJSON("e", "e")), 0o644); err != nil {
		t.Fatal(err)
	}
	if m, err := ReadManifest(dir, EnginesEnv); err != nil || len(m.Engines) != 1 {
		t.Fatalf("read with no workspace: %+v, %v; want the entry", m, err)
	}
	_, err := ReadManifest(dir, EnginesEnv, t.TempDir(), workspace)
	var fault *ManifestError
	if !errors.As(err, &fault) || fault.Path != dir || !strings.Contains(fault.Detail, "is under the workspace "+workspace) {
		t.Fatalf("read under the workspace: %v, want the directory refused unread", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManifest(link, EnginesEnv, workspace); !errors.As(err, &fault) {
		t.Fatalf("read through a link: %v, want the directory refused through its link", err)
	}
	t.Setenv(EnginesEnv, dir)
	t.Setenv(ToolsEnv, "")
	if _, err := ManifestsFromEnv(workspace); !errors.Is(err, ErrManifest) {
		t.Fatalf("ManifestsFromEnv: %v, want the manifest's fault", err)
	}
	manifests, err := ManifestsFromEnv()
	if err != nil || len(manifests) != 1 || manifests[0].Env != EnginesEnv {
		t.Fatalf("ManifestsFromEnv: %v, %v; want the one manifest the environment names", manifests, err)
	}
}

// An entry or its directory writable by anyone but its owner is refused.
func TestManifestRefusesWritableByOthers(t *testing.T) {
	for _, mode := range []os.FileMode{0o664, 0o646, 0o666} {
		dir := writeManifest(t, map[string]string{"e.json": engineJSON("e", "e")})
		if err := os.Chmod(filepath.Join(dir, "e.json"), mode); err != nil {
			t.Fatal(err)
		}
		_, err := ReadManifest(dir, EnginesEnv)
		var fault *ManifestError
		if !errors.As(err, &fault) || fault.Path != filepath.Join(dir, "e.json") || !strings.Contains(fault.Detail, "writable by others") {
			t.Errorf("mode %04o: %v, want the entry refused as writable by others", mode, err)
		}
		if err := os.Chmod(dir, 0o775); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadManifest(dir, EnginesEnv); !errors.As(err, &fault) || fault.Path != dir || !strings.Contains(fault.Detail, "writable by others") {
			t.Errorf("directory mode 0775: %v, want the directory refused before its entries", err)
		}
	}
}
