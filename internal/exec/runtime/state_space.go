package runtime

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Errors a state-space run reports, each wrapping the detail of what went wrong.
var (
	// errStateSpaceValue reports a state, input, derivative, difference or output
	// that is not the vector the protocol asks for, or one the action does not bind.
	errStateSpaceValue = errors.New("state-space value")
	// errStateSpaceStep reports a time step the dynamics cannot advance by: none
	// stated, not a duration, or not positive.
	errStateSpaceStep = errors.New("state-space time step")
	// errStateSpaceDiverged reports a step that left the state with a component
	// that is no longer a finite number.
	errStateSpaceDiverged = errors.New("state-space dynamics diverged")
)

// stateSpaceSemantics answers lowering's semantic questions from the context's model.
type stateSpaceSemantics struct{ ctx *Context }

func (m stateSpaceSemantics) LibrarySymbol(fqn string) *symbols.Symbol {
	return m.ctx.librarySymbol(fqn)
}
func (m stateSpaceSemantics) Specializes(sym, general *symbols.Symbol) bool {
	return m.ctx.conforms(sym, general)
}
func (m stateSpaceSemantics) MembersOf(sym *symbols.Symbol) []*symbols.Symbol {
	return m.ctx.model.semantics.MembersOf(sym)
}
func (m stateSpaceSemantics) FeatureTypes(sym *symbols.Symbol) []*symbols.Symbol {
	return m.ctx.model.semantics.FeatureTypes(sym)
}
func (m stateSpaceSemantics) ParameterDefault(sym *symbols.Symbol) (ast.Node, *symbols.Scope) {
	return m.ctx.model.semantics.ParameterDefault(sym)
}
func (m stateSpaceSemantics) LibraryDeclared(sym *symbols.Symbol) bool {
	return m.ctx.libraryDeclared(sym)
}

// stateSpaceKindOf classifies an action by the library dynamics it specializes.
func (ctx *Context) stateSpaceKindOf(action *symbols.Symbol) lower.StateSpaceKind {
	if ctx.model.semantics == nil {
		return lower.NotStateSpace
	}
	return lower.StateSpaceKindOf(action, stateSpaceSemantics{ctx})
}

// stateSpaceRun is the progress of an action run as state-space dynamics: one
// token stands at the action itself, parked on the clock between steps.
type stateSpaceRun struct {
	dyn *lower.StateSpaceDynamics
	// start is the clock's instant the run began at, steps the steps taken since;
	// the next step is due at start + (steps+1)*step, so the instants do not drift.
	start float64
	steps int
	step  float64
	// stop is the duration the run lasts from its start, stops false where it runs while driven.
	stop  float64
	stops bool
	// stepValue is timeStep as the action holds it, what the state's rate is scaled by.
	stepValue Value
	// guards holds what each crossing watches after the latest step, for its signs: its
	// guard, or the derivative's components; nil until the first step settles the start.
	guards [][]float64
}

// settled reports whether the run has sampled its start, which its first step does.
func (run *stateSpaceRun) settled() bool { return run.guards != nil }

// soFar is the duration the run has stepped from its own start: the time feature's
// value, exact where the clock's instant less the start would round at a late start.
func (run *stateSpaceRun) soFar() float64 { return float64(run.steps) * run.step }

// pastStop reports a run whose next step would end past the duration it stops at.
func (run *stateSpaceRun) pastStop() bool {
	return run.stops && float64(run.steps+1)*run.step > run.stop+run.step*1e-9
}

// ownsFeature reports a feature the run writes itself, whose declared default the
// action leaves unevaluated: the output, sampled by getOutput as each step settles.
func (run *stateSpaceRun) ownsFeature(name string) bool {
	return run != nil && run.dyn.Output != nil && name == run.dyn.Output.Name
}

// clone is the run's progress by value, nil for an action running no dynamics;
// a snapshot keeps one and hands a fresh one back at each restore.
func (run *stateSpaceRun) clone() *stateSpaceRun {
	if run == nil {
		return nil
	}
	saved := *run
	saved.guards = slices.Clone(run.guards)
	for i, watched := range saved.guards {
		saved.guards[i] = slices.Clone(watched)
	}
	return &saved
}

// dynamicsNode is the node the run's one token stands at: the action itself.
func (e *ActionExecutor) dynamicsNode() ast.Node { return e.action.Decl }

