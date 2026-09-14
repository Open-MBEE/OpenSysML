package doccounts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const complianceFixture = `# Compliance

| Rule | Where | Test | Status |
|---|---|---|---|
| a | x | y | ✅ Faithful |
| b | x | y | ⚠️ Approximate |
| c | x | y | ❌ Not implemented |
| d | x | y | ⛔ Deliberate |
| notes | mentions ✅ and ❌ together | y | ⚠️ Approximate |
`

func TestCountRulesCountsOneMarkerPerRow(t *testing.T) {
	counts := CountRules(complianceFixture)
	want := RuleCounts{Total: 4, Faithful: 1, Approximate: 1, NotImplemented: 1, Deliberate: 1}
	if counts != want {
		t.Fatalf("census: want %+v, got %+v", want, counts)
	}
}

func TestCountRulesIgnoresProseAndHeaders(t *testing.T) {
	if counts := CountRules("✅ prose outside a table\n\n| header | Status |\n|---|---|\n"); counts.Total != 0 {
		t.Fatalf("census of a table with no rule rows: want 0 rows, got %+v", counts)
	}
}

func TestReadRefereedCountsDerivesAllBaselineFigures(t *testing.T) {
	root := t.TempDir()
	writeDoccountsFixture(t, root)
	counts, err := ReadRefereedCounts(root)
	if err != nil {
		t.Fatalf("read baselines: %v", err)
	}
	if counts.Files != 2 || counts.FilesAgreeing != 1 || counts.OursOnly != 3 || counts.PilotOnly != 4 {
		t.Fatalf("differential counts: %+v", counts)
	}
	if counts.DeclaredErrors != 11 || counts.Silent != 1 || counts.DeclaredAgree != 6 ||
		counts.WordingOnly != 2 || counts.LocationOnly != 1 || counts.SeverityDiffers != 0 ||
		counts.Elsewhere != 0 || counts.ScopeExact != 7 || counts.ScopeTotal != 8 {
		t.Fatalf("Xpect counts: %+v", counts)
	}
	if counts.RejectCases != 12 || counts.RejectBoth != 11 || counts.RejectPilotOnly != 1 ||
		counts.RejectDefaultBoth != 9 || counts.RejectDefaultPilotOnly != 3 || counts.RejectStrictOnly != 2 {
		t.Fatalf("rejection counts: %+v", counts)
	}
	if counts.PilotTag != "2026-05" || counts.PilotArtifact != "0.60.1" {
		t.Fatalf("derived metadata: %+v", counts)
	}
	want := ErrataCounts{Registry: 2, Corrections: 1, Documented: 1,
		Files: 2, FilesAgreeing: 2, OursOnly: 2, PilotOnly: 4, Silent: 0, RejectCases: 12, RejectPilotOnly: 1}
	if counts.Errata != want {
		t.Fatalf("errata counts: %+v, want %+v", counts.Errata, want)
	}
}

// TestReadRefereedCountsRejectsABaselineWithoutErrata keeps the second figure a
// measurement: a baseline predating the overlay is stale, not zero.
func TestReadRefereedCountsRejectsABaselineWithoutErrata(t *testing.T) {
	for _, path := range []string{
		"docs/project/pilot-differential-baseline.json",
		"docs/project/pilot-xpect-baseline.json",
		"docs/project/pilot-rejection-baseline.json",
	} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			writeDoccountsFixture(t, root)
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(content, &decoded); err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			delete(decoded, "errata")
			stripped, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("encode fixture: %v", err)
			}
			writeAt(t, root, path, string(stripped))
			if _, err := ReadRefereedCounts(root); err == nil {
				t.Fatalf("%s without an errata section: want an error", path)
			}
		})
	}
}

func TestRewriteBlockUsesConsumerRelativeLinksAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	writeDoccountsFixture(t, root)
	refereed, err := ReadRefereedCounts(root)
	if err != nil {
		t.Fatalf("read baselines: %v", err)
	}
	counts := Counts{Refereed: refereed}
	spec := Block{Path: "README.md", Name: "refereed-figures", LinkPrefix: "docs/project/"}
	content := "before\n<!-- doc-counts:begin refereed-figures -->\nstale\n<!-- doc-counts:end refereed-figures -->\nafter\n"
	got, err := RewriteBlock(content, spec, counts)
	if err != nil {
		t.Fatalf("rewrite block: %v", err)
	}
	for _, want := range []string{
		"`PILOT_TAG=2026-05`", "artifact `0.60.1`", "1 of 2 files",
		"6 we report word-for-word", "9 both reject",
		"[differential](docs/project/pilot-differential.md)",
		"[spec compliance](docs/project/spec-compliance.md)",
		"must be read by root",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated block lacks %q:\n%s", want, got)
		}
	}
	again, err := RewriteBlock(got, spec, counts)
	if err != nil {
		t.Fatalf("second block rewrite: %v", err)
	}
	if again != got {
		t.Fatal("block rewrite is not idempotent")
	}
}

