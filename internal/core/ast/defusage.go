package ast

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// DefinitionKind discriminates the concrete definition taxonomy element.
type DefinitionKind int

const (
	DefPart DefinitionKind = iota
	DefAttribute
	// Tier A — pure keyword-swaps of the part/attribute pattern.
	DefItem
	DefOccurrence
	DefIndividual
	DefMetaclass
	DefMetadata
	DefEnumeration
	DefView
	DefViewpoint
	DefRendering
	DefConcern
	// Tier B — distinctive declaration grammar.
	DefConnection
	DefFlow
	DefPort
	DefInterface
	DefAllocation
	DefBinding
	// Tier C — nested behavioral bodies (generic body this cycle).
	DefAction
	DefState
	DefCalc
	DefConstraint
	DefRequirement
	DefCase
	DefAnalysisCase
	DefVerificationCase
	DefUseCase
	// KerML structural kinds
	DefBehavior
	DefAssoc
	DefStruct
	DefClass
	DefPredicate
	DefBool
)

// definitionKindNames are the notations of the kinds, indexed by kind.
var definitionKindNames = [...]string{
	DefPart:             "part",
	DefAttribute:        "attribute",
	DefItem:             "item",
	DefOccurrence:       "occurrence",
	DefIndividual:       "individual",
	DefMetaclass:        "metaclass",
	DefMetadata:         "metadata",
	DefEnumeration:      "enum",
	DefView:             "view",
	DefViewpoint:        "viewpoint",
	DefRendering:        "rendering",
	DefConcern:          "concern",
	DefConnection:       "connection",
	DefFlow:             "flow",
	DefPort:             "port",
	DefInterface:        "interface",
	DefAllocation:       "allocation",
	DefBinding:          "binding",
	DefAction:           "action",
	DefState:            "state",
	DefCalc:             "calc",
	DefConstraint:       "constraint",
	DefRequirement:      "requirement",
	DefCase:             "case",
	DefAnalysisCase:     "analysis case",
	DefVerificationCase: "verification case",
	DefUseCase:          "use case",
	DefBehavior:         "behavior",
	DefAssoc:            "assoc",
	DefStruct:           "struct",
	DefClass:            "class",
	DefPredicate:        "predicate",
	DefBool:             "bool",
}

func (k DefinitionKind) String() string {
	if int(k) < 0 || int(k) >= len(definitionKindNames) {
		return "unknown"
	}
	return definitionKindNames[k]
}

// PortionKind is the portion an occurrence usage declares of its type
// (OccurrenceUsage::portionKind, SysML v2 8.3.9.11).
type PortionKind int

const (
	PortionNone PortionKind = iota
	PortionSnapshot
	PortionTimeslice
)

// Keyword returns the notation for the portion kind, empty for PortionNone.
func (k PortionKind) Keyword() string {
	switch k {
	case PortionSnapshot:
		return "snapshot"
	case PortionTimeslice:
		return "timeslice"
	}
	return ""
}

// UsageKind discriminates the concrete usage taxonomy element.
type UsageKind int

const (
	UsagePart UsageKind = iota
	UsageAttribute
	// Tier A.
	UsageItem
	UsageOccurrence
	UsageIndividual
	UsageMetadata
	UsageEnumeration
	UsageView
	UsageViewpoint
	UsageRendering
	// UsageViewRendering is the rendering a view body names with `render`
	// (SysML.xtext ViewRenderingMember/ViewRenderingUsage, SysML v2 §8.3.26): a
	// RenderingUsage owned by a ViewRenderingMembership, which either references
	// an existing rendering (`render asTreeDiagram;`) or declares one
	// (`render rendering r : AsTree;`).
	UsageViewRendering
	UsageConcern
	// UsageFramedConcern is the concern a requirement, concern or viewpoint body
	// frames with `frame` (SysML.xtext FramedConcernMember/FramedConcernUsage,
	// SysML v2 §8.3.20): a ConcernUsage owned by a FramedConcernMembership,
	// either referencing an existing concern (`frame 'system breakdown';`) or
	// declaring one (`frame concern c : SafetyConcern;`).
	UsageFramedConcern
	// Tier B.
	UsageConnection
	UsageConnector
	UsageSuccession
	UsageFlow
	UsagePort
	UsageInterface
	UsageInteraction
	UsageAllocation
	UsageBinding
	// Tier C.
	UsageAction
	UsageState
	UsageTransition
	UsageStep
	UsageCalc
	UsageExpr
	UsageConstraint
	UsageRequirement
	UsageSatisfy // satisfy requirement ... by ...
	UsageSubject
	// UsageActor is an actor of a requirement, use case or viewpoint
	// (SysML.xtext ActorMember/ActorUsage, SysML v2 §8.3.19): a PartUsage owned
	// by an ActorMembership. The notation declares it; it has no reference form.
	UsageActor
	// UsageStakeholder is a stakeholder of a requirement, concern or viewpoint
	// (SysML.xtext StakeholderMember/StakeholderUsage, SysML v2 §8.3.19): a
	// PartUsage owned by a StakeholderMembership, declared like an actor.
	UsageStakeholder
	UsageObjective
	UsageCase
	UsageAnalysisCase
	UsageVerificationCase
	UsageUseCase
	// KerML structural kinds
	UsageBehavior
	UsageAssoc
	UsageStruct
	UsageClass
	UsagePredicate
	UsageBool
)

