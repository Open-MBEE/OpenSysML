package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
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

// ExpectedValue represents a typed value in expected.json
type ExpectedValue struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
	// Unit is the measurement unit a Quantity is expressed in, as written
	// ("m/s"). A quantity carries it, so a case asserting one pins that the unit
	// survived the computation rather than only the magnitude. A MeasurementRef
	// is that unit alone, with no value.
	Unit string `json:"unit,omitempty"`
	// Im is the imaginary part of a Complex, whose value is its real part.
	Im *float64 `json:"im,omitempty"`
	// Text is how a CoordinateFrame or CoordinateTransformation prints: the
	// declaration it is over its axes (`spatialCF [m, m, m]`), or the frames it relates.
	Text string `json:"text,omitempty"`
	// Elements are the members a Sequence holds, in order, for a case asserting a
	// multi-valued feature. Set it instead of value. A Vector's are its numbers, a
	// VectorQuantity's its axes as Quantity values, an Array's or a TensorQuantity's
	// its row-major elements.
	Elements []ExpectedValue `json:"elements,omitempty"`
	// Dimensions are an Array's or a TensorQuantity's, in order.
	Dimensions []int64 `json:"dimensions,omitempty"`
	// Error is the text producing this value must fail with, for a feature value whose
	// contract is a diagnostic. Set it instead of type and value.
	Error string `json:"error,omitempty"`
}

// ExpectedEvent represents an event to inject during state machine execution:
// either a signal (`signal`) or an operation invocation (`call`).
type ExpectedEvent struct {
	Signal string                   `json:"signal,omitempty"` // Signal type name
	Call   string                   `json:"call,omitempty"`   // Invoked operation name
	Args   map[string]ExpectedValue `json:"args,omitempty"`   // Signal feature bindings or call arguments
	Value  *ExpectedValue           `json:"value,omitempty"`  // The one bare value a signal carries
}

// Performer is one object performing the case's behavior, and the outcome
// expected of that object's performance.
type Performer struct {
	Object      string                   `json:"object"` // qualified name of the object's usage
	Events      []ExpectedEvent          `json:"events,omitempty"`
	FinalState  string                   `json:"finalState,omitempty"`
	StateVisits []string                 `json:"stateVisits,omitempty"`
	Outputs     map[string]ExpectedValue `json:"outputs,omitempty"`
}

// Outcome is one complete result a case admits: an action run's outputs, or a
// state performance's final state, visits and outputs.
type Outcome struct {
	Outputs     map[string]ExpectedValue `json:"outputs,omitempty"`
	FinalState  string                   `json:"finalState,omitempty"`
	StateVisits []string                 `json:"stateVisits,omitempty"`
}

// ExpectedOutcome represents expected execution result
type ExpectedOutcome struct {
	Type string `json:"type"` // "action", "state", "calc", "constraint", "requirement", "instance"
	// Libraries loads the standard library into the case's index, for a case
	// whose model names library elements the runtime resolves — the measurement
	// unit of a quantity expression is one.
	Libraries bool `json:"libraries,omitempty"`

	// Outcomes are the complete results the case admits, in place of the single
	// outputs/finalState/stateVisits, for a model whose library semantics leave
	// more than one open. The observed run must match exactly one of them.
	Outcomes []Outcome `json:"outcomes,omitempty"`
	// Admissible cites the section of docs/project/behavior-semantic-oracle.md
	// deriving that every listed outcome is valid. Required beside Outcomes.
	Admissible string `json:"admissible,omitempty"`

	// Action fields
	Outputs    map[string]ExpectedValue `json:"outputs,omitempty"`
	TokenCount *int                     `json:"tokenCount,omitempty"`
	// Error is the text the execution is expected to fail with, for a case whose
	// contract is a diagnostic rather than a result (a loop that never
	// terminates). Empty means the execution must succeed.
	Error string `json:"error,omitempty"`
	// Diagnostics are the parse diagnostics the case's model is expected to
	// report, one text per diagnostic, matched as a substring. Any diagnostic the
	// case does not declare fails it: a model that parses with an error executes
	// recovered input, which is not what the case states.
	Diagnostics []string `json:"diagnostics,omitempty"`

	// State fields
	Events      []ExpectedEvent `json:"events,omitempty"` // Events to inject
	FinalState  string          `json:"finalState,omitempty"`
	StateVisits []string        `json:"stateVisits,omitempty"`

	// Calc fields
	Inputs []ExpectedValue `json:"inputs,omitempty"`
	Result *ExpectedValue  `json:"result,omitempty"`
	// Reads are the values expressions of the model take, keyed by the qualified
	// name of the feature whose value binding is evaluated ("M::z"). A calc
	// usage's outputs are read through such features, so a case states what a
	// model reading them computes rather than only what the usage produced.
	Reads map[string]ExpectedValue `json:"reads,omitempty"`

	// Constraint/Requirement fields
	Bindings  map[string]ExpectedValue `json:"bindings,omitempty"`
	Satisfied *bool                    `json:"satisfied,omitempty"`
	// Evaluate names the element to execute or evaluate: a qualified path
	// ("test::p::a") also reaches one nested in a part. Empty searches the model.
	Evaluate string `json:"evaluate,omitempty"`
	// Trace opts a case into a golden trace it does not carry yet, so
	// -update-traces writes one. A case already carrying a golden needs no opt-in.
	Trace bool `json:"trace,omitempty"`

	// Satisfy fields: the verdict expected of each satisfaction assertion the
	// case states, keyed by the assertion as written ("satisfy r by p"), since
	// such an assertion is anonymous.
	Assertions map[string]bool `json:"assertions,omitempty"`

	// Analysis fields: the object run as the case's subject, by the qualified
	// name of its usage, and the verdict ("satisfied", "not satisfied",
	// "undecided") expected of each objective and assertion, by name.
	Subject  string            `json:"subject,omitempty"`
	Verdicts map[string]string `json:"verdicts,omitempty"`

	// Verification fields: the VerdictKind the run of the case's body produced,
	// the text an error or inconclusive verdict carries, and the verdict of each
	// verification subcase the body performs, by qualified name.
	Verdict       string            `json:"verdict,omitempty"`
	VerdictDetail string            `json:"verdictDetail,omitempty"`
	Subcases      map[string]string `json:"subcases,omitempty"`

	// Performers are the objects that each perform the case's behavior, for a
	// case whose contract depends on which object performs it — two objects
	// selecting different variants of one variation route over their own.
	Performers []Performer `json:"performers,omitempty"`

	// Instance fields
	Instantiate   string                   `json:"instantiate,omitempty"` // qualified name of the type to instantiate
	FeatureValues map[string]ExpectedValue `json:"slots,omitempty"`       // expected feature values, derived ones included
	Constraints   map[string]bool          `json:"constraints,omitempty"` // constraint feature name -> satisfied on this instance
	// Identical states pairs of paths through the instance that must reach the
	// very same object, for a case whose contract is identity rather than a
	// value — a connector end is the feature it attaches to, so
	// ["link.source", "a.p"] holds only while they are one object.
	Identical [][]string `json:"identical,omitempty"`
	// Distinct states pairs of paths that must reach different objects, for a
	// case pinning that two connectors attached to different features are
	// told apart.
	Distinct [][]string `json:"distinct,omitempty"`
	// Objects are the behaviors materialized objects run because their type
	// exhibits or performs them, one entry per object whose performance the case
	// states.
	Objects []ObjectRun `json:"objects,omitempty"`
}

// ObjectRun is the performance a case expects of one materialized object's
// exhibited machine: which object it is, the events sent to that object's
// machine, and what the machine and the object hold afterwards.
type ObjectRun struct {
	// Instance selects which materialization of the case's type the run is of,
	// counted from 1. A case naming more than one materializes that many
	// objects, which is how two objects of one type are stated to run
	// independently.
	Instance int `json:"instance,omitempty"`
	// Path reaches a nested object from that materialization by dotted feature
	// names; empty is the materialized object itself.
	Path string `json:"path,omitempty"`
	// Behavior names the behavior run, for an object running more than one.
	// Empty is the machine the object exhibits.
	Behavior    string                   `json:"behavior,omitempty"`
	Events      []ExpectedEvent          `json:"events,omitempty"`
	FinalState  string                   `json:"finalState,omitempty"`
	StateVisits []string                 `json:"stateVisits,omitempty"`
	Values      map[string]ExpectedValue `json:"slots,omitempty"`
}

