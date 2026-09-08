package examples

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"go.lsp.dev/protocol"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/edit"
	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/highlight"
	"github.com/Open-MBEE/OpenSysML/internal/core/identity"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
	"github.com/Open-MBEE/OpenSysML/internal/docpdf"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
	"github.com/Open-MBEE/OpenSysML/internal/lsp"
)

const selfModelDir = "self-model"

// selfModelFiles returns the model files of the self-model, sorted, so the two
// gates below load the same set.
func selfModelFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(selfModelDir)
	if err != nil {
		t.Fatalf("read %s: %v", selfModelDir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || source.KindOf(entry.Name()) != source.KindSysML {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("%s holds no model files", selfModelDir)
	}
	return files
}

// TestSelfModelClean holds the architecture self-model to the standard the
// documentation it illustrates claims: it analyses without a diagnostic of any
// severity. Every file is opened before any is diagnosed, because the packages
// import across files.
func TestSelfModelClean(t *testing.T) {
	files := selfModelFiles(t)

	ws := model.NewWorkspace()
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(selfModelDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		ws.Open(name, content, 1)
	}

	for _, name := range files {
		var messages []string
		for _, d := range ws.Diagnostics(name) {
			messages = append(messages, d.Severity.String()+": "+d.Message)
		}
		if len(messages) > 0 {
			t.Errorf("%s/%s: %d diagnostic(s): %s",
				selfModelDir, name, len(messages), strings.Join(messages, "; "))
		}
	}
}

// TestSelfModelInvariantsHold evaluates the architecture invariants the model
// states, so a stated invariant that no longer describes this implementation
// fails here rather than reading as true in a diagram.
func TestSelfModelInvariantsHold(t *testing.T) {
	idx, ctx := analyseSelfModel(t)

	stated := map[string][]string{
		"quality.sysml/OpenSysMLInvariants": {
			"treeIsImmutable",
			"parserRecovers",
			"resolutionIsLazy",
			"tiersAreGated",
			"loweringIsLossless",
			"executionIsBounded",
			"libraryIsClean",
			"snapshotIsDerived",
			"evaluatorIsReference",
			"exportRoundTrips",
		},
		"identity.sysml/OpenSysMLIdentity": {
			"identityRoundTrips",
			"idsDoNotCollide",
			"identityIsBesideTheTree",
			"syncIsExplicit",
		},
		"surfaces.sysml/OpenSysMLSurfaces": {
			"documentsAreTraceable",
			"viewsAreHonest",
		},
	}

	for where, requirements := range stated {
		document, pkg, _ := strings.Cut(where, "/")
		scope := packageScope(t, idx, document, pkg)
		for _, name := range requirements {
			sym, ok := scope.LookupLocal(name)
			if !ok {
				t.Errorf("requirement %s is not declared in %s", name, pkg)
				continue
			}
			satisfied, err := ctx.EvaluateRequirement(sym, scope)
			if err != nil {
				t.Errorf("evaluate %s: %v", name, err)
				continue
			}
			if !satisfied {
				t.Errorf("%s does not hold: the model states an invariant this implementation no longer satisfies", name)
			}
		}
	}
}

// TestSelfModelFiguresMatchImplementation reads the figures and package names
// the model declares and compares them against this implementation, so the
// model's numbers cannot quietly drift from the code they describe.
func TestSelfModelFiguresMatchImplementation(t *testing.T) {
	pipeline := readModelFile(t, "pipeline.sysml")

	figures := []struct {
		attribute string
		actual    int
	}{
		{"keywordCount", len(lexer.Keywords())},
		{"bundledFileCount", len(libs.DefaultSource().List())},
		{"tierCount", int(passes.LevelConstraint) + 1},
	}
	for _, figure := range figures {
		declared, ok := declaredInteger(pipeline, figure.attribute)
		if !ok {
			t.Errorf("pipeline.sysml declares no integer %s", figure.attribute)
			continue
		}
		if declared != figure.actual {
			t.Errorf("pipeline.sysml says %s = %d, the implementation has %d",
				figure.attribute, declared, figure.actual)
		}
	}

	// Every modelled unit names the directory that implements it, and every
	// path-valued attribute names something in the repository.
	for _, file := range []string{"pipeline.sysml", "surfaces.sysml", "identity.sysml"} {
		matches := repositoryPathPattern.FindAllStringSubmatch(readModelFile(t, file), -1)
		if len(matches) == 0 {
			t.Errorf("%s names no implementing package", file)
			continue
		}
		for _, match := range matches {
			for _, path := range strings.Split(match[2], ", ") {
				if _, err := os.Stat(filepath.Join("..", path)); err != nil {
					t.Errorf("%s points %s at %s, which does not exist", file, match[1], path)
				}
			}
		}
	}
}

// TestSelfModelPassRegistryMatchesImplementation instantiates the modelled
// pass registry and compares it, pass by pass, with the default registry: the
// Go type, the tier it runs at, and whether it gates itself per element.
func TestSelfModelPassRegistryMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	registry := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "PassRegistry")

	type passFacts struct {
		level         string
		elementScoped bool
	}
	declared := map[string]passFacts{}
	for _, part := range registry.parts() {
		if !part.has("goType") {
			continue
		}
		goType := part.str("goType")
		if _, dup := declared[goType]; dup {
			t.Errorf("PassRegistry declares %s twice", goType)
		}
		declared[goType] = passFacts{part.str("level"), part.boolean("elementScoped")}
	}

	actual := map[string]passFacts{}
	for _, p := range passes.DefaultRegistry().Passes() {
		goType := reflect.Indirect(reflect.ValueOf(p)).Type().Name()
		_, scoped := p.(passes.ElementScoped)
		actual[goType] = passFacts{p.Level().String(), scoped}
	}

	if declared, actual := registry.integer("passCount"), len(actual); declared != actual {
		t.Errorf("pipeline.sysml says passCount = %d, the default registry registers %d", declared, actual)
	}
	for goType, want := range actual {
		got, ok := declared[goType]
		if !ok {
			t.Errorf("the default registry registers %s, which PassRegistry does not model", goType)
			continue
		}
		if got != want {
			t.Errorf("PassRegistry models %s as %+v, the implementation has %+v", goType, got, want)
		}
	}
	for goType := range declared {
		if _, ok := actual[goType]; !ok {
			t.Errorf("PassRegistry models %s, which the default registry does not register", goType)
		}
	}
}

