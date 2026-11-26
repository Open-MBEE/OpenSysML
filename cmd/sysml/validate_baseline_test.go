package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateBaselineNotAReport: a -baseline file that is not the JSON report
// -validate -json -o writes is refused in the report's own terms.
func TestValidateBaselineNotAReport(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	const refusal = "-baseline baseline.json: not a JSON report, as -validate -json -o writes: "
	for _, tc := range []struct {
		name, text, want string
	}{
		{"an array", `[]`, "it holds a JSON array, not an object"},
		{"a string", `"baseline"`, "it holds a JSON string, not an object"},
		{"null", `null`, `it has no "diagnostics"`},
		{"an object of another kind", `{}`, `it has no "diagnostics"`},
		{"a SARIF log", `{"version": "2.1.0", "runs": []}`, `it has no "diagnostics"`},
		{"diagnostics of another type", `{"diagnostics": "none"}`, `its "diagnostics" holds a JSON string`},
		{"a diagnostic of another type", `{"diagnostics": [7]}`, `its "diagnostics.0" holds a JSON number`},
		{"a diagnostic field of another type", `{"diagnostics": [{"code": 7}]}`, `its "diagnostics.0.code" holds a JSON number`},
		{"text that is not JSON", `known findings`, "invalid character 'k' looking for beginning of value"},
		{"an empty file", ``, "unexpected end of JSON input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "baseline.json"), []byte(tc.text), 0o644); err != nil {
				t.Fatal(err)
			}
			got := checkPathsIn(t, dir, binary, "-validate", "-baseline", "baseline.json", "models/clean.sysml")
			wantReport(t, got, exitUnevaluable, refusal+tc.want)
			for _, internal := range []string{"Go value", "Go struct", "main.diagnostic", "unmarshal"} {
				if strings.Contains(got.output(), internal) {
					t.Errorf("the refusal names %q, an internal of this program:\n%s", internal, got.output())
				}
			}
		})
	}
}

// TestValidateBaselineWithoutFindings: the report of a run that found nothing
// spells its diagnostics `null`, and is a baseline that knows no finding.
func TestValidateBaselineWithoutFindings(t *testing.T) {
	binary := buildCLI(t)
	dir := reportWorkspace(t)
	recorded := checkPathsIn(t, dir, binary, "-validate", "-json", "-o", "baseline.json", "models/clean.sysml")
	wantReport(t, recorded, exitHolds)
	if data, err := os.ReadFile(filepath.Join(dir, "baseline.json")); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(data), `"diagnostics": null`) {
		t.Fatalf("a clean run recorded its diagnostics as other than null:\n%s", data)
	}
	got := checkPathsIn(t, dir, binary, "-validate", "-baseline", "baseline.json", "models/clean.sysml")
	wantReport(t, got, exitHolds, "no errors")
}
