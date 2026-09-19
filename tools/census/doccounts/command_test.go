package doccounts

import (
	"encoding/json"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/fixtures"
	"github.com/Open-MBEE/OpenSysML/tools/census/doccounts/doccountstest"
)

const (
	fixtureCompliance = `# Compliance

**No external referee:** self-assessed.

| Rule | Status |
|---|---|
| a | ✅ Faithful |
| b | ⚠️ Approximate |

<!-- doc-counts:begin analysis-libraries -->
old library table
<!-- doc-counts:end analysis-libraries -->
The rows above are the map's own.

- Execution conformance: <!-- doc-counts:begin inventory-conformance -->the conformance cases<!-- doc-counts:end inventory-conformance --> — kept prose
- Runtime robustness: <!-- doc-counts:begin inventory-robustness -->the runtime robustness cases<!-- doc-counts:end inventory-robustness -->, among them kept prose
- Runtime tests: <!-- doc-counts:begin inventory-runtime-tests -->the runtime test functions<!-- doc-counts:end inventory-runtime-tests -->
- Golden ASTs: <!-- doc-counts:begin inventory-golden-asts -->the golden AST fixtures<!-- doc-counts:end inventory-golden-asts -->
- Golden traces: <!-- doc-counts:begin inventory-traces -->the golden execution traces<!-- doc-counts:end inventory-traces -->
- Negative parser tests: <!-- doc-counts:begin inventory-negatives -->the negative parser subtests<!-- doc-counts:end inventory-negatives -->
- gRPC: <!-- doc-counts:begin inventory-grpc -->the gRPC cases<!-- doc-counts:end inventory-grpc -->
- Test functions: <!-- doc-counts:begin inventory-tests -->the top-level Test functions<!-- doc-counts:end inventory-tests -->

**Measured coverage:** <!-- doc-counts:begin lsp-tests -->the LSP test functions<!-- doc-counts:end lsp-tests -->, plus kept prose.
`
	fixtureReadme = `# README

**Behavioral execution:** the send statement (<!-- doc-counts:begin conformance-passing -->stale<!-- doc-counts:end conformance-passing -->).
`
	fixtureBookkeeping = `# Guide

**Reference differential:** 99 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (` + "`" + `old` + "`" + `), 99 in full agreement;
**Rejection oracle:** the reverse direction — do we reject what the reference rejects? 99 hand-written invalid models validated by both implementations, 99 rejected by both, 99 the pinned pilot rejects and we accept;
<!-- doc-counts:begin refereed-figures -->
old generated block
<!-- doc-counts:end refereed-figures -->
Nothing else on this line's neighbours moves.
`
	fixtureDifferentialBaseline = `{"pilotRelease":"2026-05 (jupyter-sysml-kernel 0.60.1)","totals":{"files":2,"filesFullyAgreeing":1,"openSysMLOnly":3,"pilotOnly":4},` +
		`"errata":{"registryEntries":2,"corrections":1,"documentedWithoutCorrection":1,"totals":{"files":2,"filesFullyAgreeing":2,"openSysMLOnly":2,"pilotOnly":4}}}`
	fixtureXpectBaseline = `{"kinds":[{"kind":"errors","assertions":2,"rows":2,"agree":2,"wordingOnly":1,"sameLocation":0,"sameLine":0,"severityDiffers":0,"elsewhereInFile":0},{"kind":"scope","assertions":3,"agree":2}],` +
		`"errata":{"kinds":[{"kind":"errors","assertions":2,"rows":2,"agree":2,"wordingOnly":1}]}}`
	fixtureRejectionBaseline = `{"totals":{"cases":2,"bothReject":2,"pilotOnlyRejects":0},"strictOnlyAgreements":[],` +
		`"errata":{"totals":{"cases":2,"bothReject":2,"pilotOnlyRejects":0}}}`
	fixtureLibraryCensus = `{"command":"go test -run TestFixtureCensus ./x","packages":[` +
		`{"name":"Alpha","path":"a.sysml","declarations":["Alpha::a","Alpha::b"],"evaluated":["Alpha::a"],` +
		`"refused":[{"declaration":"Alpha::b","error":"ErrNoValue","message":"no value: b"}],"wrong":[]}]}`
)

