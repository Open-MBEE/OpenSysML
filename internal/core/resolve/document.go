package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ResolveDocument walks the document's references and resolves each, recording
// diagnostics on the Resolver. name identifies the document in the index.
func (r *Resolver) ResolveDocument(name string, root *ast.RootNamespace) {
	rootScope := r.idx.DocumentRoot(name)
	if rootScope == nil {
		return
	}
	saved := r.document
	r.document = name
	defer func() { r.document = saved }()
	r.walkMembers(rootScope, membersOf(root))
	r.checkDistinguishability(rootScope)
}

// membersOf returns the top-level members of a RootNamespace.
func membersOf(root *ast.RootNamespace) []ast.Node {
	if root == nil {
		return nil
	}
	return root.Members
}

// walkMembers resolves references in each member, descending into child scopes.
func (r *Resolver) walkMembers(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		decl, _ := unwrapForResolve(m)
		r.resolveDecl(scope, decl)
	}
}

// unwrapForResolve mirrors the builder's unwrapMember: it strips *ast.Membership
// wrappers so we resolve against the inner declaration.
func unwrapForResolve(m ast.Node) (ast.Node, ast.Visibility) {
	switch v := m.(type) {
	case *ast.Membership:
		return v.Member, v.Visibility
	case *ast.Import:
		return v, v.Visibility
	case *ast.Alias:
		return v, v.Visibility
	default:
		return m, ast.VisibilityDefault
	}
}

// resolveDecl resolves references contributed by a single declaration and
// recurses into declarations that own a child scope.
func (r *Resolver) resolveDecl(scope *symbols.Scope, decl ast.Node) {
	switch {
	case r.resolveNamespaceDecl(scope, decl):
	case r.resolveTypeDecl(scope, decl):
	case r.resolveBehaviorDecl(scope, decl):
	default:
		// A bare expression member is the body's result, as in a calc body
		// whose result is its last expression.
		r.resolveExpr(scope, decl)
	}
}

// resolveNamespaceDecl resolves a namespace-level declaration, reporting
// whether decl was one.
func (r *Resolver) resolveNamespaceDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Package:
		r.resolvePrefixes(scope, d, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.Namespace:
		r.resolvePrefixes(scope, d, d.Prefixes)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.Import:
		r.resolveImportTarget(scope, d)
		if d.FilterExpr != nil {
			r.InCondition(func() { r.resolveExpr(scope, d.FilterExpr) })
		}
		return true
	case *ast.Alias:
		r.ResolveQualified(scope, d.For)
		return true
	case *ast.RelationshipMember:
		// Both ends of a keyword-first relationship name elements, in the scope
		// the relationship is a member of.
		r.resolveRelationshipEnd(scope, d.Source)
		r.resolveRelationshipEnd(scope, d.Target)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
		return true
	case *ast.Dependency:
		r.resolvePrefixes(scope, d, d.Prefixes)
		for _, c := range d.Clients {
			r.ResolveQualified(scope, c)
		}
		for _, s := range d.Suppliers {
			r.ResolveQualified(scope, s)
		}
		return true
	case *ast.MultiplicityDecl:
		r.resolveMultiplicity(scope, d.Range)
		if d.Subsets != nil {
			r.ResolveQualified(scope, d.Subsets)
		}
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Members)
		}
		return true
	case *ast.Comment:
		for _, a := range d.About {
			r.ResolveQualified(scope, a)
		}
		return true
	case *ast.PrefixMetadata:
		// A metadata usage written as a member of its own names its type the same
		// way a prefix does.
		r.resolveMetadataPrefix(scope, scope, d)
		return true
	case *ast.FilterMember:
		r.InCondition(func() { r.resolveExpr(scope, d.Condition) })
		return true
	default:
		return false
	}
}

// resolveTypeDecl resolves a definition, usage or constraint member, reporting
// whether decl was one.
func (r *Resolver) resolveTypeDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.Definition:
		r.resolvePrefixes(scope, d, d.Prefixes)
		child := r.childScope(scope, d)
		r.resolveHeaderRelationships(scope, child, d, d.Relationships)
		if child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.Usage:
		r.resolvePrefixes(scope, d, d.Prefixes)
		child := r.childScope(scope, d)
		r.resolveHeaderRelationships(scope, child, d, d.Relationships)
		r.resolveMultiplicity(scope, d.Multiplicity)
		if cross := d.CrossFeature; cross != nil {
			r.resolveHeaderRelationships(scope, child, cross, cross.Relationships)
			r.resolveMultiplicity(scope, cross.Multiplicity)
		}
		// An accept node keeps its trigger in the usage's value, and a trigger's
		// names are not all references (see resolveTrigger).
		if d.IsAccept {
			r.resolveTrigger(scope, d.Value)
		} else {
			r.resolveExpr(scope, d.Value)
		}
		for _, end := range d.ConnectorEnds {
			// ConnectorEnd has both Target and Reference fields
			// Target: primary connector target (part being connected)
			// Reference: optional "references X" clause, or state transition target
			if end == nil {
				continue
			}
			// An end that reference-subsets the feature it attaches to declares
			// its own name (`connect bead references t.bead`), so that name is a
			// declaration, not a reference to resolve.
			_, declaresName := end.DeclaredName()
			// A redefinition names an end of the connector's own type, so it
			// resolves in the connector's scope; everything else an end names —
			// the feature it attaches to, its type — is a feature of the
			// connector's owner and resolves in the enclosing scope.
			endScope := scope
			redefinitionScope := scope
			if inner := r.childScope(scope, d); inner != nil {
				redefinitionScope = inner
			}
			redefines, others := ast.SplitRedefinitions(end.Relationships)
			r.resolveRelationships(redefinitionScope, end, redefines)
			r.resolveRelationships(endScope, end, others)
			endpointKind := d.Kind == ast.UsageSuccession || d.Kind == ast.UsageTransition
			resolveAsEndpoint := endpointKind && inStateMachine(endScope) && !declaresName
			resolveEnd := func(target ast.Node) {
				// A calc's binding may name its implicit result feature as an end.
				if d.Kind == ast.UsageBinding && isImplicitCalcResult(scope, target) {
					return
				}
				// A machine succession/transition end names a vertex like a transition endpoint.
				if qn, ok := target.(*ast.QualifiedName); ok {
					if resolveAsEndpoint {
						r.ResolveEndpoint(endScope, qn)
					} else {
						r.ResolveQualified(endScope, qn)
					}
				} else {
					r.resolveExpr(endScope, target)
				}
			}
			if end.Target != nil && !declaresName {
				resolveEnd(end.Target)
			}
			if end.Reference != nil {
				resolveEnd(end.Reference)
			}
		}
		if d.FlowEnds != nil {
			r.resolveExpr(scope, d.FlowEnds.From)
			r.resolveExpr(scope, d.FlowEnds.To)
			// A declared payload (`of name : Type`) names a member of the flow
			// itself, not an element of the enclosing scope.
			payloadScope := scope
			if d.FlowEnds.PayloadDecl != nil && child != nil {
				payloadScope = child
			}
			r.resolveExpr(payloadScope, d.FlowEnds.Payload)
		}
		if child != nil {
			r.walkMembers(child, d.Members)
			r.checkDistinguishability(child)
		}
		return true
	case *ast.SubjectMember:
		r.resolvePrefixes(scope, d, d.Prefixes)
		if d.TypeRef != nil {
			r.ResolveQualified(scope, d.TypeRef)
		}
		if d.Multiplicity != nil {
			r.resolveExpr(scope, d.Multiplicity.Lower)
			r.resolveExpr(scope, d.Multiplicity.Upper)
		}
		r.resolveRelationships(scope, d, d.Relationships)
		r.resolveExpr(scope, d.BindingExpr)
		if child := r.childScope(scope, d); child != nil {
			r.walkMembers(child, d.Body)
		}
		return true
	case *ast.InitialNode:
		r.resolveInitial(scope, d)
		r.resolveEdgeEnd(scope, d.Successor, nil, false)
		r.resolveExpr(scope, d.Guard)
		r.walkMembers(r.bodyScope(scope, d), d.Members)
		return true
	case *ast.SuccessionEdge:
		r.resolveSuccessionEdge(scope, d)
		r.walkMembers(r.bodyScope(scope, d), d.Members)
		return true
	case *ast.ControlFlowEdge:
		r.resolveControlFlowEdge(scope, d)
		return true
	case *ast.FinalNode:
		// Final nodes have no references
		return true
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		// The node's name is a label; its body declares features of the node.
		r.walkMembers(r.bodyScope(scope, d), ast.NodeBodyMembers(d))
		return true
	case *ast.ConstraintMember:
		r.resolveExpr(scope, d.Expression)
		r.walkConstraintBody(scope, d, nil, d.Body)
		return true
	case *ast.AssumeMember:
		r.resolvePrefixes(scope, d, d.Prefixes)
		r.resolveCondition(scope, d, d.Expression)
		r.resolveRelationships(scope, d, d.Relationships)
		r.resolveMultiplicity(scope, d.Multiplicity)
		r.resolveExpr(scope, d.Value)
		r.walkConstraintBody(scope, d, r.resolveConstraintReference(scope, d, d.Reference), d.Body)
		return true
	case *ast.RequireMember:
		r.resolvePrefixes(scope, d, d.Prefixes)
		r.resolveCondition(scope, d, d.Expression)
		r.resolveRelationships(scope, d, d.Relationships)
		r.resolveMultiplicity(scope, d.Multiplicity)
		r.resolveExpr(scope, d.Value)
		r.walkConstraintBody(scope, d, r.resolveConstraintReference(scope, d, d.Reference), d.Body)
		return true
	default:
		return false
	}
}

