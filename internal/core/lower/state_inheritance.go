package lower

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrRecursiveStateTyping reports a state definition whose content contains a
// state typed by that same definition, which has no finite materialization.
var ErrRecursiveStateTyping = errors.New("recursive state typing")

// ErrUnsupportedStateContent reports content of a state machine that lowering
// cannot represent, rather than dropping it.
var ErrUnsupportedStateContent = errors.New("unsupported state machine content")

// StateTypeResolver resolves the declaration a type name reaches and its body scope.
// Lowering falls back to the scope tree only without one or for a name it cannot resolve.
type StateTypeResolver interface {
	TypeDecl(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, *symbols.Scope, bool)
}

// StateTypeWithholder names resolved declarations whose content lowering must not
// take. A withheld type is settled: it is never looked up again through the scope tree.
type StateTypeWithholder interface {
	WithholdsStateType(decl ast.Node) bool
}

// inheritedMember is one member a state inherits, with the body that declares
// it: names written in it resolve in that body's scope, not in the usage's.
type inheritedMember struct {
	node  ast.Node
	owner ast.Node
	scope *symbols.Scope
}

// stateInstance is one materialization of a state definition's content: the
// vertices its declarations lowered into under one typed usage. Two usages of
// one definition materialize into two instances, so neither the graph nodes nor
// the runtime state behind them are shared.
type stateInstance struct {
	outer   *stateInstance
	state   *ast.StateNode
	owners  []ast.Node
	members []inheritedMember

	vertexOf     map[ast.Node]ast.Node
	stateByDecl  map[ast.Node]*ast.StateNode
	completionOf map[ast.Node]*ast.StateNode
}

// push makes inst the innermost materialization, so vertices collected under it
// are recorded as this usage's own.
func (g *StateGraph) push(inst *stateInstance) {
	inst.outer = g.cur
	g.cur = inst
}

// pop leaves the innermost materialization.
func (g *StateGraph) pop() {
	if g.cur != nil {
		g.cur = g.cur.outer
	}
}

// putVertex records the graph node a declaration lowered into, in the
// materialization being collected.
func (g *StateGraph) putVertex(decl, node ast.Node) {
	if g.cur != nil {
		g.cur.vertexOf[decl] = node
		return
	}
	g.vertexOf[decl] = node
}

// findVertex is the graph node a declaration lowered into, searched from the
// innermost materialization outwards: a definition's own member resolves to
// this usage's copy of it, an outer name to the machine's.
func (g *StateGraph) findVertex(decl ast.Node) (ast.Node, bool) {
	for inst := g.cur; inst != nil; inst = inst.outer {
		if node, ok := inst.vertexOf[decl]; ok {
			return node, true
		}
	}
	node, ok := g.vertexOf[decl]
	return node, ok
}

func (g *StateGraph) putStateDecl(decl ast.Node, state *ast.StateNode) {
	if g.cur != nil {
		g.cur.stateByDecl[decl] = state
		return
	}
	g.stateByDecl[decl] = state
}

func (g *StateGraph) findStateDecl(decl ast.Node) *ast.StateNode {
	for inst := g.cur; inst != nil; inst = inst.outer {
		if state, ok := inst.stateByDecl[decl]; ok {
			return state
		}
	}
	return g.stateByDecl[decl]
}

func (g *StateGraph) findCompletion(owner ast.Node) *ast.StateNode {
	if g.cur != nil {
		return g.cur.completionOf[owner]
	}
	return g.completionOf[owner]
}

func (g *StateGraph) putCompletion(owner ast.Node, vertex *ast.StateNode) {
	if g.cur != nil {
		g.cur.completionOf[owner] = vertex
		return
	}
	g.completionOf[owner] = vertex
}

// stateType resolves what a type name reaches: through the name-resolution tier
// when lowering runs with it, else from the scope tree; a withheld type reaches nothing.
func (g *StateGraph) stateType(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, *symbols.Scope, bool) {
	types, ok := g.endpoints.(StateTypeResolver)
	if !ok {
		return resolve.TypeDeclInScope(scope, qn)
	}
	decl, body, found := types.TypeDecl(scope, qn)
	if !found {
		return resolve.TypeDeclInScope(scope, qn)
	}
	if withholder, ok := g.endpoints.(StateTypeWithholder); ok && withholder.WithholdsStateType(decl) {
		return nil, nil, false
	}
	return decl, body, true
}