// TestSelfModelBudgetsMatchImplementation instantiates the modelled runtime and
// compares each budget it declares with the runtime's: the variable that sets
// it, its default, and the error it reports when exhausted.
func TestSelfModelBudgetsMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	runtimeModel := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "Runtime")

	type budgetFacts struct {
		defaultBound int64
		exhaustion   string
	}
	actual := map[string]budgetFacts{
		runtime.MaxStepsEnvVar:       {runtime.DefaultMaxSteps, runtime.ErrStepLimitExceeded.Error()},
		runtime.MaxActionStepsEnvVar: {runtime.DefaultMaxActionSteps, runtime.ErrActionStepLimitExceeded.Error()},
		runtime.MaxStateEventsEnvVar: {runtime.DefaultMaxStateEvents, runtime.ErrStateEventLimitExceeded.Error()},
		runtime.MaxDoStepsEnvVar:     {runtime.DefaultMaxDoSteps, runtime.ErrDoStepLimitExceeded.Error()},
		runtime.MaxElementsEnvVar:    {runtime.DefaultMaxElements, runtime.ErrElementLimitExceeded.Error()},
		runtime.MaxCalcDepthEnvVar:   {runtime.DefaultMaxCalcDepth, runtime.ErrCalcRecursionLimit.Error()},
		runtime.MaxSweepRunsEnvVar:   {runtime.DefaultMaxSweepRuns, runtime.ErrSweepBudget.Error()},
	}
	if fields := reflect.TypeOf(runtime.Budgets{}).NumField(); fields != len(actual) {
		t.Fatalf("runtime.Budgets has %d fields, this test knows %d", fields, len(actual))
	}
	if declared := runtimeModel.integer("budgetCount"); declared != len(actual) {
		t.Errorf("pipeline.sysml says budgetCount = %d, runtime.Budgets has %d", declared, len(actual))
	}

	declared := map[string]budgetFacts{}
	for name, part := range runtimeModel.parts() {
		if !part.has("envVar") {
			continue
		}
		envVar := part.str("envVar")
		if _, dup := declared[envVar]; dup {
			t.Errorf("Runtime declares %s twice", envVar)
		}
		declared[envVar] = budgetFacts{int64(part.integer("defaultBound")), part.str("exhaustion")}
		if _, known := actual[envVar]; !known {
			t.Errorf("Runtime.%s is set by %s, which the runtime does not read", name, envVar)
		}
	}
	for envVar, want := range actual {
		got, ok := declared[envVar]
		if !ok {
			t.Errorf("the runtime reads %s, which Runtime does not model", envVar)
			continue
		}
		if got != want {
			t.Errorf("Runtime models %s as %+v, the implementation has %+v", envVar, got, want)
		}
	}
}

