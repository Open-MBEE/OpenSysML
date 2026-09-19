// Package symbols builds an immutable per-document scope tree and a global
// qualified-name index over a parsed ast.RootNamespace.
package symbols

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// SymbolKind classifies a declared name.
type SymbolKind int

const (
	SymbolUnknown SymbolKind = iota
	SymbolPackage
	SymbolNamespace
	SymbolAlias
	SymbolDependency
	SymbolComment
	SymbolDocumentation
	SymbolTextualRepresentation
	SymbolPartDef
	SymbolAttributeDef
	SymbolPartUsage
	SymbolAttributeUsage
	// Tier A definitions.
	SymbolItemDef
	SymbolOccurrenceDef
	SymbolIndividualDef
	SymbolMetadataDef
	SymbolMetaclass // KerML metaclass (similar to metadata def)
	SymbolEnumerationDef
	SymbolViewDef
	SymbolViewpointDef
	SymbolRenderingDef
	SymbolConcernDef
	// Tier B definitions.
	SymbolConnectionDef
	SymbolFlowDef
	SymbolPortDef
	SymbolInterfaceDef
	SymbolAllocationDef
	// Tier C definitions.
	SymbolActionDef
	SymbolStateDef
	SymbolCalcDef
	SymbolConstraintDef
	SymbolRequirementDef
	SymbolCaseDef
	SymbolAnalysisCaseDef
	SymbolVerificationCaseDef
	SymbolUseCaseDef
	// Tier A usages.
	SymbolItemUsage
	SymbolOccurrenceUsage
	SymbolIndividualUsage
	SymbolMetadataUsage
	SymbolEnumerationUsage
	SymbolViewUsage
	SymbolViewpointUsage
	SymbolRenderingUsage
	SymbolConcernUsage
	// Tier B usages.
	SymbolConnectionUsage
	// SymbolSuccessionUsage classifies a succession usage: a SuccessionAsUsage
	// (SysML v2 §8.3.13.7), a ConnectorAsUsage that is not a ConnectionUsage.
	SymbolSuccessionUsage
	SymbolFlowUsage
	SymbolPortUsage
	SymbolInterfaceUsage
	SymbolAllocationUsage
	// Tier C usages.
	SymbolActionUsage
	SymbolStateUsage
	SymbolCalcUsage
	SymbolConstraintUsage
	SymbolRequirementUsage
	SymbolCaseUsage
	SymbolAnalysisCaseUsage
	SymbolVerificationCaseUsage
	SymbolUseCaseUsage
	// SymbolConnectorEnd is an end feature a connector usage declares in its
	// connect clause (`connect bead references t.bead`).
	SymbolConnectorEnd
	// SymbolCrossFeature is the cross feature an end feature declares inline
	// ahead of its own declaration (`end x1 [0..1] feature x : C1`).
	SymbolCrossFeature
	// SymbolKerMLType classifies a KerML type declaration — `class`,
	// `classifier`, `struct`, `assoc`, `behavior`, `predicate` — which the SysML
	// definition taxonomy has no counterpart for.
	SymbolKerMLType
	// SymbolSatisfyRequirementUsage classifies a `satisfy requirement` usage: a
	// RequirementUsage specialization (SysML v2 §8.3.19) with its own kind, since
	// only a satisfy usage may be an assertion of a requirement's satisfaction.
	SymbolSatisfyRequirementUsage
	// SymbolRelationship classifies a relationship written keyword-first as a
	// member of its own (`specialization Gen subtype A specializes B`), whose two
	// ends are ordered (KerML §7.2).
	SymbolRelationship
	// SymbolMultiplicity classifies a named multiplicity member, `multiplicity
	// exactlyOne [1..1]` or `multiplicity single subsets exactlyOne`.
	SymbolMultiplicity
)

