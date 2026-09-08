package runtime

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var updateTraces = flag.Bool("update-traces", false, "Update golden trace files")

// TestExecutionTrace runs golden trace tests against conformance cases.
// Traces capture step-by-step execution (token movements for actions, state
// transitions for states, parameter binding and sub-expression order for calc
// and constraint evaluation).
//
// The test owns the goldens already on disk plus the ones cases opt into with
// "trace": true; -update-traces regenerates every one and writes nothing else.
func TestExecutionTrace(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")

	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("read conformance dir: %v", err)
	}
	knownFailures := loadKnownFailures(t, conformanceDir)

	updated := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sysml") {
			continue
		}

		testName := strings.TrimSuffix(entry.Name(), ".sysml")
		if knownFailures[testName] {
			continue // the case does not execute yet, so it owns no trace
		}
		goldenPath := filepath.Join(conformanceDir, testName+".trace.golden")
		expected := loadExpectedOutcome(t, conformanceDir, testName)
		if !ownsGolden(goldenPath, expected) {
			continue
		}

		updated++
		t.Run(testName, func(t *testing.T) {
			runTraceTest(t, conformanceDir, testName, goldenPath, expected)
		})
	}

	if updated == 0 {
		t.Fatalf("no trace-checked cases in %s", conformanceDir)
	}
}

// ownsGolden reports whether the trace harness owns a golden for a case: one it
// already carries, or one the case opts into.
func ownsGolden(goldenPath string, expected ExpectedOutcome) bool {
	if expected.Trace {
		return true
	}
	_, err := os.Stat(goldenPath)
	return err == nil
}

func runTraceTest(t *testing.T, conformanceDir, testName, goldenPath string, expected ExpectedOutcome) {
	sysmlPath := filepath.Join(conformanceDir, testName+".sysml")

	// Load file
	sysmlData, err := os.ReadFile(sysmlPath)
	if err != nil {
		t.Fatalf("load source: %v", err)
	}

	// Parse and build model
	p := parser.New(source.New(sysmlPath, sysmlData))
	file := p.ParseFile()
	checkDiagnostics(t, p.Diagnostics, expected.Diagnostics)

	idx := symbols.NewIndex()
	// A case whose model names library elements — the measurement unit of a
	// quantity is one — resolves them only with the standard library indexed,
	// exactly as the conformance harness loads it.
	if expected.Libraries {
		idx = libs.NewModelIndex()
	}
	idx.AddDocument(sysmlPath, file)
	if expected.Libraries {
		idx.ExpandWildcardImports()
	}
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	ctx := NewContext(model, resolver, 10000)

	// Find behavioral symbol and execute with trace
	trace := NewTraceRecorder()
	var traceOutput string

	rootScope := idx.DocumentRoot(sysmlPath)

	// A qualified path drives the case through the behavior it names, which is how
	// one nested in a part is reached.
	var actionEntry, stateEntry string
	switch expected.Type {
	case "action":
		actionEntry = expected.Evaluate
	case "state":
		stateEntry = expected.Evaluate
	}

	// Evaluation-based cases (calc, constraint) trace through the context rather
	// than an executor, and the expected outcome supplies the calc arguments.
	switch expected.Type {
	case "calc":
		ctx.SetTrace(trace)
		calcSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)
		args := make([]Value, len(expected.Inputs))
		for i, input := range expected.Inputs {
			args[i] = expectedToRuntimeValue(t, input)
		}
		if _, err := ctx.InvokeCalc(calcSym, args, rootScope); err != nil {
			t.Fatalf("invoke calc: %v", err)
		}
		traceOutput = trace.String()
	case "calcUsage":
		// Reading every output of the usage traces one evaluation of its body,
		// which is what makes the evaluate-once guarantee visible in the golden.
		ctx.SetTrace(trace)
		usageSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)
		if _, err := ctx.CalcUsageOutputs(usageSym, usageSym.OwnerScope, nil); err != nil {
			t.Fatalf("evaluate calc usage: %v", err)
		}
		traceOutput = trace.String()
	case "analysis":
		// The run of the case traces the binding of its subject and inputs, each
		// step of its body in the order they ran, and the outputs read after.
		ctx.SetTrace(trace)
		caseSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefAnalysisCase, ast.UsageAnalysisCase)
		if _, err := ctx.RunAnalysis(caseSym, analysisArgsOf(t, ctx, idx, expected), rootScope, nil); err != nil {
			t.Fatalf("run analysis: %v", err)
		}
		traceOutput = trace.String()
	case "verification":
		// A verification case's body runs as an analysis case's does, so its
		// trace records the same binding, steps and outputs.
		ctx.SetTrace(trace)
		caseSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefVerificationCase, ast.UsageVerificationCase)
		if _, err := ctx.RunVerification(caseSym, analysisArgsOf(t, ctx, idx, expected), rootScope, nil); err != nil {
			t.Fatalf("run verification: %v", err)
		}
		traceOutput = trace.String()
	case "instance":
		// Materializing the object records its own start: the objects built, the
		// behaviors their types bind, and the bodies those behaviors run.
		ctx.SetTrace(trace)
		typeSym := oneSymbol(t, idx, expected.Instantiate)
		inst, err := ctx.Instantiate(typeSym)
		if err != nil {
			t.Fatalf("instantiate %s: %v", expected.Instantiate, err)
		}
		traceObjectRuns(t, ctx, inst, expected)
		traceOutput = trace.String()
	case "constraint":
		ctx.SetTrace(trace)
		constraintSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefConstraint, ast.UsageConstraint)
		if _, err := ctx.EvaluateConstraint(constraintSym, rootScope); err != nil {
			t.Fatalf("evaluate constraint: %v", err)
		}
		traceOutput = trace.String()
	}

	// Try action execution
	if actionSym := entryBehavior(idx, actionEntry, rootScope, ast.DefAction, ast.UsageAction); actionSym != nil {
		exec, err := ctx.CreateActionExecutor(actionSym)
		if err != nil {
			t.Fatalf("create action executor: %v", err)
		}
		exec.SetTrace(trace)

		// Drive the traced executor itself: ctx.ExecuteAction would build a
		// second, untraced one and leave the recorder empty.
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("action execution: %v", err)
		}
		traceOutput = trace.String()
	}

	// Try state execution
	if stateSym := entryBehavior(idx, stateEntry, rootScope, ast.DefState, ast.UsageState); stateSym != nil {
		exec, err := ctx.CreateStateExecutor(stateSym)
		if err != nil {
			t.Fatalf("create state executor: %v", err)
		}
		exec.SetTrace(trace)

		// The case's events drive the trace too, so ordering under an event —
		// which transition wins, and in what order states are left — is recorded
		// rather than only the initial entry.
		injectEvents(t, exec, expected.Events)

		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("state execution: %v", err)
		}
		traceOutput = trace.String()
	}
	if traceOutput == "" {
		t.Fatalf("%s has a golden trace but produced none", testName)
	}

	// Update or compare golden
	if *updateTraces {
		if err := os.WriteFile(goldenPath, []byte(traceOutput+"\n"), 0644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden trace: %s", goldenPath)
	} else {
		golden, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden: %v", err)
		}

		want := strings.TrimSpace(string(golden))
		got := strings.TrimSpace(traceOutput)

		if got != want {
			t.Errorf("trace mismatch for %s\n=== WANT ===\n%s\n=== GOT ===\n%s\n", testName, want, got)
		}
	}
}

