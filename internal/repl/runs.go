package repl

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

const runsUsage = "usage: %runs <n> <seed> <action> [<observable>...]"

// ClockObservable names the observable every run of a Monte Carlo reports
// beside the action's features: the simulation clock when the action completed.
const ClockObservable = "clock"

// ErrRunsReplay is the refusal of a Monte Carlo while the schedule replays a
// witness: the witness fixes every draw, so the runs could not differ.
var ErrRunsReplay = errors.New("a Monte Carlo runs under a driving schedule, not a replay")

// doRuns carries out %runs at the prompt: the number of runs and the seed their
// seeds derive from, the action, then the observables to report.
func (s *Session) doRuns(tail string) ([]string, bool, error) {
	count, seed, rest, err := splitRunsTail(tail)
	if err != nil {
		return []string{errPrefix + err.Error(), runsUsage}, false, nil
	}
	fields := splitQueryArgs(rest)
	if len(fields) == 0 {
		return []string{runsUsage}, false, nil
	}
	observables := make([]string, 0, len(fields)-1)
	for _, name := range fields[1:] {
		observables = append(observables, unquoteSpecName(name))
	}
	return s.withTrace(s.runsVerdict(Behavior{Name: fields[0]}, count, seed, observables)).Lines, false, nil
}

// splitRunsTail reads the number of runs and the seed off the front of a %runs tail.
func splitRunsTail(tail string) (int64, uint64, string, error) {
	fields := strings.Fields(strings.TrimSpace(tail))
	if len(fields) < 3 {
		return 0, 0, "", errors.New("name the number of runs, the seed, then the action")
	}
	count, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || count <= 0 {
		return 0, 0, "", fmt.Errorf("%w: %q is not a number of runs to make", runtime.ErrSweepRuns, fields[0])
	}
	seed, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("%w: %q is not a seed", runtime.ErrSweepRuns, fields[1])
	}
	rest := strings.TrimSpace(tail)
	for range 2 {
		rest = strings.TrimSpace(rest[strings.IndexFunc(rest, unicode.IsSpace):])
	}
	return count, seed, rest, nil
}

// RunRuns runs the action count times, each run drawing its modeled randomness
// from a seed of its own derived from seed, on an object of performer when one
// is named, and reports the table of the observables named — every feature the
// action holds and the clock when none are — with each one's distribution.
func (s *Session) RunRuns(name string, performer []string, count int64, seed uint64, observables []string) Verdict {
	defer s.enter()()
	return s.withTrace(s.runsVerdict(Behavior{Name: name, Performer: performer}, count, seed, observables))
}

// runsVerdict makes the runs and reports them as a sweep reports its table, the
// distributions after it. An observable no completed run produced is an error.
func (s *Session) runsVerdict(action Behavior, count int64, seed uint64, observables []string) Verdict {
	label := "runs " + action.Name
	if _, replaying := s.drivenSchedule().Replay(); replaying {
		return unresolvedVerdict(label, ErrRunsReplay.Error())
	}
	if dup := duplicateName(observables); dup != "" {
		return unresolvedVerdict(label, fmt.Sprintf("%s: observable %s is named twice", runtime.ErrSweepRuns, dup))
	}
	inv, unresolved := s.resolveInvocation([]Behavior{action}, nil, nil)
	if inv == nil {
		return unresolved[0]
	}
	plan := runtime.MonteCarloPlan(count, seed)
	run := func(rt *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		i, ok := runtime.RunNumber(bindings)
		if !ok {
			return runtime.SweepRunResult{}, fmt.Errorf("%w: the row numbers no run", runtime.ErrSweepRuns)
		}
		rt.SetModelSeed(runtime.RunSeed(seed, i))
		outcome, err := inv.run(rt)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		return runtime.SweepRunResult{Outputs: observe(rt, outcome, observables)}, nil
	}
	model := s.freshModel()
	s.state.Unlock()
	answered, err := s.sweep(inv.subject(), model, plan, run)
	s.state.Lock()
	if err != nil {
		return standing(unresolvedVerdict(label, err.Error()), &answered)
	}
	table := answered.Result.Table()
	if err := unobserved(table, observables); err != nil {
		return standing(unresolvedVerdict(label, err.Error()), &answered)
	}
	status, rows := sweepStatus(table)
	lines := append(sweepTraces(table), sweepTableLines(table)...)
	lines = append(lines, distributionLines(table)...)
	return standing(Verdict{
		Subject: label,
		Status:  status,
		Lines:   lines,
		Values:  sweepValues(table, rows),
		Rows:    rows,
	}, &answered)
}

// duplicateName is the first name listed twice, "" for none.
func duplicateName(names []string) string {
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			return name
		}
		seen[name] = true
	}
	return ""
}

// observe is what one run reports: the observables named, in that order, from
// the clock and the outcome's features; every feature in name order and the
// clock when none are named. The clock's name is reserved, so a feature called
// clock is never reported; a feature the run does not hold is left out.
func observe(ctx *runtime.Context, outcome runtime.Outcome, observables []string) []runtime.CalcOutputValue {
	names := observables
	if len(names) == 0 {
		names = make([]string, 0, len(outcome.Outputs)+1)
		for name := range outcome.Outputs {
			if name != ClockObservable {
				names = append(names, name)
			}
		}
		slices.Sort(names)
		names = append(names, ClockObservable)
	}
	outputs := make([]runtime.CalcOutputValue, 0, len(names))
	for _, name := range names {
		if name == ClockObservable {
			outputs = append(outputs, runtime.CalcOutputValue{Name: name, Value: ctx.ClockValue()})
			continue
		}
		if value, ok := outcome.Outputs[name]; ok {
			outputs = append(outputs, runtime.CalcOutputValue{Name: name, Value: value})
		}
	}
	return outputs
}

