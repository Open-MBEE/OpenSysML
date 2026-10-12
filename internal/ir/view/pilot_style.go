package view

import (
	"sort"
	"strings"
)

// PilotStyle is a style name the OMG pilot kernel's %viz accepts that no form
// here draws: a PlantUML line routing, a compartment layout, a visibility
// switch of the pilot's own visualizer. Such a style is accepted so a notebook
// written for the pilot runs, and is reported as not represented so it is not
// dropped silently.
type PilotStyle string

// pilotStyles are the pilot's style names, each with the pilot's description
// of what it does.
var pilotStyles = map[PilotStyle]string{
	"STDCOLOR":             "standard style with colors",
	"PLANTUML":             "PlantUML style",
	"POLYLINE":             "polyline style",
	"ORTHOLINE":            "orthogonal line style",
	"SHOWLIB":              "show elements of the standard libraries",
	"SHOWINHERITED":        "show inherited members",
	"COMPMOST":             "show as many memberships in a compartment as possible",
	"COMPTREE":             "show nested ports in a compartment",
	"SHOWIMPORTED":         "show imported elements",
	"HIDEMETADATA":         "hide metadata",
	"SHOWMETACLASS":        "show metaclasses of metaobjects",
	"EVAL":                 "evaluate expressions",
	"NODEMULTIPLICITY":     "show multiplicities in nodes",
	"EDGEMULTIPLICITY":     "show multiplicities on edges",
	"IMPLICITMULTIPLICITY": "show implicit multiplicities",
}

// PilotStyles are the pilot's undrawn style names in alphabetical order, as
// the pilot spells them.
func PilotStyles() []PilotStyle {
	out := make([]PilotStyle, 0, len(pilotStyles))
	for style := range pilotStyles {
		out = append(out, style)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ParsePilotStyle reports the pilot style name names, in any letter case, and
// whether it names one.
func ParsePilotStyle(name string) (PilotStyle, bool) {
	style := PilotStyle(strings.ToUpper(name))
	_, ok := pilotStyles[style]
	return style, ok
}

// Notice is what a rendering reports for a pilot style it was asked for and
// does not draw.
func (s PilotStyle) Notice() string {
	return "style " + string(s) + " (" + pilotStyles[s] + ") is not drawn"
}
