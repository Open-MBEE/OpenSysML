package runtime

import (
	"fmt"

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
	arrived := arrivingInput(graph, node, via)
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

// arrivingInput is the input of node the object flow a token arrived along delivers to,
// empty for a token that arrived along a succession carrying no value.
func arrivingInput(graph *lower.ActionGraph, node ast.Node, via lower.ActionEdge) string {
	if via.Decl == nil {
		return ""
	}
	for _, flow := range graph.DataFlows[via.Source] {
		if flow.Target == node && flow.Decl == via.Decl {
			return flow.TargetPin
		}
	}
	return ""
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
