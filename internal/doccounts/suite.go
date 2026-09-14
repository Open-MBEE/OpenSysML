package doccounts

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/fixtures"
)

// The directories the suite figures are counted from, relative to the root.
const (
	runtimeDir            = "internal/core/runtime"
	runtimeConformanceDir = runtimeDir + "/testdata/conformance"
	parserDir             = "internal/core/parser"
	parserGoldenDir       = parserDir + "/testdata/parse"
	grpcDir               = "internal/grpc"
	grpcConformanceDir    = grpcDir + "/testdata/conformance"
	lspDir                = "internal/lsp"
)

// SuiteCounts are the test-suite figures counted from the tree: the fixtures
// the gates enumerate and the tests the `_test.go` files declare. None of them
// needs the suite to run.
type SuiteCounts struct {
	Conformance     ConformanceCounts
	Traces          TraceCounts
	GoldenASTs      GoldenCounts
	Negatives       NegativeCounts
	Robustness      int // first-level subtests of TestRuntimeRobustness
	GRPCConformance int // cases under internal/grpc/testdata/conformance
	GRPCRobustness  int // first-level subtests of TestGRPCRobustness
	// TestFunctions counts the module's top-level `Test` functions; the other
	// two count one package's.
	TestFunctions        int
	RuntimeTestFunctions int
	LSPTestFunctions     int
}

// ConformanceCounts are the runtime conformance cases, by file-name prefix (the
// text before the first `_`), and the ones known_failures.txt lists.
type ConformanceCounts struct {
	Cases         int
	Passing       int
	Prefixes      map[string]int
	KnownFailures map[string]int
}

// Of counts the cases carrying any of the prefixes.
func (c ConformanceCounts) Of(prefixes ...string) int {
	total := 0
	for _, prefix := range prefixes {
		total += c.Prefixes[prefix]
	}
	return total
}

// PassingOf counts the cases carrying any of the prefixes that are not known failures.
func (c ConformanceCounts) PassingOf(prefixes ...string) int {
	total := c.Of(prefixes...)
	for _, prefix := range prefixes {
		total -= c.KnownFailures[prefix]
	}
	return total
}

// TraceCounts are the golden execution traces: one per case under the default
// schedule, by case prefix, and the further goldens pinning a case under a policy.
type TraceCounts struct {
	Default  int
	Policy   int
	Prefixes map[string]int
}

// GoldenCounts are the parser's golden AST fixtures by notation.
type GoldenCounts struct {
	Total int
	SysML int
	KerML int
}

// NegativeCounts are the parser's first-level negative subtests: the table of
// TestNegative, every `TestNegative*` function, those of them whose name says
// KerML, and every parser test whose name says Negative.
type NegativeCounts struct {
	Table    int
	Prefixed int
	KerML    int
	All      int
}

// ReadSuiteCounts counts every suite figure from the tree under root.
func ReadSuiteCounts(root string) (SuiteCounts, error) {
	var counts SuiteCounts
	var err error
	conformanceDir := filepath.Join(root, filepath.FromSlash(runtimeConformanceDir))
	if counts.Conformance, err = readConformanceCounts(conformanceDir); err != nil {
		return counts, err
	}
	if counts.Traces, err = readTraceCounts(conformanceDir); err != nil {
		return counts, err
	}
	if counts.GoldenASTs, err = readGoldenCounts(filepath.Join(root, filepath.FromSlash(parserGoldenDir))); err != nil {
		return counts, err
	}
	parserTests, err := parseTestFiles(filepath.Join(root, filepath.FromSlash(parserDir)))
	if err != nil {
		return counts, err
	}
	if counts.Negatives, err = readNegativeCounts(parserTests); err != nil {
		return counts, err
	}
	runtimeTests, err := parseTestFiles(filepath.Join(root, filepath.FromSlash(runtimeDir)))
	if err != nil {
		return counts, err
	}
	if counts.Robustness, err = countSubtestsOf(runtimeTests, "TestRuntimeRobustness"); err != nil {
		return counts, err
	}
	counts.RuntimeTestFunctions = countTestFunctions(runtimeTests)
	grpcCases, err := fixtures.Cases(filepath.Join(root, filepath.FromSlash(grpcConformanceDir)))
	if err != nil {
		return counts, err
	}
	counts.GRPCConformance = len(grpcCases)
	grpcTests, err := parseTestFiles(filepath.Join(root, filepath.FromSlash(grpcDir)))
	if err != nil {
		return counts, err
	}
	if counts.GRPCRobustness, err = countSubtestsOf(grpcTests, "TestGRPCRobustness"); err != nil {
		return counts, err
	}
	lspTests, err := parseTestFiles(filepath.Join(root, filepath.FromSlash(lspDir)))
	if err != nil {
		return counts, err
	}
	counts.LSPTestFunctions = countTestFunctions(lspTests)
	if counts.TestFunctions, err = countModuleTestFunctions(root); err != nil {
		return counts, err
	}
	return counts, nil
}

