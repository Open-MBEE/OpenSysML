package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrAmbiguousSubject is returned when more than one object carries the checked
// element, so which one the verdict would be about is a question, not an answer.
var ErrAmbiguousSubject = errors.New("ambiguous subject")

// Condition is one boolean check a constraint or requirement states, with the
// scope its expression resolves names in. A condition states either an
// expression or a group, which holds when all of its conditions hold.
type Condition struct {
	// Expr is the condition's expression, nil for a group.
	Expr ast.Node

	// Scope is where Expr's names resolve, or the type a Conflict is found on;
	// nil for a group.
	Scope *symbols.Scope

	// Group is the conditions a body states, all of which must hold; nil for a
	// condition stating an expression.
	Group []Condition

	// Statement is an action statement the body states before its conditions,
	// which the evaluator does not execute; nil for a condition or a group.
	Statement ast.Node

	// Conflict is a second result expression, stated or inherited (KerML
	// 8.3.4.8); no verdict is reached. Nil otherwise.
	Conflict *semantics.ResultExpressionConflict

	// Negated is the negation the declaration wrote, applied to Expr or to the
	// whole conjunction Group stands for.
	Negated bool

	// Required distinguishes a required condition from an assumption, which is
	// trusted rather than required to hold.
	Required bool

	// Constraints are the named constraint usages stating the condition through
	// their types, outermost first: each one's parameter values mask the
	// enclosing ones', which mask the checked element's. Empty reads the latter alone.
	Constraints []*symbols.Symbol
}

// Label renders the condition as written, negation and grouping included.
func (c Condition) Label() string { return conditionLabel(c) }

// Owner is the nearest named element declaring the condition (the supertype an
// inherited one came from), or nil; a body-local or anonymous scope is skipped.
func (c Condition) Owner() *symbols.Symbol {
	for s := c.Scope; s != nil; s = s.Parent() {
		if owner := s.Owner(); owner != nil && owner.Name != "" {
			return owner
		}
	}
	return nil
}

// ConditionsOf returns the conditions sym states, its inherited ones first: the
// same collection, in the same order, that evaluating sym checks. scope stands
// in for sym's own scope when sym declares none.
func (ctx *Context) ConditionsOf(sym *symbols.Symbol, scope *symbols.Scope) []Condition {
	if sym == nil {
		return nil
	}
	if scope == nil {
		scope = sym.OwnerScope
	}
	return ctx.conditionsOf(sym, ctx.chainMembers(sym, scope))
}

// scopedExpr is an expression with the scope its names resolve in.
type scopedExpr struct {
	expr  ast.Node
	scope *symbols.Scope
	decl  *symbols.Symbol // feature the expression was written on

	// env is the environment the expression's names resolve in when it is a
	// named constraint's parameter value; nil for a feature read in place.
	env *conditionEnv
}

// conditionEnv is what a condition's names resolve to: the features in scope
// and the values the checked element binds by name (subject, actors).
type conditionEnv struct {
	features map[string]scopedExpr
	bindings frame

	// enclosing marks the environment around a constraint usage, which its
	// arguments read: `in v = v` names the outer v, not the parameter it binds.
	enclosing bool
}

// conditionsOf returns the conditions sym's members state, inherited ones first.
// A member states its condition either directly (`require x < y;`, `assert x < y;`)
// or through the body of an anonymous nested constraint (`require constraint { x < y }`).
func (ctx *Context) conditionsOf(sym *symbols.Symbol, members []scopedMember) []Condition {
	return ctx.appendMemberConditions(nil, sym, members, true, nil)
}

// appendMemberConditions appends the conditions sym's members state, leaving out
// an inherited named constraint a closer one shadows or redefines (KerML 7.3.4.5);
// an anonymous one, which no name can redefine, is always inherited.
func (ctx *Context) appendMemberConditions(out []Condition, sym *symbols.Symbol, members []scopedMember,
	required bool, seen map[*symbols.Symbol]bool) []Condition {
	out = ctx.appendResultConflict(out, sym, required)
	var effective map[*symbols.Symbol]bool
	for _, member := range members {
		if owner := ctx.namedConstraintOf(member); owner != nil && owner != sym && owner.Name != "" {
			if effective == nil {
				effective = ctx.effectiveMembers(sym)
			}
			if !effective[owner] {
				continue
			}
		}
		out = ctx.appendConditions(out, member.node, member.scope, required, false, seen)
	}
	return out
}

