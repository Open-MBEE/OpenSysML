package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// calcParameter is one input parameter of a calc, in positional order.
type calcParameter struct {
	Name    string          // parameter name arguments bind to
	Default ast.Node        // value-binding expression used when no argument is passed (nil if none)
	Owner   *symbols.Symbol // the calc that declares the default (a supertype for an inherited one)
	Decl    calcMemberDecl  // the declaration, closest to the invoked calc, a bound value answers to
	// IsSubject marks a case's subject parameter, the object the case is about.
	IsSubject bool
	// IsCalc marks a calc usage parameter (`in calc f`), which a function value binds.
	IsCalc bool
}

// calcMemberDecl is the type and multiplicity a calc's parameter or output
// declares, and the calc whose declaration states them.
type calcMemberDecl struct {
	Target *writeTarget
	Owner  *symbols.Symbol
	// multStated: Target.mult is declared rather than the assumed 1..1.
	multStated bool
}

// redeclaring refines the declaration d redeclares: a redeclaration stating no
// type or multiplicity keeps the redeclared feature's (KerML 1.0 §7.3.4.5).
func (d calcMemberDecl) redeclaring(redeclared calcMemberDecl) calcMemberDecl {
	if d.Target == nil {
		return redeclared
	}
	if redeclared.Target == nil {
		return d
	}
	if d.Target.typ == nil {
		d.Target.typ = redeclared.Target.typ
	}
	if !d.multStated {
		d.Target.mult, d.multStated = redeclared.Target.mult, redeclared.multStated
	}
	return d
}

// check reports a value outside the declared multiplicity or type, described by
// what (asked only on refusal, a binding being the hot path); unknown: not judged.
func (d *calcMemberDecl) check(ctx *Context, value *Value, what func() string) error {
	if d.Target == nil {
		return nil
	}
	if msg := ctx.writeCountRefusal(d.Target, value); msg != "" {
		return fmt.Errorf("%s: %w: %s", what(), ErrMultiplicityViolation, msg)
	}
	if refusal, refused := ctx.writeTypeRefusal(declScope(d.Owner), d.Target.typ, value, admitWritten); refused {
		return fmt.Errorf("%s: %w: %s", what(), ErrTypeMismatch, refusal)
	}
	ctx.spellForDeclared(value, d.Target.typ)
	return nil
}

// admits reports a value the declaration cannot hold as a declared feature value: more or fewer
// values than its multiplicity, or one not of its type. what names the binding; scope answers its names.
func (d calcMemberDecl) admits(ctx *Context, scope *symbols.Scope, what string, value Value) error {
	if d.Target == nil {
		return nil
	}
	if msg := ctx.writeCountRefusal(d.Target, &value); msg != "" {
		return fmt.Errorf("%s: %w: %s", what, ErrMultiplicityViolation, msg)
	}
	typ := d.Target.typ
	if typ == nil {
		return nil
	}
	for _, element := range elementsOf(value) {
		conforms, _, err := ctx.valueConforms(scope, &element, typ, admitDeclared)
		if err != nil {
			return fmt.Errorf("%s: %w", what, err)
		}
		if !conforms {
			return fmt.Errorf("%s: %w: %s is not a %s", what, ErrTypeMismatch, ctx.elementText(element), symbolText(typ))
		}
	}
	return nil
}

// elementText names one value in a refusal: an object by its carrier label, a value by what it is.
func (ctx *Context) elementText(element Value) string {
	if id, ok := element.Object(); ok {
		if inst := ctx.instances[id]; inst != nil && inst.Type != nil {
			return ctx.carrierLabels([]carrier{{instance: inst}})[0]
		}
	}
	return fmt.Sprintf("%s (%s)", FormatValue(element), describeValue(element))
}

// boundMemberDecl is what a member of owner declares for a value bound to it, folded along the
// features it redefines, most specific first: a redeclaration keeps the type and multiplicity it omits.
func (ctx *Context) boundMemberDecl(owner *symbols.Symbol, features []*symbols.Symbol) calcMemberDecl {
	var decl calcMemberDecl
	for i := len(features) - 1; i >= 0; i-- {
		decl = ctx.calcMemberDeclFor(owner, features[i], features[i].Name).redeclaring(decl)
	}
	return decl
}

// calcMemberDeclOf resolves what a member of link declares for a value bound to it.
func (ctx *Context) calcMemberDeclOf(link *symbols.Symbol, sym *symbols.Symbol, name string) calcMemberDecl {
	if sym == nil {
		return calcMemberDecl{}
	}
	return ctx.calcMemberDeclFor(link, sym, name)
}

// calcMemberDeclFor is what the member sym of link declares for a value bound to it.
func (ctx *Context) calcMemberDeclFor(link, sym *symbols.Symbol, name string) calcMemberDecl {
	mult, stated := ctx.extractMultiplicity(sym)
	return calcMemberDecl{
		Target:     &writeTarget{name: name, typ: ctx.extractType(sym), mult: mult},
		Owner:      link,
		multStated: stated,
	}
}

