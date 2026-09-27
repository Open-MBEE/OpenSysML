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
	// Withheld is the typed reason the registry lists the engine but does not run it; nil
	// when it is served.
	Withheld error
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
	// Exchange is a tool's ToolEntry.Protocol; empty for an engine entry.
	Exchange string
}

// Manifested is an engine registered from a manifest entry, which reports its origin.
type Manifested interface {
	External
	Origin() Origin
}

// Prober is an engine that can start its process once to check it against its manifest,
// as -engines -probe asks; Probe answers as Process does, after the handshake.
type Prober interface {
	External
	Probe() (string, error)
}

// KindText names the origin's kind as a listing prints it: `built-in` for the build's own.
func (o Origin) KindText() string {
	if o.Kind == "" {
		return "built-in"
	}
	return string(o.Kind)
}

// ProtocolText names how the engine is spoken to: `-` for one built in, `object` for a tool's
// one JSON object each way, `argv+<stdin>` for one composed from its invocation block — the
// reply's format alone or `argv+<stdin>/<format>` when it is not `object` — and
// `<transport>/<protocol>` for an engine entry.
func (o Origin) ProtocolText() string {
	switch {
	case o.Kind == "":
		return "-"
	case o.Kind == KindTool:
		return o.Exchange
	}
	return fmt.Sprintf("%s/%d", o.Transport, o.Protocol)
}

// Listings lists every engine in name order with its description, status and origin;
// no process is started.
func (r *Registry) Listings() []Listing {
	return r.listings(func(e External) (string, error) { return e.Process() })
}

// Probed lists as Listings does, but starts each external engine's process once and checks
// its handshake against its manifest, reporting the outcome as the status.
func (r *Registry) Probed() []Listing {
	return r.listings(func(e External) (string, error) {
		if p, ok := e.(Prober); ok {
			return p.Probe()
		}
		return e.Process()
	})
}

func (r *Registry) listings(status func(External) (string, error)) []Listing {
	engines := r.Engines()
	listings := make([]Listing, len(engines))
	for i, e := range engines {
		listings[i] = Listing{Status: Status{Engine: e.Name()}, Description: e.Describe()}
		if external, ok := e.(External); ok {
			listings[i].Status.Process, listings[i].Status.Err = status(external)
		}
		if m, ok := e.(Manifested); ok {
			listings[i].Origin = m.Origin()
		}
		if w, ok := e.(Withheld); ok {
			listings[i].Withheld = w.Withheld()
		}
	}
	return listings
}

// Served reports whether the registry runs the engine when a question reaches it.
func (l Listing) Served() bool { return l.Withheld == nil }

// Ready reports whether the engine can run: it is served, and it needs no process or its
// process is found.
func (l Listing) Ready() bool { return l.Served() && l.Status.Err == nil }

// StatusText is the engine's state in a word: `ready`, `ready (z3 at /usr/bin/z3)`,
// `unavailable: <why>` or `withheld: <why>`.
func (l Listing) StatusText() string {
	if l.Withheld != nil {
		return "withheld: " + l.Withheld.Error()
	}
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

// OriginText is the manifest entry's source line, empty for a built-in engine:
// `spin-bridge 1.4.0: engine from /etc/opensysml/engines/spin-bridge.json, runs
// /opt/spin-bridge/bin/spin-bridge, not admitted`.
func (l Listing) OriginText() string {
	if l.Kind == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(l.Engine)
	if l.Version != "" {
		b.WriteString(" " + l.Version)
	}
	fmt.Fprintf(&b, ": %s from %s", l.Kind, l.File)
	if l.Command != "" {
		b.WriteString(", runs " + l.Command)
	}
	if l.Kind == KindEngine {
		b.WriteString(", not admitted")
	}
	return b.String()
}

// Lines tables listings as a report prints them — name, kind, protocol, authority, kinds,
// status — then one source line per manifest entry.
func Lines(listings []Listing) []string {
	rows := [][]string{{"engine", "kind", "protocol", "authority", "answers", "status"}}
	var origins []string
	for _, l := range listings {
		rows = append(rows, []string{l.Engine, l.KindText(), l.ProtocolText(), l.Authority.String(), l.Kinds(), l.StatusText()})
		if origin := l.OriginText(); origin != "" {
			origins = append(origins, origin)
		}
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
	return append(lines, origins...)
}