// initializeDynamics starts a state-space run: binds the action's features, checks
// the state and step, computes the output at the start and parks a token until the first step.
func (e *ActionExecutor) initializeDynamics() error {
	dyn, err := lower.ToStateSpaceDynamics(e.action, e.graph.Scope, stateSpaceSemantics{e.ctx}, e.graph.Exprs)
	if err != nil {
		return err
	}
	if err := e.checkResultParameters(); err != nil {
		return err
	}
	run := &stateSpaceRun{dyn: dyn}
	e.dynamics = run

	e.ctx.beginPerformanceLife(e.occurrence, e.ctx.newActivation())
	if err := e.bindInputs(); err != nil {
		return err
	}
	if err := e.readStep(run); err != nil {
		return err
	}
	if _, err := e.stateVector(); err != nil {
		return err
	}
	run.start = e.ctx.clock.now
	e.tokens = append(e.tokens, Token{ID: e.nextTokenID, Location: NodeRef{Graph: e.graph}, frame: e.root})
	e.nextTokenID++
	e.state = StateRunning
	return nil
}

// readStep reads the step the dynamics advance by and the duration they run for, in clock units.
func (e *ActionExecutor) readStep(run *stateSpaceRun) error {
	dyn := run.dyn
	what := fmt.Sprintf("%s of action %s", lower.TimeStepFeature, SymbolText(e.action))
	step, ok, err := e.ctx.readBodyValue(e.root.cells, e.root.data, e.root.key(lower.TimeStepFeature))
	if err != nil {
		return fmt.Errorf("%w: %s: %w", errStateSpaceStep, what, err)
	}
	if dyn.TimeStep == nil || !ok {
		return fmt.Errorf("%w: action %s states no %s; specialize %s or declare one",
			errStateSpaceStep, SymbolText(e.action), lower.TimeStepFeature, lower.FixedStepDynamicsFQN)
	}
	// A bound parameter holds the step as a one-element sequence; the rate is
	// scaled by the duration itself.
	step = soleElement(step)
	magnitude, err := e.ctx.timeMagnitude(step, what)
	if err != nil {
		return fmt.Errorf("%w: %w", errStateSpaceStep, err)
	}
	if math.IsNaN(magnitude) || math.IsInf(magnitude, 0) || magnitude <= 0 {
		return fmt.Errorf("%w: %s is %s, not a positive duration", errStateSpaceStep, what, FormatValue(step))
	}
	run.step, run.stepValue = magnitude, step
	if dyn.StopTime == nil {
		return nil
	}
	what = fmt.Sprintf("%s of action %s", lower.StopTimeFeature, SymbolText(e.action))
	stop, ok, err := e.ctx.readBodyValue(e.root.cells, e.root.data, e.root.key(lower.StopTimeFeature))
	if err != nil {
		return fmt.Errorf("%w: %s: %w", errStateSpaceStep, what, err)
	}
	if !ok || stop.Kind == ValNull {
		return nil
	}
	if run.stop, err = e.ctx.timeMagnitude(stop, what); err != nil {
		return fmt.Errorf("%w: %w", errStateSpaceStep, err)
	}
	if math.IsNaN(run.stop) || run.stop < 0 {
		return fmt.Errorf("%w: %s is %s, not a duration", errStateSpaceStep, what, FormatValue(stop))
	}
	run.stops = true
	return nil
}

// stateVector is the state the action holds, checked to be a vector.
func (e *ActionExecutor) stateVector() (Value, error) {
	state, ok, err := e.ctx.readBodyValue(e.root.cells, e.root.data, e.root.key(lower.StateSpaceFeature))
	if err != nil {
		return Value{}, fmt.Errorf("%w: action %s: read %s: %w",
			errStateSpaceValue, SymbolText(e.action), lower.StateSpaceFeature, err)
	}
	if !ok || state.Kind == ValNull {
		return Value{}, fmt.Errorf("%w: action %s binds no %s; give it an initial value",
			errStateSpaceValue, SymbolText(e.action), lower.StateSpaceFeature)
	}
	return e.checkVector(lower.StateSpaceFeature, state)
}

// dynamicsSteps is the number of steps the dynamics took, for telling progress; 0 for other actions.
func (e *ActionExecutor) dynamicsSteps() int {
	if e.dynamics == nil {
		return 0
	}
	return e.dynamics.steps
}