func readConformanceCounts(dir string) (ConformanceCounts, error) {
	cases, err := fixtures.Cases(dir)
	if err != nil {
		return ConformanceCounts{}, err
	}
	if len(cases) == 0 {
		return ConformanceCounts{}, fmt.Errorf("%s: no conformance cases", dir)
	}
	knownFailures, err := fixtures.KnownFailures(dir)
	if err != nil {
		return ConformanceCounts{}, fmt.Errorf("%s: %w", dir, err)
	}
	counts := ConformanceCounts{Prefixes: map[string]int{}, KnownFailures: map[string]int{}}
	for _, name := range cases {
		counts.Cases++
		counts.Prefixes[casePrefix(name)]++
		if knownFailures[name] {
			counts.KnownFailures[casePrefix(name)]++
			continue
		}
		counts.Passing++
	}
	for name := range knownFailures {
		if !containsCase(cases, name) {
			return ConformanceCounts{}, fmt.Errorf("%s: known_failures.txt lists %q, which is no case", dir, name)
		}
	}
	return counts, nil
}

// casePrefix is the text before a case name's first `_`: its family.
func casePrefix(name string) string {
	prefix, _, _ := strings.Cut(name, "_")
	return prefix
}

func containsCase(sorted []string, name string) bool {
	i := sort.SearchStrings(sorted, name)
	return i < len(sorted) && sorted[i] == name
}

// readTraceCounts counts the goldens the trace harness reads, a case's own
// (`<case>.trace.golden`) apart from those pinning it under a sweep policy.
func readTraceCounts(dir string) (TraceCounts, error) {
	traces, err := fixtures.Traces(dir)
	if err != nil {
		return TraceCounts{}, fmt.Errorf("%s: %w", dir, err)
	}
	counts := TraceCounts{Prefixes: map[string]int{}}
	for _, trace := range traces {
		if trace.Policy != "" {
			counts.Policy++
			continue
		}
		counts.Default++
		counts.Prefixes[casePrefix(trace.Case)]++
	}
	if counts.Default == 0 {
		return TraceCounts{}, fmt.Errorf("%s: no golden traces", dir)
	}
	return counts, nil
}

// readGoldenCounts counts the fixtures TestGolden parses: every `.sysml` and
// `.kerml` under the parse directory, each of which must own a `.golden`.
func readGoldenCounts(dir string) (GoldenCounts, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return GoldenCounts{}, err
	}
	var counts GoldenCounts
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if entry.IsDir() || (ext != ".sysml" && ext != ".kerml") {
			continue
		}
		golden := strings.TrimSuffix(entry.Name(), ext) + ".golden"
		if _, err := os.Stat(filepath.Join(dir, golden)); err != nil {
			return GoldenCounts{}, fmt.Errorf("%s: %s owns no %s", dir, entry.Name(), golden)
		}
		counts.Total++
		if ext == ".sysml" {
			counts.SysML++
		} else {
			counts.KerML++
		}
	}
	if counts.Total == 0 {
		return GoldenCounts{}, fmt.Errorf("%s: no golden AST fixtures", dir)
	}
	return counts, nil
}

