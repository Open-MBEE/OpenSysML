package repl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// checkSettings is what the check engines are asked beside the behaviors: the features
// that may not diverge, the properties every state must satisfy, the inputs left
// free and the assumptions made over the initial state, where witnesses go.
type checkSettings struct {
	diverge    []string
	properties []string
	inputs     []string
	assume     []string
	witnessDir string
	// depth, states and unroll bound the search, 0 for the engine's default; timeout is
	// the plan's clock and each solver query's, 0 for none and the solver's own.
	depth, states, unroll int
	timeout               time.Duration
	// checked is the last invocation searched, for %advance to search again.
	checked *checkedInvocation
}

// deadline is the plan's clock as the timeout sets it, zero for none.
func (c checkSettings) deadline() time.Time {
	if c.timeout <= 0 {
		return time.Time{}
	}
	return time.Now().Add(c.timeout)
}

// given reports whether any check setting was made, which under %engine all puts
// a behavior to the check engine beside the others.
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

// explicitOnly names the settings made that the check engine alone reads.
func (c checkSettings) explicitOnly() []string {
	var made []string
	if c.states > 0 {
		made = append(made, "%check-bounds states")
	}
	return made
}

// symbolicOnly names the settings made that the smt engine alone reads.
func (c checkSettings) symbolicOnly() []string {
	var made []string
	if len(c.inputs) > 0 {
		made = append(made, "%check-input")
	}
	if len(c.assume) > 0 {
		made = append(made, "%check-assume")
	}
	if c.unroll > 0 {
		made = append(made, "%check-bounds unroll")
	}
	return made
}

// checkerMisuse reports why a check setting made checks nothing under the engine
// selected alone, and "" when every setting made reaches an engine that reads it.
func (s *Session) checkerMisuse() string {
	switch {
	case s.checkOnly() && len(s.checker.symbolicOnly()) > 0:
		return misuseText(s.checker.symbolicOnly(), analysis.SMTEngineName, analysis.CheckEngineName)
	case s.symbolic() && len(s.checker.explicitOnly()) > 0:
		return misuseText(s.checker.explicitOnly(), analysis.CheckEngineName, analysis.SMTEngineName)
	}
	return ""
}

// misuseText spells the settings made that reader alone reads, which %engine
// selected leaves out.
func misuseText(made []string, reader, selected string) string {
	return fmt.Sprintf("%s %s the %s engine's, which %%engine %s leaves out; select it, as %%engine %s, or every engine, as %%engine all",
		spelled(made), plural(len(made), "is", "are"), reader, selected, reader)
}

// spelled lists names as prose: `a`, `a and b`, `a, b and c`.
func spelled(names []string) string {
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// checking reports whether the session's selection puts a behavior to the check
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
// engine they would search instead, so the hint names the selection to leave.
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
		out = append(out, fmt.Sprintf("Under %%engine %s, %%action and %%state search every schedule; select %%engine auto to step the one the witness records", s.engine))
	}
	return out
}

// checkAction decides every schedule of the action: a violation, a deadlock, a
// failure or a divergence of the selected features, witnesses replayed and written.
func (s *Session) checkAction(name string, performer []string) Verdict {
	return s.checkInvocation([]Behavior{{Name: name, Performer: performer}}, nil, nil)
}

// checkInvocation searches every schedule of the behaviors named, started on one
// clock and, with a horizon, run up to it: a violation, a deadlock, a failure or a
// divergence of the selected features is witnessed, replayed and written. The
// invocation is remembered so %advance can search it again up to a horizon.
func (s *Session) checkInvocation(actions, states []Behavior, horizon *float64) Verdict {
	return s.checkInvocations(actions, states, horizon)[0]
}

// checkInvocations is checkInvocation reporting every behavior that did not
// resolve, one verdict each, where one verdict answers for the searched invocation.
func (s *Session) checkInvocations(actions, states []Behavior, horizon *float64) []Verdict {
	inv, unresolved := s.resolveInvocation(actions, states, horizon)
	if inv == nil {
		if len(unresolved) == 0 {
			return []Verdict{unresolvedVerdict("", "no behavior to check; name an action or a state machine")}
		}
		return unresolved
	}
	return []Verdict{s.checkResolved(inv, actions, states)}
}

