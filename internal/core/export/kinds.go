package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
)

// The tables below are the single source of truth for the correspondence
// between a SysML declaration keyword, the AST kind the parser produced for it,
// and the metaclass name written into RDF. Conversion runs in both directions,
// so keeping one table per direction in sync by hand would drift: the reverse
// maps are derived here instead.

// definitionMetaclass maps a definition kind to its SysML metaclass name.
var definitionMetaclass = map[ast.DefinitionKind]string{
	ast.DefPart:             "PartDefinition",
	ast.DefAttribute:        "AttributeDefinition",
	ast.DefItem:             "ItemDefinition",
	ast.DefOccurrence:       "OccurrenceDefinition",
	ast.DefIndividual:       "IndividualDefinition",
	ast.DefMetaclass:        "Metaclass",
	ast.DefMetadata:         "MetadataDefinition",
	ast.DefEnumeration:      "EnumerationDefinition",
	ast.DefView:             "ViewDefinition",
	ast.DefViewpoint:        "ViewpointDefinition",
	ast.DefRendering:        "RenderingDefinition",
	ast.DefConcern:          "ConcernDefinition",
	ast.DefConnection:       "ConnectionDefinition",
	ast.DefFlow:             "FlowDefinition",
	ast.DefPort:             "PortDefinition",
	ast.DefInterface:        "InterfaceDefinition",
	ast.DefAllocation:       "AllocationDefinition",
	ast.DefBinding:          "BindingConnectorDefinition",
	ast.DefAction:           "ActionDefinition",
	ast.DefState:            "StateDefinition",
	ast.DefCalc:             "CalculationDefinition",
	ast.DefConstraint:       "ConstraintDefinition",
	ast.DefRequirement:      "RequirementDefinition",
	ast.DefCase:             "CaseDefinition",
	ast.DefAnalysisCase:     "AnalysisCaseDefinition",
	ast.DefVerificationCase: "VerificationCaseDefinition",
	ast.DefUseCase:          "UseCaseDefinition",
	ast.DefBehavior:         "Behavior",
	ast.DefAssoc:            "Association",
	ast.DefStruct:           "Structure",
	ast.DefClass:            "Class",
	ast.DefPredicate:        "Predicate",
	ast.DefBool:             "BooleanExpression",
}

// usageMetaclass maps a usage kind to its SysML metaclass name.
var usageMetaclass = map[ast.UsageKind]string{
	ast.UsagePart:             "PartUsage",
	ast.UsageAttribute:        "AttributeUsage",
	ast.UsageItem:             "ItemUsage",
	ast.UsageOccurrence:       "OccurrenceUsage",
	ast.UsageIndividual:       "IndividualUsage",
	ast.UsageMetadata:         "MetadataUsage",
	ast.UsageEnumeration:      "EnumerationUsage",
	ast.UsageView:             "ViewUsage",
	ast.UsageViewpoint:        "ViewpointUsage",
	ast.UsageRendering:        "RenderingUsage",
	ast.UsageViewRendering:    "ViewRenderingMembership",
	ast.UsageConcern:          "ConcernUsage",
	ast.UsageFramedConcern:    "FramedConcernMembership",
	ast.UsageConnection:       "ConnectionUsage",
	ast.UsageConnector:        "ConnectorAsUsage",
	ast.UsageSuccession:       "SuccessionAsUsage",
	ast.UsageFlow:             "FlowUsage",
	ast.UsagePort:             "PortUsage",
	ast.UsageInterface:        "InterfaceUsage",
	ast.UsageInteraction:      "InteractionUsage",
	ast.UsageAllocation:       "AllocationUsage",
	ast.UsageBinding:          "BindingConnectorAsUsage",
	ast.UsageAction:           "ActionUsage",
	ast.UsageState:            "StateUsage",
	ast.UsageTransition:       "TransitionUsage",
	ast.UsageStep:             "Step",
	ast.UsageCalc:             "CalculationUsage",
	ast.UsageExpr:             "Expression",
	ast.UsageConstraint:       "ConstraintUsage",
	ast.UsageRequirement:      "RequirementUsage",
	ast.UsageSatisfy:          "SatisfyRequirementUsage",
	ast.UsageSubject:          "SubjectMembership",
	ast.UsageActor:            "ActorMembership",
	ast.UsageStakeholder:      "StakeholderMembership",
	ast.UsageObjective:        "ObjectiveMembership",
	ast.UsageCase:             "CaseUsage",
	ast.UsageAnalysisCase:     "AnalysisCaseUsage",
	ast.UsageVerificationCase: "VerificationCaseUsage",
	ast.UsageUseCase:          "UseCaseUsage",
	ast.UsageBehavior:         "BehaviorUsage",
	ast.UsageAssoc:            "AssociationUsage",
	ast.UsageStruct:           "StructureUsage",
	ast.UsageClass:            "ClassUsage",
	ast.UsagePredicate:        "PredicateUsage",
	ast.UsageBool:             "BooleanUsage",
}