// stateTypeNames are the names a state declaration takes content from: the
// definition typing a usage and the definitions either specializes.
func stateTypeNames(decl ast.Node) []*ast.QualifiedName {
	var rels []*ast.Relationship
	switch n := decl.(type) {
	case *ast.Usage:
		rels = n.Relationships
	case *ast.Definition:
		rels = n.Relationships
	default:
		return nil
	}
	var names []*ast.QualifiedName
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
		default:
			continue
		}
		target := rel.Target
		if ref, ok := target.(*ast.FeatureReference); ok {
			target = ref.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			names = append(names, qn)
		}
	}
	return names
}

// isStateDecl reports whether a declaration is a state definition or usage,
// which is what a state usage inherits content from.
func isStateDecl(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.Definition:
		return n.Kind == ast.DefState
	case *ast.Usage:
		return n.Kind == ast.UsageState
	}
	return false
}

// declName is the name a declaration was written with, for a message naming it.
func declName(decl ast.Node) string {
	switch n := decl.(type) {
	case *ast.Definition:
		return n.Ident.Name
	case *ast.Usage:
		name, _ := ast.EffectiveName(n)
		return name
	}
	return ast.SimpleName(decl)
}

// inheritedContent is the content a state declaration inherits: the members of
// every state definition it is typed by or specializes, most general first, each
// with the body it was declared in. declScope is the scope the declaration
// itself was written in.
func (g *StateGraph) inheritedContent(decl ast.Node, declScope *symbols.Scope) ([]inheritedMember, []ast.Node, error) {
	var members []inheritedMember
	var owners []ast.Node
	for _, qn := range stateTypeNames(decl) {
		def, body, ok := g.stateType(declScope, qn)
		// An unresolved type is reported by the name-resolution tier; lowering has
		// no content to materialize from it.
		if !ok || def == nil || !isStateDecl(def) || def == decl {
			continue
		}
		if g.materializing[def] {
			return nil, nil, fmt.Errorf("%w: state %s is typed by %s, whose content contains it",
				ErrRecursiveStateTyping, declName(decl), declName(def))
		}
		g.materializing[def] = true
		superMembers, superOwners, err := g.inheritedContent(def, outerScope(body, def))
		g.materializing[def] = false
		if err != nil {
			return nil, nil, err
		}
		members = append(members, superMembers...)
		owners = append(owners, superOwners...)
		owners = append(owners, def)
		for _, member := range declMembers(def) {
			members = append(members, inheritedMember{node: member, owner: def, scope: body})
		}
	}
	return members, owners, nil
}

// outerScope is the scope a declaration itself was written in, given the scope
// of its own body.
func outerScope(body *symbols.Scope, decl ast.Node) *symbols.Scope {
	if body == nil {
		return nil
	}
	if body.Node() == decl && body.Parent() != nil {
		return body.Parent()
	}
	return body
}

// declMembers is the body of a definition or usage.
func declMembers(decl ast.Node) []ast.Node {
	switch n := decl.(type) {
	case *ast.Definition:
		return n.Members
	case *ast.Usage:
		return n.Members
	}
	return nil
}

// newInstance is the materialization of the content state inherits, recorded so
// the transition pass lowers the same content in the same context.
func (g *StateGraph) newInstance(state *ast.StateNode, members []inheritedMember, owners []ast.Node, replaced map[ast.Node]ast.Node) *stateInstance {
	inst := &stateInstance{
		state:        state,
		owners:       owners,
		members:      members,
		vertexOf:     make(map[ast.Node]ast.Node),
		stateByDecl:  make(map[ast.Node]*ast.StateNode),
		completionOf: make(map[ast.Node]*ast.StateNode),
	}
	// A body inherited from a definition names that definition's own members and
	// itself: both reach this usage's copy.
	for _, owner := range owners {
		inst.vertexOf[owner] = state
		inst.stateByDecl[owner] = state
	}
	// A substate the usage redeclares keeps the inherited name: what the definition
	// declared reaches the usage's replacement of it.
	for dropped, replacement := range replaced {
		decl := dropped
		if node, ok := g.declOf[stateNodeOf(dropped)]; ok {
			decl = node
		}
		inst.vertexOf[decl] = replacement
		if node, ok := replacement.(*ast.StateNode); ok {
			inst.stateByDecl[decl] = node
		}
	}
	g.instanceOf[state] = inst
	return inst
}

