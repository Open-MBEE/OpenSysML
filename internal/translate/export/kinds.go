package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
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
	ast.DefIndividual:       "OccurrenceDefinition",
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
	ast.UsagePart:          "PartUsage",
	ast.UsageAttribute:     "AttributeUsage",
	ast.UsageItem:          "ItemUsage",
	ast.UsageOccurrence:    "OccurrenceUsage",
	ast.UsageIndividual:    "OccurrenceUsage",
	ast.UsageMetadata:      "MetadataUsage",
	ast.UsageEnumeration:   "EnumerationUsage",
	ast.UsageView:          "ViewUsage",
	ast.UsageViewpoint:     "ViewpointUsage",
	ast.UsageRendering:     "RenderingUsage",
	ast.UsageViewRendering: "RenderingUsage",
	ast.UsageConcern:       "ConcernUsage",
	ast.UsageFramedConcern: "ConcernUsage",
	ast.UsageConnection:    "ConnectionUsage",
	ast.UsageConnector:     "Connector",
	ast.UsageSuccession:    "SuccessionAsUsage",
	ast.UsageFlow:          "FlowUsage",
	ast.UsagePort:          mPortUsage,
	ast.UsageInterface:     "InterfaceUsage",
	ast.UsageAllocation:    "AllocationUsage",
	ast.UsageBinding:       "BindingConnectorAsUsage",
	ast.UsageAction:        "ActionUsage",
	ast.UsageState:         "StateUsage",
	ast.UsageTransition:    "TransitionUsage",
	ast.UsageStep:          "Step",
	ast.UsageCalc:          "CalculationUsage",
	ast.UsageExpr:          "Expression",
	ast.UsageConstraint:    "ConstraintUsage",
	ast.UsageRequirement:   "RequirementUsage",
	ast.UsageSatisfy:       "SatisfyRequirementUsage",
	// The parameter a subject/actor/stakeholder/objective member declares is the
	// usage the membership owns (SysML.xtext SubjectMember & co.); the
	// membership metaclasses older graphs typed the element by are read in
	// metaclassKeywordUsage.
	ast.UsageSubject:          "ReferenceUsage",
	ast.UsageActor:            "PartUsage",
	ast.UsageStakeholder:      "PartUsage",
	ast.UsageObjective:        "RequirementUsage",
	ast.UsageCase:             "CaseUsage",
	ast.UsageAnalysisCase:     "AnalysisCaseUsage",
	ast.UsageVerificationCase: "VerificationCaseUsage",
	ast.UsageUseCase:          "UseCaseUsage",
	ast.UsageBool:             "BooleanUsage",
}

// kermlTypeUsageMetaclass names the metaclass of each usage kind the parser reads a KerML
// type declaration under, for a node whose keyword does not say (ast.KerMLTypeMetaclass).
var kermlTypeUsageMetaclass = map[ast.UsageKind]string{
	ast.UsageClass:       "Class",
	ast.UsageStruct:      "Structure",
	ast.UsageAssoc:       "Association",
	ast.UsageBehavior:    "Behavior",
	ast.UsagePredicate:   "Predicate",
	ast.UsageInteraction: "Interaction",
}

// legacyKerMLTypeMetaclasses maps the names earlier releases typed a KerML type declaration
// with to the metaclass it builds; a graph typed so is read, and none is written.
var legacyKerMLTypeMetaclasses = metamodel.LegacyKerMLTypeMetaclasses()

// legacyKerMLTypeMetaclass is the metaclass of an element typed with a legacy KerML type
// name: the one its recorded keyword builds (`type`, `assoc struct`), else the name's own.
func legacyKerMLTypeMetaclass(metaclass, keyword string) (string, bool) {
	standard, legacy := legacyKerMLTypeMetaclasses[metaclass]
	if !legacy {
		return "", false
	}
	if declared := ast.KerMLTypeMetaclass(keyword); declared != "" {
		return declared, true
	}
	return standard, true
}

