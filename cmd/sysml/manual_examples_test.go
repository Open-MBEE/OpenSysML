package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestManualWorkedExampleMatchesCommittedOutput renders the manual's worked
// example and compares it against the committed output, so the two never drift.
func TestManualWorkedExampleMatchesCommittedOutput(t *testing.T) {
	binary := buildCLI(t)
	examples := filepath.Join("..", "..", "docs", "manual", "examples")
	source := filepath.Join(examples, "observatory.sysml")
	committed, err := os.ReadFile(filepath.Join(examples, "observatory.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "observatory.md")
	cmd := exec.Command(binary, source, "-render-document", "Observatory::MassReport", "-o", out)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(committed) {
		t.Errorf("rendered manual example differs from docs/manual/examples/observatory.md:\n%s", written)
	}
}

// TestManualCookbookModelAnalysesCleanly parses and analyses the manual's
// query-cookbook model, so every recipe the manual quotes keeps compiling.
func TestManualCookbookModelAnalysesCleanly(t *testing.T) {
	binary := buildCLI(t)
	source := filepath.Join("..", "..", "docs", "manual", "examples", "cookbook.sysml")
	for _, query := range []string{
		"Cookbook::MassTable root=Cookbook::telescope",
		"Cookbook::MassBudget root=Cookbook::telescope",
		"Cookbook::HeldParts root=Cookbook::telescope",
	} {
		cmd := exec.Command(binary, source, "-run-query", query)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cookbook query %s: %v\n%s", query, err, output)
		}
	}
}

// TestManualCookbookObjectRecipes runs the cookbook's recipes over the objects
// the run holds, and checks they print the rows the manual quotes.
func TestManualCookbookObjectRecipes(t *testing.T) {
	binary := buildCLI(t)
	source := filepath.Join("..", "..", "docs", "manual", "examples", "cookbook.sysml")
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"Cookbook::HeldParts root=telescope", []string{
			"returned 3 rows",
			"Row 1: Cookbook::telescope.primaryMirror (#2)",
			`qualifiedName = "Cookbook::telescope.primaryMirror"`,
			"Row 2: Cookbook::telescope.instrumentCluster (#4)",
			"Row 3: Cookbook::telescope.mountControl (#7)",
			"mass = 15.0",
		}},
		{"Cookbook::HeldSubsystems", []string{
			"returned 3 rows",
			"Row 1: Cookbook::telescope.mountControl (#7)",
			"Row 2: Cookbook::telescope.primaryMirror (#2)",
			"Row 3: Cookbook::telescope.instrumentCluster (#4)",
		}},
	} {
		cmd := exec.Command(binary, source, "-instantiate", "Cookbook::telescope", "-run-query", tc.query)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cookbook query %s: %v\n%s", tc.query, err, output)
		}
		for _, want := range tc.want {
			if !strings.Contains(string(output), want) {
				t.Errorf("cookbook query %s output is missing %q:\n%s", tc.query, want, output)
			}
		}
	}
	cmd := exec.Command(binary, source, "-run-query", "Cookbook::HeldSubsystems")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cookbook query without objects: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "returned 0 rows") {
		t.Errorf("Objects over a run holding nothing should return no rows:\n%s", output)
	}
}

