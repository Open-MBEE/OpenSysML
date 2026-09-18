package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// TestProbesReportTheirConstraint is the evidence behind every ✅/⚠️ row of the
// census: each probe is a minimal violating model, and we must report the
// diagnostic its header names at the severity it names.
func TestProbesReportTheirConstraint(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	probes, err := loadProbes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(probes) == 0 {
		t.Fatalf("no probes under %s", probesDir)
	}
	for _, p := range probes {
		t.Run(filepath.Base(p.Path), func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(root, p.Path))
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Base(p.Path)
			ws := model.NewWorkspace()
			ws.Open(name, content, 1)
			var seen []string
			for _, d := range ws.Diagnostics(name) {
				line := d.Severity.String() + ": " + d.Message
				if d.Severity.String() == p.Severity && strings.Contains(d.Message, p.Message) {
					return
				}
				seen = append(seen, line)
			}
			t.Errorf("%s: expected a %s containing %q for %s; got:\n  %s",
				p.Path, p.Severity, p.Message, p.Constraint, strings.Join(seen, "\n  "))
		})
	}
}
