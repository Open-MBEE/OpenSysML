package metamodel

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleConstraintsExistInPilotUML(t *testing.T) {
	root := repositoryRoot(t)
	pilotDir := filepath.Join(root, "build", "pilot-metamodel")
	if _, err := os.Stat(pilotDir); errors.Is(err, os.ErrNotExist) {
		t.Skip("pilot metamodel is not present")
	} else if err != nil {
		t.Fatal(err)
	}
	uml, err := os.ReadFile(filepath.Join(pilotDir, "SysML.uml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range Rules() {
		if rule.Constraint == "" {
			continue
		}
		if !bytes.Contains(uml, []byte(`name="`+rule.Constraint+`"`)) {
			t.Errorf("constraint %q for %s::%s is absent from SysML.uml",
				rule.Constraint, rule.DefiningClass, rule.Property)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

func TestRuleBasisIsBrief(t *testing.T) {
	for _, rule := range Rules() {
		if rule.Basis != "" && strings.ContainsAny(rule.Basis, "\n\r") {
			t.Errorf("basis for %s::%s is not one line", rule.DefiningClass, rule.Property)
		}
	}
}
