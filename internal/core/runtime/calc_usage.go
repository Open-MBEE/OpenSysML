package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A calc computes as many values as it declares output features, but an
// invocation of it yields exactly one result (KerML 7.4.9). Several outputs are
// therefore read from a calc usage rather than from an invocation: the usage's
// body runs once and each `out` feature is a value readable by name.
//
//	calc def Two { in n; out a = n + 1; out b = n * 2; }
//	calc c : Two { in n = 5; }
//	attribute z = c.b;   // 10
//
// This file evaluates such a usage: its inputs bind from its own member values
// and the defaults along its specialization chain, its computation runs through
// the same lowered statement engine an invocation uses, and its outputs are
// evaluated on demand against the environment that computation left behind.

// calcOutput is one output feature of a calc: an `out` parameter, or the result
// parameter a `return` declares. Its value is a feature of the calc, read by
// name, rather than a statement of the calc's body.
type calcOutput struct {
	Name     string          // feature name the output answers to, empty for an anonymous result parameter
	Value    ast.Node        // value-binding expression, nil when the calc gives the output no value
	Owner    *symbols.Symbol // the calc declaring the binding, whose scope it is written in
	IsResult bool            // declared with `return`: the result an invocation of the calc yields
	// IsInOut marks an `inout` parameter: an output feature (KerML 7.4.9) bound by
	// the invocation rather than by a declaration or an assignment.
	IsInOut bool
	// IsInitial marks a value declared with `:=`: what the output holds when the
	// body starts, which the body's assignments may then replace.
	IsInitial bool
	Decl      calcMemberDecl // the declaration, closest to the invoked calc, the output's value answers to
}

// calcOutputs flattens the output features declared along chain (most general
// first), as calcParameters does the inputs, recording renamed ones in aliases.
func (ctx *Context) calcOutputs(chain []*symbols.Symbol, aliases *map[string]string) []calcOutput {
	var outs []calcOutput
	index := make(map[string]int)

	for _, link := range chain {
		for _, member := range declMembers(link.Decl) {
			usage, ok := member.(*ast.Usage)
			if !ok {
				continue
			}
			if usage.Direction != ast.DirOut && usage.Direction != ast.DirInOut && !usage.IsResult {
				continue
			}
			// An output written as a redefinition names the one it overrides.
			name, _ := ast.EffectiveName(usage)
			sym := memberSymbol(declScope(link), usage)
			out := calcOutput{Name: name, Value: usage.Value, Owner: link, IsResult: usage.IsResult,
				IsInitial: usage.Value != nil && usage.ValueIsInitial, Decl: ctx.calcMemberDeclOf(link, sym, name)}
			if usage.Direction == ast.DirInOut {
				// The value an `inout` declares is the default of the parameter, not a
				// binding of the output: the invocation binds it either way.
				out.IsInOut, out.Value = true, nil
			}

			var at int
			var seen bool
			if name != "" {
				at, seen = ctx.redeclaredIndex(index, sym, name)
			} else {
				// An anonymous result parameter has no name to redeclare by, so
				// it refines the anonymous result it inherits.
				at, seen = indexOfAnonymousResult(outs)
			}
			if seen {
				if out.Value == nil {
					out.Value = outs[at].Value
					out.Owner = outs[at].Owner
					out.IsInitial = outs[at].IsInitial
				}
				out.Decl = out.Decl.redeclaring(outs[at].Decl)
				if name != "" {
					*aliases = aliasRedefined(*aliases, outs[at].Name, name)
					index[name] = at
				}
				outs[at] = out
				continue
			}
			if name != "" {
				index[name] = len(outs)
			}
			outs = append(outs, out)
		}
	}
	return outs
}

// resultOutput returns the result parameter a `return` declares, if the calc has one.
func (shape *calcShape) resultOutput() *calcOutput {
	for i := range shape.Outputs {
		if shape.Outputs[i].IsResult {
			return &shape.Outputs[i]
		}
	}
	return nil
}

// hasInitialOutput reports whether some output starts the body holding a value
// (`:=`), which the calc yields when the body leaves it as it is.
func (shape *calcShape) hasInitialOutput() bool {
	for _, out := range shape.Outputs {
		if out.IsInitial {
			return true
		}
	}
	return false
}

// indexOfAnonymousResult finds the unnamed result parameter among outs, which is
// the one an unnamed redeclaration refines.
func indexOfAnonymousResult(outs []calcOutput) (int, bool) {
	for i, out := range outs {
		if out.Name == "" {
			return i, true
		}
	}
	return 0, false
}

// calcSteps is the computation an invoked calc runs: its lowered body without
// the value bindings of its `out` features. An `out` binding states what that
// feature is, not a step of the body, so a calc declaring several of them
// returns none of them by falling off the end of its body.
func calcSteps(body []lower.Statement) []lower.Statement {
	steps := make([]lower.Statement, 0, len(body))
	for _, stmt := range body {
		if ret, ok := stmt.(lower.Return); ok && isOutputBinding(ret.Node) {
			continue
		}
		steps = append(steps, stmt)
	}
	return steps
}