// kermlTypeUsage marks the usage kinds the parser records a KerML type
// declaration under: their metaclass names a Type the ontology does not declare.
var kermlTypeUsage = map[ast.UsageKind]bool{
	ast.UsageClass:       true,
	ast.UsageStruct:      true,
	ast.UsageAssoc:       true,
	ast.UsageBehavior:    true,
	ast.UsagePredicate:   true,
	ast.UsageInteraction: true,
}

// isType reports whether a metaclass is a Type, by the ontology or by kermlTypeUsage.
func isType(metaclass string) bool {
	return ontology.IsAncestorOrSelf(metaclass, "Type") || kermlTypeUsage[metaclassUsage[metaclass]]
}

// keywordMetaclass names the metaclass a keyword builds where several spellings
// share one AST kind (KerML.xtext:788, :924; SysML.xtext:632).
var keywordMetaclass = map[string]string{
	"datatype": "DataType",
	"function": "Function",
	"ref":      "ReferenceUsage",
}

// crossFeatureMetaclass is what an end's head-written cross feature builds: a bare
// Feature in KerML, a ReferenceUsage in SysML (SysML.xtext OwnedCrossFeature).
func crossFeatureMetaclass(kerml bool) string {
	if kerml {
		return "Feature"
	}
	return keywordMetaclass["ref"]
}

// The metaclasses an `event` or `assert` declaration builds: the keyword is a
// type of its own in the metamodel, so the graph types it rather than spelling it.
const (
	mEventOccurrenceUsage  = "EventOccurrenceUsage"
	mAssertConstraintUsage = "AssertConstraintUsage"
)

// usageMetaclassOf gives the metaclass a usage builds, reading the keyword where
// the kind does not decide it; a kindless one is a DefaultReferenceUsage, as is
// a kindless `name = value;` in a metadata body (SysML.xtext MetadataBodyUsage).
func usageMetaclassOf(n *ast.Usage, inMetadataBody bool) (string, bool) {
	if m, ok := keywordMetaclass[n.Keyword]; ok {
		return m, true
	}
	switch {
	case eventOccurrence(n):
		return mEventOccurrenceUsage, true
	case assertedConstraint(n):
		return mAssertConstraintUsage, true
	}
	if n.Keyword == "" && (n.Kind == ast.UsageAttribute || inMetadataBody && n.Kind == ast.UsageEnumeration) {
		return "ReferenceUsage", true
	}
	m, ok := usageMetaclass[n.Kind]
	return m, ok
}

// eventOccurrence reports an occurrence declared with `event`, whether as its
// kind (`event m.start`) or as a modifier (`event occurrence e`).
func eventOccurrence(n *ast.Usage) bool {
	return n.Kind == ast.UsageOccurrence && (n.IsEvent || n.Keyword == "event")
}

