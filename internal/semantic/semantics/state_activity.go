package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// StateActivityPackage is the OpenSysML extension library declaring isActive,
// which no standard library file (States.sysml, StatePerformances.kerml) does.
const StateActivityPackage = "StateActivity"

// StateActivityFQN names the extension feature reading a state usage's activity.
const StateActivityFQN = StateActivityPackage + "::isActive"

// IsStateActivity reports whether sym is the StateActivity::isActive feature.
func IsStateActivity(sym *symbols.Symbol) bool {
	return sym != nil && symbols.FQNOf(sym) == StateActivityFQN
}

// FeaturingTypes are the types sym declares itself `featured by` (KerML
// 7.3.4.3), from the recorded facts of a cached library symbol or from its
// declaration.
func (m *Model) FeaturingTypes(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	if sym.Recorded() {
		return m.RecordedRelationshipTargets(sym, ast.RelFeaturedBy)
	}
	var out []*symbols.Symbol
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelFeaturedBy {
			continue
		}
		if target := m.RelationshipTarget(sym, rel); target != nil {
			out = append(out, target)
		}
	}
	return out
}