// usageKindNames are the notations of the kinds, indexed by kind.
var usageKindNames = [...]string{
	UsagePart:             "part",
	UsageAttribute:        "attribute",
	UsageItem:             "item",
	UsageOccurrence:       "occurrence",
	UsageIndividual:       "individual",
	UsageMetadata:         "metadata",
	UsageEnumeration:      "enum",
	UsageView:             "view",
	UsageViewpoint:        "viewpoint",
	UsageRendering:        "rendering",
	UsageViewRendering:    "render",
	UsageConcern:          "concern",
	UsageFramedConcern:    "frame",
	UsageConnection:       "connection",
	UsageConnector:        "connector",
	UsageSuccession:       "succession",
	UsageFlow:             "flow",
	UsagePort:             "port",
	UsageInterface:        "interface",
	UsageInteraction:      "interaction",
	UsageAllocation:       "allocation",
	UsageBinding:          "binding",
	UsageAction:           "action",
	UsageState:            "state",
	UsageTransition:       "transition",
	UsageStep:             "step",
	UsageCalc:             "calc",
	UsageExpr:             "expr",
	UsageConstraint:       "constraint",
	UsageRequirement:      "requirement",
	UsageSatisfy:          "satisfy",
	UsageSubject:          "subject",
	UsageActor:            "actor",
	UsageStakeholder:      "stakeholder",
	UsageObjective:        "objective",
	UsageCase:             "case",
	UsageAnalysisCase:     "analysis case",
	UsageVerificationCase: "verification case",
	UsageUseCase:          "use case",
	UsageBehavior:         "behavior",
	UsageAssoc:            "assoc",
	UsageStruct:           "struct",
	UsageClass:            "class",
	UsagePredicate:        "predicate",
	UsageBool:             "bool",
}

func (k UsageKind) String() string {
	if int(k) < 0 || int(k) >= len(usageKindNames) {
		return "unknown"
	}
	return usageKindNames[k]
}

// IsEdge reports whether a usage of the kind is a connector or transition relating
// other members rather than declaring one: a `then` stating no source sequences past it.
func (k UsageKind) IsEdge() bool {
	switch k {
	case UsageSuccession, UsageTransition, UsageConnector, UsageFlow, UsageBinding,
		UsageConnection, UsageInterface, UsageAllocation:
		return true
	}
	return false
}

// RelationshipKind discriminates a specialization/typing edge at a
// definition or usage declaration head.
type RelationshipKind int

const (
	RelTyping      RelationshipKind = iota // ':' / 'defined by'
	RelSpecializes                         // 'specializes' / ':>'
	RelSubsets                             // 'subsets' / ':>'
	RelRedefines                           // 'redefines' / ':>>'
	// RelReferences models a KerML ReferenceSubsetting: the single owned
	// subsetting a feature may syntactically distinguish with 'references' /
	// '::>' (KerML 8.3.3.3.9). The 'via', 'about' and 'by' clauses of accept
	// actions, metadata usages and satisfy usages name related elements
	// without subsetting them, so they carry their own kinds below.
	RelReferences // 'references' / '::>'
	RelCrosses    // 'crosses' / '=>'
	RelDisjoint   // 'disjoint from'
	RelIntersects // 'intersects'
	RelInverseOf  // 'inverse of'
	RelUnions     // 'unions'
	RelChains     // 'chains'
	RelIncludes   // 'includes' (use case inclusion)
	RelVia        // 'via' (accept action receiving port)
	RelAnnotates  // 'about' (metadata annotated element)
	RelSubject    // 'by' (satisfy/verify subject)
	// RelFeaturedBy models a KerML TypeFeaturing written in a feature
	// declaration: `featured by T` states a type the feature is a feature of
	// (KerML.xtext TypeFeaturingPart, 8.3.3.1.4). KerML notation only.
	RelFeaturedBy  // 'featured by'
	RelDifferences // 'differences'
)

