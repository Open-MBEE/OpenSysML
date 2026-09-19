package repl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/objref"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
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
// every explored run creates its objects afresh, so an object of the session,
// named by id, is in no run.
type ExploredObjectError struct {
	Ref string
}

func (e *ExploredObjectError) Error() string {
	return fmt.Sprintf("%q names an object of this session, which an exploration does not run on: each explored run creates its own objects, so name a declaration to instantiate, or a path from one to an object it holds (Assembly::part.nested)", e.Ref)
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
// is held: declarations to instantiate and paths from them, the declarations the
// run was given objects of, a nested case's owner, the exhibits declared.
type freshPlan struct {
	refs     map[string]freshRef
	owners   map[string]freshRef
	given    []freshRef
	exhibits []exhibitEntry
	idx      *symbols.Index
}

// freshRef is a declaration an explored run instantiates and the path walked from its object
// to the one named, or the error the name resolved to; a swept one stands for held objects.
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

// planFresh resolves the object names an exploration's runs will ask for, and the
// declarations each run instantiates first: those the run was given objects of.
func (s *Session) planFresh(names ...string) *freshPlan {
	p := &freshPlan{
		refs:     make(map[string]freshRef, len(names)),
		owners:   make(map[string]freshRef),
		given:    s.givenRoots(),
		exhibits: collectExhibits(s.docScopes()),
		idx:      s.idx,
	}
	for _, text := range names {
		if _, done := p.refs[text]; !done {
			p.refs[text] = s.freshRef(text)
		}
	}
	return p
}

// givenRoots are the declarations the run was given objects of (see Session.given),
// in the order given, those the session still holds an object under.
func (s *Session) givenRoots() []freshRef {
	refs := make([]freshRef, 0, len(s.given))
	for _, fqn := range s.given {
		sym, resolved, err := s.lookupSymbol(fqn)
		if err != nil || resolved != fqn || s.instances[fqn] == nil {
			continue
		}
		refs = append(refs, freshRef{sym: sym, fqn: fqn, name: s.declaredName(fqn), label: s.declaredName(fqn)})
	}
	return refs
}

// failed is the error the first of names resolved to, nil when every one is planned.
func (p *freshPlan) failed(names []string) error {
	for _, text := range names {
		if ref, ok := p.refs[text]; ok && ref.err != nil {
			return ref.err
		}
	}
	return nil
}

// freshRef resolves one object name to the longest leading run of segments naming a
// declaration, the root each run instantiates, and the path walked from it; an id is refused.
func (s *Session) freshRef(text string) freshRef {
	ref, err := objref.Parse(text)
	if err != nil {
		return freshRef{err: err}
	}
	if ref.ID > 0 {
		return freshRef{err: &ExploredObjectError{Ref: text}}
	}
	head := objref.Head(ref.Segments)
	var unresolved error
	for i := head; i > 0; i-- {
		name := objref.JoinTyped(ref.Segments[:i])
		sym, fqn, err := s.lookupSymbol(name)
		if err != nil {
			var ambiguous *AmbiguousNameError
			if errors.As(err, &ambiguous) {
				return freshRef{err: err}
			}
			if i == head {
				unresolved = err
			}
			continue
		}
		// A member reached through a usage's type (pair::ground) is a feature of
		// the usage's object, walked there rather than instantiated on its own.
		if i > 1 && len(s.browseIndex().LookupQualified(objref.DeclaredRun(ref.Segments[:i]))) == 0 {
			continue
		}
		if objref.IsNamespace(sym) {
			if i == head && head < len(ref.Segments) {
				shown := declarationNotation(sym)
				return freshRef{err: &ObjectRefError{Ref: text, Detail: fmt.Sprintf("%s is a %s, not an object: its member is written %s::%s", shown, objref.NamespaceKind(sym), shown, ref.Segments[head].Text)}}
			}
			break
		}
		r := freshRef{sym: sym, fqn: fqn, name: s.declaredName(fqn), label: text, path: ref.Segments[i:]}
		r.err = s.checkFreshPath(name, sym, r.path)
		return r
	}
	if unresolved == nil {
		_, _, unresolved = s.lookupSymbol(objref.JoinTyped(ref.Segments[:max(head, 1)]))
	}
	return freshRef{err: unresolved}
}

// checkFreshPath reports what the declarations already tell about a path from an object of
// root: an unknown feature, an index on one value or off a fixed count, a feature holding data.
func (s *Session) checkFreshPath(label string, root *symbols.Symbol, path []objectSegment) error {
	if len(path) == 0 {
		return nil
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return err
	}
	walker := s.walker(ctx)
	typ := root
	for _, seg := range path {
		features := ctx.FeaturesOf(typ)
		feat := featureNamed(features, seg.Name)
		if feat == nil {
			return freshPathError(label, seg, "%s has no feature %q%s", label, seg.Name, walker.DeclaredFeatureHint(features))
		}
		shown := source.NameText(seg.Name)
		if ctx.Semantics().IsDataType(feat.Type) {
			return freshPathError(label, seg, "%s of %s holds a value, not an object", shown, label)
		}
		next := label + "." + shown
		switch count, fixed := fixedCount(feat.Multiplicity); {
		case feat.Scalar() && seg.Index > 0:
			return freshPathError(label, seg, "%s of %s holds one value and takes no index: write %s, not %s", shown, label, shown, seg.Text)
		case feat.Scalar():
		case seg.Index == 0 && fixed:
			return freshPathError(label, seg, "%s of %s holds %d objects: pick one by index, %s[1] to %s[%d]", shown, label, count, shown, shown, count)
		case seg.Index > count && fixed:
			return freshPathError(label, seg, "%s of %s holds %d objects, so %s names none (indexes run from 1 to %d)", shown, label, count, seg.Text, count)
		case seg.Index > count && count > 0:
			return freshPathError(label, seg, "%s of %s holds at most %d objects, so %s names none", shown, label, count, seg.Text)
		case seg.Index > 0:
			next = fmt.Sprintf("%s[%d]", next, seg.Index)
		}
		// A variation or a bound part holds an object of a type only its run tells.
		if typ = s.objectTypeOf(feat); typ == nil {
			return nil
		}
		label = next
	}
	return nil
}

// fixedCount is a multiplicity's upper bound when finite, and whether the lower
// bound meets it, so every object of the feature's owner holds exactly that many.
func fixedCount(m semantics.Range) (count int, fixed bool) {
	if !m.Upper.Known || m.Upper.Infinite {
		return 0, false
	}
	return int(m.Upper.Value), m.Lower.Known && !m.Lower.Infinite && m.Lower.Value == m.Upper.Value
}

func freshPathError(object string, seg objectSegment, format string, args ...any) *ObjectPathError {
	return &ObjectPathError{Object: object, Segment: seg.Text, Detail: fmt.Sprintf(format, args...)}
}

// planOwner resolves the object owning the case at fqn when the session holds one to the
// declaration it is held under and the path to it, which each run walks in its own object.
func (s *Session) planOwner(p *freshPlan, sym *symbols.Symbol, fqn string) {
	if !isNestedCase(sym) {
		return
	}
	segments := strings.Split(fqn, "::")
	if len(segments) < 2 {
		return
	}
	ownerFQN := strings.Join(segments[:len(segments)-1], "::")
	if held, _ := s.owningInstance(fqn); held == nil {
		return
	}
	_, key, rest := s.heldRoot(ownerFQN)
	root, rootFQN, err := s.lookupSymbol(key)
	if err != nil {
		return
	}
	p.owners[fqn] = freshRef{sym: root, fqn: rootFQN, name: s.declaredName(rootFQN), label: s.declaredName(ownerFQN), path: pathSegments(rest)}
}

// bind is the plan's objects in one run's context, with the session's objects
// created afresh from their declarations before any behavior starts: one per
// -instantiate, the last of a declaration the one its name denotes.
func (p *freshPlan) bind(ctx *runtime.Context) (*freshObjects, error) {
	f := &freshObjects{
		plan:  p,
		ctx:   ctx,
		roots: make(map[string]*runtime.Instance),
		made:  make(map[string]reachedObject),
		walker: objref.Walker{
			Runtime: ctx,
			Index:   p.idx,
			Format:  func(val runtime.Value) string { return formatValue(ctx, val) },
		},
	}
	for _, ref := range p.given {
		inst, err := f.instantiate(ref)
		if err != nil {
			return nil, err
		}
		if previous, ok := f.roots[ref.fqn]; ok {
			f.displaced = append(f.displaced, previous)
		}
		f.roots[ref.fqn] = inst
	}
	return f, nil
}

// freshObjects finds the objects an explored run names in the run's own context:
// each declaration the plan resolved is instantiated there once, by qualified name,
// and the object each path reaches from it is walked to once, by the path as written.
type freshObjects struct {
	plan  *freshPlan
	ctx   *runtime.Context
	roots map[string]*runtime.Instance
	// displaced are given objects a later -instantiate of their declaration
	// displaced from its name, still the run's and reached by id alone.
	displaced []*runtime.Instance
	made      map[string]reachedObject
	walker    objref.Walker
}

// reachedObject is an object a path reached, under the label the walk gave it.
type reachedObject struct {
	inst  *runtime.Instance
	label string
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
	if made, ok := f.made[text]; ok {
		return made.inst, made.label, nil
	}
	root, err := f.root(ref)
	if err != nil {
		return nil, "", err
	}
	inst, label, err := f.walker.Walk(root, ref.name, ref.path)
	if err != nil {
		return nil, "", err
	}
	f.made[text] = reachedObject{inst: inst, label: label}
	return inst, label, nil
}

// root is the object of ref's declaration in the run, instantiated once however
// many paths start from it, so siblings reached from one root share it.
func (f *freshObjects) root(ref freshRef) (*runtime.Instance, error) {
	if inst, ok := f.roots[ref.fqn]; ok {
		return inst, nil
	}
	inst, err := f.instantiate(ref)
	if err != nil {
		return nil, err
	}
	f.roots[ref.fqn] = inst
	return inst, nil
}

// instantiate creates an object of ref's declaration in the run.
func (f *freshObjects) instantiate(ref freshRef) (*runtime.Instance, error) {
	inst, err := f.ctx.Instantiate(ref.sym)
	if err != nil {
		return nil, fmt.Errorf("instantiation of %s failed: %w", ref.name, err)
	}
	return inst, nil
}

// owner is the run's object owning a nested usage when the plan found the session
// holding one: reached in the run as the prompt's run reaches the held one.
func (f *freshObjects) owner(fqn string) (*runtime.Instance, string) {
	ref, ok := f.plan.owners[fqn]
	if !ok {
		return nil, ""
	}
	root, err := f.root(ref)
	if err != nil {
		return nil, ""
	}
	inst, label, err := f.walker.Walk(root, ref.name, ref.path)
	if err != nil {
		return nil, ""
	}
	return inst, label
}

// exhibitors finds the run's objects exhibiting sym's machine.
func (f *freshObjects) exhibitors(sym *symbols.Symbol) []exhibitor {
	return f.runners(func(inst *runtime.Instance) []*runtime.ObjectBehavior { return inst.ExhibitedStatesOf(sym) })
}

// performers finds the run's objects performing sym's action.
func (f *freshObjects) performers(sym *symbols.Symbol) []exhibitor {
	return f.runners(func(inst *runtime.Instance) []*runtime.ObjectBehavior { return inst.PerformedActionsOf(sym) })
}

// runners finds the run's objects running the behaviors of picks out: those the
// run was given, in the order given and then the displaced ones by id, and what they hold.
func (f *freshObjects) runners(of func(*runtime.Instance) []*runtime.ObjectBehavior) []exhibitor {
	roots := make([]carrier, 0, len(f.plan.given))
	named := make(map[string]bool, len(f.roots))
	for _, ref := range f.plan.given {
		if inst, ok := f.roots[ref.fqn]; ok && !named[ref.fqn] {
			named[ref.fqn] = true
			roots = append(roots, carrier{name: ref.name, inst: inst})
		}
	}
	for _, inst := range f.displaced {
		roots = append(roots, carrier{name: fmt.Sprintf("#%d", inst.ID), inst: inst})
	}
	var found []exhibitor
	walkFrom(roots, materializedObjectsIn(f.ctx), func(cur carrier) bool {
		if behaviors := of(cur.inst); len(behaviors) > 0 {
			found = append(found, exhibitor{carrier: cur, machines: behaviors})
		}
		return true
	}, 0)
	return found
}

// freshExhibitorsError is exhibitorsError over an explored run's objects.
func freshExhibitorsError(name string, types []*symbols.Symbol, exhibitors []exhibitor) error {
	e := exhibitorsError(name, types, exhibitors).(*ExhibitorsError)
	e.Fresh = true
	return e
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

// exploredMachine resolves the state machine an exploration runs.
func (s *Session) exploredMachine(name string) (*symbols.Symbol, error) {
	sym, _, err := s.lookupSymbolOfKinds(name, symbols.SymbolStateDef, symbols.SymbolStateUsage)
	if err != nil {
		return nil, err
	}
	if !isMachineSymbol(sym) {
		return nil, fmt.Errorf("%q is not a state machine", name)
	}
	return sym, nil
}

// PerformersError reports an action `-action <action>` alone cannot attach to under a
// fresh-run engine: several of the run's objects perform it, or one performs it as
// several usages, so no one performance is meant.
type PerformersError struct {
	Action  string          // the action asked for, as the prompt prints names
	Objects []RelatedObject // the run's objects performing it, in walk order
	Usages  []string        // with one object, the performed usages running it, as the prompt prints names; "" for one unnamed
}

func (e *PerformersError) Error() string {
	labels := make([]string, len(e.Objects))
	for i, o := range e.Objects {
		labels[i] = fmt.Sprintf("#%d", o.ID)
		if o.Label != "" {
			labels[i] = fmt.Sprintf("#%d of %q", o.ID, o.Label)
		}
	}
	if len(e.Objects) == 1 {
		names := make([]string, len(e.Usages))
		for i, u := range e.Usages {
			names[i] = u
			if u == "" {
				names[i] = "an unnamed one"
			}
		}
		return fmt.Sprintf("object %s of the explored run performs %q as %d actions, so naming the definition attaches to none of them: name the performed usage instead — %s",
			labels[0], e.Action, len(e.Usages), strings.Join(names, " or "))
	}
	return fmt.Sprintf("%d objects of the explored run perform %q (%s), so naming the action alone attaches to none of them: name one as %s <Assembly::part>",
		len(e.Objects), e.Action, strings.Join(labels, ", "), e.Action)
}

// performersError is the PerformersError over the run's objects performing name's action.
func performersError(name string, performers []exhibitor) error {
	e := &PerformersError{Action: name}
	for _, p := range performers {
		ref := RelatedObject{ID: p.inst.ID}
		if !objref.IsID(p.name) {
			ref.Label = p.name
		}
		e.Objects = append(e.Objects, ref)
	}
	if len(performers) == 1 {
		for _, b := range performers[0].machines {
			usage := ""
			if member := b.Member(); member != nil && member.Name != "" {
				usage = declarationNotation(member)
			}
			e.Usages = append(e.Usages, usage)
		}
	}
	return e
}

// freshAction starts the action on an explored run's context: the performance the
// object performer names (or, named alone, the run's one object performing it) already
// runs of it, else a fresh run of the declaration on that object or on none.
func freshAction(objects *freshObjects, sym *symbols.Symbol, name string, performer []string) (exec *runtime.ActionExecutor, label string, err error) {
	ctx := objects.ctx
	var self *runtime.Instance
	switch {
	case len(performer) > 0:
		if self, _, err = objects.object(performer[0]); err != nil {
			return nil, "", err
		}
		label = performer[0]
	default:
		switch performers := objects.performers(sym); len(performers) {
		case 0:
		case 1:
			self, label = performers[0].inst, performers[0].name
		default:
			return nil, "", performersError(name, performers)
		}
	}
	if self != nil {
		switch performed := self.PerformedActionsOf(sym); len(performed) {
		case 0:
		case 1:
			exec = performed[0].Action
		default:
			return nil, "", performersError(name, []exhibitor{{carrier: carrier{name: label, inst: self}, machines: performed}})
		}
	}
	if exec == nil {
		if exec, err = ctx.CreateActionExecutorFor(sym, self); err != nil {
			return nil, "", fmt.Errorf("failed to create executor: %w", err)
		}
	}
	exec.SetTrace(ctx.Trace())
	return exec, label, nil
}

// freshMachine starts the machine on an explored run's context: the one an object of performer
// exhibits, or, named alone, the run's one object exhibiting it runs, else a run of the declaration.
func freshMachine(objects *freshObjects, sym *symbols.Symbol, name string, performer []string) (exec *runtime.StateExecutor, label string, err error) {
	ctx := objects.ctx
	var self *runtime.Instance
	switch {
	case len(performer) > 0:
		if self, _, err = objects.object(performer[0]); err != nil {
			return nil, "", err
		}
		label = performer[0]
	default:
		switch exhibitors := objects.exhibitors(sym); len(exhibitors) {
		case 0:
			if types := exhibitingTypes(ctx, objects.plan.exhibits, sym); len(types) > 0 {
				return nil, "", freshExhibitorsError(name, types, nil)
			}
		case 1:
			ex := exhibitors[0]
			if len(ex.machines) > 1 {
				return nil, "", ambiguousMachine(name, ex.inst, ex.name, ex.machines)
			}
			exec, label = ex.machines[0].State, ex.name
		default:
			return nil, "", freshExhibitorsError(name, nil, exhibitors)
		}
	}
	if self != nil {
		switch exhibited := self.ExhibitedStatesOf(sym); len(exhibited) {
		case 0:
		case 1:
			exec = exhibited[0].State
		default:
			return nil, "", ambiguousMachine(name, self, label, exhibited)
		}
	}
	if exec == nil {
		if exec, err = ctx.CreateStateExecutorFor(sym, self); err != nil {
			return nil, "", fmt.Errorf("failed to create executor: %w", err)
		}
	}
	exec.SetTrace(ctx.Trace())
	return exec, label, nil
}

// completedActionOutcome is the outcome of an action run that ended, completed or
// terminated; one that stopped short is an error, as the prompt's run reports it.
func completedActionOutcome(ctx *runtime.Context, exec *runtime.ActionExecutor, name string) (runtime.Outcome, error) {
	if state := exec.State(); !state.Ended() {
		return runtime.Outcome{}, fmt.Errorf("action %s stopped at %s at simulation time %s without completing",
			name, state, semantics.FormatReal(ctx.Clock().Now()))
	}
	return exec.Outcome(), nil
}

// exploreAction explores an action run to completion, on an object of what
// performer names when it names one; the check engine searches the same schedules
// beside the exploration where the selection consults it.
func (s *Session) exploreAction(name string, performer []string) Verdict {
	inv, unresolved := s.resolveInvocation([]Behavior{{Name: name, Performer: performer}}, nil, nil)
	if inv == nil {
		return unresolved[0]
	}
	policy, _ := s.exploring()
	return s.checkVerdict(inv, policy, analysis.Outcomes, inv.asks(s.checker), s.checkBudget(policy, analysis.Outcomes))
}

// exploreStateMachine explores a machine started and, when duration is given,
// its clock advanced by it, on an object of performer when one is named.
func (s *Session) exploreStateMachine(name string, duration *float64, performer []string) Verdict {
	sym, err := s.exploredMachine(name)
	if err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	plan := s.planFresh(performer...)
	if err := plan.failed(performer); err != nil {
		return unresolvedVerdict(name, err.Error())
	}
	return s.exploreVerdict(name, func(ctx *runtime.Context) (runtime.Outcome, error) {
		objects, err := plan.bind(ctx)
		if err != nil {
			return runtime.Outcome{}, err
		}
		exec, _, err := freshMachine(objects, sym, name, performer)
		if err != nil {
			return runtime.Outcome{}, err
		}
		if duration != nil {
			if _, err := ctx.Advance(*duration); err != nil {
				return runtime.Outcome{}, err
			}
		}
		inv := runtime.Invocation{States: []*runtime.StateExecutor{exec}}
		return inv.Outcome(), nil
	})
}

// exploreRunFor explores the behaviors named started on one clock and advanced
// by duration once, as RunFor runs them: one verdict, tabling the outcomes of the
// whole run. Several behaviors come to a joint outcome, each one's observables
// under its name; one behavior's outcome is its own.
func (s *Session) exploreRunFor(actions, states []Behavior, duration float64) []Verdict {
	inv, unresolved := s.resolveInvocation(actions, states, &duration)
	if inv == nil {
		return unresolved
	}
	return []Verdict{s.exploreVerdict(inv.subject(), inv.run)}
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
	if err := plan.failed(names); err != nil {
		return unresolvedVerdict(label, err.Error())
	}
	return s.exploreVerdict(label, func(ctx *runtime.Context) (runtime.Outcome, error) {
		objects, err := plan.bind(ctx)
		if err != nil {
			return runtime.Outcome{}, err
		}
		run, err := s.runAnalysisIn(s.direct(), ctx, inv, sym, fqn, objects)
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