// isType reports whether a metaclass is a Type, a legacy KerML type name included.
func isType(metaclass string) bool {
	if standard, legacy := legacyKerMLTypeMetaclasses[metaclass]; legacy {
		metaclass = standard
	}
	return ontology.IsAncestorOrSelf(metaclass, "Type")
}

// keywordMetaclass names the metaclass a keyword builds where several spellings
// share one AST kind (SysML.xtext:632).
var keywordMetaclass = map[string]string{
	"ref": "ReferenceUsage",
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
	mEventOccurrenceUsage    = "EventOccurrenceUsage"
	mAssertConstraintUsage   = "AssertConstraintUsage"
	mExhibitStateUsage       = "ExhibitStateUsage"
	mIncludeUseCaseUsage     = "IncludeUseCaseUsage"
	mSatisfyRequirementUsage = "SatisfyRequirementUsage"
	mPortUsage               = "PortUsage"
)

// usageQualifier maps the metaclass a qualified-usage keyword types to that
// keyword (SysML.xtext PerformActionUsage, ExhibitStateUsage,
// IncludeUseCaseUsage, AssertConstraintUsage, SatisfyRequirementUsage).
var usageQualifier = map[string]string{
	mPerform:                 "perform",
	mExhibitStateUsage:       "exhibit",
	mIncludeUseCaseUsage:     "include",
	mAssertConstraintUsage:   "assert",
	mSatisfyRequirementUsage: "satisfy",
}

// qualifierMetaclass reverses usageQualifier, for reporting the metaclass a
// recorded qualifier keyword contradicts.
var qualifierMetaclass = map[string]string{}

func init() {
	for metaclass, qualifier := range usageQualifier {
		qualifierMetaclass[qualifier] = metaclass
	}
}

// declaredMetaclass gives the metaclass a declaration outside any metadata body
// builds, or "" for a node that declares no element of its own.
func declaredMetaclass(decl ast.Node) string {
	switch n := decl.(type) {
	case *ast.Namespace:
		return "Namespace"
	case *ast.Package:
		if n.IsLibrary {
			return mLibraryPackage
		}
		return mPackage
	case *ast.Definition:
		m, _ := definitionMetaclassOf(n)
		return m
	case *ast.Usage:
		m, _ := usageMetaclassOf(n, false)
		return m
	}
	return ""
}

// definitionMetaclassOf gives the metaclass a definition builds: a KerML type declaration
// written without `def` takes its keyword's (`classifier X` is a Classifier).
func definitionMetaclassOf(n *ast.Definition) (string, bool) {
	if m := ast.KerMLTypeMetaclass(n.Keyword); m != "" && !n.HasDefKeyword {
		return m, true
	}
	m, ok := definitionMetaclass[n.Kind]
	return m, ok
}

// kermlTypeKeywordRecorded is the keyword a def-less KerML type declaration always records:
// it shares its metaclass with the `def` form (`class def X`), which records none.
func kermlTypeKeywordRecorded(metaclass, keyword string, hasDef bool) (string, bool) {
	if _, kermlType := kermlTypeUsageKind[metaclass]; !kermlType || hasDef {
		return "", false
	}
	if ast.KerMLTypeMetaclass(keyword) == "" {
		keyword = ast.KerMLTypeKeyword(metaclass)
	}
	return keyword, true
}

// classifierDeclaration reports whether node declares a Classifier, whose `specializes` is a
// Subclassification: the identity reader's form of relationshipSpec's test on the written type.
func classifierDeclaration(node ast.Node) bool {
	return ontology.IsAncestorOrSelf(declaredMetaclass(node), "Classifier")
}

