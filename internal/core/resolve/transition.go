package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// CodeNotAVertex marks an endpoint naming an element no transition can start or
// end at (SysML 7.19.2).
const CodeNotAVertex = "not-a-vertex"

// ResolveEndpoint resolves the vertex a transition endpoint names, reporting one
// that names nothing or no vertex. A nil name is the sourceless form: legal.
func (r *Resolver) ResolveEndpoint(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if qn == nil || len(qn.Parts) == 0 {
		return nil, false
	}
	if res, done := r.endpoints[qn]; done {
		return res.sym, res.ok
	}
	sym, ok := r.lookupEndpoint(scope, qn)
	res := resolution{sym: sym, ok: ok}
	switch {
	case completionEndpoint(scope, qn, sym):
		// `then done;` names the end shot every state inherits, which completes the
		// machine: no vertex of its own, so lowering supplies one.
		res = resolution{}
	case !ok:
		last := qn.Parts[len(qn.Parts)-1]
		r.report(Diagnostic{
			Span:    qn.Span(),
			Message: r.endpointMessage(scope, qn),
			Code:    "unresolved",
			// A suggestion names a vertex, so it replaces the last segment alone,
			// leaving the qualifiers saying which state or region it lives in.
			Fixes: endpointFixes(last.Text, last.Span, r.vertexSuggestions(scope, qn)),
		})
	case stateMachineEndpoint(scope) && !r.endpointIsVertex(scope, qn, sym):
		r.report(Diagnostic{
			Span:    qn.Span(),
			Message: "transition endpoint " + qnText(qn) + " is not a state or pseudostate",
			Code:    CodeNotAVertex,
		})
		res = resolution{}
	}
	if res.ok || r.quiet == 0 {
		journalNew(r, r.endpoints, qn, qn)
		r.endpoints[qn] = res
	}
	return res.sym, res.ok
}

// EndpointSymbol returns the resolved symbol for a transition endpoint.
func (r *Resolver) EndpointSymbol(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	var sym *symbols.Symbol
	var ok bool
	r.aside(func() { sym, ok = r.ResolveEndpoint(scope, qn) })
	return sym, ok
}

// Endpoint returns the declaration an endpoint names, which lowering builds its
// edges from (lower.EndpointResolver); the lookup itself reports nothing, since
// this tier reports an endpoint naming no vertex when it resolves the document.
func (r *Resolver) Endpoint(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	var sym *symbols.Symbol
	var ok bool
	r.aside(func() { sym, ok = r.ResolveEndpoint(scope, qn) })
	if ok && sym != nil {
		return sym.Decl, true
	}
	return nil, false
}

// VertexInScope finds the vertex an endpoint names from the scope tree alone,
// innermost scope first, for a machine lowered without a resolver over its
// document (lower.ToStateGraph): no index is consulted, so nothing outside the
// scopes the endpoint was written in can answer it.
func VertexInScope(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, bool) {
	if scope == nil || qn == nil || len(qn.Parts) == 0 {
		return nil, false
	}
	parts := qualifiedParts(qn)
	machine := machineScope(scope)
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := firstVertex(s, parts); ok {
			return sym.Decl, true
		}
		if s == machine {
			break
		}
	}
	// A vertex a state inherits from the definition typing it is a member of that
	// state, so an endpoint may name it, but no scope of the machine holds it.
	if sym, ok := inheritedVertexInScope(scope, parts); ok {
		return sym.Decl, true
	}
	return nil, false
}

// ActionNodeInScope finds an inherited action node from the scope tree alone.
// The final bool reports an unresolved generalization that makes absence uncertain.
func ActionNodeInScope(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, *symbols.Scope, bool, bool) {
	if scope == nil || qn == nil || len(qn.Parts) == 0 {
		return nil, nil, false, false
	}
	body := enclosingActionScope(scope)
	if body == nil {
		return nil, nil, false, false
	}
	name := qn.Parts[len(qn.Parts)-1].Text
	if len(qn.Parts) == 1 {
		if _, ok := body.LookupLocal(name); ok {
			return nil, nil, false, false
		}
	}
	var qualifier *symbols.Symbol
	if len(qn.Parts) > 1 {
		var ok bool
		qualifier, ok = lookupScopeParts(body.Parent(), qn.Parts[:len(qn.Parts)-1])
		if !ok {
			return nil, nil, false, false
		}
	}
	return inheritedActionNode(body, name, qualifier, make(map[*symbols.Symbol]bool))
}

