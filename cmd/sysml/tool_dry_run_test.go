package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/tests/testutil/gobuild"
)

// toolCaseModel is a tool-computed action and the case performing it.
const toolCaseModel = `package Tools {
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
	toolOnce sync.Once
	toolPath string
	toolErr  error
)

// dryRunStandin builds the analysis package's tool stand-in once per test binary.
func dryRunStandin(t *testing.T) string {
	t.Helper()
	toolOnce.Do(func() {
		dir, err := os.MkdirTemp("", "toolstandin")
		if err != nil {
			toolErr = err
			return
		}
		toolPath = filepath.Join(dir, "toolstandin")
		build := exec.Command("go", gobuild.Args(toolPath)...)
		build.Dir = filepath.Join("..", "..", "internal", "exec", "analysis", "testdata", "toolstandin")
		if out, err := build.CombinedOutput(); err != nil {
			toolErr = fmt.Errorf("go build: %v\n%s", err, out)
		}
	})
	if toolErr != nil {
		t.Fatalf("building the stand-in tool: %v", toolErr)
	}
	return toolPath
}

// dryRunManifest writes the tool entries into a manifest directory of the test's
// own and points OPENSYSML_TOOLS at it.
func dryRunManifest(t *testing.T, entries ...string) {
	t.Helper()
	dir := t.TempDir()
	for i, entry := range entries {
		name := fmt.Sprintf("%02d%s", i, analysis.ManifestExt)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(entry), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(analysis.ToolsEnv, dir)
}

// -tool-dry-run previews the process a case's tool call would start — argv, env,
// stdin and reply — and starts nothing.
func TestToolDryRunPreviewsTheComposedInvocation(t *testing.T) {
	binary := buildCLI(t)
	record := filepath.Join(t.TempDir(), "requests.jsonl")
	t.Setenv("TOOL_STANDIN_RECORD", record)
	entry := `{"kind":"tool","toolName":"Solver","version":"2.3",` +
		`"executable":"` + dryRunStandin(t) + `","variables":["mass","tMax"],` +
		`"invocation":{"args":["solve.py","--mass","{mass}"],"env":{"SOLVER_HOME":"/opt/solver"},"stdin":"csv"},` +
		`"reply":{"format":"csv","source":"stdout","outputs":{"tMax":{"column":"tmax","type":"number","unit":"K"}}}}`
	dryRunManifest(t, entry)

	got := check(t, binary, toolCaseModel, "-tool-dry-run", "Tools::CheckHeating")
	wantReport(t, got, 0,
		"✓ Tools::CheckHeating: dry run of tool 'Solver' for Tools::Heating",
		"tool: Solver 2.3", "protocol: argv+csv/csv", "executable: "+dryRunStandin(t),
		`"solve.py"`, `"--mass"`, `"12.5"`, "SOLVER_HOME=/opt/solver", "stdin: csv",
		"  mass", "  12.5", `tMax: column "tmax", row last, type number, unit K`,
		"the process was not started")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("the tool recorded a request; a dry run must not start it")
	}
}

// Without the manifest the dry run is an unresolved verdict naming the tool and
// the variable registering it.
func TestToolDryRunWithoutAManifest(t *testing.T) {
	binary := buildCLI(t)
	t.Setenv(analysis.ToolsEnv, "")
	got := check(t, binary, toolCaseModel, "-tool-dry-run", "Tools::CheckHeating")
	wantReport(t, got, 2, "tool 'Solver' is not registered; set OPENSYSML_TOOLS")
	rejectReport(t, got, "the process was not started")
}

// An invocation naming an input the call does not send reports the same refusal
// the real run would.
func TestToolDryRunRefusesAnUnsentInput(t *testing.T) {
	binary := buildCLI(t)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + dryRunStandin(t) + `","variables":["mass","power","tMax"],` +
		`"invocation":{"args":["{power}"]}}`
	dryRunManifest(t, entry)
	got := check(t, binary, toolCaseModel, "-tool-dry-run", "Tools::CheckHeating")
	wantReport(t, got, 2, "the invocation names {power} but the call sent no value for power")
	rejectReport(t, got, "the process was not started")
}

// -help documents -tool-dry-run beside -analysis.
func TestToolDryRunIsInTheHelp(t *testing.T) {
	binary := buildCLI(t)
	out := run(t, binary, "-h")
	if !strings.Contains(out, "-tool-dry-run") {
		t.Fatalf("-help does not name -tool-dry-run:\n%s", out)
	}
}

// -tool-dry-run on an action binds the invocation's argument into the preview.
func TestToolDryRunBindsAnActionsArguments(t *testing.T) {
	binary := buildCLI(t)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + dryRunStandin(t) + `","variables":["mass","tMax"],` +
		`"invocation":{"args":["--mass","{mass}"],"stdin":"none"}}`
	dryRunManifest(t, entry)
	got := check(t, binary, toolCaseModel, "-tool-dry-run", "Tools::Heating(30)")
	wantReport(t, got, 0, "dry run of tool 'Solver' for Tools::Heating", `"--mass"`, `"30"`,
		"inputs:", "  mass = 30", "the process was not started")
}