// assignedOutputs are the outputs the body's statements assign, on any path
// through it: what the calc computes, whichever way an execution branches.
func assignedOutputs(stmts []lower.Statement, outputs []calcOutput, aliases map[string]string) map[string]bool {
	declared := make(map[string]string, len(outputs)+len(aliases))
	for _, out := range outputs {
		if out.Name != "" && !out.IsInOut {
			declared[out.Name] = out.Name
		}
	}
	for alias, name := range aliases {
		if _, ok := declared[name]; ok {
			declared[alias] = name
		}
	}
	assigned := make(map[string]bool)
	collectAssignedOutputs(stmts, declared, assigned)
	return assigned
}

// collectAssignedOutputs walks the statements, and the blocks they carry, for
// assignments to a declared output, by any name declared maps to it. A chained
// target writes a feature of another object, so it binds no output of the calc.
func collectAssignedOutputs(stmts []lower.Statement, declared map[string]string, assigned map[string]bool) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case lower.Assign:
			if name, ok := declared[s.Target]; ok && s.Chain == nil {
				assigned[name] = true
			}
		case lower.Block:
			collectAssignedOutputs(s.Steps(), declared, assigned)
		case lower.Loop:
			collectAssignedOutputs(s.Body.Steps(), declared, assigned)
		case lower.If:
			collectAssignedOutputs(s.Then.Steps(), declared, assigned)
			if s.Else != nil {
				collectAssignedOutputs(s.Else.Steps(), declared, assigned)
			}
		}
	}
}

// isOutputBinding reports whether a lowered return states an `out` feature's
// value rather than a `return`. A result parameter stays a return: it is the
// one value the calc designates.
func isOutputBinding(node ast.Node) bool {
	usage, ok := node.(*ast.Usage)
	return ok && usage.Direction == ast.DirOut && !usage.IsResult
}

// output finds the output feature of that name.
func (shape *calcShape) output(name string) (calcOutput, bool) {
	if name == "" {
		return calcOutput{}, false
	}
	name = canonical(shape.Aliases, name)
	for _, out := range shape.Outputs {
		if out.Name == name {
			return out, true
		}
	}
	if name == resultOutputName {
		return shape.anonymousResult()
	}
	return calcOutput{}, false
}

// resultOutputName is the name an unnamed result parameter answers to.
const resultOutputName = "result"

// anonymousResult is the unnamed result a `return`, a trailing result expression
// or a body returning a value produces, which reads under the name `result`.
func (shape *calcShape) anonymousResult() (calcOutput, bool) {
	for _, out := range shape.Outputs {
		if out.IsResult && out.Name == "" {
			return calcOutput{Name: resultOutputName, Value: out.Value, Owner: out.Owner, Decl: out.Decl, IsResult: true}, true
		}
	}
	if shape.ResultExpr != nil {
		return calcOutput{Name: resultOutputName, Value: shape.ResultExpr, Owner: shape.Sym, IsResult: true}, true
	}
	if shape.resultOutput() == nil && lower.Returns(shape.Steps) {
		// The body's `return` binds the result, so the run holds it and nothing evaluates it.
		return calcOutput{Name: resultOutputName, Owner: shape.BodyOwner, IsResult: true}, true
	}
	return calcOutput{}, false
}

// returnsResult reports a calc usage whose value is the unnamed result it
// returns, so naming the usage itself reads that result.
func (ctx *Context) returnsResult(sym *symbols.Symbol) bool {
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return false
	}
	for _, out := range shape.Outputs {
		if out.Name == resultOutputName {
			return false
		}
	}
	_, anonymous := shape.anonymousResult()
	return anonymous
}