// checkResolved puts the resolved invocation to the check engines.
func (s *Session) checkResolved(inv *freshInvocation, actions, states []Behavior) Verdict {
	s.checker.checked = &checkedInvocation{actions: actions, states: states}
	if misuse := s.checkerMisuse(); misuse != "" {
		return unresolvedVerdict(inv.subject(), misuse)
	}
	if s.symbolic() && !inv.singleAction() {
		return unresolvedVerdict(inv.subject(), "the smt engine decides an action's schedules alone; name one, as %action <name>, or select the check engine, as %engine check, to search state machines and behaviors on one clock")
	}
	asks, err := s.checkAsks(inv)
	if err != nil {
		return unresolvedVerdict(inv.subject(), err.Error())
	}
	kind := analysis.CheckKind(asks.check, asks.holds, s.checker.unroll)
	if s.symbolic() && kind == analysis.Outcomes {
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
	return s.checkVerdict(inv, policy, kind, asks, budget)
}

// checkedInvocation is the behaviors the last check searched, as they were named.
type checkedInvocation struct {
	actions, states []Behavior
}

// checkAdvance searches the last checked invocation again, its clock bounded at
// duration: %advance under the check engine, where no debugger steps.
func (s *Session) checkAdvance(duration float64) ([]string, error) {
	last := s.checker.checked
	if last == nil {
		return nil, errors.New("no behavior checked yet; under %engine check, %action or %state searches every schedule of a behavior, and %advance <time> then searches it again up to that instant")
	}
	return s.checkInvocation(last.actions, last.states, &duration).Lines, nil
}

// checkAsks is the invocation's schedules as the engines are asked about them: the
// explicit-state search's ask, the symbolic one's, and one run for an exploration.
// The symbolic ask is an action's alone: nil for a machine or several behaviors.
type checkAsks struct {
	check *analysis.CheckAsk
	holds *analysis.HoldsAsk
	run   analysis.Linearization
}

// checkAsks resolves the session's check settings against the invocation: how
// one run starts, the properties, the assumptions.
func (s *Session) checkAsks(inv *freshInvocation) (checkAsks, error) {
	asks := inv.asks(s.checker)
	properties, conditions, scope, err := s.checkProperties()
	if err != nil {
		return checkAsks{}, err
	}
	assume, err := s.checkAssumptions()
	if err != nil {
		return checkAsks{}, err
	}
	asks.check.Properties = properties
	if asks.holds != nil {
		asks.holds.Conditions, asks.holds.Scope, asks.holds.Assume = conditions, scope, assume
	}
	return asks, nil
}

// checkBudget is the question's budget under policy with the check's bounds and clock on
// it where they were set; the clock is the plan's deadline and each solver query's time.
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
	budget.Solver = s.checker.timeout
	return budget
}

// checkVerdict puts the question to the engines under the session's selection and
// budget, the session's state released for the run, and reports what stood.
func (s *Session) checkVerdict(inv *freshInvocation, policy runtime.SchedulePolicy, kind analysis.Kind, asks checkAsks, budget analysis.Budget) Verdict {
	model := s.freshModel()
	selection := s.engine
	subject, label := inv.subject(), inv.label()
	ctx := s.planContext()
	free := s.checker.frees()
	modelSeed, draws, step := s.askedModelSeed(), s.draws, s.clockStep
	s.state.Unlock()
	answered, err := s.engines.Check(ctx, analysis.Request{
		Model:     model,
		Subject:   subject,
		Schedule:  policy,
		ModelSeed: modelSeed,
		Draws:     draws,
		ClockStep: step,
		Budget:    budget,
		Selection: selection,
	}, kind, free, asks.check, asks.holds, asks.run)
	s.state.Lock()
	if err != nil {
		return standing(checkStoppedVerdict(subject, label, err), &answered)
	}
	if x := answered.Result.Exploration(); x != nil {
		return standing(explorationVerdict(subject, x), &answered)
	}
	return standing(checkedVerdict(subject, label, answered.Result), &answered)
}

