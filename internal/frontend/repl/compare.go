package repl

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
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
	// ClockStep replaces every configuration's clock step when set; 0 is a continuous clock.
	ClockStep *float64
	// Observe names the stored observables compared, each with the feature of the run that answers
	// it; without any, every stored observable is read from the target's feature of its own name.
	Observe []ObservablePair
	// Only names the configurations compared, by qualified or simple name; all when empty.
	Only []string
}

// The verdict-name prefix and the runs-column label the comparison repeats.
const (
	comparePrefix  = "compare "
	openSysMLLabel = "OpenSysML ("
)

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
	selected, refused := opts.selection(results.Configurations)
	for i := range results.Configurations {
		cfg := &results.Configurations[i]
		if len(opts.Only) > 0 && !selected[i] {
			continue
		}
		verdicts = append(verdicts, s.withTrace(s.compareVerdict(cfg, results.Repeats(i), opts)))
	}
	return append(verdicts, refused...)
}

// selection resolves each name of Only to the one configuration it names — by
// id or qualified name, or by simple name when one alone bears it — marking it
// in selected; a name naming none or several is a refusal instead.
func (o CompareOptions) selection(cfgs []simresults.ConfigurationResults) (selected []bool, refused []Verdict) {
	selected = make([]bool, len(cfgs))
	for _, name := range o.Only {
		var exact, simple []int
		for i := range cfgs {
			switch {
			case name == cfgs[i].ID || name == cfgs[i].Name:
				exact = append(exact, i)
			case sameName(name, cfgs[i].Name):
				simple = append(simple, i)
			}
		}
		found := exact
		if len(found) == 0 {
			found = simple
		}
		switch len(found) {
		case 1:
			selected[found[0]] = true
		case 0:
			refused = append(refused, unresolvedVerdict(comparePrefix+name, fmt.Sprintf("no configuration is named %s", name)))
		default:
			names := make([]string, len(found))
			for j, i := range found {
				names[j] = cfgs[i].Name
			}
			refused = append(refused, unresolvedVerdict(comparePrefix+name,
				fmt.Sprintf("%d configurations are named %s (%s); name one by its qualified name", len(found), name, strings.Join(names, ", "))))
		}
	}
	return selected, refused
}

// sameName reports whether name spells qualified or its last segment, quoted or
// bare; a `::` inside a quoted segment is part of that segment, not a separator.
func sameName(name, qualified string) bool {
	if name == qualified {
		return true
	}
	segments, ok := nameSegments(qualified)
	if !ok {
		return nameText(name) == nameText(qualified)
	}
	want := nameText(name)
	return want == strings.Join(segments, "::") || want == segments[len(segments)-1]
}

