package lower

import (
	"iter"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// CalcEnv is what lowering a calc reads beyond the syntax tree: the semantic
// model and resolver, and which library calcs the runtime realizes itself.
type CalcEnv struct {
	Semantics *semantics.Model
	Resolver  *resolve.Resolver
	// Realized reports a library calc the runtime realizes rather than runs as
	// written; it contributes no members to the chain of a calc specializing it.
	Realized func(*symbols.Symbol) bool
}

// qualifiedName renders a symbol's qualified name, falling back to its local
// name when no index is reachable.
func (env CalcEnv) qualifiedName(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if env.Resolver != nil && env.Resolver.Index() != nil {
		if fqn := env.Resolver.Index().GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}

// CalcTrees is what a lowered calc still keeps as syntax trees beside its
// program.Calc: the body's statements, the bindings and the expression table
// the record's ExprIDs index, until expressions are lowered themselves.
type CalcTrees struct {
	// Body is the computation the calc states, lowered in the scope of the calc
	// that declares it: local declarations, assignments, conditionals, loops and
	// returns, in declaration order; BodyOrder is its calculation-statement order.
	Body      []Statement
	BodyOrder *StatementOrder
	// Steps is Body without the bindings of its `out` features, which are
	// evaluated when those features are read rather than run as statements.
	Steps []Statement
	// Nodes are the action nodes the body's flow performs as the steps of a case.
	Nodes    []ast.Node
	Bindings []Binding
	exprs    []ast.Node
}

// Expr is the expression behind an id, nil for none.
func (t *CalcTrees) Expr(id program.ExprID) ast.Node {
	if t == nil || id == 0 || int(id) >= len(t.exprs) {
		return nil
	}
	return t.exprs[id]
}

// intern gives node an id, 0 for nil.
func (t *CalcTrees) intern(node ast.Node) program.ExprID {
	if node == nil {
		return 0
	}
	if len(t.exprs) == 0 {
		t.exprs = make([]ast.Node, 1, 8)
	}
	t.exprs = append(t.exprs, node)
	return program.ExprID(tableIndex(len(t.exprs) - 1))
}

// IsCalcDecl reports whether a declaration is a calc definition or usage, or an
// analysis or verification case, each a calculation (SysML v2 §7.22, §7.23):
// each is invoked the same way.
func IsCalcDecl(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefCalc || d.Kind == ast.DefAnalysisCase || d.Kind == ast.DefVerificationCase
	case *ast.Usage:
		return d.Kind == ast.UsageCalc || d.Kind == ast.UsageAnalysisCase || d.Kind == ast.UsageVerificationCase
	default:
		return false
	}
}

// IsPredicateDecl reports a declaration an invocation applies as a predicate: a
// constraint or requirement definition or usage, an objective among the latter.
func IsPredicateDecl(decl ast.Node) bool {
	if _, ok := ast.OwnedConstraintOf(decl); ok {
		return true
	}
	switch d := decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefConstraint || d.Kind == ast.DefRequirement
	case *ast.Usage:
		return d.Kind == ast.UsageConstraint || d.Kind == ast.UsageRequirement || d.Kind == ast.UsageObjective
	}
	return false
}

// PredicateKind names the kind of predicate sym declares, as verdicts name it.
func PredicateKind(sym *symbols.Symbol) string {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if d.Kind == ast.DefConstraint {
			return "constraint"
		}
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageConstraint:
			return "constraint"
		case ast.UsageObjective:
			return "objective"
		}
	}
	return "requirement"
}

// isVerificationCase reports whether sym declares a verification case, reading
// its declaration when it has one and its symbol kind otherwise.
func isVerificationCase(sym *symbols.Symbol) bool {
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefVerificationCase
	case *ast.Usage:
		return d.Kind == ast.UsageVerificationCase
	}
	if sym.Decl != nil {
		return false
	}
	return sym.Kind == symbols.SymbolVerificationCaseDef || sym.Kind == symbols.SymbolVerificationCaseUsage
}

// CalcKind names what sym declares — a calc, an analysis case or a verification
// case — as diagnostics refer to it.
func CalcKind(sym *symbols.Symbol) string {
	switch {
	case isVerificationCase(sym):
		return "verification"
	case PerformsSteps(sym.Decl):
		return "analysis"
	default:
		return "calc"
	}
}

