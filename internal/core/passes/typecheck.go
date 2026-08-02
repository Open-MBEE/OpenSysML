package passes

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// TypeCheckPass validates that each def/usage relationship target has a symbol
// kind compatible with the source node and relationship kind (spec §6.3).
// It runs at LevelType, after name resolution; unresolved targets are skipped.
type TypeCheckPass struct{}

func (TypeCheckPass) Level() PassLevel { return LevelType }

func (TypeCheckPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	tc := &typeChecker{resolver: ctx.Resolver()}
	tc.walk(rootScope, root.Members)
	return tc.diags
}

type typeChecker struct {
	resolver *resolve.Resolver
	diags    []Diagnostic
}

func (tc *typeChecker) walk(scope *symbols.Scope, members []ast.Node) {
	for _, m := range members {
		switch d := unwrapType(m).(type) {
		case *ast.Definition:
			tc.checkRelationships(scope, d.Relationships, true, d.Kind, 0)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Usage:
			tc.checkRelationships(scope, d.Relationships, false, 0, d.Kind)
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Package:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		case *ast.Namespace:
			if child := childScopeOf(scope, d); child != nil {
				tc.walk(child, d.Members)
			}
		}
	}
}

func (tc *typeChecker) checkRelationships(scope *symbols.Scope, rels []*ast.Relationship, isDef bool, defKind ast.DefinitionKind, useKind ast.UsageKind) {
	for _, rel := range rels {
		if rel == nil || rel.Target == nil {
			continue
		}
		// Unwrap FeatureReference if needed
		targetNode := rel.Target
		if fr, ok := targetNode.(*ast.FeatureReference); ok {
			targetNode = fr.Name
		}
		qn, isQN := targetNode.(*ast.QualifiedName)
		if !isQN {
			continue
		}
		sym, ok := tc.resolver.ResolveQualified(scope, qn)
		if !ok || sym == nil {
			continue // unresolved: name-resolution tier owns this
		}
		if msg := compatMessage(isDef, defKind, useKind, rel.Kind, sym.Kind); msg != "" {
			tc.diags = append(tc.diags, Diagnostic{
				Severity: SeverityError,
				Span:     rel.Target.Span(),
				Message:  msg,
				Code:     "type",
				Source:   "type",
			})
		}
	}
}

func compatMessage(isDef bool, defKind ast.DefinitionKind, useKind ast.UsageKind, rel ast.RelationshipKind, target symbols.SymbolKind) string {
	switch rel {
	case ast.RelSpecializes:
		want := defSymbolKind(defKind)
		if !isDef {
			return "only a definition may specialize; found a usage"
		}
		if !isDefKind(target) {
			return fmt.Sprintf("%s cannot specialize %s (target is not a definition)", defKind, target)
		}
		if target != want {
			return fmt.Sprintf("%s cannot specialize %s (kind mismatch)", defKind, target)
		}
	case ast.RelSubsets, ast.RelRedefines:
		if isDef {
			return fmt.Sprintf("a definition may not %s a feature", rel)
		}
		if !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	case ast.RelTyping:
		if isDef {
			return "" // typing on a definition is not produced by the parser; ignore
		}
		if !isDefKind(target) {
			return fmt.Sprintf("type must be a definition, found %s", target)
		}
		// Check if typing is compatible
		// Allow structural kinds (part/attribute/item/occurrence) to cross-type
		// since they're structurally compatible in SysML
		if !isCompatibleTyping(useKind, target) {
			return fmt.Sprintf("%s cannot be typed by %s (kind mismatch)", useKind, target)
		}
	case ast.RelReferences, ast.RelCrosses:
		if !isDef && !isUsageKind(target) {
			return fmt.Sprintf("%s target must be a usage, found %s", rel, target)
		}
	}
	return ""
}

