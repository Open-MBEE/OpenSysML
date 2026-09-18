package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// maxActionNestingDepth bounds how deep action-in-action invocation may go. An
// action that reaches itself, directly or through a cycle, would otherwise
// recurse until the process ran out of stack rather than reporting the model
// error, since each nested invocation runs on a fresh executor.
const maxActionNestingDepth = 32

// actionInvocation is a nested action usage that performs another action, in any
// of the three forms the parser produces:
//
//	perform Callee;            // anonymous usage, 'references' relationship
//	action call : Callee;      // named usage, 'typing' relationship
//	action call = Callee(1);   // named usage, invocation expression value
//	perform part.callee;       // a feature chain: the action a part of the performer performs
type actionInvocation struct {
	target *ast.QualifiedName
	args   []ast.Node
	named  []ast.NamedArg
	// chain is the `part.callee` form of the target, whose operand denotes the object
	// performing the callee (SysML 7.17.6); target is then its last member.
	chain *ast.FeatureChainExpr
	// expr is the `Callee(...)` form, whose argument list, even an empty one, states
	// every input the caller passes and selects among same-named actions as for calcs.
	expr *ast.InvocationExpr
	// referrer is the usage owning a reference subsetting, whose own effective
	// name is the one the target names (see resolve.ResolveReferenceTarget).
	referrer ast.Node
	// step is the usage declaring the invocation, whose metadata and parameters
	// bind the performance; nil for an invocation no usage of the body declares.
	step *symbols.Symbol
}

// performed is what the invocation performs: the step declaring it, else the callee itself.
func (inv actionInvocation) performed(callee *symbols.Symbol) *symbols.Symbol {
	if inv.step != nil {
		return inv.step
	}
	return callee
}

// name renders the action as invoked, for diagnostics: the whole chain of a `part.callee`.
func (inv actionInvocation) name() string {
	if inv.chain != nil {
		return lower.FeaturePath(inv.chain)
	}
	return qualifiedNameText(inv.target)
}

// chainedInvocation reads a `part.callee` target: the callee is the chain's last member,
// performed by the object its operand denotes. A chain naming no member performs nothing.
func chainedInvocation(chain *ast.FeatureChainExpr, referrer ast.Node) (actionInvocation, bool) {
	if chain.Member == nil || len(chain.Member.Parts) == 0 {
		return actionInvocation{}, false
	}
	return actionInvocation{target: chain.Member, chain: chain, referrer: referrer}, true
}

// nestedInvocation reports the action a nested usage performs, if any. A usage
// that only carries its own body (assignments, sends, accepts) performs nothing.
// Only typing and reference-subsetting edges name a performed action: the port
// of `accept msg : T via p` is a via edge, not a reference subsetting.
func nestedInvocation(usage *ast.Usage) (actionInvocation, bool) {
	if invocation := usage.PerformedInvocation(); invocation != nil {
		return expressionInvocation(invocation), true
	}
	for _, rel := range usage.Relationships {
		if rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
			continue
		}
		switch target := rel.Target.(type) {
		case *ast.QualifiedName:
			inv := actionInvocation{target: target}
			if rel.Kind == ast.RelReferences {
				inv.referrer = usage
			}
			return inv, true
		case *ast.FeatureChainExpr:
			if rel.Kind != ast.RelReferences {
				continue
			}
			return chainedInvocation(target, usage)
		}
	}
	return actionInvocation{}, false
}

// expressionInvocation reads an invocation expression as the action call it
// writes. A receiver is the first argument, as `seq->size()` is for a calc.
func expressionInvocation(e *ast.InvocationExpr) actionInvocation {
	args := e.Args
	if e.Operand != nil {
		args = append([]ast.Node{e.Operand}, e.Args...)
	}
	return actionInvocation{target: e.Type, args: args, named: e.NamedArgs, expr: e}
}