// TestSelfModelLibraryMatchesImplementation compares the modelled library with libs:
// the override variable, the snapshot decoding, its Make targets and the CI check.
func TestSelfModelLibraryMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	stdlib := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "StandardLibrary")

	if got, want := stdlib.str("overrideEnvVar"), libs.LibraryPathEnvVar; got != want {
		t.Errorf("pipeline.sysml says the library is overridden by %s, libs reads %s", got, want)
	}

	snapshotIdx, err := libs.SnapshotIndex()
	if decodes := err == nil && snapshotIdx != nil; decodes != stdlib.boolean("snapshotEmbedded") {
		t.Errorf("pipeline.sysml says snapshotEmbedded = %t, libs.SnapshotIndex() returned (%v, %v)",
			stdlib.boolean("snapshotEmbedded"), snapshotIdx != nil, err)
	}

	generator, ok := stdlib.parts()["generator"]
	if !ok {
		t.Fatal("StandardLibrary declares no generator part")
	}
	makefile, err := os.ReadFile(filepath.Join("..", "Makefile"))
	if err != nil {
		t.Fatalf("read the Makefile: %v", err)
	}
	targets := map[string]bool{}
	for _, match := range makeTargetPattern.FindAllStringSubmatch(string(makefile), -1) {
		targets[match[1]] = true
	}
	for _, attribute := range []string{"makeTarget", "checkTarget"} {
		if target := generator.str(attribute); !targets[target] {
			t.Errorf("pipeline.sysml says %s = %q, which the Makefile does not define", attribute, target)
		}
	}

	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "pr.yml"))
	if err != nil {
		t.Fatalf("read the pull request workflow: %v", err)
	}
	gate := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "StdlibSnapshotGate")
	if gate.str("baseline") != stdlib.str("snapshotFile") {
		t.Errorf("surfaces.sysml gates %s, pipeline.sysml embeds %s", gate.str("baseline"), stdlib.str("snapshotFile"))
	}
	if runs := strings.Contains(string(workflow), "make "+generator.str("checkTarget")); runs != gate.boolean("gating") {
		t.Errorf("surfaces.sysml says the snapshot gate is gating = %t, the pull request workflow runs make %s: %t",
			gate.boolean("gating"), generator.str("checkTarget"), runs)
	}
}

// TestSelfModelEvaluatorMatchesImplementation checks the evaluator's memoization
// claim against runtime.Context, which must key a side table by syntax node.
func TestSelfModelEvaluatorMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	evaluator := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "Evaluator")

	node := reflect.TypeOf((*ast.Node)(nil)).Elem()
	keyedByNode := false
	contextType := reflect.TypeOf(runtime.Context{})
	for i := 0; i < contextType.NumField(); i++ {
		field := contextType.Field(i).Type
		if field.Kind() == reflect.Map && field.Key().Implements(node) {
			keyedByNode = true
		}
	}
	if keyedByNode != evaluator.boolean("memoized") {
		t.Errorf("pipeline.sysml says the evaluator is memoized = %t, runtime.Context keyed a side table by syntax node: %t",
			evaluator.boolean("memoized"), keyedByNode)
	}
}

