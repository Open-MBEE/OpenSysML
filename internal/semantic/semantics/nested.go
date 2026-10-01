package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// ImplicitSubsettings returns the feature of its owning type a nested usage
// implicitly subsets (SysML v2 §8.3: a composite part in an item is one of its
// `subparts`, an action in a part one of its `ownedActions`), or nothing.
func (m *Model) ImplicitSubsettings(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.implicitSubsettings[sym]; ok {
		return cached
	}
	m.resolver.Enter()
	out := m.computeImplicitSubsettings(sym)
	if m.resolver.Leave() {
		journal(m, m.implicitSubsettings, sym, sym.Decl)
		m.implicitSubsettings[sym] = out
	}
	return out
}

// computeImplicitSubsettings derives ImplicitSubsettings' answer: at most one
// library feature, the first whose rule the usage kind and its owner's kind
// match, resolved to a symbol when the library declares it.
func (m *Model) computeImplicitSubsettings(sym *symbols.Symbol) []*symbols.Symbol {
	if sym.OwnerScope == nil {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil {
		return nil
	}
	switch owner.Decl.(type) {
	case *ast.Definition, *ast.Usage, *ast.SubstateMember, *ast.TransitionMember,
		*ast.AssumeMember, *ast.RequireMember:
	default:
		return nil
	}
	usage, ok := sym.Decl.(*ast.Usage)
	portion := ast.PortionNone
	composite := true
	if ok {
		if usage.IsVariant || usage.IsBodyParameter || IsParameter(sym) {
			return nil
		}
		portion = usage.Portion
		composite = usageIsComposite(usage)
	} else if _, isSubstate := sym.Decl.(*ast.SubstateMember); !isSubstate {
		// Only usages carry these rules, plus the `state s;` of a state body,
		// which is a composite StateUsage the parser records as a SubstateMember.
		return nil
	}
	fqn := m.implicitSubsettingFQN(sym, usage, owner, composite, portion)
	if fqn == "" {
		return nil
	}
	if target := m.symbolByFQN(fqn); target != nil && target != sym {
		return []*symbols.Symbol{target}
	}
	return nil
}

// implicitSubsettingFQN selects the qualified name of the owner feature usage
// implicitly subsets; composite is whether usage is composite, which every rule
// but the performed, exhibited and included ones requires.
func (m *Model) implicitSubsettingFQN(sym *symbols.Symbol, usage *ast.Usage, owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	kind, ok := ownerUsageKind(sym)
	if !ok {
		return ""
	}
	u := NestedUsage{Kind: kind, Composite: composite, Portion: portion}
	if usage != nil {
		u.Performed = usage.IsPerformedAction()
		u.Exhibited = usage.IsExhibitedState()
		u.Included = usage.IsIncludedUseCase()
		u.RequirementConstraint = usage.IsRequirementConstraint()
	}
	o, ok := nestedOwnerOf(owner)
	if !ok {
		return ""
	}
	fqn, step := nestedRuleFQN(u, o)
	if step {
		return m.stepNestedFQN(owner)
	}
	return fqn
}

// NestedUsage is what a usage's declaration alone says of it to the implicit
// subsetting rules, for deciding them before the usage has a symbol.
type NestedUsage struct {
	Kind      ast.UsageKind
	Composite bool
	Portion   ast.PortionKind
	// Performed, Exhibited and Included are the `perform action`, `exhibit
	// state` and `include use case` forms; RequirementConstraint is a
	// `require` or `assume` constraint, which subsets nothing implicitly.
	Performed, Exhibited, Included, RequirementConstraint bool
}

// NestedOwner is the declaration kind of the type a usage nests in: a usage's
// kind, or a definition's when IsDef.
type NestedOwner struct {
	Usage ast.UsageKind
	Def   ast.DefinitionKind
	IsDef bool
}

// nestedOwnerOf is the declaration kind of owner, false for a declaration
// that is neither a usage (or a member form standing in for one) nor a definition.
func nestedOwnerOf(owner *symbols.Symbol) (NestedOwner, bool) {
	if u, ok := ownerUsageKind(owner); ok {
		return NestedOwner{Usage: u}, true
	}
	if d, ok := owner.Decl.(*ast.Definition); ok {
		return NestedOwner{Def: d.Kind, IsDef: true}, true
	}
	return NestedOwner{}, false
}

// stepFallbackFQNs are the features the KerML step fallback selects among,
// by what the owner conforms to (KerML 1.1 §8.3.4.8).
var stepFallbackFQNs = []string{
	"Performances::Performance::subperformances",
	"Objects::Object::ownedPerformances",
	"Performances::Performance::enclosedPerformances",
}

// ImplicitSubsettingCandidates lists, by declaration kinds alone, the library
// features a usage so declared may implicitly subset: the one the kind rules
// settle on, or the step fallback's three under an owner whose kind is an
// occurrence's, which of them depending on what the owner conforms to.
func ImplicitSubsettingCandidates(u NestedUsage, owner NestedOwner) []string {
	fqn, step := nestedRuleFQN(u, owner)
	switch {
	case fqn != "":
		return []string{fqn}
	case step && ownerInOccurrenceFamily(owner):
		return slices.Clone(stepFallbackFQNs)
	}
	return nil
}

// nestedRuleFQN runs the rule chains by declaration kinds: the feature they
// settle on, or step when they end in the KerML step fallback, which what the
// owner conforms to decides.
func nestedRuleFQN(u NestedUsage, owner NestedOwner) (fqn string, step bool) {
	if fqn := familyNestedFQN(u, owner); fqn != "" {
		return fqn, false
	}
	switch u.Kind {
	case ast.UsagePart, ast.UsageActor, ast.UsageStakeholder,
		ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation,
		ast.UsageRendering, ast.UsageViewRendering, ast.UsageView:
		return partNestedFQN(owner, u.Composite, u.Portion)
	case ast.UsageItem, ast.UsagePort, ast.UsageOccurrence, ast.UsageIndividual:
		return occurrenceNestedFQN(owner, u.Composite, u.Portion, false)
	case ast.UsageAction, ast.UsageTransition, ast.UsageFlow:
		return actionNestedFQN(owner, u.Composite, u.Portion)
	case ast.UsageState:
		return stateNestedFQN(owner, u.Composite, u.Portion)
	case ast.UsageCalc:
		return calcNestedFQN(owner, u.Composite, u.Portion)
	case ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase:
		return caseNestedFQN(owner, u.Composite, u.Portion)
	case ast.UsageConstraint:
		if u.RequirementConstraint {
			return "", false
		}
		return constraintNestedFQN(owner, u.Composite, u.Portion)
	case ast.UsageRequirement, ast.UsageSatisfy, ast.UsageConcern, ast.UsageViewpoint:
		return requirementNestedFQN(owner, u.Composite, u.Portion)
	case ast.UsageStep:
		if u.Composite {
			return "", true
		}
	}
	return "", false
}

// familyNestedFQN is the feature a usage of one kind nested in an owner of its
// own family subsets ahead of the general rule chains, "" when none applies.
func familyNestedFQN(u NestedUsage, owner NestedOwner) string {
	switch u.Kind {
	case ast.UsageAction:
		if u.Performed && ownerInPartFamily(owner) {
			return "Parts::Part::performedActions"
		}
	case ast.UsageState:
		if u.Exhibited && ownerInPartFamily(owner) {
			return "Parts::Part::exhibitedStates"
		}
	case ast.UsageUseCase:
		if ownerInUseCaseFamily(owner) && u.Included {
			return "UseCases::UseCase::includedUseCases"
		}
	}
	if !u.Composite {
		return ""
	}
	switch u.Kind {
	case ast.UsageItem:
		if ownerInItemFamily(owner) {
			return "Items::Item::subitems"
		}
	case ast.UsageRendering, ast.UsageViewRendering:
		if ownerInRenderingFamily(owner) {
			return "Views::Rendering::subrenderings"
		}
	case ast.UsageView:
		if ownerInViewFamily(owner) {
			return "Views::View::subviews"
		}
	case ast.UsagePort:
		if ownerInPartFamily(owner) {
			return "Parts::Part::ownedPorts"
		}
		if ownerInPortFamily(owner) {
			return "Ports::Port::subports"
		}
	case ast.UsageAnalysisCase:
		if ownerInAnalysisCaseFamily(owner) {
			return "AnalysisCases::AnalysisCase::subAnalysisCases"
		}
	case ast.UsageVerificationCase:
		if ownerInVerificationCaseFamily(owner) {
			return "VerificationCases::VerificationCase::subVerificationCases"
		}
	case ast.UsageUseCase:
		if ownerInUseCaseFamily(owner) {
			return "UseCases::UseCase::subUseCases"
		}
	}
	return ""
}

// partNestedFQN is the rule chain of a usage whose kind nests under an item:
// the item's `subparts` when composite, else the occurrence rules.
func partNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind) (string, bool) {
	if composite && ownerInItemFamily(owner) {
		return "Items::Item::subparts", false
	}
	return occurrenceNestedFQN(owner, composite, portion, false)
}