// TestExecutionConformance runs all behavioral execution conformance tests.
// Each case consists of <name>.sysml and <name>.expected.json.
// Cases listed in known_failures.txt are skipped with SKIP log.
func TestExecutionConformance(t *testing.T) {
	conformanceDir := filepath.Join("testdata", "conformance")

	// Load known failures
	knownFailures := loadKnownFailures(t, conformanceDir)

	// Walk conformance directory for .expected.json files
	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("failed to read conformance directory: %v", err)
	}

	testCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".expected.json") {
			continue
		}

		caseName := strings.TrimSuffix(entry.Name(), ".expected.json")

		// Check if known failure
		if knownFailures[caseName] {
			t.Logf("SKIP %s (known failure)", caseName)
			continue
		}

		testCount++
		t.Run(caseName, func(t *testing.T) {
			t.Parallel()
			runConformanceCase(t, conformanceDir, caseName)
		})
	}

	if testCount == 0 {
		t.Fatalf("no runnable conformance cases in %s", conformanceDir)
	}
}

// loadKnownFailures reads known_failures.txt and returns set of case names to skip
func loadKnownFailures(t *testing.T, conformanceDir string) map[string]bool {
	knownPath := filepath.Join(conformanceDir, "known_failures.txt")
	data, err := os.ReadFile(knownPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no known failures file
		}
		t.Logf("warning: failed to read known_failures.txt: %v", err)
		return nil
	}

	failures := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		failures[line] = true
	}
	return failures
}

// runConformanceCase executes a single conformance test case
func runConformanceCase(t *testing.T, conformanceDir, caseName string) {
	// Load .sysml file
	sysmlPath := filepath.Join(conformanceDir, caseName+".sysml")
	sysmlData, err := os.ReadFile(sysmlPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", sysmlPath, err)
	}

	// Load .expected.json
	expectedPath := filepath.Join(conformanceDir, caseName+".expected.json")
	expectedData, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", expectedPath, err)
	}

	var expected ExpectedOutcome
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("failed to parse expected.json: %v", err)
	}
	for _, problem := range admissibleSchemaProblems(expected, oracleSectionTitles(t)) {
		t.Error(problem)
	}
	if t.Failed() {
		t.FailNow()
	}

	// Parse and build model
	p := parser.New(source.New(sysmlPath, sysmlData))
	file := p.ParseFile()
	checkDiagnostics(t, p.Diagnostics, expected.Diagnostics)

	idx := symbols.NewIndex()
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

	// Dispatch based on type
	switch expected.Type {
	case "action":
		runActionConformance(t, ctx, idx, sysmlPath, expected)
	case "state":
		runStateConformance(t, ctx, idx, sysmlPath, expected)
	case "calc":
		runCalcConformance(t, ctx, idx, sysmlPath, expected)
	case "calcUsage":
		runCalcUsageConformance(t, ctx, idx, sysmlPath, expected)
	case "constraint":
		runConstraintConformance(t, ctx, idx, sysmlPath, expected)
	case "requirement":
		runRequirementConformance(t, ctx, idx, sysmlPath, expected)
	case "satisfy":
		runSatisfyConformance(t, ctx, idx, sysmlPath, expected)
	case "analysis":
		runAnalysisConformance(t, ctx, idx, sysmlPath, expected)
	case "verification":
		runVerificationConformance(t, ctx, idx, sysmlPath, expected)
	case "instance":
		runInstanceConformance(t, ctx, idx, expected)
	default:
		t.Fatalf("unknown test type: %s", expected.Type)
	}
}

// oraclePath is the semantic oracle an admissible set must cite a section of.
const oraclePath = "../../../docs/project/behavior-semantic-oracle.md"

// oracleSectionTitles reads the section titles of the semantic oracle, which are
// the citations an admissible set may resolve to.
func oracleSectionTitles(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatalf("read the semantic oracle: %v", err)
	}
	titles := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		if title, ok := strings.CutPrefix(line, "### "); ok {
			titles[strings.TrimSpace(title)] = true
		} else if title, ok := strings.CutPrefix(line, "## "); ok {
			titles[strings.TrimSpace(title)] = true
		}
	}
	return titles
}

// admissibleSchemaProblems reports how a case misuses outcomes and admissible:
// the two go together, replace the single outcome rather than sit beside it,
// list at least two distinct results, and cite a section the oracle has.
func admissibleSchemaProblems(expected ExpectedOutcome, oracleTitles map[string]bool) []string {
	var problems []string
	if len(expected.Outcomes) == 0 {
		if expected.Admissible != "" {
			problems = append(problems, "admissible is stated without outcomes to admit")
		}
		return problems
	}
	if expected.Type != "action" && expected.Type != "state" {
		problems = append(problems, fmt.Sprintf("outcomes apply to action and state cases, not %q", expected.Type))
	}
	if expected.Outputs != nil || expected.FinalState != "" || expected.StateVisits != nil {
		problems = append(problems, "outcomes and a single outputs/finalState/stateVisits are stated together; a case uses one or the other")
	}
	if len(expected.Performers) > 0 {
		problems = append(problems, "outcomes and performers are stated together")
	}
	if len(expected.Outcomes) < 2 {
		problems = append(problems, "outcomes lists one result; state it as the single outcome")
	}
	for i, outcome := range expected.Outcomes {
		if outcome.Outputs == nil && outcome.FinalState == "" && outcome.StateVisits == nil {
			problems = append(problems, fmt.Sprintf("outcome %d states nothing", i+1))
		}
	}
	switch {
	case expected.Admissible == "":
		problems = append(problems, "outcomes are listed without admissible citing the oracle section deriving them")
	case !oracleTitles[expected.Admissible]:
		problems = append(problems, fmt.Sprintf("admissible %q is not a section title of %s", expected.Admissible, filepath.Base(oraclePath)))
	}
	return problems
}

// reporter is what a validation reports mismatches to: the test itself, or a
// problem log when the harness probes an outcome that may legitimately not match.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Logf(format string, args ...any)
}

// problemLog collects the mismatches a validation reports.
type problemLog struct {
	problems []string
}

func (l *problemLog) Helper() {}

func (l *problemLog) Errorf(format string, args ...any) {
	l.problems = append(l.problems, fmt.Sprintf(format, args...))
}

func (l *problemLog) Logf(string, ...any) {}

// matchOutcome checks that exactly one listed outcome matches the observed run:
// none means the run is inadmissible, several means the set is not distinct.
func matchOutcome(t *testing.T, outcomes []Outcome, validate func(r reporter, outcome Outcome)) {
	t.Helper()
	matched, report := matchedOutcomes(outcomes, validate)
	switch len(matched) {
	case 1:
		t.Logf("matched admissible outcome %d of %d", matched[0], len(outcomes))
	case 0:
		t.Errorf("the run matches none of the %d admissible outcomes:\n  %s", len(outcomes), strings.Join(report, "\n  "))
	default:
		t.Errorf("the run matches admissible outcomes %v; an admissible set lists distinct outcomes", matched)
	}
}

// matchedOutcomes is the 1-based positions of the outcomes the run matches, with
// the problems that rule out each of the others.
func matchedOutcomes(outcomes []Outcome, validate func(r reporter, outcome Outcome)) (matched []int, report []string) {
	for i, outcome := range outcomes {
		log := &problemLog{}
		validate(log, outcome)
		if len(log.problems) == 0 {
			matched = append(matched, i+1)
			continue
		}
		report = append(report, fmt.Sprintf("outcome %d: %s", i+1, strings.Join(log.problems, "; ")))
	}
	return matched, report
}

// checkDiagnostics fails the case unless the diagnostics its model reported are
// exactly the ones it declares: an undeclared diagnostic means the case executes
// recovered input, and a declared one that no diagnostic matches is stale.
func checkDiagnostics(t *testing.T, got []parser.Diagnostic, want []string) {
	t.Helper()
	for _, problem := range diagnosticProblems(got, want) {
		t.Error(problem)
	}
}

