package runtime

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// EvalContext is the lexical environment during evaluation (Tier 3).
type EvalContext struct {
	ctx    *Context       // runtime context
	scope  *symbols.Scope // scope context for name resolution
	self   *Instance      // instance a feature name resolves against, nil when unbound
	frames []frame        // stack of local bindings (innermost = frames[len-1])
	trace  *TraceRecorder // evaluation trace recorder, nil when not tracing

	// features are the features of the element being evaluated — a requirement's
	// or constraint's own, inherited and rebound features — which its conditions
	// may name wherever those conditions were written.
	features map[string]scopedExpr

	// resolving holds the features whose own value is being evaluated, so a value
	// written in terms of a same-named outer one does not resolve to itself.
	resolving map[string]bool

	// calcRun is the calc evaluation whose output feature is being computed, so an
	// output binding written in terms of the calc's other outputs reads them from
	// the same evaluation. It is nil everywhere else.
	calcRun *calcRun

	// inBehaviorBody marks a statement of a behavior body, which reaches the
	// object performing it only through names that resolve to its features.
	inBehaviorBody bool

	// activation identifies the execution of the body this evaluation belongs to,
	// so every output read of one calc usage within it comes from one evaluation
	// of that usage. It is zero outside a body, where nothing can change between
	// two reads.
	activation int64

	// entered is the last activation allocated before the built-in being applied
	// was entered: an occurrence begun since began during the call.
	entered int64
}

// NewEvalContext creates an evaluation context with an empty frame stack. It
// inherits the runtime context's trace recorder, so every evaluation reached
// from a traced context is recorded, including nested calc invocations.
func NewEvalContext(ctx *Context, scope *symbols.Scope) *EvalContext {
	return &EvalContext{
		ctx:    ctx,
		scope:  scope,
		frames: nil,
		trace:  ctx.trace,
	}
}

// NewEvalContextIn creates an evaluation context bound to an instance, so that
// a feature name resolves to that instance's feature value rather than to the
// declared default of the same name.
func NewEvalContextIn(ctx *Context, scope *symbols.Scope, self *Instance) *EvalContext {
	ec := NewEvalContext(ctx, scope)
	ec.self = self
	return ec
}

// beginStep gives an evaluation outside a body a scope of its own, so what it reads
// - a calc usage's outputs, a collection's elements - is not held past the step. The
// returned function ends it.
func (ec *EvalContext) beginStep() func() {
	activation, end := ec.ctx.beginStep()
	ec.activation = activation
	return end
}

// evalIn returns a context that resolves names in scope while sharing this
// one's bindings and trace, for a body member written in another declaration's
// scope (an inherited calc result or parameter default).
func (ec *EvalContext) evalIn(scope *symbols.Scope) *EvalContext {
	if scope == nil || scope == ec.scope {
		return ec
	}
	return &EvalContext{
		ctx: ec.ctx, scope: scope, self: ec.self, frames: ec.frames, trace: ec.trace,
		features: ec.features, resolving: ec.resolving, calcRun: ec.calcRun,
		activation: ec.activation, inBehaviorBody: ec.inBehaviorBody,
	}
}

// valuedFeature is the value the element being evaluated binds to name, unless
// that value is the one being evaluated (`in mass = mass` reads the outer mass).
func (ec *EvalContext) valuedFeature(name string) (scopedExpr, bool) {
	bound, declared := ec.features[name]
	return bound, declared && bound.expr != nil && !ec.resolving[name]
}

// valuedFeatureValue evaluates the value the element being evaluated binds to
// name; ok is false when it binds none or that value is the one being evaluated.
func (ec *EvalContext) valuedFeatureValue(name string) (val Value, ok bool, err error) {
	bound, ok := ec.valuedFeature(name)
	if !ok {
		return Value{}, false, nil
	}
	if ec.resolving == nil {
		ec.resolving = map[string]bool{}
	}
	ec.resolving[name] = true
	val, err = ec.evalIn(bound.scope).inEnv(bound.env).Eval(bound.expr)
	delete(ec.resolving, name)
	return val, true, err
}

// inEnv returns a context reading env instead of this one's features and
// bindings. An enclosing environment holds other features than the ones being
// resolved, so a same name there is a fresh read rather than a cycle.
func (ec *EvalContext) inEnv(env *conditionEnv) *EvalContext {
	if env == nil {
		return ec
	}
	out := ec.over(ec.scope, []frame{env.bindings})
	out.features = env.features
	if env.enclosing {
		out.resolving = nil
	}
	return out
}

// nestedEnv returns a context resolving names in scope over this one's
// environment, for a declaration nested in the body being evaluated: its
// bindings stay in force under whatever frame the nested declaration pushes.
func (ec *EvalContext) nestedEnv(scope *symbols.Scope) *EvalContext {
	frames := make([]frame, len(ec.frames))
	copy(frames, ec.frames)
	return ec.over(scope, frames)
}

// closure snapshots the environment for an expression evaluated later; bindings
// are copied since an invocation's frame storage is reused once it returns, and
// the calc evaluation whose outputs it may name is detached from that storage.
func (ec *EvalContext) closure() *EvalContext {
	out := ec.over(ec.scope, snapshotFrames(ec.frames))
	out.calcRun = ec.calcRun.detached()
	return out
}

// snapshotFrames copies the bindings of frames, which their runs may reuse or drop.
func snapshotFrames(frames []frame) []frame {
	if len(frames) == 0 {
		return nil
	}
	out := make([]frame, len(frames))
	for i, f := range frames {
		out[i] = f.snapshot()
	}
	return out
}

// over is this environment resolving names in scope over frames of its own.
func (ec *EvalContext) over(scope *symbols.Scope, frames []frame) *EvalContext {
	return &EvalContext{
		ctx: ec.ctx, scope: scope, self: ec.self, frames: frames, trace: ec.trace,
		features: ec.features, resolving: ec.resolving, calcRun: ec.calcRun,
		activation: ec.activation, inBehaviorBody: ec.inBehaviorBody,
	}
}

// Push adds a new frame to the stack (on calc invocation, lambda entry).
func (ec *EvalContext) Push(bindings map[string]Value) {
	ec.frames = append(ec.frames, mapFrame(bindings))
}

// pushFrame adds a frame to the stack.
func (ec *EvalContext) pushFrame(f frame) {
	ec.frames = append(ec.frames, f)
}

// hasPerformanceFrame reports whether an action performance is on the stack.
func (ec *EvalContext) hasPerformanceFrame() bool {
	for i := range ec.frames {
		if ec.frames[i].perf != nil {
			return true
		}
	}
	return false
}

// lookupSubaction finds the node named name in the flow of an action performance
// on the stack, innermost first, and returns its latest performance. Where the
// name resolves in the reading scope, it is that declaration's node — or no node
// at all when the declaration is a feature, which shadows a same-named node.
func (ec *EvalContext) lookupSubaction(name string) (perf *actionFrame, declared bool, err error) {
	if !ec.hasPerformanceFrame() {
		return nil, false, nil
	}
	var decl ast.Node
	if ec.ctx.model.resolver != nil {
		if sym, ok := ec.ctx.lookupName(ec.scope, name); ok && sym != nil {
			if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind != ast.UsageAction && !lower.IsCaseNode(usage) {
				return nil, false, nil
			}
			decl = sym.Decl
		}
	}
	for i := len(ec.frames) - 1; i >= 0; i-- {
		f := ec.frames[i].perf
		if f == nil {
			continue
		}
		if perf, declared, err = f.subaction(name, decl); declared {
			return perf, true, err
		}
	}
	return nil, false, nil
}

// evalSubactionPath reads `node.pin` or `node.inner.pin` through the performances
// of the nodes the path names; the rest of the path past a pin (`node.pin.member`)
// is chained through the pin's value, and a path ending at a node reads its result.
func (ec *EvalContext) evalSubactionPath(perf *actionFrame, parts []ast.NameSegment) (Value, error) {
	for i, part := range parts {
		if inner, declared, err := perf.subaction(part.Text, nil); declared {
			if err != nil {
				return Value{}, err
			}
			perf = inner
			continue
		}
		if i != len(parts)-1 && !perf.declares(part.Text) {
			return Value{}, fmt.Errorf("%w: %s declares no node or pin %s to read %s through",
				ErrNodePin, perf.describe(), part.Text, parts[len(parts)-1].Text)
		}
		value, err := perf.pin(part.Text)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(value, parts[i+1:], perf.path()+"."+part.Text)
	}
	return perf.resultValue()
}

// Pop removes the top frame from the stack (on return, lambda exit).
func (ec *EvalContext) Pop() {
	if len(ec.frames) > 0 {
		ec.frames = ec.frames[:len(ec.frames)-1]
	}
}

// Lookup searches for a name in the frame stack (innermost first).
func (ec *EvalContext) Lookup(name string) (Value, bool) {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if val, ok := ec.frames[i].lookup(name); ok {
			return val, true
		}
	}
	return Value{}, false
}

// Eval evaluates an expression node. Returns a Value or an error.
// Increments the run's steps on each eval call; errors when they are >= ctx.maxSteps.
// When the context is traced, the evaluation is recorded after its
// sub-expressions, which makes sub-expression order part of the trace.
func (ec *EvalContext) Eval(node ast.Node) (Value, error) {
	if ec.trace == nil {
		return ec.eval(node)
	}
	ec.trace.BeginEval()
	value, err := ec.eval(node)
	ec.trace.EndEval(TraceLabel(node), value, err)
	return value, err
}

// eval dispatches one expression node, without trace bookkeeping.
func (ec *EvalContext) eval(node ast.Node) (Value, error) {
	// Step counter
	if err := ec.ctx.incrementStep(); err != nil {
		return Value{}, err
	}

	// Dispatch by node type (scaffolding; full implementation in later tasks)
	switch n := node.(type) {
	case *ast.LiteralInteger:
		return ec.evalLiteralInteger(n)
	case *ast.LiteralReal:
		return ec.evalLiteralReal(n)
	case *ast.LiteralBool:
		return ec.evalLiteralBool(n)
	case *ast.LiteralString:
		return ec.evalLiteralString(n)
	case *ast.LiteralInfinity:
		return ec.evalLiteralInfinity(n)
	case *ast.MetadataAccessExpr:
		return ec.evalMetadataAccess(n)
	case *ast.NullExpr:
		return ec.evalNull(n)
	case *ast.FeatureReference:
		val, err := ec.evalFeatureReference(n)
		return ec.declaredElements(n, val, err)
	case *ast.QualifiedName:
		val, err := ec.evalName(n)
		return ec.declaredElements(n, val, err)
	case *ast.FeatureChainExpr:
		val, err := ec.evalFeatureChain(n)
		return ec.declaredElements(n, val, err)
	case *ast.OperatorExpr:
		return ec.evalOperator(n)
	case *ast.SequenceExpr:
		return ec.evalSequenceExpr(n)
	case *ast.CollectExpr:
		return ec.evalCollectExpr(n)
	case *ast.SelectExpr:
		return ec.evalSelectExpr(n)
	case *ast.InvocationExpr:
		return ec.evalInvocation(n)
	case *ast.IndexExpr:
		return ec.evalIndexExpr(n)
	case *ast.ConstructorExpr:
		return ec.evalConstructor(n)
	case *ast.BodyExpr:
		// A body is a value closed over its environment, applied where it is called.
		return NewExprValue(n, ec.closure()), nil
	default:
		return Value{}, fmt.Errorf("unsupported node type: %T", node)
	}
}

// Eval is the top-level entry point for evaluating an expression in an empty environment.
// Resolves names from the root scope.
func (ctx *Context) Eval(node ast.Node) (Value, error) {
	defer ctx.beginRun()()

	// Use resolver's root scope for name resolution
	// (In a full implementation, this would track evaluation context scope)
	ec := NewEvalContext(ctx, nil)
	return ec.Eval(node)
}

// EvalWithScope evaluates an expression with a given scope context for name resolution.
func (ctx *Context) EvalWithScope(node ast.Node, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

	ec := NewEvalContext(ctx, scope)
	return ec.Eval(node)
}

// EvalDeclaredValue evaluates the value a usage declaration binds, as a read of
// the declaration does: in its own scope, and answering to its declared type. A
// usage binding none but shaped as an Array by its own features is that Array.
func (ctx *Context) EvalDeclaredValue(sym *symbols.Symbol) (Value, error) {
	value := ctx.extractDefaultValue(sym)
	if value == nil {
		defer ctx.beginRun()()
		if val, ok, err := ctx.declaredArrayValue(sym); ok {
			return val, err
		}
		// A calc definition, or a calc usage awaiting arguments, is a function.
		if val, ok, err := ctx.FunctionValue(sym); ok {
			return val, err
		}
		// A calc usage returning one unnamed result is read as that result.
		if isCalcUsageSymbol(sym) && ctx.returnsResult(sym) {
			return NewEvalContext(ctx, sym.OwnerScope).evalCalcUsageMembers(sym, resultSegments)
		}
		// Read as a name of it is read: a feature nothing values is undetermined.
		return NewEvalContext(ctx, sym.OwnerScope).withoutValue(sym, ctx.qualifiedSymbolName(sym), nil)
	}
	defer ctx.beginRun()()

	return NewEvalContext(ctx, sym.OwnerScope).declaredValue(sym, value)
}

// EvalWithScopeOn evaluates an expression against a concrete instance, so a
// feature it names reads that object's feature value. It brackets one run, as
// EvalWithScope does, which is what bounds the evaluation by the step budget.
func (ctx *Context) EvalWithScopeOn(node ast.Node, scope *symbols.Scope, self *Instance) (Value, error) {
	defer ctx.beginRun()()

	return NewEvalContextIn(ctx, scope, self).Eval(node)
}

// evalLiteralInteger evaluates an integer literal, reporting one outside the
// Integer range rather than clamping it.
func (ec *EvalContext) evalLiteralInteger(n *ast.LiteralInteger) (Value, error) {
	val, ok := ec.ctx.model.integerLiterals[n]
	if !ok {
		var err error
		if val, err = strconv.ParseInt(n.Value, 10, 64); err != nil {
			return Value{}, fmt.Errorf("%w: literal %s is outside the Integer range",
				semantics.ErrArithmeticOverflow, n.Value)
		}
		ec.ctx.model.integerLiterals[n] = val
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: val}}, nil
}

// evalLiteralReal evaluates a real literal, reporting one outside the Real
// range rather than carrying it as an infinity.
func (ec *EvalContext) evalLiteralReal(n *ast.LiteralReal) (Value, error) {
	val, ok := ec.ctx.model.realLiterals[n]
	if !ok {
		var err error
		if val, err = semantics.ParseReal(n.Value); err != nil {
			return Value{}, fmt.Errorf("%w: literal %s is outside the Real range", err, n.Value)
		}
		ec.ctx.model.realLiterals[n] = val
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: val}}, nil
}

// evalLiteralBool evaluates a boolean literal.
func (ec *EvalContext) evalLiteralBool(n *ast.LiteralBool) (Value, error) {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: n.Value}}, nil
}

// evalLiteralString evaluates a string literal, which spells its text with the
// quotes and escapes of the notation.
func (ec *EvalContext) evalLiteralString(n *ast.LiteralString) (Value, error) {
	return NewStringValue(lexer.StringValue(n.Value)), nil
}

// evalNull evaluates a null expression.
// evalLiteralInfinity evaluates `*`, the unbounded value: a scalar constant of
// its own, ordered above every finite number and refused by arithmetic.
func (ec *EvalContext) evalLiteralInfinity(_ *ast.LiteralInfinity) (Value, error) {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInfinity}}, nil
}

func (ec *EvalContext) evalNull(n *ast.NullExpr) (Value, error) {
	return Value{Kind: ValNull}, nil
}

// declaredElements types a feature read that yields no element by the quantity
// dimension the read declares (KerML 8.4.4.9: a feature's values are of its type),
// in that dimension's coherent unit; an aggregate of the read then keeps the kind.
func (ec *EvalContext) declaredElements(node ast.Node, val Value, err error) (Value, error) {
	if err != nil || !isEmptyValue(val) {
		return val, err
	}
	if typed, ok := ec.ctx.emptyOfDeclared(ec.scope, node); ok {
		return typed, nil
	}
	return val, nil
}

// emptyOfDeclared is the empty sequence of the quantities an expression is
// statically declared to yield, in their coherent unit; false where the
// declarations fix no dimension or a dimensionless one.
func (ctx *Context) emptyOfDeclared(scope *symbols.Scope, node ast.Node) (Value, bool) {
	if ctx.model.semantics == nil || scope == nil {
		return Value{}, false
	}
	return ctx.emptyOfDimension(ctx.model.semantics.DimensionOfExpr(scope, node))
}