// resolveBehaviorDecl resolves a behavioral member — a state, transition or
// action node — reporting whether decl was one.
func (r *Resolver) resolveBehaviorDecl(scope *symbols.Scope, decl ast.Node) bool {
	switch d := decl.(type) {
	case *ast.EntryMember:
		r.walkMembers(scope, d.Actions)
		return true
	case *ast.DoMember:
		r.walkMembers(scope, d.Actions)
		return true
	case *ast.ExitMember:
		r.walkMembers(scope, d.Actions)
		return true
	case *ast.DeferMember:
		// A deferred event is a trigger like a transition's, so it resolves the
		// same way: bare signal names are left to lowering.
		for _, trigger := range d.Triggers {
			r.resolveTrigger(scope, trigger)
		}
		return true
	case *ast.StateNode:
		// The state's own name is a declaration, not a reference. Its body
		// resolves in the scope the state owns, which holds its substates and
		// regions; features it reads still resolve outward from there.
		body := scope
		if child := r.childScope(scope, d); child != nil {
			body = child
		}
		r.walkMembers(body, d.Entry)
		r.walkMembers(body, d.Do)
		r.walkMembers(body, d.Exit)
		r.walkMembers(body, d.Substates)
		for _, region := range d.Regions {
			r.resolveDecl(body, region)
		}
		return true
	case *ast.StateRegion:
		states := scope
		if child := r.childScope(scope, d); child != nil {
			states = child
		}
		r.walkMembers(states, d.States)
		return true
	case *ast.TransitionMember:
		// Source and target name vertices of the enclosing machine: resolved here,
		// so a misspelled endpoint reports with the other name diagnostics rather
		// than at lowering, which consumes what this resolved.
		// The guard, effect and body resolve against the parameters the
		// transition's call trigger declares, which live in a scope of their own.
		r.ResolveEndpoint(scope, d.Source)
		r.ResolveEndpoint(scope, d.Target)
		r.resolveTrigger(scope, d.Trigger)
		if d.Via != nil {
			r.ResolveQualified(scope, d.Via)
		}
		body := symbols.TriggerScope(scope, d)
		r.resolveExpr(body, d.Guard)
		r.walkMembers(body, d.Effect)
		r.walkMembers(body, d.Members)
		return true
	case *ast.SendStatement:
		r.resolveExpr(scope, d.Message)
		r.resolveExpr(scope, d.Target)
		r.resolveExpr(scope, d.Receiver)
		r.walkMembers(r.bodyScope(scope, d), d.Members)
		return true
	case *ast.TerminateStatement:
		r.resolveExpr(scope, d.Target)
		return true
	case *ast.AssignmentActionNode:
		r.resolveExpr(scope, d.Target)
		r.resolveExpr(scope, d.Value)
		return true
	case *ast.ActionExecutionNode:
		if d.ActionRef != nil {
			r.ResolveQualified(scope, d.ActionRef)
		}
		r.resolveExpr(scope, d.Expression)
		return true
	case *ast.PerformActionNode:
		r.resolveExpr(scope, d.ActionRef)
		return true
	case *ast.WhileLoopActionNode:
		// The loop owns its body's declarations, and its condition is checked
		// against them: `loop { action charging; } until charging.done`. The
		// collection a `for` loop iterates over is evaluated before the loop is
		// entered, so it resolves outside the body.
		body := scope
		if child := r.childScope(scope, d); child != nil {
			body = child
		}
		r.resolveExpr(scope, d.Collection)
		r.resolveExpr(body, d.Condition)
		r.resolveExpr(body, d.Until)
		r.walkMembers(body, d.Body)
		return true
	case *ast.IfActionNode:
		// The condition is evaluated before either branch is entered, so it sees
		// the enclosing scope only; each branch owns its body's declarations.
		r.resolveExpr(scope, d.Condition)
		for _, branch := range d.Branches() {
			r.resolveDecl(scope, branch)
		}
		return true
	case *ast.IfBranchNode:
		body := scope
		if child := r.childScope(scope, d); child != nil {
			body = child
		}
		r.walkMembers(body, d.Body)
		return true
	default:
		return false
	}
}

func isImplicitCalcResult(scope *symbols.Scope, node ast.Node) bool {
	owner := scope.Owner()
	if owner == nil {
		return false
	}
	switch decl := owner.Decl.(type) {
	case *ast.Definition:
		if decl.Kind != ast.DefCalc {
			return false
		}
	case *ast.Usage:
		if decl.Kind != ast.UsageCalc {
			return false
		}
	default:
		return false
	}
	var name string
	switch ref := node.(type) {
	case *ast.FeatureReference:
		if ref.Name == nil || len(ref.Name.Parts) != 1 {
			return false
		}
		name = ref.Name.Parts[0].Text
	case *ast.QualifiedName:
		if len(ref.Parts) != 1 {
			return false
		}
		name = ref.Parts[0].Text
	default:
		return false
	}
	return name == "result"
}