// usageMetaclassOf gives the metaclass a usage builds, reading the keyword where
// the kind does not decide it; a kindless one is a DefaultReferenceUsage, as is
// a kindless `name = value;` in a metadata body (SysML.xtext MetadataBodyUsage).
func usageMetaclassOf(n *ast.Usage, inMetadataBody bool) (string, bool) {
	if m := ast.KerMLTypeMetaclass(n.Keyword); m != "" {
		return m, true
	}
	if m, ok := keywordMetaclass[n.Keyword]; ok {
		return m, true
	}
	switch {
	case eventOccurrence(n):
		return mEventOccurrenceUsage, true
	case assertedConstraint(n):
		return mAssertConstraintUsage, true
	case n.IsTerminate && n.Kind == ast.UsageAction:
		return mTerminate, true
	case n.IsPerformedAction():
		return mPerform, true
	case n.IsExhibitedState():
		return mExhibitStateUsage, true
	case n.IsIncludedUseCase():
		return mIncludeUseCaseUsage, true
	case acceptPayload(n) != nil:
		return mAcceptAction, true
	}
	switch s := naming.NodeStatement(n).(type) {
	case *ast.SendStatement:
		return mSend, true
	case *ast.AssignmentActionNode:
		return mAssignment, true
	case *ast.IfActionNode:
		return mIfAction, true
	case *ast.WhileLoopActionNode:
		if s.Kind == ast.LoopFor {
			return mForLoop, true
		}
		return mWhileLoop, true
	}
	if n.IsSuccessionFlow() {
		return mSuccessionFlowUsage, true
	}
	if n.IsExtendedUsage() {
		// Prefix metadata in place of a kind keyword declares a plain Usage
		// (SysML.xtext ExtendedUsage), not a reference (SysML v2 8.3.6.10).
		return mUsage, true
	}
	if n.Keyword == "" && (n.Kind == ast.UsageAttribute || inMetadataBody && n.Kind == ast.UsageEnumeration) {
		// A kindless usage is a DefaultReferenceUsage; a name decides nothing, the
		// prefix does (SysML.xtext IndividualUsage, PortionUsage, EnumeratedValue).
		return "ReferenceUsage", true
	}
	if m, ok := kermlTypeUsageMetaclass[n.Kind]; ok {
		return m, true
	}
	m, ok := usageMetaclass[n.Kind]
	return m, ok
}

