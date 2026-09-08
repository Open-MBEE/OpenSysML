package runtime

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// differentialRoots are the model trees whose calcs the differential test
// invokes on both tiers; the OMG corpora are covered when they are downloaded
// under examples/.
var differentialRoots = []string{
	filepath.Join("testdata", "conformance"),
	filepath.Join("testdata", "compiled"),
	filepath.Join("..", "..", "..", "testdata"),
	filepath.Join("..", "..", "..", "examples"),
	filepath.Join("..", "..", "..", "docs", "manual", "examples"),
}

// differentialArgs are the scalars every parameter is tried with: Integer
// extremes, Reals of either sign, both zeros, both infinities, NaN, both Booleans.
var differentialArgs = []Value{
	intArg(0), intArg(1), intArg(-1), intArg(math.MaxInt64), intArg(math.MinInt64),
	realArg(0), realArg(math.Copysign(0, -1)), realArg(1.5), realArg(-1e300),
	realArg(math.Inf(1)), realArg(math.Inf(-1)), realArg(math.NaN()),
	boolArg(true), boolArg(false),
}

// differentialMaxSteps bounds one invocation, so a body that recurses on an
// extreme argument stops at the budget on both tiers rather than at the depth.
const differentialMaxSteps int64 = 20000

// TestCompiledCalcDifferential invokes every eligible calc through both tiers on
// generated arguments, by position and by name, requiring identical outcomes.
func TestCompiledCalcDifferential(t *testing.T) {
	var files []string
	for _, root := range differentialRoots {
		files = append(files, sysmlFilesUnder(t, root)...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no .sysml files under the differential roots")
	}

	var eligible, ineligible, invocations int
	reasons := map[string]int{}
	for _, path := range files {
		e, n := differentialFile(t, path, reasons)
		eligible += e
		invocations += n
	}
	for _, n := range reasons {
		ineligible += n
	}
	if eligible == 0 {
		t.Fatal("no calc compiled in any file")
	}
	t.Logf("%d files: %d calcs eligible, %d ineligible (%.1f%% eligible), %d invocations compared",
		len(files), eligible, ineligible, 100*float64(eligible)/float64(eligible+ineligible), invocations)
	for _, reason := range reasonsByCount(reasons) {
		t.Logf("%5d ineligible: %s", reasons[reason], reason)
	}
}

// reasonsByCount orders the ineligibility reasons most frequent first.
func reasonsByCount(reasons map[string]int) []string {
	ordered := make([]string, 0, len(reasons))
	for reason := range reasons {
		ordered = append(ordered, reason)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if reasons[ordered[i]] != reasons[ordered[j]] {
			return reasons[ordered[i]] > reasons[ordered[j]]
		}
		return ordered[i] < ordered[j]
	})
	return ordered
}

// sysmlFilesUnder lists the .sysml files under root, none when root is absent.
func sysmlFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".sysml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walking %s: %v", root, err)
	}
	return files
}

// differentialFile builds path once and runs its calcs on a compiled and an
// evaluating context over the same model, returning the eligible count and the
// number of invocations compared; each ineligible calc is tallied by reason.
func differentialFile(t *testing.T, path string, reasons map[string]int) (eligible, invocations int) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(path, parser.New(source.New(path, src)).ParseFile())
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	model := semantics.NewModel(resolver)
	compiled := NewContext(model, resolver, differentialMaxSteps)
	compiled.SetCalcCompile(true)
	reference := NewContext(model, resolver, differentialMaxSteps)
	reference.SetCalcCompile(false)

	root := idx.DocumentRoot(path)
	for _, sym := range calcSymbolsUnder(root) {
		shape, err := compiled.calcShapeOf(sym)
		if err != nil {
			continue
		}
		body := compiled.compiledCalcOf(shape)
		if body == nil {
			reasons[generalizeReason(shape.ineligibleWhy)]++
			continue
		}
		eligible++
		name := path + ": " + shape.Name
		for _, args := range differentialVectors(len(body.params)) {
			got := calcOutcome{steps: -1}
			got.value, got.err = compiled.InvokeCalc(sym, args, root)
			got.steps = compiled.run.steps
			want := calcOutcome{}
			want.value, want.err = reference.InvokeCalc(sym, args, root)
			want.steps = reference.run.steps
			wantOutcomesEqual(t, name+describeArgs(args), got, want)
			invocations++
			if len(args) == 0 {
				continue
			}
			named := make(map[string]Value, len(args))
			for i, arg := range args {
				named[shape.ParamNames[i]] = arg
			}
			got = calcOutcome{steps: -1}
			got.value, got.err = compiled.InvokeCalcNamed(sym, named, root)
			got.steps = compiled.run.steps
			want = calcOutcome{}
			want.value, want.err = reference.InvokeCalcNamed(sym, named, root)
			want.steps = reference.run.steps
			wantOutcomesEqual(t, name+describeNamedArgs(named), got, want)
			invocations++
		}
	}
	return eligible, invocations
}

// quotedName matches the name a reason quotes.
var quotedName = regexp.MustCompile(`"[^"]*"|'[^']*'`)

// generalizeReason blanks the names a reason quotes, so reasons tally by kind.
func generalizeReason(why string) string {
	return quotedName.ReplaceAllString(why, "_")
}

// calcSymbolsUnder collects every calc definition and usage declared in scope
// and the scopes nested in it, in a stable order.
func calcSymbolsUnder(scope *symbols.Scope) []*symbols.Symbol {
	var calcs []*symbols.Symbol
	var walk func(*symbols.Scope)
	walk = func(s *symbols.Scope) {
		for _, name := range s.MemberNames() {
			if sym, ok := s.LookupLocal(name); ok && sym != nil && isCalcDecl(sym.Decl) {
				calcs = append(calcs, sym)
			}
		}
		for _, child := range s.Children() {
			walk(child)
		}
	}
	walk(scope)
	return calcs
}

// differentialVectors generates the argument vectors for a calc of arity n: the
// full product of differentialArgs up to three parameters, and past that every
// argument in every position with its neighbours rotated, so each parameter
// still meets every scalar.
func differentialVectors(n int) [][]Value {
	if n == 0 {
		return [][]Value{nil}
	}
	if n <= 3 {
		var vectors [][]Value
		for _, tail := range differentialVectors(n - 1) {
			for _, head := range differentialArgs {
				vectors = append(vectors, append([]Value{head}, tail...))
			}
		}
		return vectors
	}
	var vectors [][]Value
	for c := range differentialArgs {
		for rotation := 0; rotation < 3; rotation++ {
			args := make([]Value, n)
			for j := range args {
				args[j] = differentialArgs[(c+j*rotation)%len(differentialArgs)]
			}
			vectors = append(vectors, args)
		}
	}
	return vectors
}

func describeArgs(args []Value) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = FormatTraceValue(arg)
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
