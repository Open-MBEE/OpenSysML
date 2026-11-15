package lower

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// FeatureFlow is a streaming flow a type or usage declares between two features
// it reaches by path (`flow pusher.push.output to cart.roll.input`).
type FeatureFlow struct {
	// Source and Target are the ends' paths from the declaring type, feature by feature.
	Source, Target []string
	// Multiplicity is the flow's own as written (`flow f [0] from …`), nil when none,
	// read in Scope, the body the flow was written in.
	Multiplicity *ast.Multiplicity
	Scope        *symbols.Scope
	Decl         *ast.Usage
}

// ToFeatureFlows lowers the streaming flows decl directly declares whose two ends
// are feature paths; a succession flow, which orders its ends, is none.
func ToFeatureFlows(decl ast.Node, scope *symbols.Scope) []FeatureFlow {
	var out []FeatureFlow
	for _, member := range UnwrappedDeclMembers(decl) {
		u, ok := member.(*ast.Usage)
		if !ok || u.Kind != ast.UsageFlow || u.FlowEnds == nil || u.IsSuccessionFlow() {
			continue
		}
		source := endSegments(ast.EndTarget(u.FlowEnds.From))
		target := endSegments(ast.EndTarget(u.FlowEnds.To))
		if len(source) == 0 || len(target) == 0 {
			continue
		}
		out = append(out, FeatureFlow{Source: source, Target: target, Multiplicity: u.Multiplicity, Scope: scope, Decl: u})
	}
	return out
}