// memberName is the name a run of the calc binds the resolved member sym under: a
// parameter, output or local its chain declares, or an inherited result parameter
// (`Case::result`) as the result the calc designates.
func (shape *calcShape) memberName(ctx *Context, sym *symbols.Symbol) (string, bool) {
	if shape.members == nil {
		shape.members = ctx.calcMemberNames(shape)
	}
	if name, ok := shape.members[sym]; ok {
		return name, true
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.IsResult && ctx.libraryDeclared(sym) {
		if out := shape.resultOutput(); out != nil && out.Name != "" {
			return out.Name, true
		}
		if _, ok := shape.anonymousResult(); ok {
			return resultOutputName, true
		}
	}
	return "", false
}

// qualifiedBy reports whether a name qualified by qualifier (`MassCase::result`,
// `Cases::Case::result`) denotes this calc's run: the calc itself or one it specializes.
func (shape *calcShape) qualifiedBy(ctx *Context, qualifier *symbols.Symbol) bool {
	return ctx.isOrSpecializes(shape.Sym, qualifier)
}

// isOrSpecializes reports sym being general, or inheriting its members from it.
func (ctx *Context) isOrSpecializes(sym, general *symbols.Symbol) bool {
	if sym == nil || general == nil {
		return false
	}
	if sym == general {
		return true
	}
	for _, source := range ctx.model.MemberSources(sym) {
		if source == general {
			return true
		}
	}
	return false
}

// calcMemberNames indexes the named members declared along shape's chain by the
// name the run binds each under, a renamed redeclaration's name included.
func (ctx *Context) calcMemberNames(shape *calcShape) map[*symbols.Symbol]string {
	members := make(map[*symbols.Symbol]string)
	for _, link := range ctx.calcChain(shape.Sym) {
		for _, member := range declMembers(link.Decl) {
			var name string
			if subject, ok := subjectDeclaration(member); ok {
				name = subject.Name
			} else if usage, ok := member.(*ast.Usage); ok {
				name, _ = ast.EffectiveName(usage)
			}
			sym := memberSymbol(declScope(link), member)
			if name == "" || sym == nil {
				continue
			}
			members[sym] = canonical(shape.Aliases, name)
		}
	}
	return members
}

// resultSegments is the member path reading the unnamed result of a usage.
var resultSegments = []ast.NameSegment{{Text: resultOutputName}}

// outputNames renders the output features the calc declares, for a diagnostic
// about one it does not.
func (shape *calcShape) outputNames() string {
	names := make([]string, 0, len(shape.Outputs))
	for _, out := range shape.Outputs {
		if out.Name != "" {
			names = append(names, out.Name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// declaresUsage reports whether the calc's body declares a calc or analysis
// usage named name among its own members, which a read from outside may name.
func (shape *calcShape) declaresUsage(name string) bool {
	for _, stmt := range shape.Body {
		if decl, ok := stmt.(lower.DeclareUsage); ok && decl.Name == name {
			return true
		}
	}
	return false
}

// designatedOutput returns the output feature an invocation of the calc yields
// as its result: the one declared with `return`, or, failing that, the only
// output the calc binds a value to. A calc binding several of them designates
// none, so its values are read from a calc usage's output features instead of
// being narrowed to whichever output comes first.
func (shape *calcShape) designatedOutput() (calcOutput, error) {
	if shape.ResultExpr != nil {
		// The bound value is the declared result's, so it answers to that declaration.
		out := calcOutput{Name: "result", Value: shape.ResultExpr, Owner: shape.Sym, IsResult: true}
		if declared := shape.resultOutput(); declared != nil {
			out.Decl = declared.Decl
		}
		return out, nil
	}
	var valued []calcOutput
	for _, out := range shape.Outputs {
		if out.IsResult {
			if out.Value == nil {
				break
			}
			return out, nil
		}
		// An output the body assigns is computed as much as one a declaration binds,
		// so it is a candidate result of an invocation just the same.
		if out.Value != nil || shape.BodyOutputs[out.Name] {
			valued = append(valued, out)
		}
	}

	switch len(valued) {
	case 0:
		// The calc computed nothing to hand back: a body that fell off its end,
		// or outputs none of which states a value.
		return calcOutput{}, fmt.Errorf("%w: %s ended without a return", ErrCalcNoReturn, shape.Label)
	case 1:
		return valued[0], nil
	default:
		names := make([]string, 0, len(valued))
		for _, out := range valued {
			names = append(names, out.Name)
		}
		return calcOutput{}, fmt.Errorf(
			"%w: %s computes %d output features (%s) and designates no result; read them from a usage instead: %s",
			ErrAmbiguousResult, shape.Label, len(valued), strings.Join(names, ", "), shape.usageSpelling(valued[0].Name),
		)
	}
}

// usageSpelling writes the calc usage a modeler declares to read the outputs of
// a calc that designates no result, with the inputs it has to bind.
func (shape *calcShape) usageSpelling(output string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s c : %s {", shape.Kind, shape.Name)
	for _, param := range shape.Params {
		fmt.Fprintf(&b, " in %s = ...;", param.Name)
	}
	fmt.Fprintf(&b, " } then read c.%s", output)
	return b.String()
}

// calcUsageKey identifies one evaluation of one calc usage within an activation:
// the usage and the object whose state it reads. The bound input values are not
// part of it, so a read after an assignment to a feature an input names cannot
// key a second evaluation and observe a different binding.
type calcUsageKey struct {
	sym      *symbols.Symbol
	instance int64
}

// calcRun is one evaluation of a calc: the environment its computation left
// behind and the outputs read from it so far. The computation runs once, when
// the run is created, so reading several outputs of a calc usage does not run
// its body again.
type calcRun struct {
	shape *calcShape
	scope *symbols.Scope // scope of whoever reads the usage, for a calc that owns none
	self  *Instance      // object the usage is a feature of, nil when unbound
	env   frame
	// outer is the environment of the evaluation reading the usage, for a usage
	// nested in a calc: the bindings the usage itself declares are written there.
	outer  *EvalContext
	result Value
	// returned reports whether the computation returned a value, which a calc
	// whose outputs are all features does not.
	returned bool
	outputs  map[string]Value
	// computing holds the outputs whose bindings are being evaluated, so an
	// output written in terms of itself is reported as a cycle rather than
	// evaluated until the step budget runs out.
	computing map[string]bool
	// onStack reports whether this evaluation already holds a nesting feature value, so
	// the outputs of it that name each other do not count a level apiece.
	onStack bool
	// activation is the execution this run is, so a usage its outputs read is
	// evaluated once for the whole run rather than once per output.
	activation int64
	// perf is the case's performance, whose steps an output binding reads by name
	// (`step.pin`); nil for a calc, which performs none.
	perf *actionFrame
}

// newCalcRun holds the environment one evaluation of a calc computed.
func newCalcRun(shape *calcShape, scope *symbols.Scope, self *Instance, env frame) *calcRun {
	return &calcRun{
		shape:     shape,
		scope:     scope,
		self:      self,
		env:       env,
		outputs:   make(map[string]Value),
		computing: make(map[string]bool),
	}
}

// detached is this evaluation over storage of its own, for a body that outlives
// the invocation whose frame it ran in; its outputs stay memoized in common.
func (run *calcRun) detached() *calcRun {
	if run == nil {
		return nil
	}
	out := *run
	out.env = run.env.snapshot()
	if run.outer != nil {
		out.outer = run.outer.closure()
	}
	out.onStack = false
	return &out
}

// CalcOutputValue is the value one output feature of a calc usage took, in the
// order the calc declares its outputs.
type CalcOutputValue struct {
	Name  string
	Value Value
}

// CalcUsageOutput evaluates a calc usage and returns the value of one of its
// output features. The usage's inputs bind from its own member values, falling
// back to the defaults declared along its specialization chain; self, when
// non-null, is the object the usage is a feature of, whose feature values the inputs may
// name. Reading several outputs of the same usage runs its body once.
func (ctx *Context) CalcUsageOutput(sym *symbols.Symbol, name string, scope *symbols.Scope, self *Instance) (Value, error) {
	defer ctx.beginRun()()

	run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, scope, self), sym)
	if err != nil {
		return Value{}, err
	}
	return run.output(ctx, name)
}

// CalcUsageOutputs evaluates a calc usage and returns the value of every output
// feature it declares, in declaration order, from one run of its body.
func (ctx *Context) CalcUsageOutputs(sym *symbols.Symbol, scope *symbols.Scope, self *Instance) ([]CalcOutputValue, error) {
	defer ctx.beginRun()()

	run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, scope, self), sym)
	if err != nil {
		return nil, err
	}
	values := make([]CalcOutputValue, 0, len(run.shape.Outputs))
	for _, out := range run.shape.Outputs {
		if out.Name == "" {
			continue
		}
		value, err := run.output(ctx, out.Name)
		if err != nil {
			return nil, err
		}
		values = append(values, CalcOutputValue{Name: out.Name, Value: value})
	}
	return values, nil
}

// calcUsageRun returns the evaluation of a calc usage that the values read from
// it come from, running its body the first time it is needed. reader is the
// evaluation reading the usage, whose environment the usage's own input
// bindings are evaluated in. The run is memoized per usage, object and reading
// activation, so every output read of one usage within one activation answers
// from one execution of its body, while another activation gets its own.
func (ctx *Context) calcUsageRun(reader *EvalContext, sym *symbols.Symbol) (*calcRun, error) {
	if !isCalcUsageSymbol(sym) {
		return nil, fmt.Errorf(
			"%w: %s does not evaluate output features", ErrNotACalcUsage, ctx.qualifiedSymbolName(sym),
		)
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, err
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, err
	}

	if err := ctx.enterCalc(shape.Name); err != nil {
		return nil, err
	}
	defer ctx.leaveCalc()

	// The evaluation is looked up before the inputs are bound, so a usage already
	// evaluated in this activation is not re-bound: its outputs all answer from
	// the binding its first read established.
	key := calcUsageKey{sym: sym}
	if reader.self != nil {
		key.instance = reader.self.ID
	}
	// A binding of the usage that reads the usage itself reads this same evaluation.
	if run := reader.calcRun; run != nil && run.shape.reads(sym) {
		return run, nil
	}
	if run, ok := ctx.run.calcUsageRuns[reader.activation][key]; ok {
		if ctx.trace != nil {
			ctx.trace.RecordCalcUsageReuse(shape.Kind, shape.Name)
		}
		return run, nil
	}

	leave, err := ctx.enterCalcUsage(shape, key)
	if err != nil {
		return nil, err
	}
	defer leave()
	ec, nested, env, err := ctx.bindCalcUsage(shape, reader, calcArgs{})
	if err != nil {
		return nil, err
	}
	run, err := ctx.runCalcUsage(shape, ec, nested, env, reader)
	if err != nil {
		return nil, err
	}
	runs, ok := ctx.run.calcUsageRuns[reader.activation]
	if !ok {
		runs = make(map[calcUsageKey]*calcRun)
		ctx.run.calcUsageRuns[reader.activation] = runs
	}
	runs[key] = run
	return run, nil
}

// enterCalcUsage marks the usage's body as running until leave is called; a usage
// read while its own body runs would run itself without end, so that is an error.
func (ctx *Context) enterCalcUsage(shape *calcShape, key calcUsageKey) (leave func(), err error) {
	if running := ctx.calcUsageRunning[key]; running != nil && running.reads(shape.Sym) {
		return nil, fmt.Errorf("%w: %s reads itself while its body runs", ErrCalcUsageRecursion, shape.Label)
	}
	ctx.calcUsageRunning[key] = shape
	return func() { delete(ctx.calcUsageRunning, key) }, nil
}

// reads reports whether sym, read from within the body of the usage this is the
// shape of, is that usage itself rather than a same-named usage its body declares
// (a `calc next : Down` inside Down, read while a `next` runs, is the next level down).
func (shape *calcShape) reads(sym *symbols.Symbol) bool {
	return shape.Sym == sym && sym.OwnerScope != shape.bodyScope()
}

// bodyUsageSymbol resolves the usage a body-local declaration declares, in the
// scope the declaration was written in.
func (ctx *Context) bodyUsageSymbol(stmt lower.DeclareUsage) (*symbols.Symbol, error) {
	if ctx.resolver == nil {
		return nil, fmt.Errorf("%w: calc usage %s needs a resolved model", ErrNotACalcUsage, stmt.Name)
	}
	sym, ok := ctx.resolver.LookupName(stmt.Scope, stmt.Name)
	if !ok || !isCalcUsageSymbol(sym) {
		return nil, fmt.Errorf(
			"%w: calc usage %s is declared in this body but is not resolved to one",
			ErrNotACalcUsage, stmt.Name,
		)
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, err
	}
	return sym, nil
}

// forgetCalcUsage drops the evaluation of one calc usage an activation holds, so
// the next read of it in that activation evaluates its body again.
func (ctx *Context) forgetCalcUsage(activation int64, sym *symbols.Symbol) {
	runs, ok := ctx.run.calcUsageRuns[activation]
	if !ok {
		return
	}
	for key, run := range runs {
		if key.sym == sym {
			ctx.endActivation(run.activation)
			delete(runs, key)
		}
	}
}

// bindCalcUsage binds a calc usage's inputs from args and its own declarations,
// answering with the environment the usage's body runs in, the environment
// reading it (null unless it is nested in a calc), and the bindings themselves.
func (ctx *Context) bindCalcUsage(shape *calcShape, reader *EvalContext, args calcArgs) (*EvalContext, *EvalContext, frame, error) {
	ec := NewEvalContextIn(ctx, ctx.calcScope(shape.BodyOwner, shape.Sym, reader.scope), reader.self)
	if ec.trace != nil {
		ec.trace.RecordCalculationEnter(shape.Kind, shape.Name)
	}

	env := frame{vars: make(map[string]Value, len(shape.Params)), aliases: shape.Aliases, owner: shape, run: ctx.newRun()}
	ec.pushFrame(env)

	// A usage declared in a behavior's body is written in that body, so its own
	// bindings see the values the evaluation reading it holds, and none of the
	// inputs being bound here, so every input resolves names in the enclosing
	// environment alike.
	var nested *EvalContext
	if enclosedByBehaviorBody(shape.Sym) {
		nested = reader.nestedEnv(ctx.calcScope(shape.Sym, shape.Sym, reader.scope))
	}

	// Read as a feature, a usage passes no arguments: every input binds from the
	// value the usage or its definition declares for it.
	if err := ctx.bindCalcParameters(shape, ec, args, reader.scope, env, nested); err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalculationExitError(shape.Kind, shape.Name, err)
		}
		return nil, nil, frame{}, err
	}
	return ec, nested, env, nil
}

