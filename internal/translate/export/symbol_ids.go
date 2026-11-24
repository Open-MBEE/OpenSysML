package export

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// SymbolIDs spells a model's symbols by the elementIds Convert writes for the elements they reflect
// (ElementIDs), so a symbol-backed reader and the API element form name one element one way (Symbol resolves back).
type SymbolIDs struct {
	ids   *ElementIDs
	model *semantics.Model
	index *symbols.Index

	mu       sync.Mutex
	bySymbol map[*symbols.Symbol]string
	symbols  map[string]*symbols.Symbol
	order    []*symbols.Symbol
	walked   bool
}

// NewSymbolIDs spells the symbols of model, analyzed over res, by the ids ids
// records; the documents are those ids was built for.
func NewSymbolIDs(ids *ElementIDs, res *resolve.Resolver, model *semantics.Model) *SymbolIDs {
	s := &SymbolIDs{ids: ids, model: model, bySymbol: map[*symbols.Symbol]string{}, symbols: map[string]*symbols.Symbol{}}
	if res != nil {
		s.index = res.Index()
	}
	return s
}

// Documents are the names of the documents ids records the elements of, in model order.
func (ids *ElementIDs) Documents() []string {
	if ids == nil {
		return nil
	}
	names := make([]string, 0, len(ids.encoders))
	for _, e := range ids.encoders {
		if !ids.leftOut[e.file.Name()] {
			names = append(names, e.file.Name())
		}
	}
	return names
}

// Of is the elementId the conversion writes for the element sym reflects; false where it writes
// none (a `for` variable, an element the mapping does not cover).
func (s *SymbolIDs) Of(sym *symbols.Symbol) (string, bool) {
	if s == nil || sym == nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.of(sym)
}

// Symbol is the symbol whose element the conversion writes under id, among the
// elements reachable by ownership from the documents' root namespaces.
func (s *SymbolIDs) Symbol(id string) (*symbols.Symbol, bool) {
	if s == nil || id == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.walk()
	sym, ok := s.symbols[id]
	return sym, ok
}

// Reachable are the symbols reachable by ownership from the documents' root
// namespaces, the roots first, each once, in model order.
func (s *SymbolIDs) Reachable() []*symbols.Symbol {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.walk()
}

// walk reaches every symbol from each document's root through ownership once,
// recording the id of each, under s.mu.
func (s *SymbolIDs) walk() []*symbols.Symbol {
	if s.walked {
		return s.order
	}
	s.walked = true
	seen := map[*symbols.Symbol]bool{}
	var order []*symbols.Symbol
	var visit func(sym *symbols.Symbol)
	visit = func(sym *symbols.Symbol) {
		if sym == nil || seen[sym] {
			return
		}
		seen[sym] = true
		order = append(order, sym)
		for _, rel := range s.model.OwnedRelationships(sym) {
			visit(rel)
			if related, ok := s.model.ReflectiveElements(rel, "ownedRelatedElement"); ok {
				for _, owned := range related {
					visit(owned)
				}
			}
		}
	}
	for _, doc := range s.ids.Documents() {
		visit(s.model.RootNamespace(doc))
	}
	s.order = order
	for _, sym := range order {
		if id, ok := s.of(sym); ok {
			if _, taken := s.symbols[id]; !taken {
				s.symbols[id] = sym
			}
		}
	}
	return order
}

func (s *SymbolIDs) of(sym *symbols.Symbol) (string, bool) {
	if id, ok := s.bySymbol[sym]; ok {
		return id, id != ""
	}
	id, ok := s.compute(sym)
	if !ok {
		id = ""
	}
	s.bySymbol[sym] = id
	return id, ok
}

