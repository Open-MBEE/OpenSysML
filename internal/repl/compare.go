package repl

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/simresults"
)

// CompareOptions say how migrated run configurations are run beside the
// results their tool stored.
type CompareOptions struct {
	// Runs replaces every configuration's numberOfRuns when positive.
	Runs int64
	// Seed is the seed the runs derive theirs from; nil leaves it to the session's, if any.
	Seed *uint64
	// Draws replaces every configuration's durationSimulationMode when set.
	Draws *runtime.DrawPolicy
	// Observe names the stored observables compared, each with the feature of the run that answers
	// it; without any, every stored observable is read from the target's feature of its own name.
	Observe []ObservablePair
	// Only names the configurations compared, by qualified or simple name; all when empty.
	Only []string
}

// ObservablePair is a stored observable and the feature of the run that answers
// it; an empty Feature is the target's feature of the observable's own name.
type ObservablePair struct {
	Stored  string
	Feature string
}

// CompareResults runs each configuration the sidecar indexes as its tool ran
// it — on its target, for its run count, under its draw policy — and reports
// the tool's distribution of every observable beside the runs', one verdict
// per configuration. Numbers are read as they are; nothing is scaled or tuned.
func (s *Session) CompareResults(results *simresults.Results, opts CompareOptions) []Verdict {
	defer s.enter()()
	if results == nil || len(results.Configurations) == 0 {
		return []Verdict{unresolvedVerdict("compare", "the results index no run configuration")}
	}
	var verdicts []Verdict
	matched := make([]bool, len(opts.Only))
	for i := range results.Configurations {
		cfg := &results.Configurations[i]
		if !opts.selects(cfg, matched) {
			continue
		}
		verdicts = append(verdicts, s.withTrace(s.compareVerdict(cfg, opts)))
	}
	for i, name := range opts.Only {
		if !matched[i] {
			verdicts = append(verdicts, unresolvedVerdict("compare "+name, fmt.Sprintf("no configuration is named %s", name)))
		}
	}
	return verdicts
}

// selects reports whether the configuration is among those asked for, marking
// in matched every name of Only that names it.
func (o CompareOptions) selects(cfg *simresults.ConfigurationResults, matched []bool) bool {
	if len(o.Only) == 0 {
		return true
	}
	selected := false
	for i, name := range o.Only {
		if name == cfg.ID || sameName(name, cfg.Name) {
			matched[i] = true
			selected = true
		}
	}
	return selected
}

// sameName reports whether name is qualified, the last segment of it, quoted or bare.
func sameName(name, qualified string) bool {
	if name == qualified {
		return true
	}
	last := qualified
	if i := strings.LastIndex(qualified, "::"); i >= 0 {
		last = qualified[i+2:]
	}
	return name == last || name == strings.Trim(last, "'") || strings.Trim(name, "'") == strings.Trim(last, "'")
}