// enclosedByBehaviorBody reports whether sym is declared in the body of a behavior
// that holds running values — a calc, an action or a state machine, among its
// members or in a body-local block of it, which declares no owner of its own —
// rather than in a part or a package, whose members hold no running values.
func enclosedByBehaviorBody(sym *symbols.Symbol) bool {
	return holdsRunningValues(enclosingBehavior(sym))
}

// holdsRunningValues reports a behavior whose runs bind values: a calc, an action
// or a state machine.
func holdsRunningValues(sym *symbols.Symbol) bool {
	return isCalcSymbol(sym) || isActionSymbol(sym) || isStateSymbol(sym)
}

// bodyEnclosing is the part of enclosing, the bindings of the behavior body the
// calc is declared in, its body reads: all of it for a body written inside that
// behavior, none for a body inherited from a calc declared elsewhere.
func (shape *calcShape) bodyEnclosing(enclosing []frame) []frame {
	if len(enclosing) == 0 || !declaredWithin(shape.BodyOwner, enclosingBehavior(shape.Sym)) {
		return nil
	}
	return enclosing
}

// runOf is the environment of the innermost run of behavior among frames: the
// frames through the one holding that run; nil when none of them does.
func runOf(ctx *Context, frames []frame, behavior *symbols.Symbol) []frame {
	if !holdsRunningValues(behavior) {
		return nil
	}
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i].runs(ctx, behavior) {
			return frames[:i+1]
		}
	}
	return nil
}

