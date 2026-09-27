package symbols

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// LibraryFacts is the derived analysis of one library symbol: the semantic work
// whose derivation dominates a cold library load, held so that a later load can
// install it instead of repeating it.
//
// Every field is a memo of what the symbol's declaration yields, never a
// replacement for it — a library document is parsed and indexed on every load
// path, so a field left unset simply means "derive it from the declaration".
type LibraryFacts struct {
	// Supers are the fully-qualified names of the semantic direct supertypes,
	// in derivation order. Nil when an edge has no qualified name to restore it
	// by, which leaves the whole set to be derived from the declaration.
	Supers []ElementRef

	// Unit is the reduction of a measurement unit to base units. Nil for a
	// symbol that is not a measurement unit.
	Unit *UnitFacts

	// Dimension is the quantity dimension a measurement unit measures in. Nil
	// when the symbol is not a unit or its dimension is undetermined.
	Dimension *DimensionFacts

	// Abstract records that the declaration is abstract. False means concrete or
	// not recorded, which leaves the declaration to say.
	Abstract bool

	// Recorded marks the facts of an interface-record symbol, one installed with
	// no declaration at all (see DocumentRecord). Every field below is then
	// authoritative: a zero value is the answer, not "derive it".
	Recorded bool

	// Alias is the element an alias symbol's own target names, an alias itself
	// in a chain; zero when the target resolves to nothing.
	Alias ElementRef

	// Redefines are the fully-qualified names of the features the declaration
	// redefines by an explicit clause, in declaration order.
	Redefines []ElementRef

	// References is the fully-qualified name of the feature the declaration
	// reference-subsets (`references f`, `perform a`), or "".
	References ElementRef

	// Direction is the declared feature direction of a usage.
	Direction ast.FeatureDirection

	// Modifiers are the declaration's boolean modifiers (`end`, `derived`, ...).
	Modifiers Modifiers

	// Multiplicity is the declared multiplicity as evaluated, nil when the
	// declaration states none.
	Multiplicity *MultiplicityFacts

	// Annotations is the metadata annotating the element, inline on its own
	// declaration, as names and constants.
	Annotations []AnnotationFacts

	// About are the fully-qualified names of the elements a metadata usage
	// annotates through its `about` clause, and Annotation the annotation it
	// states on them: its type and the values it binds.
	About      []ElementRef
	Annotation *AnnotationFacts

	// Ends are the end features a connector owns, by position: the ends of
	// its `connect` clause, then its body's `end` features. A zero entry is
	// an end with no symbol of its own (`connect a to b`).
	Ends []ElementRef

	// Node is the class of declaration the symbol was made from.
	Node NodeKind

	// Keyword is the keyword the declaration was written with (`part`,
	// `feature`, a user-defined keyword), "" when it states none; Notation is
	// the declaration's notation as Symbol.Notation reports it.
	Keyword  string
	Notation string

	// UsageKind and DefKind are the syntactic kind of a usage or definition.
	UsageKind ast.UsageKind
	DefKind   ast.DefinitionKind

	// Relationship is what a keyword-first relationship member declares
	// (`specialization S subtype A :> B;`), nil for any other declaration.
	Relationship *RelationshipDecl

	// Relationships are the relationships the declaration writes, in order,
	// each with the fully-qualified name of what it resolved to, or "" for one
	// that resolved to nothing.
	Relationships []RelationshipFacts

	// BaseType is the fully-qualified name of the type a metadata definition's
	// own body binds `baseType` to, "" for none (see ModBindsBaseType).
	BaseType ElementRef
	// Default is the value a feature of a metadata definition declares, which
	// an annotation of the type that leaves the feature unbound takes.
	Default *FilterValue
}

// NodeKind classifies the declaring node of a recorded symbol.
type NodeKind uint8

const (
	NodeNone NodeKind = iota
	NodeDefinition
	NodeUsage
	NodeConnectorEnd
	NodeCrossFeature
	NodeSubject
	NodeAssume
	NodeRequire
	NodeBodyExpr
	NodeAlias
	NodeImport
	NodeRelationship
	NodePrefixMetadata
	NodeTransition
	NodeOther
)