// actionNestedFQN is the rule chain of a usage whose kind nests under an
// action: the action's `subactions`, the part's `ownedActions`, then the
// occurrence rules with the KerML step fallback behind them.
func actionNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind) (string, bool) {
	if composite && ownerInActionFamily(owner) {
		return "Actions::Action::subactions", false
	}
	if composite && ownerInPartFamily(owner) {
		return "Parts::Part::ownedActions", false
	}
	return occurrenceNestedFQN(owner, composite, portion, true)
}

// stateNestedFQN is the rule chain of a nested state that is not exhibited.
func stateNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind) (string, bool) {
	if composite && ownerInStateFamily(owner) {
		return "States::StateAction::substates", false
	}
	if composite && ownerInPartFamily(owner) {
		return "Parts::Part::ownedStates", false
	}
	return actionNestedFQN(owner, composite, portion)
}

// calcNestedFQN is the rule chain of a nested calculation, which continues as
// the action rules when no calculation owns it.
func calcNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind) (string, bool) {
	if composite && ownerInCalcFamily(owner) {
		return "Calculations::Calculation::subcalculations", false
	}
	return actionNestedFQN(owner, composite, portion)
}

// caseNestedFQN is the rule chain of a nested case, which continues as the
// calculation rules when no case owns it.
func caseNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind) (string, bool) {
	if composite && ownerInCaseFamily(owner) {
		return "Cases::Case::subcases", false
	}
	return calcNestedFQN(owner, composite, portion)
}

