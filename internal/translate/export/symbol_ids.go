package export

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// SymbolIDs spells the symbols of a model by the elementIds Convert writes for
// the elements they reflect (ElementIDs): declared elements by their declaration,
// the memberships, relationships and expression parts the semantic model
// reflects by the rule the conversion mints theirs with. A symbol-backed reader
// and the API element form thus name one element one way, and an id a client
// read from either resolves back to its symbol (Symbol).
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

// Documents are the names of the documents ids was built for, in model order.
func (ids *ElementIDs) Documents() []string {
	if ids == nil {
		return nil
	}
	names := make([]string, 0, len(ids.encoders))
	for _, e := range ids.encoders {
		names = append(names, e.file.Name())
	}
	return names
}

// Of is the elementId the conversion writes for the element sym reflects; false
// where the conversion writes none (a `for` variable, an element the mapping
// does not cover).
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
	case sym.Expr != nil:
		return s.expressionID(sym)
	case s.model.IsRootNamespace(sym):
		return s.rootNamespaceID(sym)
	}
	// A `for` variable is keyed by its loop, which the conversion writes as a
	// string (sysx:loopVariable), not as an element of its own.
	if _, ok := sym.Decl.(*ast.WhileLoopActionNode); ok && sym.Name != "" {
		return "", false
	}
	// A body's parameter declares no node of its own: its symbol is keyed by the
	// body, which the conversion writes as the reference the parameter is in<i> of.
	if body, ok := sym.Decl.(*ast.BodyExpr); ok && sym.Name != "" {
		element, _, ok := s.ids.OfBodyParameter(doc, body, sym.Name)
		return element, ok
	}
	// A chained end's target is the chain Feature its ReferenceSubsetting owns, minted
	// off the end (chainFeature), which the chain's own node is not written as.
	if sym.Kind == symbols.SymbolReferenceUsage && sym.OwnerScope != nil {
		if end, ok := sym.OwnerScope.Node().(*ast.ConnectorEnd); ok && end.AttachedTarget() == sym.Decl {
			element, _, ok := s.ids.OfExpressionPart(doc, end, "chain")
			return element, ok
		}
		// A flow end's flow feature is minted off the end (flowEndReferences).
		if semantics.IsFlowFeature(sym) {
			element, _, ok := s.ids.OfExpressionPart(doc, sym.OwnerScope.Node(), "ff")
			return element, ok
		}
	}
	if implied, ok := symbols.ImpliedDeclOf(sym); ok &&
		(implied.Form == symbols.ImpliedDefaultMultiplicity || implied.Form == symbols.ImpliedMultiplicityRange) {
		if element, _, ok := s.ids.OfExpressionPart(doc, implied.Of, "multiplicity"); ok {
			return element, true
		}
		owner, ok := s.ids.OfDeclaration(doc, implied.Of, "")
		if !ok {
			return "", false
		}
		return rdf.ExpressionNodeID(owner, "multiplicity"), true
	}
	return s.ids.OfDeclaration(doc, sym.Decl, s.fqn(sym))
}

// rootNamespaceID mirrors rootNamespaceIDs: the root Namespace is named off the
// document's first top-level element with RootNamespaceSuffix, repeated while
// the candidate is an id the conversion already writes.
func (s *SymbolIDs) rootNamespaceID(root *symbols.Symbol) (string, bool) {
	for _, membership := range s.model.OwnedMemberships(root) {
		if membership.Implicit == nil || membership.Implicit.Member == nil {
			continue
		}
		first, ok := s.of(membership.Implicit.Member)
		if !ok {
			return "", false
		}
		id := first + RootNamespaceSuffix
		for s.ids.writes(id) {
			id += RootNamespaceSuffix
		}
		return id, true
	}
	return "", false
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
		if rel.Member == nil {
			return "", false
		}
		if _, ok := rel.Decl.(*ast.FilterMember); ok {
			// A `filter` member is written as the ElementFilterMembership itself.
			return s.ids.OfDeclaration(doc, rel.Decl, "")
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
	case symbols.ImplicitWritten:
		if rel.Node == nil {
			return "", false
		}
		if rel.Owner != nil && isConnectorEnd(rel.Owner) && rel.Kind.ReferenceSubsets() {
			// An end's reference subsetting is minted off the end (connectorEnd).
			end, ok := s.of(rel.Owner)
			return rdf.ExpressionNodeID(end, "rs"), ok
		}
		// A flow feature's redefinition is a property the conversion writes, which the
		// api-json structure derives as a Redefinition minted off the feature.
		if rel.Kind == ast.RelRedefines && (semantics.IsFlowFeature(rel.Owner) || semantics.IsMetadataBodyFeature(rel.Owner) && rel.Node.Target == rel.Owner.Decl) {
			owner, ok := s.of(rel.Owner)
			return owner + relationshipSuffix(ast.RelRedefines, false, false, 1, 0), ok
		}
		if rel.Node.Span().Len == 0 && rel.Node.Target != nil {
			return s.ids.RelationshipOf(doc, rel.Node.Target)
		}
		return s.ids.RelationshipOf(doc, rel.Node)
	case symbols.ImplicitImport:
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
	case symbols.ImplicitFeatureChaining:
		if rel.Owner == nil || rel.Slot == "" {
			return "", false
		}
		return s.partID(rel.Owner, rel.Slot)
	}
	return "", false
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
	case symbols.ExpressionChainFeature, symbols.ExpressionOperand:
		return s.partID(owner, sym.Expr.Slot)
	case symbols.ExpressionTransitionSuccession:
		// The conversion mints a transition's succession off the transition (transitionSuccession).
		base, ok := s.of(owner)
		if !ok {
			return "", false
		}
		return base + "_succession", true
	case symbols.ExpressionConnectorEnd:
		return s.partID(owner, fmt.Sprintf("end%d", sym.Expr.Index))
	case symbols.ExpressionBound:
		// A range's bound is minted off the feature declaring the range (multiplicityBound).
		if owner.OwnerScope == nil || owner.OwnerScope.Owner() == nil {
			return "", false
		}
		return s.partID(owner.OwnerScope.Owner(), sym.Expr.Slot)
	case symbols.ExpressionTransitionParameter:
		// Minted off the transition as an element (emptyParameterMember).
		base, ok := s.of(owner)
		if !ok {
			return "", false
		}
		return base + "_" + sym.Expr.Slot, true
	case symbols.ExpressionTriggerAction:
		// The trigger's accept is minted as the transition's `@trigger` member (encodeTrigger).
		if base, ok := s.of(owner); ok {
			return base + encodedSegment(naming.TriggerSegment), true
		}
	case symbols.ExpressionPayload:
		if id, ok := s.ids.OfDeclaration(doc, sym.Decl, ""); ok {
			return id, true
		}
		// A time or change trigger's payload is an implied unnamed parameter of the accept.
		if base, ok := s.of(owner); ok {
			return base + encodedSegment(fmt.Sprintf("@%d", sym.Expr.Index)), true
		}
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

// isConnectorEnd reports whether sym is a connector's end, declared or synthesized.
func isConnectorEnd(sym *symbols.Symbol) bool {
	return sym.Kind == symbols.SymbolConnectorEnd || sym.Expr != nil && sym.Expr.Role == symbols.ExpressionConnectorEnd
}