// ActionNodeOfBody finds the action node a name written in an action's body
// names there: one the body declares, else one its declaration inherits.
func ActionNodeOfBody(body *symbols.Scope, name string) (ast.Node, bool) {
	if body == nil || name == "" {
		return nil, false
	}
	if member, ok := body.LookupLocal(name); ok {
		return member.Decl, isActionNode(member.Decl)
	}
	decl, _, found, _ := inheritedActionNode(body, name, nil, make(map[*symbols.Symbol]bool))
	return decl, found
}

// RedefinesActionNode reports whether node, declared in body, redefines decl —
// directly or through a node it redefines that does — from the scope tree alone.
func RedefinesActionNode(body *symbols.Scope, node *ast.Usage, decl ast.Node) bool {
	return redefinesActionNode(body, node, decl, make(map[*ast.Usage]bool))
}

func redefinesActionNode(body *symbols.Scope, node *ast.Usage, decl ast.Node, seen map[*ast.Usage]bool) bool {
	if body == nil || node == nil || seen[node] {
		return false
	}
	seen[node] = true
	for _, rel := range node.Relationships {
		if rel == nil || rel.Kind != ast.RelRedefines {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok || len(qn.Parts) == 0 {
			continue
		}
		var qualifier *symbols.Symbol
		if len(qn.Parts) > 1 {
			if qualifier, ok = lookupScopeParts(body.Parent(), qn.Parts[:len(qn.Parts)-1]); !ok {
				continue
			}
		}
		name := qn.Parts[len(qn.Parts)-1].Text
		redefined, redefinedBody, found, _ := inheritedActionNode(body, name, qualifier, make(map[*symbols.Symbol]bool))
		if !found {
			continue
		}
		if redefined == decl {
			return true
		}
		if u, ok := redefined.(*ast.Usage); ok && redefinesActionNode(redefinedBody, u, decl, seen) {
			return true
		}
	}
	return false
}

func enclosingActionScope(scope *symbols.Scope) *symbols.Scope {
	for s := scope; s != nil; s = s.Parent() {
		switch n := s.Node().(type) {
		case *ast.Definition:
			if n.Kind == ast.DefAction {
				return s
			}
		case *ast.Usage:
			if n.Kind == ast.UsageAction {
				return s
			}
		}
	}
	return nil
}

// ActionGeneralBodies returns the bodies of the actions the action enclosing
// scope specializes or is typed by, nearest first and each once, found from the
// scope tree alone — where a member the action inherits was declared.
func ActionGeneralBodies(scope *symbols.Scope) []*symbols.Scope {
	body := enclosingActionScope(scope)
	if body == nil {
		return nil
	}
	var bodies []*symbols.Scope
	seen := make(map[*symbols.Symbol]bool)
	var collect func(body *symbols.Scope)
	collect = func(body *symbols.Scope) {
		supers, _ := declaredGenerals(body)
		for _, super := range supers {
			if seen[super] || super.Scope == nil {
				continue
			}
			seen[super] = true
			bodies = append(bodies, super.Scope)
			collect(super.Scope)
		}
	}
	collect(body)
	return bodies
}

// declaredGenerals returns the symbols the generalizations body's declaration
// states, in declaration order, and whether one of them resolves to nothing.
func declaredGenerals(body *symbols.Scope) ([]*symbols.Symbol, bool) {
	var rels []*ast.Relationship
	switch n := body.Node().(type) {
	case *ast.Definition:
		rels = n.Relationships
	case *ast.Usage:
		rels = n.Relationships
	default:
		return nil, false
	}
	var supers []*symbols.Symbol
	var uncertain bool
	for _, rel := range rels {
		if rel == nil || (rel.Kind != ast.RelSpecializes && rel.Kind != ast.RelTyping &&
			rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines) {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		super, ok := lookupScopeQualified(body.Parent(), qn)
		if !ok || super == nil {
			uncertain = true
			continue
		}
		supers = append(supers, super)
	}
	return supers, uncertain
}

func inheritedActionNode(body *symbols.Scope, name string, qualifier *symbols.Symbol, seen map[*symbols.Symbol]bool) (ast.Node, *symbols.Scope, bool, bool) {
	supers, uncertain := declaredGenerals(body)
	for _, super := range supers {
		if qualifier != nil && super != qualifier {
			continue
		}
		if seen[super] {
			continue
		}
		seen[super] = true
		superBody := super.Scope
		if superBody == nil {
			continue
		}
		if member, found := superBody.LookupLocal(name); found {
			if isActionNode(member.Decl) {
				return member.Decl, superBody, true, uncertain
			}
			continue
		}
		found, foundScope, inherited, unknown := inheritedActionNode(superBody, name, qualifier, seen)
		if inherited {
			return found, foundScope, true, uncertain || unknown
		}
		uncertain = uncertain || unknown
	}
	return nil, nil, false, uncertain
}

// FeatureSymbolInScope resolves a dotted feature path from the scope tree
// alone, innermost scope first, so a local declaration shadows an outer one —
// for lowering a send target without a resolver over its document. At each
// scope, a member the scope's declaration inherits from its generalizations
// counts as the scope's own, so an inherited port shadows like a declared one.
func FeatureSymbolInScope(scope *symbols.Scope, segments []string) (*symbols.Symbol, bool) {
	if scope == nil || len(segments) == 0 {
		return nil, false
	}
	for s := scope; s != nil; s = s.Parent() {
		sym, ok := s.LookupLocal(segments[0])
		if !ok {
			sym, ok = inheritedFeature(s, segments[0], map[*symbols.Symbol]bool{})
		}
		if !ok {
			continue
		}
		for _, part := range segments[1:] {
			sym, ok = featureMember(sym, part)
			if !ok {
				sym = nil
				break
			}
		}
		if sym != nil {
			return sym, true
		}
	}
	return nil, false
}

// featureMember finds a member of the feature sym names: declared in sym's own
// body, or reached through the type or generalizations its declaration states.
func featureMember(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	if sym == nil || sym.Scope == nil {
		return nil, false
	}
	if member, ok := sym.Scope.LookupLocal(name); ok {
		return member, true
	}
	return inheritedFeature(sym.Scope, name, map[*symbols.Symbol]bool{})
}

// inheritedFeature finds a member named name among the members body's
// declaration inherits from the generalizations the scope tree resolves.
func inheritedFeature(body *symbols.Scope, name string, seen map[*symbols.Symbol]bool) (*symbols.Symbol, bool) {
	var rels []*ast.Relationship
	switch n := body.Node().(type) {
	case *ast.Definition:
		rels = n.Relationships
	case *ast.Usage:
		rels = n.Relationships
	default:
		return nil, false
	}
	for _, rel := range rels {
		if rel == nil || (rel.Kind != ast.RelSpecializes && rel.Kind != ast.RelTyping &&
			rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines) {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		super, found := lookupScopeQualified(body.Parent(), qn)
		if !found || super == nil || seen[super] {
			continue
		}
		seen[super] = true
		superBody := super.Scope
		if superBody == nil {
			continue
		}
		if member, held := superBody.LookupLocal(name); held {
			return member, true
		}
		if member, held := inheritedFeature(superBody, name, seen); held {
			return member, true
		}
	}
	return nil, false
}

func lookupScopeParts(scope *symbols.Scope, parts []ast.NameSegment) (*symbols.Symbol, bool) {
	if scope == nil || len(parts) == 0 {
		return nil, false
	}
	partsText := make([]string, len(parts))
	for i, part := range parts {
		partsText[i] = part.Text
	}
	return lookupScopePartsText(scope, partsText)
}

func lookupScopeQualified(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	if scope == nil || qn == nil || len(qn.Parts) == 0 {
		return nil, false
	}
	parts := qualifiedParts(qn)
	return lookupScopePartsText(scope, parts)
}

func lookupScopePartsText(scope *symbols.Scope, parts []string) (*symbols.Symbol, bool) {
	if scope == nil || len(parts) == 0 {
		return nil, false
	}
	for s := scope; s != nil; s = s.Parent() {
		sym, ok := s.LookupLocal(parts[0])
		if !ok {
			continue
		}
		for _, part := range parts[1:] {
			if sym.Scope == nil {
				sym = nil
				break
			}
			sym, ok = sym.Scope.LookupLocal(part)
			if !ok {
				sym = nil
				break
			}
		}
		if sym != nil {
			return sym, true
		}
	}
	return nil, false
}

func isActionNode(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.Usage:
		return n.Kind == ast.UsageAction
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode,
		*ast.ActionExecutionNode, *ast.WhileLoopActionNode, *ast.IfActionNode,
		*ast.AssignmentActionNode, *ast.SendStatement, *ast.TerminateStatement:
		return true
	}
	return false
}

// lookupEndpoint finds what an endpoint names: the declaration ordinary lookup
// reaches, or else a vertex of the enclosing machine, which a transition may
// name across a region or into a nested state (`TransitionUsage::source` and
// `::target : ActionUsage[1..1]`, stdlib `Systems Library/SysML.sysml`).
func (r *Resolver) lookupEndpoint(scope *symbols.Scope, qn *ast.QualifiedName) (*symbols.Symbol, bool) {
	var sym *symbols.Symbol
	var ok bool
	r.aside(func() { sym, ok = r.resolveQualified(scope, qn, nil) })
	machine := machineScope(scope)
	if ok && r.endpointIsVertex(scope, qn, sym) && declaredWithin(machine, sym) {
		return sym, true
	}
	// A vertex of the machine itself outranks one the name reaches elsewhere, so an
	// inherited member does not shadow the state declared beside the transition.
	if vertex, found := firstVertex(machine, qualifiedParts(qn)); found {
		return vertex, true
	}
	if ok && r.endpointIsVertex(scope, qn, sym) {
		return sym, true
	}
	return sym, ok
}

// completionEndpoint reports whether an endpoint names the end shot every state
// inherits — the unqualified `done` — rather than a vertex the machine declares.
func completionEndpoint(scope *symbols.Scope, qn *ast.QualifiedName, sym *symbols.Symbol) bool {
	if qn == nil || len(qn.Parts) != 1 || qn.Parts[0].Text != ast.DoneFeature {
		return false
	}
	return stateMachineEndpoint(scope) && !declaredWithin(machineScope(scope), sym)
}

// declaredWithin reports whether sym is declared in scope or one of the scopes
// under it, which for a machine's body is what makes a vertex the machine's own.
func declaredWithin(scope *symbols.Scope, sym *symbols.Symbol) bool {
	if scope == nil || sym == nil {
		return false
	}
	for _, candidate := range scope.LookupLocalAll(sym.Name) {
		if candidate == sym {
			return true
		}
	}
	for _, child := range scope.Children() {
		if declaredWithin(child, sym) {
			return true
		}
	}
	return false
}

func (r *Resolver) endpointIsVertex(scope *symbols.Scope, qn *ast.QualifiedName, sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if IsVertex(sym.Decl) {
		return true
	}
	if startAction(sym) {
		return true
	}
	if !stateMachineEndpoint(scope) {
		return false
	}
	return r.machineStateVertex(scope, qn, sym)
}

func isVertexSymbol(sym *symbols.Symbol) bool {
	return sym != nil && IsVertex(sym.Decl)
}

func (r *Resolver) machineStateVertex(scope *symbols.Scope, qn *ast.QualifiedName, sym *symbols.Symbol) bool {
	if !isVertexSymbol(sym) || r.model == nil || r.idx == nil || qn == nil || len(qn.Parts) == 0 {
		return false
	}
	machineScope := machineScope(scope)
	if machineScope == nil {
		return false
	}
	machine := machineScope.Owner()
	if machine == nil {
		return false
	}
	name := qn.Parts[len(qn.Parts)-1].Text
	member, ok := r.model.LookupMember(machine, name)
	if !ok || member == nil {
		return false
	}
	return member == sym || r.idx.GetFQN(member) == r.idx.GetFQN(sym)
}

// MachineStateVertex reports whether sym is a vertex of the state machine
// enclosing scope, using the same inherited-member predicate as endpoint
// resolution.
func (r *Resolver) MachineStateVertex(scope *symbols.Scope, qn *ast.QualifiedName, sym *symbols.Symbol) bool {
	return r.machineStateVertex(scope, qn, sym)
}

func stateMachineEndpoint(scope *symbols.Scope) bool {
	for s := scope; s != nil; s = s.Parent() {
		switch n := s.Node().(type) {
		case *ast.Definition:
			switch n.Kind {
			case ast.DefState:
				return true
			case ast.DefAction:
				return false
			}
		case *ast.Usage:
			switch n.Kind {
			case ast.UsageState:
				return true
			case ast.UsageAction:
				return false
			}
		}
	}
	return true
}

// endpointMessage is what an endpoint resolving to nothing reports: the ordinary
// unresolved wording, hinting the vertices of the machine it was written in.
func (r *Resolver) endpointMessage(scope *symbols.Scope, qn *ast.QualifiedName) string {
	name := qnText(qn)
	return suggest.With(unresolvedReferencePrefix+name, name, r.vertexSuggestions(scope, qn))
}

// vertexSuggestions ranks the vertex names of the enclosing machine near the
// endpoint's spelling: a misspelled endpoint means a vertex of that machine.
func (r *Resolver) vertexSuggestions(scope *symbols.Scope, qn *ast.QualifiedName) []string {
	parts := qualifiedParts(qn)
	return suggest.Nearest(parts[len(parts)-1], vertexNames(machineScope(scope)))
}

// endpointFixes offers each suggested vertex as an edit replacing the endpoint.
func endpointFixes(name string, span source.Span, cands []string) []quickfix.Fix {
	if span.Len == 0 {
		return nil
	}
	fixes := make([]quickfix.Fix, 0, len(cands))
	for _, cand := range cands {
		if cand == name {
			continue
		}
		fixes = append(fixes, quickfix.Fix{
			Title:     "Change '" + name + "' to '" + cand + "'",
			Edits:     []quickfix.Edit{quickfix.Replace(span, cand)},
			Preferred: len(cands) == 1,
		})
	}
	if len(fixes) == 0 {
		return nil
	}
	return fixes
}

// startAction reports whether sym is an entry action of the state declaring it,
// which stands in for a start pseudostate: `transition initial then off;`.
func startAction(sym *symbols.Symbol) bool {
	if sym == nil || sym.OwnerScope == nil {
		return false
	}
	return ast.IsEntryAction(ast.StateEntryActions(sym.OwnerScope.Node()), sym.Decl)
}

// IsVertex reports whether decl is a vertex a transition may name: a state, a
// pseudostate, or a control node standing in for one (SysML 7.19.2).
func IsVertex(decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.StateNode, *ast.SubstateMember, *ast.PseudostateNode, *ast.InitialNode, *ast.FinalNode:
		return true
	case *ast.Usage:
		return d.Kind == ast.UsageState
	}
	return false
}

// machineScope returns the scope of the machine around scope: the outermost
// state, region or machine body enclosing it.
func machineScope(scope *symbols.Scope) *symbols.Scope {
	machine := scope
	for s := scope; s != nil; s = s.Parent() {
		if !stateBody(s.Node()) {
			break
		}
		machine = s
	}
	return machine
}

// stateBody reports whether node's body belongs to a state machine: the machine
// itself, one of its states, a region, or a transition declared in one.
func stateBody(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.StateNode, *ast.SubstateMember, *ast.StateRegion, *ast.PseudostateNode, *ast.TransitionMember:
		return true
	case *ast.Usage:
		return n.Kind == ast.UsageState || n.Kind == ast.UsageTransition
	case *ast.Definition:
		return n.Kind == ast.DefState
	}
	return false
}