var symbolKindNames = map[SymbolKind]string{
	SymbolUnknown:                 "unknown",
	SymbolPackage:                 "package",
	SymbolNamespace:               "namespace",
	SymbolAlias:                   "alias",
	SymbolDependency:              "dependency",
	SymbolRelationship:            "relationship",
	SymbolMultiplicity:            "multiplicity",
	SymbolComment:                 "comment",
	SymbolDocumentation:           "documentation",
	SymbolTextualRepresentation:   "textualRepresentation",
	SymbolPartDef:                 "partDef",
	SymbolAttributeDef:            "attributeDef",
	SymbolPartUsage:               "partUsage",
	SymbolAttributeUsage:          "attributeUsage",
	SymbolItemDef:                 "itemDef",
	SymbolOccurrenceDef:           "occurrenceDef",
	SymbolIndividualDef:           "individualDef",
	SymbolMetadataDef:             "metadataDef",
	SymbolMetaclass:               "metaclass",
	SymbolEnumerationDef:          "enumDef",
	SymbolViewDef:                 "viewDef",
	SymbolViewpointDef:            "viewpointDef",
	SymbolRenderingDef:            "renderingDef",
	SymbolConcernDef:              "concernDef",
	SymbolConnectionDef:           "connectionDef",
	SymbolFlowDef:                 "flowDef",
	SymbolPortDef:                 "portDef",
	SymbolInterfaceDef:            "interfaceDef",
	SymbolAllocationDef:           "allocationDef",
	SymbolActionDef:               "actionDef",
	SymbolStateDef:                "stateDef",
	SymbolCalcDef:                 "calcDef",
	SymbolConstraintDef:           "constraintDef",
	SymbolRequirementDef:          "requirementDef",
	SymbolCaseDef:                 "caseDef",
	SymbolAnalysisCaseDef:         "analysisCaseDef",
	SymbolVerificationCaseDef:     "verificationCaseDef",
	SymbolUseCaseDef:              "useCaseDef",
	SymbolItemUsage:               "itemUsage",
	SymbolOccurrenceUsage:         "occurrenceUsage",
	SymbolIndividualUsage:         "individualUsage",
	SymbolMetadataUsage:           "metadataUsage",
	SymbolEnumerationUsage:        "enumUsage",
	SymbolViewUsage:               "viewUsage",
	SymbolViewpointUsage:          "viewpointUsage",
	SymbolRenderingUsage:          "renderingUsage",
	SymbolConcernUsage:            "concernUsage",
	SymbolConnectionUsage:         "connectionUsage",
	SymbolSuccessionUsage:         "successionUsage",
	SymbolFlowUsage:               "flowUsage",
	SymbolPortUsage:               "portUsage",
	SymbolInterfaceUsage:          "interfaceUsage",
	SymbolAllocationUsage:         "allocationUsage",
	SymbolActionUsage:             "actionUsage",
	SymbolStateUsage:              "stateUsage",
	SymbolCalcUsage:               "calcUsage",
	SymbolConstraintUsage:         "constraintUsage",
	SymbolRequirementUsage:        "requirementUsage",
	SymbolSatisfyRequirementUsage: "satisfyRequirementUsage",
	SymbolCaseUsage:               "caseUsage",
	SymbolAnalysisCaseUsage:       "analysisCaseUsage",
	SymbolVerificationCaseUsage:   "verificationCaseUsage",
	SymbolUseCaseUsage:            "useCaseUsage",
	SymbolConnectorEnd:            "connectorEnd",
	SymbolCrossFeature:            "crossFeature",
	SymbolKerMLType:               "kermlType",
}

// String returns the display name of the kind.
func (k SymbolKind) String() string {
	if s, ok := symbolKindNames[k]; ok {
		return s
	}
	return "unknown"
}

// Family names the kind without its def/usage distinction, in the notation's words:
// SymbolActionDef and SymbolActionUsage are both "action", SymbolUseCaseDef "use case".
func (k SymbolKind) Family() string {
	name := k.String()
	name = strings.TrimSuffix(strings.TrimSuffix(name, "Def"), "Usage")
	return spacedWords(name)
}

