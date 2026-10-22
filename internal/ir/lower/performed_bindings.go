package lower

import "slices"

// PerformedPinBindings lowers the bindings an object writes through the feature it
// performs graph as (`bind run.measure.report = theReport;`, prefix [run]) to pin
// bindings of graph's nodes, as the same bindings written in the action's body lower.
// Scope stays the object's, so the other end resolves where it was written.
func PerformedPinBindings(graph *ActionGraph, prefix []string, bindings []Binding) ([]PinBinding, error) {
	nodes := nodesNamed(graph.Nodes)
	var out []PinBinding
	for _, binding := range bindings {
		for i, end := range binding.Ends {
			segments, ok := throughPrefix(endSegments(end.Expr), prefix)
			if !ok || len(segments) < 2 {
				continue
			}
			node, path, pin, err := pinPathOf(graph, nodes, end.Expr, segments)
			if err != nil {
				return nil, err
			}
			if node == nil {
				continue
			}
			other := binding.Ends[1-i].Expr
			pinBinding := PinBinding{
				Node: node, Path: path, Pin: pin, Other: other, Scope: binding.Scope, Decl: binding.Decl,
			}
			if otherSegments, ok := throughPrefix(endSegments(other), prefix); ok && len(otherSegments) >= 2 {
				pinBinding.OtherNode, pinBinding.OtherPath, pinBinding.OtherPin, err = pinPathOf(graph, nodes, other, otherSegments)
				if err != nil {
					return nil, err
				}
			}
			if pinBinding.OtherNode == nil {
				if chain, feature, ok := assignTarget(other); ok {
					pinBinding.OtherChain, pinBinding.OtherFeature = chain, feature
				}
			}
			out = append(out, pinBinding)
		}
	}
	return out, nil
}

// throughPrefix returns the segments of an end past prefix, ok when the end
// walks through every name of prefix in order.
func throughPrefix(segments, prefix []string) ([]string, bool) {
	if len(prefix) == 0 || len(segments) <= len(prefix) || !slices.Equal(segments[:len(prefix)], prefix) {
		return nil, false
	}
	return segments[len(prefix):], true
}
