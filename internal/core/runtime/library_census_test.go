package runtime

import (
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
	"github.com/Open-MBEE/OpenSysML/internal/doccounts"
)

var updateLibraryCensus = flag.Bool("update-library-census", false, "Rewrite docs/project/analysis-library-census.json from this run")

// libraryCensusCommand reproduces the committed census.
const libraryCensusCommand = "go test ./internal/core/runtime -run TestAnalysisLibraryCensus -update-library-census"

// censusPackage is one library package the census measures: its file in the
// bundled standard library and a representative invocation of each callable declaration.
type censusPackage struct {
	name   string
	path   string
	probes []libraryProbe
}

// libraryProbe is one declaration's representative invocation: a model using it and the
// value checked, read as a feature (read), a slot (instantiate+slot), an action output or
// a case verdict (analysis+verdict).
type libraryProbe struct {
	decl        string
	model       string
	read        string
	instantiate string
	slot        string
	action      string
	output      string
	analysis    string
	verdict     string
	want        ExpectedValue
}

// undeterminedRefusal names the refusal an Undetermined result is: the runtime
// answering that the model does not determine the value, rather than a value.
const undeterminedRefusal = "Undetermined"

// censusSentinels names the typed errors a refusal is recorded by. A refusal
// typed by none of them fails the census, so no untyped refusal is counted.
var censusSentinels = []struct {
	name string
	err  error
}{
	{"ErrActionArity", ErrActionArity},
	{"ErrActionPerformanceOccurrence", ErrActionPerformanceOccurrence},
	{"ErrAmbiguousReference", ErrAmbiguousReference},
	{"ErrBodyArity", ErrBodyArity},
	{"ErrCalcArity", ErrCalcArity},
	{"ErrCalcNoReturn", ErrCalcNoReturn},
	{"ErrIndexOutOfRange", ErrIndexOutOfRange},
	{"ErrInvalidActionFlow", ErrInvalidActionFlow},
	{"ErrInvalidInput", ErrInvalidInput},
	{"ErrMultiplicityViolation", ErrMultiplicityViolation},
	{"ErrNoResultExpression", ErrNoResultExpression},
	{"ErrNoSuchBehavior", ErrNoSuchBehavior},
	{"ErrNoSuchFeature", ErrNoSuchFeature},
	{"ErrNoValue", ErrNoValue},
	{"ErrNotABehavior", ErrNotABehavior},
	{"ErrNotACalc", ErrNotACalc},
	{"ErrNotACalcUsage", ErrNotACalcUsage},
	{"ErrNotAFunction", ErrNotAFunction},
	{"ErrNotAnAnalysis", ErrNotAnAnalysis},
	{"ErrNotAnObject", ErrNotAnObject},
	{"ErrNotAnOccurrence", ErrNotAnOccurrence},
	{"ErrOccurrenceDestroyed", ErrOccurrenceDestroyed},
	{"ErrOccurrenceLifetime", ErrOccurrenceLifetime},
	{"ErrStatePerformanceOccurrence", ErrStatePerformanceOccurrence},
	{"ErrStepLimitExceeded", ErrStepLimitExceeded},
	{"ErrTypeMismatch", ErrTypeMismatch},
	{"ErrUnboundParameter", ErrUnboundParameter},
	{"ErrUndeterminedValueType", ErrUndeterminedValueType},
	{"ErrUnevaluableLibraryFunction", ErrUnevaluableLibraryFunction},
	{"ErrUninitializedFeatureValue", ErrUninitializedFeatureValue},
	{"ErrUnknownOutput", ErrUnknownOutput},
	{"ErrUnknownParameter", ErrUnknownParameter},
	{"ErrUnresolvedReference", ErrUnresolvedReference},
	{"ErrUnresolvedType", ErrUnresolvedType},
	{"ErrUnsupportedBodyDeclaration", ErrUnsupportedBodyDeclaration},
	{"ErrUnsupportedOperator", ErrUnsupportedOperator},
	{"ErrViolated", ErrViolated},
}

