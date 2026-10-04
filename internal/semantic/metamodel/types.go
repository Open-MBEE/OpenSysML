package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Membership identifies an element's membership without depending on an export format.
type Membership struct {
	Symbol  *symbols.Symbol
	Node    ast.Node
	Member  *symbols.Symbol
	Owner   *symbols.Symbol
	Feature bool
	Aspect  string
	kind    string
}

// Element is a semantic element and the membership that owns it, when it has one.
type Element struct {
	Symbol       *symbols.Symbol
	Node         ast.Node
	Container    *symbols.Symbol
	Membership   Membership
	IsMembership bool
	Aspect       string
}

// MembershipKey identifies a membership independently of its contextual handles.
type MembershipKey struct {
	symbol *symbols.Symbol
	member *symbols.Symbol
	node   ast.Node
	aspect string
	alias  bool
}

// ElementKey identifies an element independently of its container and embedded membership.
type ElementKey struct {
	symbol       *symbols.Symbol
	node         ast.Node
	aspect       string
	membership   MembershipKey
	isMembership bool
}

// Key returns the canonical identity of a membership.
func (m Membership) Key() MembershipKey {
	if m.Symbol != nil {
		if m.Symbol.Decl != nil {
			return MembershipKey{node: m.Symbol.Decl, alias: true}
		}
		return MembershipKey{symbol: m.Symbol, alias: true}
	}
	if m.Member != nil {
		if m.Member.Decl != nil {
			return MembershipKey{node: m.Member.Decl}
		}
		return MembershipKey{member: m.Member}
	}
	return MembershipKey{node: m.Node, aspect: m.Aspect}
}

// Key returns the canonical identity of an element.
func (e Element) Key() ElementKey {
	if e.IsMembership {
		return ElementKey{membership: e.Membership.Key(), isMembership: true}
	}
	if e.Node != nil {
		return ElementKey{node: e.Node, aspect: e.Aspect}
	}
	if e.Symbol != nil {
		return ElementKey{symbol: e.Symbol, aspect: e.Aspect}
	}
	return ElementKey{aspect: e.Aspect}
}

// ValueKind identifies the semantic value stored in Value.
type ValueKind uint8

const (
	InvalidValue ValueKind = iota
	NullValue
	ElementValue
	MembershipValue
	StringValue
	BooleanValue
	IntegerValue
	RealValue
	EnumValue
	SequenceValue
)

// Value is a semantic value returned by a property derivation.
type Value struct {
	Kind       ValueKind
	Element    Element
	Membership Membership
	String     string
	Boolean    bool
	Integer    int64
	Real       float64
	Enum       string
	Values     []Value
	Success    bool
}

// Rule describes the basis for an evaluator-served property.
type Rule struct {
	DefiningClass string
	Property      string
	Constraint    string
	Basis         string
}

// Omission explains why a derived property has no faithful evaluator rule.
type Omission struct {
	DefiningClass string
	Property      string
	Reason        string
}

// Structure reports the abstract-syntax facts the exporter states for an element.
type Structure interface {
	Metaclass(Element) (string, bool)
	Specializes(sub, super string) bool
	OwnedRelationships(Element) ([]Element, bool)
	OwningRelationship(Element) (Element, bool, bool)
	OwnedRelatedElements(Element) ([]Element, bool)
	OwningRelatedElement(Element) (Element, bool, bool)
	RelatedElements(rel Element, property string) ([]Element, bool)
	Attribute(el Element, property string) (Value, bool)
}

type elementIdentityStructure interface {
	ElementIdentity(Element) (string, bool)
}

// Options supplies graph-stated structure used by the evaluator.
type Options struct {
	Structure Structure
}

// ElementOf creates a handle for a declared symbol.
func ElementOf(sym *symbols.Symbol) Element {
	if sym == nil {
		return Element{}
	}
	var membership Membership
	owner := sym.Owner()
	if sym.Kind == symbols.SymbolAlias {
		membership = Membership{Symbol: sym, Node: sym.Decl, Member: sym, Owner: owner, kind: "Alias"}
		return Element{Symbol: sym, Node: sym.Decl, Membership: membership, IsMembership: true}
	} else if hasOwningMembership(sym, owner) {
		kind := membershipKind(sym)
		membership = Membership{
			Node: sym.Decl, Member: sym, Owner: owner,
			Feature: isFeatureMembership(sym, owner), kind: kind,
		}
	}
	return Element{Symbol: sym, Node: sym.Decl, Membership: membership}
}

// MembershipElement creates a handle for a membership that is itself an element.
func MembershipElement(membership Membership) Element {
	return Element{Membership: membership, IsMembership: true}
}

// NodeOf creates a handle for an AST-backed element without a symbol.
func NodeOf(node ast.Node) Element {
	if node == nil {
		return Element{}
	}
	return Element{Node: node}
}

func membershipKind(sym *symbols.Symbol) string {
	if sym == nil {
		return ""
	}
	if sym.Kind == symbols.SymbolAlias {
		return "Alias"
	}
	if _, ok := sym.Decl.(*ast.CrossFeatureMember); ok {
		return "OwningMembership"
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		switch {
		case usage.IsVariant:
			return "VariantMembership"
		case usage.IsResult:
			return "ReturnParameterMembership"
		case usage.IsBodyParameter:
			return "ParameterMembership"
		case usage.IsEnd:
			return "EndFeatureMembership"
		}
	}
	if sym.Kind == symbols.SymbolConnectorEnd {
		return "EndFeatureMembership"
	}
	owner := sym.Owner()
	if sym.IsFeature() && isType(owner) {
		if ownerUsage, ok := owner.Decl.(*ast.Usage); ok && ownerUsage.IsVariant {
			return "VariantMembership"
		}
		return "FeatureMembership"
	}
	return "OwningMembership"
}

func isFeatureMembership(sym, owner *symbols.Symbol) bool {
	if sym == nil || !sym.IsFeature() || !isType(owner) {
		return false
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.IsVariant {
		return false
	}
	return true
}

func hasOwningMembership(sym, owner *symbols.Symbol) bool {
	if sym == nil || owner == nil || sym.Kind == symbols.SymbolAlias {
		return false
	}
	if sym.Kind == symbols.SymbolRelationship {
		return false
	}
	if _, ok := sym.Decl.(*ast.RelationshipMember); ok {
		return false
	}
	if owner.Kind == symbols.SymbolRelationship {
		return false
	}
	if _, ok := sym.Decl.(*ast.Usage); ok && owner.Kind == symbols.SymbolDependency {
		return false
	}
	return true
}

func elementValue(el Element) Value {
	return Value{Kind: ElementValue, Element: el, Success: true}
}

func membershipValue(m Membership) Value {
	return Value{Kind: MembershipValue, Membership: m, Success: true}
}

func sequence(values ...Value) Value {
	return Value{Kind: SequenceValue, Values: values, Success: true}
}

func nullValue() Value {
	return Value{Kind: NullValue, Success: true}
}

func stringValue(value string) Value {
	return Value{Kind: StringValue, String: value, Success: true}
}

func booleanValue(value bool) Value {
	return Value{Kind: BooleanValue, Boolean: value, Success: true}
}

func integerValue(value int64) Value {
	return Value{Kind: IntegerValue, Integer: value, Success: true}
}

func realValue(value float64) Value {
	return Value{Kind: RealValue, Real: value, Success: true}
}

func enumValue(value string) Value {
	return Value{Kind: EnumValue, Enum: value, Success: true}
}