// acceptPayload is the payload parameter of an accept node
// (`action a accept p : T;`, `accept T;`), whose action is an
// AcceptActionUsage (SysML.xtext AcceptNode), or nil for any other usage.
func acceptPayload(n *ast.Usage) *ast.Usage {
	if n.Kind != ast.UsageAction || len(n.Members) == 0 {
		return nil
	}
	member := n.Members[0]
	if m, ok := member.(*ast.Membership); ok {
		member = m.Member
	}
	if payload, ok := member.(*ast.Usage); ok && payload.IsAccept {
		return payload
	}
	return nil
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

// legacyConnectorAsUsage is the abstract metaclass older graphs typed a
// `connector` with; it is read, never written.
const legacyConnectorAsUsage = "ConnectorAsUsage"

// metaclassKeywordUsage reads the keyword-decided metaclasses back to the kind
// the parser records for them, and legacy metaclasses to the kind they meant.
var metaclassKeywordUsage = map[string]ast.UsageKind{
	legacyConnectorAsUsage: ast.UsageConnector,
	"DataType":             ast.UsageAttribute,
	"Function":             ast.UsageCalc,
	"ReferenceUsage":       ast.UsageAttribute,
	mUsage:                 ast.UsageAttribute,
	mEventOccurrenceUsage:  ast.UsageOccurrence,
	mAssertConstraintUsage: ast.UsageConstraint,
	mTerminate:             ast.UsageAction,
	mPerform:               ast.UsageAction,
	mAcceptAction:          ast.UsageAction, // an accept node, its payload the accept parameter
	mSuccessionFlowUsage:   ast.UsageFlow,
	mExhibitStateUsage:     ast.UsageState,
	mIncludeUseCaseUsage:   ast.UsageUseCase,
	// PartUsage types actor and stakeholder members too, RequirementUsage an
	// objective member; the requirement is the keyword each metaclass alone
	// spells, the others come from the membership's metaclass.
	"PartUsage":        ast.UsagePart,
	"RequirementUsage": ast.UsageRequirement,
	// RenderingUsage and ConcernUsage type a `render`/`frame` member too
	// (SysML.xtext ViewRenderingUsage, FramedConcernUsage); its membership says which.
	"RenderingUsage": ast.UsageRendering,
	"ConcernUsage":   ast.UsageConcern,
	// The membership metaclasses graphs before the parameter members were
	// materialized typed the element itself with; read, never written.
	"SubjectMembership":       ast.UsageSubject,
	"ActorMembership":         ast.UsageActor,
	"StakeholderMembership":   ast.UsageStakeholder,
	"ObjectiveMembership":     ast.UsageObjective,
	"ViewRenderingMembership": ast.UsageViewRendering,
	"FramedConcernMembership": ast.UsageFramedConcern,
}

// definitionKeyword and usageKeyword give the source keyword for a kind, which
// is what makes the printer able to reconstruct a declaration head from the
// metaclass alone. The AST's String() is the keyword for every kind but the
// cases the grammar spells `analysis def`/`analysis`/`verification def`/`verification`.
func definitionKeyword(kind ast.DefinitionKind) string {
	switch kind {
	case ast.DefAnalysisCase:
		return "analysis"
	case ast.DefVerificationCase:
		return "verification"
	}
	return kind.String()
}

// metaclassDefinitionKeyword is the keyword the decoder writes for kind's metaclass:
// `individual def` is an OccurrenceDefinition, so its own keyword is a written one.
func metaclassDefinitionKeyword(kind ast.DefinitionKind) string {
	if kind == ast.DefIndividual {
		return definitionKeyword(ast.DefOccurrence)
	}
	return definitionKeyword(kind)
}

// metaclassUsageKeyword is metaclassDefinitionKeyword for a usage kind.
func metaclassUsageKeyword(kind ast.UsageKind) string {
	if kind == ast.UsageIndividual {
		return usageKeyword(ast.UsageOccurrence)
	}
	return usageKeyword(kind)
}

func usageKeyword(kind ast.UsageKind) string {
	switch kind {
	case ast.UsageAnalysisCase:
		return "analysis"
	case ast.UsageVerificationCase:
		return "verification"
	}
	return kind.String()
}

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
	// kermlTypeUsageKind is the usage kind the parser reads a KerML type declaration of a
	// metaclass under, whose head a KerML document writes without `def`.
	kermlTypeUsageKind = map[string]ast.UsageKind{"Type": ast.UsageClass, "AssociationStructure": ast.UsageAssoc}
)

func init() {
	// `individual` is an OccurrenceDefinition/OccurrenceUsage with isIndividual (SysML.xtext
	// IndividualDefinition, IndividualUsage); the metaclass reads back as the occurrence kind.
	for kind, name := range definitionMetaclass {
		if kind != ast.DefIndividual {
			metaclassDefinition[name] = kind
		}
	}
	for kind, name := range usageMetaclass {
		if kind != ast.UsageIndividual {
			metaclassUsage[name] = kind
		}
	}
	for name, kind := range metaclassKeywordUsage {
		metaclassUsage[name] = kind
	}
	for kind, name := range kermlTypeUsageMetaclass {
		kermlTypeUsageKind[name] = kind
	}
	for legacy, standard := range legacyKerMLTypeMetaclasses {
		metaclassUsage[legacy] = kermlTypeUsageKind[standard]
	}
	// Turtle this conversion wrote before it typed them so.
	metaclassDefinition["IndividualDefinition"] = ast.DefIndividual
	// A def-less `classifier` is the Classifier the parser reads under the class kind.
	metaclassDefinition[mClassifier] = ast.DefClass
	metaclassUsage["IndividualUsage"] = ast.UsageIndividual
}

// legacyMetaclasses maps the metaclass names earlier releases wrote for an `individual`
// to the standard ones; a graph typed so is read as the standard metaclass with isIndividual.
var legacyMetaclasses = map[string]string{
	"IndividualDefinition": "OccurrenceDefinition",
	"IndividualUsage":      "OccurrenceUsage",
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
