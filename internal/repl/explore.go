package repl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
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

// UnplannedObjectError reports an object name an explored run asked for that its
// plan did not resolve: runs name only what was planned while the session was held.
type UnplannedObjectError struct {
	Ref string
}

func (e *UnplannedObjectError) Error() string {
	return fmt.Sprintf("%q was not resolved before the exploration ran", e.Ref)
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

// exploring reports whether runs started from here on explore, and under what:
// the schedule set when it explores, else the default when the engine is explore.
func (s *Session) exploring() (runtime.SchedulePolicy, bool) {
	return analysis.Explores(s.engine, s.schedule)
}

// drivenSchedule is the policy the session's own context runs under: the one
// set, or the default while runs explore, which drives contexts of its own.
func (s *Session) drivenSchedule() runtime.SchedulePolicy {
	if _, ok := s.exploring(); ok {
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
// own, and tables every distinct outcome with the witness run's trace. The
// session's state is released while the plan runs, and its runs go concurrently on the
// session's jobs, so run may read only what the command lock keeps still: declarations
// and settings, never the session's objects. What a run names is resolved into a
// freshPlan before the release.
func (s *Session) exploreVerdict(subject string, run func(*runtime.Context) (runtime.Outcome, error)) Verdict {
	policy, _ := s.exploring()
	model := s.freshModel()
	selection := s.engine
	s.state.Unlock()
	plan, err := s.explore(subject, policy, selection, model, run)
	s.state.Lock()
	if err != nil {
		return standing(unresolvedVerdict(subject, err.Error()), &plan)
	}
	return standing(explorationVerdict(subject, plan.Result.Exploration()), &plan)
}

// freshModel is the model a plan builds its runs' contexts over while the session's state
// is released: a worker's own model-derived part over the index and name table warmed here,
// and a context of each run's own on it, tracing when the session traces.
func (s *Session) freshModel() *analysis.Model {
	s.browseIndex()
	s.nameTable()
	return &analysis.Model{
		Semantics: func() (*runtime.Model, error) {
			model, err := s.runtimeModel()
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
			}
			return model, nil
		},
		Fresh: func(w *analysis.Worker) (*runtime.Context, error) {
			ctx, err := s.newRuntimeOver(w.Model)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errRuntimeInit, err)
			}
			if s.trace != nil {
				ctx.SetTrace(runtime.NewTraceRecorder())
			}
			return ctx, nil
		},
	}
}

// recordedTrace is the trace a run's context recorded, none when it kept none.
func recordedTrace(ctx *runtime.Context) []string {
	if ctx == nil {
		return nil
	}
	rec := ctx.Trace()
	if rec == nil {
		return nil
	}
	return rec.Entries()
}

// witnessTrace is the trace the witness run of an outcome recorded, none when it kept none.
func witnessTrace(o runtime.ExploredOutcome) []string {
	return recordedTrace(o.Outcome.Context())
}