// compareVerdict compares one configuration: a refusal names what the runs
// cannot be made without, else the table of both distributions.
func (s *Session) compareVerdict(cfg *simresults.ConfigurationResults, opts CompareOptions) Verdict {
	label := "compare " + cfg.Name
	if len(cfg.Snapshots) == 0 {
		return unresolvedVerdict(label, withNotes("the tool stored no result of the configuration to compare with", cfg.Notes))
	}
	if cfg.Behavior == "" {
		return unresolvedVerdict(label, withNotes("the configuration performs no migrated behavior", cfg.Notes))
	}
	count := cfg.Runs
	if opts.Runs > 0 {
		count = opts.Runs
	}
	if count <= 0 {
		return unresolvedVerdict(label, "the configuration states no numberOfRuns; name the runs to make, as -runs <number>")
	}
	policy := s.draws
	switch {
	case opts.Draws != nil:
		policy = *opts.Draws
	case cfg.Draws != "":
		parsed, err := runtime.ParseDrawPolicy(cfg.Draws)
		if err != nil {
			return unresolvedVerdict(label, fmt.Sprintf("the configuration's durationSimulationMode %q is no draw policy", cfg.Draws))
		}
		policy = parsed
	}
	inv, unresolved := s.resolveInvocation([]Behavior{{Name: cfg.Name}}, nil, nil)
	if inv == nil {
		v := unresolved[0]
		v.Subject = label
		return v
	}
	was := s.draws
	s.setDraws(policy)
	answered, table, err := s.runsTable(inv, count, opts.Seed, nil)
	s.setDraws(was)
	if err != nil {
		return standing(unresolvedVerdict(label, err.Error()), answered)
	}
	completed := 0
	var failures []string
	for _, row := range table.Rows {
		if row.Err != nil {
			failures = append(failures, row.Err.Error())
			continue
		}
		completed++
	}
	header := fmt.Sprintf("%s — %d stored run(s) in %s; %d run(s) by OpenSysML, draws %s", label,
		len(cfg.Snapshots), orNone(cfg.Location), completed, policy)
	if !table.Seedless {
		header += fmt.Sprintf(", seed %d", table.Seed)
	}
	lines := append(sweepTraces(table), header)
	lines = append(lines, comparisonTable(cfg, table, opts.Observe)...)
	for _, f := range dedupe(failures) {
		lines = append(lines, "error: "+f)
	}
	for _, n := range cfg.Notes {
		lines = append(lines, "note: "+n)
	}
	status := VerdictHolds
	if len(failures) > 0 {
		status = VerdictFails
	}
	return standing(Verdict{Subject: label, Status: status, Lines: lines}, answered)
}

// comparisonTable is one row per observable and side — the tool's stored
// numbers, the runs' values, and the relative difference of each statistic.
func comparisonTable(cfg *simresults.ConfigurationResults, table runtime.SweepTable, observe []ObservablePair) []string {
	cells := [][]string{{"observable", "source", "runs", "min", "mean", "p50", "p90", "max"}}
	var notes []string
	for _, pair := range comparedObservables(cfg, observe) {
		name, feature := pair.Stored, pair.Feature
		if !slices.Contains(cfg.Observables, name) {
			notes = append(notes, fmt.Sprintf("note: the tool stored no observable named %s, which %s was to answer", name, feature))
			continue
		}
		stored := runtime.Distribute(reals(cfg.Values(name)))
		if stored == nil {
			notes = append(notes, fmt.Sprintf("note: no snapshot holds a number for %s", name))
			continue
		}
		ran, units, held := runValues(table, feature)
		cells = append(cells, statisticsRow(name, "tool", stored, ""))
		d := runtime.Distribute(ran)
		switch {
		case !held:
			cells = append(cells, []string{"", "OpenSysML (" + feature + ")", "0", "", "", "", "", ""})
			notes = append(notes, fmt.Sprintf("note: no completed run produced %s, which answers %s", feature, name))
			continue
		case d == nil:
			cells = append(cells, []string{"", "OpenSysML (" + feature + ")", "0", "", "", "", "", ""})
			notes = append(notes, fmt.Sprintf("note: %s holds no number in any completed run, so %s is not compared", feature, name))
			continue
		case len(units) > 1:
			cells = append(cells, []string{"", "OpenSysML (" + feature + ")", fmt.Sprint(len(ran)), "", "", "", "", ""})
			notes = append(notes, fmt.Sprintf("note: %s came to numbers in more than one unit (%s) over the completed runs, so %s is not compared", feature, unitList(units), name))
			continue
		}
		cells = append(cells, statisticsRow("", "OpenSysML ("+feature+")", d, units[0]))
		cells = append(cells, differenceRow(stored, d))
	}
	widths := make([]int, len(cells[0]))
	for _, row := range cells {
		for i, cell := range row {
			widths[i] = max(widths[i], len([]rune(cell)))
		}
	}
	lines := []string{renderSweepRow(cells[0], widths, " | "), sweepRule(widths)}
	for _, row := range cells[1:] {
		lines = append(lines, renderSweepRow(row, widths, " | "))
	}
	return append(lines, notes...)
}