// emptyOfFeature is emptyOfDeclared for the values a feature declares it holds.
func (ctx *Context) emptyOfFeature(feat *EffectiveFeature) (Value, bool) {
	if feat == nil {
		return Value{}, false
	}
	return ctx.emptyOfSymbol(feat.heldBy())
}

// emptyOfSymbol is emptyOfDeclared for the values a declared feature holds.
func (ctx *Context) emptyOfSymbol(sym *symbols.Symbol) (Value, bool) {
	if ctx.model.semantics == nil || sym == nil {
		return Value{}, false
	}
	return ctx.emptyOfDimension(ctx.model.semantics.DimensionOfFeature(sym))
}

// declaredCount is how many values an expression holds wherever it is evaluated,
// as far as the declarations it reads fix that: `[0..*]` where they leave it open.
func (ec *EvalContext) declaredCount(scope *symbols.Scope, node ast.Node) semantics.Range {
	if ec.ctx.model.semantics == nil || scope == nil {
		return openRange()
	}
	if count, ok := ec.ctx.model.semantics.ValuesHeldBy(scope, node); ok {
		return count
	}
	return openRange()
}

// emptyOfDimension is the empty sequence of quantities of a dimension, in its
// coherent unit; false where none is fixed or it is dimensionless.
func (ctx *Context) emptyOfDimension(dim semantics.Dimension, ok bool) (Value, bool) {
	if !ok || dim.Term.Dimensionless() {
		return Value{}, false
	}
	unit, ok := ctx.model.semantics.CoherentUnit(dim)
	if !ok {
		return Value{}, false
	}
	return NewEmptySequenceOf(unit), true
}

// readFeatureValue is what fv reads as in an expression, an empty read typed by
// the quantities its feature declares.
func (ctx *Context) readFeatureValue(fv *FeatureValue, name string) (Value, error) {
	val, err := fv.ReadValue(name)
	if err != nil || val.Kind != ValSequence || val.Sequence().Size() != 0 {
		return val, err
	}
	if typed, ok := ctx.emptyOfFeature(fv.Feature); ok {
		return typed, nil
	}
	return val, nil
}

// evalFeatureReference evaluates a feature reference (variable lookup).
func (ec *EvalContext) evalFeatureReference(n *ast.FeatureReference) (Value, error) {
	if n == nil {
		return Value{}, fmt.Errorf("empty feature reference")
	}
	return ec.evalName(n.Name)
}

// thatName is the implicit feature every usage takes from the base usage: it
// names the instance featuring the value being evaluated ([KerML, 8.4.2]).
const thatName = "that"

// thisName is the context occurrence of what is being evaluated, which for a
// performance an object owns is that object ([KerML] Occurrences::this).
const thisName = "this"

// evalName evaluates a name as a reference to what it names, which is what an
// expression written as a bare name is: `rate`, `A::B::x`.
func (ec *EvalContext) evalName(qn *ast.QualifiedName) (Value, error) {
	// Outside an expression body no body-local declaration can shadow a bound
	// name, so a frame binding is the answer: the common case, kept small.
	if qn != nil && len(qn.Parts) == 1 && (ec.scope == nil || !ec.scope.BodyLocal()) {
		if val, ok := ec.Lookup(qn.Parts[0].Text); ok {
			return val, nil
		}
	}
	return ec.evalNameGeneral(qn)
}

// evalNameGeneral evaluates a name through every source that may answer it, in
// shadowing order.
func (ec *EvalContext) evalNameGeneral(qn *ast.QualifiedName) (Value, error) {
	if qn == nil || len(qn.Parts) == 0 {
		return Value{}, fmt.Errorf("empty feature reference")
	}

	// Simple case: single-part name lookup in frame stack or scope
	if len(qn.Parts) == 1 {
		name := qn.Parts[0].Text
		// A declaration inside an expression body is local to that body and
		// shadows features of the element carrying the expression.
		if ec.scope != nil {
			if sym, ok := symbols.LookupBodyLocal(ec.scope, name); ok {
				// Body parameters and statement locals are supplied by the frame lookup below.
				_, bodyMember := sym.OwnerScope.Node().(*ast.BodyExpr)
				_, param := sym.Decl.(*ast.BodyExpr)
				if bodyMember && !param {
					if ec.resolving[name] {
						return Value{}, fmt.Errorf("%w: %s", ErrCyclicFeatureValue, name)
					}
					if value := ec.ctx.extractDefaultValue(sym); value != nil {
						if ec.resolving == nil {
							ec.resolving = map[string]bool{}
						}
						ec.resolving[name] = true
						val, err := ec.declaredValue(sym, value)
						delete(ec.resolving, name)
						return val, err
					}
					return Value{}, &NoValueError{Feature: name, Ref: qn}
				}
			}
		}
		// Try frame stack first (local bindings from calc/lambda params)
		if val, ok := ec.Lookup(name); ok {
			return val, nil
		}
		// Then a node of an action performance in the frame stack, read as a value.
		if perf, declared, err := ec.lookupSubaction(name); declared {
			if err != nil {
				return Value{}, err
			}
			return perf.resultValue()
		}
		// Then another output feature of the calc whose output is being computed:
		// an `out` binding may be written in terms of the calc's other outputs,
		// which are evaluated from the same run of its body.
		if value, ok, err := ec.calcRun.lookupOutput(ec.ctx, name); ok {
			return value, err
		}
		// Then a valued feature of the element being evaluated: it is declared
		// inside that element, so it masks a same-named member of the object
		// carrying it, and a value a typed usage binds masks the default of the
		// declaration it redefines.
		// A feature whose own value is already being evaluated is skipped, so
		// `in mass = mass` reads the outer mass rather than itself.
		if val, ok, err := ec.valuedFeatureValue(name); ok {
			return val, err
		}
		// Then the bound instance: a feature value holds the value this object actually
		// carries, which overrides the declared default the scope would yield.
		if ec.self != nil && ec.selfFeatureInScope(name) {
			if val, ok, err := ec.selfFeatureValue(name); err != nil {
				return Value{}, err
			} else if ok {
				return val, nil
			}
		}
		// Then `that`, which every usage takes from the base usage: it names the
		// instance featuring the value being evaluated, which is the bound one.
		if name == thatName && ec.self != nil {
			return Value{Kind: ValInstance, Instance: ec.self.ID}, nil
		}
		// Then `self`, the thing being evaluated, read as its value.
		if ec.self != nil && ec.namesSelf(name) {
			return ec.ctx.objectValue(ec.self)
		}
		// Then `this`, the context occurrence of what is being evaluated: the
		// object owning the performance, which is the bound instance.
		if name == thisName && ec.namesOccurrenceThis(name) {
			return ec.thisValue()
		}
		// Then the scope the expression was written in: a sibling attribute, a
		// member of an enclosing namespace, or a name an import brought in, found
		// the way a written reference finds it. The declaration's own value is
		// evaluated in the scope it was declared in, so the imports in force there
		// — rather than the ones in force here — answer the names it uses.
		if ec.scope != nil && !ec.resolving[name] {
			if sym, ok := ec.ctx.lookupName(ec.scope, name); ok && sym != nil {
				// An inherited expression reads the feature as the running behavior
				// inherits it: through the redefinition, when it states one.
				sym = ec.ctx.inheritedFeature(ec.runningBehavior(), sym)
				ec.ctx.noteDeclarationRead(sym)
				// An enumerated value is the value of its enumeration it stands
				// for; any other variant names a choice, not the value it declares.
				if semantics.EnumerationOwning(sym) != nil {
					return ec.enumLiteralValue(sym)
				}
				if ec.ctx.model.semantics.VariationPointOwning(sym) != nil {
					return variantReference(sym), nil
				}
				// A feature of an enclosing type, named from a nested usage, is read
				// from the object enclosing the bound one: `e1` inside `e3` is the
				// containing rectangle's e1, not a fresh occurrence of the declaration.
				if val, ok, err := ec.outerFeatureValue(sym); ok {
					return val, err
				}
				// A transformation's target is the frame featuring it, which its
				// value carries and no object states.
				if val, ok, err := ec.featuringReferenceValue(sym); ok {
					return val, err
				}
				// A library feature's value comes from the feature seam, not its
				// declared body: a warm library cache restores symbols without AST.
				if val, ok, err := ec.ctx.libraryFeatureValue(sym); ok {
					return val, err
				}
				// A measurement unit declaration is the measurement reference it names.
				if val, ok, err := ec.ctx.MeasurementUnitValue(sym); ok {
					return val, err
				}
				// A part, item or structured value names an object, so the name
				// evaluates to that object rather than to a value the declaration
				// would have to hold.
				if val, ok, err := ec.occurrenceReference(sym); ok {
					return val, err
				}
				// A calc definition, or a calc usage awaiting arguments, is a function.
				if val, ok, err := ec.calcAsValue(sym); ok {
					return val, err
				}
				// A calc usage returning one unnamed result is read as that result.
				if isCalcUsageSymbol(sym) && ec.ctx.returnsResult(sym) {
					return ec.evalCalcUsageMembers(sym, resultSegments)
				}
				// A feature declared with no value, whose multiplicity admits none,
				// states the empty sequence — what an object holding nothing reads.
				if val, ok := ec.emptyDeclaredFeature(sym); ok {
					return val, nil
				}
				if value := ec.ctx.extractDefaultValue(sym); value != nil {
					if ec.resolving == nil {
						ec.resolving = map[string]bool{}
					}
					ec.resolving[name] = true
					val, err := ec.declaredValue(sym, value)
					delete(ec.resolving, name)
					return val, err
				}
				// A variation holds nothing until it is bound, whether it is read
				// through an object or through its declaration.
				if ec.ctx.model.semantics.IsVariationFeature(sym) {
					return Value{}, fmt.Errorf("%w: %s", ErrVariationUnselected, name)
				}
				return ec.resolvedWithoutValue(sym, qn)
			}
		}
		// Nothing outside the feature supplies its value, so its own value depends
		// on itself.
		if ec.resolving[name] {
			return Value{}, fmt.Errorf("%w: %s", ErrCyclicFeatureValue, name)
		}
		// A feature the element declares but nothing gives a value to is
		// uninitialized rather than unresolved.
		if _, declared := ec.features[name]; declared {
			return Value{}, &NoValueError{Feature: name, Ref: qn}
		}
		if ec.ctx.model.resolver != nil && ec.scope != nil {
			return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedName(ec.scope, name, qn))
		}
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, name)
	}

	// A multi-part name A::B::x resolves as the checker resolves it — imports,
	// visibility, aliases and inherited members included — so the two agree. It
	// is read in this evaluation's scope, since one expression may be evaluated
	// in several.
	reading := ec.ctx.readQualified(ec.scope, qn)

	// A path starting at a node of an action performance on the stack reads
	// through that node's performance: `p.v`, `leg.inner.v`.
	if perf, declared, err := ec.lookupSubaction(qn.Parts[0].Text); declared && !qn.Global {
		if err != nil {
			return Value{}, err
		}
		return ec.evalSubactionPath(perf, qn.Parts[1:])
	}

	// A calc usage's output features are computed rather than declared values,
	// so a name qualified by one reads the rest from an evaluation of the usage.
	for i := 0; i < len(qn.Parts)-1; i++ {
		part, resolved := reading.Part(i)
		if !resolved {
			break
		}
		if isCalcUsageSymbol(part) {
			return ec.evalCalcUsageMembers(part, qn.Parts[i+1:])
		}
	}
	currentSym, ok := reading.Symbol()
	if !ok {
		return Value{}, ec.unresolvedQualifiedName(qn, reading)
	}
	ec.ctx.noteDeclarationRead(currentSym)

	// A feature of a behavior whose run is on the stack (`MassCase::result` in its
	// objective or assertion) reads the value that run bound to it.
	if qualifier, ok := reading.Part(len(qn.Parts) - 2); ok {
		if val, ok := ec.frameFeatureValue(qualifier, currentSym); ok {
			return val, nil
		}
	}
	// A library feature reads through the feature seam, whatever the library
	// declares for it and whether or not the cache kept its declaration.
	if val, ok, err := ec.ctx.libraryFeatureValue(currentSym); ok {
		return val, err
	}
	// A measurement unit declaration (`SI::m`) is the measurement reference it names.
	if val, ok, err := ec.ctx.MeasurementUnitValue(currentSym); ok {
		return val, err
	}
	// A qualified feature of an enclosing type (`Rectangle::length` inside its
	// `e1`) reads the enclosing object's value of that feature.
	if val, ok, err := ec.outerFeatureValue(currentSym); ok {
		return val, err
	}

	// A calc definition, or a calc usage awaiting arguments, is a function.
	if val, ok, err := ec.calcAsValue(currentSym); ok {
		return val, err
	}

	// Evaluate the final symbol's declaration
	if decl, ok := currentSym.Decl.(*ast.Usage); ok {
		// An enumerated value is a value of its enumeration, whether or not it
		// declares one of its own; any other variant names a choice its variation
		// can be bound to, and compares equal to the variation that selected it.
		if semantics.EnumerationOwning(currentSym) != nil {
			return ec.enumLiteralValue(currentSym)
		}
		if ec.ctx.model.semantics.VariationPointOwning(currentSym) != nil {
			return variantReference(currentSym), nil
		}
		if decl.Value != nil {
			return ec.declaredValue(currentSym, decl.Value)
		}
		if ec.ctx.model.semantics.IsVariationFeature(currentSym) {
			return Value{}, fmt.Errorf("%w: %s", ErrVariationUnselected, qualifiedNameToString(qn))
		}
		// A part, item or structured value names an object, read as that object.
		if val, ok, err := ec.occurrenceReference(currentSym); ok {
			return val, err
		}
		// A calc usage returning one unnamed result is read as that result.
		if isCalcUsageSymbol(currentSym) && ec.ctx.returnsResult(currentSym) {
			return ec.evalCalcUsageMembers(currentSym, resultSegments)
		}
		// A calc usage or a KerML type is never the empty sequence, whatever it admits.
		if isCalcUsageSymbol(currentSym) || declaresType(currentSym) {
			return ec.resolvedWithoutValue(currentSym, qn)
		}
		// A valueless feature admitting nothing is the empty sequence, however spelled.
		if val, ok := ec.emptyDeclaredFeature(currentSym); ok {
			return val, nil
		}
	}
	// A require/assume constraint reads the value it binds, as a constraint usage does.
	if oc, ok := ast.OwnedConstraintOf(currentSym.Decl); ok && oc.Value != nil {
		return ec.declaredValue(currentSym, oc.Value)
	}
	// A subject is bound or, admitting nothing, empty; otherwise it awaits a binding.
	if decl, ok := currentSym.Decl.(*ast.SubjectMember); ok {
		if decl.BindingExpr != nil {
			return ec.declaredValue(currentSym, decl.BindingExpr)
		}
		if val, ok := ec.emptyDeclaredFeature(currentSym); ok {
			return val, nil
		}
	}
	return ec.resolvedWithoutValue(currentSym, qn)
}

// frameFeatureValue reads the resolved member sym, qualified by qualifier, from the innermost
// frame whose owner is (or specializes) the qualifier, under the name that owner's run binds it by.
func (ec *EvalContext) frameFeatureValue(qualifier, sym *symbols.Symbol) (Value, bool) {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		f := ec.frames[i]
		if f.owner == nil || !f.owner.qualifiedBy(ec.ctx, qualifier) {
			continue
		}
		name, ok := f.owner.memberName(ec.ctx, sym)
		if !ok {
			continue
		}
		if val, ok := f.lookup(name); ok {
			return val, true
		}
	}
	return Value{}, false
}

// resolvedWithoutValue reads a name that resolves to sym but no value: undetermined
// at model level, uninitialized (not unresolved) when an object features it.
func (ec *EvalContext) resolvedWithoutValue(sym *symbols.Symbol, qn *ast.QualifiedName) (Value, error) {
	return ec.withoutValue(sym, qualifiedNameToString(qn), qn)
}