// TestRunRewritesEveryDerivedLineAndIsIdempotent is the guarantee the workflow
// rests on: one command, byte-identical output, second run a no-op.
func TestRunRewritesEveryDerivedLineAndIsIdempotent(t *testing.T) {
	root := writeFixture(t)

	rewritten, err := run(root, io.Discard)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if rewritten != 3 {
		t.Fatalf("first run rewrote %d files, want 3", rewritten)
	}
	first := map[string]string{}
	for _, path := range []string{ReadmePath, ArchitecturePath, SpecCompliancePath} {
		first[path] = read(t, root, path)
		if strings.Contains(first[path], "stale") {
			t.Fatalf("%s keeps a stale block:\n%s", path, first[path])
		}
	}
	if !strings.Contains(first[ArchitecturePath], "status of each tracked rule stays in [spec compliance]") {
		t.Fatalf("bookkeeping line not restated:\n%s", first[ArchitecturePath])
	}
	if !strings.Contains(first[ArchitecturePath], "Nothing else on this line's neighbours moves.") {
		t.Fatal("architecture lost a neighbouring line")
	}
	if sentence := "(<!-- doc-counts:begin conformance-passing -->every conformance case passing<!-- doc-counts:end conformance-passing -->)."; !strings.Contains(first[ReadmePath], sentence) {
		t.Fatalf("README lacks %q:\n%s", sentence, first[ReadmePath])
	}
	for _, sentence := range []string{
		"- Execution conformance: <!-- doc-counts:begin inventory-conformance -->the conformance cases<!-- doc-counts:end inventory-conformance --> — kept prose",
		"| b | ⚠️ Approximate |",
		"<!-- doc-counts:begin lsp-tests -->the LSP test functions<!-- doc-counts:end lsp-tests -->, plus kept prose.",
	} {
		if !strings.Contains(first[SpecCompliancePath], sentence) {
			t.Fatalf("compliance map lacks %q (a site block is rendered by the build, not written):\n%s", sentence, first[SpecCompliancePath])
		}
	}
	compliance := read(t, root, SpecCompliancePath)
	first[SpecCompliancePath] = compliance
	if !strings.Contains(compliance, "| `Alpha` | 2 | 1 | 1 | 0 |") || strings.Contains(compliance, "old library table") {
		t.Fatalf("the library table is not restated from the census:\n%s", compliance)
	}
	if !strings.Contains(compliance, "| a | ✅ Faithful |\n| b | ⚠️ Approximate |\n") || !strings.Contains(compliance, "The rows above are the map's own.") {
		t.Fatalf("the compliance map's own rows moved:\n%s", compliance)
	}

	rewritten, err = run(root, io.Discard)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if rewritten != 0 {
		t.Fatalf("second run rewrote %d files, want 0", rewritten)
	}
	for path, content := range first {
		if read(t, root, path) != content {
			t.Fatalf("%s changed on the second run", path)
		}
	}
}

// TestRunWritesNothingWhenALaterFileCannotBeRewritten keeps the tree consistent:
// a partly restated tree would state two different censuses at once.
func TestRunWritesNothingWhenALaterFileCannotBeRewritten(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, ArchitecturePath, "**Row bookkeeping:** reworded, and no longer the line the pattern states.\n")
	before := read(t, root, ReadmePath)

	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error for a derived line the pattern does not match")
	}
	if read(t, root, ReadmePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
	}
}

func TestRunWritesNothingWhenAFileIsNotWritable(t *testing.T) {
	root := writeFixture(t)
	readonly := filepath.Join(root, filepath.FromSlash(ArchitecturePath))
	if err := os.Chmod(readonly, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	before := read(t, root, ReadmePath)

	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error for a file that cannot be written")
	}
	if read(t, root, ReadmePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
	}
}

func TestRunReportsAMapWithNoRuleRows(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, SpecCompliancePath, "# Compliance\n")
	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error when the compliance map states no rule rows")
	}
}

func TestRunReportsAKnownFailureRow(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, SpecCompliancePath, fixtureCompliance+"| c | 🚧 Known failure |\n")
	if _, err := run(root, io.Discard); err == nil || !strings.Contains(err.Error(), "🚧") {
		t.Fatalf("want an error naming the 🚧 row, got %v", err)
	}
}

