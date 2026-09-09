package runtime

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// subjectDecl is a case's subject as its body declares it: the object the case
// is about, which binds as the case's first input parameter (SysML v2 §7.21).
type subjectDecl struct {
	Name  string
	Value ast.Node
	Node  ast.Node
}

// subjectDeclaration reads a subject declaration, written as a subject member
// (`subject s : T;`, `subject = expr;`) or a subject-kind usage.
func subjectDeclaration(member ast.Node) (subjectDecl, bool) {
	switch m := member.(type) {
	case *ast.SubjectMember:
		name, _ := m.EffectiveName()
		return subjectDecl{Name: name, Value: m.BindingExpr, Node: m}, true
	case *ast.Usage:
		if m.Kind != ast.UsageSubject {
			return subjectDecl{}, false
		}
		name, _ := ast.EffectiveName(m)
		return subjectDecl{Name: name, Value: m.Value, Node: m}, true
	}
	return subjectDecl{}, false
}

// subjectParameter records link's subject among params: refining the subject an
// earlier link declared, else inserting it first, the position a subject binds by.
func (ctx *Context) subjectParameter(
	params []calcParameter, index map[string]int, aliases *map[string]string,
	link *symbols.Symbol, member ast.Node, subject subjectDecl,
) []calcParameter {
	sym := memberSymbol(declScope(link), member)
	param := calcParameter{
		Name: subject.Name, Default: subject.Value, Owner: link, IsSubject: true,
		Decl: ctx.calcMemberDeclOf(link, sym, subject.Name),
	}
	at := -1
	for i := range params {
		if params[i].IsSubject {
			at = i
			break
		}
	}
	if at < 0 {
		if seen, ok := ctx.redeclaredIndex(index, sym, subject.Name); ok {
			at = seen
		}
	}
	if at >= 0 {
		// A redeclaration binding no value keeps the inherited binding; one
		// declaring no name keeps the inherited name.
		if param.Default == nil {
			param.Default, param.Owner = params[at].Default, params[at].Owner
		}
		if param.Name == "" {
			param.Name = params[at].Name
		}
		param.Decl = param.Decl.redeclaring(params[at].Decl)
		if params[at].Name != param.Name {
			*aliases = aliasRedefined(*aliases, params[at].Name, param.Name)
			delete(index, params[at].Name)
		}
		params[at] = param
		index[param.Name] = at
		return params
	}
	if param.Name == "" {
		param.Name = "subject"
	}
	for name, i := range index {
		index[name] = i + 1
	}
	index[param.Name] = 0
	return append([]calcParameter{param}, params...)
}

// subjectParameter is the parameter the case's subject binds to, if it declares one.
func (shape *calcShape) subjectParameter() (*calcParameter, bool) {
	for i := range shape.Params {
		if shape.Params[i].IsSubject {
			return &shape.Params[i], true
		}
	}
	return nil, false
}

// enclosingSubject is the subject of the case whose body declares shape's usage,
// which a nested case binding no subject of its own takes (SysML v2 §7.21.2):
// read from the environment of the evaluation reading the usage.
func (ctx *Context) enclosingSubject(shape *calcShape, enclosing *EvalContext) (Value, bool) {
	if enclosing == nil {
		return Value{}, false
	}
	owner := enclosingBehavior(shape.Sym)
	if owner == nil || !isCalcSymbol(owner) {
		return Value{}, false
	}
	outer, err := ctx.calcShapeOf(owner)
	if err != nil {
		return Value{}, false
	}
	subject, ok := outer.subjectParameter()
	if !ok {
		return Value{}, false
	}
	return enclosing.Lookup(subject.Name)
}

// enclosingBehavior is the behavior whose body, directly or through body-local
// blocks, declares sym; nil for a member of a part or a package.
func enclosingBehavior(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	for scope := sym.OwnerScope; scope != nil; scope = scope.Parent() {
		if owner := scope.Owner(); owner != nil {
			return owner
		}
		if !scope.BodyLocal() {
			return nil
		}
	}
	return nil
}