// checkProperties resolves the session's properties to the constraint or
// requirement each names: as the check engine evaluates each at every state
// about every object the invocation's behaviors perform on, holding when it holds
// of each; as the symbols the symbolic engine translates; and the scope they resolve in.
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
		properties = append(properties, runtime.CheckProperty{Name: name, Holds: func(ctx *runtime.Context, inv *runtime.Invocation) (bool, error) {
			performers := inv.Performers()
			if len(performers) == 0 {
				performers = []*runtime.Instance{nil}
			}
			for _, self := range performers {
				result, err := check(ctx, sym, scope, self)
				if err != nil && !errors.Is(err, runtime.ErrViolated) {
					return false, err
				}
				if !result.Holds {
					return false, nil
				}
			}
			return true, nil
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
func checkStoppedVerdict(subject, label string, err error) Verdict {
	var stopped *runtime.CheckStopped
	if errors.As(err, &stopped) {
		return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{
			fmt.Sprintf("? %s: incomplete: time (%d states, %d moves, depth %d)", label, stopped.States, stopped.Moves, stopped.MaxDepth),
			"  " + stopped.Cause.Error(),
		}}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{
			fmt.Sprintf("? %s: incomplete: time (the plan's clock ended before the search began)", label),
		}}
	}
	return unresolvedVerdict(subject, err.Error())
}

// checkedVerdict reports what the check engine found: a violation or a divergence
// fails the check, an exhaustive clean search holds, a bounded one is undecided. An
// answer with no search report is the symbolic engine's or an external one's claim.
func checkedVerdict(subject, label string, result analysis.Result) Verdict {
	checked := result.Check()
	if checked == nil {
		if result.Covered() || result.Engine == analysis.SMTEngineName {
			return decidedVerdict(subject, result)
		}
		return Verdict{Subject: subject, Status: VerdictUnresolved, Lines: []string{
			fmt.Sprintf("? %s could not be checked", label),
			"  " + result.Reason,
		}}
	}
	report := checked.Report
	v := Verdict{Subject: subject}
	switch {
	case result.Strength == analysis.NotCovered:
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? %s: %s", label, report.Status()), "  "+result.Reason)
	case report.Verdict == runtime.CheckViolation, report.Verdict == runtime.CheckDivergent:
		v.Status = VerdictFails
		v.Lines = append(v.Lines, fmt.Sprintf("✗ %s: %s", label, report.Status()))
	case report.Verdict == runtime.CheckExhaustive:
		v.Status = VerdictHolds
		v.Lines = append(v.Lines, fmt.Sprintf("✓ %s: %s", label, report.Status()))
	default:
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? %s: %s", label, report.Status()))
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

// decidedVerdict reports an answer carrying no check report, the symbolic engine's or an
// external one's: a violation or a sensitivity fails, a proof holds, a bounded or uncovered
// answer is undecided; the inputs, assumptions and witnesses it rests on follow, a
// sensitivity's two schedules as witness A and witness B.
func decidedVerdict(name string, result analysis.Result) Verdict {
	v := Verdict{Subject: name}
	switch {
	case !result.Covered():
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? Action %s: not covered", name), "  "+result.Reason)
	case result.Claim == analysis.ClaimViolated:
		v.Status = VerdictFails
		v.Lines = append(v.Lines, fmt.Sprintf("✗ Action %s: %s", name, result.Reason))
	case result.Claim == analysis.ClaimSensitive:
		v.Status = VerdictFails
		v.Lines = append(v.Lines, fmt.Sprintf("✗ Action %s: sensitive: %s", name, result.Reason))
	case result.Strength == analysis.Proved:
		v.Status = VerdictHolds
		v.Lines = append(v.Lines, fmt.Sprintf("✓ Action %s: %s", name, result.Claim))
	default:
		v.Status = VerdictUnresolved
		v.Lines = append(v.Lines, fmt.Sprintf("? Action %s: %s (%s)", name, result.Claim, result.Strength))
		if result.Question.Kind == analysis.Sensitive && result.Reason != "" {
			v.Lines = append(v.Lines, "  "+result.Reason)
		}
	}
	if inputs := inputLines(result.Inputs); inputs != "" {
		v.Lines = append(v.Lines, "  inputs: "+inputs)
	}
	if len(result.Assumptions) > 0 {
		v.Lines = append(v.Lines, "  assumed: "+strings.Join(result.Assumptions, ", "))
	}
	switch w := result.Witness; {
	case w != nil && result.Contrast != nil:
		v.Lines = append(v.Lines, "  witness A: "+witnessLine(w), "  witness B: "+witnessLine(result.Contrast))
	case w != nil:
		v.Lines = append(v.Lines, "  witness: "+witnessLine(w))
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

// witnessLine spells a witness: the file it was written to, else its draws and
// choices as one schedule, "no choices" for an empty one.
func witnessLine(w *analysis.Witness) string {
	if w.Written != "" {
		return w.Written
	}
	if len(w.Draws) == 0 && len(w.Choices) == 0 {
		return "no choices"
	}
	parts := make([]string, 0, len(w.Draws)+len(w.Choices))
	for _, d := range w.Draws {
		parts = append(parts, d.String())
	}
	for _, c := range w.Choices {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, "; ")
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