// stateContent is the content one body contributes to a state: the vertex it
// builds and the attributes that body declares.
type stateContent struct {
	node  *ast.StateNode
	attrs []Attribute
}

// addMembers adds the content a list of members contributes to a state, each
// member in the scope of the body it was declared in.
func (g *StateGraph) addMembers(content *stateContent, members []inheritedMember, parallel bool) error {
	for _, member := range members {
		if err := g.addMember(content, unwrapMembership(member.node), parallel, member); err != nil {
			return err
		}
	}
	return nil
}

// addMember adds one declared member to a state: its behaviors, the events it
// defers, its attributes, its regions and its substates, each recorded with the
// scope of the body it was written in.
func (g *StateGraph) addMember(content *stateContent, member ast.Node, parallel bool, from inheritedMember) error {
	state := content.node
	scope := from.scope
	// What a state declares itself is lowered as written; what it inherits is
	// copied, so two usages of one definition own two vertices.
	inherited := from.owner != nil
	switch m := member.(type) {
	case *ast.EntryMember:
		state.Entry = append(state.Entry, g.behaviorsIn(m.Actions, scope)...)
	case *ast.DoMember:
		state.Do = append(state.Do, g.behaviorsIn(m.Actions, scope)...)
	case *ast.ExitMember:
		state.Exit = append(state.Exit, g.behaviorsIn(m.Actions, scope)...)
	case *ast.DeferMember:
		state.Defer = append(state.Defer, m.Triggers...)
	case *ast.StateRegion:
		if !inherited {
			state.Regions = append(state.Regions, m)
			return nil
		}
		region := &ast.StateRegion{NodeBase: ast.NodeBase{NodeSpan: m.NodeSpan}, Name: m.Name, States: m.States}
		g.regionScopeOf[region] = childScope(scope, m)
		state.Regions = append(state.Regions, region)
	case *ast.StateNode:
		if parallel {
			return nil
		}
		if !inherited {
			state.Substates = append(state.Substates, m)
			return nil
		}
		state.Substates = append(state.Substates, cloneStateNode(g, m, scope))
	case *ast.PseudostateNode:
		if !inherited {
			state.Substates = append(state.Substates, m)
			return nil
		}
		clone := *m
		state.Substates = append(state.Substates, &clone)
	case *ast.SubstateMember:
		if parallel {
			return nil
		}
		child := &ast.StateNode{Name: m.Name}
		child.NodeSpan = m.NodeSpan
		g.declOf[child] = m
		g.scopeOf[child] = childScope(scope, m)
		state.Substates = append(state.Substates, child)
	case *ast.Usage:
		switch {
		case m.Kind == ast.UsageState && !parallel:
			child, err := stateNodeFromUsage(g, m, scope)
			if err != nil {
				return err
			}
			state.Substates = append(state.Substates, child)
		case m.Kind == ast.UsageState:
			// A typed state usage in a parallel body is a region, synthesized there.
		case g.redefinedRunToCompletionFeature(m, scope) != "":
			// A restated run-to-completion default is what the executor implements, not a slot.
		case m.Kind == ast.UsageAttribute:
			if name, _ := ast.EffectiveName(m); name != "" {
				content.attrs = append(content.attrs, Attribute{Name: name, Direction: m.Direction, IsResult: m.IsResult, Value: m.Value, Node: m, Scope: scope})
				g.attributeScope[m] = scope
			}
		default:
			return unsupportedInherited(inherited, member, state)
		}
	default:
		return unsupportedInherited(inherited, member, state)
	}
	return nil
}

// unsupportedInherited reports content a state inherits that lowering cannot
// represent, so it surfaces rather than being dropped. Content a state declares
// itself is lowered by the passes that own it.
func unsupportedInherited(inherited bool, member ast.Node, state *ast.StateNode) error {
	if !inherited || loweredElsewhere(member) {
		return nil
	}
	return fmt.Errorf("%w: %s cannot be inherited by the state %s; a state usage inherits its definition's substates, behaviors, transitions, deferred events and attributes",
		ErrUnsupportedStateContent, DescribeMember(member), state.Name)
}

