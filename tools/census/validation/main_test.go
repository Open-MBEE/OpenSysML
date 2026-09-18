package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/baseline"
)

// TestCensusIsCurrent is the gate in test form: the committed baseline, the
// census document and the probes must agree, and the baseline must list what
// the pinned jar contains whenever the jar is provisioned.
func TestCensusIsCurrent(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCheck(root, options{}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

// TestExtractionMatchesBaseline compares a fresh extraction with the committed
// baseline when the jar is provisioned, so a stale baseline fails here too.
func TestExtractionMatchesBaseline(t *testing.T) {
	root, err := moduleRoot()
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
	base, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := base.matches(extracted); err != nil {
		t.Fatal(err)
	}
	if len(extracted) != len(base.Constraints) {
		t.Fatalf("extracted %d constraints, baseline records %d", len(extracted), len(base.Constraints))
	}
}

// TestRecordedDateFollowsContent: an unchanged baseline keeps its date, a
// changed one is stamped today, and a first recording is dated.
func TestRecordedDateFollowsContent(t *testing.T) {
	previous := testBaseline(Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful})
	previous.Recorded = "2020-01-01"
	same := testBaseline(Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful})
	if got := recordedDate(previous, same); got != "2020-01-01" {
		t.Errorf("unchanged baseline re-dated to %q", got)
	}
	changed := testBaseline(Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful}, Constraint{Name: "validateB", Source: "sysml", Status: StatusUnknown})
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
// baseline carrying an old date, reading the jar through a renamed link; it must
// not change a byte, and the result must pass -check.
func TestUpdateIsIdempotentAcrossDays(t *testing.T) {
	repo, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	pin, err := baseline.ReadPin(repo)
	if err != nil {
		t.Fatal(err)
	}
	jar, present, err := options{}.jarPath(repo, pin)
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Skipf("pinned jar not provisioned at %s", jar)
	}
	root := t.TempDir()
	for _, rel := range []string{baseline.PinPath, baselinePath} {
		content, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(rel)))
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
	base, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	base.Recorded = "2020-01-01"
	if err := writeBaseline(root, base); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
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
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("-update over an unchanged extraction rewrote the baseline:\n%s", after)
	}
	updated, err := loadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := updated.validate(); err != nil {
		t.Fatal(err)
	}
	if err := compareJar(root, updated, options{jar: renamed}, io.Discard); err != nil {
		t.Fatalf("a baseline recorded from a renamed copy must compare: %v", err)
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

// testProvenance are the provenance lines of a census document whose baseline
// is testBaseline.
const testProvenance = "**Pilot:** [Pilot](https://example.test) release `2026-07`, commit `c7fc737d`, artifact `jupyter-sysml-kernel 0.61.0` — the pin\n" +
	"**Jar:** `kernel-0.61.0-all.jar` (`sha256:abc`), provisioned by a script\n"

func testBaseline(constraints ...Constraint) *Baseline {
	return &Baseline{
		PilotTag: "2026-07", PilotCommit: "c7fc737d", PilotArtifact: "0.61.0",
		Jar:         JarRecord{Name: "kernel-0.61.0-all.jar", Digest: "sha256:abc"},
		Extraction:  extractionRecord(),
		Recorded:    "2026-09-02",
		Constraints: constraints,
	}
}

// TestValidateRequiresARecordingDate: the baseline must carry an ISO calendar
// date that exists, like every other committed provenance record.
func TestValidateRequiresARecordingDate(t *testing.T) {
	if err := testBaseline().validate(); err != nil {
		t.Fatalf("a dated baseline must validate: %v", err)
	}
	for name, date := range map[string]string{
		"missing":    "",
		"prose":      "August 2026",
		"timestamp":  "2026-09-02T00:00:00Z",
		"impossible": "2026-02-30",
	} {
		t.Run(name, func(t *testing.T) {
			base := testBaseline()
			base.Recorded = date
			err := base.validate()
			if err == nil || !strings.Contains(err.Error(), "records no ISO recording date") {
				t.Fatalf("error = %v, want the missing date named", err)
			}
		})
	}
}

// TestCompareJarChecksTheRecordedName: a baseline naming a jar other than the
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
	base := testBaseline()
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

// TestRewriteDerivedLinesRestatesTheSummary checks the provenance and summary
// lines are rewritten from the baseline and nothing else moves.
func TestRewriteDerivedLinesRestatesTheSummary(t *testing.T) {
	base := testBaseline(
		Constraint{Name: "validateA", Status: StatusFaithful},
		Constraint{Name: "validateB", Status: StatusApproximate},
		Constraint{Name: "validateC", Status: StatusNotImplemented},
		Constraint{Name: "validateD", Status: StatusUnknown},
	)
	stale := "**Pilot:** [Pilot](https://example.test) release `2025-01`, commit `old`, artifact `jupyter-sysml-kernel 0.50.0` — the pin\n" +
		"**Jar:** `kernel-0.50.0-all.jar` (`sha256:old`), provisioned by a script\n"
	content := "# Census\n\n" + stale + "\n**Census:** 0 of 0 named constraints are reported by OpenSysML — 0 ✅ faithful and 0 ⚠️ approximate; 0 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 0 ❔ unknown.\n\ntrailing\n"
	got, err := rewriteDerivedLines(content, base)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Census\n\n" + testProvenance + "\n**Census:** 2 of 4 named constraints are reported by OpenSysML — 1 ✅ faithful and 1 ⚠️ approximate; 1 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 1 ❔ unknown.\n\ntrailing\n"
	if got != want {
		t.Fatalf("rewrite:\n%s\nwant:\n%s", got, want)
	}
	if _, err := rewriteDerivedLines(testProvenance+"no summary here\n", base); err == nil {
		t.Fatal("a document without the summary line must be rejected")
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

// TestCheckDocumentRejectsDrift covers the drifts the gate exists for: a row the
// baseline lacks, a baseline name without a row, a hand-edited figure, an
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
		"internal/core/passes/a.go": "package passes\n\ntype APass struct{}\n\nfunc (APass) Run() {}\n\nfunc helper() {}\n\ntype Arena[T any] struct{}\n\nfunc (a *Arena[T]) Take() {}\n",
	})
	base := testBaseline(
		Constraint{Name: "validateA", Source: "kerml", Status: StatusFaithful},
		Constraint{Name: "validateB", Source: "sysml", Status: StatusNotImplemented},
		Constraint{Name: "validateC", Source: "sysml", Status: StatusUnknown},
		Constraint{Name: "validateD", Source: "kerml", Status: StatusApproximate},
	)
	doc := testProvenance + strings.Join([]string{
		"**Census:** 2 of 4 named constraints are reported by OpenSysML — 1 ✅ faithful and 1 ⚠️ approximate; 1 ❌ not implemented, 0 ⛔ deliberate, 0 🚧 known failure, 1 ❔ unknown.",
		"",
		"| Constraint | Language | Checks | Implementation | Our message | Negative case | Status |",
		"|---|---|---|---|---|---|---|",
		"| `validateA` | KerML | a | internal/core/passes/a.go:APass.Run (and internal/core/passes/a.go:helper, internal/core/passes/a.go:Arena.Take). | same | `kerml/a.kerml`, `kerml/other.kerml`, `kerml/prose.kerml`, `kerml/spec.kerml`, `xpect/x.kerml` | ✅ faithful |",
		"| `validateB` | SysML | b | — | — | `kerml/b.kerml` | ❌ not implemented |",
		"| `validateC` | SysML | c | — | — | none | ❔ unknown — no case and no identifiable pass yet |",
		"| `validateD` | KerML | d | internal/core/passes/a.go:helper | same | `kerml/d.kerml` | ⚠️ approximate |",
		"",
	}, "\n")
	if err := checkDocument(root, doc, base); err != nil {
		t.Fatalf("a consistent document must pass: %v", err)
	}
	cases := map[string]struct {
		mutate func(string) string
		want   string
	}{
		"extra row": {
			mutate: func(s string) string {
				row := "| `validateB` | SysML | b | — | — | `kerml/b.kerml` | ❌ not implemented |\n"
				return strings.Replace(s, row, row+"| `validateE` | SysML | e | — | — | none | ❌ not implemented |\n", 1)
			},
			want: "validateE is in the table but not in",
		},
		"missing row": {
			mutate: func(s string) string {
				return strings.Replace(s, "| `validateB` | SysML | b | — | — | `kerml/b.kerml` | ❌ not implemented |\n", "", 1)
			},
			want: "validateB is in docs/project/validation-constraints-baseline.json but has no row",
		},
		"hand-edited figure": {
			mutate: func(s string) string { return strings.Replace(s, "2 of 4", "3 of 4", 1) },
			want:   "line is stale",
		},
		"stale release": {
			mutate: func(s string) string { return strings.Replace(s, "release `2026-07`", "release `2025-01`", 1) },
			want:   "line is stale",
		},
		"stale commit": {
			mutate: func(s string) string { return strings.Replace(s, "commit `c7fc737d`", "commit `deadbeef`", 1) },
			want:   "line is stale",
		},
		"stale artifact": {
			mutate: func(s string) string { return strings.Replace(s, "kernel 0.61.0`", "kernel 0.62.0`", 1) },
			want:   "line is stale",
		},
		"stale jar name": {
			mutate: func(s string) string {
				return strings.Replace(s, "`kernel-0.61.0-all.jar`", "`kernel-0.62.0-all.jar`", 1)
			},
			want: "line is stale",
		},
		"stale digest": {
			mutate: func(s string) string { return strings.Replace(s, "`sha256:abc`", "`sha256:def`", 1) },
			want:   "line is stale",
		},
		"status disagrees": {
			mutate: func(s string) string { return strings.Replace(s, "| ✅ faithful |", "| ⚠️ approximate |", 1) },
			want:   "the baseline records faithful",
		},
		"status carries a suffix": {
			mutate: func(s string) string { return strings.Replace(s, "| ✅ faithful |", "| ✅ faithfulish |", 1) },
			want:   "the baseline records faithful",
		},
		"unknown status lacks its explanation": {
			mutate: func(s string) string {
				return strings.Replace(s, "| ❔ unknown — no case and no identifiable pass yet |", "| ❔ unknown |", 1)
			},
			want: "the baseline records unknown",
		},
		"language disagrees": {
			mutate: func(s string) string {
				return strings.Replace(s, "| `validateA` | KerML |", "| `validateA` | SysML |", 1)
			},
			want: "has language \"SysML\"",
		},
		"missing negative case": {
			mutate: func(s string) string { return strings.Replace(s, "`kerml/a.kerml`", "`kerml/gone.kerml`", 1) },
			want:   "does not exist",
		},
		"negative case is a directory": {
			mutate: func(s string) string { return strings.Replace(s, "`kerml/a.kerml`", "`kerml/dir.kerml`", 1) },
			want:   "is not a file",
		},
		"negative case is not a model": {
			mutate: func(s string) string { return strings.Replace(s, "`kerml/a.kerml`", "`kerml`", 1) },
			want:   "is not a .sysml or .kerml file",
		},
		"constraint cell doubles a backtick": {
			mutate: func(s string) string { return strings.Replace(s, "| `validateA` |", "| ``validateA` |", 1) },
			want:   "is not a backticked name",
		},
		"constraint cell is not backticked": {
			mutate: func(s string) string { return strings.Replace(s, "| `validateA` |", "| validateA |", 1) },
			want:   "is not a backticked name",
		},
		"constraint cell has a trailing backtick": {
			mutate: func(s string) string { return strings.Replace(s, "| `validateA` |", "| `validateA`` |", 1) },
			want:   "is not a backticked name",
		},
		"negative case doubles a backtick": {
			mutate: func(s string) string { return strings.Replace(s, "`kerml/a.kerml`", "``kerml/a.kerml`", 1) },
			want:   "neither `none` nor a backticked corpus path",
		},
		"implementation file is missing": {
			mutate: func(s string) string {
				return strings.Replace(s, "internal/core/passes/a.go:APass.Run", "internal/core/passes/gone.go:APass.Run", 1)
			},
			want: "implementation: internal/core/passes/gone.go does not exist",
		},
		"implementation method is missing": {
			mutate: func(s string) string {
				return strings.Replace(s, "a.go:APass.Run", "a.go:APass.Check", 1)
			},
			want: "internal/core/passes/a.go declares no APass.Check",
		},
		"implementation function is missing": {
			mutate: func(s string) string { return strings.Replace(s, "a.go:helper", "a.go:renamed", 1) },
			want:   "internal/core/passes/a.go declares no renamed",
		},
		"implementation cites no location": {
			mutate: func(s string) string {
				return strings.Replace(s, "internal/core/passes/a.go:APass.Run (and internal/core/passes/a.go:helper, internal/core/passes/a.go:Arena.Take).", "internal/core/passes/a.go (APass)", 1)
			},
			want: "validateA implementation \"internal/core/passes/a.go (APass)\" cites no internal/<file>.go:<function> location",
		},
		"implementation cites a file by name only": {
			mutate: func(s string) string {
				return strings.Replace(s, "internal/core/passes/a.go:helper,", "a.go:helper,", 1)
			},
			want: "validateA implementation a.go:helper is not a repository-relative internal/<file>.go location",
		},
		"implementation cites a missing symbol in a file by name only": {
			mutate: func(s string) string {
				return strings.Replace(s, "internal/core/passes/a.go:helper,", "a.go:renamed,", 1)
			},
			want: "validateA implementation a.go:renamed is not a repository-relative internal/<file>.go location",
		},
		"implementation continues past the method": {
			mutate: func(s string) string { return strings.Replace(s, "a.go:APass.Run ", "a.go:APass.Run.extra ", 1) },
			want:   "internal/core/passes/a.go:APass.Run.extra is not a <function> or <Type>.<method> location",
		},
		"implementation continues past the function with a dash": {
			mutate: func(s string) string { return strings.Replace(s, "a.go:helper,", "a.go:helper-extra,", 1) },
			want:   "internal/core/passes/a.go:helper-extra is not a <function> or <Type>.<method> location",
		},
		"implementation continues past the function with a slash": {
			mutate: func(s string) string { return strings.Replace(s, "a.go:helper,", "a.go:helper/extra,", 1) },
			want:   "internal/core/passes/a.go:helper/extra is not a <function> or <Type>.<method> location",
		},
		"implementation continues past the function with a colon": {
			mutate: func(s string) string { return strings.Replace(s, "a.go:helper,", "a.go:helper:extra,", 1) },
			want:   "internal/core/passes/a.go:helper:extra is not a <function> or <Type>.<method> location",
		},
		"generic method is missing": {
			mutate: func(s string) string { return strings.Replace(s, "a.go:Arena.Take", "a.go:Arena.Put", 1) },
			want:   "internal/core/passes/a.go declares no Arena.Put",
		},
		"negative case attributed to another constraint": {
			mutate: func(s string) string {
				s = strings.Replace(s, "`kerml/a.kerml`, `kerml/other.kerml`", "`kerml/a.kerml`, `kerml/b.kerml`, `kerml/other.kerml`", 1)
				return strings.Replace(s, "| — | — | `kerml/b.kerml` | ❌", "| — | — | `kerml/a.kerml` | ❌", 1)
			},
			want: "validateA negative case kerml/b.kerml is attributed to validateB, not to this constraint",
		},
		"negative case names the constraint in prose but its pilot token names another": {
			mutate: func(s string) string {
				s = strings.Replace(s, ", `kerml/prose.kerml`", "", 1)
				return strings.Replace(s, "| — | — | `kerml/b.kerml` | ❌", "| — | — | `kerml/b.kerml`, `kerml/prose.kerml` | ❌", 1)
			},
			want: "validateB negative case kerml/prose.kerml is attributed to validateA, not to this constraint",
		},
		"negative case cites the constraint in its specification citation only and another row lists it": {
			mutate: func(s string) string {
				s = strings.Replace(s, ", `kerml/spec.kerml`", "", 1)
				return strings.Replace(s, "| — | — | `kerml/b.kerml` | ❌", "| — | — | `kerml/b.kerml`, `kerml/spec.kerml` | ❌", 1)
			},
			want: "validateB negative case kerml/spec.kerml is attributed to validateA, not to this constraint",
		},
		"negative case names no constraint in its header": {
			mutate: func(s string) string {
				return strings.Replace(s, "`xpect/x.kerml` | ✅", "`xpect/x.kerml`, `xpect/bare.kerml` | ✅", 1)
			},
			want: "validateA negative case xpect/bare.kerml names no constraint in its header",
		},
		"faithful row lists a pilot-only case": {
			mutate: func(s string) string {
				s = strings.Replace(s, "`kerml/a.kerml`, `kerml/other.kerml`", "`kerml/a.kerml`, `kerml/b.kerml`, `kerml/other.kerml`", 1)
				return strings.Replace(s, "| — | — | `kerml/b.kerml` | ❌", "| — | — | `kerml/a.kerml` | ❌", 1)
			},
			want: "validateA is recorded faithful but docs/project/pilot-rejection-baseline.json records its negative case kerml/b.kerml as pilot-only-rejects",
		},
		"not-implemented row lists a case we reject": {
			mutate: func(s string) string {
				return strings.Replace(s, "| — | — | `kerml/b.kerml` | ❌", "| — | — | `kerml/b.kerml`, `xpect/x.kerml` | ❌", 1)
			},
			want: "validateB is recorded not-implemented but docs/project/pilot-rejection-baseline.json records its negative case xpect/x.kerml as ours-only-rejects",
		},
		"approximate row lists a case both validators accept": {
			mutate: func(s string) string {
				return strings.Replace(s, "`kerml/d.kerml` | ⚠️", "`kerml/d.kerml`, `kerml/d-open.kerml` | ⚠️", 1)
			},
			want: "validateD negative case kerml/d-open.kerml is recorded as both-accept in docs/project/pilot-rejection-baseline.json, which is not a rejection",
		},
		"approximate row lists a case with an unknown bucket": {
			mutate: func(s string) string {
				return strings.Replace(s, "`kerml/d.kerml` | ⚠️", "`kerml/d.kerml`, `kerml/d-odd.kerml` | ⚠️", 1)
			},
			want: "validateD negative case kerml/d-odd.kerml is recorded as rejected in docs/project/pilot-rejection-baseline.json, which is not a rejection",
		},
		"faithful row lists a case both validators accept": {
			mutate: func(s string) string {
				s = strings.Replace(s, "`kerml/d.kerml` | ⚠️", "none | ⚠️", 1)
				s = strings.Replace(s, "`xpect/x.kerml` | ✅", "`xpect/x.kerml`, `kerml/d-open.kerml` | ✅", 1)
				return s
			},
			want: "validateA negative case kerml/d-open.kerml is recorded as both-accept in docs/project/pilot-rejection-baseline.json, which is not a rejection",
		},
		"unknown row lists a case": {
			mutate: func(s string) string {
				return strings.Replace(s, "| — | — | none | ❔", "| — | — | `xpect/x.kerml` | ❔", 1)
			},
			want: "validateC is recorded unknown but lists negative case xpect/x.kerml",
		},
		"attributed case is listed nowhere": {
			mutate: func(s string) string { return strings.Replace(s, ", `kerml/other.kerml`", "", 1) },
			want:   "kerml/other.kerml attributes its rejection to validateA, whose row does not list it",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := checkDocument(root, tc.mutate(doc), base)
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
	base := &Baseline{Constraints: []Constraint{
		{Name: "validateA", Source: "kerml", Status: StatusFaithful},
		{Name: "validateB", Source: "both", Status: StatusApproximate},
		{Name: "validateC", Source: "sysml", Status: StatusNotImplemented},
	}}
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
	if err == nil || !strings.Contains(err.Error(), "validateC, which the baseline records as not-implemented") {
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
}

// TestBaselineMatchesNamesEverySide checks the jar comparison names a removed and
// an added constraint.
func TestBaselineMatchesNamesEverySide(t *testing.T) {
	base := &Baseline{Constraints: []Constraint{
		{Name: "validateA", Source: "kerml", Status: StatusUnknown},
		{Name: "validateB", Source: "sysml", Raw: "validateB_", Status: StatusUnknown},
	}}
	if err := base.matches([]Extracted{{Name: "validateA", Source: "kerml"}, {Name: "validateB", Raw: "validateB_", Source: "sysml"}}); err != nil {
		t.Fatal(err)
	}
	err := base.matches([]Extracted{{Name: "validateA", Source: "kerml"}, {Name: "validateC", Source: "sysml"}})
	if err == nil {
		t.Fatal("a differing list must be rejected")
	}
	for _, want := range []string{"validateB is in the baseline but not in the jar", "validateC is in the jar but not in the baseline"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}
