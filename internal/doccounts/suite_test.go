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

	"github.com/Open-MBEE/OpenSysML/internal/doccounts/doccountstest"
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
		Robustness:           5,
		GRPCConformance:      2,
		GRPCRobustness:       2,
		TestFunctions:        8,
		RuntimeTestFunctions: 2,
		LSPTestFunctions:     1,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("suite counts\n got %+v\nwant %+v", counts, want)
	}
	if got := counts.Conformance.Of("calc", "state"); got != 5 {
		t.Fatalf("Of(calc, state) = %d, want 5", got)
	}
}

func TestReadSuiteCountsReportsAKnownFailureAsNotPassing(t *testing.T) {
	root := t.TempDir()
	doccountstest.WriteSuiteFixture(t, root)
	doccountstest.Write(t, root, "internal/core/runtime/testdata/conformance/known_failures.txt", "# pinned\ncalc_a\n")
	counts, err := ReadSuiteCounts(root)
	if err != nil {
		t.Fatalf("read suite counts: %v", err)
	}
	if counts.Conformance.Cases != 7 || counts.Conformance.Passing != 6 || counts.Conformance.KnownFailures["calc"] != 1 {
		t.Fatalf("known failure not counted: %+v", counts.Conformance)
	}
	if got := counts.Conformance.PassingOf("calc"); got != 3 {
		t.Fatalf("PassingOf(calc) = %d, want 3", got)
	}
	if got := passingOf(counts.Conformance, "calc"); got != "3 of 4" {
		t.Fatalf("passingOf = %q, want %q", got, "3 of 4")
	}
	figures := figuresOf(counts)
	if figures.AllPassing || figures.KnownFailures != 1 {
		t.Fatalf("figures do not state the known failure: %+v", figures)
	}
}

// TestReadSuiteCountsRejectsWhatNoGateWouldRead keeps a stray file from being
// counted as a fixture: each is an error naming the file.
func TestReadSuiteCountsRejectsWhatNoGateWouldRead(t *testing.T) {
	for name, mutate := range map[string]func(t *testing.T, root string){
		"a known failure that is no case": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/core/runtime/testdata/conformance/known_failures.txt", "ghost_case\n")
		},
		"a trace owned by no case": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/core/runtime/testdata/conformance/ghost.trace.golden", "trace\n")
		},
		"a parse fixture with no golden": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/core/parser/testdata/parse/orphan.sysml", "package P;\n")
		},
		"a robustness loop the source does not bound": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/core/runtime/robustness_test.go", `package runtime

import "testing"

func TestRuntimeRobustness(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run("x", func(t *testing.T) {})
	}
}
`)
		},
		"a robustness range over an unknown table": func(t *testing.T, root string) {
			doccountstest.Write(t, root, "internal/core/runtime/robustness_test.go", `package runtime

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
			doccountstest.Write(t, root, "internal/core/runtime/robustness_test.go", `package runtime

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
			doccountstest.Write(t, root, "internal/core/runtime/robustness_test.go", `package runtime

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
			doccountstest.Write(t, root, "internal/core/runtime/robustness_test.go", "package runtime\n")
		},
		"no conformance cases": func(t *testing.T, root string) {
			dir := filepath.Join(root, "internal", "core", "runtime", "testdata", "conformance")
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			doccountstest.Write(t, root, "internal/core/runtime/testdata/conformance/README.md", "empty\n")
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
	counts := Counts{Suite: suite}
	for _, block := range suiteBlocks() {
		content := "| row | before <!-- doc-counts:begin " + block.Name + " -->stale<!-- doc-counts:end " + block.Name + " --> after |\n"
		got, err := RewriteBlock(content, block, counts)
		if err != nil {
			t.Fatalf("%s: %v", block.Name, err)
		}
		if strings.Contains(got, "stale") || !strings.HasPrefix(got, "| row | before <!-- doc-counts:begin ") || !strings.HasSuffix(got, " --> after |\n") {
			t.Fatalf("%s did not rewrite within the line:\n%s", block.Name, got)
		}
		if strings.Count(got, "\n") != 1 {
			t.Fatalf("%s spilt over the line:\n%s", block.Name, got)
		}
		again, err := RewriteBlock(got, block, counts)
		if err != nil || again != got {
			t.Fatalf("%s is not idempotent: %v", block.Name, err)
		}
	}
	for name, want := range map[string]string{
		inventoryConformanceBlock: doccountstest.Expected.ConformanceSummary,
		inventoryTracesBlock:      doccountstest.Expected.TraceSummary,
		readmeConformanceBlock:    "7/7 conformance cases passing",
		readmeTierCalcBlock:       "conformance gate: 4 calc/constraint/requirement/satisfy cases passing",
		inventoryNegativesBlock:   "3 negative parser subtests (first-level subtests of `TestNegative`; 5 across the `TestNegative*` functions, 2 of them KerML, and 6 across every `*Negative*` parser test)",
		inventoryTestsBlock:       "8 top-level `Test` functions across the module",
	} {
		got, err := renderBlock(Block{Path: ReadmePath, Name: name}, counts)
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
	for _, block := range Blocks() {
		if _, ok := blockTemplates[block.Name]; !ok {
			t.Errorf("block %q on %s has no template", block.Name, block.Path)
		}
	}
}