// UnwrappedDeclMembers is ast.DeclMembers extended to named owned constraints,
// with the Membership wrappers the parser produces unwrapped.
func UnwrappedDeclMembers(decl ast.Node) []ast.Node {
	if body, ok := constraintMemberBody(decl); ok {
		return body
	}
	members := ast.DeclMembers(decl)
	if members == nil {
		return nil
	}
	out := make([]ast.Node, 0, len(members))
	for member := range EachUnwrappedDeclMember(decl) {
		out = append(out, member)
	}
	return out
}

// EachUnwrappedDeclMember yields what UnwrappedDeclMembers lists without building the list.
func EachUnwrappedDeclMember(decl ast.Node) iter.Seq[ast.Node] {
	return func(yield func(ast.Node) bool) {
		if body, ok := constraintMemberBody(decl); ok {
			for _, member := range body {
				if !yield(member) {
					return
				}
			}
			return
		}
		for _, member := range ast.DeclMembers(decl) {
			if !yield(unwrapMembership(member)) {
				return
			}
		}
	}
}

// constraintMemberBody returns the nested members of an assume or require member
// in either of its forms, and false for every other declaration.
func constraintMemberBody(decl ast.Node) ([]ast.Node, bool) {
	switch m := decl.(type) {
	case *ast.AssumeMember:
		return m.Body, true
	case *ast.RequireMember:
		return m.Body, true
	}
	return nil, false
}

// declScope is the scope a symbol's members are declared in: its own, else its owner's.
func declScope(sym *symbols.Symbol) *symbols.Scope {
	if sym == nil {
		return nil
	}
	if sym.Scope != nil {
		return sym.Scope
	}
	return sym.OwnerScope
}

// memberSymbol returns the symbol scope declares for node, anonymous ones
// included, or nil when there is none.
func memberSymbol(scope *symbols.Scope, node ast.Node) *symbols.Symbol {
	if scope == nil || node == nil {
		return nil
	}
	return scope.MemberDeclaring(node)
}

// SubjectDecl is a case's subject as declared: its name, its binding and the node.
type SubjectDecl struct {
	Name  string
	Value ast.Node
	Node  ast.Node
}

// SubjectDeclaration reads a subject declaration, written as a subject member
// (`subject s : T;`, `subject = expr;`) or a subject-kind usage.
func SubjectDeclaration(member ast.Node) (SubjectDecl, bool) {
	switch m := member.(type) {
	case *ast.SubjectMember:
		name, _ := m.EffectiveName()
		return SubjectDecl{Name: name, Value: m.BindingExpr, Node: m}, true
	case *ast.Usage:
		if m.Kind != ast.UsageSubject {
			return SubjectDecl{}, false
		}
		name, _ := ast.EffectiveName(m)
		return SubjectDecl{Name: name, Value: m.Value, Node: m}, true
	}
	return SubjectDecl{}, false
}

// CalcChain returns the calcs sym takes members from (its supertypes and the calc
// it references), most general first, then sym. Non-calc links and the library's
// frame contribute nothing; a domain library's calc contributes as a model's does.
func CalcChain(sym *symbols.Symbol, env CalcEnv) []*symbols.Symbol {
	return memberChain(sym, env, IsCalcDecl)
}

func memberChain(sym *symbols.Symbol, env CalcEnv, declares func(ast.Node) bool) []*symbols.Symbol {
	supers := env.Semantics.MemberSources(sym)
	chain := make([]*symbols.Symbol, 0, len(supers)+1)
	for i := len(supers) - 1; i >= 0; i-- {
		if supers[i] != nil && declares(supers[i].Decl) && (env.Realized == nil || !env.Realized(supers[i])) {
			chain = append(chain, supers[i])
		}
	}
	return append(chain, sym)
}

// calcOrigin names the calc the way a message does.
func calcOrigin(sym *symbols.Symbol, name, kind string) program.Origin {
	var span source.Span
	if sym.Decl != nil {
		span = sym.Decl.Span()
	}
	return program.Origin{
		Doc:  symbols.DocNameOf(declScope(sym)),
		Span: span,
		Path: name,
		Kind: kind,
		Name: name,
	}
}

