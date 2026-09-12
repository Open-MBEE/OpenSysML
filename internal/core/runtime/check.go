package runtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The model checker: a depth-first search over the schedules of one action,
// one token advancing one node per move, backtracking through snapshots. It
// finds the properties' violations, the deadlocks and the typed failures some
// schedule reaches, and the features whose final value the schedule decides
// (docs/internals/design/bounded-model-checking.md).

// CheckBudget bounds a check: the moves one schedule may make and the distinct
// states the search may visit; 0 leaves either unbounded.
type CheckBudget struct {
	Depth  int
	States int
}

// CheckOptions selects what a check reports and how it searches.
type CheckOptions struct {
	// Diverge names the features whose divergence is reported; none names the
	// action's own attributes and, when an object performs it, its features as `this.<name>`.
	Diverge []string
	// Reduce explores one representative of each class of equivalent schedules;
	// off, every schedule. It is off only to test that the reduction loses nothing.
	Reduce bool
}

// CheckProperty is a property evaluated at every stable state of a check: a
// requirement or constraint, false at a state being a violation. Holds is asked
// in the check's context of the executor the check runs.
type CheckProperty struct {
	Name  string
	Holds func(*Context, *ActionExecutor) (bool, error)
}

// ActionStarter builds and starts the action executor a check runs, in the
// context given; a replay starts the same executor the same way.
type ActionStarter func(*Context) (*ActionExecutor, error)

// ViolationKind classifies what a schedule reached.
type ViolationKind int

const (
	// ViolationProperty is a property evaluating false at a reached state.
	ViolationProperty ViolationKind = iota
	// ViolationDeadlock is a state with no move where the action is not complete.
	ViolationDeadlock
	// ViolationFailure is a typed runtime error a move raised.
	ViolationFailure
)

func (k ViolationKind) String() string {
	switch k {
	case ViolationProperty:
		return "property"
	case ViolationDeadlock:
		return "deadlock"
	case ViolationFailure:
		return "failure"
	}
	return fmt.Sprintf("ViolationKind(%d)", int(k))
}

// Violation is one violation a schedule reached, with the schedule as its witness.
type Violation struct {
	Kind ViolationKind
	// Name is the property violated, empty for a deadlock or a failure.
	Name string
	// Err is the deadlock or the failure as the executor reported it, nil for a property.
	Err error
	// Depth is how many moves the witness makes.
	Depth   int
	Witness Witness
}

// String renders the violation for a report.
func (v Violation) String() string {
	switch v.Kind {
	case ViolationProperty:
		return fmt.Sprintf("%s is false after %d moves", v.Name, v.Depth)
	case ViolationDeadlock:
		return fmt.Sprintf("deadlock after %d moves: %v", v.Depth, v.Err)
	}
	return fmt.Sprintf("failure after %d moves: %v", v.Depth, v.Err)
}

// Witness is one schedule: the choices that fix it, as a replay follows them,
// and the trace the run leaves, as the trace recorder writes it.
type Witness struct {
	Choices []ChoiceTaken
	Trace   string
	// Fails is the deadlock or failure the schedule ends in, as the executor
	// spells it; empty for a state the run goes on from.
	Fails string
}

// DivergentValue is one final value of a divergent feature and a schedule reaching it.
type DivergentValue struct {
	Value   string
	Witness Witness
}

// Divergence is a feature whose final value the schedule decides: every value
// some complete schedule leaves it with, in canonical order.
type Divergence struct {
	Feature string
	Values  []DivergentValue
}

// String spells the divergence as `x ends as 1 or 2`.
func (d Divergence) String() string {
	values := make([]string, len(d.Values))
	for i, v := range d.Values {
		values[i] = v.Value
	}
	return d.Feature + " ends as " + strings.Join(values, " or ")
}

// CheckFinal is one distinct outcome of the complete schedules and a schedule
// reaching it; Values spells every feature divergence is reported over, the
// performing object's as `this.<name>`, and Outcome spells the action's outputs
// with the object's values after them.
type CheckFinal struct {
	Outcome string
	Values  map[string]string
	Witness Witness
	// identity is the outcome's identity with the object's values, every name quoted.
	identity string
}

// CheckVerdict is how a check ended.
type CheckVerdict int