// appendResultConflict appends the marker for a second owned or inherited result
// expression of sym, which no body of the runtime's choosing may stand in for.
func (ctx *Context) appendResultConflict(out []Condition, sym *symbols.Symbol, required bool) []Condition {
	conflict := ctx.model.semantics.ResultExpressionConflict(sym)
	if conflict == nil {
		return out
	}
	scope := sym.Scope
	if scope == nil {
		scope = sym.OwnerScope
	}
	return append(out, Condition{Conflict: conflict, Scope: scope, Required: required})
}

// namedConstraintOf returns the constraint usage a require/assume constraint
// member declares, named or anonymous; nil for any other member.
func (ctx *Context) namedConstraintOf(member scopedMember) *symbols.Symbol {
	if _, ok := ast.OwnedConstraintOf(member.node); !ok {
		return nil
	}
	body := symbols.ConstraintBodyScope(member.scope, member.node)
	if body == nil || body.Owner() == nil || body.Owner().Decl != member.node {
		return nil
	}
	return body.Owner()
}

// effectiveMembers is the set of members sym has: MembersOf as a set.
func (ctx *Context) effectiveMembers(sym *symbols.Symbol) map[*symbols.Symbol]bool {
	members := ctx.model.semantics.MembersOf(sym)
	set := make(map[*symbols.Symbol]bool, len(members))
	for _, member := range members {
		set[member] = true
	}
	return set
}

// appendConditions appends the conditions node states. required says whether the
// enclosing member requires them to hold or only assumes them; negated is the
// negation the enclosing member wrote, which a nested body inherits.
// seen holds the requirements a reference-subsetting member has been expanded
// through, so a cycle between two of them ends.
func (ctx *Context) appendConditions(out []Condition, node ast.Node, scope *symbols.Scope, required, negated bool,
	seen map[*symbols.Symbol]bool) []Condition {
	switch m := node.(type) {
	case *ast.ConstraintMember:
		negated = negated != m.IsNegated
		required = required && m.IsAssert
		if m.Expression != nil {
			out = append(out, Condition{Expr: m.Expression, Scope: scope, Negated: negated, Required: required})
		}
		if len(m.Body) == 0 {
			return out
		}
		var body []Condition
		bodyScope := symbols.ConstraintBodyScope(scope, m)
		for _, nested := range m.Body {
			body = ctx.appendConditions(body, nested, bodyScope, true, false, seen)
		}
		if !negated {
			for _, c := range body {
				c.Required = c.Required && required
				out = append(out, c)
			}
			return out
		}
		// A body means the conjunction of its conditions, so negating it negates
		// that conjunction rather than each condition (De Morgan). A conjunction
		// of one is that one condition.
		if len(body) == 1 {
			only := body[0]
			only.Negated = !only.Negated
			only.Required = only.Required && required
			return append(out, only)
		}
		out = append(out, Condition{Group: body, Negated: true, Required: required})
	case *ast.RequireMember:
		out = ctx.appendRequirementConditions(out, m, m.Expression, m.Body, scope, true, seen)
	case *ast.AssumeMember:
		out = ctx.appendRequirementConditions(out, m, m.Expression, m.Body, scope, false, seen)
	case *ast.Membership:
		out = ctx.appendConditions(out, m.Member, scope, required, negated, seen)
	default:
		if _, ok := statementKeyword(m); ok {
			out = append(out, Condition{Statement: m, Scope: scope, Required: required})
		}
	}
	return out
}

// appendRequirementConditions appends what a require/assume member states: the
// conditions of the constraint it references (`require q;`, `require P::q;`),
// or else its own condition expression, then those its body owns.
func (ctx *Context) appendRequirementConditions(out []Condition, member ast.Node, expr ast.Node, body []ast.Node,
	scope *symbols.Scope, required bool, seen map[*symbols.Symbol]bool) []Condition {
	if ref := ast.ConstraintReferenceOf(member); ref != nil {
		out = ctx.appendReferencedConditions(out, member, ref, scope, required, seen)
	} else if expr != nil {
		out = append(out, Condition{Expr: expr, Scope: scope, Required: required})
	}
	return ctx.appendOwnedConditions(out, member, body, scope, required, seen)
}