func defSymbolKind(k ast.DefinitionKind) symbols.SymbolKind {
	switch k {
	case ast.DefPart:
		return symbols.SymbolPartDef
	case ast.DefAttribute:
		return symbols.SymbolAttributeDef
	case ast.DefItem:
		return symbols.SymbolItemDef
	case ast.DefOccurrence:
		return symbols.SymbolOccurrenceDef
	case ast.DefIndividual:
		return symbols.SymbolIndividualDef
	case ast.DefMetadata:
		return symbols.SymbolMetadataDef
	case ast.DefEnumeration:
		return symbols.SymbolEnumerationDef
	case ast.DefView:
		return symbols.SymbolViewDef
	case ast.DefViewpoint:
		return symbols.SymbolViewpointDef
	case ast.DefRendering:
		return symbols.SymbolRenderingDef
	case ast.DefConcern:
		return symbols.SymbolConcernDef
	case ast.DefConnection:
		return symbols.SymbolConnectionDef
	case ast.DefFlow:
		return symbols.SymbolFlowDef
	case ast.DefPort:
		return symbols.SymbolPortDef
	case ast.DefInterface:
		return symbols.SymbolInterfaceDef
	case ast.DefAllocation:
		return symbols.SymbolAllocationDef
	case ast.DefAction:
		return symbols.SymbolActionDef
	case ast.DefState:
		return symbols.SymbolStateDef
	case ast.DefCalc:
		return symbols.SymbolCalcDef
	case ast.DefConstraint:
		return symbols.SymbolConstraintDef
	case ast.DefRequirement:
		return symbols.SymbolRequirementDef
	case ast.DefCase:
		return symbols.SymbolCaseDef
	case ast.DefAnalysisCase:
		return symbols.SymbolAnalysisCaseDef
	case ast.DefVerificationCase:
		return symbols.SymbolVerificationCaseDef
	case ast.DefUseCase:
		return symbols.SymbolUseCaseDef
	}
	return symbols.SymbolUnknown
}

func usageWantsDefKind(k ast.UsageKind) symbols.SymbolKind {
	switch k {
	case ast.UsagePart:
		return symbols.SymbolPartDef
	case ast.UsageAttribute:
		return symbols.SymbolAttributeDef
	case ast.UsageItem:
		return symbols.SymbolItemDef
	case ast.UsageOccurrence:
		return symbols.SymbolOccurrenceDef
	case ast.UsageIndividual:
		return symbols.SymbolIndividualDef
	case ast.UsageMetadata:
		return symbols.SymbolMetadataDef
	case ast.UsageEnumeration:
		return symbols.SymbolEnumerationDef
	case ast.UsageView:
		return symbols.SymbolViewDef
	case ast.UsageViewpoint:
		return symbols.SymbolViewpointDef
	case ast.UsageRendering:
		return symbols.SymbolRenderingDef
	case ast.UsageConcern:
		return symbols.SymbolConcernDef
	case ast.UsageConnection:
		return symbols.SymbolConnectionDef
	case ast.UsageFlow:
		return symbols.SymbolFlowDef
	case ast.UsagePort:
		return symbols.SymbolPortDef
	case ast.UsageInterface:
		return symbols.SymbolInterfaceDef
	case ast.UsageAllocation:
		return symbols.SymbolAllocationDef
	case ast.UsageAction:
		return symbols.SymbolActionDef
	case ast.UsageState:
		return symbols.SymbolStateDef
	case ast.UsageCalc:
		return symbols.SymbolCalcDef
	case ast.UsageConstraint:
		return symbols.SymbolConstraintDef
	case ast.UsageRequirement:
		return symbols.SymbolRequirementDef
	case ast.UsageCase:
		return symbols.SymbolCaseDef
	case ast.UsageAnalysisCase:
		return symbols.SymbolAnalysisCaseDef
	case ast.UsageVerificationCase:
		return symbols.SymbolVerificationCaseDef
	case ast.UsageUseCase:
		return symbols.SymbolUseCaseDef
	}
	return symbols.SymbolUnknown
}