// closesOverBody reports the calc reading the bindings of the behavior body it is
// declared in: through a body written there, or a default it declares there.
func (shape *calcShape) closesOverBody() bool {
	if !enclosedByBehaviorBody(shape.Sym) {
		return false
	}
	behavior := enclosingBehavior(shape.Sym)
	if declaredWithin(shape.BodyOwner, behavior) {
		return true
	}
	for i := range shape.Params {
		if param := &shape.Params[i]; param.Default != nil && declaredWithin(param.Owner, behavior) {
			return true
		}
	}
	return false
}

// declaredWithin reports sym declared in the body of behavior, directly or in a
// behavior nested in it.
func declaredWithin(sym, behavior *symbols.Symbol) bool {
	if behavior == nil {
		return false
	}
	for owner := enclosingBehavior(sym); owner != nil; owner = enclosingBehavior(owner) {
		if owner == behavior {
			return true
		}
	}
	return false
}

// checkCalcTyping rejects a calc usage typed by something that is not a calc: it
// inherits no parameters, no outputs and no body from it, so reading an output
// of it would report a missing feature rather than the specialization error.
func (ctx *Context) checkCalcTyping(sym *symbols.Symbol) error {
	for _, super := range ctx.model.DirectSupertypes(sym) {
		if super == nil || isCalcSymbol(super) {
			continue
		}
		return fmt.Errorf(
			"%w: calc usage %s specializes %s, which is not a calc",
			ErrNotACalc, ctx.qualifiedSymbolName(sym), ctx.qualifiedSymbolName(super),
		)
	}
	return nil
}