// TestManualRequirementsExample runs the manual's requirements example end to
// end: the query projects each requirement's short name, name and doc text,
// and the report renders them as table cells and as prose in Markdown and HTML,
// the Markdown matching its committed output.
func TestManualRequirementsExample(t *testing.T) {
	binary := buildCLI(t)
	examples := filepath.Join("..", "..", "docs", "manual", "examples")
	source := filepath.Join(examples, "requirements.sysml")

	query := exec.Command(binary, source, "-run-query", "Requirements::Reqs root=Requirements::spec")
	output, err := query.CombinedOutput()
	if err != nil {
		t.Fatalf("run query: %v\n%s", err, output)
	}
	for _, want := range []string{
		"returned 2 rows",
		"Columns: shortName, name, documentation",
		`shortName = "HLR-R001"`, `name = "CrewSafety"`,
		`documentation = "The mission shall safely return all three crew members to Earth."`,
		`shortName = "HLR-R002"`, `name = "SoftLanding"`,
		`documentation = "The mission shall achieve a soft landing\non the lunar surface."`,
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("query output is missing %q:\n%s", want, output)
		}
	}

	committed, err := os.ReadFile(filepath.Join(examples, "requirements.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "requirements.md")
	render := exec.Command(binary, source, "-render-document", "Requirements::RequirementsReport", "-o", out)
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(committed) {
		t.Errorf("rendered example differs from docs/manual/examples/requirements.md:\n%s", written)
	}
	for _, want := range []string{
		"| HLR-R001 | CrewSafety | The mission shall safely return all three crew members to Earth. |",
		"**HLR-R001** — The mission shall safely return all three crew members to Earth.",
		"**HLR-R002** — The mission shall achieve a soft landing on the lunar surface.",
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("Markdown is missing %q:\n%s", want, written)
		}
	}

	page := exec.Command(binary, source, "-render-document", "Requirements::RequirementsReport", "-doc-form", "html")
	html, err := page.Output()
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	for _, want := range []string{
		`<td class="sysml-cell" data-column="shortName" data-value-kind="string"><span class="sysml-value" data-value-kind="string">HLR-R001</span></td>`,
		`<dl class="sysml-definitions" data-content="definitions" data-name="prose" data-query="Requirements::Reqs">`,
		`<div class="sysml-entry" data-element="Requirements::CrewSafety" data-element-kind="requirementDef">` + "\n" +
			`<dt class="sysml-term">HLR-R001</dt>` + "\n" +
			`<dd class="sysml-description">The mission shall safely return all three crew members to Earth.</dd>`,
		`<dt class="sysml-term">HLR-R002</dt>` + "\n" +
			`<dd class="sysml-description">The mission shall achieve a soft landing on the lunar surface.</dd>`,
	} {
		if !strings.Contains(string(html), want) {
			t.Errorf("HTML is missing %q:\n%s", want, html)
		}
	}
}

// TestManualTraceabilityExample runs the manual's traceability example end to
// end: one matrix query lists every requirement with its satisfiers, verifiers
// and verification count, and the report's Markdown matches its committed output.
func TestManualTraceabilityExample(t *testing.T) {
	binary := buildCLI(t)
	examples := filepath.Join("..", "..", "docs", "manual", "examples")
	source := filepath.Join(examples, "traceability.sysml")

	query := exec.Command(binary, source, "-run-query", "Traceability::TraceMatrix root=Traceability::specification")
	output, err := query.CombinedOutput()
	if err != nil {
		t.Fatalf("run query: %v\n%s", err, output)
	}
	for _, want := range []string{
		"returned 4 rows",
		"Columns: shortName, name, satisfiedBy, verifiedBy, verifications",
		`shortName = "SC-2"`,
		"satisfiedBy = Traceability::spacecraft::antenna",
		"verifiedBy = [Traceability::gainTest, Traceability::gainAnalysis]",
		"verifications = 2",
		`shortName = "SC-4"`,
		"satisfiedBy = (none)",
		"verifiedBy = (none)",
		"verifications = 0",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("query output is missing %q:\n%s", want, output)
		}
	}

	committed, err := os.ReadFile(filepath.Join(examples, "traceability.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "traceability.md")
	render := exec.Command(binary, source, "-render-document", "Traceability::TraceabilityReport", "-o", out)
	if output, err := render.CombinedOutput(); err != nil {
		t.Fatalf("render: %v\n%s", err, output)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != string(committed) {
		t.Errorf("rendered example differs from docs/manual/examples/traceability.md:\n%s", written)
	}
	for _, want := range []string{
		"| SC-2 | downlinkGain | Traceability::spacecraft::antenna | Traceability::gainTest, Traceability::gainAnalysis | 2 |",
		"| SC-4 | passivation |  |  | 0 |",
		"| satisfaction |  | Traceability::spacecraft.radiator | violated | radiator.area >= 2.0 |",
	} {
		if !strings.Contains(string(written), want) {
			t.Errorf("Markdown is missing %q:\n%s", want, written)
		}
	}

	page := exec.Command(binary, source, "-render-document", "Traceability::TraceabilityReport", "-doc-form", "html")
	html, err := page.Output()
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	for _, want := range []string{
		`data-element="Traceability::gainTest"`,
		`data-element="Traceability::gainAnalysis"`,
		`<td class="sysml-cell" data-column="verifications" data-value-kind="integer"><span class="sysml-value" data-value-kind="integer">2</span></td>`,
	} {
		if !strings.Contains(string(html), want) {
			t.Errorf("HTML is missing %q:\n%s", want, html)
		}
	}
}
