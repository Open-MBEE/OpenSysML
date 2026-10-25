package runtime

import (
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// performanceShape is what every performance of an action holds alike: its attributes
// and its root frame's directions, aliases, held declarations and unvalued names.
type performanceShape struct {
	attributes []lower.Attribute
	features   map[string]ast.FeatureDirection
	aliases    map[string]string
	declared   map[*symbols.Symbol]string
	unvalued   map[string]bool
	shadowed   bool
}

// performanceShape is the shape of e's performance of e.graph as e.action: the
// Model's memoized one for a graph it lowered, else one built for e alone.
func (e *ActionExecutor) performanceShape() *performanceShape {
	lowered := e.ctx.model.loweredAs(e.action, e.graph)
	if lowered == nil {
		return e.buildPerformanceShape(new(performanceShape))
	}
	if lowered.shape.features == nil {
		e.buildPerformanceShape(&lowered.shape)
	}
	return &lowered.shape
}

// buildPerformanceShape computes the shape from the graph and the semantic model into shape.
func (e *ActionExecutor) buildPerformanceShape(shape *performanceShape) *performanceShape {
	attributes := e.performanceFeatures()
	root := &actionFrame{graph: e.graph, features: make(map[string]ast.FeatureDirection)}
	e.declareRootFeatures(root, attributes)
	for _, attr := range attributes {
		root.declareUnvalued(attr.Name, attr.Direction, attr.IsResult, attr.Optional, attr.Value)
	}
	*shape = performanceShape{
		attributes: attributes,
		features:   root.features,
		aliases:    root.aliases,
		declared:   root.declared,
		unvalued:   root.unvalued,
		shadowed:   root.shadowed,
	}
	return shape
}
