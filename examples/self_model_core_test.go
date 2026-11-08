package examples

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	wscheck "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/smt"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	service "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/project"
)

// TestSelfModelActionRequestMatchesSurfaces checks the request and response the
// modelled ExecuteAction describes against the proto, the JSON engine and the capabilities.
func TestSelfModelActionRequestMatchesSurfaces(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	request := instantiateSelfModel(t, idx, ctx, "execution.sysml", "OpenSysMLExecution", "ActionRequest")
	results := instantiateSelfModel(t, idx, ctx, "execution.sysml", "OpenSysMLExecution", "ActionResults")

	actionFields := (&pb.ExecuteActionRequest{}).ProtoReflect().Descriptor().Fields()
	stateFields := (&pb.ExecuteStateRequest{}).ProtoReflect().Descriptor().Fields()
	for _, attribute := range []string{"advanceField", "traceField", "eventsField"} {
		name := protoreflect.Name(request.str(attribute))
		if actionFields.ByName(name) == nil {
			t.Errorf("execution.sysml says ExecuteActionRequest carries %q, the proto has no such field", name)
		}
		if attribute == "advanceField" && stateFields.ByName(name) == nil {
			t.Errorf("execution.sysml says ExecuteStateRequest carries %q too, the proto has no such field", name)
		}
	}
	trace := (&pb.ExecuteActionResponse{}).ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(request.str("traceField")))
	if trace == nil || !trace.IsList() || string(trace.Message().Name()) != results.str("traceRecord") {
		t.Errorf("execution.sysml says the trace comes back as %s records, the proto disagrees: %v", results.str("traceRecord"), trace)
	}
	if got, want := request.str("traceCapability"), service.CapabilityActionTrace; got != want {
		t.Errorf("execution.sysml names the trace capability %q, the service %q", got, want)
	}
	if got, want := request.str("stateTraceCapability"), service.CapabilityStateTrace; got != want {
		t.Errorf("execution.sysml names the state trace capability %q, the service %q", got, want)
	}
	for _, shape := range []reflect.Type{reflect.TypeOf(engine.JExecuteActionRequest{}), reflect.TypeOf(engine.JExecuteStateRequest{})} {
		for _, attribute := range []string{"advanceField", "traceField", "eventsField"} {
			if !jsonFieldDeclared(shape, request.str(attribute)) {
				t.Errorf("execution.sysml says the JSON engine's %s carries %q, it does not", shape.Name(), request.str(attribute))
			}
		}
	}
}

// jsonFieldDeclared reports whether a struct has a field serialized under the JSON name.
func jsonFieldDeclared(shape reflect.Type, name string) bool {
	for i := 0; i < shape.NumField(); i++ {
		if tag, _, _ := strings.Cut(shape.Field(i).Tag.Get("json"), ","); tag == name {
			return true
		}
	}
	return false
}

// TestSelfModelSharedModelMatchesRuntime checks the memo tables the modelled
// shared model names against the fields of runtime.Model, and its feature iteration.
func TestSelfModelSharedModelMatchesRuntime(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	shared := instantiateSelfModel(t, idx, ctx, "execution.sysml", "OpenSysMLExecution", "SharedModel")
	scheduler := instantiateSelfModel(t, idx, ctx, "execution.sysml", "OpenSysMLExecution", "Scheduler")

	if got, want := shared.str("goPackage"), "internal/exec/runtime"; got != want {
		t.Errorf("execution.sysml places the shared model in %q, want %q", got, want)
	}
	modelType := reflect.TypeOf(runtime.Model{})
	for _, table := range strings.Split(shared.str("memoTables"), ", ") {
		field, ok := modelType.FieldByName(table)
		if !ok || field.Type.Kind() != reflect.Map {
			t.Errorf("execution.sysml says runtime.Model memoizes %q, it has no such map", table)
		}
	}
	runtimeSource := readGoPackage(t, filepath.Join("..", "internal", "exec", "runtime"))
	if shared.boolean("iteratesObjectFeatures") != strings.Contains(runtimeSource, "func (ctx *Context) objectFeatures(") {
		t.Error("execution.sysml says the context iterates an object's features without listing them; the runtime declares no objectFeatures")
	}
	if got, want := scheduler.str("dueOrderChoice"), runtime.ChoiceDueOrder.String(); got != want {
		t.Errorf("execution.sysml names the due-order choice %q, the runtime %q", got, want)
	}
	standing := reflect.TypeOf(lower.Standing{})
	_, hasReads := standing.FieldByName("readIndex")
	_, hasWrites := standing.FieldByName("writeIndex")
	if scheduler.boolean("indexesStandingPerDraw") != (hasReads && hasWrites) {
		t.Error("execution.sysml says a due-order draw indexes each move's statements once; lower.Standing carries no such index")
	}
}