const (
	// CheckExhaustive found no violation and every schedule, up to equivalence, was searched.
	CheckExhaustive CheckVerdict = iota
	// CheckWithinBounds found no violation among the schedules the bounds let it search.
	CheckWithinBounds
	// CheckViolation found a violation.
	CheckViolation
	// CheckDivergent found no violation and a feature whose final value the schedule decides.
	CheckDivergent
)

func (v CheckVerdict) String() string {
	switch v {
	case CheckExhaustive:
		return "no violation, exhaustive"
	case CheckWithinBounds:
		return "no violation within bounds"
	case CheckViolation:
		return "violation"
	case CheckDivergent:
		return "divergent"
	}
	return fmt.Sprintf("CheckVerdict(%d)", int(v))
}

// CheckReport is what a check of the schedules of an action found.
type CheckReport struct {
	Verdict CheckVerdict
	// States counts the distinct states visited; Moves the moves made; MaxDepth
	// the longest schedule searched.
	States   int
	Moves    int
	MaxDepth int
	// BoundsHit names the bounds the search ran into: `depth`, `states`, and the
	// executor's budgets by name (ExecutorBounds); none when exhaustive.
	BoundsHit []string
	// Limits are the executor's budgets the search ran under.
	Limits     Budgets
	Violations []Violation
	Divergent  []Divergence
	// Finals are the distinct outcomes of the complete schedules, in canonical order.
	Finals []CheckFinal
}

// Status renders how the check ended for a report.
func (r *CheckReport) Status() string {
	s := fmt.Sprintf("%s (%d states, %d moves, depth %d", r.Verdict, r.States, r.Moves, r.MaxDepth)
	if len(r.BoundsHit) > 0 {
		s += "; bounds hit: " + strings.Join(r.BoundsHit, ", ")
	}
	return s + ")"
}

// CheckStopped is a check the caller stopped before it ended, with what it had
// searched so far; it unwraps to the caller's reason.
type CheckStopped struct {
	States   int
	Moves    int
	MaxDepth int
	Cause    error
}

func (e *CheckStopped) Error() string {
	return fmt.Sprintf("check stopped after %d states and %d moves (depth %d): %v", e.States, e.Moves, e.MaxDepth, e.Cause)
}

func (e *CheckStopped) Unwrap() error { return e.Cause }

// CheckAction searches the schedules of the action start begins in the context
// fresh makes. Every violation and every final value carries the witness a
// replay of the same starter follows (ReplayAction). It stops with a CheckStopped
// when stop ends first; it fails when the checked action is one the search
// cannot snapshot (ErrSnapshotPausedBody) or the run refused a move it selected.
func CheckAction(stop context.Context, fresh func() (*Context, error), start ActionStarter, budget CheckBudget, opts CheckOptions, props []CheckProperty) (*CheckReport, error) {
	ctx, err := fresh()
	if err != nil {
		return nil, err
	}
	script := &checkScript{branch: -1}
	if err := ctx.SetSchedule(checkPolicy(script)); err != nil {
		return nil, err
	}
	if ctx.Trace() == nil {
		ctx.SetTrace(NewTraceRecorder())
	}
	c := &checker{
		stop:     stop,
		ctx:      ctx,
		budget:   budget,
		opts:     opts,
		props:    props,
		visited:  make(map[stateKey]*visitedState),
		onStack:  make(map[stateKey]int),
		finals:   make(map[string]int),
		futures:  make(map[futureKey]lower.Footprint),
		maxSteps: int(ctx.Budgets().MaxActionSteps),
	}
	exec, err := start(ctx)
	if err != nil {
		// Failing to start fails on every schedule: a violation with no move.
		c.violate(Violation{Kind: ViolationFailure, Err: err, Witness: c.failing(err)})
		return c.result(), nil
	}
	c.exec = exec
	defer exec.Release()
	if err := c.search(); err != nil {
		return nil, err
	}
	return c.result(), nil
}

// checker is one search in progress.
type checker struct {
	stop   context.Context
	ctx    *Context
	exec   *ActionExecutor
	budget CheckBudget
	opts   CheckOptions
	props  []CheckProperty

	visited map[stateKey]*visitedState
	// onStack counts the frames on the stack at each state.
	onStack map[stateKey]int
	stack   []*checkFrame
	futures map[futureKey]lower.Footprint

	// maxSteps is the executor's budget of action steps, a bound on the depth.
	maxSteps int
	moves    int
	maxDepth int
	bounds   []string

	violations []Violation
	// finals indexes result.Finals by outcome identity.
	finals  map[string]int
	results []CheckFinal
}