// TestAnalysisLibraryCensus records, per callable library declaration, whether its
// invocation evaluated, was refused by a typed error or was wrong, and pins the committed file.
func TestAnalysisLibraryCensus(t *testing.T) {
	census := doccounts.LibraryCensus{Command: libraryCensusCommand}
	for _, pkg := range censusPackages {
		census.Packages = append(census.Packages, measureLibraryPackage(t, pkg))
	}
	if t.Failed() {
		t.Fatal("the census has probe defects above; the committed file is not compared")
	}
	got, err := doccounts.FormatLibraryCensus(census)
	if err != nil {
		t.Fatalf("census: %v", err)
	}
	path := filepath.Join("..", "..", "..", filepath.FromSlash(doccounts.LibraryCensusPath))
	if *updateLibraryCensus {
		if err := os.WriteFile(path, got, 0o644); err != nil { // #nosec G306 -- a committed documentation file
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path) // #nosec G304 -- the committed census this test maintains
	if err != nil {
		t.Fatalf("%v; run `%s`", err, libraryCensusCommand)
	}
	if string(want) != string(got) {
		t.Errorf("%s disagrees with this run; run `%s` and commit the result\n%s",
			doccounts.LibraryCensusPath, libraryCensusCommand, censusDiff(string(want), string(got)))
	}
}

// TestAnalysisLibraryCensusProbesEveryDeclaration fails a library declaration without
// a probe and a probe naming no declaration.
func TestAnalysisLibraryCensusProbesEveryDeclaration(t *testing.T) {
	ctx := newLibraryContext(t, libs.NewModelIndex())
	for _, pkg := range censusPackages {
		t.Run(pkg.name, func(t *testing.T) {
			declared := callableDeclarations(t, ctx, pkg.name)
			probed := map[string]int{}
			for _, probe := range pkg.probes {
				probed[probe.decl]++
			}
			for _, decl := range declared {
				if probed[decl] != 1 {
					t.Errorf("%s has %d probes, want 1", decl, probed[decl])
				}
				delete(probed, decl)
			}
			for decl := range probed {
				t.Errorf("%s is probed and is not a callable declaration of %s", decl, pkg.name)
			}
		})
	}
}

// measureLibraryPackage runs every probe of a package against a fresh context
// over the standard library and records the verdicts.
func measureLibraryPackage(t *testing.T, pkg censusPackage) doccounts.LibraryPackage {
	t.Helper()
	if _, err := libs.DefaultSource().Read(pkg.path); err != nil {
		t.Errorf("%s: %v", pkg.name, err)
	}
	ctx := newLibraryContext(t, libs.NewModelIndex())
	measured := doccounts.LibraryPackage{
		Name:         pkg.name,
		Path:         pkg.path,
		Declarations: callableDeclarations(t, ctx, pkg.name),
		Evaluated:    []string{},
		Refused:      []doccounts.LibraryRefusal{},
		Wrong:        []doccounts.LibraryMismatch{},
	}
	for _, probe := range pkg.probes {
		verdict := runLibraryProbe(t, probe)
		switch {
		case verdict.err != nil:
			name := sentinelName(verdict.err)
			if name == "" {
				t.Errorf("%s: refused by an error typed by no sentinel the census knows: %v", probe.decl, verdict.err)
				continue
			}
			measured.Refused = append(measured.Refused, doccounts.LibraryRefusal{
				Declaration: probe.decl, Error: name, Message: verdict.err.Error(),
			})
		case verdict.undetermined != nil:
			measured.Refused = append(measured.Refused, doccounts.LibraryRefusal{
				Declaration: probe.decl, Error: undeterminedRefusal, Message: verdict.undetermined.Reason(),
			})
		case len(verdict.problems) > 0:
			measured.Wrong = append(measured.Wrong, doccounts.LibraryMismatch{
				Declaration: probe.decl, Mismatch: strings.Join(verdict.problems, "; "),
			})
		default:
			measured.Evaluated = append(measured.Evaluated, probe.decl)
		}
	}
	return measured
}

// probeVerdict is what one probe observed: the error the runtime answered, the
// undetermined result it declined with, or the problems the check of the value reported.
type probeVerdict struct {
	err          error
	undetermined *Undetermined
	problems     []string
}

// runLibraryProbe indexes the probe's model over the standard library and evaluates the
// value it checks; a model that does not parse is a probe defect, not a verdict.
func runLibraryProbe(t *testing.T, probe libraryProbe) probeVerdict {
	t.Helper()
	path := "census/" + strings.ReplaceAll(probe.decl, "::", "/") + ".sysml"
	src := source.New(path, []byte(probe.model))
	p := parser.New(src)
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Errorf("%s: the probe model has %d parse diagnostics: %v", probe.decl, len(p.Diagnostics), p.Diagnostics)
		return probeVerdict{}
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(path, file)
	ctx := newLibraryContext(t, idx)

	var value Value
	var err error
	switch {
	case probe.read != "":
		value, err = readProbeFeature(t, ctx, idx, probe)
	case probe.instantiate != "":
		value, err = readProbeSlot(t, ctx, idx, probe)
	case probe.action != "":
		value, err = readProbeOutput(t, ctx, idx, probe)
	case probe.analysis != "":
		value, err = readProbeVerdict(t, ctx, idx, probe)
	default:
		t.Errorf("%s: the probe reads nothing", probe.decl)
		return probeVerdict{}
	}
	if err != nil {
		return probeVerdict{err: err}
	}
	if u := value.Undetermined(); u != nil {
		return probeVerdict{undetermined: u}
	}
	log := &problemLog{}
	validateValue(log, ctx, probe.decl, probe.want, value)
	return probeVerdict{problems: log.problems}
}

func readProbeFeature(t *testing.T, ctx *Context, idx *symbols.Index, probe libraryProbe) (Value, error) {
	t.Helper()
	sym := oneProbeSymbol(t, idx, probe.decl, probe.read)
	if sym == nil {
		return Value{}, nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Errorf("%s: %s binds no value to evaluate", probe.decl, probe.read)
		return Value{}, nil
	}
	return ctx.EvalWithScope(usage.Value, sym.OwnerScope)
}

func readProbeSlot(t *testing.T, ctx *Context, idx *symbols.Index, probe libraryProbe) (Value, error) {
	t.Helper()
	sym := oneProbeSymbol(t, idx, probe.decl, probe.instantiate)
	if sym == nil {
		return Value{}, nil
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return Value{}, err
	}
	fv, err := featureValueAtPath(t, ctx, inst, probe.slot)
	if err != nil {
		return Value{}, err
	}
	return fv.ReadValue(probe.slot)
}

func readProbeOutput(t *testing.T, ctx *Context, idx *symbols.Index, probe libraryProbe) (Value, error) {
	t.Helper()
	sym := oneProbeSymbol(t, idx, probe.decl, probe.action)
	if sym == nil {
		return Value{}, nil
	}
	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		return Value{}, err
	}
	value, ok := outputs[probe.output]
	if !ok {
		return Value{}, fmt.Errorf("%w: %s produced no output %q", ErrUnknownOutput, probe.action, probe.output)
	}
	return value, nil
}