// compareVerdict compares one configuration: a refusal names what the runs
// cannot be made without, else the table of both distributions — the runs'
// alone, against a row saying so, when the tool stored no result. repeats
// notes the stored snapshots another configuration's repeat.
func (s *Session) compareVerdict(cfg *simresults.ConfigurationResults, repeats []string, opts CompareOptions) Verdict {
	label := comparePrefix + cfg.Name
	if cfg.Behavior == "" {
		return unresolvedVerdict(label, withNotes("the configuration "+cfg.Name+" performs no migrated behavior", cfg.Notes))
	}
	notes := repeats
	count := cfg.Runs
	if opts.Runs > 0 {
		count = opts.Runs
	}
	if count <= 0 {
		count = 1
		notes = append(notes, "the configuration states no numberOfRuns, so one run is made, as its tool makes without one; -runs <number> makes more")
	}
	policy := s.draws
	switch {
	case opts.Draws != nil:
		policy = *opts.Draws
	case cfg.Draws != "":
		parsed, err := runtime.ParseDrawPolicy(cfg.Draws)
		if err != nil {
			return unresolvedVerdict(label, fmt.Sprintf("the durationSimulationMode %q of the configuration %s is no draw policy", cfg.Draws, cfg.Name))
		}
		policy = parsed
	}
	step := cfg.ClockStep
	if opts.ClockStep != nil {
		step = *opts.ClockStep
	}
	if err := runtime.CheckClockStep(step); err != nil {
		return unresolvedVerdict(label, "the clock step of the configuration "+cfg.Name+" cannot be run on: "+err.Error())
	}
	inv, unresolved := s.resolveInvocation([]Behavior{{Name: cfg.Name}}, nil, nil)
	if inv == nil {
		v := unresolved[0]
		v.Subject = label
		return v
	}
	answered, table, err := s.runsTable(inv, count, opts.Seed, nil, policy, step)
	if err != nil {
		return standing(unresolvedVerdict(label, "the configuration "+cfg.Name+" could not be run: "+err.Error()), answered)
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
	header := fmt.Sprintf("%s — %s in %s; %d run(s) by OpenSysML, draws %s", label,
		storedRuns(cfg), orNone(cfg.Location), completed, policy)
	if !table.Seedless {
		header += fmt.Sprintf(", seed %d", table.Seed)
	}
	if step > 0 {
		header += ", clock step " + semantics.FormatReal(step) + " s"
	}
	lines := append(sweepTraces(table), header)
	lines = append(lines, comparisonTable(cfg, table, opts.Observe)...)
	for _, f := range dedupe(failures) {
		lines = append(lines, "error: "+f)
	}
	for _, n := range append(notes, cfg.Notes...) {
		lines = append(lines, "note: "+n)
	}
	status := VerdictHolds
	if len(failures) > 0 {
		status = VerdictFails
	}
	return standing(Verdict{Subject: label, Status: status, Lines: lines}, answered)
}

// storedRuns spells how many runs the tool stored of a configuration, and over
// how many snapshots when one summarises several.
func storedRuns(cfg *simresults.ConfigurationResults) string {
	runs := cfg.StoredRuns()
	if len(cfg.Snapshots) == 0 {
		return "no stored run"
	}
	if runs == int64(len(cfg.Snapshots)) {
		return fmt.Sprintf("%d stored run(s)", runs)
	}
	return fmt.Sprintf("%d stored run(s) over %d snapshot(s)", runs, len(cfg.Snapshots))
}

// comparisonTable is one row per observable and side — the tool's stored
// numbers, the runs' values, and the relative difference of each statistic. An
// observable the tool summarised is compared by count and mean, the statistics
// it kept; one the tool stored nothing of has the runs' row alone.
func comparisonTable(cfg *simresults.ConfigurationResults, table runtime.SweepTable, observe []ObservablePair) []string {
	cells := [][]string{{"observable", "source", "runs", "min", "mean", "p50", "p90", "max"}}
	var notes []string
	for _, pair := range comparedObservables(cfg, table, observe) {
		name, feature := pair.Stored, pair.Feature
		if len(cfg.Snapshots) == 0 {
			cells = append(cells, []string{name, "tool (no stored result to compare)", "0", "", "", "", "", ""})
			cells, notes = runRows(cells, notes, name, feature, table, nil)
			continue
		}
		if !slices.Contains(cfg.Observables, name) {
			notes = append(notes, fmt.Sprintf("note: the tool stored no observable named %s, which %s was to answer", name, feature))
			continue
		}
		stored, pooled := storedDistribution(cfg, name)
		if stored == nil {
			notes = append(notes, fmt.Sprintf("note: no snapshot holds a number for %s", name))
			continue
		}
		if pooled {
			cells = append(cells, []string{name, "tool", fmt.Sprint(stored.Count), "", withUnit(drawnMean(stored), ""), "", "", ""})
			for _, snap := range cfg.Summarised(name) {
				st := snap.Statistics
				note := fmt.Sprintf("note: %s summarises %d run(s) of %s: mean %s", orUnnamed(snap.Name), st.Runs, name, spell(st.Mean))
				if st.Deviation != nil {
					note += ", deviation " + spell(*st.Deviation)
				}
				if st.OutOfSpec != nil {
					note += fmt.Sprintf(", %d out of specification", *st.OutOfSpec)
				}
				notes = append(notes, note)
			}
			if apart := cfg.Disagreeing(name); len(apart) > 0 {
				names := make([]string, len(apart))
				for i, snap := range apart {
					names[i] = orUnnamed(snap.Name)
				}
				notes = append(notes, fmt.Sprintf("note: the summaries %s of %s lie more than three standard errors apart, so they cannot be of runs of one and the same model, and the tool's mean of %s blends them", strings.Join(names, ", "), name, name))
			}
		} else {
			cells = append(cells, statisticsRow(name, "tool", stored, ""))
		}
		tool := &comparison{stored: stored, pooled: pooled}
		if pooled && name == cfg.Analysis {
			tool.declared = declaredStatistics(cfg, name)
		}
		cells, notes = runRows(cells, notes, name, feature, table, tool)
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

// comparison is the tool's side of one observable: its distribution, pooled from
// summaries (count and mean alone) or over the runs it stored one by one, and the
// statistics the migrated analysis returns of it, when it declares any.
type comparison struct {
	stored   *runtime.Distribution
	pooled   bool
	declared *declared
}

// declared are the statistics an analysis def returns of the observable it
// analyses, with the tool's pooled value of each that its summaries fix.
type declared struct {
	analysis   string
	statistics []string
	deviation  *float64
	outOfSpec  *int64
}

// declaredStatistics are the returns of the analysis def written for cfg's target,
// nil when it declares none, with the tool's deviation and out-of-specification
// count pooled over the summaries of observable.
func declaredStatistics(cfg *simresults.ConfigurationResults, observable string) *declared {
	if len(cfg.Statistics) == 0 {
		return nil
	}
	d := &declared{analysis: cfg.AnalysisCase, statistics: cfg.Statistics}
	if dev, ok := storedDeviation(cfg, observable); ok {
		d.deviation = &dev
	}
	var out int64
	counted := false
	for _, s := range cfg.Summarised(observable) {
		if s.Statistics.OutOfSpec != nil {
			out += *s.Statistics.OutOfSpec
			counted = true
		}
	}
	if counted {
		d.outOfSpec = &out
	}
	return d
}

// storedDistribution is the tool's distribution of observable: over the numbers
// its snapshots hold run by run, or pooled — count and mean — with the runs the
// snapshots summarise, when any does; nil when the tool stored no number of it.
func storedDistribution(cfg *simresults.ConfigurationResults, observable string) (d *runtime.Distribution, pooled bool) {
	values := cfg.Values(observable)
	summarised := cfg.Summarised(observable)
	if len(summarised) == 0 {
		return runtime.Distribute(reals(values)), false
	}
	var runs int64
	var sum float64
	for _, v := range values {
		runs++
		sum += v
	}
	for _, s := range summarised {
		runs += s.Statistics.Runs
		sum += float64(s.Statistics.Runs) * s.Statistics.Mean
	}
	if runs == 0 {
		return nil, true
	}
	return &runtime.Distribution{Count: int(runs), Mean: sum / float64(runs)}, true
}

// storedDeviation is the tool's sample standard deviation of observable pooled over
// the runs its snapshots store one by one and the summaries, each contributing its
// runs' spread about its mean and its mean's offset from the pooled mean; false
// when a summary kept no deviation or fewer than two runs are stored.
func storedDeviation(cfg *simresults.ConfigurationResults, observable string) (float64, bool) {
	pooled, _ := storedDistribution(cfg, observable)
	if pooled == nil || pooled.Count < 2 {
		return 0, false
	}
	var sum float64
	for _, v := range cfg.Values(observable) {
		sum += (v - pooled.Mean) * (v - pooled.Mean)
	}
	for _, s := range cfg.Summarised(observable) {
		st := s.Statistics
		if st.Deviation == nil {
			return 0, false
		}
		n := float64(st.Runs)
		sum += (n-1)**st.Deviation**st.Deviation + n*(st.Mean-pooled.Mean)*(st.Mean-pooled.Mean)
	}
	return math.Sqrt(sum / float64(pooled.Count-1)), true
}

// runRows appends the runs' row of one observable — and the difference from the
// tool's when there is one — or the note saying why the runs are not compared.
func runRows(cells [][]string, notes []string, name, feature string, table runtime.SweepTable, tool *comparison) ([][]string, []string) {
	ran, units, other, missing := runValues(table, feature)
	d := runtime.Distribute(ran)
	switch {
	case len(ran) == 0 && other == 0:
		cells = append(cells, []string{"", openSysMLLabel + feature + ")", "0", "", "", "", "", ""})
		notes = append(notes, fmt.Sprintf("note: no completed run produced %s, which answers %s", feature, name))
		return cells, notes
	case missing > 0:
		cells = append(cells, []string{"", openSysMLLabel + feature + ")", fmt.Sprint(len(ran)), "", "", "", "", ""})
		note := fmt.Sprintf("note: %s was produced by %d of the %d completed run(s)", feature, len(ran)+other, len(ran)+other+missing)
		if other > 0 {
			note += fmt.Sprintf(" and holds no number in %d of those", other)
		}
		notes = append(notes, note+fmt.Sprintf(", so %s is not compared", name))
		return cells, notes
	case d == nil:
		cells = append(cells, []string{"", openSysMLLabel + feature + ")", "0", "", "", "", "", ""})
		notes = append(notes, fmt.Sprintf("note: %s holds no number in any completed run, so %s is not compared", feature, name))
		return cells, notes
	case other > 0:
		cells = append(cells, []string{"", openSysMLLabel + feature + ")", fmt.Sprint(len(ran)), "", "", "", "", ""})
		notes = append(notes, fmt.Sprintf("note: %s holds no number in %d of the %d completed run(s) that produced it, so %s is not compared", feature, other, other+len(ran), name))
		return cells, notes
	case len(units) > 1:
		cells = append(cells, []string{"", openSysMLLabel + feature + ")", fmt.Sprint(len(ran)), "", "", "", "", ""})
		notes = append(notes, fmt.Sprintf("note: %s came to numbers in more than one unit (%s) over the completed runs, so %s is not compared", feature, unitList(units), name))
		return cells, notes
	}
	cells = append(cells, statisticsRow("", openSysMLLabel+feature+")", d, units[0]))
	switch {
	case tool == nil:
	case tool.pooled:
		cells = append(cells, []string{"", "difference", "", "", relative(drawnMean(tool.stored), drawnMean(d)), "", "", ""})
		if tool.declared != nil {
			notes = append(notes, statisticsTable(name, feature, tool, d)...)
		} else {
			notes = append(notes, fmt.Sprintf("note: %s came to a deviation of %s over the %d completed run(s)", feature, spell(d.Deviation), d.Count))
		}
	default:
		cells = append(cells, differenceRow(tool.stored, d))
	}
	return cells, notes
}

// statisticsTable is one row per statistic the analysis returns of name: the
// tool's pooled value, the runs' by the same aggregation, and the relative
// difference of the two where both are numbers of one thing. The run counts are
// each side's own choice, so they are not differenced; out-of-specification runs
// are the tool's criterion, which no migrated check evaluates.
func statisticsTable(name, feature string, tool *comparison, d *runtime.Distribution) []string {
	cells := [][]string{{"statistic", "tool", openSysMLLabel + feature + ")", "difference"}}
	var notes []string
	for _, stat := range tool.declared.statistics {
		switch stat {
		case simresults.StatisticMean:
			cells = append(cells, []string{stat, spell(tool.stored.Mean), spell(d.Mean), relative(drawnMean(tool.stored), drawnMean(d))})
		case simresults.StatisticDeviation:
			if tool.declared.deviation == nil {
				cells = append(cells, []string{stat, "", spell(d.Deviation), ""})
				notes = append(notes, fmt.Sprintf("note: a summary of %s kept no deviation, so the tool's is not pooled and %s is not compared", name, stat))
				continue
			}
			cells = append(cells, []string{stat, spell(*tool.declared.deviation), spell(d.Deviation), relative(realValue(*tool.declared.deviation), realValue(d.Deviation))})
		case simresults.StatisticRuns:
			cells = append(cells, []string{stat, fmt.Sprint(tool.stored.Count), fmt.Sprint(d.Count), ""})
		case simresults.StatisticOutOfSpec:
			stored := ""
			if tool.declared.outOfSpec != nil {
				stored = fmt.Sprint(*tool.declared.outOfSpec)
			}
			cells = append(cells, []string{stat, stored, "", ""})
			notes = append(notes, fmt.Sprintf("note: %s counts the runs the tool found out of specification by its own criterion, which no migrated check evaluates, so it is not compared", stat))
		}
	}
	widths := make([]int, len(cells[0]))
	for _, row := range cells {
		for i, cell := range row {
			widths[i] = max(widths[i], len([]rune(cell)))
		}
	}
	lines := []string{fmt.Sprintf("statistics of %s returned by %s:", name, tool.declared.analysis), renderSweepRow(cells[0], widths, " | "), sweepRule(widths)}
	for _, row := range cells[1:] {
		lines = append(lines, renderSweepRow(row, widths, " | "))
	}
	return append(lines, notes...)
}

// spell writes a statistic as the tool's numbers are written in the table.
func spell(v float64) string {
	return withUnit(realValue(v), "")
}

// realValue wraps a statistic as the Real the tool wrote it as.
func realValue(v float64) semantics.Value {
	return semantics.Value{Kind: semantics.ValReal, Real: v}
}

func orUnnamed(name string) string {
	if name == "" {
		return "an unnamed snapshot"
	}
	return strconv.Quote(name)
}

// comparedObservables are the pairs asked for, or every stored observable when
// none was — the observable the configuration's analysis summarises, else every
// feature the completed runs produced a number for, when the tool stored none;
// one naming no feature is read from the feature of its own name that the
// target object holds — `target.Time_Total` — or the bare name when the
// configuration runs on no target.
func comparedObservables(cfg *simresults.ConfigurationResults, table runtime.SweepTable, observe []ObservablePair) []ObservablePair {
	pairs := make([]ObservablePair, 0, max(len(observe), len(cfg.Observables)))
	switch {
	case len(observe) > 0:
		pairs = append(pairs, observe...)
	case len(cfg.Snapshots) == 0 && cfg.Analysis != "":
		pairs = append(pairs, ObservablePair{Stored: cfg.Analysis})
	case len(cfg.Snapshots) == 0:
		for _, feature := range numericOutputs(table) {
			pairs = append(pairs, ObservablePair{Stored: strings.TrimPrefix(feature, cfg.Target+"."), Feature: feature})
		}
	default:
		for _, name := range cfg.Observables {
			pairs = append(pairs, ObservablePair{Stored: name})
		}
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

// numericOutputs names, sorted, every feature some completed run produced as a number.
func numericOutputs(table runtime.SweepTable) []string {
	seen := map[string]bool{}
	var names []string
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		for _, out := range row.Outputs {
			if _, ok := runtime.MagnitudeValue(out.Value); ok && !seen[out.Name] {
				seen[out.Name] = true
				names = append(names, out.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// runValues collects the numbers a feature came to in the completed runs, the
// distinct units they came in, in order of first appearance ("" for a bare number),
// how many completed runs produced the feature as no number, and how many did not produce it.
func runValues(table runtime.SweepTable, feature string) (numbers []semantics.Value, units []string, other, missing int) {
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		produced := false
		for _, out := range row.Outputs {
			if out.Name != feature {
				continue
			}
			produced = true
			n, ok := runtime.MagnitudeValue(out.Value)
			if !ok {
				other++
				continue
			}
			numbers = append(numbers, n)
			unit := ""
			if q := out.Value.Quantity(); q != nil {
				unit = q.Unit.String()
			}
			if !slices.Contains(units, unit) {
				units = append(units, unit)
			}
		}
		if !produced {
			missing++
		}
	}
	return numbers, units, other, missing
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
		out[i] = realValue(v)
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