// loweredElsewhere reports whether another pass lowers a member: the edges and
// the declarations a state's body may carry beside its vertices.
func loweredElsewhere(member ast.Node) bool {
	switch n := member.(type) {
	case *ast.Comment, *ast.Documentation, *ast.TextualRepresentation,
		*ast.SuccessionEdge, *ast.TransitionEdge, *ast.TransitionMember,
		*ast.InitialNode, *ast.FinalNode,
		*ast.Definition, *ast.Package, *ast.ErrorNode:
		return true
	case *ast.Usage:
		switch n.Kind {
		case ast.UsageSuccession, ast.UsageTransition, ast.UsagePort:
			return true
		}
	}
	return false
}

// behaviorsIn records the scope each behavior was declared in, which is the
// definition's body for a behavior a usage inherits, and returns the actions.
func (g *StateGraph) behaviorsIn(actions []ast.Node, scope *symbols.Scope) []ast.Node {
	for _, action := range actions {
		if actual := unwrapMembership(action); actual != nil {
			g.behaviorScope[actual] = scope
		}
	}
	return actions
}

// cloneStateNode copies a state node a usage inherits, so two usages of one
// definition own two graph vertices rather than sharing the declaration's.
func cloneStateNode(g *StateGraph, node *ast.StateNode, scope *symbols.Scope) *ast.StateNode {
	clone := &ast.StateNode{
		NodeBase: ast.NodeBase{NodeSpan: node.NodeSpan},
		Name:     node.Name,
		Entry:    g.behaviorsIn(node.Entry, childScope(scope, node)),
		Do:       g.behaviorsIn(node.Do, childScope(scope, node)),
		Exit:     g.behaviorsIn(node.Exit, childScope(scope, node)),
		Defer:    node.Defer,
	}
	g.declOf[clone] = node
	g.scopeOf[clone] = childScope(scope, node)
	for _, substate := range node.Substates {
		switch child := unwrapMembership(substate).(type) {
		case *ast.StateNode:
			clone.Substates = append(clone.Substates, cloneStateNode(g, child, g.scopeOf[clone]))
		case *ast.PseudostateNode:
			ps := *child
			clone.Substates = append(clone.Substates, &ps)
		default:
			clone.Substates = append(clone.Substates, substate)
		}
	}
	for _, region := range node.Regions {
		copied := &ast.StateRegion{NodeBase: ast.NodeBase{NodeSpan: region.NodeSpan}, Name: region.Name, States: region.States}
		g.regionScopeOf[copied] = childScope(g.scopeOf[clone], region)
		clone.Regions = append(clone.Regions, copied)
	}
	return clone
}

// redeclare drops the inherited members a state's own body redeclares: a
// substate or a region of the same name is the one the usage writes, and a
// behavior the usage states replaces the inherited one, since a state has one
// entry, one do and one exit action (Systems Library `States.sysml`:
// `entry action entryAction :>> 'entry'`).
// It returns the inherited substate each own substate replaces, so an inherited
// transition naming that substate reaches the replacement rather than a dropped
// vertex.
func redeclare(state *ast.StateNode, inherited, own *ast.StateNode) map[ast.Node]ast.Node {
	state.Entry = pickBehaviors(inherited.Entry, own.Entry)
	state.Do = pickBehaviors(inherited.Do, own.Do)
	state.Exit = pickBehaviors(inherited.Exit, own.Exit)
	state.Defer = append(append([]ast.Node{}, inherited.Defer...), own.Defer...)
	state.Substates = append(keptSubstates(inherited.Substates, own.Substates), own.Substates...)
	state.Regions = append(keptRegions(inherited.Regions, own.Regions), own.Regions...)
	return replacedSubstates(inherited.Substates, own.Substates)
}

// replacedSubstates pairs each inherited substate with the own substate of the
// same name that replaces it.
func replacedSubstates(inherited, own []ast.Node) map[ast.Node]ast.Node {
	if len(own) == 0 || len(inherited) == 0 {
		return nil
	}
	byName := make(map[string]ast.Node, len(own))
	for _, node := range own {
		if name := vertexName(node); name != "" {
			byName[name] = unwrapMembership(node)
		}
	}
	replaced := make(map[ast.Node]ast.Node)
	for _, node := range inherited {
		name := vertexName(node)
		if name == "" {
			continue
		}
		if replacement, ok := byName[name]; ok {
			replaced[unwrapMembership(node)] = replacement
		}
	}
	if len(replaced) == 0 {
		return nil
	}
	return replaced
}