// calcShape is a calc's invocation interface: the input parameters it binds, in
// positional order, the output features it computes, and the expression its body
// returns. All are resolved across the specialization chain, so a usage typed by
// a calc definition inherits that definition's parameters, outputs and result.
type calcShape struct {
	Sym  *symbols.Symbol
	Name string // qualified name, for diagnostics and traces
	// Kind is the notation keyword of the behavior (`calc` or `analysis`), and
	// Label is Kind followed by Name, as diagnostics name the behavior.
	Kind   string
	Label  string
	Params []calcParameter
	// ParamNames are the Params' names by position: the slots an invocation binds.
	ParamNames []string
	// Aliases map the name of a parameter or output redeclared under a new name
	// (`in g :>> x`) to that name, which an inherited body reads it by.
	Aliases map[string]string
	// Outputs are the calc's output features — its `out` parameters and the
	// result parameter a `return` declares — in declaration order.
	Outputs []calcOutput
	// Body is the computation the calc states, lowered in the scope of the calc
	// that declares it: local declarations, assignments, conditionals, loops and
	// returns, in declaration order.
	Body      []lower.Statement
	BodyOwner *symbols.Symbol // the calc whose body declares Body
	// Steps is Body without the bindings of its `out` features, which are
	// evaluated when those features are read rather than run as statements.
	Steps []lower.Statement
	// Nodes are the action nodes the body's flow performs as the steps of a case.
	Nodes []ast.Node
	// BodyOutputs are the output features some statement of the body assigns,
	// whatever path the execution takes. A calc that binds its outputs this way
	// computes them though it returns nothing, so it states a computation.
	BodyOutputs map[string]bool
	Bindings    []lower.Binding
	ResultExpr  ast.Node
	// Uncomputed says why the calc computes nothing (no body, no bound output);
	// the calc is still a function value, and invoking it reports this.
	Uncomputed error
	// compiled is the body in the compiled tier once compileState says it is
	// eligible; a shape found ineligible keeps the evaluator for good.
	compiled     *compiledCalc
	compileState compileState
	// ineligibleWhy says what kept the body out of the compiled tier.
	ineligibleWhy string
	// members indexes the chain's named members by binding name, built on first use.
	members map[*symbols.Symbol]string
}

// calcShapeOf resolves the invocation interface of a calc symbol: its
// positional input parameters (own and inherited, with defaults) and its result
// expression. A calc computing nothing is an error. The result is memoized per symbol.
func (ctx *Context) calcShapeOf(sym *symbols.Symbol) (*calcShape, error) {
	shape, err := ctx.calcInterfaceOf(sym)
	if err != nil {
		return nil, err
	}
	if shape.Uncomputed != nil {
		return nil, shape.Uncomputed
	}
	return shape, nil
}

// calcInterfaceOf is calcShapeOf for a calc that may compute nothing — an
// abstract calc, or one awaiting a body — whose shape records why in Uncomputed.
func (ctx *Context) calcInterfaceOf(sym *symbols.Symbol) (*calcShape, error) {
	if sym == nil || sym.Decl == nil {
		return nil, fmt.Errorf("%w: invalid symbol", ErrNotACalc)
	}
	if cached, ok := ctx.model.calcShapes[sym]; ok {
		return cached, nil
	}

	name := ctx.qualifiedSymbolName(sym)
	if !isCalcDecl(sym.Decl) && !ctx.calcTypedFeature(sym) {
		return nil, fmt.Errorf("%w: %s is %s, not a calc definition or usage", ErrNotACalc, name, describeDecl(sym.Decl))
	}
	kind, label := calcKindLabel(sym, name)

	// Most general first, so an inherited parameter keeps the position it has in
	// the calc that declares it and a redeclaration refines it in place.
	chain := ctx.calcChain(sym)
	if conflict := ctx.model.semantics.ResultExpressionConflict(sym); conflict != nil {
		if conflict.Stated > 1 {
			return nil, fmt.Errorf("%w: %s states %d result expressions",
				ErrConflictingResultExpressions, label, conflict.Stated)
		}
		names := make([]string, len(conflict.Owners))
		for i, owner := range conflict.Owners {
			names[i] = ctx.qualifiedSymbolName(owner)
		}
		return nil, fmt.Errorf("%w: %s states or inherits a result expression from each of %s",
			ErrConflictingResultExpressions, label, strings.Join(names, ", "))
	}
	body, bodyOwner := calcBody(chain)
	shape := &calcShape{
		Sym:       sym,
		Name:      name,
		Kind:      kind,
		Label:     label,
		Body:      body,
		BodyOwner: bodyOwner,
		Steps:     calcSteps(body),
	}
	shape.Nodes = lower.BlockNodes(shape.Steps)
	shape.Params = ctx.calcParameters(chain, &shape.Aliases)
	shape.Outputs = ctx.calcOutputs(chain, &shape.Aliases)
	shape.ParamNames = make([]string, len(shape.Params))
	for i, param := range shape.Params {
		shape.ParamNames[i] = param.Name
	}
	shape.BodyOutputs = assignedOutputs(shape.Steps, shape.Outputs, shape.Aliases)
	shape.Bindings = calcBindings(chain)
	shape.ResultExpr = resultBindingExpr(shape.Bindings)
	// A calc computes nothing unless it returns or binds an output; a case also
	// computes through its steps, or answers with its verdicts alone, and a
	// library function the runtime implements natively computes through that.
	performs := shape.isCase() && (len(shape.Nodes) > 0 || ctx.analysisChecks(sym))
	_, native := ctx.libraryFunctionFor(sym)
	computes := lower.Returns(shape.Body) || len(shape.BodyOutputs) > 0 || shape.ResultExpr != nil || shape.hasInitialOutput() || performs || native
	switch {
	case computes:
	case len(shape.Outputs) > 0 && shape.resultOutput() == nil:
		shape.Uncomputed = fmt.Errorf("%w: %s binds none of its outputs (%s)",
			ErrNoResultExpression, label, shape.outputNames())
	default:
		shape.Uncomputed = fmt.Errorf("%w: %s has no return expression%s", ErrNoResultExpression, label, unboundResultHint(chain))
	}

	ctx.model.calcShapes[sym] = shape
	return shape, nil
}

func calcBindings(chain []*symbols.Symbol) []lower.Binding {
	var out []lower.Binding
	for _, link := range chain {
		if link != nil {
			out = append(out, lower.ToBindings(link.Decl, declScope(link))...)
		}
	}
	return out
}

func resultBindingExpr(bindings []lower.Binding) ast.Node {
	var result ast.Node
	for _, binding := range bindings {
		for i := range binding.Ends {
			if binding.Ends[i].Path != "result" {
				continue
			}
			other := binding.Ends[1-i]
			if other.Expr != nil {
				result = other.Expr
			}
		}
	}
	return result
}

