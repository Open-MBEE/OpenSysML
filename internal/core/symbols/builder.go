package symbols

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Build constructs the immutable scope tree for a parsed document.
func Build(root *ast.RootNamespace) *Scope {
	rootScope := NewScope(nil, root)
	if root == nil {
		return rootScope
	}
	buildMembers(rootScope, root.Members)
	// Body-expression parameter scopes hang off the scopes their expressions
	// resolve against, so they are linked once the declarations exist.
	buildBodyScopes(rootScope, root.Members)
	return rootScope
}

// buildMembers processes a member list into the given scope.
func buildMembers(scope *Scope, members []ast.Node) {
	for _, m := range members {
		decl, vis := unwrapMember(m)
		if decl == nil {
			continue
		}
		// Trivia (doc comments) is attached to the member wrapper m, not to the
		// unwrapped inner decl; capture it here so the Symbol can carry it.
		buildDecl(scope, decl, vis, m.LeadingTrivia())
	}
}

// unwrapMember returns the underlying declaration node and its visibility.
// Membership wrappers carry visibility; directly-listed Import/Alias nodes
// carry their own.
func unwrapMember(m ast.Node) (ast.Node, ast.Visibility) {
	switch v := m.(type) {
	case *ast.Membership:
		return v.Member, v.Visibility
	case *ast.Import:
		return v, v.Visibility
	case *ast.Alias:
		return v, v.Visibility
	case *ast.RelationshipMember:
		return v, v.Visibility
	default:
		return m, ast.VisibilityDefault
	}
}

// buildDecl registers a symbol (and child scope, where applicable) for a single
// declaration node. trivia is the leading trivia captured from the member
// wrapper before unwrap.
func buildDecl(scope *Scope, decl ast.Node, vis ast.Visibility, trivia []ast.Trivia) {
	if prefixes := prefixMetadataOf(decl); len(prefixes) > 0 {
		buildMetadataBodyScopes(scope, prefixes)
	}
	switch {
	case buildNamespaceDecl(scope, decl, vis, trivia):
	case buildBehaviorDecl(scope, decl, vis, trivia):
	}
}