// visitedState is what the search remembers of a state: the moves explored from
// it, the shallowest depth it was searched from, and whether a bound cut below it.
type visitedState struct {
	explored map[string]bool
	depth    int
	cut      bool
}

// checkFrame is one state on the search stack.
type checkFrame struct {
	snap  *Snapshot
	key   stateKey
	depth int
	// all lists every move enabled in the state; moves the ones the search takes,
	// in order; next indexes the one to take.
	all   []searchMove
	moves []searchMove
	next  int
	// sleep lists the moves asleep in the state: explored from an equivalent predecessor.
	sleep []searchMove
	// full is set once the state expands to every enabled move.
	full bool
	// cut is set once the depth bound cut a schedule through the state.
	cut bool
}

// searchMove is an enabled move with what the reduction needs of it: its
// canonical name, its footprint, and the footprint of its token's future.
type searchMove struct {
	enabledMove
	name      string
	footprint lower.Footprint
	future    lower.Footprint
}

// same reports whether the two are one move: one token taking one branch.
func (m searchMove) same(o searchMove) bool {
	return m.Token == o.Token && m.Branch == o.Branch
}

func containsMove(moves []searchMove, m searchMove) bool {
	return slices.ContainsFunc(moves, m.same)
}

func (c *checker) hit(bound string) {
	if !slices.Contains(c.bounds, bound) {
		c.bounds = append(c.bounds, bound)
	}
}

// violate records the violation; a property is reported once, by the shortest
// schedule found to reach a state where it is false.
func (c *checker) violate(v Violation) {
	if v.Kind == ViolationProperty {
		for i, seen := range c.violations {
			if seen.Kind != ViolationProperty || seen.Name != v.Name {
				continue
			}
			if v.Depth < seen.Depth {
				c.violations[i] = v
			}
			return
		}
	}
	c.violations = append(c.violations, v)
}

// witness is the schedule so far: the choices the run noted and its trace.
func (c *checker) witness() Witness {
	w := Witness{Choices: c.ctx.ChoicesTaken()}
	if tr := c.ctx.Trace(); tr != nil {
		w.Trace = tr.String()
	}
	return w
}

// failing is the schedule so far ending in err, which a replay must raise again.
func (c *checker) failing(err error) Witness {
	w := c.witness()
	w.Fails = err.Error()
	return w
}

// search runs the depth-first search from the started executor's state.
func (c *checker) search() error {
	if err := c.stabilize(); err != nil {
		return c.failed(err, 0)
	}
	if c.exec.State() == StateCompleted {
		return c.complete(0)
	}
	root, key, err := c.enter(0, nil)
	if err != nil || root == nil {
		return err
	}
	c.onStack[key]++
	c.stack = append(c.stack, root)
	for len(c.stack) > 0 {
		if err := c.stop.Err(); err != nil {
			c.releaseAll()
			return &CheckStopped{States: len(c.visited), Moves: c.moves, MaxDepth: c.maxDepth, Cause: err}
		}
		f := c.stack[len(c.stack)-1]
		if f.next >= len(f.moves) {
			c.stack = c.stack[:len(c.stack)-1]
			c.onStack[f.key]--
			f.snap.Release()
			if f.cut && len(c.stack) > 0 {
				c.cut(c.stack[len(c.stack)-1])
			}
			continue
		}
		m := f.moves[f.next]
		f.next++
		c.visited[f.key].explored[m.name] = true
		f.snap.Restore()
		if err := c.take(f, m); err != nil {
			c.releaseAll()
			return err
		}
	}
	return nil
}

// cut marks the frame's state as one the depth bound cut a schedule through.
func (c *checker) cut(f *checkFrame) {
	f.cut = true
	c.visited[f.key].cut = true
}

func (c *checker) releaseAll() {
	for _, f := range c.stack {
		f.snap.Release()
	}
	c.stack = nil
}

