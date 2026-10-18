package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Results is one model's benchmark run as committed under docs/project/.
type Results struct {
	Provider string `json:"provider"`
	// Model is the identifier asked for; ModelReported lists the distinct
	// identifiers the provider answered with (dated snapshots), when it did.
	Model         string `json:"model"`
	ModelReported string `json:"modelReported,omitempty"`
	Recorded      string `json:"recorded"`
	Settings      struct {
		MaxRounds   int      `json:"maxRounds"`
		Samples     int      `json:"samples"`
		Temperature float64  `json:"temperature"`
		MaxTokens   int      `json:"maxTokens"`
		Tasks       int      `json:"tasks"`
		Tools       []string `json:"mcpTools"`
	} `json:"settings"`
	Toolchain struct {
		SysmlVersion string `json:"sysmlVersion"`
		PilotRelease string `json:"pilotRelease,omitempty"`
	} `json:"toolchain"`
	Arms         map[Arm]Aggregate `json:"arms"`
	Significance []Paired          `json:"significance,omitempty"`
	Cells        []CellResult      `json:"cells"`
}

// Finish fills the aggregates from the cells.
func (r *Results) Finish() {
	r.ModelReported = reportedModels(r.Cells)
	r.Arms = map[Arm]Aggregate{}
	byArm := map[Arm][]CellResult{}
	for _, c := range r.Cells {
		byArm[c.Arm] = append(byArm[c.Arm], c)
	}
	for arm, cs := range byArm {
		r.Arms[arm] = AggregateCells(cs)
	}
	if len(byArm[ArmCompiler]) > 0 && len(byArm[ArmMCP]) > 0 {
		r.Significance = ComparePaired(byArm[ArmCompiler], byArm[ArmMCP])
	}
	sort.Slice(r.Cells, func(i, j int) bool {
		a, b := r.Cells[i], r.Cells[j]
		if a.Task != b.Task {
			return a.Task < b.Task
		}
		if a.Arm != b.Arm {
			return a.Arm < b.Arm
		}
		return a.Sample < b.Sample
	})
}

// Write saves the results as indented JSON.
func (r *Results) Write(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// ReadResults loads a results file.
func ReadResults(path string) (*Results, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the results file is the caller's
	if err != nil {
		return nil, err
	}
	var r Results
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &r, nil
}

func pct(s Stat) string {
	if s.N == 0 {
		return "—"
	}
	if s.N < 2 {
		return fmt.Sprintf("%.0f%%", 100*s.Mean)
	}
	return fmt.Sprintf("%.0f ± %.0f%%", 100*s.Mean, 100*s.Stdev)
}

func num(s Stat) string {
	if s.N == 0 {
		return "—"
	}
	if s.N < 2 {
		return fmt.Sprintf("%.2f", s.Mean)
	}
	return fmt.Sprintf("%.2f ± %.2f", s.Mean, s.Stdev)
}

// MarkdownTable renders the headline table over several runs, one row per
// model and arm.
func MarkdownTable(runs []*Results) string {
	var b strings.Builder
	b.WriteString("| Model | Arm | CR@0 | FCR | Repairs | Fidelity | Halluc. | Pilot agrees | Tool calls |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range runs {
		for _, arm := range Arms {
			a, ok := r.Arms[arm]
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "| %s/%s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
				r.Provider, r.Model, arm, pct(a.CompileRate0), pct(a.FinalCompile), num(a.RepairRounds),
				num(a.Fidelity), pct(a.Hallucination), pct(a.PilotAgreement), num(a.ToolCalls))
		}
	}
	return b.String()
}

// MarkdownSignificance renders the paired comparison of the arms.
func MarkdownSignificance(r *Results) string {
	if len(r.Significance) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "| Metric (%s/%s) | Pairs | Mean Δ (mcp − compiler) | t | p (t) | Wins/Losses | p (sign) |\n", r.Provider, r.Model)
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, p := range r.Significance {
		fmt.Fprintf(&b, "| %s | %d | %+.3f | %.2f | %.3f | %d/%d | %.3f |\n", p.Metric, p.Pairs, p.MeanDiff, p.T, p.PT, p.Wins, p.Losses, p.PSign)
	}
	return b.String()
}

// reportedModels lists the distinct model ids the provider returned, sorted.
func reportedModels(cells []CellResult) string {
	seen := map[string]bool{}
	var models []string
	for _, c := range cells {
		for _, a := range c.Attempts {
			if a.Model != "" && !seen[a.Model] {
				seen[a.Model] = true
				models = append(models, a.Model)
			}
		}
	}
	sort.Strings(models)
	return strings.Join(models, ", ")
}