// invocationArguments resolves the action a `Callee(...)` invocation names and evaluates its
// arguments in ec, the caller's context, keyed by the input parameter of performanceInterface
// they bind. A call the checker leaves tied on arguments of unknown type is settled by the
// values, evaluated once. The other forms resolve the callee and bind nothing.
func invocationArguments(
	ctx *Context, scope *symbols.Scope, inv actionInvocation, ec *EvalContext,
) (map[string]Value, *symbols.Symbol, error) {
	sym, tied, err := actionCandidates(ctx, scope, inv)
	if err != nil {
		return nil, nil, err
	}
	if inv.expr == nil {
		return nil, sym, nil
	}
	if inv.expr.Operand != nil && len(inv.named) > 0 {
		return nil, nil, fmt.Errorf(
			"%w: %s is called with a receiver and named arguments",
			ErrReceiverWithNamedArgs, inv.name(),
		)
	}
	written := writtenArguments(inv.args, inv.named)
	if sym == nil {
		if sym, err = settleAction(ec, inv, tied, written); err != nil {
			return nil, nil, err
		}
	}
	held, err := ctx.performanceInterface(inv.performed(sym), sym)
	if err != nil {
		return nil, nil, err
	}
	in, _ := parameterNames(ctx.actionParametersOf(held))
	arguments := make(map[string]Value, len(written))
	if err := bindArgumentList(ec, inv, held, in, arguments, written); err != nil {
		return nil, nil, err
	}
	return arguments, sym, nil
}

// settleAction selects among the tied actions by the types of the arguments' values,
// evaluated in source order.
func settleAction(ec *EvalContext, inv actionInvocation, tied []*symbols.Symbol, written []*writtenArgument) (*symbols.Symbol, error) {
	args := make([]semantics.Argument, 0, len(written))
	for k, w := range written {
		var name *ast.QualifiedName
		if k >= len(inv.args) {
			name = inv.named[k-len(inv.args)].Name
			if name == nil || len(name.Parts) == 0 {
				continue
			}
		}
		val, err := w.eval(ec)
		if err != nil {
			return nil, fmt.Errorf("eval argument %d of %s: %w", k+1, inv.name(), err)
		}
		args = append(args, ec.valueArgument(val, name))
	}
	sel := ec.ctx.model.semantics.SelectAmongArguments(ec.scope, tied, args, semantics.PerformsAction)
	if sel.Ambiguous || sel.Called() == nil {
		return nil, ambiguousInvocationError(inv.name(), sel.Tied)
	}
	return sel.Called(), nil
}

// invokeAction runs the action named by inv to completion as a sub-execution of
// the caller, performed by self or by the part a `part.callee` target denotes, and
// returns the values its features ended with and, among them, its output parameters'.
//
// The callee gets a fresh executor with its own tokens, so values cross the
// boundary only through parameters: arguments, evaluated in the caller's data (or,
// for an argument-less invocation, caller values of the same name) seed the
// callee's `in` and `inout` parameters, and its `out` and `inout` parameters come
// back to the caller. An action with no parameters therefore reads and writes
// nothing in its caller.
//
// A body around the call pauses where the callee waits, and the call is
// re-entered to go on with it.
func invokeAction(
	ctx *Context,
	scope *symbols.Scope,
	inv actionInvocation,
	data map[string]Value,
	self *Instance,
) (features, outputs map[string]Value, err error) {
	if callee, resumed, err := popFrame[*calleeFrame](ctx); err != nil {
		return nil, nil, err
	} else if resumed {
		return ctx.runCallee(callee)
	}
	// Arguments are evaluated as the caller's body is: over its values, performed by self.
	ec := NewEvalContextIn(ctx, scope, self)
	ec.inBehaviorBody = true
	ec.Push(data)
	defer ec.beginStep()()
	arguments, sym, err := invocationArguments(ctx, scope, inv, ec)
	if err != nil {
		return nil, nil, err
	}
	performer, err := ctx.performerOf(ec, inv, self)
	if err != nil {
		return nil, nil, err
	}
	return invokeBoundAction(ctx, inv, sym, arguments, data, performer)
}