// constraintNestedFQN is the rule chain of a nested constraint check.
func constraintNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind) (string, bool) {
	if composite && ownerInItemFamily(owner) {
		return "Items::Item::checkedConstraints", false
	}
	return occurrenceNestedFQN(owner, composite, portion, false)
}

// requirementNestedFQN is the rule chain of a nested requirement check, which
// continues as the constraint rules when no requirement owns it.
func requirementNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind) (string, bool) {
	if composite && ownerInRequirementFamily(owner) {
		return "Requirements::RequirementCheck::subrequirements", false
	}
	return constraintNestedFQN(owner, composite, portion)
}

// occurrenceNestedFQN is the occurrence fallback every chain ends in: an
// occurrence-family owner's `suboccurrences`, or its `timeSlices`/`snapshots`
// for a portion usage. Behind it, for usages routed through the action rules,
// is the KerML step fallback.
func occurrenceNestedFQN(owner NestedOwner, composite bool, portion ast.PortionKind, stepFallback bool) (string, bool) {
	if composite && ownerInOccurrenceFamily(owner) {
		switch portion {
		case ast.PortionTimeslice:
			return "Occurrences::Occurrence::timeSlices", false
		case ast.PortionSnapshot:
			return "Occurrences::Occurrence::snapshots", false
		}
		return "Occurrences::Occurrence::suboccurrences", false
	}
	return "", stepFallback
}

// stepNestedFQN is the KerML step fallback (KerML 1.1 §8.3.4.8), whose owner is
// a KerML type judged by library conformance rather than by declaration kind.
func (m *Model) stepNestedFQN(owner *symbols.Symbol) string {
	switch {
	case m.conformsByName(owner, "Performances::Performance"):
		return stepFallbackFQNs[0]
	case m.conformsByName(owner, "Objects::Object"):
		return stepFallbackFQNs[1]
	case m.conformsByName(owner, "Occurrences::Occurrence"):
		return stepFallbackFQNs[2]
	}
	return ""
}

// ownerUsageKind is the usage kind an owner's declaration declares, including
// the member forms that stand in for one (`state s` in a state body is a
// SubstateMember declaring a StateUsage).
func ownerUsageKind(owner *symbols.Symbol) (ast.UsageKind, bool) {
	switch d := owner.Decl.(type) {
	case *ast.Usage:
		return d.Kind, true
	case *ast.SubstateMember:
		return ast.UsageState, true
	case *ast.TransitionMember:
		return ast.UsageTransition, true
	case *ast.AssumeMember, *ast.RequireMember:
		return ast.UsageConstraint, true
	}
	return 0, false
}

// ownerKindIn reports whether the owner's declaration kind is one of the usage
// or definition kinds the family lists.
func ownerKindIn(owner NestedOwner, usages []ast.UsageKind, defs []ast.DefinitionKind) bool {
	if owner.IsDef {
		return slices.Contains(defs, owner.Def)
	}
	return slices.Contains(usages, owner.Usage)
}