// runCalcUsage runs a calc usage's computation once over its bound inputs,
// keeping the environment the computation ends with so the usage's outputs can
// be evaluated against it.
func (ctx *Context) runCalcUsage(
	shape *calcShape, ec, nested *EvalContext, env frame, reader *EvalContext,
) (*calcRun, error) {
	host := &calcStmtHost{ctx: ctx, shape: shape, self: reader.self}
	// A usage nested in a behavior body computes over that body's bindings, as
	// an invocation of it does.
	var enclosing []frame
	if nested != nil {
		enclosing = shape.bodyEnclosing(nested.enclosingRun(shape))
	}
	engine := newStmtEngineIn(ctx, host, env, enclosing)
	host.attachPerformances(engine)
	result, returned, err := runCalcSteps(engine, host, shape)
	if err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalculationExitError(shape.Kind, shape.Name, err)
		}
		return nil, calcFrame(shape.Kind, shape.Name, err)
	}
	if ec.trace != nil {
		if returned {
			ec.trace.RecordCalculationExit(shape.Kind, shape.Name, result)
		} else {
			ec.trace.RecordCalcUsageExit(shape.Kind, shape.Name)
		}
	}

	run := newCalcRun(shape, reader.scope, reader.self, env)
	run.outer, run.result, run.returned = nested, result, returned
	run.activation, run.perf = engine.activation, host.performance()
	// The returned value is the result parameter's, read under its name or as
	// `result`; every other output states its own value, never the returned one.
	if returned {
		if out := shape.resultOutput(); out != nil && out.Name != "" {
			run.outputs[out.Name] = result
		} else if _, anonymous := shape.anonymousResult(); anonymous {
			run.outputs[resultOutputName] = result
		}
	}
	return run, nil
}

// output returns the value of one output feature of this evaluation, evaluating
// its binding the first time it is read.
func (run *calcRun) output(ctx *Context, name string) (Value, error) {
	if value, ok := run.outputs[name]; ok {
		return value, nil
	}
	out, ok := run.shape.output(name)
	if !ok {
		return Value{}, fmt.Errorf(
			"%w: %s declares no output %s (it declares: %s)",
			ErrUnknownOutput, run.shape.Label, name, run.shape.outputNames(),
		)
	}
	value, err := run.value(ctx, out)
	if err != nil {
		return Value{}, err
	}
	if ctx.trace != nil {
		ctx.trace.RecordCalcOutput(run.shape.Name, name, value)
	}
	return value, nil
}

