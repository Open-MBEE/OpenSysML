package repl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkSettings is what the check engine is asked beside the action: the features
// that may not diverge, the properties every state must satisfy, where witnesses go.
type checkSettings struct {
	diverge    []string
	properties []string
	witnessDir string
	// depth and states bound the search, 0 for the engine's default; timeout is
	// the plan's clock, 0 for none.
	depth, states int
	timeout       time.Duration
}

// deadline is the plan's clock as the timeout sets it, zero for none.
func (c checkSettings) deadline() time.Time {
	if c.timeout <= 0 {
		return time.Time{}
	}
	return time.Now().Add(c.timeout)
}

// given reports whether any check setting was made, which under %engine all puts
// an action to the check engine beside the others.
func (c checkSettings) given() bool {
	return len(c.diverge) > 0 || len(c.properties) > 0 || c.witnessDir != "" ||
		c.depth > 0 || c.states > 0 || c.timeout > 0
}

// checking reports whether the session's selection puts an action to the check
// engine, which searches its schedules rather than stepping one run: under
// %engine check always, under %engine all once a check setting is made.
func (s *Session) checking() bool {
	return s.checkOnly() || s.engine.Mode == analysis.SelectAll && s.checker.given()
}

// checkOnly reports whether the check engine alone is selected.
func (s *Session) checkOnly() bool {
	return s.engine == analysis.Only(analysis.CheckEngineName)
}

// CheckDiverge returns the features a check compares the final values of; none
// compares every attribute of the action and of its performing object.
func (s *Session) CheckDiverge() []string {
	defer s.reading()()
	return append([]string(nil), s.checker.diverge...)
}

// SetCheckDiverge names the features a check compares the final values of.
func (s *Session) SetCheckDiverge(features []string) {
	defer s.enter()()
	s.checker.diverge = append([]string(nil), features...)
}

// CheckProperties returns the constraints and requirements a check evaluates at
// every stable state.
func (s *Session) CheckProperties() []string {
	defer s.reading()()
	return append([]string(nil), s.checker.properties...)
}

// SetCheckProperties names the constraints and requirements a check evaluates at
// every stable state; each is resolved when the check runs.
func (s *Session) SetCheckProperties(names []string) {
	defer s.enter()()
	s.checker.properties = append([]string(nil), names...)
}

// CheckWitnessDir returns where a check writes its witnesses, "" for nowhere.
func (s *Session) CheckWitnessDir() string {
	defer s.reading()()
	return s.checker.witnessDir
}

// SetCheckWitnessDir names where a check writes its witnesses, one file per
// violation and per divergent value; "" writes none.
func (s *Session) SetCheckWitnessDir(dir string) {
	defer s.enter()()
	s.checker.witnessDir = dir
}

// doCheckDiverge shows or sets the features a check compares, `off` clearing them.
func (s *Session) doCheckDiverge(args []string) []string {
	if len(args) > 0 {
		s.checker.diverge = settingList(args)
	}
	return []string{"check-diverge: " + settingText(s.checker.diverge, "every attribute of the action and of its performing object")}
}

// doCheckProperty shows or sets the properties a check evaluates, `off` clearing them.
func (s *Session) doCheckProperty(args []string) []string {
	if len(args) > 0 {
		s.checker.properties = settingList(args)
	}
	return []string{"check-property: " + settingText(s.checker.properties, "none")}
}

// doCheckWitness shows or sets where a check writes its witnesses, `off` writing none.
func (s *Session) doCheckWitness(args []string) []string {
	if len(args) > 0 {
		if args[0] == "off" {
			s.checker.witnessDir = ""
		} else {
			s.checker.witnessDir = args[0]
		}
	}
	if s.checker.witnessDir == "" {
		return []string{"check-witness: off"}
	}
	return []string{"check-witness: " + s.checker.witnessDir}
}

