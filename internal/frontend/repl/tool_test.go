package repl

import (
	"errors"
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
		"✓ Tools::CheckHeating: dry run of tool 'Solver' for Tools::Heating",
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

// toolSweepSource is a tool-computed action and a case whose input sweeps it.
const toolSweepSource = `package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	part def Block { attribute dummy : Real = 0.0; }
	part block : Block;
	action def Heating {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in mass : Real { @ToolVariable { name = "mass"; } }
		out tMax : Real { @ToolVariable { name = "tMax"; } }
	}
	analysis def SweptHeating {
		subject s : Block;
		in mass : Real;
		action h : Heating { in mass = mass; }
		out result : Real = h.tMax;
	}
	analysis swept : SweptHeating { subject s = block; }
}
`

// A sweep records each row's own tool calls: the run bound to 10 lists its argv
// value, the run bound to 11 lists its own, not the other's.
func TestRecordSweepWritesEachRowsOwnTools(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("the tool is a shell script")
	}
	s := loadSource(t, toolSweepSource)
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		`printf '{"outputs":{"tMax":{"value":87.2}}}\n'` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "solver.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	toolManifest(t, s, `{"kind":"tool","toolName":"Solver","executable":"`+filepath.Join(dir, "solver.sh")+`","variables":["mass","tMax"],`+
		`"invocation":{"args":["--mass","{mass}"],"stdin":"none"}}`)

	v := s.RecordSweep("Tools::swept", []string{"mass=10..11"}, "", "%record Tools::swept")
	if v.Status != VerdictHolds {
		t.Fatalf("verdict %+v", v)
	}
	var toolLines []string
	for _, line := range strings.Split(s.text(), "\n") {
		if strings.Contains(line, "tools = (") {
			toolLines = append(toolLines, line)
		}
	}
	if len(toolLines) != 2 {
		t.Fatalf("record tool lines %v, want one per row", toolLines)
	}
	if !strings.Contains(toolLines[0], "--mass 10") || strings.Contains(toolLines[0], "--mass 11") {
		t.Errorf("row 1's tools %q, want its own --mass 10 only", toolLines[0])
	}
	if !strings.Contains(toolLines[1], "--mass 11") || strings.Contains(toolLines[1], "--mass 10") {
		t.Errorf("row 2's tools %q, want its own --mass 11 only", toolLines[1])
	}
}

// %tool on a verification case previews the tool call its body reached: the dry
// run's error passes the case's verdict handling through unchanged.
func TestToolPreviewsAVerificationCasesCall(t *testing.T) {
	s := loadSource(t, `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	part def Board { attribute mass : Real; }
	part block : Board { attribute :>> mass = 12.5; }
	action def Heating {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in mass : Real { @ToolVariable { name = "mass"; } }
		out tMax : Real { @ToolVariable { name = "tMax"; } }
	}
	verification def HeatedCheck {
		subject p : Board;
		action h : Heating { in mass = p.mass; }
		VerificationCases::PassIf(h.tMax <= 400.0)
	}
	verification heated : HeatedCheck { subject p = block; }
}`)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	wantsInOrder(t, run(t, s, "%tool Tools::heated"),
		"✓ Tools::heated: dry run of tool 'Solver' for Tools::Heating",
		"inputs:", "  mass = 12.5",
		"the process was not started")
}

// A preview releases the executor it ran: another attaches and finishes the same
// way, and the session clock parks nothing of either's.
func TestToolReleasesThePreviewExecutor(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	wants(t, run(t, s, "%tool Tools::Heating"), "the process was not started")
	wants(t, run(t, s, "%tool Tools::Heating"), "the process was not started")
	if waits := s.rtCtx.Clock().Waits(); len(waits) != 0 {
		t.Fatalf("the clock parks %v after two previews, want none", waits)
	}
}

// A lines reply with a regex prints each output's regex group as the variable
// the group is named for — checkReplyLines reads no key beside a regex.
func TestToolPreviewsALinesRegexReply(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"],` +
		`"reply":{"format":"lines","regex":"T=(?P<tMax>[0-9.]+)K","outputs":{"tMax":{"type":"number","unit":"K"}}}}`
	toolManifest(t, s, entry)

	wantsInOrder(t, run(t, s, "%tool Tools::Heating"),
		`reply: lines from stdout, regex "T=(?P<tMax>[0-9.]+)K"`,
		`  tMax: regex group "tMax"`,
		"the process was not started")
}

