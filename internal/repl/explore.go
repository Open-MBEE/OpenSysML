package repl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ExploreAtPromptError reports `%schedule explore` at the prompt, whose
// debuggers step one run; exploration belongs to the CLI or the wire.
type ExploreAtPromptError struct {
	Policy runtime.SchedulePolicy
}

func (e *ExploreAtPromptError) Error() string {
	return fmt.Sprintf("%s replays a behavior from the start once per linearization, which %%action and %%state, stepping one run, cannot do: run `sysml -schedule %s -action <name>` (or -state, -analysis, -calc), or a request with schedule %q",
		e.Policy, e.Policy, e.Policy.String())
}

// ExploredObjectError reports an object reference an exploration cannot follow:
// every explored run creates its objects afresh, so it names declarations only.
type ExploredObjectError struct {
	Ref string
}

func (e *ExploredObjectError) Error() string {
	return fmt.Sprintf("%q names an object of this session, which an exploration does not run on: each explored run creates its own objects, so name the declaration to instantiate", e.Ref)
}

// VerdictOutcome is one distinct outcome an exploration reached: what the runs
// reaching it produced, how many did, and the choices of one that did.
type VerdictOutcome struct {
	Values []NamedValue
	// Error is what stopped the runs reaching this outcome, empty for one they completed.
	Error          string
	Linearizations int
	// Witness is one run's choice sequence, a choice per entry in run order.
	Witness []string
}

// VerdictExploration is how an exploration ended: whether every linearization
// within the budget was run, and which budget stopped it when not.
type VerdictExploration struct {
	Complete bool
	Runs     int
	// BudgetsHit names the budgets hit, `runs` before `depth`; none when complete.
	BudgetsHit []string
}

// exploring reports whether runs started from here on explore, and under what.
func (s *Session) exploring() (runtime.SchedulePolicy, bool) {
	_, ok := s.schedule.Exploration()
	return s.schedule, ok
}

// drivenSchedule is the policy the session's own context runs under: the one
// set, or the default while the one set explores, which drives contexts of its own.
func (s *Session) drivenSchedule() runtime.SchedulePolicy {
	if _, ok := s.schedule.Exploration(); ok {
		return runtime.DefaultSchedulePolicy
	}
	return s.schedule
}

// refuseExplore is the error a prompt debugger answers while the policy explores.
func (s *Session) refuseExplore() error {
	if policy, ok := s.exploring(); ok {
		return &ExploreAtPromptError{Policy: policy}
	}
	return nil
}

// exploreVerdict explores one behavior, run performing it on a context of its
// own, and tables every distinct outcome with the witness run's trace.
func (s *Session) exploreVerdict(subject string, run func(*runtime.Context) (runtime.Outcome, error)) Verdict {
	policy, _ := s.exploring()
	model, resolver, err := s.semanticModel()
	if err != nil {
		return unresolvedVerdict(subject, fmt.Errorf("%w: %w", errRuntimeInit, err).Error())
	}
	var traces [][]string
	fresh := func() (*runtime.Context, error) {
		ctx, err := s.newRuntimeOver(model, resolver)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
		}
		if s.trace != nil {
			ctx.SetTrace(runtime.NewTraceRecorder())
		}
		return ctx, nil
	}
	traced := func(ctx *runtime.Context) (runtime.Outcome, error) {
		outcome, err := run(ctx)
		if rec := ctx.Trace(); rec != nil {
			traces = append(traces, rec.Entries())
		}
		return outcome, err
	}
	x, err := runtime.Explore(policy, fresh, traced)
	if err != nil {
		return unresolvedVerdict(subject, err.Error())
	}
	return explorationVerdict(subject, x, traces)
}

// explorationVerdict tables one row per distinct outcome, then how it ended;
// a failed run or a budget hit leaves it unresolved.
func explorationVerdict(subject string, x *runtime.Exploration, traces [][]string) Verdict {
	status := VerdictHolds
	if !x.Complete() {
		status = VerdictUnresolved
	}
	outcomes := make([]VerdictOutcome, 0, len(x.Outcomes))
	cells := [][]string{{"outcome", "linearizations", "witness"}}
	for _, o := range x.Outcomes {
		vo := VerdictOutcome{Linearizations: o.Linearizations}
		if o.Outcome.Err != nil {
			vo.Error = o.Outcome.Err.Error()
			status = VerdictUnresolved
		} else {
			vo.Values = outcomeValues(o.Outcome)
		}
		for _, c := range o.Witness {
			vo.Witness = append(vo.Witness, c.String())
		}
		outcomes = append(outcomes, vo)
		cells = append(cells, []string{
			oneLine(o.Outcome.String()),
			strconv.Itoa(o.Linearizations),
			runtime.FormatChoices(o.Witness),
		})
	}
	mark := "✓"
	if status != VerdictHolds {
		mark = "?"
	}
	lines := []string{fmt.Sprintf("%s explored %s: %s", mark, subject, countOf(len(x.Outcomes), "outcome", "outcomes"))}
	lines = append(lines, tableLines(cells)...)
	lines = append(lines, x.Status())
	for i, o := range x.Outcomes {
		if o.WitnessRun > len(traces) || len(traces[o.WitnessRun-1]) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("trace of outcome %d's witness (run %d):", i+1, o.WitnessRun))
		for _, entry := range traces[o.WitnessRun-1] {
			lines = append(lines, tracePrefix+entry)
		}
	}
	return Verdict{
		Subject:  subject,
		Status:   status,
		Lines:    lines,
		Outcomes: outcomes,
		Exploration: &VerdictExploration{
			Complete:   x.Complete(),
			Runs:       x.Runs,
			BudgetsHit: x.BudgetsHit,
		},
	}
}

