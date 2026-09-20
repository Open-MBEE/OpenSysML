package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Standard-library names the implicit-base tables state more than once.
const (
	partFQN         = "Parts::Part"
	anythingFQN     = "Base::Anything"
	dataValueFQN    = "Base::DataValue"
	occurrenceFQN   = "Occurrences::Occurrence"
	renderingFQN    = "Views::Rendering"
	concernCheckFQN = "Requirements::ConcernCheck"
	linksFQN        = "Links::links"
	linkFQN         = "Links::Link"
	performanceFQN  = "Performances::Performance"
	assocStructKw   = "assoc struct"
)

// implicitUsageBases maps each usage kind to the standard-library base feature
// specified by SysML v2 §7 and the corresponding library package.
var implicitUsageBases = map[ast.UsageKind]string{
	ast.UsagePart:             "Parts::parts",
	ast.UsageAttribute:        "Base::dataValues",
	ast.UsageEnumeration:      "Base::dataValues",
	ast.UsageItem:             "Items::items",
	ast.UsageOccurrence:       "Occurrences::occurrences",
	ast.UsageIndividual:       "Occurrences::Life",
	ast.UsageMetadata:         "Metadata::metadataItems",
	ast.UsageView:             "Views::views",
	ast.UsageViewpoint:        "Views::viewpointChecks",
	ast.UsageRendering:        "Views::renderings",
	ast.UsageViewRendering:    "Views::renderings",
	ast.UsageConcern:          "Requirements::concernChecks",
	ast.UsageFramedConcern:    "Requirements::concernChecks",
	ast.UsageActor:            "Parts::parts",
	ast.UsageStakeholder:      "Parts::parts",
	ast.UsageConnection:       "Connections::connections",
	ast.UsagePort:             "Ports::ports",
	ast.UsageInterface:        "Interfaces::interfaces",
	ast.UsageAllocation:       "Allocations::allocations",
	ast.UsageAction:           "Actions::actions",
	ast.UsageState:            "States::stateActions",
	ast.UsageTransition:       "Actions::transitionActions",
	ast.UsageStep:             "Performances::performances",
	ast.UsageCalc:             "Calculations::calculations",
	ast.UsageExpr:             "Performances::evaluations",
	ast.UsageConstraint:       "Constraints::constraintChecks",
	ast.UsageRequirement:      "Requirements::requirementChecks",
	ast.UsageCase:             "Cases::cases",
	ast.UsageAnalysisCase:     "AnalysisCases::analysisCases",
	ast.UsageVerificationCase: "VerificationCases::verificationCases",
	ast.UsageUseCase:          "UseCases::useCases",
}

// implicitDefinitionBases maps a definition kind to the qualified name of the
// standard library definition every definition of that kind implicitly
// specializes, so the members that base supplies resolve inside the
// definition's body: MetadataItem supplies `annotatedElement` from
// Metaobjects::Metaobject (SysML v2 §7.27.2, [KerML, 9.2.17]), and Parts::Part
// supplies `start` and `done` inside every `part def`.
var implicitDefinitionBases = map[ast.DefinitionKind]string{
	ast.DefPart:        partFQN,
	ast.DefAttribute:   dataValueFQN,
	ast.DefEnumeration: dataValueFQN,
	ast.DefItem:        "Items::Item",
	ast.DefOccurrence:  occurrenceFQN,
	ast.DefIndividual:  "Occurrences::Life",
	ast.DefMetaclass:   "Metaobjects::Metaobject",
	ast.DefMetadata:    "Metadata::MetadataItem",
	ast.DefView:        "Views::View",
	ast.DefViewpoint:   "Views::ViewpointCheck",
	ast.DefRendering:   renderingFQN,
	ast.DefConcern:     concernCheckFQN,
	ast.DefConnection:  "Connections::Connection",
	ast.DefFlow:        "Flows::MessageAction",
	ast.DefPort:        "Ports::Port",
	ast.DefInterface:   "Interfaces::Interface",
	ast.DefAllocation:  "Allocations::Allocation",
	// A behavior definition specializes the base behavior of its kind, which is
	// what makes an occurrence's own features — `self`, `start`, `done` — visible
	// inside the definition's body the same way they are inside a usage's
	// (SysML v2 §7.16.2, §7.17.2).
	ast.DefAction:           "Actions::Action",
	ast.DefState:            "States::StateAction",
	ast.DefCalc:             "Calculations::Calculation",
	ast.DefConstraint:       "Constraints::ConstraintCheck",
	ast.DefRequirement:      "Requirements::RequirementCheck",
	ast.DefCase:             "Cases::Case",
	ast.DefAnalysisCase:     "AnalysisCases::AnalysisCase",
	ast.DefVerificationCase: "VerificationCases::VerificationCase",
	ast.DefUseCase:          "UseCases::UseCase",
}

