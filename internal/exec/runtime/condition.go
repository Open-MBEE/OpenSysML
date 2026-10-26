package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ErrAmbiguousSubject is returned when more than one object carries the checked
// element, so which one the verdict would be about is a question, not an answer.
var ErrAmbiguousSubject = errors.New("ambiguous subject")

// ErrNotCarried is returned when a case a type owns is run on an object that is
// not of that type and holds none.
var ErrNotCarried = errors.New("object does not carry the case")

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

	// Steps are the statements the body performs before its Group's conditions
	// are evaluated, as one Boolean function performance; nil for a condition
	// or a group stating none.
	Steps *BodySteps

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
	// enclosing ones', which mask the checked element's. empty reads the latter alone.
	Constraints []*symbols.Symbol

	// Circular is the requirement a `require r` names while r's own conditions
	// are being collected: a reference back into the check, stating nothing evaluable.
	Circular *symbols.Symbol

	// Subrequirement is the requirement usage whose check Group states, its assumptions
	// implying its required conditions (RequirementCheck::result); nil for a body's conjunction.
	Subrequirement *symbols.Symbol
}

// BodySteps is the statements one constraint body performs before its
// conditions are evaluated, with the body scope for spans and name resolution.
type BodySteps struct {
	Stmts     []lower.Statement
	Scope     *symbols.Scope
	Node      ast.Node // first statement, for spans
	Order     *lower.StatementOrder
	Footprint lower.Footprint
}

type constraintBodyKey struct {
	node  ast.Node
	scope *symbols.Scope
}

func (ctx *Context) constraintBodySteps(stmts []lower.Statement, members []ast.Node, scope *symbols.Scope, node ast.Node) *BodySteps {
	body := &BodySteps{Scope: scope, Node: node}
	body.Stmts, body.Order = lower.ConstraintBodyWithOrder(scope, members, stmts)
	body.Footprint = lower.ConstraintBodyFootprint(scope, body.Stmts)
	if node == nil {
		return body
	}
	key := constraintBodyKey{node: node, scope: scope}
	if cached, ok := ctx.model.constraintBodies.Load(key); ok {
		return cached.(*BodySteps)
	}
	cached, _ := ctx.model.constraintBodies.LoadOrStore(key, body)
	return cached.(*BodySteps)
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
	held  *symbols.Symbol // effective declaration holding the value, decl or one redefining it

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
	return ctx.appendMemberConditions(nil, sym, members, true, map[*symbols.Symbol]bool{sym: true})
}