// value evaluates one output feature's binding in the scope of the calc that
// declares it, with the calc's parameters and locals in scope, and memoizes it
// for this evaluation. An output that names another output has it evaluated on
// demand; one that names itself, directly or through others, is a cycle.
func (run *calcRun) value(ctx *Context, out calcOutput) (Value, error) {
	if value, ok := run.outputs[out.Name]; ok && out.Name != "" {
		return value, nil
	}
	if out.Value == nil || out.IsInitial {
		// An output the body assigned, and an `inout` the invocation bound, are values
		// the activation left behind rather than bindings to evaluate.
		if value, ok := run.env.lookup(out.Name); ok && out.Name != "" {
			run.outputs[out.Name] = value
			return value, nil
		}
	}
	if out.Value == nil {
		return Value{}, fmt.Errorf(
			"%w: output %s of %s", ErrOutputNotAssigned, run.outputDescription(out), run.shape.Label,
		)
	}
	if run.computing[out.Name] {
		return Value{}, fmt.Errorf(
			"%w: output %s of %s depends on itself",
			ErrCyclicOutput, run.outputDescription(out), run.shape.Label,
		)
	}
	run.computing[out.Name] = true
	defer delete(run.computing, out.Name)

	// An output is evaluated after the run that produced it has left the stack,
	// so the chain of usages a binding reads through is counted here.
	leave, err := run.enter(ctx)
	if err != nil {
		return Value{}, err
	}
	defer leave()

	value, err := run.bindingEnv(ctx, out.Owner).Eval(out.Value)
	if err != nil {
		return Value{}, calcFrame(run.shape.Kind, run.shape.Name, fmt.Errorf("output %s: %w", run.outputDescription(out), err))
	}
	// A binding gives the output its value as a write does, so it answers to the
	// output's declared type and multiplicity the same way.
	if err := out.Decl.check(ctx, &value, func() string {
		return fmt.Sprintf("%s: output %s", run.shape.Label, run.outputDescription(out))
	}); err != nil {
		return Value{}, err
	}
	if out.Name != "" {
		run.outputs[out.Name] = value
	}
	return value, nil
}

// enter counts this evaluation onto the calc stack while one of its outputs is
// worked out, unless it is counted already — by the invocation running it, or by
// an output of the same run whose binding named this one.
func (run *calcRun) enter(ctx *Context) (func(), error) {
	if run.onStack {
		return func() { /* counted already; nothing to take off the stack */ }, nil
	}
	if err := ctx.enterCalc(run.shape.Name); err != nil {
		return nil, err
	}
	run.onStack = true
	return func() { run.onStack = false; ctx.leaveCalc() }, nil
}

// bindingEnv is the environment one of this run's bindings is evaluated in: the
// calc's own, over the environment reading the usage for a binding the usage
// itself declares, which is written in the enclosing calc's body.
func (run *calcRun) bindingEnv(ctx *Context, owner *symbols.Symbol) *EvalContext {
	scope := ctx.calcScope(owner, run.shape.Sym, run.scope)
	ec := NewEvalContextIn(ctx, scope, run.self)
	// A binding of the calc's own belongs to this run; one the usage declares is
	// written in the enclosing body and belongs to the activation reading it.
	ec.activation = run.activation
	if run.outer != nil && owner == run.shape.Sym {
		ec = run.outer.nestedEnv(scope)
	}
	ec.pushFrame(run.env)
	if run.perf != nil {
		ec.pushFrame(performanceFrame(run.perf))
	}
	ec.calcRun = run
	return ec
}

// outputDescription names an output for a diagnostic, saying what an anonymous
// result parameter is rather than naming it with an empty string.
func (run *calcRun) outputDescription(out calcOutput) string {
	if out.Name != "" {
		return out.Name
	}
	return "(result)"
}

// lookupOutput answers a name an output binding refers to when it names another
// output of the same calc, evaluating that output on demand. It reports false
// for every other name, which the enclosing environment answers.
func (run *calcRun) lookupOutput(ctx *Context, name string) (Value, bool, error) {
	if run == nil {
		return Value{}, false, nil
	}
	out, ok := run.shape.output(name)
	if !ok {
		return Value{}, false, nil
	}
	value, err := run.value(ctx, out)
	return value, true, err
}

// isCalcUsageSymbol reports whether sym declares a calc usage or a case usage —
// analysis or verification — the forms that carry an evaluation whose outputs are
// features. A definition is a type: it is invoked, not read.
func isCalcUsageSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Decl != nil {
		usage, ok := sym.Decl.(*ast.Usage)
		return ok && (usage.Kind == ast.UsageCalc || usage.Kind == ast.UsageAnalysisCase ||
			usage.Kind == ast.UsageVerificationCase)
	}
	return sym.Kind == symbols.SymbolCalcUsage || sym.Kind == symbols.SymbolAnalysisCaseUsage ||
		sym.Kind == symbols.SymbolVerificationCaseUsage
}

// evalCalcUsageMembers reads a name written after a calc usage: its first part
// is an output feature of the usage, evaluated from one run of the usage's body,
// or a usage the body declares, whose own members the parts after it read; any
// further part is read through the object that output names.
func (ec *EvalContext) evalCalcUsageMembers(sym *symbols.Symbol, parts []ast.NameSegment) (Value, error) {
	if len(parts) == 0 {
		return Value{}, fmt.Errorf("empty member chain")
	}
	run, err := ec.ctx.calcUsageRun(ec, sym)
	if err != nil {
		return Value{}, err
	}
	for {
		nested, ok, err := run.nestedUsage(ec.ctx, parts[0].Text)
		if err != nil {
			return Value{}, err
		}
		if !ok {
			break
		}
		run, parts = nested, parts[1:]
		if len(parts) == 0 && ec.ctx.returnsResult(run.shape.Sym) {
			parts = resultSegments
		}
		if len(parts) == 0 {
			return Value{}, fmt.Errorf(
				"%w: %s computes output features (%s); read one of them",
				ErrNoValue, run.shape.Label, run.shape.outputNames(),
			)
		}
	}
	value, err := run.output(ec.ctx, parts[0].Text)
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(value, parts[1:], parts[0].Text)
}

