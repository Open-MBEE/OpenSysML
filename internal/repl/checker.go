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

// checkSettings is what the check engines are asked beside the action: the features
// that may not diverge, the properties every state must satisfy, the inputs left
// free and the assumptions made over the initial state, where witnesses go.
type checkSettings struct {
	diverge    []string
	properties []string
	inputs     []string
	assume     []string
	witnessDir string
	// depth, states and unroll bound the search, 0 for the engine's default;
	// timeout is the plan's clock, 0 for none.
	depth, states, unroll int
	timeout               time.Duration
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
	return len(c.diverge) > 0 || len(c.properties) > 0 || len(c.inputs) > 0 || len(c.assume) > 0 ||
		c.witnessDir != "" || c.depth > 0 || c.states > 0 || c.unroll > 0 || c.timeout > 0
}

// frees is what the settings leave open beside the schedule: the inputs, once one
// is released or an assumption is made about them.
func (c checkSettings) frees() analysis.Freedom {
	if len(c.inputs) > 0 || len(c.assume) > 0 {
		return analysis.FreeInputs
	}
	return analysis.FreeNothing
}

// checking reports whether the session's selection puts an action to the check
// engines, which decide its schedules rather than stepping one run: under
// %engine check or %engine smt always, under %engine all once a check setting is made.
func (s *Session) checking() bool {
	return s.checkOnly() || s.symbolic() || s.engine.Mode == analysis.SelectAll && s.checker.given()
}

// checkOnly reports whether the check engine alone is selected.
func (s *Session) checkOnly() bool {
	return s.engine == analysis.Only(analysis.CheckEngineName)
}

