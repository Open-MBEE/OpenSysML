package migrate

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// A simulation tool stores each run of a configuration as a result snapshot: an
// instance under the resultLocation package whose slots hold the values observed.
// resultSnapshots indexes them per configuration for the sidecar -migration-results writes.

// resultSnapshots reads into r the snapshots of the target's classifiers under the
// resultLocation packages that record the values the target configures, each once
// though the locations repeat or nest. A feature two slots of a snapshot hold numbers
// for has no one result there and is noted; lost says what is outside the document.
func (m *migration) resultSnapshots(r *simresults.ConfigurationResults, s *xmi.Stereotype, target executionTarget) (lost []string) {
	ids := s.IDs("resultLocation")
	if len(ids) == 0 {
		return nil
	}
	typed := m.classifierClosure(target.classifiers)
	configured := m.configuredValues(target)
	seenObservable := map[string]bool{}
	seenLocation := map[*xmi.Element]bool{}
	seenInstance := map[*xmi.Element]bool{}
	unread := map[string]int{}
	others := map[string]int{}
	var locations []string
	for _, id := range ids {
		pkg := m.model.Lookup(id)
		if pkg == nil || pkg.IsProxy() {
			lost = append(lost, "the result location "+strconv.Quote(id)+" is outside the document, so its snapshots are not read")
			continue
		}
		if seenLocation[pkg] {
			continue
		}
		seenLocation[pkg] = true
		locations = append(locations, qualifiedName(pkg))
		if len(typed) == 0 {
			continue
		}
		for _, inst := range m.descendantInstances(pkg) {
			if seenInstance[inst] || !m.isSnapshotOf(inst, typed) {
				continue
			}
			seenInstance[inst] = true
			if differ := m.recordsOtherValues(inst, configured); len(differ) > 0 {
				others[strings.Join(differ, ", ")]++
				continue
			}
			snap := simresults.Snapshot{ID: inst.ID, Name: inst.Name, Values: map[string]float64{}}
			held := map[string]int{}
			for _, slot := range inst.Owned("slot") {
				name, value, reason := m.snapshotSlot(slot)
				if reason != "" {
					unread[name+" "+reason]++
					continue
				}
				if value.kind != kindNumber {
					unread[name+" holds a "+value.spec+", which is no number"]++
					continue
				}
				snap.Values[name] = value.number
				held[name]++
			}
			for name, n := range held {
				if n > 1 {
					delete(snap.Values, name)
					unread[name+" holds "+strconv.Itoa(n)+" numbers over as many slots, and a result is one number"]++
				}
			}
			for name := range snap.Values {
				seenObservable[name] = true
			}
			r.Snapshots = append(r.Snapshots, snap)
		}
	}
	for name := range seenObservable {
		r.Observables = append(r.Observables, name)
	}
	sort.Strings(r.Observables)
	keys := make([]string, 0, len(unread))
	for k := range unread {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r.Notes = append(r.Notes, "the slot of "+k+" in "+strconv.Itoa(unread[k])+" snapshot(s), so it is not among the results")
	}
	keys = keys[:0]
	for k := range others {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r.Notes = append(r.Notes, strconv.Itoa(others[k])+" snapshot(s) record other values of "+k+" than the target configures, so they are of another configuration and not among the results")
	}
	r.Location = strings.Join(locations, ", ")
	if len(typed) == 0 && len(locations) > 0 {
		r.Notes = append(r.Notes, "the snapshots in "+r.Location+" are not read: the configuration has no target classifier they could be of")
	} else if len(r.Snapshots) == 0 && len(locations) > 0 {
		r.Notes = append(r.Notes, "the result location "+r.Location+" holds no snapshot of the target's classifier")
	}
	return lost
}

// configuredValues is the scalar the target sets each of its features to — by a
// slot, else by the default of the most special property of its classifiers — that
// a snapshot of a run on the target records too, under the property redefined as
// well. A literal spelling no value sets nothing: that is how a tool leaves the
// observables its runs fill.
func (m *migration) configuredValues(target executionTarget) map[*xmi.Element]scalarValue {
	configured := map[*xmi.Element]scalarValue{}
	if target.element == nil {
		return configured
	}
	set := func(p *xmi.Element, value scalarValue) {
		for queue := []*xmi.Element{p}; len(queue) > 0; queue = queue[1:] {
			p := queue[0]
			if _, done := configured[p]; done {
				continue
			}
			configured[p] = value
			queue = append(queue, m.model.Refs(p, "redefinedProperty")...)
			if f, _ := m.shadowed(p); f != nil {
				queue = append(queue, f)
			}
		}
	}
	configures := func(p *xmi.Element, values []*xmi.Element) {
		if len(values) != 1 || blankLiteral(values[0]) {
			return
		}
		if value, reason := m.literalScalar(values[0]); reason == "" {
			set(p, value)
		}
	}
	for _, slot := range target.element.Owned("slot") {
		if f := m.model.Ref(slot, "definingFeature"); f != nil && !f.IsProxy() {
			configures(f, slot.Owned("value"))
		}
	}
	for _, c := range m.classifierOrder(target.classifiers) {
		for _, p := range c.Owned("ownedAttribute") {
			if p.Type == "Property" {
				configures(p, p.Owned("defaultValue"))
			}
		}
	}
	return configured
}