// unboundSubject reports a case run with no object as its subject.
func (shape *calcShape) unboundSubject(param *calcParameter) error {
	return &UnboundSubjectError{Kind: shape.Kind, Element: shape.Name, Subject: param.Name}
}

// AnalysisArgs are the values a run of an analysis case supplies: an object as
// its subject, and arguments for its input parameters by position or by name.
type AnalysisArgs struct {
	// Subject is the object the case is run on; nil leaves the case's own
	// subject binding, or the enclosing case's subject, to supply it.
	Subject *Instance

	// Positional bind the input parameters the case does not bind itself, in
	// declaration order; the subject is never among them.
	Positional []Value

	// Named bind parameters by name.
	Named map[string]Value
}

// AnalysisVerdict is what one check of an analysis case decided after its body
// ran: an objective's required conditions, or an assertion in its body.
type AnalysisVerdict struct {
	// Kind is "objective" or "assertion".
	Kind string

	// Name names the objective or the asserted constraint usage, or spells an
	// anonymous assertion's condition as written.
	Name string

	// Symbol declares the objective or the named constraint asserted; nil for an
	// anonymous assertion.
	Symbol *symbols.Symbol

	// Status is what the check decided.
	Status VerdictStatus

	// Detail is the violated condition of a failed check, or why an undecided
	// one could not be evaluated; empty for a satisfied one.
	Detail string
}

// VerdictStatus is what a check of a case decided.
type VerdictStatus int

const (
	// VerdictSatisfied is a check whose required conditions all held.
	VerdictSatisfied VerdictStatus = iota
	// VerdictNotSatisfied is a check with a required condition that did not hold.
	VerdictNotSatisfied
	// VerdictUndecided is a check that could not be evaluated.
	VerdictUndecided
)

// String names the status as a report words it.
func (s VerdictStatus) String() string {
	switch s {
	case VerdictSatisfied:
		return "satisfied"
	case VerdictNotSatisfied:
		return "not satisfied"
	default:
		return "undecided"
	}
}

// AnalysisEvaluation is one application, during a case's run, of a calc held as
// a function value: a trade study's evaluation of one alternative.
type AnalysisEvaluation struct {
	// Function is the qualified name of the calc applied.
	Function string

	// Arguments are what the calc was applied to, by parameter position, a named
	// one at its parameter's; null where none was given before a later one.
	Arguments []Value

	// Result is what the calc computed; unset when Error says why it computed nothing.
	Result Value
	Error  error

	// Selected marks the evaluation of the value `selectOne` picked and the case
	// returned. Tied marks one computing what the selected did, passed over for it.
	Selected bool
	Tied     bool
}

// AnalysisResult is what one run of an analysis case produced: its output
// values in declaration order, and the verdict of each objective and assertion.
type AnalysisResult struct {
	// Case is the qualified name of the case that ran.
	Case string

	// Subject is the object the case ran on — supplied, bound by the usage or
	// taken from the enclosing case; nil for a case declaring no subject.
	Subject *Instance

	// Outputs are the case's out and return parameters, in declaration order;
	// a value the body returned into an unnamed result is named "result".
	Outputs []CalcOutputValue

	// Verdicts are the case's objectives, in order, then its assertions.
	Verdicts []AnalysisVerdict

	// Evaluations are the applications of the case's own calcs as function values
	// the run made, in the order first made, once per distinct argument list.
	Evaluations []AnalysisEvaluation
}

// RunAnalysis runs an analysis case — a definition or a usage — binding its
// subject and input parameters from args and its own declarations, and reports
// every output it declares together with the verdict of each objective and
// assertion. self, when non-null, is the object a usage is a feature of.
// A usage run with no arguments answers from the same evaluation a read of
// its outputs does, so both report the same values. A run whose output could
// not be evaluated fails, and reports beside the error what it did establish:
// the evaluations it made and the verdicts, undecided where they read that output.
func (ctx *Context) RunAnalysis(sym *symbols.Symbol, args AnalysisArgs, scope *symbols.Scope, self *Instance) (AnalysisResult, error) {
	defer ctx.beginRun()()

	_, result, err := ctx.runCase(sym, args, scope, self)
	return result, err
}