// diagnosticProblems reports how the diagnostics a model produced differ from
// the ones a case declares, pairing each declaration with one diagnostic.
func diagnosticProblems(got []parser.Diagnostic, want []string) []string {
	var problems []string
	matched := make([]bool, len(want))
	for _, d := range got {
		found := false
		for i, w := range want {
			if !matched[i] && strings.Contains(d.Message, w) {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			problems = append(problems, fmt.Sprintf("undeclared diagnostic at offset %d: %s", d.Span.Offset, d.Message))
		}
	}
	for i, w := range want {
		if !matched[i] {
			problems = append(problems, fmt.Sprintf("declared diagnostic %q was not reported", w))
		}
	}
	return problems
}

// TestMain gives the package a primed library cache of its own, so a case sees
// the restored symbol shape whatever the machine's cache holds or runs first.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "opensysml-runtime-libs")
	if err != nil {
		fmt.Fprintf(os.Stderr, "library cache directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv("XDG_CACHE_HOME", dir); err != nil {
		fmt.Fprintf(os.Stderr, "library cache directory: %v\n", err)
		os.Exit(1)
	}
	if err := primeLibraryCache(); err != nil {
		fmt.Fprintf(os.Stderr, "prime library cache: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// primeLibraryCache parses the standard library once and caches its records,
// persisting only once every file is indexed and its imports are expanded.
func primeLibraryCache() error {
	src := libs.DefaultSource()
	cache, err := libs.NewCache()
	if err != nil {
		return err
	}
	if err := libs.NewLoader(src, cache).LoadAll(symbols.NewIndex()); err != nil {
		return fmt.Errorf("load the library: %w", err)
	}
	return nil
}

// loadLibraries loads the standard library into idx, for a case that names its
// elements. The cache TestMain primed makes every load a hit, so what a case
// sees does not depend on what ran before it.
func loadLibraries(t *testing.T, idx *symbols.Index) {
	t.Helper()
	src := libs.DefaultSource()
	cache, err := libs.NewCache()
	if err != nil {
		t.Fatalf("library cache: %v", err)
	}
	if err := libs.NewLoader(src, cache).LoadAll(idx); err != nil {
		t.Fatalf("load the library: %v", err)
	}
}

// A loaded library carries its parsed declaration on every path, with the cache
// restoring only the facts derived from it.
func TestLoadLibrariesKeepsDeclarationsAndRestoresFacts(t *testing.T) {
	idx := symbols.NewIndex()
	loadLibraries(t, idx)

	matches := idx.LookupQualified("SI::metre")
	if len(matches) != 1 {
		t.Fatalf("SI::metre matched %d symbols, want 1", len(matches))
	}
	sym := matches[0]
	if sym.Decl == nil {
		t.Error("SI::metre carries no declaration, want the one it was parsed from")
	}
	if sym.Name != "metre" {
		t.Errorf("SI::metre is named %q, want its declared name", sym.Name)
	}
	if sym.Facts == nil || sym.Facts.Unit == nil {
		t.Errorf("SI::metre has facts %+v, want its unit facts restored", sym.Facts)
	}
}

// runActionConformance executes action and validates outputs
func runActionConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	rootScope := idx.DocumentRoot(path)
	actionSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefAction, ast.UsageAction)

	// Execute action
	outputs, err := ctx.ExecuteAction(actionSym)
	if expected.Error != "" {
		if err == nil {
			t.Fatalf("expected execution to fail with %q, it completed with outputs %v", expected.Error, outputs)
		}
		if !strings.Contains(err.Error(), expected.Error) {
			t.Fatalf("execution failed with %q, want an error containing %q", err, expected.Error)
		}
		return
	}
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}

	validateOutputs(t, ctx, expected.Outputs, outputs)
	if len(expected.Outcomes) > 0 {
		matchOutcome(t, expected.Outcomes, func(r reporter, outcome Outcome) {
			validateOutputs(r, ctx, outcome.Outputs, outputs)
		})
	}

	// Optional: validate token count
	if expected.TokenCount != nil {
		// Token count validation requires instrumentation in executor
		// For now, skip - can add later if needed
		t.Logf("token count validation not yet implemented (expected %d)", *expected.TokenCount)
	}
}

// runStateConformance executes a state machine and validates the final state. A
// case naming performers runs the machine once per object performing it, each
// against the outcome that object expects.
func runStateConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	rootScope := idx.DocumentRoot(path)
	stateSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefState, ast.UsageState)
	if len(expected.Performers) == 0 {
		runOneStatePerformance(t, ctx, stateSym, nil, expected)
		return
	}
	for _, performer := range expected.Performers {
		self, err := ctx.Instantiate(oneSymbol(t, idx, performer.Object))
		if err != nil {
			t.Fatalf("instantiate %s: %v", performer.Object, err)
		}
		t.Run(performer.Object, func(t *testing.T) {
			runOneStatePerformance(t, ctx, stateSym, self, ExpectedOutcome{
				Events:      performer.Events,
				FinalState:  performer.FinalState,
				StateVisits: performer.StateVisits,
				Outputs:     performer.Outputs,
			})
		})
	}
}

// injectEvents queues the events a case declares onto an executor, for the
// conformance and trace harnesses to drive the same performance.
func injectEvents(t *testing.T, exec *StateExecutor, events []ExpectedEvent) {
	t.Helper()
	for _, event := range events {
		args := make(map[string]Value, len(event.Args))
		for name, val := range event.Args {
			args[name] = expectedToRuntimeValue(t, val)
		}
		switch {
		case event.Call != "" && event.Signal != "":
			t.Fatalf("event declares both signal %q and call %q", event.Signal, event.Call)
		case event.Value != nil && (event.Call != "" || len(args) > 0):
			t.Fatalf("event %s%s carries a bare value beside its arguments", event.Signal, event.Call)
		case event.Call != "":
			exec.InvokeOperation(event.Call, args)
		case event.Value != nil:
			value := expectedToRuntimeValue(t, *event.Value)
			exec.enqueueSignal(Message{SignalType: event.Signal, Value: &value})
		case event.Signal != "":
			exec.SendSignal(event.Signal, args)
		default:
			t.Fatalf("event declares neither a signal nor a call")
		}
	}
}

// runOneStatePerformance runs one performance of a state machine, by self or by
// no object, and validates it against the outcome expected of that performance.
func runOneStatePerformance(t *testing.T, ctx *Context, stateSym *symbols.Symbol, self *Instance, expected ExpectedOutcome) {
	// Create executor manually to inject events
	exec, err := newStateExecutor(ctx, stateSym, self)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}

	// Initialize (enters initial state)
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize state machine: %v", err)
	}

	injectEvents(t, exec, expected.Events)

	// Process events until completion or suspension, through the executor's own
	// loop: a harness-local copy drifts from the semantics under test.
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run state machine: %v", err)
	}

	validateStateOutcome(t, ctx, exec, Outcome{
		Outputs:     expected.Outputs,
		FinalState:  expected.FinalState,
		StateVisits: expected.StateVisits,
	})
	if len(expected.Outcomes) > 0 {
		matchOutcome(t, expected.Outcomes, func(r reporter, outcome Outcome) {
			validateStateOutcome(r, ctx, exec, outcome)
		})
	}
}

// validateStateOutcome checks a state performance against one outcome.
func validateStateOutcome(r reporter, ctx *Context, exec *StateExecutor, outcome Outcome) {
	r.Helper()
	validateFinalState(r, exec, outcome.FinalState)
	validateStateVisits(r, exec, outcome.StateVisits)
	if outcome.Outputs != nil {
		validateOutputs(r, ctx, outcome.Outputs, exec.StateData())
	}
}

// validateOutputs checks each output a case states against the value produced.
func validateOutputs(r reporter, ctx *Context, want map[string]ExpectedValue, got map[string]Value) {
	r.Helper()
	for name, expectedVal := range want {
		actual, ok := got[name]
		if !ok {
			r.Errorf("missing output %q", name)
			continue
		}
		validateValue(r, ctx, name, expectedVal, actual)
	}
}

// validateFinalState checks the state a machine came to rest in, named as the
// case names it: orthogonal regions as "State1+State2", sorted by region name.
func validateFinalState(t reporter, exec *StateExecutor, want string) {
	t.Helper()
	if want == "" {
		return
	}
	got := finalStateName(t, exec)
	if got == "" {
		t.Errorf("expected finalState %q, got empty", want)
	} else if got != want {
		t.Errorf("finalState mismatch: expected %q, got %q", want, got)
	}
}

// finalStateName is the active configuration as a case writes it.
func finalStateName(t reporter, exec *StateExecutor) string {
	t.Helper()
	if len(exec.activeConfig.regionStates) > 0 {
		type regionStatePair struct {
			regionName string
			stateName  string
		}
		var pairs []regionStatePair
		for region, regionState := range exec.activeConfig.regionStates {
			pairs = append(pairs, regionStatePair{region.Name, regionState.Name})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].regionName < pairs[j].regionName })
		var names []string
		for _, pair := range pairs {
			names = append(names, pair.stateName)
		}
		return strings.Join(names, "+")
	}
	current := exec.CurrentState()
	if current == nil {
		return ""
	}
	stateNode, ok := current.(*ast.StateNode)
	if !ok {
		t.Errorf("expected StateNode, got %T", current)
		return ""
	}
	return stateNode.Name
}

