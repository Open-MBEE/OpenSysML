package runtime

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Qualified names of the trade-study objective definitions whose specialization
// states which way an objective's value is to be improved (SysML v2 §8.3.22.4).
const (
	minimizeObjectiveFQN = "TradeStudies::MinimizeObjective"
	maximizeObjectiveFQN = "TradeStudies::MaximizeObjective"
)

// An objective states the value it improves by redefining the trade-study
// library's `eval` calculation, from which the library derives its bound `best`.
const (
	tradeStudyObjectiveFQN = "TradeStudies::TradeStudyObjective"
	objectiveEvalName      = "eval"
	objectiveBestName      = "best"
)

// ObjectiveDirection is the way an objective's value is to be improved, taken
// from the trade-study objective definition it is typed by.
type ObjectiveDirection int

const (
	// NoDirection is an objective specializing neither MinimizeObjective nor
	// MaximizeObjective, whose direction the model therefore does not state.
	NoDirection ObjectiveDirection = iota
	// Minimize is an objective specializing TradeStudies::MinimizeObjective.
	Minimize
	// Maximize is an objective specializing TradeStudies::MaximizeObjective.
	Maximize
)

// String names the direction as the model states it.
func (d ObjectiveDirection) String() string {
	switch d {
	case Minimize:
		return "minimize"
	case Maximize:
		return "maximize"
	default:
		return "unstated"
	}
}

// Objective is one objective an analysis case states: which way its value is to
// be improved, the expression stating that value, and the conditions it states
// itself.
type Objective struct {
	// Name is the objective's name, empty for an anonymous one.
	Name string

	// Symbol is the objective usage's symbol.
	Symbol *symbols.Symbol

	// Type is the objective definition it is typed by, nil when untyped.
	Type *symbols.Symbol

	// Direction is the way its value is to be improved.
	Direction ObjectiveDirection

	// Value is the expression stating the value to improve, nil when the
	// objective states none.
	Value ast.Node

	// Scope is where Value's names resolve.
	Scope *symbols.Scope

	// Eval is the calc redefining the library's `eval` that states Value, nil
	// when the value is stated another way or not at all.
	Eval *symbols.Symbol

	// StepwiseEval reports an `eval` whose body computes in steps rather than
	// stating one expression, so no Value is read from it.
	StepwiseEval bool

	// Best is the objective's `best` feature, derived by the library from Eval;
	// nil when the objective is no trade-study objective.
	Best *symbols.Symbol

	// ReboundBest is a feature giving the library's bound `best` a value of its
	// own (`attribute :>> best = expression;`), which validation rejects; or nil.
	ReboundBest *symbols.Symbol

	// Conditions are the conditions the objective states itself, its own body's
	// and the ones it inherits from the model's own objective definitions: what
	// values are feasible, which a solver translates.
	Conditions []Condition

	// LibraryConditions are the conditions the objective inherits from the
	// trade-study library (`eval(selectedAlternative) == best`): about the
	// choice among the listed alternatives, which the analysis run checks and
	// no solver translates.
	LibraryConditions []Condition
}

// Text renders the expression stating the objective's value as written, empty
// when it states none.
func (o Objective) Text() string {
	if o.Value == nil {
		return ""
	}
	return conditionText(o.Value)
}

// RequireAnalysis returns an ErrNotAnAnalysis usage error unless sym declares an
// analysis case, so a caller can settle the kind before asking for objectives.
func RequireAnalysis(sym *symbols.Symbol) error {
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind == ast.DefAnalysisCase {
			return nil
		}
	case *ast.Usage:
		if decl.Kind == ast.UsageAnalysisCase {
			return nil
		}
	}
	return notOfKind(ErrNotAnAnalysis, sym, "analysis case")
}