// IsFeature reports whether k classifies a KerML Feature (KerML 1.0 §8.3.3): a
// usage of any kind, a connector end or a multiplicity, never a type or a namespace.
func (k SymbolKind) IsFeature() bool {
	switch k {
	case SymbolPartUsage, SymbolAttributeUsage, SymbolItemUsage, SymbolOccurrenceUsage,
		SymbolIndividualUsage, SymbolMetadataUsage, SymbolEnumerationUsage, SymbolViewUsage,
		SymbolViewpointUsage, SymbolRenderingUsage, SymbolConcernUsage, SymbolConnectionUsage,
		SymbolSuccessionUsage, SymbolFlowUsage, SymbolPortUsage, SymbolInterfaceUsage,
		SymbolAllocationUsage, SymbolActionUsage, SymbolStateUsage, SymbolCalcUsage,
		SymbolConstraintUsage, SymbolRequirementUsage, SymbolSatisfyRequirementUsage,
		SymbolCaseUsage, SymbolAnalysisCaseUsage, SymbolVerificationCaseUsage, SymbolUseCaseUsage,
		SymbolConnectorEnd, SymbolCrossFeature, SymbolMultiplicity:
		return true
	}
	return false
}

// IsFeature reports whether s declares a KerML Feature: by its kind, or by its
// usage declaration when the kind is unclassified (a named binding).
// Owner is the element s was declared in, and nil at the root of a document
// or for a nil symbol.
func (s *Symbol) Owner() *Symbol {
	if s == nil || s.OwnerScope == nil {
		return nil
	}
	return s.OwnerScope.Owner()
}

func (s *Symbol) IsFeature() bool {
	if s.Kind != SymbolUnknown {
		return s.Kind.IsFeature()
	}
	_, ok := s.Decl.(*ast.Usage)
	return ok
}

// IsDefinition reports whether k classifies a definition — a SysML `def` or a
// KerML classifier — as opposed to a usage or a non-type member.
func (k SymbolKind) IsDefinition() bool {
	switch k {
	case SymbolPartDef, SymbolAttributeDef, SymbolItemDef, SymbolOccurrenceDef,
		SymbolIndividualDef, SymbolMetadataDef, SymbolMetaclass, SymbolEnumerationDef,
		SymbolViewDef, SymbolViewpointDef, SymbolRenderingDef, SymbolConcernDef,
		SymbolConnectionDef, SymbolFlowDef, SymbolPortDef, SymbolInterfaceDef,
		SymbolAllocationDef, SymbolActionDef, SymbolStateDef, SymbolCalcDef,
		SymbolConstraintDef, SymbolRequirementDef, SymbolCaseDef, SymbolAnalysisCaseDef,
		SymbolVerificationCaseDef, SymbolUseCaseDef, SymbolKerMLType:
		return true
	}
	return false
}

// Notation names a symbol the way the notation declares it — "part def",
// "state", "render" — rather than by its internal classification.
func (s *Symbol) Notation() string {
	if n := ast.Notation(s.Decl); n != "" {
		return n
	}
	// A declaration the notation has no keyword of its own for — and a cached
	// library symbol, which carries no declaration at all — is named by its
	// classification.
	return spacedWords(s.Kind.String())
}

// spacedWords turns a camel-cased classification into lower-case words, so
// "partDef" reads as "part def".
func spacedWords(name string) string {
	var b strings.Builder
	for i, ch := range name {
		if ch >= 'A' && ch <= 'Z' {
			if i > 0 {
				b.WriteByte(' ')
			}
			ch += 'a' - 'A'
		}
		b.WriteRune(ch)
	}
	return b.String()
}