// outcomeValues lists an outcome's observables as the outcome spells them: a
// machine's final state and visits ahead of the values it holds, in name order.
func outcomeValues(o runtime.Outcome) []NamedValue {
	var values []NamedValue
	if o.FinalState != "" {
		values = append(values, NamedValue{Name: "finalState", Value: o.FinalState})
	}
	if len(o.StateVisits) > 0 {
		values = append(values, NamedValue{Name: "stateVisits", Value: strings.Join(o.StateVisits, ", ")})
	}
	for _, out := range o.RenderedOutputs() {
		values = append(values, NamedValue{Name: out.Name, Value: out.Text})
	}
	return values
}

// tableLines pads cells into columns under the first row's titles.
func tableLines(cells [][]string) []string {
	widths := make([]int, len(cells[0]))
	for _, row := range cells {
		for i, cell := range row {
			if n := len([]rune(cell)); n > widths[i] {
				widths[i] = n
			}
		}
	}
	lines := []string{renderSweepRow(cells[0], widths, " | "), sweepRule(widths)}
	for _, row := range cells[1:] {
		lines = append(lines, renderSweepRow(row, widths, " | "))
	}
	return lines
}

// freshObjects finds the objects an explored run names in the run's own context:
// a declaration is instantiated there, and an object of the session is refused.
type freshObjects struct {
	s   *Session
	ctx *runtime.Context
}

// heldObjects finds the objects a run at the prompt names: the session's own.
type heldObjects struct {
	s *Session
}

// runObjects is where a run finds the objects it names as subject or performer,
// and the object owning a usage nested in a type.
type runObjects interface {
	object(text string) (*runtime.Instance, string, error)
	owner(fqn string) (*runtime.Instance, string)
}

func (h heldObjects) object(text string) (*runtime.Instance, string, error) {
	return h.s.resolveObject(text)
}

func (h heldObjects) owner(fqn string) (*runtime.Instance, string) {
	return h.s.owningInstance(fqn)
}

func (f freshObjects) object(text string) (*runtime.Instance, string, error) {
	ref, err := parseObjectRef(text)
	if err != nil {
		return nil, "", err
	}
	if ref.id > 0 {
		return nil, "", &ExploredObjectError{Ref: text}
	}
	for _, seg := range ref.segments {
		if seg.index > 0 || seg.dotted {
			return nil, "", &ExploredObjectError{Ref: text}
		}
	}
	sym, fqn, err := f.s.lookupSymbol(joinTyped(ref.segments))
	if err != nil {
		return nil, "", err
	}
	inst, err := f.ctx.Instantiate(sym)
	if err != nil {
		return nil, "", fmt.Errorf("instantiation of %s failed: %w", f.s.declaredName(fqn), err)
	}
	return inst, f.s.declaredName(fqn), nil
}

// owner instantiates the type owning a nested usage when the session holds an
// object of it, as the prompt's run would.
func (f freshObjects) owner(fqn string) (*runtime.Instance, string) {
	if held, _ := f.s.owningInstance(fqn); held == nil {
		return nil, ""
	}
	segments := strings.Split(fqn, "::")
	sym, ownerFQN, err := f.s.lookupSymbol(strings.Join(segments[:len(segments)-1], "::"))
	if err != nil {
		return nil, ""
	}
	inst, err := f.ctx.Instantiate(sym)
	if err != nil {
		return nil, ""
	}
	return inst, f.s.declaredName(ownerFQN)
}