// probeSource is a model whose performing action loops twice and whose other
// part's behavior cannot start.
const probeSource = `package Probe {
    private import ScalarValues::*;
    part def Counter {
        attribute n : Integer := 0;
        perform action count { while n < 2 { assign n := n + 1; } }
    }
    part def Broken {
        attribute x : Integer = 1;
        perform action bad { assign x := 1 / 0; }
    }
}`

// probeRun instantiates a definition of probeSource in a fresh runtime.
func probeRun(t *testing.T, def string) (*runtime.Context, *symbols.Scope, *runtime.Instance) {
	t.Helper()

	idx, _ := model.NewIndexWithStdlib()
	idx.AddDocument("probe.sysml", parser.New(source.New("probe.sysml", []byte(probeSource))).ParseFile())
	resolver := resolve.New(idx)
	ctx := runtime.NewContext(runtime.NewModel(passes.NewTypedModel(resolver), resolver), 100000)
	for _, scope := range idx.DocumentRoot("probe.sysml").Children() {
		if sym, ok := scope.LookupLocal(def); ok {
			inst, err := ctx.Instantiate(sym)
			if err != nil {
				t.Fatalf("instantiate %s: %v", def, err)
			}
			return ctx, scope, inst
		}
	}
	t.Fatalf("probe declares no %s", def)
	return nil, nil, nil
}

// probeEval evaluates an expression over the probe's instance.
func probeEval(t *testing.T, ctx *runtime.Context, scope *symbols.Scope, self *runtime.Instance, expr string) runtime.Value {
	t.Helper()

	v, err := ctx.EvalWithScopeOn(parser.New(source.New("expr", []byte(expr))).ParseExpression(), scope, self)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return v
}

// TestSelfModelLoopPerformancesMatchRuntime runs a looping action and checks the
// loop and iteration performances the modelled LoopPerformances describes.
func TestSelfModelLoopPerformancesMatchRuntime(t *testing.T) {
	idx, selfCtx := analyseSelfModel(t)
	loops := instantiateSelfModel(t, idx, selfCtx, "execution.sysml", "OpenSysMLExecution", "LoopPerformances")

	runtimeSource := readGoPackage(t, filepath.Join("..", "internal", "exec", "runtime"))
	for _, attribute := range []string{"loopsFeature", "bodyFeature"} {
		fqn := loops.str(attribute)
		if !strings.Contains(runtimeSource, `"`+fqn+`"`) {
			t.Errorf("execution.sysml says the runtime holds performances under %q, the runtime never names it", fqn)
		}
		if len(idx.LookupQualified(fqn)) == 0 {
			t.Errorf("execution.sysml names %q, the standard library declares no such feature", fqn)
		}
	}
	for _, fqn := range strings.Split(loops.str("loopTypes"), ", ") {
		if !strings.Contains(runtimeSource, `"`+fqn+`"`) {
			t.Errorf("execution.sysml says a loop performs as %q, the runtime never names it", fqn)
		}
	}

	ctx, scope, counter := probeRun(t, "Counter")
	loopsHeld := probeEval(t, ctx, scope, counter, "count.loops")
	if n := loopsHeld.Sequence().Size(); n != 1 {
		t.Fatalf("a performing action with one while loop holds %d loops, want 1", n)
	}
	body := probeEval(t, ctx, scope, counter, "count.loops#(1).body")
	if got, want := body.Sequence().Size(), 2; (got == want) != loops.boolean("iterationPerformsBody") {
		t.Errorf("two iterations held %d body performances, want %d", got, want)
	}
}