// TestSelfModelCalcCompilerMatchesImplementation checks the compiled calc tier's
// switch and its fallback against the runtime, invoking the self-model's own
// StepBudget calc through both tiers.
func TestSelfModelCalcCompilerMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	evaluator := instantiateSelfModel(t, idx, ctx, "pipeline.sysml", "OpenSysMLPipeline", "Evaluator")
	compiled, ok := evaluator.parts()["compiled"]
	if !ok {
		t.Fatal("Evaluator declares no compiled part")
	}

	envVar := compiled.str("overrideEnvVar")
	if envVar != runtime.CalcCompileEnvVar {
		t.Errorf("pipeline.sysml says the compiled tier is switched by %s, runtime reads %s", envVar, runtime.CalcCompileEnvVar)
	}
	reference, err := os.ReadFile(filepath.Join("..", "docs", "reference", "environment.md"))
	if err != nil {
		t.Fatalf("read the environment reference: %v", err)
	}
	if !strings.Contains(string(reference), "`"+envVar+"`") {
		t.Errorf("docs/reference/environment.md does not document %s", envVar)
	}

	t.Setenv(runtime.CalcCompileEnvVar, "")
	_, onByDefault := analyseSelfModel(t)
	if onByDefault.CalcCompile() != evaluator.boolean("compilesPureCalcs") {
		t.Errorf("pipeline.sysml says compilesPureCalcs = %t, a fresh runtime.Context compiles calcs: %t",
			evaluator.boolean("compilesPureCalcs"), onByDefault.CalcCompile())
	}
	t.Setenv(runtime.CalcCompileEnvVar, "0")
	_, switchedOff := analyseSelfModel(t)
	if switchedOff.CalcCompile() {
		t.Errorf("%s=0 left a fresh runtime.Context compiling calcs", runtime.CalcCompileEnvVar)
	}
	t.Setenv(runtime.CalcCompileEnvVar, "")

	// The same invocation through both tiers, plain and traced: same value, and
	// a traced run records the evaluator's sub-expressions whichever tier is on.
	integer := func(i int64) runtime.Value {
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}
	}
	var values, traces [2]string
	for i, compile := range []bool{true, false} {
		for _, traced := range []bool{false, true} {
			tierIdx, tierCtx := analyseSelfModel(t)
			tierCtx.SetCalcCompile(compile)
			scope := packageScope(t, tierIdx, "behavior.sysml", "OpenSysMLBehavior")
			sym, ok := scope.LookupLocal("StepBudget")
			if !ok {
				t.Fatal("OpenSysMLBehavior declares no StepBudget")
			}
			var tr *runtime.TraceRecorder
			if traced {
				tr = runtime.NewTraceRecorder()
				tierCtx.SetTrace(tr)
			}
			got, err := tierCtx.InvokeCalc(sym, []runtime.Value{integer(6), integer(7)}, scope)
			if err != nil {
				t.Fatalf("compile=%t traced=%t: StepBudget(6, 7): %v", compile, traced, err)
			}
			if got.Kind != runtime.ValConst || got.Const.Int != 42 {
				t.Errorf("compile=%t traced=%t: StepBudget(6, 7) = %s", compile, traced, runtime.FormatTraceValue(got))
			}
			if traced {
				traces[i] = tr.String()
			} else {
				values[i] = runtime.FormatTraceValue(got)
			}
		}
	}
	if values[0] != values[1] {
		t.Errorf("the compiled tier and the evaluator disagree on StepBudget(6, 7): %s vs %s", values[0], values[1])
	}
	if !strings.Contains(traces[1], "eval operator *") {
		t.Fatalf("the evaluator's trace records no sub-expression:\n%s", traces[1])
	}
	if fellBack := traces[0] == traces[1]; fellBack != compiled.boolean("fallsBackToEvaluator") {
		t.Errorf("pipeline.sysml says fallsBackToEvaluator = %t, a traced run under the compiled tier matched the evaluator's trace: %t\n%s\n---\n%s",
			compiled.boolean("fallsBackToEvaluator"), fellBack, traces[0], traces[1])
	}
}

// TestSelfModelViewKindsMatchImplementation compares the rendering kinds the
// modelled view engine declares with those the view package recognizes, and
// which of them it renders.
func TestSelfModelViewKindsMatchImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	engine := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "ViewEngine")

	declared := map[string]bool{}
	for _, part := range engine.parts() {
		if !part.has("kind") {
			continue
		}
		kind := part.str("kind")
		if _, dup := declared[kind]; dup {
			t.Errorf("ViewEngine declares kind %q twice", kind)
		}
		declared[kind] = part.boolean("supported")
	}

	supported := 0
	for _, kind := range view.Kinds() {
		if kind.Supported() {
			supported++
		}
		got, ok := declared[string(kind)]
		if !ok {
			t.Errorf("the view package recognizes %q, which ViewEngine does not model", kind)
			continue
		}
		if got != kind.Supported() {
			t.Errorf("ViewEngine says %q supported = %v, the implementation says %v", kind, got, kind.Supported())
		}
	}
	if len(declared) != len(view.Kinds()) {
		t.Errorf("ViewEngine models %d kinds, the view package recognizes %d", len(declared), len(view.Kinds()))
	}
	if declared := engine.integer("recognizedKinds"); declared != len(view.Kinds()) {
		t.Errorf("surfaces.sysml says recognizedKinds = %d, the implementation has %d", declared, len(view.Kinds()))
	}
	if declared := engine.integer("supportedKinds"); declared != supported {
		t.Errorf("surfaces.sysml says supportedKinds = %d, the implementation has %d", declared, supported)
	}
}