// inputVector is the input the action holds, or the one a flow carries into it; nil when
// it has none, so a calc that does not need one runs and one that does reports its parameter unbound.
func (e *ActionExecutor) inputVector() (*Value, error) {
	if flows := e.inputFlows(); len(flows) > 0 {
		return e.flowedInput(flows)
	}
	input, ok, err := e.ctx.readBodyValue(e.root.cells, e.root.data, e.root.key(lower.InputFeature))
	if err != nil {
		return nil, fmt.Errorf("%w: action %s: read %s: %w",
			errStateSpaceValue, SymbolText(e.action), lower.InputFeature, err)
	}
	if !ok || input.Kind == ValNull {
		return nil, nil
	}
	checked, err := e.checkVector(lower.InputFeature, input)
	if err != nil {
		return nil, err
	}
	return &checked, nil
}

// checkVector refuses a value that is no vector, naming the protocol feature it stood for.
func (e *ActionExecutor) checkVector(what string, val Value) (Value, error) {
	val = soleElement(val)
	if !isVectorKind(val) {
		return Value{}, fmt.Errorf("%w: %s of action %s is %s, not a vector",
			errStateSpaceValue, what, SymbolText(e.action), DescribeValue(val))
	}
	return val, nil
}

// dynamicsFootprint is what a step of the dynamics may touch: the model's calcs
// read what they name, so the checker holds the step dependent on every move.
func dynamicsFootprint() lower.Footprint { return lower.Footprint{Dynamic: true} }

// dynamicsDue reports whether the run's token may step at this instant: not
// parked, or parked until an instant the clock has reached.
func (e *ActionExecutor) dynamicsDue(token Token) bool {
	return token.Wait == nil || token.Wait.Due <= e.ctx.clock.now
}

// stepDynamics samples the start on the token's first step, then performs the
// step the token is due for, or leaves it parked.
func (e *ActionExecutor) stepDynamics(tokenIdx int) error {
	token := &e.tokens[tokenIdx]
	run := e.dynamics
	if !e.dynamicsDue(*token) {
		return nil
	}
	token.Wait = nil
	token.moved = e.sweep

	state, err := e.stateVector()
	if err != nil {
		return err
	}
	input, err := e.inputVector()
	if err != nil {
		return err
	}
	if !run.settled() {
		ended, err := e.settleStep(run, state, input)
		if err != nil {
			return err
		}
		if ended || run.pastStop() {
			return e.retireToken(tokenIdx)
		}
		return e.parkDynamics(run, &e.tokens[tokenIdx])
	}
	next, err := e.nextState(run, state, input)
	if err != nil {
		return e.divergence(run, err)
	}
	run.steps++
	if err := e.checkFinite(next); err != nil {
		return err
	}
	if err := e.setFeature(lower.StateSpaceFeature, next); err != nil {
		return err
	}
	ended, err := e.settleStep(run, next, input)
	if err != nil {
		return err
	}
	if ended || run.pastStop() {
		return e.retireToken(tokenIdx)
	}
	return e.parkDynamics(run, &e.tokens[tokenIdx])
}

// due is the instant the run's next step is due at.
func (run *stateSpaceRun) due() float64 {
	return run.start + float64(run.steps+1)*run.step
}

// stepStart is the instant the run's next step advances from: where its latest settled.
func (run *stateSpaceRun) stepStart() float64 {
	return run.start + float64(run.steps)*run.step
}

// parkDynamics parks the run's token on the clock until its next step is due; a
// step due past the last instant a float64 holds is refused, so the clock stays finite.
func (e *ActionExecutor) parkDynamics(run *stateSpaceRun, token *Token) error {
	due := run.due()
	if math.IsInf(due, 0) {
		return fmt.Errorf("%w: step %d of action %s from t=%s leads past the last instant the clock can hold",
			errStateSpaceStep, run.steps+1, SymbolText(e.action), semantics.FormatReal(run.stepStart()))
	}
	token.Wait = &AcceptWait{
		Trigger: fmt.Sprintf("step %d of %s", run.steps+1, SymbolText(e.action)),
		Since:   e.stepCount + 1,
		Timed:   true,
		Due:     due,
	}
	return nil
}

// settleStep records the state reached at the clock's instant: the time feature as the
// duration stepped so far, the output, the trace line and the zero crossings, true when one ends the run.
func (e *ActionExecutor) settleStep(run *stateSpaceRun, state Value, input *Value) (bool, error) {
	now := e.ctx.clock.now
	if run.dyn.Time != nil {
		if err := e.setFeature(lower.TimeFeature, e.instantValue(run.soFar())); err != nil {
			return false, err
		}
	}
	if input == nil {
		var err error
		if input, err = e.inputVector(); err != nil {
			return false, err
		}
	}
	output, err := e.invokeProtocolCalc(run.dyn.OutputCalc, lower.GetOutputCalc, input, state, nil)
	if err != nil {
		return false, err
	}
	if _, err := e.checkVector(lower.OutputFeature, output); err != nil {
		return false, err
	}
	if err := e.setFeature(lower.OutputFeature, output); err != nil {
		return false, err
	}
	if tr := e.trace(); tr != nil {
		tr.recordStateSpaceStep(SymbolText(e.action), now, state, output)
	}
	return e.watchCrossings(run, now, state, input)
}