// NodeKindOf classifies a declaring node.
func NodeKindOf(decl ast.Node) NodeKind {
	switch decl.(type) {
	case nil:
		return NodeNone
	case *ast.Definition:
		return NodeDefinition
	case *ast.Usage:
		return NodeUsage
	case *ast.ConnectorEnd:
		return NodeConnectorEnd
	case *ast.CrossFeatureMember:
		return NodeCrossFeature
	case *ast.SubjectMember:
		return NodeSubject
	case *ast.AssumeMember:
		return NodeAssume
	case *ast.RequireMember:
		return NodeRequire
	case *ast.BodyExpr:
		return NodeBodyExpr
	case *ast.Alias:
		return NodeAlias
	case *ast.Import:
		return NodeImport
	case *ast.RelationshipMember:
		return NodeRelationship
	case *ast.PrefixMetadata:
		return NodePrefixMetadata
	case *ast.TransitionMember:
		return NodeTransition
	}
	return NodeOther
}

// Clone returns a copy of f sharing no slice or pointer with it.
func (f LibraryFacts) Clone() LibraryFacts {
	f.Supers = cloneRefs(f.Supers)
	f.Redefines = cloneRefs(f.Redefines)
	f.About = cloneRefs(f.About)
	f.Ends = cloneRefs(f.Ends)
	f.Alias = f.Alias.Clone()
	f.References = f.References.Clone()
	f.BaseType = f.BaseType.Clone()
	f.Relationships = slices.Clone(f.Relationships)
	for i := range f.Relationships {
		f.Relationships[i].Target = f.Relationships[i].Target.Clone()
	}
	f.Annotations = slices.Clone(f.Annotations)
	for i := range f.Annotations {
		f.Annotations[i] = f.Annotations[i].Clone()
	}
	if f.Annotation != nil {
		a := f.Annotation.Clone()
		f.Annotation = &a
	}
	if f.Unit != nil {
		u := *f.Unit
		u.Factors = slices.Clone(u.Factors)
		f.Unit = &u
	}
	if f.Dimension != nil {
		d := *f.Dimension
		d.Factors = slices.Clone(d.Factors)
		f.Dimension = &d
	}
	if f.Multiplicity != nil {
		m := *f.Multiplicity
		f.Multiplicity = &m
	}
	if f.Default != nil {
		v := *f.Default
		f.Default = &v
	}
	if f.Relationship != nil {
		r := *f.Relationship
		f.Relationship = &r
	}
	return f
}

// Clone returns a copy sharing no slice with a.
func (a AnnotationFacts) Clone() AnnotationFacts {
	a.Type = a.Type.Clone()
	a.Values = slices.Clone(a.Values)
	return a
}

// RelationshipFacts is one written relationship of a declaration.
type RelationshipFacts struct {
	Kind   ast.RelationshipKind
	Target ElementRef
}

// RelationshipDecl is the relationship a keyword-first member declares, which
// conjugation writes as a form of its own (`conjugation C conjugate A ~ B;`).
type RelationshipDecl struct {
	Kind       ast.RelationshipKind
	Conjugated bool
}

// RelationshipDecl is the relationship the symbol's keyword-first member
// declares, from its declaration or its record; ok is false for any other symbol.
func (s *Symbol) RelationshipDecl() (RelationshipDecl, bool) {
	if s.Recorded() {
		if s.Facts.Relationship == nil {
			return RelationshipDecl{}, false
		}
		return *s.Facts.Relationship, true
	}
	if rel, ok := s.Decl.(*ast.RelationshipMember); ok {
		return RelationshipDecl{Kind: rel.Kind, Conjugated: rel.Conjugated}, true
	}
	return RelationshipDecl{}, false
}

// DeclaresTransition reports whether the symbol was declared by a transition
// member (`transition t first a then b;`), from its declaration or its record.
func (s *Symbol) DeclaresTransition() bool {
	if s.Recorded() {
		return s.Facts.Node == NodeTransition
	}
	_, ok := s.Decl.(*ast.TransitionMember)
	return ok
}

// DeclaresUsage reports whether the symbol was declared by a usage, from its
// declaration or its record.
func (s *Symbol) DeclaresUsage() bool {
	if s.Recorded() {
		return s.Facts.Node == NodeUsage
	}
	_, ok := s.Decl.(*ast.Usage)
	return ok
}

// DeclaresDefinition reports whether the symbol was declared by a definition.
func (s *Symbol) DeclaresDefinition() bool {
	if s.Recorded() {
		return s.Facts.Node == NodeDefinition
	}
	_, ok := s.Decl.(*ast.Definition)
	return ok
}

// UsageKind is the syntactic kind of the usage declaring the symbol.
func (s *Symbol) UsageKind() (ast.UsageKind, bool) {
	if s.Recorded() {
		return s.Facts.UsageKind, s.Facts.Node == NodeUsage
	}
	if u, ok := s.Decl.(*ast.Usage); ok {
		return u.Kind, true
	}
	return 0, false
}