// calcChain returns the calcs sym takes members from (its supertypes and the calc
// it references), most general first, then sym. Non-calc links and the library's
// frame contribute nothing; a domain library's calc contributes as a model's does.
func (ctx *Context) calcChain(sym *symbols.Symbol) []*symbols.Symbol {
	supers := ctx.model.semantics.MemberSources(sym)
	chain := make([]*symbols.Symbol, 0, len(supers)+1)
	for i := len(supers) - 1; i >= 0; i-- {
		if supers[i] != nil && isCalcDecl(supers[i].Decl) && !ctx.frameDeclared(supers[i]) {
			chain = append(chain, supers[i])
		}
	}
	return append(chain, sym)
}

// calcParameters flattens the input parameters declared along chain (most general
// first): a redeclaration keeps its position and default, a renamed one is aliased.
func (ctx *Context) calcParameters(chain []*symbols.Symbol, aliases *map[string]string) []calcParameter {
	var params []calcParameter
	index := make(map[string]int)

	for _, link := range chain {
		for _, member := range declMembers(link.Decl) {
			if subject, ok := subjectDeclaration(member); ok {
				params = ctx.subjectParameter(params, index, aliases, link, member, subject)
				continue
			}
			usage, ok := member.(*ast.Usage)
			if !ok {
				continue
			}
			name, _ := ast.EffectiveName(usage)
			if name == "" {
				continue
			}
			if usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut {
				continue
			}
			sym := memberSymbol(declScope(link), usage)
			param := calcParameter{
				Name: name, Default: usage.Value, Owner: link,
				Decl: ctx.calcMemberDeclOf(link, sym, name), IsCalc: isCalcUsageSymbol(sym),
			}
			if at, seen := ctx.redeclaredIndex(index, sym, name); seen {
				// A redeclaration binding no value keeps the inherited default,
				// which is written in the scope of the calc that stated it.
				if param.Default == nil {
					param.Default = params[at].Default
					param.Owner = params[at].Owner
				}
				param.Decl = param.Decl.redeclaring(params[at].Decl)
				*aliases = aliasRedefined(*aliases, params[at].Name, name)
				params[at] = param
				index[name] = at
				continue
			}
			index[param.Name] = len(params)
			params = append(params, param)
		}
	}
	return params
}

// redeclaredIndex finds the flattened member the member sym, named name,
// redeclares: the one of its own name, else one it redefines (`in y :>> x`).
func (ctx *Context) redeclaredIndex(index map[string]int, sym *symbols.Symbol, name string) (int, bool) {
	if at, seen := index[name]; seen {
		return at, true
	}
	for _, redefined := range ctx.model.semantics.RedefinedFeatures(sym) {
		if at, seen := index[redefined.Name]; seen {
			return at, true
		}
	}
	return 0, false
}

// calcBody returns the computation the invoked calc runs — its own body if that
// states one, otherwise the closest inherited one — with the calc that declares
// it, whose scope the body's statements are written in.
func calcBody(chain []*symbols.Symbol) ([]lower.Statement, *symbols.Symbol) {
	var stated []lower.Statement
	var owner *symbols.Symbol
	for i := len(chain) - 1; i >= 0; i-- {
		link := chain[i]
		stmts := lower.CalcBody(link.Decl, declMembers(link.Decl), link.Scope)
		if lower.Returns(stmts) {
			return stmts, link
		}
		// A body that computes but returns nothing leaves an inherited result in
		// force, so keep looking up the chain before settling for it.
		if stated == nil && len(stmts) > 0 {
			stated, owner = stmts, link
		}
	}
	return stated, owner
}

// unboundResultHint explains a `return` that declares a result parameter without
// binding it (`return h;` declares h, it does not return the member h).
func unboundResultHint(chain []*symbols.Symbol) string {
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i] == nil {
			continue
		}
		members := declMembers(chain[i].Decl)
		result := unboundResultParameter(members)
		if result == nil {
			continue
		}
		name, _ := ast.EffectiveName(result)
		who, trailing, expr := "the result parameter", "of the body", "<expr>"
		typ := usageTypeText(result)
		if name != "" {
			spelled := lexer.NameText(name)
			who = "result parameter " + spelled
			if sibling := valuedMemberNamed(members, name, result); sibling != nil {
				trailing, expr = "`"+spelled+"`", spelled
				if typ == "" {
					typ = usageTypeText(sibling)
				}
			}
		}
		if typ == "" {
			typ = "<type>"
		}
		return fmt.Sprintf(": %s binds no value; write the result as the trailing expression %s, or bind it with `return : %s = %s;`",
			who, trailing, typ, expr)
	}
	return ""
}

// unboundResultParameter returns the body's `return` member that binds no value.
func unboundResultParameter(members []ast.Node) *ast.Usage {
	for _, member := range members {
		if u, ok := member.(*ast.Usage); ok && u.IsResult && u.Value == nil {
			return u
		}
	}
	return nil
}

// valuedMemberNamed returns the body member called name, other than except,
// whose declaration binds a value; an unbound one would not compute a result.
func valuedMemberNamed(members []ast.Node, name string, except *ast.Usage) *ast.Usage {
	for _, member := range members {
		u, ok := member.(*ast.Usage)
		if !ok || u == except || u.Value == nil {
			continue
		}
		if actual, _ := ast.EffectiveName(u); actual == name {
			return u
		}
	}
	return nil
}

// usageTypeText spells the type a usage declares with `:` as the notation
// writes it (each segment quoted when it must be), or "" without one.
func usageTypeText(u *ast.Usage) string {
	for _, rel := range u.Relationships {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		qn, ok := rel.Target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		segments := make([]string, 0, len(qn.Parts))
		for _, part := range qn.Parts {
			segments = append(segments, lexer.NameText(part.Text))
		}
		text := strings.Join(segments, "::")
		if qn.Global {
			text = "$::" + text
		}
		return text
	}
	return ""
}

// calcArgs are the arguments of one calc invocation. The notation keeps the two
// forms mutually exclusive: positional arguments bind in parameter order, named
// arguments by parameter name.
type calcArgs struct {
	positional []Value
	named      map[string]Value
}