func readNegativeCounts(files []*ast.File) (NegativeCounts, error) {
	var counts NegativeCounts
	var err error
	if counts.Table, err = countSubtestsOf(files, "TestNegative"); err != nil {
		return counts, err
	}
	for _, fn := range testFunctions(files) {
		name := fn.Name.Name
		if !strings.Contains(name, "Negative") {
			continue
		}
		subtests, err := countSubtests(fn)
		if err != nil {
			return counts, err
		}
		counts.All += subtests
		if strings.HasPrefix(name, "TestNegative") {
			counts.Prefixed += subtests
			if strings.Contains(name, "KerML") {
				counts.KerML += subtests
			}
		}
	}
	return counts, nil
}

// parseTestFiles parses the `_test.go` files of one directory.
func parseTestFiles(dir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s: no _test.go files", dir)
	}
	return files, nil
}

// countModuleTestFunctions counts the top-level Test functions `go test ./...`
// would run: every `_test.go` file under root outside testdata, vendor, hidden
// and underscore directories, and outside any nested module.
func countModuleTestFunctions(root string) (int, error) {
	total := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			name := entry.Name()
			if name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		total += countTestFunctions([]*ast.File{file})
		return nil
	})
	if err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, fmt.Errorf("%s: no Test functions", root)
	}
	return total, nil
}

// testFunctions lists the top-level Test functions of the files, in file order.
func testFunctions(files []*ast.File) []*ast.FuncDecl {
	var tests []*ast.FuncDecl
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && isTestFunction(fn) {
				tests = append(tests, fn)
			}
		}
	}
	return tests
}

func countTestFunctions(files []*ast.File) int {
	return len(testFunctions(files))
}

// isTestFunction applies `go test`'s rule: a top-level `TestXxx(t *testing.T)`
// whose Xxx does not start with a lower-case letter.
func isTestFunction(fn *ast.FuncDecl) bool {
	if fn.Recv != nil || fn.Type.Results != nil || len(fn.Type.Params.List) != 1 {
		return false
	}
	param := fn.Type.Params.List[0]
	if len(param.Names) > 1 || !isTestingT(param.Type) {
		return false
	}
	rest, ok := strings.CutPrefix(fn.Name.Name, "Test")
	if !ok {
		return false
	}
	first, _ := utf8.DecodeRuneInString(rest)
	return rest == "" || !unicode.IsLower(first)
}

func isTestingT(expr ast.Expr) bool {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := star.X.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "T" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "testing"
}

func countSubtestsOf(files []*ast.File, name string) (int, error) {
	for _, fn := range testFunctions(files) {
		if fn.Name.Name == name {
			return countSubtests(fn)
		}
	}
	return 0, fmt.Errorf("no test function %s", name)
}

// countSubtests counts the first-level subtests a test runs: each `t.Run` in its
// body outside nested function literals, multiplied out over a `range` of a
// table the function declares as a literal. A loop whose trip count the source
// does not state, or a `t.Run` under a condition, cannot be counted and is an error.
func countSubtests(fn *ast.FuncDecl) (int, error) {
	if fn.Body == nil {
		return 0, fmt.Errorf("%s has no body", fn.Name.Name)
	}
	param := fn.Type.Params.List[0]
	if len(param.Names) != 1 {
		return 0, fmt.Errorf("%s names no *testing.T", fn.Name.Name)
	}
	counter := subtestCounter{t: param.Names[0].Name, tables: map[string]int{}}
	counter.collectTables(fn.Body)
	count, err := counter.count(fn.Body, 1)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", fn.Name.Name, err)
	}
	return count, nil
}

type subtestCounter struct {
	t      string
	tables map[string]int
}