// implicitKerMLBinaryBases maps a KerML association keyword to the base a
// declaration with exactly two ends specializes (KerML 1.1 §8.3.3.5): the binary
// link rather than the n-ary one of its kind.
var implicitKerMLBinaryBases = map[string]string{
	"assoc":       binaryConnectorBaseFQN,
	"association": binaryConnectorBaseFQN,
	assocStructKw: "Objects::BinaryLinkObject",
	"interaction": binaryConnectorBaseFQN,
}

// implicitKerMLBinaryFeatureBases maps a KerML connector keyword to the base
// feature a connector with exactly two ends subsets (KerML 1.1 §8.3.4.7).
var implicitKerMLBinaryFeatureBases = map[string]string{
	"connector": "Links::binaryLinks",
}

// implicitKerMLBehaviorBases is the behavior base a kind that is also an
// association specializes: an interaction is a Performance and a Link (KerML 1.1 §7.4.10.2).
var implicitKerMLBehaviorBases = map[string]string{
	"interaction": performanceFQN,
}

var implicitKerMLBases = map[string]string{
	"classifier":  anythingFQN,
	"class":       occurrenceFQN,
	"struct":      "Objects::Object",
	"assoc":       linkFQN,
	"association": linkFQN,
	assocStructKw: "Objects::LinkObject",
	"behavior":    performanceFQN,
	"function":    "Performances::Evaluation",
	"predicate":   "Performances::BooleanEvaluation",
	"interaction": linkFQN,
	"metaclass":   "Metaobjects::Metaobject",
	"datatype":    dataValueFQN,
	"type":        anythingFQN,
}

// implicitKerMLFeatureBases maps a KerML feature keyword to the base feature
// every feature of that kind subsets (KerML 1.1 §8.4.2).
var implicitKerMLFeatureBases = map[string]string{
	"":            baseUsageFQN, // a member declared with no kind keyword (`end a;`) is a feature
	"type":        baseUsageFQN,
	"classifier":  baseUsageFQN,
	"feature":     baseUsageFQN,
	"class":       "Occurrences::occurrences",
	"struct":      "Objects::objects",
	"datatype":    "Base::dataValues",
	"assoc":       linksFQN,
	"association": linksFQN,
	assocStructKw: "Objects::linkObjects",
	"connector":   linksFQN,
	"binding":     "Links::selfLinks",
	"bind":        "Links::selfLinks",
	"succession":  "Occurrences::happensBeforeLinks",
	"behavior":    "Performances::performances",
	"step":        "Performances::performances",
	"function":    "Performances::evaluations",
	"expr":        "Performances::evaluations",
	"predicate":   "Performances::booleanEvaluations",
	"bool":        "Performances::booleanEvaluations",
	"inv":         "Performances::trueEvaluations",
	"interaction": "Transfers::transfers",
	"flow":        "Transfers::flowTransfers",
	"metaclass":   "Metaobjects::metaobjects",
}

// KindBaseFQNs returns the standard-library bases every declaration of sym's
// kind conforms to, implicitly or through its declared chain: one for most
// kinds, an association and a behavior base for an interaction.
func (m *Model) KindBaseFQNs(sym *symbols.Symbol, isKerML bool) []string {
	return m.kindBaseDefinitionFQNs(sym, isKerML)
}