// appendOwnedConditions appends what a require/assume member's constraint states:
// a named one its whole chain, an anonymous body its own conditions.
func (ctx *Context) appendOwnedConditions(out []Condition, member ast.Node, body []ast.Node, scope *symbols.Scope,
	required bool, seen map[*symbols.Symbol]bool) []Condition {
	if owner := ctx.namedConstraintOf(scopedMember{node: member, scope: scope}); owner != nil {
		start := len(out)
		out = ctx.appendMemberConditions(out, owner, ctx.chainMembers(owner, scope), required, seen)
		setConstraint(out[start:], owner)
		return out
	}
	bodyScope := symbols.ConstraintBodyScope(scope, member)
	for _, nested := range body {
		out = ctx.appendConditions(out, nested, bodyScope, required, false, seen)
	}
	return out
}

// setConstraint marks conds as stated by owner, enclosing any nested named
// constraint already marked: the innermost usage's parameter values win.
func setConstraint(conds []Condition, owner *symbols.Symbol) {
	for i := range conds {
		conds[i].Constraints = append([]*symbols.Symbol{owner}, conds[i].Constraints...)
		setConstraint(conds[i].Group, owner)
	}
}

// conflictingResultExpression returns the first result-expression conflict conds
// record, groups included, or nil when they record none.
func conflictingResultExpression(conds []Condition) *semantics.ResultExpressionConflict {
	for _, cond := range conds {
		if cond.Conflict != nil {
			return cond.Conflict
		}
		if nested := conflictingResultExpression(cond.Group); nested != nil {
			return nested
		}
	}
	return nil
}

// conflictText says what a result-expression conflict is: a second result stated
// in one body, a condition stated over an inherited one, or a declaration inheriting two.
func conflictText(conflict *semantics.ResultExpressionConflict) string {
	switch {
	case conflict.Stated > 1:
		return "`" + conditionText(conflict.Node) + "` is a second result expression of one body"
	case conflict.Stated == 1:
		return "`" + conditionText(conflict.Node) + "` is stated over an inherited result expression"
	default:
		return "it inherits a result expression from more than one supertype"
	}
}

// unexecutedStatement returns the first statement conds state, groups included,
// or nil when they state none.
func unexecutedStatement(conds []Condition) ast.Node {
	for _, cond := range conds {
		if cond.Statement != nil {
			return cond.Statement
		}
		if stmt := unexecutedStatement(cond.Group); stmt != nil {
			return stmt
		}
	}
	return nil
}

// statementKeyword names the keyword a body item the evaluator does not run
// was written with: an action statement, an action node, or a succession.
func statementKeyword(node ast.Node) (string, bool) {
	switch n := node.(type) {
	case *ast.AssignmentActionNode:
		return "assign", true
	case *ast.IfActionNode:
		return "if", true
	case *ast.WhileLoopActionNode:
		return "loop", true
	case *ast.SendStatement:
		return "send", true
	case *ast.TerminateStatement:
		return "terminate", true
	case *ast.PerformActionNode:
		return "perform", true
	case *ast.ActionExecutionNode, *ast.AcceptActionUsage:
		return "action", true
	case *ast.InitialNode:
		return "first", true
	case *ast.SuccessionEdge, *ast.ControlFlowEdge:
		return "then", true
	case *ast.ForkNode:
		return "fork", true
	case *ast.JoinNode:
		return "join", true
	case *ast.MergeNode:
		return "merge", true
	case *ast.DecisionNode:
		return "decide", true
	case *ast.FinalNode:
		return "done", true
	case *ast.Usage:
		switch {
		case n.IsPerformedAction():
			return "perform", true
		case n.Kind == ast.UsageAction:
			return "action", true
		case n.IsSuccessionFlow():
			return "succession flow", true
		case n.Kind == ast.UsageSuccession:
			return "succession", true
		}
	}
	return "", false
}

// appendReferencedConditions appends what a require/assume member that
// reference-subsets a requirement states: that requirement's own conditions,
// which requiring it requires. A reference naming anything else, or one that
// does not resolve, states the condition its name evaluates to.
func (ctx *Context) appendReferencedConditions(out []Condition, decl ast.Node, ref ast.Node, scope *symbols.Scope,
	required bool, seen map[*symbols.Symbol]bool) []Condition {
	if ref == nil {
		return out
	}
	sym := ctx.referencedRequirement(scope, decl, ref)
	if sym == nil {
		return append(out, Condition{Expr: ref, Scope: scope, Required: required})
	}
	if seen[sym] {
		return out
	}
	if seen == nil {
		seen = map[*symbols.Symbol]bool{}
	}
	seen[sym] = true
	defer delete(seen, sym)
	conds := ctx.appendMemberConditions(nil, sym, ctx.chainMembers(sym, nil), true, seen)
	for i := range conds {
		conds[i].Required = required
	}
	return append(out, conds...)
}

