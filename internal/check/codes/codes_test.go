package codes

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

var kebab = regexp.MustCompile(`^[a-z][a-z0-9]*([.-][a-z0-9]+)*$`)

func TestEntriesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range All() {
		if !kebab.MatchString(e.Code) {
			t.Errorf("%q is not a kebab-case code", e.Code)
		}
		if seen[e.Code] {
			t.Errorf("%q is registered twice", e.Code)
		}
		seen[e.Code] = true
		if e.Severity != "error" && e.Severity != "warning" {
			t.Errorf("%s: severity %q is neither error nor warning", e.Code, e.Severity)
		}
		if e.Tier.Title() == "" {
			t.Errorf("%s: tier %q has no title", e.Code, e.Tier)
		}
		if e.Summary == "" || strings.HasSuffix(e.Summary, ".") || strings.Contains(e.Summary, "\n") {
			t.Errorf("%s: summary must be one line without a final period: %q", e.Code, e.Summary)
		}
		if len(e.Meaning) < 40 {
			t.Errorf("%s: meaning is too short to explain the rule: %q", e.Code, e.Meaning)
		}
		for _, k := range e.Kinds {
			if !kebab.MatchString(k) {
				t.Errorf("%s: kind %q is not kebab-case", e.Code, k)
			}
		}
		for k, severity := range e.KindSeverities {
			if !slices.Contains(e.Kinds, k) {
				t.Errorf("%s: severity for %q, which is no kind of the family", e.Code, k)
			}
			if severity == e.Severity || (severity != "error" && severity != "warning") {
				t.Errorf("%s-%s: severity %q is not the other of error and warning", e.Code, k, severity)
			}
		}
	}
}

func TestFamilyMembersCarryTheirOwnSeverity(t *testing.T) {
	const missing, failed = "document-plan-markup-tool-missing", "document-plan-markup-tool-failed"
	severities := make(map[string]string)
	for _, row := range Rows() {
		severities[row.Code] = row.Severity
	}
	if severities[missing] != "warning" || severities[failed] != "error" || severities["document-plan"] != "error" {
		t.Errorf("Rows severities: %s=%q %s=%q document-plan=%q, want warning, error, error", missing, severities[missing], failed, severities[failed], severities["document-plan"])
	}
	for code, want := range map[string]string{missing: missing + ": warning (document)", failed: failed + ": error (document)"} {
		text, err := Explain(code)
		if err != nil || !strings.HasPrefix(text, want) {
			t.Errorf("Explain(%s) = %q, %v, want a text opening %q", code, text, err, want)
		}
	}
}

func TestLookupResolvesFamilyMembers(t *testing.T) {
	e, kind, ok := Lookup("document-plan-missing-title")
	if !ok || e.Code != "document-plan" || kind != "missing-title" {
		t.Fatalf("Lookup(document-plan-missing-title) = %q, %q, %v", e.Code, kind, ok)
	}
	if _, _, ok := Lookup("document-plan-no-such-rule"); ok {
		t.Error("an unknown family member resolved")
	}
	if _, _, ok := Lookup("no-such-code"); ok {
		t.Error("an unknown code resolved")
	}
	if e, kind, ok := Lookup("unresolved"); !ok || kind != "" || e.Tier != TierNameRes {
		t.Errorf("Lookup(unresolved) = %q, %q, %v", e.Code, kind, ok)
	}
}

func TestExplainAndList(t *testing.T) {
	text, err := Explain("unresolved")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"unresolved: error (name-resolution)", "Specification:"} {
		if !strings.Contains(text, want) {
			t.Errorf("Explain(unresolved) lacks %q:\n%s", want, text)
		}
	}
	if _, err := Explain("no-such-code"); err == nil {
		t.Error("Explain accepted an unknown code")
	}
	list := List()
	for _, e := range All() {
		if !strings.Contains(list, e.Code) {
			t.Errorf("List() lacks %s", e.Code)
		}
	}
	for _, c := range Codes() {
		if !Known(c) {
			t.Errorf("Codes() lists %s, which Lookup does not know", c)
		}
	}
}

// Every code a diagnostic can carry is a row of the listing, families expanded.
func TestListNamesEveryEmittedCode(t *testing.T) {
	listing := List()
	rows := map[string]bool{}
	for _, r := range Rows() {
		rows[r.Code] = true
	}
	var all []string
	for _, e := range All() {
		all = append(all, e.Code)
	}
	for _, code := range append(all, Codes()...) {
		if !rows[code] {
			t.Errorf("Rows lacks %s", code)
		}
		if !strings.Contains(listing, "  "+code+" ") {
			t.Errorf("List lacks %s", code)
		}
	}
}
