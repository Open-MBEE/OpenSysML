package ast

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Visibility mirrors SysML VisibilityKind.
type Visibility int

const (
	VisibilityDefault Visibility = iota // no explicit indicator
	VisibilityPublic
	VisibilityPrivate
	VisibilityProtected
)

// NameSegment is one identifier in a qualified name, with its source span.
// It carries no semantic information: the symbol a segment resolves to lives in
// the resolver's side table, reachable through Resolver.PartSymbol.
type NameSegment struct {
	Text string
	Span source.Span
	// Chained records that the segment was written after '.' rather than after
	// '::': a feature chain step (`S2.S3`) rather than a namespace member.
	Chained bool
}

// QualifiedName is an unresolved dotted/`::`-separated name reference.
// Global is true when the name began with `$::`.
type QualifiedName struct {
	NodeBase
	Global bool
	Parts  []NameSegment
	// part0 backs Parts for the common single-segment name, so parsing one
	// costs no slice allocation of its own (see SetSingleton).
	part0 [1]NameSegment
}

// SetSingleton makes seg the name's only part, backed by the node's own
// storage. The slice is capped so appending a later segment copies out.
func (q *QualifiedName) SetSingleton(seg NameSegment) {
	q.part0[0] = seg
	q.Parts = q.part0[:1:1]
}

// QualifiedNameOf is the name a reference written as segments joined by "::"
// parses to, without positions.
func QualifiedNameOf(segments ...string) *QualifiedName {
	qn := &QualifiedName{}
	for _, segment := range segments {
		qn.Parts = append(qn.Parts, NameSegment{Text: segment})
	}
	return qn
}

// Text renders the name as written, "A::B::C", and "" for a nil name.
func (q *QualifiedName) Text() string {
	if q == nil {
		return ""
	}
	parts := make([]string, len(q.Parts))
	for i, part := range q.Parts {
		parts[i] = part.Text
	}
	return strings.Join(parts, "::")
}

// AsQualifiedName unwraps the two forms a name reference parses to: a bare
// QualifiedName, or one wrapped in a FeatureReference. It returns nil for
// anything else.
func AsQualifiedName(node Node) *QualifiedName {
	switch n := node.(type) {
	case *QualifiedName:
		return n
	case *FeatureReference:
		return n.Name
	}
	return nil
}

// SimpleName returns a reference's last segment — the name the resolver and the
// runtime match on — or "" when node names nothing.
func SimpleName(node Node) string {
	qname := AsQualifiedName(node)
	if qname == nil || len(qname.Parts) == 0 {
		return ""
	}
	return qname.Parts[len(qname.Parts)-1].Text
}

// QualifiedText renders a reference's whole name as written ("A::B::C"), or ""
// when node names nothing.
func QualifiedText(node Node) string {
	qname := AsQualifiedName(node)
	if qname == nil || len(qname.Parts) == 0 {
		return ""
	}
	out := qname.Parts[0].Text
	if qname.Global {
		out = "$::" + out
	}
	for _, part := range qname.Parts[1:] {
		out += "::" + part.Text
	}
	return out
}

// TargetName returns the last segment of a relationship target — a qualified
// name, a feature reference, or a feature chain (`providePower.generateTorque`)
// — together with its span, or "" when node names nothing.
func TargetName(node Node) (string, source.Span) {
	if chain, ok := node.(*FeatureChainExpr); ok {
		if chain.Member == nil {
			return "", source.Span{}
		}
		node = chain.Member
	}
	qname := AsQualifiedName(node)
	if qname == nil || len(qname.Parts) == 0 {
		return "", source.Span{}
	}
	last := qname.Parts[len(qname.Parts)-1]
	return last.Text, last.Span
}

// NamingFeature returns the relationship that names a usage lacking a declared
// name (KerML 7.3.4.5): its first redefinition, or for the SysML members named
// by what they reference (Usage.NamedByReference) that reference. A declared
// short name is a declaration too: the feature then derives no name. A
// requirement's assume/require/verify member is named by its reference alone
// (SysML ConstraintUsage::namingFeature): a redefinition leaves it anonymous.
func NamingFeature(u *Usage) *Relationship {
	if u == nil || u.Ident.Declared() {
		return nil
	}
	if u.NamedByReference() {
		if ref := u.ReferenceSubsetting(); ref != nil {
			if name, _ := TargetName(ref.Target); name != "" {
				return ref
			}
		}
		if u.IsRequirementConstraint() || u.IsVerifiedRequirement() {
			return nil
		}
	}
	return firstRedefinition(u.Relationships)
}