// ObjectivesOf returns the objectives sym states, its inherited ones first and
// in declaration order. An objective restating an inherited one, by name or by
// redefinition, is the same objective declared again and keeps its place.
func (ctx *Context) ObjectivesOf(sym *symbols.Symbol, scope *symbols.Scope) []Objective {
	if sym == nil {
		return nil
	}
	if scope == nil {
		scope = sym.OwnerScope
	}
	var out []Objective
	for _, member := range ctx.chainMembers(sym, scope) {
		usage, ok := member.node.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageObjective {
			continue
		}
		objSym := memberSymbol(member.scope, member.node)
		if objSym == nil {
			continue
		}
		out = ctx.placeObjective(out, ctx.objectiveOf(objSym, sym))
	}
	return out
}

// placeObjective adds obj to out: in place of the first objective it restates,
// nowhere when it is already placed or redefined by a placed one, at the end otherwise.
func (ctx *Context) placeObjective(out []Objective, obj Objective) []Objective {
	for i, prev := range out {
		if prev.Symbol == obj.Symbol || ctx.redefines(prev, obj) {
			return out
		}
		if ctx.redefines(obj, prev) || (obj.Name != "" && obj.Name == prev.Name) {
			out[i] = obj
			return out
		}
	}
	return append(out, obj)
}

// redefines reports whether obj redefines prev, by clause, position or role.
func (ctx *Context) redefines(obj, prev Objective) bool {
	return slices.Contains(ctx.model.AllRedefinedFeatures(obj.Symbol), prev.Symbol)
}

// objectiveOf reads one objective usage: its direction from the definition it is
// typed by, its value from the `eval` calculation it redefines, and its
// conditions from its own body. owner is the case stating it, which is where a
// value it takes from a redeclared objective is looked for.
func (ctx *Context) objectiveOf(objSym, owner *symbols.Symbol) Objective {
	typ := ctx.extractType(objSym)
	obj := Objective{
		Name:      ctx.model.EffectiveNameOf(objSym),
		Symbol:    objSym,
		Type:      typ,
		Direction: ctx.objectiveDirection(typ),
		Scope:     objSym.OwnerScope,
	}
	obj.Conditions, obj.LibraryConditions = ctx.objectiveConditionsOf(objSym)
	tradeStudy := ctx.specializesLibraryType(typ, tradeStudyObjectiveFQN)
	if tradeStudy {
		obj.ReboundBest = ctx.reboundBestOf(objSym)
		if best, ok := ctx.model.LookupMember(objSym, objectiveBestName); ok {
			obj.Best = best
		}
	}
	if obj.Value = ctx.extractDefaultValue(objSym); obj.Value != nil {
		return obj
	}
	if tradeStudy && ctx.readEvalValue(&obj, objSym) {
		return obj
	}
	if value, decl := ctx.redefinedDefault(objSym, owner); value != nil {
		obj.Value, obj.Scope = value, decl.OwnerScope
		return obj
	}
	// An objective restating another takes the value the restated one states.
	for _, restated := range ctx.restatedObjectives(objSym, owner) {
		if obj.ReboundBest == nil {
			obj.ReboundBest = ctx.reboundBestOf(restated)
		}
		if tradeStudy && ctx.readEvalValue(&obj, restated) {
			return obj
		}
	}
	return obj
}

// restatedObjectives returns what objSym restates as seen from owner, nearest
// first: its clause's targets, then positional and role redefinitions, transitively.
func (ctx *Context) restatedObjectives(objSym, owner *symbols.Symbol) []*symbols.Symbol {
	seen := map[*symbols.Symbol]bool{objSym: true}
	var out []*symbols.Symbol
	add := func(syms []*symbols.Symbol) {
		for _, sym := range syms {
			if !seen[sym] {
				seen[sym] = true
				out = append(out, sym)
			}
		}
	}
	add(ctx.relatedFeatures(objSym, owner, ast.RelRedefines))
	for i := 0; i < len(out); i++ {
		add(ctx.model.AllRedefinedFeatures(out[i]))
	}
	add(ctx.model.AllRedefinedFeatures(objSym))
	return out
}