// withoutValue reads sym, spelled as written, when it binds no value.
func (ec *EvalContext) withoutValue(sym *symbols.Symbol, spelled string, qn *ast.QualifiedName) (Value, error) {
	// Definitions are types, not values.
	if declaresType(sym) {
		return Value{}, fmt.Errorf("cannot evaluate definition %s", spelled)
	}
	// A calc usage is an evaluation, not a value: it is read through the output
	// features it computes, since a name it does not designate a result for has
	// no one value.
	if isCalcUsageSymbol(sym) {
		return Value{}, fmt.Errorf(
			"%w: calc usage %s computes output features (%s); read one of them",
			ErrNoValue, spelled, ec.ctx.calcUsageOutputSummary(sym),
		)
	}
	// A usage of any kind — a subject or a state included — is a feature.
	if _, usage := sym.Decl.(*ast.Usage); usage || semantics.IsShapeFeature(sym) {
		if ec.modelLevel() {
			return ec.undeterminedFeature(sym, spelled), nil
		}
		return Value{}, &NoValueError{Feature: spelled, Ref: qn}
	}
	return Value{}, fmt.Errorf("cannot evaluate %s %s", sym.Kind, spelled)
}

// modelLevel reports an evaluation featured by nothing: no object, element or
// behavior supplies feature values, so the model alone determines them.
func (ec *EvalContext) modelLevel() bool {
	return ec.self == nil && ec.features == nil && !ec.inBehaviorBody && !ec.hasPerformanceFrame()
}

// undeterminedFeature is the model-level value of a feature nothing gives a
// value to: as many values as its multiplicity states, none of them known;
// the empty sequence where it states there are none.
func (ec *EvalContext) undeterminedFeature(sym *symbols.Symbol, spelled string) Value {
	count := ec.ctx.featureMultiplicity(sym, ec.ctx.findOwnerType(sym))
	if n, exact := count.Exactly(); exact && n == 0 {
		if typed, ok := ec.ctx.emptyOfSymbol(sym); ok {
			return typed
		}
		return sequenceOf(nil)
	}
	return undeterminedFeatureValue(noValueReason(spelled), count, sym)
}

// declaresType reports a symbol that declares a type: a definition, or a KerML
// class, struct, behavior, datatype or function, which the parser records as a
// usage and the symbol builder classifies as the type it declares.
func declaresType(sym *symbols.Symbol) bool {
	if isDefinitionSymbol(sym) {
		return true
	}
	switch sym.Kind {
	case symbols.SymbolKerMLType, symbols.SymbolMetaclass, symbols.SymbolAttributeDef, symbols.SymbolCalcDef:
		return true
	default:
		return false
	}
}

// unresolvedQualifiedName reports a multi-part name the resolver rejected: as
// ambiguous when it named several elements; otherwise against the variants or
// literals of a variation or enumeration the deepest resolved segment reached.
func (ec *EvalContext) unresolvedQualifiedName(qn *ast.QualifiedName, reading resolve.Reading) error {
	written := qualifiedNameToString(qn)
	if qn.Global {
		written = "$::" + written
	}
	if n, ok := reading.Ambiguity(); ok {
		return fmt.Errorf("%w: %s (%d candidates)", ErrAmbiguousReference, written, n)
	}
	for i := len(qn.Parts) - 2; i >= 0; i-- {
		owner, ok := reading.Part(i)
		if !ok {
			continue
		}
		memberName := qn.Parts[i+1].Text
		if owner.Kind == symbols.SymbolEnumerationDef {
			return fmt.Errorf("%w: %s is not a literal of %s (%s)",
				ErrNotALiteral, memberName, owner.Name, ec.ctx.enumerationSummary(owner))
		}
		if ec.ctx.model.semantics.IsVariationFeature(owner) {
			return fmt.Errorf("%w: %s is not a variant of %s (%s)",
				ErrNotAVariant, memberName, owner.Name, ec.ctx.variantSummary(owner))
		}
		if ec.ctx.model.resolver != nil {
			return fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedMember(ec.scope, qn, owner, i+1))
		}
		break
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
}

// declaredValue evaluates the value a declaration binds in the scope it was written
// in (its units and imports answer its names); the value answers to the declared type.
// A namespace-level object usage's value is one binding (KerML 1.0 §7.4.11), kept for the run.
func (ec *EvalContext) declaredValue(sym *symbols.Symbol, value ast.Node) (Value, error) {
	ec.ctx.noteDeclarationRead(sym)
	if val, ok := ec.ctx.namespaceBindings[sym]; ok {
		return val, nil
	}
	if !namespaceObjectUsage(sym) {
		return ec.evaluateDeclared(sym, value)
	}
	if ec.ctx.binding(sym) {
		return Value{}, &CyclicBindingError{Usage: sym, Stated: ec.ctx.qualifiedSymbolName(sym)}
	}
	ec.ctx.bindingStack = append(ec.ctx.bindingStack, sym)
	defer func() { ec.ctx.bindingStack = ec.ctx.bindingStack[:len(ec.ctx.bindingStack)-1] }()
	// The binding is made whole or not at all: a value refused after constructing
	// objects leaves none of them, nor their behaviors, behind.
	commit, rollback := ec.ctx.beginJournal()
	val, err := ec.evaluateDeclared(sym, value)
	if err != nil {
		rollback()
		delete(ec.ctx.bindingReads, sym)
		return Value{}, err
	}
	ec.ctx.bindNamespace(sym, val)
	commit()
	return val, nil
}

// binding reports whether sym's value is being evaluated.
func (ctx *Context) binding(sym *symbols.Symbol) bool {
	return slices.Contains(ctx.bindingStack, sym)
}

// bindNamespace records the value a namespace-level usage denotes for the run; a probe
// that made it is undone with it.
func (ctx *Context) bindNamespace(sym *symbols.Symbol, val Value) {
	ctx.noteProbeUndo(func() { ctx.unbindNamespace(sym) })
	ctx.namespaceBindings[sym] = val
}

// unbindNamespace forgets a namespace-level usage's binding, and what was read to make it.
func (ctx *Context) unbindNamespace(sym *symbols.Symbol) {
	delete(ctx.namespaceBindings, sym)
	delete(ctx.bindingReads, sym)
}

// evaluateDeclared evaluates a declaration's value anew, answering to its declared type.
func (ec *EvalContext) evaluateDeclared(sym *symbols.Symbol, value ast.Node) (Value, error) {
	val, err := ec.evalIn(sym.OwnerScope).Eval(value)
	if err != nil {
		return Value{}, err
	}
	what := fmt.Sprintf("feature value %s", ec.ctx.qualifiedSymbolName(sym))
	if err := ec.ctx.checkWriteType(sym.OwnerScope, what, ec.ctx.extractType(sym), &val, admitDeclared); err != nil {
		return Value{}, err
	}
	if msg := ec.ctx.declaredUniquenessRefusal(sym, &val); msg != "" {
		return Value{}, fmt.Errorf("%s: %w: %s", what, ErrUniquenessViolation, msg)
	}
	if msg := ec.ctx.declaredCountRefusal(sym, &val); msg != "" {
		return Value{}, fmt.Errorf("%s: %w: %s", what, ErrMultiplicityViolation, msg)
	}
	if err := ec.ctx.classifyHeld(sym, val); err != nil {
		return Value{}, fmt.Errorf("%s: %w", what, err)
	}
	return ec.bindVariationOf(sym, ec.ctx.classifiedFrame(sym, ec.ctx.declaredCollection(sym, val)))
}

// occurrenceReference evaluates a name denoting one object — an occurrence or a
// structured value — as that object, materialized once. Reports whether the
// symbol denotes such an object.
func (ec *EvalContext) occurrenceReference(sym *symbols.Symbol) (Value, bool, error) {
	if !ec.ctx.namesOneObject(sym) {
		return Value{}, false, nil
	}
	ec.ctx.noteDeclarationRead(sym)
	inst, err := ec.ctx.occurrenceOf(sym)
	if err != nil {
		return Value{}, true, fmt.Errorf("usage %s: %w", symbolText(sym), err)
	}
	val, err := ec.ctx.objectValue(inst)
	return val, true, err
}

// emptyDeclaredFeature reads a valueless feature declaration whose lower bound is
// zero as the empty sequence an object holding nothing reads. At model level no
// object holds it, so it stays undetermined. A variation is a choice, not an empty feature.
func (ec *EvalContext) emptyDeclaredFeature(sym *symbols.Symbol) (Value, bool) {
	if ec.modelLevel() || !ec.ctx.optionalValueless(sym) || ec.ctx.model.semantics.IsVariationFeature(sym) {
		return Value{}, false
	}
	return sequenceOf(nil), true
}

// namesSelf reports whether the name resolves, where the expression was written,
// to the `self` feature every thing has of itself or a restatement of it.
func (ec *EvalContext) namesSelf(name string) bool {
	if ec.scope == nil {
		return false
	}
	sym, ok := ec.ctx.lookupName(ec.scope, name)
	return ok && ec.ctx.model.semantics.IsSelf(sym)
}

// namesOccurrenceThis reports whether the name resolves to the library's
// context occurrence feature `this` where the expression was written.
func (ec *EvalContext) namesOccurrenceThis(name string) bool {
	if ec.scope == nil {
		return false
	}
	sym, ok := ec.ctx.lookupName(ec.scope, name)
	return ok && ec.ctx.model.resolver.IsOccurrenceThis(sym)
}

// thisValue is the object owning the performance being evaluated. A body no
// object owns has none: `this` there is the performance itself.
func (ec *EvalContext) thisValue() (Value, error) {
	object := ec.ctx.model.resolver.ThisContext(ec.scope)
	if object == nil {
		return Value{}, fmt.Errorf("%w: this names the performance itself, which no object owns",
			ErrThisNotAnObject)
	}
	if ec.self == nil {
		return Value{}, fmt.Errorf("%w: no object of %s performs this body",
			ErrThisNotAnObject, symbolText(object))
	}
	return Value{Kind: ValInstance, Instance: ec.self.ID}, nil
}

// selfFeatureInScope reports whether the bound instance's feature of that name
// may answer here: in a behavior body only when the name resolves to it.
func (ec *EvalContext) selfFeatureInScope(name string) bool {
	if !ec.inBehaviorBody {
		return true
	}
	return namesPerformerFeature(ec.ctx, ec.self, ec.scope, name)
}

// selfFeatureValue reads the named feature value of the bound instance. Reports whether the
// instance has such a feature value; an error means the feature value exists but could not be
// materialized.
func (ec *EvalContext) selfFeatureValue(name string) (Value, bool, error) {
	if _, ok := ec.self.FeatureValues[name]; !ok {
		return Value{}, false, nil
	}
	fv, err := ec.self.GetFeatureValue(ec.ctx, name)
	if err != nil {
		return Value{}, true, err
	}
	value, err := ec.ctx.readFeatureValue(fv, name)
	if err != nil {
		return value, true, err
	}
	// An object the feature holds is read as what it denotes, as a chain reads it.
	if inst, ok := ec.ctx.instances[value.Instance]; ok && value.Kind == ValInstance {
		value, err = ec.ctx.objectValue(inst)
	}
	return value, true, err
}

// evalFeatureChain evaluates a feature chain expression (e.g., obj.member.submember).
func (ec *EvalContext) evalFeatureChain(n *ast.FeatureChainExpr) (Value, error) {
	if n.Member == nil || len(n.Member.Parts) == 0 {
		return Value{}, fmt.Errorf("empty member chain")
	}
	base, parts := chainBase(n)

	// A node of an action performance on the stack carries its pins in its own
	// performance, which `p.v` reads.
	if name := simpleEndName(base); name != "" {
		if perf, declared, err := ec.lookupSubaction(name); declared {
			if err != nil {
				return Value{}, err
			}
			return ec.evalSubactionPath(perf, parts)
		}
	}

	// A calc usage carries no value of its own: its output features are computed
	// by evaluating it, so `c.a` runs the usage — once — and reads the output
	// from that evaluation rather than from a feature value.
	if sym, ok := ec.calcUsageOperand(base); ok {
		return ec.evalCalcUsageMembers(sym, parts)
	}

	// A part carries no value of its own: it denotes an occurrence, whose features
	// `lander.mass.mDry` reads, so the chain is read from that object.
	if sym, ok := ec.occurrenceOperand(base); ok {
		// A usage of an enclosing object is read from that object, so a sibling
		// chain `e1.length` inside `e3` reads the containing rectangle's e1.
		if val, ok, err := ec.outerFeatureValue(sym); ok {
			if err != nil {
				return Value{}, err
			}
			return ec.chainMemberValue(val, parts, sym.Name)
		}
		inst, err := ec.ctx.occurrenceOf(sym)
		if err != nil {
			return Value{}, fmt.Errorf("usage %s: %w", sym.Name, err)
		}
		// The object reads as its value, whose own members it answers before the object's.
		val, err := ec.ctx.objectValue(inst)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, parts, sym.Name)
	}

	// Evaluate the operand (left side of the chain)
	operand, err := ec.Eval(n.Operand)
	if err != nil {
		var noValue *NoValueError
		if errors.As(err, &noValue) {
			if unresolved := ec.chainMembersDeclared(base, parts); unresolved != nil {
				return Value{}, unresolved
			}
		}
		return Value{}, err
	}

	if operand.Kind == ValInstance {
		if _, ok := ec.ctx.instances[operand.Instance]; !ok {
			return Value{}, fmt.Errorf("instance ID %d not found", operand.Instance)
		}
	}
	// An undetermined value a feature holds is chained as that feature's, so its
	// members are the ones the feature declares however the value was computed.
	if u := operand.Undetermined(); u != nil && u.feature == nil {
		if sym, ok := ec.chainBaseSymbol(base); ok {
			operand = undeterminedFeatureValue(u.reason, u.count, sym)
		}
	}

	return ec.chainMemberValue(operand, n.Member.Parts, "")
}

// chainMembersDeclared reports the first chain member nothing declares, so
// `wheels.nonexistent` is unresolved rather than unset when wheels has no value.
func (ec *EvalContext) chainMembersDeclared(base ast.Node, parts []ast.NameSegment) error {
	cur, ok := ec.chainBaseSymbol(base)
	if !ok {
		return nil
	}
	for _, part := range parts {
		next, ok := ec.ctx.declaredMember(cur, part.Text)
		if !ok {
			return fmt.Errorf("%w: %s has no member %s", ErrUnresolvedReference, cur.Name, part.Text)
		}
		cur = next
	}
	return nil
}

// chainBaseSymbol is the feature a chain's base names, when it names one.
func (ec *EvalContext) chainBaseSymbol(base ast.Node) (*symbols.Symbol, bool) {
	ref, ok := base.(*ast.FeatureReference)
	if !ok || ref.Name == nil || ec.ctx.model.resolver == nil {
		return nil, false
	}
	sym, ok := ec.ctx.resolveQualified(ec.scope, ref.Name)
	return sym, ok && sym != nil
}

// declaredMember is what an object of sym holds under name: a feature of its
// shape, or a member (calc usage, variant) the model reaches by name.
func (ctx *Context) declaredMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	for _, feat := range ctx.FeaturesOf(sym) {
		if feat.Name == name && feat.Symbol != nil {
			return feat.Symbol, true
		}
	}
	return ctx.model.semantics.LookupMember(sym, name)
}

// chainBase flattens a nested feature chain: `lander.mass.mDry` is one chain of
// members from `lander`, not a chain through the value of `lander.mass`.
func chainBase(n *ast.FeatureChainExpr) (ast.Node, []ast.NameSegment) {
	operand, parts := n.Operand, n.Member.Parts
	for {
		inner, ok := operand.(*ast.FeatureChainExpr)
		if !ok || inner.Member == nil || len(inner.Member.Parts) == 0 {
			return operand, parts
		}
		parts = append(append([]ast.NameSegment{}, inner.Member.Parts...), parts...)
		operand = inner.Operand
	}
}

