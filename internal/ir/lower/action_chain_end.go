package lower

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ChainEnd classifies a succession end written as a feature chain whose first
// segment names no node of the flow (SysML v2 §7.17.4 admits a chain at either end).
type ChainEnd int

const (
	// ChainEndNone is an end that is no such chain, or one the scope cannot follow.
	ChainEndNone ChainEnd = iota
	// ChainEndAction reaches an action usage: a step performing it on the object reached.
	ChainEndAction
	// ChainEndOccurrence reaches an occurrence usage that is no action: legal
	// notation (§7.13.5) that orders nothing the flow performs.
	ChainEndOccurrence
	// ChainEndValue reaches an attribute or enumeration usage, which is no occurrence.
	ChainEndValue
)

// ClassifyChainEnd says what a succession end written in scope reaches when it
// is a feature chain rooted outside nodes, and the feature it reaches.
func ClassifyChainEnd(nodes []ast.Node, scope *symbols.Scope, resolver *resolve.Resolver, ref ast.Node) (ChainEnd, *symbols.Symbol) {
	if !isChainEnd(ref) || findNodeByReference(nodes, ref) != nil {
		return ChainEndNone, nil
	}
	sym := chainFeature(scope, resolver, ref)
	if sym == nil {
		return ChainEndNone, nil
	}
	usage, isUsage := sym.Decl.(*ast.Usage)
	switch {
	case !isUsage:
		return ChainEndNone, nil
	case isActionUsageNode(usage):
		return ChainEndAction, sym
	case usage.Kind == ast.UsageAttribute || usage.Kind == ast.UsageEnumeration:
		return ChainEndValue, sym
	}
	return ChainEndOccurrence, sym
}

// chainFeature is the feature a chain written in scope names: by the scope tree,
// else by the resolver, which follows the imports a type on the way is read through.
func chainFeature(scope *symbols.Scope, resolver *resolve.Resolver, ref ast.Node) *symbols.Symbol {
	if sym, ok := resolve.FeatureSymbolInScope(scope, endSegments(ref)); ok && sym != nil {
		return sym
	}
	if resolver == nil {
		return nil
	}
	sym, _ := resolver.ResolveTarget(scope, ref)
	return sym
}

// chainKey names the action a chain reaches: its path from the feature it is rooted at.
type chainKey struct {
	root *symbols.Symbol
	path string
}

// chainKeyIn is the key of a chain written in scope. A root a general declares is
// the action's own feature where the action redefines it.
func (g *ActionGraph) chainKeyIn(scope *symbols.Scope, ref ast.Node) chainKey {
	segments := endSegments(ref)
	key := chainKey{path: strings.Join(segments, "\x00")}
	if len(segments) == 0 {
		return key
	}
	key.root = chainFeature(scope, g.resolver, ast.QualifiedNameOf(segments[0]))
	own, ok := resolve.FeatureSymbolInScope(g.Scope, segments[:1])
	if !ok || own == nil || own == key.root || key.root == nil {
		return key
	}
	var r redefinitionResolver
	if g.resolver != nil {
		r = g.resolver
	}
	if usage, isUsage := own.Decl.(*ast.Usage); isUsage && redefinesByClause(r, g.DeclScope(usage), usage, key.root) {
		key.root = own
	}
	return key
}

// isChainEnd reports whether an end is written with the feature chain dot.
func isChainEnd(ref ast.Node) bool {
	switch n := ref.(type) {
	case *ast.FeatureChainExpr:
		return n != nil
	case *ast.QualifiedName:
		return chainStart(n) > 0
	}
	return false
}

// chainStart is the index of the first segment written after a '.', 0 for none.
func chainStart(qn *ast.QualifiedName) int {
	if qn == nil {
		return 0
	}
	for i, part := range qn.Parts {
		if part.Chained {
			return i
		}
	}
	return 0
}

// chainRoot is the name a chained end starts from: `a` of `a.b.c`.
func chainRoot(qn *ast.QualifiedName) *ast.QualifiedName {
	start := chainStart(qn)
	if start == 0 {
		return qn
	}
	root := &ast.QualifiedName{Parts: qn.Parts[:start:start], Global: qn.Global}
	root.NodeSpan = qn.Span()
	return root
}