// %tool answers as the selected engine would: under %engine run the named
// engine's refusal is reported, and the default selection previews again.
func TestToolFollowsTheEngineSelection(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	wants(t, run(t, s, "%engine run"), "engine: run")
	wants(t, run(t, s, "%tool Tools::Heating"), "run does not answer", "compute")
	wants(t, run(t, s, "%engine auto"), "engine: auto")
	wants(t, run(t, s, "%tool Tools::Heating"), "the process was not started")
}

// A file reply's source renders like argv: {outputDir} spells its name in angle
// brackets, not the raw manifest text.
func TestToolPreviewsAFileReplysRenderedSource(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"],` +
		`"invocation":{"args":["--out","{outputDir}"],"stdin":"none"},` +
		`"reply":{"format":"csv","source":"file:{outputDir}/result.csv","outputs":{"tMax":{"column":"tmax","type":"number","unit":"K"}}}}`
	toolManifest(t, s, entry)

	wantsInOrder(t, run(t, s, "%tool Tools::Heating"),
		`  "--out"`, `  "<outputDir>"`,
		"reply: csv from file:<outputDir>/result.csv",
		"the process was not started")
}

// %tool resolves a named argument as an invocation does: a redefined parameter's
// spelling binds the parameter it redefines.
func TestToolBindsARedefinedParametersName(t *testing.T) {
	s := loadSource(t, `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	action def Base {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in x : Real { @ToolVariable { name = "mass"; } }
		out tMax : Real { @ToolVariable { name = "tMax"; } }
	}
	action def Sub : Base { in attribute load :>> x; }
}`)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	wants(t, run(t, s, "%tool Tools::Sub(load=30)"), "mass = 30", "the process was not started")
}

// %tool refuses a named argument given twice, as an invocation does.
func TestToolRefusesARepeatedNamedArgument(t *testing.T) {
	s := loadSource(t, `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	action def Base {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in x : Real { @ToolVariable { name = "mass"; } }
		out tMax : Real { @ToolVariable { name = "tMax"; } }
	}
	action def Sub : Base { in attribute load :>> x; }
}`)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	out := run(t, s, "%tool Tools::Sub(load=10,load=20)")
	wants(t, out, "more than one argument")
	if strings.Contains(out, "the process was not started") {
		t.Errorf("a refused argument still previewed:\n%s", out)
	}
}

// %tool performs the case to its tool call and discards what it did: an
// attribute the body assigned on the subject is unchanged afterwards.
func TestToolLeavesTheSessionAsItFoundIt(t *testing.T) {
	s := loadSource(t, `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	action def Heating {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in mass : Real = 12.5 { @ToolVariable { name = "mass"; } }
		out tMax : Real      { @ToolVariable { name = "tMax"; } }
	}
	part def Probe {
		attribute t : Real = 3.0;
	}
	analysis def CheckProbe {
		subject analysed : Probe;
		action prep { assign analysed.t := 9.9; }
		action h : Heating;
		first prep;
		then h;
	}
}`)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)
	run(t, s, "%instantiate Tools::Probe")

	wants(t, run(t, s, "%tool Tools::CheckProbe Probe"), "the process was not started")
	wants(t, run(t, s, "%eval in Probe : t"), "3.0")
	// A following run starts the case over — the preview left nothing performed.
	wants(t, run(t, s, "%analysis Tools::CheckProbe Probe"), "analysis run failed")
	wants(t, run(t, s, "%eval in Probe : t"), "9.9")
}

// A value bound in the session before %tool is what the preview substitutes.
func TestToolPreviewsTheSessionsCurrentValues(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)

	wants(t, run(t, s, "%tool Tools::Heating"), "mass = 12.5")
}

// %tool makes and abandons objects the run created: the session's object
// listing reads the same afterwards.
func TestToolLeavesNoObjectsBehind(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)
	before := run(t, s, "%instances")

	wants(t, run(t, s, "%tool Tools::Heating"), "the process was not started")
	if got := run(t, s, "%instances"); got != before {
		t.Errorf("%%instances after %%tool = %q, want %q (the preview left its objects)", got, before)
	}
}

// A tool call an objective's condition reaches ends the run undecided, with no
// error to read — %tool still prints the preview the runner recorded.
func TestToolPreviewsACallAnObjectiveReached(t *testing.T) {
	s := loadSource(t, `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	calc def Solve {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in mass : Real { @ToolVariable { name = "mass"; } }
		return : Real { @ToolVariable { name = "out"; } }
	}
	analysis def CheckSolve {
		attribute mass : Real = 12.5;
		out x : Real = 1.0;
		objective { require constraint { Solve(mass) >= 0.0 } }
	}
}`)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","out"]}`
	toolManifest(t, s, entry)

	out := run(t, s, "%tool Tools::CheckSolve")
	wants(t, out, "dry run of tool 'Solver'", "mass = 12.5", "the process was not started")
}

