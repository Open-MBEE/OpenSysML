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
	switch kind {
	case ast.UsagePart, ast.UsageActor, ast.UsageStakeholder,
		ast.UsageConnection, ast.UsageInterface, ast.UsageAllocation:
		return m.partNestedFQN(owner, composite, portion)
	case ast.UsageItem:
		if composite && ownerInItemFamily(owner) {
			return "Items::Item::subitems"
		}
		return m.occurrenceNestedFQN(owner, composite, portion, false)
	case ast.UsageRendering, ast.UsageViewRendering:
		if composite && ownerInRenderingFamily(owner) {
			return "Views::Rendering::subrenderings"
		}
		return m.partNestedFQN(owner, composite, portion)
	case ast.UsageView:
		if composite && ownerInViewFamily(owner) {
			return "Views::View::subviews"
		}
		return m.partNestedFQN(owner, composite, portion)
	case ast.UsagePort:
		if composite && ownerInPartFamily(owner) {
			return "Parts::Part::ownedPorts"
		}
		if composite && ownerInPortFamily(owner) {
			return "Ports::Port::subports"
		}
		return m.occurrenceNestedFQN(owner, composite, portion, false)
	case ast.UsageAction:
		if usage.IsPerformedAction() && ownerInPartFamily(owner) {
			return "Parts::Part::performedActions"
		}
		return m.actionNestedFQN(owner, composite, portion)
	case ast.UsageTransition, ast.UsageFlow:
		return m.actionNestedFQN(owner, composite, portion)
	case ast.UsageState:
		if usage != nil && usage.IsExhibitedState() && ownerInPartFamily(owner) {
			return "Parts::Part::exhibitedStates"
		}
		return m.stateNestedFQN(owner, composite, portion)
	case ast.UsageCalc:
		return m.calcNestedFQN(owner, composite, portion)
	case ast.UsageCase:
		return m.caseNestedFQN(owner, composite, portion)
	case ast.UsageAnalysisCase:
		if composite && ownerInAnalysisCaseFamily(owner) {
			return "AnalysisCases::AnalysisCase::subAnalysisCases"
		}
		return m.caseNestedFQN(owner, composite, portion)
	case ast.UsageVerificationCase:
		if composite && ownerInVerificationCaseFamily(owner) {
			return "VerificationCases::VerificationCase::subVerificationCases"
		}
		return m.caseNestedFQN(owner, composite, portion)
	case ast.UsageUseCase:
		if ownerInUseCaseFamily(owner) &&
			(usage.IsIncludedUseCase() || composite) {
			if usage.IsIncludedUseCase() {
				return "UseCases::UseCase::includedUseCases"
			}
			return "UseCases::UseCase::subUseCases"
		}
		return m.caseNestedFQN(owner, composite, portion)
	case ast.UsageConstraint:
		if usage.IsRequirementConstraint() {
			return ""
		}
		return m.constraintNestedFQN(owner, composite, portion)
	case ast.UsageRequirement, ast.UsageSatisfy, ast.UsageConcern, ast.UsageViewpoint:
		return m.requirementNestedFQN(owner, composite, portion)
	case ast.UsageOccurrence, ast.UsageIndividual:
		return m.occurrenceNestedFQN(owner, composite, portion, false)
	case ast.UsageStep:
		if composite {
			return m.stepNestedFQN(owner)
		}
		return ""
	}
	return ""
}

// partNestedFQN is the rule chain of a usage whose kind nests under an item:
// the item's `subparts` when composite, else the occurrence rules.
func (m *Model) partNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	if composite && ownerInItemFamily(owner) {
		return "Items::Item::subparts"
	}
	return m.occurrenceNestedFQN(owner, composite, portion, false)
}

// actionNestedFQN is the rule chain of a usage whose kind nests under an
// action: the action's `subactions`, the part's `ownedActions`, then the
// occurrence rules with the KerML step fallback behind them.
func (m *Model) actionNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	if composite && ownerInActionFamily(owner) {
		return "Actions::Action::subactions"
	}
	if composite && ownerInPartFamily(owner) {
		return "Parts::Part::ownedActions"
	}
	return m.occurrenceNestedFQN(owner, composite, portion, true)
}

// stateNestedFQN is the rule chain of a nested state that is not exhibited.
func (m *Model) stateNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	if composite && ownerInStateFamily(owner) {
		return "States::StateAction::substates"
	}
	if composite && ownerInPartFamily(owner) {
		return "Parts::Part::ownedStates"
	}
	return m.actionNestedFQN(owner, composite, portion)
}