// resolveTrigger resolves the references a transition trigger carries.
//
// A bare name after `when` is classified by lowering as a signal, and signals
// are injected by the event source rather than declared in the model, so bare
// names are left unresolved here; resolving them would report every signal-
// triggered transition as an unresolved reference.
func (r *Resolver) resolveTrigger(scope *symbols.Scope, trigger ast.Node) {
	switch t := trigger.(type) {
	case nil:
		return
	case *ast.TimeEvent:
		r.resolveExpr(scope, t.Duration)
	case *ast.ChangeEvent:
		r.resolveExpr(scope, t.Condition)
	case *ast.AcceptEvent:
		// The payload of an accept names a type, and `:> f` an event feature,
		// as the pinned validator resolves them; a bare `when` name does not.
		if t.SignalType != nil {
			r.resolveQualified(scope, t.SignalType, nil)
		}
		if t.Subsets != nil {
			r.resolveQualified(scope, t.Subsets, nil)
		}
		if t.Payload != nil {
			r.resolveDecl(scope, t.Payload)
		}
	case *ast.Usage:
		// A named payload (`accept m : Warning`) declares a parameter, so its
		// typing resolves like any other declaration's.
		r.resolveDecl(scope, t)
	case *ast.QualifiedName, *ast.FeatureReference, *ast.CallEvent:
		// Signal and call triggers name events, not model elements.
	default:
		r.resolveExpr(scope, trigger)
	}
}

// ParameterizedByName reports whether sym is a case or requirement — including
// a concern or viewpoint, which are requirements — whose subject, actors and
// stakeholders redefine the inherited ones by name (SysML 7.18.4, 7.19.4). That
// is not modelled and not distinguishable from an ordinary feature here, so the
// conflict rule skips such a body entirely.
func ParameterizedByName(sym *symbols.Symbol) bool {
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		switch decl.Kind {
		case ast.UsageRequirement, ast.UsageSatisfy, ast.UsageConcern,
			ast.UsageFramedConcern, ast.UsageViewpoint,
			ast.UsageCase, ast.UsageAnalysisCase,
			ast.UsageVerificationCase, ast.UsageUseCase:
			return true
		}
	case *ast.Definition:
		switch decl.Kind {
		case ast.DefRequirement, ast.DefConcern, ast.DefViewpoint, ast.DefCase,
			ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase:
			return true
		}
	}
	return false
}

// childScope finds the child scope whose node is decl.
func (r *Resolver) childScope(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	return scope.ChildFor(decl)
}

// bodyScope is the scope the body of an action node resolves against: its own
// where the builder gave it one, and the enclosing scope otherwise.
func (r *Resolver) bodyScope(scope *symbols.Scope, node ast.Node) *symbols.Scope {
	if child := r.childScope(scope, node); child != nil {
		return child
	}
	return scope
}

// resolvePrefixes resolves the prefix annotations of decl, a member of scope.
// The annotated element owns them (KerML 8.2.4.2 PrefixMetadataMember), so
// their names resolve in its own scope.
func (r *Resolver) resolvePrefixes(scope *symbols.Scope, decl ast.Node, prefixes []*ast.PrefixMetadata) {
	names := r.bodyScope(scope, decl)
	for _, p := range prefixes {
		r.resolveMetadataPrefix(names, scope, p)
	}
}

// resolveMetadataPrefix resolves an annotation whose names are read in names
// and whose body scope, if any, the builder hung off parent.
func (r *Resolver) resolveMetadataPrefix(names, parent *symbols.Scope, prefix *ast.PrefixMetadata) {
	if prefix == nil {
		return
	}
	for _, a := range prefix.About {
		r.ResolveQualified(names, a)
	}
	owner, ok := r.ResolveQualified(names, prefix.Type)
	if !ok || owner == nil || len(prefix.Body) == 0 {
		return
	}
	if target, aliasOK := r.ResolveAliasTarget(owner); aliasOK {
		owner = target
	}
	body := parent.ChildFor(prefix)
	if body == nil {
		return
	}
	// Body values resolve against the metadata definition, not the annotated element.
	if body.Owner() == nil {
		body.SetOwner(owner)
	}
	r.resolveMetadataBody(body, prefix.Body)
}

func (r *Resolver) resolveMetadataBody(scope *symbols.Scope, members []ast.Node) {
	for _, member := range members {
		decl, _ := unwrapForResolve(member)
		if decl == nil {
			continue
		}
		r.resolveDecl(scope, decl)
	}
}

// resolveMultiplicity resolves the bounds of a multiplicity, which may name
// features rather than state literals (`[n..m]`).
func (r *Resolver) resolveMultiplicity(scope *symbols.Scope, mult *ast.Multiplicity) {
	if mult == nil {
		return
	}
	r.resolveExpr(scope, mult.Lower)
	r.resolveExpr(scope, mult.Upper)
}

// resolveRelationships resolves each relationship target of decl as a qualified
// name. Redefinitions resolve in the inherited scope, and reference subsettings
// resolve outside decl's own name binding (see refFilter).
func (r *Resolver) resolveRelationships(scope *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	for _, rel := range rels {
		if rel != nil && rel.Target != nil {
			// Unwrap FeatureReference if needed (relationship targets parsed as expressions)
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}

			referencing := ast.IsReferenceSubsetting(decl, rel)
			// Special case: redefinitions should resolve in inherited scope
			// Self-subsetting must resolve in the declaration scope so cycle checks see `p4 :> p4`.
			if rel.Kind == ast.RelRedefines || (rel.Kind == ast.RelSubsets && !referencing && !relationshipTargetsDecl(rel, decl)) {
				if qn, ok := target.(*ast.QualifiedName); ok {
					if rel.Kind == ast.RelSubsets && r.resolveOwnSibling(scope, qn, decl) {
						continue
					}
					r.resolveRedefinition(scope, qn, decl, rel.Kind == ast.RelRedefines)
					continue
				}
			}
			if referencing && isImplicitCalcResult(scope, target) {
				continue
			}

			// A reference subsetting resolves its leading segment past the
			// name decl borrows from it; memoizing that result makes the
			// chain walk below see the referenced feature, not decl.
			if referencing {
				hide := referenceFilter(decl, target)
				// A connector end's participant is featured where the connector
				// is, so a feature of the connector itself is not one
				// (KerML 8.3.4.5).
				if u, ok := decl.(*ast.Usage); ok && u.IsEnd && declaresConnector(scope) {
					hide.featuredBy = scope
				}
				if _, ok := target.(*ast.QualifiedName); ok {
					r.resolveTarget(scope, target, hide)
					continue
				}
				r.resolveTarget(scope, leadingName(target), hide)
			}

			// Standard resolution in current scope
			if qn, ok := target.(*ast.QualifiedName); ok {
				r.ResolveQualified(scope, qn)
			} else if fc, ok := target.(*ast.FeatureChainExpr); ok {
				r.resolveFeatureChain(scope, fc)
			}
		}
	}
}

// resolveRelationshipEnd resolves one end of a keyword-first relationship,
// which the notation writes as a name or a feature chain.
func (r *Resolver) resolveRelationshipEnd(scope *symbols.Scope, end ast.Node) {
	switch e := end.(type) {
	case *ast.QualifiedName:
		r.ResolveQualified(scope, e)
	case *ast.FeatureReference:
		r.ResolveQualified(scope, e.Name)
	case *ast.FeatureChainExpr:
		r.resolveFeatureChain(scope, e)
	}
}

func (r *Resolver) resolveHeaderRelationships(parent, header *symbols.Scope, decl ast.Node, rels []*ast.Relationship) {
	if header == nil || header == parent {
		r.resolveRelationships(parent, decl, rels)
		return
	}
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		r.resolveRelationships(r.relationshipScope(parent, header, rel), decl, []*ast.Relationship{rel})
	}
}

// relationshipScope is the scope a head relationship's target resolves in: the
// declaring element's own when the target opens there, else the enclosing one.
func (r *Resolver) relationshipScope(parent, header *symbols.Scope, rel *ast.Relationship) *symbols.Scope {
	if header == nil || header == parent {
		return parent
	}
	target := rel.Target
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	switch target := target.(type) {
	case *ast.QualifiedName:
		if r.resolvesInHeader(header, target, false, rel.Kind) {
			return header
		}
	case *ast.FeatureChainExpr:
		if r.resolvesInHeader(header, target.Member, true, rel.Kind) {
			return header
		}
	}
	return parent
}

