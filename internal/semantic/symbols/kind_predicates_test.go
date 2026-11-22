package symbols

import "testing"

// TestKindPredicatesOverEveryKind pins the object and behavior predicates over
// every named kind, so a new kind is classified here rather than by default.
func TestKindPredicatesOverEveryKind(t *testing.T) {
	const (
		object = iota + 1
		behaviorDef
		behaviorUsage
	)
	want := map[SymbolKind]int{
		SymbolPartDef: object, SymbolPartUsage: object,
		SymbolItemDef: object, SymbolItemUsage: object,
		SymbolOccurrenceDef: object, SymbolOccurrenceUsage: object,
		SymbolIndividualDef: object, SymbolIndividualUsage: object,

		SymbolActionDef: behaviorDef, SymbolStateDef: behaviorDef, SymbolCalcDef: behaviorDef,
		SymbolCaseDef: behaviorDef, SymbolAnalysisCaseDef: behaviorDef,
		SymbolVerificationCaseDef: behaviorDef, SymbolUseCaseDef: behaviorDef,

		SymbolStateUsage: behaviorUsage, SymbolCalcUsage: behaviorUsage,
		SymbolCaseUsage: behaviorUsage, SymbolAnalysisCaseUsage: behaviorUsage,
		SymbolVerificationCaseUsage: behaviorUsage, SymbolUseCaseUsage: behaviorUsage,
		SymbolConstraintUsage: behaviorUsage, SymbolRequirementUsage: behaviorUsage,
		SymbolActionUsage: behaviorUsage, SymbolSendActionUsage: behaviorUsage,
		SymbolAcceptActionUsage: behaviorUsage, SymbolAssignmentActionUsage: behaviorUsage,
		SymbolPerformActionUsage: behaviorUsage, SymbolIfActionUsage: behaviorUsage,
		SymbolWhileLoopActionUsage: behaviorUsage, SymbolForLoopActionUsage: behaviorUsage,
		SymbolTerminateActionUsage: behaviorUsage,
	}
	for kind := range want {
		if !kind.Named() {
			t.Errorf("kind %d is expected a class but has no name", kind)
		}
	}
	for kind, name := range symbolKindNames {
		class := want[kind]
		if got := kind.IsObject(); got != (class == object) {
			t.Errorf("%s.IsObject() = %v", name, got)
		}
		if got := kind.IsBehaviorDef(); got != (class == behaviorDef) {
			t.Errorf("%s.IsBehaviorDef() = %v", name, got)
		}
		if got := kind.IsBehaviorUsage(); got != (class == behaviorUsage) {
			t.Errorf("%s.IsBehaviorUsage() = %v", name, got)
		}
		if got := kind.IsPerformance(); got != (class == behaviorDef || class == behaviorUsage) {
			t.Errorf("%s.IsPerformance() = %v", name, got)
		}
		if kind.IsActionUsage() && !kind.IsBehaviorUsage() {
			t.Errorf("%s is an action usage but not a behavior usage", name)
		}
		if kind.IsBehaviorDef() && !kind.IsDefinition() {
			t.Errorf("%s is a behavior definition but not a definition", name)
		}
		if kind.IsBehaviorUsage() && !kind.IsFeature() {
			t.Errorf("%s is a behavior usage but not a feature", name)
		}
	}
}