// assertedConstraint reports a constraint declared with `assert`, prefixing
// `constraint` or standing in for it (SysML.xtext AssertConstraintUsage).
func assertedConstraint(n *ast.Usage) bool {
	return n.Kind == ast.UsageConstraint && (n.Keyword == "assert" || n.PrefixKeyword == "assert")
}

// portionKeyword gives the `snapshot`/`timeslice` a usage is a portion of, or "".
func portionKeyword(portion ast.PortionKind) string {
	switch portion {
	case ast.PortionSnapshot:
		return "snapshot"
	case ast.PortionTimeslice:
		return "timeslice"
	}
	return ""
}

// metaclassKeywordUsage reads the keyword-decided metaclasses back to the kind
// the parser records for them.
var metaclassKeywordUsage = map[string]ast.UsageKind{
	"DataType":             ast.UsageAttribute,
	"Function":             ast.UsageCalc,
	"ReferenceUsage":       ast.UsageAttribute,
	mEventOccurrenceUsage:  ast.UsageOccurrence,
	mAssertConstraintUsage: ast.UsageConstraint,
}

// definitionKeyword and usageKeyword give the source keyword for a kind. The
// AST's own String() is the keyword for every kind, which is what makes the
// printer able to reconstruct a declaration head from the metaclass alone.
func definitionKeyword(kind ast.DefinitionKind) string { return kind.String() }
func usageKeyword(kind ast.UsageKind) string           { return kind.String() }

// memberDeclarationKeyword gives the kind keyword a member usage states after
// its own keyword when it declares an element rather than referencing one, or
// "" for a kind with no such form: `render rendering r : AsTree` declares a
// rendering where `render r` names one (SysML.xtext ViewRenderingUsage,
// FramedConcernUsage).
func memberDeclarationKeyword(kind ast.UsageKind) string {
	switch kind {
	case ast.UsageViewRendering:
		return "rendering"
	case ast.UsageFramedConcern:
		return "concern"
	}
	return ""
}

// relationshipProperty maps a declaration-head relationship to its RDF
// predicate name in the SysML vocabulary.
var relationshipProperty = map[ast.RelationshipKind]string{
	ast.RelTyping:      "type",
	ast.RelSpecializes: "specializes",
	ast.RelSubsets:     "subsets",
	ast.RelRedefines:   "redefines",
	ast.RelReferences:  "references",
	ast.RelCrosses:     "crosses",
	ast.RelDisjoint:    "disjointFrom",
	ast.RelIntersects:  "intersects",
	ast.RelDifferences: "differences",
	ast.RelInverseOf:   "inverseOf",
	ast.RelUnions:      "unions",
	ast.RelChains:      "chains",
	ast.RelIncludes:    "includes",
	ast.RelVia:         "via",
	ast.RelAnnotates:   pAnnotatedElement,
	ast.RelSubject:     "subject",
	ast.RelFeaturedBy:  "featuringType",
}

// relationshipEndForm describes how a keyword-first relationship element is
// written as a graph: its metaclass and the properties naming its two ends,
// which the OMG metamodel keeps ordered.
type relationshipEndForm struct {
	metaclass string
	source    string
	target    string
}

// relationshipElementForm maps a keyword-first relationship to its metaclass and
// its ordered end properties (KerML §7.2, §8.3).
var relationshipElementForm = map[ast.RelationshipKind]relationshipEndForm{
	ast.RelSpecializes: {"Specialization", "specific", "general"},
	ast.RelTyping:      {"FeatureTyping", "typedFeature", "type"},
	ast.RelSubsets:     {"Subsetting", "subsettingFeature", "subsettedFeature"},
	ast.RelRedefines:   {"Redefinition", "redefiningFeature", "redefinedFeature"},
	ast.RelInverseOf:   {"FeatureInverting", "invertingFeature", "featureInverted"},
	ast.RelFeaturedBy:  {"TypeFeaturing", "featureOfType", "featuringType"},
	ast.RelDisjoint:    {"Disjoining", "typeDisjoined", "disjoiningType"},
}

