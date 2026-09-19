package migrate

import (
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A simulation tool stores each run of a configuration as a result snapshot: an
// instance under the resultLocation package whose slots hold the values observed.
// resultSnapshots indexes them per configuration for the sidecar -migration-results writes.

// resultSnapshots reads into r the snapshots of the target's classifiers under the
// resultLocation packages that record the values the target configures, each once
// though the locations repeat or nest. A feature two slots of a snapshot hold numbers
// for, or a number no float64 spells exactly, is no result there and is noted; lost
// says what is outside the document.
func (m *migration) resultSnapshots(r *simresults.ConfigurationResults, s *sysmlv1.Stereotype, target executionTarget) (lost []string) {
	ids := s.IDs("resultLocation")
	if len(ids) == 0 {
		return nil
	}
	typed := m.classifierClosure(target.classifiers)
	configured := m.configuredValues(target)
	seenObservable := map[string]bool{}
	seenLocation := map[*sysmlv1.Element]bool{}
	seenInstance := map[*sysmlv1.Element]bool{}
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
				if !value.carried() {
					held[name]++
					unread[name+" holds "+strconv.Quote(value.text)+", which no float64 spells exactly, and a result is a float64"]++
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
func (m *migration) configuredValues(target executionTarget) map[*sysmlv1.Element]scalarValue {
	configured := map[*sysmlv1.Element]scalarValue{}
	if target.element == nil {
		return configured
	}
	set := func(p *sysmlv1.Element, value scalarValue) {
		for queue := []*sysmlv1.Element{p}; len(queue) > 0; queue = queue[1:] {
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
	configures := func(p *sysmlv1.Element, values []*sysmlv1.Element) {
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
func (m *migration) recordsOtherValues(inst *sysmlv1.Element, configured map[*sysmlv1.Element]scalarValue) []string {
	held := map[*sysmlv1.Element][]scalarValue{}
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
		if len(values) == 1 && !values[0].equals(configured[f]) {
			differ = append(differ, f.Name)
		}
	}
	sort.Strings(differ)
	return differ
}

// classifierClosure is the classifiers and every general of theirs.
func (m *migration) classifierClosure(classifiers []*sysmlv1.Element) map[*sysmlv1.Element]bool {
	closure := map[*sysmlv1.Element]bool{}
	for _, c := range m.classifierOrder(classifiers) {
		closure[c] = true
	}
	return closure
}

// classifierOrder is the classifiers and every general of theirs, each special
// before its generals.
func (m *migration) classifierOrder(classifiers []*sysmlv1.Element) []*sysmlv1.Element {
	seen := map[*sysmlv1.Element]bool{}
	var order []*sysmlv1.Element
	queue := append([]*sysmlv1.Element(nil), classifiers...)
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
func (m *migration) descendantInstances(pkg *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
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
	classifiers []*sysmlv1.Element
	config      *sysmlv1.Element
	note        string
}

// indexSnapshots types each classifier-less instance under a run configuration's
// result location by the target classifier its slots prove it a snapshot of.
func (m *migration) indexSnapshots(configs []*sysmlv1.Element) {
	for _, cfg := range configs {
		s := simulationConfig(cfg)
		_, classifiers, _ := m.targetClassifiers(s)
		typed := m.classifierClosure(classifiers)
		seen := map[*sysmlv1.Element]bool{}
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
func (m *migration) snapshotTyping(inst, cfg *sysmlv1.Element, typed map[*sysmlv1.Element]bool) snapshotTyping {
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
	return snapshotTyping{classifiers: []*sysmlv1.Element{special}, config: cfg}
}

// slotOwners lists, in slot order, the classifiers in the document owning the
// defining features of inst's slots, each once.
func (m *migration) slotOwners(inst *sysmlv1.Element) []*sysmlv1.Element {
	var owners []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
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
func (m *migration) mostSpecial(classifiers []*sysmlv1.Element) *sysmlv1.Element {
	for _, c := range classifiers {
		closure := m.classifierClosure([]*sysmlv1.Element{c})
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
func (m *migration) isSnapshotOf(inst *sysmlv1.Element, typed map[*sysmlv1.Element]bool) bool {
	for c := range m.classifierClosure(m.classifiersOf(inst)) {
		if typed[c] {
			return true
		}
	}
	return false
}

// snapshotSlot reads a slot as the one scalar it holds for its defining feature;
// reason says why it holds none.
func (m *migration) snapshotSlot(slot *sysmlv1.Element) (name string, value scalarValue, reason string) {
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
// Boolean, a string or an enumeration literal — with spec the UML kind it was read
// from. A number is held exactly, and as the float64 the sidecar carries.
type scalarValue struct {
	kind   string
	spec   string
	exact  *big.Rat
	number float64
	text   string
}

// equals reports whether two scalars are one value: a number is the same exactly,
// however its literal is spelled, and any other value is of one kind and text.
func (v scalarValue) equals(o scalarValue) bool {
	if v.kind != o.kind {
		return false
	}
	if v.kind == kindNumber {
		return v.exact.Cmp(o.exact) == 0
	}
	return v.text == o.text
}

// carried reports whether a number's float64 denotes it exactly: the shortest
// decimal that reads back as the float64 is the number itself, so no digit is lost.
func (v scalarValue) carried() bool {
	if v.kind != kindNumber {
		return false
	}
	shortest, ok := new(big.Rat).SetString(strconv.FormatFloat(v.number, 'g', -1, 64))
	return ok && shortest.Cmp(v.exact) == 0
}

const (
	kindNumber      = "number"
	kindBoolean     = "boolean"
	kindString      = "string"
	kindEnumLiteral = "literal"
)

// blankLiteral reports whether v is a literal a tool left without a value.
func blankLiteral(v *sysmlv1.Element) bool {
	return strings.HasPrefix(v.Type, "Literal") && strings.TrimSpace(v.Attrs["value"]) == ""
}

// literalScalar reads a value specification as the one scalar it holds; reason says
// why it holds none. A numeric literal with no value is zero, a Boolean one false. An
// instance value is a scalar only when it names an enumeration literal: a snapshot
// holds copies of the parts of its object, which name no configuration.
func (m *migration) literalScalar(v *sysmlv1.Element) (value scalarValue, reason string) {
	value.spec = v.Type
	switch v.Type {
	case "LiteralReal", "LiteralInteger", "LiteralUnlimitedNatural":
		value.kind, value.text = kindNumber, strings.TrimSpace(v.Attrs["value"])
		value.exact, value.number, reason = literalNumber(v)
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

// literalNumber reads a numeric literal as the one finite number it holds, exactly
// and as a float64; reason says why it holds none. A numeric literal with no value is zero.
func literalNumber(v *sysmlv1.Element) (exact *big.Rat, value float64, reason string) {
	text := strings.TrimSpace(v.Attrs["value"])
	if text == "" {
		text = "0"
	}
	if text == "*" {
		return nil, 0, "holds the unbounded natural *, which is no number"
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, 0, "holds " + strconv.Quote(text) + ", which is no finite number"
	}
	if exact, ok := new(big.Rat).SetString(text); ok {
		return exact, value, ""
	}
	return new(big.Rat).SetFloat64(value), value, ""
}