// performerOf is the object the callee runs as: for a `part.callee` target, the one
// object the chain's operand denotes over the caller's values in ec; else self.
func (ctx *Context) performerOf(ec *EvalContext, inv actionInvocation, self *Instance) (*Instance, error) {
	if inv.chain == nil {
		return self, nil
	}
	value, err := ec.Eval(inv.chain.Operand)
	if err != nil {
		return nil, fmt.Errorf("eval performer of %s: %w", inv.name(), err)
	}
	if value.Kind != ValInstance {
		return nil, fmt.Errorf("%w: %s is performed by %s, which is no one object",
			ErrPerformerNotObject, inv.name(), FormatValue(value))
	}
	performer, ok := ctx.Instance(value.Instance)
	if !ok {
		return nil, fmt.Errorf("%w: %s is performed by object #%d, which no longer exists",
			ErrPerformerNotObject, inv.name(), value.Instance)
	}
	return performer, nil
}

// invokeBoundAction is invokeAction with the callee sym resolved and its inputs already
// bound in pins (the performing node's, arguments included); a bare `perform`/typed usage
// still reads data.
func invokeBoundAction(
	ctx *Context,
	inv actionInvocation,
	sym *symbols.Symbol,
	pins map[string]Value,
	data map[string]Value,
	self *Instance,
) (features, outputs map[string]Value, err error) {
	if callee, resumed, err := popFrame[*calleeFrame](ctx); err != nil {
		return nil, nil, err
	} else if resumed {
		return ctx.runCallee(callee)
	}
	if ctx.actionDepth >= maxActionNestingDepth {
		return nil, nil, fmt.Errorf(
			"action invocation nested more than %d deep at %s (recursive action?)",
			maxActionNestingDepth, inv.name(),
		)
	}

	params, err := ctx.performanceParameters(inv.performed(sym), sym)
	if err != nil {
		return nil, nil, err
	}
	in, out := parameterNames(params)
	inputs := make(map[string]Value, len(in))
	for _, name := range in {
		if value, ok := pins[name]; ok {
			inputs[name] = value
		}
	}
	if inv.expr == nil {
		// A bare `perform`/typed usage reads the caller's values of the parameters'
		// own names, which is how data reaches an action performed inside a flow.
		for _, name := range in {
			if _, bound := inputs[name]; bound {
				continue
			}
			if value, ok := data[name]; ok {
				inputs[name] = value
			}
		}
	}
	if err := checkInputsBound(inv, params, inputs); err != nil {
		return nil, nil, err
	}

	callee, err := ctx.beginOrJoinCallee(inv, sym, self, inputs)
	if err != nil {
		return nil, nil, fmt.Errorf("invoke action %s: %w", inv.name(), err)
	}
	callee.name, callee.out = inv.name(), out
	return ctx.runCallee(callee)
}

// calleeFrame is an action a body performs as a sub-execution, kept where the body
// paused on the action's wait: name is the action as invoked and out the names of
// its output parameters, read once it completes. joined marks the performance the
// performer already runs of the action, which the caller waits on rather than owns.
type calleeFrame struct {
	exec   *ActionExecutor
	name   string
	out    []string
	joined bool
}

func (f *calleeFrame) abandon(*Context) {
	if !f.joined {
		f.exec.Release()
	}
}

func (f *calleeFrame) clone() bodyFrame { c := *f; return &c }

// beginOrJoinCallee begins a performance of the action inv names on self, or, for a
// `part.callee` whose object performs the callee already (its type performs it), joins
// that one performance, as a run of the action named on an object does (performAction).
func (ctx *Context) beginOrJoinCallee(inv actionInvocation, sym *symbols.Symbol, self *Instance, inputs map[string]Value) (*calleeFrame, error) {
	if inv.chain != nil {
		exec, err := performanceOf(sym, self, inputs)
		if err != nil {
			return nil, err
		}
		if exec != nil {
			return &calleeFrame{exec: exec, joined: true}, nil
		}
	}
	return ctx.beginCallee(inv.performed(sym), sym, self, inputs)
}