// InvokeCalc invokes a calculation with the given positional arguments and
// returns its result. Arguments bind to the calc's input parameters in
// declaration order; a parameter with no argument falls back to its declared
// default. The body is evaluated in the calc's own scope, so scope is used only
// as a fallback for a symbol that owns no scope. A library function's `expr`
// parameter takes a body or expression value, applied only when selected, or
// the operand's value itself.
func (ctx *Context) InvokeCalc(sym *symbols.Symbol, args []Value, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

	if err := ctx.requireCalcNotCase(sym); err != nil {
		return Value{}, err
	}
	return ctx.invokeCalc(sym, calcArgs{positional: args}, scope)
}

// requireCalcNotCase refuses a case as the target of a calc invocation: run as
// a case, it also binds a subject and reports its verdicts.
func (ctx *Context) requireCalcNotCase(sym *symbols.Symbol) error {
	if sym == nil || !IsRunnableCaseSymbol(sym) {
		return nil
	}
	runAs := "an analysis case"
	if IsVerificationCaseSymbol(sym) {
		runAs = "a verification case"
	}
	return fmt.Errorf("%w: %s is %s, not a calc definition or usage; run it as %s",
		ErrNotACalc, ctx.qualifiedSymbolName(sym), describeDecl(sym.Decl), runAs)
}

// InvokeCalcWith invokes a calculation with positional arguments and arguments
// bound by parameter name together, which is what an argument list plus a swept
// parameter binds. Arguments bind exactly as InvokeCalc and InvokeCalcNamed
// bind them: by position first, then by name.
func (ctx *Context) InvokeCalcWith(sym *symbols.Symbol, positional []Value, named map[string]Value, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

	if err := ctx.requireCalcNotCase(sym); err != nil {
		return Value{}, err
	}
	return ctx.invokeCalc(sym, calcArgs{positional: positional, named: named}, scope)
}

// InvokeCalcNamed invokes a calculation with arguments bound by parameter name.
// A parameter with no argument falls back to its declared default.
func (ctx *Context) InvokeCalcNamed(sym *symbols.Symbol, args map[string]Value, scope *symbols.Scope) (Value, error) {
	defer ctx.beginRun()()

	if err := ctx.requireCalcNotCase(sym); err != nil {
		return Value{}, err
	}
	return ctx.invokeCalc(sym, calcArgs{named: args}, scope)
}

func (ctx *Context) invokeCalcNamedShapeOn(shape *calcShape, args map[string]Value, scope *symbols.Scope, self *Instance) (Value, error) {
	return ctx.invokeCalcShape(shape, calcArgs{named: args}, scope, self)
}

// invokeCalc resolves the calc's shape and invokes it, the single path every
// calc invocation takes. A function library declaration the library gives no
// body is applied by its built-in implementation instead.
func (ctx *Context) invokeCalc(sym *symbols.Symbol, args calcArgs, scope *symbols.Scope) (Value, error) {
	return ctx.invokeCalcWithSelf(sym, args, scope, nil)
}

func (ctx *Context) invokeCalcWithSelf(sym *symbols.Symbol, args calcArgs, scope *symbols.Scope, self *Instance) (Value, error) {
	if perf := ctx.libraryCalcPerformed(sym); perf != nil {
		if perf.signature != nil {
			return ctx.invokeLibraryPerformance(perf, args, scope, self)
		}
		sym = perf.lib
	}
	if fn, ok := ctx.builtinFor(sym); ok {
		return ctx.invokeBuiltinValues(sym, fn, args, scope, self)
	}
	if fn, ok := ctx.libraryFunctionFor(sym); ok {
		return fn.invoke(ctx, args)
	}

	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return Value{}, err
	}
	return ctx.invokeCalcShape(shape, args, scope, self)
}

// invokeBuiltinValues applies a built-in to arguments a direct invocation has
// already evaluated, bound to its declared parameters as a call would bind them.
func (ctx *Context) invokeBuiltinValues(sym *symbols.Symbol, fn builtinFunc, args calcArgs, callerScope *symbols.Scope, self *Instance) (Value, error) {
	name := ctx.qualifiedSymbolName(sym)
	entered := ctx.activations
	return tracedBuiltin(ctx.trace, name,
		func() ([]Value, error) { return bindBuiltinValues(name, args) },
		func(bound []Value) (Value, error) {
			ec := NewEvalContextIn(ctx, ctx.calcScope(sym, nil, callerScope), self)
			ec.entered = entered
			return fn(ec, bound)
		},
	)
}

// invocationFrame is the storage one calc invocation runs in, held off the
// context's free list until the invocation returns, so no two active ones share it.
type invocationFrame struct {
	ec     EvalContext
	frames [1]frame
	// slots hold the parameters; bindings the locals the body declares beside them.
	slots    slotFrame
	bindings map[string]Value
	aliases  map[string]string
	owner    *calcShape // the calc invoked, whose members the locals bind
	run      int64      // the run this invocation is (Context.newRun)
	host     calcStmtHost
	env      stmtEnv
	engine   stmtEngine
}

// locals is the frame the invocation's parameters and body locals are bound in.
func (f *invocationFrame) locals() frame {
	return frame{slots: &f.slots, vars: f.bindings, aliases: f.aliases, owner: f.owner, run: f.run}
}

// maxFreeInvocationFrames bounds the frames kept, so one deep recursion does not
// pin that many for the context's whole life.
const maxFreeInvocationFrames = 1024

// maxPooledBindings is the widest frame whose storage a free frame keeps: clearing
// keeps the backing storage, so a wider one is dropped rather than pinned.
const maxPooledBindings = 32

// acquireInvocationFrame takes a frame off the free list, or makes one when it is empty.
func (ctx *Context) acquireInvocationFrame() *invocationFrame {
	if n := len(ctx.freeInvocationFrames); n > 0 {
		frame := ctx.freeInvocationFrames[n-1]
		ctx.freeInvocationFrames[n-1] = nil
		ctx.freeInvocationFrames = ctx.freeInvocationFrames[:n-1]
		if frame.bindings == nil {
			frame.bindings = make(map[string]Value)
		}
		return frame
	}
	return &invocationFrame{bindings: make(map[string]Value)}
}

