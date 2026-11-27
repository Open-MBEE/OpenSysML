package queryexec

import (
	"fmt"

	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ElementIDDocument is a document of the model a conversion writes, with its
// parse's errors, which refuse a model of several documents.
type ElementIDDocument struct {
	File       *source.SourceFile
	Root       *ast.RootNamespace
	ParseDiags []parser.Diagnostic
}

// ElementIDReader reads the elementIds a conversion of the model writes: Read, for the
// element declared at decl in document doc, named fqn; ReadOwningMembership, for the
// membership through which its namespace owns it; ReadRelationship, for the relationship
// element the notation implies at a node (a head clause, a bound end's attachment, a
// dependency's prefix metadata). Each reports why the model has none.
type ElementIDReader interface {
	Read(doc string, decl ast.Node, fqn string) (string, error)
	ReadOwningMembership(doc string, decl ast.Node, fqn string) (string, error)
	ReadRelationship(doc string, node ast.Node) (string, error)
	ReadExpressionPart(doc string, node ast.Node, slot string) (element, membership string, err error)
	// ReadAddress is the address the conversion stated for the element written with id: its
	// qualifiedName, or a positional address (`P::@2::x`); none for a minted or unknown id.
	ReadAddress(id string) (string, bool)
}

// defaultMultiplicitySlot is the slot the conversion mints a usage's default
// Multiplicity under: the property it states, as the declared range is.
const defaultMultiplicitySlot = "multiplicity"

// loopVariableSlot is the slot the conversion mints a `for` loop's variable under.
const loopVariableSlot = "loopVariable"

// loopBodySlot is the slot the conversion files a loop's body at.
const loopBodySlot = "loopBody"

// defaultMultiplicityOwner is the node of the usage whose implicit [1..1] sym
// states (semantics.DefaultMultiplicityMember), by its span when the usage is a record.
func defaultMultiplicityOwner(sym *symbols.Symbol) (ast.Node, bool) {
	decl, ok := symbols.ImpliedDeclOf(sym)
	if !ok || (decl.Form != symbols.ImpliedDefaultMultiplicity && decl.Form != symbols.ImpliedMultiplicityRange) {
		return nil, false
	}
	if decl.Of != nil {
		return decl.Of, true
	}
	return spanNode{sym.DeclSpan}, true
}

// successionEndSlot is the `then` succession edge, or the flow, and the slot (end0, end1) the conversion
// mints the end feature sym under (encodeSuccessionEdge, connectorEnd).
func successionEndSlot(sym *symbols.Symbol) (ast.Node, string, bool) {
	i, ok := symbols.ImpliedConnectorEndIndex(sym)
	if !ok {
		return nil, "", false
	}
	decl, _ := symbols.ImpliedDeclOf(sym)
	if symbols.ImpliedTransitionSuccessionEnd(sym) {
		// A transition's succession mints its ends under itself, filed under the transition (recordTransitionSuccession).
		return decl.Of, fmt.Sprintf("succession.end%d", i), true
	}
	return decl.Of, fmt.Sprintf("end%d", i), true
}

// flowEndFeatureSlot is the flow and the slot (end0.ff, end1.ff) the conversion mints the ReferenceUsage
// a flow's end owns under (flowEndReferences).
func flowEndFeatureSlot(sym *symbols.Symbol) (ast.Node, string, bool) {
	i, ok := symbols.ImpliedFlowEndFeatureIndex(sym)
	if !ok {
		return nil, "", false
	}
	decl, _ := symbols.ImpliedDeclOf(sym)
	return decl.Of, fmt.Sprintf("end%d.ff", i), true
}

// expressionPartSlot is the slot the conversion names a synthesized
// expression parameter by: `in%d` for an operand's or type argument's
// parameter, `out` for the result.
func expressionPartSlot(sym *symbols.Symbol) (string, bool) {
	if sym == nil || sym.Expr == nil || sym.Expr.Owner == nil {
		return "", false
	}
	// A change wrapper's parts are minted off the wrapper (`a0`), itself
	// minted off the trigger whose node every part shares.
	prefix := ""
	if owner := sym.Expr.Owner; owner.Expr != nil && owner.Expr.Role == symbols.ExpressionChangeWrapper {
		prefix = "a0."
	}
	switch sym.Expr.Role {
	case symbols.ExpressionParameter:
		return prefix + fmt.Sprintf("in%d", sym.Expr.Index), true
	case symbols.ExpressionResult:
		return prefix + "out", true
	case symbols.ExpressionChangeWrapper:
		return "a0", true
	case symbols.ExpressionBody:
		return "body", true
	case symbols.ExpressionBodyResult:
		if sym.Expr.Owner.Expr == nil {
			// A constraint body's closing condition is minted as the member stating it (constraintResultMember).
			return "", false
		}
		return "result", true
	case symbols.ExpressionChainFeature:
		return sym.Expr.Slot, true
	case symbols.ExpressionTransitionSuccession:
		return "succession", true
	case symbols.ExpressionTransitionLinkParameter:
		return []string{"linkparam", "triggerparam"}[sym.Expr.Index], true
	case symbols.ExpressionTransitionLink:
		return "transitionlink", true
	case symbols.ExpressionTriggerAction:
		// A trigger's accept shares its payload's node, so it is filed under the transition's (recordTransitionHead).
		return "trigger", true
	}
	return "", false
}

// rangeBoundSlot is the node and slot a multiplicity range's bound is minted under: the feature
// declaring the range, or a multiplicity declaration itself (multiplicityBound).
func rangeBoundSlot(sym *symbols.Symbol) (ast.Node, string, bool) {
	if sym == nil || sym.Expr == nil || sym.Expr.Role != symbols.ExpressionBound || sym.Expr.Owner == nil {
		return nil, "", false
	}
	owner := sym.Expr.Owner
	if _, declared := owner.Decl.(*ast.MultiplicityDecl); declared {
		return owner.Decl, sym.Expr.Slot, true
	}
	if owner.OwnerScope == nil || owner.OwnerScope.Owner() == nil {
		return nil, "", false
	}
	return owner.OwnerScope.Owner().Decl, sym.Expr.Slot, true
}

// nodeParameterSlot is the statement whose parameter or loop body sym is and the slot the
// conversion files it at: a parameter's own, under those of the parameters nesting it.
func nodeParameterSlot(sym *symbols.Symbol) (ast.Node, string, bool) {
	if sym == nil || sym.Expr == nil {
		return nil, "", false
	}
	slot, owner := loopBodySlot, sym.Expr.Owner
	switch sym.Expr.Role {
	case symbols.ExpressionLoopBody:
	default:
		stated, ok := semantics.NodeParameterSlot(sym)
		if !ok {
			return nil, "", false
		}
		slot = stated
		for ; owner != nil; owner = owner.Expr.Owner {
			nesting, nested := semantics.NodeParameterSlot(owner)
			if !nested {
				break
			}
			slot = nesting + "." + slot
		}
	}
	if owner == nil || owner.Decl == nil {
		return nil, "", false
	}
	if usage, named := owner.Decl.(*ast.Usage); named {
		// `action a for …` is the loop: its parts are filed under the statement.
		if statement := semantics.NodeStatementOf(usage); statement != nil {
			return statement, slot, true
		}
	}
	return owner.Decl, slot, true
}

// sendArgumentSlot is the send whose argument parameter sym is and the slot the conversion
// files it at (`arg%d`, the argument's position: payload, sender, receiver).
func sendArgumentSlot(sym *symbols.Symbol) (ast.Node, string, bool) {
	if sym == nil || sym.Expr == nil || sym.Expr.Role != symbols.ExpressionPayload || sym.Expr.Owner == nil || sym.Expr.Owner.Expr != nil {
		return nil, "", false
	}
	send := symbols.SendStatementOf(sym.Expr.Owner.Decl)
	if send == nil {
		return nil, "", false
	}
	return send, fmt.Sprintf("arg%d", sym.Expr.Index), true
}

// bodyParameterSlot is the slot the conversion mints a body's parameter at
// (`in%d`, its position among the body's declarations), keyed by the
// parameter's own span: every parameter of a body shares the body as its node.
func bodyParameterSlot(sym *symbols.Symbol) (span source.Span, slot string, ok bool) {
	if sym == nil || sym.Expr != nil || sym.OwnerScope == nil || !sym.OwnerScope.BodyLocal() {
		return source.Span{}, "", false
	}
	body, isBody := sym.Decl.(*ast.BodyExpr)
	if !isBody || sym.OwnerScope.Node() != body {
		return source.Span{}, "", false
	}
	for _, decl := range ast.BodyDeclarations(body.Params, body.Members) {
		if decl.Param != nil && decl.Param.Span == sym.DeclSpan {
			return decl.Param.Span, decl.Slot(), true
		}
	}
	return source.Span{}, "", false
}

// ElementIDOf is sym's elementId through read. A relationship the notation writes as
// part of another element reads as the conversion writes it: an implied OwningMembership
// as its member's owning membership, an import as its declaration, a written clause
// or an end's attachment as the element materialized for it, a prefix metadata's
// Annotation as the metadata's; one the conversion never writes reads empty.
func ElementIDOf(read ElementIDReader, idx *symbols.Index, sym *symbols.Symbol) (string, error) {
	if rel := sym.Implicit; rel != nil {
		if id, ok, err := implicitRelationshipID(read, idx, sym, rel); ok {
			return id, err
		}
	}
	if slot, ok := expressionPartSlot(sym); ok {
		element, _, err := read.ReadExpressionPart(sym.DocName, sym.Expr.Owner.Decl, slot)
		return element, err
	}
	if send, slot, ok := sendArgumentSlot(sym); ok {
		element, _, err := read.ReadExpressionPart(sym.DocName, send, slot)
		return element, err
	}
	if declaring, slot, ok := rangeBoundSlot(sym); ok {
		element, _, err := read.ReadExpressionPart(sym.DocName, declaring, slot)
		return element, err
	}
	if owner, ok := defaultMultiplicityOwner(sym); ok {
		// The Multiplicity stating a usage's implicit [1..1] is minted under the usage (defaultMultiplicity).
		element, _, err := read.ReadExpressionPart(sym.DocName, owner, defaultMultiplicitySlot)
		return element, err
	}
	if edge, slot, ok := successionEndSlot(sym); ok {
		// A `then` succession's end is minted under the edge at end0/end1 (encodeSuccessionEdge).
		element, _, err := read.ReadExpressionPart(sym.DocName, edge, slot)
		return element, err
	}
	if flow, slot, ok := flowEndFeatureSlot(sym); ok {
		element, _, err := read.ReadExpressionPart(sym.DocName, flow, slot)
		return element, err
	}
	if end, ok := connectorEndChain(sym); ok {
		// The chain Feature a connector end's ReferenceSubsetting owns is minted under the end.
		element, _, err := read.ReadExpressionPart(sym.DocName, end, "chain")
		return element, err
	}
	if prefix, ok := flowEndChain(sym); ok {
		// The chain Feature a flow's end references ahead of its payload is minted under the end (flowEndReferences).
		element, _, err := read.ReadExpressionPart(sym.DocName, prefix, "chain")
		return element, err
	}
	if target, ok := headClauseChain(sym); ok {
		// The chain Feature a head clause's chain target denotes is minted under the head (headChain).
		element, _, err := read.ReadExpressionPart(sym.DocName, target, "chain")
		return element, err
	}
	if statement, slot, ok := nodeParameterSlot(sym); ok {
		element, _, err := read.ReadExpressionPart(sym.DocName, statement, slot)
		return element, err
	}
	if semantics.IsLoopVariable(sym) {
		// A `for` variable is minted under its loop, whose node its symbol shares.
		element, _, err := read.ReadExpressionPart(sym.DocName, sym.Decl, loopVariableSlot)
		return element, err
	}
	if span, slot, ok := bodyParameterSlot(sym); ok {
		element, _, err := read.ReadExpressionPart(sym.DocName, spanNode{span}, slot)
		return element, err
	}
	if member := constraintResultMember(sym); member != nil {
		// A constraint body's closing bare condition is written as that member (encodeMember).
		return read.Read(sym.DocName, member, "")
	}
	return read.Read(sym.DocName, sym.Decl, query.QualifiedIdentity(idx, sym))
}

// constraintResultMember is the ConstraintMember stating the closing bare condition
// sym reflects as a constraint's result expression (semantics.ConstraintResultMember), or nil.
func constraintResultMember(sym *symbols.Symbol) ast.Node {
	if sym == nil || sym.Expr == nil || sym.Expr.Role != symbols.ExpressionBodyResult || sym.Expr.Owner == nil || sym.Expr.Owner.Expr != nil {
		return nil
	}
	if c := semantics.ConstraintResultCondition(sym.Expr.Owner); c != nil && c.Expression == sym.Decl {
		return c
	}
	return nil
}

// implicitRelationshipID reads the elementId of the relationship sym records implicitly,
// as the conversion writes it; ok is false when the id is sym's own element's.
func implicitRelationshipID(read ElementIDReader, idx *symbols.Index, sym *symbols.Symbol, rel *symbols.ImplicitRelationship) (id string, ok bool, err error) {
	switch rel.Form {
	case symbols.ImplicitMembership:
		return implicitMembershipID(read, idx, rel.Member)
	case symbols.ImplicitImport:
		if rel.Decl != nil {
			id, err = read.Read(sym.DocName, rel.Decl, "")
			return id, true, err
		}
	case symbols.ImplicitFeatureChaining:
		id, err = featureChainingID(read, rel)
		return id, true, err
	case symbols.ImplicitReferenceMembership:
		if rel.Decl != nil {
			// A `first x;` marker is the Membership itself, identified as the conversion mints it.
			id, err = read.Read(sym.DocName, rel.Decl, "")
			return id, true, err
		}
		if rel.Owner != nil && rel.Owner.Decl != nil {
			_, membership, err := read.ReadExpressionPart(rel.Owner.DocName, rel.Owner.Decl, rel.Slot)
			return membership, true, err
		}
		return "", true, nil
	case symbols.ImplicitWritten:
		if node := writtenRelationshipNode(rel); node != nil {
			id, err = read.ReadRelationship(sym.DocName, node)
			return id, true, err
		}
		return "", true, nil
	case symbols.ImplicitOwnedAnnotation:
		if rel.Member != nil && rel.Member.Decl != nil {
			id, err = read.ReadRelationship(rel.Member.DocName, rel.Member.Decl)
			return id, true, err
		}
		return "", true, nil
	case symbols.ImplicitAnnotatingRelationship:
		if node := aboutClauseNode(rel.Owner, rel.Ordinal); node != nil {
			id, err = read.ReadRelationship(rel.Owner.DocName, node)
			return id, true, err
		}
		return "", true, nil
	case symbols.ImplicitSuccessionEndReference:
		// The target end's ReferenceSubsetting is minted under the end as its `rs` (referenceSubsetting).
		if edge, slot, ok := successionEndSlot(rel.Owner); ok {
			element, _, err := read.ReadExpressionPart(rel.Owner.DocName, edge, slot+".rs")
			return element, true, err
		}
		return "", true, nil
	case symbols.ImplicitPortConjugation, symbols.ImplicitMetadataBodyRedefinition, symbols.ImplicitTypeArgumentTyping,
		symbols.ImplicitShorthandReference:
		if slot, ok := expressionPartSlot(rel.Owner); ok && rel.Form == symbols.ImplicitTypeArgumentTyping {
			// The FeatureTyping of a type argument's parameter is minted under the parameter (`ft0`).
			element, _, err := read.ReadExpressionPart(rel.Owner.DocName, rel.Owner.Expr.Owner.Decl, slot+".ft0")
			return element, true, err
		}
		if flow, slot, ok := flowEndFeatureSlot(rel.Owner); ok && rel.Form != symbols.ImplicitShorthandReference {
			// The Redefinition the ReferenceUsage a flow's end owns states is minted under it (`rd0`).
			element, _, err := read.ReadExpressionPart(rel.Owner.DocName, flow, slot+".rd0")
			return element, true, err
		}
		// The conversion keys a relationship the declaration states without a clause by the declaration.
		if rel.Owner != nil && rel.Owner.Decl != nil {
			id, err = read.ReadRelationship(rel.Owner.DocName, rel.Owner.Decl)
			return id, true, err
		}
		return "", true, nil
	}
	return "", false, nil
}

// implicitMembershipID reads the elementId of the OwningMembership implied for member,
// as the conversion mints it; ok is false for no member.
func implicitMembershipID(read ElementIDReader, idx *symbols.Index, member *symbols.Symbol) (id string, ok bool, err error) {
	if slot, ok := expressionPartSlot(member); ok {
		_, membership, err := read.ReadExpressionPart(member.DocName, member.Expr.Owner.Decl, slot)
		return membership, true, err
	}
	if send, slot, ok := sendArgumentSlot(member); ok {
		_, membership, err := read.ReadExpressionPart(member.DocName, send, slot)
		return membership, true, err
	}
	if flow, slot, ok := flowEndFeatureSlot(member); ok {
		// The ReferenceUsage a flow's end owns is minted under the end with its FeatureMembership (`_pff_om`).
		_, membership, err := read.ReadExpressionPart(member.DocName, flow, slot)
		return membership, true, err
	}
	if statement, slot, ok := nodeParameterSlot(member); ok {
		_, membership, err := read.ReadExpressionPart(member.DocName, statement, slot)
		return membership, true, err
	}
	if semantics.IsLoopVariable(member) {
		_, membership, err := read.ReadExpressionPart(member.DocName, member.Decl, loopVariableSlot)
		return membership, true, err
	}
	if span, slot, ok := bodyParameterSlot(member); ok {
		_, membership, err := read.ReadExpressionPart(member.DocName, spanNode{span}, slot)
		return membership, true, err
	}
	if declaring, slot, ok := rangeBoundSlot(member); ok {
		_, membership, err := read.ReadExpressionPart(member.DocName, declaring, slot)
		return membership, true, err
	}
	if owner, ok := defaultMultiplicityOwner(member); ok {
		_, membership, err := read.ReadExpressionPart(member.DocName, owner, defaultMultiplicitySlot)
		return membership, true, err
	}
	if edge, slot, ok := successionEndSlot(member); ok {
		_, membership, err := read.ReadExpressionPart(member.DocName, edge, slot)
		return membership, true, err
	}
	if node := constraintResultMember(member); node != nil {
		id, err = read.ReadOwningMembership(member.DocName, node, "")
		return id, true, err
	}
	if member != nil {
		id, err = read.ReadOwningMembership(member.DocName, member.Decl, query.QualifiedIdentity(idx, member))
		return id, true, err
	}
	return "", false, nil
}

// featureChainingID reads the elementId of the FeatureChaining rel records under the chain
// Feature that owns it; empty for a chain the conversion never writes.
func featureChainingID(read ElementIDReader, rel *symbols.ImplicitRelationship) (string, error) {
	slot := "chain." + rel.Slot
	if chain := rel.Owner; chain != nil && chain.Expr != nil && chain.Expr.Owner != nil {
		element, _, err := read.ReadExpressionPart(chain.DocName, chain.Expr.Owner.Decl, chain.Expr.Slot+"."+rel.Slot)
		return element, err
	}
	if end, ok := connectorEndChain(rel.Owner); ok {
		element, _, err := read.ReadExpressionPart(rel.Owner.DocName, end, slot)
		return element, err
	}
	if prefix, ok := flowEndChain(rel.Owner); ok {
		element, _, err := read.ReadExpressionPart(rel.Owner.DocName, prefix, slot)
		return element, err
	}
	if target, ok := headClauseChain(rel.Owner); ok {
		element, _, err := read.ReadExpressionPart(rel.Owner.DocName, target, slot)
		return element, err
	}
	return "", nil
}

// connectorEndChain is the connector end whose attachment chain sym denotes (`connect a.b to c`):
// the ReferenceSubsetting of the attached target, not a chained redefinition the end also owns.
func connectorEndChain(sym *symbols.Symbol) (*ast.ConnectorEnd, bool) {
	if sym.Chain == nil || sym.Chain.Relationship == nil || sym.Chain.Relationship.Implicit == nil {
		return nil, false
	}
	rel := sym.Chain.Relationship.Implicit
	if rel.Owner == nil || rel.Node == nil || !rel.Kind.ReferenceSubsets() {
		return nil, false
	}
	end, ok := rel.Owner.Decl.(*ast.ConnectorEnd)
	if !ok || rel.Node.Target != end.AttachedTarget() {
		return nil, false
	}
	return end, true
}

// flowEndChain is the chain as written that sym, the chain Feature a flow's end
// references ahead of its payload (`a.b` of `flow a.b.x to c.y`), denotes.
func flowEndChain(sym *symbols.Symbol) (ast.Node, bool) {
	if sym == nil || sym.Chain == nil || sym.Chain.Flow == nil || sym.Chain.Node == nil {
		return nil, false
	}
	return sym.Chain.Node, true
}

// headClauseChain is the chain target of the head clause (`part b :> a.x`) whose chain Feature
// sym is; false for a connector end's attachment, which is minted under the end.
func headClauseChain(sym *symbols.Symbol) (ast.Node, bool) {
	if sym.Chain == nil || sym.Chain.Relationship == nil || sym.Chain.Relationship.Implicit == nil {
		return nil, false
	}
	rel := sym.Chain.Relationship.Implicit
	if rel.Node == nil || rel.Node.Target == nil || rel.Owner == nil {
		return nil, false
	}
	if _, isEnd := rel.Owner.Decl.(*ast.ConnectorEnd); isEnd {
		return nil, false
	}
	return rel.Node.Target, true
}

// writtenRelationshipNode is the node the conversion writes rel's relationship element
// for: the end whose attachment it is (`connect a to b`), the type of a typing the head
// writes without a clause (`subject s : T`), the multiplicity a `multiplicity m subsets n`
// names, else the clause; nil for a recorded owner.
func writtenRelationshipNode(rel *symbols.ImplicitRelationship) ast.Node {
	if rel.Owner == nil {
		return nil
	}
	if rel.Node == nil {
		// A multiplicity's `subsets` is held on its declaration, not as a clause.
		if decl, ok := rel.Owner.Decl.(*ast.MultiplicityDecl); ok && rel.Kind == ast.RelSubsets && decl.Subsets != nil {
			return decl.Subsets
		}
		return nil
	}
	if end, ok := rel.Owner.Decl.(*ast.ConnectorEnd); ok && rel.Kind == ast.RelReferences &&
		rel.Node.Target == end.AttachedTarget() && !slices.Contains(end.Relationships, rel.Node) {
		return end
	}
	if rel.Node.Span().Len == 0 && rel.Node.Target != nil {
		return rel.Node.Target
	}
	return rel.Node
}

// ElementIDSource opens a reader over a model's documents, converting them
// once, on the first read: how a query's elementId column is filled. The
// converter registers one (queryexec/elementids), so a binary that links no
// translator reports elementId absent.
type ElementIDSource func(docs []ElementIDDocument) ElementIDReader

var elementIDSource ElementIDSource

// RegisterElementIDSource installs the source elementId is read from.
func RegisterElementIDSource(s ElementIDSource) {
	if elementIDSource != nil {
		panic("queryexec: element-id source registered twice")
	}
	elementIDSource = s
}

// OpenElementIDs opens the registered source over docs; nil when none is
// linked, and elementId is then absent.
func OpenElementIDs(docs []ElementIDDocument) ElementIDReader {
	if elementIDSource == nil {
		return nil
	}
	return elementIDSource(docs)
}

// spanNode stands for a body parameter, which has no node of its own, at the
// span the conversion keys its ids by.
type spanNode struct{ span source.Span }

func (n spanNode) Span() source.Span            { return n.span }
func (n spanNode) LeadingTrivia() []ast.Trivia  { return nil }
func (n spanNode) TrailingTrivia() []ast.Trivia { return nil }

// aboutClauseNode is the node the conversion keys owner's i-th `about` Annotation by.
func aboutClauseNode(owner *symbols.Symbol, i int) ast.Node {
	if owner == nil {
		return nil
	}
	return ast.AboutClauseNode(owner.Decl, i)
}