// ToCalc lowers the invocation interface of the calc sym: its positional input
// parameters (own and inherited, with defaults), its outputs, its body and its
// result, each resolved across the chain. The runtime judges whether the calc
// computes anything from the record; sym is a calc declaration or a feature typed by one.
func ToCalc(sym *symbols.Symbol, env CalcEnv) (*program.Calc, *CalcTrees) {
	name := env.qualifiedName(sym)
	kind := CalcKind(sym)
	chain := CalcChain(sym, env)
	calc := &program.Calc{
		Sym:    sym,
		Origin: calcOrigin(sym, name, kind),
		Name:   name,
		Kind:   kind,
		Label:  kind + " " + name,
		Chain:  chain,
	}
	trees := &CalcTrees{}
	trees.Body, trees.BodyOrder, calc.BodyOwner = calcBodyWithOrder(chain, env.Resolver)
	trees.Steps = CalcSteps(trees.Body)
	trees.Nodes = BlockNodes(trees.Steps)
	calc.StepCount = len(trees.Nodes)
	calc.Params = calcParameters(chain, env, trees, &calc.Aliases)
	calc.Outputs = calcOutputs(chain, env, trees, &calc.Aliases)
	calc.ParamNames = paramNames(calc.Params)
	calc.BodyOutputs = assignedOutputs(trees.Steps, calc.Outputs, calc.Aliases)
	trees.Bindings = calcBindings(chain)
	calc.Result = trees.intern(resultBindingExpr(trees.Bindings))
	calc.Returns = Returns(trees.Body)
	calc.StepsReturn = Returns(trees.Steps)
	return calc, trees
}

// ToPredicate lowers the invocation interface of the predicate sym: its
// parameters, the subject first; its conditions stand for a body.
func ToPredicate(sym *symbols.Symbol, env CalcEnv) (*program.Calc, *CalcTrees) {
	chain := memberChain(sym, env, IsPredicateDecl)
	name := env.qualifiedName(sym)
	if sym.Name == "" {
		// An anonymous usage answers to the name it redefines (`objective : MinimizeObjective;`).
		name += env.Semantics.EffectiveNameOf(sym)
	}
	kind := PredicateKind(sym)
	calc := &program.Calc{
		Sym:    sym,
		Origin: calcOrigin(sym, name, kind),
		Name:   name,
		Kind:   kind,
		Label:  kind + " " + name,
		Chain:  chain,
	}
	trees := &CalcTrees{}
	calc.Params = calcParameters(chain, env, trees, &calc.Aliases)
	calc.ParamNames = paramNames(calc.Params)
	return calc, trees
}

// ToCalcSignature lowers the effective inputs of a model calc whose call applies
// an inherited library function: each input as a call binds it — named as written,
// declared and defaulted by the nearest of its redefinitions the model states —
// with, per parameter, the position of the library input among libInputs it
// redefines, -1 for none. The record is bodiless.
func ToCalcSignature(sym *symbols.Symbol, libInputs []*symbols.Symbol, env CalcEnv) (*program.Calc, *CalcTrees, []int) {
	name := env.qualifiedName(sym)
	kind := CalcKind(sym)
	calc := &program.Calc{
		Sym:    sym,
		Origin: calcOrigin(sym, name, kind),
		Name:   name,
		Kind:   kind,
		Label:  kind + " " + name,
	}
	trees := &CalcTrees{}
	var positions []int
	for _, p := range env.Semantics.BehaviorParametersOf(sym) {
		if p.IsResult || (p.Direction != ast.DirIn && p.Direction != ast.DirInOut) {
			continue
		}
		param, position := effectiveParameter(p.Symbol, libInputs, env, trees)
		calc.Params = append(calc.Params, param)
		positions = append(positions, position)
	}
	calc.ParamNames = paramNames(calc.Params)
	return calc, trees, positions
}