// readEvalValue reads the objective's value from the lowered body of its own
// `eval`: one returned expression, or a stepwise mark. False when it states none.
func (ctx *Context) readEvalValue(obj *Objective, objSym *symbols.Symbol) bool {
	evalSym := ctx.objectiveMember(objSym, objectiveEvalName)
	if evalSym == nil || evalSym.Kind != symbols.SymbolCalcUsage {
		return false
	}
	obj.Eval = evalSym
	body := bodyScope(evalSym, evalSym.OwnerScope)
	stmts := lower.CalcBody(evalSym.Decl, declMembers(evalSym.Decl), body)
	if len(stmts) == 1 {
		if ret, ok := stmts[0].(lower.Return); ok && ret.Value != nil {
			obj.Value, obj.Scope = ret.Value, ret.Scope
			return true
		}
	}
	obj.StepwiseEval = len(stmts) > 0
	return obj.StepwiseEval
}

// reboundBestOf returns the member of objSym's own body giving the library's
// `best` a value of its own, nil when there is none.
func (ctx *Context) reboundBestOf(objSym *symbols.Symbol) *symbols.Symbol {
	best := ctx.objectiveMember(objSym, objectiveBestName)
	if best == nil || ctx.extractDefaultValue(best) == nil {
		return nil
	}
	return best
}

// objectiveMember returns the member of objSym's own body redefining the
// trade-study feature of the given name, or nil.
func (ctx *Context) objectiveMember(objSym *symbols.Symbol, name string) *symbols.Symbol {
	if objSym == nil {
		return nil
	}
	body := bodyScope(objSym, objSym.OwnerScope)
	if body == nil {
		return nil
	}
	for _, node := range declMembers(objSym.Decl) {
		member := memberSymbol(body, node)
		if member != nil && restatesFeatureNamed(member, name) {
			return member
		}
	}
	return nil
}

// restatesFeatureNamed reports whether sym redefines a feature of the given
// name, whichever way the redefinition names it.
func restatesFeatureNamed(sym *symbols.Symbol, name string) bool {
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		target := rel.Target
		if ref, ok := target.(*ast.FeatureReference); ok {
			target = ref.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		if qn.Parts[len(qn.Parts)-1].Text == name {
			return true
		}
	}
	return false
}

// specializesLibraryType reports whether typ is the library type the qualified
// name states, or specializes it, matched by identity so a type merely named
// alike is not one.
func (ctx *Context) specializesLibraryType(typ *symbols.Symbol, fqn string) bool {
	if typ == nil || ctx.resolver == nil || ctx.resolver.Index() == nil {
		return false
	}
	stated := make(map[*symbols.Symbol]bool, 1)
	for _, sym := range ctx.resolver.Index().LookupQualified(fqn) {
		if sym != nil {
			stated[sym] = true
		}
	}
	if stated[typ] {
		return true
	}
	for _, super := range ctx.model.AllSupertypes(typ) {
		if stated[super] {
			return true
		}
	}
	return false
}

// objectiveDirection classifies an objective definition against the trade-study
// library: the nearest of MinimizeObjective and MaximizeObjective it
// specializes, matched by identity so a type merely named alike is not one.
func (ctx *Context) objectiveDirection(typ *symbols.Symbol) ObjectiveDirection {
	if typ == nil || ctx.resolver == nil || ctx.resolver.Index() == nil {
		return NoDirection
	}
	idx := ctx.resolver.Index()
	directions := make(map[*symbols.Symbol]ObjectiveDirection, 2)
	for fqn, direction := range map[string]ObjectiveDirection{
		minimizeObjectiveFQN: Minimize,
		maximizeObjectiveFQN: Maximize,
	} {
		for _, sym := range idx.LookupQualified(fqn) {
			if sym != nil {
				directions[sym] = direction
			}
		}
	}
	if direction, ok := directions[typ]; ok {
		return direction
	}
	// AllSupertypes is breadth-first over declaration order, so the nearest
	// trade-study ancestor is found first.
	for _, super := range ctx.model.AllSupertypes(typ) {
		if direction, ok := directions[super]; ok {
			return direction
		}
	}
	return NoDirection
}