// recordsOtherValues names the configured features whose value inst records, in
// one slot, as another scalar: such a snapshot is of a run on another configuration.
// A feature held by several slots records no one value and is left to resultSnapshots.
func (m *migration) recordsOtherValues(inst *xmi.Element, configured map[*xmi.Element]scalarValue) []string {
	held := map[*xmi.Element][]scalarValue{}
	for _, slot := range inst.Owned("slot") {
		f := m.model.Ref(slot, "definingFeature")
		if _, ok := configured[f]; !ok {
			continue
		}
		if _, value, reason := m.snapshotSlot(slot); reason == "" {
			held[f] = append(held[f], value)
		}
	}
	var differ []string
	for f, values := range held {
		if len(values) == 1 && values[0] != configured[f] {
			differ = append(differ, f.Name)
		}
	}
	sort.Strings(differ)
	return differ
}

// classifierClosure is the classifiers and every general of theirs.
func (m *migration) classifierClosure(classifiers []*xmi.Element) map[*xmi.Element]bool {
	closure := map[*xmi.Element]bool{}
	for _, c := range m.classifierOrder(classifiers) {
		closure[c] = true
	}
	return closure
}

// classifierOrder is the classifiers and every general of theirs, each special
// before its generals.
func (m *migration) classifierOrder(classifiers []*xmi.Element) []*xmi.Element {
	seen := map[*xmi.Element]bool{}
	var order []*xmi.Element
	queue := append([]*xmi.Element(nil), classifiers...)
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil || seen[c] {
			continue
		}
		seen[c] = true
		order = append(order, c)
		for _, g := range c.Owned("generalization") {
			if general := m.model.Ref(g, "general"); general != nil && !general.IsProxy() {
				queue = append(queue, general)
			}
		}
	}
	return order
}

