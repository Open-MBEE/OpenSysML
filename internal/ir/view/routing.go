package view

import (
	"errors"
	"fmt"
	"strings"
)

// Routing is who lays the path of an edge the model does not route itself:
// the libavoid router the diagram panel uses, run in-process so the written
// file carries every bend, or the engine that draws the form. The empty
// Routing is each form's default: RoutingLibavoid for the draw.io form, which
// otherwise leaves the paths to draw.io; the other forms route in their engines.
type Routing string

const (
	// RoutingDefault is each form's default routing.
	RoutingDefault Routing = ""
	// RoutingLibavoid routes every unrouted edge with libavoid, around the
	// boxes and to the ports the form draws, and writes the bends.
	RoutingLibavoid Routing = "libavoid"
	// RoutingEngine leaves unrouted edges to the form's drawing engine.
	RoutingEngine Routing = "engine"
)

// RoutingChoices are the routings there are, in the order help lists them.
func RoutingChoices() []Routing { return []Routing{RoutingLibavoid, RoutingEngine} }

// ParseRouting is the routing a name selects; the empty name is RoutingDefault.
func ParseRouting(name string) (Routing, bool) {
	if name == "" {
		return RoutingDefault, true
	}
	for _, routing := range RoutingChoices() {
		if string(routing) == name {
			return routing, true
		}
	}
	return "", false
}

// RoutingNames spells the routings as a list, for help and error text.
func RoutingNames() string {
	names := make([]string, 0, len(RoutingChoices()))
	for _, routing := range RoutingChoices() {
		names = append(names, string(routing))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownRouting is a routing name that names none. UnknownRoutingError wraps it.
var ErrUnknownRouting = errors.New("unknown edge routing")

// UnknownRoutingError is a routing asked for by a name none has; it names the
// routings there are.
type UnknownRoutingError struct {
	Name string
}

func (e *UnknownRoutingError) Error() string {
	return fmt.Sprintf("unknown edge routing %q; the routings are %s", e.Name, RoutingNames())
}

func (e *UnknownRoutingError) Unwrap() error { return ErrUnknownRouting }

func (r Routing) check() error {
	if _, ok := ParseRouting(string(r)); !ok {
		return &UnknownRoutingError{Name: string(r)}
	}
	return nil
}

// routesDrawio reports whether the draw.io form routes its edges in-process.
func (r Routing) routesDrawio() bool { return r != RoutingEngine }