// DeclaresKerMLClassifier reports whether sym is a KerML classifier declaration
// the symbol table records as a usage, such as `datatype D;` or `function F;`.
func (m *Model) DeclaresKerMLClassifier(sym *symbols.Symbol) bool {
	if sym == nil || !m.isKerMLDoc(sym) {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	_, ok = implicitKerMLBases[usage.Keyword]
	return ok
}

// FeatureBaseFQN returns the standard-library base feature a feature declaration
// or usage kind subsets when it declares none.
func (m *Model) FeatureBaseFQN(sym *symbols.Symbol) (string, bool) {
	if sym == nil {
		return "", false
	}
	if typed, ok := m.declaredTypeFeatureBase(sym); ok {
		return typed, true
	}
	if !m.isKerMLDoc(sym) {
		usage, ok := sym.Decl.(*ast.Usage)
		if !ok {
			return "", false
		}
		if fqn, ok := implicitUsageBases[usage.Kind]; ok {
			return fqn, true
		}
		// A usage of no particular kind still subsets the base feature every
		// usage does, which is what types it (SysML v2 §7.3.2).
		return baseUsageFQN, true
	}
	return m.kermlFeatureBaseFQN(sym)
}

// kermlFeatureBaseFQN returns the base feature a KerML feature declaration
// subsets by its keyword, taking the binary base when it has two ends.
func (m *Model) kermlFeatureBaseFQN(sym *symbols.Symbol) (string, bool) {
	keyword := keywordOf(sym)
	if fqn, ok := implicitKerMLBinaryFeatureBases[keyword]; ok && m.declaredEndCount(sym) == 2 {
		return fqn, true
	}
	fqn, ok := implicitKerMLFeatureBases[keyword]
	return fqn, ok
}

// RelationshipTarget resolves the element rel names from sym's scope.
func (m *Model) RelationshipTarget(sym *symbols.Symbol, rel *ast.Relationship) *symbols.Symbol {
	return m.relationshipTarget(sym, rel)
}

func (m *Model) kindBaseFQNs(sym *symbols.Symbol, isKerML bool) []string {
	fqn, ok := m.kindBaseFQN(sym, isKerML)
	if !ok {
		return nil
	}
	out := []string{fqn}
	if isKerML {
		if behavior, ok := implicitKerMLBehaviorBases[keywordOf(sym)]; ok {
			out = append(out, behavior)
		}
	}
	return out
}

// kindBaseDefinitionFQNs returns definition qualified names for sym's implicit
// kind bases, deriving definitions from feature bases when necessary.
func (m *Model) kindBaseDefinitionFQNs(sym *symbols.Symbol, isKerML bool) []string {
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	var out []string
	for _, fqn := range m.kindBaseFQNs(sym, isKerML) {
		for _, base := range m.resolver.Index().LookupQualified(fqn) {
			if base == nil {
				continue
			}
			if base.IsFeature() {
				for _, typ := range m.baseFeatureTypes(base, nil) {
					if typ != nil {
						out = append(out, m.resolver.Index().GetFQN(typ))
					}
				}
			} else {
				out = append(out, fqn)
			}
			break
		}
	}
	return out
}

// kindBaseFQN returns the base a declaration of sym's kind specializes for the
// kind itself; kindBaseFQNs adds the further bases a kind with two facets has.
// A KerML association is binary by its effective ends, a SysML connection,
// interface or flow by its owned ones (KerML 1.1 §7.4.8, SysML v2 §7.13.2, §7.14.2).
func (m *Model) kindBaseFQN(sym *symbols.Symbol, isKerML bool) (string, bool) {
	if sym == nil {
		return "", false
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		if isKerML {
			if fqn, ok := implicitKerMLBinaryBases[d.Keyword]; ok && m.declaredEndCount(sym) == 2 {
				return fqn, true
			}
			fqn, ok := implicitKerMLBases[d.Keyword]
			return fqn, ok
		}
		// A usage with no kind keyword (`ref x`, `x;`, `#M x`) is a reference or plain
		// usage typed by Anything, not an attribute's DataValue (SysML v2 §7.6.2).
		if d.Keyword == "" && d.Kind == ast.UsageAttribute {
			return anythingFQN, true
		}
		if len(ownedEnds(sym)) == 2 {
			switch d.Kind {
			case ast.UsageConnection:
				return "Connections::binaryConnections", true
			case ast.UsageInterface:
				return "Interfaces::binaryInterfaces", true
			}
		}
		if d.IsIndividual && d.Kind == ast.UsageOccurrence {
			// An individual occurrence is a life, not an arbitrary occurrence
			// (SysML v2 §7.9.4), however the modifier is spelled.
			fqn, ok := implicitUsageBases[ast.UsageIndividual]
			return fqn, ok
		}
		fqn, ok := implicitUsageBases[d.Kind]
		return fqn, ok
	case *ast.Definition:
		if isKerML {
			if fqn, ok := implicitKerMLBinaryBases[d.Keyword]; ok && m.declaredEndCount(sym) == 2 {
				return fqn, true
			}
			fqn, ok := implicitKerMLBases[d.Keyword]
			return fqn, ok
		}
		if len(ownedEnds(sym)) == 2 {
			switch d.Kind {
			case ast.DefConnection:
				return "Connections::BinaryConnection", true
			case ast.DefInterface:
				return "Interfaces::BinaryInterface", true
			case ast.DefFlow:
				return "Flows::Message", true
			}
		}
		fqn, ok := implicitDefinitionBases[d.Kind]
		return fqn, ok
	case *ast.SubstateMember:
		// `state s;` in a state body: a bodyless, always-untyped state usage.
		fqn, ok := implicitUsageBases[ast.UsageState]
		return fqn, ok
	case *ast.TransitionMember:
		// A textual transition is a TransitionUsage (SysML v2 §7.19.2), so it
		// gets the same base as `transition` written as a usage.
		fqn, ok := implicitUsageBases[ast.UsageTransition]
		return fqn, ok
	case *ast.AssumeMember, *ast.RequireMember:
		// An assume/require member owns a ConstraintUsage (SysML v2
		// RequirementConstraintUsage), so it takes a constraint's base.
		fqn, ok := implicitUsageBases[ast.UsageConstraint]
		return fqn, ok
	// A control node is the action usage of its ControlAction (SysML v2 §8.3.17).
	case *ast.ForkNode:
		return "Actions::ForkAction", true
	case *ast.JoinNode:
		return "Actions::JoinAction", true
	case *ast.MergeNode:
		return "Actions::MergeAction", true
	case *ast.DecisionNode:
		return "Actions::DecisionAction", true
	}
	return "", false
}

// isKerMLDoc reports whether sym is declared by a KerML document, as recorded
// by the index rather than inferred from the document name.
func (m *Model) isKerMLDoc(sym *symbols.Symbol) bool {
	if m.resolver == nil || m.resolver.Index() == nil {
		return false
	}
	return m.resolver.Index().DocumentKind(sym.DocName) == source.KindKerML
}

// implicitBases returns the stdlib definitions sym is implicitly typed by, or nil when sym
// is not a declaration of a kind with a known base; memoized once the chains it reads settle.
func (m *Model) implicitBases(sym *symbols.Symbol) []*symbols.Symbol {
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.implicitBase[sym]; ok {
		return cached
	}
	m.resolver.Enter()
	out := m.computeImplicitBases(sym)
	if m.resolver.Leave() {
		journal(m, m.implicitBase, sym, sym.Decl)
		m.implicitBase[sym] = out
	}
	return out
}

// computeImplicitBases derives implicitBases' answer.
func (m *Model) computeImplicitBases(sym *symbols.Symbol) []*symbols.Symbol {
	// A conjugated type takes its supertypes from what it conjugates rather than
	// from an implicit specialization of its kind's base (KerML §8.3.3.1.1).
	if declaresConjugation(sym) {
		return nil
	}
	var out []*symbols.Symbol
	for _, fqn := range m.kindBaseDefinitionFQNs(sym, m.isKerMLDoc(sym)) {
		// A declaration keeps its kind's base unless a declared chain already
		// reaches it — the same rule for a usage and in either language (KerML §8.4.2).
		if m.declaredGeneralizationReaches(sym, fqn, nil) {
			continue
		}
		for _, base := range m.resolver.Index().LookupQualified(fqn) {
			if base != nil && base != sym {
				out = append(out, base)
				break
			}
		}
	}
	return out
}

// baseFeatureTypes returns the definition types supplied by a feature and all
// feature bases it subsets, retaining first-seen most-specific types.
func (m *Model) baseFeatureTypes(base *symbols.Symbol, visiting map[*symbols.Symbol]bool) []*symbols.Symbol {
	if base == nil {
		return nil
	}
	if visiting == nil {
		visiting = make(map[*symbols.Symbol]bool)
	}
	if visiting[base] {
		return nil
	}
	visiting[base] = true
	defer delete(visiting, base)

	var types, features []*symbols.Symbol
	for _, sup := range m.DirectSupertypes(base) {
		if sup.IsFeature() {
			features = append(features, sup)
		} else {
			types = append(types, sup)
		}
	}
	for _, feature := range features {
		types = append(types, m.baseFeatureTypes(feature, visiting)...)
	}

	var out []*symbols.Symbol
	for _, typ := range types {
		if typ == nil || slices.Contains(out, typ) {
			continue
		}
		out = append(out, typ)
	}
	pruned := out[:0]
	for i, typ := range out {
		redundant := false
		for j, other := range out {
			if i == j {
				continue
			}
			if m.Conforms(other, typ) && !m.Conforms(typ, other) {
				redundant = true
				break
			}
		}
		if !redundant {
			pruned = append(pruned, typ)
		}
	}
	return pruned
}

// declaresConjugation reports whether sym conjugates a type.
func declaresConjugation(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Conjugated {
			return true
		}
	}
	return false
}