// releaseInvocationFrame empties frame and keeps it for the next invocation; the
// caller has ended the activation, so nothing memoized still reads the bindings.
func (ctx *Context) releaseInvocationFrame(frame *invocationFrame) {
	bindings, slots := frame.bindings, frame.slots
	if frame.locals().width() > maxPooledBindings {
		bindings, slots = nil, slotFrame{}
	} else {
		clear(bindings)
		slots.release()
	}
	frameBuf := frame.engine.frameBuf
	clear(frameBuf)
	*frame = invocationFrame{bindings: bindings, slots: slots}
	frame.engine.frameBuf = frameBuf[:0]
	if len(ctx.freeInvocationFrames) < maxFreeInvocationFrames {
		ctx.freeInvocationFrames = append(ctx.freeInvocationFrames, frame)
	}
}

// invokeCalcShape binds arguments and evaluates the calc body. Binding happens
// in the calc's own environment: defaults and the result expression see the
// parameters and the calc's lexical scope, never the caller's frames.
func (ctx *Context) invokeCalcShape(shape *calcShape, args calcArgs, callerScope *symbols.Scope, self *Instance) (Value, error) {
	return ctx.invokeCalcShapeIn(shape, args, callerScope, self, nil)
}

// invokeCalcShapeIn is invokeCalcShape for a calc declared in a behavior body:
// enclosing holds that body's bindings, outermost first, which the calc's own shadow.
func (ctx *Context) invokeCalcShapeIn(shape *calcShape, args calcArgs, callerScope *symbols.Scope, self *Instance, enclosing []frame) (Value, error) {
	if shape.Uncomputed != nil {
		return Value{}, shape.Uncomputed
	}
	if err := shape.checkArgs(args); err != nil {
		return Value{}, err
	}

	// A pure body runs compiled unless the run is traced, which records every
	// sub-expression, an argument is not a scalar, a bound object may answer
	// a library constant the body reads before the library does, or the body
	// reads the bindings enclosing it.
	if ctx.compileCalcs && ctx.trace == nil && len(enclosing) == 0 {
		if compiled := ctx.compiledCalcOf(shape); compiled != nil && (self == nil || !compiled.readsLibrary) {
			if result, ran, err := compiled.invokeBoxed(ctx, args); ran {
				return result, err
			}
		}
	}

	if err := ctx.enterCalc(shape.Name); err != nil {
		return Value{}, err
	}
	defer ctx.leaveCalc()

	frame := ctx.acquireInvocationFrame()
	defer ctx.releaseInvocationFrame(frame)

	// The invocation is one activation, defaults included, so a calc usage read while
	// binding answers this invocation alone and ends with it, before the frame is reused.
	activation := ctx.newActivation()
	defer ctx.endActivation(activation)

	frame.slots.reset(shape.ParamNames)
	frame.aliases, frame.owner, frame.run = shape.Aliases, shape, ctx.newRun()
	locals := frame.locals()
	ec := &frame.ec
	*ec = EvalContext{
		ctx:        ctx,
		scope:      ctx.calcScope(shape.BodyOwner, shape.Sym, callerScope),
		self:       self,
		frames:     append(append(frame.frames[:0], enclosing...), locals),
		trace:      ctx.trace,
		activation: activation,
	}

	if ec.trace != nil {
		ec.trace.RecordCalculationEnter(shape.Kind, shape.Name)
	}

	if err := ctx.bindCalcParameters(shape, ec, args, callerScope, locals, nil); err != nil {
		if ec.trace != nil {
			ec.trace.RecordCalculationExitError(shape.Kind, shape.Name, err)
		}
		return Value{}, err
	}

	result, err := ctx.runCalcBody(shape, frame, callerScope, self, activation, enclosing)
	if ec.trace != nil {
		if err != nil {
			ec.trace.RecordCalculationExitError(shape.Kind, shape.Name, err)
		} else {
			ec.trace.RecordCalculationExit(shape.Kind, shape.Name, result)
		}
	}
	if err != nil {
		return Value{}, calcFrame(shape.Kind, shape.Name, err)
	}
	return result, nil
}

// enterCalc spends one of the run's calc depth budget, so a recursion evaluates
// while it terminates within the budget. leaveCalc takes it back off.
func (ctx *Context) enterCalc(name string) error {
	if int64(ctx.calcDepth) >= ctx.maxCalcDepth {
		return ctx.calcDepthExceeded(name)
	}
	ctx.calcDepth++
	return nil
}

// calcDepthExceeded reports the calc depth budget spent; kept out of line so
// enterCalc inlines into every invocation.
//
//go:noinline
func (ctx *Context) calcDepthExceeded(name string) error {
	return fmt.Errorf(
		"%w: calc %s nested %d deep (unbounded recursion?; raise %s to allow more)",
		ErrCalcRecursionLimit, name, ctx.maxCalcDepth, MaxCalcDepthEnvVar,
	)
}

// leaveCalc returns the calc depth enterCalc spent.
func (ctx *Context) leaveCalc() {
	ctx.calcDepth--
}

// bindCalcParameters binds the calc's input parameters into bindings: each
// parameter's argument, or, for a parameter no argument supplies, the default
// declared closest to the invoked calc, evaluated in the scope of the calc
// declaring it. ec supplies the environment defaults are evaluated in; nested,
// when non-null, is the environment reading a nested usage, which the bindings
// the usage itself declares are written in and evaluate against.
func (ctx *Context) bindCalcParameters(
	shape *calcShape,
	ec *EvalContext,
	args calcArgs,
	callerScope *symbols.Scope,
	bindings frame,
	nested *EvalContext,
) error {
	for i := range shape.Params {
		param := &shape.Params[i]
		defaultScope := ctx.calcScope(param.Owner, shape.Sym, callerScope)
		binder := ec
		if nested != nil && param.Owner == shape.Sym {
			binder = nested
		}
		value, source, err := binder.evalIn(defaultScope).bindCalcParameter(shape, param, args, i, nested)
		if err != nil {
			return err
		}
		// The parameter holds the value bound to it, so that value answers to the
		// parameter's declaration as a written one does.
		what := func() string {
			return fmt.Sprintf("%s: %s for parameter %q", shape.Label, source, param.Name)
		}
		if err := param.Decl.check(ctx, &value, what); err != nil {
			return err
		}
		if err := param.checkFunction(&value, what); err != nil {
			return err
		}
		bindings.bindParam(i, param.Name, value)
		if ec.trace != nil {
			ec.trace.RecordCalcBind(param.Name, value, source)
		}
	}
	return nil
}