func TestCheckReportsStaleFilesWithoutWriting(t *testing.T) {
	root := writeFixture(t)
	before := read(t, root, ReadmePath)
	var output strings.Builder
	stale, err := check(root, &output)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stale != 3 {
		t.Fatalf("check reported %d stale files, want 3", stale)
	}
	for _, name := range []string{"README.md is stale", "docs/project/spec-compliance.md is stale"} {
		if !strings.Contains(output.String(), name) {
			t.Fatalf("check report lacks %q:\n%s", name, output.String())
		}
	}
	if read(t, root, ReadmePath) != before {
		t.Fatal("check mode changed README.md")
	}
	if read(t, root, SpecCompliancePath) != fixtureCompliance {
		t.Fatal("check mode changed the compliance map")
	}
}

// TestCheckReportsAnEditedCensusFigure is the drift gate: a census figure that
// moves in the JSON leaves the rendered table stale until it is regenerated.
func TestCheckReportsAnEditedCensusFigure(t *testing.T) {
	root := writeFixture(t)
	if _, err := run(root, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stale, err := check(root, io.Discard); err != nil || stale != 0 {
		t.Fatalf("check after run: %d stale, %v", stale, err)
	}
	moved := strings.Replace(fixtureLibraryCensus, `"evaluated":["Alpha::a"],"refused":[{"declaration":"Alpha::b","error":"ErrNoValue","message":"no value: b"}]`,
		`"evaluated":["Alpha::a","Alpha::b"],"refused":[]`, 1)
	if moved == fixtureLibraryCensus {
		t.Fatal("the fixture census did not move")
	}
	writeAt(t, root, fixtures.LibraryCensusPath, moved)
	var output strings.Builder
	stale, err := check(root, &output)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stale != 1 || !strings.Contains(output.String(), "docs/project/spec-compliance.md is stale") {
		t.Fatalf("check reported %d stale files:\n%s", stale, output.String())
	}
	if !strings.Contains(output.String(), "| `Alpha` | 2 | 2 | 0 | 0 |") {
		t.Fatalf("check report does not show the moved figure:\n%s", output.String())
	}
}

// TestRunReportsACensusMissingItsBlock keeps the table from disappearing: a
// compliance map without the markers is an error, not a map without a table.
func TestRunReportsACensusMissingItsBlock(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, SpecCompliancePath, "# Compliance\n\n| Rule | Status |\n|---|---|\n| a | ✅ Faithful |\n")
	before := read(t, root, ReadmePath)
	if _, err := run(root, io.Discard); err == nil || !strings.Contains(err.Error(), "analysis-libraries") {
		t.Fatalf("want an error naming the missing block, got %v", err)
	}
	if read(t, root, ReadmePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
	}
}