// resolvesInHeader reports whether a head relationship's target, opening with name,
// resolves in the declaring element's own scope; a plain `: T`, `:> T`, `:>> T` never does.
func (r *Resolver) resolvesInHeader(header *symbols.Scope, name *ast.QualifiedName, chain bool, kind ast.RelationshipKind) bool {
	if name == nil || len(name.Parts) == 0 {
		return false
	}
	switch kind {
	case ast.RelTyping, ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines:
		if !chain {
			return false
		}
	}
	return r.headerHasName(header, name.Parts[0].Text, kind)
}

func (r *Resolver) headerHasName(scope *symbols.Scope, name string, kind ast.RelationshipKind) bool {
	if scope == nil {
		return false
	}
	if _, ok := scope.LookupLocal(name); ok {
		return true
	}
	for _, imp := range r.importsOf(scope.Node()) {
		if !r.importPrefixAvailable(scope, imp, name) {
			continue
		}
		if _, ok := r.matchImport(scope, imp, name); ok {
			return true
		}
	}
	// A featuring or crossing name may be one the declaration inherits from its
	// type; a redefinition or subsetting target may not, as it would find itself.
	if kind == ast.RelFeaturedBy || kind == ast.RelCrosses {
		if _, ok := r.lookupContributedMember(scope.Owner(), name, nil); ok {
			return true
		}
	}
	return false
}

func relationshipTargetsDecl(rel *ast.Relationship, decl ast.Node) bool {
	// Keep a declaration's self-reference out of inherited lookup; cycle detection handles it.
	if rel == nil || decl == nil || rel.Target == nil {
		return false
	}
	target := rel.Target
	if fr, ok := target.(*ast.FeatureReference); ok {
		target = fr.Name
	}
	qn, ok := target.(*ast.QualifiedName)
	return ok && namesDecl(qn, decl)
}

// namesDecl reports whether qn is decl's own name written bare.
func namesDecl(qn *ast.QualifiedName, decl ast.Node) bool {
	if qn == nil || len(qn.Parts) != 1 {
		return false
	}
	name := ""
	if oc, ok := ast.OwnedConstraintOf(decl); ok {
		name, _ = oc.EffectiveName()
	} else {
		switch d := decl.(type) {
		case *ast.Definition:
			name = d.Ident.Name
		case *ast.Usage:
			name, _ = ast.EffectiveName(d)
		case *ast.CrossFeatureMember:
			name = d.Ident.Name
		case *ast.SubjectMember:
			name, _ = d.EffectiveName()
		}
	}
	return name != "" && qn.Parts[0].Text == name
}

// resolveCondition resolves the condition expression of the require/assume
// member decl; a lone name is the reference form, so it resolves as one.
func (r *Resolver) resolveCondition(scope *symbols.Scope, decl ast.Node, expr ast.Node) {
	if ref := ast.ConditionReference(decl); ref != nil {
		r.resolveTarget(scope, ref, referenceFilter(decl, ref))
		return
	}
	r.resolveExpr(scope, expr)
}

// resolveConstraintReference resolves the requirement a require/assume member
// decl subsets by reference (SysML.xtext RequirementConstraintUsage); the member
// borrows its name, so it is no target of its own.
func (r *Resolver) resolveConstraintReference(scope *symbols.Scope, decl ast.Node, ref *ast.QualifiedName) *symbols.Symbol {
	if ref == nil || len(ref.Parts) == 0 {
		return nil
	}
	sym, ok := r.resolveTarget(scope, ref, referenceFilter(decl, ref))
	if !ok {
		return nil
	}
	return sym
}

// walkConstraintBody walks the body of a require/assume member, in the scope its
// declarations were built into so that nested bodies resolve too. The member
// reference-subsets the requirement ref, so the body may redefine that
// requirement's features by plain name (SysML.xtext RequirementConstraintUsage).
func (r *Resolver) walkConstraintBody(scope *symbols.Scope, decl ast.Node, ref *symbols.Symbol, body []ast.Node) {
	scope = symbols.ConstraintBodyScope(scope, decl)
	if scope == nil {
		return
	}
	if ref != nil {
		members := make(map[ast.Node]bool, len(body))
		for _, m := range body {
			member, _ := unwrapForResolve(m)
			members[member] = true
		}
		r.constraintRefs = append(r.constraintRefs, constraintRef{ref: ref, members: members})
		defer func() { r.constraintRefs = r.constraintRefs[:len(r.constraintRefs)-1] }()
	}
	r.walkMembers(scope, body)
}

// constraintRef is a requirement referenced by a require/assume member, with
// the direct members of that member's body, which redefine its features.
type constraintRef struct {
	ref     *symbols.Symbol
	members map[ast.Node]bool
}

// lookupConstraintRefFeature finds name among the features of the requirement
// referenced by the require/assume member whose body declares decl.
func (r *Resolver) lookupConstraintRefFeature(name string, decl ast.Node) (*symbols.Symbol, bool) {
	for i := len(r.constraintRefs) - 1; i >= 0; i-- {
		if !r.constraintRefs[i].members[decl] {
			continue // a nested declaration inherits from its own type, not the reference
		}
		return r.featureOf(r.constraintRefs[i].ref, name, newFeatureWalk(nil))
	}
	return nil, false
}

// featureWalk is one featureOf walk: seen maps a symbol to the feature it
// offers under the name, or to nil while visited or once it offered nothing;
// hide removes the bindings the reference being resolved must not see.
type featureWalk struct {
	hide *refFilter
	seen map[*symbols.Symbol]*symbols.Symbol
}

func newFeatureWalk(hide *refFilter) featureWalk {
	return featureWalk{hide: hide, seen: map[*symbols.Symbol]*symbols.Symbol{}}
}

// featureOf finds the feature named name declared by sym or inherited through
// its typings, specializations and featurings, walking live-parsed and
// cache-restored symbols alike. walk makes the search cycle-safe (the standard
// library holds specialization cycles) and keeps a general reachable along a
// second path when the first one masked what it offers.
func (r *Resolver) featureOf(sym *symbols.Symbol, name string, walk featureWalk) (*symbols.Symbol, bool) {
	if sym == nil {
		return nil, false
	}
	if found, visited := walk.seen[sym]; visited {
		return found, found != nil
	}
	walk.seen[sym] = nil
	found, ok := r.searchFeatureOf(sym, name, walk)
	if ok {
		walk.seen[sym] = found
	}
	return found, ok
}

func (r *Resolver) searchFeatureOf(sym *symbols.Symbol, name string, walk featureWalk) (*symbols.Symbol, bool) {
	if sym.Scope != nil {
		if found, ok := walk.hide.lookupLocal(sym.Scope, name); ok && inheritableMember(found) {
			return found, true
		}
		if found, ok := r.importedFeatureOf(sym, name); ok && !walk.hide.hides(found) {
			return found, true
		}
	} else if sym.Decl == nil && r.idx != nil {
		// A restored library symbol has no scope; its members are indexed.
		for _, found := range r.idx.LookupQualified(sym.Name + "::" + name) {
			resolved, ok := r.ResolveAliasTarget(found)
			if ok && inheritableMember(resolved) && !walk.hide.hides(resolved) {
				return resolved, true
			}
		}
	}
	for _, general := range r.generalsOf(sym) {
		// A feature of sym redefining what a general declares means sym does not
		// inherit it, under that name or any other (KerML 8.3.3.3).
		if found, ok := r.featureOf(general, name, walk); ok {
			if found, ok = r.passedOnAs(sym, found); ok {
				return found, true
			}
		}
	}
	return nil, false
}