// ReferenceSubsets reports whether k is a ReferenceSubsetting: `::>` or a use
// case inclusion, an OwnedReferenceSubsetting in SysML.xtext IncludeUseCaseUsage.
func (k RelationshipKind) ReferenceSubsets() bool {
	return k == RelReferences || k == RelIncludes
}

func (k RelationshipKind) String() string {
	switch k {
	case RelTyping:
		return "typing"
	case RelSpecializes:
		return "specializes"
	case RelSubsets:
		return "subsets"
	case RelRedefines:
		return "redefines"
	case RelReferences:
		return "references"
	case RelCrosses:
		return "crosses"
	case RelDisjoint:
		return "disjoint"
	case RelIntersects:
		return "intersects"
	case RelInverseOf:
		return "inverse"
	case RelUnions:
		return "unions"
	case RelChains:
		return "chains"
	case RelIncludes:
		return "includes"
	case RelVia:
		return "via"
	case RelAnnotates:
		return "about"
	case RelSubject:
		return "by"
	case RelFeaturedBy:
		return "featured by"
	case RelDifferences:
		return "differences"
	default:
		return "unknown"
	}
}

// FeatureDirection is the in/out/inout direction modifier on a usage.
type FeatureDirection int

const (
	DirNone FeatureDirection = iota
	DirIn
	DirOut
	DirInOut
)

func (d FeatureDirection) String() string {
	switch d {
	case DirIn:
		return "in"
	case DirOut:
		return "out"
	case DirInOut:
		return "inout"
	default:
		return "none"
	}
}

// Relationship is one specialization/typing edge: exactly one Target. A
// clause with multiple targets (`specializes A, B`) produces multiple
// Relationship entries sharing the same Kind.
type Relationship struct {
	NodeBase
	Kind   RelationshipKind
	Target Node // QualifiedName or Expression (e.g., FeatureChainExpr for interfacingPorts.incomingTransfers)
	// Conjugated records the `~` of a ConjugatedPortTyping (SysML v2
	// 8.3.12.3): the type is the conjugate of Target, which reverses the
	// directions of the target port definition's features.
	Conjugated bool
	// Multiplicity is the end multiplicity written ahead of a connector end this
	// relationship states: the `[0..1]` of `bind [0..1] a = b` (SysML.xtext:1020).
	Multiplicity *Multiplicity
}

// Multiplicity is a `[n]` / `[lo..hi]` / `[*]` bound on a usage. Bounds are
// expression Nodes (reusing the expression AST); `*` becomes LiteralInfinity.
// For the single-bound form `[n]`, Upper holds n and IsRange is false.
type Multiplicity struct {
	NodeBase
	Lower   Node
	Upper   Node
	IsRange bool
}

// Definition is a `part def` / `attribute def` (and future kinds) node.
type Definition struct {
	NodeBase
	Prefixes []*PrefixMetadata
	Kind     DefinitionKind
	// Keyword is the kind keyword as written, kept for the same reason as
	// Usage.Keyword.
	Keyword string
	// HasDefKeyword records the `def` of a SysML `part def`, which a KerML
	// classifier declaration of the same kind is written without.
	HasDefKeyword bool
	IsAbstract    bool
	IsVariation   bool
	IsAll         bool // 'all' multiplicity propagation modifier
	IsConstant    bool // 'constant' feature modifier
	IsEvent       bool // 'event' modifier for event-driven occurrences
	IsIndividual  bool // 'individual' modifier: `individual part def` (SysML v2 8.3.9.11)
	// IsParallel is the `parallel` before a state body: the state's substates
	// are orthogonal (SysML v2 StateDefinition::isParallel).
	IsParallel    bool
	Visibility    Visibility
	Ident         Identification
	Multiplicity  *Multiplicity
	Relationships []*Relationship
	Members       []Node
	HasBody       bool
}

