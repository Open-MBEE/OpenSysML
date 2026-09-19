package doccounts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/census/doccounts/doccountstest"
)

func TestReadSuiteCountsCountsTheTreeAsTheGatesDo(t *testing.T) {
	root := t.TempDir()
	doccountstest.WriteSuiteFixture(t, root)
	counts, err := ReadSuiteCounts(root)
	if err != nil {
		t.Fatalf("read suite counts: %v", err)
	}
	want := SuiteCounts{
		Conformance: ConformanceCounts{
			Cases: 7, Passing: 7,
			Prefixes:      map[string]int{"calc": 4, "action": 1, "state": 1, "send": 1},
			KnownFailures: map[string]int{},
		},
		Traces:               TraceCounts{Default: 3, Policy: 1, Prefixes: map[string]int{"calc": 2, "action": 1}},
		GoldenASTs:           GoldenCounts{Total: 3, SysML: 2, KerML: 1},
		Negatives:            NegativeCounts{Table: 3, Prefixed: 5, KerML: 2, All: 6},
		Robustness:           7,
		GRPCConformance:      2,
		GRPCRobustness:       3,
		TestFunctions:        10,
		RuntimeTestFunctions: 3,
		LSPTestFunctions:     1,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("suite counts\n got %+v\nwant %+v", counts, want)
	}
}

func TestReadSuiteCountsReportsAKnownFailureAsNotPassing(t *testing.T) {
	root := t.TempDir()
	doccountstest.WriteSuiteFixture(t, root)
	doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/known_failures.txt", "# pinned\ncalc_a\n")
	counts, err := ReadSuiteCounts(root)
	if err != nil {
		t.Fatalf("read suite counts: %v", err)
	}
	if counts.Conformance.Cases != 7 || counts.Conformance.Passing != 6 || counts.Conformance.KnownFailures["calc"] != 1 {
		t.Fatalf("known failure not counted: %+v", counts.Conformance)
	}
	if counts.Traces.Default != 2 || counts.Traces.Policy != 0 || counts.Traces.Prefixes["calc"] != 1 {
		t.Fatalf("the known failure's traces are counted, which the harness skips: %+v", counts.Traces)
	}
	figures := figuresOf(counts)
	if figures.AllPassing || figures.KnownFailures != 1 {
		t.Fatalf("figures do not state the known failure: %+v", figures)
	}
	got, err := renderBlock(Block{Path: ReadmePath, Name: readmeConformanceBlock}, Figures{Suite: counts})
	if err != nil {
		t.Fatal(err)
	}
	if want := "every conformance case passing but the 1 `known_failures.txt` lists"; got != want {
		t.Fatalf("conformance-passing renders %q, want %q", got, want)
	}
}

// TestReadSuiteCountsRejectsWhatNoGateWouldRead keeps a stray file from being
// counted as a fixture: each is an error naming the file.
func TestReadSuiteCountsRejectsWhatNoGateWouldRead(t *testing.T) {
	for name, mutate := range map[string]func(t *testing.T, root string){
		"a known failure that is no case": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/known_failures.txt", "ghost_case\n")
		},
		"a trace owned by no case": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/ghost.trace.golden", "trace\n")
		},
		"a trace under no sweep policy": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/calc_a.typo.trace.golden", "trace\n")
		},
		"a policy trace of a case admitting one outcome": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/calc_b.declared.trace.golden", "trace\n")
		},
		"a policy trace of a case owning no default golden": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/state_a.expected.json", `{"outcomes": [{}, {}]}`+"\n")
			doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/state_a.seed-1.trace.golden", "trace\n")
		},
		"a parse fixture with no golden": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "tests/parser/testdata/parse/orphan.sysml", "package P;\n")
		},
		"a robustness loop the source does not bound": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/robustness_test.go", `package runtime

import "testing"

func TestRuntimeRobustness(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run("x", func(t *testing.T) {})
	}
}
`)
		},
		"a robustness range over an unknown table": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/robustness_test.go", `package runtime

import "testing"

func TestRuntimeRobustness(t *testing.T) {
	for _, tc := range cases() {
		t.Run(tc, func(t *testing.T) {})
	}
}