func (m *Model) declaredGeneralizationReaches(sym *symbols.Symbol, want string, visiting map[*symbols.Symbol]bool) bool {
	if sym == nil {
		return false
	}
	if visiting == nil {
		visiting = make(map[*symbols.Symbol]bool)
	}
	if visiting[sym] {
		return false
	}
	visiting[sym] = true
	defer delete(visiting, sym)

	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		target := m.relationshipTarget(sym, rel)
		// A declaration on the path back to itself is a cycle, not a path to
		// the base, so it does not displace the implicit one.
		if target == nil || visiting[target] {
			continue
		}
		sameBase := m.resolver.Index() != nil && symbols.HasFQN(target, want)
		if !sameBase && !visiting[target] {
			// A declaration conforms to its kind's base whether the edge is
			// declared or implicit, so reaching one of the same kind suffices —
			// except back through a cycle, which reaches nothing new, or a
			// conjugated one, whose supertypes come from what it conjugates.
			if (slices.Contains(m.kindBaseFQNs(target, m.isKerMLDoc(target)), want) ||
				slices.Contains(m.kindBaseDefinitionFQNs(target, m.isKerMLDoc(target)), want)) &&
				!declaresConjugation(target) && !m.declaredReaches(target, sym, nil) {
				sameBase = true
			}
		}
		if sameBase || m.declaredGeneralizationReaches(target, want, visiting) {
			return true
		}
	}
	return false
}