// runCase runs a case's body once and reports the run beside what it produced,
// so a caller reading more of the run than its outputs — a verification case's
// subcase verdicts — reads it from the same run.
func (ctx *Context) runCase(sym *symbols.Symbol, args AnalysisArgs, scope *symbols.Scope, self *Instance) (*calcRun, AnalysisResult, error) {
	if err := ctx.RequireAnalysisCase(sym); err != nil {
		return nil, AnalysisResult{}, err
	}
	if err := ctx.checkCalcTyping(sym); err != nil {
		return nil, AnalysisResult{}, err
	}
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		return nil, AnalysisResult{}, err
	}
	reader := NewEvalContextIn(ctx, scope, self)
	log := ctx.beginEvaluationLog(sym)
	defer ctx.endEvaluationLog(log)

	var run *calcRun
	if args.Subject == nil && len(args.Positional) == 0 && len(args.Named) == 0 && isCalcUsageSymbol(sym) {
		run, err = ctx.calcUsageRun(reader, sym)
	} else {
		run, err = ctx.analysisRun(shape, reader, args)
	}
	if err != nil {
		result := AnalysisResult{Case: shape.Name, Evaluations: log.evaluations(Value{}, false)}
		result.Verdicts = ctx.undecidedVerdicts(sym, scope, err)
		return nil, result, err
	}

	// The outputs computed before one failed stay reported; the verdicts and the
	// pick do not, since the case established neither.
	result := AnalysisResult{Case: shape.Name, Subject: run.boundSubject(ctx)}
	outputs, err := run.outputValues(ctx)
	result.Outputs = outputs
	if err != nil {
		result.Verdicts = ctx.undecidedVerdicts(sym, scope, err)
		result.Evaluations = log.evaluations(Value{}, false)
		return nil, result, err
	}
	result.Verdicts = ctx.analysisVerdicts(run, sym, scope)
	result.Evaluations = log.evaluations(run.caseResult(run.bindingsFrame(ctx).vars))
	return run, result, nil
}

// undecidedVerdicts leaves every objective and required assertion of a case
// whose body failed undecided, the failure as the reason.
func (ctx *Context) undecidedVerdicts(sym *symbols.Symbol, scope *symbols.Scope, cause error) []AnalysisVerdict {
	var verdicts []AnalysisVerdict
	for _, obj := range ctx.ObjectivesOf(sym, scope) {
		name := obj.Name
		if name == "" {
			name = "objective"
		}
		verdicts = append(verdicts, AnalysisVerdict{
			Kind: "objective", Name: name, Symbol: obj.Symbol,
			Status: VerdictUndecided, Detail: cause.Error(),
		})
	}
	for _, cond := range ctx.CaseConditionsOf(sym, scope) {
		if !cond.Required {
			continue
		}
		name, named := assertionName(cond)
		verdicts = append(verdicts, AnalysisVerdict{
			Kind: "assertion", Name: name, Symbol: named,
			Status: VerdictUndecided, Detail: cause.Error(),
		})
	}
	return verdicts
}

// analysisRun binds a case's parameters from args and runs its body once,
// unmemoized: arguments make it an invocation of its own, not the evaluation
// the case's outputs answer from when read as features.
func (ctx *Context) analysisRun(shape *calcShape, reader *EvalContext, args AnalysisArgs) (*calcRun, error) {
	if err := ctx.enterCalc(shape.Name); err != nil {
		return nil, err
	}
	defer ctx.leaveCalc()

	calcArgs, err := shape.analysisArgs(args)
	if err != nil {
		return nil, err
	}
	key := calcUsageKey{sym: shape.Sym}
	if reader.self != nil {
		key.instance = reader.self.ID
	}
	leave, err := ctx.enterCalcUsage(shape, key)
	if err != nil {
		return nil, err
	}
	defer leave()
	ec, nested, env, err := ctx.bindCalcUsage(shape, reader, calcArgs)
	if err != nil {
		return nil, err
	}
	return ctx.runCalcUsage(shape, ec, nested, env, reader)
}