// descendantInstances lists the instance specifications under pkg at any depth, in document order.
func (m *migration) descendantInstances(pkg *xmi.Element) []*xmi.Element {
	var out []*xmi.Element
	var walk func(e *xmi.Element)
	walk = func(e *xmi.Element) {
		for _, c := range e.Children {
			if c.Type == "InstanceSpecification" {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(pkg)
	return out
}

// snapshotTyping is what a classifier-less instance's slots prove under a result
// location: the target classifier it is a snapshot of, or why none.
type snapshotTyping struct {
	classifiers []*xmi.Element
	config      *xmi.Element
	note        string
}

// indexSnapshots types each classifier-less instance under a run configuration's
// result location by the target classifier its slots prove it a snapshot of.
func (m *migration) indexSnapshots(configs []*xmi.Element) {
	for _, cfg := range configs {
		s := simulationConfig(cfg)
		_, classifiers, _ := m.targetClassifiers(s)
		typed := m.classifierClosure(classifiers)
		seen := map[*xmi.Element]bool{}
		for _, id := range s.IDs("resultLocation") {
			pkg := m.model.Lookup(id)
			if pkg == nil || pkg.IsProxy() || seen[pkg] {
				continue
			}
			seen[pkg] = true
			for _, inst := range m.descendantInstances(pkg) {
				if len(m.model.Refs(inst, "classifier")) > 0 {
					continue
				}
				typing := m.snapshotTyping(inst, cfg, typed)
				if prev, ok := m.snapshots[inst]; ok && (prev.classifiers != nil || typing.classifiers == nil) {
					continue
				}
				m.snapshots[inst] = typing
			}
		}
	}
}

// snapshotTyping types inst by the owners of its slots' features when they are one
// lineage ending in one of typed (cfg's target classifiers and their generals), else says why not.
func (m *migration) snapshotTyping(inst, cfg *xmi.Element, typed map[*xmi.Element]bool) snapshotTyping {
	owners := m.slotOwners(inst)
	if len(owners) == 0 {
		return snapshotTyping{config: cfg}
	}
	names := make([]string, len(owners))
	for i, o := range owners {
		names[i] = qualifiedName(o)
	}
	where := " under the result location of the run configuration " + describe(cfg)
	special := m.mostSpecial(owners)
	switch {
	case special == nil:
		return snapshotTyping{config: cfg, note: "its slots are of features of " + strings.Join(names, ", ") + ", none a special of all the others, so no one classifier is inferred" + where}
	case !typed[special]:
		return snapshotTyping{config: cfg, note: "its slots are of features of " + strings.Join(names, ", ") + ", neither a classifier of the configuration's target nor a general of one, so it is no snapshot of a run on it" + where}
	}
	return snapshotTyping{classifiers: []*xmi.Element{special}, config: cfg}
}

// slotOwners lists, in slot order, the classifiers in the document owning the
// defining features of inst's slots, each once.
func (m *migration) slotOwners(inst *xmi.Element) []*xmi.Element {
	var owners []*xmi.Element
	seen := map[*xmi.Element]bool{}
	for _, slot := range inst.Owned("slot") {
		f := m.model.Ref(slot, "definingFeature")
		if f == nil || f.IsProxy() || f.Parent == nil || seen[f.Parent] {
			continue
		}
		switch f.Parent.Type {
		case "Class", "Actor", "DataType", "PrimitiveType", "Enumeration", "Signal", "Interface", "AssociationClass":
		default:
			continue
		}
		seen[f.Parent] = true
		owners = append(owners, f.Parent)
	}
	return owners
}

// mostSpecial is the one of classifiers every other is a general of, or nil
// when they are no one lineage.
func (m *migration) mostSpecial(classifiers []*xmi.Element) *xmi.Element {
	for _, c := range classifiers {
		closure := m.classifierClosure([]*xmi.Element{c})
		lineage := true
		for _, o := range classifiers {
			if !closure[o] {
				lineage = false
				break
			}
		}
		if lineage {
			return c
		}
	}
	return nil
}

// isSnapshotOf reports whether inst is classified, by name or by its slots' features,
// by one of typed or a special of one.
func (m *migration) isSnapshotOf(inst *xmi.Element, typed map[*xmi.Element]bool) bool {
	for c := range m.classifierClosure(m.classifiersOf(inst)) {
		if typed[c] {
			return true
		}
	}
	return false
}

// snapshotSlot reads a slot as the one scalar it holds for its defining feature;
// reason says why it holds none.
func (m *migration) snapshotSlot(slot *xmi.Element) (name string, value scalarValue, reason string) {
	f := m.model.Ref(slot, "definingFeature")
	switch {
	case f == nil:
		return "(" + slot.ID + ")", scalarValue{}, "names no defining feature in the document"
	case f.IsProxy():
		name = f.QualifiedName
		if name == "" {
			name = f.ID
		}
	default:
		name = f.Name
	}
	values := slot.Owned("value")
	switch len(values) {
	case 0:
		return name, scalarValue{}, "holds no value"
	case 1:
	default:
		return name, scalarValue{}, "holds " + strconv.Itoa(len(values)) + " values, and a result is one number"
	}
	if value, reason = m.literalScalar(values[0]); reason != "" {
		return name, scalarValue{}, reason
	}
	if f.IsProxy() {
		return name, value, "is defined outside the document"
	}
	return name, value, ""
}

// scalarValue is the one scalar a value specification spells — a finite number, a
// Boolean, a string or an enumeration literal — with spec the UML kind it was read from.
type scalarValue struct {
	kind   string
	spec   string
	number float64
	text   string
}

const (
	kindNumber      = "number"
	kindBoolean     = "boolean"
	kindString      = "string"
	kindEnumLiteral = "literal"
)

// blankLiteral reports whether v is a literal a tool left without a value.
func blankLiteral(v *xmi.Element) bool {
	return strings.HasPrefix(v.Type, "Literal") && strings.TrimSpace(v.Attrs["value"]) == ""
}

// literalScalar reads a value specification as the one scalar it holds; reason says
// why it holds none. A numeric literal with no value is zero, a Boolean one false. An
// instance value is a scalar only when it names an enumeration literal: a snapshot
// holds copies of the parts of its object, which name no configuration.
func (m *migration) literalScalar(v *xmi.Element) (value scalarValue, reason string) {
	value.spec = v.Type
	switch v.Type {
	case "LiteralReal", "LiteralInteger", "LiteralUnlimitedNatural":
		value.kind = kindNumber
		value.number, reason = literalNumber(v)
	case "LiteralBoolean":
		value.kind = kindBoolean
		switch text := strings.TrimSpace(v.Attrs["value"]); text {
		case "true", "1":
			value.text = "true"
		case "false", "0", "":
			value.text = "false"
		default:
			reason = "holds " + strconv.Quote(text) + ", which is no Boolean"
		}
	case "LiteralString":
		value.kind, value.text = kindString, v.Attrs["value"]
	default:
		if inst := m.model.Ref(v, "instance"); v.Type == "InstanceValue" && inst != nil && inst.Type == "EnumerationLiteral" {
			value.kind, value.text = kindEnumLiteral, inst.ID
			break
		}
		return scalarValue{}, "holds a " + v.Type + ", which is no number"
	}
	if reason != "" {
		return scalarValue{}, reason
	}
	return value, ""
}

// literalNumber reads a numeric literal as the one finite number it holds; reason
// says why it holds none. A numeric literal with no value is zero.
func literalNumber(v *xmi.Element) (value float64, reason string) {
	text := strings.TrimSpace(v.Attrs["value"])
	if text == "" {
		text = "0"
	}
	if text == "*" {
		return 0, "holds the unbounded natural *, which is no number"
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, "holds " + strconv.Quote(text) + ", which is no finite number"
	}
	return value, ""
}