// validateStateVisits checks the states a machine entered, in order.
func validateStateVisits(t reporter, exec *StateExecutor, want []string) {
	t.Helper()
	if len(want) == 0 {
		return
	}
	got := exec.GetStateVisits()
	if len(got) != len(want) {
		t.Errorf("stateVisits length mismatch: expected %d, got %d", len(want), len(got))
		t.Logf("  expected: %v", want)
		t.Logf("  actual:   %v", got)
		return
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("stateVisits[%d] mismatch: expected %q, got %q", i, name, got[i])
		}
	}
}

// runCalcConformance invokes calc and validates result
func runCalcConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find calc definition/usage
	rootScope := idx.DocumentRoot(path)
	calcSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)

	// Convert expected inputs to runtime Values
	args := make([]Value, len(expected.Inputs))
	for i, input := range expected.Inputs {
		args[i] = expectedToRuntimeValue(t, input)
	}

	// Invoke calc
	result, err := ctx.InvokeCalc(calcSym, args, rootScope)
	if expected.Error != "" {
		requireError(t, "InvokeCalc", err, expected.Error)
		return
	}
	if err != nil {
		t.Fatalf("InvokeCalc failed: %v", err)
	}

	// Validate result
	if expected.Result != nil {
		validateValue(t, ctx, "result", *expected.Result, result)
	}
}

// runCalcUsageConformance evaluates a calc usage and validates the value of each
// output feature it computes, plus the value of every model feature the case
// reads those outputs through.
func runCalcUsageConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	rootScope := idx.DocumentRoot(path)
	usageSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefCalc, ast.UsageCalc)

	outputs, err := ctx.CalcUsageOutputs(usageSym, usageSym.OwnerScope, nil)
	if expected.Error != "" {
		requireError(t, "CalcUsageOutputs", err, expected.Error)
		return
	}
	if err != nil {
		t.Fatalf("CalcUsageOutputs(%s) failed: %v", ctx.qualifiedSymbolName(usageSym), err)
	}

	values := make(map[string]Value, len(outputs))
	for _, out := range outputs {
		values[out.Name] = out.Value
	}
	for name, expectedVal := range expected.Outputs {
		actual, ok := values[name]
		if !ok {
			t.Errorf("missing output %q among %v", name, outputNames(outputs))
			continue
		}
		validateValue(t, ctx, name, expectedVal, actual)
	}

	for name, expectedVal := range expected.Reads {
		validateRead(t, ctx, idx, name, expectedVal)
	}

	// A calc that designates a result also answers an invocation with it, so a
	// case may state both what the usage's outputs are and what invoking it
	// yields — a statement-bodied calc has to get both right.
	if expected.Result != nil {
		args := make([]Value, len(expected.Inputs))
		for i, input := range expected.Inputs {
			args[i] = expectedToRuntimeValue(t, input)
		}
		result, err := ctx.InvokeCalc(usageSym, args, rootScope)
		if err != nil {
			t.Fatalf("InvokeCalc(%s) failed: %v", ctx.qualifiedSymbolName(usageSym), err)
		}
		validateValue(t, ctx, "result", *expected.Result, result)
	}
}

// validateRead evaluates the value binding of the named feature, in the scope it
// is written in, and validates the value it takes.
func validateRead(t *testing.T, ctx *Context, idx *symbols.Index, name string, expected ExpectedValue) {
	t.Helper()
	matches := idx.LookupQualified(name)
	if len(matches) != 1 {
		t.Errorf("read %q: %d matching symbols, want 1", name, len(matches))
		return
	}
	usage, ok := matches[0].Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Errorf("read %q: the feature binds no value to evaluate", name)
		return
	}
	value, err := ctx.EvalWithScope(usage.Value, matches[0].OwnerScope)
	if expected.Error != "" {
		requireError(t, "read "+name, err, expected.Error)
		return
	}
	if err != nil {
		t.Errorf("read %q: %v", name, err)
		return
	}
	validateValue(t, ctx, name, expected, value)
}

// outputNames names the outputs an evaluation produced, for a message about one
// it did not.
func outputNames(outputs []CalcOutputValue) []string {
	names := make([]string, 0, len(outputs))
	for _, out := range outputs {
		names = append(names, out.Name)
	}
	return names
}

// runConstraintConformance evaluates constraint and validates satisfaction
func runConstraintConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find constraint definition/usage
	rootScope := idx.DocumentRoot(path)
	constraintSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefConstraint, ast.UsageConstraint)

	// An object the case materializes first is what the check is about, for a
	// case whose contract is the subject the runtime picks.
	if expected.Instantiate != "" {
		if _, err := ctx.Instantiate(oneSymbol(t, idx, expected.Instantiate)); err != nil {
			t.Fatalf("instantiate %s: %v", expected.Instantiate, err)
		}
	}

	// Apply bindings to context (if any)
	if expected.Bindings != nil {
		// Bindings need to be added to scope or instance
		// For now, assume bindings are already in model
		t.Logf("constraint bindings application not yet implemented")
	}

	// Evaluate constraint. A violated assertion is a verdict, not a failure.
	satisfied, err := ctx.EvaluateConstraint(constraintSym, constraintSym.OwnerScope)
	if expected.Error != "" {
		requireError(t, "EvaluateConstraint", err, expected.Error)
		return
	}
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateConstraint failed: %v", err)
	}

	// Validate satisfaction
	if expected.Satisfied != nil {
		if satisfied != *expected.Satisfied {
			t.Errorf("constraint satisfied = %v, want %v", satisfied, *expected.Satisfied)
		}
	}
}

// runRequirementConformance evaluates requirement and validates satisfaction
func runRequirementConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	// Find requirement definition/usage
	rootScope := idx.DocumentRoot(path)
	reqSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefRequirement, ast.UsageRequirement)

	// Evaluate requirement using symbol's defining scope (where sibling features
	// visible). A violated condition is a verdict, not a failure.
	satisfied, err := ctx.EvaluateRequirement(reqSym, reqSym.OwnerScope)
	if err != nil && !errors.Is(err, ErrViolated) {
		t.Fatalf("EvaluateRequirement failed: %v", err)
	}

	// Validate satisfaction
	if expected.Satisfied != nil {
		if satisfied != *expected.Satisfied {
			t.Errorf("requirement satisfied = %v, want %v", satisfied, *expected.Satisfied)
		}
	}
}

// runAnalysisConformance runs the analysis case the outcome names: on the
// object `subject` names, with `inputs` as positional and `bindings` as named
// arguments, and checks its outputs, its verdicts and the model's reads of it.
func runAnalysisConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	rootScope := idx.DocumentRoot(path)
	caseSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefAnalysisCase, ast.UsageAnalysisCase)

	result, err := ctx.RunAnalysis(caseSym, analysisArgsOf(t, ctx, idx, expected), rootScope, nil)
	if expected.Error != "" {
		requireError(t, "RunAnalysis", err, expected.Error)
		return
	}
	if err != nil {
		t.Fatalf("RunAnalysis(%s) failed: %v", ctx.qualifiedSymbolName(caseSym), err)
	}

	checkCaseRun(t, ctx, idx, result, expected)
}

// checkCaseRun validates what one run of a case produced: its outputs, its
// result, the verdict of every objective and assertion, and the values a model
// reading its outputs computes. Analysis and verification cases run the same
// body, so their outcomes are checked the same way.
func checkCaseRun(t *testing.T, ctx *Context, idx *symbols.Index, result AnalysisResult, expected ExpectedOutcome) {
	t.Helper()
	values := make(map[string]Value, len(result.Outputs))
	for _, out := range result.Outputs {
		values[out.Name] = out.Value
	}
	for name, expectedVal := range expected.Outputs {
		actual, ok := values[name]
		if !ok {
			t.Errorf("missing output %q among %v", name, outputNames(result.Outputs))
			continue
		}
		validateValue(t, ctx, name, expectedVal, actual)
	}
	if expected.Result != nil {
		actual, ok := values[resultOutputName]
		if !ok {
			t.Errorf("the case returned no result; its outputs are %v", outputNames(result.Outputs))
		} else {
			validateValue(t, ctx, resultOutputName, *expected.Result, actual)
		}
	}

	verdicts := make(map[string]AnalysisVerdict, len(result.Verdicts))
	for _, verdict := range result.Verdicts {
		verdicts[verdict.Name] = verdict
	}
	for name, want := range expected.Verdicts {
		verdict, ok := verdicts[name]
		if !ok {
			t.Errorf("no verdict for %q among %v", name, verdictNames(result.Verdicts))
			continue
		}
		if got := verdict.Status.String(); got != want {
			t.Errorf("%s %s: verdict %q (%s), want %q", verdict.Kind, name, got, verdict.Detail, want)
		}
	}
	if len(result.Verdicts) != len(expected.Verdicts) {
		t.Errorf("the case reported %d verdict(s) %v, the outcome states %d",
			len(result.Verdicts), verdictNames(result.Verdicts), len(expected.Verdicts))
	}

	for name, expectedVal := range expected.Reads {
		validateRead(t, ctx, idx, name, expectedVal)
	}
}