// Usage is a `part` / `attribute` usage node (and all other usage kinds).
type Usage struct {
	NodeBase
	Prefixes []*PrefixMetadata
	Kind     UsageKind
	// Keyword is the kind keyword as written. Several synonyms map to one Kind
	// (`datatype`, `feature` and `attribute` all give UsageAttribute), so it is
	// kept to tell those spellings apart.
	Keyword string
	// PrefixKeyword is the keyword written ahead of the kind keyword to qualify
	// it rather than to name the declaration: the `assert` of
	// `assert constraint c : C` (SysML.xtext AssertConstraintUsage).
	PrefixKeyword string
	IsAbstract    bool
	IsVariation   bool // 'variation' modifier: the usage is a variation point
	IsVariant     bool // declared with 'variant': a variant of the enclosing variation
	IsReference   bool
	IsVariable    bool // 'var' feature modifier
	IsAll         bool // 'all' multiplicity propagation modifier
	IsEnd         bool // 'end' feature modifier
	IsChain       bool // 'chain' feature modifier
	IsConstant    bool // 'constant' feature modifier
	IsEvent       bool // 'event' modifier for event-driven occurrences
	IsIndividual  bool // 'individual' modifier: OccurrenceUsage::isIndividual
	// Portion is the `snapshot` or `timeslice` prefix of an occurrence usage
	// (OccurrenceUsage::portionKind, SysML v2 8.3.9.11).
	Portion  PortionKind
	IsAccept bool // 'accept' action for message consumption
	// IsBodyParameter marks the `action [<name>] { … }` a loop or branch body is
	// written as (SysML.xtext ActionBodyParameter), not a nested action node.
	IsBodyParameter bool
	// IsActionNode marks an action written as one node with its statement
	// (`action s send x to r;`, SysML.xtext SendNode): Members hold that statement, not a body.
	IsActionNode bool
	// IsTerminate marks a terminate action usage (`action stop terminate;`,
	// SysML.xtext TerminateActionUsage): a token reaching it ends the containing performance.
	IsTerminate bool
	IsResult    bool // declared with 'return': the result parameter of a calculation/expression
	// IsNegated is the `not` of `assert not constraint { … }` and
	// `assert not satisfy … by …`: the conditions are asserted to be false
	// (Invariant::isNegated, SysML v2 §8.3.21.10).
	IsNegated bool
	// DeclaresRequirement is the `requirement` of `satisfy requirement r by v`:
	// the satisfy/verify usage declares its requirement rather than referencing one.
	DeclaresRequirement bool
	Visibility          Visibility
	Direction           FeatureDirection
	IsComposite         bool
	// IsPortion is the `portion` of `portion feature p`: the feature's values
	// are portions of its featuring instances (KerML Feature::isPortion).
	IsPortion bool
	// IsParallel is the `parallel` before a state body (StateUsage::isParallel).
	IsParallel    bool
	IsDerived     bool
	IsOrdered     bool
	IsNonunique   bool
	Ident         Identification
	Relationships []*Relationship
	Multiplicity  *Multiplicity
	// CrossFeature is the cross feature an end declares inline ahead of its own
	// declaration (KerML.xtext OwnedCrossFeatureMember).
	CrossFeature      *CrossFeatureMember
	Value             Node
	ValueOperatorSpan source.Span
	// ValueIsDefault and ValueIsInitial are the `default` and `:=` of the value
	// part (KerML FeatureValue::isDefault, isInitial).
	ValueIsDefault bool
	ValueIsInitial bool
	Members        []Node
	HasBody        bool

	// Tier B connection/flow/port grammar. These are nil/zero for kinds
	// that do not use them.
	ConnectorEnds []*ConnectorEnd // connection / interface / allocation usage ends
	FlowEnds      *FlowEnds       // flow usage ends
}

// IsSuccessionFlow reports whether the usage is a `succession flow`: a flow that
// also orders its ends (SysML v2 SuccessionFlowUsage).
func (u *Usage) IsSuccessionFlow() bool {
	return u.Kind == UsageFlow && u.PrefixKeyword == "succession"
}

// IsPerformedAction reports whether the usage is a `perform action a : A;` or a
// `perform a;`: an action the owner performs (SysML v2 PerformActionUsage).
func (u *Usage) IsPerformedAction() bool {
	return u.Kind == UsageAction && (u.Keyword == "perform" || u.PrefixKeyword == "perform")
}

// IsVariantReference reports a bare `variant x;` (SysML.xtext VariantReference),
// a reference to an existing feature rather than a declaration of a new one.
func (u *Usage) IsVariantReference() bool {
	return u.IsVariant && u.Keyword == "variant"
}

