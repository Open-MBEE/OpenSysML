package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// arrivals indexes the signals reaching each port over the document's connectors and
// inward delegations; v1 hands a signal at any port to the owner, v2 accepts it only `via` the port.
type arrivals struct {
	// at gives the signals arriving at each port.
	at map[*xmi.Element]map[*xmi.Element]bool
	// peers are the ports joined within one assembly; outward and inward the delegations
	// from a part's port to its owner's port and back.
	peers, outward, inward map[*xmi.Element][]*xmi.Element
}

// arrivalIndex builds the index on first use from the sends via a port and the
// connectors the walk collected.
func (m *migration) arrivalIndex() *arrivals {
	if m.arrived != nil {
		return m.arrived
	}
	idx := &arrivals{
		at:      map[*xmi.Element]map[*xmi.Element]bool{},
		peers:   map[*xmi.Element][]*xmi.Element{},
		outward: map[*xmi.Element][]*xmi.Element{},
		inward:  map[*xmi.Element][]*xmi.Element{},
	}
	m.arrived = idx
	peers, outward, inward := idx.peers, idx.outward, idx.inward
	for _, c := range m.connectors {
		if c.Parent == nil {
			continue
		}
		segs, note := m.connectorEnds(c, c.Parent)
		if note != "" {
			continue
		}
		p0, p1 := segs[0][len(segs[0])-1], segs[1][len(segs[1])-1]
		if p0.Type != "Port" || p1.Type != "Port" {
			continue
		}
		own0, own1 := len(segs[0]) == 1, len(segs[1]) == 1
		switch {
		case own0 && !own1:
			outward[p1] = append(outward[p1], p0)
			inward[p0] = append(inward[p0], p1)
		case own1 && !own0:
			outward[p0] = append(outward[p0], p1)
			inward[p1] = append(inward[p1], p0)
		default:
			peers[p0] = append(peers[p0], p1)
			peers[p1] = append(peers[p1], p0)
		}
	}
	sent := map[*xmi.Element]map[*xmi.Element]bool{}
	var leave, reach func(p, sig *xmi.Element)
	leave = func(p, sig *xmi.Element) {
		if sent[p][sig] {
			return
		}
		if sent[p] == nil {
			sent[p] = map[*xmi.Element]bool{}
		}
		sent[p][sig] = true
		for _, q := range peers[p] {
			reach(q, sig)
		}
		for _, q := range outward[p] {
			leave(q, sig)
		}
	}
	reach = func(p, sig *xmi.Element) {
		if idx.at[p][sig] {
			return
		}
		if idx.at[p] == nil {
			idx.at[p] = map[*xmi.Element]bool{}
		}
		idx.at[p][sig] = true
		for _, q := range inward[p] {
			reach(q, sig)
		}
	}
	for _, send := range m.portSends {
		leave(m.model.Ref(send, "onPort"), m.model.Ref(send, "signal"))
	}
	return idx
}

// peerBlocks lists the blocks whose ports connectors join p to, following outward
// and inward delegations; the owners along a delegation are not peers.
func (idx *arrivals) peerBlocks(p *xmi.Element) map[*xmi.Element]bool {
	blocks := map[*xmi.Element]bool{}
	outer := map[*xmi.Element]bool{}
	var up func(*xmi.Element)
	up = func(q *xmi.Element) {
		if outer[q] {
			return
		}
		outer[q] = true
		for _, o := range idx.outward[q] {
			up(o)
		}
	}
	up(p)
	seen := map[*xmi.Element]bool{}
	var down func(*xmi.Element)
	down = func(q *xmi.Element) {
		if seen[q] {
			return
		}
		seen[q] = true
		if q.Parent != nil {
			blocks[q.Parent] = true
		}
		for _, i := range idx.inward[q] {
			down(i)
		}
	}
	for q := range outer {
		for _, peer := range idx.peers[q] {
			down(peer)
		}
	}
	return blocks
}

// sentPorts lists the ports the send signal actions of behavior b and of the
// behaviors it calls go through.
func (m *migration) sentPorts(b *xmi.Element) []*xmi.Element {
	var out []*xmi.Element
	seen := map[*xmi.Element]bool{}
	var walk func(*xmi.Element)
	walk = func(b *xmi.Element) {
		if b == nil || seen[b] {
			return
		}
		seen[b] = true
		m.walkActions(b, func(e *xmi.Element) {
			switch e.Type {
			case "SendSignalAction", "SendObjectAction":
				if p := m.model.Ref(e, "onPort"); p != nil {
					out = append(out, p)
				}
			case "CallBehaviorAction":
				walk(m.model.Ref(e, "behavior"))
			}
		})
	}
	walk(b)
	return out
}

