package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// parameterElements reflects the parameter features of Type, Behavior, Step, Expression and
// Function; ok is false for any other feature or metaclass.
func (m *Model) parameterElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	switch feature {
	case "input":
		return m.inputElements(sym)
	case "output":
		return m.outputElements(sym)
	case "parameter":
		return m.parameterListElements(sym)
	case "directedFeature":
		return m.directedFeatureElements(sym)
	case "subjectParameter":
		return m.subjectParameterElements(sym)
	case "objectiveRequirement":
		return m.objectiveRequirementElements(sym)
	case "result":
		return m.resultElements(sym)
	}
	return nil, false
}

func (m *Model) inputElements(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !m.reflectiveMetaclassConforms(sym, "Type") {
		return nil, false
	}
	if isExpressionSymbol(sym) {
		return m.ExpressionParameters(sym), true
	}
	if behaviorLike(sym) {
		out := m.InputParametersOf(sym)
		return includeSubjectParameterInOrder(m, sym, out), true
	}
	out := m.directedFeatures(sym, ast.DirIn, ast.DirInOut)
	if m.reflectiveMetaclassConforms(sym, "RequirementUsage") {
		out = includeSubjectParameterInOrder(m, sym, out)
	}
	return out, true
}

func (m *Model) outputElements(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !m.reflectiveMetaclassConforms(sym, "Type") {
		return nil, false
	}
	if isExpressionSymbol(sym) {
		return expressionResultList(m, sym), true
	}
	if !behaviorLike(sym) {
		return m.directedFeatures(sym, ast.DirOut, ast.DirInOut), true
	}
	var out []*symbols.Symbol
	for _, parameter := range m.BehaviorParametersOf(sym) {
		if parameter.Symbol == nil || parameter.IsResult ||
			(parameter.Direction != ast.DirOut && parameter.Direction != ast.DirInOut) {
			continue
		}
		out = append(out, parameter.Symbol)
	}
	for _, parameter := range m.BehaviorParametersOf(sym) {
		if parameter.Symbol != nil && parameter.IsResult {
			out = append(out, parameter.Symbol)
		}
	}
	return out, true
}

func (m *Model) parameterListElements(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !m.reflectiveMetaclassConforms(sym, "Behavior") &&
		!m.reflectiveMetaclassConforms(sym, "Step") {
		return nil, false
	}
	if isExpressionSymbol(sym) {
		return m.expressionParameterParts(sym), true
	}
	var out []*symbols.Symbol
	for _, parameter := range m.BehaviorParametersOf(sym) {
		if parameter.Symbol != nil {
			out = append(out, parameter.Symbol)
		}
	}
	return includeSubjectParameterInOrder(m, sym, out), true
}

func (m *Model) directedFeatureElements(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !m.reflectiveMetaclassConforms(sym, "Type") {
		return nil, false
	}
	if isExpressionSymbol(sym) {
		return m.expressionParameterParts(sym), true
	}
	return m.directedFeatures(sym, ast.DirIn, ast.DirInOut, ast.DirOut), true
}

func (m *Model) subjectParameterElements(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !m.ownsSubjectParameter(sym) {
		return nil, false
	}
	if subject := m.SubjectParameterOf(sym); subject != nil {
		return []*symbols.Symbol{subject}, true
	}
	return nil, true
}

func (m *Model) objectiveRequirementElements(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !m.ownsObjectiveRequirement(sym) {
		return nil, false
	}
	owned, inherited := m.ObjectivesOf(sym)
	objectives := append(owned, inherited...)
	if len(objectives) > 0 && objectives[0] != nil {
		return []*symbols.Symbol{objectives[0]}, true
	}
	return nil, true
}

func (m *Model) resultElements(sym *symbols.Symbol) ([]*symbols.Symbol, bool) {
	if !m.reflectiveMetaclassConforms(sym, "Expression") &&
		!m.reflectiveMetaclassConforms(sym, "Function") {
		return nil, false
	}
	if isExpressionSymbol(sym) {
		return expressionResultList(m, sym), true
	}
	if result := m.ResultParameterOf(sym); result != nil {
		return []*symbols.Symbol{result}, true
	}
	return nil, true
}