// Under an exploring schedule %tool refuses: a preview shows one run's first
// call while exploration runs every linearization.
func TestToolRefusesAnExploringSchedule(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)
	if err := s.SetSchedule(mustSchedule(t, "explore")); err != nil {
		t.Fatalf("SetSchedule: %v", err)
	}

	out := run(t, s, "%tool Tools::CheckHeating")
	wants(t, out, "explores every linearization", "%schedule declared")
	if strings.Contains(out, "the process was not started") {
		t.Errorf("an exploring schedule still previewed:\n%s", out)
	}
	if err := s.SetSchedule(mustSchedule(t, "declared")); err != nil {
		t.Fatalf("SetSchedule: %v", err)
	}
	wants(t, run(t, s, "%tool Tools::CheckHeating"), "the process was not started")
}

// %tool gives the session's runner back when it ends: the runner keeps the
// reply history its divergence detection reads.
func TestToolRestoresTheSessionsToolRunner(t *testing.T) {
	s := loadSource(t, toolCaseSource)
	entry := `{"kind":"tool","toolName":"Solver","executable":"` + toolStandin(t) + `","variables":["mass","tMax"]}`
	toolManifest(t, s, entry)
	run(t, s, "%analysis Tools::CheckHeating")
	prior := s.rtCtx.ToolRunner()
	if prior == nil {
		t.Fatal("the session's analysis attached no tool runner")
	}
	wants(t, run(t, s, "%tool Tools::CheckHeating"), "the process was not started")
	if got := s.rtCtx.ToolRunner(); got != prior {
		t.Errorf("the preview left a different tool runner %T, want the session's %T", got, prior)
	}
	if _, dry := s.rtCtx.ToolRunner().(*analysis.DryRunner); dry {
		t.Error("the preview left the dry runner attached")
	}
}

// A preview a condition reached does not vouch for a run that then failed for
// another reason: the verdict is unresolved, the preview shown, the failure named.
func TestToolPreviewWithAnotherFailureIsUnresolved(t *testing.T) {
	dry := &analysis.ToolDryRunError{
		Action:  "Tools::Solve",
		Preview: analysis.DryRun{Tool: "Solver", Executable: "/bin/solver"},
	}
	v := toolPreviewVerdict("Tools::CheckSolve", dry, errors.New("output x: division by zero"))
	if v.Status != VerdictUnresolved {
		t.Fatalf("status = %v, want unresolved:\n%s", v.Status, strings.Join(v.Lines, "\n"))
	}
	out := strings.Join(v.Lines, "\n")
	wants(t, out, "? Tools::CheckSolve: dry run of tool 'Solver'", "/bin/solver", "error: output x: division by zero")

	if v := toolPreviewVerdict("Tools::CheckSolve", dry, nil); v.Status != VerdictHolds {
		t.Errorf("status without a failure = %v, want holds", v.Status)
	}
}

// %tool on an object performing the action runs that performance, as running the
// action on the object does: an already-completed one reaches no call, and
// arguments of its own are refused, the declaration binding them.
func TestToolPreviewsAnObjectsOwnPerformance(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("the tool is a shell script")
	}
	s := loadSource(t, `
package Tools {
	private import ScalarValues::Real;
	private import AnalysisTooling::*;
	action def Heating {
		metadata ToolExecution { toolName = "Solver"; uri = "solver://eq"; }
		in mass : Real default = 12.5 { @ToolVariable { name = "mass"; } }
		out tMax : Real              { @ToolVariable { name = "tMax"; } }
	}
	part def Rig {
		perform action heat : Heating { in mass = 7.0; }
	}
}`)
	dir := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\n" +
		`printf '{"outputs":{"tMax":{"value":87.2}}}\n'` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "solver.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	toolManifest(t, s, `{"kind":"tool","toolName":"Solver","executable":"`+filepath.Join(dir, "solver.sh")+`","variables":["mass","tMax"]}`)
	wants(t, run(t, s, "%instantiate Tools::Rig"), "Created instance of Tools::Rig")

	// Instantiation performed heat to completion; the preview joins it and finds no call to make.
	wants(t, run(t, s, "%tool Tools::Heating Rig"), "no ToolExecution-annotated action was reached")
	out := run(t, s, "%tool Tools::Heating(30) Rig")
	wants(t, out, "the object performs Heating already, with the arguments its declaration binds")
	if strings.Contains(out, "the process was not started") {
		t.Errorf("%%tool with arguments on the performer previewed a call:\n%s", out)
	}
	wants(t, run(t, s, "%eval in Rig : heat.tMax"), "87.2")
}