// analysisArgs spells the run's arguments as bindings by parameter name: the
// supplied subject binds the subject parameter, and positional arguments bind
// the inputs in declaration order — a defaulted one included, as a calc
// invocation binds them; the subject is an object, so only Subject or a named
// argument binds it.
func (shape *calcShape) analysisArgs(args AnalysisArgs) (calcArgs, error) {
	named := make(map[string]Value, len(args.Named)+len(args.Positional)+1)
	for name, value := range args.Named {
		if !shape.hasParameter(name) {
			return calcArgs{}, fmt.Errorf("%w: %s has no input parameter %q", ErrUnknownParameter, shape.Label, name)
		}
		named[name] = value
	}
	if args.Subject != nil {
		subject, ok := shape.subjectParameter()
		if !ok {
			return calcArgs{}, fmt.Errorf("%w: %s declares no subject to bind an object to",
				ErrUnknownParameter, shape.Label)
		}
		named[subject.Name] = Value{Kind: ValInstance, Instance: args.Subject.ID}
	}
	open := make([]*calcParameter, 0, len(shape.Params))
	for i := range shape.Params {
		param := &shape.Params[i]
		if _, bound := named[param.Name]; bound || param.IsSubject {
			continue
		}
		open = append(open, param)
	}
	if len(args.Positional) > len(open) {
		return calcArgs{}, fmt.Errorf("%w: %s takes %d argument(s), got %d",
			ErrCalcArity, shape.Label, len(open), len(args.Positional))
	}
	for i, value := range args.Positional {
		named[open[i].Name] = value
	}
	return calcArgs{named: named}, nil
}

// boundSubject is the object the run's subject parameter holds, nil when the
// case declares none or the binding names no object.
func (run *calcRun) boundSubject(ctx *Context) *Instance {
	param, ok := run.shape.subjectParameter()
	if !ok {
		return nil
	}
	value, ok := run.env.lookup(param.Name)
	if !ok || value.Kind != ValInstance {
		return nil
	}
	if inst, ok := ctx.Instance(value.Instance); ok {
		return inst
	}
	return nil
}

// outputValues evaluates every output the case declares, in declaration order,
// naming a value the body returned into an unnamed result "result". An output
// that cannot be evaluated fails the run; the ones before it are reported.
func (run *calcRun) outputValues(ctx *Context) ([]CalcOutputValue, error) {
	values := make([]CalcOutputValue, 0, len(run.shape.Outputs)+1)
	for _, out := range run.shape.Outputs {
		if out.Name == "" {
			if run.returned {
				values = append(values, CalcOutputValue{Name: resultOutputName, Value: run.result})
			}
			continue
		}
		value, err := run.output(ctx, out.Name)
		if err != nil {
			return values, err
		}
		values = append(values, CalcOutputValue{Name: out.Name, Value: value})
	}
	if len(run.shape.Outputs) == 0 && run.returned {
		values = append(values, CalcOutputValue{Name: resultOutputName, Value: run.result})
	}
	return values, nil
}

// analysisVerdicts checks the case's objectives and assertions against the
// values its run bound: its parameters, its locals and its outputs.
func (ctx *Context) analysisVerdicts(run *calcRun, sym *symbols.Symbol, scope *symbols.Scope) []AnalysisVerdict {
	bindings := run.bindingsFrame(ctx)
	var verdicts []AnalysisVerdict
	for _, obj := range ctx.ObjectivesOf(sym, scope) {
		name := obj.Name
		if name == "" {
			name = "objective"
		}
		var verdict AnalysisVerdict
		if own, err := ctx.objectiveBindings(run, obj.Symbol, name, bindings); err != nil {
			verdict = AnalysisVerdict{Kind: "objective", Name: name, Status: VerdictUndecided, Detail: err.Error()}
		} else {
			check := conditionCheck{
				sym: obj.Symbol, kind: "objective", what: "require condition",
				element: name, self: run.self, bindings: own,
			}
			conds := append(slices.Clone(obj.LibraryConditions), obj.Conditions...)
			verdict = ctx.analysisVerdict("objective", name, check, conds)
		}
		verdict.Symbol = obj.Symbol
		verdicts = append(verdicts, verdict)
	}
	for _, cond := range ctx.CaseConditionsOf(sym, scope) {
		if !cond.Required {
			continue
		}
		check := conditionCheck{
			sym: sym, kind: run.shape.Kind, what: "assertion",
			element: run.shape.Name, self: run.self, bindings: bindings,
		}
		name, named := assertionName(cond)
		verdict := ctx.analysisVerdict("assertion", name, check, []Condition{cond})
		verdict.Symbol = named
		verdicts = append(verdicts, verdict)
	}
	return verdicts
}