// runVerificationConformance runs a verification case and validates the verdict
// its body produced, the verdict of each subcase it performs, and everything the
// same run reports as an analysis case's run does.
func runVerificationConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	rootScope := idx.DocumentRoot(path)
	caseSym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefVerificationCase, ast.UsageVerificationCase)

	result, err := ctx.RunVerification(caseSym, analysisArgsOf(t, ctx, idx, expected), rootScope, nil)
	if err != nil {
		t.Fatalf("RunVerification(%s) failed: %v", ctx.qualifiedSymbolName(caseSym), err)
	}
	if got := string(result.Verdict.Kind); got != expected.Verdict {
		t.Errorf("verdict = %q (%s), want %q", got, result.Verdict.Detail, expected.Verdict)
	}
	if expected.VerdictDetail != "" && !strings.Contains(result.Verdict.Detail, expected.VerdictDetail) {
		t.Errorf("verdict detail = %q, want it to contain %q", result.Verdict.Detail, expected.VerdictDetail)
	}
	checkSubcaseVerdicts(t, result.Subcases, expected.Subcases)
	if result.Verdict.Kind == VerdictError {
		return
	}
	checkCaseRun(t, ctx, idx, result.Run, expected)
}

// checkSubcaseVerdicts validates the verdict of each verification subcase the
// case performs, which the library states no roll-up for.
func checkSubcaseVerdicts(t *testing.T, got []VerificationVerdict, want map[string]string) {
	t.Helper()
	kinds := make(map[string]string, len(got))
	for _, verdict := range got {
		kinds[verdict.Case] = string(verdict.Kind)
	}
	for name, wantKind := range want {
		kind, ok := kinds[name]
		if !ok {
			t.Errorf("no subcase verdict for %q among %v", name, kinds)
			continue
		}
		if kind != wantKind {
			t.Errorf("subcase %s verdict = %q, want %q", name, kind, wantKind)
		}
	}
	if len(got) != len(want) {
		t.Errorf("the case reported %d subcase verdict(s) %v, the outcome states %d", len(got), kinds, len(want))
	}
}

// analysisArgsOf builds the arguments an analysis outcome states: the object its
// subject names, materialized, and its positional and named parameter values.
func analysisArgsOf(t *testing.T, ctx *Context, idx *symbols.Index, expected ExpectedOutcome) AnalysisArgs {
	t.Helper()
	args := AnalysisArgs{}
	if expected.Subject != "" {
		subject, err := ctx.Instantiate(oneSymbol(t, idx, expected.Subject))
		if err != nil {
			t.Fatalf("Instantiate(%s) failed: %v", expected.Subject, err)
		}
		args.Subject = subject
	}
	for _, input := range expected.Inputs {
		args.Positional = append(args.Positional, expectedToRuntimeValue(t, input))
	}
	if len(expected.Bindings) > 0 {
		args.Named = make(map[string]Value, len(expected.Bindings))
		for name, value := range expected.Bindings {
			args.Named[name] = expectedToRuntimeValue(t, value)
		}
	}
	return args
}

func verdictNames(verdicts []AnalysisVerdict) []string {
	names := make([]string, len(verdicts))
	for i, verdict := range verdicts {
		names[i] = verdict.Name
	}
	return names
}

// runSatisfyConformance evaluates the satisfaction assertions the case states
// and validates the verdict of each: the assertion binds the requirement's
// subject to the object its `by` operand names, so the verdict is about that
// object's values.
func runSatisfyConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	scope := idx.DocumentRoot(path)
	if expected.Evaluate != "" {
		matches := idx.LookupQualified(expected.Evaluate)
		if len(matches) != 1 {
			t.Fatalf("evaluate %q: %d matching symbols, want 1", expected.Evaluate, len(matches))
		}
		if matches[0].Scope == nil {
			t.Fatalf("evaluate %q: the element owns no scope, so it states no assertion", expected.Evaluate)
		}
		scope = matches[0].Scope
	}

	assertions := ctx.SatisfyAssertionsIn(scope)
	if len(assertions) == 0 {
		t.Fatalf("no satisfaction assertion found")
	}
	if expected.Error != "" || expected.Satisfied != nil {
		if len(assertions) != 1 {
			t.Fatalf("%d satisfaction assertions found, want 1: name each verdict under \"assertions\"", len(assertions))
		}
	}

	verdicts := make(map[string]bool, len(assertions))
	for _, a := range assertions {
		satisfied, err := ctx.EvaluateSatisfaction(a)
		if expected.Error != "" {
			if err == nil {
				t.Fatalf("expected evaluation to fail with %q, it reported satisfied = %v", expected.Error, satisfied)
			}
			if !strings.Contains(err.Error(), expected.Error) {
				t.Fatalf("evaluation failed with %q, want an error containing %q", err, expected.Error)
			}
			return
		}
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Fatalf("EvaluateSatisfaction(%s) failed: %v", a.Text(), err)
		}
		verdicts[a.Text()] = satisfied
	}

	if expected.Satisfied != nil {
		for text, satisfied := range verdicts {
			if satisfied != *expected.Satisfied {
				t.Errorf("%s: satisfied = %v, want %v", text, satisfied, *expected.Satisfied)
			}
		}
	}
	for text, want := range expected.Assertions {
		satisfied, ok := verdicts[text]
		if !ok {
			t.Errorf("no assertion %q among %v", text, sortedKeys(verdicts))
			continue
		}
		if satisfied != want {
			t.Errorf("%s: satisfied = %v, want %v", text, satisfied, want)
		}
	}
}

// sortedKeys returns the keys of m in order, for a deterministic message.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// runInstanceConformance instantiates a type and validates the values its feature values
// hold, including derived defaults, plus the verdict of each constraint the
// instance carries.
func runInstanceConformance(t *testing.T, ctx *Context, idx *symbols.Index, expected ExpectedOutcome) {
	if expected.Instantiate == "" {
		t.Fatalf("instance case declares no \"instantiate\" type")
	}
	matches := idx.LookupQualified(expected.Instantiate)
	if len(matches) != 1 {
		t.Fatalf("instantiate %q: %d matching symbols, want 1", expected.Instantiate, len(matches))
	}
	typeSym := matches[0]

	inst, err := ctx.Instantiate(typeSym)
	if expected.Error != "" {
		requireError(t, "Instantiate("+expected.Instantiate+")", err, expected.Error)
		return
	}
	if err != nil {
		t.Fatalf("Instantiate(%s) failed: %v", expected.Instantiate, err)
	}

	for name, expectedVal := range expected.FeatureValues {
		// A slot is read as an expression reads it: an optional one holding
		// nothing is the empty sequence, a required one an error.
		var value Value
		fv, err := featureValueAtPath(t, ctx, inst, name)
		if err == nil {
			value, err = fv.ReadValue(name)
		}
		if expectedVal.Error != "" {
			requireError(t, "feature value "+name, err, expectedVal.Error)
			continue
		}
		if err != nil {
			t.Errorf("feature value %q: %v", name, err)
			continue
		}
		validateValue(t, ctx, name, expectedVal, value)
	}

	validateIdentity(t, ctx, inst, expected)
	validateObjectRuns(t, ctx, typeSym, inst, expected)

	for name, wantSatisfied := range expected.Constraints {
		feat := featureNamed(ctx, typeSym, name)
		if feat == nil || feat.Symbol == nil {
			t.Errorf("constraint %q: no such feature on %s", name, expected.Instantiate)
			continue
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Errorf("constraint %q: %v", name, err)
			continue
		}
		if satisfied != wantSatisfied {
			t.Errorf("constraint %q: satisfied = %v, want %v", name, satisfied, wantSatisfied)
		}
	}
}

// featureValueAtPath reads a slot on the instance or on a nested object.
func featureValueAtPath(t *testing.T, ctx *Context, inst *Instance, path string) (*FeatureValue, error) {
	t.Helper()
	current := inst
	parts := strings.Split(path, ".")
	for i, name := range parts {
		fv, err := current.GetFeatureValue(ctx, name)
		if err != nil || i == len(parts)-1 {
			return fv, err
		}
		id, ok := fv.HeldValue().Object()
		if !ok {
			return nil, fmt.Errorf("%s: %q holds %s, want an object", path, name, fv.HeldValue().Kind)
		}
		next, ok := ctx.Instance(id)
		if !ok {
			return nil, fmt.Errorf("%s: object %d is not materialized", path, id)
		}
		current = next
	}
	return nil, fmt.Errorf("empty feature path %q", path)
}