// importedFeatureOf finds name among the memberships sym imports publicly or
// protectedly, which its specializations inherit like its own (KerML 8.2.3.5).
func (r *Resolver) importedFeatureOf(sym *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	for _, imp := range r.importsOf(sym.Scope.Node()) {
		if !inheritedThroughSpecialization(imp) || !r.importPrefixAvailable(sym.Scope, imp, name) {
			continue
		}
		if found, ok := r.matchImport(sym.Scope, imp, name); ok {
			return found, true
		}
	}
	return nil, false
}

// inheritableMember reports whether a member can be reached through a
// specialization: KerML 8.2.3.5 excludes private memberships from
// inheritedMembership, so only public and protected members are inherited.
func inheritableMember(sym *symbols.Symbol) bool {
	return sym != nil && symbols.VisibleAs(sym.Visibility, false, true)
}

// inheritedAs returns what sym has where found was reached by name: found itself
// when sym inherits it, else the feature of sym that redefines found and so
// answers to its names (KerML 7.3.4.5), else nothing.
func (r *Resolver) inheritedAs(sym, found *symbols.Symbol) (*symbols.Symbol, bool) {
	return r.inheritedAsFrom(sym, found, nil)
}

// passedOnAs returns what sym passes on where found was reached through one of
// its generals: found itself unless a feature sym declares redefines it, else that
// feature when it is named by redefining found (KerML 7.3.4.5), else nothing.
// Masks along another path to found are that path's to apply.
func (r *Resolver) passedOnAs(sym, found *symbols.Symbol) (*symbols.Symbol, bool) {
	model, ok := r.model.(maskChecker)
	if !ok || !model.OwnRedefinitionMasked(sym, found) {
		return found, true
	}
	if redefiner := model.OwnNamingRedefiner(sym, found); redefiner != nil {
		return redefiner, true
	}
	return nil, false
}

// inheritedAsFrom returns what sym has where found was reached by name among
// everything it inherits: found itself when no feature of sym redefines it, else
// the feature of sym named by redefining it, else nothing. In the type owning a
// redefinition being resolved, that type's own redefinitions mask nothing, so the
// target it names stays resolvable (KerML 8.3.3.3.6); elsewhere every one masks.
func (r *Resolver) inheritedAsFrom(sym, found *symbols.Symbol, hide *refFilter) (*symbols.Symbol, bool) {
	model, ok := r.model.(maskChecker)
	if !ok {
		return found, true
	}
	if hide.resolvesRedefinition() && (hide.redefiner == nil || hide.redefiner.OwnerScope == sym.Scope) {
		if !model.InheritanceMaskedRedefining(sym, found) {
			return found, true
		}
		if redefiner := model.NamingRedefinerRedefining(sym, found); redefiner != nil {
			return redefiner, true
		}
		return nil, false
	}
	if !model.InheritanceMasked(sym, found) {
		return found, true
	}
	if redefiner := model.NamingRedefiner(sym, found); redefiner != nil {
		return redefiner, true
	}
	return nil, false
}

// generalsOf returns the symbols sym inherits features from: its supertypes as
// the semantic model derives them, implicit ones included, else the resolved
// specialization, subsetting, redefinition and typing targets of its
// declaration; and the types featuring it either way.
func (r *Resolver) generalsOf(sym *symbols.Symbol) []*symbols.Symbol {
	var rels []*ast.Relationship
	if oc, ok := ast.OwnedConstraintOf(sym.Decl); ok {
		rels = oc.Relationships
	} else {
		switch decl := sym.Decl.(type) {
		case *ast.Definition:
			rels = decl.Relationships
		case *ast.Usage:
			rels = decl.Relationships
		case *ast.CrossFeatureMember:
			rels = decl.Relationships
		}
	}
	var generals []*symbols.Symbol
	if model, ok := r.model.(supertypeProvider); ok {
		generals = append(generals, model.DirectSupertypes(sym)...)
	} else if rels != nil {
		generals = r.findGeneralizationTargets(sym, rels)
	}
	if rels != nil {
		generals = append(generals, r.findFeaturedByTargets(sym, rels)...)
	}
	return generals
}

// resolveOwnSibling resolves a subsetting target to a sibling of decl that redefines
// the inherited feature of that name, which it shadows in the owning type (KerML 7.3.4.5).
func (r *Resolver) resolveOwnSibling(scope *symbols.Scope, qn *ast.QualifiedName, decl ast.Node) bool {
	if qn == nil || len(qn.Parts) != 1 || scope == nil {
		return false
	}
	switch scope.Node().(type) {
	case *ast.Definition, *ast.Usage:
	default:
		return false
	}
	sym, ok := scope.LookupLocal(qn.Parts[0].Text)
	if !ok || sym == nil || sym.Decl == decl {
		return false
	}
	if !redefinesUnderItsName(sym.Decl, sym.EffectiveName()) {
		return false
	}
	r.recordRedefined(qn, sym, true)
	return true
}

// redefinesUnderItsName reports whether decl is a feature redefinition found under
// the name it borrows from its target (`:>> x`) or restates itself (`x :>> x`).
func redefinesUnderItsName(decl ast.Node, borrowed bool) bool {
	if !isFeatureDecl(decl) {
		return false
	}
	if borrowed {
		naming := ast.DeclNamingFeature(decl)
		return naming != nil && naming.Kind == ast.RelRedefines
	}
	return len(redefinesRelationships(decl)) > 0
}

