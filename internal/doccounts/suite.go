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
// table the function binds to a literal before the loop, in the loop's scope. A
// loop whose trip count the source does not state there, or a `t.Run` under a
// condition, cannot be counted and is an error.
func countSubtests(fn *ast.FuncDecl) (int, error) {
	if fn.Body == nil {
		return 0, fmt.Errorf("%s has no body", fn.Name.Name)
	}
	param := fn.Type.Params.List[0]
	if len(param.Names) != 1 {
		return 0, fmt.Errorf("%s names no *testing.T", fn.Name.Name)
	}
	counter := subtestCounter{t: param.Names[0].Name}
	count, err := counter.countBlock(fn.Body.List, newTableScope(nil), 1)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", fn.Name.Name, err)
	}
	return count, nil
}

type subtestCounter struct {
	t string
}

// tableScope binds the names of one lexical scope to the length of the literal
// each holds at the point of the walk; a name assigned anything else is bound
// to unknownLength.
type tableScope struct {
	parent  *tableScope
	lengths map[string]int
}

const unknownLength = -1

func newTableScope(parent *tableScope) *tableScope {
	return &tableScope{parent: parent, lengths: map[string]int{}}
}

// define binds name in this scope, as `:=` and `var` do.
func (s *tableScope) define(name string, length int) {
	s.lengths[name] = length
}

// assign rebinds name where it is bound, as `=` does; an unbound name is bound here.
func (s *tableScope) assign(name string, length int) {
	for scope := s; scope != nil; scope = scope.parent {
		if _, ok := scope.lengths[name]; ok {
			scope.lengths[name] = length
			return
		}
	}
	s.define(name, length)
}

func (s *tableScope) lookup(name string) (int, bool) {
	for scope := s; scope != nil; scope = scope.parent {
		if length, ok := scope.lengths[name]; ok {
			return length, true
		}
	}
	return 0, false
}

// countBlock walks the statements in order, so a range sees the bindings that
// precede it and none that follow.
func (c *subtestCounter) countBlock(stmts []ast.Stmt, scope *tableScope, multiplier int) (int, error) {
	total := 0
	for _, stmt := range stmts {
		count, err := c.countStmt(stmt, scope, multiplier)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (c *subtestCounter) countStmt(stmt ast.Stmt, scope *tableScope, multiplier int) (int, error) {
	switch x := stmt.(type) {
	case *ast.BlockStmt:
		return c.countBlock(x.List, newTableScope(scope), multiplier)
	case *ast.LabeledStmt:
		return c.countStmt(x.Stmt, scope, multiplier)
	case *ast.DeclStmt:
		c.bindDecl(x, scope)
		return c.countExprs(x, scope, multiplier), nil
	case *ast.AssignStmt:
		count := c.countExprs(x, scope, multiplier)
		c.bindAssign(x, scope)
		return count, nil
	case *ast.RangeStmt:
		return c.countRange(x, scope, multiplier)
	case *ast.ForStmt:
		if c.runsSubtests(x.Body) {
			return 0, fmt.Errorf("a for loop runs subtests, and its trip count is not a table's length")
		}
		c.forgetAssigned(x, scope)
		return 0, nil
	case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		if c.runsSubtests(x) {
			return 0, fmt.Errorf("a subtest runs under a condition, so the source does not state whether it runs")
		}
		c.forgetAssigned(x, scope)
		return 0, nil
	}
	return c.countExprs(stmt, scope, multiplier), nil
}

// countRange multiplies the body's subtests by the table's length, or walks the
// body of a range that runs none only for what it rebinds.
func (c *subtestCounter) countRange(x *ast.RangeStmt, scope *tableScope, multiplier int) (int, error) {
	length := 1
	if c.runsSubtests(x.Body) {
		var err error
		if length, err = c.tableLength(x.X, scope); err != nil {
			return 0, err
		}
	}
	inner, err := c.countBlock(x.Body.List, newTableScope(scope), multiplier*length)
	return c.countExprs(x.X, scope, multiplier) + inner, err
}

// countExprs counts the `t.Run` calls in a node outside nested function
// literals; a name whose address is taken there is no longer a known table.
func (c *subtestCounter) countExprs(node ast.Node, scope *tableScope, multiplier int) int {
	total := 0
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.UnaryExpr:
			if ident, ok := x.X.(*ast.Ident); ok && x.Op == token.AND {
				scope.assign(ident.Name, unknownLength)
			}
		case *ast.CallExpr:
			if c.isRun(x) {
				total += multiplier
			}
		}
		return true
	})
	return total
}

// bindDecl binds each `var` name to its literal's length, or to unknownLength.
func (c *subtestCounter) bindDecl(decl *ast.DeclStmt, scope *tableScope) {
	gen, ok := decl.Decl.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, spec := range gen.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range value.Names {
			length := unknownLength
			if len(value.Values) == len(value.Names) {
				length = literalLength(value.Values[i])
			}
			scope.define(name.Name, length)
		}
	}
}

// bindAssign binds each assigned name to its literal's length, or to unknownLength.
func (c *subtestCounter) bindAssign(assign *ast.AssignStmt, scope *tableScope) {
	for i, lhs := range assign.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		length := unknownLength
		if len(assign.Rhs) == len(assign.Lhs) {
			length = literalLength(assign.Rhs[i])
		}
		if assign.Tok == token.DEFINE {
			scope.define(ident.Name, length)
		} else {
			scope.assign(ident.Name, length)
		}
	}
}

// forgetAssigned unbinds every name a statement the walk does not enter assigns
// or takes the address of, since the source does not state what it holds after.
func (c *subtestCounter) forgetAssigned(node ast.Node, scope *tableScope) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					scope.assign(ident.Name, unknownLength)
				}
			}
		case *ast.IncDecStmt:
			if ident, ok := x.X.(*ast.Ident); ok {
				scope.assign(ident.Name, unknownLength)
			}
		case *ast.UnaryExpr:
			if ident, ok := x.X.(*ast.Ident); ok && x.Op == token.AND {
				scope.assign(ident.Name, unknownLength)
			}
		}
		return true
	})
}

// literalLength is the element count of a composite literal, or unknownLength.
func literalLength(expr ast.Expr) int {
	if literal, ok := expr.(*ast.CompositeLit); ok {
		return len(literal.Elts)
	}
	return unknownLength
}

// tableLength is the element count of a range expression that is a composite
// literal or names one bound to a literal at this point of the function.
func (c *subtestCounter) tableLength(expr ast.Expr, scope *tableScope) (int, error) {
	switch x := expr.(type) {
	case *ast.CompositeLit:
		return len(x.Elts), nil
	case *ast.Ident:
		length, ok := scope.lookup(x.Name)
		if !ok {
			return 0, fmt.Errorf("ranges over %s, which is not a table literal the function declares", x.Name)
		}
		if length == unknownLength {
			return 0, fmt.Errorf("ranges over %s, whose length the source does not state at the loop", x.Name)
		}
		return length, nil
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