// validateIdentity checks the identity a case states between the objects two
// paths through the instance reach, which is what a connector end asserts: it
// is the connected feature, not a copy of it.
func validateIdentity(t *testing.T, ctx *Context, inst *Instance, expected ExpectedOutcome) {
	t.Helper()
	for _, pair := range expected.Identical {
		left, right := identityPair(t, ctx, inst, pair)
		if left != right {
			t.Errorf("%s is object %d and %s is object %d, want one object",
				pair[0], left, pair[1], right)
		}
	}
	for _, pair := range expected.Distinct {
		left, right := identityPair(t, ctx, inst, pair)
		if left == right {
			t.Errorf("%s and %s are both object %d, want different objects",
				pair[0], pair[1], left)
		}
	}
}

// validateObjectRuns drives the behaviors materialized objects run because
// their type exhibits them, and checks each object's own machine and values: a
// case naming several materializations states that they run independently.
func validateObjectRuns(t *testing.T, ctx *Context, typeSym *symbols.Symbol, first *Instance, expected ExpectedOutcome) {
	t.Helper()
	if len(expected.Objects) == 0 {
		return
	}
	objects := materializations(t, ctx, typeSym, first, expected.Objects)
	for _, run := range expected.Objects {
		obj := objects[instanceIndexOf(run)]
		if run.Path != "" {
			obj = instanceAtPath(t, ctx, obj, run.Path)
		}
		t.Run(runName(run), func(t *testing.T) {
			exec := objectMachine(t, obj, run.Behavior)
			injectEvents(t, exec, run.Events)
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("run machine of object #%d: %v", obj.ID, err)
			}
			validateFinalState(t, exec, run.FinalState)
			validateStateVisits(t, exec, run.StateVisits)
			for name, want := range run.Values {
				fv, err := featureValueAtPath(t, ctx, obj, name)
				if err != nil {
					t.Errorf("feature value %q of object #%d: %v", name, obj.ID, err)
					continue
				}
				validateValue(t, ctx, name, want, fv.HeldValue())
			}
		})
	}
}

// materializations returns the objects the runs name, materializing as many of
// the case's type as the highest instance number asks for.
func materializations(t *testing.T, ctx *Context, typeSym *symbols.Symbol, first *Instance, runs []ObjectRun) []*Instance {
	t.Helper()
	count := 1
	for _, run := range runs {
		if n := instanceIndexOf(run) + 1; n > count {
			count = n
		}
	}
	objects := []*Instance{first}
	for i := 1; i < count; i++ {
		next, err := ctx.Instantiate(typeSym)
		if err != nil {
			t.Fatalf("instantiate object %d: %v", i+1, err)
		}
		objects = append(objects, next)
	}
	return objects
}

// instanceIndexOf is the zero-based materialization a run names.
func instanceIndexOf(run ObjectRun) int {
	if run.Instance <= 0 {
		return 0
	}
	return run.Instance - 1
}

// runName names a run in subtest output.
func runName(run ObjectRun) string {
	name := fmt.Sprintf("object%d", instanceIndexOf(run)+1)
	if run.Path != "" {
		name += "." + run.Path
	}
	if run.Behavior != "" {
		name += "/" + run.Behavior
	}
	return name
}

// objectMachine is the executor of the machine a run names on an object: the one
// it exhibits when the run names none.
func objectMachine(t *testing.T, obj *Instance, name string) *StateExecutor {
	t.Helper()
	behavior, ok := obj.ExhibitedState()
	if name != "" {
		behavior, ok = obj.Behavior(name)
	}
	if !ok {
		t.Fatalf("object #%d runs no behavior %q", obj.ID, name)
	}
	if behavior.State == nil {
		t.Fatalf("behavior %q of object #%d is not a state machine", behavior.Name, obj.ID)
	}
	return behavior.State
}

// instanceAtPath walks a dotted path of feature names from inst to the object it
// reaches.
func instanceAtPath(t *testing.T, ctx *Context, inst *Instance, path string) *Instance {
	t.Helper()
	id := objectAtPath(t, ctx, inst, path)
	obj, held := ctx.Instance(id)
	if !held {
		t.Fatalf("%s names object %d, which the context does not hold", path, id)
	}
	return obj
}

// identityPair resolves the two paths of an identity assertion to the objects
// they reach.
func identityPair(t *testing.T, ctx *Context, inst *Instance, pair []string) (int64, int64) {
	t.Helper()
	if len(pair) != 2 {
		t.Fatalf("identity assertion %v names %d paths, want two", pair, len(pair))
	}
	return objectAtPath(t, ctx, inst, pair[0]), objectAtPath(t, ctx, inst, pair[1])
}

// objectAtPath walks a dotted path of feature names from inst and returns the
// object it reaches. A numeric segment indexes into the collection the feature
// before it holds, counted from 1 ("units.2").
func objectAtPath(t *testing.T, ctx *Context, inst *Instance, path string) int64 {
	t.Helper()
	cur := inst
	segments := strings.Split(path, ".")
	for i := 0; i < len(segments); i++ {
		name := segments[i]
		fv, err := cur.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("%s: feature value %q: %v", path, name, err)
		}
		held := fv.HeldValue()
		if i+1 < len(segments) {
			if n, numErr := strconv.Atoi(segments[i+1]); numErr == nil {
				elements := heldElements(held)
				if n < 1 || n > len(elements) {
					t.Fatalf("%s: %q holds %d elements, index %d is out of range", path, name, len(elements), n)
				}
				held = elements[n-1]
				i++
			}
		}
		id, isObject := held.Object()
		if !isObject {
			t.Fatalf("%s: %q holds %s, want an object", path, name, held.Kind)
		}
		if i == len(segments)-1 {
			return id
		}
		next, isHeld := ctx.Instance(id)
		if !isHeld {
			t.Fatalf("%s: %q names object %d, which the context does not hold", path, name, id)
		}
		cur = next
	}
	t.Fatalf("identity path is empty")
	return 0
}

// requireError checks that what a case states as a diagnostic contract failed
// with the text it expects, since a case whose subject cannot be evaluated says
// so rather than asserting a value.
func requireError(t *testing.T, what string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected an error containing %q, it succeeded", what, want)
		return
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("%s: failed with %q, want an error containing %q", what, err, want)
	}
}

// featureNamed returns the effective feature of typeSym called name, or nil.
func featureNamed(ctx *Context, typeSym *symbols.Symbol, name string) *EffectiveFeature {
	features := ctx.FeaturesOf(typeSym)
	for i := range features {
		if features[i].Name == name {
			return &features[i]
		}
	}
	return nil
}

// findBehavioralSymbol searches for first symbol matching defKind or usageKind,
// failing the test when there is none.
func findBehavioralSymbol(t *testing.T, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	sym := lookupBehavioralSymbol(scope, defKind, usageKind)
	if sym == nil {
		t.Fatalf("no behavioral symbol found (defKind=%v, usageKind=%v)", defKind, usageKind)
	}
	return sym
}

// namedOrFoundSymbol returns the symbol the case names by qualified path, which
// reaches a nested element, or searches the model when it names none.
func namedOrFoundSymbol(t *testing.T, idx *symbols.Index, fqn string, scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	if fqn == "" {
		return findBehavioralSymbol(t, scope, defKind, usageKind)
	}
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("evaluate %q: %d matching symbols, want 1", fqn, len(matches))
	}
	if !behavioralKind(matches[0], defKind, usageKind) {
		t.Fatalf("evaluate %q: names a %T, want a %v/%v", fqn, matches[0].Decl, defKind, usageKind)
	}
	return matches[0]
}

// namedSymbol returns the one symbol of the asked-for kind that a qualified path
// names, or nil.
func namedSymbol(idx *symbols.Index, fqn string, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 || !behavioralKind(matches[0], defKind, usageKind) {
		return nil
	}
	return matches[0]
}

// behavioralKind reports whether a symbol declares the definition or usage kind
// an entry point asks for.
func behavioralKind(sym *symbols.Symbol, defKind ast.DefinitionKind, usageKind ast.UsageKind) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		return decl.Kind == defKind
	case *ast.Usage:
		return decl.Kind == usageKind
	default:
		return false
	}
}

