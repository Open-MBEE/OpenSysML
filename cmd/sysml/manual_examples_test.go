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
	named := exec.Command(binary, source, "-run-query", "Cookbook::NamedParts")
	output, err := named.CombinedOutput()
	if err != nil {
		t.Fatalf("cookbook query Cookbook::NamedParts: %v\n%s", err, output)
	}
	for _, want := range []string{
		"returned 12 rows",
		"Row 1: Cookbook::telescope::primaryMirror",
		"Row 6: Cookbook::Traceability::gimbal",
	} {
		if !strings.Contains(string(output), want) {
			t.Errorf("cookbook query Cookbook::NamedParts output is missing %q:\n%s", want, output)
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

// TestManualTraceabilityLadder renders the four graded traceability examples end to end:
// Markdown matches the committed output, the tier's distinguishing rows are present, HTML keeps element cells.
func TestManualTraceabilityLadder(t *testing.T) {
	binary := buildCLI(t)
	examples := filepath.Join("..", "..", "docs", "manual", "examples")
	tiers := []struct {
		name     string
		document string
		markdown []string
		html     []string
		tables   int
	}{
		{
			name:     "trace-1-basic",
			document: "RoverBasic::BasicReport",
			markdown: []string{
				"| RV-1 | The rover shall travel at least 20 km on one charge. | RoverBasic::rover::battery | true |",
				"| RV-3 | The rover shall accept commands from the lander at 2 kbps. |  | false |",
				"Requirements no part satisfies:\n\n- RV-3\n",
			},
			html: []string{
				`data-element="RoverBasic::rover::battery"`,
				`data-column="satisfied" data-value-kind="boolean"`,
			},
			tables: 1,
		},
		{
			name:     "trace-2-hierarchy",
			document: "LanderHierarchy::HierarchyReport",
			markdown: []string{
				"| L-1 | mass | LanderHierarchy::lander | LanderHierarchy::weighLander |",
				"| L-2.1 | thrust | LanderHierarchy::lander::propulsion::engine | LanderHierarchy::hotFire |",
				"| L-4 | beacon | LanderHierarchy::lander::avionics::transponder |  |",
				"| verification | hotFire | LanderHierarchy::lander.propulsion.engine | violated |",
				"| satisfaction |  | LanderHierarchy::lander.avionics.transponder | undecided |",
			},
			html: []string{
				`data-element="LanderHierarchy::weighLander"`,
				`data-element="LanderHierarchy::lander::avionics::radar"`,
			},
			tables: 7,
		},
		{
			name:     "trace-3-derivation",
			document: "RangeDerivation::DerivationReport",
			markdown: []string{
				"| M-1 | range |  | RangeDerivation::specification::system::dailyRange, RangeDerivation::specification::system::energyBudget | 4 |  |  |",
				"| S-2 | energyBudget | RangeDerivation::specification::mission::range | RangeDerivation::specification::subsystem::batteryCapacity, RangeDerivation::specification::subsystem::driveEfficiency | 2 | RangeDerivation::EnergyBudgetAnalysis |  |",
				"| B-1 | batteryCapacity | RangeDerivation::specification::system::energyBudget |  | 0 |  | RangeDerivation::rover::battery |",
			},
			html: []string{
				`data-element="RangeDerivation::EnergyBudgetAnalysis"`,
				`data-column="descendants" data-value-kind="integer"`,
			},
			tables: 6,
		},
		{
			name:     "trace-4-program",
			document: "ProgramReport::ProgramTraceability",
			markdown: []string{
				"**team: Comms**",
				"**team: Thermal**",
				"| Power | PWR-3 | cellDegradation | high | ProgramRequirements::specification::subsystem::arrayOutput | 0 | PowerDesign::ArrayDegradationAnalysis | PowerDesign::powerSubsystem::array | 0 | true |",
				"| Program | ST-1 | science | critical |  | 7 |  |  | 0 | false |",
				"| Thermal | THM-1 | heaterPower | critical | ProgramRequirements::specification::system::survival | 0 | OrbiterVocabulary::HeaterBank |  | 0 | false |",
				"| verification | linkBudgetAnalysis | CommsDesign::commsSubsystem.transmitter | violated |",
			},
			html: []string{
				`data-element="CommsDesign::commsSubsystem::transmitter"`,
				`data-element="PowerDesign::ArrayDegradationAnalysis"`,
			},
			tables: 7,
		},
	}
	for _, tier := range tiers {
		t.Run(tier.name, func(t *testing.T) {
			source := filepath.Join(examples, tier.name+".sysml")
			committed, err := os.ReadFile(filepath.Join(examples, tier.name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), tier.name+".md")
			render := exec.Command(binary, source, "-render-document", tier.document, "-o", out)
			if output, err := render.CombinedOutput(); err != nil {
				t.Fatalf("render: %v\n%s", err, output)
			}
			written, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if string(written) != string(committed) {
				t.Errorf("rendered example differs from docs/manual/examples/%s.md:\n%s", tier.name, written)
			}
			for _, want := range tier.markdown {
				if !strings.Contains(string(written), want) {
					t.Errorf("Markdown is missing %q:\n%s", want, written)
				}
			}

			page := exec.Command(binary, source, "-render-document", tier.document, "-doc-form", "html")
			html, err := page.Output()
			if err != nil {
				t.Fatalf("render HTML: %v", err)
			}
			for _, want := range tier.html {
				if !strings.Contains(string(html), want) {
					t.Errorf("HTML is missing %q:\n%s", want, html)
				}
			}
			if got := strings.Count(string(html), "<table"); got != tier.tables {
				t.Errorf("HTML has %d tables, want %d", got, tier.tables)
			}
		})
	}
}

// TestManualTraceabilityLadderQueries runs the ladder's queries ad hoc and checks
// they return the rows the rendered reports show.
func TestManualTraceabilityLadderQueries(t *testing.T) {
	binary := buildCLI(t)
	examples := filepath.Join("..", "..", "docs", "manual", "examples")
	cases := []struct {
		source string
		query  string
		want   []string
	}{
		{
			source: "trace-1-basic.sysml",
			query:  "RoverBasic::Unsatisfied root=RoverBasic::specification",
			want:   []string{"returned 1 row", `shortName = "RV-3"`},
		},
		{
			source: "trace-2-hierarchy.sysml",
			query:  "LanderHierarchy::SatisfiedButUnverified root=LanderHierarchy::specification",
			want:   []string{"returned 1 row", `shortName = "L-4"`},
		},
		{
			source: "trace-3-derivation.sysml",
			query:  "RangeDerivation::AllDerived req=RangeDerivation::specification::mission::range",
			want: []string{
				"returned 4 rows",
				`shortName = "S-1"`, `shortName = "S-2"`, `shortName = "B-1"`, `shortName = "D-1"`,
			},
		},
		{
			source: "trace-3-derivation.sysml",
			query:  "RangeDerivation::Origins req=RangeDerivation::specification::subsystem::batteryCapacity",
			want:   []string{"returned 2 rows", `shortName = "S-2"`, `shortName = "M-1"`},
		},
		{
			source: "trace-4-program.sysml",
			query:  "ProgramReport::Matrix",
			want: []string{
				"returned 12 rows",
				"Columns: team, shortName, name, priority, derivedFrom, descendants, refinedBy, satisfiedBy, verifications, satisfied",
				`shortName = "ST-1"`, "descendants = 7",
				`shortName = "PWR-2"`, "verifications = 2",
				"refinedBy = OrbiterVocabulary::HeaterBank",
			},
		},
		{
			source: "trace-4-program.sysml",
			query:  "ProgramReport::CriticalUncovered",
			want:   []string{"returned 1 row", `shortName = "THM-1"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			run := exec.Command(binary, filepath.Join(examples, tc.source), "-run-query", tc.query)
			output, err := run.CombinedOutput()
			if err != nil {
				t.Fatalf("run query: %v\n%s", err, output)
			}
			for _, want := range tc.want {
				if !strings.Contains(string(output), want) {
					t.Errorf("query output is missing %q:\n%s", want, output)
				}
			}
		})
	}
}