// doCheckBounds shows or sets the bounds a check searches within, each as
// `depth=<n>`, `states=<n>` or `timeout=<d>`; `off` restores the engine's defaults.
func (s *Session) doCheckBounds(args []string) []string {
	if len(args) == 1 && args[0] == "off" {
		s.checker.depth, s.checker.states, s.checker.timeout = 0, 0, 0
		args = nil
	}
	depth, states, timeout := s.checker.depth, s.checker.states, s.checker.timeout
	for _, arg := range args {
		name, value, found := strings.Cut(arg, "=")
		if !found {
			return []string{"usage: %check-bounds [depth=<n>] [states=<n>] [timeout=<duration>] | off"}
		}
		var err error
		switch name {
		case "depth":
			depth, err = parseBound(name, value)
		case "states":
			states, err = parseBound(name, value)
		case "timeout":
			timeout, err = time.ParseDuration(value)
			if err == nil && timeout <= 0 {
				err = fmt.Errorf("timeout takes a duration above zero, not %q", value)
			}
		default:
			err = fmt.Errorf("%q is not a bound; the bounds are depth, states and timeout", name)
		}
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
	}
	s.checker.depth, s.checker.states, s.checker.timeout = depth, states, timeout
	return []string{"check-bounds: " + boundText("depth", depth, analysis.DefaultCheckDepth) +
		", " + boundText("states", states, analysis.DefaultCheckStates) + ", " + timeoutText(timeout)}
}

// parseBound reads a search bound, a count of at least one.
func parseBound(name, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%s takes a bound of at least one, not %q", name, value)
	}
	return n, nil
}

// boundText spells a bound, marking the engine's default where none was set.
func boundText(name string, value, fallback int) string {
	if value <= 0 {
		return fmt.Sprintf("%s=%d (default)", name, fallback)
	}
	return fmt.Sprintf("%s=%d", name, value)
}

// timeoutText spells the check's clock, off where none was set.
func timeoutText(timeout time.Duration) string {
	if timeout <= 0 {
		return "timeout=off"
	}
	return "timeout=" + timeout.String()
}

// settingList reads a list setting's arguments: `off` is the empty list.
func settingList(args []string) []string {
	if len(args) == 1 && args[0] == "off" {
		return nil
	}
	return append([]string(nil), args...)
}

// settingText spells a list setting, or what the empty list means.
func settingText(values []string, empty string) string {
	if len(values) == 0 {
		return "off (" + empty + ")"
	}
	return strings.Join(values, " ")
}

// doReplay installs the schedule a witness file fixes, so the next %action or
// %state steps the run it records under %step and %continue; under the check
// engine %action would search instead, so the hint names the selection to leave.
func (s *Session) doReplay(args []string) []string {
	if len(args) != 1 {
		return []string{"usage: %replay <witness>"}
	}
	policy, err := runtime.ParseSchedulePolicy("replay:" + args[0])
	if err != nil {
		return []string{errPrefix + err.Error()}
	}
	if err := s.setSchedule(policy); err != nil {
		return []string{errPrefix + err.Error()}
	}
	out := []string{fmt.Sprintf("schedule: %s", s.schedule), "Use %action or %state to start the run the witness records, then %step or %continue"}
	if s.checking() {
		out = append(out, fmt.Sprintf("Under %%engine %s, %%action searches every schedule; select %%engine auto to step the one the witness records", s.engine))
	}
	return out
}

// checkAction searches every schedule of the action for a violation, a deadlock,
// a failure or a divergence of the selected features, witnesses replayed and written.
func (s *Session) checkAction(name string, performer []string) Verdict {
	properties, err := s.checkProperties()
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	ask, run, err := s.checkAsk(name, performer)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	ask.Properties = properties
	kind := analysis.Outcomes
	if len(properties) > 0 {
		kind = analysis.Holds
	}
	policy, explores := s.exploring()
	if !explores {
		policy = runtime.DefaultExploreSchedulePolicy
	}
	budget := s.checkBudget(policy, kind)
	// The check engine alone searches under its own defaults; beside an exploration,
	// the one figure is the exploration policy's unless a check bound sets it.
	if s.checkOnly() && !explores && s.checker.depth <= 0 {
		budget.Depth = analysis.DefaultCheckDepth
	}
	if s.checkOnly() && !explores && s.checker.states <= 0 {
		budget.Runs = analysis.DefaultCheckStates
	}
	return s.checkVerdict(name, policy, kind, ask, run, budget)
}