// explorationVerdict tables one row per distinct outcome, then how it ended;
// a failed run or a budget hit leaves it unresolved.
func explorationVerdict(subject string, x *runtime.Exploration) Verdict {
	if x == nil {
		return unresolvedVerdict(subject, "exploration of "+subject+" reached no outcome")
	}
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
		trace := witnessTrace(o)
		if len(trace) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("trace of outcome %d's witness (run %d):", i+1, o.WitnessRun))
		for _, entry := range trace {
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

// freshPlan is what an exploration's runs name, resolved while the session's state
// is held: declarations to instantiate, a nested case's held owner, the exhibits declared.
type freshPlan struct {
	refs     map[string]freshRef
	owners   map[string]freshRef
	exhibits []exhibitEntry
}

// freshRef is a declaration an explored run instantiates, or the error its name resolved
// to. A swept one stands for held objects: root's, and the one reached along path from it.
type freshRef struct {
	sym  *symbols.Symbol
	fqn  string
	name string
	// label is the reference as written, for reporting; name is the declared root.
	label      string
	path       []objectSegment
	root, held int64
	// imaged takes the object from an image of the held graph, not its declaration:
	// it is named by identity, or no longer as its declaration made it.
	imaged bool
	err    error
}

// planFresh resolves the object names an exploration's runs will ask for.
func (s *Session) planFresh(names ...string) *freshPlan {
	p := &freshPlan{
		refs:     make(map[string]freshRef, len(names)),
		owners:   make(map[string]freshRef),
		exhibits: collectExhibits(s.docScopes()),
	}
	for _, text := range names {
		if _, done := p.refs[text]; !done {
			p.refs[text] = s.freshRef(text)
		}
	}
	return p
}

// freshRef resolves one object name to the declaration an explored run
// instantiates; an object of the session is refused.
func (s *Session) freshRef(text string) freshRef {
	ref, err := parseObjectRef(text)
	if err != nil {
		return freshRef{err: err}
	}
	if ref.id > 0 {
		return freshRef{err: &ExploredObjectError{Ref: text}}
	}
	for _, seg := range ref.segments {
		if seg.index > 0 || seg.dotted {
			return freshRef{err: &ExploredObjectError{Ref: text}}
		}
	}
	sym, fqn, err := s.lookupSymbol(joinTyped(ref.segments))
	if err != nil {
		return freshRef{err: err}
	}
	return freshRef{sym: sym, fqn: fqn, name: s.declaredName(fqn)}
}

// planOwner resolves the type owning the case at fqn when the session holds an
// object of it, so each run instantiates one as the prompt's run performs on the held one.
func (s *Session) planOwner(p *freshPlan, sym *symbols.Symbol, fqn string) {
	if !isNestedCase(sym) {
		return
	}
	if held, _ := s.owningInstance(fqn); held == nil {
		return
	}
	segments := strings.Split(fqn, "::")
	owner, ownerFQN, err := s.lookupSymbol(strings.Join(segments[:len(segments)-1], "::"))
	if err != nil {
		return
	}
	p.owners[fqn] = freshRef{sym: owner, fqn: ownerFQN, name: s.declaredName(ownerFQN)}
}

// bind is the plan's objects in one run's context.
func (p *freshPlan) bind(ctx *runtime.Context) *freshObjects {
	return &freshObjects{plan: p, ctx: ctx, made: make(map[string]*runtime.Instance)}
}

// freshObjects finds the objects an explored run names in the run's own context:
// each declaration the plan resolved is instantiated there once.
type freshObjects struct {
	plan *freshPlan
	ctx  *runtime.Context
	made map[string]*runtime.Instance
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

func (f *freshObjects) object(text string) (*runtime.Instance, string, error) {
	ref, planned := f.plan.refs[text]
	if !planned {
		return nil, "", &UnplannedObjectError{Ref: text}
	}
	if ref.err != nil {
		return nil, "", ref.err
	}
	if inst, ok := f.made[ref.fqn]; ok {
		return inst, ref.name, nil
	}
	inst, err := f.ctx.Instantiate(ref.sym)
	if err != nil {
		return nil, "", fmt.Errorf("instantiation of %s failed: %w", ref.name, err)
	}
	f.made[ref.fqn] = inst
	return inst, ref.name, nil
}

// owner instantiates the type owning a nested usage when the plan found the
// session holding an object of it, as the prompt's run would perform on that one.
func (f *freshObjects) owner(fqn string) (*runtime.Instance, string) {
	ref, ok := f.plan.owners[fqn]
	if !ok {
		return nil, ""
	}
	inst, err := f.ctx.Instantiate(ref.sym)
	if err != nil {
		return nil, ""
	}
	return inst, ref.name
}

// exploredAction resolves the action an exploration runs.
func (s *Session) exploredAction(name string) (*symbols.Symbol, error) {
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolActionDef, symbols.SymbolActionUsage)
	if err != nil {
		return nil, err
	}
	if sym.Kind != symbols.SymbolActionDef && sym.Kind != symbols.SymbolActionUsage {
		return nil, fmt.Errorf("%q is not an action", name)
	}
	return sym, nil
}

// exploredMachine resolves the state machine an exploration runs, which names a
// declaration: an object of the session is not run on.
func (s *Session) exploredMachine(name string) (*symbols.Symbol, error) {
	if looksLikeObjectPath(name) {
		return nil, &ExploredObjectError{Ref: name}
	}
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolStateDef, symbols.SymbolStateUsage)
	if err != nil {
		return nil, err
	}
	if !isMachineSymbol(sym) {
		return nil, fmt.Errorf("%q is not a state machine", name)
	}
	return sym, nil
}