// instantValue is the clock's instant t as the context spells one.
func (e *ActionExecutor) instantValue(t float64) Value {
	return e.ctx.instantValue(t)
}

// nextState is the state one step on: the library's getNextState integrates the
// derivative or adds the difference, one the model bodies itself is called as written.
func (e *ActionExecutor) nextState(run *stateSpaceRun, state Value, input *Value) (Value, error) {
	dyn := run.dyn
	if dyn.NextState != nil {
		next, err := e.invokeProtocolCalc(dyn.NextState, lower.GetNextStateCalc, input, state, &run.stepValue)
		if err != nil {
			return Value{}, err
		}
		return e.checkVector(lower.GetNextStateCalc, next)
	}
	if dyn.Kind == lower.DiscreteDynamics {
		diff, err := e.invokeProtocolCalc(dyn.Difference, lower.GetDifferenceCalc, input, state, nil)
		if err != nil {
			return Value{}, err
		}
		diff, err = e.checkVector(lower.GetDifferenceCalc, diff)
		if err != nil {
			return Value{}, err
		}
		return e.vectorOp(ast.OpAdd, state, diff)
	}
	switch dyn.Integrator {
	case lower.IntegratorEuler:
		return e.eulerStep(run, state, input)
	default:
		return e.rk4Step(run, state, input)
	}
}

// eulerStep advances the state by the step times the derivative at its start.
func (e *ActionExecutor) eulerStep(run *stateSpaceRun, state Value, input *Value) (Value, error) {
	k, err := e.derivative(run, input, state)
	if err != nil {
		return Value{}, err
	}
	rate, err := e.vectorOp(ast.OpMul, k, run.stepValue)
	if err != nil {
		return Value{}, err
	}
	return e.vectorOp(ast.OpAdd, state, rate)
}

// rk4Step advances the state by the classical Runge-Kutta scheme: the derivative
// at the start, twice at the midpoint and at the end, each stage at its own
// duration into the step, weighted 1:2:2:1.
func (e *ActionExecutor) rk4Step(run *stateSpaceRun, state Value, input *Value) (Value, error) {
	half, err := e.vectorOp(ast.OpMul, run.stepValue, realConst(0.5))
	if err != nil {
		return Value{}, err
	}
	start := run.soFar()
	mid, end := start+run.step/2, start+run.step
	k1, err := e.derivative(run, input, state)
	if err != nil {
		return Value{}, err
	}
	k2, err := e.derivativeAt(run, input, state, k1, half, mid)
	if err != nil {
		return Value{}, err
	}
	k3, err := e.derivativeAt(run, input, state, k2, half, mid)
	if err != nil {
		return Value{}, err
	}
	k4, err := e.derivativeAt(run, input, state, k3, run.stepValue, end)
	if err != nil {
		return Value{}, err
	}
	sum := k1
	for _, term := range []struct {
		k      Value
		weight float64
	}{{k2, 2}, {k3, 2}, {k4, 1}} {
		weighted, err := e.vectorOp(ast.OpMul, term.k, realConst(term.weight))
		if err != nil {
			return Value{}, err
		}
		if sum, err = e.vectorOp(ast.OpAdd, sum, weighted); err != nil {
			return Value{}, err
		}
	}
	sixth, err := e.vectorOp(ast.OpDiv, run.stepValue, realConst(6))
	if err != nil {
		return Value{}, err
	}
	rate, err := e.vectorOp(ast.OpMul, sum, sixth)
	if err != nil {
		return Value{}, err
	}
	return e.vectorOp(ast.OpAdd, state, rate)
}

// derivativeAt is the derivative at the instant t and the state reached from
// state by advancing along slope for a span of the step.
func (e *ActionExecutor) derivativeAt(run *stateSpaceRun, input *Value, state, slope, span Value, t float64) (Value, error) {
	advance, err := e.vectorOp(ast.OpMul, slope, span)
	if err != nil {
		return Value{}, err
	}
	at, err := e.vectorOp(ast.OpAdd, state, advance)
	if err != nil {
		return Value{}, err
	}
	return e.atInstant(run, t, func() (Value, error) { return e.derivative(run, input, at) })
}

