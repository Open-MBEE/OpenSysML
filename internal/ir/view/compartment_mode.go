package view

import (
	"errors"
	"fmt"
	"strings"
)

// Compartments is which members of a tree rendering fold into compartment
// rows of their owner's box instead of being drawn as nodes of their own.
type Compartments string

const (
	// CompartmentsPositioned folds only the layout-less members of a
	// partially positioned tree, the default.
	CompartmentsPositioned Compartments = "positioned"
	// CompartmentsMost folds every usage member that nothing keeps a node:
	// the pilot's COMPMOST style.
	CompartmentsMost Compartments = "most"
	// CompartmentsPorts folds port usages alone, each listing the ports nested
	// under it: the pilot's COMPTREE style.
	CompartmentsPorts Compartments = "ports"
)

// CompartmentsChoices lists the compartment foldings a caller may ask for.
func CompartmentsChoices() []Compartments {
	return []Compartments{CompartmentsPositioned, CompartmentsMost, CompartmentsPorts}
}

// ParseCompartments reads a folding by name.
func ParseCompartments(name string) (Compartments, bool) {
	if name == "" {
		return CompartmentsPositioned, true
	}
	for _, choice := range CompartmentsChoices() {
		if string(choice) == name {
			return choice, true
		}
	}
	return "", false
}

// CompartmentsNames lists the foldings for a usage message.
func CompartmentsNames() string {
	names := make([]string, 0, len(CompartmentsChoices()))
	for _, choice := range CompartmentsChoices() {
		names = append(names, string(choice))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownCompartments is wrapped by every UnknownCompartmentsError.
var ErrUnknownCompartments = errors.New("unknown compartment folding")

// UnknownCompartmentsError reports a folding that is none of the choices.
type UnknownCompartmentsError struct {
	Name string
}

func (e *UnknownCompartmentsError) Error() string {
	return fmt.Sprintf("unknown compartment folding %q; the foldings are %s", e.Name, CompartmentsNames())
}

func (e *UnknownCompartmentsError) Unwrap() error { return ErrUnknownCompartments }

func (c Compartments) check() error {
	if c == "" {
		return nil
	}
	if _, ok := ParseCompartments(string(c)); !ok {
		return &UnknownCompartmentsError{Name: string(c)}
	}
	return nil
}

// automatic reports a folding that applies whether or not the tree is
// positioned, by what its members are rather than where they are drawn.
func (c Compartments) automatic() bool {
	return c == CompartmentsMost || c == CompartmentsPorts
}