// firstRedefinition returns the first redefinition among rels, the naming
// feature of an unnamed feature (KerML Feature::namingFeature). A redefined
// feature chain is a nameless feature of its own, so it names nothing.
func firstRedefinition(rels []*Relationship) *Relationship {
	for _, rel := range rels {
		if rel != nil && rel.Kind == RelRedefines {
			if IsFeatureChain(rel.Target) {
				return nil
			}
			return rel
		}
	}
	return nil
}

// IsFeatureChain reports whether a relationship target is written as a feature
// chain (`p.q`) rather than a plain or qualified name.
func IsFeatureChain(node Node) bool {
	switch n := node.(type) {
	case *FeatureChainExpr:
		return true
	case *FeatureReference:
		return IsFeatureChain(n.Name)
	case *QualifiedName:
		for _, part := range n.Parts {
			if part.Chained {
				return true
			}
		}
	}
	return false
}

// namingReference returns the named reference subsetting among rels, the
// naming feature of a member named by what it references.
func namingReference(rels []*Relationship) *Relationship {
	for _, rel := range rels {
		if rel == nil || !rel.Kind.ReferenceSubsets() {
			continue
		}
		if name, _ := TargetName(rel.Target); name != "" {
			return rel
		}
	}
	return nil
}

// EffectiveName returns the name a usage answers to: its declared name (a short
// name alone included), else the name its naming feature supplies.
func EffectiveName(u *Usage) (string, source.Span) {
	if u == nil {
		return "", source.Span{}
	}
	if u.Ident.Declared() {
		return u.Ident.DeclaredName()
	}
	if rel := NamingFeature(u); rel != nil {
		return TargetName(rel.Target)
	}
	return "", source.Span{}
}

// Identification captures `<shortName> name` or `name` on a declaration.
type Identification struct {
	ShortName     string
	ShortNameSpan source.Span
	Name          string
	NameSpan      source.Span
}

// Declared reports whether either name is declared, which is when a feature
// takes no name from its naming feature (KerML 7.3.4.5).
func (id Identification) Declared() bool {
	return id.Name != "" || id.ShortName != ""
}

// DeclaredName returns the name a declaration is known by: its name, else its
// short name, as the symbol table registers it.
func (id Identification) DeclaredName() (string, source.Span) {
	if id.Name != "" {
		return id.Name, id.NameSpan
	}
	return id.ShortName, id.ShortNameSpan
}

// Membership wraps a namespace member with a visibility prefix. Member is
// the owned element (a Package/Namespace/Dependency/Comment/... or ErrorNode).
// A `then` prefixing a member is not recorded here: it sequences the members
// either side of it rather than describing one of them, so the parser desugars
// it to a SuccessionEdge of its own (see internal/syntax/parser/succession.go).
type Membership struct {
	NodeBase
	Visibility    Visibility
	IsTypeFeature bool
	Member        Node
}

// RootNamespace is the top of every parsed file: a flat list of members.
type RootNamespace struct {
	NodeBase
	Members []Node // *Membership | *Import | *Alias | *ErrorNode
	// UndefinedOperators are the `~` operator expressions of the document in
	// source order: KerML leaves DataFunctions::'~' undefined, so a tool warns at each.
	UndefinedOperators []*OperatorExpr
}

// PrefixMetadata records a `# QualifiedName` metadata annotation reference.
type PrefixMetadata struct {
	NodeBase
	// Ident names the usage when it declares one before its typing:
	// `@ m : Meta;`, `@ : Meta;` (SysML.xtext MetadataUsageDeclaration).
	Ident Identification
	Type  *QualifiedName
	Body  []Node // optional body with property initializers: @Meta{prop = value;}
	// HasBody records that the usage was written with braces, which an empty
	// body does not otherwise show.
	HasBody bool
	// Elements the usage annotates: `@Meta about a, b;` (SysML.xtext:145-147).
	About []*QualifiedName
}