// freshAction starts the action on an explored run's context, on an object of
// what performer names when it names one.
func freshAction(objects *freshObjects, sym *symbols.Symbol, performer []string) (*runtime.ActionExecutor, error) {
	ctx := objects.ctx
	var self *runtime.Instance
	if len(performer) > 0 {
		var err error
		if self, _, err = objects.object(performer[0]); err != nil {
			return nil, err
		}
	}
	exec, err := ctx.CreateActionExecutorFor(sym, self)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}
	exec.SetTrace(ctx.Trace())
	return exec, nil
}

// freshMachine starts the machine on an explored run's context: the one an
// object of performer exhibits when it names one, else a run of the declaration.
func freshMachine(objects *freshObjects, sym *symbols.Symbol, name string, performer []string) (*runtime.StateExecutor, error) {
	ctx := objects.ctx
	if len(performer) == 0 {
		if types := exhibitingTypes(ctx, objects.plan.exhibits, sym); len(types) > 0 {
			return nil, exhibitorsError(name, types, nil)
		}
	}
	var (
		self  *runtime.Instance
		label string
	)
	if len(performer) > 0 {
		var err error
		if self, label, err = objects.object(performer[0]); err != nil {
			return nil, err
		}
	}
	var exec *runtime.StateExecutor
	if self != nil {
		switch exhibited := self.ExhibitedStatesOf(sym); len(exhibited) {
		case 0:
		case 1:
			exec = exhibited[0].State
		default:
			return nil, ambiguousMachine(name, self, label, exhibited)
		}
	}
	if exec == nil {
		var err error
		if exec, err = ctx.CreateStateExecutorFor(sym, self); err != nil {
			return nil, fmt.Errorf("failed to create executor: %w", err)
		}
	}
	exec.SetTrace(ctx.Trace())
	return exec, nil
}

// completedActionOutcome is the outcome of an action run that reached its end;
// one that stopped short is an error, as the prompt's run reports it.
func completedActionOutcome(ctx *runtime.Context, exec *runtime.ActionExecutor, name string) (runtime.Outcome, error) {
	if state := exec.State(); state != runtime.StateCompleted {
		return runtime.Outcome{}, fmt.Errorf("action %s stopped at %s at simulation time %s without completing",
			name, state, semantics.FormatReal(ctx.Clock().Now()))
	}
	return ctx.ActionOutcome(exec.Results()), nil
}

// exploreAction explores an action run to completion, on an object of what
// performer names when it names one; the check engine searches the same schedules
// beside the exploration where the selection consults it.
func (s *Session) exploreAction(name string, performer []string) Verdict {
	ask, run, err := s.checkAsk(name, performer)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	policy, _ := s.exploring()
	return s.checkVerdict(name, policy, analysis.Outcomes, ask, run, s.checkBudget(policy, analysis.Outcomes))
}

// exploreStateMachine explores a machine started and, when duration is given,
// its clock advanced by it, on an object of performer when one is named.
func (s *Session) exploreStateMachine(name string, duration *float64, performer []string) Verdict {
	sym, err := s.exploredMachine(name)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	plan := s.planFresh(performer...)
	return s.exploreVerdict(name, func(ctx *runtime.Context) (runtime.Outcome, error) {
		exec, err := freshMachine(plan.bind(ctx), sym, name, performer)
		if err != nil {
			return runtime.Outcome{}, err
		}
		if duration != nil {
			if _, err := ctx.Advance(*duration); err != nil {
				return runtime.Outcome{}, err
			}
		}
		return exec.Outcome(), nil
	})
}