// atInstant evaluates a stage with the time feature holding t, a duration from the
// run's start, then restores the step's start; without a time feature it just evaluates.
func (e *ActionExecutor) atInstant(run *stateSpaceRun, t float64, stage func() (Value, error)) (Value, error) {
	if run.dyn.Time == nil {
		return stage()
	}
	saved, held := e.root.data[e.root.key(lower.TimeFeature)]
	if !held {
		saved = e.instantValue(run.soFar())
	}
	if err := e.setFeature(lower.TimeFeature, e.instantValue(t)); err != nil {
		return Value{}, err
	}
	val, err := stage()
	if restoreErr := e.setFeature(lower.TimeFeature, saved); restoreErr != nil && err == nil {
		err = restoreErr
	}
	return val, err
}

// derivative is the model's getDerivative at a state, checked to be a vector.
func (e *ActionExecutor) derivative(run *stateSpaceRun, input *Value, state Value) (Value, error) {
	k, err := e.invokeProtocolCalc(run.dyn.Derivative, lower.GetDerivativeCalc, input, state, nil)
	if err != nil {
		return Value{}, err
	}
	return e.checkVector(lower.GetDerivativeCalc, k)
}

// vectorOp applies an arithmetic operator over vectors and scalars, as the
// VectorFunctions and VectorCalculations libraries define it; a scalar pair
// multiplies or divides as quantities do.
func (e *ActionExecutor) vectorOp(op ast.OperatorKind, left, right Value) (Value, error) {
	if !isVectorKind(left) && !isVectorKind(right) {
		val, err := e.ctx.arithmeticValues(lower.OperatorOf(op), left, right, source.Span{})
		if err != nil {
			return Value{}, fmt.Errorf("%w: step of action %s: %w", errStateSpaceValue, SymbolText(e.action), err)
		}
		return val, nil
	}
	val, handled, err := e.ctx.vectorArithmetic(op, left, right)
	if err != nil {
		return Value{}, fmt.Errorf("%w: step of action %s: %w", errStateSpaceValue, SymbolText(e.action), err)
	}
	if !handled {
		return Value{}, fmt.Errorf("%w: step of action %s: %s %s %s is not defined over these values",
			errStateSpaceValue, SymbolText(e.action), DescribeValue(left), op, DescribeValue(right))
	}
	return val, nil
}

// divergence reports a step whose arithmetic overflowed as the dynamics diverging,
// with the step and instant it happened at; any other error passes through.
func (e *ActionExecutor) divergence(run *stateSpaceRun, err error) error {
	if !errors.Is(err, semantics.ErrArithmeticOverflow) {
		return err
	}
	return fmt.Errorf("%w: step %d of action %s at t=%s: %w", errStateSpaceDiverged,
		run.steps+1, SymbolText(e.action), semantics.FormatReal(e.ctx.clock.now), err)
}

// checkFinite refuses a state with a component that is not a finite number.
func (e *ActionExecutor) checkFinite(state Value) error {
	v, err := readVector(lower.StateSpaceFeature, lower.StateSpaceFeature, state)
	if err != nil {
		return fmt.Errorf("%w: %w", errStateSpaceValue, err)
	}
	for i, n := range v.num {
		if x := asReal(n); math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("%w: component %d of %s of action %s is %s after step %d at t=%s",
				errStateSpaceDiverged, i+1, lower.StateSpaceFeature, SymbolText(e.action),
				semantics.FormatReal(x), e.dynamics.steps, semantics.FormatReal(e.ctx.clock.now))
		}
	}
	return nil
}

// invokeProtocolCalc calls one of the protocol's calcs as the action provides it,
// binding input, stateSpace and, for getNextState, timeStep by name; the calc runs
// over the action's own frame, so it reads the action's other features.
func (e *ActionExecutor) invokeProtocolCalc(calc *symbols.Symbol, what string, input *Value, state Value, step *Value) (Value, error) {
	shape, err := e.ctx.calcShapeOf(calc)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %s of action %s: %w", lower.ErrUnsupportedStateSpace, what, SymbolText(e.action), err)
	}
	named := map[string]Value{lower.StateSpaceFeature: state}
	if input != nil {
		named[lower.InputFeature] = *input
	}
	if step != nil {
		named[lower.TimeStepFeature] = *step
	}
	ec := e.evalContextFor(e.root, e.graph.Scope)
	defer ec.beginStep()()
	result, err := e.ctx.invokeCalcShapeIn(shape, calcArgs{named: named}, ec.scope, ec.self, ec.enclosingRun(shape))
	if err != nil {
		return Value{}, fmt.Errorf("%s of action %s: %w", what, SymbolText(e.action), err)
	}
	return result, nil
}