// exploreAction explores an action run to completion, on an object of what
// performer names when it names one.
func (s *Session) exploreAction(name string, performer []string) Verdict {
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolActionDef, symbols.SymbolActionUsage)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	if sym.Kind != symbols.SymbolActionDef && sym.Kind != symbols.SymbolActionUsage {
		return unresolvedVerdict(name, fmt.Sprintf("%q is not an action", name))
	}
	return s.exploreVerdict(name, func(ctx *runtime.Context) (runtime.Outcome, error) {
		var self *runtime.Instance
		if len(performer) > 0 {
			var err error
			if self, _, err = (freshObjects{s, ctx}).object(performer[0]); err != nil {
				return runtime.Outcome{}, err
			}
		}
		exec, err := ctx.CreateActionExecutorFor(sym, self)
		if err != nil {
			return runtime.Outcome{}, fmt.Errorf("failed to create executor: %w", err)
		}
		exec.SetTrace(ctx.Trace())
		if err := exec.RunToCompletion(); err != nil {
			return runtime.Outcome{}, err
		}
		if state := exec.State(); state != runtime.StateCompleted {
			return runtime.Outcome{}, fmt.Errorf("action %s stopped at %s without completing", name, state)
		}
		return ctx.ActionOutcome(exec.Results()), nil
	})
}

// exploreStateMachine explores a machine started and, when duration is given,
// advanced, on an object of performer when one is named.
func (s *Session) exploreStateMachine(name string, duration *float64, performer []string) Verdict {
	if looksLikeObjectPath(name) {
		return unresolvedVerdict(name, (&ExploredObjectError{Ref: name}).Error())
	}
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolStateDef, symbols.SymbolStateUsage)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	if !isMachineSymbol(sym) {
		return unresolvedVerdict(name, fmt.Sprintf("%q is not a state machine", name))
	}
	return s.exploreVerdict(name, func(ctx *runtime.Context) (runtime.Outcome, error) {
		if len(performer) == 0 {
			if types := s.exhibitingTypes(ctx, sym); len(types) > 0 {
				return runtime.Outcome{}, s.exhibitorsError(name, types, nil)
			}
		}
		var (
			self  *runtime.Instance
			label string
		)
		if len(performer) > 0 {
			var err error
			if self, label, err = (freshObjects{s, ctx}).object(performer[0]); err != nil {
				return runtime.Outcome{}, err
			}
		}
		var exec *runtime.StateExecutor
		if self != nil {
			switch exhibited := self.ExhibitedStatesOf(sym); len(exhibited) {
			case 0:
			case 1:
				exec = exhibited[0].State
			default:
				return runtime.Outcome{}, ambiguousMachine(name, self, label, exhibited)
			}
		}
		if exec == nil {
			var err error
			if exec, err = ctx.CreateStateExecutorFor(sym, self); err != nil {
				return runtime.Outcome{}, fmt.Errorf("failed to create executor: %w", err)
			}
		}
		exec.SetTrace(ctx.Trace())
		if duration != nil {
			ss := &stateSession{name: name, symbol: sym, executor: exec, rtCtx: ctx, now: exec.CurrentTime()}
			if _, err := s.advanceSession(ss, *duration); err != nil {
				return runtime.Outcome{}, err
			}
		}
		return exec.Outcome(), nil
	})
}

// exploreCalc explores a calculation, which has choice points when it performs
// actions; one that does not explores in a single run.
func (s *Session) exploreCalc(invocation string) Verdict {
	name, argText := splitCalcArgs(invocation)
	sym, err := s.calcSymbol(name)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	return s.exploreVerdict(name, func(ctx *runtime.Context) (runtime.Outcome, error) {
		_, results, err := s.evalCalcIn(ctx, sym, name, argText)
		if err != nil {
			return runtime.Outcome{}, err
		}
		outputs := make(map[string]runtime.Value, len(results))
		for _, out := range results {
			outputs[out.Name] = out.Value
		}
		return ctx.ActionOutcome(outputs), nil
	})
}

// exploreAnalysis explores an analysis case: its outputs and verdicts are the
// outcome, as the prompt's run reports them.
func (s *Session) exploreAnalysis(inv analysisInvocation) Verdict {
	label := inv.name
	if inv.argText != "" {
		label += "(" + strings.TrimSpace(inv.argText) + ")"
	}
	sym, fqn, err := s.analysisSymbol(inv)
	if err != nil {
		return unresolvedVerdict(label, err.Error())
	}
	return s.exploreVerdict(label, func(ctx *runtime.Context) (runtime.Outcome, error) {
		run, err := s.runAnalysisIn(ctx, inv, sym, fqn, freshObjects{s, ctx})
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.VerifiedOutcome(run.result, run.verdicts), nil
	})
}

// nestedCaseOwner is the object a case usage nested in a type is performed on:
// one of that type, found where objects finds them.
func nestedCaseOwner(sym *symbols.Symbol, fqn string, objects runObjects) *runtime.Instance {
	if usage, ok := sym.Decl.(*ast.Usage); ok &&
		(usage.Kind == ast.UsageAnalysisCase || usage.Kind == ast.UsageVerificationCase) {
		self, _ := objects.owner(fqn)
		return self
	}
	return nil
}