// take makes the move from the frame's state and enters the state it reaches.
func (c *checker) take(f *checkFrame, m searchMove) error {
	depth := f.depth + 1
	if c.budget.Depth > 0 && depth > c.budget.Depth {
		c.hit("depth")
		c.cut(f)
		return nil
	}
	if c.maxSteps > 0 && depth > c.maxSteps {
		c.hit(BoundActionSteps)
		c.cut(f)
		return nil
	}
	branches, err := c.exec.makeMove(m.enabledMove)
	c.moves++
	c.maxDepth = max(c.maxDepth, depth)
	if m.Branch < 0 && branches > 1 {
		f.branches(m, branches)
	}
	if err != nil {
		return c.failed(err, depth)
	}
	if err := c.stabilize(); err != nil {
		return c.failed(err, depth)
	}
	if c.exec.State() == StateCompleted {
		return c.complete(depth)
	}
	child, key, err := c.enter(depth, c.childSleep(f, m))
	if err != nil {
		return err
	}
	if seen := c.visited[key]; seen != nil && seen.cut {
		c.cut(f)
	}
	if c.onStack[key] > 0 {
		// A move closing a cycle on the stack: the state expands fully, so no move is ignored.
		f.expand()
	}
	if child != nil {
		c.onStack[key]++
		c.stack = append(c.stack, child)
	}
	return nil
}

// failed classifies the error a move raised: a budget is a bound the search
// hit, a move the run made otherwise than selected fails the check, anything
// else is a violation on the schedule that reached it.
func (c *checker) failed(err error, depth int) error {
	if bound, isBound := boundOf(err); isBound {
		c.hit(bound)
		return nil
	}
	if errors.Is(err, ErrCheckRefused) || errors.Is(err, ErrSnapshotPausedBody) {
		return err
	}
	kind := ViolationFailure
	if errors.Is(err, ErrActionDeadlock) || errors.Is(err, ErrAcceptDeadlock) {
		kind = ViolationDeadlock
	}
	c.violate(Violation{Kind: kind, Err: err, Depth: depth, Witness: c.failing(err)})
	return nil
}

// stabilize settles the state until a move is enabled or the action is complete;
// a settling that changes nothing is a deadlock.
func (c *checker) stabilize() error {
	for !c.stable() {
		now, state := c.ctx.clock.now, c.exec.State()
		if err := c.exec.settle(); err != nil {
			return err
		}
		if c.ctx.clock.now == now && c.exec.State() == state && !c.stable() {
			return c.exec.deadlockError(nil)
		}
	}
	return nil
}

// stable reports whether the state is one to search from: complete, or with a move enabled.
func (c *checker) stable() bool {
	return c.exec.State() == StateCompleted || len(c.exec.enabledMoves()) > 0
}

// complete visits the completed state the executor reached: a state like any
// other, its properties evaluated when new, whose outcome is a final.
func (c *checker) complete(depth int) error {
	if _, _, seen, _, err := c.visit(depth); err != nil || seen == nil {
		return err
	}
	c.final()
	return nil
}

// visit records the stable state the executor stands in, evaluating the
// properties at a new one; seen is nil when the states bound keeps the search out.
func (c *checker) visit(depth int) (form canonicalForm, key stateKey, seen *visitedState, visited bool, err error) {
	if form, err = c.exec.canonicalState(); err != nil {
		return form, "", nil, false, err
	}
	key = form.key()
	if seen, visited = c.visited[key]; !visited {
		if c.budget.States > 0 && len(c.visited) >= c.budget.States {
			c.hit("states")
			return form, key, nil, false, nil
		}
		seen = &visitedState{explored: make(map[string]bool), depth: depth}
		c.visited[key] = seen
		c.properties(depth)
	} else if depth < seen.depth {
		if seen.cut {
			seen.explored = make(map[string]bool)
			seen.cut = false
		}
		seen.depth = depth
	}
	return form, key, seen, visited, nil
}

// enter visits the stable state the executor stands in and returns the frame to
// search it from, nil when nothing remains to explore from it or a bound keeps
// the search out.
func (c *checker) enter(depth int, sleep []searchMove) (*checkFrame, stateKey, error) {
	form, key, seen, visited, err := c.visit(depth)
	if err != nil || seen == nil {
		return nil, key, err
	}
	all := c.movesOf(form)
	f := &checkFrame{key: key, depth: depth, all: all, sleep: sleep}
	f.moves = c.persistent(all, sleep)
	if visited && !seen.wanted(f) {
		return nil, key, nil
	}
	snap, err := c.exec.Snapshot()
	if err != nil {
		return nil, key, err
	}
	f.snap = snap
	seen.mark(f.moves)
	return f, key, nil
}

// wanted drops from the frame the moves already explored from its state and
// reports whether any remain.
func (s *visitedState) wanted(f *checkFrame) bool {
	kept := f.moves[:0]
	for _, m := range f.moves {
		if !s.explored[m.name] {
			kept = append(kept, m)
		}
	}
	f.moves = kept
	return len(kept) > 0
}