// exploreRunFor explores the behaviors named started on one clock and advanced
// by duration once, as RunFor runs them: one verdict, tabling the outcomes of the
// whole run. Several behaviors come to a joint outcome, each one's observables
// under its name; one behavior's outcome is its own.
func (s *Session) exploreRunFor(actions, states []Behavior, duration float64) []Verdict {
	type explored struct {
		Behavior
		sym    *symbols.Symbol
		action bool
	}
	var (
		runs       []explored
		unresolved []Verdict
		names      []string
		performers []string
	)
	for _, b := range actions {
		sym, err := s.exploredAction(b.Name)
		if err != nil {
			unresolved = append(unresolved, unresolvedVerdict(b.Name, err.Error()))
			continue
		}
		runs = append(runs, explored{Behavior: b, sym: sym, action: true})
		names = append(names, b.Name)
		performers = append(performers, b.Performer...)
	}
	for _, b := range states {
		sym, err := s.exploredMachine(b.Name)
		if err != nil {
			unresolved = append(unresolved, unresolvedVerdict(b.Name, err.Error()))
			continue
		}
		runs = append(runs, explored{Behavior: b, sym: sym})
		names = append(names, b.Name)
		performers = append(performers, b.Performer...)
	}
	if len(unresolved) > 0 || len(runs) == 0 {
		return unresolved
	}
	subject := strings.Join(names, ", ")
	plan := s.planFresh(performers...)
	verdict := s.exploreVerdict(subject, func(ctx *runtime.Context) (runtime.Outcome, error) {
		objects := plan.bind(ctx)
		actionExecs := make(map[int]*runtime.ActionExecutor)
		stateExecs := make(map[int]*runtime.StateExecutor)
		for i, r := range runs {
			if r.action {
				exec, err := freshAction(objects, r.sym, r.Performer)
				if err != nil {
					return runtime.Outcome{}, err
				}
				actionExecs[i] = exec
				continue
			}
			exec, err := freshMachine(objects, r.sym, r.Name, r.Performer)
			if err != nil {
				return runtime.Outcome{}, err
			}
			stateExecs[i] = exec
		}
		if _, err := ctx.Advance(duration); err != nil {
			return runtime.Outcome{}, err
		}
		outcomes := make([]runtime.Outcome, len(runs))
		for i, r := range runs {
			if exec, ok := actionExecs[i]; ok {
				outcome, err := completedActionOutcome(ctx, exec, r.Name)
				if err != nil {
					return runtime.Outcome{}, err
				}
				outcomes[i] = outcome
				continue
			}
			outcomes[i] = stateExecs[i].Outcome()
		}
		if len(outcomes) == 1 {
			return outcomes[0], nil
		}
		return ctx.JointOutcome(names, outcomes), nil
	})
	return []Verdict{verdict}
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
		_, results, _, err := s.evalCalcIn(s.direct(), ctx, sym, name, argText)
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
	var names []string
	if inv.object != "" {
		names = append(names, inv.object)
	}
	plan := s.planFresh(names...)
	s.planOwner(plan, sym, fqn)
	return s.exploreVerdict(label, func(ctx *runtime.Context) (runtime.Outcome, error) {
		run, err := s.runAnalysisIn(s.direct(), ctx, inv, sym, fqn, plan.bind(ctx))
		if err != nil {
			return runtime.Outcome{}, err
		}
		return ctx.VerifiedOutcome(run.result, run.verdicts), nil
	})
}

// nestedCaseOwner is the object a case usage nested in a type is performed on:
// one of that type, found where objects finds them.
func nestedCaseOwner(sym *symbols.Symbol, fqn string, objects runObjects) *runtime.Instance {
	if isNestedCase(sym) {
		self, _ := objects.owner(fqn)
		return self
	}
	return nil
}

// isNestedCase reports whether sym is a case usage, which is performed on an
// object of the type owning it.
func isNestedCase(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && (usage.Kind == ast.UsageAnalysisCase || usage.Kind == ast.UsageVerificationCase)
}