// objectiveBindings are the case run's bindings plus the subject and actors the objective
// binds itself, as a requirement's are; a subject left unbound is the case's result
// (Cases::Case::obj). The frame stays the case's, so `Case::result` reads the run's.
func (ctx *Context) objectiveBindings(run *calcRun, obj *symbols.Symbol, name string, caseBindings frame) (frame, error) {
	members := ctx.chainMembers(obj, obj.OwnerScope)
	own, err := ctx.memberBindings(obj, "objective", name, members, run.self, nil, caseBindings)
	if err != nil {
		return frame{}, err
	}
	bindings := make(map[string]Value, len(caseBindings.vars)+len(own))
	for k, v := range caseBindings.vars {
		bindings[k] = v
	}
	for k, v := range own {
		bindings[k] = v
	}
	subject, decl, unbound := ctx.unboundObjectiveSubject(obj, members, own)
	if subject == nil {
		return caseBindings.withVars(bindings), nil
	}
	result, ok := run.caseResult(caseBindings.vars)
	if !ok {
		return frame{}, &UnboundSubjectError{Kind: "objective", Element: name, Subject: subject.Name}
	}
	what := fmt.Sprintf("objective %s: subject %s defaults to the case's result (Cases::Case::obj)", name, subject.Name)
	if err := ctx.holdAs(declScope(obj), what, decl, result, subject); err != nil {
		return frame{}, err
	}
	for unboundName := range unbound {
		bindings[unboundName] = result
	}
	return caseBindings.withVars(bindings), nil
}

// unboundObjectiveSubject is the objective's subject no binding the model writes supplies, its
// declaration folded along the chain (a redeclaration keeps what it omits) and its names; nil when bound.
func (ctx *Context) unboundObjectiveSubject(obj *symbols.Symbol, members []scopedMember, own map[string]Value) (*symbols.Symbol, calcMemberDecl, map[string]bool) {
	features := ctx.conditionFeatures(obj)
	var subject *symbols.Symbol
	var decl calcMemberDecl
	unbound := make(map[string]bool)
	for _, member := range members {
		declared, ok := subjectDeclaration(member.node)
		if !ok {
			continue
		}
		sym := memberSymbol(member.scope, member.node)
		if sym == nil {
			continue
		}
		names := ctx.memberNames(obj, member, declared.Name, sym.ShortName)
		if _, bound := boundUnder(own, names); bound || declared.Value != nil {
			return nil, calcMemberDecl{}, nil
		}
		for _, n := range names {
			if feat, ok := features[n]; ok && feat.expr != nil && !ctx.libraryDeclared(feat.decl) {
				return nil, calcMemberDecl{}, nil
			}
			unbound[n] = true
		}
		if subject == nil || ctx.extractType(sym) != nil {
			subject = sym
		}
		decl = ctx.calcMemberDeclFor(obj, sym, subject.Name).redeclaring(decl)
	}
	return subject, decl, unbound
}

// caseResult is the value the run's result parameter holds; false when the case returns none.
func (run *calcRun) caseResult(bindings map[string]Value) (Value, bool) {
	value, ok := bindings[run.resultName()]
	return value, ok
}

// resultName is the name the run's result reads under: the one the case's result
// parameter declares, or `result` for a result it leaves unnamed.
func (run *calcRun) resultName() string {
	if out := run.shape.resultOutput(); out != nil && out.Name != "" {
		return out.Name
	}
	return resultOutputName
}