// resolveRedefinition resolves a redefinition target from the generals of the
// owning type, then from the enclosing namespace outward; the owning type's own
// members, imports and aliases are never consulted (KerML 8.2.3.5.2).
//
// decl owns the redefinition. An unnamed redefining feature takes the redefined
// feature's name (KerML 7.3.4.5), so that borrowed binding is hidden from the
// target, which names the redefined feature itself.
func (r *Resolver) resolveRedefinition(scope *symbols.Scope, qn *ast.QualifiedName, decl ast.Node, redefines bool) {
	if qn == nil || len(qn.Parts) == 0 {
		return
	}
	if _, done := r.memo[qn]; done {
		return
	}
	// The generals of the owner are found by resolving its own redefinition
	// targets, so a re-entrant query of the same target is cut short.
	if depth := r.redefining[qn]; depth != 0 {
		r.CutShort(depth)
		return
	}

	hide := &refFilter{
		decl:             decl,
		redefiner:        scope.MemberDeclaring(decl),
		skipBorrowedName: true,
		redefining:       redefines,
	}
	r.redefining[qn] = r.Enter()
	defer delete(r.redefining, qn)

	if len(qn.Parts) == 1 {
		featureName := qn.Parts[0].Text
		if sym, ok := r.lookupConstraintRefFeature(featureName, decl); ok {
			r.recordRedefined(qn, sym, r.Leave())
			return
		}
	}

	ownerNode := scope.Node()
	switch ownerNode.(type) {
	case *ast.Definition, *ast.Usage:
	case *ast.PrefixMetadata:
		// An annotation body's declarations redefine the metaclass's own
		// features (KerML 7.4.7), so the target is looked up there first.
		if len(qn.Parts) == 1 {
			if sym, ok := r.featureOf(r.scopeOwner(scope), qn.Parts[0].Text, newFeatureWalk(hide)); ok {
				r.recordRedefined(qn, sym, r.Leave())
				return
			}
		}
		r.Leave()
		r.resolveQualified(scope, qn, hide)
		return
	case *ast.Package:
		r.Leave()
		r.resolveQualified(scope, qn, hide)
		return
	default:
		r.Leave()
		r.resolveQualified(scope, qn, hide)
		return
	}

	parents := r.generalsOf(scope.Owner())
	if def, ok := ownerNode.(*ast.Definition); ok {
		parents = append(parents, r.findImplicitSpecializations(scope, def)...)
	}

	if len(qn.Parts) == 1 {
		featureName := qn.Parts[0].Text

		// Search every parent's inheritance chain, cached and live alike. What a
		// feature of the owner redefines is not inherited, so a subsetting
		// cannot name it; a redefinition of it can (KerML 8.3.3.3.6).
		owner := scope.Owner()
		walk := newFeatureWalk(hide)
		var candidates []*symbols.Symbol
		for _, parentSym := range parents {
			sym, ok := r.featureOf(parentSym, featureName, walk)
			if !ok {
				continue
			}
			if !redefines {
				if sym, ok = r.inheritedAs(owner, sym); !ok {
					continue
				}
			}
			candidates = append(candidates, sym)
		}

		// The semantic model also contributes members through the base every
		// usage or KerML feature implicitly subsets and what it reference-subsets.
		if sym, ok := r.lookupContributedMember(owner, featureName, hide); ok &&
			visibleAsInheritedMember(owner, sym) {
			if sym, ok = r.inheritedAsFrom(owner, sym, hide); ok {
				candidates = append(candidates, sym)
			}
		}
		if sym := r.preferRedefining(candidates); sym != nil {
			r.recordRedefined(qn, sym, r.Leave())
			return
		}
	} else {
		// The first segment is looked up as a single name is; the tail walks
		// the members of what it named (KerML 8.2.3.5.2).
		first := qn.Parts[0].Text
		owner := scope.Owner()
		walk := newFeatureWalk(hide)
		var shadowing *symbols.Symbol
		for _, parentSym := range parents {
			head, ok := r.featureOf(parentSym, first, walk)
			if !ok {
				continue
			}
			if !redefines {
				if head, ok = r.inheritedAs(owner, head); !ok {
					continue
				}
			}
			var result resolution
			if r.probe(qn, func() bool {
				result = r.walkQualifiedTail(scope, qn, r.resolvedPart(qn, 0, head), 1, hide)
				return result.ok
			}) {
				if r.Leave() {
					r.memoize(qn, result)
				}
				return
			}
			if shadowing == nil {
				shadowing = head
			}
		}
		// A general that offers the first segment shadows the enclosing
		// namespaces: the tail fails under it rather than binding elsewhere.
		if redefines && shadowing != nil {
			result := r.walkQualifiedTail(scope, qn, r.resolvedPart(qn, 0, shadowing), 1, hide)
			if r.Leave() && r.quiet == 0 {
				r.memoize(qn, result)
			}
			return
		}
	}

	r.Leave()
	if redefines {
		// A redefinition target the generals lack is looked up from the enclosing
		// namespace outward; the owning type's own scope is never consulted.
		r.resolveQualified(scope.Parent(), qn, hide)
		return
	}
	r.resolveQualified(scope, qn, hide)
}

// declaresConnector reports whether scope is the body of a connector, whose
// ends relate features of the type featuring it (KerML 8.3.4.5).
func declaresConnector(scope *symbols.Scope) bool {
	u, ok := scope.Node().(*ast.Usage)
	if !ok {
		return false
	}
	switch u.Kind {
	case ast.UsageConnector, ast.UsageConnection, ast.UsageBinding,
		ast.UsageSuccession, ast.UsageFlow, ast.UsageAllocation:
		return true
	}
	return false
}

// preferRedefining chooses among the features the generals offer under one name:
// the first found, displaced by a later one that redefines it (KerML 8.2.3.5.2).
func (r *Resolver) preferRedefining(candidates []*symbols.Symbol) *symbols.Symbol {
	var chosen *symbols.Symbol
	for _, candidate := range candidates {
		if chosen == nil || candidate == chosen {
			chosen = candidate
			continue
		}
		if r.redefinedClosure(candidate)[chosen] {
			chosen = candidate
		}
	}
	return chosen
}

// recordRedefined records sym as what the redefinition target qn names, and
// memoizes it when settled, so a later unfiltered query — the semantic model
// reading the same relationship — does not find the borrowed name instead.
func (r *Resolver) recordRedefined(qn *ast.QualifiedName, sym *symbols.Symbol, settled bool) {
	r.recordPart(qn, 0, sym)
	if settled {
		r.memoize(qn, resolution{sym: sym, ok: true})
	}
}

// findGeneralizationTargets resolves the generals rels declare for sym: what
// it specializes, subsets, redefines and is typed by, each target read as the
// document walk reads it, so a redefinition target is looked up in sym's
// generals and a chain target generalizes to its final feature.
func (r *Resolver) findGeneralizationTargets(sym *symbols.Symbol, rels []*ast.Relationship) []*symbols.Symbol {
	var parents []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	add := func(target *symbols.Symbol) {
		resolved, ok := r.ResolveAliasTarget(target)
		if !ok || resolved == nil || resolved == sym || seen[resolved] {
			return
		}
		seen[resolved] = true
		parents = append(parents, resolved)
	}
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSpecializes, ast.RelSubsets, ast.RelRedefines, ast.RelTyping:
		default:
			continue
		}
		scope := r.relationshipScope(sym.OwnerScope, sym.Scope, rel)
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		switch target := target.(type) {
		case *ast.FeatureChainExpr:
			if found, ok := r.ResolveTarget(scope, target); ok {
				add(found)
			}
		case *ast.QualifiedName:
			ref := Reference{Scope: scope, QN: target}
			switch rel.Kind {
			case ast.RelRedefines:
				ref.Referrer, ref.Redefines = sym.Decl, true
			case ast.RelSubsets:
				ref = Reference{Scope: scope, Subsetting: sym.Decl}.Spelled(target)
			}
			if found, ok := r.ResolveReference(ref); ok {
				add(found)
			}
		}
	}
	return parents
}

// findTypingTargets resolves the types named by the typing relationships in
// rels, which are the generals a usage inherits members from.
func (r *Resolver) findTypingTargets(scope *symbols.Scope, rels []*ast.Relationship) []*symbols.Symbol {
	var parents []*symbols.Symbol
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			if sym, ok := r.ResolveQualified(scope, qn); ok && sym != nil {
				if resolved, aliasOK := r.ResolveAliasTarget(sym); aliasOK {
					sym = resolved
				} else {
					continue
				}
				parents = append(parents, sym)
			}
		}
	}
	return parents
}

// findFeaturedByTargets resolves the types rels declare sym featured by, each
// target read in the scope the document walk reads it in.
func (r *Resolver) findFeaturedByTargets(sym *symbols.Symbol, rels []*ast.Relationship) []*symbols.Symbol {
	var parents []*symbols.Symbol
	for _, rel := range rels {
		if rel == nil || rel.Kind != ast.RelFeaturedBy || rel.Target == nil {
			continue
		}
		scope := r.relationshipScope(sym.OwnerScope, sym.Scope, rel)
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		if qn, ok := target.(*ast.QualifiedName); ok {
			if found, ok := r.ResolveQualified(scope, qn); ok && found != nil {
				if resolved, aliasOK := r.ResolveAliasTarget(found); aliasOK {
					parents = append(parents, resolved)
				}
			}
		}
	}
	return parents
}

