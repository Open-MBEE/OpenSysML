package pssm

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// LogAttribute is the machine attribute the emitted model appends each trace
// segment to; its value at the end of a run is the run's trace.
const LogAttribute = "log"

// MaxSteps is the step budget one run of a translated test has unless the
// environment names another (runtime.MaxStepsEnvVar). The runtime's own default
// lets a runaway machine run for minutes before it reports; a translated test
// that needs more than this many steps is looping, and exhausting the budget is
// a run error, never a trace.
const MaxSteps = 100000

// DefaultBudget is the exploration budget a translated test is run under; a
// test that exhausts it fails, since its reachable traces are then unknown; Event 016 B,
// the largest, has 1152 linearizations, past the runtime's default of 1024 runs.
var DefaultBudget = runtime.ExploreBudget{Runs: 4096, Depth: runtime.DefaultExploreBudget.Depth}

// Execution is what running a translated model under the exploring scheduler
// found, compared against the traces the suite admits.
type Execution struct {
	// Reached are the distinct traces exploration reached, sorted.
	Reached []string
	// Missing are admitted traces no run reached; Extra are reached traces the
	// suite does not admit. Either makes the test fail.
	Missing []string
	Extra   []string
	// Errors are the distinct errors runs ended in, sorted; any makes the test fail.
	Errors []string
	// Runs is how many linearizations were run; Status is how exploration ended.
	Runs     int
	Status   string
	Complete bool
}

// Passed reports whether the reachable traces are exactly the admitted ones and
// every run within a complete exploration came to a trace.
func (x *Execution) Passed() bool {
	return len(x.Missing) == 0 && len(x.Extra) == 0 && len(x.Errors) == 0 && x.Complete
}

// Reasons names why the execution failed, one line each; none for a pass.
func (x *Execution) Reasons() []string {
	var reasons []string
	for _, e := range x.Errors {
		reasons = append(reasons, "run error: "+e)
	}
	if !x.Complete {
		reasons = append(reasons, "exploration "+x.Status)
	}
	for _, t := range x.Extra {
		reasons = append(reasons, "reached a trace the suite does not admit: "+t)
	}
	for _, t := range x.Missing {
		reasons = append(reasons, "admitted trace not reached: "+t)
	}
	return reasons
}

// Execute builds the model's runtime, drives its machine through the tester's
// stimulation once per linearization within budget, jobs at a time, and compares
// the traces reached against expected. An error is a model that builds no runtime or
// an exploration that could not be trusted; a run that fails is recorded, not returned.
func Execute(stop context.Context, m *Model, expected []string, budget runtime.ExploreBudget, jobs int) (*Execution, error) {
	budgets, err := runBudgets()
	if err != nil {
		return nil, err
	}
	machine, fresh, err := build(m, budgets)
	if err != nil {
		return nil, err
	}
	steps, err := driverSteps(m.Events)
	if err != nil {
		return nil, err
	}
	policy, err := runtime.ExplorePolicy(budget)
	if err != nil {
		return nil, err
	}
	run := func(ctx *runtime.Context) (runtime.Outcome, error) {
		exec, err := ctx.CreateStateExecutorFor(machine, nil)
		if err != nil {
			return runtime.Outcome{}, err
		}
		if err := drive(exec, steps); err != nil {
			exec.Release()
			return runtime.Outcome{}, err
		}
		return exec.Outcome(), nil
	}
	exploration, err := runtime.ExploreWith(stop, policy, jobs, fresh, run)
	if err != nil {
		return nil, err
	}
	return compare(exploration, expected), nil
}

// runBudgets is the runtime's bounds with the environment's overrides, as the
// sysml command applies them, except that the step bound defaults to MaxSteps.
func runBudgets() (runtime.Budgets, error) {
	budgets, err := runtime.BudgetsFromEnv()
	if err != nil {
		return runtime.Budgets{}, err
	}
	if strings.TrimSpace(envvar.Lookup(runtime.MaxStepsEnvVar)) == "" {
		budgets.MaxSteps = MaxSteps
	}
	return budgets, nil
}

