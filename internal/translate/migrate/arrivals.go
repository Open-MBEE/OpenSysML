package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// signalArrives opens the note a duplicate acceptance route carries.
const signalArrives = "the signal arrives at the "

// arrivals indexes the signals reaching each port over the document's connectors and
// inward delegations; v1 hands a signal at any port to the owner, v2 accepts it only `via` the port.
type arrivals struct {
	// at gives the signals the document sends to each port; carried the signal types its
	// declarations let in, which any special of them may arrive as.
	at      map[*sysmlv1.Element]map[*sysmlv1.Element]bool
	carried map[*sysmlv1.Element]map[*sysmlv1.Element]bool
	// peers are the ports joined within one assembly; outward and inward the delegations
	// from a part's port to its owner's port and back.
	peers, outward, inward map[*sysmlv1.Element][]*sysmlv1.Element
}

// arrivalIndex builds the index on first use from the sends via a port, the ports' and
// item flows' declarations, and the connectors the walk collected.
func (m *migration) arrivalIndex() *arrivals {
	if m.arrived != nil {
		return m.arrived
	}
	idx := &arrivals{
		at:      map[*sysmlv1.Element]map[*sysmlv1.Element]bool{},
		carried: map[*sysmlv1.Element]map[*sysmlv1.Element]bool{},
		peers:   map[*sysmlv1.Element][]*sysmlv1.Element{},
		outward: map[*sysmlv1.Element][]*sysmlv1.Element{},
		inward:  map[*sysmlv1.Element][]*sysmlv1.Element{},
	}
	m.arrived = idx
	conveyed := m.portDelegations(idx)
	idx.propagate(m, conveyed)
	return idx
}

// portDelegations fills idx's peer and delegation edges from the port connectors and
// lists, as port and signal pairs, the signals their item flows convey to an end port.
func (m *migration) portDelegations(idx *arrivals) [][2]*sysmlv1.Element {
	peers, outward, inward := idx.peers, idx.outward, idx.inward
	var conveyed [][2]*sysmlv1.Element
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
		conveyed = append(conveyed, m.conveyedTo(c, p0, p1)...)
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
	return conveyed
}

// propagate spreads the sends and declared arrivals through idx's delegations: a send
// reaches a peer's arrival set and travels outward; declared and conveyed signals are carried.
func (idx *arrivals) propagate(m *migration, conveyed [][2]*sysmlv1.Element) {
	w := &arrivalWalk{idx: idx, sent: map[*sysmlv1.Element]map[*sysmlv1.Element]bool{}}
	for _, send := range m.portSends {
		w.leave(m.model.Ref(send, "onPort"), m.model.Ref(send, "signal"))
	}
	for _, p := range m.ports {
		for _, sig := range m.declaredArrivals(p) {
			w.reach(idx.carried, p, sig)
		}
	}
	for _, pair := range conveyed {
		w.reach(idx.carried, pair[0], pair[1])
	}
}

// arrivalWalk carries the send set while a propagation walks the delegations.
type arrivalWalk struct {
	idx  *arrivals
	sent map[*sysmlv1.Element]map[*sysmlv1.Element]bool
}

// reach records sig arriving at p and follows inward delegations.
func (w *arrivalWalk) reach(into map[*sysmlv1.Element]map[*sysmlv1.Element]bool, p, sig *sysmlv1.Element) {
	if into[p][sig] {
		return
	}
	if into[p] == nil {
		into[p] = map[*sysmlv1.Element]bool{}
	}
	into[p][sig] = true
	for _, q := range w.idx.inward[p] {
		w.reach(into, q, sig)
	}
}

// leave records sig sent from p, reaches its peers, and travels outward.
func (w *arrivalWalk) leave(p, sig *sysmlv1.Element) {
	if w.sent[p][sig] {
		return
	}
	if w.sent[p] == nil {
		w.sent[p] = map[*sysmlv1.Element]bool{}
	}
	w.sent[p][sig] = true
	for _, q := range w.idx.peers[p] {
		w.reach(w.idx.at, q, sig)
	}
	for _, q := range w.idx.outward[p] {
		w.leave(q, sig)
	}
}

// conveyedTo lists, as port and signal, the signals the item flows connector c realizes
// convey to one of its end ports p0 and p1, the flow's information target.
func (m *migration) conveyedTo(c, p0, p1 *sysmlv1.Element) [][2]*sysmlv1.Element {
	var out [][2]*sysmlv1.Element
	for _, f := range m.flows[c] {
		for _, t := range m.model.Refs(f, "informationTarget") {
			if t != p0 && t != p1 {
				continue
			}
			for _, sig := range m.model.Refs(f, "conveyed") {
				if sig.Type == "Signal" {
					out = append(out, [2]*sysmlv1.Element{t, sig})
				}
			}
		}
	}
	return out
}