// TestRewriteBlockRejectsABlockWithNoTemplate keeps a new consumer from silently
// emptying a block: a name no template renders is an error, not empty markup.
func TestRewriteBlockRejectsABlockWithNoTemplate(t *testing.T) {
	content := "<!-- doc-counts:begin invented -->\nkept\n<!-- doc-counts:end invented -->\n"
	if _, err := RewriteBlock(content, Block{Path: ReadmePath, Name: "invented"}, Counts{}); err == nil {
		t.Fatal("want an error for a block name no template renders")
	}
}

func TestRewriteBlockRejectsMalformedMarkers(t *testing.T) {
	spec := Block{Path: "README.md", Name: "refereed-figures"}
	counts := Counts{}
	for name, content := range map[string]string{
		"missing begin":          "<!-- doc-counts:end refereed-figures -->\n",
		"missing end":            "<!-- doc-counts:begin refereed-figures -->\n",
		"reversed":               "<!-- doc-counts:end refereed-figures -->\n<!-- doc-counts:begin refereed-figures -->\n",
		"duplicate":              "<!-- doc-counts:begin refereed-figures -->\n<!-- doc-counts:begin refereed-figures -->\n<!-- doc-counts:end refereed-figures -->\n",
		"inline unterminated":    "x <!-- doc-counts:begin refereed-figures --> stale\n",
		"inline reversed":        "x <!-- doc-counts:end refereed-figures --> stale <!-- doc-counts:begin refereed-figures -->\n",
		"inline and block mixed": "x <!-- doc-counts:begin refereed-figures --> stale <!-- doc-counts:end refereed-figures -->\n<!-- doc-counts:end refereed-figures -->\n",
		"inline twice":           "x <!-- doc-counts:begin refereed-figures --> a <!-- doc-counts:end refereed-figures --> <!-- doc-counts:begin refereed-figures --> b <!-- doc-counts:end refereed-figures -->\n",
		"multi-line inline":      "x <!-- doc-counts:begin refereed-figures --> stale <!-- doc-counts:end refereed-figures -->\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := RewriteBlock(content, spec, counts); err == nil {
				t.Fatal("want malformed marker error")
			}
		})
	}
}

func TestRewriteBaselineLineRestatesOnlyCapturedValues(t *testing.T) {
	spec := BaselineLines()[0]
	content := "before\n**Reference differential:** 99 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (`old`), 99 in full agreement;\nafter\n"
	counts := RefereedCounts{Files: 2, PilotTag: "2026-05", FilesAgreeing: 1}
	got, err := RewriteBaselineLine(content, spec, counts)
	if err != nil {
		t.Fatalf("rewrite baseline line: %v", err)
	}
	want := "before\n**Reference differential:** 2 files compared diagnostic-by-diagnostic against the pinned OMG pilot implementation (`2026-05`), 1 in full agreement;\nafter\n"
	if got != want {
		t.Fatalf("rewritten baseline line:\n%s", got)
	}
}

func writeDoccountsFixture(t *testing.T, root string) {
	t.Helper()
	writeAt(t, root, SpecCompliancePath, `# Compliance

**No external referee:** self-assessed.

| Rule | Status |
|---|---|
| a | ✅ Faithful |
`)
	writeAt(t, root, "docs/project/pilot-differential-baseline.json", `{"pilotRelease":"2026-05 (jupyter-sysml-kernel 0.60.1)","totals":{"files":2,"filesFullyAgreeing":1,"openSysMLOnly":3,"pilotOnly":4},`+
		`"errata":{"registryEntries":2,"corrections":1,"documentedWithoutCorrection":1,"totals":{"files":2,"filesFullyAgreeing":2,"openSysMLOnly":2,"pilotOnly":4}}}`)
	writeAt(t, root, "docs/project/pilot-xpect-baseline.json", `{"kinds":[{"kind":"errors","rows":11,"agree":8,"wordingOnly":2,"sameLocation":1,"sameLine":1,"severityDiffers":0,"elsewhereInFile":0},{"kind":"scope","assertions":8,"agree":7}],`+
		`"errata":{"kinds":[{"kind":"errors","rows":11,"agree":9,"wordingOnly":2,"sameLocation":1,"sameLine":1}]}}`)
	writeAt(t, root, "docs/project/pilot-rejection-baseline.json", `{"totals":{"cases":12,"bothReject":11,"pilotOnlyRejects":1},"strictOnlyAgreements":["a","b"],`+
		`"errata":{"totals":{"cases":12,"bothReject":11,"pilotOnlyRejects":1}}}`)
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
