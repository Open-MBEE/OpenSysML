package pssm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmitSuite is the gate over the pinned suite: every expressible test
// translates and its model parses, validates and lowers clean.
func TestEmitSuite(t *testing.T) {
	s := loadSuite(t)
	keep := os.Getenv("OPENSYSML_PSSM_KEEP")
	for _, tt := range s.Tests {
		if !Classify(tt).Class.Expressible() {
			continue
		}
		t.Run(strings.ReplaceAll(tt.Name, " ", "_"), func(t *testing.T) {
			m, err := Emit(s, tt)
			if err != nil {
				t.Errorf("emit: %v", err)
				return
			}
			if keep != "" {
				if err := os.WriteFile(filepath.Join(keep, m.Name), []byte(m.Text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if problems := Validate(m); len(problems) > 0 {
				t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
			}
		})
	}
}
