package pilot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDiagnostic(t *testing.T) {
	tests := []struct {
		name, line string
		want       Diagnostic
		ok         bool
	}{
		{
			name: "SysML path with spaces",
			line: "Part Definition Example.sysml:12:3: warning: Bound features should have conforming types",
			want: Diagnostic{Path: "Part Definition Example.sysml", Line: 12, Column: 3, Severity: "warning", Message: "Bound features should have conforming types"},
			ok:   true,
		},
		{
			name: "KerML nested path",
			line: "Nested Folder/Example.kerml:7:4: error: unresolved reference",
			want: Diagnostic{Path: "Nested Folder/Example.kerml", Line: 7, Column: 4, Severity: "error", Message: "unresolved reference"},
			ok:   true,
		},
		{
			name: "message holding a colon",
			line: "a.sysml:1:1: info: note: expected one of: x, y",
			want: Diagnostic{Path: "a.sysml", Line: 1, Column: 1, Severity: "info", Message: "note: expected one of: x, y"},
			ok:   true,
		},
		{name: "ignore severity", line: "a.kerml:3:9: ignore: ", want: Diagnostic{Path: "a.kerml", Line: 3, Column: 9, Severity: "ignore"}, ok: true},
		{name: "unknown severity", line: "a.sysml:1:1: fatal: boom"},
		{name: "other extension", line: "a.txt:1:1: error: boom"},
		{name: "no column", line: "a.sysml:1: error: boom"},
		{name: "line beyond an int", line: "a.sysml:99999999999999999999:1: error: boom"},
		{name: "logging notice", line: "log4j:WARN No appenders could be found"},
		{name: "blank"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ParseDiagnostic(test.line)
			if ok != test.ok || got != test.want {
				t.Errorf("ParseDiagnostic(%q) = %+v, %v; want %+v, %v", test.line, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestNoise(t *testing.T) {
	for line, want := range map[string]bool{
		"":                             true,
		"  \t":                         true,
		"log4j:WARN No appenders":      true,
		"Exception in thread \"main\"": false,
		"a.sysml:1:1: fatal: boom":     false,
	} {
		if got := Noise(line); got != want {
			t.Errorf("Noise(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestVersion(t *testing.T) {
	dir := t.TempDir()
	validator := filepath.Join(dir, "validator")
	if _, err := Version(validator); err == nil {
		t.Error("Version without a pin succeeded")
	}
	pin := filepath.Join(dir, "pilot-pin.txt")
	if err := os.WriteFile(pin, []byte("sysml.release.tag = 2025-02\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Version(validator); err == nil {
		t.Error("Version with half a pin succeeded")
	}
	if err := os.WriteFile(pin, []byte("# pin\nsysml.release.tag = 2025-02\nsysml.artifact.version=0.48.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Version(validator)
	if err != nil || got != "2025-02 (jupyter-sysml-kernel 0.48.0)" {
		t.Errorf("Version() = %q, %v", got, err)
	}
}