func (s *SymbolIDs) compute(sym *symbols.Symbol) (string, bool) {
	doc := sym.DocName
	switch {
	case sym.Implicit != nil:
		return s.implicitID(sym.Implicit)
	case semantics.IsFilterPackage(sym):
		// The filter package is minted off the import that imports and owns it (filterPackage).
		base, ok := s.of(s.model.FilterPackageImport(sym))
		return base + filterPackageSuffix, ok
	case sym.Expr != nil:
		return s.expressionID(sym)
	case s.model.IsRootNamespace(sym):
		return s.rootNamespaceID(sym)
	}
	// A library alias is written under the id its Membership is minted.
	if len(s.ids.encoders) > 0 {
		if membership, ok := s.ids.encoders[0].ids.libraryAliasMembership(sym); ok {
			return rdf.LocalName(membership.Value), true
		}
	}
	// A `for` variable declares no node of its own: its symbol is keyed by the
	// loop, which the conversion writes it under (loopVariable).
	if semantics.IsLoopVariable(sym) {
		element, _, ok := s.ids.OfExpressionPart(doc, sym.Decl, xLoopVariable)
		return element, ok
	}
	// A body's parameter declares no node of its own: its symbol is keyed by the
	// body, which the conversion writes as the reference the parameter is in<i> of.
	if body, ok := sym.Decl.(*ast.BodyExpr); ok && sym.Name != "" {
		element, _, ok := s.ids.OfBodyParameter(doc, body, sym.Name)
		return element, ok
	}
	if id, ok, settled := s.chainFeatureID(sym); settled {
		return id, ok
	}
	if id, ok, settled := s.impliedPartID(sym); settled {
		return id, ok
	}
	fqn := s.fqn(sym)
	id, ok := s.ids.OfDeclaration(doc, sym.Decl, fqn)
	if ok && sym.Name == "" && id == fqn {
		// An unnamed member of a document the table does not hold (a library
		// type's `return : Boolean;`) has no name to be keyed by; its catalog does.
		return "", false
	}
	return id, ok
}

// chainFeatureID is the id of the chain Feature sym is, minted off the end, flow or relationship
// the chain is written on. settled is false when sym is no such chain Feature.
func (s *SymbolIDs) chainFeatureID(sym *symbols.Symbol) (id string, ok, settled bool) {
	doc := sym.DocName
	// A chained end's target is the chain Feature its ReferenceSubsetting owns, minted
	// off the end (chainFeature), which the chain's own node is not written as.
	if sym.Kind == symbols.SymbolReferenceUsage && sym.OwnerScope != nil {
		if end, ok := sym.OwnerScope.Node().(*ast.ConnectorEnd); ok && end.AttachedTarget() == sym.Decl {
			if element, _, ok := s.ids.OfExpressionPart(doc, end, "chain"); ok {
				return element, true, true
			}
		}
	}
	if sym.Chain != nil && sym.Chain.Flow != nil && sym.Chain.Node != nil {
		// The chain Feature a flow's end references is minted under the end, filed by the chain as written.
		element, _, ok := s.ids.OfExpressionPart(doc, sym.Chain.Node, "chain")
		return element, ok, true
	}
	// The chain Feature a relationship's chain target denotes is minted off the element
	// the relationship is written on: an end's `_pend0_pchain`, a transition's `_psourcechain`.
	if sym.Chain != nil && sym.Chain.Relationship != nil && sym.Chain.Relationship.Implicit != nil && sym.Chain.Relationship.Implicit.Owner != nil {
		if element, _, ok := s.ids.OfExpressionPart(doc, sym.Decl, "chain"); ok {
			// A head clause's chain (`subsets a.b`) is minted off the feature at its position among the head's chains (headChain).
			return element, true, true
		}
		if base, ok := s.of(sym.Chain.Relationship.Implicit.Owner); ok {
			slot := "chain"
			if sym.Chain.Relationship.Implicit.Slot == "sourcemember" {
				slot = "sourcechain"
			}
			return rdf.ExpressionNodeID(base, slot), true, true
		}
	}
	return "", false, false
}