// ownedEndCount counts the `end` features def declares in its body.
func ownedEndCount(def *ast.Definition) int {
	n := 0
	for _, member := range def.Members {
		if mb, ok := member.(*ast.Membership); ok {
			member = mb.Member
		}
		if u, ok := member.(*ast.Usage); ok && u.IsEnd {
			n++
		}
	}
	return n
}

// findImplicitSpecializations returns implicit base types for a definition based on its kind.
// For example, 'flow def' implicitly specializes 'MessageAction' from the systems library.
func (r *Resolver) findImplicitSpecializations(scope *symbols.Scope, def *ast.Definition) []*symbols.Symbol {
	var parents []*symbols.Symbol

	// Map definition kind to base type FQN
	var baseFQN string
	switch def.Kind {
	case ast.DefPart:
		baseFQN = "Parts::Part"
	case ast.DefItem:
		baseFQN = "Items::Item"
	case ast.DefFlow:
		baseFQN = "Flows::MessageAction"
		if ownedEndCount(def) == 2 {
			baseFQN = "Flows::Message"
		}
	case ast.DefConnection:
		baseFQN = "Connections::Connection"
	case ast.DefInterface:
		baseFQN = "Interfaces::Interface"
	case ast.DefAllocation:
		baseFQN = "Allocations::Allocation"
	// Add more as needed
	default:
		return nil
	}

	// Look up base type in index
	if r.idx != nil {
		candidates := r.idx.LookupQualified(baseFQN)
		if len(candidates) == 1 {
			parents = append(parents, candidates[0])
		}
	}

	return parents
}

// resolveExpr walks an expression subtree resolving feature references and
// classification type references.
func (r *Resolver) resolveExpr(scope *symbols.Scope, e ast.Node) {
	switch v := e.(type) {
	case nil:
		return
	case *ast.FeatureReference:
		r.ResolveQualified(scope, v.Name)
	case *ast.OperatorExpr:
		for _, op := range v.Operands {
			r.resolveExpr(scope, op)
		}
		if v.TypeRef != nil {
			r.ResolveQualified(scope, v.TypeRef)
		}
	case *ast.FeatureChainExpr:
		r.resolveFeatureChain(scope, v)
	case *ast.IndexExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Index)
	case *ast.InvocationExpr:
		r.resolveExpr(scope, v.Operand)
		if v.Type != nil {
			r.ResolveInvocationName(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
		for _, na := range v.NamedArgs {
			// Named argument names are parameter identifiers, not references
			// Don't resolve na.Name - it's looked up in callee's parameter list
			r.resolveExpr(scope, na.Value)
		}
	case *ast.CollectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.SelectExpr:
		r.resolveExpr(scope, v.Operand)
		r.resolveExpr(scope, v.Body)
	case *ast.ConstructorExpr:
		var typ *symbols.Symbol
		if v.Type != nil {
			typ, _ = r.ResolveQualified(scope, v.Type)
		}
		for _, a := range v.Args {
			r.resolveExpr(scope, a)
		}
		for _, na := range v.NamedArgs {
			// A simple label is a feature of the instantiated type; a qualified
			// one is resolved in scope.
			switch {
			case na.Name == nil:
			case len(na.Name.Parts) > 1:
				r.ResolveQualified(scope, na.Name)
			case typ != nil:
				r.resolveMemberChain(typ, na.Name, nil)
			}
			r.resolveExpr(scope, na.Value)
		}
	case *ast.BodyExpr:
		for i := range v.Params {
			p := &v.Params[i]
			if p.Type != nil {
				r.ResolveQualified(scope, p.Type)
			}
			r.resolveRelationships(scope, v, p.Relationships)
			r.resolveMultiplicity(scope, p.Multiplicity)
			r.resolveExpr(scope, p.Value)
		}
		// A body expression's parameters and declarations live in a scope of its
		// own, and its declarations are members of it (F64).
		inner := symbols.BodyExprScope(scope, v)
		r.walkMembers(inner, v.Members)
		r.resolveExpr(inner, v.Result)
	case *ast.SequenceExpr:
		for _, el := range v.Elements {
			r.resolveExpr(scope, el)
		}
	case *ast.MetadataAccessExpr:
		r.ResolveQualified(scope, v.Ref)
	case *ast.CastExpr:
		if v.TargetType != nil {
			r.ResolveQualified(scope, v.TargetType)
		}
		r.resolveMultiplicity(scope, v.Multiplicity)
	case *ast.QualifiedName:
		// A bare name in expression position parses straight to a qualified
		// name rather than to a FeatureReference wrapper.
		r.ResolveQualified(scope, v)
	}
	// Literals (LiteralBool/String/Integer/Real/Infinity, NullExpr) have no refs.
}

// resolveFeatureChain resolves a FeatureChainExpr and returns its final symbol.
func (r *Resolver) resolveFeatureChain(scope *symbols.Scope, fc *ast.FeatureChainExpr) *symbols.Symbol {
	if fc == nil {
		return nil
	}
	key := featureChainKey{scope: scope, node: fc}
	if res, done := r.featureChains[key]; done {
		return res.sym
	}
	r.Enter()
	res := r.walkFeatureChain(scope, fc)
	r.memoizeFeatureChain(scope, fc, res, r.Leave())
	return res.sym
}

// walkFeatureChain reads fc in scope as resolveFeatureChain does, unmemoized.
func (r *Resolver) walkFeatureChain(scope *symbols.Scope, fc *ast.FeatureChainExpr) resolution {
	// Get the operand symbol without following its type for inline members.
	operandSym := r.getOperandSymbol(scope, fc.Operand)
	if operandSym == nil || fc.Member == nil {
		return resolution{}
	}
	if featuring := r.featuringOf(scope, operandSym); featuring != nil {
		operandSym = featuring
	}
	// A chain from `this` reads the object the body is owned by, which the
	// library declares as its context occurrence rather than as its type.
	if r.IsOccurrenceThis(operandSym) {
		if object := r.ThisContext(scope); object != nil {
			operandSym = object
		}
	}

	// A chaining feature spelled as a qualified name resolves outward through the
	// enclosing namespaces when the previous element has no such member (KerML
	// §7.2.5): in `A::B.C::D`, `C::D` names a declaration, not a member of `B`.
	// The outward reading is probed, so that a chain the walk below reads instead
	// keeps its own diagnostics and per-segment symbols (see probe).
	if len(fc.Member.Parts) > 1 {
		if _, ok := r.chainMember(operandSym, fc.Member.Parts[0].Text, fc); !ok {
			var outwardSym *symbols.Symbol
			outward := r.probe(fc.Member, func() bool {
				var ok bool
				outwardSym, ok = r.ResolveQualified(scope, fc.Member)
				return ok
			})
			if outward {
				return resolution{sym: outwardSym, ok: outwardSym != nil}
			}
		}
	}

	memberSym := r.resolveMemberChain(operandSym, fc.Member, fc)
	return resolution{sym: memberSym, ok: memberSym != nil}
}

// chainMember looks a chain segment up as a member of sym itself — flattened over
// its generalizations when a model is attached — else of its type or its value.
func (r *Resolver) chainMember(sym *symbols.Symbol, name string, chain ast.Node) (*symbols.Symbol, bool) {
	return r.chainMemberOf(sym, name, chain, nil)
}

func (r *Resolver) chainMemberOf(sym *symbols.Symbol, name string, chain ast.Node, seen map[*symbols.Symbol]bool) (*symbols.Symbol, bool) {
	if sym == nil {
		return nil, false
	}
	found, ok := r.lookupMember(sym, name, nil)
	if ok && namedByChain(found, chain) {
		found, ok = r.lookupContributedMember(sym, name, nil)
	}
	if ok && r.namedThroughNamespace(found) {
		if found, ok = r.inheritedAs(sym, found); ok {
			return found, true
		}
	}
	found, ok = r.LocalBinding(sym.Scope, name)
	if ok && !namedByChain(found, chain) && r.namedThroughNamespace(found) {
		return found, true
	}
	// A usage without the member reads it from its type or, untyped, from the
	// feature its value names; seen stops a chain of values from looping.
	usage, isUsage := sym.Decl.(*ast.Usage)
	if !isUsage || seen[sym] || r.IsBaseThat(sym) || r.IsOccurrenceThis(sym) {
		return nil, false
	}
	if seen == nil {
		seen = make(map[*symbols.Symbol]bool)
	}
	seen[sym] = true
	if typed := r.getUsageType(sym.OwnerScope, usage); typed != nil {
		return r.chainMemberOf(typed, name, chain, seen)
	}
	return nil, false
}

// namedByChain reports whether sym borrowed its name from chain, the feature
// chain now being resolved: `assert h.q;` inside h names H's q, not itself.
func namedByChain(sym *symbols.Symbol, chain ast.Node) bool {
	return chain != nil && sym != nil && sym.NamingTarget == chain
}

// resolveMemberChain walks a qualified name member-by-member in the given scope,
// assigning each part's symbol explicitly (for feature chain member access).
func (r *Resolver) resolveMemberChain(parentSym *symbols.Symbol, qn *ast.QualifiedName, chain ast.Node) *symbols.Symbol {
	if qn == nil || len(qn.Parts) == 0 {
		return nil
	}
	r.Enter()
	cur, ok := r.walkMemberChain(parentSym, qn, chain)
	if r.Leave() && ok {
		r.memoize(qn, resolution{cur, true})
	}
	return cur
}

// walkMemberChain reads the members qn names, one per part, from parentSym,
// reporting the first part that names none.
func (r *Resolver) walkMemberChain(parentSym *symbols.Symbol, qn *ast.QualifiedName, chain ast.Node) (*symbols.Symbol, bool) {
	// Resolve first part using model.LookupMember if available, else
	// scope.LookupLocal. A chained feature names a member of what precedes it,
	// so it reaches only the visible ones (KerML 8.2.3.5).
	cur, ok := r.chainMember(parentSym, qn.Parts[0].Text, chain)

	if !ok {
		msg := "unresolved member: " + qn.Parts[0].Text
		if r.memberless(parentSym) {
			msg = "no scope for member lookup in " + parentSym.Name
		}
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Span: qn.Parts[0].Span, Message: msg})
		return nil, false
	}
	r.recordPart(qn, 0, cur)

	// Walk remaining parts via member lookup
	for i := 1; i < len(qn.Parts); i++ {
		next, found := r.chainMember(cur, qn.Parts[i].Text, chain)
		if !found && r.memberless(cur) {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Span:    qn.Parts[i].Span,
				Message: "no members in " + cur.Name,
			})
			return nil, false
		}

		if !found {
			r.Diagnostics = append(r.Diagnostics, Diagnostic{
				Span:    qn.Parts[i].Span,
				Message: "unresolved member: " + qn.Parts[i].Text + " in " + cur.Name,
			})
			return nil, false
		}

		r.recordPart(qn, i, next)
		cur = next
	}
	return cur, true
}