func cases() []string { return nil }
`)
		},
		"a robustness subtest under a condition": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/robustness_test.go", `package runtime

import (
	"runtime"
	"testing"
)

func TestRuntimeRobustness(t *testing.T) {
	t.Run("a", func(t *testing.T) {})
	if runtime.GOOS == "windows" {
		t.Run("windows", func(t *testing.T) {})
	}
}
`)
		},
		"a robustness subtest under a switch": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/robustness_test.go", `package runtime

import "testing"

func TestRuntimeRobustness(t *testing.T) {
	switch len(t.Name()) {
	case 0:
		t.Run("never", func(t *testing.T) {})
	}
}
`)
		},
		"no robustness test": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/exec/runtime/robustness_test.go", "package runtime\n")
			doccountstest.Write(t, root, "internal/exec/runtime/robustness_signals_test.go", "package runtime\n")
		},
		"no conformance cases": func(t *testing.T, root string) {
			dir := filepath.Join(root, "internal", "exec", "runtime", "testdata", "conformance")
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			doccountstest.Write(t, root, "internal/exec/runtime/testdata/conformance/README.md", "empty\n")
		},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			doccountstest.WriteSuiteFixture(t, root)
			mutate(t, root)
			if _, err := ReadSuiteCounts(root); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

// TestCountSubtestsReadsTablesInStatementOrder binds a range to the literal its
// table holds at the loop, not to the last one the function assigns anywhere.
func TestCountSubtestsReadsTablesInStatementOrder(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want int
		err  string
	}{
		"a table reassigned after the loop": {
			body: `cases := []string{"a", "b"}
	for range cases {
		t.Run("x", nil)
	}
	cases = []string{"c", "d", "e"}
	_ = cases`,
			want: 2,
		},
		"a table reassigned between two loops": {
			body: `cases := []string{"a", "b"}
	for range cases {
		t.Run("x", nil)
	}
	cases = []string{"c", "d", "e"}
	for range cases {
		t.Run("y", nil)
	}`,
			want: 5,
		},
		"a table shadowed in an inner block": {
			body: `cases := []string{"a", "b"}
	{
		cases := []string{"c"}
		for range cases {
			t.Run("inner", nil)
		}
	}
	for range cases {
		t.Run("outer", nil)
	}`,
			want: 3,
		},
		"a var table and a literal range": {
			body: `var cases = []int{1, 2, 3}
	for range cases {
		for range []int{1, 2} {
			t.Run("x", nil)
		}
	}`,
			want: 6,
		},
		"a table shadowed by := under a condition": {
			body: `cases := []string{"a"}
	if len(t.Name()) > 0 {
		cases := []string{"x", "y"}
		_ = cases
	}
	for range cases {
		t.Run("x", nil)
	}`,
			want: 1,
		},
		"a table shadowed in a switch clause and an if init": {
			body: `cases := []string{"a", "b"}
	switch cases := []string{"x"}; len(cases) {
	case 1:
		cases := []string{"y", "z", "w"}
		_ = cases
	}
	if cases := []string{"q"}; len(cases) > 0 {
		_ = cases
	}
	for range cases {
		t.Run("x", nil)
	}`,
			want: 2,
		},
		"a table shadowed by a range variable": {
			body: `cases := []string{"a", "b"}
	for _, cases := range [][]string{{"x"}} {
		_ = cases
	}
	for range cases {
		t.Run("x", nil)
	}`,
			want: 2,
		},
		"a table reassigned in a loop that runs no subtest": {
			body: `cases := []string{"a"}
	for range []int{1} {
		cases = []string{"x", "y"}
	}
	for range cases {
		t.Run("x", nil)
	}`,
			err: "whose length the source does not state",
		},
		"a table appended to before the loop": {
			body: `cases := []string{"a"}
	cases = append(cases, "b")
	for range cases {
		t.Run("x", nil)
	}`,
			err: "whose length the source does not state",
		},
		"a table reassigned under a condition": {
			body: `cases := []string{"a"}
	if len(t.Name()) > 0 {
		cases = []string{"b", "c"}
	}
	for range cases {
		t.Run("x", nil)
	}`,
			err: "whose length the source does not state",
		},
		"a table whose address is taken": {
			body: `cases := []string{"a"}
	grow(&cases)
	for range cases {
		t.Run("x", nil)
	}`,
			err: "whose length the source does not state",
		},
		"a table declared after the loop": {
			body: `for range cases {
		t.Run("x", nil)
	}
	cases := []string{"a"}
	_ = cases`,
			err: "not a table literal the function declares",
		},
	} {
		t.Run(name, func(t *testing.T) {
			src := "package p\n\nimport \"testing\"\n\nvar cases []string\n\nfunc grow(c *[]string) {}\n\nfunc TestIt(t *testing.T) {\n\t" + tc.body + "\n}\n"
			file, err := parser.ParseFile(token.NewFileSet(), "p_test.go", src, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			got, err := countSubtestsOf([]*ast.File{file}, "TestIt")
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want one saying %q", err, tc.err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("count = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
}

func TestIsTestFunctionAppliesGoTestsRule(t *testing.T) {
	src := `package p