// checkAsk is the action's schedules as a question: how one run starts the action
// on its performer, and what the session has a check compare and write.
func (s *Session) checkAsk(name string, performer []string) (*analysis.CheckAsk, analysis.Linearization, error) {
	sym, err := s.exploredAction(name)
	if err != nil {
		return nil, nil, err
	}
	plan := s.planFresh(performer...)
	start := func(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
		return freshAction(plan.bind(ctx), sym, performer)
	}
	run := func(ctx *runtime.Context) (runtime.Outcome, error) {
		exec, err := start(ctx)
		if err != nil {
			return runtime.Outcome{}, err
		}
		if err := exec.RunToCompletion(); err != nil {
			return runtime.Outcome{}, err
		}
		return completedActionOutcome(ctx, exec, name)
	}
	ask := &analysis.CheckAsk{
		Start:      start,
		Diverge:    append([]string(nil), s.checker.diverge...),
		WitnessDir: s.checker.witnessDir,
	}
	if len(performer) > 0 {
		ask.Performer = performer[0]
	}
	return ask, run, nil
}

// checkBudget is the question's budget under policy with the check's bounds and
// clock on it where they were set.
func (s *Session) checkBudget(policy runtime.SchedulePolicy, kind analysis.Kind) analysis.Budget {
	budget := s.budgetFor(policy, kind)
	if s.checker.depth > 0 {
		budget.Depth = s.checker.depth
	}
	if s.checker.states > 0 {
		budget.Runs = s.checker.states
	}
	budget.Deadline = s.checker.deadline()
	return budget
}

// checkVerdict puts the question to the engines under the session's selection and
// budget, the session's state released for the run, and reports what stood.
func (s *Session) checkVerdict(name string, policy runtime.SchedulePolicy, kind analysis.Kind, ask *analysis.CheckAsk, run analysis.Linearization, budget analysis.Budget) Verdict {
	model := s.freshModel()
	selection := s.engine
	s.state.Unlock()
	answered, err := s.engines.Check(context.Background(), model, name, policy, kind, ask, run, budget, selection)
	s.state.Lock()
	if err != nil {
		return standing(checkStoppedVerdict(name, err), &answered)
	}
	if x := answered.Result.Exploration(); x != nil {
		return standing(explorationVerdict(name, x), &answered)
	}
	return standing(checkedVerdict(name, answered.Result), &answered)
}

// checkProperties resolves the session's properties to the constraint and
// requirement each names, evaluated about the action's performer at every state.
func (s *Session) checkProperties() ([]runtime.CheckProperty, error) {
	docScopes := s.docScopes()
	if len(s.checker.properties) > 0 && len(docScopes) == 0 {
		return nil, errors.New("no declarations loaded")
	}
	properties := make([]runtime.CheckProperty, 0, len(s.checker.properties))
	for _, name := range s.checker.properties {
		sym, _, err := s.lookupSymbol(name)
		if err != nil {
			return nil, err
		}
		scope := declaringScope(sym, docScopes[0])
		var check func(*runtime.Context, *symbols.Symbol, *symbols.Scope, *runtime.Instance) (runtime.CheckResult, error)
		switch {
		case runtime.RequireConstraint(sym) == nil:
			check = (*runtime.Context).CheckConstraintOn
		case runtime.RequireRequirement(sym) == nil:
			check = (*runtime.Context).CheckRequirementOn
		default:
			return nil, fmt.Errorf("%q is not a constraint or requirement", name)
		}
		properties = append(properties, runtime.CheckProperty{Name: name, Holds: func(ctx *runtime.Context, exec *runtime.ActionExecutor) (bool, error) {
			result, err := check(ctx, sym, scope, exec.Performer())
			if err != nil && !errors.Is(err, runtime.ErrViolated) {
				return false, err
			}
			return result.Holds, nil
		}})
	}
	return properties, nil
}

