package repl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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
	comparePrefix = "compare "
)

// ObservablePair is a stored observable and the feature of the run that answers
// it; an empty Feature is the target's feature of the observable's own name.
type ObservablePair = simresults.ObservablePair

// CompareResults runs each configuration the sidecar indexes as its tool ran
// it — on its target, for its run count, under its draw policy — and reports
// the tool's distribution of every observable beside the runs', one verdict
// per configuration. Numbers are read as they are; nothing is scaled or tuned.
func (s *Session) CompareResults(results *simresults.Results, opts CompareOptions) []Verdict {
	var verdicts []Verdict
	s.CompareResultsEach(results, opts, func(v Verdict) { verdicts = append(verdicts, v) })
	return verdicts
}

// CompareResultsEach is CompareResults reporting each verdict to report as it is
// decided, in the order CompareResults lists them, so a caller prints a
// configuration's comparison while the next one runs.
func (s *Session) CompareResultsEach(results *simresults.Results, opts CompareOptions, report func(Verdict)) {
	defer s.enter()()
	if results == nil || len(results.Configurations) == 0 {
		report(unresolvedVerdict("compare", "the results index no run configuration"))
		return
	}
	selected, refused := opts.selection(results)
	for i := range results.Configurations {
		cfg := &results.Configurations[i]
		if len(opts.Only) > 0 && !selected[i] {
			continue
		}
		report(s.withTrace(s.compareVerdict(cfg, results.Repeats(i), opts)))
	}
	for _, v := range refused {
		report(v)
	}
}

// selection is the configurations -action names, and the names it refuses.
func (o CompareOptions) selection(results *simresults.Results) (selected []bool, refused []Verdict) {
	selected, refusals := results.Select(o.Only)
	for _, r := range refusals {
		refused = append(refused, unresolvedVerdict(comparePrefix+r.Name, r.Reason))
	}
	return selected, refused
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
	answered, table, err := s.runsTable(inv, count, opts.Seed, nil, policy, step, true)
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

// storedRuns spells how many runs the tool stored of a configuration.
func storedRuns(cfg *simresults.ConfigurationResults) string { return cfg.StoredRunsText() }

// comparisonTable tables the comparison as simresults renders it.
func comparisonTable(cfg *simresults.ConfigurationResults, table sweep.SweepTable, observe []ObservablePair) []string {
	return simresults.Compare(cfg, table, observe).Lines()
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
