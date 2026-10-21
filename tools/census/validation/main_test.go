package validation

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/census/manifest"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/baseline"
	"github.com/Open-MBEE/OpenSysML/tools/oracle/repo"
)

// TestCensusIsCurrent is the gate in test form: the committed manifest, the
// census document and the probes must agree, and the manifest must list what
// the pinned jar contains whenever the jar is provisioned.
func TestCensusIsCurrent(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCheck(root, options{}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

// TestExtractionMatchesBaseline compares a fresh extraction with the committed
// manifest when the jar is provisioned, so a stale manifest fails here too.
func TestExtractionMatchesBaseline(t *testing.T) {
	root, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := baseline.ReadPin(root)
	if err != nil {
		t.Fatal(err)
	}
	jar, present, err := options{}.jarPath(root, pin)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Skipf("pinned jar not provisioned at %s", jar)
	}
	extracted, err := extractFromJar(jar)
	if err != nil {
		t.Fatal(err)
	}
	base, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := matches(base, extracted); err != nil {
		t.Fatal(err)
	}
	if pilot := base.PilotRows(); len(extracted) != len(pilot) {
		t.Fatalf("extracted %d constraints, manifest records %d", len(extracted), len(pilot))
	}
}

// TestRecordedDateFollowsContent: an unchanged baseline keeps its date, a
// changed one is stamped today, and a first recording is dated.
func TestRecordedDateFollowsContent(t *testing.T) {
	previous := testManifest(testRow("validateA", "kerml", manifest.StatusFaithful))
	previous.Recorded = "2020-01-01"
	same := testManifest(testRow("validateA", "kerml", manifest.StatusFaithful))
	if got := recordedDate(previous, same); got != "2020-01-01" {
		t.Errorf("unchanged baseline re-dated to %q", got)
	}
	changed := testManifest(testRow("validateA", "kerml", manifest.StatusFaithful), testRow("validateB", "sysml", manifest.StatusUnknown))
	if got := recordedDate(previous, changed); got != baseline.Today() {
		t.Errorf("changed baseline dated %q, want today", got)
	}
	if got := recordedDate(nil, same); got != baseline.Today() {
		t.Errorf("first recording dated %q, want today", got)
	}
	previous.Recorded = ""
	if got := recordedDate(previous, same); got != baseline.Today() {
		t.Errorf("undated previous baseline kept %q, want today", got)
	}
}

// TestUpdateIsIdempotentAcrossDays runs -update over a copy of the committed
// manifest carrying an old date, reading the jar through a renamed link; it must
// not change a byte, and the result must pass -check.
func TestUpdateIsIdempotentAcrossDays(t *testing.T) {
	checkout, err := repo.Root()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := baseline.ReadPin(checkout)
	if err != nil {
		t.Fatal(err)
	}
	jar, present, err := options{}.jarPath(checkout, pin)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Skipf("pinned jar not provisioned at %s", jar)
	}
	root := t.TempDir()
	for _, rel := range []string{baseline.PinPath, manifest.Path} {
		content, err := os.ReadFile(filepath.Join(checkout, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	base.Recorded = "2020-01-01"
	if err := manifest.Write(root, base); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Path)))
	if err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(root, "local-copy.jar")
	if err := os.Symlink(jar, renamed); err != nil {
		t.Skipf("cannot link the jar: %v", err)
	}
	if err := runUpdate(root, options{jar: renamed}, io.Discard); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Path)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("-update over an unchanged extraction rewrote the manifest:\n%s", after)
	}
	updated, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRecord(updated); err != nil {
		t.Fatal(err)
	}
	if err := compareJar(root, updated, options{jar: renamed}, io.Discard); err != nil {
		t.Fatalf("a manifest recorded from a renamed copy must compare: %v", err)
	}
}