// checkStoppedVerdict reports a check that answered nothing: a refusal, a fault,
// or the plan's clock ending, with what the search reached before it stopped.
func checkStoppedVerdict(name string, err error) Verdict {
	var stopped *runtime.CheckStopped
	if errors.As(err, &stopped) {
		return Verdict{Subject: name, Status: VerdictUnresolved, Lines: []string{
			fmt.Sprintf("? Action %s: incomplete: time (%d states, %d moves, depth %d)", name, stopped.States, stopped.Moves, stopped.MaxDepth),
			"  " + stopped.Cause.Error(),
		}}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Verdict{Subject: name, Status: VerdictUnresolved, Lines: []string{
			fmt.Sprintf("? Action %s: incomplete: time (the plan's clock ended before the search began)", name),
		}}
	}
	return unresolvedVerdict(name, err.Error())
}

// checkedVerdict reports what the check engine found: a violation or a divergence
// fails the check, an exhaustive clean search holds, a bounded one is undecided.
func checkedVerdict(name string, result analysis.Result) Verdict {
	checked := result.Check()
	if checked == nil {
		return Verdict{Subject: name, Status: VerdictUnresolved, Lines: []string{
			fmt.Sprintf("? Action %s could not be checked", name),
			"  " + result.Reason,
		}}
	}
	report := checked.Report
	v := Verdict{Subject: name}
	switch {
	case result.Strength == analysis.NotCovered:
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? Action %s: %s", name, report.Status()), "  "+result.Reason)
	case report.Verdict == runtime.CheckViolation, report.Verdict == runtime.CheckDivergent:
		v.Status = VerdictFails
		v.Lines = append(v.Lines, fmt.Sprintf("✗ Action %s: %s", name, report.Status()))
	case report.Verdict == runtime.CheckExhaustive:
		v.Status = VerdictHolds
		v.Lines = append(v.Lines, fmt.Sprintf("✓ Action %s: %s", name, report.Status()))
	default:
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? Action %s: %s", name, report.Status()))
	}
	for i, violation := range report.Violations {
		v.Lines = append(v.Lines, "  violation: "+violation.String()+witnessPath(checked.Violations, i))
	}
	for i, d := range report.Divergent {
		v.Lines = append(v.Lines, "  divergent: "+d.String())
		for j, value := range d.Values {
			var path []string
			if i < len(checked.Divergent) {
				path = checked.Divergent[i]
			}
			v.Lines = append(v.Lines, fmt.Sprintf("    %s = %s%s", d.Feature, value.Value, witnessPath(path, j)))
		}
	}
	for _, final := range report.Finals {
		v.Lines = append(v.Lines, "  outcome: "+final.Outcome)
	}
	return v
}

// witnessPath spells where the i'th witness was written, "" when none was.
func witnessPath(paths []string, i int) string {
	if i >= len(paths) || paths[i] == "" {
		return ""
	}
	return " (witness " + paths[i] + ")"
}

// CheckBounds returns the search bounds a check runs under: depth and states, 0
// for the engine's defaults, and the timeout, 0 for none.
func (s *Session) CheckBounds() (depth, states int, timeout time.Duration) {
	defer s.reading()()
	return s.checker.depth, s.checker.states, s.checker.timeout
}

// SetCheckBounds sets the search bounds a check runs under; 0 keeps the engine's
// default for depth and states, and no clock for the timeout.
func (s *Session) SetCheckBounds(depth, states int, timeout time.Duration) {
	defer s.enter()()
	s.checker.depth, s.checker.states, s.checker.timeout = depth, states, timeout
}