// impliedPartID is the id of an element its owner's declaration implies — a succession's or flow's
// end, a flow end's feature, a multiplicity — minted under that declaration. settled is false for any other symbol.
func (s *SymbolIDs) impliedPartID(sym *symbols.Symbol) (id string, ok, settled bool) {
	doc := sym.DocName
	if i, ok := symbols.ImpliedConnectorEndIndex(sym); ok {
		// A `then` succession's or a flow's end is minted under its edge at end0/end1 (recordSuccessionEnds,
		// recordFlowEnds); a transition's succession's under the transition at succession.end0/end1.
		implied, _ := symbols.ImpliedDeclOf(sym)
		slot := fmt.Sprintf("end%d", i)
		if symbols.ImpliedTransitionSuccessionEnd(sym) {
			slot = "succession." + slot
		}
		element, _, ok := s.ids.OfExpressionPart(doc, implied.Of, slot)
		return element, ok, true
	}
	if i, ok := symbols.ImpliedFlowEndFeatureIndex(sym); ok {
		// The ReferenceUsage a flow's end owns is minted under the end (recordFlowEnds).
		implied, _ := symbols.ImpliedDeclOf(sym)
		element, _, ok := s.ids.OfExpressionPart(doc, implied.Of, fmt.Sprintf("end%d.ff", i))
		return element, ok, true
	}
	if implied, ok := symbols.ImpliedDeclOf(sym); ok &&
		(implied.Form == symbols.ImpliedDefaultMultiplicity || implied.Form == symbols.ImpliedMultiplicityRange) {
		if element, _, ok := s.ids.OfExpressionPart(doc, implied.Of, "multiplicity"); ok {
			return element, true, true
		}
		owner, ok := s.ids.OfDeclaration(doc, implied.Of, "")
		if !ok {
			return "", false, true
		}
		return rdf.ExpressionNodeID(owner, "multiplicity"), true, true
	}
	return "", false, false
}

// rootNamespaceID mirrors rootNamespaceIDs: the root Namespace is named off the document's first
// top-level element (an alias or import included) with RootNamespaceSuffix, repeated while written.
func (s *SymbolIDs) rootNamespaceID(root *symbols.Symbol) (string, bool) {
	relationships := s.model.OwnedRelationships(root)
	if len(relationships) == 0 {
		return "", false
	}
	element := relationships[0]
	if element.Implicit != nil && element.Implicit.Member != nil {
		element = element.Implicit.Member
	}
	first, ok := s.of(element)
	if !ok {
		return "", false
	}
	id := first + RootNamespaceSuffix
	for s.ids.writes(id) {
		id += RootNamespaceSuffix
	}
	return id, true
}

// writes reports whether the conversion writes id for a declared element or a
// membership of the documents.
func (ids *ElementIDs) writes(id string) bool {
	for _, written := range ids.byName {
		if written == id {
			return true
		}
	}
	for _, written := range ids.membershipByName {
		if written == id {
			return true
		}
	}
	return false
}

// implicitID mirrors how the conversion mints a relationship it writes for the
// notation: a membership from its member, a written clause from its node.
func (s *SymbolIDs) implicitID(rel *symbols.ImplicitRelationship) (string, bool) {
	doc := ""
	if rel.Owner != nil {
		doc = rel.Owner.DocName
	}
	switch rel.Form {
	case symbols.ImplicitMembership:
		return s.implicitMembershipID(rel, doc)
	case symbols.ImplicitWritten:
		return s.implicitWrittenID(rel, doc)
	case symbols.ImplicitSuccessionEndReference:
		if rel.Owner == nil {
			return "", false
		}
		return s.partID(rel.Owner, "rs")
	case symbols.ImplicitShorthandReference:
		if rel.Owner == nil {
			return "", false
		}
		return s.ids.RelationshipOf(doc, rel.Owner.Decl)
	case symbols.ImplicitTypeArgumentTyping:
		// A type argument's typing is minted off its parameter, as the conversion writes it (typeArgument).
		owner, ok := s.of(rel.Owner)
		return owner + relationshipSuffix(ast.RelTyping, false, 1, 0), ok
	case symbols.ImplicitMetadataBodyRedefinition, symbols.ImplicitParameterRedefinition:
		// The Redefinition a metadata body feature's name or a constructor's named
		// argument implies is minted off the feature, as the conversion writes it.
		owner, ok := s.of(rel.Owner)
		return owner + relationshipSuffix(ast.RelRedefines, false, 1, 0), ok
	case symbols.ImplicitImport:
		if semantics.IsFilterPackage(rel.Owner) {
			base, ok := s.of(rel.Owner)
			return base + filterImportSuffix, ok
		}
		return s.ids.OfDeclaration(doc, rel.Decl, "")
	case symbols.ImplicitAnnotatingRelationship:
		if rel.Owner == nil {
			return "", false
		}
		node := ast.AboutClauseNode(rel.Owner.Decl, rel.Ordinal)
		if node == nil {
			return "", false
		}
		return s.ids.RelationshipOf(doc, node)
	case symbols.ImplicitReferenceMembership:
		return s.implicitReferenceMembershipID(rel, doc)
	case symbols.ImplicitFeatureChaining:
		return s.implicitFeatureChainingID(rel, doc)
	case symbols.ImplicitPortConjugation:
		// A conjugated port definition's PortConjugation is minted off the conjugate (conjugatedPortDefinition).
		implied, ok := symbols.ImpliedDeclOf(rel.Owner)
		if !ok {
			return "", false
		}
		conjugate, ok := s.ids.conjugateOf(doc, implied)
		return conjugate.conjugation, ok
	}
	return "", false
}