// referencedRequirement resolves the requirement or constraint the require/assume
// member decl reference-subsets, and returns nil when the reference names
// anything else or does not resolve.
func (ctx *Context) referencedRequirement(scope *symbols.Scope, decl ast.Node, ref ast.Node) *symbols.Symbol {
	if ctx.model.resolver == nil {
		return nil
	}
	sym, ok := ctx.model.resolver.ResolveReferenceTarget(scope, decl, ref)
	if !ok || sym == nil {
		return nil
	}
	if canonical, ok := ctx.model.resolver.ResolveAliasTarget(sym); ok {
		sym = canonical
	}
	if RequireRequirement(sym) != nil && RequireConstraint(sym) != nil {
		return nil
	}
	return sym
}

// conditionCheck is one evaluation of the conditions an element states: the
// element itself, how it is named in messages, and what its conditions are
// evaluated against.
type conditionCheck struct {
	sym  *symbols.Symbol
	kind string // "constraint", "requirement", "satisfaction"
	what string // "assertion", "require condition"

	// element names the checked element in messages. Empty takes sym's name,
	// which an anonymous declaration such as a satisfaction assertion lacks.
	element string

	// self is the object a feature name resolves against, nil when unbound.
	self *Instance

	// bindings are the values the element binds by name (subject, actor), and, for a
	// check within a case run, the run's, owned by the case; the zero frame binds nothing.
	bindings frame

	// negated inverts the verdict: the element asserts that its required
	// conditions do not all hold (`assert not …`, Invariant::isNegated).
	negated bool
}

// name returns how the checked element is named in messages.
func (c conditionCheck) name() string {
	if c.element != "" {
		return c.element
	}
	return c.sym.Name
}

// evaluateConditions evaluates conds in order and reports whether every required
// one holds, or — for a negated element — whether one of them fails. check.self
// is the subject already, as checkSubject resolved it.
func (ctx *Context) evaluateConditions(check conditionCheck, conds []Condition) (bool, error) {
	if len(conds) == 0 {
		return false, fmt.Errorf("%s %s: %w", check.kind, check.name(), ErrNoConditions)
	}
	// A statement anywhere in the body could change what the conditions read, so
	// no verdict is reached, not even from a condition stated before it.
	if stmt := unexecutedStatement(conds); stmt != nil {
		keyword, _ := statementKeyword(stmt)
		return false, fmt.Errorf("%s %s: %s evaluation failed: `%s` %w; bind the value as a feature value or compute it in a calc the condition reads",
			check.kind, check.name(), check.what, keyword, ErrStatementNotExecuted)
	}
	if conflict := conflictingResultExpression(conds); conflict != nil {
		return false, fmt.Errorf("%s %s: %s evaluation failed: %s: %w; a redefinition keeps the inherited condition and tightens it with a nested `assert constraint { … }`",
			check.kind, check.name(), check.what, conflictText(conflict), ErrConflictingResultExpressions)
	}
	features := ctx.conditionFeatures(check.sym)
	self := check.self
	// One check is one evaluation: its conditions share what a calc usage they
	// read answers, and the next check reads it again.
	activation, endStep := ctx.beginStep()
	defer endStep()
	required := false
	for _, cond := range conds {
		required = required || cond.Required
		holds, err := ctx.conditionHolds(activation, cond, features, self, check.bindings)
		if err != nil {
			return false, fmt.Errorf("%s %s: %s evaluation failed: %w", check.kind, check.name(), check.what, err)
		}
		if cond.Required && !holds {
			// A negated element asserts exactly this: one required condition
			// failing makes the negated assertion hold.
			if check.negated {
				return true, nil
			}
			return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: conditionLabel(cond)}
		}
	}
	if check.negated {
		// An assumption is trusted rather than checked, so a negated element
		// stating only assumptions denies nothing.
		if !required {
			return false, fmt.Errorf("%s %s: %w", check.kind, check.name(), ErrNoConditions)
		}
		return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: negatedText(conds)}
	}
	return true, nil
}

