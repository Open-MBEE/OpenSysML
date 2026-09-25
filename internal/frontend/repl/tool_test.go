package repl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// toolCaseSource is a tool-computed action and the case performing it.
const toolCaseSource = `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	action def Heating {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in mass : Real = 12.5 { @ToolVariable { name = "mass"; } }
		out tMax : Real      { @ToolVariable { name = "tMax"; } }
	}
	analysis def CheckHeating {
		action h : Heating;
		out result : Real = h.tMax;
	}
}
`

var (
	toolStandinOnce sync.Once
	toolStandinPath string
	toolStandinErr  error
)

// toolStandin builds the tool protocol stand-in once per test binary.
func toolStandin(t *testing.T) string {
	t.Helper()
	toolStandinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolstandin")
		if err != nil {
			toolStandinErr = err
			return
		}
		toolStandinPath = filepath.Join(dir, "toolstandin")
		build := exec.Command("go", gobuild.Args(toolStandinPath)...)
		build.Dir = filepath.Join("..", "..", "exec", "analysis", "testdata", "toolstandin")
		if out, err := build.CombinedOutput(); err != nil {
			toolStandinErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if toolStandinErr != nil {
		t.Fatalf("building the stand-in tool: %v", toolStandinErr)
	}
	return toolStandinPath
}

// toolManifest writes one manifest entry per JSON string into a directory of the
// test's own and puts the registry it makes on the session.
func toolManifest(t *testing.T, s *Session, entries ...string) {
	t.Helper()
	dir := t.TempDir()
	for i, entry := range entries {
		name := fmt.Sprintf("%02d%s", i, analysis.ManifestExt)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(analysis.ToolsEnv, dir)
	r, err := analysis.DefaultFromEnv()
	if err != nil {
		t.Fatalf("%s=%s: %v", analysis.ToolsEnv, dir, err)
	}
	if err := s.SetEngines(r); err != nil {
		t.Fatalf("SetEngines: %v", err)
	}
}

// %tool previews the invocation block's composition of a case's tool call — argv,
// environment, stdin, input file and reply — without starting the process.
func TestToolPreviewsTheComposedInvocation(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	record := filepath.Join(t.TempDir(), "requests.jsonl")
	t.Setenv("TOOL_STANDIN_RECORD", record)
	entry := `{"kind":"tool","toolName":"Solver","version":"2.3",` +
		`"executable":"` + toolStandin(t) + `","variables":["mass","tMax"],` +
		`"invocation":{"args":["solve.py","--mass","{mass}","{outputDir}/out.txt"],` +
		`"env":{"SOLVER_HOME":"/opt/solver"},"stdin":"csv",` +
		`"inputFile":{"format":"csv","name":"inputs.csv"}},` +
		`"reply":{"format":"csv","source":"stdout","outputs":{"tMax":{"column":"tmax","type":"number","unit":"K"}}}}`
	toolManifest(t, s, entry)

	out := run(t, s, "%tool Tools::CheckHeating")
	wantsInOrder(t, out,
		"✓ Tools::CheckHeating: dry run of tool 'Solver' for Tools::CheckHeating",
		"tool: Solver 2.3",
		"protocol: argv+csv/csv",
		"manifest: "+filepath.Join(os.Getenv(analysis.ToolsEnv)),
		"executable: "+toolStandin(t),
		"argv:",
		`  "solve.py"`, `  "--mass"`, `  "12.5"`, `  "<outputDir>/out.txt"`,
		"env:", "  SOLVER_HOME=/opt/solver",
		"cwd: inherited",
		"stdin: csv", "  mass", "  12.5",
		"input file: <inputFile> (csv, named inputs.csv)", "  mass", "  12.5",
		"output dir: <outputDir>",
		"inputs:", "  mass = 12.5",
		"outputs:", "  tMax",
		`reply: csv from stdout, header, delimiter ","`,
		`  tMax: column "tmax", row last, type number, unit K`,
		"the process was not started")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("the tool recorded a request; a dry run must not start it")
	}
}

// %tool on an action previews the same composition, run to completion without a
// debugging session left active.
func TestToolPreviewsAnActionsCall(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	out := run(t, s, "%tool Tools::Heating")
	wantsInOrder(t, out,
		"✓ Tools::Heating: dry run of tool 'Solver' for Tools::Heating",
		"protocol: object",
		"argv: (none)",
		"env: inherited from this process",
		"stdin: json",
		`  {"toolName":"Solver"`,
		"inputs:", "  mass = 12.5",
		"reply: object — the protocol's JSON object, one key per output variable",
		"the process was not started")
	if s.actionExec != nil {
		t.Fatal("the command left a debugging session active")
	}
}

// Without the manifest the call previews nothing: the typed refusal names the
// tool and the variable that registers it.
func TestToolWithoutAManifestIsNotRegistered(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	t.Setenv(analysis.ToolsEnv, "")
	if err := s.SetEngines(analysis.Default()); err != nil {
		t.Fatalf("SetEngines: %v", err)
	}
	out := run(t, s, "%tool Tools::CheckHeating")
	wants(t, out, "error: ", "tool 'Solver' is not registered; set OPENSYSML_TOOLS")
	rejects(t, out, "the process was not started", "argv:")
}

// An invocation naming an input the call does not send refuses as the real run
// does, with the same typed error.
func TestToolRefusesAnUnsentInput(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","power","tMax"],` +
		`"invocation":{"args":["{power}"]}}`
	toolManifest(t, s, entry)

	out := run(t, s, "%tool Tools::CheckHeating")
	wants(t, out, "error: ", "the invocation names {power} but the call sent no value for power")
	rejects(t, out, "the process was not started")
}

// A case reaching no tool says so rather than previewing nothing.
func TestToolOnACaseReachingNoTool(t *testing.T) {
	s := loadSource(t, `package Plain {
	private import ScalarValues::Real;
	analysis def Bare { out x : Real = 2.0; }
}`)
	out := run(t, s, "%tool Plain::Bare")
	wants(t, out, "error: ", "no ToolExecution-annotated action was reached; nothing to preview")
	rejects(t, out, "the process was not started")
}

// %tool with no argument, or a malformed one, prints its usage.
func TestToolUsage(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	wants(t, run(t, s, "%tool"), toolUsage)
	wants(t, run(t, s, "%tool Tools::CheckHeating(3.0"), "argument list", toolUsage)
}

// %help lists %tool with its argument shape.
func TestToolIsListedInHelp(t *testing.T) {
	wants(t, strings.Join(helpText(), "\n"), "%tool <case|action>[(<args>)] [<object>]")
}

// %record of a case whose action is tool-computed writes the tools the run
// reached into the record's provenance, one element per call in call order.
func TestRecordWritesTheToolsARunReached(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("the tool is a shell script")
	}
	s := loadSource(t, toolCaseSource)
	dir := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\n" +
		`printf '{"outputs":{"tMax":{"value":87.2}}}\n'` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "solver.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	toolManifest(t, s, `{"kind":"tool","toolName":"Solver","executable":"`+filepath.Join(dir, "solver.sh")+`","variables":["mass","tMax"]}`)

	out := run(t, s, "%record Tools::CheckHeating into Tools::Log")
	wants(t, out, "recorded Tools::Log::CheckHeating_run1")
	wants(t, s.text(), `tools = ("Solver from `, "solver.sh")
}