// isCompatibleTyping checks if a usage kind can be typed by a definition kind.
// Allows structural compatibility: part/attribute/item/occurrence can cross-type
// since they're all structural classifiers in SysML.
func isCompatibleTyping(useKind ast.UsageKind, defKind symbols.SymbolKind) bool {
	// Exact match always allowed
	if defKind == usageWantsDefKind(useKind) {
		return true
	}
	
	// Structural kinds can cross-type (part, attribute, item, occurrence)
	structuralUsages := map[ast.UsageKind]bool{
		ast.UsagePart:       true,
		ast.UsageAttribute:  true,
		ast.UsageItem:       true,
		ast.UsageOccurrence: true,
	}
	structuralDefs := map[symbols.SymbolKind]bool{
		symbols.SymbolPartDef:       true,
		symbols.SymbolAttributeDef:  true,
		symbols.SymbolItemDef:       true,
		symbols.SymbolOccurrenceDef: true,
	}
	
	if structuralUsages[useKind] && structuralDefs[defKind] {
		return true
	}
	
	return false
}

// defSymbolKinds is the set of SymbolKinds that classify a definition.
var defSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolPartDef:             true,
	symbols.SymbolAttributeDef:        true,
	symbols.SymbolItemDef:             true,
	symbols.SymbolOccurrenceDef:       true,
	symbols.SymbolIndividualDef:       true,
	symbols.SymbolMetadataDef:         true,
	symbols.SymbolEnumerationDef:      true,
	symbols.SymbolViewDef:             true,
	symbols.SymbolViewpointDef:        true,
	symbols.SymbolRenderingDef:        true,
	symbols.SymbolConcernDef:          true,
	symbols.SymbolConnectionDef:       true,
	symbols.SymbolFlowDef:             true,
	symbols.SymbolPortDef:             true,
	symbols.SymbolInterfaceDef:        true,
	symbols.SymbolAllocationDef:       true,
	symbols.SymbolActionDef:           true,
	symbols.SymbolStateDef:            true,
	symbols.SymbolCalcDef:             true,
	symbols.SymbolConstraintDef:       true,
	symbols.SymbolRequirementDef:      true,
	symbols.SymbolCaseDef:             true,
	symbols.SymbolAnalysisCaseDef:     true,
	symbols.SymbolVerificationCaseDef: true,
	symbols.SymbolUseCaseDef:          true,
}

// usageSymbolKinds is the set of SymbolKinds that classify a usage.
var usageSymbolKinds = map[symbols.SymbolKind]bool{
	symbols.SymbolPartUsage:             true,
	symbols.SymbolAttributeUsage:        true,
	symbols.SymbolItemUsage:             true,
	symbols.SymbolOccurrenceUsage:       true,
	symbols.SymbolIndividualUsage:       true,
	symbols.SymbolMetadataUsage:         true,
	symbols.SymbolEnumerationUsage:      true,
	symbols.SymbolViewUsage:             true,
	symbols.SymbolViewpointUsage:        true,
	symbols.SymbolRenderingUsage:        true,
	symbols.SymbolConcernUsage:          true,
	symbols.SymbolConnectionUsage:       true,
	symbols.SymbolFlowUsage:             true,
	symbols.SymbolPortUsage:             true,
	symbols.SymbolInterfaceUsage:        true,
	symbols.SymbolAllocationUsage:       true,
	symbols.SymbolActionUsage:           true,
	symbols.SymbolStateUsage:            true,
	symbols.SymbolCalcUsage:             true,
	symbols.SymbolConstraintUsage:       true,
	symbols.SymbolRequirementUsage:      true,
	symbols.SymbolCaseUsage:             true,
	symbols.SymbolAnalysisCaseUsage:     true,
	symbols.SymbolVerificationCaseUsage: true,
	symbols.SymbolUseCaseUsage:          true,
}

func isDefKind(k symbols.SymbolKind) bool {
	return defSymbolKinds[k]
}

func isUsageKind(k symbols.SymbolKind) bool {
	return usageSymbolKinds[k]
}

func unwrapType(n ast.Node) ast.Node {
	if m, ok := n.(*ast.Membership); ok {
		return m.Member
	}
	return n
}

func childScopeOf(scope *symbols.Scope, decl ast.Node) *symbols.Scope {
	for _, c := range scope.Children() {
		if c.Node() == decl {
			return c
		}
	}
	return nil
}
