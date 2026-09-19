package xpect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeSuite lays out a suite directory the way the downloader does.
func writeSuite(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const clean = `//*
XPECT_SETUP a.B
	ResourceSet {
		ThisFile {}
		File {from ="/library/Base.kerml"}
		File {from ="/library/Occurrences.kerml"}
	}
END_SETUP
*/
// XPECT noErrors --> ""
package test {
	class A;
	// XPECT linkedName at A --> test.A
	// XPECT scope at A ---> A, B, test.A, test.B
	class B specializes A;
}
`

// baseLib stands in for the suite's own copy of a library file a fixture
// declares, so that a declared resource is present as it is in the download.
const baseLib = `standard library package Base {
	abstract classifier Anything {
		feature self: Anything;
	}
}
`

// occurrencesLib is the base a `class` specializes implicitly, which a clean
// file's resource set has to supply as the download does.
const occurrencesLib = `standard library package Occurrences {
	abstract class Occurrence;
}
`

const broken = `//*
XPECT_SETUP a.B
	ResourceSet {
		ThisFile {}
	}
END_SETUP
*/
// XPECT noErrors --> ""
package test {
	// XPECT errors --> "Couldn't resolve reference to Classifier 'Absent'." at "class B specializes Absent;"
	class B specializes Absent;
}
`

func TestW5CCompareCleanFile(t *testing.T) {
	dir := writeSuite(t, map[string]string{"a/clean.kerml.xt": clean,
		"library/Base.kerml": baseLib, "library/Occurrences.kerml": occurrencesLib})
	res := compareOne(dir, "a/clean.kerml.xt", newLibraryCache())
	if len(res.Problems) != 0 || len(res.Missing) != 0 {
		t.Fatalf("problems = %v, missing = %v", res.Problems, res.Missing)
	}
	byKind := map[string]row{}
	for _, r := range res.Rows {
		byKind[r.Kind] = r
	}
	if r := byKind[kindNoErrors]; r.Verdict != verdictAgree {
		t.Errorf("noErrors = %s (%s)", r.Verdict, r.Actual)
	}
	if r := byKind[kindLinkedName]; r.Verdict != verdictAgree || r.Actual != "test.A" {
		t.Errorf("linkedName = %s %q", r.Verdict, r.Actual)
	}
	if r := byKind[kindScope]; r.Verdict != verdictAgree {
		t.Errorf("scope = %s (%s)", r.Verdict, r.Actual)
	}
}

func TestW5CCompareDisagreements(t *testing.T) {
	dir := writeSuite(t, map[string]string{"a/broken.kerml.xt": broken})
	res := compareOne(dir, "a/broken.kerml.xt", newLibraryCache())
	if len(res.Problems) != 0 {
		t.Fatalf("problems = %v", res.Problems)
	}
	var noErrors, errors row
	for _, r := range res.Rows {
		switch r.Kind {
		case kindNoErrors:
			noErrors = r
		case kindErrors:
			errors = r
		}
	}
	// Xpect matches the error against the errors expectation's line, so it is
	// not residue and the file's noErrors assertion still holds.
	if noErrors.Verdict != verdictAgree {
		t.Errorf("noErrors = %s (%s)", noErrors.Verdict, noErrors.Actual)
	}
	// We do report an error on that line, but our span is the reference rather
	// than the whole member the pilot attaches it to, so the strict verdict is
	// a disagreement with a same-line tolerance.
	if errors.Verdict != verdictDisagree || errors.Tolerance != toleranceLine {
		t.Errorf("errors = %s (%s): %s", errors.Verdict, errors.Tolerance, errors.Actual)
	}
}

// undeclared holds an error no expectation in the file declares, which is the
// residue Xpect fails a file on.
const undeclared = `//*
XPECT_SETUP a.B
	ResourceSet {
		ThisFile {}
	}
END_SETUP
*/
// XPECT noErrors --> ""
package test {
	class B specializes Absent;
}
`

func TestW5CCompareUndeclaredErrorDisagrees(t *testing.T) {
	dir := writeSuite(t, map[string]string{"a/undeclared.kerml.xt": undeclared})
	res := compareOne(dir, "a/undeclared.kerml.xt", newLibraryCache())
	if len(res.Problems) != 0 {
		t.Fatalf("problems = %v", res.Problems)
	}
	for _, r := range res.Rows {
		if r.Kind == kindNoErrors && r.Verdict != verdictDisagree {
			t.Errorf("noErrors = %s (%s)", r.Verdict, r.Actual)
		}
	}
}

func TestW5CCompareLoadsDeclaredResources(t *testing.T) {
	const dep = "package dep { class A; }\n"
	const main = `//*
XPECT_SETUP a.B
	ResourceSet {
		ThisFile {}
		File "Dep.kerml" {}
		File {from ="/a/Absent.kerml"}
	}
END_SETUP
*/
// XPECT linkedName at dep::A --> dep.A
package test {
	class B specializes dep::A;
}
`
	dir := writeSuite(t, map[string]string{"a/main.kerml.xt": main, "a/Dep.kerml": dep})
	res := compareOne(dir, "a/main.kerml.xt", newLibraryCache())

	// The `File "Dep.kerml" {}` form resolves beside the .xt file, so the
	// cross-file reference resolves; the absent one is reported, not ignored.
	if len(res.Rows) != 1 || res.Rows[0].Verdict != verdictAgree {
		t.Errorf("rows = %+v", res.Rows)
	}
	if len(res.Missing) != 1 || res.Missing[0] != "/a/Absent.kerml" {
		t.Errorf("missing = %v", res.Missing)
	}
}

func TestW5CReportIsDeterministicAndOrdered(t *testing.T) {
	dir := writeSuite(t, map[string]string{"a/clean.kerml.xt": clean, "b/broken.kerml.xt": broken})
	files, err := collectXT(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0] != "a/clean.kerml.xt" {
		t.Fatalf("files = %v, want sorted", files)
	}

	render := func(jobs int) (string, string) {
		report := &Report{Pilot: "2026-05", Corpus: "testdata"}
		report.Suites = []SuiteReport{{Name: "kerml", Dir: "testdata", Files: compareAll(dir, files, jobs)}}
		report.summarize()
		encoded, err := json.MarshalIndent(report.pruned(), "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		return renderText(report), string(encoded)
	}
	text, encoded := render(1)
	otherText, otherJSON := render(4)
	if text != otherText {
		t.Error("text report depends on the number of workers")
	}
	if encoded != otherJSON {
		t.Error("json report depends on the number of workers")
	}

	report := &Report{}
	report.Suites = []SuiteReport{{Name: "kerml", Files: compareAll(dir, files, 2)}}
	report.summarize()
	if report.Totals.Files != 2 || report.Totals.Rows != 5 {
		t.Errorf("totals = %+v", report.Totals)
	}
	var kinds []string
	for _, kt := range report.Kinds {
		kinds = append(kinds, kt.Kind)
	}
	want := []string{kindErrors, kindNoErrors, kindLinkedName, kindScope}
	for i, k := range want {
		if i >= len(kinds) || kinds[i] != k {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
}