// memberless reports a symbol with neither a scope nor a type to read members from,
// such as an untyped body parameter over a collection whose elements are untyped.
func (r *Resolver) memberless(sym *symbols.Symbol) bool {
	if sym.Scope != nil {
		return false
	}
	model, ok := r.model.(supertypeProvider)
	return !ok || len(model.DirectSupertypes(sym)) == 0
}

// getOperandSymbol returns the feature an expression operand names, which the
// chain's next segment is read as a member of (see chainMember).
func (r *Resolver) getOperandSymbol(scope *symbols.Scope, e ast.Node) *symbols.Symbol {
	switch v := e.(type) {
	case *ast.FeatureReference:
		if v.Name == nil {
			return nil
		}
		var sym *symbols.Symbol
		var ok bool
		if len(v.Name.Parts) == 1 && !v.Name.Global {
			sym, ok = r.LookupName(scope, v.Name.Parts[0].Text)
			if ok {
				r.resolvedPart(v.Name, 0, sym)
			} else {
				sym, ok = r.ResolveQualified(scope, v.Name)
			}
		} else {
			sym, ok = r.ResolveQualified(scope, v.Name)
		}
		if !ok {
			return nil
		}
		return sym
	case *ast.FeatureChainExpr:
		return r.resolveFeatureChain(scope, v)
	case *ast.IndexExpr:
		// `a#(i)` names one element of the sequence a, of a's own type.
		if v.Bracket {
			r.resolveExpr(scope, e)
			return nil
		}
		r.resolveExpr(scope, v.Index)
		return r.getOperandSymbol(scope, v.Operand)
	default:
		r.resolveExpr(scope, e)
		return nil
	}
}

// baseThatFQN is the implicit `that` feature every usage takes from the base
// usage Base::things ([KerML, 8.4.2]).
const baseThatFQN = "Base::things::that"

// featuringOf returns what a member chain from `that` reads its members from:
// the usage enclosing the expression, whose value features the value being
// written. It is nil for any other operand, and where no usage encloses the
// expression — `that` is typed Anything, which has no members of its own.
func (r *Resolver) featuringOf(scope *symbols.Scope, operand *symbols.Symbol) *symbols.Symbol {
	if !r.IsBaseThat(operand) {
		return nil
	}
	for s := scope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == nil {
			continue
		}
		if _, ok := owner.Decl.(*ast.Usage); ok {
			return owner
		}
		return nil
	}
	return nil
}

// IsBaseThat reports whether sym is the implicit `that` feature of the base
// usage: it names the object featuring a usage's values, so it is reachable in
// any usage body and its declared type Anything is not what a chain reads.
func (r *Resolver) IsBaseThat(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	return sym.Name == baseThatFQN || r.registeredFQN(sym) == baseThatFQN
}

// getUsageType returns the type symbol of a usage by resolving its typing relationship.
func (r *Resolver) getUsageType(scope *symbols.Scope, usage *ast.Usage) *symbols.Symbol {
	for _, rel := range usage.Relationships {
		if rel.Kind == ast.RelTyping && rel.Target != nil {
			// Unwrap FeatureReference if needed
			target := rel.Target
			if fr, ok := target.(*ast.FeatureReference); ok {
				target = fr.Name
			}
			if qn, ok := target.(*ast.QualifiedName); ok {
				typeSym, _ := r.ResolveQualified(scope, qn)
				return typeSym
			}
		}
	}
	// A feature with no declared type takes the type of the value bound to it
	// (KerML 1.0 §7.4.9 FeatureValue), so its members are the value's members.
	return r.valueType(scope, usage)
}

// valueType returns the feature a usage's value expression names, for the member
// lookups a chain through the usage makes. Only the forms that denote a feature
// are followed; anything else has no members to reach.
func (r *Resolver) valueType(scope *symbols.Scope, usage *ast.Usage) *symbols.Symbol {
	if usage.Value == nil || r.valuesInProgress[usage] {
		return nil
	}
	r.valuesInProgress[usage] = true
	defer delete(r.valuesInProgress, usage)

	expr := usage.Value
	var sym *symbols.Symbol
	// The value is read on behalf of a member lookup, so its own diagnostics
	// belong to the reference that wrote it, not to this one.
	r.aside(func() {
		if found, ok := r.ResolveTarget(scope, expr); ok {
			sym = found
		}
	})
	return sym
}