// TestSelfModelRunContextShotsAndFailures checks that a run answers occurrence shots and
// time slices from what it tracked, and that a behavior failing to start keeps its object.
func TestSelfModelRunContextShotsAndFailures(t *testing.T) {
	idx, selfCtx := analyseSelfModel(t)
	run := instantiateSelfModel(t, idx, selfCtx, "execution.sysml", "OpenSysMLExecution", "RunContext")
	instance := instantiateSelfModel(t, idx, selfCtx, "execution.sysml", "OpenSysMLExecution", "Instance")

	ctx, scope, counter := probeRun(t, "Counter")
	count, err := counter.GetFeatureValue(ctx, "count")
	if err != nil {
		t.Fatal(err)
	}
	id, _ := count.Value.Object()
	performance, _ := ctx.Instance(id)
	_, inShape := performance.FeatureValues["start"]
	member, err := performance.MemberValue(ctx, "start")
	if instance.boolean("kernelMembersOutsideShape") != (!inShape && err == nil && member != nil) {
		t.Errorf("execution.sysml says kernel members stay outside the object shape; start is held %t and answered %v", inShape, err)
	}
	if _, ok := member.Value.Object(); !ok {
		t.Errorf("the performance's start shot is %s, want an occurrence", runtime.FormatTraceValue(member.Value))
	}
	slices := probeEval(t, ctx, scope, counter, "self.timeSlices")
	if !run.boolean("answersShotsFromRun") || slices.Undetermined() == nil {
		t.Errorf("an unended occurrence's time slices read %s, want undetermined until its end shot happens", runtime.FormatTraceValue(slices))
	}

	brokenCtx, _, broken := probeRun(t, "Broken")
	failures := brokenCtx.StartFailures(broken)
	if run.boolean("keepsObjectOnStartFailure") != (broken != nil && len(failures) == 1) {
		t.Fatalf("a behavior that cannot start left %d failures on a standing object", len(failures))
	}
	failure := runtime.BehaviorStartFailure{Behavior: failures[0]}
	if got, want := run.str("startFailureCode"), failure.Diagnostic().Code; got != want {
		t.Errorf("execution.sysml records a failed start as %q, the runtime as %q", got, want)
	}
	if got, want := run.str("startFailureCode"), runtime.BehaviorStartFailureCode; got != want {
		t.Errorf("execution.sysml records a failed start as %q, the runtime's constant is %q", got, want)
	}
}

// TestSelfModelKernelReadFollowsRuntime runs the modelled feature read over a
// kernel member the shape leaves out and checks it is answered, not refused.
func TestSelfModelKernelReadFollowsRuntime(t *testing.T) {
	flow := selfModelFlow(t, "ReadFeatureValue", map[string]string{"declared": "false", "kernelMember": "true"})
	if !flow.boolean("answeredFromRun") || flow.boolean("refused") {
		t.Errorf("a kernel member read answered %t and refused %t, want answered and not refused", flow.boolean("answeredFromRun"), flow.boolean("refused"))
	}
	if flow.integer("evaluations") != 0 {
		t.Errorf("a kernel member read evaluated %d times, want none", flow.integer("evaluations"))
	}
}

// TestSelfModelTokenFlowMatchesExecutor checks the executor states the modelled
// token flow names, and that it has a state for each the runtime can park in.
func TestSelfModelTokenFlowMatchesExecutor(t *testing.T) {
	text := readModelFile(t, "behavior.sysml")
	declared, ok := declaredString(text, "executorStates")
	if !ok {
		t.Fatal("behavior.sysml declares no executorStates")
	}
	var states []string
	for s := runtime.StateReady; s.String() != "Unknown"; s++ {
		states = append(states, s.String())
	}
	if got, want := declared, strings.Join(states, ", "); got != want {
		t.Errorf("behavior.sysml names the executor states %q, the runtime %q", got, want)
	}
	flow := text[strings.Index(text, "state def TokenFlow"):]
	flow = flow[:strings.Index(flow, "\n    }")]
	for _, state := range []string{"waiting", "terminated"} {
		if !strings.Contains(flow, "state "+state+";") {
			t.Errorf("TokenFlow has no %s state for the runtime's %s executors", state, strings.ToUpper(state[:1])+state[1:])
		}
	}
}

// TestSelfModelWorkspaceProjectsMatchImplementation checks project discovery and the
// workspace checks against internal/workspace/project and internal/check/workspace.
func TestSelfModelWorkspaceProjectsMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	discovery := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "ProjectDiscovery")
	checks := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "WorkspaceChecks")

	if got, want := discovery.str("projectFile"), project.ProjectFile; got != want {
		t.Errorf("pipeline.sysml names the project file %q, the implementation %q", got, want)
	}
	if got, want := discovery.str("libraryRootsEnvVar"), project.LibraryRootsEnvVar; got != want {
		t.Errorf("pipeline.sysml names the library roots variable %q, the implementation %q", got, want)
	}
	if got, want := discovery.str("archiveExtension"), project.ArchiveExt; got != want {
		t.Errorf("pipeline.sysml names the archive extension %q, the implementation %q", got, want)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, project.ProjectFile), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, layout := range strings.Split(discovery.str("libraryLayouts"), ", ") {
		dir := filepath.Join(root, filepath.FromSlash(layout))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if !project.IsLibraryDir(root, strings.SplitN(layout, "/", 2)[0]) {
			t.Errorf("pipeline.sysml says %q beside %s is a library directory, discovery disagrees", layout, project.ProjectFile)
		}
	}
	if project.IsLibraryDir(root, "src") {
		t.Error("discovery takes any directory beside the project file for a library")
	}
	if got, want := checks.str("source"), wscheck.Source; got != want {
		t.Errorf("pipeline.sysml says workspace findings carry source %q, the implementation %q", got, want)
	}
	if got, want := checks.str("libraryDeclaredTwice"), wscheck.CodeLibraryDeclaredTwice; got != want {
		t.Errorf("pipeline.sysml codes a twice-declared library %q, the implementation %q", got, want)
	}
	checksSource := readGoPackage(t, filepath.Join("..", "internal", "check", "workspace"))
	if checks.boolean("oncePerWorkspace") != strings.Contains(checksSource, "once per workspace") {
		t.Error("pipeline.sysml says a twice-declared library is reported once per workspace; the check does not say so")
	}
}