// TestSelfModelExportMatchesImplementation compares the modelled exporter with
// the export package: the formats it writes and every name it accepts for them.
func TestSelfModelExportMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	exporter := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "Exporter")

	names := export.FormatNames()
	if declared, actual := exporter.str("formatNames"), strings.Join(names, ", "); declared != actual {
		t.Errorf("surfaces.sysml says formatNames = %q, the implementation accepts %q", declared, actual)
	}

	seen := map[string]bool{}
	var formats []string
	for _, name := range names {
		format, err := export.ParseFormat(name)
		if err != nil {
			t.Fatalf("ParseFormat(%q): %v", name, err)
		}
		if !seen[format.String()] {
			seen[format.String()] = true
			formats = append(formats, format.String())
		}
	}
	sort.Strings(formats)
	if declared, actual := exporter.str("formats"), strings.Join(formats, ", "); declared != actual {
		t.Errorf("surfaces.sysml says formats = %q, the implementation writes %q", declared, actual)
	}
}

// TestSelfModelApiMatchesImplementation compares the modelled API schema with
// the generated service descriptor: the service name and every RPC on it.
func TestSelfModelApiMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	schema := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "ApiSchema")

	desc := pb.SysMLService_ServiceDesc
	if declared, actual := schema.str("service"), strings.TrimPrefix(desc.ServiceName, "sysml."); declared != actual {
		t.Errorf("surfaces.sysml says service = %q, the schema declares %q", declared, actual)
	}

	var rpcs []string
	for _, method := range desc.Methods {
		rpcs = append(rpcs, method.MethodName)
	}
	for _, stream := range desc.Streams {
		rpcs = append(rpcs, stream.StreamName)
	}
	if declared, actual := schema.str("rpcs"), strings.Join(rpcs, ", "); declared != actual {
		t.Errorf("surfaces.sysml says rpcs = %q, the schema declares %q", declared, actual)
	}
	if declared := schema.integer("rpcCount"); declared != len(rpcs) {
		t.Errorf("surfaces.sysml says rpcCount = %d, the schema declares %d", declared, len(rpcs))
	}

	generate, err := os.ReadFile(filepath.Join("..", schema.str("goPackage"), "generate.go"))
	if err != nil {
		t.Fatalf("read the schema's generate directive: %v", err)
	}
	if generator := schema.str("generator"); !strings.Contains(string(generate), generator) {
		t.Errorf("surfaces.sysml says generator = %q, the generate directive does not name it", generator)
	}
}

// TestSelfModelLanguageServerMatchesImplementation initializes the language
// server and compares the capabilities it advertises with the modelled ones.
func TestSelfModelLanguageServerMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	server := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "LanguageServer")

	result, err := lsp.NewServer(model.NewWorkspace()).Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("initialize the language server: %v", err)
	}
	caps := result.Capabilities
	sync, _ := caps.TextDocumentSync.(*protocol.TextDocumentSyncOptions)
	rename, _ := caps.RenameProvider.(*protocol.RenameOptions)
	tokens := caps.SemanticTokensProvider != nil
	codeActions, _ := caps.CodeActionProvider.(*protocol.CodeActionOptions)
	quickFixes := false
	if codeActions != nil {
		for _, kind := range codeActions.CodeActionKinds {
			quickFixes = quickFixes || kind == protocol.QuickFix
		}
	}
	experimental, _ := caps.Experimental.(map[string]any)

	advertised := map[string]bool{
		"incrementalSync":       sync != nil && sync.Change == protocol.TextDocumentSyncKindIncremental,
		"publishesQuickFixes":   quickFixes,
		"hover":                 caps.HoverProvider == true,
		"definition":            caps.DefinitionProvider == true,
		"findReferences":        caps.ReferencesProvider == true,
		"documentSymbols":       caps.DocumentSymbolProvider == true,
		"workspaceSymbols":      caps.WorkspaceSymbolProvider == true,
		"completion":            caps.CompletionProvider != nil,
		"formatting":            caps.DocumentFormattingProvider == true,
		"rename":                rename != nil,
		"semanticTokens":        tokens,
		"experimentalRendering": experimental["openSysmlRender"] == true,
	}
	for attribute, actual := range advertised {
		if declared := server.boolean(attribute); declared != actual {
			t.Errorf("surfaces.sysml says LanguageServer.%s = %v, the server advertises %v", attribute, declared, actual)
		}
	}
}

