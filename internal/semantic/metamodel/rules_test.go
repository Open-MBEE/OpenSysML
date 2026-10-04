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

func TestUsageOwningSubsetRulesUseOwningType(t *testing.T) {
	want := map[string]string{
		"Usage::owningDefinition": "owningType when it is a Definition",
		"Usage::owningUsage":      "owningType when it is a Usage",
	}
	for _, rule := range Rules() {
		key := rule.DefiningClass + "::" + rule.Property
		basis, ok := want[key]
		if !ok {
			continue
		}
		if rule.Basis != basis || rule.Constraint != "" {
			t.Errorf("%s rule = %#v; want basis %q", key, rule, basis)
		}
		delete(want, key)
	}
	for key := range want {
		t.Errorf("missing Usage owning-type rule for %s", key)
	}
}

func TestMembershipElementIDRulesAreWriterLevel(t *testing.T) {
	want := map[string]string{
		"Membership::memberElementId":            "writer emits elementId of memberElement",
		"OwningMembership::ownedMemberElementId": "elementId of ownedMemberElement",
	}
	for _, rule := range Rules() {
		key := rule.DefiningClass + "::" + rule.Property
		basis, ok := want[key]
		if !ok {
			continue
		}
		if rule.Constraint != "" || rule.Basis != basis {
			t.Errorf("%s rule = %#v, want writer-level basis %q", key, rule, basis)
		}
		delete(want, key)
	}
	for key := range want {
		t.Errorf("missing writer-level rule for %s", key)
	}
}

func TestOmittedPropertiesNameRequiredEngineFacts(t *testing.T) {
	for _, omission := range Omitted() {
		key := omission.DefiningClass + "::" + omission.Property
		if !strings.Contains(omission.Reason, "Requires the "+key+" engine fact:") {
			t.Errorf("omission for %s does not name its required engine fact: %q", key, omission.Reason)
		}
		if strings.Contains(strings.ToLower(omission.Reason), "not implemented") {
			t.Errorf("omission for %s uses a generic implementation reason: %q", key, omission.Reason)
		}
	}
}
