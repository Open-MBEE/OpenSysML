package analysis

import (
	"fmt"
	"strings"
)

// Listing is one engine as -engines, %engines and ListEngines report it: its
// capability declaration beside the state of its process and where it comes from.
type Listing struct {
	Status
	Description
	Origin
}

// Origin is where an engine comes from: the build for one built in, otherwise the manifest
// entry that registered it and what it runs.
type Origin struct {
	// Kind is the manifest entry's kind; empty for a built-in engine.
	Kind EntryKind
	// Version is the entry's.
	Version string
	// File is the manifest entry; Command the executable it resolved to.
	File    string
	Command string
	// Transport and Protocol are an engine entry's; empty and zero for a tool.
	Transport string
	Protocol  int
}

// Manifested is an engine registered from a manifest entry, which reports its origin.
type Manifested interface {
	External
	Origin() Origin
}

// KindText names the origin's kind as a listing prints it: `built-in` for the build's own.
func (o Origin) KindText() string {
	if o.Kind == "" {
		return "built-in"
	}
	return string(o.Kind)
}

// Listings lists every engine in name order with its description, status and origin.
func (r *Registry) Listings() []Listing {
	engines := r.Engines()
	statuses := r.Statuses()
	listings := make([]Listing, len(engines))
	for i, e := range engines {
		listings[i] = Listing{Status: statuses[i], Description: e.Describe()}
		if m, ok := e.(Manifested); ok {
			listings[i].Origin = m.Origin()
		}
	}
	return listings
}

// Ready reports whether the engine can run: it needs no process, or its process is found.
func (l Listing) Ready() bool { return l.Status.Err == nil }

// StatusText is the engine's state in a word: `ready`, `ready (z3 at /usr/bin/z3)`, or
// `unavailable: <why>`.
func (l Listing) StatusText() string {
	if l.Status.Err != nil {
		return "unavailable: " + l.Status.Err.Error()
	}
	if found := l.Status.Process; found != "" {
		return "ready (" + found + ")"
	}
	return "ready"
}

// Kinds spells the question kinds the engine answers, comma-separated.
func (l Listing) Kinds() string {
	kinds := make([]string, len(l.Questions))
	for i, k := range l.Questions {
		kinds[i] = k.String()
	}
	return strings.Join(kinds, ", ")
}

// Lines tables listings as a report prints them: name, kind, authority, kinds, status.
func Lines(listings []Listing) []string {
	rows := [][]string{{"engine", "kind", "authority", "answers", "status"}}
	for _, l := range listings {
		rows = append(rows, []string{l.Engine, l.KindText(), l.Authority.String(), l.Kinds(), l.StatusText()})
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			widths[i] = max(widths[i], len([]rune(cell)))
		}
	}
	lines := make([]string, len(rows))
	for i, row := range rows {
		cells := make([]string, len(row))
		for j, cell := range row {
			if j == len(row)-1 {
				cells[j] = cell
				continue
			}
			cells[j] = fmt.Sprintf("%-*s", widths[j], cell)
		}
		lines[i] = strings.TrimRight(strings.Join(cells, "  "), " ")
	}
	return lines
}
