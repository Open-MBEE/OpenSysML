package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// carryObjects passes what object flows brought to a fork, join or merge on along the
// flows out of it, each output valued over the inputs; a merge takes the arriving one only.
func (e *performances) carryObjects(frame *actionFrame, node ast.Node, via lower.ActionEdge) error {
	graph := frame.graph
	features := graph.Features[node]
	if len(features) == 0 {
		return nil
	}
	_, merge := node.(*ast.MergeNode)
	var arrived string
	if merge {
		var err error
		if arrived, err = arrivingInput(frame, graph, node, via); err != nil {
			return err
		}
	}
	inputs := make(map[string]Value)
	for _, f := range features {
		if f.Direction != ast.DirIn {
			continue
		}
		inputs[f.Name] = sequenceOf(nil)
		if merge && f.Name != arrived {
			continue
		}
		if value, ok := frame.takeQueued(node, f.Name); ok {
			inputs[f.Name] = value
		}
	}
	produced := make(map[string]Value)
	for _, f := range features {
		if f.Direction != ast.DirOut || f.Value == nil {
			continue
		}
		ec := e.evalContextFor(frame, f.Scope)
		ec.pushFrame(mapFrame(inputs))
		value, err := ec.Eval(f.Value)
		if err != nil {
			return fmt.Errorf("%s: %s: %w", nodeDescription(node), f.Name, err)
		}
		if len(elementsOf(value)) > 0 {
			produced[f.Name] = soleElement(value)
		}
	}
	for _, flow := range graph.DataFlows[node] {
		value, ok := produced[flow.SourcePin]
		if !ok {
			continue
		}
		if err := e.deliverFlow(frame, graph, flow, value); err != nil {
			return err
		}
	}
	return nil
}

// arrivingInput is the input of node a token arrived for: the one the succession flow
// it arrived along delivers to, else the first holding a value of those a flow from
// where it came from delivers to. A token no flow from its source brought takes the
// one input a plain flow left a value at; a succession flow's is its own token's.
func arrivingInput(frame *actionFrame, graph *lower.ActionGraph, node ast.Node, via lower.ActionEdge) (string, error) {
	var from []string
	for _, flow := range graph.DataFlows[via.Source] {
		if flow.Target != node {
			continue
		}
		if via.Decl != nil && flow.Decl == via.Decl {
			return flow.TargetPin, nil
		}
		from = append(from, flow.TargetPin)
	}
	if len(from) > 0 {
		for _, pin := range from {
			if len(frame.pending[node][pin]) > 0 {
				return pin, nil
			}
		}
		return "", nil
	}
	var held []string
	for _, f := range graph.Features[node] {
		if f.Direction == ast.DirIn && len(frame.pending[node][f.Name]) > 0 && !successionFed(graph, node, f.Name) {
			held = append(held, f.Name)
		}
	}
	switch len(held) {
	case 0:
		return "", nil
	case 1:
		return held[0], nil
	}
	return "", fmt.Errorf("%w: %s holds values at %s", ErrAmbiguousMergeInput, nodeDescription(node), strings.Join(held, ", "))
}

// successionFed reports whether a succession flow delivers to node's pin.
func successionFed(graph *lower.ActionGraph, node ast.Node, pin string) bool {
	for _, flows := range graph.DataFlows {
		for _, flow := range flows {
			if flow.Target == node && flow.TargetPin == pin && flow.Kind == lower.FlowSuccession {
				return true
			}
		}
	}
	return false
}

// queueControlObject holds a value an object flow delivers at an input of a fork, join
// or merge until a token passes the node.
func (e *performances) queueControlObject(frame *actionFrame, graph *lower.ActionGraph, flow lower.ObjectFlow, value Value) error {
	for _, f := range graph.Features[flow.Target] {
		if f.Name == flow.TargetPin && f.Direction == ast.DirIn {
			if err := e.ctx.checkNamedWrite(graph.Scopes[flow.Target], nodeDescription(flow.Target), f.Name, &value); err != nil {
				return fmt.Errorf("%s: %w", flowDescription(flow), err)
			}
			frame.queue(flow.Target, f.Name, value)
			return nil
		}
	}
	return fmt.Errorf("%s: %w: %s declares no input %s", flowDescription(flow), ErrNodePin, nodeDescription(flow.Target), flow.TargetPin)
}

// takeQueued removes and returns the oldest value queued at node's pin.
func (f *actionFrame) takeQueued(node ast.Node, pin string) (Value, bool) {
	values := f.pending[node][pin]
	if len(values) == 0 {
		return Value{}, false
	}
	value := values[0]
	if len(values) > 1 {
		f.pending[node][pin] = values[1:]
		return value, true
	}
	delete(f.pending[node], pin)
	if len(f.pending[node]) == 0 {
		delete(f.pending, node)
	}
	return value, true
}