func (s *visitedState) mark(moves []searchMove) {
	for _, m := range moves {
		s.explored[m.name] = true
	}
}

// movesOf lists the state's enabled moves with their canonical names and
// footprints, in canonical order.
func (c *checker) movesOf(form canonicalForm) []searchMove {
	enabled := c.exec.enabledMoves()
	moves := make([]searchMove, 0, len(enabled))
	for _, m := range enabled {
		moves = append(moves, c.named(form, m))
	}
	slices.SortFunc(moves, func(a, b searchMove) int { return strings.Compare(a.name, b.name) })
	return moves
}

func (c *checker) named(form canonicalForm, m enabledMove) searchMove {
	name := form.tokens[m.Token]
	if m.Branch >= 0 {
		name = fmt.Sprintf("%s branch %d", name, m.Branch+1)
	}
	return searchMove{enabledMove: m, name: name, footprint: c.footprintOf(m), future: c.futureOf(m)}
}

// branches adds the other branches of a decision the move revealed, right after it.
func (f *checkFrame) branches(m searchMove, n int) {
	var more []searchMove
	for branch := 1; branch < n; branch++ {
		other := m
		other.Branch = branch
		other.name = fmt.Sprintf("%s branch %d", m.name, branch+1)
		more = append(more, other)
	}
	f.all = slices.Insert(f.all, slices.IndexFunc(f.all, m.same)+1, more...)
	f.moves = slices.Insert(f.moves, f.next, more...)
}

// expand makes the frame explore every enabled move, the asleep ones included.
func (f *checkFrame) expand() {
	if f.full {
		return
	}
	f.full = true
	f.sleep = nil
	for _, m := range f.all {
		if !containsMove(f.moves, m) {
			f.moves = append(f.moves, m)
		}
	}
}

// properties evaluates every property at the state; one false is a violation.
func (c *checker) properties(depth int) {
	for _, p := range c.props {
		holds, err := c.evaluate(p)
		if err != nil {
			c.violate(Violation{Kind: ViolationFailure, Name: p.Name, Err: err, Depth: depth, Witness: c.witness()})
			continue
		}
		if !holds {
			c.violate(Violation{Kind: ViolationProperty, Name: p.Name, Depth: depth, Witness: c.witness()})
		}
	}
}

// evaluate asks the property of the state under a probe: what evaluating it
// derives is given back.
func (c *checker) evaluate(p CheckProperty) (bool, error) {
	defer c.ctx.beginProbe()()
	return p.Holds(c.ctx, c.exec)
}

// final records the outcome of a complete schedule, the first schedule
// reaching each distinct outcome being its witness.
func (c *checker) final() {
	outcome := c.ctx.ActionOutcome(c.exec.Results())
	values := c.divergenceValues(outcome)
	spelled, identity := outcome.String(), outcome.identity()
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if !strings.HasPrefix(name, "this.") {
			continue
		}
		spelled += "; " + name + " = " + values[name]
		identity += "; " + name + " = " + strconv.Quote(values[name])
	}
	if _, seen := c.finals[identity]; seen {
		return
	}
	c.finals[identity] = len(c.results)
	c.results = append(c.results, CheckFinal{
		Outcome:  spelled,
		Values:   values,
		Witness:  c.witness(),
		identity: identity,
	})
}

// divergenceValues spells the features divergence is reported over as the
// schedule left them: the named ones, or the action's own attributes and the
// performing object's features.
func (c *checker) divergenceValues(outcome Outcome) map[string]string {
	defer c.ctx.beginProbe()()
	values := make(map[string]string)
	for _, out := range outcome.RenderedOutputs() {
		if c.reportsDivergenceOf(out.Name, nil) {
			values[out.Name] = out.Text
		}
	}
	if self := c.exec.Performer(); self != nil {
		own := make(map[string]Value)
		for name, held := range self.FeatureValues {
			if !c.reportsDivergenceOf(name, held.Feature) {
				continue
			}
			fv, err := self.GetFeatureValue(c.ctx, name)
			if err != nil {
				values["this."+name] = "<error: " + err.Error() + ">"
				continue
			}
			switch {
			case !fv.Materialized:
			case fv.Feature.Scalar():
				own[name] = fv.Value
			default:
				own[name] = fv.Values
			}
		}
		for _, out := range c.ctx.ActionOutcome(own).RenderedOutputs() {
			values["this."+out.Name] = out.Text
		}
	}
	return values
}