// objectiveConditionsOf returns the conditions an objective states in its own
// body together with the ones it inherits from a model's own definitions, and
// apart from those the ones it inherits from the trade-study library, which are
// about the choice among alternatives rather than about which values are feasible.
func (ctx *Context) objectiveConditionsOf(sym *symbols.Symbol) (model, library []Condition) {
	if sym == nil {
		return nil, nil
	}
	// An inherited condition is read where it is inherited: the objective's own
	// body, where the `best` it inherits answers that name.
	body := bodyScope(sym, sym.OwnerScope)
	var members, libraryMembers []scopedMember
	supers := ctx.model.AllSupertypes(sym)
	for i := len(supers) - 1; i >= 0; i-- {
		link := supers[i]
		if link == nil || ctx.frameDeclared(link) {
			continue
		}
		for _, node := range declMembers(link.Decl) {
			if ctx.libraryDeclared(link) {
				libraryMembers = append(libraryMembers, scopedMember{node: node, scope: body})
			} else {
				members = append(members, scopedMember{node: node, scope: body})
			}
		}
	}
	for _, node := range declMembers(sym.Decl) {
		members = append(members, scopedMember{node: node, scope: body})
	}
	return ctx.conditionsOf(sym, members), ctx.conditionsOf(sym, libraryMembers)
}

// CaseConditionsOf returns the conditions a case states as what it holds true of
// its parameters: the conditions its own members state, and the ones stated by
// the constraints it requires, assumes or asserts in its body. A constraint
// declared without one of those keywords states nothing the case checks, so it
// is left out.
func (ctx *Context) CaseConditionsOf(sym *symbols.Symbol, scope *symbols.Scope) []Condition {
	if sym == nil {
		return nil
	}
	if scope == nil {
		scope = sym.OwnerScope
	}
	out := ctx.appendResultConflict(nil, sym, true)
	for _, member := range ctx.chainMembers(sym, scope) {
		// A case's own steps are its procedure, not a statement its conditions miss.
		if _, step := statementKeyword(member.node); step {
			continue
		}
		out = ctx.appendConditions(out, member.node, member.scope, true, false, nil)
		out = ctx.appendCheckedConstraint(out, member)
	}
	return out
}

// appendCheckedConstraint appends the conditions a nested constraint usage
// states when the case requires, assumes, asserts or states it as an invariant.
// A negation it wrote negates what its conditions state together (De Morgan),
// as a negated constraint body does.
func (ctx *Context) appendCheckedConstraint(out []Condition, member scopedMember) []Condition {
	usage, ok := member.node.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageConstraint {
		return out
	}
	// The keyword sits ahead of a kind keyword (`require constraint { … }`) or
	// stands for the kind itself (`require c : C`, `inv { … }`).
	keyword := usage.PrefixKeyword
	if keyword == "" {
		keyword = usage.Keyword
	}
	var required bool
	switch keyword {
	case "require", "assert", "inv":
		required = true
	case "assume":
		required = false
	default:
		return out
	}
	sym := memberSymbol(member.scope, member.node)
	if sym == nil {
		return out
	}
	conds := ctx.ConditionsOf(sym, nil)
	for i := range conds {
		conds[i].Required = required
	}
	if !usage.IsNegated {
		return append(out, conds...)
	}
	if len(conds) == 1 {
		only := conds[0]
		only.Negated = !only.Negated
		return append(out, only)
	}
	return append(out, Condition{Group: conds, Negated: true, Required: required})
}

// memberSymbol returns the symbol scope declares for node, anonymous ones
// included, or nil when there is none.
func memberSymbol(scope *symbols.Scope, node ast.Node) *symbols.Symbol {
	if scope == nil || node == nil {
		return nil
	}
	for _, member := range scope.AllMembers() {
		if member.Decl == node {
			return member
		}
	}
	return nil
}
