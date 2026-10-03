package view

import (
	"errors"
	"fmt"
	"strings"
)

// Ports is how much of a part's ports an interconnection draws. The empty
// Ports is PortsMinimal.
type Ports string

const (
	// PortsMinimal draws the ports a connector of the view ends at and no
	// other, each a small square on its part's border named beside it, so a
	// crowded diagram shows what it connects and nothing more. The default.
	PortsMinimal Ports = "minimal"
	// PortsFull draws every port a part has, labelled `name : Type`.
	PortsFull Ports = "full"
)

// PortsChoices are the port displays a rendering can be asked for, the
// default first.
func PortsChoices() []Ports { return []Ports{PortsMinimal, PortsFull} }

// ParsePorts reports the display name names, and whether it names one. The
// empty name is the default, PortsMinimal.
func ParsePorts(name string) (Ports, bool) {
	if name == "" {
		return PortsMinimal, true
	}
	for _, ports := range PortsChoices() {
		if string(ports) == name {
			return ports, true
		}
	}
	return "", false
}

// PortsNames spells the port displays as a list, for help and error text.
func PortsNames() string {
	names := make([]string, 0, len(PortsChoices()))
	for _, ports := range PortsChoices() {
		names = append(names, string(ports))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownPorts is a port display name that names none. UnknownPortsError
// wraps it.
var ErrUnknownPorts = errors.New("unknown port display")

// UnknownPortsError is a port display asked for by a name none has; it names
// the displays there are.
type UnknownPortsError struct {
	Name string
}

func (e *UnknownPortsError) Error() string {
	return fmt.Sprintf("unknown port display %q; the displays are %s", e.Name, PortsNames())
}

func (e *UnknownPortsError) Unwrap() error { return ErrUnknownPorts }

// check is the UnknownPortsError of a display no registry entry has; the
// empty display and every registered one pass.
func (p Ports) check() error {
	if _, ok := ParsePorts(string(p)); !ok {
		return &UnknownPortsError{Name: string(p)}
	}
	return nil
}

// SupportsPorts reports whether a rendering of the kind draws a part's ports,
// which the Ports display chooses among: the interconnection alone. An
// action's pins are drawn whole whatever the display.
func (k Kind) SupportsPorts() bool { return k == KindInterconnection }

// portView is the ports a form draws of each node under a Ports display, and
// how it names them: every port, labelled `name : Type`, under PortsFull and
// for a kind the display does not apply to — the zero portView — and under
// PortsMinimal those an edge ends at, named alone.
type portView struct {
	// interconnection reports whether the rendering is one, whose ports the
	// text and Mermaid forms write; action reports an action rendering, whose
	// pins the text form writes under their nodes.
	interconnection bool
	action          bool
	minimal         bool
	connected       map[string]bool
}

// portView is the ports the rendering draws under display, the minimal
// display following the rendering's edges.
func (r *Rendering) portView(display Ports) portView { return r.portViewOver(display, r.Edges) }

// portViewOver is the ports the rendering draws under display when a form
// draws edges alone of the rendering's: the minimal display follows those, so
// no pin stands for a connector the form leaves out.
func (r *Rendering) portViewOver(display Ports, edges []Edge) portView {
	v := portView{interconnection: r.Kind == KindInterconnection, action: r.Kind == KindAction, minimal: display != PortsFull && r.Kind.SupportsPorts()}
	if !v.minimal {
		return v
	}
	v.connected = map[string]bool{}
	for _, edge := range edges {
		if edge.FromPort != "" {
			v.connected[edge.FromPort] = true
		}
		if edge.ToPort != "" {
			v.connected[edge.ToPort] = true
		}
	}
	return v
}

// of is the ports of node the form draws, in the node's order.
func (v portView) of(node *Node) []Port {
	if !v.minimal {
		return node.Ports
	}
	var drawn []Port
	for _, port := range node.Ports {
		if v.connected[port.ID] {
			drawn = append(drawn, port)
		}
	}
	return drawn
}

// pinLabel is the text a drawn pin is named by: the whole `name : Type` when
// every port is drawn, the name alone when the connected ones are.
func (v portView) pinLabel(port Port) string {
	if !v.minimal {
		return port.label()
	}
	return port.Name
}

// undrawnPins names the pins of an action rendering a form leaves undrawn, as
// `node.pin` in node then pin order, drawn saying which it draws.
func (r *Rendering) undrawnPins(drawn func(node *Node, port Port) bool) []string {
	var names []string
	var walk func(*Node)
	walk = func(node *Node) {
		for _, port := range node.Ports {
			if !drawn(node, port) {
				names = append(names, nodeLabel(node)+"."+port.Name)
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range r.Roots {
		walk(root)
	}
	return names
}
