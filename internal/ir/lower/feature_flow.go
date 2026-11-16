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
		if !ok || u.Kind != ast.UsageFlow || u.IsSuccessionFlow() {
			continue
		}
		source, target := flowEndPaths(u)
		if len(source) == 0 || len(target) == 0 {
			continue
		}
		out = append(out, FeatureFlow{Source: source, Target: target, Multiplicity: u.Multiplicity, Scope: scope, Decl: u})
	}
	return out
}

// flowEndPaths returns the paths of the source output and target input features of flow:
// its `from` and `to` ends, else what the two ends of its body state (bodyEndPath).
func flowEndPaths(flow *ast.Usage) (source, target []string) {
	if flow.FlowEnds != nil {
		return endSegments(ast.EndTarget(flow.FlowEnds.From)), endSegments(ast.EndTarget(flow.FlowEnds.To))
	}
	var ends []*ast.Usage
	for _, member := range UnwrappedDeclMembers(flow) {
		if end, ok := member.(*ast.Usage); ok && end.IsEnd {
			ends = append(ends, end)
		}
	}
	if len(ends) != 2 {
		return nil, nil
	}
	return bodyEndPath(ends[0]), bodyEndPath(ends[1])
}

// bodyEndPath is the feature end references followed by the one its first owned feature
// redefines (KerML 8.3.4.9.2, deriveFlowSourceOutputFeature); nil for an end that owns none.
func bodyEndPath(end *ast.Usage) []string {
	var related []string
	for _, rel := range end.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
			related = endSegments(rel.Target)
			break
		}
	}
	if len(related) == 0 {
		return nil
	}
	for _, member := range UnwrappedDeclMembers(end) {
		owned, ok := member.(*ast.Usage)
		if !ok {
			continue
		}
		for _, rel := range owned.Relationships {
			if rel == nil || rel.Kind != ast.RelRedefines {
				continue
			}
			if redefined := endSegments(rel.Target); len(redefined) > 0 {
				return append(related, redefined[len(redefined)-1])
			}
		}
		return nil
	}
	return nil
}