// IsStateAction reports whether the usage is a state's `entry a;`, `do a;` or
// `exit a;`, or the `entry action ...` form of one, each a performed action
// (SysML v2 StateSubactionMembership).
func (u *Usage) IsStateAction() bool {
	if u.Kind != UsageAction {
		return false
	}
	switch u.Keyword {
	case "entry", "do", "exit":
		return true
	}
	switch u.PrefixKeyword {
	case "entry", "do", "exit":
		return true
	}
	return false
}

// IsExhibitedState reports whether the usage is an `exhibit state s : S;` or an
// `exhibit s;`: a state the owner exhibits (SysML v2 ExhibitStateUsage).
func (u *Usage) IsExhibitedState() bool {
	return u.Kind == UsageState && (u.Keyword == "exhibit" || u.Keyword == "exhibit state")
}

// IsIncludedUseCase reports whether the usage is an `include use case u : U;` or
// an `include u;`: a use case the owner includes (SysML v2 IncludeUseCaseUsage).
func (u *Usage) IsIncludedUseCase() bool {
	if u.Kind != UsageUseCase {
		return false
	}
	if u.PrefixKeyword == "include" {
		return true
	}
	for _, rel := range u.Relationships {
		if rel != nil && rel.Kind == RelIncludes {
			return true
		}
	}
	return false
}

// IsVerifiedRequirement reports whether the usage is an objective's `verify r;`
// (SysML v2 RequirementVerificationMembership).
func (u *Usage) IsVerifiedRequirement() bool {
	return u.Kind == UsageSatisfy && u.Keyword == "verify"
}

// IsRequirementConstraint reports whether the usage is a requirement's
// `assume`/`require` constraint (SysML v2 RequirementConstraintMembership).
func (u *Usage) IsRequirementConstraint() bool {
	if u.Kind != UsageConstraint {
		return false
	}
	switch u.Keyword {
	case "require", "assume":
		return true
	}
	return u.PrefixKeyword == "require" || u.PrefixKeyword == "assume"
}

// ReferenceSubsetting returns the usage's OwnedReferenceSubsetting clause, or
// nil. The reference form of satisfy/verify (`satisfy r;`) is parsed as a plain
// subsetting of the requirement it names (SysML.xtext:2119, 2272).
func (u *Usage) ReferenceSubsetting() *Relationship {
	if u == nil {
		return nil
	}
	for _, rel := range u.Relationships {
		if rel == nil || rel.Target == nil {
			continue
		}
		if rel.Kind.ReferenceSubsets() || (rel.Kind == RelSubsets && u.Kind == UsageSatisfy && !u.DeclaresRequirement) {
			return rel
		}
	}
	return nil
}

// IsReferenceSubsetting reports whether rel is decl's OwnedReferenceSubsetting,
// whatever kind the parser recorded it under.
func IsReferenceSubsetting(decl Node, rel *Relationship) bool {
	if rel == nil || rel.Target == nil {
		return false
	}
	if rel.Kind.ReferenceSubsets() {
		return true
	}
	u, ok := decl.(*Usage)
	return ok && u.ReferenceSubsetting() == rel
}

// NamedByReference reports whether an unnamed usage takes its name from the
// feature it references: the members whose membership kind makes the reference
// their naming feature in SysML v2 (perform, exhibit, include, assume/require,
// verify, frame, render, variant), never a plain reference subsetting like
// `assert q;` or `satisfy r;`.
func (u *Usage) NamedByReference() bool {
	if u.IsVariant {
		return true
	}
	switch u.Kind {
	case UsageAction:
		return u.IsPerformedAction() || u.IsStateAction()
	case UsageState:
		return u.IsExhibitedState()
	case UsageUseCase:
		return u.IsIncludedUseCase()
	case UsageConstraint:
		return u.IsRequirementConstraint()
	case UsageSatisfy:
		return u.IsVerifiedRequirement()
	case UsageFramedConcern, UsageViewRendering:
		return true
	}
	return false
}

// HasConjugatedTyping reports whether the usage declares a `: ~P` typing.
func (u *Usage) HasConjugatedTyping() bool {
	_, ok := u.ConjugatedTyping()
	return ok
}

// ConjugatedTyping returns the usage's conjugated port typing (`: ~P`), if any.
func (u *Usage) ConjugatedTyping() (*Relationship, bool) {
	for _, r := range u.Relationships {
		if r != nil && r.Kind == RelTyping && r.Conjugated {
			return r, true
		}
	}
	return nil, false
}