// chainMemberValue reads the members named by parts from the object value names,
// navigating through the objects the intermediate members name. from names the
// member value came from, for a diagnostic about chaining through it.
//
// A chain's values are its last feature's values over every object the features
// before it name (KerML 1.0 §7.3.4.6), so a multi-valued member is navigated
// through each of its objects, concatenated in order and flattened one level.
func (ec *EvalContext) chainMemberValue(value Value, parts []ast.NameSegment, from string) (Value, error) {
	if len(parts) == 0 {
		if inst, ok := ec.ctx.instances[value.Instance]; ok && value.Kind == ValInstance {
			return ec.ctx.objectValue(inst)
		}
		return value, nil
	}
	name, rest := parts[0].Text, parts[1:]

	if literal := value.EnumerationLiteral(); literal != nil {
		// A literal is an occurrence of its enumeration, so its own features are
		// read from the object that literal stands for, whatever scalar it equals.
		inst, err := ec.ctx.enumLiteralObject(literal)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(Value{Kind: ValInstance, Instance: inst.ID}, parts, from)
	}
	switch value.Kind {
	case ValSequence, ValSet:
		return ec.chainOverElements(value, parts, from)
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity, ValQuantity, ValMeasurementRef, ValCoordinateFrame, ValCoordinateTransformation:
		// An array or vector read from an object keeps that object's members; a
		// frame answers its own features from the value, then from its object.
		if inst, ok := ec.ctx.structuredObject(value); ok && isStructuredValue(&value) {
			return ec.chainMemberValue(Value{Kind: ValInstance, Instance: inst.ID}, parts, from)
		}
		return ec.chainOwnFeature(value, parts, from)
	case ValInstance, ValVariant:
		// handled below
	case ValMetaobject:
		// A metaobject answers its metaclass's features for the element it denotes.
		return ec.chainOwnFeature(value, parts, from)
	case ValUndetermined:
		return ec.chainThroughUndetermined(value, parts, from)
	default:
		if err := metadataOfAValue(value, parts); err != nil {
			return Value{}, err
		}
		return Value{}, fmt.Errorf("cannot chain through non-instance member %s (%v)", from, value.Kind)
	}

	// A selected variant is chained through the object it materialized.
	id, isObject := value.Object()
	if !isObject {
		if err := metadataOfAValue(value, parts); err != nil {
			return Value{}, err
		}
		return Value{}, fmt.Errorf("cannot chain through non-instance member %s (%v)", from, value.Kind)
	}
	inst, ok := ec.ctx.instances[id]
	if !ok {
		return Value{}, fmt.Errorf("instance ID %d not found for member %s", id, from)
	}
	// A frame, scale or transformation object answers its members from the value it is.
	if ref, isRef, err := ec.ctx.referenceValueOfObject(inst); isRef {
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(ref, parts, from)
	}
	// A shaped Array object answers Array's features and their redefinitions from
	// the value; its other members stay the object's, whatever their names.
	if arr, isArray, err := ec.ctx.arrayOfObject(inst); isArray {
		if err != nil {
			return Value{}, err
		}
		if base, member, ok := ec.ctx.arrayFeatureNamed(inst.Type, name); ok {
			answer, err := ec.ctx.structuredMember(arr, base, member, inst.Type)
			if err != nil {
				return Value{}, err
			}
			return ec.chainMemberValue(answer, rest, name)
		}
	}
	fvDecl, ok := inst.FeatureValues[name]
	if !ok {
		// A calc usage is an evaluation rather than a feature value, so its outputs are
		// read from a run of it against this object.
		if sym, found := ec.ctx.model.semantics.LookupMember(inst.Type, name); found && isCalcUsageSymbol(sym) {
			return ec.calcUsageMemberValue(sym, inst, rest)
		}
		return Value{}, fmt.Errorf("%w: member %s not found in instance", ErrNoSuchFeature, name)
	}
	// A variant named through the variation feature it belongs to is the choice
	// itself, not a member of the variation's value.
	if variant, rest, ok := ec.variantSegment(fvDecl.Feature, rest); ok {
		if len(rest) == 0 {
			return variantReference(variant), nil
		}
		// Members are read from the object the variant stands for.
		val, err := ec.ctx.variantValue(fvDecl.Feature.Symbol, variant, inst.ID)
		if err != nil {
			return Value{}, err
		}
		return ec.chainMemberValue(val, rest, variant.Name)
	}
	// Read through the feature value so a derived or composite member is materialized
	// on demand rather than read as an empty feature value.
	fv, open, err := ec.memberFeatureValue(inst, name)
	if err != nil {
		return Value{}, err
	}
	// An object read through the model holds, for a feature whose count the model
	// leaves open, no fixed sequence of values.
	if val, ok := ec.openFeatureRead(inst, fv, open, from, name); ok {
		return ec.chainMemberValue(val, rest, name)
	}
	member, err := ec.ctx.readFeatureValue(fv, name)
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(member, rest, name)
}

// chainOwnFeature continues a chain through a feature the value answers from
// itself rather than from an object it names.
func (ec *EvalContext) chainOwnFeature(value Value, parts []ast.NameSegment, from string) (Value, error) {
	member, err := ec.ownFeature(value, parts[0].Text)
	if err != nil {
		return Value{}, err
	}
	return ec.chainMemberValue(member, parts[1:], from)
}

// ownFeature reads a feature a value answers from itself: a metaobject's
// reflective feature, or a library feature of an array, vector, quantity,
// measurement reference or frame.
func (ec *EvalContext) ownFeature(value Value, name string) (Value, error) {
	if value.Kind == ValMetaobject {
		return ec.metaobjectFeature(value, name)
	}
	member, ok, err := ec.ctx.structuredFeature(value, name)
	if err != nil {
		return Value{}, err
	}
	if !ok {
		return Value{}, fmt.Errorf("%w: %s has no feature %s", ErrTypeMismatch, describeValue(value), name)
	}
	return member, nil
}

// chainOverElements reads the rest of a chain from every element of a
// multi-valued member, concatenating the values each contributes.
func (ec *EvalContext) chainOverElements(value Value, parts []ast.NameSegment, from string) (Value, error) {
	var collected, reads []Value
	for _, elem := range elementsOf(value) {
		val, err := ec.chainMemberValue(elem, parts, from)
		if err != nil {
			return Value{}, err
		}
		contributed := elementsOf(val)
		if err := ec.ctx.chargeElements(int64(len(contributed))); err != nil {
			return Value{}, err
		}
		collected = append(collected, contributed...)
		reads = append(reads, val)
	}
	// A member some element leaves undetermined is undetermined over all of them.
	if _, open := undeterminedIn(reads...); open {
		return undeterminedElements(reads), nil
	}
	if len(collected) == 0 {
		if unit, ok := elementUnitOf(reads...); ok {
			return NewEmptySequenceOf(unit), nil
		}
	}
	return sequenceOf(collected), nil
}

// enumLiteralValue is the value a literal declares — a scalar-specializing
// enumeration's literal *is* its value — else the identity of the literal.
func (ec *EvalContext) enumLiteralValue(sym *symbols.Symbol) (Value, error) {
	value := semantics.LiteralValue(sym)
	if value == nil {
		return NewEnumLiteral(sym), nil
	}
	val, err := ec.evalIn(declScope(sym)).Eval(value)
	if err != nil {
		return Value{}, fmt.Errorf("enumeration literal %s: %w", sym.Name, err)
	}
	return val.ofLiteral(sym), nil
}

// EnumerationLiteralValue is the value sym has when it is an enumeration
// literal, reported as such so a caller holding only a symbol — an `%eval` of a
// literal — answers with the value rather than "no value".
func (ctx *Context) EnumerationLiteralValue(sym *symbols.Symbol) (Value, bool, error) {
	if semantics.EnumerationOwning(sym) == nil {
		return Value{}, false, nil
	}
	val, err := NewEvalContext(ctx, declScope(sym)).enumLiteralValue(sym)
	return val, true, err
}

// enumLiteralObject returns the object a literal stands for, materialized once
// so the features it carries read the same object every time.
func (ctx *Context) enumLiteralObject(literal *symbols.Symbol) (*Instance, error) {
	if literal == nil {
		return nil, fmt.Errorf("%w: the literal was never resolved", ErrNotALiteral)
	}
	inst, err := ctx.occurrenceOf(literal)
	if err != nil {
		return nil, fmt.Errorf("enumeration literal %s: %w", literal.Name, err)
	}
	return inst, nil
}

// enumerationSummary names the literals an enumeration declares, for a report
// about a qualified name that is none of them.
func (ctx *Context) enumerationSummary(enum *symbols.Symbol) string {
	literals := ctx.model.semantics.LiteralsOf(enum)
	if len(literals) == 0 {
		return "it declares no literals"
	}
	names := make([]string, 0, len(literals))
	for _, lit := range literals {
		names = append(names, lit.Name)
	}
	return "literals: " + strings.Join(names, ", ")
}

// unimplementedOperators names the operators the runtime does not evaluate and
// says what each would need, so reaching one reports why rather than "unsupported".
var unimplementedOperators = map[ast.OperatorKind]string{
	ast.OpBitNot: "bitwise complement is declared by no function library the runtime applies",
	ast.OpIndex:  "indexing is evaluated from an IndexExpression, not this operator",
}

// evalOperator evaluates an operator expression. A constant one is answered by
// the folder; every other operator the folder recognizes is evaluated here, so
// an operand that depends on a parameter does not make the operator fail.
func (ec *EvalContext) evalOperator(n *ast.OperatorExpr) (Value, error) {
	// Try constant folding first
	if semVal, ok := ec.ctx.model.semantics.Eval(n); ok {
		return Value{Kind: ValConst, Const: semVal}, nil
	}

	// Otherwise, recursively eval operands
	switch n.Operator {
	case ast.OpConditional:
		return ec.evalConditional(n)
	case ast.OpNullCoalesce:
		return ec.evalNullCoalesce(n)
	case ast.OpAdd, ast.OpSub, ast.OpMul, ast.OpDiv, ast.OpMod, ast.OpPow:
		return ec.evalArithmetic(n)
	case ast.OpEq, ast.OpNeq:
		return ec.evalEquality(n)
	case ast.OpEqEqEq, ast.OpNeqEqEq:
		return ec.evalIdentity(n)
	case ast.OpLt, ast.OpLe, ast.OpGt, ast.OpGe:
		return ec.evalComparison(n)
	case ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr, ast.OpXor, ast.OpImplies:
		return ec.evalLogical(n)
	case ast.OpNeg, ast.OpPos, ast.OpNot:
		return ec.evalUnary(n)
	case ast.OpRange:
		return ec.evalRange(n)
	case ast.OpAt:
		if ec.classifiesValue(n) {
			return ec.evalTypeClassification(n)
		}
		return ec.evalClassification(n)
	case ast.OpMetaAt:
		return ec.evalClassification(n)
	case ast.OpHasType, ast.OpIsType:
		return ec.evalTypeClassification(n)
	case ast.OpAs:
		return ec.evalCast(n)
	case ast.OpAll:
		return ec.evalExtent(n)
	case ast.OpMeta:
		return ec.evalMetaCast(n)
	default:
		if why, ok := unimplementedOperators[n.Operator]; ok {
			return Value{}, fmt.Errorf("%w: '%s': %s", ErrUnsupportedOperator, n.Operator, why)
		}
		return Value{}, fmt.Errorf("%w: '%s'", ErrUnsupportedOperator, n.Operator)
	}
}

// classifiesValue reports whether `x @ T` is `x istype T`: a subject is written
// and T is an ordinary type, not a metadata type (which only annotations have).
func (ec *EvalContext) classifiesValue(n *ast.OperatorExpr) bool {
	if len(n.Operands) != 1 || n.TypeRef == nil {
		return false
	}
	target, ok := ec.resolveClassificationType(n.TypeRef)
	return ok && !semantics.IsMetadataType(target)
}

// resolveClassificationType resolves the type a classification names, seeing
// through an alias to the type it stands for.
func (ec *EvalContext) resolveClassificationType(qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	target, ok := ec.ctx.resolveQualified(ec.scope, qn)
	if !ok || target == nil {
		return nil, false
	}
	if canonical, ok := ec.ctx.resolveAliasTarget(target); ok {
		target = canonical
	}
	ec.ctx.noteDeclarationRead(target)
	return target, true
}

// evalTypeClassification evaluates `x hastype T`, `x istype T` and the value form of `x @ T`
// (KerML 1.0 §7.4.9.2): `hastype` reads the direct types alone, `@` holds when any value is of T.
func (ec *EvalContext) evalTypeClassification(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 1 || n.TypeRef == nil {
		return Value{}, fmt.Errorf("%w: '%s' requires one value and one type",
			ErrTypeMismatch, n.Operator)
	}
	target, ok := ec.resolveClassificationType(n.TypeRef)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedType,
			qualifiedNameToString(n.TypeRef))
	}
	value, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	declared := ec.declaredOperandTypes(n.Operands[0])
	if value.Kind == ValUndetermined {
		return classifyUndetermined(n.Operator, value, ec.undeterminedClassification(declared, target)), nil
	}
	by := byAnyType
	if n.Operator == ast.OpHasType {
		by = byOwnType
	}
	matches, err := ec.valuesClassified(soleElement(value), target, declared, by, n.Operator == ast.OpAt)
	if err != nil {
		return Value{}, err
	}
	return boolValue(matches), nil
}

// valuesClassified reports whether target classifies every value of value — or, for `@`, any
// (KerML 1.0 §7.4.9.2) — so an empty value satisfies `istype` and `hastype` and fails `@`.
func (ec *EvalContext) valuesClassified(
	value Value, target *symbols.Symbol, declared []*symbols.Symbol, by classifiedBy, anyOf bool,
) (bool, error) {
	var elements []Value
	switch value.Kind {
	case ValNull, ValInvalid, ValSequence, ValSet:
		elements = elementsOf(value)
	default:
		elements = []Value{value}
	}
	for _, element := range elements {
		verdict, err := ec.ctx.classifyValue(ec.scope, element, target, declared, by)
		if err != nil {
			return false, err
		}
		if (verdict == semantics.ClassifiesAll) == anyOf {
			return anyOf, nil
		}
	}
	return !anyOf, nil
}

// directValueTypes names the types a value is of, resolved in the scope reading it:
// the scalar type of a constant, the enumeration a literal belongs to, for an object its
// declared type then the type of each feature it was held as a value of, and for a selected
// variant the variant itself then what its object was held by.
func (ctx *Context) directValueTypes(scope *symbols.Scope, value Value) ([]*symbols.Symbol, error) {
	id, ok := value.Object()
	if !ok {
		typ, err := ctx.directValueType(scope, value)
		if err != nil {
			return nil, err
		}
		return []*symbols.Symbol{typ}, nil
	}
	inst, ok := ctx.instances[id]
	if !ok || inst == nil || inst.Type == nil {
		return nil, fmt.Errorf("%w: instance %d", ErrUndeterminedValueType, id)
	}
	var out []*symbols.Symbol
	types := inst.types()
	if value.Kind == ValVariant {
		out, types = []*symbols.Symbol{value.Variant()}, inst.classifiers
	}
	for _, sym := range types {
		if typ := ctx.directType(sym); !slices.Contains(out, typ) {
			out = append(out, typ)
		}
	}
	return out, nil
}

// directType is the type an object is of for being of sym: the type a feature is typed
// by, else sym itself.
func (ctx *Context) directType(sym *symbols.Symbol) *symbols.Symbol {
	if typ := ctx.extractType(sym); typ != nil {
		return typ
	}
	return sym
}

// directValueType names the one type a value is of (see directValueTypes); an object
// answers with its declared type, a selected variant with the variant.
func (ctx *Context) directValueType(scope *symbols.Scope, value Value) (*symbols.Symbol, error) {
	if _, ok := value.Object(); ok {
		types, err := ctx.directValueTypes(scope, value)
		if err != nil {
			return nil, err
		}
		return types[0], nil
	}
	switch value.Kind {
	case ValConst:
		switch value.Const.Kind {
		case semantics.ValInt, semantics.ValReal, semantics.ValBool:
			return ctx.scalarValueType(scope, value)
		case semantics.ValInfinity:
			// `*` is the natural number exceeding every other (KerML 8.4.4.6).
			if positive := ctx.librarySymbol(positiveTypeFQN); positive != nil {
				return positive, nil
			}
			return nil, fmt.Errorf("%w: direct type %q", ErrUndeterminedValueType, positiveTypeFQN)
		default:
			return nil, fmt.Errorf("%w: %s", ErrUndeterminedValueType, value.Kind)
		}
	case ValString, ValComplex:
		return ctx.scalarValueType(scope, value)
	case ValVariant:
		if value.Variant() == nil {
			return nil, fmt.Errorf("%w: variant", ErrUndeterminedValueType)
		}
		return value.Variant(), nil
	case ValEnumLiteral:
		if value.Literal() == nil {
			return nil, fmt.Errorf("%w: enumeration literal", ErrUndeterminedValueType)
		}
		enum := semantics.EnumerationOwning(value.Literal())
		if enum == nil {
			return nil, fmt.Errorf("%w: enumeration literal %s",
				ErrUndeterminedValueType, value.Literal().Name)
		}
		return enum, nil
	case ValFunction:
		// A function is of the calc it is a value of: a usage's type is that usage.
		if value.Function() == nil {
			return nil, fmt.Errorf("%w: function", ErrUndeterminedValueType)
		}
		return value.Function(), nil
	case ValMetaobject:
		// A metaobject is an instance of the reflective metaclass of its element.
		if value.MetaobjectClass() == nil {
			return nil, fmt.Errorf("%w: metaobject", ErrUndeterminedValueType)
		}
		return value.MetaobjectClass(), nil
	case ValQuantity:
		if value.Quantity() == nil {
			return nil, fmt.Errorf("%w: quantity", ErrUndeterminedValueType)
		}
		return ctx.directValueType(scope, Value{Kind: ValConst, Const: value.Quantity().Num})
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity:
		return ctx.structuredValueType(value)
	case ValMeasurementRef:
		return ctx.measurementRefValueType(value.MeasurementRef())
	case ValCoordinateFrame:
		return ctx.frameValueType(value.CoordinateFrame())
	case ValCoordinateTransformation:
		return ctx.transformationValueType(value.CoordinateTransformation())
	default:
		return nil, fmt.Errorf("%w: %s", ErrUndeterminedValueType, value.Kind)
	}
}