// collectTables records the length of every composite literal the body binds
// to a name outside nested function literals.
func (c *subtestCounter) collectTables(body ast.Node) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if len(x.Lhs) == 1 && len(x.Rhs) == 1 {
				c.recordTable(x.Lhs[0], x.Rhs[0])
			}
		case *ast.ValueSpec:
			if len(x.Names) == 1 && len(x.Values) == 1 {
				c.recordTable(x.Names[0], x.Values[0])
			}
		}
		return true
	})
}

func (c *subtestCounter) recordTable(name ast.Expr, value ast.Expr) {
	ident, ok := name.(*ast.Ident)
	literal, isLiteral := value.(*ast.CompositeLit)
	if ok && isLiteral {
		c.tables[ident.Name] = len(literal.Elts)
	}
}

func (c *subtestCounter) count(body ast.Node, multiplier int) (int, error) {
	total := 0
	var failure error
	ast.Inspect(body, func(n ast.Node) bool {
		if failure != nil {
			return false
		}
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.RangeStmt:
			if !c.runsSubtests(x.Body) {
				return true
			}
			length, err := c.tableLength(x.X)
			if err != nil {
				failure = err
				return false
			}
			inner, err := c.count(x.Body, multiplier*length)
			if err != nil {
				failure = err
				return false
			}
			total += inner
			return false
		case *ast.ForStmt:
			if c.runsSubtests(x.Body) {
				failure = fmt.Errorf("a for loop runs subtests, and its trip count is not a table's length")
				return false
			}
		case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			if c.runsSubtests(x) {
				failure = fmt.Errorf("a subtest runs under a condition, so the source does not state whether it runs")
				return false
			}
			return false
		case *ast.CallExpr:
			if c.isRun(x) {
				total += multiplier
			}
		}
		return true
	})
	return total, failure
}

// tableLength is the element count of a range expression that is a composite
// literal or names one the function declared.
func (c *subtestCounter) tableLength(expr ast.Expr) (int, error) {
	switch x := expr.(type) {
	case *ast.CompositeLit:
		return len(x.Elts), nil
	case *ast.Ident:
		if length, ok := c.tables[x.Name]; ok {
			return length, nil
		}
		return 0, fmt.Errorf("ranges over %s, which is not a table literal the function declares", x.Name)
	}
	return 0, fmt.Errorf("ranges over an expression that is not a table literal")
}

func (c *subtestCounter) runsSubtests(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok && c.isRun(call) {
			found = true
		}
		return !found
	})
	return found
}

// isRun recognises `t.Run(...)` on the function's own *testing.T.
func (c *subtestCounter) isRun(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Run" {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == c.t
}

// Breakdown spells counts by prefix the way the inventory reads them: `calc×175`
// in descending order for four and more, then `three each of a, b and c`, `two
// each of …` and `one each of …`; a group of one is spelt `a×3`.
func Breakdown(counts map[string]int) string {
	type entry struct {
		name  string
		count int
	}
	var entries []entry
	for name, count := range counts {
		entries = append(entries, entry{name, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].name < entries[j].name
	})
	var parts []string
	groups := map[int][]string{}
	for _, e := range entries {
		if e.count >= 4 {
			parts = append(parts, e.name+"×"+strconv.Itoa(e.count))
			continue
		}
		groups[e.count] = append(groups[e.count], e.name)
	}
	grouped := false
	for _, count := range []int{3, 2, 1} {
		names := groups[count]
		switch len(names) {
		case 0:
		case 1:
			parts = append(parts, names[0]+"×"+strconv.Itoa(count))
		default:
			grouped = true
			parts = append(parts, []string{"", "one", "two", "three"}[count]+" each of "+andList(names))
		}
	}
	if grouped {
		parts[len(parts)-1] = "and " + parts[len(parts)-1]
	}
	return strings.Join(parts, ", ")
}

// andList joins names as `a, b and c`.
func andList(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// Thousands formats a count with a thousands separator, as the prose spells it.
func Thousands(n int) string {
	digits := strconv.Itoa(n)
	var b strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(digit)
	}
	return b.String()
}