// declaredReaches reports whether want is on from's declared generalization
// chain: a cycle, when asked of a type's own supertype.
func (m *Model) declaredReaches(from, want *symbols.Symbol, visiting map[*symbols.Symbol]bool) bool {
	if from == nil || want == nil {
		return false
	}
	if visiting == nil {
		visiting = make(map[*symbols.Symbol]bool)
	}
	if visiting[from] {
		return false
	}
	visiting[from] = true

	for _, rel := range RelationshipsOf(from) {
		if rel == nil || !GeneralizationKind(rel.Kind) {
			continue
		}
		target := m.relationshipTarget(from, rel)
		if target == want || m.declaredReaches(target, want, visiting) {
			return true
		}
	}
	return false
}

// baseUsageFQN is the most general base usage every usage element subsets,
// directly or indirectly (SysML v2 §7.6, [KerML, 8.4.2]). Its only member is
// `that`, the featuring instance of a usage's value, so subsetting it is what
// makes an unqualified `that` resolve inside a usage body.
const baseUsageFQN = "Base::things"

// isKerMLTypeDecl reports whether sym declares a KerML type rather than a
// feature — `class`, `struct`, `assoc`, `behavior`, `predicate`, `interaction` —
// which the parser records as a usage node (KerML §8.3).
func isKerMLTypeDecl(sym *symbols.Symbol) bool {
	return sym != nil && sym.Kind == symbols.SymbolKerMLType
}