// traceObjectRuns drives the object runs a case names, so the trace records how
// several objects' behaviors interleave rather than only the first object's start.
func traceObjectRuns(t *testing.T, ctx *Context, first *Instance, expected ExpectedOutcome) {
	t.Helper()
	if len(expected.Objects) == 0 {
		return
	}
	objects := materializations(t, ctx, first.Type, first, expected.Objects)
	for _, run := range expected.Objects {
		obj := objects[instanceIndexOf(run)]
		if run.Path != "" {
			obj = instanceAtPath(t, ctx, obj, run.Path)
		}
		exec := objectMachine(t, obj, run.Behavior)
		injectEvents(t, exec, run.Events)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run machine of object #%d: %v", obj.ID, err)
		}
	}
}

// entryBehavior returns the behavior a case is driven through: the one its
// qualified path names, or the first of that kind the document declares. It is
// nil when the model declares none, since the harness probes each kind in turn.
func entryBehavior(idx *symbols.Index, fqn string, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	if fqn == "" {
		return lookupBehavioralSymbol(scope, defKind, usageKind)
	}
	return namedSymbol(idx, fqn, defKind, usageKind)
}

// loadExpectedOutcome reads a case's conformance expectation, which tells the
// trace harness how the case is driven. A case without one is untyped and only
// the executor paths apply.
func loadExpectedOutcome(t *testing.T, conformanceDir, testName string) ExpectedOutcome {
	data, err := os.ReadFile(filepath.Join(conformanceDir, testName+".expected.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return ExpectedOutcome{}
		}
		t.Fatalf("read expected outcome: %v", err)
	}

	var expected ExpectedOutcome
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("parse expected outcome: %v", err)
	}
	return expected
}

// A trace names control nodes by what they do, so an unnamed fork or final
// node does not surface a Go type name to whoever reads the trace.
func TestNodeIdentifierNamesControlNodes(t *testing.T) {
	cases := []struct {
		node ast.Node
		want string
	}{
		{&ast.InitialNode{}, "initial"},
		{&ast.FinalNode{}, "done"},
		{&ast.ForkNode{}, "fork"},
		{&ast.JoinNode{}, "join"},
		{&ast.MergeNode{}, "merge"},
		{&ast.DecisionNode{}, "decision"},
		{&ast.ForkNode{Name: "split"}, "split"},
	}

	for _, tc := range cases {
		if got := nodeIdentifier(tc.node); got != tc.want {
			t.Errorf("nodeIdentifier(%T) = %q, want %q", tc.node, got, tc.want)
		}
	}
}
