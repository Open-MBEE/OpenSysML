package runtime

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// nodeObject is a value a control node holds at an input: flow is the declaration
// that brought it, carried where that is a succession flow, whose own token takes it.
type nodeObject struct {
	value   Value
	flow    ast.Node
	carried bool
}

// carryObjects passes what object flows brought to a fork, join or merge on along the
// flows out of it, each output valued over the inputs; a merge takes the arriving one only.
func (e *performances) carryObjects(frame *actionFrame, node ast.Node, via lower.ActionEdge) error {
	graph := frame.graph
	features := graph.Features[node]
	if len(features) == 0 {
		return nil
	}
	inputs := make(map[string]Value)
	for _, f := range features {
		if f.Direction == ast.DirIn {
			inputs[f.Name] = sequenceOf(nil)
		}
	}
	if _, merge := node.(*ast.MergeNode); merge {
		pin, at, err := arrivingObject(frame, graph, node, via)
		if err != nil {
			return err
		}
		if at >= 0 {
			inputs[pin] = frame.takeHeld(node, pin, at)
		}
	} else {
		for _, f := range features {
			if f.Direction == ast.DirIn && len(frame.held[node][f.Name]) > 0 {
				inputs[f.Name] = frame.takeHeld(node, f.Name, 0)
			}
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

// arrivingObject locates the value a token arriving at merge node along via takes: the
// one its succession flow brought, else the oldest at the one input plain flows from its source
// left values at, else, for a token no flow came with, the one input any plain flow left values at.
// at is -1 where there is none.
func arrivingObject(frame *actionFrame, graph *lower.ActionGraph, node ast.Node, via lower.ActionEdge) (pin string, at int, err error) {
	var from []lower.ObjectFlow
	for _, flow := range graph.DataFlows[via.Source] {
		if flow.Target != node {
			continue
		}
		if via.Carries && flow.Kind == lower.FlowSuccession && flow.Decl == via.Decl {
			if at := frame.heldFrom(node, flow.TargetPin, flow.Decl); at >= 0 {
				return flow.TargetPin, at, nil
			}
			continue
		}
		if flow.Kind != lower.FlowSuccession {
			from = append(from, flow)
		}
	}
	var held []string
	first := make(map[string]int)
	for _, flow := range from {
		at := frame.heldFrom(node, flow.TargetPin, flow.Decl)
		if at < 0 {
			continue
		}
		if was, seen := first[flow.TargetPin]; !seen {
			held = append(held, flow.TargetPin)
			first[flow.TargetPin] = at
		} else if at < was {
			first[flow.TargetPin] = at
		}
	}
	switch {
	case len(held) == 1:
		return held[0], first[held[0]], nil
	case len(held) > 1:
		return "", -1, fmt.Errorf("%w: %s holds values at %s", ErrAmbiguousMergeInput, nodeDescription(node), strings.Join(held, ", "))
	case len(from) > 0 || via.Carries:
		return "", -1, nil
	}
	for _, f := range graph.Features[node] {
		if f.Direction == ast.DirIn && slices.ContainsFunc(frame.held[node][f.Name], func(h nodeObject) bool { return !h.carried }) {
			held = append(held, f.Name)
		}
	}
	switch len(held) {
	case 0:
		return "", -1, nil
	case 1:
		return held[0], slices.IndexFunc(frame.held[node][held[0]], func(h nodeObject) bool { return !h.carried }), nil
	}
	return "", -1, fmt.Errorf("%w: %s holds values at %s", ErrAmbiguousMergeInput, nodeDescription(node), strings.Join(held, ", "))
}

// queueControlObject holds a value an object flow delivers at an input of a fork, join
// or merge until a token passes the node.
func (e *performances) queueControlObject(frame *actionFrame, graph *lower.ActionGraph, flow lower.ObjectFlow, value Value) error {
	for _, f := range graph.Features[flow.Target] {
		if f.Name == flow.TargetPin && f.Direction == ast.DirIn {
			if err := e.ctx.checkNamedWrite(graph.Scopes[flow.Target], nodeDescription(flow.Target), f.Name, &value); err != nil {
				return fmt.Errorf("%s: %w", flowDescription(flow), err)
			}
			frame.hold(flow.Target, f.Name, nodeObject{value: value, flow: flow.Decl, carried: flow.Kind == lower.FlowSuccession})
			return nil
		}
	}
	return fmt.Errorf("%s: %w: %s declares no input %s", flowDescription(flow), ErrNodePin, nodeDescription(flow.Target), flow.TargetPin)
}

// hold appends a value to those node holds at pin.
func (f *actionFrame) hold(node ast.Node, pin string, object nodeObject) {
	if f.held == nil {
		f.held = make(map[ast.Node]map[string][]nodeObject)
	}
	if f.held[node] == nil {
		f.held[node] = make(map[string][]nodeObject)
	}
	f.held[node][pin] = append(f.held[node][pin], object)
}

// heldFrom is the position of the oldest value flow brought that node holds at pin; -1 if none.
func (f *actionFrame) heldFrom(node ast.Node, pin string, flow ast.Node) int {
	return slices.IndexFunc(f.held[node][pin], func(h nodeObject) bool { return h.flow == flow })
}

// takeHeld removes and returns the value node holds at pin's position at.
func (f *actionFrame) takeHeld(node ast.Node, pin string, at int) Value {
	objects := f.held[node][pin]
	value := objects[at].value
	if len(objects) > 1 {
		f.held[node][pin] = slices.Delete(slices.Clone(objects), at, at+1)
		return value
	}
	delete(f.held[node], pin)
	if len(f.held[node]) == 0 {
		delete(f.held, node)
	}
	return value
}