// scalarValueType is the ScalarValues type a scalar's representation states (a finite
// real a Rational, KerML 8.4.4.9.2), whatever same-named type scope sees; a NaN is of none.
func (ctx *Context) scalarValueType(scope *symbols.Scope, value Value) (*symbols.Symbol, error) {
	prim := representationPrim(value)
	if prim == semantics.PrimUnknown {
		return nil, fmt.Errorf("%w: NaN is of no scalar type", ErrUndeterminedValueType)
	}
	if scalar := ctx.scalarLibraryType(value); scalar != nil {
		return scalar, nil
	}
	fqn := semantics.ScalarFQN(prim)
	// Only in a library-free model may a same-named type the model declares stand in.
	if !ctx.libraryLoaded() {
		if typ := ctx.resolveType(scope, fqn[strings.LastIndex(fqn, "::")+2:]); typ != nil {
			return typ, nil
		}
	}
	return nil, fmt.Errorf("%w: direct type %q", ErrUndeterminedValueType, fqn)
}

// evalClassification evaluates `@T` (metadata T annotates the subject) and `@@T`
// (the subject's own metaclass conforms to T) through the semantic model, so the
// verdict is the one an element filter writing the same test reaches.
func (ec *EvalContext) evalClassification(n *ast.OperatorExpr) (Value, error) {
	elem, err := ec.classifiedElement(n)
	if err != nil {
		return Value{}, err
	}
	classified, err := ec.ctx.model.semantics.EvalClassification(ec.scope, n, elem)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: classified}}, nil
}

// classifiedElement is the element a classification's subject denotes, since
// metadata annotates elements and not values: the object being evaluated for an
// implicit subject or `self`, the element a name names, else what its value
// denotes.
func (ec *EvalContext) classifiedElement(n *ast.OperatorExpr) (*symbols.Symbol, error) {
	if len(n.Operands) > 1 {
		return nil, semantics.UnevaluableClassification(
			fmt.Sprintf("`%s` classifies one subject, and %d were given", n.Operator, len(n.Operands)), n.Span())
	}
	if len(n.Operands) == 0 || isSelfName(n.Operands[0]) {
		if ec.self == nil {
			return nil, semantics.UnevaluableClassification(
				fmt.Sprintf("`%s` leaves its subject implicit and no object is being evaluated", n.Operator), n.Span())
		}
		return ec.self.Type, nil
	}
	subject := n.Operands[0]
	// A name is the element it names: what `p @ Safety` classifies is the
	// declaration p, the same element a filter condition would be judged for.
	if qn := subjectName(subject); qn != nil {
		if sym, ok := ec.ctx.resolveQualified(ec.scope, qn); ok && sym != nil {
			return sym, nil
		}
	}
	val, err := ec.Eval(subject)
	if err != nil {
		return nil, err
	}
	elem, ok := ec.elementDenotedBy(val)
	if !ok {
		return nil, semantics.UnevaluableClassification(
			fmt.Sprintf("a %s denotes no element to classify", val.Kind), subject.Span())
	}
	return elem, nil
}

// elementDenotedBy is the element a value stands for: the classifier an object
// was materialized from, the variant a variation was bound to, or the literal an
// enumeration value is. Every other value is a datum, which nothing annotates.
func (ec *EvalContext) elementDenotedBy(val Value) (*symbols.Symbol, bool) {
	switch val.Kind {
	case ValInstance:
		inst, ok := ec.ctx.instances[val.Instance]
		if !ok || inst.Type == nil {
			return nil, false
		}
		return inst.Type, true
	case ValVariant:
		return val.Variant(), val.Variant() != nil
	case ValMetaobject:
		return val.MetaobjectElement(), val.MetaobjectElement() != nil
	default:
		literal := val.EnumerationLiteral()
		return literal, literal != nil
	}
}

// subjectName is the qualified name a classification's subject is written as, or
// nil for a subject that is no name.
func subjectName(n ast.Node) *ast.QualifiedName {
	switch subject := n.(type) {
	case *ast.FeatureReference:
		return subject.Name
	case *ast.QualifiedName:
		return subject
	default:
		return nil
	}
}

// isSelfName reports whether a subject is written as `self`, which names the
// object being evaluated — the same subject the notation leaves out.
func isSelfName(n ast.Node) bool {
	qn := subjectName(n)
	return qn != nil && len(qn.Parts) == 1 && qn.Parts[0].Text == "self"
}

// evalConditional evaluates `if c ? a else b`, evaluating only the branch the
// condition selects — the other one is never evaluated, so a guarded recursion
// terminates at its base case.
func (ec *EvalContext) evalConditional(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 3 {
		return Value{}, fmt.Errorf("conditional requires 3 operands, got %d", len(n.Operands))
	}
	cond, err := ec.valueOperand(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	// A condition the model leaves open selects no branch, so neither is
	// evaluated; the result holds as many values as either branch declares.
	if cond.Kind == ValUndetermined {
		count := ec.declaredCount(ec.scope, n.Operands[1]).Covering(ec.declaredCount(ec.scope, n.Operands[2]))
		return undeterminedOf(count, cond), nil
	}
	held, err := boolOperand("condition of 'if'", cond)
	if err != nil {
		return Value{}, err
	}
	if held {
		return ec.Eval(n.Operands[1])
	}
	return ec.Eval(n.Operands[2])
}

// evalNullCoalesce evaluates `a ?? b`, evaluating b only when a is empty.
func (ec *EvalContext) evalNullCoalesce(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("'??' requires 2 operands, got %d", len(n.Operands))
	}
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	second := func() (Value, error) { return ec.Eval(n.Operands[1]) }
	return coalesceNull(left, second, ec.declaredCount(ec.scope, n.Operands[1]))
}

// coalesceNull is `??` over an evaluated first operand: the operand unless it
// is empty, else the second operand, evaluated only then. fallback is the count
// the second would yield, as far as the declarations fix it.
func coalesceNull(first Value, second func() (Value, error), fallback semantics.Range) (Value, error) {
	// An open operand that may be empty leaves which operand `??` yields open:
	// the first, holding at least one value, or the second.
	if first.Kind == ValUndetermined && !certainlyNonEmpty(first) {
		if certainlyEmpty(first) {
			return second()
		}
		return undeterminedOf(nonEmptyCount(countOf(first)).Covering(fallback), first), nil
	}
	if !isEmptyValue(first) {
		return first, nil
	}
	return second()
}

// isEmptyValue reports whether a value is the empty sequence, which `null`,
// `()` and an empty set all denote.
func isEmptyValue(val Value) bool {
	switch val.Kind {
	case ValNull:
		return true
	case ValSequence, ValSet:
		return len(elementsOf(val)) == 0
	}
	return false
}

// evalIdentity evaluates the identity operators (===, !==). Two values are the
// same one when they have the same kind and the same content, so an Integer is
// never identical to a Real of equal magnitude.
func (ec *EvalContext) evalIdentity(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("identity requires 2 operands, got %d", len(n.Operands))
	}
	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	if _, open := undeterminedIn(left, right); open {
		return undeterminedResult(left, right), nil
	}

	same := valueIdentical(left, right)
	if n.Operator == ast.OpNeqEqEq {
		same = !same
	}
	return boolValue(same), nil
}

// valueIdentical reports whether two values are the same value, which is what
// the identity operator `===` and SequenceFunctions::same ask. Identity is
// stricter than equality: a value of another kind, or a constant of another
// kind, is never the same value, so an Integer is not identical to a Real of
// equal magnitude, nor an enumeration's literal to the bare scalar it equals or
// to another enumeration's literal of that value.
func valueIdentical(left, right Value) bool {
	if isEmptyValue(left) || isEmptyValue(right) {
		return isEmptyValue(left) && isEmptyValue(right)
	}
	if left.Kind != right.Kind || left.EnumerationLiteral() != right.EnumerationLiteral() {
		return false
	}
	if left.Kind == ValConst && left.Const.Kind != right.Const.Kind {
		return false
	}
	return valueEqual(left, right)
}

// evalArithmetic evaluates arithmetic operators (+, -, *, /, %, **).
func (ec *EvalContext) evalArithmetic(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) < 2 {
		return Value{}, fmt.Errorf("arithmetic operator requires 2 operands")
	}
	left, err := ec.valueOperand(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	right, err := ec.valueOperand(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	// Operator notation over a vector is the VectorFunctions operator of the same
	// symbol, which specializes DataFunctions'; over a tensor, TensorCalculations'.
	if val, ok, err := ec.ctx.vectorArithmetic(n.Operator, left, right); ok {
		return val, err
	}
	if val, ok, err := ec.ctx.tensorArithmetic(n.Operator, left, right); ok {
		return val, err
	}
	return ec.ctx.arithmeticValues(n.Operator, left, right, n.Span())
}

// arithmeticValues applies a binary arithmetic operator to two evaluated
// operands; the operator notation and the library's `'+'` forms both use it.
func (ctx *Context) arithmeticValues(op ast.OperatorKind, left, right Value, span source.Span) (Value, error) {
	// An operand the model leaves open leaves the result open, unless the
	// determined operand alone makes the operation fail.
	if _, open := undeterminedIn(left, right); open {
		if err := definiteArithmeticError(op, left, right); err != nil {
			return Value{}, err
		}
		return undeterminedResult(left, right), nil
	}
	// '+' over two strings concatenates, the one arithmetic operator
	// StringFunctions declares; a non-string operand is not coerced.
	if op == ast.OpAdd && left.Kind == ValString && right.Kind == ValString {
		return concatStrings(left.Str(), right.Str()), nil
	}

	// A product, quotient or power of measurement references is the unit it composes.
	if ref, ok, err := ctx.composeMeasurementRefs(op, left, right); ok {
		return ref, err
	}
	// A coordinate frame times or over a unit is the frame of composed axes.
	if frame, ok, err := ctx.composeFrame(op, left, right); ok {
		return frame, err
	}

	// A quantity carries its unit through arithmetic: a sum converts, a product
	// composes units.
	if lq, rq, ok := quantityOperands(left, right); ok {
		switch op {
		case ast.OpAdd, ast.OpSub:
			return ctx.addQuantities(op, lq, rq)
		case ast.OpMul, ast.OpDiv:
			return ctx.scaleQuantities(op, lq, rq)
		case ast.OpPow:
			if right.Kind != ValConst {
				return Value{}, fmt.Errorf("%w: exponent of a quantity is a quantity", ErrTypeMismatch)
			}
			return ctx.powQuantity(lq, right.Const)
		case ast.OpMod:
			return Value{}, fmt.Errorf("%w: '%%' is not defined for a quantity", ErrTypeMismatch)
		}
	}

	// A complex operand makes the operation ComplexFunctions', the numeric
	// operand beside it being a Complex too.
	if lz, rz, ok := complexOperands(left, right); ok {
		return complexArithmetic(op, lz, rz, left, right, span)
	}

	// Arithmetic is defined on constants; anything else names the operator and
	// both operand types rather than reporting a bare mismatch.
	if left.Kind != ValConst || right.Kind != ValConst {
		return Value{}, &OperandTypeError{
			Op:    op.String(),
			Left:  describeOperand(left),
			Right: describeOperand(right),
			Span:  span,
		}
	}

	res, err := constArithmetic(op, left.Const, right.Const)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: res}, nil
}

// definiteArithmeticError is why op over an open operand fails whatever that
// operand holds: the other is a zero divisor or the unbounded `*`.
func definiteArithmeticError(op ast.OperatorKind, left, right Value) error {
	for _, operand := range []Value{left, right} {
		if operand.Kind == ValConst && operand.Const.IsUnbounded() {
			return fmt.Errorf("%w: operator '%s' is not defined for the unbounded value '*'", ErrTypeMismatch, op)
		}
	}
	if op != ast.OpDiv && op != ast.OpMod {
		return nil
	}
	if q, ok := asQuantity(right); ok && q.Num.IsNumeric() && q.Num.AsReal() == 0 {
		return ErrDivisionByZero
	}
	return nil
}

// constArithmetic is arithmetic over two scalar constants, the core the
// evaluator and the compiled calc tier share so both report the same results
// and the same errors.
func constArithmetic(op ast.OperatorKind, left, right semantics.Value) (semantics.Value, error) {
	// The unbounded `*` is no number: arithmetic over it is refused rather than
	// answered with a finite result or an infinity.
	if left.IsUnbounded() || right.IsUnbounded() {
		return semantics.Value{}, fmt.Errorf("%w: operator '%s' is not defined for the unbounded value '*': %s %s %s",
			ErrTypeMismatch, op, semantics.FormatConst(left), op, semantics.FormatConst(right))
	}

	// Exponentiation shares the folder's implementation, so a folded and an
	// evaluated `**` agree; the folder declines where this reports the error.
	if op == ast.OpPow {
		return semantics.Pow(left, right)
	}

	// Integer arithmetic: an out-of-range result is reported, not wrapped.
	if left.Kind == semantics.ValInt && right.Kind == semantics.ValInt {
		// A quotient is a Rational: the exact ratio, rounded once to float64 so
		// operands beyond 2^53 are not rounded before dividing.
		if op == ast.OpDiv {
			q, ok := semantics.IntQuotient(left.Int, right.Int)
			if !ok {
				return semantics.Value{}, ErrDivisionByZero
			}
			return semantics.Value{Kind: semantics.ValReal, Real: q}, nil
		}
		var result int64
		switch op {
		case ast.OpAdd, ast.OpSub, ast.OpMul:
			var ok bool
			if result, ok = semantics.IntArith(op, left.Int, right.Int); !ok {
				return semantics.Value{}, semantics.IntegerOverflow(op, left.Int, right.Int)
			}
		case ast.OpMod:
			if right.Int == 0 {
				return semantics.Value{}, ErrDivisionByZero
			}
			result = left.Int % right.Int
		}
		return semantics.Value{Kind: semantics.ValInt, Int: result}, nil
	}

	// Real arithmetic (coerce int to real if needed)
	leftReal := toReal(left)
	rightReal := toReal(right)
	var result float64
	switch op {
	case ast.OpAdd:
		result = leftReal + rightReal
	case ast.OpSub:
		result = leftReal - rightReal
	case ast.OpMul:
		result = leftReal * rightReal
	case ast.OpDiv:
		// A real quotient by zero is reported, as an integer one, a quantity one
		// and the constant folder all report it, rather than carried as an infinity.
		if rightReal == 0 {
			return semantics.Value{}, ErrDivisionByZero
		}
		result = leftReal / rightReal
	case ast.OpMod:
		if rightReal == 0 {
			return semantics.Value{}, ErrDivisionByZero
		}
		result = math.Mod(leftReal, rightReal)
	}
	// A result that is not a finite Real is reported, not carried as an infinity.
	return semantics.RealResult(result)
}

// toReal converts a semantics.Value to float64.
func toReal(v semantics.Value) float64 {
	if v.Kind == semantics.ValInt {
		return float64(v.Int)
	}
	return v.Real
}

// evalEquality evaluates equality operators (==, !=).
func (ec *EvalContext) evalEquality(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("equality requires 2 operands, got %d", len(n.Operands))
	}

	left, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	return ec.ctx.equalityValues(n.Operator, soleElement(left), soleElement(right))
}