// TestSelfModelEditorPipelineMatchesImplementation compares the editor-facing
// units with the packages behind them: the token legend, the edit operations,
// and the packages that consume provenance.
func TestSelfModelEditorPipelineMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)

	highlighter := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "Highlighter")
	if declared, actual := highlighter.integer("tokenClasses"), len(highlight.Classes()); declared != actual {
		t.Errorf("surfaces.sysml says tokenClasses = %d, the legend has %d", declared, actual)
	}

	editor := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "SourceEditor")
	operations := strings.Split(editor.str("operations"), ", ")
	if declared, actual := len(operations), int(edit.OpDelete)+1; declared != actual {
		t.Errorf("surfaces.sysml lists %d edit operations, the edit package has %d", declared, actual)
	}

	provenance := instantiateSelfModel(t, idx, ctx, "surfaces.sysml", "OpenSysMLSurfaces", "Provenance")
	importPath := "\"github.com/Open-MBEE/OpenSysML/" + provenance.str("goPackage") + "\""
	for _, consumer := range strings.Split(provenance.str("consumers"), ", ") {
		if !packageImports(t, filepath.Join("..", consumer), importPath) {
			t.Errorf("surfaces.sysml says %s consumes provenance, it does not import it", consumer)
		}
	}
}

// TestSelfModelSyncMatchesImplementation compares the modelled repository
// synchronisation with the reposync package and the command line over it.
func TestSelfModelSyncMatchesImplementation(t *testing.T) {
	idx, ctx := analyseSelfModel(t)
	sync := instantiateSelfModel(t, idx, ctx, "identity.sysml", "OpenSysMLIdentity", "RepositorySync")

	if !sync.boolean("implemented") {
		t.Error("identity.sysml says repository synchronisation is not implemented; internal/interop/reposync is")
	}
	changeKinds := []string{
		string(reposync.KindCreate), string(reposync.KindUpdate), string(reposync.KindDelete), string(reposync.KindConflict),
	}
	if declared, actual := sync.str("changeKinds"), strings.Join(changeKinds, ", "); declared != actual {
		t.Errorf("identity.sysml says changeKinds = %q, the implementation has %q", declared, actual)
	}
	conflictKinds := []string{string(reposync.ConflictMissingID), string(reposync.ConflictRepositoryChanged)}
	if declared, actual := sync.str("conflictKinds"), strings.Join(conflictKinds, ", "); declared != actual {
		t.Errorf("identity.sysml says conflictKinds = %q, the implementation has %q", declared, actual)
	}
	if declared, actual := sync.str("stateFileSuffix"), strings.TrimPrefix(reposync.StatePath("m"), "m"); declared != actual {
		t.Errorf("identity.sysml says stateFileSuffix = %q, the implementation uses %q", declared, actual)
	}
	options := reflect.TypeOf(reposync.Options{})
	if _, ok := options.FieldByName("MintIDs"); ok != sync.boolean("mintsOnlyOnRequest") {
		t.Errorf("identity.sysml says mintsOnlyOnRequest = %v, reposync.Options has a MintIDs switch: %v", sync.boolean("mintsOnlyOnRequest"), ok)
	}
	if _, ok := options.FieldByName("ConfirmDeletes"); ok != sync.boolean("deletesNeedConfirmation") {
		t.Errorf("identity.sysml says deletesNeedConfirmation = %v, reposync.Options has a ConfirmDeletes switch: %v", sync.boolean("deletesNeedConfirmation"), ok)
	}

	command := instantiateSelfModel(t, idx, ctx, "identity.sysml", "OpenSysMLIdentity", "SyncCommand")
	declaredFlags := strings.Split(command.str("flags"), ", ")
	var actualFlags []string
	for _, match := range syncFlagPattern.FindAllStringSubmatch(readGoPackage(t, filepath.Join("..", command.str("goPackage"))), -1) {
		actualFlags = append(actualFlags, "-"+match[1])
	}
	sort.Strings(declaredFlags)
	sort.Strings(actualFlags)
	if !reflect.DeepEqual(declaredFlags, actualFlags) {
		t.Errorf("identity.sysml says the sync flags are %v, %s defines %v", declaredFlags, command.str("goPackage"), actualFlags)
	}
}

// TestSelfModelIdentityMatchesImplementation checks the identity model against
// the identity implementation it describes: the library file it points at, the
// metadata definitions it names, and the tier its validation pass runs at.
func TestSelfModelIdentityMatchesImplementation(t *testing.T) {
	text := readModelFile(t, "identity.sysml")

	names := []struct {
		attribute string
		actual    string
	}{
		{"elementIdDefinition", identity.ElementIdFQN},
		{"projectRefDefinition", identity.ProjectRefFQN},
	}
	for _, figure := range names {
		declared, ok := declaredString(text, figure.attribute)
		if !ok {
			t.Errorf("identity.sysml declares no %s", figure.attribute)
			continue
		}
		if declared != figure.actual {
			t.Errorf("identity.sysml says %s = %q, the implementation has %q",
				figure.attribute, declared, figure.actual)
		}
	}

	file, ok := declaredString(text, "libraryFile")
	if !ok {
		t.Fatal("identity.sysml names no identity library file")
	}
	if _, err := os.Stat(filepath.Join("..", file)); err != nil {
		t.Errorf("identity.sysml points at %s, which does not exist", file)
	}

	if level := (passes.IdentityMetadataPass{}).Level(); level != passes.LevelConstraint {
		t.Errorf("identity.sysml models the identity pass at the constraint tier, the implementation runs it at %v", level)
	}
}