// calcNestedFQN is the rule chain of a nested calculation, which continues as
// the action rules when no calculation owns it.
func (m *Model) calcNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	if composite && ownerInCalcFamily(owner) {
		return "Calculations::Calculation::subcalculations"
	}
	return m.actionNestedFQN(owner, composite, portion)
}

// caseNestedFQN is the rule chain of a nested case, which continues as the
// calculation rules when no case owns it.
func (m *Model) caseNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	if composite && ownerInCaseFamily(owner) {
		return "Cases::Case::subcases"
	}
	return m.calcNestedFQN(owner, composite, portion)
}

// constraintNestedFQN is the rule chain of a nested constraint check.
func (m *Model) constraintNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	if composite && ownerInItemFamily(owner) {
		return "Items::Item::checkedConstraints"
	}
	return m.occurrenceNestedFQN(owner, composite, portion, false)
}

// requirementNestedFQN is the rule chain of a nested requirement check, which
// continues as the constraint rules when no requirement owns it.
func (m *Model) requirementNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind) string {
	if composite && ownerInRequirementFamily(owner) {
		return "Requirements::RequirementCheck::subrequirements"
	}
	return m.constraintNestedFQN(owner, composite, portion)
}

// occurrenceNestedFQN is the occurrence fallback every chain ends in: an
// occurrence-family owner's `suboccurrences`, or its `timeSlices`/`snapshots`
// for a portion usage. Behind it, for usages routed through the action rules,
// is the KerML step fallback.
func (m *Model) occurrenceNestedFQN(owner *symbols.Symbol, composite bool, portion ast.PortionKind, stepFallback bool) string {
	if composite && ownerInOccurrenceFamily(owner) {
		switch portion {
		case ast.PortionTimeslice:
			return "Occurrences::Occurrence::timeSlices"
		case ast.PortionSnapshot:
			return "Occurrences::Occurrence::snapshots"
		}
		return "Occurrences::Occurrence::suboccurrences"
	}
	if stepFallback {
		return m.stepNestedFQN(owner)
	}
	return ""
}

// stepNestedFQN is the KerML step fallback (KerML 1.1 §8.3.4.8), whose owner is
// a KerML type judged by library conformance rather than by declaration kind.
func (m *Model) stepNestedFQN(owner *symbols.Symbol) string {
	switch {
	case m.conformsByName(owner, "Performances::Performance"):
		return "Performances::Performance::subperformances"
	case m.conformsByName(owner, "Objects::Object"):
		return "Objects::Object::ownedPerformances"
	case m.conformsByName(owner, "Occurrences::Occurrence"):
		return "Performances::Performance::enclosedPerformances"
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
func ownerKindIn(owner *symbols.Symbol, usages []ast.UsageKind, defs []ast.DefinitionKind) bool {
	if u, ok := ownerUsageKind(owner); ok {
		return slices.Contains(usages, u)
	}
	if d, ok := owner.Decl.(*ast.Definition); ok {
		return slices.Contains(defs, d.Kind)
	}
	return false
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

func ownerInPartFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, nestedPartFamilyUsages, nestedPartFamilyDefs)
}

func ownerInItemFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, nestedItemFamilyUsages, nestedItemFamilyDefs)
}

func ownerInPortFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsagePort}, []ast.DefinitionKind{ast.DefPort})
}

func ownerInOccurrenceFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, nestedOccurrenceFamilyUsages, nestedOccurrenceFamilyDefs)
}

func ownerInActionFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, nestedActionFamilyUsages, nestedActionFamilyDefs)
}

func ownerInStateFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageState}, []ast.DefinitionKind{ast.DefState})
}

func ownerInCalcFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, nestedCalcFamilyUsages, nestedCalcFamilyDefs)
}

func ownerInCaseFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, nestedCaseFamilyUsages, nestedCaseFamilyDefs)
}

func ownerInAnalysisCaseFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageAnalysisCase}, []ast.DefinitionKind{ast.DefAnalysisCase})
}

func ownerInVerificationCaseFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageVerificationCase}, []ast.DefinitionKind{ast.DefVerificationCase})
}

func ownerInUseCaseFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageUseCase}, []ast.DefinitionKind{ast.DefUseCase})
}

func ownerInRequirementFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, nestedRequirementFamilyUsages, nestedRequirementFamilyDefs)
}

func ownerInViewFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner, []ast.UsageKind{ast.UsageView}, []ast.DefinitionKind{ast.DefView})
}

func ownerInRenderingFamily(owner *symbols.Symbol) bool {
	return ownerKindIn(owner,
		[]ast.UsageKind{ast.UsageRendering, ast.UsageViewRendering},
		[]ast.DefinitionKind{ast.DefRendering})
}