// implicitKerMLFeatureBase returns the base feature a KerML feature declaration
// subsets (KerML §8.4.2); it contributes members only, like implicitBaseUsage.
func (m *Model) implicitKerMLFeatureBase(sym *symbols.Symbol) *symbols.Symbol {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || m.resolver == nil || m.resolver.Index() == nil || !m.isKerMLDoc(sym) {
		return nil
	}
	// Only a feature subsets a base feature; a type specializes a base type.
	if isKerMLTypeDecl(sym) {
		return nil
	}
	fqn, ok := implicitKerMLFeatureBases[usage.Keyword]
	// A typed feature subsets the base feature of its type's kind, not of its own
	// keyword: `feature f : C` with C a class subsets Occurrences::occurrences.
	if typed, tok := m.declaredTypeFeatureBase(sym); tok {
		fqn, ok = typed, true
	}
	if binary, bok := implicitKerMLBinaryFeatureBases[usage.Keyword]; bok && m.declaredEndCount(sym) == 2 {
		fqn, ok = binary, true
	}
	if !ok || m.declaredGeneralizationReaches(sym, fqn, nil) {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(fqn) {
		if base == nil || base == sym || enclosedBy(sym, base) {
			continue
		}
		return base
	}
	return nil
}

// implicitUsageBaseFeature returns a usage's standard-library base feature as
// a contributor and implicit general, never as a DirectSupertypes entry.
func (m *Model) implicitUsageBaseFeature(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || m.resolver == nil || m.resolver.Index() == nil || m.isKerMLDoc(sym) {
		return nil
	}
	if m.computingUsageBase[sym] {
		return nil
	}
	m.computingUsageBase[sym] = true
	defer delete(m.computingUsageBase, sym)
	var fqn string
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		var ok bool
		fqn, ok = implicitUsageBases[d.Kind]
		if !ok {
			return nil
		}
	case *ast.SubstateMember:
		fqn = implicitUsageBases[ast.UsageState]
	case *ast.TransitionMember:
		fqn = implicitUsageBases[ast.UsageTransition]
	case *ast.AssumeMember, *ast.RequireMember:
		fqn = implicitUsageBases[ast.UsageConstraint]
	default:
		return nil
	}
	if fqn == "" || m.declaredGeneralizationReaches(sym, fqn, nil) {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(fqn) {
		if base != nil && base != sym && !enclosedBy(sym, base) {
			return base
		}
	}
	return nil
}

// declaredTypeFeatureBase returns the base feature implied by the kind of the
// type sym is declared to have, if it declares one that is a KerML type.
func (m *Model) declaredTypeFeatureBase(sym *symbols.Symbol) (string, bool) {
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelTyping {
			continue
		}
		target := m.relationshipTarget(sym, rel)
		if target == nil || !isKerMLTypeDecl(target) {
			continue
		}
		if fqn, ok := implicitKerMLFeatureBases[keywordOf(target)]; ok {
			return fqn, true
		}
	}
	return "", false
}

// keywordOf returns the declaration keyword sym was written with.
func keywordOf(sym *symbols.Symbol) string {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.Keyword
	case *ast.Definition:
		return d.Keyword
	}
	return ""
}

// implicitBaseUsage returns Base::things for a usage element, or nil when sym is
// not a usage, is that base usage, or is owned by it.
func (m *Model) implicitBaseUsage(sym *symbols.Symbol) *symbols.Symbol {
	if _, ok := sym.Decl.(*ast.Usage); !ok {
		return nil
	}
	// A KerML type declaration is a classifier, not a usage of one.
	if isKerMLTypeDecl(sym) {
		return nil
	}
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	// A KerML feature typed by a kind with its own base subsets that base instead.
	if typed, ok := m.declaredTypeFeatureBase(sym); ok && typed != baseUsageFQN && m.isKerMLDoc(sym) {
		return nil
	}
	for _, base := range m.resolver.Index().LookupQualified(baseUsageFQN) {
		if base == nil || base == sym || enclosedBy(sym, base) {
			continue
		}
		return base
	}
	return nil
}

// enclosedBy reports whether owner's own scope encloses sym.
func enclosedBy(sym, owner *symbols.Symbol) bool {
	if owner.Scope == nil {
		return false
	}
	for s := sym.OwnerScope; s != nil; s = s.Parent() {
		if s == owner.Scope {
			return true
		}
	}
	return false
}

// ImplicitGenerals returns the general types sym has by its kind rather than by
// declaration. A scope reached through a recursive import does not traverse them
// (KerML 8.2.3.5).
func (m *Model) ImplicitGenerals(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, base := range slices.Concat(m.implicitBases(sym), []*symbols.Symbol{
		m.implicitBaseUsage(sym),
		m.implicitUsageBaseFeature(sym),
		m.implicitKerMLFeatureBase(sym),
	}) {
		if base != nil && base != sym {
			out = append(out, base)
		}
	}
	return out
}
