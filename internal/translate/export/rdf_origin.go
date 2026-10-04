package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

type relationshipOriginKey struct {
	subject  string
	property string
	target   rdf.Term
}

func (e *encoder) recordOrigin(subject rdf.Term, element metamodel.Element) {
	if subject.IsIRI() && element.Key() != (metamodel.ElementKey{}) {
		e.origins[subject.Value] = element
	}
}

func (e *encoder) originForNode(node ast.Node) metamodel.Element {
	if node == nil {
		return metamodel.Element{}
	}
	if sym := e.ids.declSym[node]; sym != nil {
		return metamodel.ElementOf(sym)
	}
	if owner := e.annotationOwners[node]; owner != nil {
		return metamodel.Element{Node: node, Container: owner, Aspect: "annotation"}
	}
	if imp, ok := node.(*ast.Import); ok {
		return metamodel.Element{Node: imp, Container: e.importOwners[imp]}
	}
	return metamodel.NodeOf(node)
}

func (e *encoder) recordDerivedOrigin(subject, parent rdf.Term, aspect string) {
	if !subject.IsIRI() {
		return
	}
	parentOrigin, ok := e.origins[parent.Value]
	if !ok {
		return
	}
	container := parentOrigin.Symbol
	if container == nil {
		container = parentOrigin.Container
	}
	node := parentOrigin.Node
	if node == nil && parentOrigin.Symbol != nil {
		node = parentOrigin.Symbol.Decl
	}
	if parentOrigin.Aspect != "" {
		aspect = parentOrigin.Aspect + "/" + aspect
	}
	e.recordOrigin(subject, metamodel.Element{
		Node: node, Container: container, Aspect: aspect,
	})
}

func (e *encoder) minted(iri, parent rdf.Term, suffix string) rdf.Term {
	subject := e.ids.minted(iri, parent, suffix)
	e.recordDerivedOrigin(subject, parent, suffix)
	return subject
}

func (e *encoder) mintedNode(iri, parent rdf.Term, slot string) rdf.Term {
	subject := e.ids.mintedNode(iri, parent, slot)
	e.recordDerivedOrigin(subject, parent, slot)
	return subject
}

func (e *encoder) recordMembershipOrigin(membership, member, owner rdf.Term, metaclass string) {
	if !membership.IsIRI() {
		return
	}
	memberOrigin, known := e.origins[member.Value]
	if known && memberOrigin.Symbol != nil {
		semanticMembership := metamodel.ElementOf(memberOrigin.Symbol).Membership
		if hasSemanticMembership(semanticMembership) {
			e.recordOrigin(membership, metamodel.MembershipElement(semanticMembership))
			return
		}
	}
	var node ast.Node
	var container *symbols.Symbol
	if known {
		node = memberOrigin.Node
		container = memberOrigin.Symbol
		if container == nil {
			container = memberOrigin.Container
		}
	}
	if node == nil && memberOrigin.Symbol != nil {
		node = memberOrigin.Symbol.Decl
	}
	ownerSymbol := originSymbol(e.origins[owner.Value])
	if ownerSymbol == nil {
		ownerSymbol = container
	}
	e.recordOrigin(membership, metamodel.MembershipElement(metamodel.Membership{
		Node: node, Owner: ownerSymbol,
		Feature: metaclass == mFeatureMembership,
		Aspect:  "synthetic:" + rdf.LocalName(membership.Value),
	}))
}

func originSymbol(element metamodel.Element) *symbols.Symbol {
	if element.Symbol != nil {
		return element.Symbol
	}
	return element.Container
}

func (e *encoder) recordRelationshipOrigin(subject rdf.Term, property string, target rdf.Term, relationship *ast.Relationship) {
	if subject.Value == "" || relationship == nil {
		return
	}
	parent, ok := e.origins[subject.Value]
	if !ok {
		return
	}
	element := metamodel.Element{
		Node: relationship, Container: originSymbol(parent),
	}
	key := relationshipOriginKey{subject: subject.Value, property: property, target: target}
	e.relationshipOrigins[key] = append(e.relationshipOrigins[key], element)
}

func (e *encoder) recordSemanticRelationshipOrigin(subject rdf.Term, property string, target rdf.Term, element metamodel.Element) {
	if subject.Value == "" || element.Key() == (metamodel.ElementKey{}) {
		return
	}
	key := relationshipOriginKey{subject: subject.Value, property: property, target: target}
	e.relationshipOrigins[key] = append(e.relationshipOrigins[key], element)
}

func (e *encoder) takeRelationshipOrigin(subject rdf.Term, property string, target rdf.Term) (metamodel.Element, bool) {
	key := relationshipOriginKey{subject: subject.Value, property: property, target: target}
	origins := e.relationshipOrigins[key]
	if len(origins) == 0 {
		return metamodel.Element{}, false
	}
	origin := origins[0]
	if len(origins) == 1 {
		delete(e.relationshipOrigins, key)
	} else {
		e.relationshipOrigins[key] = origins[1:]
	}
	return origin, true
}