// implicitMembershipID is the id of the membership through which rel's owner owns its member.
func (s *SymbolIDs) implicitMembershipID(rel *symbols.ImplicitRelationship, doc string) (string, bool) {
	if rel.Member == nil {
		return "", false
	}
	if filter := semantics.FilterMemberOf(rel.Member); filter != nil {
		// A `filter` member is written as the ElementFilterMembership itself.
		return s.ids.OfDeclaration(doc, filter, "")
	}
	if semantics.IsFilterPackage(rel.Owner) {
		base, ok := s.of(rel.Owner)
		return base + filterMembershipSuffix, ok
	}
	if node, ok := semantics.SubactionMemberOf(rel.Owner, rel.Member); ok {
		// An action a subaction performs is owned through the member written as the membership.
		return s.ids.OfDeclaration(doc, node, "")
	}
	if semantics.IsLoopVariable(rel.Member) {
		// A `for` variable is keyed by its loop; its FeatureMembership is minted off the variable.
		_, membership, ok := s.ids.OfExpressionPart(doc, rel.Member.Decl, xLoopVariable)
		return membership, ok
	}
	if body, ok := rel.Member.Decl.(*ast.BodyExpr); ok && rel.Member.Name != "" {
		// A body's parameter is keyed by its body; its FeatureMembership is minted
		// off the parameter (bodyParameter), not off the body's own membership.
		_, membership, ok := s.ids.OfBodyParameter(doc, body, rel.Member.Name)
		return membership, ok
	}
	if id, ok := s.ids.OwningMembershipOfDeclaration(doc, rel.Member.Decl, s.fqn(rel.Member)); ok && rel.Member.Expr == nil {
		return id, true
	}
	member, ok := s.of(rel.Member)
	if !ok {
		return "", false
	}
	id := member + rdf.OwningMembershipSuffix
	if rel.Owner != nil && s.model.IsRootNamespace(rel.Owner) {
		// A root's membership is minted at wrap time, past any id the conversion writes (rootNamespaceIDs).
		for s.ids.writes(id) {
			id += rdf.OwningMembershipSuffix
		}
	}
	return id, true
}

// implicitWrittenID is the id of a relationship written as a clause of its owner's declaration.
func (s *SymbolIDs) implicitWrittenID(rel *symbols.ImplicitRelationship, doc string) (string, bool) {
	if rel.Node == nil {
		return "", false
	}
	if _, ok := rel.Owner.Decl.(*ast.BodyExpr); ok && rel.Owner.Name != "" {
		// A body parameter's head clauses are materialized off the parameter, in
		// the order written per kind (bodyParameter, materializeRelationships).
		return s.bodyParameterRelationshipID(rel)
	}
	if semantics.IsLoopVariable(rel.Owner) {
		// A `for` variable's head clauses are materialized off the variable (loopVariable).
		return s.bodyParameterRelationshipID(rel)
	}
	if rel.Owner != nil && isConnectorEnd(rel.Owner) && rel.Kind.ReferenceSubsets() {
		// An end's reference subsetting is minted off the end (connectorEnd).
		end, ok := s.of(rel.Owner)
		return rdf.ExpressionNodeID(end, "rs"), ok
	}
	if rel.Node.Span().Len == 0 && rel.Node.Target != nil {
		return s.ids.RelationshipOf(doc, rel.Node.Target)
	}
	return s.ids.RelationshipOf(doc, rel.Node)
}