// equalityValues applies `==` or `!=` to two evaluated operands; the operator
// notation and the library's `'=='` forms both use it.
func (ctx *Context) equalityValues(op ast.OperatorKind, left, right Value) (Value, error) {
	if _, open := undeterminedIn(left, right); open {
		return undeterminedResult(left, right), nil
	}
	// Comparing a value with a variant compares it with the value that variant
	// declares; comparing two variants compares the choice itself.
	if (left.Kind == ValVariant) != (right.Kind == ValVariant) {
		var err error
		if left, err = ctx.variantAsValue(left); err != nil {
			return Value{}, err
		}
		if right, err = ctx.variantAsValue(right); err != nil {
			return Value{}, err
		}
	}

	// Quantities compare in a common unit; incommensurable ones are an error,
	// not an inequality.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return ctx.equalQuantities(op, lq, rq)
	}

	// Two Collection objects compare by their elements (CollectionFunctions::'==').
	if lc, ok, err := ctx.collectionObjectElements(left); err != nil {
		return Value{}, err
	} else if ok {
		if rc, ok, err := ctx.collectionObjectElements(right); err != nil {
			return Value{}, err
		} else if ok {
			left, right = lc, rc
		}
	}

	equal := ctx.equalValues(left, right)
	if op == ast.OpNeq {
		equal = !equal
	}
	return boolValue(equal), nil
}

// evalComparison evaluates comparison operators (<, <=, >, >=).
func (ec *EvalContext) evalComparison(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("comparison requires 2 operands, got %d", len(n.Operands))
	}

	left, err := ec.valueOperand(n.Operands[0])
	if err != nil {
		return Value{}, err
	}

	right, err := ec.valueOperand(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	return ec.ctx.comparisonValues(n.Operator, left, right, n.Span())
}

// comparisonValues applies an ordering operator to two evaluated operands; the
// operator notation and the library's `'<'` forms both use it.
func (ctx *Context) comparisonValues(op ast.OperatorKind, left, right Value, span source.Span) (Value, error) {
	if _, open := undeterminedIn(left, right); open {
		return undeterminedResult(left, right), nil
	}
	// Quantities are ordered on a common reference, so a magnitude is never
	// compared across units or scales without conversion.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return ctx.compareQuantities(op, lq, rq)
	}

	// StringFunctions declares the comparisons over two String operands, so a
	// string orders against a string and against nothing else.
	if left.Kind == ValString || right.Kind == ValString {
		if left.Kind != ValString || right.Kind != ValString {
			return Value{}, &OperandTypeError{
				Op:    op.String(),
				Left:  describeOperand(left),
				Right: describeOperand(right),
				Span:  span,
			}
		}
		ordered, err := compareStrings(op, left.Str(), right.Str())
		if err != nil {
			return Value{}, err
		}
		return boolValue(ordered), nil
	}

	// Numbers, Strings and quantities are ordered, so the operand blamed is the
	// other one, or the pairing when a quantity meets the unbounded value.
	refuse := func(library string) (Value, error) {
		return Value{}, &OperandTypeError{
			Op:      op.String(),
			Left:    describeOperand(left),
			Right:   describeOperand(right),
			Library: library,
			Span:    span,
		}
	}
	for _, val := range []Value{left, right} {
		if val.Kind != ValConst && val.Kind != ValQuantity {
			return refuse(ctx.orderingGap(op, val))
		}
	}
	if left.Kind == ValQuantity || right.Kind == ValQuantity {
		for _, val := range []Value{left, right} {
			if val.Kind == ValConst && val.Const.Kind == semantics.ValBool {
				return refuse(booleanOrderingGap(op))
			}
			if val.Kind == ValConst {
				return refuse(fmt.Sprintf("QuantityCalculations::'%s' takes ScalarQuantityValue operands and %s is none", op, describeOperand(val)))
			}
		}
	}

	result, err := constComparison(op, left.Const, right.Const)
	var mismatch *OperandTypeError
	if errors.As(err, &mismatch) {
		mismatch.Span = span
	}
	if err != nil {
		return Value{}, err
	}
	return boolValue(result), nil
}

// orderingGap says which Kernel Function Library function would have to declare
// the ordering operator op for val, and why none does: DataFunctions::'<' and
// ScalarFunctions::'<' are abstract, declared concretely by the numeric libraries
// and StringFunctions alone.
func (ctx *Context) orderingGap(op ast.OperatorKind, val Value) string {
	fn := func(pkg string) string { return fmt.Sprintf("%s::'%s'", pkg, op) }
	notScalar := func(what string) string {
		return fmt.Sprintf("%s is abstract and no library function declares '%s' for %s, which is no ScalarValue", fn("DataFunctions"), op, what)
	}
	switch val.Kind {
	case ValComplex:
		return fmt.Sprintf("%s is abstract and ComplexFunctions declares no '%s' for Complex", fn("NumericalFunctions"), op)
	case ValEnumLiteral:
		enum := semantics.EnumerationOwning(val.Literal())
		if enum == nil {
			return notScalar(val.LiteralText())
		}
		if ctx.conformsToLibrary(enum, "ScalarValues::ScalarValue") {
			return fmt.Sprintf("the library orders %s by value and %s declares none", enum.Name, val.LiteralText())
		}
		return notScalar("the enumeration " + enum.Name)
	case ValNull, ValSequence, ValSet:
		return fmt.Sprintf("%s takes one DataValue per operand", fn("DataFunctions"))
	case ValInstance, ValVariant:
		what := describeOperand(val)
		if types, err := ctx.directValueTypes(nil, val); err == nil && len(types) > 0 {
			what = fmt.Sprintf("an instance of the %s %s", ast.Notation(types[0].Decl), types[0].Name)
		}
		if ctx.isDataValue(val) {
			return notScalar(what)
		}
		return fmt.Sprintf("%s takes DataValue operands and %s is none", fn("DataFunctions"), what)
	case ValFunction, ValExpr:
		return fmt.Sprintf("%s takes DataValue operands and %s is none", fn("DataFunctions"), describeOperand(val))
	}
	return notScalar(describeOperand(val))
}

// booleanOrderingGap is orderingGap for a Boolean, which the compiled tier
// reports without a Context.
func booleanOrderingGap(op ast.OperatorKind) string {
	return fmt.Sprintf("ScalarFunctions::'%s' is abstract and BooleanFunctions declares no '%s' for Boolean", op, op)
}

// constComparison orders two scalar constants, the core the evaluator and the
// compiled calc tier share.
func constComparison(op ast.OperatorKind, left, right semantics.Value) (bool, error) {
	// A Boolean is a ScalarValue no library orders, so it is refused rather
	// than read as a number.
	if left.Kind == semantics.ValBool || right.Kind == semantics.ValBool {
		return false, &OperandTypeError{
			Op:      op.String(),
			Left:    describeValue(Value{Kind: ValConst, Const: left}),
			Right:   describeValue(Value{Kind: ValConst, Const: right}),
			Library: booleanOrderingGap(op),
		}
	}
	// The unbounded `*` orders above every finite number and equals itself.
	if left.IsUnbounded() || right.IsUnbounded() {
		order, ok := semantics.UnboundedOrder(left, right)
		if !ok {
			return false, fmt.Errorf("%w: '%s' is not defined between %s and %s",
				ErrTypeMismatch, op, semantics.FormatConst(left), semantics.FormatConst(right))
		}
		res, ok := semantics.OrderSatisfies(op, order)
		if !ok {
			return false, fmt.Errorf("unknown comparison operator: %v", op)
		}
		return res, nil
	}

	// Compare integers
	if left.Kind == semantics.ValInt && right.Kind == semantics.ValInt {
		switch op {
		case ast.OpLt:
			return left.Int < right.Int, nil
		case ast.OpLe:
			return left.Int <= right.Int, nil
		case ast.OpGt:
			return left.Int > right.Int, nil
		case ast.OpGe:
			return left.Int >= right.Int, nil
		default:
			return false, fmt.Errorf("unknown comparison operator: %v", op)
		}
	}

	// Compare reals (coerce int to real)
	leftReal := toReal(left)
	rightReal := toReal(right)
	switch op {
	case ast.OpLt:
		return leftReal < rightReal, nil
	case ast.OpLe:
		return leftReal <= rightReal, nil
	case ast.OpGt:
		return leftReal > rightReal, nil
	case ast.OpGe:
		return leftReal >= rightReal, nil
	default:
		return false, fmt.Errorf("unknown comparison operator: %v", op)
	}
}

// evalLogical evaluates the Boolean binary operators. `and`, `or` and `implies`
// decide on their left operand alone where they can, so the operand a guard
// rules out is never evaluated.
func (ec *EvalContext) evalLogical(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 2 {
		return Value{}, fmt.Errorf("logical operator requires 2 operands, got %d", len(n.Operands))
	}

	left, err := ec.valueOperand(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	// An open left operand decides nothing, so the right one is read: where it
	// fixes the result on its own, the result is known.
	if left.Kind == ValUndetermined {
		right, err := ec.valueOperand(n.Operands[1])
		if err != nil {
			return Value{}, err
		}
		return logicalWithOpenLeft(n.Operator, left, right)
	}
	l, err := boolOperand(fmt.Sprintf("left operand of '%s'", n.Operator), left)
	if err != nil {
		return Value{}, err
	}

	if decided, result := shortCircuit(n.Operator, l); decided {
		return boolValue(result), nil
	}

	right, err := ec.Eval(n.Operands[1])
	if err != nil {
		return Value{}, err
	}
	if right.Kind == ValUndetermined {
		return undeterminedResult(right), nil
	}
	r, err := boolOperand(fmt.Sprintf("right operand of '%s'", n.Operator), right)
	if err != nil {
		return Value{}, err
	}
	return combineBooleans(n.Operator, l, r)
}

// logicalWithOpenLeft is a Boolean operator over an open left operand: a right one
// fixing the result alone (`and false`, `or true`, `implies true`) fixes it here too.
func logicalWithOpenLeft(op ast.OperatorKind, left, right Value) (Value, error) {
	if right.Kind == ValUndetermined {
		return undeterminedResult(left, right), nil
	}
	r, err := boolOperand(fmt.Sprintf("right operand of '%s'", op), right)
	if err != nil {
		return Value{}, err
	}
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		if !r {
			return boolValue(false), nil
		}
	case ast.OpOr, ast.OpConditionalOr, ast.OpImplies:
		if r {
			return boolValue(true), nil
		}
	}
	return undeterminedResult(left), nil
}

// combineBooleanValues applies a binary Boolean operator to two evaluated
// operands, either of which the model may leave open.
func combineBooleanValues(op ast.OperatorKind, left, right Value) (Value, error) {
	if left.Kind == ValUndetermined {
		return logicalWithOpenLeft(op, left, right)
	}
	l, err := boolOperand(fmt.Sprintf("left operand of '%s'", op), left)
	if err != nil {
		return Value{}, err
	}
	if decided, result := shortCircuit(op, l); decided {
		return boolValue(result), nil
	}
	if right.Kind == ValUndetermined {
		return undeterminedResult(right), nil
	}
	r, err := boolOperand(fmt.Sprintf("right operand of '%s'", op), right)
	if err != nil {
		return Value{}, err
	}
	return combineBooleans(op, l, r)
}

// shortCircuit reports whether a Boolean operator is decided by its left
// operand alone, and the result when it is: `and` by false, `or` by true and
// `implies` by false. `xor`, `|` and `&` always read both operands.
func shortCircuit(op ast.OperatorKind, l bool) (decided, result bool) {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return !l, false
	case ast.OpOr, ast.OpConditionalOr:
		return l, true
	case ast.OpImplies:
		return !l, true
	}
	return false, false
}

// combineBooleans applies a binary Boolean operator to two Booleans; the
// operator notation and the library's `'xor'` forms both use it.
func combineBooleans(op ast.OperatorKind, l, r bool) (Value, error) {
	switch op {
	case ast.OpAnd, ast.OpConditionalAnd:
		return boolValue(l && r), nil
	case ast.OpOr, ast.OpConditionalOr:
		return boolValue(l || r), nil
	case ast.OpXor:
		return boolValue(l != r), nil
	case ast.OpImplies:
		return boolValue(!l || r), nil
	}
	return Value{}, fmt.Errorf("%w: '%s' is not a Boolean operator", ErrUnsupportedOperator, op)
}

// valueOperand evaluates an operand an operator needs one value of: a feature
// holding none is reported as such, not as an operand of the wrong type, and a
// one-element collection is the value it holds.
func (ec *EvalContext) valueOperand(node ast.Node) (Value, error) {
	val, err := ec.Eval(node)
	if err != nil {
		return Value{}, err
	}
	if ec.ctx.HoldsNoValue(val) {
		return Value{}, ec.ctx.noValueError(val, node)
	}
	return soleElement(val), nil
}

// boolOperand reads a Boolean out of a value, naming what was expected when the
// value is not one.
func boolOperand(what string, v Value) (bool, error) {
	v = soleElement(v)
	if v.Kind != ValConst || v.Const.Kind != semantics.ValBool {
		return false, fmt.Errorf("%w: %s must be Boolean, got %s", ErrTypeMismatch, what, v.Kind)
	}
	return v.Const.Bool, nil
}

// evalUnary evaluates the unary operators (-, +, not).
func (ec *EvalContext) evalUnary(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 1 {
		return Value{}, fmt.Errorf("unary operator requires 1 operand, got %d", len(n.Operands))
	}

	// The least Integer is the one literal whose magnitude alone is outside the
	// range, so its sign is read together with it; every other operand is
	// evaluated as usual.
	if n.Operator == ast.OpNeg {
		if lit, ok := n.Operands[0].(*ast.LiteralInteger); ok {
			if _, err := strconv.ParseInt(lit.Value, 10, 64); err != nil {
				if val, err := strconv.ParseInt("-"+lit.Value, 10, 64); err == nil {
					return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: val}}, nil
				}
			}
		}
	}

	operand, err := ec.valueOperand(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	if isVectorKind(operand) {
		switch n.Operator {
		case ast.OpPos:
			return operand, nil
		case ast.OpNeg:
			return vectorSubtract("VectorFunctions::'-'", ec.ctx, []Value{operand, nullValue()})
		}
	}
	return ec.ctx.unaryValue(n.Operator, operand)
}

// unaryValue applies `not`, `-` or `+` to an evaluated operand; the operator
// notation and the library's `'not'` forms both use it.
func (ctx *Context) unaryValue(op ast.OperatorKind, operand Value) (Value, error) {
	if operand.Kind == ValUndetermined {
		return undeterminedResult(operand), nil
	}
	switch op {
	case ast.OpNot:
		if operand.Kind != ValConst {
			return Value{}, fmt.Errorf("%w: logical not requires bool operand, got %v", ErrTypeMismatch, operand.Kind)
		}
	case ast.OpNeg, ast.OpPos:
		if operand.Kind == ValQuantity {
			if op == ast.OpPos {
				return operand, nil
			}
			return ctx.negateQuantity(operand.Quantity())
		}
		if operand.Kind == ValComplex {
			if op == ast.OpPos {
				return operand, nil
			}
			return NewComplex(-operand.Complex()), nil
		}
		// Arithmetic sign: -number, +number
		if operand.Kind != ValConst {
			return Value{}, fmt.Errorf("%w: unary '%s' requires numeric operand, got %v", ErrTypeMismatch, op, operand.Kind)
		}
	default:
		return Value{}, fmt.Errorf("%w: '%s' is not a unary operator", ErrUnsupportedOperator, op)
	}
	result, err := constUnary(op, operand.Const)
	if err != nil {
		return Value{}, err
	}
	return Value{Kind: ValConst, Const: result}, nil
}

// constUnary applies `not`, `-` or `+` to a scalar constant, the core the
// evaluator and the compiled calc tier share.
func constUnary(op ast.OperatorKind, operand semantics.Value) (semantics.Value, error) {
	if op == ast.OpNot {
		// Logical not: not bool
		if operand.Kind != semantics.ValBool {
			return semantics.Value{}, fmt.Errorf("%w: logical not requires bool operand, got %s", ErrTypeMismatch, semantics.FormatConst(operand))
		}
		return semantics.Value{Kind: semantics.ValBool, Bool: !operand.Bool}, nil
	}
	if op == ast.OpNeg && operand.Kind == semantics.ValInt && operand.Int == math.MinInt64 {
		return semantics.Value{}, fmt.Errorf("%w: -(%d) exceeds the Integer range",
			semantics.ErrArithmeticOverflow, operand.Int)
	}
	result, ok := semantics.EvalUnary(op, operand)
	if !ok {
		return semantics.Value{}, fmt.Errorf("%w: unary '%s' is not defined for %s", ErrTypeMismatch, op, semantics.FormatConst(operand))
	}
	return result, nil
}