// crosses reports a guard that changed sign or arrived at zero over a step; one
// resting at zero, or leaving it, crossed at its arrival and does not again.
func crosses(was, guard float64) bool {
	if guard == 0 {
		return was != 0
	}
	return was != 0 && math.Signbit(guard) != math.Signbit(was)
}

// anyCrosses reports whether a component of what a crossing watches crossed zero over a step.
func anyCrosses(was, watched []float64) bool {
	for i := range watched {
		if i < len(was) && crosses(was[i], watched[i]) {
			return true
		}
	}
	return false
}

// watchCrossings evaluates what every crossing watches at the state and input a step settled
// and raises the event of each that crosses zero; true when a terminal one fired.
func (e *ActionExecutor) watchCrossings(run *stateSpaceRun, now float64, state Value, input *Value) (bool, error) {
	first := run.guards == nil
	if first {
		run.guards = make([][]float64, len(run.dyn.Crossings))
	}
	ended := false
	for i, crossing := range run.dyn.Crossings {
		watched, err := e.watched(run, crossing, state, input)
		if err != nil {
			return false, err
		}
		was := run.guards[i]
		run.guards[i] = watched
		if first || !anyCrosses(was, watched) {
			continue
		}
		terminal, err := e.crossingTerminal(crossing)
		if err != nil {
			return false, err
		}
		e.ctx.PostMessage(Message{
			SignalType:  crossing.EventType.Name,
			Signal:      crossing.EventType,
			Event:       crossing.Event,
			EventName:   crossing.Name,
			EventObject: objectID(e.occurrence),
			Payload:     map[string]Value{},
		})
		if tr := e.trace(); tr != nil {
			tr.recordEvent("zero crossing "+crossing.Name+" of "+SymbolText(e.action), now)
		}
		ended = ended || terminal
	}
	return ended, nil
}

// watched is what a crossing watches at a settled step: the guard it binds, or
// the components of the derivative there for one whose type states no guard.
func (e *ActionExecutor) watched(run *stateSpaceRun, crossing lower.ZeroCrossing, state Value, input *Value) ([]float64, error) {
	if !crossing.Derivative {
		guard, err := e.evalGuard(crossing)
		if err != nil {
			return nil, err
		}
		return []float64{guard}, nil
	}
	k, err := e.derivative(run, input, state)
	if err != nil {
		return nil, fmt.Errorf("zero crossing %s of action %s: %w", crossing.Name, SymbolText(e.action), err)
	}
	v, err := readVector(lower.GetDerivativeCalc, lower.GetDerivativeCalc, k)
	if err != nil {
		return nil, fmt.Errorf("%w: zero crossing %s of action %s: %w", errStateSpaceValue, crossing.Name, SymbolText(e.action), err)
	}
	components := make([]float64, len(v.num))
	for i, n := range v.num {
		components[i] = asReal(n)
	}
	return components, nil
}

// evalGuard is a crossing's guard as a real number, over the action's features.
func (e *ActionExecutor) evalGuard(crossing lower.ZeroCrossing) (float64, error) {
	val, err := e.evalCrossingExpr(crossing.GuardID, crossing.GuardScope)
	if err != nil {
		return 0, fmt.Errorf("guard of zero crossing %s of action %s: %w", crossing.Name, SymbolText(e.action), err)
	}
	if q, ok := asQuantity(val); ok {
		return asReal(q.Num), nil
	}
	return 0, fmt.Errorf("%w: guard of zero crossing %s of action %s is %s, not a number",
		errStateSpaceValue, crossing.Name, SymbolText(e.action), DescribeValue(val))
}

// crossingTerminal reads whether a crossing ends the run, false where it states nothing.
func (e *ActionExecutor) crossingTerminal(crossing lower.ZeroCrossing) (bool, error) {
	if crossing.Terminal == nil {
		return false, nil
	}
	return e.ctx.guardUnderStatementOrders(crossing.Terminal, e.stepCount+1, crossing.TerminalScope, func() (bool, error) {
		val, err := e.evalCrossingExpr(crossing.TerminalID, crossing.TerminalScope)
		if err != nil {
			return false, fmt.Errorf("terminal of zero crossing %s of action %s: %w", crossing.Name, SymbolText(e.action), err)
		}
		if val.Kind != ValConst || val.Const.Kind != semantics.ValBool {
			return false, fmt.Errorf("%w: terminal of zero crossing %s of action %s is %s, not a Boolean",
				errStateSpaceValue, crossing.Name, SymbolText(e.action), DescribeValue(val))
		}
		return val.Const.Bool, nil
	})
}

