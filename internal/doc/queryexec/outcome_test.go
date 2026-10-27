package queryexec

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// The record behind Named's two outcomes, field for field with lean/QVTS/Outcome.lean.
func TestNamedOutcomesCarryTheQVTSRecord(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Subsystem;
part telescope : Subsystem;
package Duplicates { part a; part a; }
alias Broken for Nowhere;
calc def Misspelled :> Query { Named(qualifiedName = "Observatory::telescop") }
calc def Twice :> Query { Named(qualifiedName = "Observatory::Duplicates::a") }
calc def Dangling :> Query { Named(qualifiedName = "Observatory::Broken::x") }
package Kinds { part a; part def a; }
calc def TwoKinds :> Query { Named(qualifiedName = "Observatory::Kinds::missing") }
calc def RootMiss :> Query { Named(qualifiedName = "Observatry") }
calc def Partial :> Query { Named(qualifiedName = "Duplicates::a") }
calc def Spread :> Query { Named(qualifiedName = "a") }
`)
	outcome := func(query string) Outcome {
		t.Helper()
		_, err := fixture.execute(t, query, nil, Options{})
		var typed *Error
		if !errors.As(err, &typed) || typed.Outcome == nil {
			t.Fatalf("%s: error = %v, want one carrying an outcome", query, err)
		}
		return *typed.Outcome
	}

	misspelled := outcome("Misspelled")
	if misspelled.Kind() != "pathError" || len(misspelled.Path.Failures) != 1 {
		t.Fatalf("Misspelled: outcome = %+v, want one path failure", misspelled)
	}
	failure := misspelled.Path.Failures[0]
	if failure.Step.Label() != "Named(Observatory::telescop)" || failure.Expected != expectedNamed {
		t.Errorf("Misspelled: step %s expected %q", failure.Step.Label(), failure.Expected)
	}
	if failure.Frontier.At != "Observatory" || failure.Frontier.Nearest != "telescope" {
		t.Errorf("Misspelled: frontier at %q nearest %q, want Observatory / telescope", failure.Frontier.At, failure.Frontier.Nearest)
	}
	var members []string
	for _, m := range failure.Frontier.Members {
		members = append(members, m.Name+":"+m.Metaclass)
	}
	if got := strings.Join(members, ","); !strings.Contains(got, "telescope:PartUsage") || !strings.Contains(got, "Subsystem:PartDefinition") {
		t.Errorf("Misspelled: members = %s, want the namespace's members with their metaclasses", got)
	}
	encoded, err := json.Marshal(misspelled)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"outcome":"pathError"`, `"step":"Named(Observatory::telescop)"`, `"at":"Observatory"`, `["telescope","PartUsage"]`, `"nearest":"telescope"`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("Misspelled JSON lacks %s:\n%s", want, encoded)
		}
	}
	if lines := misspelled.Lines(); len(lines) != 2 || !strings.HasPrefix(lines[0], "  path error at Named(Observatory::telescop): expected exactly one element") || !strings.Contains(lines[1], "nearest: telescope") {
		t.Errorf("Misspelled lines = %q", lines)
	}

	twice := outcome("Twice")
	if twice.Kind() != "modelError" || len(twice.Model.Findings) != 1 {
		t.Fatalf("Twice: outcome = %+v, want one finding", twice)
	}
	finding := twice.Model.Findings[0]
	if finding.Constraint != ConstraintAmbiguousName || finding.Element != "Observatory::Duplicates" || finding.Observed != "Observatory::Duplicates::a (declared 2 times)" {
		t.Errorf("Twice: finding = %+v", finding)
	}
	if partial := outcome("Partial"); len(partial.Model.Findings) != 1 || partial.Model.Findings[0].Element != "Observatory::Duplicates" {
		t.Errorf("Partial: findings = %+v, want the owner of the resolved candidates, not a slice of the partial name", partial.Model.Findings)
	}
	spread := outcome("Spread")
	if len(spread.Model.Findings) != 2 || spread.Model.Findings[0].Element != "Observatory::Duplicates" || spread.Model.Findings[1].Element != "Observatory::Kinds" ||
		spread.Model.Findings[0].Observed != "Observatory::Duplicates::a (declared 2 times)" || spread.Model.Findings[1].Observed != "Observatory::Kinds::a (declared 2 times)" {
		t.Errorf("Spread: findings = %+v, want one per owning namespace", spread.Model.Findings)
	}
	encoded, _ = json.Marshal(twice)
	if !strings.Contains(string(encoded), `"outcome":"modelError"`) || !strings.Contains(string(encoded), `"pathFailures":[]`) {
		t.Errorf("Twice JSON = %s", encoded)
	}

	twoKinds := outcome("TwoKinds")
	var pairs []string
	for _, m := range twoKinds.Path.Failures[0].Frontier.Members {
		pairs = append(pairs, m.Name+":"+m.Metaclass)
	}
	if got := strings.Join(pairs, ","); got != "a:PartDefinition,a:PartUsage" {
		t.Errorf("TwoKinds: members = %s, want both declarations of a with their metaclasses", got)
	}

	root := outcome("RootMiss")
	if f := root.Path.Failures[0]; f.Frontier.At != "" || f.Frontier.Nearest != "Observatory" || len(f.Frontier.Members) == 0 {
		t.Errorf("RootMiss: frontier = %+v, want no At, the root namespaces as members, nearest Observatory", f.Frontier)
	}

	dangling := outcome("Dangling")
	if f := dangling.Path.Failures[0]; f.Frontier.At != "Observatory::Broken" || f.Expected != "an alias denoting an element" {
		t.Errorf("Dangling: failure = %+v", f)
	}
}

func TestOutcomeLinesIsEmptyForErrorsWithoutARecord(t *testing.T) {
	if lines := OutcomeLines(&Error{Kind: ErrorUnknownProperty}); lines != nil {
		t.Fatalf("lines = %q, want none", lines)
	}
	if lines := OutcomeLines(errors.New("plain")); lines != nil {
		t.Fatalf("lines = %q, want none", lines)
	}
}