// evalSequenceExpr evaluates a sequence expression, `(1, 2, 3)`. A KerML
// sequence is flat: an element that is itself a collection contributes its
// elements, which is what makes SequenceFunctions::union the sequence
// expression `(seq1, seq2)` rather than a two-element sequence of sequences.
func (ec *EvalContext) evalSequenceExpr(n *ast.SequenceExpr) (Value, error) {
	elements := make([]Value, 0, len(n.Elements))
	var open bool
	for _, elem := range n.Elements {
		val, err := ec.Eval(elem)
		if err != nil {
			return Value{}, err
		}
		if val.Kind == ValUndetermined {
			open = true
			elements = append(elements, val)
			continue
		}
		elements = append(elements, elementsOf(val)...)
	}
	if open {
		return undeterminedElements(elements), nil
	}
	return ec.newSequence(elements)
}

// evalCollectExpr evaluates `operand.{in x; ...}`, the collect notation, which
// KerML defines as ControlFunctions::collect of the operand and the body. It
// evaluates through that one implementation, so the notation and the call
// `collect(seq, {in x; ...})` compute the same result.
func (ec *EvalContext) evalCollectExpr(n *ast.CollectExpr) (Value, error) {
	return ec.evalCollectionNotation("collect", n.Operand, n.Body, builtinControlCollect)
}

// evalSelectExpr evaluates `operand.?{in x; ...}`, the select notation, which
// KerML defines as ControlFunctions::select of the operand and the body.
func (ec *EvalContext) evalSelectExpr(n *ast.SelectExpr) (Value, error) {
	return ec.evalCollectionNotation("select", n.Operand, n.Body, builtinControlSelect)
}

// evalCollectionNotation evaluates a notation whose meaning is a library
// function of an operand and a body: the body is evaluated to the function it
// denotes rather than to a value, and the operation decides what to call it
// with.
func (ec *EvalContext) evalCollectionNotation(
	notation string,
	operandExpr, bodyExpr ast.Node,
	fn builtinFunc,
) (Value, error) {
	operand, err := ec.Eval(operandExpr)
	if err != nil {
		return Value{}, err
	}
	if bodyExpr == nil {
		return Value{}, fmt.Errorf("%w: %s states no body", ErrNoResultExpression, notation)
	}
	body, err := ec.Eval(bodyExpr)
	if err != nil {
		return Value{}, err
	}
	return fn(ec, []Value{operand, body})
}

// invocationKey identifies one invocation expression in the scope it is
// evaluated in, which is what its written name resolves against.
type invocationKey struct {
	node  *ast.InvocationExpr
	scope *symbols.Scope
	// running is the behavior whose run evaluates the expression: it applies a
	// callee it redefines through the redefining feature.
	running *symbols.Symbol
}

// invocationTarget is what an invocation expression denotes, resolved once per
// context; at most one implementation is set, in the order they are tried.
type invocationTarget struct {
	qualName     string
	ambiguous    []*symbols.Symbol // the equally specific declarations the written name denotes
	undetermined []*symbols.Symbol // the declarations the static argument types leave open, settled by the values
	calc         *symbols.Symbol   // the declaration the written name resolves to, nil for none
	builtin      builtinFunc       // the built-in the name denotes: the library declaration calc is
	builtinName  string            // the built-in's registered name, keying its declared signature
	library      *libraryFunction  // the library function the name denotes: the library declaration calc is
	shape        *calcShape        // calc's invocation interface, nil when it has none
	predicate    bool              // calc is a constraint or requirement, applied as a predicate
	names        []string          // the parameter each named argument binds, as calc's signature spells it
	unbound      []error           // per named argument, why calc has no parameter for it; nil when it binds
	candidates   []*symbols.Symbol // the declarations the written name may denote, which the selection chose among
}

// invocationTarget resolves what n denotes in this context's scope, memoized
// per context: resolution reads only the model, which is fixed for its life.
// The declaration is the one the checker selects for the call, so the two agree: a
// library function is callable only where the model imports it or writes it qualified,
// and a declaration of the model's own is invoked as written even under a name a
// library built-in is registered by. What the selection read of the model is recorded for the
// binding being made on every use, a memoized target's included.
func (ec *EvalContext) invocationTarget(n *ast.InvocationExpr) *invocationTarget {
	key := invocationKey{node: n, scope: ec.scope, running: ec.runningBehavior()}
	target, ok := ec.ctx.model.invocationTargets[key]
	if !ok {
		target = ec.selectInvocationTarget(key, n)
		ec.ctx.model.invocationTargets[key] = target
	}
	ec.ctx.noteInvocationRead(ec.scope, n.Type, target.candidates)
	for b := key.running; b != nil; b = enclosingBehavior(b) {
		ec.ctx.noteTypeRead(b)
	}
	ec.ctx.noteDeclarationRead(target.calc)
	return target
}

// selectInvocationTarget resolves n as the checker selects it, from the candidates its name denotes.
func (ec *EvalContext) selectInvocationTarget(key invocationKey, n *ast.InvocationExpr) *invocationTarget {
	model := ec.ctx.model
	target := &invocationTarget{
		qualName:   qualifiedNameToString(n.Type),
		candidates: model.resolver.InvocationCandidates(ec.scope, n.Type),
	}
	sel := passes.SelectInvocation(model.resolver, model.semantics, ec.scope, n, semantics.PerformsBehavior)
	switch {
	case sel.Ambiguous && sel.Undetermined:
		target.undetermined = sel.Tied
	case sel.Ambiguous:
		target.ambiguous = sel.Tied
	case sel.Called() != nil:
		sym := sel.Called()
		ec.ctx.implementInvocation(target, ec.ctx.inheritedFeature(key.running, sym))
	}
	if len(n.NamedArgs) > 0 {
		target.names, target.unbound = ec.ctx.boundParameterNames(ec.scope, target.calc, n.NamedArgs)
	}
	return target
}

// runningBehavior is the behavior whose run the expression is evaluated in: the
// owner of the innermost frame a run pushed, nil outside any run.
func (ec *EvalContext) runningBehavior() *symbols.Symbol {
	for i := len(ec.frames) - 1; i >= 0; i-- {
		if owner := ec.frames[i].owner; owner != nil {
			return owner.Sym
		}
	}
	return nil
}

// inheritedFeature is sym as the running behavior, or one enclosing it, inherits
// it: a feature redefined there answers to the redefinition (KerML §7.3.4.5).
func (ctx *Context) inheritedFeature(running, sym *symbols.Symbol) *symbols.Symbol {
	for b := running; b != nil && sym != nil; b = enclosingBehavior(b) {
		if !ctx.model.semantics.InheritanceMasked(b, sym) {
			continue
		}
		if redefiner := ctx.model.semantics.NamingRedefiner(b, sym); redefiner != nil {
			return redefiner
		}
	}
	return sym
}

// boundParameterNames is the parameter each named argument binds in callee, spelled as
// callee's signature spells it, and per argument the error when callee has no such
// parameter; a label kept as written. Without a callee every label is kept as written.
func (ctx *Context) boundParameterNames(scope *symbols.Scope, callee *symbols.Symbol, named []ast.NamedArg) ([]string, []error) {
	names := make([]string, len(named))
	unbound := make([]error, len(named))
	for i, arg := range named {
		if arg.Name == nil || len(arg.Name.Parts) == 0 {
			continue
		}
		names[i] = semantics.QualifiedNameText(arg.Name)
		if callee == nil || ctx.model.semantics == nil {
			continue
		}
		if name, ok := ctx.model.semantics.BoundParameter(scope, callee, arg.Name); ok {
			names[i] = name
		} else {
			unbound[i] = fmt.Errorf("%w: %s has no parameter named %q",
				ErrUnknownParameter, ctx.qualifiedSymbolName(callee), names[i])
		}
	}
	return names, unbound
}

// implementInvocation records how a call of the selected sym is applied: by a
// library implementation, by its calc shape, or — when a model calc binds the
// arguments through parameters of its own — by invokeCalcWithSelf.
func (ctx *Context) implementInvocation(target *invocationTarget, sym *symbols.Symbol) {
	target.calc = sym
	if perf := ctx.libraryCalcPerformed(sym); perf != nil {
		if perf.signature != nil {
			return
		}
		sym = perf.lib
	}
	if fn, ok := ctx.builtinFor(sym); ok {
		target.builtin, target.builtinName = fn, ctx.qualifiedSymbolName(sym)
	} else if fn, ok := ctx.libraryFunctionFor(sym); ok {
		target.library = fn
	} else if shape, err := ctx.calcShapeOf(sym); err == nil {
		target.shape = shape
	} else if isPredicateDecl(sym.Decl) {
		target.predicate = true
	}
}

// ambiguousInvocationError names the equally specific declarations a call of
// qualName denotes.
func ambiguousInvocationError(qualName string, candidates []*symbols.Symbol) error {
	names := make([]string, len(candidates))
	for i, sym := range candidates {
		names[i] = symbols.FQNOf(sym)
	}
	return fmt.Errorf("%w: %s denotes %s", ErrAmbiguousInvocation, qualName, strings.Join(names, ", "))
}

// unresolvedInvocation reports a call to a name that denotes nothing, with the
// same "did you mean" hint the validator gives a reference: for a qualified
// name, the quoted member the segment past the deepest resolved one may start.
func (ec *EvalContext) unresolvedInvocation(qn *ast.QualifiedName, written string) error {
	if qn == nil || ec.ctx.model.resolver == nil {
		return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
	}
	if len(qn.Parts) == 1 && !qn.Global {
		return fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedName(ec.scope, written, qn))
	}
	reading := ec.ctx.readQualified(ec.scope, qn)
	for i := len(qn.Parts) - 2; i >= 0; i-- {
		if owner, ok := reading.Part(i); ok {
			return fmt.Errorf("%w: %s", ErrUnresolvedReference, ec.ctx.model.resolver.UnresolvedMember(ec.scope, qn, owner, i+1))
		}
	}
	return fmt.Errorf("%w: %s", ErrUnresolvedReference, written)
}

// evalInvocation evaluates a function/calc invocation.
func (ec *EvalContext) evalInvocation(n *ast.InvocationExpr) (Value, error) {
	// `holder.f(a)`: the chain names the function applied, not the callee's type.
	if chain := passes.ChainCallee(n); chain != nil {
		return ec.evalChainInvocation(n, chain)
	}
	target := ec.invocationTarget(n)
	qualName := target.qualName
	if len(target.ambiguous) > 0 {
		return Value{}, ambiguousInvocationError(qualName, target.ambiguous)
	}
	if len(target.undetermined) > 0 {
		return ec.evalUndeterminedInvocation(n, target)
	}

	// A receiver binds by position, so it has no meaning beside arguments that
	// bind by name: reported rather than evaluated and dropped.
	if n.Operand != nil && len(n.NamedArgs) > 0 {
		return Value{}, fmt.Errorf(
			"%w: %s is called with a receiver and named arguments",
			ErrReceiverWithNamedArgs, qualName,
		)
	}

	// Eval args in source order. An operand is the first argument of the
	// invocation it is written before: `seq->size()` invokes size with seq, which
	// is how the semantics layer reads the same expression, so the two agree on
	// which parameter an argument binds.
	exprs := passes.InvocationArgs(n)
	// A calc-typed feature bound to a function value here — a parameter given a
	// calc as its argument — applies that value, not the feature's own declaration.
	if fn, ok, err := ec.boundFunction(target.calc, n.Type); ok {
		if err != nil {
			return Value{}, err
		}
		// Named arguments bind parameters of the calc applied, not of the feature named.
		applied := *target
		if len(n.NamedArgs) > 0 {
			applied.names, applied.unbound = ec.ctx.boundParameterNames(ec.scope, fn.Function(), n.NamedArgs)
		}
		callArgs, err := ec.evalInvocationArgs(qualName, exprs, n.NamedArgs, &applied)
		if err != nil {
			return Value{}, err
		}
		return ec.invokeFunction(qualName, fn, callArgs)
	}
	// A calc bound by position alone consumes its arguments within the call, so
	// they live on the context's argument stack rather than in a slice of their own.
	if target.shape != nil && len(n.NamedArgs) == 0 {
		return ec.invokeCalcShapeStacked(target.shape, exprs, ec.enclosingRun(target.shape))
	}
	// A built-in binds its arguments by its declared signature.
	if target.builtin != nil {
		return ec.invokeBuiltin(target.builtinName, target.builtin, exprs, n.NamedArgs, target.names, target.unbound)
	}

	callArgs, err := ec.evalInvocationArgs(qualName, exprs, n.NamedArgs, target)
	if err != nil {
		return Value{}, err
	}

	// An argument that fails is reported before the target is judged. A name
	// that resolves to nothing denotes nothing, not the library function of
	// that name: the validator reports the same expression unresolved.
	if target.calc == nil && target.library == nil {
		return Value{}, ec.unresolvedInvocation(n.Type, qualName)
	}
	return ec.applyInvocation(target, callArgs)
}

// applyInvocation applies target to arguments already evaluated, through the one calc
// path, so an expression and a direct InvokeCalc bind parameters and trace identically.
func (ec *EvalContext) applyInvocation(target *invocationTarget, callArgs calcArgs) (Value, error) {
	if target.library != nil {
		return target.library.invoke(ec.ctx, callArgs)
	}
	if target.predicate {
		return ec.invokePredicate(target.calc, callArgs)
	}
	if target.shape == nil {
		return ec.ctx.invokeCalcWithSelf(target.calc, callArgs, ec.scope, ec.self)
	}
	return ec.ctx.invokeCalcShapeIn(target.shape, callArgs, ec.scope, ec.self, ec.enclosingRun(target.shape))
}

// enclosingRun is the environment a nested calc closes over here: the frames through
// the innermost run of the behavior it is declared in, none when no such run is active.
func (ec *EvalContext) enclosingRun(shape *calcShape) []frame {
	return runOf(ec.ctx, ec.frames, enclosingBehavior(shape.Sym))
}

// evalChainInvocation applies the function value a feature chain denotes to the
// arguments written after it (KerMLExpressions InstantiatedTypeMember → OwnedFeatureChain).
func (ec *EvalContext) evalChainInvocation(n *ast.InvocationExpr, chain *ast.FeatureChainExpr) (Value, error) {
	callee := chainText(chain)
	fn, err := ec.chainCallee(chain)
	if err != nil {
		return Value{}, err
	}
	if fn.Kind != ValFunction {
		return Value{}, fmt.Errorf("%w: %s is %s, not a function", ErrNotAFunction, callee, describeValue(fn))
	}
	target := &invocationTarget{qualName: callee, calc: fn.Function()}
	if len(n.NamedArgs) > 0 {
		target.names, target.unbound = ec.ctx.boundParameterNames(ec.scope, fn.Function(), n.NamedArgs)
	}
	callArgs, err := ec.evalInvocationArgs(callee, n.Args, n.NamedArgs, target)
	if err != nil {
		return Value{}, err
	}
	return ec.invokeFunction(callee, fn, callArgs)
}

// chainCallee is what a feature chain denotes in call position: a calc of the receiver's
// object is the function applied over it even where a bare read would compute its result.
func (ec *EvalContext) chainCallee(chain *ast.FeatureChainExpr) (Value, error) {
	if chain.Member == nil || len(chain.Member.Parts) != 1 {
		return ec.Eval(chain)
	}
	receiver, err := ec.Eval(chain.Operand)
	if err != nil {
		return Value{}, err
	}
	name := chain.Member.Parts[0].Text
	if id, isObject := receiver.Object(); isObject {
		if inst, ok := ec.ctx.instances[id]; ok {
			if _, held := inst.FeatureValues[name]; !held {
				if sym, found := ec.ctx.model.semantics.LookupMember(inst.Type, name); found && isCalcUsageSymbol(sym) {
					return NewEvalContextIn(ec.ctx, sym.OwnerScope, inst).functionValueOf(sym)
				}
			}
		}
	}
	return ec.chainMemberValue(receiver, chain.Member.Parts, "")
}

// chainText spells a feature chain as written, `holder.scale`.
func chainText(n ast.Node) string {
	switch c := n.(type) {
	case *ast.FeatureChainExpr:
		return chainText(c.Operand) + "." + qualifiedNameToString(c.Member)
	case *ast.FeatureReference:
		return qualifiedNameToString(c.Name)
	}
	return TraceLabel(n)
}