// evalCrossingExpr evaluates an expression of a crossing in the scope it was
// written, over the action's own frame.
func (e *ActionExecutor) evalCrossingExpr(id program.ExprID, scope *symbols.Scope) (Value, error) {
	if scope == nil {
		scope = e.graph.Scope
	}
	ec := e.evalContextFor(e.root, scope).reading(e.graph.Exprs)
	defer ec.beginStep()()
	return ec.evalPayload(id)
}

// inputFeed is a flow into the action's input, with the object declaring it.
type inputFeed struct {
	lower.FeatureFlow
	on *Instance
}

// objectFlows lowers the streaming flows typeSym and the model's types it specializes
// declare between features, once per type.
func (ctx *Context) objectFlows(typeSym *symbols.Symbol) []lower.FeatureFlow {
	if typeSym == nil {
		return nil
	}
	if cached, ok := ctx.model.flowIR[typeSym]; ok {
		return cached
	}
	var flows []lower.FeatureFlow
	for _, decl := range append(ctx.model.semantics.AllSupertypes(typeSym), typeSym) {
		if decl == nil || ctx.libraryDeclared(decl) {
			continue
		}
		flows = append(flows, lower.ToFeatureFlows(decl.Decl, DeclScope(decl))...)
	}
	ctx.model.flowIR[typeSym] = flows
	return flows
}

// inputFlows are the flows the performer and its owners declare into the performed action's
// input, under its name or one it redefines; a flow stated `[0]` carries nothing and is none.
func (e *ActionExecutor) inputFlows() []inputFeed {
	if e.performed == nil || e.self == nil {
		return nil
	}
	names := map[string]bool{e.performed.Name: true}
	for _, typ := range e.self.types() {
		for _, redefined := range e.ctx.redefinedFeatures(e.performed, typ) {
			names[redefined.Name] = true
		}
	}
	var feeds []inputFeed
	seen := make(map[*ast.Usage]bool)
	var below []string
	for current := e.self; current != nil; {
		for _, typ := range current.types() {
			for _, flow := range e.ctx.objectFlows(typ) {
				if seen[flow.Decl] || !flowsInto(flow.Target, below, names) || e.ctx.flowCarriesNothing(flow) {
					continue
				}
				seen[flow.Decl] = true
				feeds = append(feeds, inputFeed{FeatureFlow: flow, on: current})
			}
		}
		owner, ownerFeature := current.Owner()
		if owner == nil || ownerFeature == "" {
			break
		}
		below = append([]string{ownerFeature}, below...)
		current = owner
	}
	return feeds
}

// flowsInto reports a target path that walks below, then an action of one of names, to its input.
func flowsInto(target, below []string, names map[string]bool) bool {
	n := len(below)
	return len(target) == n+2 && slices.Equal(target[:n], below) && names[target[n]] && target[n+1] == lower.InputFeature
}

// flowCarriesNothing reports a flow whose own multiplicity admits no flow at all.
func (ctx *Context) flowCarriesNothing(flow lower.FeatureFlow) bool {
	r, ok := ctx.model.semantics.RangeIn(flow.Scope, flow.Multiplicity)
	return ok && r.Upper.Known && !r.Upper.Infinite && r.Upper.Value == 0
}

// flowedInput is the input a flow carries from its source as that stands now, written to the action's
// input; a source holding nothing leaves what was last carried. Two flows into one input are refused.
func (e *ActionExecutor) flowedInput(flows []inputFeed) (*Value, error) {
	if len(flows) > 1 {
		return nil, fmt.Errorf("%w: %s of action %s is the target of %d flows; the dynamics take their input from one",
			errStateSpaceValue, lower.InputFeature, SymbolText(e.action), len(flows))
	}
	feed := flows[0]
	source := strings.Join(feed.Source, ".")
	val, held, err := e.ctx.flowSourceValue(feed)
	if err != nil {
		return nil, fmt.Errorf("%w: %s of action %s: read %s, the source of the flow into it: %w",
			errStateSpaceValue, lower.InputFeature, SymbolText(e.action), source, err)
	}
	if val = soleElement(val); !held || val.Kind == ValNull || val.Kind == ValInvalid {
		return e.carriedInput()
	}
	if !isVectorKind(val) {
		return nil, fmt.Errorf("%w: %s, flowing into %s of action %s, is %s, not a vector",
			errStateSpaceValue, source, lower.InputFeature, SymbolText(e.action), DescribeValue(val))
	}
	if err := e.setFeature(lower.InputFeature, val); err != nil {
		return nil, err
	}
	return &val, nil
}