// runCalcBody runs the calc's computation in the invocation's frame — its
// bindings hold the parameters and the locals — and answers with the one value
// the invocation yields: what the body returned, or, for a body that returns
// nothing, the calc's designated output feature, evaluated in the invocation's
// activation, which the caller ends after it.
func (ctx *Context) runCalcBody(shape *calcShape, frame *invocationFrame, callerScope *symbols.Scope, self *Instance, activation int64, enclosing []frame) (Value, error) {
	frame.host = calcStmtHost{ctx: ctx, shape: shape, self: self}
	frame.env = stmtEnv{data: frame.locals(), enclosing: shape.bodyEnclosing(enclosing)}
	frame.engine = stmtEngine{ctx: ctx, host: &frame.host, env: &frame.env, activation: activation, frameBuf: frame.engine.frameBuf}
	frame.host.attachPerformances(&frame.engine)
	result, returned, err := runCalcSteps(&frame.engine, &frame.host, shape)
	if err != nil {
		return Value{}, err
	}
	if returned {
		return result, nil
	}
	out, err := shape.designatedOutput()
	if err != nil {
		return Value{}, err
	}
	// The designated output's binding may name the calc's other outputs, and one
	// naming itself is a cycle rather than an evaluation, so it is evaluated
	// through the same run bookkeeping a calc usage's outputs use.
	run := newCalcRun(shape, callerScope, self, frame.locals())
	run.activation, run.perf = activation, frame.host.performance()
	if len(enclosing) > 0 {
		run.outer = &EvalContext{ctx: ctx, scope: callerScope, self: self, frames: enclosing, trace: ctx.trace, activation: activation}
	}
	// The invocation already holds this evaluation's nesting feature value.
	run.onStack = true
	return run.value(ctx, out)
}

// runCalcSteps runs the calc's lowered computation on engine, whose data holds
// the calc's parameters on the way in and its locals on the way out, reporting
// the value host took from a `return` and whether the body returned one.
func runCalcSteps(engine *stmtEngine, host *calcStmtHost, shape *calcShape) (Value, bool, error) {
	flow, err := engine.run(shape.Steps)
	if err != nil {
		return Value{}, false, err
	}
	return host.result, flow == flowReturn, nil
}

// checkArgs rejects an argument list that cannot bind to the parameters at all:
// more positional arguments than parameters, or a name that is not one.
func (shape *calcShape) checkArgs(args calcArgs) error {
	if len(args.positional) > len(shape.Params) {
		return fmt.Errorf(
			"%w: %s takes %d argument(s), got %d",
			ErrCalcArity, shape.Label, len(shape.Params), len(args.positional),
		)
	}

	// Sorted, so the reported name does not depend on map iteration order.
	names := make([]string, 0, len(args.named))
	for name := range args.named {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if !shape.hasParameter(name) {
			return fmt.Errorf(
				"%w: %s has no input parameter %q",
				ErrUnknownParameter, shape.Label, name,
			)
		}
	}
	return nil
}

// optional reports whether the parameter may go without an argument: its
// declared multiplicity admits no value, as `[0..1]` does.
func (param *calcParameter) optional() bool {
	return param.Decl.Target != nil && param.Decl.multStated && param.Decl.Target.mult.AllowsNone()
}

// hasParameter reports whether the calc declares an input parameter of that name.
func (shape *calcShape) hasParameter(name string) bool {
	for _, param := range shape.Params {
		if param.Name == name {
			return true
		}
	}
	return false
}

// performs reports whether the body's action nodes are performed as steps: a case
// is an action, so its body is; a calc's body performs nothing.
func (shape *calcShape) performs() bool {
	return shape.isCase()
}

// isCase reports whether the shape is a case — an analysis or a verification —
// rather than a plain calc.
func (shape *calcShape) isCase() bool {
	return shape.Kind == "analysis" || shape.Kind == "verification"
}

// bodyScope is the namespace the body's names resolve in.
func (shape *calcShape) bodyScope() *symbols.Scope {
	if shape.BodyOwner != nil {
		return shape.BodyOwner.Scope
	}
	return shape.Sym.Scope
}

// bindCalcParameter resolves the value of one parameter: its argument (by
// position or by name), else its declared default, else null for one whose
// multiplicity admits no value. A parameter with none of these is unbound,
// which is a modeling error rather than a null value. A subject with none of
// these takes the enclosing case's subject, read from enclosing.
func (ec *EvalContext) bindCalcParameter(
	shape *calcShape,
	param *calcParameter,
	args calcArgs,
	position int,
	enclosing *EvalContext,
) (Value, string, error) {
	if position < len(args.positional) {
		return args.positional[position], "argument", nil
	}
	if value, ok := args.named[param.Name]; ok {
		return value, "argument", nil
	}
	if param.Default == nil {
		if param.IsSubject {
			if value, ok := ec.ctx.enclosingSubject(shape, enclosing); ok {
				return value, "enclosing subject", nil
			}
			return Value{}, "", shape.unboundSubject(param)
		}
		if param.optional() {
			return nullValue(), "omitted", nil
		}
		return Value{}, "", fmt.Errorf(
			"%w: %s parameter %q has no argument and no default",
			ErrUnboundParameter, shape.Label, param.Name,
		)
	}
	value, err := ec.Eval(param.Default)
	if err != nil {
		return Value{}, "", calcDefaultError(shape.Kind, shape.Name, param.Name, err)
	}
	return value, "default", nil
}