// Namespace is `namespace <id> { ... }`.
type Namespace struct {
	NodeBase
	Prefixes []*PrefixMetadata
	Ident    Identification
	Members  []Node
	HasBody  bool // false when body was `;`
}

// Package is `package <id> { ... }`. Library/Standard flags cover
// `library package` and `standard library package`.
type Package struct {
	NodeBase
	Prefixes   []*PrefixMetadata
	Ident      Identification
	IsLibrary  bool
	IsStandard bool
	Members    []Node
	HasBody    bool
}

// ImportKind distinguishes membership vs namespace imports.
type ImportKind int

const (
	ImportMembership ImportKind = iota // import A::B ;
	ImportNamespace                    // import A::B::* ;
)

// Import is `[visibility] import [all] QualifiedName[::*][::**] ;|{}`.
//
// An `expose` declaration in a view body is also an Import: SysML v2 8.3.26.2
// makes Expose a specialization of Import (MembershipExpose specializes
// MembershipImport, NamespaceExpose specializes NamespaceImport), so it is
// represented by this node with IsExpose set.
type Import struct {
	NodeBase
	Visibility  Visibility
	IsAll       bool
	Kind        ImportKind
	Imported    *QualifiedName
	IsRecursive bool // `::**`
	FilterExpr  Node // Optional filter expression [<expr>]
	Body        []Node
	HasBody     bool
	// IsExpose marks an `expose` declaration. Per SysML v2 8.3.26.2 an Expose
	// always imports all elements regardless of visibility (isImportAll = true)
	// and always has protected visibility, so IsAll and Visibility are fixed
	// accordingly by the parser.
	IsExpose bool
}

// Alias is `alias <shortName> name for QualifiedName ;|{}`.
type Alias struct {
	NodeBase
	Visibility Visibility
	Ident      Identification
	For        *QualifiedName
	Body       []Node
	HasBody    bool
}

// MultiplicityDecl is `multiplicity <id> [range] ;|{}` (a MultiplicityRange) or
// `multiplicity <id> subsets f ;|{}` (a MultiplicitySubset, KerML.xtext:754).
// Declares a named multiplicity like exactlyOne [1..1].
type MultiplicityDecl struct {
	NodeBase
	Ident Identification
	Range *Multiplicity // range bounds, in the MultiplicityRange form
	// Subsets is the subsetted multiplicity of a MultiplicitySubset; the two
	// forms are exclusive.
	Subsets *QualifiedName
	Members []Node // optional - body members (typically doc comments)
	HasBody bool   // true if has {}, false if just ;
}

// Dependency is `dependency [<id> from] clients to suppliers ;|{}`.
type Dependency struct {
	NodeBase
	Prefixes  []*PrefixMetadata
	Ident     Identification
	Clients   []*QualifiedName
	Suppliers []*QualifiedName
	Body      []Node
	HasBody   bool
}

// RelationshipMember is a KerML relationship written keyword-first as a member
// of its own: `specialization Gen subtype A specializes B;` (KerML.xtext:390).
// Its two ends are ordered — Source relates to Target, never the reverse.
type RelationshipMember struct {
	NodeBase
	Ident Identification
	Kind  RelationshipKind
	// Keyword is the relationship keyword as written (`subtype`, `subclassifier`),
	// and PrefixKeyword the one that may precede it (`specialization`).
	Keyword       string
	PrefixKeyword string
	Source        Node // the specific/subsetting/featured end
	Target        Node // the general/subsetted/featuring end
	// Conjugated marks a Conjugation, whose target is the conjugate of Source.
	Conjugated bool
	Visibility Visibility
	Members    []Node
	HasBody    bool
}

// Comment is `[comment <id> [about refs]] [locale s] /* ... */`.
type Comment struct {
	NodeBase
	Ident    Identification
	About    []*QualifiedName
	Locale   string
	BodySpan source.Span // the REGULAR_COMMENT token span
}

// Documentation is `doc <id> [locale s] /* ... */`.
type Documentation struct {
	NodeBase
	Ident    Identification
	Locale   string
	BodySpan source.Span
}

// TextualRepresentation is `[rep <id>] language s /* ... */`.
type TextualRepresentation struct {
	NodeBase
	Ident    Identification
	Language string
	BodySpan source.Span
}

// FilterMember is an `filter <expression> ;` element filter.
type FilterMember struct {
	NodeBase
	Condition Node
}