// carriedInput is the vector a flow last carried into the action's input, nil before any.
func (e *ActionExecutor) carriedInput() (*Value, error) {
	input, ok, err := e.ctx.readBodyValue(e.root.cells, e.root.data, e.root.key(lower.InputFeature))
	if err != nil || !ok {
		return nil, err
	}
	if input = soleElement(input); !isVectorKind(input) {
		return nil, nil
	}
	return &input, nil
}

// flowSourceValue reads the feature a flow leaves, down its source path from the object declaring
// it: an action's from its performance, any other from its value; held is false where it holds nothing.
func (ctx *Context) flowSourceValue(feed inputFeed) (Value, bool, error) {
	holder := feed.on
	path := feed.Source
	for ; len(path) > 1; path = path[1:] {
		if len(path) == 2 {
			if val, held, performs, err := ctx.performedFeatureValue(holder, path[0], path[1]); performs {
				return val, held, err
			}
		}
		next, held, err := ctx.flowSourceObject(holder, path[0])
		if err != nil || !held {
			return Value{}, false, err
		}
		holder = next
	}
	fv, err := holder.MemberValue(ctx, path[0])
	if errors.Is(err, ErrNoValue) {
		return Value{}, false, nil
	}
	if err != nil {
		return Value{}, false, err
	}
	val := fv.HeldValue()
	return val, val.Kind != ValInvalid, nil
}

// flowSourceObject is the one object a feature of holder holds, held false while it holds nothing.
func (ctx *Context) flowSourceObject(holder *Instance, name string) (*Instance, bool, error) {
	fv, err := holder.MemberValue(ctx, name)
	if errors.Is(err, ErrNoValue) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	held := soleElement(fv.HeldValue())
	if held.Kind == ValNull || held.Kind == ValInvalid {
		return nil, false, nil
	}
	id, isObject := held.Object()
	next, found := ctx.Instance(id)
	if !isObject || !found {
		return nil, false, fmt.Errorf("%w: %s holds %s, not one object", errBindingEnd, name, DescribeValue(held))
	}
	return next, true, nil
}

// performedFeatureValue reads feature of the action holder performs as action: from its performance
// under way, else what the ended one left; performs is false where action names no action of holder.
func (ctx *Context) performedFeatureValue(holder *Instance, action, feature string) (val Value, held, performs bool, err error) {
	fv, declared := holder.FeatureValues[action]
	if !declared || fv.Feature == nil || fv.Feature.Symbol == nil || !fv.Feature.Symbol.Kind.IsAction() {
		return Value{}, false, false, nil
	}
	for _, exec := range ctx.actionExecutors {
		if exec.self == holder && exec.performed != nil && exec.performed.Name == action {
			// Dynamics write their output as each step settles; before the first it holds none.
			if exec.dynamicsKind != lower.NotStateSpace && feature == lower.OutputFeature && (exec.dynamics == nil || !exec.dynamics.settled()) {
				return Value{}, false, true, nil
			}
			val, held, err = ctx.readBodyValue(exec.root.cells, exec.root.data, exec.root.key(feature))
			if errors.Is(err, ErrNoValue) {
				return Value{}, false, true, nil
			}
			return val, held, true, err
		}
	}
	return ctx.endedFeatureValue(fv, feature)
}

// endedFeatureValue is what the occurrence of a performance not under way holds for feature,
// read as it stands: held is false before the performance began, or where it left none.
func (ctx *Context) endedFeatureValue(performed *FeatureValue, feature string) (val Value, held, performs bool, err error) {
	id, isObject := soleElement(performed.HeldValue()).Object()
	occurrence, found := ctx.Instance(id)
	if !isObject || !found {
		return Value{}, false, true, nil
	}
	left, declared := occurrence.FeatureValues[feature]
	if !declared {
		return Value{}, false, true, nil
	}
	val = left.HeldValue()
	return val, val.Kind != ValInvalid && val.Kind != ValNull, true, nil
}