// build parses the emitted model over the shared standard library and returns
// its machine and a maker of fresh contexts, each job on a runtime model of its
// own: a model's resolver caches are not safe to share between runs.
func build(m *Model, budgets runtime.Budgets) (*symbols.Symbol, func(int) (*runtime.Context, error), error) {
	src := source.New(m.Name, []byte(m.Text))
	p := parser.New(src)
	file := p.ParseFile()
	for _, d := range p.Diagnostics {
		return nil, nil, fmt.Errorf("%s: parse: %s", m.Name, d.Message)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(m.Name, file)
	idx.ExpandWildcardImports()
	matches := idx.LookupQualified(m.Qualified)
	if len(matches) != 1 {
		return nil, nil, fmt.Errorf("%s: %d symbols named %s, want 1", m.Name, len(matches), m.Qualified)
	}
	machine := matches[0]
	if usage, ok := machine.Decl.(*ast.Usage); !ok || usage.Kind != ast.UsageState {
		return nil, nil, fmt.Errorf("%s: %s is a %T, not a state usage", m.Name, m.Qualified, machine.Decl)
	}
	text := source.TextOf(map[string]*source.SourceFile{m.Name: src}, nil)
	var mu sync.Mutex
	models := map[int]*runtime.Model{}
	fresh := func(job int) (*runtime.Context, error) {
		mu.Lock()
		defer mu.Unlock()
		model, ok := models[job]
		if !ok {
			resolver := resolve.New(idx)
			sem := passes.NewTypedModel(resolver)
			sem.SetSourceText(text)
			model = runtime.NewModel(sem, resolver)
			model.SetExpressionParser(parser.ParseOneExpression)
			models[job] = model
		}
		ctx := runtime.NewContext(model, budgets.MaxSteps)
		if err := ctx.SetBudgets(budgets); err != nil {
			return nil, err
		}
		return ctx, nil
	}
	return machine, fresh, nil
}

// compare sorts what exploration reached into the traces the suite admits, the
// ones it does not, and the runs that ended in an error.
func compare(x *runtime.Exploration, expected []string) *Execution {
	admitted := make(map[string]bool, len(expected))
	for _, t := range expected {
		admitted[t] = true
	}
	reached := make(map[string]bool)
	errors := make(map[string]bool)
	for _, o := range x.Outcomes {
		if o.Outcome.Err != nil {
			errors[o.Outcome.Err.Error()] = true
			continue
		}
		reached[o.Outcome.Outputs[LogAttribute].Str()] = true
	}
	ex := &Execution{
		Reached:  sortedKeys(reached),
		Errors:   sortedKeys(errors),
		Runs:     x.Runs,
		Status:   x.Status(),
		Complete: x.Complete(),
	}
	for _, t := range ex.Reached {
		if !admitted[t] {
			ex.Extra = append(ex.Extra, t)
		}
	}
	for _, t := range sortedKeys(admitted) {
		if !reached[t] {
			ex.Missing = append(ex.Missing, t)
		}
	}
	return ex
}

// driverStep is one stimulus with its literals as runtime values.
type driverStep struct {
	event runtime.QueuedEvent
	trace *Expr
	calls map[string]runtime.QueuedEvent
	text  string
}

// driverSteps spells the tester's stimulation as the steps the driver performs.
func driverSteps(stimuli []Stimulus) ([]driverStep, error) {
	steps := make([]driverStep, 0, len(stimuli))
	for _, s := range stimuli {
		step := driverStep{event: runtime.QueuedEvent{Signal: s.Signal, Call: s.Call}, trace: s.Trace, text: s.String()}
		if s.Value != nil {
			v, err := literalValue(s.Value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", s, err)
			}
			step.event.Value = &v
		}
		if len(s.Args) > 0 {
			args, err := argumentValues(s.Args)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", s, err)
			}
			step.event.Args = args
		}
		for id, call := range s.Calls {
			args, err := argumentValues(call.Args)
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", s, call, err)
			}
			if step.calls == nil {
				step.calls = map[string]runtime.QueuedEvent{}
			}
			step.calls[id] = runtime.QueuedEvent{Call: call.Call, Args: args}
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// argumentValues binds a call's literal arguments by parameter name.
func argumentValues(args []Argument) (map[string]runtime.Value, error) {
	values := make(map[string]runtime.Value, len(args))
	for _, a := range args {
		v, err := literalValue(a.Value)
		if err != nil {
			return nil, fmt.Errorf("argument %s: %w", a.Name, err)
		}
		values[a.Name] = v
	}
	return values, nil
}

// drive performs the steps in the tester's order: signals are queued as sent, a
// call returns once its run-to-completion step is done, a trace is logged there.
func drive(exec *runtime.StateExecutor, steps []driverStep) error {
	for _, step := range steps {
		var err error
		switch {
		case step.trace != nil:
			var value runtime.Value
			if value, err = traceValue(exec, step.trace, step.calls); err == nil {
				err = appendLog(exec, value)
			}
		case step.event.Call != "":
			_, err = exec.Call(step.event.Call, step.event.Args)
		default:
			err = exec.Enqueue(step.event)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", step.text, err)
		}
	}
	return exec.RunToCompletion()
}

// appendLog appends a traced segment to the machine's log as the emitted
// machine's own trace statement does: "::"-joined after what is logged already.
func appendLog(exec *runtime.StateExecutor, segment runtime.Value) error {
	if segment.Kind != runtime.ValString {
		return fmt.Errorf("traces a %s, not a String", valueKind(segment))
	}
	log := exec.StateData()[LogAttribute]
	if log.Kind != runtime.ValString {
		return fmt.Errorf("%s holds a %s, not a String", LogAttribute, valueKind(log))
	}
	text := segment.Str()
	if log.Str() != "" {
		text = log.Str() + "::" + text
	}
	return exec.WriteAttribute(LogAttribute, runtime.NewStringValue(text))
}

// traceValue evaluates a traced value: literals, library behaviors and the
// calls it embeds, made in order; an output never returned is empty.
func traceValue(exec *runtime.StateExecutor, x *Expr, calls map[string]runtime.QueuedEvent) (runtime.Value, error) {
	switch x.Kind {
	case ExprLiteral:
		return literalValue(x.Literal)
	case ExprCall:
		call, ok := calls[x.ID]
		if !ok {
			return runtime.Value{}, fmt.Errorf("%s is not a call the stimulation bound", x)
		}
		outputs, err := exec.Call(call.Call, call.Args)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("%s: %w", x, err)
		}
		if v, ok := outputs[x.Result]; ok {
			return v, nil
		}
		return runtime.Value{Kind: runtime.ValNull}, nil
	case ExprApply:
		if x.Library == nil {
			return runtime.Value{}, fmt.Errorf("%s is not a library behavior", x.Name)
		}
		fn, ok := libraryBehaviors[x.Library.Qualified]
		if !ok || len(x.Args) != fn.arity {
			return runtime.Value{}, fmt.Errorf("%s applied to %d arguments has no evaluation", x.Library.Qualified, len(x.Args))
		}
		args := make([]runtime.Value, len(x.Args))
		for i := range x.Args {
			v, err := traceValue(exec, &x.Args[i], calls)
			if err != nil {
				return runtime.Value{}, err
			}
			args[i] = v
		}
		return fn.eval(args)
	}
	return runtime.Value{}, fmt.Errorf("%s has no evaluation", x)
}

// literalValue is the runtime value of a UML literal.
func literalValue(l *Literal) (runtime.Value, error) {
	if l == nil {
		return runtime.Value{}, fmt.Errorf("no literal")
	}
	text := l.String()
	switch l.Kind {
	case LiteralString:
		return runtime.NewStringValue(l.Text), nil
	case LiteralBoolean:
		b, err := strconv.ParseBool(text)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("boolean literal %q: %w", text, err)
		}
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}, nil
	case LiteralInteger:
		i, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("integer literal %q: %w", text, err)
		}
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}, nil
	case LiteralUnlimitedNatural:
		if text == "*" {
			return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}, nil
		}
		i, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return runtime.Value{}, fmt.Errorf("unlimited natural literal %q: %w", text, err)
		}
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}, nil
	case LiteralReal:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsNaN(f) {
			return runtime.Value{}, fmt.Errorf("real literal %q is not a number", text)
		}
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}, nil
	case LiteralNull:
		return runtime.Value{Kind: runtime.ValNull}, nil
	}
	return runtime.Value{}, fmt.Errorf("literal kind %d has no runtime value", l.Kind)
}