// UnitFacts is a measurement unit reduced to a scale factor over base units,
// each named by its qualified name, in the form a library index cache carries.
type UnitFacts struct {
	ScaleNum float64
	ScaleDen float64
	Factors  []UnitFactorFacts

	// Irreducible marks a measurement unit whose reduction is not derivable
	// from its declaration, so that it is still known to be a unit.
	Irreducible bool
}

// UnitFactorFacts is one base unit of a reduced measurement unit, named by its
// qualified name, and its exponent.
type UnitFactorFacts struct {
	FQN      string
	Exponent float64
}

// DimensionFacts is a quantity dimension in the form a library index cache
// carries: the base quantities it is a product of powers of, each named by its
// qualified name. No factors is the dimension of a count or of a like ratio.
type DimensionFacts struct {
	Factors []DimensionFactorFacts
}

// DimensionFactorFacts is one base quantity of a dimension, named by its
// qualified name, and its exponent.
type DimensionFactorFacts struct {
	FQN      string
	Exponent float64
}

// Symbol describes one declared name. The same declaration may be reachable
// through more than one Symbol only when it declares both a short and a
// primary name; in that case a single Symbol is registered under both keys.
type Symbol struct {
	Name       string         // the key this Symbol was primarily created for
	Kind       SymbolKind     // classification
	Decl       ast.Node       // the declaring AST node
	Visibility ast.Visibility // declared visibility
	DeclSpan   source.Span    // span of the declaration (for diagnostics)
	NameSpan   source.Span    // span of the declared identifier alone (for rename); zero when unknown
	Scope      *Scope         // the child scope this declaration owns, or nil for leaves
	OwnerScope *Scope         // the enclosing scope this declaration was declared in

	// LeadingTrivia is the comment/note trivia attached to the member wrapper
	// preceding this declaration (captured before unwrap, since wrappers carry
	// the trivia while the unwrapped inner Decl does not). Used for doc hover.
	LeadingTrivia []ast.Trivia

	DocName string // name of the document that declares this symbol (stamped after Build)

	// Facts is the derived analysis installed for a library symbol, a memo of
	// work its declaration would otherwise repeat. Nil for everything else.
	Facts *LibraryFacts

	// ShortName is the short name from Identification (e.g., "kg" for "kilogram").
	// Empty if the declaration states none.
	ShortName string

	// Naming records how the symbol came by its Name: declared, or derived
	// from its naming feature (KerML 7.3.4.5 Feature::effectiveName).
	Naming Naming

	// NamingTarget is the reference that named this symbol when Naming is
	// derived: the target of its reference subsetting or redefinition. Resolving
	// that reference must not see the name it gave away, or it would resolve to
	// the feature that borrowed it (KerML 7.3.4.5).
	NamingTarget ast.Node
}

// Naming tells where a symbol's Name comes from.
type Naming uint8

const (
	// NamedByDeclaration: Name is the declared name, or empty.
	NamedByDeclaration Naming = iota

	// NamedByReference: Name is the name of the feature the declaration
	// references (`perform a;`), an ordinary member name that hides nothing.
	NamedByReference

	// NamedByRedefinition: Name is the name of the feature the declaration
	// redefines (`part :>> p;`), which it replaces in its owner.
	NamedByRedefinition
)

// EffectiveName reports whether Name was derived from a naming feature
// rather than declared.
func (s *Symbol) EffectiveName() bool {
	return s.Naming != NamedByDeclaration
}

// AnnotationFacts is one metadata annotation reduced to names and constants: the
// fully-qualified name of the metadata type annotating it, and the values the
// annotation body binds its features to, as written.
type AnnotationFacts struct {
	TypeFQN string
	Values  []AnnotationValueFacts
}

// AnnotationValueFacts is one feature binding inside an annotation body
// (`@Safety{isMandatory = true;}`), holding the value already evaluated, so that
// a filter condition reading it decides the same way where the declaration it
// came from is gone.
type AnnotationValueFacts struct {
	Feature string
	Value   FilterValue
}
