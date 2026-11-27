package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const (
	transferPayloadFQN      = "Transfers::Transfer::payload"
	transferSourceOutputFQN = "Transfers::Transfer::source::sourceOutput"
	transferTargetInputFQN  = "Transfers::Transfer::target::targetInput"
)

// ImpliedRedefinitions returns the features of the implied Redefinitions sym owns once they are
// included (KerML §8.4.2, Tables 9 and 11; SysML Table 33); the redundancy rules do not apply to them.
func (m *Model) ImpliedRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	add := func(features []*symbols.Symbol) {
		for _, feature := range features {
			if feature != nil && feature != sym && !slices.Contains(out, feature) {
				out = append(out, feature)
			}
		}
	}
	add(m.ImplicitParameterRedefinitions(sym))
	add(m.expressionParameterRedefinitions(sym))
	add(m.implicitEndRedefinitions(sym))
	add(m.ImplicitRoleRedefinitions(sym))
	add(m.flowFeatureRedefinitions(sym))
	add(m.controlParameterRedefinitions(sym))
	// A written redefinition of the same feature leaves nothing to imply.
	written := m.writtenRedefinedFeatures(sym)
	return slices.DeleteFunc(out, func(feature *symbols.Symbol) bool {
		return slices.ContainsFunc(written, func(other *symbols.Symbol) bool { return symbols.SameElement(other, feature) })
	})
}

// writtenRedefinedFeatures returns the features sym redefines by what its notation states: a
// clause, the feature a flow's end names, the name a metadata body restates or an argument is given under.
func (m *Model) writtenRedefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	out := slices.Clone(m.RedefinedFeatures(sym))
	if target := m.flowEndFeatureRedefinition(sym); target != nil {
		out = append(out, target)
	}
	out = append(out, m.implicitMetadataBodyRedefinitions(sym)...)
	return append(out, m.namedArgumentRedefinitions(sym)...)
}

// flowFeatureRedefinitions returns the library features a flow's payload and the features its
// ends own redefine (KerML checkPayloadFeatureRedefinition, checkFeatureFlowFeatureRedefinition).
func (m *Model) flowFeatureRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	fqn := ""
	if i, ok := symbols.ImpliedFlowEndFeatureIndex(sym); ok {
		fqn = [...]string{transferSourceOutputFQN, transferTargetInputFQN}[i]
	} else if IsFlowPayload(sym) || m.IsFlowPayload(sym) {
		fqn = transferPayloadFQN
	}
	if fqn == "" {
		return nil
	}
	if feature := m.typeByFQN(fqn); feature != nil {
		return []*symbols.Symbol{feature}
	}
	return nil
}

// ImpliedTypeFeaturings returns the featuring types of the implied TypeFeaturings sym owns once
// they are included (KerML §8.4.2, Tables 9 and 11; SysML Table 33): no redundancy rule applies.
func (m *Model) ImpliedTypeFeaturings(sym *symbols.Symbol) []FeaturingType {
	if m == nil || sym == nil {
		return nil
	}
	return m.impliedTypeFeaturings(sym, map[*symbols.Symbol]bool{})
}