// analysisChecks reports whether the case states an objective or asserts a
// condition, so running it decides a verdict even when it computes no output.
func (ctx *Context) analysisChecks(sym *symbols.Symbol) bool {
	if len(ctx.ObjectivesOf(sym, nil)) > 0 {
		return true
	}
	for _, cond := range ctx.CaseConditionsOf(sym, nil) {
		if cond.Required {
			return true
		}
	}
	return false
}

// assertionName names an asserted condition: the named constraint stating it, or
// the condition as written when only a case (the one run, or its definition)
// names it. The symbol is the named constraint, nil when there is none.
func assertionName(cond Condition) (string, *symbols.Symbol) {
	for i := len(cond.Constraints) - 1; i >= 0; i-- {
		if c := cond.Constraints[i]; c.Name != "" {
			return c.Name, c
		}
	}
	if owner := cond.Owner(); owner != nil && !IsRunnableCaseSymbol(owner) {
		return owner.Name, owner
	}
	return cond.Label(), nil
}

// analysisVerdict evaluates one check's conditions and words what it decided.
func (ctx *Context) analysisVerdict(kind, name string, check conditionCheck, conds []Condition) AnalysisVerdict {
	verdict := AnalysisVerdict{Kind: kind, Name: name}
	if len(conds) == 0 {
		verdict.Status, verdict.Detail = VerdictUndecided, "states no condition to check"
		return verdict
	}
	holds, err := ctx.evaluateConditions(check, conds)
	var violation *ViolationError
	switch {
	case err == nil && holds:
		verdict.Status = VerdictSatisfied
	case errors.As(err, &violation):
		verdict.Status, verdict.Detail = VerdictNotSatisfied, violation.Condition
	case err != nil:
		verdict.Status, verdict.Detail = VerdictUndecided, err.Error()
	default:
		verdict.Status = VerdictNotSatisfied
	}
	return verdict
}

// bindingsFrame is the run's bindings as a frame the case owns, so a condition reads
// its features by qualified name (`MassCase::result`) and its steps' pins (`step.out`).
func (run *calcRun) bindingsFrame(ctx *Context) frame {
	return frame{vars: run.bindings(ctx), perf: run.perf, owner: run.shape, run: run.env.run}
}

// bindings are the values a run bound, by name: its parameters and locals, and
// the outputs it can evaluate, so a condition over the case reads them.
func (run *calcRun) bindings(ctx *Context) map[string]Value {
	bindings := make(map[string]Value, run.env.width()+len(run.shape.Outputs))
	run.env.each(func(name string, value Value) { bindings[name] = value })
	for _, out := range run.shape.Outputs {
		if out.Name == "" {
			continue
		}
		if value, err := run.output(ctx, out.Name); err == nil {
			bindings[out.Name] = value
		}
	}
	if run.returned {
		if out, err := run.shape.designatedOutput(); err != nil || out.Name == "" {
			bindings[resultOutputName] = run.result
		}
	}
	return bindings
}

// IsRunnableCaseSymbol reports whether sym declares a case whose body runs as
// steps: an analysis case or a verification case, definition or usage.
func IsRunnableCaseSymbol(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Decl != nil {
		return lower.PerformsSteps(sym.Decl)
	}
	switch sym.Kind {
	case symbols.SymbolAnalysisCaseDef, symbols.SymbolAnalysisCaseUsage,
		symbols.SymbolVerificationCaseDef, symbols.SymbolVerificationCaseUsage:
		return true
	}
	return false
}

// RequireAnalysisCase reports ErrNotAnAnalysis for a symbol that is not a case
// whose body runs — an analysis or verification case definition or usage —
// describing what it is instead.
func (ctx *Context) RequireAnalysisCase(sym *symbols.Symbol) error {
	if sym == nil {
		return fmt.Errorf("%w: invalid symbol", ErrNotAnAnalysis)
	}
	if !IsRunnableCaseSymbol(sym) {
		return fmt.Errorf("%w: %s is %s, not an analysis or verification case definition or usage",
			ErrNotAnAnalysis, ctx.qualifiedSymbolName(sym), describeDecl(sym.Decl))
	}
	return nil
}