// TestSelfModelSolversMatchImplementation checks the solver and SMT engine facts
// the pipeline model states against internal/exec/solve and internal/exec/smt.
func TestSelfModelSolversMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	solver := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "Solver")
	smtEngine := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "SmtEngine")

	if got, want := solver.str("solverEnvVar"), solve.SolverEnv; got != want {
		t.Errorf("pipeline.sysml names the solver variable %q, the implementation %q", got, want)
	}
	var sources []string
	for _, s := range []solve.PinSource{solve.PinHeld, solve.PinDeclared, solve.PinChosen} {
		sources = append(sources, s.String())
	}
	if got, want := solver.str("pinSources"), strings.Join(sources, ", "); got != want {
		t.Errorf("pipeline.sysml spells the pin sources %q, the solver %q", got, want)
	}
	if got, want := solver.str("derivedRole"), solve.RoleDerived.String(); got != want {
		t.Errorf("pipeline.sysml says a derived value plays the role %q, the solver %q", got, want)
	}
	if _, ok := reflect.TypeOf(solve.Query{}).FieldByName("Derived"); !ok {
		t.Error("pipeline.sysml says the solver asserts derived values as defining equations, but a Query carries none")
	}
	if got, want := smtEngine.integer("defaultMoves"), smt.DefaultMoves; got != want {
		t.Errorf("pipeline.sysml says the SMT engine takes %d moves by default, the engine %d", got, want)
	}
	if got, want := smtEngine.integer("maxSlots"), smt.MaxSlots; got != want {
		t.Errorf("pipeline.sysml says the SMT engine encodes %d slots at most, the engine %d", got, want)
	}
	_, derived := reflect.TypeOf(smt.Encoding{}).FieldByName("derived")
	if smtEngine.boolean("encodesDerivedFeatures") != derived {
		t.Error("pipeline.sysml says the SMT encoding substitutes derived features; the Encoding keeps none")
	}
}

// TestSelfModelIdentityLanguagesMatchImplementation checks the library languages,
// the extension prefix and the name separator the identity model states.
func TestSelfModelIdentityLanguagesMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	table := instantiateSelfModel(t, idx, ctx, "identity.sysml", "OpenSysMLIdentity", "IdentityTable")
	harness := instantiateSelfModel(t, idx, ctx, "identity.sysml", "OpenSysMLIdentity", "InteropHarness")

	var languages []string
	for l := identity.KerML; l.String() != "unknown"; l++ {
		languages = append(languages, l.String())
	}
	if got, want := table.str("libraryLanguages"), strings.Join(languages, ", "); got != want {
		t.Errorf("identity.sysml names the library languages %q, the implementation %q", got, want)
	}
	if got, want := table.str("extensionPrefix"), identity.ExtensionPrefix; got != want {
		t.Errorf("identity.sysml hashes the extension libraries under %q, the implementation %q", got, want)
	}
	if got, want := table.str("qualifiedNameSeparator"), identity.Separator; got != want {
		t.Errorf("identity.sysml separates qualified names with %q, the implementation %q", got, want)
	}
	measure := filepath.Join("..", filepath.FromSlash(harness.str("measuredBy")))
	if _, err := os.Stat(filepath.Join(measure, "measure.go")); err != nil {
		t.Fatalf("identity.sysml says the interop harness is measured by %s, which has no measure.go: %v", harness.str("measuredBy"), err)
	}
	gate := regexp.MustCompile(`EnvGate\s*=\s*"([^"]+)"`).FindStringSubmatch(readGoPackage(t, measure))
	if gate == nil || gate[1] != harness.str("gateEnvVar") {
		t.Errorf("identity.sysml gates the interop measurement on %q, the referee on %v", harness.str("gateEnvVar"), gate)
	}
}