// implicitReferenceMembershipID is the id of a Membership naming a member its owner does not own.
func (s *SymbolIDs) implicitReferenceMembershipID(rel *symbols.ImplicitRelationship, doc string) (string, bool) {
	if rel.Decl != nil {
		// A `first x;` marker's Membership is minted from the marker itself.
		return s.ids.OfDeclaration(doc, rel.Decl, "")
	}
	if rel.Owner == nil || rel.Slot == "" {
		return "", false
	}
	_, membership, ok := s.ids.OfExpressionPart(doc, rel.Owner.Decl, rel.Slot)
	if !ok && rel.Owner.Expr != nil {
		return s.partID(rel.Owner, rel.Slot)
	}
	if !ok {
		// A transition's source Membership is minted off the transition (transitionSourceMember).
		base, ok := s.of(rel.Owner)
		return rdf.ExpressionNodeID(base, rel.Slot), ok
	}
	return membership, ok
}

// implicitFeatureChainingID is the id of a FeatureChaining of the chain Feature that owns it.
func (s *SymbolIDs) implicitFeatureChainingID(rel *symbols.ImplicitRelationship, doc string) (string, bool) {
	if rel.Owner == nil || rel.Slot == "" {
		return "", false
	}
	if owner := rel.Owner; owner.Expr != nil && owner.Expr.Slot == "referent" {
		// A subject's `by a.b` chain owns its chainings at referent.fcN (recordSatisfySubjectValue).
		if element, _, ok := s.ids.OfExpressionPart(doc, owner.Decl, "referent."+rel.Slot); ok && element != "" {
			return element, true
		}
	}
	return s.partID(rel.Owner, rel.Slot)
}