// conditionSubject is the object a check is about: the one supplied when it
// carries the checked element, else the single object of this runtime that does.
// A nested object counts, since a redefinition on an object gives a nested
// feature values of its own; no such object leaves the check about the
// declaration.
func (ctx *Context) conditionSubject(sym *symbols.Symbol, self *Instance) (carrier, error) {
	owner := declaringType(sym)
	if owner == nil {
		return carrier{instance: self, root: self}, nil
	}
	roots := []*Instance{self}
	if self == nil {
		roots = ctx.rootInstances()
	} else if ctx.model.semantics.Conforms(self.Type, owner) {
		return carrier{instance: self, root: self}, nil
	}
	carriers := ctx.carriersUnder(roots, owner)
	switch len(carriers) {
	case 0:
		return carrier{instance: self, root: self}, nil
	case 1:
		return carriers[0], nil
	}
	return carrier{}, fmt.Errorf("%w: %s is carried by %s: check it on one of them",
		ErrAmbiguousSubject, sym.Name, strings.Join(ctx.carrierLabels(carriers), ", "))
}

// checkSubject resolves the object a check is about before its bindings are
// evaluated, so the bindings and the conditions read one object. An ambiguity is
// named after the checked element, as a verdict is.
func (ctx *Context) checkSubject(kind, element string, sym *symbols.Symbol, self *Instance) (carrier, error) {
	subject, err := ctx.conditionSubject(sym, self)
	if err != nil {
		return carrier{}, fmt.Errorf("%s %s: %w", kind, element, err)
	}
	return subject, nil
}

// declaringType is the type whose objects carry sym, nil when sym is declared
// somewhere that has no objects — a package, a library namespace.
func declaringType(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return nil
	}
	switch owner.Decl.(type) {
	case *ast.Definition, *ast.Usage:
		return owner
	}
	return nil
}

// nestedFeature reports whether sym is a feature a type declares. A definition
// nested in another is not one: objects materialize it in their own right.
func nestedFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return false
	}
	return declaringType(sym) != nil
}

// readThrough reports whether inst is the object a value expression materialized
// to read a declaration through, which is an occurrence of nothing.
func (ctx *Context) readThrough(inst *Instance) bool {
	if inst.explicit {
		return false
	}
	id, ok := ctx.occurrences[inst.Type]
	return ok && id == inst.ID
}

// rootInstances returns the objects this runtime holds that stand on their own,
// in identity order: an object a feature value holds is reached through its holder, and one
// materialized to read a nested declaration through is an occurrence of nothing,
// while an object a caller asked for is a root whatever it materializes. One
// declaration materialized twice is one object here, the latest.
func (ctx *Context) rootInstances() []*Instance {
	held := ctx.heldObjectIDs()
	latest := make(map[*symbols.Symbol]*Instance, len(ctx.instances))
	for _, inst := range ctx.instances {
		if inst == nil || held[inst.ID] || (nestedFeature(inst.Type) && ctx.readThrough(inst)) {
			continue
		}
		if kept, ok := latest[inst.Type]; ok && kept.ID > inst.ID {
			continue
		}
		latest[inst.Type] = inst
	}
	out := make([]*Instance, 0, len(latest))
	for _, inst := range latest {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// heldObjectIDs returns the identities a feature value of another object already holds, so
// an object reached through its holder is no root of its own. Feature values are read as
// they stand, since materializing one is the search that asked for these.
func (ctx *Context) heldObjectIDs() map[int64]bool {
	held := make(map[int64]bool)
	for _, inst := range ctx.instances {
		if inst == nil {
			continue
		}
		for _, fv := range inst.FeatureValues {
			for _, id := range heldObjects(fv.HeldValue()) {
				held[id] = true
			}
		}
	}
	return held
}

// carriersUnder returns the objects reachable from roots whose type carries the
// features owner declares, roots included, in identity order. A declaration is
// descended into once per path, so recursive composition is a finite search, and
// one object stands for each declaration reached, so objects a multiplicity
// repeated are one candidate however deep the named declaration sits in them.
func (ctx *Context) carriersUnder(roots []*Instance, owner *symbols.Symbol) []carrier {
	var out []carrier
	seen := make(map[int64]bool, len(roots))
	declared := make(map[carrierOccurrence]bool)
	path := make(map[*symbols.Symbol]bool)
	var descend func(root, inst *Instance, through string, features []string)
	descend = func(root, inst *Instance, through string, features []string) {
		if inst == nil || seen[inst.ID] {
			return
		}
		seen[inst.ID] = true
		occurrence := carrierOccurrence{through: through, decl: inst.Type}
		if ctx.model.semantics.Conforms(inst.Type, owner) && !declared[occurrence] {
			declared[occurrence] = true
			out = append(out, carrier{instance: inst, root: root, features: features})
		}
		if inst.Type != nil {
			if path[inst.Type] {
				return
			}
			path[inst.Type] = true
			defer delete(path, inst.Type)
		}
		for _, child := range ctx.nestedObjects(inst) {
			nested := append(features[:len(features):len(features)], child.feature)
			descend(root, child.instance, through+"/"+strconv.Quote(child.feature), nested)
		}
	}
	for _, root := range roots {
		descend(root, root, strconv.FormatInt(root.ID, 10), nil)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].instance.ID < out[j].instance.ID })
	return out
}