import "testing"

func TestA(t *testing.T) {}
func Test(t *testing.T) {}
func Test_underscore(t *testing.T) {}
func Testlower(t *testing.T) {}
func TestTwo(t *testing.T, u int) {}
func TestReturns(t *testing.T) error { return nil }
func TestBench(b *testing.B) {}
func (r receiver) TestMethod(t *testing.T) {}
func helper(t *testing.T) {}
type receiver struct{}
`
	file, err := parser.ParseFile(token.NewFileSet(), "p_test.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, fn := range testFunctions([]*ast.File{file}) {
		names = append(names, fn.Name.Name)
	}
	if got := strings.Join(names, ","); got != "TestA,Test,Test_underscore" {
		t.Fatalf("test functions = %s", got)
	}
}

func TestBreakdownSpellsTheInventory(t *testing.T) {
	for _, tc := range []struct {
		counts map[string]int
		want   string
	}{
		{map[string]int{"calc": 175, "state": 167, "unit": 7}, "calc×175, state×167, unit×7"},
		{map[string]int{"b": 4, "a": 4}, "a×4, b×4"},
		{map[string]int{"calc": 5, "x": 3, "y": 3, "z": 2, "w": 1}, "calc×5, three each of x and y, z×2, and w×1"},
		{map[string]int{"p": 1, "q": 1, "r": 2, "s": 2}, "two each of r and s, and one each of p and q"},
		{map[string]int{"only": 3}, "only×3"},
		{map[string]int{}, ""},
	} {
		if got := Breakdown(tc.counts); got != tc.want {
			t.Errorf("Breakdown(%v) = %q, want %q", tc.counts, got, tc.want)
		}
	}
}

func TestThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 22637: "22,637", 1234567: "1,234,567"} {
		if got := Thousands(n); got != want {
			t.Errorf("Thousands(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSuiteBlocksRenderInlineSentences(t *testing.T) {
	root := t.TempDir()
	doccountstest.WriteSuiteFixture(t, root)
	suite, err := ReadSuiteCounts(root)
	if err != nil {
		t.Fatalf("read suite counts: %v", err)
	}
	figures := Figures{Suite: suite}
	for _, block := range append(suiteBlocks(), siteSuiteBlocks()...) {
		content := "| row | before <!-- doc-counts:begin " + block.Name + " -->stale<!-- doc-counts:end " + block.Name + " --> after |\n"
		got, err := RewriteBlock(content, block, figures)
		if err != nil {
			t.Fatalf("%s: %v", block.Name, err)
		}
		if strings.Contains(got, "stale") || !strings.HasPrefix(got, "| row | before <!-- doc-counts:begin ") || !strings.HasSuffix(got, " --> after |\n") {
			t.Fatalf("%s did not rewrite within the line:\n%s", block.Name, got)
		}
		if strings.Count(got, "\n") != 1 {
			t.Fatalf("%s spilt over the line:\n%s", block.Name, got)
		}
		again, err := RewriteBlock(got, block, figures)
		if err != nil || again != got {
			t.Fatalf("%s is not idempotent: %v", block.Name, err)
		}
	}
	for name, want := range map[string]string{
		inventoryConformanceBlock: doccountstest.Expected.ConformanceSummary,
		inventoryTracesBlock:      doccountstest.Expected.TraceSummary,
		readmeConformanceBlock:    "every conformance case passing",
		inventoryRobustnessBlock:  "7 runtime robustness cases (first-level subtests across the `TestRuntimeRobustness*` functions)",
		inventoryGRPCBlock:        "2 gRPC conformance cases and 3 gRPC robustness cases",
		inventoryNegativesBlock:   "3 negative parser subtests (first-level subtests of `TestNegative`; 5 across the `TestNegative*` functions, 2 of them KerML, and 6 across every `*Negative*` parser test)",
		inventoryTestsBlock:       "10 top-level `Test` functions across the module",
	} {
		got, err := renderBlock(Block{Path: ReadmePath, Name: name}, figures)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(got, want) {
			t.Errorf("%s renders %q, want it to contain %q", name, got, want)
		}
	}
}

// TestEveryBlockNamesATemplate keeps a registered block from being reported as
// unrenderable at generation time.
func TestEveryBlockNamesATemplate(t *testing.T) {
	for _, block := range append(Blocks(), SiteBlocks()...) {
		if _, ok := blockTemplates[block.Name]; !ok {
			t.Errorf("block %q on %s has no template", block.Name, block.Path)
		}
	}
}

// TestSiteBlocksAreNotCommittedBlocks keeps a block from being both rewritten
// into the tree and rendered by the build.
func TestSiteBlocksAreNotCommittedBlocks(t *testing.T) {
	committed := map[string]bool{}
	for _, block := range Blocks() {
		committed[block.Path+"#"+block.Name] = true
	}
	for _, block := range SiteBlocks() {
		if committed[block.Path+"#"+block.Name] {
			t.Errorf("block %q on %s is both committed and rendered by the build", block.Name, block.Path)
		}
	}
	if got := SitePaths(); !reflect.DeepEqual(got, []string{SpecCompliancePath}) {
		t.Errorf("SitePaths() = %v", got)
	}
}

func TestRenderSiteBlocksRendersEveryBlockByPage(t *testing.T) {
	root := t.TempDir()
	doccountstest.WriteSuiteFixture(t, root)
	suite, err := ReadSuiteCounts(root)
	if err != nil {
		t.Fatalf("read suite counts: %v", err)
	}
	rendered, err := RenderSiteBlocks(Figures{Suite: suite})
	if err != nil {
		t.Fatal(err)
	}
	if len(rendered) != 1 || len(rendered[SpecCompliancePath]) != len(SiteBlocks()) {
		t.Fatalf("rendered %d pages, %d blocks", len(rendered), len(rendered[SpecCompliancePath]))
	}
	if got := rendered[SpecCompliancePath][inventoryConformanceBlock]; got != doccountstest.Expected.ConformanceSummary {
		t.Fatalf("inventory-conformance renders %q", got)
	}
}

func TestCheckSiteBlockAcceptsASentenceAndRefusesAFigure(t *testing.T) {
	block := Block{Path: SpecCompliancePath, Name: inventoryTestsBlock}
	for content, wantErr := range map[string]bool{
		"- Tests: <!-- doc-counts:begin inventory-tests -->top-level `Test` functions across the module<!-- doc-counts:end inventory-tests -->\n":       false,
		"<!-- doc-counts:begin inventory-tests -->\ntop-level `Test` functions\n<!-- doc-counts:end inventory-tests -->\n":                              false,
		"- Tests: <!-- doc-counts:begin inventory-tests -->8,585 top-level `Test` functions across the module<!-- doc-counts:end inventory-tests -->\n": true,
		"<!-- doc-counts:begin inventory-tests -->\n8 top-level `Test` functions\n<!-- doc-counts:end inventory-tests -->\n":                            true,
		"- Tests: <!-- doc-counts:begin inventory-tests -->none<!-- doc-counts:end inventory-tests -->\n" +
			"- Tests: <!-- doc-counts:begin inventory-tests -->none<!-- doc-counts:end inventory-tests -->\n": true,
		"- Tests: none\n": true,
	} {
		err := CheckSiteBlock(content, block)
		if (err != nil) != wantErr {
			t.Errorf("CheckSiteBlock(%q) = %v, want error %v", content, err, wantErr)
		}
	}
}