// firstVertex searches scope's subtree, outermost and in declaration order, for
// a vertex whose name path ends in parts, or an entry action standing in for a
// start pseudostate.
func firstVertex(scope *symbols.Scope, parts []string) (*symbols.Symbol, bool) {
	if scope == nil || len(parts) == 0 {
		return nil, false
	}
	name := parts[len(parts)-1]
	for _, key := range scope.MemberNames() {
		if key != name {
			continue
		}
		if !ownedBy(scope, parts[:len(parts)-1]) {
			continue
		}
		for _, sym := range symbols.PreferDeclared(scope.LookupLocalAll(key)) {
			if IsVertex(sym.Decl) || startAction(sym) {
				return sym, true
			}
		}
	}
	for _, child := range scope.Children() {
		if sym, ok := firstVertex(child, parts); ok {
			return sym, true
		}
	}
	return nil, false
}

// ownedBy reports whether the scopes enclosing scope are named by owners,
// innermost last: `outer::inner` need not name the scopes between them.
func ownedBy(scope *symbols.Scope, owners []string) bool {
	remaining := len(owners)
	for s := scope; s != nil && remaining > 0; s = s.Parent() {
		if scopeName(s) == owners[remaining-1] {
			remaining--
		}
	}
	return remaining == 0
}

// scopeName is the name of the declaration owning scope, empty when it has none.
func scopeName(scope *symbols.Scope) string {
	if owner := scope.Owner(); owner != nil {
		return owner.Name
	}
	return ast.SimpleName(scope.Node())
}

// vertexNames lists the distinct names of the vertices declared in scope's
// subtree: a name declared in two regions is one suggestion, not two.
func vertexNames(scope *symbols.Scope) []string {
	var names []string
	seen := map[string]bool{}
	var walk func(scope *symbols.Scope)
	walk = func(scope *symbols.Scope) {
		if scope == nil {
			return
		}
		for _, key := range scope.MemberNames() {
			for _, sym := range scope.LookupLocalAll(key) {
				if IsVertex(sym.Decl) && !seen[key] {
					seen[key] = true
					names = append(names, key)
					break
				}
			}
		}
		for _, child := range scope.Children() {
			walk(child)
		}
	}
	walk(scope)
	return names
}

// qualifiedParts is the segments of a qualified name as written.
func qualifiedParts(qn *ast.QualifiedName) []string {
	parts := make([]string, len(qn.Parts))
	for i, part := range qn.Parts {
		parts[i] = part.Text
	}
	return parts
}
