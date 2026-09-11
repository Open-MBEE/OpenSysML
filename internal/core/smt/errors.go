// Package smt is the `smt` analysis engine: it encodes the moves of an action's
// token flow as a transition relation over SMT terms, asks a solver whether a
// requirement can fail or a schedule can deadlock within k moves, and replays
// every witness through the interpreter, which stays normative.
//
// The stage implemented here covers actions on concrete inputs: straight-line
// bodies, fork, join, merge, decisions, body loops with bounded unrolling, pins
// and object flows. Messages, the clock, nested flows and free inputs are not
// encoded; a behavior using them is refused with a typed reason before any query.
package smt

import (
	"errors"
	"fmt"
)

// ErrNotEncoded is the typed error for a construct this engine does not encode.
// The behavior is reported not covered; nothing is approximated.
var ErrNotEncoded = errors.New("smt: not encoded")

// ErrMalformedFlow is the typed error for an action flow the interpreter would
// refuse to run: no initial node, a join with two successors, and the like.
var ErrMalformedFlow = errors.New("smt: malformed action flow")

// ErrSlotOverflow is the typed error for a flow whose forks may put more tokens
// in flight within k moves than the encoding has slots for.
var ErrSlotOverflow = errors.New("smt: token slots exceeded")

// UnsupportedError names the node and the construct that keep a behavior out
// of the encoding, and why. It is ErrNotEncoded.
type UnsupportedError struct {
	// Node is the action node the construct belongs to, as a trace names it.
	Node string
	// Construct is what was found: "accept", "send", "nested flow", …
	Construct string
	// Reason says which stage of the design encodes it, or why nothing does.
	Reason string
}

func (e *UnsupportedError) Error() string {
	if e.Node == "" {
		return fmt.Sprintf("%v: %s: %s", ErrNotEncoded, e.Construct, e.Reason)
	}
	return fmt.Sprintf("%v: node %s: %s: %s", ErrNotEncoded, e.Node, e.Construct, e.Reason)
}

// Is makes an UnsupportedError match ErrNotEncoded.
func (e *UnsupportedError) Is(target error) bool { return target == ErrNotEncoded }

// FlowError names the node whose shape the interpreter would refuse. It is
// ErrMalformedFlow.
type FlowError struct {
	Node   string
	Reason string
}

func (e *FlowError) Error() string {
	if e.Node == "" {
		return fmt.Sprintf("%v: %s", ErrMalformedFlow, e.Reason)
	}
	return fmt.Sprintf("%v: node %s: %s", ErrMalformedFlow, e.Node, e.Reason)
}

// Is makes a FlowError match ErrMalformedFlow.
func (e *FlowError) Is(target error) bool { return target == ErrMalformedFlow }
