package migrate

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// portReceiver writes the operation usage a call over port performs: the part
// the caller's connectors join to its own port, or the target object's port when
// the port is the target's. The note says why the call is not written that way.
func (a *activity) portReceiver(port, t, op *xmi.Element) (receiver, note string, ok bool) {
	if !a.m.written(port) {
		return "", "the call runs in the caller's context: the port " + qualifiedName(port) + " it goes through has no v2 declaration", false
	}
	usage := writeName(a.m.operationUsage(op))
	if a.hasPort(port) {
		path, why := a.m.connectedReceiver(a.selfType(), port, op)
		if why != "" {
			return "", "the call runs in the caller's context: " + why, false
		}
		return a.on(a.self(), path+"."+usage), "the call performs the usage " + usage + " of the part connected to the port " + a.m.nameFor(port), true
	}
	obj, typ, found := a.objectOf(t)
	switch {
	case t == nil || !found:
		return "", "the call runs in the caller's context: the port " + qualifiedName(port) + " it goes through is no port of the caller, and no flow names the target object", false
	case typ == nil:
		return "", "the call runs in the caller's context: the target " + obj + " has no known type to hold the port " + a.m.nameFor(port), false
	case !a.m.hasFeature(typ, port):
		return "", "the call runs in the caller's context: the target " + obj + " is a " + qualifiedName(typ) + ", which has no port " + a.m.nameFor(port), false
	}
	path := a.on(obj, writeName(a.m.nameFor(port)))
	if pt := a.m.model.Ref(port, "type"); pt != nil && a.m.hasFeature(pt, op) {
		return path + "." + usage, "the call performs the usage " + usage + " of the target's port " + path, true
	}
	if !a.m.hasFeature(typ, op) {
		return "", "the call runs in the caller's context: neither the target " + obj + " nor its port " + a.m.nameFor(port) + " has the operation " + a.m.nameOf(op), false
	}
	if obj == a.self() {
		return a.on(obj, usage), "the target is " + obj + ", whose usage " + usage + " the call performs; its port " + a.m.nameFor(port) + " is not written, as a v2 perform names the operation on the object", true
	}
	return a.on(obj, usage), "the call performs the usage " + usage + " of the target " + obj + "; its port " + a.m.nameFor(port) + " is not written, as a v2 perform names the operation on the object", true
}

// connectedReceiver follows the connectors of classifier c from its port to the
// path of the part, or the part's port, whose type has operation op; the reason
// when none does, or when several do, since the source names no one of them.
func (m *migration) connectedReceiver(c, port, op *xmi.Element) (string, string) {
	joined := 0
	var paths []string
	for _, cn := range m.connectorsOf(c) {
		segs, note := m.connectorEnds(cn, c)
		if note != "" {
			continue
		}
		for i, end := range segs {
			if len(end) != 1 || end[0] != port {
				continue
			}
			joined++
			other := segs[1-i]
			if path, ok := m.operationHolder(other, op); ok && !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	switch {
	case joined == 0:
		return "", "no connector of " + qualifiedName(c) + " joins its port " + m.nameFor(port) + " to a part"
	case len(paths) == 1:
		return paths[0], ""
	case len(paths) > 1:
		return "", "the port " + m.nameFor(port) + " of " + qualifiedName(c) + " connects to several parts whose types have the operation " + m.nameOf(op) + " (" + strings.Join(paths, ", ") + "), and the call names no one of them"
	}
	return "", "the port " + m.nameFor(port) + " of " + qualifiedName(c) + " connects to no part whose type has the operation " + m.nameOf(op)
}

// operationHolder writes the longest prefix of a connector end's path whose last
// segment's type has op: the port's own path when its type declares the operation,
// else the part's the port belongs to.
func (m *migration) operationHolder(end []*xmi.Element, op *xmi.Element) (string, bool) {
	for n := len(end); n > 0; n-- {
		t := m.model.Ref(end[n-1], "type")
		if t == nil || t.IsProxy() || !m.hasFeature(t, op) {
			continue
		}
		parts := make([]string, n)
		for i, s := range end[:n] {
			if !m.written(s) {
				return "", false
			}
			parts[i] = writeName(m.nameFor(s))
		}
		return strings.Join(parts, "."), true
	}
	return "", false
}

// connectorsOf lists the connectors classifier c and its generals own, nearest first.
func (m *migration) connectorsOf(c *xmi.Element) []*xmi.Element {
	var out []*xmi.Element
	seen := map[*xmi.Element]bool{}
	var walk func(*xmi.Element)
	walk = func(cur *xmi.Element) {
		if cur == nil || seen[cur] {
			return
		}
		seen[cur] = true
		out = append(out, cur.Owned("ownedConnector")...)
		for _, g := range cur.Owned("generalization") {
			walk(m.model.Ref(g, "general"))
		}
	}
	walk(c)
	return out
}
