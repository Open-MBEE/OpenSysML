package diff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F7: the third column is optional. Without SysIDE the report must carry no
// trace of it, so the committed two-way baseline still reproduces byte for byte.
func TestRunWithoutSysideLeavesTheReportUnchanged(t *testing.T) {
	repo := t.TempDir()
	validator, kermlValidator := writeMixedRoot(t, repo, filepath.Join(repo, "sysml-args.txt"), filepath.Join(repo, "kerml-args.txt"))

	out := filepath.Join(repo, "out")
	if err := run(options{repo: repo, validator: validator, kermlValidator: kermlValidator, out: out}); err != nil {
		t.Fatal(err)
	}

	encoded := readFile(t, filepath.Join(out, "pilot-diff.json"))
	if strings.Contains(encoded, "syside") {
		t.Errorf("the JSON report mentions syside without it:\n%s", encoded)
	}
	if text := readFile(t, filepath.Join(out, "pilot-diff.txt")); strings.Contains(text, "syside") {
		t.Errorf("the text report mentions syside without it:\n%s", text)
	}
}

// F7: with SysIDE present the third column is additive — every two-way bucket
// and total stays exactly what the pilot comparison produced, because the
// adjudication rests on those.
func TestRunWithSysideOnlyAddsTheThirdColumn(t *testing.T) {
	repo := t.TempDir()
	validator, kermlValidator := writeMixedRoot(t, repo, filepath.Join(repo, "sysml-args.txt"), filepath.Join(repo, "kerml-args.txt"))
	syside := writeStubSyside(t, repo,
		// corroborates the pilot on the SysML file, and reports one finding of
		// its own on a file both other implementations are silent about
		"Model.sysml:3:1: error: [parsing-error] Expecting token of type ';' but found `x`",
		"Lib.kerml:9:2: warning: [validateImportExplicitVisibility] An Import must have explicit visibility.")

	twoWay := filepath.Join(repo, "two-way")
	if err := run(options{repo: repo, validator: validator, kermlValidator: kermlValidator, out: twoWay}); err != nil {
		t.Fatal(err)
	}
	threeWay := filepath.Join(repo, "three-way")
	if err := run(options{repo: repo, validator: validator, kermlValidator: kermlValidator, syside: syside, out: threeWay}); err != nil {
		t.Fatal(err)
	}

	before, after := decodeReport(t, twoWay), decodeReport(t, threeWay)
	if after.Syside == nil {
		t.Fatal("the report has no third column")
	}
	if after.Syside.Version != "9.9.9" || after.Syside.Library != "2024-12" {
		t.Errorf("syside provenance = %+v", after.Syside)
	}
	if !strings.Contains(after.Syside.Scope, "executes nothing") {
		t.Errorf("the scope does not say SysIDE executes nothing: %q", after.Syside.Scope)
	}
	if before.Totals != after.Totals {
		t.Errorf("two-way totals moved: %+v -> %+v", before.Totals, after.Totals)
	}
	for i, root := range before.Roots {
		if root.Totals != after.Roots[i].Totals {
			t.Errorf("%s totals moved: %+v -> %+v", root.Name, root.Totals, after.Roots[i].Totals)
		}
	}
	byPath := map[string]FileReport{}
	for _, file := range after.Roots[0].Files {
		byPath[file.Path] = file
	}
	for _, file := range before.Roots[0].Files {
		got := byPath[file.Path]
		if len(got.PilotOnly) != len(file.PilotOnly) || len(got.OpenSysMLOnly) != len(file.OpenSysMLOnly) {
			t.Errorf("%s buckets moved: %+v -> %+v", file.Path, file, got)
		}
	}

	sysml := byPath["Model.sysml"].Syside
	if sysml == nil || len(sysml.Entries) != 1 || sysml.Entries[0].Sides != sidesPilotSyside {
		t.Fatalf("Model.sysml third column = %+v", sysml)
	}
	if got := after.Syside.Totals.WithPilot; got != 1 {
		t.Errorf("withPilotAgainstOpenSysML = %d", got)
	}
	if got := after.Syside.Totals.SysideOnly; got != 1 {
		t.Errorf("sysideOnly = %d", got)
	}
}

// F7: a launcher named on the command line must fail loudly rather than
// silently dropping back to a two-way comparison.
func TestRunReportsTheMissingSysideLauncher(t *testing.T) {
	repo := t.TempDir()
	validator, kermlValidator := writeMixedRoot(t, repo, filepath.Join(repo, "sysml-args.txt"), filepath.Join(repo, "kerml-args.txt"))

	err := run(options{repo: repo, validator: validator, kermlValidator: kermlValidator,
		syside: filepath.Join(repo, "absent", "validate-syside"), out: filepath.Join(repo, "out")})
	if err == nil || !strings.Contains(err.Error(), "download-syside.sh") {
		t.Fatalf("run() error = %v", err)
	}
}