// lookupBehavioralSymbol is findBehavioralSymbol for callers that probe several
// kinds and treat absence as "not this kind of model".
func lookupBehavioralSymbol(scope *symbols.Scope, defKind ast.DefinitionKind, usageKind ast.UsageKind) *symbols.Symbol {
	// Check all child scopes (packages/namespaces)
	for _, child := range scope.Children() {
		// Look for named symbols
		for _, name := range child.MemberNames() {
			sym, ok := child.LookupLocal(name)
			if !ok {
				continue
			}
			if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == defKind {
				return sym
			}
			if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == usageKind {
				return sym
			}
		}
	}

	// Also check root scope directly
	for _, name := range scope.MemberNames() {
		sym, ok := scope.LookupLocal(name)
		if !ok {
			continue
		}
		if def, ok := sym.Decl.(*ast.Definition); ok && def.Kind == defKind {
			return sym
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == usageKind {
			return sym
		}
	}

	return nil
}

// expectedToRuntimeValue converts ExpectedValue to runtime Value
func expectedToRuntimeValue(t *testing.T, ev ExpectedValue) Value {
	switch ev.Type {
	case "Integer":
		var intVal int64
		switch v := ev.Value.(type) {
		case float64:
			intVal = int64(v)
		case int64:
			intVal = v
		default:
			t.Fatalf("invalid Integer value type: %T", ev.Value)
		}
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: intVal}}
	case "Real":
		if v, ok := ev.Value.(float64); ok {
			return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: v}}
		}
		t.Fatalf("invalid Real value type: %T", ev.Value)
	case "Boolean":
		if v, ok := ev.Value.(bool); ok {
			return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: v}}
		}
		t.Fatalf("invalid Boolean value type: %T", ev.Value)
	case "String":
		if v, ok := ev.Value.(string); ok {
			return NewStringValue(v)
		}
		t.Fatalf("invalid String value type: %T", ev.Value)
	case "Null":
		return Value{Kind: ValNull}
	case "Infinity":
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}
	case "Variant":
		t.Fatalf("a variant is named by the model, so it cannot be built from a case value")
	case "EnumLiteral":
		t.Fatalf("an enumeration literal is declared by the model, so it cannot be built from a case value")
	case "MeasurementRef":
		t.Fatalf("a measurement reference names a unit the model declares, so it cannot be built from a case value")
	case "CoordinateFrame", "CoordinateTransformation":
		t.Fatalf("a %s is declared by the model, so it cannot be built from a case value", ev.Type)
	case "Complex":
		v, ok := ev.Value.(float64)
		if !ok || ev.Im == nil {
			t.Fatalf("invalid Complex value: %v (im %v)", ev.Value, ev.Im)
		}
		return NewComplex(complex(v, *ev.Im))
	case "Quantity":
		v, ok := ev.Value.(float64)
		if !ok {
			t.Fatalf("invalid Quantity value type: %T", ev.Value)
		}
		return NewQuantityValue(&Quantity{
			Num:  semantics.Value{Kind: semantics.ValReal, Real: v},
			Unit: Unit{Text: ev.Unit},
		})
	default:
		t.Fatalf("unknown type: %s", ev.Type)
	}
	return Value{}
}

// validateValue checks if runtime Value matches ExpectedValue
func validateValue(t reporter, ctx *Context, name string, expected ExpectedValue, actual Value) {
	switch expected.Type {
	case "Sequence":
		if actual.Kind != ValSequence {
			t.Errorf("%s: type = %v, want Sequence", name, actual.Kind)
			return
		}
		elements := elementsOf(actual)
		if len(elements) != len(expected.Elements) {
			t.Errorf("%s: %d elements, want %d", name, len(elements), len(expected.Elements))
			return
		}
		for i, want := range expected.Elements {
			validateValue(t, ctx, fmt.Sprintf("%s#(%d)", name, i+1), want, elements[i])
		}
	case "Vector":
		if actual.Kind != ValVector || actual.Vector() == nil {
			t.Errorf("%s: type = %v, want Vector", name, actual.Kind)
			return
		}
		validateElements(t, ctx, name, expected.Elements, constValues(actual.Vector().Elements))
	case "VectorQuantity":
		if actual.Kind != ValVectorQuantity || actual.VectorQuantity() == nil {
			t.Errorf("%s: type = %v, want VectorQuantity", name, actual.Kind)
			return
		}
		vq := actual.VectorQuantity()
		axes := make([]Value, vq.Dimension())
		for i := range axes {
			axes[i] = NewQuantityValue(vq.component(i))
		}
		validateElements(t, ctx, name, expected.Elements, axes)
	case "Array":
		if actual.Kind != ValArray || actual.Array() == nil {
			t.Errorf("%s: type = %v, want Array", name, actual.Kind)
			return
		}
		if got := actual.Array().Dimensions; !slices.Equal(got, expected.Dimensions) {
			t.Errorf("%s: dimensions = %v, want %v", name, got, expected.Dimensions)
		}
		validateElements(t, ctx, name, expected.Elements, actual.Array().Elements)
	case "TensorQuantity":
		if actual.Kind != ValTensorQuantity || actual.TensorQuantity() == nil {
			t.Errorf("%s: type = %v, want TensorQuantity", name, actual.Kind)
			return
		}
		tq := actual.TensorQuantity()
		if got := tq.Dimensions; !slices.Equal(got, expected.Dimensions) {
			t.Errorf("%s: dimensions = %v, want %v", name, got, expected.Dimensions)
		}
		validateElements(t, ctx, name, expected.Elements, tq.components())
	case "Instance":
		if actual.Kind != ValInstance {
			t.Errorf("%s: type = %v, want Instance", name, actual.Kind)
		}
		if ctx != nil && ctx.HoldsNoValue(actual) {
			t.Errorf("%s: holds no value, want an instance holding one", name)
		}
	case "Unset":
		// A valueless feature of a value type: materialized, holding no value.
		if ctx == nil || !ctx.HoldsNoValue(actual) {
			t.Errorf("%s: type = %v, want %s", name, actual.Kind, UnsetText)
		}
	case "Integer":
		if actual.Kind != ValConst || actual.Const.Kind != semantics.ValInt {
			t.Errorf("%s: type = %v (Const.Kind=%v), want Integer", name, actual.Kind, actual.Const.Kind)
			return
		}
		want := int64(expected.Value.(float64))
		if actual.Const.Int != want {
			t.Errorf("%s: value = %d, want %d", name, actual.Const.Int, want)
		}
	case "Real":
		if actual.Kind != ValConst || actual.Const.Kind != semantics.ValReal {
			t.Errorf("%s: type = %v (Const.Kind=%v), want Real", name, actual.Kind, actual.Const.Kind)
			return
		}
		want := expected.Value.(float64)
		if actual.Const.Real != want {
			t.Errorf("%s: value = %f, want %f", name, actual.Const.Real, want)
		}
	case "Boolean":
		if actual.Kind != ValConst || actual.Const.Kind != semantics.ValBool {
			t.Errorf("%s: type = %v (Const.Kind=%v), want Boolean", name, actual.Kind, actual.Const.Kind)
			return
		}
		want := expected.Value.(bool)
		if actual.Const.Bool != want {
			t.Errorf("%s: value = %v, want %v", name, actual.Const.Bool, want)
		}
	case "String":
		if actual.Kind != ValString {
			t.Errorf("%s: type = %v, want String", name, actual.Kind)
			return
		}
		want := expected.Value.(string)
		if actual.Str() != want {
			t.Errorf("%s: value = %q, want %q", name, actual.Str(), want)
		}
	case "Null":
		if actual.Kind != ValNull {
			t.Errorf("%s: type = %v, want Null", name, actual.Kind)
		}
	case "Infinity":
		if actual.Kind != ValConst || !actual.Const.IsUnbounded() {
			t.Errorf("%s: type = %v (Const.Kind=%v), want the unbounded `*`", name, actual.Kind, actual.Const.Kind)
		}
	case "Variant":
		if actual.Kind != ValVariant || actual.Variant() == nil {
			t.Errorf("%s: type = %v, want Variant", name, actual.Kind)
			return
		}
		want := expected.Value.(string)
		if actual.Variant().Name != want {
			t.Errorf("%s: variant = %q, want %q", name, actual.Variant().Name, want)
		}
	case "EnumLiteral":
		if actual.Kind != ValEnumLiteral || actual.Literal() == nil {
			t.Errorf("%s: type = %v, want EnumLiteral", name, actual.Kind)
			return
		}
		want := expected.Value.(string)
		if got := actual.LiteralText(); got != want {
			t.Errorf("%s: literal = %q, want %q", name, got, want)
		}
	case "Complex":
		if actual.Kind != ValComplex {
			t.Errorf("%s: type = %v, want Complex", name, actual.Kind)
			return
		}
		if expected.Im == nil {
			t.Errorf("%s: a Complex case states its im", name)
			return
		}
		if want := complex(expected.Value.(float64), *expected.Im); actual.Complex() != want {
			t.Errorf("%s: value = %s, want %s", name, FormatComplex(actual.Complex()), FormatComplex(want))
		}
	case "Quantity":
		if actual.Kind != ValQuantity || actual.Quantity() == nil {
			t.Errorf("%s: type = %v, want Quantity", name, actual.Kind)
			return
		}
		if got := actual.Quantity().Unit.String(); got != expected.Unit {
			t.Errorf("%s: unit = %q, want %q", name, got, expected.Unit)
		}
		want := expected.Value.(float64)
		got := actual.Quantity().Num
		switch got.Kind {
		case semantics.ValReal:
			// A magnitude computed by repeated arithmetic is compared within a
			// tolerance: the case pins the physics, not the last bit of a float.
			if math.Abs(got.Real-want) > 1e-9*math.Max(1, math.Abs(want)) {
				t.Errorf("%s: magnitude = %v, want %v", name, got.Real, want)
			}
		case semantics.ValInt:
			if float64(got.Int) != want {
				t.Errorf("%s: magnitude = %d, want %v", name, got.Int, want)
			}
		default:
			t.Errorf("%s: magnitude kind = %v, want a number", name, got.Kind)
		}
	case "MeasurementRef":
		if actual.Kind != ValMeasurementRef || actual.MeasurementRef() == nil {
			t.Errorf("%s: type = %v, want MeasurementRef", name, actual.Kind)
			return
		}
		if got := actual.MeasurementRef().Unit.String(); got != expected.Unit {
			t.Errorf("%s: unit = %q, want %q", name, got, expected.Unit)
		}
	case "CoordinateFrame":
		actual = denotedObjectValue(t, ctx, name, actual)
		if actual.Kind != ValCoordinateFrame || actual.CoordinateFrame() == nil {
			t.Errorf("%s: type = %v, want CoordinateFrame", name, actual.Kind)
			return
		}
		if got := actual.CoordinateFrame().String(); got != expected.Text {
			t.Errorf("%s: frame = %q, want %q", name, got, expected.Text)
		}
	case "CoordinateTransformation":
		actual = denotedObjectValue(t, ctx, name, actual)
		if actual.Kind != ValCoordinateTransformation || actual.CoordinateTransformation() == nil {
			t.Errorf("%s: type = %v, want CoordinateTransformation", name, actual.Kind)
			return
		}
		if got := actual.CoordinateTransformation().String(); got != expected.Text {
			t.Errorf("%s: transformation = %q, want %q", name, got, expected.Text)
		}
	default:
		t.Errorf("%s: unknown expected type %s", name, expected.Type)
	}
}