// pairedPorts narrows the ports a signal arrives at to those joined to a block
// the behavior b itself sends to, the route a reply to its request takes.
func (m *migration) pairedPorts(ports []*xmi.Element, b *xmi.Element) []*xmi.Element {
	idx := m.arrivalIndex()
	sentTo := map[*xmi.Element]bool{}
	for _, p := range m.sentPorts(b) {
		for block := range idx.peerBlocks(p) {
			sentTo[block] = true
		}
	}
	var out []*xmi.Element
	for _, p := range ports {
		for block := range idx.peerBlocks(p) {
			if sentTo[block] {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

// arrivalPorts lists the written ports of block c, own and inherited in
// declaration order, at which sig or a special of it arrives.
func (m *migration) arrivalPorts(c, sig *xmi.Element) []*xmi.Element {
	idx := m.arrivalIndex()
	var out []*xmi.Element
	seen := map[*xmi.Element]bool{}
	var walk func(*xmi.Element)
	walk = func(cur *xmi.Element) {
		if cur == nil || seen[cur] {
			return
		}
		seen[cur] = true
		for _, p := range cur.Owned("ownedAttribute") {
			if p.Type != "Port" || !m.written(p) {
				continue
			}
			for arriving := range idx.at[p] {
				if arriving == sig || m.inherits(arriving, sig) {
					out = append(out, p)
					break
				}
			}
		}
		for _, g := range cur.Owned("generalization") {
			walk(m.model.Ref(g, "general"))
		}
	}
	walk(c)
	return out
}

// portRoutes lists the ports a trigger of block c accepts through: the ports it names,
// else the object itself plus each port the signal arrives at. note says which named port is dropped.
func (m *migration) portRoutes(tr, c, sig *xmi.Element) (ports []*xmi.Element, direct bool, info, note string) {
	named := m.model.Refs(tr, "port")
	if len(named) == 0 {
		if c == nil {
			return nil, true, "", ""
		}
		ports = m.arrivalPorts(c, sig)
		if len(ports) == 0 {
			return nil, true, "", ""
		}
		return ports, true, "the signal arrives at the " + m.portNames(ports) + " over the document's connectors, so the trigger is also written accepting via each", ""
	}
	var dropped []string
	for _, p := range named {
		switch {
		case c == nil || !m.hasFeature(c, p):
			dropped = append(dropped, "the trigger's port "+qualifiedName(p)+" is no port of the behavior's owner")
		case !m.written(p):
			dropped = append(dropped, "the trigger's port "+qualifiedName(p)+" has no v2 declaration")
		default:
			ports = append(ports, p)
		}
	}
	note = strings.Join(dropped, "; ")
	if len(ports) == 0 {
		return nil, true, "", joinNotes(note, "the signal is accepted from the object itself instead")
	}
	return ports, false, "the trigger accepts via the " + m.portNames(ports) + " it names", note
}

// actionRoute picks the one route an accept action of b takes: the named port, the sole
// arrival port, or the arrival port joined to a block b sends to; else the object itself.
func (m *migration) actionRoute(clause string, tr, c, b *xmi.Element, via string, sig *xmi.Element) (string, string) {
	ports, direct, _, note := m.portRoutes(tr, c, sig)
	switch {
	case len(ports) == 0:
		return clause, note
	case !direct && len(ports) == 1:
		return clause + " via " + via + writeName(m.nameFor(ports[0])), note
	case !direct:
		return clause + " via " + via + writeName(m.nameFor(ports[0])), joinNotes(note,
			"an action accepts through one route, so of the "+m.portNames(ports)+" the trigger names only the first is written")
	case len(ports) == 1:
		return clause + " via " + via + writeName(m.nameFor(ports[0])), joinNotes(note,
			"the signal arrives at the "+m.portNames(ports)+" over the document's connectors, so the action accepts via it; an action accepts through one route, and one sent to the object itself is not taken")
	}
	if paired := m.pairedPorts(ports, b); len(paired) == 1 {
		return clause + " via " + via + writeName(m.nameFor(paired[0])), joinNotes(note,
			"the signal arrives at the "+m.portNames(ports)+" over the document's connectors; an action accepts through one route, so it accepts via "+
				writeName(m.nameFor(paired[0]))+", the port joined to a block the behavior sends to, and one arriving at another port or sent to the object itself is not taken")
	}
	return clause, joinNotes(note,
		"the signal arrives at the "+m.portNames(ports)+" over the document's connectors; an action accepts through one route, so it takes one sent to the object itself, and one arriving at a port is not taken")
}

// portNames writes "port p" or "ports p, q" with the v2 names of the ports.
func (m *migration) portNames(ports []*xmi.Element) string {
	names := make([]string, len(ports))
	for i, p := range ports {
		names[i] = writeName(m.nameFor(p))
	}
	if len(names) == 1 {
		return "port " + names[0]
	}
	return "ports " + strings.Join(names, ", ")
}