// comparedObservables are the pairs asked for, or every stored observable when
// none was; one naming no feature is read from the feature of its own name that
// the target object holds — `target.Time_Total` — or the bare name when the
// configuration runs on no target.
func comparedObservables(cfg *simresults.ConfigurationResults, observe []ObservablePair) []ObservablePair {
	pairs := make([]ObservablePair, 0, max(len(observe), len(cfg.Observables)))
	if len(observe) == 0 {
		for _, name := range cfg.Observables {
			pairs = append(pairs, ObservablePair{Stored: name})
		}
	} else {
		pairs = append(pairs, observe...)
	}
	for i := range pairs {
		if pairs[i].Feature != "" {
			continue
		}
		pairs[i].Feature = pairs[i].Stored
		if cfg.Target != "" {
			pairs[i].Feature = cfg.Target + "." + pairs[i].Stored
		}
	}
	return pairs
}

// runValues collects the numbers a feature came to in the completed runs and the
// distinct units they came in, in order of first appearance ("" for a bare number);
// held says whether any completed run produced the feature at all.
func runValues(table runtime.SweepTable, feature string) (numbers []semantics.Value, units []string, held bool) {
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		for _, out := range row.Outputs {
			if out.Name != feature {
				continue
			}
			held = true
			if n, ok := runtime.MagnitudeValue(out.Value); ok {
				numbers = append(numbers, n)
				unit := ""
				if q := out.Value.Quantity(); q != nil {
					unit = q.Unit.String()
				}
				if !slices.Contains(units, unit) {
					units = append(units, unit)
				}
			}
		}
	}
	return numbers, units, held
}

// unitList spells the units numbers came in, a bare number's as "none".
func unitList(units []string) string {
	names := make([]string, len(units))
	for i, u := range units {
		if u == "" {
			u = "none"
		}
		names[i] = u
	}
	return strings.Join(names, ", ")
}

// reals wraps stored numbers as the Reals the tool wrote them as.
func reals(values []float64) []semantics.Value {
	out := make([]semantics.Value, len(values))
	for i, v := range values {
		out[i] = semantics.Value{Kind: semantics.ValReal, Real: v}
	}
	return out
}

// statisticsRow spells one distribution: its count and five statistics.
func statisticsRow(observable, source string, d *runtime.Distribution, unit string) []string {
	return []string{observable, source, fmt.Sprint(d.Count),
		withUnit(d.Min, unit), withUnit(drawnMean(d), unit), withUnit(d.P50, unit), withUnit(d.P90, unit), withUnit(d.Max, unit)}
}

// differenceRow is the runs' statistics relative to the tool's, (ran − stored) / stored.
func differenceRow(stored, ran *runtime.Distribution) []string {
	return []string{"", "difference", "",
		relative(stored.Min, ran.Min), relative(drawnMean(stored), drawnMean(ran)),
		relative(stored.P50, ran.P50), relative(stored.P90, ran.P90), relative(stored.Max, ran.Max)}
}

// relative spells (ran − stored) / stored as a signed percentage; a zero
// reference has no relative difference, so the absolute one is given.
func relative(stored, ran semantics.Value) string {
	a, b := realOf(stored), realOf(ran)
	if a == 0 {
		if b == 0 {
			return "+0.0%"
		}
		return fmt.Sprintf("%+.4g (of 0)", b)
	}
	return fmt.Sprintf("%+.1f%%", (b-a)/math.Abs(a)*100)
}

func realOf(v semantics.Value) float64 {
	if v.Kind == semantics.ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// withNotes appends the sidecar's notes to a refusal, so it says why there is nothing to compare.
func withNotes(msg string, notes []string) string {
	if len(notes) == 0 {
		return msg
	}
	return msg + "; " + strings.Join(notes, "; ")
}

func orNone(s string) string {
	if s == "" {
		return "no result location"
	}
	return s
}

// dedupe sorts messages and drops repeats.
func dedupe(messages []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range messages {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}