// beginCallee starts action, a performance of performed, as a sub-execution of
// the caller nested one deeper, on the clock until run to completion.
func (ctx *Context) beginCallee(performed, action *symbols.Symbol, self *Instance, inputs map[string]Value) (*calleeFrame, error) {
	ctx.actionDepth++
	defer func() { ctx.actionDepth-- }()
	defer ctx.nestRun()()
	exec, err := ctx.beginPerformed(performed, action, self, inputs, false, startActionStep)
	if err != nil {
		return nil, err
	}
	return &calleeFrame{exec: exec}, nil
}

// runCallee runs the sub-execution to completion, nested one deeper while it runs,
// and returns the values its features ended with and those of its outputs; a body
// around it pauses on the action's waits, keeping the frame to go on from.
func (ctx *Context) runCallee(callee *calleeFrame) (features, outputs map[string]Value, err error) {
	ctx.actionDepth++
	defer func() { ctx.actionDepth-- }()
	defer ctx.nestRun()()
	if err := ctx.runCalleePerformance(callee); err != nil {
		if paused(err) {
			return nil, nil, ctx.pausing(callee, err)
		}
		return nil, nil, fmt.Errorf("invoke action %s: %w", callee.name, err)
	}
	features = callee.exec.root.data
	outputs = make(map[string]Value, len(callee.out))
	for _, name := range callee.out {
		if value, ok := features[name]; ok {
			outputs[name] = value
		}
	}
	return features, outputs, nil
}

// runCalleePerformance runs the callee's performance on; one the caller owns leaves
// the clock once run, one joined stays the object's, on the clock as its type binds it.
func (ctx *Context) runCalleePerformance(callee *calleeFrame) error {
	if callee.joined {
		return ctx.runPerformance(callee.exec, false)
	}
	return ctx.runPerformed(callee.exec, false)
}

// resolveActionSymbol is the action inv names; a call only its arguments' values can
// settle is refused as ambiguous.
func resolveActionSymbol(
	ctx *Context,
	scope *symbols.Scope,
	inv actionInvocation,
) (*symbols.Symbol, error) {
	sym, tied, err := actionCandidates(ctx, scope, inv)
	if err != nil {
		return nil, err
	}
	if sym == nil {
		return nil, ambiguousInvocationError(inv.name(), tied)
	}
	return sym, nil
}

// actionCandidates resolves the action inv names: the one it denotes, or, for a call the
// checker leaves tied on arguments of unknown type, nil and the tied actions.
func actionCandidates(
	ctx *Context,
	scope *symbols.Scope,
	inv actionInvocation,
) (*symbols.Symbol, []*symbols.Symbol, error) {
	target := inv.target
	if target == nil || len(target.Parts) == 0 {
		return nil, nil, fmt.Errorf("empty action reference")
	}
	name := qualifiedNameText(target)
	if scope == nil || ctx.model.resolver == nil {
		return nil, nil, fmt.Errorf("cannot resolve action %s: no scope", name)
	}
	var sym *symbols.Symbol
	var ok bool
	switch {
	case inv.chain != nil:
		sym, ok = ctx.resolveReferenceTarget(scope, inv.referrer, inv.chain)
	case inv.referrer != nil:
		sym, ok = ctx.resolveReferenceTarget(scope, inv.referrer, target)
	case inv.expr != nil:
		sel, err := ctx.selectInvocation(scope, inv.expr, semantics.PerformsAction)
		if err != nil {
			return nil, nil, err
		}
		switch {
		case sel.Ambiguous && sel.Undetermined:
			return nil, sel.Tied, nil
		case sel.Ambiguous:
			return nil, nil, ambiguousInvocationError(name, sel.Tied)
		}
		sym = sel.Called()
		ok = sym != nil
	default:
		sym, ok = ctx.resolveQualified(scope, target)
	}
	if !ok || sym == nil {
		return nil, nil, fmt.Errorf("unresolved action reference: %s", name)
	}
	if inv.referrer != nil && sym.Decl == inv.referrer {
		return nil, nil, fmt.Errorf("unresolved action reference: %s (a perform statement cannot perform itself)", name)
	}
	if !ctx.model.semantics.Performable(semantics.PerformsAction, sym) {
		return nil, nil, fmt.Errorf("%s is not an action (%v)", name, sym.Kind)
	}
	return sym, nil, nil
}