// readProbeVerdict runs the case and reads the named verdict as a Boolean; an
// undecided verdict is the undetermined value its detail explains.
func readProbeVerdict(t *testing.T, ctx *Context, idx *symbols.Index, probe libraryProbe) (Value, error) {
	t.Helper()
	sym := oneProbeSymbol(t, idx, probe.decl, probe.analysis)
	if sym == nil {
		return Value{}, nil
	}
	result, err := ctx.RunAnalysis(sym, AnalysisArgs{}, nil, nil)
	if err != nil {
		return Value{}, err
	}
	for _, verdict := range result.Verdicts {
		if verdict.Name != probe.verdict {
			continue
		}
		if verdict.Status == VerdictUndecided {
			return NewUndeterminedValue(verdict.Detail, semantics.Range{}), nil
		}
		return boolValue(verdict.Status == VerdictSatisfied), nil
	}
	t.Errorf("%s: %s decided no verdict %q", probe.decl, probe.analysis, probe.verdict)
	return Value{}, nil
}

// oneProbeSymbol resolves a qualified name the probe states, reporting a probe
// defect when it names no one symbol.
func oneProbeSymbol(t *testing.T, idx *symbols.Index, decl, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Errorf("%s: %q names %d symbols, want 1", decl, fqn, len(matches))
		return nil
	}
	return matches[0]
}

func newLibraryContext(t *testing.T, idx *symbols.Index) *Context {
	t.Helper()
	resolver := resolve.New(idx)
	return NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
}

// sentinelName is the census's name for the sentinel an error is typed by, or
// "" when no known sentinel types it.
func sentinelName(err error) string {
	for _, sentinel := range censusSentinels {
		if errors.Is(err, sentinel.err) {
			return sentinel.name
		}
	}
	return ""
}

// callableDeclarations lists, in declaration order, the package's public calc, behavior,
// action, constraint, predicate and case definitions and usages at any depth.
func callableDeclarations(t *testing.T, ctx *Context, pkg string) []string {
	t.Helper()
	idx := ctx.model.resolver.Index()
	matches := idx.LookupQualified(pkg)
	if len(matches) != 1 || matches[0].Kind != symbols.SymbolPackage {
		t.Fatalf("%q names %d symbols, want one package", pkg, len(matches))
	}
	names := []string{}
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		for _, sym := range scope.AllMembers() {
			if sym.Visibility == ast.VisibilityPrivate {
				continue
			}
			if sym.Name != "" && isCallableDeclaration(sym.Decl) {
				names = append(names, ctx.qualifiedSymbolName(sym))
			}
			if sym.Scope != nil && sym.Kind != symbols.SymbolAlias {
				walk(sym.Scope)
			}
		}
	}
	walk(matches[0].Scope)
	return names
}

func isCallableDeclaration(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		switch d.Kind {
		case ast.DefCalc, ast.DefBehavior, ast.DefAction, ast.DefConstraint, ast.DefPredicate,
			ast.DefRequirement, ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	case *ast.Usage:
		if d.Direction != ast.DirNone {
			return false
		}
		switch d.Kind {
		case ast.UsageCalc, ast.UsageExpr, ast.UsageBehavior, ast.UsageAction, ast.UsagePredicate,
			ast.UsageConstraint, ast.UsageRequirement, ast.UsageObjective,
			ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
			return true
		}
	}
	return false
}

// censusDiff is the lines of the committed census and of the run that differ.
func censusDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var b strings.Builder
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var old, fresh string
		if i < len(wantLines) {
			old = wantLines[i]
		}
		if i < len(gotLines) {
			fresh = gotLines[i]
		}
		if old == fresh {
			continue
		}
		fmt.Fprintf(&b, "@@ line %d @@\n-%s\n+%s\n", i+1, old, fresh)
	}
	return b.String()
}