// evalUndeterminedInvocation evaluates a call left open among target.undetermined: the
// values' types select; a tie they leave is reported, values fitting none run the first.
func (ec *EvalContext) evalUndeterminedInvocation(n *ast.InvocationExpr, target *invocationTarget) (Value, error) {
	qualName := target.qualName
	if n.Operand != nil && len(n.NamedArgs) > 0 {
		return Value{}, fmt.Errorf("%w: %s is called with a receiver and named arguments", ErrReceiverWithNamedArgs, qualName)
	}
	exprs := passes.InvocationArgs(n)
	// An argument some candidate takes as an `expr` stays unevaluated, unknown to the
	// selection, so a short-circuiting built-in never sees a branch it would not have run.
	written := writtenArguments(exprs, n.NamedArgs)
	deferred := ec.deferredArguments(target.undetermined, exprs, n.NamedArgs)
	args := make([]semantics.Argument, 0, len(written))
	for k, w := range written {
		var name *ast.QualifiedName
		if k >= len(exprs) {
			name = n.NamedArgs[k-len(exprs)].Name
			if name == nil || len(name.Parts) == 0 {
				continue
			}
		}
		if deferred[k] {
			args = append(args, semantics.Argument{Name: name})
			continue
		}
		val, err := w.eval(ec)
		if err != nil {
			return Value{}, err
		}
		args = append(args, ec.valueArgument(val, name))
	}
	sel := ec.ctx.model.semantics.SelectAmongArguments(ec.scope, target.undetermined, args, semantics.PerformsBehavior)
	if sel.Ambiguous {
		return Value{}, ambiguousInvocationError(qualName, sel.Tied)
	}
	settled := &invocationTarget{qualName: qualName}
	ec.ctx.implementInvocation(settled, ec.ctx.inheritedFeature(ec.runningBehavior(), sel.Called()))
	callee := settled.calc
	fn, applied, err := ec.boundFunction(settled.calc, n.Type)
	if err != nil {
		return Value{}, err
	}
	if applied {
		callee = fn.Function()
	}
	if len(n.NamedArgs) > 0 {
		settled.names, settled.unbound = ec.ctx.boundParameterNames(ec.scope, callee, n.NamedArgs)
	}
	if settled.builtin != nil && !applied {
		return ec.invokeBuiltinWith(settled.builtinName, settled.builtin, exprs, n.NamedArgs, settled.names, settled.unbound,
			func(params []declaredParam, param, at int) (Value, error) {
				w := written[at]
				if _, body := w.expr.(*ast.BodyExpr); !body && param < len(params) && params[param].deferred {
					return NewExprValue(w.expr, ec.closure()), nil
				}
				return w.eval(ec)
			})
	}
	values := make([]Value, len(written))
	for k, w := range written {
		val, err := w.eval(ec)
		if err != nil {
			return Value{}, err
		}
		values[k] = val
	}
	callArgs, err := bindEvaluatedArgs(qualName, values[:len(exprs)], values[len(exprs):], settled)
	if err != nil {
		return Value{}, err
	}
	if applied {
		return ec.invokeFunction(qualName, fn, callArgs)
	}
	return ec.applyInvocation(settled, callArgs)
}

// writtenArgument is one argument of a call as written, and its value once evaluated.
type writtenArgument struct {
	expr      ast.Node
	value     Value
	evaluated bool
}

// writtenArguments lists a call's arguments in source order, the positional ones first.
func writtenArguments(exprs []ast.Node, named []ast.NamedArg) []*writtenArgument {
	written := make([]*writtenArgument, 0, len(exprs)+len(named))
	for _, expr := range exprs {
		written = append(written, &writtenArgument{expr: expr})
	}
	for _, arg := range named {
		written = append(written, &writtenArgument{expr: arg.Value})
	}
	return written
}

// eval evaluates the argument once in ec; a later ask reads the value.
func (w *writtenArgument) eval(ec *EvalContext) (Value, error) {
	if w.evaluated {
		return w.value, nil
	}
	val, err := ec.Eval(w.expr)
	if err != nil {
		return Value{}, err
	}
	w.value, w.evaluated = val, true
	return val, nil
}

// deferredArguments marks, per argument written, whether any of the candidates binds it to
// an `expr` parameter of a built-in, which takes the expression rather than its value.
func (ec *EvalContext) deferredArguments(candidates []*symbols.Symbol, exprs []ast.Node, named []ast.NamedArg) []bool {
	deferred := make([]bool, len(exprs)+len(named))
	for _, c := range candidates {
		var candidate invocationTarget
		ec.ctx.implementInvocation(&candidate, ec.ctx.inheritedFeature(ec.runningBehavior(), c))
		if candidate.builtin == nil {
			continue
		}
		params := builtinSignatures[candidate.builtinName]
		for i := range exprs {
			if i < len(params) && params[i].deferred {
				deferred[i] = true
			}
		}
		if len(named) == 0 {
			continue
		}
		names, _ := ec.ctx.boundParameterNames(ec.scope, candidate.calc, named)
		for j, name := range names {
			for _, p := range params {
				if p.name == name && p.deferred {
					deferred[len(exprs)+j] = true
				}
			}
		}
	}
	return deferred
}

// valueArgument types an evaluated argument for overload selection: a scalar as the literal
// spelling it, an object by every type classifying it, a collection by what its elements share.
func (ec *EvalContext) valueArgument(val Value, name *ast.QualifiedName) semantics.Argument {
	arg := semantics.Argument{Name: name}
	elements := elementsOf(val)
	if len(elements) == 0 {
		arg.Empty = true
		return arg
	}
	if prim := spelledPrim(elements); prim != semantics.PrimUnknown {
		arg.Prim, arg.Exact = prim, true
		return arg
	}
	var common []*symbols.Symbol
	for i, el := range elements {
		types, err := ec.ctx.valueTypes(ec.scope, el)
		if err != nil || len(types) == 0 {
			return arg
		}
		if i == 0 {
			common = types
			continue
		}
		if common = ec.sharedTypes(common, types); len(common) == 0 {
			return arg
		}
	}
	arg.Type, arg.Also = common[0], common[1:]
	arg.Prim = ec.ctx.model.semantics.PrimTypeOf(common[0])
	return arg
}

// spelledPrim is the type the checker gives a literal spelling each element (a nonnegative
// integer a Natural), widened over them; PrimUnknown for a non-scalar element.
func spelledPrim(elements []Value) semantics.PrimType {
	common := semantics.PrimUnknown
	for i, el := range elements {
		prim := representationPrim(el)
		if prim == semantics.PrimInteger && el.Const.Int >= 0 {
			prim = semantics.PrimNatural
		}
		switch {
		case prim == semantics.PrimUnknown:
			return semantics.PrimUnknown
		case i == 0, semantics.PrimConforms(common, prim):
			common = prim
		case !semantics.PrimConforms(prim, common):
			return semantics.PrimUnknown
		}
	}
	return common
}

// sharedTypes narrows the types the elements so far share to those an element of types
// also is: a type it conforms to stays, one it generalizes widens to the element's, else drops.
func (ec *EvalContext) sharedTypes(common, types []*symbols.Symbol) []*symbols.Symbol {
	var shared []*symbols.Symbol
	for _, c := range common {
		kept := c
		found := false
		for _, t := range types {
			switch {
			case ec.ctx.modelConforms(t, c):
				kept, found = c, true
			case ec.ctx.modelConforms(c, t):
				kept, found = t, true
			}
			if found {
				break
			}
		}
		if found && !slices.Contains(shared, kept) {
			shared = append(shared, kept)
		}
	}
	return shared
}

// evalInvocationArgs evaluates an invocation's arguments in source order into the
// calc arguments they bind: positional, or named against target's parameter names.
// The notation keeps the two forms mutually exclusive.
func (ec *EvalContext) evalInvocationArgs(qualName string, exprs []ast.Node, namedArgs []ast.NamedArg, target *invocationTarget) (calcArgs, error) {
	args := make([]Value, len(exprs))
	for i, arg := range exprs {
		val, err := ec.Eval(arg)
		if err != nil {
			return calcArgs{}, err
		}
		args[i] = val
	}
	if len(namedArgs) == 0 {
		return calcArgs{positional: args}, nil
	}
	named := make(map[string]Value, len(namedArgs))
	for i, arg := range namedArgs {
		name, err := target.namedBinding(qualName, i, named)
		if err != nil {
			return calcArgs{}, err
		}
		val, err := ec.Eval(arg.Value)
		if err != nil {
			return calcArgs{}, err
		}
		named[name] = val
	}
	return calcArgs{named: named}, nil
}

// bindEvaluatedArgs binds arguments already evaluated in source order, as
// evalInvocationArgs binds the ones it evaluates.
func bindEvaluatedArgs(qualName string, positional, namedValues []Value, target *invocationTarget) (calcArgs, error) {
	if len(namedValues) == 0 {
		return calcArgs{positional: positional}, nil
	}
	named := make(map[string]Value, len(namedValues))
	for i, val := range namedValues {
		name, err := target.namedBinding(qualName, i, named)
		if err != nil {
			return calcArgs{}, err
		}
		named[name] = val
	}
	return calcArgs{named: named}, nil
}

// namedBinding is the parameter the i-th named argument binds, or why it binds none:
// unnamed, no such parameter of the target, or a parameter already bound in named.
func (target *invocationTarget) namedBinding(qualName string, i int, named map[string]Value) (string, error) {
	name := target.names[i]
	if name == "" {
		return "", fmt.Errorf("unnamed argument in invocation of %s", qualName)
	}
	if err := target.unbound[i]; err != nil {
		return "", err
	}
	if _, dup := named[name]; dup {
		return "", fmt.Errorf("%w: %s binds parameter %q twice", ErrCalcArity, qualName, name)
	}
	return name, nil
}

// invokeCalcShapeStacked evaluates exprs onto the context's argument stack and
// invokes shape with them, popping them however the invocation ends.
func (ec *EvalContext) invokeCalcShapeStacked(shape *calcShape, exprs []ast.Node, enclosing []frame) (Value, error) {
	ctx := ec.ctx
	base := len(ctx.argStack)
	for _, arg := range exprs {
		val, err := ec.Eval(arg)
		if err != nil {
			ctx.popArgs(base)
			return Value{}, err
		}
		ctx.argStack = append(ctx.argStack, val)
	}
	top := len(ctx.argStack)
	args := ctx.argStack[base:top:top]
	result, err := ctx.invokeCalcShapeIn(shape, calcArgs{positional: args}, ec.scope, ec.self, enclosing)
	ctx.popArgs(base)
	return result, err
}

// popArgs releases the arguments pushed since the stack was base deep.
func (ctx *Context) popArgs(base int) {
	clear(ctx.argStack[base:])
	ctx.argStack = ctx.argStack[:base]
}

// qualifiedNameToString converts a QualifiedName AST node to "Package::Name" format.
func qualifiedNameToString(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	parts := make([]string, 0, len(qn.Parts))
	for _, seg := range qn.Parts {
		if seg.Text != "" {
			parts = append(parts, seg.Text)
		}
	}
	return strings.Join(parts, "::")
}

// equalValues is `==` over two operands: a set meeting a sequence flows into the
// ordered context and is compared as its canonical sequence; otherwise valueEqual.
func (ctx *Context) equalValues(a, b Value) bool {
	if a.Kind == ValSet && b.Kind == ValSequence || a.Kind == ValSequence && b.Kind == ValSet {
		return ctx.sequenceEqual(sequenceOf(elementsOf(a)).Sequence(), sequenceOf(elementsOf(b)).Sequence())
	}
	return ctx.valueEqual(a, b)
}

// equalValues is `==` with no context, for values that carry no scale point.
func equalValues(a, b Value) bool {
	return (*Context)(nil).equalValues(a, b)
}

// valueEqual is deep equality with no context: a point on a scale is one value
// with a magnitude on that scale only.
func valueEqual(a, b Value) bool {
	return (*Context)(nil).valueEqual(a, b)
}

// valueEqual checks deep equality of two runtime values: whether they are one
// value, as a set's membership judges. A set is never the sequence of its members.
// With a context a point on a scale equals the magnitude it is on its reference.
func (ctx *Context) valueEqual(a, b Value) bool {
	if isEmptyValue(a) || isEmptyValue(b) {
		return isEmptyValue(a) && isEmptyValue(b)
	}
	// A complex number equals the number it is, whichever kind carries it.
	if a.Kind == ValComplex || b.Kind == ValComplex {
		return complexEqual(a, b)
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case ValConst:
		// Delegate to semantics layer for const equality
		result, ok := semantics.EvalBinary(ast.OpEq, a.Const, b.Const)
		return ok && result.Kind == semantics.ValBool && result.Bool
	case ValString:
		return a.Str() == b.Str()
	case ValNull:
		return true
	case ValInstance:
		return a.Instance == b.Instance
	case ValSequence:
		return ctx.sequenceEqual(a.Sequence(), b.Sequence())
	case ValSet:
		return ctx.setsEqual(a.Set(), b.Set())
	case ValVariant:
		// A variation compares equal to the variant it selected.
		return a.Variant() == b.Variant()
	case ValEnumLiteral:
		// A literal is its own identity: two literals are equal exactly when they
		// are the same declaration, across enumerations included.
		return a.Literal() == b.Literal()
	case ValQuantity:
		// Incommensurable units are not equal here: an equality that has to hold
		// or fail (a set member, a sequence element) has no error to report.
		c, err := ctx.canonicalMagnitudes(*a.Quantity(), *b.Quantity())
		return err == nil && c == 0
	case ValArray:
		return ctx.arrayEqual(a.Array(), b.Array())
	case ValVector:
		return ctx.vectorEqual(a.Vector(), b.Vector())
	case ValVectorQuantity:
		return ctx.vectorQuantityEqual(a.VectorQuantity(), b.VectorQuantity())
	case ValTensorQuantity:
		return ctx.tensorQuantityEqual(a.TensorQuantity(), b.TensorQuantity())
	case ValMeasurementRef:
		return a.MeasurementRef().equal(b.MeasurementRef())
	case ValCoordinateFrame:
		return a.CoordinateFrame().equal(b.CoordinateFrame())
	case ValCoordinateTransformation:
		return a.CoordinateTransformation().equal(b.CoordinateTransformation())
	case ValFunction:
		// A function is the calc it is a value of, read against the same object and, for
		// one closing over a body's bindings, within the same run of that body.
		return a.Function() == b.Function() && a.FunctionSelf() == b.FunctionSelf() &&
			a.functionRun() == b.functionRun()
	case ValMetaobject:
		// A metaobject is the element it denotes, whichever metaclass it was cast to.
		return symbols.SameElement(a.MetaobjectElement(), b.MetaobjectElement())
	case ValUndetermined:
		return a.ref == b.ref
	default:
		return false
	}
}

// arrayEqual holds for arrays of the same dimensions with equal elements.
func (ctx *Context) arrayEqual(a, b *Array) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Dimensions) != len(b.Dimensions) || len(a.Elements) != len(b.Elements) {
		return false
	}
	for i := range a.Dimensions {
		if a.Dimensions[i] != b.Dimensions[i] {
			return false
		}
	}
	for i := range a.Elements {
		if !ctx.valueEqual(a.Elements[i], b.Elements[i]) {
			return false
		}
	}
	return true
}

// vectorEqual holds for vectors of one dimension whose numbers are equal.
func (ctx *Context) vectorEqual(a, b *Vector) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Elements) != len(b.Elements) {
		return false
	}
	for i := range a.Elements {
		if !ctx.valueEqual(constValue(a.Elements[i]), constValue(b.Elements[i])) {
			return false
		}
	}
	return true
}

// vectorQuantityEqual holds for vector quantities whose axes are equal quantities.
func (ctx *Context) vectorQuantityEqual(a, b *VectorQuantity) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Dimension() != b.Dimension() {
		return false
	}
	// Vectors are equal over one mRef only: the same frame, or none for both.
	if !sameVectorFrame(a, b) {
		return false
	}
	for i := 0; i < a.Dimension(); i++ {
		if !ctx.valueEqual(NewQuantityValue(a.component(i)), NewQuantityValue(b.component(i))) {
			return false
		}
	}
	return true
}

// sameVectorFrame holds when both vectors are over one frame or neither is over any.
func sameVectorFrame(a, b *VectorQuantity) bool {
	if (a.Frame == nil) != (b.Frame == nil) {
		return false
	}
	return a.Frame == nil || a.Frame.equal(b.Frame)
}

// sequenceEqual checks structural equality of sequences (element-wise).
func (ctx *Context) sequenceEqual(a, b *Sequence) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Size() != b.Size() {
		return false
	}
	for i := 0; i < a.Size(); i++ {
		aElem, _ := a.At(i)
		bElem, _ := b.At(i)
		if !ctx.valueEqual(aElem, bElem) {
			return false
		}
	}
	return true
}