// carrier is an object a search reached: the object the search started from and
// the features walked from it name a nested one, which has no name of its own.
type carrier struct {
	instance *Instance
	root     *Instance
	features []string
}

// carrierOccurrence identifies the declaration an object occurs as: the features
// walked through to reach it, each quoted so any name is one segment, and the
// declaration it materializes. Objects a multiplicity repeated share both, while
// ones a collection gathers from different declarations — the features
// subsetting it — do not.
type carrierOccurrence struct {
	through string
	decl    *symbols.Symbol
}

// heldObject is an object a feature of another object holds, named by that
// feature.
type heldObject struct {
	feature  string
	instance *Instance
}

// nestedObjects returns the objects the object-valued features of inst hold,
// materializing a lazy one as reading its feature value does. A feature value that cannot be read
// yields no object: one that is not there is no subject either. The names a redefinition
// chain gives one feature value hold its objects once, under the first.
func (ctx *Context) nestedObjects(inst *Instance) []heldObject {
	var out []heldObject
	read := map[*FeatureValue]bool{}
	for _, of := range ctx.FeaturesOfObject(inst) {
		if of.Name == "" || !holdsObjects(of.Feature) {
			continue
		}
		fv, err := inst.GetFeatureValue(ctx, of.Name)
		if err != nil || fv == nil || read[fv] {
			continue
		}
		read[fv] = true
		for _, id := range heldObjects(fv.HeldValue()) {
			if child, ok := ctx.instances[id]; ok {
				out = append(out, heldObject{feature: of.Name, instance: child})
			}
		}
	}
	return out
}

// holdsObjects reports whether a feature holds objects rather than values: a
// nested part has features and conditions of its own, an attribute has neither.
func holdsObjects(feat *EffectiveFeature) bool {
	if feat.Symbol == nil {
		return false
	}
	usage, ok := feat.Symbol.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	switch usage.Kind {
	case ast.UsagePart, ast.UsageItem, ast.UsageOccurrence, ast.UsageIndividual, ast.UsagePort:
		return true
	}
	return false
}

// heldObjects returns the identities a feature value denotes, a collection's
// elements included.
func heldObjects(val Value) []int64 {
	if id, ok := val.Object(); ok {
		return []int64{id}
	}
	var elements []Value
	switch {
	case val.Kind == ValSequence && val.Sequence() != nil:
		elements = val.Sequence().Elements()
	case val.Kind == ValSet && val.Set() != nil:
		elements = val.Set().Elements()
	}
	var out []int64
	for _, element := range elements {
		if id, ok := element.Object(); ok {
			out = append(out, id)
		}
	}
	return out
}

// carrierLabels names carriers as a diagnostic can quote them: the definition
// each is an object of, its identity, and the feature path telling two apart.
func (ctx *Context) carrierLabels(carriers []carrier) []string {
	out := make([]string, 0, len(carriers))
	for _, c := range carriers {
		inst := c.instance
		name := "object"
		if def := ctx.definitionOf(inst.Type); def != nil && def.Name != "" {
			name = def.Name
		}
		label := fmt.Sprintf("%s #%d", name, inst.ID)
		if path := carrierPath(c, name); len(path) != 0 {
			label += " (" + qualifiedText(path) + ")"
		}
		out = append(out, label)
	}
	return out
}

// qualifiedText spells walked feature names as a `::`-joined path in the
// notation, each quoted where its spelling needs it.
func qualifiedText(names []string) string {
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = lexer.NameText(name)
	}
	return strings.Join(parts, "::")
}