// appendMemberConditions appends the conditions sym's members state, leaving out
// an inherited named constraint a closer one shadows or redefines (KerML 7.3.4.5);
// an anonymous one, which no name can redefine, is always inherited.
func (ctx *Context) appendMemberConditions(out []Condition, sym *symbols.Symbol, members []scopedMember,
	required bool, seen map[*symbols.Symbol]bool) []Condition {
	out = ctx.appendResultConflict(out, sym, required)
	var effective map[*symbols.Symbol]bool
	start := len(out)
	hasStatements := false
	for _, member := range members {
		if _, ok := statementKeyword(unwrapBodyMember(member.node)); ok {
			hasStatements = true
			break
		}
	}
	var steps []lower.Statement
	var stepMembers []ast.Node
	var stepScope *symbols.Scope
	var stepNode ast.Node
	for _, member := range members {
		if owner := ctx.conditionOwnerOf(member); owner != nil && owner != sym && owner.Name != "" {
			if effective == nil {
				effective = ctx.effectiveMembers(sym)
			}
			if !effective[owner] {
				continue
			}
		}
		if hasStatements {
			if stmt, ok := ctx.constraintBodyStep(member.node, member.scope); ok {
				if stepNode == nil {
					stepNode, stepScope = member.node, member.scope
				}
				steps = append(steps, stmt)
				stepMembers = append(stepMembers, member.node)
				continue
			}
		}
		out = ctx.appendConditions(out, member.node, member.scope, required, false, seen)
	}
	if len(steps) > 0 {
		group := append([]Condition(nil), out[start:]...)
		out = append(out[:start], Condition{Group: group, Required: required,
			Steps: ctx.constraintBodySteps(steps, stepMembers, stepScope, stepNode)})
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

// conditionOwnerOf returns the usage a member states its conditions through: the
// constraint usage of a require/assume member, or a subrequirement; nil otherwise.
func (ctx *Context) conditionOwnerOf(member scopedMember) *symbols.Symbol {
	if owner := ctx.namedConstraintOf(member); owner != nil {
		return owner
	}
	return ctx.subrequirementOf(member.node, member.scope)
}

// subrequirementOf returns the subrequirement a requirement body member declares (SysML v2
// §7.21.2), nil for any other member; an abstract one, like the library's `subrequirements`, is none.
func (ctx *Context) subrequirementOf(node ast.Node, scope *symbols.Scope) *symbols.Symbol {
	usage, ok := node.(*ast.Usage)
	if !ok || usage.IsAbstract {
		return nil
	}
	sym := memberSymbol(scope, node)
	if sym == nil || !ctx.model.semantics.IsSubrequirement(sym) {
		return nil
	}
	return sym
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
		var steps []lower.Statement
		var stepMembers []ast.Node
		var stepNode ast.Node
		hasStatements := bodyHasStatements(m.Body)
		bodyScope := symbols.ConstraintBodyScope(scope, m)
		for _, nested := range m.Body {
			if hasStatements {
				if stmt, ok := ctx.constraintBodyStep(nested, bodyScope); ok {
					if stepNode == nil {
						stepNode = nested
					}
					steps = append(steps, stmt)
					stepMembers = append(stepMembers, nested)
					continue
				}
			}
			body = ctx.appendConditions(body, nested, bodyScope, true, false, seen)
		}
		if len(steps) > 0 {
			withSteps := Condition{Group: body, Required: required,
				Steps: ctx.constraintBodySteps(steps, stepMembers, bodyScope, stepNode)}
			if !negated {
				for i := range withSteps.Group {
					withSteps.Group[i].Required = withSteps.Group[i].Required && required
				}
				return append(out, withSteps)
			}
			withSteps.Negated = true
			return append(out, withSteps)
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
	case *ast.Usage:
		if sub := ctx.subrequirementOf(m, scope); sub != nil {
			out = ctx.appendSubrequirementConditions(out, sub, scope, required, seen)
		}
	default:
		// A member stating neither a condition nor a step — an expression, a
		// declaration — states nothing here; the body's steps were collected by
		// the caller that knew the body.
	}
	return out
}

// appendSubrequirementConditions appends sub as one constraint of its container (Requirements.sysml
// `subrequirements :> requirementChecks, constraints`): a group valued by sub's own check over its chain.
func (ctx *Context) appendSubrequirementConditions(out []Condition, sub *symbols.Symbol, scope *symbols.Scope,
	required bool, seen map[*symbols.Symbol]bool) []Condition {
	if seen[sub] {
		return append(out, Condition{Scope: scope, Required: required, Circular: sub})
	}
	if seen == nil {
		seen = map[*symbols.Symbol]bool{}
	}
	seen[sub] = true
	defer delete(seen, sub)
	start := len(out)
	out = ctx.appendMemberConditions(out, sub, ctx.chainMembers(sub, scope), true, seen)
	group := append([]Condition(nil), out[start:]...)
	return append(out[:start], Condition{Group: group, Required: required, Subrequirement: sub, Constraints: []*symbols.Symbol{sub}})
}

// appendRequirementConditions appends what a require/assume member states: the
// conditions of the constraint it references (`require q;`, `require P::q;`),
// or else its own condition expression, then those its body owns.
func (ctx *Context) appendRequirementConditions(out []Condition, member ast.Node, expr ast.Node, body []ast.Node,
	scope *symbols.Scope, required bool, seen map[*symbols.Symbol]bool) []Condition {
	if ref := ast.ConstraintReferenceOf(member); ref != nil {
		return ctx.appendReferencedConditions(out, member, ref, body, scope, required, seen)
	}
	if expr != nil {
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
	start := len(out)
	hasStatements := bodyHasStatements(body)
	var steps []lower.Statement
	var stepMembers []ast.Node
	var stepNode ast.Node
	for _, nested := range body {
		if hasStatements {
			if stmt, ok := ctx.constraintBodyStep(nested, bodyScope); ok {
				if stepNode == nil {
					stepNode = nested
				}
				steps = append(steps, stmt)
				stepMembers = append(stepMembers, nested)
				continue
			}
		}
		out = ctx.appendConditions(out, nested, bodyScope, required, false, seen)
	}
	if len(steps) > 0 {
		group := append([]Condition(nil), out[start:]...)
		out = append(out[:start], Condition{Group: group, Required: required,
			Steps: ctx.constraintBodySteps(steps, stepMembers, bodyScope, stepNode)})
	}
	return out
}

// setConstraint marks conds as stated by owner, enclosing any nested named constraint already
// marked (the innermost usage's parameter values win); a subrequirement's group is marked as a whole.
func setConstraint(conds []Condition, owner *symbols.Symbol) {
	for i := range conds {
		conds[i].Constraints = append([]*symbols.Symbol{owner}, conds[i].Constraints...)
		if conds[i].Subrequirement == nil {
			setConstraint(conds[i].Group, owner)
		}
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

// bodyHasStatements reports whether a constraint body's members include a
// statement — only then are the body's attribute and kind-less declarations
// steps interleaved with them; a body of declarations alone declares features
// of the check, read as they always were.
func bodyHasStatements(members []ast.Node) bool {
	for _, member := range members {
		if _, ok := statementKeyword(unwrapBodyMember(member)); ok {
			return true
		}
	}
	return false
}

// unwrapBodyMember is the element a membership carries, else the node itself.
func unwrapBodyMember(node ast.Node) ast.Node {
	if m, ok := node.(*ast.Membership); ok && m.Member != nil {
		return m.Member
	}
	return node
}

// constraintBodyStep lowers a member of a constraint body to the step it
// performs, once per member node over the run's model; false for the members
// stating conditions instead (see lower.ConstraintStep).
func (ctx *Context) constraintBodyStep(node ast.Node, scope *symbols.Scope) (lower.Statement, bool) {
	if ctx.model.constraintSteps == nil {
		ctx.model.constraintSteps = make(map[ast.Node]lower.Statement)
	}
	if stmt, ok := ctx.model.constraintSteps[node]; ok {
		return stmt, true
	}
	stmt, ok := lower.ConstraintStep(node, scope, ctx.model.Resolver())
	if !ok {
		return nil, false
	}
	ctx.model.constraintSteps[node] = stmt
	return stmt, true
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
// reference-subsets a requirement states: that requirement's conditions, which
// requiring it requires and assuming it assumes, read through the referencing
// usage so the arguments its body binds (`require r { in w = unit; }`) are seen.
// A reference naming anything else, or one that does not resolve, states the
// condition its name evaluates to, followed by what its body owns.
func (ctx *Context) appendReferencedConditions(out []Condition, decl ast.Node, ref ast.Node, body []ast.Node,
	scope *symbols.Scope, required bool, seen map[*symbols.Symbol]bool) []Condition {
	sym := ctx.referencedRequirement(scope, decl, ref)
	if sym == nil {
		out = append(out, Condition{Expr: ref, Scope: scope, Required: required})
		return ctx.appendOwnedConditions(out, decl, body, scope, required, seen)
	}
	if seen[sym] {
		return append(out, Condition{Scope: scope, Required: required, Circular: sym})
	}
	if seen == nil {
		seen = map[*symbols.Symbol]bool{}
	}
	seen[sym] = true
	defer delete(seen, sym)
	start := len(out)
	if usage := ctx.referencingConstraintOf(decl, scope); usage != nil {
		out = ctx.appendMemberConditions(out, usage, ctx.chainMembers(usage, scope), true, seen)
		setAssumed(out[start:], !required)
		setConstraint(out[start:], usage)
		return out
	}
	out = ctx.appendMemberConditions(out, sym, ctx.chainMembers(sym, nil), true, seen)
	setAssumed(out[start:], !required)
	return ctx.appendOwnedConditions(out, decl, body, scope, required, seen)
}

// referencingConstraintOf returns the constraint usage a `require r;` or
// `assume r;` member declares, which reference-subsets r and owns the member's
// body; nil when the symbol table declares none.
func (ctx *Context) referencingConstraintOf(member ast.Node, scope *symbols.Scope) *symbols.Symbol {
	body := symbols.ConstraintBodyScope(scope, member)
	if body == nil || body.Owner() == nil || body.Owner().Decl != member {
		return nil
	}
	return body.Owner()
}

// setAssumed turns conds, groups included, into assumptions when assumed is set; the assumptions
// among them stay assumptions either way, and what a subrequirement's group states stays its own check.
func setAssumed(conds []Condition, assumed bool) {
	for i := range conds {
		conds[i].Required = conds[i].Required && !assumed
		if conds[i].Subrequirement == nil {
			setAssumed(conds[i].Group, assumed)
		}
	}
}

// referencedRequirement resolves the requirement or constraint the require/assume
// member decl reference-subsets, and returns nil when the reference names
// anything else or does not resolve.
func (ctx *Context) referencedRequirement(scope *symbols.Scope, decl ast.Node, ref ast.Node) *symbols.Symbol {
	if ctx.model.resolver == nil {
		return nil
	}
	sym, ok := ctx.resolveReferenceTarget(scope, decl, ref)
	if !ok || sym == nil {
		return nil
	}
	if canonical, ok := ctx.resolveAliasTarget(sym); ok {
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

	// element names the checked element in messages. empty takes sym's name,
	// which an anonymous declaration such as a satisfaction assertion lacks.
	element string

	// self is the object a feature name resolves against, nil when unbound.
	self *Instance

	// bindings are the values the element binds by name (subject, actor), and, for a
	// check within a case run, the run's, owned by the case; the zero frame binds nothing.
	bindings frame

	// frames are the frames a check within a behavior run reads, innermost last:
	// the performance's own values and those around it. empty outside a run.
	frames []frame

	// negated inverts the verdict: the element asserts that its required
	// conditions do not all hold (`assert not …`, Invariant::isNegated).
	negated bool

	// about is the requirement whose claim a satisfaction judges; nil reads sym.
	about *symbols.Symbol
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
	if conflict := conflictingResultExpression(conds); conflict != nil {
		return false, fmt.Errorf("%s %s: %s evaluation failed: %s: %w; a redefinition keeps the inherited condition and tightens it with a nested `assert constraint { … }`",
			check.kind, check.name(), check.what, conflictText(conflict), errConflictingResultExpressions)
	}
	// A requirement over no required constraint is true whatever its assumptions
	// read, so they are not evaluated; a constraint stating nothing binds no result.
	if check.isRequirementCheck() && !requiresAny(conds) {
		if check.negated {
			return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: vacuousClaim(check.claimed())}
		}
		return true, nil
	}
	if len(conds) == 0 {
		return false, check.noResultValue("the constraint declares no condition")
	}
	features := ctx.conditionFeatures(check.sym)
	self := check.self
	if self == nil {
		defer ctx.beginExemplars()()
	}
	// One check is one evaluation: its conditions share what a calc usage they
	// read answers, and the next check reads it again.
	activation, endStep := ctx.beginStep()
	defer endStep()
	required := false
	for _, cond := range conds {
		required = required || cond.Required
		if err := ctx.circularCondition(check, cond); err != nil {
			return false, err
		}
		if cond.Steps != nil && !cond.Negated {
			// The body's steps ran once; each condition it left is judged on its
			// own Required, as the conditions of a body stating no steps are.
			done, err := ctx.evaluateStepsConditions(activation, check, cond, features, self)
			if err != nil {
				return false, err
			}
			if done {
				return true, nil
			}
			continue
		}
		holds, err := ctx.conditionHolds(activation, cond, features, self, check.frames, check.bindings)
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
		// A constraint's assumption is trusted rather than checked, so a negated
		// constraint stating only assumptions denies nothing.
		if !required {
			return false, check.noResultValue("the constraint declares no required condition, only assumptions")
		}
		return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: negatedText(conds)}
	}
	return true, nil
}

// noResultValue is the error for a constraint whose result nothing binds (KerML 1.1
// §7.4.8.2): no value, not a verdict, with the unbound-return hint or else cause.
func (check conditionCheck) noResultValue(cause string) error {
	if hint := unboundResultHint([]*symbols.Symbol{check.sym}); hint != "" {
		return fmt.Errorf("%s %s: %w%s", check.kind, check.name(), ErrNoValue, hint)
	}
	return fmt.Errorf("%s %s: %w: %s", check.kind, check.name(), ErrNoValue, cause)
}

// circularCondition is the error for a condition that, reached in order, names
// the requirement being checked; nil for one that does not.
func (ctx *Context) circularCondition(check conditionCheck, cond Condition) error {
	cycle := circularReference([]Condition{cond})
	if cycle == nil {
		return nil
	}
	return check.noResultValue(fmt.Sprintf("require %s names the requirement being checked", cycle.Name))
}

// requiresAny reports whether any of conds is required rather than assumed; a required
// subrequirement requires what its own conditions require, an assumed one nothing.
func requiresAny(conds []Condition) bool {
	for _, cond := range conds {
		if cond.Subrequirement != nil {
			if cond.Required && requiresAny(cond.Group) {
				return true
			}
			continue
		}
		if cond.Required {
			return true
		}
	}
	return false
}

// isRequirementCheck reports whether the checked element is a requirement
// (a requirement, concern, viewpoint or objective usage, or a satisfaction of
// one), whose verdict is Requirements::RequirementConstraintCheck::result.
func (c conditionCheck) isRequirementCheck() bool {
	switch c.kind {
	case "requirement", "satisfaction", "objective":
		return true
	}
	return false
}

// vacuity is why a requirement check over no required condition is true:
// `allTrue(assumptions()) implies allTrue(constraints())` over no constraint is
// true (SysML v2 9.2.11 Requirements; KerML 9.2.3 ControlFunctions::allTrue over
// an empty sequence). "" for a check that is not vacuous.
func vacuity(check conditionCheck, conds []Condition) string {
	if !check.isRequirementCheck() || requiresAny(conds) {
		return ""
	}
	return "vacuous: " + vacuousClaim(check.claimed())
}

// claimed is the requirement whose claim the check judges.
func (c conditionCheck) claimed() *symbols.Symbol {
	if c.about != nil {
		return c.about
	}
	return c.sym
}

// vacuousClaim says what a requirement over no required condition claims.
func vacuousClaim(sym *symbols.Symbol) string {
	return fmt.Sprintf("%s %s states no required constraint, so it holds vacuously", requirementKeyword(sym), sym.Name)
}

// requirementKeyword is the keyword a requirement was declared with
// (`requirement`, `concern`, `viewpoint`, `objective`), `requirement` when it has none.
func requirementKeyword(sym *symbols.Symbol) string {
	if sym != nil {
		switch d := sym.Decl.(type) {
		case *ast.Usage:
			if d.Keyword != "" {
				return d.Keyword
			}
		case *ast.Definition:
			if d.Keyword != "" {
				return d.Keyword
			}
		}
	}
	return "requirement"
}

// circularReference is the requirement a condition of conds names back into the
// check, nil when none does.
func circularReference(conds []Condition) *symbols.Symbol {
	for _, cond := range conds {
		if cond.Circular != nil {
			return cond.Circular
		}
		if nested := circularReference(cond.Group); nested != nil {
			return nested
		}
	}
	return nil
}

// evaluateStepsConditions is the loop of evaluateConditions for one body that
// states steps: the steps run once against the chain's features and bindings,
// then each condition of the body is evaluated in the frame they left — judged
// on its own Required, so an assumption failing among them still denies
// nothing. done reports a verdict reached early, which happens only for a
// negated element, where one required condition failing is the verdict.
func (ctx *Context) evaluateStepsConditions(activation int64, check conditionCheck, cond Condition, features map[string]scopedExpr, self *Instance) (done bool, err error) {
	scoped, bindings := features, check.bindings
	for _, constraint := range cond.Constraints {
		scoped, bindings = ctx.constraintScope(scoped, bindings, constraint)
	}
	stepFrame, err := ctx.runConstraintSteps(cond.Steps, scoped, self, check.frames, bindings)
	if err != nil {
		return false, fmt.Errorf("%s %s: %s evaluation failed: %w", check.kind, check.name(), check.what, err)
	}
	if len(cond.Group) == 0 {
		return false, check.noResultValue("the body's steps leave it no result expression to evaluate")
	}
	for _, sub := range cond.Group {
		if err := ctx.circularCondition(check, sub); err != nil {
			return false, err
		}
		holds, err := ctx.conditionHoldsAt(activation, sub, scoped, self, check.frames, bindings, &stepFrame)
		if err != nil {
			return false, fmt.Errorf("%s %s: %s evaluation failed: %w", check.kind, check.name(), check.what, err)
		}
		if sub.Required && !holds {
			if check.negated {
				return true, nil
			}
			return false, &ViolationError{Kind: check.kind, Element: check.name(), What: check.what, Condition: conditionLabel(sub)}
		}
	}
	return false, nil
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
	} else if ctx.modelConforms(self.Type, owner) {
		return carrier{instance: self, root: self}, nil
	}
	carriers := ctx.carriersUnder(roots, owner)
	switch len(carriers) {
	case 0:
		return carrier{instance: self, root: self}, nil
	case 1:
		return carriers[0], nil
	}
	name := sym.Name
	if name == "" {
		name = "it"
	}
	return carrier{}, fmt.Errorf("%w: %s is carried by %s: check it on one of them",
		ErrAmbiguousSubject, name, strings.Join(ctx.carrierLabels(carriers), ", "))
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

// ConditionCarrier is the object a check of sym is about, resolved as
// evaluation resolves it — self when it carries the element, else the single
// object of this runtime that does — with the same error, ambiguity included,
// an evaluation reports.
func (ctx *Context) ConditionCarrier(kind, element string, sym *symbols.Symbol, self *Instance) (*Instance, error) {
	subject, err := ctx.checkSubject(kind, element, sym, self)
	if err != nil {
		return nil, err
	}
	return subject.instance, nil
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
	return !inst.explicit && ctx.denotesOccurrence(inst)
}

// rootInstances returns the objects this runtime holds that stand on their own,
// in identity order: an object a feature value holds is reached through its holder, and one
// materialized to read a nested declaration through is an occurrence of nothing,
// while an object a caller asked for is a root whatever it materializes. One
// declaration stands as the objects it denotes, every one of them where it denotes
// several, and otherwise as one object, the latest materialized of it.
func (ctx *Context) rootInstances() []*Instance {
	held := ctx.heldObjectIDs()
	denoted := make(map[*symbols.Symbol][]*Instance)
	latest := make(map[*symbols.Symbol]*Instance, len(ctx.instances))
	for _, inst := range ctx.instances {
		if inst == nil || held[inst.ID] || (nestedFeature(inst.Type) && ctx.readThrough(inst)) {
			continue
		}
		// A destroyed object is no subject of anything; what it held is reached from where it stands.
		if ctx.checkNotDestroyed(inst) != nil {
			continue
		}
		if ctx.denotesOccurrence(inst) {
			denoted[inst.Type] = append(denoted[inst.Type], inst)
			continue
		}
		if kept, ok := latest[inst.Type]; ok && kept.ID > inst.ID {
			continue
		}
		latest[inst.Type] = inst
	}
	var out []*Instance
	for sym, inst := range latest {
		if _, ok := denoted[sym]; !ok {
			out = append(out, inst)
		}
	}
	for _, insts := range denoted {
		out = append(out, insts...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LiveRoots returns every object this runtime holds that stands on its own — not held
// by another's feature value, not materialized only to read a nested declaration, not
// destroyed — in identity order: rootInstances without its one-object-per-declaration
// choice, for a sweep that is about each object rather than about a declaration.
func (ctx *Context) LiveRoots() []*Instance {
	held := ctx.heldObjectIDs()
	var out []*Instance
	for _, inst := range ctx.instances {
		if inst == nil || held[inst.ID] || (nestedFeature(inst.Type) && ctx.readThrough(inst)) {
			continue
		}
		if ctx.checkNotDestroyed(inst) != nil {
			continue
		}
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
		destroyed := ctx.checkNotDestroyed(inst) != nil
		for _, fv := range inst.FeatureValues {
			for _, id := range heldObjects(fv.HeldValue()) {
				// A destroyed holder is not walked, so a live object it refers to stands on its own.
				if child, ok := ctx.instances[id]; destroyed && ok && ctx.checkNotDestroyed(child) == nil {
					continue
				}
				held[id] = true
			}
		}
	}
	return held
}

// carriersUnder returns the objects reachable from roots whose type carries the
// features owner declares, roots included, one object standing for each declaration
// reached, in identity order: the subject a check that names a declaration is about,
// so objects a multiplicity repeated are one candidate however deep the named
// declaration sits in them, while two declarations feeding one collection are two.
func (ctx *Context) carriersUnder(roots []*Instance, owner *symbols.Symbol) []carrier {
	return sortedCarriers(oneCarrierPerOccurrence(ctx.reachCarriers(roots, ctx.objectOf(owner))))
}

// objectsUnder returns every object reachable from roots whose type carries the
// features owner declares, roots included, in identity order: each one is an object a
// feature owner declares is a feature of, the objects a multiplicity repeated each on their own.
func (ctx *Context) objectsUnder(roots []*Instance, owner *symbols.Symbol) []carrier {
	return ctx.carriersWhere(roots, ctx.objectOf(owner))
}

// objectOf chooses the objects whose type conforms to owner.
func (ctx *Context) objectOf(owner *symbols.Symbol) func(*Instance) bool {
	return func(inst *Instance) bool { return ctx.modelConforms(inst.Type, owner) }
}

// carriersWhere is objectsUnder with the carrying objects chosen by carries.
func (ctx *Context) carriersWhere(roots []*Instance, carries func(*Instance) bool) []carrier {
	return sortedCarriers(ctx.reachCarriers(roots, carries))
}

// reachCarriers walks the objects reachable from roots, roots included, and returns
// each one carries chooses in the order reached, with the declaration it occurs as; a
// destroyed object is passed over with what it holds. A declaration is descended into
// once per path, so recursive composition is a finite search.
func (ctx *Context) reachCarriers(roots []*Instance, carries func(*Instance) bool) []carrier {
	var out []carrier
	seen := make(map[int64]bool, len(roots))
	path := make(map[*symbols.Symbol]bool)
	var descend func(root, inst *Instance, through string, features []string)
	descend = func(root, inst *Instance, through string, features []string) {
		// A destroyed object a live holder retains is no carrier, and holds nothing to walk.
		if inst == nil || seen[inst.ID] || ctx.checkNotDestroyed(inst) != nil {
			return
		}
		seen[inst.ID] = true
		if carries(inst) {
			occurrence := carrierOccurrence{through: through, decl: inst.Type}
			out = append(out, carrier{instance: inst, root: root, features: features, occurrence: occurrence})
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
		descend(root, root, ctx.rootPath(root), nil)
	}
	return out
}

// oneCarrierPerOccurrence keeps the first carrier reached of each declaration occurrence.
func oneCarrierPerOccurrence(carriers []carrier) []carrier {
	out := make([]carrier, 0, len(carriers))
	declared := make(map[carrierOccurrence]bool, len(carriers))
	for _, c := range carriers {
		if !declared[c.occurrence] {
			declared[c.occurrence] = true
			out = append(out, c)
		}
	}
	return out
}

// sortedCarriers orders carriers by object identity.
func sortedCarriers(carriers []carrier) []carrier {
	sort.Slice(carriers, func(i, j int) bool { return carriers[i].instance.ID < carriers[j].instance.ID })
	return carriers
}

// rootPath is the path a subject search starts a root at: the objects one usage denotes share
// its name, as objects a multiplicity repeated do, while any other root is its own.
func (ctx *Context) rootPath(root *Instance) string {
	if ctx.denotesOccurrence(root) {
		return strconv.Quote(ctx.QualifiedName(root.Type))
	}
	return strconv.FormatInt(root.ID, 10)
}

// carrier is an object a search reached: the object the search started from and
// the features walked from it name a nested one, which has no name of its own.
type carrier struct {
	instance   *Instance
	root       *Instance
	features   []string
	occurrence carrierOccurrence
}

// carrierOccurrence identifies the declaration an object occurs as: the features
// walked through to reach it, each quoted so any name is one segment, and the
// declaration it materializes. Objects a multiplicity repeated share both, while
// ones a collection gathers from different declarations — the features
// subsetting it — do not. A check about a declaration resolves one object per
// occurrence; a sweep over the objects of a type visits every one.
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
// yields no object: one that is not there is no subject either.
func (ctx *Context) nestedObjects(inst *Instance) []heldObject {
	out, _ := ctx.heldObjectsOf(inst, nil, false, holdsObjects)
	return out
}

// heldObjectsOf is nestedObjects reading a feature as `through` reads it (every one, when nil),
// taking what a feature it leaves unread already holds; a failed read is skipped or, where every
// object counts, is the error.
func (ctx *Context) heldObjectsOf(inst *Instance, through func(*Instance, ObjectFeature) (*FeatureValue, error), everyObject bool, holds func(*EffectiveFeature) bool) ([]heldObject, error) {
	var out []heldObject
	read := map[*FeatureValue]bool{}
	for _, of := range ctx.FeaturesOfObject(inst) {
		if of.Name == "" || !holds(of.Feature) {
			continue
		}
		var fv *FeatureValue
		var err error
		if through == nil {
			fv, err = inst.GetFeatureValue(ctx, of.Name)
		} else {
			fv, err = through(inst, of)
		}
		if err != nil {
			if everyObject {
				return nil, fmt.Errorf("feature %s: %w", of.Name, err)
			}
			continue
		}
		if fv == nil {
			if fv = inst.FeatureValues[of.Name]; fv == nil || !fv.Materialized {
				continue
			}
		}
		if read[fv] {
			continue
		}
		read[fv] = true
		held, err := ctx.heldInFull(fv.HeldValue())
		if err != nil {
			if everyObject {
				return nil, fmt.Errorf("feature %s: %w", of.Name, err)
			}
			held = NewSequenceValue(&Sequence{elements: standingElements(fv.HeldValue())})
		}
		for _, id := range heldObjects(held) {
			if child, ok := ctx.instances[id]; ok {
				out = append(out, heldObject{feature: of.Name, instance: child})
			}
		}
	}
	return out, nil
}

// holdsObjects reports whether a feature holds objects rather than values: a
// nested part has features and conditions of its own, an attribute has neither.
func holdsObjects(feat *EffectiveFeature) bool {
	return objectFeature(feat.Symbol)
}

// objectFeature reports whether sym declares a feature whose values are objects.
func objectFeature(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	return objectUsageKind(usage.Kind) || usage.Kind == ast.UsagePort
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
		elements = standingElements(val)
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
		parts[i] = source.NameText(name)
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
func (ctx *Context) conditionHolds(activation int64, cond Condition, features map[string]scopedExpr, self *Instance, frames []frame, bindings frame) (bool, error) {
	return ctx.conditionHoldsAt(activation, cond, features, self, frames, bindings, nil)
}

// conditionHoldsAt is conditionHolds evaluated in the state stepFrame left when
// the body's statements ran — innermost of the frames read. A condition stating
// steps runs them first, replacing stepFrame for the group that follows them.
func (ctx *Context) conditionHoldsAt(activation int64, cond Condition, features map[string]scopedExpr, self *Instance, frames []frame, bindings frame, stepFrame *frame) (bool, error) {
	if stepFrame != nil && (len(cond.Constraints) > 0 || cond.Steps != nil) {
		// A nested body is its own performance: the outer body's step frame
		// encloses it read-only, shadowed by its bindings and its own steps.
		frames = append(append([]frame{}, frames...), *stepFrame)
		stepFrame = nil
	}
	for _, constraint := range cond.Constraints {
		features, bindings = ctx.constraintScope(features, bindings, constraint)
	}
	if cond.Steps != nil {
		left, err := ctx.runConstraintSteps(cond.Steps, features, self, frames, bindings)
		if err != nil {
			return false, err
		}
		stepFrame = &left
		if len(cond.Group) == 0 {
			return false, fmt.Errorf("%w: the body's steps leave it no result expression to evaluate", ErrNoValue)
		}
	}
	holds := true
	if cond.Subrequirement != nil {
		var err error
		if holds, err = ctx.subrequirementHoldsAt(activation, cond, features, self, frames, bindings, stepFrame); err != nil {
			return false, err
		}
	} else if cond.Group != nil {
		for _, sub := range cond.Group {
			subHolds, err := ctx.conditionHoldsAt(activation, sub, features, self, frames, bindings, stepFrame)
			if err != nil {
				return false, err
			}
			holds = holds && subHolds
		}
	} else {
		ec := NewEvalContextIn(ctx, cond.Scope, self)
		ec.activation = activation
		ec.features = features
		for _, f := range frames {
			ec.pushFrame(f)
		}
		if bindings.vars != nil {
			ec.pushFrame(bindings)
		}
		if stepFrame != nil {
			ec.pushFrame(*stepFrame)
		}
		result, err := ec.Eval(cond.Expr)
		if err != nil {
			return false, err
		}
		if u := result.Undetermined(); u != nil {
			return false, fmt.Errorf("%w: condition is undetermined: %s", ErrNoValue, u.Reason())
		}
		result = soleElement(result)
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

// subrequirementHoldsAt is a subrequirement's RequirementCheck::result within its container's
// check: `allTrue(assumptions()) implies allTrue(constraints())` over its group, true over no required one.
func (ctx *Context) subrequirementHoldsAt(activation int64, cond Condition, features map[string]scopedExpr, self *Instance, frames []frame, bindings frame, stepFrame *frame) (bool, error) {
	for _, required := range []bool{false, true} {
		for _, sub := range cond.Group {
			if sub.Required != required {
				continue
			}
			holds, err := ctx.conditionHoldsAt(activation, sub, features, self, frames, bindings, stepFrame)
			if err != nil {
				return false, err
			}
			if !holds {
				return !required, nil
			}
		}
	}
	return true, nil
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
		out[feat.Name] = scopedExpr{expr: expr, scope: feat.DefaultScope(), decl: feat.DefaultDecl, held: feat.Symbol}
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
	if names, value, ok := ctx.subrequirementSubject(constraint, bindings); ok {
		vars := make(map[string]Value, len(inner.bindings.vars)+len(names))
		for name, v := range inner.bindings.vars {
			vars[name] = v
		}
		for _, name := range names {
			vars[name] = value
		}
		inner.bindings = inner.bindings.withVars(vars)
	}
	for name, feat := range own {
		feat.env = inner
		if isArgument(feat.decl) {
			feat.env = enclosing
		}
		out[name] = feat
	}
	return out, inner.bindings
}

// subrequirementSubject is the container's subject value a subrequirement owning no subject declaration
// binds under its inherited subject names (SysML v2 §7.21.2); ok is false otherwise or when it is unbound.
func (ctx *Context) subrequirementSubject(sub *symbols.Symbol, bindings frame) (names []string, value Value, ok bool) {
	if !ctx.model.semantics.IsSubrequirement(sub) {
		return nil, Value{}, false
	}
	if owned, _ := ctx.model.semantics.SubjectsOf(sub); len(owned) > 0 {
		return nil, Value{}, false
	}
	names = ctx.subjectParameterNames(sub)
	if len(names) == 0 {
		return nil, Value{}, false
	}
	for _, name := range ctx.subjectParameterNames(declaringType(sub)) {
		if value, ok := bindings.lookup(name); ok {
			return names, value, true
		}
	}
	return nil, Value{}, false
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
	if cond.Steps != nil {
		parts := make([]string, 0, len(cond.Group))
		for _, sub := range cond.Group {
			parts = append(parts, conditionLabel(sub))
		}
		if len(parts) == 0 {
			return "the body's steps"
		}
		text := "the body's steps then { " + strings.Join(parts, "; ") + " }"
		if cond.Negated {
			text = "not " + text
		}
		return text
	}
	if cond.Conflict != nil {
		return "conflicting result expression"
	}
	if cond.Circular != nil {
		return "require " + cond.Circular.Name
	}
	text := conditionText(cond.Expr)
	if cond.Group != nil {
		parts := make([]string, 0, len(cond.Group))
		for _, sub := range cond.Group {
			if cond.Subrequirement != nil && !sub.Required {
				continue
			}
			parts = append(parts, conditionLabel(sub))
		}
		text = "{ " + strings.Join(parts, "; ") + " }"
		if cond.Subrequirement != nil {
			text = requirementKeyword(cond.Subrequirement) + " " + cond.Subrequirement.Name + " " + text
		}
	}
	if cond.Negated {
		text = "not " + text
	}
	return text
}

// ConditionText is the text a NoValueError names the expression n by.
func ConditionText(n ast.Node) string { return conditionText(n) }

// qualifiedNameText writes a qualified name as the notation does, each segment
// quoted when it needs quoting.
func qualifiedNameText(qn *ast.QualifiedName) string {
	if qn == nil {
		return ""
	}
	names := make([]string, 0, len(qn.Parts))
	for _, seg := range qn.Parts {
		if seg.Text != "" {
			names = append(names, seg.Text)
		}
	}
	return source.QualifiedNameOf(names)
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
		return qualifiedNameText(e.Name)
	case *ast.FeatureChainExpr:
		return conditionText(e.Operand) + "." + qualifiedNameText(e.Member)
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
		return qualifiedNameText(e.Type) + "(" + strings.Join(args, ", ") + ")"
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
	return traceLabel(n)
}