// TestUpdateAdoptsASpecSourcedRow: a spec-sourced row whose name a new jar
// names becomes the pilot row under that name — once, keeping its clause and
// evidence, without its OCL — and the other spec-sourced rows stay last.
func TestUpdateAdoptsASpecSourcedRow(t *testing.T) {
	adopted := testRow("validateB", "spec", manifest.StatusUnknown)
	adopted.Spec.Name, adopted.OCL = "validateB", "b = true"
	kept := testRow("validateC", "spec", manifest.StatusUnknown)
	kept.Spec.Name = "validateC"
	previous := testManifest(testRow("validateA", "kerml", manifest.StatusFaithful), adopted, kept)
	if err := previous.Validate(); err != nil {
		t.Fatal(err)
	}
	next := *previous
	next.Constraints = recordRows(previous, []Extracted{{Name: "validateA", Source: "kerml"}, {Name: "validateB", Raw: "validateB_", Source: "sysml"}})
	if err := next.Validate(); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range next.Constraints {
		names = append(names, c.Name+"/"+c.Source)
	}
	if got, want := strings.Join(names, ","), "validateA/kerml,validateB/sysml,validateC/spec"; got != want {
		t.Fatalf("rows %s, want %s", got, want)
	}
	row, _ := next.Constraint("validateB")
	if row.OCL != "" || row.Raw != "validateB_" || row.Spec == nil || row.Spec.Name != "validateB" || row.Spec.Clause != "8.3.2.1.2" || row.Status != manifest.StatusUnknown {
		t.Fatalf("adopted row %+v", row)
	}
}

