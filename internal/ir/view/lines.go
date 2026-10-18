package view

import (
	"errors"
	"fmt"
	"strings"
)

// Lines is how a graph-shaped form routes its edges, where the form's engine
// takes a routing: Graphviz's `splines` on the DOT form and PlantUML's
// `linetype`, the two routings the Pilot visualizer's ORTHOLINE and POLYLINE
// styles ask for. The empty Lines is LinesSplines.
type Lines string

const (
	// LinesSplines leaves each engine its default, curved routing. The default.
	LinesSplines Lines = "splines"
	// LinesOrtho routes every edge as axis-parallel segments, as the Pilot's
	// ORTHOLINE style does.
	LinesOrtho Lines = "ortho"
	// LinesPolyline routes every edge as straight segments, as the Pilot's
	// POLYLINE style does.
	LinesPolyline Lines = "polyline"
)

// LinesChoices are the routings a rendering can be asked for, the default first.
func LinesChoices() []Lines { return []Lines{LinesSplines, LinesOrtho, LinesPolyline} }

// ParseLines reports the routing name names, and whether it names one. The
// empty name is the default, LinesSplines.
func ParseLines(name string) (Lines, bool) {
	if name == "" {
		return LinesSplines, true
	}
	for _, lines := range LinesChoices() {
		if string(lines) == name {
			return lines, true
		}
	}
	return "", false
}

// LinesNames spells the routings as a list, for help and error text.
func LinesNames() string {
	names := make([]string, 0, len(LinesChoices()))
	for _, lines := range LinesChoices() {
		names = append(names, string(lines))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownLines is a routing name that names none. UnknownLinesError wraps it.
var ErrUnknownLines = errors.New("unknown line routing")

// UnknownLinesError is a routing asked for by a name none has; it names the
// routings there are.
type UnknownLinesError struct {
	Name string
}

func (e *UnknownLinesError) Error() string {
	return fmt.Sprintf("unknown line routing %q; the routings are %s", e.Name, LinesNames())
}

func (e *UnknownLinesError) Unwrap() error { return ErrUnknownLines }

// check is the UnknownLinesError of a routing no registry entry has; the empty
// routing and every registered one pass.
func (l Lines) check() error {
	if _, ok := ParseLines(string(l)); !ok {
		return &UnknownLinesError{Name: string(l)}
	}
	return nil
}

// routed reports whether the routing asks for other than the engine's default.
func (l Lines) routed() bool { return l != "" && l != LinesSplines }

// dotAttribute is the Graphviz graph attribute drawing the routing; empty for
// the default, which writes none.
func (l Lines) dotAttribute() string {
	if !l.routed() {
		return ""
	}
	return "splines=" + string(l)
}

// plantumlStatement is the PlantUML statement drawing the routing; empty for
// the default, which writes none.
func (l Lines) plantumlStatement() string {
	if !l.routed() {
		return ""
	}
	return "skinparam linetype " + string(l)
}

// linesNotice is the notice a form that routes no edges itself writes for a
// routing asked for, so the request is not dropped silently.
func linesNotice(lines Lines, form Form) string {
	return fmt.Sprintf("lines %s; %s routes no edge itself, the dot and plantuml forms do", lines, form)
}