// TestCheckFailsOnAMutatedFigure is what makes the committed figures a gate:
// a digit slipped into any committed block, and -check names the file.
func TestCheckFailsOnAMutatedFigure(t *testing.T) {
	root := writeFixture(t)
	if _, err := run(root, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, block := range Blocks() {
		t.Run(block.Path+"/"+block.Name, func(t *testing.T) {
			current := read(t, root, block.Path)
			defer writeAt(t, root, block.Path, current)
			begin := "<!-- doc-counts:begin " + block.Name + " -->"
			start := strings.Index(current, begin) + len(begin)
			end := strings.Index(current[start:], "<!-- doc-counts:end "+block.Name+" -->")
			if start < len(begin) || end < 0 {
				t.Fatalf("%s lacks the block %q", block.Path, block.Name)
			}
			generated := current[start : start+end]
			digit := max(strings.IndexAny(generated, "0123456789"), 0)
			mutated := generated[:digit] + "9" + generated[digit:]
			writeAt(t, root, block.Path, current[:start]+mutated+current[start+end:])
			var output strings.Builder
			stale, err := check(root, &output)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			if stale != 1 || !strings.Contains(output.String(), block.Path+" is stale") {
				t.Fatalf("a mutated %q figure was not caught (%d stale):\n%s", block.Name, stale, output.String())
			}
		})
	}
}

// TestAMutatedSuiteTreeMovesTheSiteBlocksAlone is what keeps a branch adding a
// fixture from touching a committed page: the tree stays current, and the new
// case shows in what the build renders.
func TestAMutatedSuiteTreeMovesTheSiteBlocksAlone(t *testing.T) {
	root := writeFixture(t)
	if _, err := run(root, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	writeAt(t, root, "internal/exec/runtime/testdata/conformance/state_b.expected.json", "{}\n")
	writeAt(t, root, "internal/exec/runtime/robustness_more_test.go", "package runtime\n\nimport \"testing\"\n\nfunc TestRuntimeRobustnessMore(t *testing.T) {\n\tt.Run(\"h\", func(t *testing.T) {})\n}\n")
	var output strings.Builder
	stale, err := check(root, &output)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stale != 0 {
		t.Fatalf("a new conformance case left %d files stale, want none:\n%s", stale, output.String())
	}
	rendered := siteBlocksOf(t, root)
	compliance := rendered[SpecCompliancePath]
	if got := compliance["inventory-conformance"]; !strings.Contains(got, "8 conformance cases") || !strings.Contains(got, "state×2") {
		t.Fatalf("the build does not render the new case: %q", got)
	}
	if got := compliance["inventory-robustness"]; !strings.HasPrefix(got, "8 runtime robustness cases") {
		t.Fatalf("the build does not count the new robustness file: %q", got)
	}
}

// TestAKnownFailureMovesTheCommittedConformanceBlock: known_failures.txt is a
// committed adjudication, so the README's passing statement follows it.
func TestAKnownFailureMovesTheCommittedConformanceBlock(t *testing.T) {
	root := writeFixture(t)
	if _, err := run(root, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	writeAt(t, root, "internal/exec/runtime/testdata/conformance/known_failures.txt", "calc_a\n")
	var output strings.Builder
	stale, err := check(root, &output)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stale != 1 || !strings.Contains(output.String(), "README.md is stale") {
		t.Fatalf("a known failure left %d files stale, want the README:\n%s", stale, output.String())
	}
	if !strings.Contains(output.String(), "every conformance case passing but the 1 `known_failures.txt` lists") {
		t.Fatalf("check does not restate the known failure:\n%s", output.String())
	}
}

// TestSiteBlocksRenderTheTreeAsJSON pins the contract the MkDocs hook reads.
func TestSiteBlocksRenderTheTreeAsJSON(t *testing.T) {
	root := writeFixture(t)
	rendered := siteBlocksOf(t, root)
	if got := len(rendered); got != 1 {
		t.Fatalf("rendered %d pages, want the compliance map alone: %v", got, rendered)
	}
	compliance := rendered[SpecCompliancePath]
	want := doccountstest.Expected
	for name, text := range map[string]string{
		"inventory-conformance": want.ConformanceSummary,
		"inventory-traces":      want.TraceSummary,
		"inventory-robustness":  "7 runtime robustness cases (first-level subtests across the `TestRuntimeRobustness*` functions)",
		"inventory-grpc":        "2 gRPC conformance cases and 3 gRPC robustness cases (first-level subtests across the `TestGRPCRobustness*` functions)",
		"inventory-tests":       want.TestFunctions + " top-level `Test` functions across the module",
		"lsp-tests":             "1 top-level `Test` functions in `internal/frontend/lsp`",
	} {
		if !strings.HasPrefix(compliance[name], text) {
			t.Errorf("%s renders %q, want it to open with %q", name, compliance[name], text)
		}
	}
	var names []string
	for _, block := range SiteBlocks() {
		names = append(names, block.Name)
	}
	for name := range compliance {
		if !strings.Contains(" "+strings.Join(names, " ")+" ", " "+name+" ") {
			t.Errorf("rendered an unregistered block %q", name)
		}
	}
	if len(compliance) != len(names) {
		t.Errorf("rendered %d blocks, want %d", len(compliance), len(names))
	}
}

// TestCheckRefusesAFigureInASiteBlock keeps the volatile figures out of git:
// a contributor who types one in is refused, whatever the number.
func TestCheckRefusesAFigureInASiteBlock(t *testing.T) {
	root := writeFixture(t)
	if _, err := run(root, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, block := range SiteBlocks() {
		t.Run(block.Name, func(t *testing.T) {
			current := read(t, root, block.Path)
			defer writeAt(t, root, block.Path, current)
			begin := "<!-- doc-counts:begin " + block.Name + " -->"
			start := strings.Index(current, begin) + len(begin)
			if start < len(begin) {
				t.Fatalf("%s lacks the block %q", block.Path, block.Name)
			}
			writeAt(t, root, block.Path, current[:start]+"7 "+current[start:])
			_, err := check(root, io.Discard)
			if err == nil || !strings.Contains(err.Error(), block.Name) || !strings.Contains(err.Error(), "states a figure") {
				t.Fatalf("a figure typed into %q was not refused: %v", block.Name, err)
			}
			if _, err := run(root, io.Discard); err == nil {
				t.Fatal("run accepted the figure")
			}
		})
	}
}

// TestRunReportsAMissingSiteBlock keeps the build's consumer from dropping a
// block: the page must carry every site block registered for it.
func TestRunReportsAMissingSiteBlock(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, SpecCompliancePath, strings.Replace(fixtureCompliance, "<!-- doc-counts:begin lsp-tests -->the LSP test functions<!-- doc-counts:end lsp-tests -->", "the LSP test functions", 1))
	if _, err := run(root, io.Discard); err == nil || !strings.Contains(err.Error(), "lsp-tests") {
		t.Fatalf("want an error naming the missing block, got %v", err)
	}
}

func siteBlocksOf(t *testing.T, root string) map[string]map[string]string {
	t.Helper()
	var output strings.Builder
	if err := renderSiteBlocks(root, &output); err != nil {
		t.Fatalf("site blocks: %v", err)
	}
	var rendered map[string]map[string]string
	if err := json.Unmarshal([]byte(output.String()), &rendered); err != nil {
		t.Fatalf("site blocks are not JSON: %v\n%s", err, output.String())
	}
	return rendered
}

// TestSiteBlocksOfTheCommittedTreeRender keeps the build from failing on the
// tree as committed: every site block renders, and none is empty.
func TestSiteBlocksOfTheCommittedTreeRender(t *testing.T) {
	rendered := siteBlocksOf(t, "../../..")
	if pages, want := pagesOf(rendered), slices.Sorted(slices.Values(SitePaths())); !slices.Equal(pages, want) {
		t.Fatalf("rendered pages %v, want %v", pages, want)
	}
	for page, blocks := range rendered {
		for name, text := range blocks {
			if strings.TrimSpace(text) == "" || !strings.ContainsAny(text, "0123456789") {
				t.Errorf("%s/%s renders %q", page, name, text)
			}
		}
	}
}

func pagesOf(rendered map[string]map[string]string) []string {
	return slices.Sorted(maps.Keys(rendered))
}

// TestRunReportsAMissingSuiteBlock keeps a consumer from dropping a block: the
// page must carry every block registered for it.
func TestRunReportsAMissingSuiteBlock(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, ReadmePath, strings.Replace(fixtureReadme+fixtureBookkeeping, "<!-- doc-counts:begin conformance-passing -->stale<!-- doc-counts:end conformance-passing -->", "889/889", 1))
	before := read(t, root, SpecCompliancePath)
	if _, err := run(root, io.Discard); err == nil || !strings.Contains(err.Error(), "conformance-passing") {
		t.Fatalf("want an error naming the missing block, got %v", err)
	}
	if read(t, root, SpecCompliancePath) != before {
		t.Fatal("a failed run rewrote another file")
	}
}

func TestCheckCommittedTreeIsCurrent(t *testing.T) {
	var output strings.Builder
	stale, err := check("../../..", &output)
	if err != nil {
		t.Fatalf("check committed tree: %v", err)
	}
	if stale != 0 {
		t.Fatalf("committed tree has %d stale files:\n%s", stale, output.String())
	}
}

func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeAt(t, root, SpecCompliancePath, fixtureCompliance)
	writeAt(t, root, ReadmePath, fixtureReadme+fixtureBookkeeping)
	writeAt(t, root, ArchitecturePath, fixtureBookkeeping)
	doccountstest.WriteSuiteFixture(t, root)
	writeAt(t, root, "docs/project/pilot-differential-baseline.json", fixtureDifferentialBaseline)
	writeAt(t, root, "docs/project/pilot-xpect-baseline.json", fixtureXpectBaseline)
	writeAt(t, root, "docs/project/pilot-rejection-baseline.json", fixtureRejectionBaseline)
	writeAt(t, root, fixtures.LibraryCensusPath, fixtureLibraryCensus)
	return root
}

func read(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