// calcScope returns the scope an inherited or own calc body member is written
// in: the declaring calc's own scope, falling back to the invoked calc's and
// then to the caller's for a declaration that owns no scope.
func (ctx *Context) calcScope(declarer, invoked *symbols.Symbol, callerScope *symbols.Scope) *symbols.Scope {
	for _, sym := range []*symbols.Symbol{declarer, invoked} {
		if sym != nil && sym.Scope != nil {
			return sym.Scope
		}
	}
	return callerScope
}

// calcTypedFeature reports whether sym is a feature typed by a calc — a step, which an
// invocation performs as the calc it is typed by.
func (ctx *Context) calcTypedFeature(sym *symbols.Symbol) bool {
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return false
	}
	return len(ctx.calcChain(sym)) > 1
}

// libraryPerformance is how a call of a model calc applies an inherited library
// function: the library declaration implementing it and, when the model calcs
// redeclare inputs, the effective signature the call's arguments bind through.
type libraryPerformance struct {
	lib       *symbols.Symbol
	signature *calcShape // the effective inputs, bodiless; nil when the library's own bind as written
	positions []int      // per signature parameter, the library input it redefines; -1 for none
	arity     int        // the library declaration's input parameter count
}

// libraryCalcPerformed returns the library function whose implementation a call of
// sym applies: the nearest one sym specializes, when neither sym nor a model calc
// between them states a computation of its own; nil otherwise.
func (ctx *Context) libraryCalcPerformed(sym *symbols.Symbol) *libraryPerformance {
	if sym == nil || ctx.model.semantics == nil || ctx.libraryDeclared(sym) {
		return nil
	}
	if cached, ok := ctx.model.libraryPerformances[sym]; ok {
		return cached
	}
	perf := ctx.resolveLibraryPerformance(sym)
	ctx.model.libraryPerformances[sym] = perf
	return perf
}

func (ctx *Context) resolveLibraryPerformance(sym *symbols.Symbol) *libraryPerformance {
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
		return nil
	}
	chain := ctx.calcChain(sym)
	if ctx.calcComputes(chain) {
		return nil
	}
	lib := ctx.implementedLibraryCalc(sym)
	if lib == nil {
		return nil
	}
	libInputs := ctx.ownedInputSymbols(lib)
	perf := &libraryPerformance{lib: lib, arity: len(libInputs)}
	if !ctx.redeclaresInputs(chain) {
		return perf
	}
	perf.signature = &calcShape{Sym: sym, Name: ctx.qualifiedSymbolName(sym)}
	perf.signature.Kind, perf.signature.Label = calcKindLabel(sym, perf.signature.Name)
	for _, p := range ctx.model.semantics.BehaviorParametersOf(sym) {
		if p.IsResult || (p.Direction != ast.DirIn && p.Direction != ast.DirInOut) {
			continue
		}
		param, position := ctx.effectiveParameter(p.Symbol, libInputs)
		perf.signature.Params = append(perf.signature.Params, param)
		perf.signature.ParamNames = append(perf.signature.ParamNames, param.Name)
		perf.positions = append(perf.positions, position)
	}
	return perf
}

// effectiveParameter is the input parameter sym as a call binds it — named as
// written, declared and defaulted by the nearest of its redefinitions the model
// states — with the position of the library input it redefines, -1 for none.
func (ctx *Context) effectiveParameter(sym *symbols.Symbol, libInputs []*symbols.Symbol) (calcParameter, int) {
	param := calcParameter{Name: sym.Name, IsCalc: isCalcUsageSymbol(sym)}
	if effective, _ := ast.EffectiveName(sym.Decl.(*ast.Usage)); effective != "" {
		param.Name = effective
	}
	for _, link := range ctx.model.semantics.ParameterRedefinitionChain(sym) {
		owner := link.OwnerScope.Owner()
		param.Decl = param.Decl.redeclaring(ctx.calcMemberDeclFor(owner, link, param.Name))
		if at := indexOfSymbol(libInputs, link); at >= 0 {
			return param, at
		}
		if ctx.libraryDeclared(link) {
			break
		}
		if value := ctx.extractDefaultValue(link); param.Default == nil && value != nil {
			param.Default, param.Owner = value, owner
		}
	}
	return param, -1
}

// implementedLibraryCalc returns the nearest library calc sym specializes that
// this runtime implements, or nil when none is.
func (ctx *Context) implementedLibraryCalc(sym *symbols.Symbol) *symbols.Symbol {
	for _, super := range ctx.model.semantics.AllSupertypes(sym) {
		if super == nil || !isCalcDecl(super.Decl) || !ctx.libraryDeclared(super) {
			continue
		}
		if _, ok := ctx.builtinFor(super); ok {
			return super
		}
		if _, ok := ctx.libraryFunctionFor(super); ok {
			return super
		}
	}
	return nil
}

// redeclaresInputs reports whether a model calc along chain declares an input
// parameter of its own, redefining or adding to the ones it inherits.
func (ctx *Context) redeclaresInputs(chain []*symbols.Symbol) bool {
	for _, link := range chain {
		if len(ctx.ownedInputSymbols(link)) > 0 {
			return true
		}
	}
	return false
}

// ownedInputSymbols returns the symbols of the input parameters sym's declaration
// owns, in declaration order.
func (ctx *Context) ownedInputSymbols(sym *symbols.Symbol) []*symbols.Symbol {
	var inputs []*symbols.Symbol
	for _, member := range declMembers(sym.Decl) {
		usage, ok := member.(*ast.Usage)
		if !ok || (usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut) {
			continue
		}
		if found := memberSymbol(declScope(sym), usage); found != nil {
			inputs = append(inputs, found)
		}
	}
	return inputs
}

func indexOfSymbol(syms []*symbols.Symbol, want *symbols.Symbol) int {
	for i, sym := range syms {
		if sym == want {
			return i
		}
	}
	return -1
}

