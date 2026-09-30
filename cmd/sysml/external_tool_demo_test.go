package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
)

// The external-tool demo registers a Python script as a tool, previews the
// call, records a run — tools included — and renders the record in a document.
func TestExternalToolDemo(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("the demo's tool is a Python 3 script")
	}
	binary := buildCLI(t)

	dir := t.TempDir()
	for _, name := range []string{"thermal.sysml", filepath.Join("tools", "thermal.py"), filepath.Join("tools", "thermal.json")} {
		data, err := os.ReadFile(filepath.Join("..", "..", "examples", "external-tool-demo", name))
		if err != nil {
			t.Fatal(err)
		}
		dst := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(analysis.ToolsEnv, filepath.Join(dir, "tools"))
	model := filepath.Join(dir, "thermal.sysml")

	dry := checkPaths(t, binary, "-tool-dry-run", "ThermalDemo::heating", model)
	wantReport(t, dry, 0, "dry run of tool 'ThermalSolver'", "protocol: argv+none/csv",
		`"thermal.py"`, `"--mass"`, `"12.5"`, "the process was not started")

	saved := filepath.Join(dir, "recorded.sysml")
	got := checkPaths(t, binary, "-record-run", "ThermalDemo::heating", "-convert", "sysml", "-o", saved, model)
	wantReport(t, got, 0, "tMax = 306.0", "recorded Records::heating_run1")
	data, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `tools = ("ThermalSolver 1.0 from `) {
		t.Errorf("the record names no tool:\n%s", data)
	}

	report := filepath.Join(dir, "report.md")
	got = checkPaths(t, binary, "-record-run", "ThermalDemo::heating",
		"-render-document", "Reporting::HeatingReport", "-o", report, model)
	wantReport(t, got, 0, "recorded Records::heating_run1")
	data, err = os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"heating\\_run1", "| 306 |", "satisfied"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("report is missing %q:\n%s", want, data)
		}
	}
}