// TestSelfModelSourceTreeMatchesRepository checks every modelled directory exists and
// that every tracked top-level directory of the repository is modelled.
func TestSelfModelSourceTreeMatchesRepository(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	tree := instantiateSelfModel(t, idx, ctx, "quality.sysml", "OpenSysMLCodebase", "SourceTree")

	modelled := map[string]bool{}
	for name, dir := range tree.parts() {
		path := dir.str("path")
		modelled[path] = true
		if info, err := os.Stat(filepath.Join("..", path)); err != nil || !info.IsDir() {
			t.Errorf("quality.sysml places %s at %q, which is no directory of the repository", name, path)
		}
	}
	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || untrackedTopLevel[name] || modelled[name] {
			continue
		}
		t.Errorf("the repository has a top-level directory %q that quality.sysml's SourceTree does not model", name)
	}
}

// untrackedTopLevel names the top-level directories that are build outputs or site
// overrides rather than source the self-model describes.
var untrackedTopLevel = map[string]bool{"bin": true, "build": true, "overrides": true, "node_modules": true}

// TestSelfModelRaceSuiteMatchesCI checks the race-suite shards and node count the
// gates model states against scripts/race-shard.sh and the CircleCI configuration.
func TestSelfModelRaceSuiteMatchesCI(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	suite := instantiateSelfModel(t, idx, ctx, "quality.sysml", "OpenSysMLGates", "RaceSuite")

	script, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(suite.str("shardList"))))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^shards='([^']+)'`).FindStringSubmatch(string(script))
	if match == nil {
		t.Fatalf("%s declares no shard list", suite.str("shardList"))
	}
	shards := strings.Split(match[1], "|")
	if got, want := suite.str("shards"), strings.Join(shards, ", "); got != want {
		t.Errorf("quality.sysml names the shards %q, the script %q", got, want)
	}
	if got, want := suite.integer("shardCount"), len(shards); got != want {
		t.Errorf("quality.sysml counts %d shards, the script %d", got, want)
	}
	config, err := os.ReadFile(filepath.Join("..", ".circleci", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	job := string(config)[strings.Index(string(config), "\n  go-race-test:"):]
	nodes := regexp.MustCompile(`parallelism: (\d+)`).FindStringSubmatch(job)
	if nodes == nil || nodes[1] != strconv.Itoa(suite.integer("nodeCount")) {
		t.Errorf("quality.sysml says the race suite runs on %d nodes, CircleCI on %v", suite.integer("nodeCount"), nodes)
	}
}

// TestSelfModelLandAChangeMatchesWorkflow checks the contributor workflow the gates
// model states against the pull-request workflow and the contributor guide.
func TestSelfModelLandAChangeMatchesWorkflow(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	change := instantiateSelfModel(t, idx, ctx, "quality.sysml", "OpenSysMLGates", "LandAChange")

	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "pr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	events := regexp.MustCompile(`(?m)^  pull_request:\n    types: \[([^\]]+)\]`).FindStringSubmatch(string(workflow))
	if events == nil || events[1] != change.str("pullRequestEvents") {
		t.Errorf("quality.sysml says the workflow runs on %q, pr.yml on %v", change.str("pullRequestEvents"), events)
	}
	if !regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(change.str("mergeQueueEvent")) + `:`).Match(workflow) {
		t.Errorf("quality.sysml says the workflow runs again on %q, pr.yml does not", change.str("mergeQueueEvent"))
	}
	if !strings.Contains(string(workflow), "'"+change.str("fullTierLabel")+"'") {
		t.Errorf("quality.sysml says the label %q forces the full tier, pr.yml names no such label", change.str("fullTierLabel"))
	}
	if change.boolean("draftRunsQuickTier") != strings.Contains(string(workflow), "A draft pull request") {
		t.Error("quality.sysml says a draft runs the quick tier; pr.yml does not say so")
	}
	guide, err := os.ReadFile(filepath.Join("..", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if change.boolean("opensReadyForReview") != strings.Contains(string(guide), "Open the PR ready for review") {
		t.Error("quality.sysml says a change opens ready for review; AGENTS.md does not say so")
	}
	if !strings.Contains(string(guide), "target `"+change.str("baseBranch")+"`") {
		t.Errorf("quality.sysml says changes target %q, AGENTS.md does not", change.str("baseBranch"))
	}
}