// DefinitionKind is the syntactic kind of the definition declaring the symbol.
func (s *Symbol) DefinitionKind() (ast.DefinitionKind, bool) {
	if s.Recorded() {
		return s.Facts.DefKind, s.Facts.Node == NodeDefinition
	}
	if d, ok := s.Decl.(*ast.Definition); ok {
		return d.Kind, true
	}
	return 0, false
}

// Keyword is the keyword the symbol's definition or usage was written with.
func (s *Symbol) Keyword() string {
	if s.Recorded() {
		return s.Facts.Keyword
	}
	switch d := s.Decl.(type) {
	case *ast.Definition:
		return d.Keyword
	case *ast.Usage:
		return d.Keyword
	}
	return ""
}

// RecordedRelationships are the relationships a recorded symbol's declaration
// wrote of one kind, by target name.
func (s *Symbol) RecordedRelationships(kind ast.RelationshipKind) []ElementRef {
	if !s.Recorded() {
		return nil
	}
	var out []ElementRef
	for _, rel := range s.Facts.Relationships {
		if rel.Kind == kind {
			out = append(out, rel.Target)
		}
	}
	return out
}

// MultiplicityFacts is a declared multiplicity range as evaluated: a bound is
// either known, unbounded (`*`), or unknown when it names a value the record
// cannot carry.
type MultiplicityFacts struct {
	Lower, Upper BoundFacts
}

// BoundFacts is one evaluated multiplicity bound. A Named bound is written as
// a feature name; its value is the feature's in the declaring scope, which a
// reader that evaluates bounds without a scope does not see.
type BoundFacts struct {
	Value    int64
	Known    bool
	Infinite bool
	Named    bool
}

// Modifiers is the set of boolean modifiers a declaration states, as a
// recorded fact reads them.
type Modifiers uint32

// The modifiers a declaration can state (KerML 8.3.3, SysML v2 8.3.9).
const (
	ModEnd Modifiers = 1 << iota
	ModDerived
	ModInitial
	ModReference
	ModComposite
	ModPortion
	ModVariation
	ModVariant
	ModIndividual
	ModConjugated
	ModResult
	ModOrdered
	ModNonunique
	ModConstant
	ModVariable
	ModParallel
	ModDefault
	ModNegated
	ModDeclaresRequirement
	ModAll
	ModChain
	ModEvent
	// The traits below answer, for a recorded symbol, what the resolver asks
	// of a declaration (see resolve.DeclarationTraits).
	ModParameter
	ModImplicitlyRedefined
	ModContributesName
	ModUnresolvedRedefinition
	ModParameterizedByName
	ModBindsBaseType
	ModAcceptPayload
	// ModNamesNothing marks a derived name its target did not supply: the
	// symbol binds no name (see resolve.Resolver.BindsName).
	ModNamesNothing
	// ModValued marks a usage whose declaration binds it a value (`= v`, `default v`).
	ModValued
)

// Has reports whether every modifier of mask is set.
func (m Modifiers) Has(mask Modifiers) bool { return m&mask == mask }

// Recorded reports whether sym is an interface-record symbol: one installed
// from a document's record, with no declaration to read. A reader that needs
// the declaration answers such a symbol with ErrNeedsHydration.
func (s *Symbol) Recorded() bool {
	return s != nil && s.Decl == nil && s.Facts != nil && s.Facts.Recorded
}

// IsAbstract reports whether sym is declared abstract, reading the installed
// fact when one memoizes it and the declaration otherwise.
func IsAbstract(sym *Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Facts != nil && sym.Facts.Abstract {
		return true
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.IsAbstract
	case *ast.Usage:
		return d.IsAbstract
	}
	return false
}

// InstallLibraryFacts installs derived facts on the symbols the named document
// declares, keyed by the fully-qualified name they are declared under. A name
// the document does not declare is ignored: facts are a memo of that document's
// own declarations.
func (idx *Index) InstallLibraryFacts(name string, facts map[string]*LibraryFacts) {
	idx.mustBeWritable("InstallLibraryFacts")
	for _, e := range idx.contributions.at(name) {
		if idx.declaredAt.at(e.sym) != e.fqn {
			continue // a short-name key of a symbol declared under its primary one
		}
		if f, ok := facts[e.fqn]; ok {
			e.sym.Facts = f
		}
	}
}