// effectiveParameter is the input parameter sym as a call binds it, with the
// position of the library input it redefines, -1 for none.
func effectiveParameter(sym *symbols.Symbol, libInputs []*symbols.Symbol, env CalcEnv, trees *CalcTrees) (program.CalcParam, int) {
	param := program.CalcParam{Name: sym.Name, Sym: sym, IsCalc: IsCalcUsageSymbol(sym)}
	if effective, _ := ast.EffectiveName(sym.Decl.(*ast.Usage)); effective != "" {
		param.Name = effective
	}
	var dflt ast.Node
	for _, link := range env.Semantics.ParameterRedefinitionChain(sym) {
		owner := link.OwnerScope.Owner()
		// The chain runs nearest first, so each declaration is more general than the recorded ones.
		param.Declarations = slices.Insert(param.Declarations, 0, program.CalcMember{Feature: link, Owner: owner, Name: param.Name})
		if at := slices.Index(libInputs, link); at >= 0 {
			param.Default = trees.intern(dflt)
			return param, at
		}
		if libraryDeclared(link, env) {
			break
		}
		if value := DefaultValueOf(link); dflt == nil && value != nil {
			dflt, param.Owner = value, owner
		}
	}
	param.Default = trees.intern(dflt)
	return param, -1
}

// libraryDeclared reports whether sym was declared by one of the library
// documents rather than by the model under evaluation.
func libraryDeclared(sym *symbols.Symbol, env CalcEnv) bool {
	if env.Resolver == nil {
		return false
	}
	idx := env.Resolver.Index()
	return idx != nil && idx.Library(sym)
}

// DefaultValueOf is the value a feature's declaration binds: a usage's value, a
// subject's binding or an owned constraint's; nil for none.
func DefaultValueOf(featureSym *symbols.Symbol) ast.Node {
	if oc, ok := ast.OwnedConstraintOf(featureSym.Decl); ok {
		return oc.Value
	}
	switch decl := featureSym.Decl.(type) {
	case *ast.Usage:
		return decl.Value
	case *ast.SubjectMember:
		return decl.BindingExpr
	}
	return nil
}