// symbolic reports whether the smt engine alone is selected.
func (s *Session) symbolic() bool {
	return s.engine == analysis.Only(analysis.SMTEngineName)
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

// CheckInputs returns the features a check leaves free although the model binds them.
func (s *Session) CheckInputs() []string {
	defer s.reading()()
	return append([]string(nil), s.checker.inputs...)
}

// SetCheckInputs names the features a check leaves free although the model binds
// them; each is resolved when the check runs.
func (s *Session) SetCheckInputs(features []string) {
	defer s.enter()()
	s.checker.inputs = append([]string(nil), features...)
}

// CheckAssume returns the constraints and requirements a check assumes over the
// initial state.
func (s *Session) CheckAssume() []string {
	defer s.reading()()
	return append([]string(nil), s.checker.assume...)
}

// SetCheckAssume names the constraints and requirements a check assumes over the
// initial state; each is resolved when the check runs.
func (s *Session) SetCheckAssume(names []string) {
	defer s.enter()()
	s.checker.assume = append([]string(nil), names...)
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

// doCheckInput shows or sets the features a check leaves free, `off` freeing only
// the ones the model leaves unbound.
func (s *Session) doCheckInput(args []string) []string {
	if len(args) > 0 {
		s.checker.inputs = settingList(args)
	}
	return []string{"check-input: " + settingText(s.checker.inputs, "only the inputs the model leaves unbound")}
}

// doCheckAssume shows or sets the conditions a check assumes over the initial
// state, `off` assuming none.
func (s *Session) doCheckAssume(args []string) []string {
	if len(args) > 0 {
		s.checker.assume = settingList(args)
	}
	return []string{"check-assume: " + settingText(s.checker.assume, "none")}
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
// `depth=<n>`, `states=<n>`, `unroll=<n>` or `timeout=<d>`; `off` restores the engines' defaults.
func (s *Session) doCheckBounds(args []string) []string {
	if len(args) == 1 && args[0] == "off" {
		s.checker.depth, s.checker.states, s.checker.unroll, s.checker.timeout = 0, 0, 0, 0
		args = nil
	}
	depth, states, unroll, timeout := s.checker.depth, s.checker.states, s.checker.unroll, s.checker.timeout
	for _, arg := range args {
		name, value, found := strings.Cut(arg, "=")
		if !found {
			return []string{"usage: %check-bounds [depth=<n>] [states=<n>] [unroll=<n>] [timeout=<duration>] | off"}
		}
		var err error
		switch name {
		case "depth":
			depth, err = parseBound(name, value)
		case "states":
			states, err = parseBound(name, value)
		case "unroll":
			unroll, err = parseBound(name, value)
		case "timeout":
			timeout, err = time.ParseDuration(value)
			if err == nil && timeout <= 0 {
				err = fmt.Errorf("timeout takes a duration above zero, not %q", value)
			}
		default:
			err = fmt.Errorf("%q is not a bound; the bounds are depth, states, unroll and timeout", name)
		}
		if err != nil {
			return []string{errPrefix + err.Error()}
		}
	}
	s.checker.depth, s.checker.states, s.checker.unroll, s.checker.timeout = depth, states, unroll, timeout
	return []string{"check-bounds: " + boundText("depth", depth, analysis.DefaultCheckDepth) +
		", " + boundText("states", states, analysis.DefaultCheckStates) +
		", " + boundText("unroll", unroll, analysis.DefaultUnroll) + ", " + timeoutText(timeout)}
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

// checkAction decides every schedule of the action: a violation, a deadlock, a
// failure or a divergence of the selected features, witnesses replayed and written.
func (s *Session) checkAction(name string, performer []string) Verdict {
	asks, err := s.checkAsks(name, performer)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	kind := analysis.CheckKind(asks.check, asks.holds)
	if s.symbolic() {
		kind = analysis.Holds
	}
	policy, explores := s.exploring()
	if !explores {
		policy = runtime.DefaultExploreSchedulePolicy
	}
	budget := s.checkBudget(policy, kind)
	// The check engine alone searches under its own defaults, the smt engine alone
	// unrolls to its own; beside an exploration, the one figure is the exploration
	// policy's unless a check bound sets it.
	if s.checkOnly() && !explores && s.checker.depth <= 0 {
		budget.Depth = analysis.DefaultCheckDepth
	}
	if s.symbolic() && !explores && s.checker.depth <= 0 {
		budget.Depth = 0
	}
	if s.checkOnly() && !explores && s.checker.states <= 0 {
		budget.Runs = analysis.DefaultCheckStates
	}
	return s.checkVerdict(name, policy, kind, asks, budget)
}

// checkAsks is the action's schedules as the engines are asked about them: the
// explicit-state search's ask, the symbolic one's, and one run for an exploration.
type checkAsks struct {
	check *analysis.CheckAsk
	holds *analysis.HoldsAsk
	run   analysis.Linearization
}

// checkAsks resolves the session's check settings against the action on its
// performer: how one run starts, the properties, the assumptions.
func (s *Session) checkAsks(name string, performer []string) (checkAsks, error) {
	asks, err := s.actionAsks(name, performer)
	if err != nil {
		return checkAsks{}, err
	}
	properties, conditions, scope, err := s.checkProperties()
	if err != nil {
		return checkAsks{}, err
	}
	assume, err := s.checkAssumptions()
	if err != nil {
		return checkAsks{}, err
	}
	asks.check.Properties = properties
	asks.holds.Conditions, asks.holds.Scope, asks.holds.Assume = conditions, scope, assume
	return asks, nil
}

// actionAsks is the action on its performer as the engines start it: one run
// for an exploration, and the asks of a search with no property or assumption yet.
func (s *Session) actionAsks(name string, performer []string) (checkAsks, error) {
	sym, err := s.exploredAction(name)
	if err != nil {
		return checkAsks{}, err
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
	asks := checkAsks{
		check: &analysis.CheckAsk{
			Start:      start,
			Diverge:    append([]string(nil), s.checker.diverge...),
			WitnessDir: s.checker.witnessDir,
		},
		holds: &analysis.HoldsAsk{
			Behavior:   sym,
			Start:      start,
			Inputs:     append([]string(nil), s.checker.inputs...),
			WitnessDir: s.checker.witnessDir,
		},
		run: run,
	}
	if len(performer) > 0 {
		asks.check.Performer, asks.holds.Performer = performer[0], performer[0]
	}
	return asks, nil
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
	budget.Unroll = s.checker.unroll
	budget.Deadline = s.checker.deadline()
	return budget
}

// checkVerdict puts the question to the engines under the session's selection and
// budget, the session's state released for the run, and reports what stood.
func (s *Session) checkVerdict(name string, policy runtime.SchedulePolicy, kind analysis.Kind, asks checkAsks, budget analysis.Budget) Verdict {
	model := s.freshModel()
	selection := s.engine
	free := s.checker.frees()
	s.state.Unlock()
	answered, err := s.engines.Check(context.Background(), analysis.Request{
		Model:     model,
		Subject:   name,
		Schedule:  policy,
		Budget:    budget,
		Selection: selection,
	}, kind, free, asks.check, asks.holds, asks.run)
	s.state.Lock()
	if err != nil {
		return standing(checkStoppedVerdict(name, err), &answered)
	}
	if x := answered.Result.Exploration(); x != nil {
		return standing(explorationVerdict(name, x), &answered)
	}
	if answered.Result.Check() == nil && answered.Result.Engine == analysis.SMTEngineName {
		return standing(decidedVerdict(name, answered.Result), &answered)
	}
	return standing(checkedVerdict(name, answered.Result), &answered)
}

// checkProperties resolves the session's properties to the constraint or
// requirement each names: as the check engine evaluates each about the action's
// performer at every state, as the symbols the symbolic engine translates, and
// the scope they resolve in.
func (s *Session) checkProperties() ([]runtime.CheckProperty, []*symbols.Symbol, *symbols.Scope, error) {
	docScopes := s.docScopes()
	if (len(s.checker.properties) > 0 || len(s.checker.assume) > 0) && len(docScopes) == 0 {
		return nil, nil, nil, errors.New("no declarations loaded")
	}
	var root *symbols.Scope
	if len(docScopes) > 0 {
		root = docScopes[0]
	}
	properties := make([]runtime.CheckProperty, 0, len(s.checker.properties))
	conditions := make([]*symbols.Symbol, 0, len(s.checker.properties))
	for _, name := range s.checker.properties {
		sym, check, err := s.checkCondition(name)
		if err != nil {
			return nil, nil, nil, err
		}
		scope := declaringScope(sym, root)
		properties = append(properties, runtime.CheckProperty{Name: name, Holds: func(ctx *runtime.Context, exec *runtime.ActionExecutor) (bool, error) {
			result, err := check(ctx, sym, scope, exec.Performer())
			if err != nil && !errors.Is(err, runtime.ErrViolated) {
				return false, err
			}
			return result.Holds, nil
		}})
		conditions = append(conditions, sym)
	}
	return properties, conditions, root, nil
}

// checkAssumptions resolves the session's assumptions to the constraint or
// requirement each names.
func (s *Session) checkAssumptions() ([]*symbols.Symbol, error) {
	assume := make([]*symbols.Symbol, 0, len(s.checker.assume))
	for _, name := range s.checker.assume {
		sym, _, err := s.checkCondition(name)
		if err != nil {
			return nil, err
		}
		assume = append(assume, sym)
	}
	return assume, nil
}

// conditionCheck evaluates a constraint or requirement about an object.
type conditionCheck func(*runtime.Context, *symbols.Symbol, *symbols.Scope, *runtime.Instance) (runtime.CheckResult, error)

// checkCondition resolves name to the constraint or requirement it names and
// how the runtime evaluates it about an object.
func (s *Session) checkCondition(name string) (*symbols.Symbol, conditionCheck, error) {
	sym, _, err := s.lookupSymbol(name)
	if err != nil {
		return nil, nil, err
	}
	switch {
	case runtime.RequireConstraint(sym) == nil:
		return sym, (*runtime.Context).CheckConstraintOn, nil
	case runtime.RequireRequirement(sym) == nil:
		return sym, (*runtime.Context).CheckRequirementOn, nil
	}
	return nil, nil, fmt.Errorf("%q is not a constraint or requirement", name)
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

// decidedVerdict reports what the symbolic engine decided about the action: a
// violation fails, a proof holds, and a bounded or uncovered answer is undecided;
// the inputs it ranged over or a witness chose, and its assumptions, follow.
func decidedVerdict(name string, result analysis.Result) Verdict {
	v := Verdict{Subject: name}
	switch {
	case !result.Covered():
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? Action %s: not covered", name), "  "+result.Reason)
	case result.Claim == analysis.ClaimViolated:
		v.Status = VerdictFails
		v.Lines = append(v.Lines, fmt.Sprintf("✗ Action %s: %s", name, result.Reason))
	case result.Strength == analysis.Proved:
		v.Status = VerdictHolds
		v.Lines = append(v.Lines, fmt.Sprintf("✓ Action %s: %s", name, result.Claim))
	default:
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? Action %s: %s (%s)", name, result.Claim, result.Strength))
	}
	if inputs := inputLines(result.Inputs); inputs != "" {
		v.Lines = append(v.Lines, "  inputs: "+inputs)
	}
	if len(result.Assumptions) > 0 {
		v.Lines = append(v.Lines, "  assumed: "+strings.Join(result.Assumptions, ", "))
	}
	if w := result.Witness; w != nil && w.Written != "" {
		v.Lines = append(v.Lines, "  witness: "+w.Written)
	}
	return v
}

// inputLines spells the inputs a symbolic engine reported, "" when it reported none.
func inputLines(inputs []analysis.Input) string {
	parts := make([]string, 0, len(inputs))
	for _, in := range inputs {
		parts = append(parts, in.String())
	}
	return strings.Join(parts, ", ")
}

// witnessPath spells where the i'th witness was written, "" when none was.
func witnessPath(paths []string, i int) string {
	if i >= len(paths) || paths[i] == "" {
		return ""
	}
	return " (witness " + paths[i] + ")"
}

// CheckBounds returns the search bounds a check runs under: depth, states and
// unroll, 0 for the engines' defaults, and the timeout, 0 for none.
func (s *Session) CheckBounds() (depth, states, unroll int, timeout time.Duration) {
	defer s.reading()()
	return s.checker.depth, s.checker.states, s.checker.unroll, s.checker.timeout
}

// SetCheckBounds sets the search bounds a check runs under; 0 keeps the engines'
// default for depth, states and unroll, and no clock for the timeout.
func (s *Session) SetCheckBounds(depth, states, unroll int, timeout time.Duration) {
	defer s.enter()()
	s.checker.depth, s.checker.states, s.checker.unroll, s.checker.timeout = depth, states, unroll, timeout
}
