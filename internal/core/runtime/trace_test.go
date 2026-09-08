package runtime

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
// A case carrying a .trace.order is checked against its constraints as well, or
// instead when it owns no golden.
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
		orderPath := filepath.Join(conformanceDir, testName+".trace.order")
		expected := loadExpectedOutcome(t, conformanceDir, testName)
		if !ownsGolden(goldenPath, expected) && !fileExists(orderPath) {
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
	return fileExists(goldenPath)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
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
		t.Fatalf("%s is trace-checked but produced no trace", testName)
	}

	checkTraceOrder(t, strings.TrimSuffix(goldenPath, ".golden")+".order", trace.Entries())
	if !ownsGolden(goldenPath, expected) {
		return
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

// checkTraceOrder checks the recorded trace against the order constraints a case
// carries, if any.
func checkTraceOrder(t *testing.T, orderPath string, entries []string) {
	t.Helper()
	data, err := os.ReadFile(orderPath)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("read order constraints: %v", err)
	}
	constraints, err := parseOrderConstraints(string(data))
	if err != nil {
		t.Fatalf("%s: %v", filepath.Base(orderPath), err)
	}
	for _, violation := range orderViolations(entries, constraints) {
		t.Error(violation)
	}
}

// orderConstraint states that the label before happens before the label after.
type orderConstraint struct {
	before, after string
}

// parseOrderConstraints reads `a < b` lines; blank lines and `#` comments are skipped.
func parseOrderConstraints(text string) ([]orderConstraint, error) {
	var constraints []orderConstraint
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		before, after, ok := strings.Cut(line, "<")
		before, after = strings.TrimSpace(before), strings.TrimSpace(after)
		if !ok || before == "" || after == "" || strings.Contains(after, "<") {
			return nil, fmt.Errorf("line %d: %q is not of the form `a < b`", i+1, line)
		}
		constraints = append(constraints, orderConstraint{before, after})
	}
	if len(constraints) == 0 {
		return nil, errors.New("states no constraint")
	}
	return constraints, nil
}

// orderViolations reports every constraint the trace does not satisfy: a label
// no entry mentions, or one whose first entry is not strictly before the other's.
func orderViolations(entries []string, constraints []orderConstraint) []string {
	var violations []string
	for _, c := range constraints {
		before, okBefore := labelPosition(entries, c.before)
		after, okAfter := labelPosition(entries, c.after)
		switch {
		case !okBefore:
			violations = append(violations, fmt.Sprintf("%s < %s: no trace entry mentions %q", c.before, c.after, c.before))
		case !okAfter:
			violations = append(violations, fmt.Sprintf("%s < %s: no trace entry mentions %q", c.before, c.after, c.after))
		case before == after:
			violations = append(violations, fmt.Sprintf("%s < %s: both first appear in entry %d, %q, which leaves them unordered", c.before, c.after, before+1, entries[before]))
		case before > after:
			violations = append(violations, fmt.Sprintf("%s < %s: %q first appears in entry %d, %q in entry %d", c.before, c.after, c.before, before+1, c.after, after+1))
		}
	}
	return violations
}

// labelPosition is the index of the first entry mentioning a label: a
// performance label names a node a token holds at in a step entry, an action
// node entered or a state entered; a statement label is the text after `stmt `.
func labelPosition(entries []string, label string) (int, bool) {
	for i, entry := range entries {
		if entryMentions(entry, label) {
			return i, true
		}
	}
	return 0, false
}

func entryMentions(entry, label string) bool {
	entry = strings.TrimLeft(entry, " ")
	if rest, ok := strings.CutPrefix(entry, "step "); ok {
		_, tokens, _ := strings.Cut(rest, ": ")
		for _, token := range strings.Split(tokens, ", ") {
			if _, node, ok := strings.Cut(token, "@"); ok && node == label {
				return true
			}
		}
		return false
	}
	for _, prefix := range []string{"enter action node: ", "enter: ", "stmt "} {
		if rest, ok := strings.CutPrefix(entry, prefix); ok {
			return rest == label || strings.TrimSuffix(rest, " (entry action)") == label
		}
	}
	return false
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

// An order constraint is load-bearing: a trace that violates it, mentions one
// label only in the same step as the other, or never mentions it fails.
func TestTraceOrderViolationFails(t *testing.T) {
	entries := []string{
		"step 1: token 1@split",
		"step 2: token 2@left, token 3@right",
		"stmt assign x",
		"  eval literal 2 -> 2",
		"step 3: token 2@sync, token 3@sync",
		"enter action node: inner",
		"enter: Idle (entry action)",
	}
	tests := []struct {
		name        string
		constraints string
		violations  int
	}{
		{"satisfied", "split < left\nright < sync\n# comment\n\nleft < assign x\nsync < inner\ninner < Idle", 0},
		{"reversed", "sync < left", 1},
		{"same step is unordered", "left < right\nright < left", 2},
		{"unknown label", "split < nowhere", 1},
		{"a value is not a label", "split < literal 2", 1},
		{"a token is not a label", "token 1 < left", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constraints, err := parseOrderConstraints(tt.constraints)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := orderViolations(entries, constraints); len(got) != tt.violations {
				t.Errorf("violations = %v, want %d", got, tt.violations)
			}
		})
	}

	for _, malformed := range []string{"", "# only a comment", "left sync", "< sync", "left <", "a < b < c"} {
		if _, err := parseOrderConstraints(malformed); err == nil {
			t.Errorf("parseOrderConstraints(%q) accepted a malformed file", malformed)
		}
	}
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
