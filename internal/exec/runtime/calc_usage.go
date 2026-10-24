package runtime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
		for _, member := range unwrappedDeclMembers(link.Decl) {
			usage, ok := member.(*ast.Usage)
			if !ok {
				continue
			}
			if usage.Direction != ast.DirOut && usage.Direction != ast.DirInOut && !usage.IsResult {
				continue
			}
			// An output written as a redefinition names the one it overrides.
			name, _ := ast.EffectiveName(usage)
			sym := memberSymbol(DeclScope(link), usage)
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
// parameter, output or local its chain declares, a feature one of those redefines
// (`VerificationCase::subj` as the case's subject), or an inherited result parameter
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
	for _, source := range ctx.model.semantics.MemberSources(sym) {
		if source == general {
			return true
		}
	}
	return false
}

// calcMemberNames indexes the named members declared along shape's chain, and the features
// they redefine (KerML §7.3.4.5, one feature), by the name the run binds each under.
func (ctx *Context) calcMemberNames(shape *calcShape) map[*symbols.Symbol]string {
	members := make(map[*symbols.Symbol]string)
	redefined := make(map[*symbols.Symbol]string)
	for _, link := range ctx.calcChain(shape.Sym) {
		for _, member := range unwrappedDeclMembers(link.Decl) {
			var name string
			if subject, ok := subjectDeclaration(member); ok {
				name = subject.Name
			} else if usage, ok := member.(*ast.Usage); ok {
				name, _ = ast.EffectiveName(usage)
			}
			sym := memberSymbol(DeclScope(link), member)
			if name == "" || sym == nil {
				continue
			}
			name = canonical(shape.Aliases, name)
			members[sym] = name
			for _, feature := range ctx.redefinedFeatures(sym, shape.Sym) {
				redefined[feature] = name
			}
		}
	}
	for feature, name := range redefined {
		if _, declared := members[feature]; !declared {
			members[feature] = name
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

// declaresLocal reports whether the calc's body declares a feature of that name.
func (shape *calcShape) declaresLocal(name string) bool {
	for _, stmt := range shape.Body {
		if decl, ok := stmt.(lower.Declare); ok && decl.Name == name {
			return true
		}
	}
	return false
}

// memberNames lists the features a usage of the calc has: its parameters, the
// features its body declares and its outputs, in that order.
func (shape *calcShape) memberNames() string {
	var names []string
	for _, param := range shape.Params {
		names = append(names, param.Name)
	}
	for _, stmt := range shape.Body {
		if decl, ok := stmt.(lower.Declare); ok && decl.Name != "" {
			names = append(names, decl.Name)
		}
	}
	for _, out := range shape.Outputs {
		switch {
		case out.Name != "":
			names = append(names, out.Name)
		case out.IsResult:
			names = append(names, shape.resultName())
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
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
		return calcOutput{}, fmt.Errorf("%w: %s ended without a return", errCalcNoReturn, shape.Label)
	case 1:
		return valued[0], nil
	default:
		names := make([]string, 0, len(valued))
		for _, out := range valued {
			names = append(names, out.Name)
		}
		return calcOutput{}, fmt.Errorf(
			"%w: %s computes %d output features (%s) and designates no result; read them from a usage instead: %s",
			errAmbiguousResult, shape.Label, len(valued), strings.Join(names, ", "), shape.usageSpelling(valued[0].Name),
		)
	}
}

// usageSpelling writes the calc usage a modeler declares to read the outputs of
// a calc that designates no result, with the inputs it has to bind.
func (shape *calcShape) usageSpelling(output string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s c : %s {", shape.Kind, shape.Name)
	for _, param := range shape.Params {
		if param.IsActor {
			continue
		}
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
	// bodyFrames are the root statement locals left by the run, which output
	// bindings read after its statements have completed.
	bodyFrames []frame
	// perf is the case's performance, whose steps an output binding reads by name
	// (`step.pin`); nil for a calc, which performs none.
	perf *actionFrame
	// boundInputs are the values the run's input parameters were bound to.
	boundInputs []InputBinding
	// unsupplied names the parameters nothing bound: a read of one is undetermined,
	// whatever the body ran with (SysML v2 §7.17.2: the caller binds a parameter).
	unsupplied map[string]bool
	// open names the required input nothing supplied, which the usage computes
	// nothing without: its body did not run, and its outputs read undetermined.
	open string
	// occurrence is the performance instance a definition invocation's `this`
	// denotes, shared with the invocation so its output bindings see the one
	// made there; nil for a usage, which materializes none.
	occurrence *calcOccurrence
}

// lookup searches the calc's body frames before its enclosing evaluation frame.
func (run *calcRun) lookup(name string) (Value, bool) {
	for i := len(run.bodyFrames) - 1; i >= 0; i-- {
		if value, ok := run.bodyFrames[i].lookup(name); ok {
			return value, true
		}
	}
	return run.env.lookup(name)
}

// boundInputs are the values each non-subject input parameter of shape was
// bound to in env, in declaration order.
func boundInputs(shape *calcShape, env frame) []InputBinding {
	var inputs []InputBinding
	for i := range shape.Params {
		param := &shape.Params[i]
		if param.IsSubject || param.IsActor {
			continue
		}
		if value, ok := env.lookup(param.Name); ok {
			inputs = append(inputs, InputBinding{Name: param.Name, Value: value})
		}
	}
	return inputs
}

// inputs are the values the run's input parameters were bound to.
func (run *calcRun) inputs() []InputBinding {
	return run.boundInputs
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
	out.bodyFrames = snapshotFrames(run.bodyFrames)
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

// calcUsageOutput evaluates a calc usage and returns the value of one of its
// output features. The usage's inputs bind from its own member values, falling
// back to the defaults declared along its specialization chain; self, when
// non-null, is the object the usage is a feature of, whose feature values the inputs may
// name. Reading several outputs of the same usage runs its body once.
func (ctx *Context) calcUsageOutput(sym *symbols.Symbol, name string, scope *symbols.Scope, self *Instance) (Value, error) {
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
	start, run, err := ctx.beginCalcUsage(reader, sym)
	if err != nil || run != nil {
		return run, err
	}
	return ctx.finishCalcUsageRun(start)
}

// finishCalcUsageRun runs the begun usage's body to its end, refusing a wait the
// body would pause on while its outputs are read.
func (ctx *Context) finishCalcUsageRun(start *calcUsageStart) (*calcRun, error) {
	run, err := ctx.finishCalcUsage(start)
	if paused(err) && ctx.body.paused.onWait {
		return nil, ctx.caseReadWaits(start)
	}
	return run, err
}

// caseReadWaits refuses the wait a case's body paused the body on the stack for
// while an expression read the case's outputs: the read takes the case whole.
func (ctx *Context) caseReadWaits(start *calcUsageStart) error {
	wait := ctx.body.paused.wait
	ctx.body.paused = bodyPause{}
	what := "the clock"
	if wait.onMessage {
		what = "a message"
	}
	err := fmt.Errorf("%w: %s waits for %s (%s) while its outputs are read, which only a case performed as a step may",
		errCaseReadWaits, start.shape.Label, what, wait.exec.describeWaits(wait.perf))
	if start.ec.trace != nil {
		start.ec.trace.recordCalculationExitError(start.shape.Kind, start.shape.Name, err)
	}
	return calcFrame(start.shape.Kind, start.shape.Name, err)
}

// calcUsageStart is an evaluation of a calc usage under way: its inputs bound and
// its body about to run, or paused part-way for a wait, as a case's steps may.
type calcUsageStart struct {
	shape              *calcShape
	key                calcUsageKey
	ec, nested, reader *EvalContext
	env                frame
	host               *calcStmtHost
	engine             *stmtEngine
	// occurrence is the box the run and host share with `this`: the occurrence
	// the usage itself is, materialized on the first read of it.
	occurrence *calcOccurrence
	// deferResults leaves the results ending the steps unrun, for a Monte Carlo to
	// evaluate over its sample once the run's observation is in.
	deferResults bool
	// unsupplied names the parameters no argument and no default bound.
	unsupplied map[string]bool
	// inputs are the values each input parameter was bound to, captured before the
	// body runs so a body's assignments cannot rewrite what it was given.
	inputs []InputBinding
}

// beginCalcUsage binds the usage's inputs and makes ready to run its body; the
// run is answered instead, with no start, where this activation evaluated the
// usage already.
func (ctx *Context) beginCalcUsage(reader *EvalContext, sym *symbols.Symbol) (*calcUsageStart, *calcRun, error) {
	if !isCalcUsageSymbol(sym) {
		return nil, nil, fmt.Errorf(
			"%w: %s does not evaluate output features", errNotACalcUsage, ctx.QualifiedName(sym),
		)
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, nil, err
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, nil, err
	}

	// The evaluation is looked up before the inputs are bound, so a usage already
	// evaluated in this activation is not re-bound: its outputs all answer from
	// the binding its first read established.
	key := calcUsageKey{sym: sym}
	if reader.self != nil {
		key.instance = reader.self.ID
	}
	// A binding of the usage that reads the usage itself reads this same evaluation.
	if run := reader.calcRun; run != nil && run.shape.reads(sym) {
		return nil, run, nil
	}
	if run, ok := ctx.run.calcUsageRuns[reader.activation][key]; ok {
		if ctx.trace != nil {
			ctx.trace.recordCalcUsageReuse(shape.Kind, shape.Name)
		}
		return nil, run, nil
	}

	start, err := ctx.startCalcUsage(shape, key, reader, calcArgs{})
	if err != nil {
		return nil, nil, err
	}
	return start, nil, nil
}

// startCalcUsage binds the usage's inputs from args and makes its body ready to run.
func (ctx *Context) startCalcUsage(shape *calcShape, key calcUsageKey, reader *EvalContext, args calcArgs) (*calcUsageStart, error) {
	leave, err := ctx.enterCalcUsageRun(shape, key)
	if err != nil {
		return nil, err
	}
	defer leave()
	start := &calcUsageStart{shape: shape, key: key, reader: reader}
	// `this` in the usage's bindings denotes the occurrence the usage itself is,
	// made on the first read of it as every other usage's occurrence is. A
	// definition's `this` denotes the occurrence the run itself is — one per run,
	// seeded with the parameters bound so far, ending with it — not the object
	// occurrenceOf caches under the symbol, which every run would share.
	start.occurrence = &calcOccurrence{}
	if isBehaviorDefKind(shape.Sym.Kind) {
		start.occurrence.materialize = func() (*Instance, error) {
			if start.occurrence.inst != nil {
				return start.occurrence.inst, nil
			}
			inst, err := ctx.materialize(shape.Sym, 0, nil, "")
			if err != nil {
				return nil, err
			}
			for _, name := range shape.ParamNames {
				if value, held := start.occurrence.params.lookup(name); held {
					if err := inst.BindFeatureValue(ctx, name, value); err != nil {
						return nil, fmt.Errorf("%w: seed %s of object #%d: %w",
							errActionPerformanceOccurrence, name, inst.ID, err)
					}
				}
			}
			activation := reader.activation
			if start.engine != nil {
				activation = start.engine.activation
			}
			ctx.beginPerformanceLife(inst, activation)
			start.occurrence.inst = inst
			return inst, nil
		}
	} else {
		start.occurrence.materialize = func() (*Instance, error) {
			if start.occurrence.inst != nil {
				return start.occurrence.inst, nil
			}
			var inst *Instance
			var err error
			if reader.self != nil {
				inst, err = ctx.usageOccurrenceOf(start.key, reader.self)
			} else {
				inst, err = ctx.occurrenceOf(start.key.sym)
			}
			if err != nil {
				return nil, err
			}
			start.occurrence.inst = inst
			return inst, nil
		}
	}
	start.ec, start.nested, start.env, start.unsupplied, err = ctx.bindCalcUsage(shape, reader, args, start.occurrence)
	if err != nil {
		// A definition run that fails to bind never runs, so its occurrence —
		// materialized already if an earlier parameter read `this` — ends with
		// it; a usage's symbol-cached object is no run's to end.
		if isBehaviorDefKind(shape.Sym.Kind) && start.occurrence.inst != nil {
			start.occurrence.ended = true
			ctx.endPerformanceLife(start.occurrence.inst)
		}
		return nil, err
	}
	start.inputs = boundInputs(shape, start.env)
	start.host = &calcStmtHost{ctx: ctx, shape: shape, self: reader.self, occ: start.occurrence}
	// A usage nested in a behavior body computes over that body's bindings, as
	// an invocation of it does.
	var enclosing []frame
	if start.nested != nil {
		enclosing = shape.bodyEnclosing(start.nested.enclosingRun(shape))
	}
	start.engine = newStmtEngineIn(ctx, start.host, start.env, enclosing)
	start.host.attachPerformances(start.engine)
	return start, nil
}

// enterCalcUsageRun spends the calc depth on the usage and marks it running for the
// call under way (binding its inputs, or running its body until it ends or pauses).
func (ctx *Context) enterCalcUsageRun(shape *calcShape, key calcUsageKey) (leave func(), err error) {
	if err := ctx.enterCalc(shape.Name); err != nil {
		return nil, err
	}
	leaveUsage, err := ctx.enterCalcUsage(shape, key)
	if err != nil {
		ctx.leaveCalc()
		return nil, err
	}
	return func() {
		leaveUsage()
		ctx.leaveCalc()
	}, nil
}

// finishCalcUsage runs the started usage's body to its end and records the run for
// this activation; a body that pauses for a wait answers the pause, to be
// finished again once the wait is over.
func (ctx *Context) finishCalcUsage(start *calcUsageStart) (*calcRun, error) {
	run, err := ctx.runCalcUsage(start)
	if err != nil {
		return nil, err
	}
	runs, ok := ctx.run.calcUsageRuns[start.reader.activation]
	if !ok {
		runs = make(map[calcUsageKey]*calcRun)
		ctx.run.calcUsageRuns[start.reader.activation] = runs
	}
	runs[start.key] = run
	return run, nil
}

// enterCalcUsage marks the usage's body as running until leave is called; a usage
// read while its own body runs would run itself without end, so that is an error.
func (ctx *Context) enterCalcUsage(shape *calcShape, key calcUsageKey) (leave func(), err error) {
	if running := ctx.calcUsageRunning[key]; running != nil && running.reads(shape.Sym) {
		return nil, fmt.Errorf("%w: %s reads itself while its body runs", errCalcUsageRecursion, shape.Label)
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
	if ctx.model.resolver == nil {
		return nil, fmt.Errorf("%w: calc usage %s needs a resolved model", errNotACalcUsage, stmt.Name)
	}
	sym, ok := ctx.lookupName(stmt.Scope, stmt.Name)
	if !ok || !isCalcUsageSymbol(sym) {
		return nil, fmt.Errorf(
			"%w: calc usage %s is declared in this body but is not resolved to one",
			errNotACalcUsage, stmt.Name,
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
func (ctx *Context) bindCalcUsage(shape *calcShape, reader *EvalContext, args calcArgs, occurrence *calcOccurrence) (*EvalContext, *EvalContext, frame, map[string]bool, error) {
	ec := NewEvalContextIn(ctx, ctx.calcScope(shape.BodyOwner, shape.Sym, reader.scope), reader.self)
	if ec.trace != nil {
		ec.trace.recordCalculationEnter(shape.Kind, shape.Name)
	}
	// A parameter default written `this` reads the run's own occurrence, made on
	// that first read, as an invoked def's does; the run's bindings then mirror
	// into it through bindCalcParameters.
	if occurrence != nil {
		ec.occurrence, ec.thisOccurrence = occurrence.inst, occurrence.thisOccurrence()
	}

	env := frame{vars: make(map[string]Value, len(shape.Params)), aliases: shape.Aliases, owner: shape, run: ctx.newRun()}
	env.write = calcFeatureWriter(ctx, shape, occurrence)
	ec.pushFrame(env)
	if occurrence != nil {
		// The frame's vars are shared, so a `this` materializing mid-binding
		// seeds the inputs already bound — env is not the run's until returned.
		occurrence.params = env
	}

	// A usage declared in a behavior's body is written in that body, so its own
	// bindings see the values the evaluation reading it holds, and none of the
	// inputs being bound here, so every input resolves names in the enclosing
	// environment alike.
	var nested *EvalContext
	if enclosedByBehaviorBody(shape.Sym) {
		nested = reader.nestedEnv(ctx.calcScope(shape.Sym, shape.Sym, reader.scope))
	}

	// Read as a feature, a usage passes no arguments: every input binds from the value
	// the usage or its definition declares, and one declaring none is left unsupplied.
	unsupplied := shape.unsuppliedParameters(args)
	var open map[string]bool
	if shape.readsOpen(nested, args) {
		open = unsupplied
	}
	if err := ctx.bindCalcParameters(shape, ec, args, reader.scope, env, nested, occurrence, open); err != nil {
		if ec.trace != nil {
			ec.trace.recordCalculationExitError(shape.Kind, shape.Name, err)
		}
		return nil, nil, frame{}, nil, err
	}
	return ec, nested, env, unsupplied, nil
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
	for _, super := range ctx.model.semantics.DirectSupertypes(sym) {
		if super == nil || isCalcSymbol(super) {
			continue
		}
		return fmt.Errorf(
			"%w: calc usage %s specializes %s, which is not a calc",
			ErrNotACalc, ctx.QualifiedName(sym), ctx.QualifiedName(super),
		)
	}
	return nil
}

// runCalcUsage runs a calc usage's computation once over its bound inputs,
// keeping the environment the computation ends with so the usage's outputs can
// be evaluated against it.
func (ctx *Context) runCalcUsage(start *calcUsageStart) (*calcRun, error) {
	shape, ec, nested, env, reader, host, engine := start.shape, start.ec, start.nested, start.env, start.reader, start.host, start.engine
	leave, err := ctx.enterCalcUsageRun(shape, start.key)
	if err != nil {
		return nil, err
	}
	defer leave()
	// With no body to pause, a case's flow drives the clock itself, so it is on the
	// clock for the run as a top-level executor is; a body performing the case lists it.
	if flow := host.flow; flow != nil && ctx.body == nil {
		ctx.clock.attach(flow)
		defer ctx.clock.detach(flow)
	}
	run := newCalcRun(shape, reader.scope, reader.self, env)
	run.outer = nested
	run.activation, run.perf, run.boundInputs, run.occurrence = engine.activation, host.performance(), start.inputs, start.occurrence
	run.unsupplied = start.unsupplied
	if name := start.openInput(); name != "" {
		run.open = name
		if ec.trace != nil {
			ec.trace.recordCalcUsageExit(shape.Kind, shape.Name)
		}
		return run, nil
	}
	// A definition's occurrence lives for the whole run: a run abandoned to an
	// error has nothing left to read, so its occurrence ends here; one returned
	// is its caller's to end after the outputs and verdicts are read.
	kept := false
	defer func() {
		if !kept {
			run.endOccurrence(ctx)
		}
	}()

	// A tool-computed calc runs no body: the tool's answers are its outputs, the
	// result parameter's answer its result, and reading an unanswered one is the
	// reply's failure rather than a binding to evaluate.
	if shape.Tool != nil {
		result, returned, outputs, err := ctx.computeCalcByTool(shape, ec.scope, env.lookup)
		if err != nil {
			if ec.trace != nil {
				ec.trace.recordCalculationExitError(shape.Kind, shape.Name, err)
			}
			return nil, calcFrame(shape.Kind, shape.Name, err)
		}
		run.result, run.returned = result, returned
		for name, value := range outputs {
			run.outputs[name] = value
		}
		if ec.trace != nil {
			if returned {
				ec.trace.recordCalculationExit(shape.Kind, shape.Name, result)
			} else {
				ec.trace.recordCalcUsageExit(shape.Kind, shape.Name)
			}
		}
		kept = true
		return run, nil
	}

	steps := shape.Steps
	if start.deferResults {
		steps, _ = shape.observationSteps()
	}
	result, returned, err := runCalcSteps(engine, host, steps)
	run.env.cells = engine.env.data.cells
	run.bodyFrames = []frame{engine.env.localFrame().snapshot()}
	if err != nil {
		if paused(err) {
			return nil, err
		}
		if ec.trace != nil {
			ec.trace.recordCalculationExitError(shape.Kind, shape.Name, err)
		}
		return nil, calcFrame(shape.Kind, shape.Name, err)
	}
	if ec.trace != nil {
		if returned {
			ec.trace.recordCalculationExit(shape.Kind, shape.Name, result)
		} else {
			ec.trace.recordCalcUsageExit(shape.Kind, shape.Name)
		}
	}

	run.result, run.returned = result, returned
	// The returned value is the result parameter's, read under its name or as
	// `result`; every other output states its own value, never the returned one.
	if returned {
		if out := shape.resultOutput(); out != nil && out.Name != "" {
			run.outputs[out.Name] = result
		} else if _, anonymous := shape.anonymousResult(); anonymous {
			run.outputs[resultOutputName] = result
		}
	}
	kept = true
	return run, nil
}

// endOccurrence ends the performance occurrence a definition's run materialized
// for `this`, once the run's outputs and verdicts have all been read; a usage
// run's occurrence is the object the symbol caches, which outlives the run.
func (run *calcRun) endOccurrence(ctx *Context) {
	if run == nil || run.occurrence == nil || !isBehaviorDefKind(run.shape.Sym.Kind) || run.occurrence.ended {
		return
	}
	run.occurrence.ended = true
	if run.occurrence.inst != nil {
		ctx.endPerformanceLife(run.occurrence.inst)
	}
}

// output returns the value of one output feature of this evaluation, evaluating
// its binding the first time it is read.
func (run *calcRun) output(ctx *Context, name string) (Value, error) {
	if value, ok := run.outputs[name]; ok {
		if out, found := run.shape.output(name); found && (out.Value == nil || out.IsInitial) {
			if _, held := run.env.lookup(out.Name); held {
				return run.readBodyOutput(ctx, out.Name)
			}
		}
		return value, nil
	}
	out, ok := run.shape.output(name)
	if !ok {
		if value, answered, err := run.inheritedResult(ctx, name); answered {
			return value, err
		}
		return Value{}, fmt.Errorf(
			"%w: %s declares no member %s (it declares: %s)",
			errUnknownOutput, run.shape.Label, name, run.shape.memberNames(),
		)
	}
	if run.open != "" {
		var count semantics.Range
		var feature *symbols.Symbol
		if target := out.Decl.Target; target != nil {
			count, feature = target.mult, target.feature
		}
		return run.openValue(name, count, feature), nil
	}
	value, err := run.value(ctx, out)
	if err != nil {
		return Value{}, err
	}
	if ctx.trace != nil {
		ctx.trace.recordCalcOutput(run.shape.Name, name, value)
	}
	return value, nil
}

// inheritedResult answers a read of a result parameter the calc inherits rather
// than declares (`Case::result`, `VerificationCase::verdict`): the result parameter
// the calc declares redefines it and answers for it; a calc declaring none binds it no value.
func (run *calcRun) inheritedResult(ctx *Context, name string) (Value, bool, error) {
	sym, found := ctx.model.semantics.LookupMember(run.shape.Sym, name)
	if !found || !resultParameter(sym) {
		return Value{}, false, nil
	}
	if out := run.shape.resultOutput(); out != nil && out.Name != "" && out.Name != name {
		value, err := run.output(ctx, out.Name)
		return value, true, err
	}
	feat := ctx.effectiveFeature(name, sym, run.shape.Sym)
	return undeterminedFeatureValue(fmt.Sprintf("%s binds no value to %s", run.shape.Label, name), feat.Multiplicity, sym), true, nil
}

// readBodyOutput returns a calc output after deriving its body-local dependencies.
func (run *calcRun) readBodyOutput(ctx *Context, name string) (Value, error) {
	if run.env.cells == nil && len(ctx.deriving) != 0 {
		run.env.cells = newBodyCells(run.env.vars, nil)
	}
	value, ok, err := run.env.read(ctx, name)
	if err != nil {
		return Value{}, err
	}
	if !ok {
		return Value{}, &NoValueError{Feature: name}
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
	if run.shape.Tool != nil {
		// The tool's reply is the output's whole computation: one it left
		// unanswered has no binding to fall back on.
		return Value{}, &ToolError{Tool: run.shape.Tool.tool, Kind: ToolMissingOutput,
			Detail: fmt.Sprintf("%s (%s of %s) was not answered", out.Name, run.outputDescription(out), run.shape.Label)}
	}
	if out.Value == nil || out.IsInitial {
		// An output the body assigned, and an `inout` the invocation bound, are values
		// the activation left behind rather than bindings to evaluate.
		if _, ok := run.env.lookup(out.Name); ok && out.Name != "" {
			value, err := run.readBodyOutput(ctx, out.Name)
			if err != nil {
				return Value{}, err
			}
			run.outputs[out.Name] = value
			return value, nil
		}
	}
	if out.Value == nil {
		return Value{}, &unassignedOutputError{Output: run.outputDescription(out), Calc: run.shape.Label}
	}
	if run.computing[out.Name] {
		return Value{}, fmt.Errorf(
			"%w: output %s of %s depends on itself",
			errCyclicOutput, run.outputDescription(out), run.shape.Label,
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
	if run.occurrence != nil {
		ec.occurrence, ec.thisOccurrence = run.occurrence.inst, run.occurrence.thisOccurrence()
	}
	ec.pushFrame(run.env)
	if run.perf != nil {
		ec.pushFrame(performanceFrame(run.perf))
	}
	for _, frame := range run.bodyFrames {
		ec.pushFrame(frame)
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
	start, run, err := ec.ctx.beginCalcUsage(ec, sym)
	if err == nil && run == nil {
		run, err = ec.ctx.finishCalcUsageRun(start)
	}
	if err != nil {
		if value, ok := ec.ctx.unboundSubjectRead(sym, parts[0].Text, err); ok {
			return ec.chainMemberValue(value, parts[1:], parts[0].Text)
		}
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
	name := parts[0].Text
	if value, ok, err := run.memberValue(ec.ctx, name); ok {
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(value, parts[1:], name)
	}
	value, err := run.output(ec.ctx, name)
	if errors.Is(err, errUnknownOutput) {
		if value, ok, readErr := ec.occurrenceMember(run, parts); ok {
			return value, readErr
		}
	}
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(value, parts[1:], name)
}

// memberValue reads a member of the usage that is no output — a parameter (subject,
// actor or input) or a body local as the run bound it; undetermined where it bound none.
func (run *calcRun) memberValue(ctx *Context, name string) (Value, bool, error) {
	if _, isOutput := run.shape.output(name); isOutput {
		return Value{}, false, nil
	}
	param := run.shape.parameter(name)
	if param == nil && !run.shape.declaresUsage(name) {
		if value, ok := run.lookupBody(name); ok {
			return value, true, nil
		}
		if run.open != "" && run.shape.declaresLocal(name) {
			var count semantics.Range
			var feature *symbols.Symbol
			if sym, found := ctx.model.semantics.LookupMember(run.shape.Sym, name); found {
				count, feature = ctx.effectiveFeature(name, sym, run.shape.Sym).Multiplicity, sym
			}
			return run.openValue(name, count, feature), true, nil
		}
		return Value{}, false, nil
	}
	if value, ok := run.lookup(name); ok && !run.unsupplied[name] {
		return value, true, nil
	}
	var member *symbols.Symbol
	if param != nil {
		member = param.Sym
	}
	var count semantics.Range
	if member != nil {
		count = ctx.effectiveFeature(name, member, run.shape.Sym).Multiplicity
	}
	return undeterminedFeatureValue(fmt.Sprintf("%s binds no value to %s", run.shape.Label, name), count, member), true, nil
}

// openInput names the required input (no value, no default, whatever its lower bound) of
// a usage read as a feature: it computes nothing without it, so no body runs on a null.
func (start *calcUsageStart) openInput() string {
	shape := start.shape
	if !shape.readsOpen(start.nested, calcArgs{}) {
		return ""
	}
	for i := range shape.Params {
		param := &shape.Params[i]
		if !start.unsupplied[param.Name] {
			continue
		}
		// A calc computes nothing without any input; a case runs without a subject or
		// actor (they read undetermined) and without an input whose range admits none.
		if shape.isCase() && (param.IsSubject || param.IsActor || param.optional(start.ec.ctx.model.semantics)) {
			continue
		}
		if !param.declaredOptional() {
			return param.Name
		}
	}
	return ""
}

// readsOpen reports a calc or case usage read as a feature, whose unsupplied inputs
// are left open: one passed no arguments (an invocation binds its parameters, KerML
// §7.4.9.4) and not written in a behavior body (nested reads it), which binds its
// inputs from that body's evaluation.
func (shape *calcShape) readsOpen(nested *EvalContext, args calcArgs) bool {
	sym := shape.Sym
	if !isCalcUsageSymbol(sym) || nested != nil || len(args.positional) > 0 || len(args.named) > 0 {
		return false
	}
	owner := sym.Owner()
	return owner == nil || !(isBehaviorDefKind(owner.Kind) || isCalcUsageSymbol(owner))
}

// openValue is the undetermined value an output or body local of an open run reads:
// nothing supplies the input, so the usage computed nothing for name.
func (run *calcRun) openValue(name string, count semantics.Range, feature *symbols.Symbol) Value {
	reason := fmt.Sprintf("%s: nothing supplies parameter %q, so %s is not computed", run.shape.Label, run.open, name)
	return undeterminedFeatureValue(reason, count, feature)
}

// unsuppliedParameters names the parameters args and the declared defaults leave
// unbound; the subject binds through its own rule and is never among them.
func (shape *calcShape) unsuppliedParameters(args calcArgs) map[string]bool {
	var out map[string]bool
	positional := 0
	for i := range shape.Params {
		param := &shape.Params[i]
		// An actor takes no positional slot, so the cursor passes it by.
		supplied := false
		if !param.IsActor {
			supplied = positional < len(args.positional)
			positional++
		}
		if param.Default != nil || param.IsSubject || supplied {
			continue
		}
		if _, named := args.named[param.Name]; named {
			continue
		}
		if out == nil {
			out = make(map[string]bool)
		}
		out[param.Name] = true
	}
	return out
}

// lookupBody answers a local the run's body statements left bound, never a parameter.
func (run *calcRun) lookupBody(name string) (Value, bool) {
	for i := len(run.bodyFrames) - 1; i >= 0; i-- {
		if value, ok := run.bodyFrames[i].lookup(name); ok {
			return value, true
		}
	}
	return Value{}, false
}

// unboundSubjectRead answers a read of the subject the case binds nothing to as
// undetermined — a feature holding no value — though the case cannot run for want of it.
func (ctx *Context) unboundSubjectRead(sym *symbols.Symbol, name string, err error) (Value, bool) {
	var unbound *subjectUnboundError
	if !errors.As(err, &unbound) || unbound.Subject != name {
		return Value{}, false
	}
	shape, shapeErr := ctx.calcShapeOf(sym)
	if shapeErr != nil || unbound.Element != shape.Name {
		return Value{}, false
	}
	param, ok := shape.subjectParameter()
	if !ok || param.Name != name {
		return Value{}, false
	}
	feat := ctx.effectiveFeature(name, param.Sym, sym)
	reason := fmt.Sprintf("%s binds no value to its subject %s", shape.Label, name)
	return undeterminedFeatureValue(reason, feat.Multiplicity, param.Sym), true
}

// usageOccurrenceOf returns the occurrence the usage is on the object it is a feature
// of, materializing it once per object: two owners read two objectives, two parts.
func (ctx *Context) usageOccurrenceOf(key calcUsageKey, owner *Instance) (*Instance, error) {
	if id, ok := ctx.usageOccurrences[key]; ok {
		if inst, live := ctx.instances[id]; live {
			return inst, nil
		}
	}
	mark := len(ctx.created)
	inst, err := ctx.materialize(key.sym, 0, owner, key.sym.Name)
	if err != nil {
		ctx.abandonInstancesSince(mark)
		return nil, err
	}
	ctx.usageOccurrences[key] = inst.ID
	if err := ctx.startClassifierBehaviors(inst, mark); err != nil {
		return nil, err
	}
	return inst, nil
}

// occurrenceMember reads a feature the run holds no binding for — the objective, a
// requirement or part the usage declares — from the usage's occurrence, as on any object.
func (ec *EvalContext) occurrenceMember(run *calcRun, parts []ast.NameSegment) (Value, bool, error) {
	name := parts[0].Text
	sym, found := ec.ctx.model.semantics.LookupMember(run.shape.Sym, name)
	if !found || declaresType(sym) || run.occurrence == nil || run.occurrence.materialize == nil {
		return Value{}, false, nil
	}
	inst, err := run.occurrence.materialize()
	if err != nil {
		return Value{}, true, err
	}
	if _, carried := inst.FeatureValues[name]; !carried {
		return Value{}, false, nil
	}
	value, err := ec.chainFeatureValue(inst, name, parts[1:], name)
	return value, true, err
}

// nestedUsage is the evaluation of a calc or analysis usage this run's body
// declares under name, read over this run's environment so its bindings see the
// values the body holds; false when the body declares no such usage.
func (run *calcRun) nestedUsage(ctx *Context, name string) (*calcRun, bool, error) {
	if _, isOutput := run.shape.output(name); isOutput || ctx.model.resolver == nil {
		return nil, false, nil
	}
	if !run.shape.declaresUsage(name) {
		return nil, false, nil
	}
	scope := ctx.calcScope(run.shape.BodyOwner, run.shape.Sym, run.scope)
	sym, ok := ctx.lookupName(scope, name)
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
func (ec *EvalContext) calcUsageOperand(operand ast.Node) (*symbols.Symbol, bool, error) {
	ref, ok := operand.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) == 0 || ec.ctx.model.resolver == nil {
		return nil, false, nil
	}
	if len(ref.Name.Parts) == 1 {
		names, err := ec.namesValue(ref.Name.Parts[0].Text)
		if err != nil {
			return nil, false, err
		}
		if names {
			return nil, false, nil
		}
	}
	sym, ok := ec.ctx.resolveQualified(ec.scope, ref.Name)
	if !ok || !isCalcUsageSymbol(sym) {
		return nil, false, nil
	}
	return sym, true, nil
}

// namesValue reports whether name is a local binding or a valued feature of the
// element being evaluated, which the expression names rather than a declaration.
func (ec *EvalContext) namesValue(name string) (bool, error) {
	if _, bound, err := ec.Lookup(name); err != nil {
		return false, err
	} else if bound {
		return true, nil
	}
	_, valued := ec.valuedFeature(name)
	return valued, nil
}

// occurrenceOperand reports whether the operand of a feature chain names occurrences of
// its own — a part or item, or a namespace's collection of them — that no local binding
// or valued feature, and no feature value of the object being evaluated, already answers with.
func (ec *EvalContext) occurrenceOperand(operand ast.Node) (*symbols.Symbol, bool, error) {
	ref, ok := operand.(*ast.FeatureReference)
	if !ok || ref.Name == nil || len(ref.Name.Parts) == 0 || ec.ctx.model.resolver == nil {
		return nil, false, nil
	}
	if len(ref.Name.Parts) == 1 {
		name := ref.Name.Parts[0].Text
		names, err := ec.namesValue(name)
		if err != nil {
			return nil, false, err
		}
		if names {
			return nil, false, nil
		}
		if ec.self != nil {
			if _, carried := ec.self.FeatureValues[name]; carried {
				return nil, false, nil
			}
		}
	}
	sym, ok := ec.ctx.resolveQualified(ec.scope, ref.Name)
	if !ok || !ec.ctx.namesOneObject(sym) && !ec.ctx.namesObjects(sym) {
		return nil, false, nil
	}
	if isReferenceUsage(sym) {
		// A reference member of a behavior — `Raise::context` — holds the value
		// its run bound, not an occurrence of its own: read it as a value.
		return nil, false, nil
	}
	return sym, true, nil
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