// chainTarget is the end as the reference a `perform` of it states.
func chainTarget(ref ast.Node) ast.Node {
	qn, ok := ref.(*ast.QualifiedName)
	if !ok {
		return ref
	}
	start := chainStart(qn)
	var target ast.Node = &ast.FeatureReference{NodeBase: ast.NodeBase{NodeSpan: qn.Span()}, Name: chainRoot(qn)}
	for _, part := range qn.Parts[start:] {
		member := &ast.QualifiedName{}
		member.NodeSpan = part.Span
		member.SetSingleton(ast.NameSegment{Text: part.Text, Span: part.Span})
		target = &ast.FeatureChainExpr{NodeBase: ast.NodeBase{NodeSpan: qn.Span()}, Operand: target, Member: member}
	}
	return target
}

// chainSteps returns the steps performing the action a chain end reaches: those the
// body or a general states by reference (`perform v.close;`), else one the end implies.
func (g *ActionGraph) chainSteps(ref ast.Node, scope *symbols.Scope) []ast.Node {
	key := g.chainKeyIn(scope, ref)
	var steps []ast.Node
	add := func(nodes ...ast.Node) {
		for _, node := range nodes {
			if node != nil && !slices.Contains(steps, node) {
				steps = append(steps, node)
			}
		}
	}
	for _, node := range g.Nodes {
		if g.performs(node, g.DeclScope(node), key) {
			add(node)
		}
	}
	// A general's step joins the flow only after the body's own successions are lowered.
	for i, body := range g.generals {
		if body == nil || body.Node() == nil {
			continue
		}
		for _, member := range ast.DeclMembers(body.Node()) {
			decl := unwrapMembership(member)
			if !g.performs(decl, body, key) {
				continue
			}
			decl, body := g.nearestRedefinition(i, decl)
			if matches := inheritedNodeMatches(g, body, decl); len(matches) > 0 {
				add(matches...)
			} else if !shadowsInheritedActionNode(g, body, decl) {
				add(ensureDeclaredActionNode(g, decl, body))
			}
		}
	}
	if len(steps) > 0 {
		return steps
	}
	if step := g.impliedSteps[key]; step != nil {
		return []ast.Node{step}
	}
	step := &ast.PerformActionNode{ActionRef: chainTarget(ref)}
	step.NodeSpan = ref.Span()
	if g.impliedSteps == nil {
		g.impliedSteps = make(map[chainKey]ast.Node)
	}
	g.impliedSteps[key] = step
	g.Nodes = append(g.Nodes, step)
	g.recordDeclaredIn(step, scope)
	g.Bodies[step] = []Statement{withExprs(performEffect(step, scope), g.Exprs)}
	return []ast.Node{step}
}

// nearestRedefinition follows decl, a member of general i, to the member of the
// nearest general redefining it, and the body declaring that member.
func (g *ActionGraph) nearestRedefinition(i int, decl ast.Node) (ast.Node, *symbols.Scope) {
	body := g.generals[i]
	for nearer := i - 1; nearer >= 0; nearer-- {
		general := g.generals[nearer]
		if general == nil || general.Node() == nil {
			continue
		}
		for _, member := range ast.DeclMembers(general.Node()) {
			redefining := unwrapMembership(member)
			if slices.Contains(resolve.ActionNodeRedefinitionTargets(general, redefining, false), decl) {
				decl, body = redefining, general
				break
			}
		}
	}
	return decl, body
}

// performs reports whether a node declared in scope is an action step stated to
// perform the action key names.
func (g *ActionGraph) performs(node ast.Node, scope *symbols.Scope, key chainKey) bool {
	usage, ok := node.(*ast.Usage)
	if !ok || !isActionUsageNode(usage) {
		return false
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind.ReferenceSubsets() && isChainEnd(rel.Target) && g.chainKeyIn(scope, rel.Target) == key {
			return true
		}
	}
	return false
}