// TestSelfModelDocumentPathMatchesImplementation checks the document path
// against the generator it describes: the PDF converters it lists and the
// library the document notation comes from.
func TestSelfModelDocumentPathMatchesImplementation(t *testing.T) {
	text := readModelFile(t, "surfaces.sysml")

	declared, ok := declaredString(text, "engines")
	if !ok {
		t.Fatal("surfaces.sysml lists no PDF engine")
	}
	if actual := strings.Join(docpdf.Engines(), ", "); declared != actual {
		t.Errorf("surfaces.sysml says engines = %q, the implementation has %q", declared, actual)
	}

	file, ok := declaredString(text, "libraryFile")
	if !ok {
		t.Fatal("surfaces.sysml names no document library file")
	}
	if _, err := os.Stat(filepath.Join("..", file)); err != nil {
		t.Errorf("surfaces.sysml points at %s, which does not exist", file)
	}
}

// TestSelfModelDocumentRenders renders the architecture document the model
// declares, so a query that stops binding or an embedded view that is renamed
// fails here rather than dropping a section from the rendering.
func TestSelfModelDocumentRenders(t *testing.T) {
	files := selfModelFiles(t)

	ws := model.NewWorkspace()
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(selfModelDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		ws.Open(name, content, 1)
	}

	markdown, err := ws.RenderDocumentMarkdown("OpenSysMLDocument::ArchitectureDocument")
	if err != nil {
		t.Fatalf("render the architecture document: %v", err)
	}

	// The tables come from queries and the diagrams from the views, so an
	// empty rendering of either would otherwise pass unnoticed.
	for _, want := range []string{
		"# OpenSysML Architecture",
		"| sources | internal/core/source |",
		"| notation | syntax | false |",
		"| actionEndpoints | name-resolution | true |",
		"| calcDepth | nested calculation depth | OPENSYSML\\_MAX\\_CALC\\_DEPTH | 10000 | calc recursion limit exceeded |",
		"| sequence | true |",
		"| geometry | false |",
		"| differential | pilot validator | docs/project/pilot-differential-baseline.json |",
		"| snapshotGate | the bundled library files | internal/core/libs/stdlib.snapshot |",
		"OpenSysMLViews::pipelineStructure",
		"OpenSysMLViews::libraryLoadFlow",
		"[snapshotCurrent]",
		"OpenSysMLViews::budgetExhaustion",
		"OpenSysMLViews::editorPipeline",
		"OpenSysMLViews::identityRoundTrip",
		"OpenSysMLViews::syncFlow",
	} {
		if !strings.Contains(markdown, want) {
			t.Errorf("the rendered architecture document does not contain %q", want)
		}
	}
}

// repositoryPathPattern matches the attributes whose values are paths in this
// repository, singly or as a comma-separated list.
var repositoryPathPattern = regexp.MustCompile(
	`(goPackage|generatedStubs|schemaFile|generatedGo|connectAdapter|stdioTransport|consumers|designRecord|snapshotFile) = "([^"]+)"`)

// makeTargetPattern matches the definition of one target in the Makefile.
var makeTargetPattern = regexp.MustCompile(`(?m)^([a-z-]+):`)

// syncFlagPattern matches the definition of one -sync-* flag in cmd/sysml.
var syncFlagPattern = regexp.MustCompile(`\w+\.\w+Var\(&\w+, "(sync-[a-z-]+)"`)