// expressionID mirrors the conversion's expression ids: a node of the notation
// by its node, a part it mints (parameters, results) by its slot.
func (s *SymbolIDs) expressionID(sym *symbols.Symbol) (string, bool) {
	owner := sym.Expr.Owner
	if owner == nil {
		return "", false
	}
	doc := sym.DocName
	switch sym.Expr.Role {
	case symbols.ExpressionParameter:
		return s.partID(owner, fmt.Sprintf("in%d", sym.Expr.Index))
	case symbols.ExpressionResult:
		return s.partID(owner, "out")
	case symbols.ExpressionBody:
		// The body is minted off the reference wrapping it (bodyExpression); the
		// node itself is recorded at the wrapper.
		return s.partID(owner, "body")
	case symbols.ExpressionBodyResult:
		// A type body's result expression is a member of its own; a `{ … }` body's is
		// minted off the body (result).
		if id, ok := s.ids.OfDeclaration(doc, sym.Decl, ""); ok {
			return id, true
		}
		return s.partID(owner, "result")
	case symbols.ExpressionGuard:
		return s.partID(owner, "guard")
	case symbols.ExpressionChangeWrapper:
		if owner.Expr == nil || owner.Expr.Owner == nil {
			return "", false
		}
		return s.partID(owner.Expr.Owner, "a0")
	case symbols.ExpressionChangeCondition:
		// The wrapper's referent is the condition, minted off the wrapper (changeArgument).
		return s.partID(owner, "change")
	case symbols.ExpressionChainFeature, symbols.ExpressionOperand:
		if sym.Expr.Slot == "filter" && owner.Implicit != nil && semantics.IsFilterPackage(owner.Implicit.Owner) {
			// A filter package's condition is minted off the import written with the filter.
			base, ok := s.of(s.model.FilterPackageImport(owner.Implicit.Owner))
			return rdf.ExpressionNodeID(base, "filter"), ok
		}
		return s.partID(owner, sym.Expr.Slot)
	case symbols.ExpressionNodeParameter:
		// An assignment's or loop's parameter is minted at its slot (nodeParameter).
		return s.partID(owner, sym.Expr.Slot)
	case symbols.ExpressionLoopBody:
		// A loop's body is minted off the loop (loopBody).
		base, ok := s.of(owner)
		return base + loopBodySuffix, ok
	case symbols.ExpressionTransitionSuccession:
		// The conversion mints a transition's succession off the transition (recordTransitionSuccession).
		return s.partID(owner, "succession")
	case symbols.ExpressionTransitionLink:
		// A transition's transitionLink is minted off the transition too (transitionLinkMember).
		return s.partID(owner, "transitionlink")
	case symbols.ExpressionConnectorEnd:
		return s.partID(owner, fmt.Sprintf("end%d", sym.Expr.Index))
	case symbols.ExpressionBound:
		// A range's bound is minted off the feature declaring the range, or off a
		// multiplicity declaration's own id (multiplicityBound).
		if _, declared := owner.Decl.(*ast.MultiplicityDecl); declared {
			return s.partID(owner, sym.Expr.Slot)
		}
		if owner.OwnerScope == nil || owner.OwnerScope.Owner() == nil {
			return "", false
		}
		return s.partID(owner.OwnerScope.Owner(), sym.Expr.Slot)
	case symbols.ExpressionTransitionLinkParameter:
		// A transition's link parameters are minted at linkparam and triggerparam (transitionHeadMembers).
		return s.partID(owner, []string{"linkparam", "triggerparam"}[sym.Expr.Index])
	case symbols.ExpressionTriggerAction:
		// The trigger's accept is minted as the transition's `@trigger` member (encodeTrigger).
		if base, ok := s.of(owner); ok {
			return base + encodedSegment(naming.TriggerSegment), true
		}
	case symbols.ExpressionPayload:
		if owner.Kind == symbols.SymbolSendActionUsage {
			// A send's argument parameter is its unnamed member at the argument's position (encodeSendArguments).
			base, ok := s.of(owner)
			return base + encodedSegment(fmt.Sprintf("@%d", sym.Expr.Index)), ok
		}
		if id, ok := s.ids.OfDeclaration(doc, sym.Decl, ""); ok {
			return id, true
		}
		// A time or change trigger's payload is an implied unnamed parameter of the accept.
		if base, ok := s.of(owner); ok {
			return base + encodedSegment(fmt.Sprintf("@%d", sym.Expr.Index)), true
		}
	}
	return s.declaredExpressionID(sym, owner)
}

// declaredExpressionID is the id of an expression element no role above mints: a value, the
// declaration it is written as, or an argument at its position under its expression.
func (s *SymbolIDs) declaredExpressionID(sym, owner *symbols.Symbol) (string, bool) {
	doc := sym.DocName
	if sym.Expr.Role == symbols.ExpressionValue && owner.Expr != nil && owner.Expr.Role == symbols.ExpressionNodeParameter {
		// The value of an assignment's or loop's parameter is the parameter's `value` part (nodeParameterValue).
		base, ok := s.of(owner)
		return rdf.ExpressionNodeID(base, pValue), ok
	}
	if sym.Expr.Role == symbols.ExpressionValue && owner.Expr != nil && owner.Expr.Role == symbols.ExpressionPayload &&
		owner.Expr.Owner != nil && owner.Expr.Owner.Kind == symbols.SymbolSendActionUsage {
		// The value of a send's argument parameter is the parameter's `value` part.
		base, ok := s.of(owner)
		return rdf.ExpressionNodeID(base, "value"), ok
	}
	if sym.Decl != nil && sym.Decl != owner.Decl {
		if id, ok := s.ids.OfDeclaration(doc, sym.Decl, ""); ok {
			return id, true
		}
	}
	if sym.Expr.Role == symbols.ExpressionValue {
		// A time or change trigger's payload is valued by the event it accepts.
		return s.partID(owner, "value")
	}
	if sym.Expr.Role == symbols.ExpressionArgument {
		// An argument under a statement's operand is minted off its position under
		// the expression, not the parameter binding it.
		expression := owner
		if expression.Expr != nil && expression.Expr.Role == symbols.ExpressionParameter && expression.Expr.Owner != nil {
			expression = expression.Expr.Owner
		}
		slot := sym.Expr.Slot
		if slot == "" {
			slot = fmt.Sprintf("a%d", sym.Expr.Index)
		}
		return s.partID(expression, slot)
	}
	return "", false
}