// unobserved is the error of an observable no completed run produced: the
// action holds no such feature. Nothing is held against a table whose every run failed.
func unobserved(table runtime.SweepTable, observables []string) error {
	produced := make(map[string]bool)
	completed := 0
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		completed++
		for _, out := range row.Outputs {
			produced[out.Name] = true
		}
	}
	if completed == 0 {
		return nil
	}
	for _, name := range observables {
		if !produced[name] {
			return fmt.Errorf("%w: %s holds no feature named %s (the clock is observed as %s)",
				runtime.ErrSweepRuns, table.Target, name, ClockObservable)
		}
	}
	return nil
}

// distributionLines summarise each observable over the completed runs: its
// extremes, mean and percentiles where the runs produced numbers, the count of
// each distinct value otherwise, and a histogram of the numbers.
func distributionLines(table runtime.SweepTable) []string {
	var lines []string
	for _, name := range newSweepColumns(table).outputs {
		lines = append(lines, observableLines(table, name)...)
	}
	return lines
}

// observableLines is the summary of one observable.
func observableLines(table runtime.SweepTable, name string) []string {
	var numbers []semantics.Value
	var unit string
	units := make(map[string]bool)
	counts := make(map[string]int)
	var order []string
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		for _, out := range row.Outputs {
			if out.Name != name {
				continue
			}
			if n, ok := runtime.MagnitudeValue(out.Value); ok {
				numbers = append(numbers, n)
				unit = ""
				if q := out.Value.Quantity(); q != nil {
					unit = q.Unit.String()
				}
				units[unit] = true
				continue
			}
			text := objectText(row.Context, out.Value)
			if counts[text] == 0 {
				order = append(order, text)
			}
			counts[text]++
		}
	}
	if len(units) > 1 {
		return []string{fmt.Sprintf("%s: %d run(s) produced numbers in more than one unit; no distribution", name, len(numbers))}
	}
	var lines []string
	if d := runtime.Distribute(numbers); d != nil {
		lines = append(lines, distributionLine(name, d, unit))
		lines = append(lines, histogramLines(d, unit)...)
	}
	if len(order) > 0 {
		slices.Sort(order)
		parts := make([]string, len(order))
		for i, text := range order {
			parts[i] = fmt.Sprintf("%s ×%d", text, counts[text])
		}
		lines = append(lines, fmt.Sprintf("%s: %s", name, strings.Join(parts, ", ")))
	}
	return lines
}

// distributionLine is the one-line summary of a distribution, its numbers in unit.
func distributionLine(name string, d *runtime.Distribution, unit string) string {
	return fmt.Sprintf("%s: %d run(s), min %s, mean %s, max %s, p50 %s, p90 %s", name, d.Count,
		withUnit(d.Min, unit), withUnit(drawnMean(d), unit), withUnit(d.Max, unit), withUnit(d.P50, unit), withUnit(d.P90, unit))
}

// drawnMean is a distribution's mean as the Real it is.
func drawnMean(d *runtime.Distribution) semantics.Value {
	return semantics.Value{Kind: semantics.ValReal, Real: d.Mean}
}

// histogramLines render a histogram one bin per line: the bin's bounds to four
// significant digits (one number when they meet), a bar scaled to the runs,
// and the count, the columns aligned.
func histogramLines(d *runtime.Distribution, unit string) []string {
	const barWidth = 20
	labels := make([]string, len(d.Histogram))
	width := 0
	for i, bin := range d.Histogram {
		labels[i] = binLabel(bin, unit)
		width = max(width, len([]rune(labels[i])))
	}
	lines := make([]string, len(d.Histogram))
	for i, bin := range d.Histogram {
		bar := strings.Repeat("#", (bin.Count*barWidth+d.Count-1)/d.Count)
		lines[i] = fmt.Sprintf("  %-*s %-*s %d", width, labels[i], barWidth, bar, bin.Count)
	}
	return lines
}

// binLabel spells a bin's bounds, `lo..hi` in the unit, or the one number both are.
func binLabel(bin runtime.HistogramBin, unit string) string {
	if bin.Lo == bin.Hi {
		return withUnit(bin.Lo, unit)
	}
	label := compactNumber(bin.Lo) + ".." + compactNumber(bin.Hi)
	if unit != "" {
		label += " [" + unit + "]"
	}
	return label
}

// compactNumber spells a bin bound: an Integer in full, a Real to four significant digits.
func compactNumber(x semantics.Value) string {
	if x.Kind == semantics.ValInt {
		return strconv.FormatInt(x.Int, 10)
	}
	return strconv.FormatFloat(x.Real, 'g', 4, 64)
}

// withUnit spells a number as the table spells it, in unit, bare when it has none.
func withUnit(x semantics.Value, unit string) string {
	text := semantics.FormatConst(x)
	if unit == "" {
		return text
	}
	return text + " [" + unit + "]"
}
