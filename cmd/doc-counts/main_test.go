package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/doccounts"
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
	for _, path := range []string{doccounts.ReadmePath, doccounts.ArchitecturePath} {
		first[path] = read(t, root, path)
		if !strings.Contains(first[path], "status of each tracked rule stays in [spec compliance]") {
			t.Fatalf("%s bookkeeping line not restated:\n%s", path, first[path])
		}
		if !strings.Contains(first[path], "Nothing else on this line's neighbours moves.") {
			t.Fatalf("%s lost a neighbouring line", path)
		}
	}
	compliance := read(t, root, doccounts.SpecCompliancePath)
	first[doccounts.SpecCompliancePath] = compliance
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
	for _, name := range []string{"README.md is stale", "docs/project/spec-compliance.md is stale"} {
		if !strings.Contains(output.String(), name) {
			t.Fatalf("check report lacks %q:\n%s", name, output.String())
		}
	}
	if read(t, root, doccounts.ReadmePath) != before {
		t.Fatal("check mode changed README.md")
	}
	if read(t, root, doccounts.SpecCompliancePath) != fixtureCompliance {
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
	writeAt(t, root, doccounts.LibraryCensusPath, moved)
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
	writeAt(t, root, doccounts.SpecCompliancePath, "# Compliance\n\n| Rule | Status |\n|---|---|\n| a | ✅ Faithful |\n")
	before := read(t, root, doccounts.ReadmePath)
	if _, err := run(root, io.Discard); err == nil || !strings.Contains(err.Error(), "analysis-libraries") {
		t.Fatalf("want an error naming the missing block, got %v", err)
	}
	if read(t, root, doccounts.ReadmePath) != before {
		t.Fatal("a failed run rewrote an earlier file")
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
	writeAt(t, root, doccounts.ReadmePath, fixtureBookkeeping)
	writeAt(t, root, doccounts.ArchitecturePath, fixtureBookkeeping)
	writeAt(t, root, "docs/project/pilot-differential-baseline.json", fixtureDifferentialBaseline)
	writeAt(t, root, "docs/project/pilot-xpect-baseline.json", fixtureXpectBaseline)
	writeAt(t, root, "docs/project/pilot-rejection-baseline.json", fixtureRejectionBaseline)
	writeAt(t, root, doccounts.LibraryCensusPath, fixtureLibraryCensus)
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