// nestedUsage is the evaluation of a calc or analysis usage this run's body
// declares under name, read over this run's environment so its bindings see the
// values the body holds; false when the body declares no such usage.
func (run *calcRun) nestedUsage(ctx *Context, name string) (*calcRun, bool, error) {
	if _, isOutput := run.shape.output(name); isOutput || ctx.resolver == nil {
		return nil, false, nil
	}
	if !run.shape.declaresUsage(name) {
		return nil, false, nil
	}
	scope := ctx.calcScope(run.shape.BodyOwner, run.shape.Sym, run.scope)
	sym, ok := ctx.resolver.LookupName(scope, name)
	if !ok || !isCalcUsageSymbol(sym) {
		return nil, false, nil
	}
	nested, err := ctx.calcUsageRun(run.bindingEnv(ctx, run.shape.BodyOwner), sym)
	if err != nil {
		return nil, true, err
	}
	return nested, true, nil
}

// calcUsageOperand reports whether the operand of a feature chain names a calc
// usage, whose members are computed rather than declared values. A local binding
// or valued feature of the same name is the value the expression names, so it
// masks the declaration.
func (ec *EvalContext) calcUsageOperand(operand ast.Node) (*symbols.Symbol, bool) {
	ref, ok := operand.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) == 0 || ec.ctx.resolver == nil {
		return nil, false
	}
	if len(ref.Name.Parts) == 1 && ec.namesValue(ref.Name.Parts[0].Text) {
		return nil, false
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, ref.Name)
	if !ok || !isCalcUsageSymbol(sym) {
		return nil, false
	}
	return sym, true
}

// namesValue reports whether name is a local binding or a valued feature of the
// element being evaluated, which the expression names rather than a declaration.
func (ec *EvalContext) namesValue(name string) bool {
	if _, bound := ec.Lookup(name); bound {
		return true
	}
	_, valued := ec.valuedFeature(name)
	return valued
}

// occurrenceOperand reports whether the operand of a feature chain names one
// occurrence — a part or item — that no local binding or valued feature, and no
// feature value of the object being evaluated, already answers with.
func (ec *EvalContext) occurrenceOperand(operand ast.Node) (*symbols.Symbol, bool) {
	ref, ok := operand.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) == 0 || ec.ctx.resolver == nil {
		return nil, false
	}
	if len(ref.Name.Parts) == 1 {
		name := ref.Name.Parts[0].Text
		if ec.namesValue(name) {
			return nil, false
		}
		if ec.self != nil {
			if _, carried := ec.self.FeatureValues[name]; carried {
				return nil, false
			}
		}
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, ref.Name)
	if !ok || !ec.ctx.namesOneObject(sym) {
		return nil, false
	}
	return sym, true
}

// calcUsageMemberValue reads parts from a calc usage a part declares, running it
// against that object so its inputs read the object's feature values. Naming the usage
// itself names no value: its outputs are what it computes.
func (ec *EvalContext) calcUsageMemberValue(sym *symbols.Symbol, self *Instance, parts []ast.NameSegment) (Value, error) {
	if len(parts) == 0 && ec.ctx.readsAsFunction(sym) {
		return NewEvalContextIn(ec.ctx, sym.OwnerScope, self).functionValueOf(sym)
	}
	if len(parts) == 0 && ec.ctx.returnsResult(sym) {
		parts = resultSegments
	}
	if len(parts) == 0 {
		return Value{}, fmt.Errorf(
			"%w: calc usage %s computes output features (%s); read one of them",
			ErrNoValue, sym.Name, ec.ctx.calcUsageOutputSummary(sym),
		)
	}
	scope := sym.OwnerScope
	if scope == nil {
		scope = ec.scope
	}
	// The chained read belongs to the evaluation making it, so it shares its
	// activation: what it reads lives no longer than that evaluation.
	chained := NewEvalContextIn(ec.ctx, scope, self)
	chained.activation = ec.activation
	return chained.evalCalcUsageMembers(sym, parts)
}

// calcUsageOutputSummary describes what a calc usage computes, for a diagnostic
// about reading the usage itself rather than one of its outputs.
func (ctx *Context) calcUsageOutputSummary(sym *symbols.Symbol) string {
	names := make([]string, 0)
	var aliases map[string]string
	for _, out := range ctx.calcOutputs(ctx.calcChain(sym), &aliases) {
		if out.Name != "" {
			names = append(names, out.Name)
		}
	}
	if len(names) == 0 {
		return "no output feature"
	}
	return strings.Join(names, ", ")
}