// bindArgumentList binds an invocation's arguments, written, into inputs by the callee's
// parameter order (positional) or names (named); each is evaluated in ec, the caller's
// context, unless settling the callee already did. A parameter two arguments would bind
// is rejected rather than taking the later one.
func bindArgumentList(ec *EvalContext, inv actionInvocation, callee *symbols.Symbol, in []string, inputs map[string]Value, written []*writtenArgument) error {
	if len(inv.args) > len(in) {
		return fmt.Errorf(
			"%w: action %s takes %d input parameter(s), got %d argument(s)",
			ErrActionArity, inv.name(), len(in), len(inv.args),
		)
	}
	bound := make(map[string]bool, len(written))
	for i := range inv.args {
		value, err := written[i].eval(ec)
		if err != nil {
			return fmt.Errorf("eval argument %d of %s: %w", i+1, inv.name(), err)
		}
		inputs[in[i]] = value
		bound[in[i]] = true
	}

	names, unbound := ec.ctx.boundParameterNames(ec.scope, callee, inv.named)
	for i := range inv.named {
		name := names[i]
		if name == "" {
			return fmt.Errorf("unnamed argument in invocation of %s", inv.name())
		}
		if err := unbound[i]; err != nil {
			return err
		}
		if !contains(in, name) {
			return fmt.Errorf(
				"%w: action %s has no input parameter %q",
				ErrUnknownParameter, inv.name(), name,
			)
		}
		if bound[name] {
			return fmt.Errorf(
				"%w: input parameter %q of %s is given more than one argument",
				ErrDuplicateArgument, name, inv.name(),
			)
		}
		bound[name] = true
		value, err := written[len(inv.args)+i].eval(ec)
		if err != nil {
			return fmt.Errorf("eval argument %q of %s: %w", name, inv.name(), err)
		}
		inputs[name] = value
	}
	return nil
}

// actionParameter is one parameter an action declares.
type actionParameter struct {
	Name string
	// Direction is the parameter's declared direction, which decides whether the
	// caller writes it, reads it back, or both.
	Direction ast.FeatureDirection
	// Optional reports whether an invocation may bind no argument to the parameter: it
	// or a parameter it redefines gives a value, or its multiplicity admits none.
	Optional bool
	// IsResult marks the `return` parameter, what the action's value read yields.
	IsResult bool
}

// actionParametersOf returns an action's parameters in invocation order: its own, then
// the inherited ones none redefines (KerML 7.4.7.2) — the signature the type checker uses.
func (ctx *Context) actionParametersOf(sym *symbols.Symbol) []actionParameter {
	var params []actionParameter
	for _, param := range ctx.model.semantics.BehaviorParametersOf(sym) {
		if param.Symbol == nil || param.Symbol.Name == "" {
			continue
		}
		params = append(params, actionParameter{
			Name:      param.Symbol.Name,
			Direction: param.Direction,
			Optional:  ctx.model.semantics.OptionalParameter(param.Symbol),
			IsResult:  param.IsResult,
		})
	}
	return params
}

// parameterNames splits parameters into those the caller writes and reads back.
func parameterNames(params []actionParameter) (in, out []string) {
	for _, param := range params {
		switch param.Direction {
		case ast.DirIn:
			in = append(in, param.Name)
		case ast.DirOut:
			out = append(out, param.Name)
		case ast.DirInOut:
			in = append(in, param.Name)
			out = append(out, param.Name)
		}
	}
	return in, out
}

func contains(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

// qualifiedNameText renders a qualified name as written, for diagnostics.
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, part := range qn.Parts {
		parts = append(parts, part.Text)
	}
	return strings.Join(parts, "::")
}
