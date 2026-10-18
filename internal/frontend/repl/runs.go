package repl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep"
)

const runsUsage = "usage: %runs <n> [<seed>] <action> [<observable>...] | %runs <n> [<seed>] <analysis>[(<args>)] [<object>]; the seed is left out under %draws min, max or average"

// ClockObservable names the observable every run of a Monte Carlo reports
// beside the action's features: the simulation clock when the action completed.
const ClockObservable = analysis.ClockObservable

// ErrRunsReplay is the refusal of a Monte Carlo while the schedule replays a
// witness: the witness fixes every draw, so the runs could not differ.
var ErrRunsReplay = errors.New("a Monte Carlo runs under a driving schedule, not a replay")

// doRuns carries out %runs at the prompt: the number of runs, the seed their seeds
// derive from — left out under a fixed %draws policy — the action, then the observables;
// or, in the action's place, a Simulation::MonteCarlo analysis case with its arguments.
func (s *Session) doRuns(tail string) ([]string, bool, error) {
	count, seed, rest, err := splitRunsTail(tail, s.draws.Fixed())
	if err != nil {
		return []string{errPrefix + err.Error(), runsUsage}, false, nil
	}
	if s.namesAnalysisCase(rest) {
		inv, _ := splitAnalysisArgs(rest)
		return s.withTrace(s.monteCarloVerdict(inv, count, seed)).Lines, false, nil
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

// splitRunsTail reads the number of runs and the seed off the front of a %runs tail;
// under a fixed draw policy a second word that is no number is the action, and no seed.
func splitRunsTail(tail string, fixed bool) (int64, *uint64, string, error) {
	fields := strings.Fields(strings.TrimSpace(tail))
	if len(fields) < 2 || (len(fields) < 3 && !fixed) {
		return 0, nil, "", errors.New("name the number of runs, the seed, then the action")
	}
	count, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || count <= 0 {
		return 0, nil, "", fmt.Errorf("%w: %q is not a number of runs to make", sweep.ErrSweepRuns, fields[0])
	}
	taken := 2
	seed, err := strconv.ParseUint(fields[1], 10, 64)
	drawn := &seed
	switch {
	case err == nil:
	case fixed:
		taken, drawn = 1, nil
	default:
		return 0, nil, "", fmt.Errorf("%w: %q is not a seed; the seed may be left out only under a fixed %%draws policy", sweep.ErrSweepRuns, fields[1])
	}
	rest := strings.TrimSpace(tail)
	for range taken {
		rest = strings.TrimSpace(rest[strings.IndexFunc(rest, unicode.IsSpace):])
	}
	return count, drawn, rest, nil
}

// RunRuns runs the action count times, each run drawing its modeled randomness
// from a seed of its own derived from seed — the session's when seed is nil, and
// none when it has none, as a fixed draw policy allows — on an object of performer
// when one is named, and reports the table of the observables named — every
// feature the action holds and the clock when none are — with each one's distribution.
func (s *Session) RunRuns(name string, performer []string, count int64, seed *uint64, observables []string) Verdict {
	defer s.enter()()
	return s.withTrace(s.runsVerdict(Behavior{Name: name, Performer: performer}, count, seed, observables))
}

// runsVerdict makes the runs and reports them as a sweep reports its table, the
// distributions after it. An observable no completed run produced is an error.
func (s *Session) runsVerdict(action Behavior, count int64, seed *uint64, observables []string) Verdict {
	label := "runs " + action.Name
	inv, unresolved := s.resolveInvocation([]Behavior{action}, nil, nil)
	if inv == nil {
		return unresolved[0]
	}
	answered, table, err := s.runsTable(inv, count, seed, observables, s.draws, s.clockStep, false)
	if err != nil {
		return standing(unresolvedVerdict(label, err.Error()), answered)
	}
	status, rows := sweepStatus(table)
	lines := append(sweepTraces(table), sweepTableLines(table)...)
	lines = append(lines, analysis.DistributionLines(table, objectText)...)
	return standing(Verdict{
		Subject: label,
		Status:  status,
		Lines:   lines,
		Values:  sweepValues(table, rows),
		Rows:    rows,
	}, answered)
}

// runsTable makes the runs of a Monte Carlo of the invocation under the draw policy,
// on a clock stepping by step, and returns their table with the plan that answered, nil
// for a refusal made before any engine ran. A seedless Monte Carlo under the random
// policy is refused: its draws would have no source. A caller reading only the
// numbers and traces the runs produced sets release, and no row keeps its context.
func (s *Session) runsTable(inv *freshInvocation, count int64, seed *uint64, observables []string, draws runtime.DrawPolicy, step float64, release bool) (*analysis.Plan, sweep.SweepTable, error) {
	if _, replaying := s.drivenSchedule().Replay(); replaying {
		return nil, sweep.SweepTable{}, ErrRunsReplay
	}
	if dup := duplicateName(observables); dup != "" {
		return nil, sweep.SweepTable{}, fmt.Errorf("%w: observable %s is named twice", sweep.ErrSweepRuns, dup)
	}
	if seed == nil && s.modelSeed.set {
		session := s.modelSeed.value
		seed = &session
	}
	plan := sweep.SeedlessMonteCarloPlan(count)
	if seed != nil {
		plan = sweep.MonteCarloPlan(count, *seed)
	} else if !draws.Fixed() {
		return nil, sweep.SweepTable{}, fmt.Errorf("%w: the runs draw at random; name the seed they draw from, or fix the draws as %%draws min|max|average", sweep.ErrSweepRuns)
	}
	run := func(rt *runtime.Context, bindings []sweep.SweepBinding) (sweep.SweepRunResult, error) {
		i, ok := sweep.RunNumber(bindings)
		if !ok {
			return sweep.SweepRunResult{}, fmt.Errorf("%w: the row numbers no run", sweep.ErrSweepRuns)
		}
		if seed != nil {
			rt.SetModelSeed(runtime.RunSeed(*seed, i))
		} else {
			rt.ClearModelSeed()
		}
		outcome, err := inv.run(rt)
		if err != nil {
			return sweep.SweepRunResult{ReleaseContext: release}, err
		}
		return sweep.SweepRunResult{Outputs: analysis.Observe(rt, outcome, observables), ReleaseContext: release}, nil
	}
	model := s.freshModel()
	s.state.Unlock()
	answered, err := s.sweep(inv.subject(), model, plan, run, draws, step)
	s.state.Lock()
	if err != nil {
		return &answered, sweep.SweepTable{}, err
	}
	table := answered.Result.Table()
	if err := analysis.Unobserved(table, observables); err != nil {
		return &answered, table, err
	}
	return &answered, table, nil
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