// partID is the id of the part the conversion mints at slot under owner.
func (s *SymbolIDs) partID(owner *symbols.Symbol, slot string) (string, bool) {
	if element, _, ok := s.ids.OfExpressionPart(owner.DocName, owner.Decl, slot); ok && element != "" {
		return element, true
	}
	if owner.Expr != nil && owner.Expr.Role == symbols.ExpressionBody && owner.Expr.Owner != nil {
		// A body's parts keep the ids minted off the reference wrapping it (bodyExpression).
		owner = owner.Expr.Owner
	}
	if owner.Expr != nil && owner.Expr.Role == symbols.ExpressionResult && owner.Expr.Owner != nil {
		// A constructor's result owns the argument features; their parts keep the
		// ids minted off the constructor.
		owner = owner.Expr.Owner
	}
	base, ok := s.of(owner)
	if !ok {
		return "", false
	}
	return rdf.ExpressionNodeID(base, slot), true
}

func (s *SymbolIDs) fqn(sym *symbols.Symbol) string {
	if s.index == nil || sym == nil {
		return ""
	}
	return s.index.GetFQN(sym)
}

// encodedSegment is how a qualified name's final segment lengthens an encoded element id.
func encodedSegment(segment string) string {
	return strings.TrimPrefix(rdf.EncodeElementID("x::"+segment), "x")
}

// clauseTarget tells apart the targets owner's head clauses name, as clauseObject does: a
// name by its referent, or as written when it resolves to none, and a chain by its own clause.
func (s *SymbolIDs) clauseTarget(owner *symbols.Symbol, rel *ast.Relationship) any {
	name, ok := rel.Target.(*ast.QualifiedName)
	if !ok || qualifiedNameHasChain(name) {
		return rel
	}
	if target := s.model.RelationshipTarget(owner, rel); target != nil {
		return target
	}
	return qualifiedText(name)
}

// isConnectorEnd reports whether sym is a connector's end, declared or synthesized.
func isConnectorEnd(sym *symbols.Symbol) bool {
	return sym.Kind == symbols.SymbolConnectorEnd || sym.Expr != nil && sym.Expr.Role == symbols.ExpressionConnectorEnd
}

// bodyParameterRelationshipID spells the relationship a head clause of a body parameter (`in a : T :> f`)
// or of a `for` variable writes: the parameter's id and the suffix the conversion mints for the clause's kind at its position.
func (s *SymbolIDs) bodyParameterRelationshipID(rel *symbols.ImplicitRelationship) (string, bool) {
	element, ok := s.of(rel.Owner)
	if !ok {
		return "", false
	}
	// As pendingRelationships numbers them: the distinct targets in clause order, then each repeat.
	var targets []any
	var repeated []bool
	at, ordinal := -1, 0
	for _, written := range semantics.RelationshipsOf(rel.Owner) {
		if written == nil {
			continue
		}
		if written.Kind == rel.Kind {
			if ordinal == rel.Ordinal {
				at = len(targets)
			}
			target := s.clauseTarget(rel.Owner, written)
			repeated = append(repeated, slices.Contains(targets, target))
			targets = append(targets, target)
		}
		ordinal++
	}
	if at < 0 {
		return "", false
	}
	index := 0
	for i, repeat := range repeated {
		// A distinct target counts ahead of every repeat, and its own kind in clause order.
		if !repeat && repeated[at] || repeat == repeated[at] && i < at {
			index++
		}
	}
	return element + relationshipSuffix(rel.Kind, false, len(targets), index), true
}