// declaredArrivals lists the signals port p's type lets in, generals included: flow properties
// flowing in (out when conjugated) and, unconjugated, the receptions it or a realized interface has.
func (m *migration) declaredArrivals(p *sysmlv1.Element) []*sysmlv1.Element {
	conjugated := p.Attrs["isConjugated"] == "true"
	var out []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	var walk func(*sysmlv1.Element)
	walk = func(t *sysmlv1.Element) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		out = append(out, m.flowSignals(t, conjugated)...)
		if !conjugated {
			out = append(out, m.receptionSignals(t)...)
		}
		m.arrivalSupertypes(t, conjugated, walk)
	}
	walk(m.model.Ref(p, "type"))
	return out
}

// receptionSignals lists the signals of t's owned receptions.
func (m *migration) receptionSignals(t *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	for _, r := range t.Owned("ownedReception") {
		if sig := m.model.Ref(r, "signal"); sig != nil {
			out = append(out, sig)
		}
	}
	return out
}

// arrivalSupertypes walks t's realized interface contracts — unconjugated only —
// and its general classifiers.
func (m *migration) arrivalSupertypes(t *sysmlv1.Element, conjugated bool, walk func(*sysmlv1.Element)) {
	if !conjugated {
		for _, ir := range t.Owned("interfaceRealization") {
			walk(m.model.Ref(ir, "contract"))
		}
	}
	for _, g := range t.Owned("generalization") {
		walk(m.model.Ref(g, "general"))
	}
}

// flowSignals lists the signal types of t's flow properties that arrive when conjugation
// reads as given: flowing in, or out when conjugated.
func (m *migration) flowSignals(t *sysmlv1.Element, conjugated bool) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	for _, a := range t.Owned("ownedAttribute") {
		fp := stereo(a, "FlowProperty")
		if fp == nil {
			continue
		}
		dir := fp.Tag("direction")
		if dir == "in" && conjugated || dir == "out" && !conjugated {
			continue
		}
		if sig := m.model.Ref(a, "type"); sig != nil && sig.Type == "Signal" {
			out = append(out, sig)
		}
	}
	return out
}

// arrivesAt reports whether sig, or a signal a trigger for sig also accepts, arrives at port p:
// the document sends it or a special there, or the port carries it, a general or a special.
func (m *migration) arrivesAt(p, sig *sysmlv1.Element) bool {
	idx := m.arrivalIndex()
	for arriving := range idx.at[p] {
		if arriving == sig || m.inherits(arriving, sig) {
			return true
		}
	}
	for carried := range idx.carried[p] {
		if carried == sig || m.inherits(carried, sig) || m.inherits(sig, carried) {
			return true
		}
	}
	return false
}