// carrierPath names a carrier apart from its siblings: the features walked to
// it, ending in the declaration it materializes — which differs from the feature
// holding it when a collection gathers objects of several declarations.
func carrierPath(c carrier, definition string) []string {
	decl := ""
	if c.instance.Type != nil && c.instance.Type.Name != definition {
		decl = c.instance.Type.Name
	}
	if len(c.features) == 0 {
		if decl == "" {
			return nil
		}
		return []string{decl}
	}
	walked := append([]string(nil), c.features...)
	if decl != "" && walked[len(walked)-1] != decl {
		walked[len(walked)-1] = decl
	}
	return walked
}

// carrierFeatures is the feature path to a nested carrier, one name a segment,
// corrected the way carrierLabels names an ambiguity's carriers.
func (ctx *Context) carrierFeatures(c carrier) []string {
	if len(c.features) == 0 || c.instance == nil {
		return nil
	}
	name := "object"
	if def := ctx.definitionOf(c.instance.Type); def != nil && def.Name != "" {
		name = def.Name
	}
	return carrierPath(c, name)
}

// definitionOf is the definition objects of sym are objects of: sym itself when
// it declares one, else the nearest definition it specializes.
func (ctx *Context) definitionOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil {
		return nil
	}
	if _, ok := sym.Decl.(*ast.Definition); ok {
		return sym
	}
	for _, super := range ctx.model.semantics.AllSupertypes(sym) {
		if _, ok := super.Decl.(*ast.Definition); ok {
			return super
		}
	}
	return sym
}

// conditionHolds evaluates one condition: an expression, or a group that holds
// when all of its conditions hold. Its negation, if any, is applied last.
func (ctx *Context) conditionHolds(activation int64, cond Condition, features map[string]scopedExpr, self *Instance, bindings frame) (bool, error) {
	for _, constraint := range cond.Constraints {
		features, bindings = ctx.constraintScope(features, bindings, constraint)
	}
	holds := true
	if cond.Group != nil {
		for _, sub := range cond.Group {
			subHolds, err := ctx.conditionHolds(activation, sub, features, self, bindings)
			if err != nil {
				return false, err
			}
			holds = holds && subHolds
		}
	} else {
		ec := NewEvalContextIn(ctx, cond.Scope, self)
		ec.activation = activation
		ec.features = features
		if bindings.vars != nil {
			ec.pushFrame(bindings)
		}
		result, err := ec.Eval(cond.Expr)
		if err != nil {
			return false, err
		}
		if u := result.Undetermined(); u != nil {
			return false, fmt.Errorf("%w: condition is undetermined: %s", ErrNoValue, u.Reason())
		}
		if result.Kind != ValConst || result.Const.Kind != semantics.ValBool {
			return false, fmt.Errorf("condition must evaluate to boolean, got %v", result.Kind)
		}
		holds = result.Const.Bool
	}
	if cond.Negated {
		holds = !holds
	}
	return holds, nil
}

// negatedText renders what a negated element asserted and did not get: that not
// every required condition holds.
func negatedText(conds []Condition) string {
	var texts []string
	for _, cond := range conds {
		if !cond.Required {
			continue
		}
		texts = append(texts, conditionLabel(cond))
	}
	if len(texts) == 1 {
		return "not " + texts[0]
	}
	return "not (" + strings.Join(texts, " and ") + ")"
}

// conditionFeatures returns the features the conditions of sym may name: its
// own, the ones it inherits, and the ones a typed usage rebinds, which mask the
// declaration they redefine. A feature carrying no value maps to a nil
// expression, so naming it reports an uninitialized feature rather than an
// unresolved one.
func (ctx *Context) conditionFeatures(sym *symbols.Symbol) map[string]scopedExpr {
	features := ctx.FeaturesOf(sym)
	params := ctx.parameterFeatures(sym)
	if len(features)+len(params) == 0 {
		return nil
	}
	out := make(map[string]scopedExpr, len(features)+len(params))
	add := func(feat *EffectiveFeature) {
		if _, present := out[feat.Name]; feat.Name == "" || present {
			return
		}
		expr := feat.DefaultValue
		if !ctx.valueBinds(feat) {
			// A body governing over the inherited value supersedes it, so a
			// condition read without an object reports the feature
			// uninitialized rather than the value materializing replaces.
			expr = nil
		}
		out[feat.Name] = scopedExpr{expr: expr, scope: feat.DefaultScope(), decl: feat.DefaultDecl}
	}
	for i := range features {
		add(&features[i])
	}
	for i := range params {
		add(&params[i])
	}
	return out
}