// conjugationForm is the form a `conjugate x conjugates y` member takes, which
// is a Conjugation rather than the Specialization its kind records.
var conjugationForm = relationshipEndForm{"Conjugation", "conjugatedType", "originalType"}

// relationshipMemberSyntax reads a keyword-first relationship back from its
// graph, keyed by the keyword the notation states.
var relationshipMemberSyntax = map[string]struct {
	source, target, separator string
}{
	"subtype":       {"specific", "general", "specializes"},
	"subclassifier": {"specific", "general", "specializes"},
	"typing":        {"typedFeature", "type", "typed by"},
	"subset":        {"subsettingFeature", "subsettedFeature", "subsets"},
	"redefinition":  {"redefiningFeature", "redefinedFeature", "redefines"},
	"conjugate":     {"conjugatedType", "originalType", "conjugates"},
	"inverse":       {"invertingFeature", "featureInverted", "of"},
	"featuring":     {"featureOfType", "featuringType", "by"},
	"disjoint":      {"typeDisjoined", "disjoiningType", "from"},
}

// relationshipSyntax gives the source syntax that introduces a relationship
// when a declaration head is rebuilt from RDF.
var relationshipSyntax = map[ast.RelationshipKind]string{
	ast.RelTyping:      ":",
	ast.RelSpecializes: "specializes",
	ast.RelSubsets:     "subsets",
	ast.RelRedefines:   "redefines",
	ast.RelReferences:  "references",
	ast.RelCrosses:     "crosses",
	ast.RelDisjoint:    "disjoint from",
	ast.RelIntersects:  "intersects",
	ast.RelDifferences: "differences",
	ast.RelInverseOf:   "inverse of",
	ast.RelUnions:      "unions",
	ast.RelChains:      "chains",
	ast.RelIncludes:    "includes",
	ast.RelVia:         "via",
	ast.RelAnnotates:   "about",
	ast.RelSubject:     "by",
	ast.RelFeaturedBy:  "featured by",
}

// Reverse lookups, derived once so the two directions cannot disagree.
var (
	metaclassDefinition = map[string]ast.DefinitionKind{}
	metaclassUsage      = map[string]ast.UsageKind{}
)

func init() {
	for kind, name := range definitionMetaclass {
		metaclassDefinition[name] = kind
	}
	for kind, name := range usageMetaclass {
		metaclassUsage[name] = kind
	}
	for name, kind := range metaclassKeywordUsage {
		metaclassUsage[name] = kind
	}
}

// relationshipOrder is the canonical order of a head's relationships, in the
// graph and in the notation written back; typing comes first as the ':' clause.
var relationshipOrder = []ast.RelationshipKind{
	ast.RelTyping,
	ast.RelSpecializes,
	ast.RelSubsets,
	ast.RelRedefines,
	ast.RelReferences,
	ast.RelCrosses,
	ast.RelDisjoint,
	ast.RelIntersects,
	ast.RelDifferences,
	ast.RelInverseOf,
	ast.RelUnions,
	ast.RelChains,
	ast.RelIncludes,
	ast.RelVia,
	ast.RelAnnotates,
	ast.RelSubject,
	ast.RelFeaturedBy,
}

// visibilityKeyword renders a declared visibility, or "" for the default.
func visibilityKeyword(v ast.Visibility) string {
	switch v {
	case ast.VisibilityPublic:
		return "public"
	case ast.VisibilityPrivate:
		return "private"
	case ast.VisibilityProtected:
		return "protected"
	}
	return ""
}

// visibilityOf reverses visibilityKeyword.
func visibilityOf(keyword string) ast.Visibility {
	switch keyword {
	case "public":
		return ast.VisibilityPublic
	case "private":
		return ast.VisibilityPrivate
	case "protected":
		return ast.VisibilityProtected
	}
	return ast.VisibilityDefault
}

// directionKeyword renders a feature direction, or "" for none.
func directionKeyword(d ast.FeatureDirection) string {
	if d == ast.DirNone {
		return ""
	}
	return d.String()
}