// reportsDivergenceOf reports whether the feature — the performing object's when
// of is given, named `this.<name>`, else the action's, named bare — is one
// divergence is reported over; absent names, both's attributes are.
func (c *checker) reportsDivergenceOf(name string, of *EffectiveFeature) bool {
	if len(c.opts.Diverge) == 0 {
		if of != nil {
			return of.Symbol != nil && of.Symbol.Kind == symbols.SymbolAttributeUsage
		}
		return !strings.Contains(name, ".")
	}
	if of != nil {
		name = "this." + name
	}
	return slices.Contains(c.opts.Diverge, name)
}

// result assembles what the search found.
func (c *checker) result() *CheckReport {
	r := &CheckReport{
		States:     len(c.visited),
		Moves:      c.moves,
		MaxDepth:   c.maxDepth,
		BoundsHit:  c.bounds,
		Limits:     c.ctx.Budgets(),
		Violations: c.violations,
		Finals:     slices.Clone(c.results),
	}
	sort.Slice(r.Finals, func(i, j int) bool { return r.Finals[i].identity < r.Finals[j].identity })
	r.Divergent = divergences(r.Finals)
	switch {
	case len(r.Violations) > 0:
		r.Verdict = CheckViolation
	case len(r.Divergent) > 0:
		r.Verdict = CheckDivergent
	case len(r.BoundsHit) > 0:
		r.Verdict = CheckWithinBounds
	default:
		r.Verdict = CheckExhaustive
	}
	return r
}

// divergences finds the features the finals disagree on, each value with the
// first final reaching it as its witness, features and values in order.
func divergences(finals []CheckFinal) []Divergence {
	byFeature := make(map[string]map[string]Witness)
	for _, final := range finals {
		for name, value := range final.Values {
			if byFeature[name] == nil {
				byFeature[name] = make(map[string]Witness)
			}
			if _, seen := byFeature[name][value]; !seen {
				byFeature[name][value] = final.Witness
			}
		}
	}
	var out []Divergence
	for name, values := range byFeature {
		if len(values) < 2 {
			continue
		}
		d := Divergence{Feature: name}
		for value, witness := range values {
			d.Values = append(d.Values, DivergentValue{Value: value, Witness: witness})
		}
		sort.Slice(d.Values, func(i, j int) bool { return d.Values[i].Value < d.Values[j].Value })
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Feature < out[j].Feature })
	return out
}

// The executor budgets a search runs under, by the name a bound hit reports;
// each names one field of Budgets, so a report spells the limit that stopped it.
const (
	BoundSteps       = "steps"
	BoundActionSteps = "actionSteps"
	BoundEvents      = "events"
	BoundDoSteps     = "doSteps"
	BoundElements    = "elements"
	BoundBehaviors   = "behaviors"
)

// ExecutorBounds lists the executor budgets a bound hit may name, in report order.
var ExecutorBounds = []string{BoundSteps, BoundActionSteps, BoundEvents, BoundDoSteps, BoundElements, BoundBehaviors}

// ExecutorBound is the limit the named executor bound has under the budgets,
// false for a name that is no executor bound's. The object behaviors' rounds
// are counted in events, so both names spell the events limit.
func ExecutorBound(name string, b Budgets) (int64, bool) {
	switch name {
	case BoundSteps:
		return b.MaxSteps, true
	case BoundActionSteps:
		return b.MaxActionSteps, true
	case BoundEvents, BoundBehaviors:
		return b.MaxStateEvents, true
	case BoundDoSteps:
		return b.MaxDoSteps, true
	case BoundElements:
		return b.MaxElements, true
	}
	return 0, false
}

// boundOf names the executor budget an error reports exhausted, false for an
// error that is no budget's; the behaviors' budget wraps the events one, so it
// is told apart first.
func boundOf(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrBehaviorBudget):
		return BoundBehaviors, true
	case errors.Is(err, ErrStepLimitExceeded):
		return BoundSteps, true
	case errors.Is(err, ErrActionStepLimitExceeded):
		return BoundActionSteps, true
	case errors.Is(err, ErrStateEventLimitExceeded):
		return BoundEvents, true
	case errors.Is(err, ErrDoStepLimitExceeded):
		return BoundDoSteps, true
	case errors.Is(err, ErrElementLimitExceeded):
		return BoundElements, true
	}
	return "", false
}