// PerformedInvocation is the call an action usage performs as its value
// (`action a = tag(x);`); nil for any other usage or value.
func (u *Usage) PerformedInvocation() *InvocationExpr {
	if u.Kind != UsageAction {
		return nil
	}
	if inv, ok := u.Value.(*InvocationExpr); ok && inv.Type != nil {
		return inv
	}
	return nil
}

// FlowEnds holds the ends of a flow usage: the `from`/`to` targets and an
// optional payload from the `of` clause. It embeds NodeBase (spannable) but is
// only ever reached through the *ast.Usage traversal case.
type FlowEnds struct {
	NodeBase
	From    Node // Flow source (qualified name or feature chain)
	To      Node // Flow target (qualified name or feature chain)
	Payload Node // optional; from the `of` clause (qualified name or feature chain)
	// PayloadDecl is set when the `of` clause declares the payload feature
	// (`of name : Type`) instead of referring to an existing one. That usage is
	// also a member of the owning flow, and Payload names it.
	PayloadDecl *Usage
	// PayloadMultiplicity is the multiplicity stated with a payload type that
	// declares no feature of its own (`of Publish[1]`, `of [1] Publish`). Where
	// the payload is a declaration the multiplicity is that usage's own.
	PayloadMultiplicity *Multiplicity
}

// CrossFeatureMember is the cross feature declared inline by an end feature,
// the `x1 [0..1]` of `end x1 [0..1] feature x : C1` (KerML 8.3.4.5).
type CrossFeatureMember struct {
	NodeBase
	// The prefix after `end` is the cross feature's (KerML.xtext BasicFeaturePrefix).
	Direction     FeatureDirection
	IsDerived     bool
	IsAbstract    bool
	IsVariation   bool
	IsComposite   bool
	IsPortion     bool
	IsVariable    bool
	IsConstant    bool
	IsReference   bool
	Ident         Identification
	Multiplicity  *Multiplicity
	IsOrdered     bool
	IsNonunique   bool
	Relationships []*Relationship
}

// ConnectorEnd represents a single connector end with optional multiplicity.
type ConnectorEnd struct {
	NodeBase
	Target        Node // QualifiedName or Expression (e.g., FeatureChainExpr for occ.startShot)
	Multiplicity  *Multiplicity
	Reference     Node            // Optional "references X" clause - QualifiedName or FeatureChainExpr
	Relationships []*Relationship // Optional relationships (e.g., ::> for interface binding)
}

// ReferencedTarget returns the node naming the feature the end reference-subsets
// (`references x`, `::> x`), or nil when the end has no such clause.
func (c *ConnectorEnd) ReferencedTarget() Node {
	if c == nil {
		return nil
	}
	for _, rel := range c.Relationships {
		if rel != nil && rel.Kind == RelReferences && rel.Target != nil {
			return rel.Target
		}
	}
	return c.Reference
}

// AttachedTarget returns the node naming the feature this end attaches to: what
// it reference-subsets when it declares a name of its own (`connect bead
// references t.bead`), and the target it names otherwise (`connect a.p to b.q`).
func (c *ConnectorEnd) AttachedTarget() Node {
	if c == nil {
		return nil
	}
	if _, declaresName := c.DeclaredName(); declaresName {
		return c.ReferencedTarget()
	}
	if c.Target != nil {
		return c.Target
	}
	return c.Reference
}

// SplitRedefinitions partitions rels into the redefinitions and the rest,
// which resolve in different scopes when owned by a connector end.
func SplitRedefinitions(rels []*Relationship) (redefines, others []*Relationship) {
	for _, rel := range rels {
		if rel != nil && rel.Kind == RelRedefines {
			redefines = append(redefines, rel)
			continue
		}
		others = append(others, rel)
	}
	return redefines, others
}

// DeclaredName returns the name the end declares for itself, and whether it
// declares one at all. A connector end names an end feature of the connector
// only when it also reference-subsets the feature that end attaches to
// (`connect bead references t.bead`, `connect supplierPort ::> a.p`); without
// that clause the end names an existing feature instead (`connect a to b`).
func (c *ConnectorEnd) DeclaredName() (Identification, bool) {
	if c == nil || c.ReferencedTarget() == nil {
		return Identification{}, false
	}
	qname := AsQualifiedName(c.Target)
	if qname == nil || len(qname.Parts) != 1 {
		return Identification{}, false
	}
	part := qname.Parts[0]
	return Identification{Name: part.Text, NameSpan: part.Span}, true
}
