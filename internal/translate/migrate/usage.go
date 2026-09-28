package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// planUsages marks the block-owned behaviors whose bodies read the block's
// features; they are written as action usages of the block, whose bodies run on
// its objects, since a def nested in the block reaches none of its features.
func (m *migration) planUsages(behaviors []*sysmlv1.Element) {
	var candidates []*sysmlv1.Element
	for _, b := range behaviors {
		if m.usageCandidate(b) {
			candidates = append(candidates, b)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, b := range candidates {
			if !m.asUsage[b] && m.readsOwner(b) {
				m.asUsage[b] = true
				changed = true
			}
		}
	}
	for _, b := range behaviors {
		if !m.asUsage[b] {
			continue
		}
		name, ok := m.usageOf[b]
		if b.Type == "Operation" {
			name, ok = m.opUsage[b]
		}
		if !ok {
			name = m.usageName(b)
		}
		m.names[b] = name
		if b.Type == "Operation" {
			m.opUsage[b] = name
		} else {
			m.usageOf[b] = name
		}
	}
}

// usageName names the usage a behavior of a block is written as: its name in
// lower case as a usage's, or its own name when another member bears that, and
// a numbered one only when both are taken.
func (m *migration) usageName(b *sysmlv1.Element) string {
	name := m.nameFor(b)
	lower := lowerFirst(name)
	if !m.nameTaken(b.Parent, lower) {
		return m.freshName(b.Parent, lower)
	}
	for _, c := range b.Parent.Children {
		if c != b && m.nameOf(c) == name {
			return m.freshName(b.Parent, lower)
		}
	}
	if m.taken[b.Parent][name] {
		return m.freshName(b.Parent, lower)
	}
	m.take(b.Parent, name)
	return name
}

// usageCandidate reports whether b is a behavior a block owns, written as the
// block's action usage: an activity or operation whose body an object of the
// block runs, rather than one acting on another classifier's object.
func (m *migration) usageCandidate(b *sysmlv1.Element) bool {
	if !m.blockOwner(b.Parent) || !m.written(b) {
		return false
	}
	if c, _ := m.classify(b); c != catActionDef {
		return false
	}
	switch b.Type {
	case "Activity", "OpaqueBehavior", "FunctionBehavior", "Interaction":
		// A context through ports excludes a usage; the owner's own does not:
		// the usage's this is the object the context parameter would bind.
		if m.methodOf[b] != nil || contextThroughPorts(m.contextOf(b)) {
			return false
		}
	case "Operation":
		if method := m.bodyMethod(b); method != nil && contextThroughPorts(m.contextOf(method)) {
			return false
		}
	default:
		return false
	}
	invokers := m.invokers[b]
	if method := m.bodyMethod(b); method != nil {
		invokers = append(append([]*sysmlv1.Element{}, invokers...), m.invokers[method]...)
	}
	for _, e := range invokers {
		if !m.reachesUsage(e, b.Parent) {
			return false
		}
	}
	return true
}

// reachesUsage reports whether the invoker e of a behavior of the block owner can
// name a usage of the block: an object of the owner performs it from creation, a
// swimlane's object performs it, or the calling activity runs on an object of the
// owner or holds one. A state's or transition's action cannot, being a member of
// a state def, from which no feature of the block is accessible; nor can a call
// from an activity nested in one.
func (m *migration) reachesUsage(e, owner *sysmlv1.Element) bool {
	if e.Type != "CallBehaviorAction" && e.Type != "CallOperationAction" {
		return !behaviorScope(e)
	}
	host := enclosingActivity(e)
	if host == nil {
		return false
	}
	if e.Type == "CallBehaviorAction" {
		if l, _, _ := m.lanePerformer(e); l != nil {
			return true
		}
	} else if !m.targetIsStatic(host, firstOwned(e, "target")) {
		return false
	}
	selfType, self := classifierOf(host), "this"
	if c := m.contextOf(host); c != nil {
		selfType, self = c.classifier, c.name
	} else if selfType != nil && host.Parent != selfType {
		return false
	}
	obj, _ := m.objectOf(owner, selfType, self)
	return obj != ""
}

// targetIsStatic reports whether what a call's target pin holds is nameable as a
// feature path: this, or a structural feature read, so the call can perform a
// usage on it. An unfed pin holds this; an object created or passed in has no path.
func (m *migration) targetIsStatic(host, t *sysmlv1.Element) bool {
	if t == nil {
		return true
	}
	into := map[*sysmlv1.Element][]*sysmlv1.Element{}
	m.walkActions(host, func(e *sysmlv1.Element) {
		if e.Type != "ObjectFlow" {
			return
		}
		if src, tgt := m.model.Ref(e, "source"), m.model.Ref(e, "target"); src != nil && tgt != nil {
			into[tgt] = append(into[tgt], src)
		}
	})
	seen := map[*sysmlv1.Element]bool{}
	var static func(e *sysmlv1.Element) bool
	static = func(e *sysmlv1.Element) bool {
		if seen[e] {
			return true
		}
		seen[e] = true
		for _, s := range into[e] {
			switch nodeKind(s) {
			case nodePin:
				if s.Parent == nil {
					return false
				}
				switch s.Parent.Type {
				case "ReadSelfAction":
				case "ReadStructuralFeatureAction":
					if !static(firstOwned(s.Parent, "object")) {
						return false
					}
				default:
					return false
				}
			case nodeParam:
				return false
			default:
				if !static(s) {
					return false
				}
			}
		}
		return true
	}
	return static(t)
}

// readsOwner reports whether b's body reaches features of the block owning it:
// its attributes, ports and parts, whether read by an action or by a branch
// probability, or the behaviors written as its usages.
func (m *migration) readsOwner(b *sysmlv1.Element) bool {
	owner := b.Parent
	body := b
	if b.Type == "Operation" {
		if body = m.bodyMethod(b); body == nil {
			return false
		}
	}
	if m.readsFeaturesOf(body, owner) || m.lifelinesOn(body, owner) {
		return true
	}
	for _, o := range m.namedPortOwners(body) {
		if o == owner || m.inherits(owner, o) {
			return true
		}
	}
	reads := false
	m.walkActions(body, func(e *sysmlv1.Element) {
		switch e.Type {
		case "ControlFlow":
			if p := m.probabilityProperty(body, e); p != nil && m.hasFeature(owner, p) {
				reads = true
			}
		case "CallOperationAction":
			if op := m.model.Ref(e, "operation"); op != nil && m.asUsage[op] && m.hasFeature(owner, op) {
				reads = true
			}
		case "CallBehaviorAction":
			if c := m.model.Ref(e, "behavior"); c != nil && c.Parent == owner && m.asUsage[c] {
				reads = true
			}
		}
	})
	return reads
}

func (m *migration) readsFeaturesOf(b, c *sysmlv1.Element) bool {
	uses := false
	m.walkActions(b, func(e *sysmlv1.Element) {
		switch e.Type {
		case "ReadSelfAction":
			uses = true
		case "ReadStructuralFeatureAction", "AddStructuralFeatureValueAction", "RemoveStructuralFeatureValueAction", "ClearStructuralFeatureAction":
			if f := m.model.Ref(e, "structuralFeature"); f != nil && m.hasFeature(c, f) {
				uses = true
			}
		case "OpaqueAction":
			if opaqueUsesOwnerFeature(e, c, m) {
				uses = true
			}
		}
	})
	return uses
}

// probabilityProperty is the property the «Probability» on the edge e of body
// names, as probability resolves it; nil when it carries none or a number.
func (m *migration) probabilityProperty(body, e *sysmlv1.Element) *sysmlv1.Element {
	if m.strict {
		return nil
	}
	s := probability(e)
	if s == nil {
		return nil
	}
	text := strings.TrimSpace(s.Tag("probability"))
	if _, ok := finiteNumber(text); ok || text == "" {
		return nil
	}
	visible, _ := m.visibleFrom(body)
	p := visible[text]
	if p == nil {
		if t := m.model.Lookup(text); t != nil && !t.IsProxy() && visible[m.nameOf(t)] == t {
			p = t
		}
	}
	if p == nil || p.Type != "Property" {
		return nil
	}
	return p
}

// lifelinesOn reports whether a lifeline of the interaction b stands for a part
// or port an object of c reaches.
func (m *migration) lifelinesOn(b, c *sysmlv1.Element) bool {
	if b.Type != "Interaction" {
		return false
	}
	for _, line := range b.Children {
		if line.Type != "Lifeline" {
			continue
		}
		rep := m.model.Ref(line, "represents")
		if rep != nil && (rep.Type == "Property" || rep.Type == "Port") && len(m.partPaths(c, rep)) > 0 {
			return true
		}
	}
	return false
}

// blockOwner reports whether c is a block, whose objects have features a
// behavior's body may read: a part or individual definition.
func (m *migration) blockOwner(c *sysmlv1.Element) bool {
	if c == nil {
		return false
	}
	switch cat, _ := m.classify(c); cat {
	case catPartDef, catIndividualDef:
		return true
	}
	return false
}

// performed reports whether b's usage is performed by every object of its owner
// from creation: the owner's classifier behavior, or a reception's handler.
func (m *migration) performed(b *sysmlv1.Element) bool {
	if !m.asUsage[b] {
		return false
	}
	c := b.Parent
	cb := m.model.Ref(c, "classifierBehavior")
	return cb == b || (cb != nil && m.methodOf[cb] == b)
}

// usageHeader writes the declaration of a behavior written as its owner's action
// usage, whose generals type or subset it. Only a classifier behavior is a
// performed action, which its object runs from creation; any other is a
// composite action, which a call on the object runs.
func (m *migration) usageHeader(e *sysmlv1.Element, cat category, name string) (string, string) {
	var b strings.Builder
	if e.Attrs["isAbstract"] == "true" || m.abstractOperation(e) {
		b.WriteString("abstract ")
	}
	if m.performed(e) {
		b.WriteString("perform ")
	}
	b.WriteString(actionKw + writeName(name))
	gens, n := m.generals(e, cat)
	if gens != "" {
		b.WriteString(" : " + gens)
	}
	if subsets := m.usageGenerals(e); len(subsets) > 0 {
		b.WriteString(" :> " + strings.Join(subsets, ", "))
	}
	return b.String(), n
}

// usageGenerals are the generals of e written as action usages, which the usage
// e subsets; a definition general types it (generals).
func (m *migration) usageGenerals(e *sysmlv1.Element) []string {
	var refs []string
	for _, g := range e.Owned("generalization") {
		if target := m.model.Ref(g, "general"); target != nil && m.asUsage[target] {
			refs = append(refs, m.ref(target, m.scope))
		}
	}
	return refs
}

// usageNote explains a behavior written as its owner's usage.
func (m *migration) usageNote(e *sysmlv1.Element) string {
	owner := qualifiedName(e.Parent)
	if m.performed(e) {
		return "written as an action usage every object of " + owner + " performs from creation, so its body runs on the object and reaches its features"
	}
	return "written as an action usage of " + owner + ", which a call on an object performs, so its body runs on the object and reaches its features"
}

// placeAllocation places the pairs of an Allocate whose ends are both features of
// one definition in that definition's body, where an `allocate` names its ends
// as the features of the object it is written in; the pairs it cannot place
// are left for the dependency's own scope, where a pair of definitions is
// written as an allocation def and any other as a plain dependency.
func (m *migration) placeAllocation(d *sysmlv1.Element) {
	pl := &placement{}
	m.unplaced[d] = pl
	pairs, failed, note := m.dependencyPairs(d)
	pl.failed += failed
	if note != "" {
		pl.notes = append(pl.notes, note)
	}
	for _, p := range pairs {
		scope, from, to := m.allocationEnds(p.client, p.supplier)
		if scope == nil {
			pl.pending = append(pl.pending, p)
			continue
		}
		name := m.nameOf(d)
		if name == "" {
			if base := m.edgeName(d, spoken(from)+" to "+spoken(to)); base != "" {
				name = m.freshName(scope, base)
			}
		} else {
			name, note = m.placedName(scope, name)
			if note != "" {
				pl.notes = append(pl.notes, note)
			}
		}
		var nodes []*sysmlv1.Element
		for _, end := range []*sysmlv1.Element{p.client, p.supplier} {
			if act, _ := m.nodeGraph(end); act != nil {
				nodes = append(nodes, end)
			}
		}
		target := m.placedTarget(scope, name)
		if len(nodes) > 0 {
			pl.nodePairs = append(pl.nodePairs, nodePair{nodes, target})
		}
		m.extras[scope] = append(m.extras[scope], func() {
			m.wroteEdgeAlso(d, scope, "allocation", nil, name)
			alloc := "allocate " + from + " to " + to
			if name != "" {
				alloc = "allocation " + writeName(name) + " " + alloc
				m.madeUp(d, writeName(name))
			}
			m.w.block(alloc, func() { m.metadataUsages(d) })
		})
		pl.write(target)
	}
}

// allocationEnds finds the definition in whose body an allocate between client
// and supplier is written, and the two ends as feature chains from it; nil
// when the ends are not both features of one definition.
func (m *migration) allocationEnds(client, supplier *sysmlv1.Element) (scope *sysmlv1.Element, from, to string) {
	co, cp := m.featurePath(client)
	so, sp := m.featurePath(supplier)
	if co == nil || co != so {
		return nil, "", ""
	}
	return co, featureChain(cp), featureChain(sp)
}

// packageFeature reports whether e is written as a feature of no type: a usage
// a package owns, which an allocate written in a package reaches. A node of a
// behavior, or anything under a definition, is a feature of a type.
func (m *migration) packageFeature(e *sysmlv1.Element) bool {
	if act, _ := m.nodeGraph(e); act != nil {
		return false
	}
	if m.isDefinition(e) {
		return false
	}
	for p := e.Parent; p != nil; p = p.Parent {
		if m.isDefinition(p) || p.Type == "Property" || p.Type == "Port" {
			return false
		}
	}
	return true
}

// featureChain writes a feature chain from the names of the usages along it.
func featureChain(path []string) string {
	names := make([]string, len(path))
	for i, n := range path {
		names[i] = writeName(n)
	}
	return strings.Join(names, ".")
}

// featurePath locates e as a feature of the definition whose connectors reach
// it: the definition and the chain of usage names leading to e. A part's
// connectors are variable features, which reach its attributes, ports, parts
// and performed actions, and chains starting from those; a node of a behavior
// written as the definition's performed action is thus one such feature. A
// node of a composite action usage is not, that usage being constant; nor is a
// node of an action def, or a feature of no definition.
func (m *migration) featurePath(e *sysmlv1.Element) (owner *sysmlv1.Element, path []string) {
	switch e.Type {
	case "Property", "Port":
		if e.Parent != nil && m.isDefinition(e.Parent) && m.written(e) {
			return e.Parent, []string{m.nameFor(e)}
		}
		return nil, nil
	}
	act, _ := m.nodeGraph(e)
	if act == nil {
		return nil, nil
	}
	b := act
	for b.Parent != nil && !m.isDefinition(b.Parent) {
		b = b.Parent
	}
	usage := b
	if op := m.methodOf[b]; op != nil {
		usage = op
	}
	if !m.asUsage[usage] || !m.performed(usage) {
		return nil, nil
	}
	for cur := e; cur != b; cur = cur.Parent {
		path = append([]string{m.nameFor(cur)}, path...)
	}
	return b.Parent, append([]string{m.nameFor(usage)}, path...)
}