// analyseSelfModel indexes the self-model over the standard library and returns
// a runtime over it, so a test can instantiate and evaluate what it declares.
func analyseSelfModel(t *testing.T) (*symbols.Index, *runtime.Context) {
	t.Helper()

	idx, _ := model.NewIndexWithStdlib()
	for _, name := range selfModelFiles(t) {
		content, err := os.ReadFile(filepath.Join(selfModelDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		idx.AddDocument(name, parser.New(source.New(name, content)).ParseFile())
	}
	resolver := resolve.New(idx)
	return idx, runtime.NewContext(semantics.NewModel(resolver), resolver, 100000)
}

// modelInstance is an instantiated definition of the self-model, read by feature.
type modelInstance struct {
	t    *testing.T
	ctx  *runtime.Context
	inst *runtime.Instance
	name string
}

// instantiateSelfModel instantiates a definition of one of the self-model's packages.
func instantiateSelfModel(t *testing.T, idx *symbols.Index, ctx *runtime.Context, document, pkg, def string) *modelInstance {
	t.Helper()

	sym, ok := packageScope(t, idx, document, pkg).LookupLocal(def)
	if !ok {
		t.Fatalf("%s declares no %s", pkg, def)
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("instantiate %s::%s: %v", pkg, def, err)
	}
	return &modelInstance{t: t, ctx: ctx, inst: inst, name: pkg + "::" + def}
}

// has reports whether the instance declares the named feature.
func (m *modelInstance) has(feature string) bool {
	_, ok := m.inst.FeatureValues[feature]
	return ok
}

// value reads the named scalar feature, failing the test if it has none.
func (m *modelInstance) value(feature string) runtime.Value {
	m.t.Helper()

	fv, err := m.inst.GetFeatureValue(m.ctx, feature)
	if err != nil {
		m.t.Fatalf("%s.%s: %v", m.name, feature, err)
	}
	return fv.Value
}

// str reads a String attribute.
func (m *modelInstance) str(feature string) string {
	m.t.Helper()

	v := m.value(feature)
	if v.Kind != runtime.ValString {
		m.t.Fatalf("%s.%s is %s, not a string", m.name, feature, v.Kind)
	}
	return v.Str()
}

// integer reads an Integer attribute.
func (m *modelInstance) integer(feature string) int {
	m.t.Helper()

	v := m.value(feature)
	if v.Kind != runtime.ValConst || v.Const.Kind != semantics.ValInt {
		m.t.Fatalf("%s.%s is %s, not an integer", m.name, feature, v.Kind)
	}
	return int(v.Const.Int)
}

// boolean reads a Boolean attribute.
func (m *modelInstance) boolean(feature string) bool {
	m.t.Helper()

	v := m.value(feature)
	if v.Kind != runtime.ValConst || v.Const.Kind != semantics.ValBool {
		m.t.Fatalf("%s.%s is %s, not a boolean", m.name, feature, v.Kind)
	}
	return v.Const.Bool
}

// parts returns the instance's features that hold one object each, by name.
func (m *modelInstance) parts() map[string]*modelInstance {
	m.t.Helper()

	parts := map[string]*modelInstance{}
	for feature := range m.inst.FeatureValues {
		fv, err := m.inst.GetFeatureValue(m.ctx, feature)
		if err != nil {
			m.t.Fatalf("%s.%s: %v", m.name, feature, err)
		}
		id, ok := fv.Value.Object()
		if !ok {
			continue
		}
		inst, ok := m.ctx.Instance(id)
		if !ok {
			m.t.Fatalf("%s.%s denotes object %d, which the runtime does not hold", m.name, feature, id)
		}
		parts[feature] = &modelInstance{t: m.t, ctx: m.ctx, inst: inst, name: m.name + "." + feature}
	}
	return parts
}

// readGoPackage returns the concatenated non-test Go source of one directory.
func readGoPackage(t *testing.T, dir string) string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var text strings.Builder
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		text.Write(content)
		text.WriteByte('\n')
	}
	return text.String()
}

// packageImports reports whether the non-test Go source of a directory imports a path.
func packageImports(t *testing.T, dir, quotedImportPath string) bool {
	t.Helper()
	return strings.Contains(readGoPackage(t, dir), quotedImportPath)
}

// readModelFile returns the text of one file of the self-model.
func readModelFile(t *testing.T, name string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(selfModelDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

// declaredInteger returns the value the model text assigns to an integer attribute.
func declaredInteger(text, attribute string) (int, bool) {
	match := regexp.MustCompile(attribute + ` : Integer = (\d+);`).FindStringSubmatch(text)
	if match == nil {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	return value, err == nil
}

// declaredString returns the value the model text assigns to a string attribute.
func declaredString(text, attribute string) (string, bool) {
	match := regexp.MustCompile(attribute + `\s*:\s*String\s*=\s*\n?\s*"([^"]+)";`).FindStringSubmatch(text)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// packageScope returns the scope of a top-level package of one document.
func packageScope(t *testing.T, idx *symbols.Index, document, name string) *symbols.Scope {
	t.Helper()

	root := idx.DocumentRoot(document)
	if root == nil {
		t.Fatalf("%s is not indexed", document)
	}
	for _, child := range root.Children() {
		if child.Node() == nil {
			continue
		}
		if sym, ok := root.LookupLocal(name); ok && sym.Decl == child.Node() {
			return child
		}
	}
	t.Fatalf("package %s is not declared in %s", name, document)
	return nil
}