// constraintScope overlays a named constraint usage's features on those in scope:
// its parameters mask same-named ones (subject and actors included). The
// arguments binding them (`in v = v`) read the enclosing environment; a default
// its definition wrote (`in y default = x`) reads the usage's own parameters.
func (ctx *Context) constraintScope(features map[string]scopedExpr, bindings frame, constraint *symbols.Symbol) (map[string]scopedExpr, frame) {
	own := ctx.conditionFeatures(constraint)
	if len(own) == 0 {
		return features, bindings
	}
	enclosing := &conditionEnv{features: features, bindings: bindings, enclosing: true}
	out := make(map[string]scopedExpr, len(features)+len(own))
	for name, feat := range features {
		out[name] = feat
	}
	inner := &conditionEnv{features: out, bindings: unmasked(bindings, own)}
	for name, feat := range own {
		feat.env = inner
		if isArgument(feat.decl) {
			feat.env = enclosing
		}
		out[name] = feat
	}
	return out, inner.bindings
}

// isArgument reports whether a parameter value was written on a usage rather
// than on the definition declaring the parameter.
func isArgument(decl *symbols.Symbol) bool {
	if decl == nil || decl.OwnerScope == nil || decl.OwnerScope.Owner() == nil {
		return false
	}
	_, definition := decl.OwnerScope.Owner().Decl.(*ast.Definition)
	return !definition
}

// unmasked returns bindings without the names features declare.
func unmasked(bindings frame, features map[string]scopedExpr) frame {
	masked := false
	for name := range bindings.vars {
		if _, ok := features[name]; ok {
			masked = true
			break
		}
	}
	if !masked {
		return bindings
	}
	out := make(map[string]Value, len(bindings.vars))
	for name, value := range bindings.vars {
		if _, ok := features[name]; !ok {
			out[name] = value
		}
	}
	return bindings.withVars(out)
}

// conditionLabel renders a condition as written, so a violation names the
// condition that failed, negation and grouping included.
func conditionLabel(cond Condition) string {
	if cond.Statement != nil {
		keyword, _ := statementKeyword(cond.Statement)
		return "`" + keyword + "` statement"
	}
	if cond.Conflict != nil {
		return "conflicting result expression"
	}
	text := conditionText(cond.Expr)
	if cond.Group != nil {
		parts := make([]string, 0, len(cond.Group))
		for _, sub := range cond.Group {
			parts = append(parts, conditionLabel(sub))
		}
		text = "{ " + strings.Join(parts, "; ") + " }"
	}
	if cond.Negated {
		text = "not " + text
	}
	return text
}

// conditionText renders a condition compactly, so a violation names the
// condition that failed rather than only the element that states it.
func conditionText(n ast.Node) string {
	switch e := n.(type) {
	case *ast.LiteralInteger:
		return e.Value
	case *ast.LiteralReal:
		return e.Value
	case *ast.LiteralString:
		return e.Value
	case *ast.LiteralBool:
		if e.Value {
			return "true"
		}
		return "false"
	case *ast.FeatureReference:
		return qualifiedNameToString(e.Name)
	case *ast.FeatureChainExpr:
		return conditionText(e.Operand) + "." + qualifiedNameToString(e.Member)
	case *ast.OperatorExpr:
		switch len(e.Operands) {
		case 1:
			return e.Operator.String() + " " + conditionText(e.Operands[0])
		case 2:
			return conditionText(e.Operands[0]) + " " + e.Operator.String() + " " + conditionText(e.Operands[1])
		}
	case *ast.InvocationExpr:
		args := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			args = append(args, conditionText(arg))
		}
		return qualifiedNameToString(e.Type) + "(" + strings.Join(args, ", ") + ")"
	case *ast.IndexExpr:
		// The bracket form is a quantity, `1.0 [m]`; `#` indexes a sequence.
		if e.Bracket {
			unit := semantics.UnitExprText(e.Index)
			if unit == "" {
				unit = conditionText(e.Index)
			}
			return conditionText(e.Operand) + " [" + unit + "]"
		}
		return conditionText(e.Operand) + "#(" + conditionText(e.Index) + ")"
	}
	return TraceLabel(n)
}