// denotedObjectValue reads an object a slot holds as an expression naming the
// slot does: a frame or transformation declaration is the value it declares.
func denotedObjectValue(t reporter, ctx *Context, name string, actual Value) Value {
	t.Helper()
	if ctx == nil || actual.Kind != ValInstance {
		return actual
	}
	inst, ok := ctx.Instance(actual.Instance)
	if !ok {
		return actual
	}
	val, err := ctx.objectValue(inst)
	if err != nil {
		t.Errorf("%s: reading the object it holds: %v", name, err)
		return actual
	}
	return val
}

// validateElements checks the elements of a structured value one by one.
func validateElements(t reporter, ctx *Context, name string, expected []ExpectedValue, actual []Value) {
	if len(actual) != len(expected) {
		t.Errorf("%s: %d elements, want %d", name, len(actual), len(expected))
		return
	}
	for i, want := range expected {
		validateValue(t, ctx, fmt.Sprintf("%s#(%d)", name, i+1), want, actual[i])
	}
}

// TestAdmissibleOutcomesSchema pins that an admissible set cannot hide a bug: it
// replaces the single outcome, lists at least two, and cites an oracle section.
func TestAdmissibleOutcomesSchema(t *testing.T) {
	titles := oracleSectionTitles(t)
	const cited = "Concurrent branches writing one feature: the value is open, the writes are not"
	if !titles[cited] {
		t.Fatalf("the oracle no longer has a section titled %q", cited)
	}
	one := ExpectedValue{Type: "Integer", Value: 1.0}
	two := ExpectedValue{Type: "Integer", Value: 2.0}
	outcomes := []Outcome{
		{Outputs: map[string]ExpectedValue{"x": one}},
		{Outputs: map[string]ExpectedValue{"x": two}},
	}
	tests := []struct {
		name     string
		expected ExpectedOutcome
		problems int
	}{
		{"single outcome", ExpectedOutcome{Type: "action", Outputs: outcomes[0].Outputs}, 0},
		{"admissible set", ExpectedOutcome{Type: "action", Outcomes: outcomes, Admissible: cited}, 0},
		{"state admissible set", ExpectedOutcome{Type: "state", Outcomes: []Outcome{{FinalState: "A"}, {FinalState: "B"}}, Admissible: cited}, 0},
		{"outcomes beside outputs", ExpectedOutcome{Type: "action", Outputs: outcomes[0].Outputs, Outcomes: outcomes, Admissible: cited}, 1},
		{"outcomes beside finalState", ExpectedOutcome{Type: "state", FinalState: "A", Outcomes: outcomes, Admissible: cited}, 1},
		{"outcomes beside performers", ExpectedOutcome{Type: "state", Performers: []Performer{{Object: "P::a"}}, Outcomes: outcomes, Admissible: cited}, 1},
		{"missing admissible", ExpectedOutcome{Type: "action", Outcomes: outcomes}, 1},
		{"admissible cites no section", ExpectedOutcome{Type: "action", Outcomes: outcomes, Admissible: "the value is open"}, 1},
		{"admissible without outcomes", ExpectedOutcome{Type: "action", Outputs: outcomes[0].Outputs, Admissible: cited}, 1},
		{"one outcome listed", ExpectedOutcome{Type: "action", Outcomes: outcomes[:1], Admissible: cited}, 1},
		{"empty outcome", ExpectedOutcome{Type: "action", Outcomes: []Outcome{outcomes[0], {}}, Admissible: cited}, 1},
		{"calc case", ExpectedOutcome{Type: "calc", Outcomes: outcomes, Admissible: cited}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if problems := admissibleSchemaProblems(tt.expected, titles); len(problems) != tt.problems {
				t.Errorf("problems = %v, want %d", problems, tt.problems)
			}
		})
	}
}

// TestMatchOutcomeRequiresExactlyOne pins that a run matching no listed outcome
// fails, and so does one matching several, since the set must be distinct.
func TestMatchOutcomeRequiresExactlyOne(t *testing.T) {
	outcomes := []Outcome{
		{Outputs: map[string]ExpectedValue{"x": {Type: "Integer", Value: 1.0}}},
		{Outputs: map[string]ExpectedValue{"x": {Type: "Integer", Value: 2.0}}},
		{Outputs: map[string]ExpectedValue{"y": {Type: "Boolean", Value: true}}},
	}
	intValue := func(n int64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	}
	trueValue := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}}
	tests := []struct {
		name    string
		got     map[string]Value
		matched []int
	}{
		{"one", map[string]Value{"x": intValue(2)}, []int{2}},
		{"none", map[string]Value{"x": intValue(3)}, nil},
		{"several", map[string]Value{"x": intValue(1), "y": trueValue}, []int{1, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, report := matchedOutcomes(outcomes, func(r reporter, outcome Outcome) {
				validateOutputs(r, nil, outcome.Outputs, tt.got)
			})
			if !slices.Equal(matched, tt.matched) {
				t.Fatalf("matched %v, want %v (%v)", matched, tt.matched, report)
			}
			if len(report) != len(outcomes)-len(matched) {
				t.Fatalf("report %v does not cover every unmatched outcome", report)
			}
		})
	}
}

// TestConformanceDiagnosticsGate pins that a conformance case fails on a
// diagnostic it does not declare, and on a declaration nothing reported: a model
// that parses with an error would otherwise execute recovered input unnoticed.
func TestConformanceDiagnosticsGate(t *testing.T) {
	diag := func(msg string) parser.Diagnostic {
		return parser.Diagnostic{Message: msg, Span: source.Span{Offset: 7}}
	}
	tests := []struct {
		name     string
		got      []parser.Diagnostic
		want     []string
		problems int
	}{
		{"clean model, nothing declared", nil, nil, 0},
		{"undeclared diagnostic", []parser.Diagnostic{diag("expected ';' after transition")}, nil, 1},
		{"declared diagnostic", []parser.Diagnostic{diag("expected ';' after transition")}, []string{"after transition"}, 0},
		{"stale declaration", nil, []string{"after transition"}, 1},
		{"one declaration covers one diagnostic", []parser.Diagnostic{diag("bad"), diag("bad")}, []string{"bad"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if problems := diagnosticProblems(tt.got, tt.want); len(problems) != tt.problems {
				t.Errorf("problems = %v, want %d", problems, tt.problems)
			}
		})
	}
}