// pickBehaviors is the usage's own behaviors when it states any, else the
// inherited ones.
func pickBehaviors(inherited, own []ast.Node) []ast.Node {
	if len(own) > 0 {
		return own
	}
	return inherited
}

// keptSubstates are the inherited substates the usage's own body does not
// redeclare under the same name.
func keptSubstates(inherited, own []ast.Node) []ast.Node {
	if len(own) == 0 {
		return inherited
	}
	redeclared := make(map[string]bool, len(own))
	for _, node := range own {
		if name := vertexName(node); name != "" {
			redeclared[name] = true
		}
	}
	kept := make([]ast.Node, 0, len(inherited))
	for _, node := range inherited {
		if name := vertexName(node); name != "" && redeclared[name] {
			continue
		}
		kept = append(kept, node)
	}
	return kept
}

// keptRegions are the inherited regions the usage's own body does not redeclare.
func keptRegions(inherited, own []*ast.StateRegion) []*ast.StateRegion {
	if len(own) == 0 {
		return inherited
	}
	redeclared := make(map[string]bool, len(own))
	for _, region := range own {
		redeclared[region.Name] = true
	}
	kept := make([]*ast.StateRegion, 0, len(inherited))
	for _, region := range inherited {
		if region.Name != "" && redeclared[region.Name] {
			continue
		}
		kept = append(kept, region)
	}
	return kept
}

// stateNodeOf is a substate node as a state node, for the declaration side table.
func stateNodeOf(node ast.Node) *ast.StateNode {
	if state, ok := node.(*ast.StateNode); ok {
		return state
	}
	return nil
}

// vertexName is the name a substate node was declared with.
func vertexName(node ast.Node) string {
	switch n := unwrapMembership(node).(type) {
	case *ast.StateNode:
		return n.Name
	case *ast.PseudostateNode:
		return n.Name
	case *ast.SubstateMember:
		return n.Name
	case *ast.Usage:
		name, _ := ast.EffectiveName(n)
		return name
	}
	return ""
}

// keptAttributes are the inherited attributes a body of its own does not
// redeclare: a redeclaring attribute is the same feature, with the value the
// redeclaration states.
func keptAttributes(inherited, own []Attribute) []Attribute {
	if len(own) == 0 {
		return inherited
	}
	redeclared := make(map[string]bool, len(own))
	for _, attr := range own {
		redeclared[attr.Name] = true
	}
	kept := make([]Attribute, 0, len(inherited)+len(own))
	for _, attr := range inherited {
		if redeclared[attr.Name] {
			continue
		}
		kept = append(kept, attr)
	}
	return append(kept, own...)
}

// DescribeMember names a state machine member in modelling terms, for a message
// about content that cannot be lowered where it was written.
func DescribeMember(member ast.Node) string {
	switch n := member.(type) {
	case *ast.EntryMember:
		return "the entry action"
	case *ast.DoMember:
		return "the do action"
	case *ast.ExitMember:
		return "the exit action"
	case *ast.TransitionMember:
		if n.Name != "" {
			return "the transition " + n.Name
		}
		return "the transition"
	case *ast.Comment:
		return "the comment"
	case *ast.Documentation:
		return "the documentation"
	case *ast.Import:
		return "the import"
	case *ast.Definition:
		return fmt.Sprintf("the %s definition %s", n.Kind, n.Ident.Name)
	case *ast.Usage:
		name, _ := ast.EffectiveName(n)
		if name == "" {
			return fmt.Sprintf("an unnamed %s usage", n.Kind)
		}
		if n.Direction != ast.DirNone {
			return fmt.Sprintf("the %s parameter %s", n.Direction, name)
		}
		return fmt.Sprintf("the %s usage %s", n.Kind, name)
	case *ast.StateNode:
		return "the state " + n.Name
	case *ast.SubstateMember:
		return "the state " + n.Name
	case *ast.StateRegion:
		return "the region " + n.Name
	case *ast.PseudostateNode:
		return fmt.Sprintf("the %s %s", n.Kind, n.Name)
	case *ast.InitialNode:
		return "the succession from " + n.Name
	case *ast.FinalNode:
		return "the `done` marker"
	case *ast.Package:
		return "the package " + n.Ident.Name
	}
	return "a member of this state"
}
