package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doccounts"
	"github.com/Open-MBEE/OpenSysML/internal/doccounts/doccountstest"
)

const (
	fixtureCompliance = `# Compliance

**No external referee:** self-assessed.

| Rule | Status |
|---|---|
| a | ✅ Faithful |
| b | ⚠️ Approximate |

- Execution conformance: <!-- doc-counts:begin inventory-conformance -->stale<!-- doc-counts:end inventory-conformance --> — kept prose
- Runtime robustness: <!-- doc-counts:begin inventory-robustness -->stale<!-- doc-counts:end inventory-robustness -->, among them kept prose
- Runtime tests: <!-- doc-counts:begin inventory-runtime-tests -->stale<!-- doc-counts:end inventory-runtime-tests -->
- Golden ASTs: <!-- doc-counts:begin inventory-golden-asts -->stale<!-- doc-counts:end inventory-golden-asts -->
- Golden traces: <!-- doc-counts:begin inventory-traces -->stale<!-- doc-counts:end inventory-traces -->
- Negative parser tests: <!-- doc-counts:begin inventory-negatives -->stale<!-- doc-counts:end inventory-negatives -->
- gRPC: <!-- doc-counts:begin inventory-grpc -->stale<!-- doc-counts:end inventory-grpc -->
- Test functions: <!-- doc-counts:begin inventory-tests -->stale<!-- doc-counts:end inventory-tests -->

**Measured coverage:** <!-- doc-counts:begin lsp-tests -->stale<!-- doc-counts:end lsp-tests -->, plus kept prose.
`
	fixtureReadme = `# README

| Behavioral parser | ✅ Complete (<!-- doc-counts:begin tier-behavioral-parser -->stale<!-- doc-counts:end tier-behavioral-parser -->) |
| Calc | ✅ Complete (<!-- doc-counts:begin tier-calc-evaluation -->stale<!-- doc-counts:end tier-calc-evaluation -->) |
| Action | ✅ Complete (<!-- doc-counts:begin tier-action-execution -->stale<!-- doc-counts:end tier-action-execution -->) |
| State | ✅ Complete (<!-- doc-counts:begin tier-state-machine -->stale<!-- doc-counts:end tier-state-machine -->: kept prose) |

**Test coverage:** <!-- doc-counts:begin test-suite -->stale<!-- doc-counts:end test-suite --> Kept prose.
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
)

// TestRunRewritesEveryDerivedLineAndIsIdempotent is the guarantee the wave-9
// workflow rests on: one command, byte-identical output, second run a no-op.
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
	for _, path := range []string{doccounts.ReadmePath, doccounts.ArchitecturePath, doccounts.SpecCompliancePath} {
		first[path] = read(t, root, path)
		if strings.Contains(first[path], "stale") {
			t.Fatalf("%s keeps a stale block:\n%s", path, first[path])
		}
	}
	if !strings.Contains(first[doccounts.ArchitecturePath], "status of each tracked rule stays in [spec compliance]") {
		t.Fatalf("bookkeeping line not restated:\n%s", first[doccounts.ArchitecturePath])
	}
	if !strings.Contains(first[doccounts.ArchitecturePath], "Nothing else on this line's neighbours moves.") {
		t.Fatal("architecture lost a neighbouring line")
	}
	want := doccountstest.Expected
	for _, sentence := range []string{
		"| Behavioral parser | ✅ Complete (<!-- doc-counts:begin tier-behavioral-parser -->3 golden ASTs, 3 negative tests<!-- doc-counts:end tier-behavioral-parser -->) |",
		"| State | ✅ Complete (<!-- doc-counts:begin tier-state-machine -->1 conformance cases passing<!-- doc-counts:end tier-state-machine -->: kept prose) |",
		"**Test coverage:** <!-- doc-counts:begin test-suite -->" + want.TestFunctions + " top-level `Test` functions",
		"7 conformance cases, 3 golden traces, 5 runtime robustness cases, 2 gRPC conformance cases and 2 gRPC robustness cases.<!-- doc-counts:end test-suite --> Kept prose.",
		"(<!-- doc-counts:begin conformance-passing -->7/7 conformance cases passing<!-- doc-counts:end conformance-passing -->).",
	} {
		if !strings.Contains(first[doccounts.ReadmePath], sentence) {
			t.Fatalf("README lacks %q:\n%s", sentence, first[doccounts.ReadmePath])
		}
	}
	for _, sentence := range []string{
		"- Execution conformance: <!-- doc-counts:begin inventory-conformance -->" + want.ConformanceSummary + "<!-- doc-counts:end inventory-conformance --> — kept prose",
		"- Golden traces: <!-- doc-counts:begin inventory-traces -->" + want.TraceSummary,
		"| b | ⚠️ Approximate |",
		"<!-- doc-counts:begin lsp-tests -->1 top-level `Test` functions in `internal/lsp`<!-- doc-counts:end lsp-tests -->, plus kept prose.",
	} {
		if !strings.Contains(first[doccounts.SpecCompliancePath], sentence) {
			t.Fatalf("compliance map lacks %q:\n%s", sentence, first[doccounts.SpecCompliancePath])
		}
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
	writeAt(t, root, doccounts.ArchitecturePath, "**Row bookkeeping:** reworded, and no longer the line the pattern states.\n")
	before := read(t, root, doccounts.ReadmePath)

	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error for a derived line the pattern does not match")
	}
	if read(t, root, doccounts.ReadmePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
	}
}

func TestRunWritesNothingWhenAFileIsNotWritable(t *testing.T) {
	root := writeFixture(t)
	readonly := filepath.Join(root, filepath.FromSlash(doccounts.ArchitecturePath))
	if err := os.Chmod(readonly, 0o444); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	before := read(t, root, doccounts.ReadmePath)

	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error for a file that cannot be written")
	}
	if read(t, root, doccounts.ReadmePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
	}
}

func TestRunReportsAMapWithNoRuleRows(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, doccounts.SpecCompliancePath, "# Compliance\n")
	if _, err := run(root, io.Discard); err == nil {
		t.Fatal("want an error when the compliance map states no rule rows")
	}
}

func TestRunReportsAKnownFailureRow(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, doccounts.SpecCompliancePath, fixtureCompliance+"| c | 🚧 Known failure |\n")
	if _, err := run(root, io.Discard); err == nil || !strings.Contains(err.Error(), "🚧") {
		t.Fatalf("want an error naming the 🚧 row, got %v", err)
	}
}

func TestCheckReportsStaleFilesWithoutWriting(t *testing.T) {
	root := writeFixture(t)
	before := read(t, root, doccounts.ReadmePath)
	var output strings.Builder
	stale, err := check(root, &output)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stale != 3 {
		t.Fatalf("check reported %d stale files, want 3", stale)
	}
	if !strings.Contains(output.String(), "README.md is stale") {
		t.Fatalf("check report does not name README.md:\n%s", output.String())
	}
	if read(t, root, doccounts.ReadmePath) != before {
		t.Fatal("check mode changed README.md")
	}
}

// TestCheckFailsOnAMutatedFigure is what makes the generated figures a gate:
// one digit changed in any generated block, and -check names the file.
func TestCheckFailsOnAMutatedFigure(t *testing.T) {
	root := writeFixture(t)
	if _, err := run(root, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, block := range doccounts.Blocks() {
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
			digit := strings.IndexAny(generated, "0123456789")
			if digit < 0 {
				t.Fatalf("block %q states no figure:\n%s", block.Name, generated)
			}
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

// TestCheckFailsOnAMutatedSuiteTree is the other direction: a fixture added to
// the tree makes the committed figures stale until they are regenerated.
func TestCheckFailsOnAMutatedSuiteTree(t *testing.T) {
	root := writeFixture(t)
	if _, err := run(root, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}
	writeAt(t, root, "internal/core/runtime/testdata/conformance/state_b.expected.json", "{}\n")
	var output strings.Builder
	stale, err := check(root, &output)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if stale != 2 {
		t.Fatalf("a new conformance case left %d files stale, want the README and the compliance map:\n%s", stale, output.String())
	}
	if !strings.Contains(output.String(), "8 conformance cases") || !strings.Contains(output.String(), "state×2") {
		t.Fatalf("check does not restate the new census:\n%s", output.String())
	}
}

// TestRunReportsAMissingSuiteBlock keeps a consumer from dropping a block: the
// page must carry every block registered for it.
func TestRunReportsAMissingSuiteBlock(t *testing.T) {
	root := writeFixture(t)
	writeAt(t, root, doccounts.ReadmePath, strings.Replace(fixtureReadme+fixtureBookkeeping, "<!-- doc-counts:begin conformance-passing -->stale<!-- doc-counts:end conformance-passing -->", "889/889", 1))
	before := read(t, root, doccounts.SpecCompliancePath)
	if _, err := run(root, io.Discard); err == nil || !strings.Contains(err.Error(), "conformance-passing") {
		t.Fatalf("want an error naming the missing block, got %v", err)
	}
	if read(t, root, doccounts.SpecCompliancePath) != before {
		t.Fatal("a failed run rewrote another file")
	}
}

func TestCheckCommittedTreeIsCurrent(t *testing.T) {
	var output strings.Builder
	stale, err := check("../..", &output)
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
	writeAt(t, root, doccounts.SpecCompliancePath, fixtureCompliance)
	writeAt(t, root, doccounts.ReadmePath, fixtureReadme+fixtureBookkeeping)
	writeAt(t, root, doccounts.ArchitecturePath, fixtureBookkeeping)
	doccountstest.WriteSuiteFixture(t, root)
	writeAt(t, root, "docs/project/pilot-differential-baseline.json", fixtureDifferentialBaseline)
	writeAt(t, root, "docs/project/pilot-xpect-baseline.json", fixtureXpectBaseline)
	writeAt(t, root, "docs/project/pilot-rejection-baseline.json", fixtureRejectionBaseline)
	return root
}

func writeAt(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func read(t *testing.T, root, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