// peerBlocks lists the blocks whose ports connectors join p to, following outward
// and inward delegations; the owners along a delegation are not peers.
func (idx *arrivals) peerBlocks(p *sysmlv1.Element) map[*sysmlv1.Element]bool {
	blocks := map[*sysmlv1.Element]bool{}
	outer := map[*sysmlv1.Element]bool{}
	var up func(*sysmlv1.Element)
	up = func(q *sysmlv1.Element) {
		if outer[q] {
			return
		}
		outer[q] = true
		for _, o := range idx.outward[q] {
			up(o)
		}
	}
	up(p)
	seen := map[*sysmlv1.Element]bool{}
	var down func(*sysmlv1.Element)
	down = func(q *sysmlv1.Element) {
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
func (m *migration) sentPorts(b *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	var walk func(*sysmlv1.Element)
	walk = func(b *sysmlv1.Element) {
		if b == nil || seen[b] {
			return
		}
		seen[b] = true
		m.walkActions(b, func(e *sysmlv1.Element) {
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
func (m *migration) pairedPorts(ports []*sysmlv1.Element, b *sysmlv1.Element) []*sysmlv1.Element {
	idx := m.arrivalIndex()
	sentTo := map[*sysmlv1.Element]bool{}
	for _, p := range m.sentPorts(b) {
		for block := range idx.peerBlocks(p) {
			sentTo[block] = true
		}
	}
	var out []*sysmlv1.Element
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
func (m *migration) arrivalPorts(c, sig *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	var walk func(*sysmlv1.Element)
	walk = func(cur *sysmlv1.Element) {
		if cur == nil || seen[cur] {
			return
		}
		seen[cur] = true
		for _, p := range cur.Owned("ownedAttribute") {
			if p.Type == "Port" && m.written(p) && m.arrivesAt(p, sig) {
				out = append(out, p)
			}
		}
		for _, g := range cur.Owned("generalization") {
			walk(m.model.Ref(g, "general"))
		}
	}
	walk(c)
	return out
}

// openPorts lists the written ports of block c, own and inherited, at which sig is not known
// to arrive and whose type declares no flow property or reception, so what reaches them is unsaid.
func (m *migration) openPorts(c, sig *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	var walk func(*sysmlv1.Element)
	walk = func(cur *sysmlv1.Element) {
		if cur == nil || seen[cur] {
			return
		}
		seen[cur] = true
		for _, p := range cur.Owned("ownedAttribute") {
			if p.Type == "Port" && m.written(p) && !m.arrivesAt(p, sig) && !m.declaresCarriage(m.model.Ref(p, "type")) {
				out = append(out, p)
			}
		}
		for _, g := range cur.Owned("generalization") {
			walk(m.model.Ref(g, "general"))
		}
	}
	walk(c)
	return out
}

// declaresCarriage reports whether port type t, or a general or realized interface of
// it, declares a flow property or reception, saying what its ports carry.
func (m *migration) declaresCarriage(t *sysmlv1.Element) bool {
	seen := map[*sysmlv1.Element]bool{}
	var walk func(*sysmlv1.Element) bool
	walk = func(t *sysmlv1.Element) bool {
		if t == nil || seen[t] {
			return false
		}
		seen[t] = true
		for _, a := range t.Owned("ownedAttribute") {
			if has(a, "FlowProperty") {
				return true
			}
		}
		if len(t.Owned("ownedReception")) > 0 {
			return true
		}
		for _, ir := range t.Owned("interfaceRealization") {
			if walk(m.model.Ref(ir, "contract")) {
				return true
			}
		}
		for _, g := range t.Owned("generalization") {
			if walk(m.model.Ref(g, "general")) {
				return true
			}
		}
		return false
	}
	return walk(t)
}

// portRoutes lists the ports a trigger of block c accepts through: the ports it names,
// else the object itself plus each port the signal arrives at; info and note explain the rest.
func (m *migration) portRoutes(tr, c, sig *sysmlv1.Element) (ports []*sysmlv1.Element, direct bool, info, note string) {
	named := m.model.Refs(tr, "port")
	if len(named) == 0 {
		if c == nil {
			return nil, true, "", ""
		}
		ports, info = m.arrivalRoutes(c, sig, "trigger")
		return ports, true, info, ""
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

// arrivalRoutes lists the ports of block c the signal arrives at, which an acceptor of
// kind what also accepts via; info says so and names the ports nothing is known to reach.
func (m *migration) arrivalRoutes(c, sig *sysmlv1.Element, what string) (ports []*sysmlv1.Element, info string) {
	ports = m.arrivalPorts(c, sig)
	if len(ports) > 0 {
		info = signalArrives + m.portNames(ports) + " over the document's connectors or declarations, so the " + what + " is also written accepting via each"
	}
	if open := m.openPorts(c, sig); len(open) > 0 {
		info = joinNotes(info, "nothing in the document declares or sends a signal to the "+m.portNames(open)+", so one arriving there is not accepted")
	}
	return ports, info
}

// actionRoute picks the one route an accept action of b takes: the named port, the sole
// arrival port, or the arrival port joined to a block b sends to; else the object itself.
func (m *migration) actionRoute(clause string, tr, c, b *sysmlv1.Element, via string, sig *sysmlv1.Element) (string, string) {
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
			signalArrives+m.portNames(ports)+" over the document's connectors or declarations, so the action accepts via it; an action accepts through one route, and one sent to the object itself is not taken")
	}
	if paired := m.pairedPorts(ports, b); len(paired) == 1 {
		return clause + " via " + via + writeName(m.nameFor(paired[0])), joinNotes(note,
			signalArrives+m.portNames(ports)+" over the document's connectors or declarations; an action accepts through one route, so it accepts via "+
				writeName(m.nameFor(paired[0]))+", the port joined to a block the behavior sends to, and one arriving at another port or sent to the object itself is not taken")
	}
	return clause, joinNotes(note,
		signalArrives+m.portNames(ports)+" over the document's connectors or declarations; an action accepts through one route, so it takes one sent to the object itself, and one arriving at a port is not taken")
}

// portNames writes "port p" or "ports p, q" with the v2 names of the ports.
func (m *migration) portNames(ports []*sysmlv1.Element) string {
	names := make([]string, len(ports))
	for i, p := range ports {
		names[i] = writeName(m.nameFor(p))
	}
	if len(names) == 1 {
		return "port " + names[0]
	}
	return "ports " + strings.Join(names, ", ")
}