// F7: the report names the SysIDE release and standard library it was produced
// with, so an unpinned launcher is an error rather than an unlabelled column.
func TestSysideReleaseRequiresThePin(t *testing.T) {
	dir := t.TempDir()
	launcher := filepath.Join(dir, "validate-syside")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, _, err := sysideRelease(launcher); err == nil {
		t.Fatal("a missing pin file was accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "syside-pin.txt"), []byte("SYSIDE_VERSION=0.9.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sysideRelease(launcher); err == nil || !strings.Contains(err.Error(), "SYSIDE_SPEC") {
		t.Fatalf("sysideRelease() error = %v", err)
	}
}

// F7: the three-way partition is over multisets, so a tuple two
// implementations report twice and the third once splits across buckets rather
// than counting as agreement three times.
func TestCompareSysideFilePartitionsMultisets(t *testing.T) {
	at := func(line int, category Category, message string) diagnostic {
		return diagnostic{Line: line, Severity: "error", Category: category, Message: message}
	}
	ours := []diagnostic{
		at(1, CategorySyntax, "ours twice a"), at(1, CategorySyntax, "ours twice b"),
		at(2, CategoryUnresolved, "ours alone"),
		at(4, CategoryKindMismatch, "ours, pilot agrees"),
	}
	theirs := []diagnostic{
		at(1, CategorySyntax, "pilot once"),
		at(3, CategoryUnresolved, "pilot alone"),
		at(4, CategoryKindMismatch, "pilot, ours agrees"),
	}
	syside := []diagnostic{
		at(1, CategorySyntax, "syside once"),
		at(3, CategoryUnresolved, "syside with the pilot"),
		at(5, CategoryUnmapped, "syside alone"),
	}

	got := map[string]int{}
	file := compareSysideFile(ours, theirs, syside)
	for _, e := range file.Entries {
		got[e.Sides] += e.Count
	}
	want := map[string]int{
		sidesAll:               1, // line 1, one of each
		sidesOpenSysML:         2, // line 1's second, and line 2
		sidesPilotSyside:       1, // line 3
		sidesOpenSysMLAndPilot: 1, // line 4
		sidesSyside:            1, // line 5
	}
	for sides, count := range want {
		if got[sides] != count {
			t.Errorf("%s = %d, want %d (all: %v)", sides, got[sides], count, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("unexpected buckets: %v", got)
	}
	if file.Diagnostics != len(syside) {
		t.Errorf("sysideDiagnostics = %d", file.Diagnostics)
	}

	// Examples are the diagnostics their own entry accounts for: no message may
	// be shown under two buckets.
	seen := map[string]bool{}
	for _, e := range file.Entries {
		for _, example := range e.Examples {
			if seen[example] {
				t.Errorf("%q appears under two buckets", example)
			}
			seen[example] = true
		}
	}
}

// F7: under-mapping is the point. A SysIDE diagnostic is only mapped into a
// shared category when it is the same finding; anything else stays unmapped,
// because a wrong mapping manufactures agreement.
func TestCategorizeSysideUnderMaps(t *testing.T) {
	for message, want := range map[string]Category{
		"[parsing-error] Expecting token of type '}' but found `(`.":                                              CategorySyntax,
		"[lexing-error] unexpected character: ->":                                                                 CategorySyntax,
		"[linking-error] Could not resolve reference to Feature named 'cc'.":                                      CategoryUnresolved,
		"[validateSubsettingMultiplicityConformance] Subsetting feature should not have":                          CategoryMultiplicity,
		"[validateOperatorExpressionQuantity] Invalid quantity expression, expected a measurement reference unit": CategoryUnits,
		"[validateAttributeUsageTyping] An AttributeUsage must be typed by DataTypes only.":                       CategoryKindMismatch,
		"[validateImportExplicitVisibility] An Import must have explicit visibility.":                             CategoryUnmapped,
		"[validateNamespaceDistinguishability] Duplicate of another member named item1.":                          CategoryUnmapped,
		"a diagnostic with no code at all":                                                                        CategoryUnmapped,
	} {
		if got := categorizeSyside(message); got != want {
			t.Errorf("categorizeSyside(%q) = %q, want %q", message, got, want)
		}
	}
}

func decodeReport(t *testing.T, dir string) Report {
	t.Helper()
	var report Report
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "pilot-diff.json"))), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

// writeStubSyside lays out a pinned stub launcher reporting the given
// GNU-format diagnostics, and returns its path.
func writeStubSyside(t *testing.T, repo string, diagnostics ...string) string {
	t.Helper()
	dir := filepath.Join(repo, "syside")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pin := "SYSIDE_VERSION=9.9.9\nSYSIDE_SPEC=2024-12\n"
	if err := os.WriteFile(filepath.Join(dir, "syside-pin.txt"), []byte(pin), 0o644); err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(dir, "validate-syside")
	script := "#!/bin/sh\ncat >&2 <<'DIAGNOSTICS'\n" + strings.Join(diagnostics, "\n") + "\nDIAGNOSTICS\nexit 1\n"
	if err := os.WriteFile(launcher, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return launcher
}
