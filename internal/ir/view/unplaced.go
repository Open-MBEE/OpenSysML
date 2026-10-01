package view

import (
	"errors"
	"fmt"
	"strings"
)

// Unplaced is what the DOT form does with the nodes a positioned drawing leaves
// unplaced: those with no Layout, no route meeting them and, for a cluster, no
// placed member. The empty Unplaced is UnplacedOmit.
type Unplaced string

const (
	// UnplacedOmit leaves them undrawn: the drawing shows what the Layouts
	// place, and nothing lands on a placed box.
	UnplacedOmit Unplaced = "omit"
	// UnplacedStrip draws them in rows in a strip below the drawing, clear of
	// the canvas, every placed box and every route.
	UnplacedStrip Unplaced = "strip"
)

// UnplacedChoices are the placements unplaced nodes can be asked for, in the
// order they are offered.
func UnplacedChoices() []Unplaced { return []Unplaced{UnplacedOmit, UnplacedStrip} }

// ParseUnplaced reports the placement name names, and whether it names one. The
// empty name is no placement: the default is asked for by stating none.
func ParseUnplaced(name string) (Unplaced, bool) {
	for _, choice := range UnplacedChoices() {
		if string(choice) == name {
			return choice, true
		}
	}
	return "", false
}

// UnplacedNames spells the placements as a list, for help and error text.
func UnplacedNames() string {
	names := make([]string, 0, len(UnplacedChoices()))
	for _, choice := range UnplacedChoices() {
		names = append(names, string(choice))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownUnplaced is a placement name that names none. UnknownUnplacedError
// wraps it.
var ErrUnknownUnplaced = errors.New("unknown placement of unplaced nodes")

// UnknownUnplacedError is a placement asked for by a name none has; it names the
// placements there are, so no surface refuses one without saying which.
type UnknownUnplacedError struct {
	Name string
}

func (e *UnknownUnplacedError) Error() string {
	return fmt.Sprintf("unknown placement %q of unplaced nodes; the placements are %s", e.Name, UnplacedNames())
}

func (e *UnknownUnplacedError) Unwrap() error { return ErrUnknownUnplaced }

// check is the UnknownUnplacedError of an Unplaced value that names no
// placement; the empty one and every named one pass.
func (u Unplaced) check() error {
	if u == "" {
		return nil
	}
	if _, ok := ParseUnplaced(string(u)); !ok {
		return &UnknownUnplacedError{Name: string(u)}
	}
	return nil
}