// invokeLibraryPerformance binds args to the effective signature of a calc performing
// a library function as any calc binds its parameters — each default evaluated where
// it is declared with the earlier parameters bound, each value checked against its
// declaration — then applies the function to them in its own parameter order.
func (ctx *Context) invokeLibraryPerformance(perf *libraryPerformance, args calcArgs, callerScope *symbols.Scope, self *Instance) (Value, error) {
	sig := perf.signature
	if err := sig.checkArgs(args); err != nil {
		return Value{}, err
	}
	if err := ctx.enterCalc(sig.Name); err != nil {
		return Value{}, err
	}
	defer ctx.leaveCalc()
	if ctx.trace != nil {
		ctx.trace.RecordCalcEnter(sig.Name)
	}
	result, err := ctx.applyLibraryPerformance(perf, args, callerScope, self)
	if ctx.trace != nil {
		if err != nil {
			ctx.trace.RecordCalcExitError(sig.Name, err)
		} else {
			ctx.trace.RecordCalcExit(sig.Name, result)
		}
	}
	return result, err
}

func (ctx *Context) applyLibraryPerformance(perf *libraryPerformance, args calcArgs, callerScope *symbols.Scope, self *Instance) (Value, error) {
	sig := perf.signature
	frame := ctx.acquireInvocationFrame()
	defer ctx.releaseInvocationFrame(frame)
	activation := ctx.newActivation()
	defer ctx.endActivation(activation)

	frame.slots.reset(sig.ParamNames)
	locals := frame.locals()
	ec := &frame.ec
	*ec = EvalContext{
		ctx:        ctx,
		scope:      ctx.calcScope(sig.Sym, sig.Sym, callerScope),
		self:       self,
		frames:     append(frame.frames[:0], locals),
		trace:      ctx.trace,
		activation: activation,
	}
	if err := ctx.bindCalcParameters(sig, ec, args, callerScope, locals, nil); err != nil {
		return Value{}, err
	}

	values := make([]Value, perf.arity)
	for i := range values {
		values[i] = nullValue()
	}
	for i, position := range perf.positions {
		if position < 0 {
			return Value{}, fmt.Errorf("%w: function %s has no input parameter for %q of calc %s",
				ErrUnknownParameter, ctx.qualifiedSymbolName(perf.lib), sig.ParamNames[i], sig.Name)
		}
		values[position] = frame.slots.values[i]
	}
	libArgs := calcArgs{positional: values}
	if fn, ok := ctx.builtinFor(perf.lib); ok {
		return ctx.invokeBuiltinValues(perf.lib, fn, libArgs, callerScope, self)
	}
	fn, _ := ctx.libraryFunctionFor(perf.lib)
	return fn.invoke(ctx, libArgs)
}

// calcComputes reports whether a calc chain states a computation: a body that
// returns or assigns an output, or a binding of the result.
func (ctx *Context) calcComputes(chain []*symbols.Symbol) bool {
	body, _ := calcBody(chain)
	if lower.Returns(body) || resultBindingExpr(calcBindings(chain)) != nil {
		return true
	}
	var aliases map[string]string
	return len(assignedOutputs(calcSteps(body), ctx.calcOutputs(chain, &aliases), aliases)) > 0
}

// isCalcDecl reports whether a declaration is a calc definition or usage, or an
// analysis or verification case, each a calculation (SysML v2 §7.22, §7.23):
// each is invoked the same way.
func isCalcDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefCalc || d.Kind == ast.DefAnalysisCase || d.Kind == ast.DefVerificationCase
	case *ast.Usage:
		return d.Kind == ast.UsageCalc || d.Kind == ast.UsageAnalysisCase || d.Kind == ast.UsageVerificationCase
	default:
		return false
	}
}

// calcKindLabel names what sym declares — a calc, an analysis case or a
// verification case — and how diagnostics about it refer to it.
func calcKindLabel(sym *symbols.Symbol, name string) (kind, label string) {
	switch {
	case IsVerificationCaseSymbol(sym):
		kind = "verification"
	case lower.PerformsSteps(sym.Decl):
		kind = "analysis"
	default:
		kind = "calc"
	}
	return kind, kind + " " + name
}

// isCalcSymbol reports whether sym declares a calc, reading its declaration when
// it has one and its symbol kind otherwise.
func isCalcSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Decl != nil {
		return isCalcDecl(sym.Decl)
	}
	switch sym.Kind {
	case symbols.SymbolCalcDef, symbols.SymbolCalcUsage,
		symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseDef, symbols.SymbolVerificationCaseUsage:
		return true
	}
	return false
}

// isActionSymbol reports whether sym declares an action, reading its declaration
// when it has one and its symbol kind otherwise.
func isActionSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefAction
	case *ast.Usage:
		return d.Kind == ast.UsageAction
	}
	if sym.Decl != nil {
		return false
	}
	return sym.Kind == symbols.SymbolActionDef || sym.Kind == symbols.SymbolActionUsage
}

// isStateSymbol reports whether sym declares a state, reading its declaration when
// it has one and its symbol kind otherwise.
func isStateSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefState
	case *ast.Usage:
		return d.Kind == ast.UsageState
	}
	if sym.Decl != nil {
		return false
	}
	return sym.Kind == symbols.SymbolStateDef || sym.Kind == symbols.SymbolStateUsage
}

// declMembers returns the body members of a definition, usage or named owned
// constraint, unwrapping the Membership wrappers the parser produces.
func declMembers(decl ast.Node) []ast.Node {
	var members []ast.Node
	if oc, ok := ast.OwnedConstraintOf(decl); ok {
		return oc.Body
	}
	switch d := decl.(type) {
	case *ast.Definition:
		members = d.Members
	case *ast.Usage:
		members = d.Members
	default:
		return nil
	}

	out := make([]ast.Node, 0, len(members))
	for _, member := range members {
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		out = append(out, member)
	}
	return out
}

// qualifiedSymbolName renders a symbol's qualified name, falling back to its
// local name when no index is reachable.
func (ctx *Context) qualifiedSymbolName(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if ctx.model.resolver != nil {
		if idx := ctx.model.resolver.Index(); idx != nil {
			if fqn := idx.GetFQN(sym); fqn != "" {
				return fqn
			}
		}
	}
	return sym.Name
}