// buildNamespaceDecl registers a namespace, definition or usage
// declaration, reporting whether decl was one.
func buildNamespaceDecl(scope *Scope, decl ast.Node, vis ast.Visibility, trivia []ast.Trivia) bool {
	switch d := decl.(type) {
	case *ast.Package:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolPackage, d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
		return true
	case *ast.Namespace:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolNamespace, d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
		return true
	case *ast.Alias:
		sym := newSymbol(d.Ident, SymbolAlias, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		return true
	case *ast.Dependency:
		sym := newSymbol(d.Ident, SymbolDependency, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		return true
	case *ast.MultiplicityDecl:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolMultiplicity, d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
		return true
	case *ast.RelationshipMember:
		// A keyword-first relationship owns its members, and names one only when
		// the notation gives it an identification.
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, SymbolRelationship, d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
		return true
	case *ast.Comment:
		sym := newSymbol(d.Ident, SymbolComment, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		return true
	case *ast.Documentation:
		sym := newSymbol(d.Ident, SymbolDocumentation, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		return true
	case *ast.TextualRepresentation:
		sym := newSymbol(d.Ident, SymbolTextualRepresentation, d, vis, nil, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		return true
	case *ast.Definition:
		child := NewScope(scope, d)
		sym := newSymbol(d.Ident, definitionSymbolKind(d.Kind), d, vis, child, scope, trivia)
		defineIdent(scope, d.Ident, sym)
		scope.AddChild(child)
		buildMembers(child, d.Members)
		return true
	case *ast.Usage:
		child := NewScope(scope, d)
		// Phase 4: Parser treats 'datatype' uniformly as usage. Builder classifies based on context.
		// If usage is attribute kind with specializes/subsets but no typing, treat as definition.
		kind := classifyUsage(d)
		id, naming := effectiveIdent(d)
		sym := newSymbol(id, kind, d, vis, child, scope, trivia)
		naming.apply(sym)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		buildCrossFeature(child, d)
		buildMembers(child, d.Members)
		buildConnectorEnds(child, d)
		return true
	case *ast.SubstateMember:
		// SubstateMember represents simple state declaration: state <name>;
		// Create a state usage symbol for it
		id := ast.Identification{Name: d.Name, NameSpan: d.NameSpan}
		child := NewScope(scope, d)
		sym := newSymbol(id, SymbolStateUsage, d, vis, child, scope, trivia)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		return true
	case *ast.SubjectMember:
		// SubjectMember represents requirement subject: subject <name> : <Type>;
		// Create a part usage symbol (subject is structural usage like part)
		id, naming := namingIdent(d.Ident, d.NamingFeature())
		child := NewScope(scope, d)
		sym := newSymbol(id, SymbolPartUsage, d, vis, child, scope, trivia)
		naming.apply(sym)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		if len(d.Body) > 0 {
			buildMembers(child, d.Body)
		}
		return true
	default:
		return false
	}
}

// buildBehaviorDecl registers a state, transition or action node
// declaration, reporting whether decl was one.
func buildBehaviorDecl(scope *Scope, decl ast.Node, vis ast.Visibility, trivia []ast.Trivia) bool {
	switch d := decl.(type) {
	case *ast.Import, *ast.FilterMember, *ast.ErrorNode:
		// Imports are processed during resolution; filters hold expressions;
		// error nodes have no declaration. Nothing to register here.
		return true
	case *ast.PrefixMetadata:
		child := buildMetadataBodyScope(scope, d)
		// An identification names the usage as a member of its namespace, exactly
		// as the `metadata` spelling of the same declaration does.
		if d.Ident.Name != "" || d.Ident.ShortName != "" {
			if child == nil {
				child = NewScope(scope, d)
				scope.AddChild(child)
			}
			sym := newSymbol(d.Ident, SymbolMetadataUsage, d, vis, child, scope, trivia)
			defineIdent(scope, d.Ident, sym)
		}
		return true
	case *ast.InitialNode:
		// Register initial node by name so transitions can reference it; an
		// unnamed one with a body owns a body-local scope for its members.
		if d.Name == "" && len(d.Members) == 0 {
			return true
		}
		child := NewScope(scope, d)
		if d.Name == "" {
			child.markBodyLocal()
		} else {
			id := ast.Identification{Name: d.Name}
			// Use attribute usage kind (control flow nodes are structural members)
			sym := newSymbol(id, SymbolAttributeUsage, d, vis, child, scope, trivia)
			defineIdent(scope, id, sym)
		}
		scope.AddChild(child)
		buildMembers(child, d.Members)
		return true
	case *ast.SendStatement:
		// The send is the action usage it was written on (`action a send x { in x; }`),
		// whose body declares the parameters; anywhere else it is a node of its own.
		if usage, ok := scope.Node().(*ast.Usage); ok && usage.IsActionNode {
			buildMembers(scope, d.Members)
			return true
		}
		buildAnonymousNode(scope, d, SymbolActionUsage, d.Members, vis, trivia)
		return true
	case *ast.SuccessionEdge:
		// A succession with a body is a SuccessionAsUsage whose body declares its
		// own features (SysML.xtext ActionTargetSuccession ends in a UsageBody).
		if len(d.Members) > 0 {
			buildAnonymousNode(scope, d, SymbolSuccessionUsage, d.Members, vis, trivia)
		}
		return true
	case *ast.StateNode:
		// Register state node by name (including initial/final pseudostates)
		// so transitions and successions can reference it
		if d.Name == "" {
			return true
		}
		id := ast.Identification{Name: d.Name}
		child := NewScope(scope, d)
		sym := newSymbol(id, SymbolStateUsage, d, vis, child, scope, trivia)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
		// Substates and regions declare the names the state's own body refers to.
		buildMembers(child, d.Substates)
		for _, region := range d.Regions {
			buildDecl(child, region, ast.VisibilityDefault, nil)
		}
		return true
	case *ast.TransitionMember:
		// A transition is a feature of the state that declares it (SysML v2
		// §7.19.2: TransitionUsage specializes ActionUsage), and its effect
		// behaviors are features of the transition, so `t.effectAction` resolves.
		// An unnamed one is an anonymous member, found by its declaration.
		defineParams := triggerParameterDefiner(d.Trigger)
		child := NewScope(scope, d)
		id := ast.Identification{Name: d.Name, NameSpan: d.NameSpan}
		defineIdent(scope, id, newSymbol(id, SymbolActionUsage, d, vis, child, scope, trivia))
		scope.AddChild(child)
		body := child
		if defineParams != nil {
			// The trigger's parameters are visible to the guard, effect and body,
			// however deeply they nest, but are not features of the transition: they
			// live in a body-local scope of their own that both are built in.
			body = NewScope(child, d.Trigger)
			body.markBodyLocal()
			child.AddChild(body)
			defineParams(body)
		}
		// The body's members read the trigger's parameters as the effect does,
		// and are the transition's features like it.
		params := len(body.AllMembers())
		buildEffect(body, d.Effect)
		buildMembers(body, d.Members)
		if body != child {
			ownEffectMembers(child, body.AllMembers()[params:])
		}
		defineTransitionEffect(child, body, d)
		return true
	case *ast.StateRegion:
		// A region is a namespace of its own: sibling regions routinely reuse
		// state names (each region declaring its own `initial start`), so their
		// states must not collide in the composite state's scope.
		regionScope := NewScope(scope, d)
		if d.Name != "" {
			id := ast.Identification{Name: d.Name}
			sym := newSymbol(id, SymbolStateUsage, d, vis, regionScope, scope, trivia)
			defineIdent(scope, id, sym)
		}
		scope.AddChild(regionScope)
		buildMembers(regionScope, d.States)
		return true
	case *ast.ConstraintMember:
		buildConstraintBodyScope(scope, d, d.Body)
		return true
	case *ast.AssumeMember:
		buildRequirementConstraint(scope, d, d.Body, vis, trivia)
		return true
	case *ast.RequireMember:
		buildRequirementConstraint(scope, d, d.Body, vis, trivia)
		return true
	case *ast.EntryMember:
		// An entry/do/exit action is a feature of the state declaring it, so a
		// named one (`entry action entryAction :>> 'entry';`) is a member of the
		// state's scope rather than of the wrapper the parser puts it in.
		buildMembers(scope, d.Actions)
		return true
	case *ast.DoMember:
		buildMembers(scope, d.Actions)
		return true
	case *ast.ExitMember:
		buildMembers(scope, d.Actions)
		return true
	case *ast.PseudostateNode:
		// fork/join/choice/junction/history named in a state body are
		// transition endpoints, so they must be referenceable.
		if d.Name != "" {
			id := ast.Identification{Name: d.Name}
			child := NewScope(scope, d)
			sym := newSymbol(id, SymbolStateUsage, d, vis, child, scope, trivia)
			defineIdent(scope, id, sym)
			scope.AddChild(child)
		}
		return true
	case *ast.WhileLoopActionNode:
		// A loop is an anonymous action namespace: its body's declarations (and a
		// `for` iteration variable) are members visible to the body and condition.
		child := NewScope(scope, d)
		child.markBodyLocal()
		scope.AddChild(child)
		if d.Kind == ast.LoopFor && d.Variable.Name != "" {
			// The variable's own span, not the whole loop's: the editor renames
			// through NameSpan and jumps to DeclSpan.
			child.Define(d.Variable.Name, &Symbol{
				Name:       d.Variable.Name,
				Kind:       SymbolAttributeUsage,
				Decl:       d,
				DeclSpan:   d.Variable.NameSpan,
				NameSpan:   d.Variable.NameSpan,
				OwnerScope: child,
			})
		}
		buildMembers(child, d.Body)
		return true
	case *ast.IfActionNode:
		// Each branch is a namespace of its own: `if c { action a; } else { action a; }`
		// declares two distinct actions, neither of them a member of the enclosing
		// behavior. The condition is evaluated outside both branches, so it is not
		// resolved against them.
		for _, branch := range d.Branches() {
			buildDecl(scope, branch, vis, trivia)
		}
		return true
	case *ast.IfBranchNode:
		child := NewScope(scope, d)
		child.markBodyLocal()
		scope.AddChild(child)
		buildMembers(child, d.Body)
		return true
	case *ast.ForkNode:
		// A control node is an action usage (Actions::ForkAction et al.), so a
		// named one is a member a succession may name as source or target.
		buildControlNode(scope, d, d.Name, d.NameSpan, vis, trivia)
		return true
	case *ast.JoinNode:
		buildControlNode(scope, d, d.Name, d.NameSpan, vis, trivia)
		return true
	case *ast.MergeNode:
		buildControlNode(scope, d, d.Name, d.NameSpan, vis, trivia)
		return true
	case *ast.DecisionNode:
		buildControlNode(scope, d, d.Name, d.NameSpan, vis, trivia)
		return true
	default:
		return false
	}
}

// prefixMetadataOf returns the prefix metadata written on a declaration.
func prefixMetadataOf(decl ast.Node) []*ast.PrefixMetadata {
	if d, ok := decl.(*ast.Dependency); ok {
		return d.Prefixes
	}
	prefixes, _, _ := ast.DeclaredMetadata(decl)
	return prefixes
}

func buildMetadataBodyScopes(scope *Scope, prefixes []*ast.PrefixMetadata) {
	for _, prefix := range prefixes {
		if prefix != nil && len(prefix.Body) > 0 {
			buildMetadataBodyScope(scope, prefix)
		}
	}
}

// buildMetadataBodyScope builds the scope of a metadata usage's body and
// returns it, or nil when the usage has no body.
func buildMetadataBodyScope(parent *Scope, prefix *ast.PrefixMetadata) *Scope {
	if parent == nil || prefix == nil || len(prefix.Body) == 0 {
		return nil
	}
	child := NewScope(parent, prefix)
	child.markBodyLocal()
	parent.AddChild(child)
	buildMembers(child, prefix.Body)
	return child
}

// buildControlNode registers a named fork/join/merge/decision node the way a
// final node is registered, at its name's span; an unnamed one declares no name
// but still owns the scope its body declares into.
func buildControlNode(scope *Scope, decl ast.Node, name string, nameSpan source.Span, vis ast.Visibility, trivia []ast.Trivia) {
	body := ast.NodeBodyMembers(decl)
	if name == "" {
		if len(body) > 0 {
			buildAnonymousNode(scope, decl, SymbolActionUsage, body, vis, trivia)
		}
		return
	}
	id := ast.Identification{Name: name, NameSpan: nameSpan}
	child := NewScope(scope, decl)
	sym := newSymbol(id, SymbolActionUsage, decl, vis, child, scope, trivia)
	defineIdent(scope, id, sym)
	scope.AddChild(child)
	// A control node ends in ActionBody, so what its body declares are features
	// of the node a flow may name (`flow F.b1 to B1.b`).
	buildMembers(child, body)
}

// buildRequirementConstraint registers the constraint usage an assume/require member
// declares as a member of its requirement (SysML v2 §7.20.5), anonymous if unnamed.
func buildRequirementConstraint(scope *Scope, decl ast.Node, body []ast.Node, vis ast.Visibility, trivia []ast.Trivia) {
	var id ast.Identification
	var naming derivedNaming
	if oc, ok := ast.OwnedConstraintOf(decl); ok {
		id, naming = namingIdent(oc.Ident, oc.NamingFeature())
	} else if ref := ast.ConstraintReferenceOf(decl); ref != nil {
		// `require r1;` owns a constraint named by the one it references.
		id, naming = derivedIdent(id, NamedByReference, ref)
	} else {
		buildConstraintBodyScope(scope, decl, body)
		return
	}
	child := NewScope(scope, decl)
	sym := newSymbol(id, SymbolConstraintUsage, decl, vis, child, scope, trivia)
	naming.apply(sym)
	defineIdent(scope, id, sym)
	scope.AddChild(child)
	buildMembers(child, body)
}

// buildConstraintBodyScope links the scope a nested constraint body declares
// into. The body states the constraint its member owns (SysML v2 §7.20.5), so
// its declarations are visible inside it and are no members of the namespace the
// member itself is declared in.
func buildConstraintBodyScope(scope *Scope, decl ast.Node, body []ast.Node) {
	if len(body) == 0 {
		return
	}
	child := NewScope(scope, decl)
	child.markBodyLocal()
	scope.AddChild(child)
	buildMembers(child, body)
}

// ConstraintBodyScope returns the scope a nested constraint body resolves against:
// the one its declarations were built into, or parent for a body declaring none.
func ConstraintBodyScope(parent *Scope, decl ast.Node) *Scope {
	if parent == nil {
		return nil
	}
	if child := parent.ChildFor(decl); child != nil {
		return child
	}
	return parent
}

// buildConnectorEnds registers a symbol for every end of a connector usage that
// declares a name (`connect bead references t.bead`). Such an end is an end
// feature of the connector itself (SysML v2 §7.13.2), so it is a member of the
// connector's own scope, never of the scope the connector is declared in.
func buildConnectorEnds(scope *Scope, u *ast.Usage) {
	for _, end := range u.ConnectorEnds {
		if end == nil {
			continue
		}
		id, ok := end.DeclaredName()
		if !ok {
			continue
		}
		child := NewScope(scope, end)
		sym := newSymbol(id, SymbolConnectorEnd, end, ast.VisibilityDefault, child, scope, nil)
		defineIdent(scope, id, sym)
		scope.AddChild(child)
	}
}

// buildCrossFeature registers the cross feature an end declares inline ahead of
// itself (`end x1 [0..1] feature x : C1`) as a member of the end.
func buildCrossFeature(scope *Scope, u *ast.Usage) {
	cross := u.CrossFeature
	if cross == nil {
		return
	}
	child := NewScope(scope, cross)
	sym := newSymbol(cross.Ident, SymbolCrossFeature, cross, ast.VisibilityDefault, child, scope, nil)
	defineIdent(scope, cross.Ident, sym)
	scope.AddChild(child)
}

// transitionEffectName is the TransitionAction feature a transition's effect
// action redefines.
const transitionEffectName = "effect"

// defineTransitionEffect names a transition's effect action `effect`, the
// TransitionAction feature it redefines (SysML v2 §7.19.2), so `t.effect.x`
// reads through the effect action rather than the abstract library feature.
// The effect was built in body, the transition's scope or the one holding
// its trigger's parameters.
func defineTransitionEffect(scope, body *Scope, trans *ast.TransitionMember) {
	if len(trans.Effect) != 1 {
		return
	}
	if _, exists := scope.LookupLocal(transitionEffectName); exists {
		return
	}
	effect, _ := unwrapMember(trans.Effect[0])
	if effect == nil {
		return
	}
	// A declared effect action is already a member of the transition's scope:
	// name that symbol rather than building a second one for it.
	if sym, ok := memberDeclaring(body, effect); ok {
		scope.Define(transitionEffectName, sym)
		return
	}
	// A statement (`do send x to y`) declares no member of its own, so the
	// action it performs is named here, its members staying the transition's.
	statement := NewScope(body, effect)
	sym := newSymbol(
		ast.Identification{Name: transitionEffectName, NameSpan: effect.Span()},
		SymbolActionUsage, effect, ast.VisibilityDefault, statement, scope, nil,
	)
	scope.Define(transitionEffectName, sym)
	body.AddChild(statement)
}

// buildEffect builds a transition's effect. A `do send` statement's parameters
// are the transition's own members, not a nested node's (ownEffectMembers).
func buildEffect(body *Scope, effect []ast.Node) {
	for _, m := range effect {
		if decl, _ := unwrapMember(m); decl != nil {
			if send, ok := decl.(*ast.SendStatement); ok {
				buildMembers(body, send.Members)
				continue
			}
		}
		buildMembers(body, []ast.Node{m})
	}
}

// ownEffectMembers makes the members an effect or body built after the trigger's
// parameters (an action's symbol, a `do send`'s own parameters) the transition's.
func ownEffectMembers(scope *Scope, members []*Symbol) {
	seen := map[*Symbol]bool{}
	for _, sym := range members {
		if seen[sym] {
			continue
		}
		seen[sym] = true
		sym.OwnerScope = scope
		defineIdent(scope, ast.Identification{Name: sym.Name, ShortName: sym.ShortName}, sym)
	}
}

// memberDeclaring returns the member of scope that decl declared.
func memberDeclaring(scope *Scope, decl ast.Node) (*Symbol, bool) {
	for _, sym := range scope.AllMembers() {
		if sym.Decl == decl {
			return sym, true
		}
	}
	return nil, false
}

// newSymbol builds a Symbol from an identification. scope is the child scope the
// declaration owns (nil for leaf declarations); owner is the enclosing scope the
// declaration was declared in. trivia is the leading trivia from the member wrapper.
func newSymbol(id ast.Identification, kind SymbolKind, decl ast.Node, vis ast.Visibility, scope, owner *Scope, trivia []ast.Trivia) *Symbol {
	name := id.Name
	nameSpan := id.NameSpan
	if name == "" {
		name, nameSpan = id.ShortName, id.ShortNameSpan
	}
	sym := &Symbol{
		Name:          name,
		ShortName:     id.ShortName,
		Kind:          kind,
		Decl:          decl,
		Visibility:    vis,
		DeclSpan:      decl.Span(),
		NameSpan:      nameSpan,
		Scope:         scope,
		OwnerScope:    owner,
		LeadingTrivia: trivia,
	}
	// Set scope's owner back-reference for inheritance lookup
	if scope != nil {
		scope.SetOwner(sym)
	}
	return sym
}

// effectiveIdent returns the identification a usage is registered under, and
// how it was derived: the name of the usage's naming feature
// (ast.NamingFeature) keeps its own declared short name.
//
// The naming feature's own effective name is approximated by the reference's
// last segment, since resolution has not run when scopes are built.
func effectiveIdent(u *ast.Usage) (ast.Identification, derivedNaming) {
	return namingIdent(u.Ident, ast.NamingFeature(u))
}

// namingIdent is effectiveIdent for any declaration: id completed with the name
// its naming feature rel supplies, and the target that supplied it.
func namingIdent(id ast.Identification, rel *ast.Relationship) (ast.Identification, derivedNaming) {
	if rel == nil {
		return id, derivedNaming{}
	}
	kind := NamedByReference
	if rel.Kind == ast.RelRedefines {
		kind = NamedByRedefinition
	}
	return derivedIdent(id, kind, rel.Target)
}

// derivedIdent is namingIdent for a target reached through a relationship of
// the given kind.
func derivedIdent(id ast.Identification, kind Naming, target ast.Node) (ast.Identification, derivedNaming) {
	name, span := ast.TargetName(target)
	if name == "" {
		return id, derivedNaming{}
	}
	id.Name, id.NameSpan = name, span
	return id, derivedNaming{kind: kind, target: namingTargetNode(target)}
}

// derivedNaming is the name derivation namingIdent found, if any.
type derivedNaming struct {
	kind   Naming
	target ast.Node
}

func (n derivedNaming) apply(sym *Symbol) {
	sym.Naming = n.kind
	sym.NamingTarget = n.target
}

// namingTargetNode returns the node a relationship target is resolved as, so
// that the resolver can recognise the reference that named a feature: the
// qualified name a plain reference unwraps to, or the target itself.
func namingTargetNode(target ast.Node) ast.Node {
	if qn := ast.AsQualifiedName(target); qn != nil {
		return qn
	}
	return target
}

// buildAnonymousNode registers an unnamed node as a member of scope with a
// child scope its body members are built in.
func buildAnonymousNode(scope *Scope, node ast.Node, kind SymbolKind, members []ast.Node, vis ast.Visibility, trivia []ast.Trivia) {
	child := NewScope(scope, node)
	sym := newSymbol(ast.Identification{}, kind, node, vis, child, scope, trivia)
	defineIdent(scope, ast.Identification{}, sym)
	scope.AddChild(child)
	buildMembers(child, members)
}

// defineIdent registers sym under its short and primary name keys, skipping
// any that are empty (e.g. anonymous usages).
func defineIdent(scope *Scope, id ast.Identification, sym *Symbol) {
	if id.ShortName != "" {
		scope.Define(id.ShortName, sym)
	}
	if id.Name != "" {
		scope.Define(id.Name, sym)
	}
	// If both names empty, register as anonymous
	if id.ShortName == "" && id.Name == "" {
		scope.DefineAnonymous(sym)
	}
}

// definitionSymbolKind maps an ast.DefinitionKind to its SymbolKind.
func definitionSymbolKind(k ast.DefinitionKind) SymbolKind {
	switch k {
	case ast.DefPart:
		return SymbolPartDef
	case ast.DefAttribute:
		return SymbolAttributeDef
	case ast.DefItem:
		return SymbolItemDef
	case ast.DefOccurrence:
		return SymbolOccurrenceDef
	case ast.DefIndividual:
		return SymbolIndividualDef
	case ast.DefMetadata:
		return SymbolMetadataDef
	case ast.DefMetaclass:
		return SymbolMetaclass
	case ast.DefEnumeration:
		return SymbolEnumerationDef
	case ast.DefView:
		return SymbolViewDef
	case ast.DefViewpoint:
		return SymbolViewpointDef
	case ast.DefRendering:
		return SymbolRenderingDef
	case ast.DefConcern:
		return SymbolConcernDef
	case ast.DefConnection:
		return SymbolConnectionDef
	case ast.DefFlow:
		return SymbolFlowDef
	case ast.DefPort:
		return SymbolPortDef
	case ast.DefInterface:
		return SymbolInterfaceDef
	case ast.DefAllocation:
		return SymbolAllocationDef
	case ast.DefAction:
		return SymbolActionDef
	case ast.DefState:
		return SymbolStateDef
	case ast.DefCalc:
		return SymbolCalcDef
	case ast.DefConstraint:
		return SymbolConstraintDef
	case ast.DefRequirement:
		return SymbolRequirementDef
	case ast.DefCase:
		return SymbolCaseDef
	case ast.DefAnalysisCase:
		return SymbolAnalysisCaseDef
	case ast.DefVerificationCase:
		return SymbolVerificationCaseDef
	case ast.DefUseCase:
		return SymbolUseCaseDef
	case ast.DefClass, ast.DefStruct, ast.DefAssoc, ast.DefBehavior, ast.DefPredicate:
		return SymbolKerMLType
	default:
		return SymbolUnknown
	}
}

// usageSymbolKinds maps each ast.UsageKind to its SymbolKind.
var usageSymbolKinds = map[ast.UsageKind]SymbolKind{
	ast.UsagePart:        SymbolPartUsage,
	ast.UsageAttribute:   SymbolAttributeUsage,
	ast.UsageItem:        SymbolItemUsage,
	ast.UsageOccurrence:  SymbolOccurrenceUsage,
	ast.UsageIndividual:  SymbolIndividualUsage,
	ast.UsageMetadata:    SymbolMetadataUsage,
	ast.UsageEnumeration: SymbolEnumerationUsage,
	ast.UsageView:        SymbolViewUsage,
	ast.UsageViewpoint:   SymbolViewpointUsage,
	// A view's `render` member owns a rendering usage (SysML v2 §8.3.26).
	ast.UsageRendering:     SymbolRenderingUsage,
	ast.UsageViewRendering: SymbolRenderingUsage,
	// A framed concern is a concern usage (SysML v2 §8.3.20).
	ast.UsageConcern:       SymbolConcernUsage,
	ast.UsageFramedConcern: SymbolConcernUsage,
	// A KerML `connector` is the connection usage of the kernel layer
	// (KerML 1.0 §7.4.6), so it is one kind of symbol.
	ast.UsageConnection: SymbolConnectionUsage,
	ast.UsageConnector:  SymbolConnectionUsage,
	// A succession is a SuccessionAsUsage (SysML v2 §8.3.13.7): a connector
	// usage of its own kind, so it is a redefinition target like any feature.
	ast.UsageSuccession:  SymbolSuccessionUsage,
	ast.UsageFlow:        SymbolFlowUsage,
	ast.UsagePort:        SymbolPortUsage,
	ast.UsageInterface:   SymbolInterfaceUsage,
	ast.UsageAllocation:  SymbolAllocationUsage,
	ast.UsageAction:      SymbolActionUsage,
	ast.UsageState:       SymbolStateUsage,
	ast.UsageCalc:        SymbolCalcUsage,
	ast.UsageConstraint:  SymbolConstraintUsage,
	ast.UsageRequirement: SymbolRequirementUsage,
	// A satisfy requirement usage is a requirement usage (SysML v2 §8.3.19).
	ast.UsageSatisfy:          SymbolSatisfyRequirementUsage,
	ast.UsageCase:             SymbolCaseUsage,
	ast.UsageAnalysisCase:     SymbolAnalysisCaseUsage,
	ast.UsageVerificationCase: SymbolVerificationCaseUsage,
	ast.UsageUseCase:          SymbolUseCaseUsage,
	// Subject is a requirement parameter - treat as part usage for structural purposes
	ast.UsageSubject: SymbolPartUsage,
	// Objective is a requirement parameter - treat as part usage for structural purposes
	ast.UsageObjective: SymbolPartUsage,
	// An actor and a stakeholder are part usages (SysML v2 §8.3.19).
	ast.UsageActor:       SymbolPartUsage,
	ast.UsageStakeholder: SymbolPartUsage,
	// The parser records a KerML type declaration as a usage; it declares a
	// type, not a feature (KerML 1.0 §8.3).
	ast.UsageClass:       SymbolKerMLType,
	ast.UsageStruct:      SymbolKerMLType,
	ast.UsageAssoc:       SymbolKerMLType,
	ast.UsageBehavior:    SymbolKerMLType,
	ast.UsagePredicate:   SymbolKerMLType,
	ast.UsageInteraction: SymbolKerMLType,
	// A KerML step is a feature typed by a behavior, which is what a SysML
	// action usage is (SysML v2 §8.3.14).
	ast.UsageStep: SymbolActionUsage,
	// A KerML `expr`/`bool` declares an Expression: a feature typed by a
	// function (KerML 1.0 §9.2.10), as a SysML calc usage is.
	ast.UsageExpr: SymbolCalcUsage,
	ast.UsageBool: SymbolCalcUsage,
}

// usageSymbolKind maps an ast.UsageKind to its SymbolKind.
func usageSymbolKind(k ast.UsageKind) SymbolKind {
	if kind, ok := usageSymbolKinds[k]; ok {
		return kind
	}
	return SymbolUnknown
}

// classifyUsage determines the correct symbol kind for a usage AST node.
// Per Phase 4: Parser treats some keywords (like 'datatype') uniformly as usage.
// Builder classifies based on semantic context (relationships, body structure).
//
// Classification rules:
// - Attribute usage with specializes (but NO typing/subsets) → AttributeDef
// - Attribute usage with typing or subsets/redefines → AttributeUsage
// - All other usages → use usageSymbolKind directly
func classifyUsage(u *ast.Usage) SymbolKind {
	// A `datatype` declares a KerML DataType (KerML 1.0 §8.3.2): a definition whatever
	// it specializes, unlike the `attribute`/`feature` keywords classified below.
	if u.Keyword == "datatype" {
		return SymbolAttributeDef
	}
	// A KerML `function` declares a Function: a Behavior specialization, so a
	// definition (KerML 1.0 §9.2.9), which is what `calc def` declares in SysML.
	if u.Keyword == "function" {
		return SymbolCalcDef
	}
	// Only classify attribute usages (datatype, attribute, feature keywords)
	if u.Kind != ast.UsageAttribute {
		return usageSymbolKind(u.Kind)
	}

	// Check relationships to determine if this is def-like or usage-like
	hasTyping := false
	hasSpecializes := false
	hasSubsetsOrRedefines := false

	for _, rel := range u.Relationships {
		switch rel.Kind {
		case ast.RelTyping:
			hasTyping = true
		case ast.RelSpecializes:
			hasSpecializes = true
		case ast.RelSubsets, ast.RelRedefines:
			hasSubsetsOrRedefines = true
		}
	}

	// Attribute usage with ONLY specializes (no typing/subsets) → classify as definition
	// Pattern: datatype Real specializes Complex;
	// NOT: datatype MyReal :>> Real; (this has subsets, stays as usage)
	if hasSpecializes && !hasTyping && !hasSubsetsOrRedefines {
		return SymbolAttributeDef
	}

	// Default: treat as usage
	return SymbolAttributeUsage
}