var (
	nestedPartFamilyUsages = []ast.UsageKind{
		ast.UsagePart, ast.UsageActor, ast.UsageStakeholder, ast.UsageConnection,
		ast.UsageInterface, ast.UsageFlow, ast.UsageAllocation, ast.UsageView,
		ast.UsageRendering, ast.UsageViewRendering,
	}
	nestedPartFamilyDefs = []ast.DefinitionKind{
		ast.DefPart, ast.DefConnection, ast.DefInterface, ast.DefFlow,
		ast.DefAllocation, ast.DefView, ast.DefRendering,
	}
	nestedItemFamilyUsages = slices.Concat(nestedPartFamilyUsages, []ast.UsageKind{
		ast.UsageItem, ast.UsageMetadata,
	})
	nestedItemFamilyDefs = slices.Concat(nestedPartFamilyDefs, []ast.DefinitionKind{
		ast.DefItem, ast.DefMetadata,
	})
	nestedRequirementFamilyUsages = []ast.UsageKind{
		ast.UsageRequirement, ast.UsageSatisfy, ast.UsageConcern,
		ast.UsageFramedConcern, ast.UsageViewpoint,
	}
	nestedRequirementFamilyDefs = []ast.DefinitionKind{
		ast.DefRequirement, ast.DefConcern, ast.DefViewpoint,
	}
	nestedConstraintFamilyUsages = slices.Concat([]ast.UsageKind{
		ast.UsageConstraint,
	}, nestedRequirementFamilyUsages)
	nestedConstraintFamilyDefs = slices.Concat([]ast.DefinitionKind{
		ast.DefConstraint,
	}, nestedRequirementFamilyDefs)
	nestedCaseFamilyUsages = []ast.UsageKind{
		ast.UsageCase, ast.UsageAnalysisCase, ast.UsageVerificationCase, ast.UsageUseCase,
	}
	nestedCaseFamilyDefs = []ast.DefinitionKind{
		ast.DefCase, ast.DefAnalysisCase, ast.DefVerificationCase, ast.DefUseCase,
	}
	nestedCalcFamilyUsages = slices.Concat([]ast.UsageKind{
		ast.UsageCalc,
	}, nestedCaseFamilyUsages)
	nestedCalcFamilyDefs = slices.Concat([]ast.DefinitionKind{
		ast.DefCalc,
	}, nestedCaseFamilyDefs)
	nestedActionFamilyUsages = slices.Concat([]ast.UsageKind{
		ast.UsageAction, ast.UsageTransition, ast.UsageFlow, ast.UsageState,
	}, nestedCalcFamilyUsages)
	nestedActionFamilyDefs = slices.Concat([]ast.DefinitionKind{
		ast.DefAction, ast.DefState, ast.DefFlow,
	}, nestedCalcFamilyDefs)
	nestedOccurrenceFamilyUsages = slices.Concat([]ast.UsageKind{
		ast.UsageOccurrence, ast.UsageIndividual, ast.UsagePort,
	}, nestedItemFamilyUsages, nestedActionFamilyUsages, nestedConstraintFamilyUsages)
	nestedOccurrenceFamilyDefs = slices.Concat([]ast.DefinitionKind{
		ast.DefOccurrence, ast.DefIndividual, ast.DefPort,
	}, nestedItemFamilyDefs, nestedActionFamilyDefs, nestedConstraintFamilyDefs)
)

func ownerInPartFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, nestedPartFamilyUsages, nestedPartFamilyDefs)
}

func ownerInItemFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, nestedItemFamilyUsages, nestedItemFamilyDefs)
}

func ownerInPortFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsagePort}, []ast.DefinitionKind{ast.DefPort})
}

func ownerInOccurrenceFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, nestedOccurrenceFamilyUsages, nestedOccurrenceFamilyDefs)
}

func ownerInActionFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, nestedActionFamilyUsages, nestedActionFamilyDefs)
}

func ownerInStateFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageState}, []ast.DefinitionKind{ast.DefState})
}

func ownerInCalcFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, nestedCalcFamilyUsages, nestedCalcFamilyDefs)
}

func ownerInCaseFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, nestedCaseFamilyUsages, nestedCaseFamilyDefs)
}

func ownerInAnalysisCaseFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageAnalysisCase}, []ast.DefinitionKind{ast.DefAnalysisCase})
}

func ownerInVerificationCaseFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageVerificationCase}, []ast.DefinitionKind{ast.DefVerificationCase})
}

func ownerInUseCaseFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageUseCase}, []ast.DefinitionKind{ast.DefUseCase})
}

func ownerInRequirementFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, nestedRequirementFamilyUsages, nestedRequirementFamilyDefs)
}

func ownerInViewFamily(owner NestedOwner) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageView}, []ast.DefinitionKind{ast.DefView})
}

func ownerInRenderingFamily(owner NestedOwner) bool {
	return ownerKindIn(owner,
		[]ast.UsageKind{ast.UsageRendering, ast.UsageViewRendering},
		[]ast.DefinitionKind{ast.DefRendering})
}