// TestStringConstantsReadsThePool builds a class file whose pool holds one
// string constant and one bare Utf8 entry, and expects only the former.
func TestStringConstantsReadsThePool(t *testing.T) {
	var pool bytes.Buffer
	write := func(v any) {
		if err := binary.Write(&pool, binary.BigEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	write(uint32(0xCAFEBABE))
	write(uint16(0))
	write(uint16(65))
	write(uint16(6)) // constant_pool_count: entries 1..5
	// 1: Utf8 "validateFoo_"
	write(uint8(1))
	write(uint16(len("validateFoo_")))
	pool.WriteString("validateFoo_")
	// 2: String -> 1
	write(uint8(8))
	write(uint16(1))
	// 3: Utf8 "validateBareMethodName" (not a string constant)
	write(uint8(1))
	write(uint16(len("validateBareMethodName")))
	pool.WriteString("validateBareMethodName")
	// 4: Long (two slots)
	write(uint8(5))
	write(uint64(7))
	got, err := stringConstants(pool.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "validateFoo_" {
		t.Fatalf("string constants = %q, want [validateFoo_]", got)
	}
	if _, err := stringConstants([]byte{1, 2, 3}); err == nil {
		t.Fatal("a non-class file must be rejected")
	}
}

// testProvenance are the provenance lines of a census document whose manifest
// is testManifest.
const testProvenance = "**Pilot:** [Pilot](https://example.test) release `2026-07`, commit `c7fc737d`, artifact `jupyter-sysml-kernel 0.61.0` — the pin\n" +
	"**Jar:** `kernel-0.61.0-all.jar` (`sha256:abc`), provisioned by a script\n"

// testSpecifications are the pinned documents the test rows cite.
var testSpecifications = map[string]manifest.Specification{
	"kerml": {Name: "KerML", Version: "1.0", Document: "formal/26-03-01", SHA256: "abc"},
}

func testManifest(constraints ...manifest.Constraint) *manifest.Manifest {
	return &manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		PilotTag:      "2026-07", PilotCommit: "c7fc737d", PilotArtifact: "0.61.0",
		Jar:            manifest.JarRecord{Name: "kernel-0.61.0-all.jar", Digest: "sha256:abc"},
		Extraction:     extractionRecord(),
		Recorded:       "2026-09-02",
		Specifications: testSpecifications,
		Constraints:    constraints,
	}
}

// testRow is a minimal consistent row: a metaclass, a clause, a rationale for
// every status but faithful and, for an implemented one, a diagnostic code.
func testRow(name, source, status string) manifest.Constraint {
	short := strings.ToLower(name[len("validate"):])
	c := manifest.Constraint{Name: name, Source: source, Kind: "validate", Metaclass: "Element",
		Spec:   &manifest.SpecRef{Document: "kerml", Clause: "8.3.2.1.2"},
		Checks: short, Implementation: "—", Message: "—", NegativeCase: "none", Codes: []string{}, Passes: []string{}, Status: status}
	if status != manifest.StatusFaithful {
		c.Rationale = "no case and no identifiable pass yet"
	}
	if manifest.Implemented(status) {
		c.Codes = []string{"code-" + short}
		c.Passes = []string{"constraint"}
	}
	return c
}

// TestValidateRequiresARecordingDate: the manifest must carry an ISO calendar
// date that exists, like every other committed provenance record.
func TestValidateRequiresARecordingDate(t *testing.T) {
	if err := validateRecord(testManifest()); err != nil {
		t.Fatalf("a dated manifest must validate: %v", err)
	}
	for name, date := range map[string]string{
		"missing":    "",
		"prose":      "August 2026",
		"timestamp":  "2026-09-02T00:00:00Z",
		"impossible": "2026-02-30",
	} {
		t.Run(name, func(t *testing.T) {
			base := testManifest()
			base.Recorded = date
			err := validateRecord(base)
			if err == nil || !strings.Contains(err.Error(), "records no ISO recording date") {
				t.Fatalf("error = %v, want the missing date named", err)
			}
		})
	}
}

// TestCompareJarChecksTheRecordedName: a manifest naming a jar other than the
// pinned artifact fails before any digest is read, jar present or not.
func TestCompareJarChecksTheRecordedName(t *testing.T) {
	root := t.TempDir()
	pinPath := filepath.Join(root, filepath.FromSlash(baseline.PinPath))
	if err := os.MkdirAll(filepath.Dir(pinPath), 0o755); err != nil {
		t.Fatal(err)
	}
	pin := "PILOT_TAG=\"${PILOT_TAG:-2026-07}\"\n" +
		"PILOT_COMMIT=\"${PILOT_COMMIT:-c7fc737d56da9e2d78f9d7df6d38efbec2e7e965}\"\n" +
		"PILOT_ARTIFACT_VERSION=\"${PILOT_ARTIFACT_VERSION:-0.61.0}\"\n"
	if err := os.WriteFile(pinPath, []byte(pin), 0o644); err != nil {
		t.Fatal(err)
	}
	base := testManifest()
	base.PilotCommit = "c7fc737d56da9e2d78f9d7df6d38efbec2e7e965"
	base.Jar.Name = "jupyter-sysml-kernel-0.61.0-all.jar"
	if err := compareJar(root, base, options{}, io.Discard); err != nil {
		t.Fatalf("the pinned name must pass without a jar: %v", err)
	}
	base.Jar.Name = "jupyter-sysml-kernel-0.62.0-all.jar"
	err := compareJar(root, base, options{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), `records jar "jupyter-sysml-kernel-0.62.0-all.jar" but the pin names "jupyter-sysml-kernel-0.61.0-all.jar"`) {
		t.Fatalf("error = %v, want the recorded name rejected", err)
	}
}

// testTableMarkers wrap the generated census table in a document under test.
const testTableMarkers = "<!-- doc-counts:begin validation-census-table -->\nstale table\n<!-- doc-counts:end validation-census-table -->\n"

// TestRewriteDerivedLinesRestatesTheSummary checks the provenance and summary
// lines and the census table are rewritten from the manifest and nothing else moves.
func TestRewriteDerivedLinesRestatesTheSummary(t *testing.T) {
	base := testManifest(
		testRow("validateA", "kerml", manifest.StatusFaithful),
		testRow("validateB", "sysml", manifest.StatusApproximate),
		testRow("validateC", "kerml", manifest.StatusNotImplemented),
		testRow("validateD", "both", manifest.StatusUnknown),
	)
	stale := "**Pilot:** [Pilot](https://example.test) release `2025-01`, commit `old`, artifact `jupyter-sysml-kernel 0.50.0` — the pin\n" +
		"**Jar:** `kernel-0.50.0-all.jar` (`sha256:old`), provisioned by a script\n"
	content := "# Census\n\n" + stale + "\n**Census:** 0 of 0 named constraints are reported by OpenSysML — 0 ✅ faithful and 0 ⚠️ approximate; 0 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 0 ❔ unknown.\n\n" + testTableMarkers + "\ntrailing\n"
	got, err := rewriteDerivedLines(content, base)
	if err != nil {
		t.Fatal(err)
	}
	table := "<!-- doc-counts:begin validation-census-table -->\n" +
		"| Constraint | Language | Checks | Implementation | Our message | Negative case | Status |\n" +
		"|---|---|---|---|---|---|---|\n" +
		"| `validateA` | KerML | a | — | — | none | ✅ faithful |\n" +
		"| `validateC` | KerML | c | — | — | none | ❌ not implemented — no case and no identifiable pass yet |\n" +
		"| `validateD` | both | d | — | — | none | ❔ unknown — no case and no identifiable pass yet |\n" +
		"| `validateB` | SysML | b | — | — | none | ⚠️ approximate — no case and no identifiable pass yet |\n" +
		"<!-- doc-counts:end validation-census-table -->\n"
	want := "# Census\n\n" + testProvenance + "\n**Census:** 2 of 4 named constraints are reported by OpenSysML — 1 ✅ faithful and 1 ⚠️ approximate; 1 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 1 ❔ unknown.\n\n" + table + "\ntrailing\n"
	if got != want {
		t.Fatalf("rewrite:\n%s\nwant:\n%s", got, want)
	}
	if _, err := rewriteDerivedLines(testProvenance+"no summary here\n"+testTableMarkers, base); err == nil {
		t.Fatal("a document without the summary line must be rejected")
	}
	if _, err := rewriteDerivedLines(testProvenance+"**Census:** old\n", base); err == nil {
		t.Fatal("a document without the table block must be rejected")
	}
	if _, err := rewriteDerivedLines("**Pilot:** unrecognised\n", base); err == nil {
		t.Fatal("a provenance line the pattern does not match must be rejected")
	}
}

// writeTestRepo lays out the corpus, rejection record and Go sources a census
// document under test cites. Each entry of files is repository-relative.
func writeTestRepo(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestCheckDocumentRejectsDrift covers the drifts the gate exists for: a table
// edited by hand or stale against the manifest, a hand-edited figure, an
// implementation location that resolves to nothing, and a negative case that
// does not exist, is attributed elsewhere, or is bucketed against its row.
func TestCheckDocumentRejectsDrift(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(negativeCorpusDir), "kerml", "dir.kerml"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestRepo(t, root, map[string]string{
		negativeCorpusDir + "/kerml/a.kerml":      "// Invalid: a (KerML 7.1; pilot validateA).\npackage P;\n",
		negativeCorpusDir + "/kerml/b.kerml":      "// Invalid: b (KerML 7.2; pilot validateB).\npackage P;\n",
		negativeCorpusDir + "/kerml/other.kerml":  "// Invalid: a (KerML 7.1; pilot validateA).\npackage P;\n",
		negativeCorpusDir + "/kerml/prose.kerml":  "// Invalid: a, which validateB also requires (KerML 7.1; pilot validateA).\npackage P;\n",
		negativeCorpusDir + "/kerml/spec.kerml":   "// Invalid: a (KerML 7.1 validateA; pilot Fixture_invalid.kerml.xt).\npackage P;\n",
		negativeCorpusDir + "/xpect/x.kerml":      "// Invalid: a (KerML 7.1; pilot validateA).\npackage P;\n",
		negativeCorpusDir + "/xpect/bare.kerml":   "// Invalid: from the pilot's Xpect suite.\npackage P;\n",
		negativeCorpusDir + "/kerml/d.kerml":      "// Invalid: d (KerML 7.4; pilot validateD).\npackage P;\n",
		negativeCorpusDir + "/kerml/d-open.kerml": "// Invalid: d (KerML 7.4 validateD; pilot Fixture_open.kerml.xt).\npackage P;\n",
		negativeCorpusDir + "/kerml/d-odd.kerml":  "// Invalid: d (KerML 7.4 validateD; pilot Fixture_odd.kerml.xt).\npackage P;\n",
		rejectionBaselinePath: `{"cases": [
			{"path": "kerml/a.kerml", "bucket": "both-reject"},
			{"path": "kerml/b.kerml", "bucket": "pilot-only-rejects"},
			{"path": "kerml/other.kerml", "bucket": "both-reject"},
			{"path": "kerml/prose.kerml", "bucket": "both-reject"},
			{"path": "kerml/spec.kerml", "bucket": "both-reject"},
			{"path": "xpect/x.kerml", "bucket": "ours-only-rejects"},
			{"path": "xpect/bare.kerml", "bucket": "both-reject"},
			{"path": "kerml/d.kerml", "bucket": "pilot-only-rejects"},
			{"path": "kerml/d-open.kerml", "bucket": "both-accept"},
			{"path": "kerml/d-odd.kerml", "bucket": "rejected"}]}`,
		"internal/check/passes/a.go": "package passes\n\ntype APass struct{}\n\nfunc (APass) Run() {}\n\nfunc helper() {}\n\ntype Arena[T any] struct{}\n\nfunc (a *Arena[T]) Take() {}\n",
	})
	rowA := testRow("validateA", "kerml", manifest.StatusFaithful)
	rowA.Implementation = "internal/check/passes/a.go:APass.Run (and internal/check/passes/a.go:helper, internal/check/passes/a.go:Arena.Take)."
	rowA.Message = "same"
	rowA.NegativeCase = "`kerml/a.kerml`, `kerml/other.kerml`, `kerml/prose.kerml`, `kerml/spec.kerml`, `xpect/x.kerml`"
	rowB := testRow("validateB", "sysml", manifest.StatusNotImplemented)
	rowB.NegativeCase = "`kerml/b.kerml`"
	rowC := testRow("validateC", "sysml", manifest.StatusUnknown)
	rowD := testRow("validateD", "kerml", manifest.StatusApproximate)
	rowD.Implementation = "internal/check/passes/a.go:helper"
	rowD.Message = "same"
	rowD.NegativeCase = "`kerml/d.kerml`"
	base := testManifest(rowA, rowB, rowC, rowD)
	doc, err := rewriteDerivedLines(testProvenance+"**Census:** stale\n\n"+testTableMarkers, base)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkDocument(root, doc, base); err != nil {
		t.Fatalf("a consistent document must pass: %v", err)
	}
	// edit returns a copy of the manifest with one row changed by set.
	edit := func(name string, set func(*manifest.Constraint)) *manifest.Manifest {
		m := *base
		m.Constraints = append([]manifest.Constraint(nil), base.Constraints...)
		for i := range m.Constraints {
			if m.Constraints[i].Name == name {
				set(&m.Constraints[i])
			}
		}
		return &m
	}
	editNegative := func(name, old, replacement string) *manifest.Manifest {
		return edit(name, func(c *manifest.Constraint) { c.NegativeCase = strings.Replace(c.NegativeCase, old, replacement, 1) })
	}
	editImplementation := func(name, old, replacement string) *manifest.Manifest {
		return edit(name, func(c *manifest.Constraint) {
			c.Implementation = strings.Replace(c.Implementation, old, replacement, 1)
		})
	}
	withNegative := func(m *manifest.Manifest, name, cell string) *manifest.Manifest {
		for i := range m.Constraints {
			if m.Constraints[i].Name == name {
				m.Constraints[i].NegativeCase = cell
			}
		}
		return m
	}
	rowBText := "| `validateB` | SysML | b | — | — | `kerml/b.kerml` | ❌ not implemented — no case and no identifiable pass yet |\n"
	cases := map[string]struct {
		manifest *manifest.Manifest
		mutate   func(string) string
		want     string
	}{
		"extra row": {
			mutate: func(s string) string {
				return strings.Replace(s, rowBText, rowBText+"| `validateE` | SysML | e | — | — | none | ❌ not implemented |\n", 1)
			},
			want: "the census table is stale",
		},
		"missing row": {
			mutate: func(s string) string { return strings.Replace(s, rowBText, "", 1) },
			want:   "the census table is stale",
		},
		"hand-edited figure": {
			mutate: func(s string) string { return strings.Replace(s, "2 of 4", "3 of 4", 1) },
			want:   "is stale",
		},
		"stale release": {
			mutate: func(s string) string { return strings.Replace(s, "release `2026-07`", "release `2025-01`", 1) },
			want:   "is stale",
		},
		"stale commit": {
			mutate: func(s string) string { return strings.Replace(s, "commit `c7fc737d`", "commit `deadbeef`", 1) },
			want:   "is stale",
		},
		"stale artifact": {
			mutate: func(s string) string { return strings.Replace(s, "kernel 0.61.0`", "kernel 0.62.0`", 1) },
			want:   "is stale",
		},
		"stale jar name": {
			mutate: func(s string) string {
				return strings.Replace(s, "`kernel-0.61.0-all.jar`", "`kernel-0.62.0-all.jar`", 1)
			},
			want: "is stale",
		},
		"stale digest": {
			mutate: func(s string) string { return strings.Replace(s, "`sha256:abc`", "`sha256:def`", 1) },
			want:   "is stale",
		},
		"status edited in the table": {
			mutate: func(s string) string { return strings.Replace(s, "| ✅ faithful |", "| ⚠️ approximate |", 1) },
			want:   "the census table is stale",
		},
		"status carries a suffix in the table": {
			mutate: func(s string) string { return strings.Replace(s, "| ✅ faithful |", "| ✅ faithfulish |", 1) },
			want:   "the census table is stale",
		},
		"rationale dropped from the table": {
			mutate: func(s string) string {
				return strings.Replace(s, "| ❔ unknown — no case and no identifiable pass yet |", "| ❔ unknown |", 1)
			},
			want: "the census table is stale",
		},
		"language edited in the table": {
			mutate: func(s string) string {
				return strings.Replace(s, "| `validateA` | KerML |", "| `validateA` | SysML |", 1)
			},
			want: "the census table is stale",
		},
		"table markers removed": {
			mutate: func(s string) string { return strings.ReplaceAll(s, "validation-census-table", "renamed") },
			want:   "is missing or unterminated",
		},
		"status edited in the manifest": {
			manifest: edit("validateA", func(c *manifest.Constraint) { c.Status = manifest.StatusApproximate; c.Rationale = "x" }),
			mutate:   func(s string) string { return s },
			want:     "the census table is stale",
		},
		"implementation dropped from an implemented row": {
			manifest: edit("validateD", func(c *manifest.Constraint) { c.Implementation = "—" }),
			want:     "validateD is recorded approximate but names no implementation",
		},
		"missing negative case": {
			manifest: editNegative("validateA", "`kerml/a.kerml`", "`kerml/gone.kerml`"),
			want:     "does not exist",
		},
		"negative case is a directory": {
			manifest: editNegative("validateA", "`kerml/a.kerml`", "`kerml/dir.kerml`"),
			want:     "is not a file",
		},
		"negative case is not a model": {
			manifest: editNegative("validateA", "`kerml/a.kerml`", "`kerml`"),
			want:     "is not a .sysml or .kerml file",
		},
		"negative case doubles a backtick": {
			manifest: editNegative("validateA", "`kerml/a.kerml`", "``kerml/a.kerml`"),
			want:     "neither `none` nor a backticked corpus path",
		},
		"implementation file is missing": {
			manifest: editImplementation("validateA", "internal/check/passes/a.go:APass.Run", "internal/check/passes/gone.go:APass.Run"),
			want:     "implementation: internal/check/passes/gone.go does not exist",
		},
		"implementation method is missing": {
			manifest: editImplementation("validateA", "a.go:APass.Run", "a.go:APass.Check"),
			want:     "internal/check/passes/a.go declares no APass.Check",
		},
		"implementation function is missing": {
			manifest: editImplementation("validateA", "a.go:helper", "a.go:renamed"),
			want:     "internal/check/passes/a.go declares no renamed",
		},
		"implementation cites no location": {
			manifest: edit("validateA", func(c *manifest.Constraint) { c.Implementation = "internal/check/passes/a.go (APass)" }),
			want:     "validateA implementation \"internal/check/passes/a.go (APass)\" cites no internal/<file>.go:<function> location",
		},
		"implementation cites a file by name only": {
			manifest: editImplementation("validateA", "internal/check/passes/a.go:helper,", "a.go:helper,"),
			want:     "validateA implementation a.go:helper is not a repository-relative internal/<file>.go location",
		},
		"implementation cites a missing symbol in a file by name only": {
			manifest: editImplementation("validateA", "internal/check/passes/a.go:helper,", "a.go:renamed,"),
			want:     "validateA implementation a.go:renamed is not a repository-relative internal/<file>.go location",
		},
		"implementation continues past the method": {
			manifest: editImplementation("validateA", "a.go:APass.Run ", "a.go:APass.Run.extra "),
			want:     "internal/check/passes/a.go:APass.Run.extra is not a <function> or <Type>.<method> location",
		},
		"implementation continues past the function with a dash": {
			manifest: editImplementation("validateA", "a.go:helper,", "a.go:helper-extra,"),
			want:     "internal/check/passes/a.go:helper-extra is not a <function> or <Type>.<method> location",
		},
		"implementation continues past the function with a slash": {
			manifest: editImplementation("validateA", "a.go:helper,", "a.go:helper/extra,"),
			want:     "internal/check/passes/a.go:helper/extra is not a <function> or <Type>.<method> location",
		},
		"implementation continues past the function with a colon": {
			manifest: editImplementation("validateA", "a.go:helper,", "a.go:helper:extra,"),
			want:     "internal/check/passes/a.go:helper:extra is not a <function> or <Type>.<method> location",
		},
		"generic method is missing": {
			manifest: editImplementation("validateA", "a.go:Arena.Take", "a.go:Arena.Put"),
			want:     "internal/check/passes/a.go declares no Arena.Put",
		},
		"negative case attributed to another constraint": {
			manifest: withNegative(editNegative("validateA", "`kerml/a.kerml`, `kerml/other.kerml`", "`kerml/a.kerml`, `kerml/b.kerml`, `kerml/other.kerml`"), "validateB", "`kerml/a.kerml`"),
			want:     "validateA negative case kerml/b.kerml is attributed to validateB, not to this constraint",
		},
		"negative case names the constraint in prose but its pilot token names another": {
			manifest: withNegative(editNegative("validateA", ", `kerml/prose.kerml`", ""), "validateB", "`kerml/b.kerml`, `kerml/prose.kerml`"),
			want:     "validateB negative case kerml/prose.kerml is attributed to validateA, not to this constraint",
		},
		"negative case cites the constraint in its specification citation only and another row lists it": {
			manifest: withNegative(editNegative("validateA", ", `kerml/spec.kerml`", ""), "validateB", "`kerml/b.kerml`, `kerml/spec.kerml`"),
			want:     "validateB negative case kerml/spec.kerml is attributed to validateA, not to this constraint",
		},
		"negative case names no constraint in its header": {
			manifest: editNegative("validateA", "`xpect/x.kerml`", "`xpect/x.kerml`, `xpect/bare.kerml`"),
			want:     "validateA negative case xpect/bare.kerml names no constraint in its header",
		},
		"faithful row lists a pilot-only case": {
			manifest: withNegative(editNegative("validateA", "`kerml/a.kerml`, `kerml/other.kerml`", "`kerml/a.kerml`, `kerml/b.kerml`, `kerml/other.kerml`"), "validateB", "`kerml/a.kerml`"),
			want:     "validateA is recorded faithful but docs/project/pilot-rejection-baseline.json records its negative case kerml/b.kerml as pilot-only-rejects",
		},
		"not-implemented row lists a case we reject": {
			manifest: withNegative(edit("validateB", func(*manifest.Constraint) {}), "validateB", "`kerml/b.kerml`, `xpect/x.kerml`"),
			want:     "validateB is recorded not-implemented but docs/project/pilot-rejection-baseline.json records its negative case xpect/x.kerml as ours-only-rejects",
		},
		"approximate row lists a case both validators accept": {
			manifest: editNegative("validateD", "`kerml/d.kerml`", "`kerml/d.kerml`, `kerml/d-open.kerml`"),
			want:     "validateD negative case kerml/d-open.kerml is recorded as both-accept in docs/project/pilot-rejection-baseline.json, which is not a rejection",
		},
		"approximate row lists a case with an unknown bucket": {
			manifest: editNegative("validateD", "`kerml/d.kerml`", "`kerml/d.kerml`, `kerml/d-odd.kerml`"),
			want:     "validateD negative case kerml/d-odd.kerml is recorded as rejected in docs/project/pilot-rejection-baseline.json, which is not a rejection",
		},
		"faithful row lists a case both validators accept": {
			manifest: withNegative(editNegative("validateA", "`xpect/x.kerml`", "`xpect/x.kerml`, `kerml/d-open.kerml`"), "validateD", "none"),
			want:     "validateA negative case kerml/d-open.kerml is recorded as both-accept in docs/project/pilot-rejection-baseline.json, which is not a rejection",
		},
		"unknown row lists a case": {
			manifest: editNegative("validateC", "none", "`xpect/x.kerml`"),
			want:     "validateC is recorded unknown but lists negative case xpect/x.kerml",
		},
		"attributed case is listed nowhere": {
			manifest: editNegative("validateA", ", `kerml/other.kerml`", ""),
			want:     "kerml/other.kerml attributes its rejection to validateA, whose row does not list it",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m, content := base, doc
			if tc.manifest != nil {
				m = tc.manifest
				rendered, err := rewriteDerivedLines(doc, m)
				if err != nil {
					t.Fatal(err)
				}
				content = rendered
			}
			if tc.mutate != nil {
				// A hand edit of the page, or a manifest edit the page was not regenerated for.
				content = tc.mutate(doc)
			}
			err := checkDocument(root, content, m)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestCheckProbesCoverage covers what the probe check demands: a probe for every
// implemented row, one per notation when both validators declare the
// constraint, and none for a row recorded as unreported.
func TestCheckProbesCoverage(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, filepath.FromSlash(probesDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, constraint string) {
		content := "// Census: " + constraint + "\n// Expect: error: x\npackage P;\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	base := testManifest(
		testRow("validateA", "kerml", manifest.StatusFaithful),
		testRow("validateB", "both", manifest.StatusApproximate),
		testRow("validateC", "sysml", manifest.StatusNotImplemented),
	)
	write("validateA.sysml", "validateA")
	write("validateB.kerml", "validateB")
	err := checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateB is recorded approximate in both notations but no .sysml probe") {
		t.Fatalf("a shared constraint with one notation probed must fail, got %v", err)
	}
	write("validateB.sysml", "validateB")
	if err := checkProbes(root, base); err != nil {
		t.Fatalf("both notations probed must pass: %v", err)
	}
	write("validateC.sysml", "validateC")
	err = checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateC, which the manifest records as not-implemented") {
		t.Fatalf("a probe for an unreported constraint must fail, got %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "validateC.sysml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "validateA.sysml")); err != nil {
		t.Fatal(err)
	}
	err = checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateA is recorded faithful but no probe") {
		t.Fatalf("an implemented row without a probe must fail, got %v", err)
	}
	write("validateA.sysml", "validateA")
	base.Constraints[0].Codes = nil
	err = checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateA is recorded faithful but records no diagnostic code or pass") {
		t.Fatalf("an implemented row without a code must fail, got %v", err)
	}
	base.Constraints[0].Codes = []string{"a"}
	base.Constraints[2].Passes = []string{"constraint"}
	err = checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "validateC is recorded not-implemented but records diagnostic codes") {
		t.Fatalf("an unreported row with a code must fail, got %v", err)
	}
	base.Constraints[2].Passes = []string{}
	derived := testRow("validateD", "spec", manifest.StatusFaithful)
	derived.Name, derived.Kind, derived.Codes, derived.Passes = "deriveD", "derive", []string{}, []string{}
	derived.Spec.Name, derived.Implementation = "deriveD", "internal/semantic/d.go:D"
	checked := testRow("validateE", "spec", manifest.StatusApproximate)
	checked.Name, checked.Kind, checked.Codes, checked.Passes = "checkE", "check", []string{}, []string{}
	checked.Spec.Name, checked.Implementation = "checkE", "internal/semantic/e.go:E"
	base.Constraints = append(base.Constraints, checked, derived)
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := checkProbes(root, base); err != nil {
		t.Fatalf("an implemented check* or derive* row needs no probe or code: %v", err)
	}
	write("checkE.sysml", "checkE")
	if err := checkProbes(root, base); err != nil {
		t.Fatalf("a probe may back an implemented check* row: %v", err)
	}
	base.Constraints[3].Status, base.Constraints[3].Rationale = manifest.StatusUnknown, "not yet assessed"
	err = checkProbes(root, base)
	if err == nil || !strings.Contains(err.Error(), "checkE, which the manifest records as unknown") {
		t.Fatalf("a probe for an unreported check* row must fail, got %v", err)
	}
}

// TestBaselineMatchesNamesEverySide checks the jar comparison names a removed and
// an added constraint.
func TestBaselineMatchesNamesEverySide(t *testing.T) {
	rowB := testRow("validateB", "sysml", manifest.StatusUnknown)
	rowB.Raw = "validateB_"
	base := testManifest(testRow("validateA", "kerml", manifest.StatusUnknown), rowB)
	if err := matches(base, []Extracted{{Name: "validateA", Source: "kerml"}, {Name: "validateB", Raw: "validateB_", Source: "sysml"}}); err != nil {
		t.Fatal(err)
	}
	err := matches(base, []Extracted{{Name: "validateA", Source: "kerml"}, {Name: "validateC", Source: "sysml"}})
	if err == nil {
		t.Fatal("a differing list must be rejected")
	}
	for _, want := range []string{"validateB is in the manifest but not in the jar", "validateC is in the jar but not in the manifest"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