// IsCalcUsageSymbol reports whether sym declares a calc usage or a case usage —
// analysis or verification — the forms that carry an evaluation whose outputs are
// features. A definition is a type: it is invoked, not read.
func IsCalcUsageSymbol(sym *symbols.Symbol) bool {
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

func paramNames(params []program.CalcParam) []string {
	names := make([]string, len(params))
	for i, param := range params {
		names[i] = param.Name
	}
	return names
}

// calcParameters flattens the input parameters declared along chain (most general
// first): a redeclaration keeps its position and default, a renamed one is aliased.
func calcParameters(chain []*symbols.Symbol, env CalcEnv, trees *CalcTrees, aliases *map[string]string) []program.CalcParam {
	var params []program.CalcParam
	index := make(map[string]int)

	for _, link := range chain {
		for _, member := range UnwrappedDeclMembers(link.Decl) {
			if subject, ok := SubjectDeclaration(member); ok {
				params = subjectParameter(params, index, aliases, link, member, subject, env, trees)
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
			actor := usage.Kind == ast.UsageActor
			if !actor && usage.Direction != ast.DirIn && usage.Direction != ast.DirInOut {
				continue
			}
			sym := memberSymbol(declScope(link), usage)
			param := program.CalcParam{
				Name: name, Default: trees.intern(usage.Value), Owner: link,
				Sym: sym, IsActor: actor,
				Declarations: declarationsOf(nil, link, sym, name), IsCalc: IsCalcUsageSymbol(sym),
			}
			if at, seen := redeclaredIndex(index, sym, name, env); seen {
				// A redeclaration binding no value keeps the inherited default,
				// which is written in the scope of the calc that stated it.
				if param.Default == 0 {
					param.Default = params[at].Default
					param.Owner = params[at].Owner
				}
				param.Declarations = declarationsOf(params[at].Declarations, link, sym, name)
				*aliases = program.AliasRedefined(*aliases, params[at].Name, name)
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

// declarationsOf appends the declaration of sym by link, under name, to those it
// redeclares; a member link declares no symbol for states nothing.
func declarationsOf(redeclared []program.CalcMember, link, sym *symbols.Symbol, name string) []program.CalcMember {
	if sym == nil {
		return redeclared
	}
	out := make([]program.CalcMember, len(redeclared), len(redeclared)+1)
	copy(out, redeclared)
	return append(out, program.CalcMember{Feature: sym, Owner: link, Name: name})
}

// redeclaredIndex finds the flattened member the member sym, named name,
// redeclares: the one of its own name, one it redefines (`in y :>> x`), else
// the one its role redefines by position (an actor over the general's actor).
func redeclaredIndex(index map[string]int, sym *symbols.Symbol, name string, env CalcEnv) (int, bool) {
	if at, seen := index[name]; seen {
		return at, true
	}
	for _, redefined := range env.Semantics.RedefinedFeatures(sym) {
		if at, seen := index[redefined.Name]; seen {
			return at, true
		}
	}
	for _, redefined := range env.Semantics.ImplicitRoleRedefinitions(sym) {
		if at, seen := index[redefined.Name]; seen {
			return at, true
		}
	}
	return 0, false
}

// subjectParameter records link's subject among params: refining the subject an
// earlier link declared, else inserting it first, the position a subject binds by.
func subjectParameter(
	params []program.CalcParam, index map[string]int, aliases *map[string]string,
	link *symbols.Symbol, member ast.Node, subject SubjectDecl, env CalcEnv, trees *CalcTrees,
) []program.CalcParam {
	sym := memberSymbol(declScope(link), member)
	param := program.CalcParam{
		Name: subject.Name, Default: trees.intern(subject.Value), Owner: link, IsSubject: true,
		Sym: sym,
	}
	at := -1
	for i := range params {
		if params[i].IsSubject {
			at = i
			break
		}
	}
	if at < 0 {
		if seen, ok := redeclaredIndex(index, sym, subject.Name, env); ok {
			at = seen
		}
	}
	if at >= 0 {
		// A redeclaration binding no value keeps the inherited binding; one
		// declaring no name keeps the inherited name.
		if param.Default == 0 {
			param.Default, param.Owner = params[at].Default, params[at].Owner
		}
		if param.Name == "" {
			param.Name = params[at].Name
		}
		param.Declarations = declarationsOf(params[at].Declarations, link, sym, subject.Name)
		if params[at].Name != param.Name {
			*aliases = program.AliasRedefined(*aliases, params[at].Name, param.Name)
			delete(index, params[at].Name)
		}
		params[at] = param
		index[param.Name] = at
		return params
	}
	param.Declarations = declarationsOf(nil, link, sym, subject.Name)
	if param.Name == "" {
		param.Name = "subject"
	}
	for name, i := range index {
		index[name] = i + 1
	}
	index[param.Name] = 0
	return append([]program.CalcParam{param}, params...)
}

// calcOutputs flattens the output features declared along chain (most general
// first), as calcParameters does the inputs, recording renamed ones in aliases.
func calcOutputs(chain []*symbols.Symbol, env CalcEnv, trees *CalcTrees, aliases *map[string]string) []program.CalcOutput {
	var outs []program.CalcOutput
	index := make(map[string]int)

	for _, link := range chain {
		for _, member := range UnwrappedDeclMembers(link.Decl) {
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
			out := program.CalcOutput{Name: name, Value: trees.intern(usage.Value), Owner: link, IsResult: usage.IsResult,
				IsInitial: usage.Value != nil && usage.ValueIsInitial, Declarations: declarationsOf(nil, link, sym, name)}
			if usage.Direction == ast.DirInOut {
				// The value an `inout` declares is the default of the parameter, not a
				// binding of the output: the invocation binds it either way.
				out.IsInOut, out.Value = true, 0
			}

			var at int
			var seen bool
			if name != "" {
				at, seen = redeclaredIndex(index, sym, name, env)
			} else {
				// An anonymous result parameter has no name to redeclare by, so
				// it refines the anonymous result it inherits.
				at, seen = indexOfAnonymousResult(outs)
			}
			if seen {
				if out.Value == 0 {
					out.Value = outs[at].Value
					out.Owner = outs[at].Owner
					out.IsInitial = outs[at].IsInitial
				}
				out.Declarations = declarationsOf(outs[at].Declarations, link, sym, name)
				if name != "" {
					*aliases = program.AliasRedefined(*aliases, outs[at].Name, name)
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

// indexOfAnonymousResult finds the unnamed result parameter among outs, which is
// the one an unnamed redeclaration refines.
func indexOfAnonymousResult(outs []program.CalcOutput) (int, bool) {
	for i, out := range outs {
		if out.Name == "" {
			return i, true
		}
	}
	return 0, false
}

// assignedOutputs are the outputs the body's statements assign, on any path
// through it: what the calc computes, whichever way an execution branches.
func assignedOutputs(stmts []Statement, outputs []program.CalcOutput, aliases map[string]string) map[string]bool {
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
func collectAssignedOutputs(stmts []Statement, declared map[string]string, assigned map[string]bool) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case Assign:
			if name, ok := declared[s.Target]; ok && s.Chain == nil {
				assigned[name] = true
			}
		case Block:
			collectAssignedOutputs(s.Steps(), declared, assigned)
		case Loop:
			collectAssignedOutputs(s.Body.Steps(), declared, assigned)
		case If:
			collectAssignedOutputs(s.Then.Steps(), declared, assigned)
			if s.Else != nil {
				collectAssignedOutputs(s.Else.Steps(), declared, assigned)
			}
		}
	}
}

// calcBodyWithOrder returns the computation the invoked calc runs — its own body
// if that states one, otherwise the closest inherited one — with its statement
// order and the calc that declares it, whose scope the body's statements are written in.
func calcBodyWithOrder(chain []*symbols.Symbol, resolver *resolve.Resolver) ([]Statement, *StatementOrder, *symbols.Symbol) {
	var stated []Statement
	var statedOrder *StatementOrder
	var owner *symbols.Symbol
	for i := len(chain) - 1; i >= 0; i-- {
		link := chain[i]
		stmts, order := CalcBodyWithOrder(link.Decl, UnwrappedDeclMembers(link.Decl), link.Scope, resolver)
		if Returns(stmts) {
			return stmts, order, link
		}
		// A body that computes but returns nothing leaves an inherited result in
		// force, so keep looking up the chain before settling for it.
		if stated == nil && len(stmts) > 0 {
			stated, statedOrder, owner = stmts, order, link
		}
	}
	return stated, statedOrder, owner
}

func calcBindings(chain []*symbols.Symbol) []Binding {
	var out []Binding
	for _, link := range chain {
		if link != nil {
			out = append(out, ToBindings(link.Decl, declScope(link))...)
		}
	}
	return out
}

// resultBindingExpr is the expression a binding connector binds to `result`, nil for none.
func resultBindingExpr(bindings []Binding) ast.Node {
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

// CalcStatesComputation reports whether a calc chain states a computation: a body
// that returns or assigns an output, or a binding of the result.
func CalcStatesComputation(chain []*symbols.Symbol, env CalcEnv) bool {
	body, _, _ := calcBodyWithOrder(chain, env.Resolver)
	if Returns(body) || resultBindingExpr(calcBindings(chain)) != nil {
		return true
	}
	var aliases map[string]string
	trees := &CalcTrees{}
	return len(assignedOutputs(CalcSteps(body), calcOutputs(chain, env, trees, &aliases), aliases)) > 0
}

// UnboundResultHint explains a `return` that declares a result parameter without
// binding it (`return h;` declares h, it does not return the member h).
func UnboundResultHint(chain []*symbols.Symbol) string {
	for i := len(chain) - 1; i >= 0; i-- {
		if chain[i] == nil {
			continue
		}
		members := UnwrappedDeclMembers(chain[i].Decl)
		result := unboundResultParameter(members)
		if result == nil {
			continue
		}
		name, _ := ast.EffectiveName(result)
		who, trailing, expr := "the result parameter", "of the body", "<expr>"
		typ := TypeText(result)
		if name != "" {
			spelled := source.NameText(name)
			who = "result parameter " + spelled
			if sibling := valuedMemberNamed(members, name, result); sibling != nil {
				trailing, expr = "`"+spelled+"`", spelled
				if typ == "" {
					typ = TypeText(sibling)
				}
			}
		}
		if typ == "" {
			typ = "<type>"
		}
		return ": " + who + " binds no value; write the result as the trailing expression " + trailing +
			", or bind it with `return : " + typ + " = " + expr + ";`"
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
