package migrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// A simulation tool stores each run of a configuration as a result snapshot: an
// instance under the resultLocation package whose slots hold the values observed.
// Results indexes them per configuration for the JSON sidecar -migration-results writes.

// Results are the result snapshots of every run configuration of a document,
// in the order the configurations are written.
type Results struct {
	// Source names the v1 document migrated.
	Source         string                 `json:"source"`
	Configurations []ConfigurationResults `json:"configurations"`
}

// ConfigurationResults are one configuration's run settings and the snapshots the tool stored.
type ConfigurationResults struct {
	// ID is the configuration's xmi:id, Name the qualified name of its action def.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Runs is numberOfRuns (0 for none); Draws is durationSimulationMode as a draw policy ("" for none).
	Runs  int64  `json:"runs,omitempty"`
	Draws string `json:"draws,omitempty"`
	// Target is the part holding the execution target, Behavior the action usage performing it; "" for none.
	Target   string `json:"target,omitempty"`
	Behavior string `json:"behavior,omitempty"`
	// Location is the qualified v1 name of the result package, "" for none.
	Location string `json:"resultLocation,omitempty"`
	// Observables are the properties the snapshots hold numbers for, sorted.
	Observables []string   `json:"observables"`
	Snapshots   []Snapshot `json:"snapshots"`
	// Notes say what of the tool's results has no place in the sidecar.
	Notes []string `json:"notes,omitempty"`
}

// Snapshot is one run the tool stored: the numbers its slots hold, by property.
type Snapshot struct {
	ID     string             `json:"id"`
	Name   string             `json:"name,omitempty"`
	Values map[string]float64 `json:"values"`
}

// Values are the numbers every snapshot holds for observable, in snapshot order.
func (c *ConfigurationResults) Values(observable string) []float64 {
	var out []float64
	for _, s := range c.Snapshots {
		if v, ok := s.Values[observable]; ok {
			out = append(out, v)
		}
	}
	return out
}

// Summary counts what the sidecar indexes: configurations, those with snapshots, and snapshots.
func (r *Results) Summary() string {
	stored, snapshots := 0, 0
	for _, c := range r.Configurations {
		if len(c.Snapshots) > 0 {
			stored++
		}
		snapshots += len(c.Snapshots)
	}
	return fmt.Sprintf("results of %d run configuration(s): %d with %d stored snapshot(s)", len(r.Configurations), stored, snapshots)
}

// ReadResults reads a sidecar -migration-results wrote: exactly one JSON document
// indexing at least one configuration; content after it is an error.
func ReadResults(r io.Reader) (*Results, error) {
	var out Results
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: %w", err)
	}
	var trailing json.RawMessage
	switch err := dec.Decode(&trailing); {
	case err == nil:
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: content follows the document")
	case !errors.Is(err, io.EOF):
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: %w", err)
	}
	if out.Source == "" || out.Configurations == nil {
		return nil, fmt.Errorf("the results are not the JSON -migration-results writes: no source or configurations")
	}
	return &out, nil
}

// resultSnapshots reads into r the snapshots of the target's classifiers under the
// resultLocation packages that record the values the target configures; lost says
// what of the results is outside the document.
func (m *migration) resultSnapshots(r *ConfigurationResults, s *xmi.Stereotype, target executionTarget) (lost []string) {
	ids := s.Tags["resultLocation"]
	if len(ids) == 0 {
		return nil
	}
	typed := m.classifierClosure(target.classifiers)
	configured := m.configuredValues(target)
	seenObservable := map[string]bool{}
	unread := map[string]int{}
	others := map[string]int{}
	var locations []string
	for _, id := range ids {
		pkg := m.model.Lookup(id)
		if pkg == nil || pkg.IsProxy() {
			lost = append(lost, "the result location "+strconv.Quote(id)+" is outside the document, so its snapshots are not read")
			continue
		}
		locations = append(locations, qualifiedName(pkg))
		if len(typed) == 0 {
			continue
		}
		for _, inst := range m.descendantInstances(pkg) {
			if !m.isSnapshotOf(inst, typed) {
				continue
			}
			if differ := m.recordsOtherValues(inst, configured); len(differ) > 0 {
				others[strings.Join(differ, ", ")]++
				continue
			}
			snap := Snapshot{ID: inst.ID, Name: inst.Name, Values: map[string]float64{}}
			for _, slot := range inst.Owned("slot") {
				name, value, reason := m.snapshotSlot(slot)
				if reason != "" {
					unread[name+" "+reason]++
					continue
				}
				snap.Values[name] = value
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

// configuredValues is the number the target sets each of its features to — by a
// slot, else by the default of the most special property of its classifiers — that
// a snapshot of a run on the target records too, under the property redefined as
// well. A literal spelling no number sets nothing: that is how a tool leaves the
// observables its runs fill.
func (m *migration) configuredValues(target executionTarget) map[*xmi.Element]float64 {
	configured := map[*xmi.Element]float64{}
	if target.element == nil {
		return configured
	}
	set := func(p *xmi.Element, value float64) {
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
		if len(values) != 1 || strings.TrimSpace(values[0].Attrs["value"]) == "" {
			return
		}
		if value, reason := literalNumber(values[0]); reason == "" {
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

// recordsOtherValues names the configured features whose value inst records
// as another number: such a snapshot is of a run on another configuration.
func (m *migration) recordsOtherValues(inst *xmi.Element, configured map[*xmi.Element]float64) []string {
	var differ []string
	for _, slot := range inst.Owned("slot") {
		f := m.model.Ref(slot, "definingFeature")
		want, ok := configured[f]
		if !ok {
			continue
		}
		if _, value, reason := m.snapshotSlot(slot); reason == "" && value != want {
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

// snapshotSlot reads a slot as the number it holds for its defining feature; reason
// says why it holds none.
func (m *migration) snapshotSlot(slot *xmi.Element) (name string, value float64, reason string) {
	f := m.model.Ref(slot, "definingFeature")
	switch {
	case f == nil:
		return "(" + slot.ID + ")", 0, "names no defining feature in the document"
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
		return name, 0, "holds no value"
	case 1:
	default:
		return name, 0, "holds " + strconv.Itoa(len(values)) + " values, and a result is one number"
	}
	if value, reason = literalNumber(values[0]); reason != "" {
		return name, 0, reason
	}
	if f.IsProxy() {
		return name, value, "is defined outside the document"
	}
	return name, value, ""
}

// literalNumber reads a value specification as the one finite number it holds;
// reason says why it holds none. A numeric literal with no value is zero.
func literalNumber(v *xmi.Element) (value float64, reason string) {
	switch v.Type {
	case "LiteralReal", "LiteralInteger", "LiteralUnlimitedNatural":
	default:
		return 0, "holds a " + v.Type + ", which is no number"
	}
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
