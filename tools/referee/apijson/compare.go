package apijson

import (
	"reflect"
	"sort"
	"strings"
)

// Bucket names, as the report and the baseline spell them.
const (
	bucketPresentEqual     = "present-equal"
	bucketPresentDifferent = "present-different"
	bucketOursMissing      = "ours-missing"
	bucketPilotMissing     = "pilot-missing"
	bucketAbsentBoth       = "absent-both"
	bucketIndeterminate    = "indeterminate"
	// bucketPilotNondeterministic scores every comparison of a property the pinned
	// pilot exports differently from one JVM to the next; see nondeterminism.go.
	bucketPilotNondeterministic = "pilot-nondeterministic"
)

// Buckets counts the property comparisons of one scope by outcome. AbsentBoth
// and Indeterminate are kept apart from the four buckets the scorecard ranks: a
// property both sides leave empty is agreement, but not evidence that either
// derives it, and a reference to a target neither key table addresses can be
// called neither equal nor different.
type Buckets struct {
	PresentEqual     int `json:"presentEqual"`
	PresentDifferent int `json:"presentDifferent"`
	OursMissing      int `json:"oursMissing"`
	PilotMissing     int `json:"pilotMissing"`
	AbsentBoth       int `json:"absentBoth"`
	Indeterminate    int `json:"indeterminate"`
	// PilotNondeterministic counts the comparisons of a declared property whose
	// pilot value is not reproducible, so neither side's value is scored.
	PilotNondeterministic int `json:"pilotNondeterministic"`
}

func (b *Buckets) add(bucket string) {
	switch bucket {
	case bucketPresentEqual:
		b.PresentEqual++
	case bucketPresentDifferent:
		b.PresentDifferent++
	case bucketOursMissing:
		b.OursMissing++
	case bucketPilotMissing:
		b.PilotMissing++
	case bucketAbsentBoth:
		b.AbsentBoth++
	case bucketIndeterminate:
		b.Indeterminate++
	case bucketPilotNondeterministic:
		b.PilotNondeterministic++
	}
}

func (b *Buckets) addAll(other Buckets) {
	b.PresentEqual += other.PresentEqual
	b.PresentDifferent += other.PresentDifferent
	b.OursMissing += other.OursMissing
	b.PilotMissing += other.PilotMissing
	b.AbsentBoth += other.AbsentBoth
	b.Indeterminate += other.Indeterminate
	b.PilotNondeterministic += other.PilotNondeterministic
}

// Differing is the number of comparisons that found the two sides apart.
func (b Buckets) Differing() int { return b.PresentDifferent + b.OursMissing + b.PilotMissing }

// ElementTotals counts the elements of one scope by how they matched.
type ElementTotals struct {
	Matched          int `json:"matched"`
	OursOnly         int `json:"oursOnly"`
	PilotOnly        int `json:"pilotOnly"`
	PilotOnlyImplied int `json:"pilotOnlyImplied"`
	TypeMismatch     int `json:"typeMismatch"`
}

func (t *ElementTotals) addAll(other ElementTotals) {
	t.Matched += other.Matched
	t.OursOnly += other.OursOnly
	t.PilotOnly += other.PilotOnly
	t.PilotOnlyImplied += other.PilotOnlyImplied
	t.TypeMismatch += other.TypeMismatch
}

// Row is the scorecard entry of one (metaclass, property) pair.
type Row struct {
	Metaclass string `json:"metaclass"`
	Property  string `json:"property"`
	Buckets
}

// Example is one concrete comparison behind a scorecard row.
type Example struct {
	Metaclass string `json:"metaclass"`
	Property  string `json:"property"`
	Bucket    string `json:"bucket"`
	Root      string `json:"root"`
	File      string `json:"file"`
	Element   string `json:"element"`
	Ours      string `json:"ours"`
	Pilot     string `json:"pilot"`
}

// Unmatched lists, per side and metaclass, elements the other side has no key
// for, with the first few keys.
type Unmatched struct {
	Side      string   `json:"side"`
	Metaclass string   `json:"metaclass"`
	Count     int      `json:"count"`
	Elements  []string `json:"elements"`
}

const unmatchedExamples = 3

// comparison accumulates the outcome of matching one or more roots.
type comparison struct {
	unmatched map[string]*Unmatched
	matches   map[string]*RootMatches
	elements  ElementTotals
	buckets   Buckets
	rows      map[string]*Row
	examples  map[string]*Example
	// unkeyedReferences counts references that compared through the unkeyed
	// sentinel on either side.
	unkeyedReferences int
	// extensions counts our properties outside the metamodel (the sysx: ones),
	// which are left out of the comparison.
	extensions int
}

func newComparison() *comparison {
	return &comparison{rows: make(map[string]*Row), examples: make(map[string]*Example), unmatched: make(map[string]*Unmatched), matches: make(map[string]*RootMatches)}
}

// compareRoot matches one root's elements by key and buckets every property of
// every matched pair; the pilot's metaclass names the row when the two differ.
func (c *comparison) compareRoot(root string, ours, pilot []*element) (ElementTotals, Buckets) {
	var elements ElementTotals
	var buckets Buckets
	byKey := make(map[string]*element, len(ours))
	for _, e := range ours {
		byKey[e.Key] = e
	}
	matched := make(map[string]bool, len(pilot))
	inventory := newRootMatches()
	c.matches[root] = inventory
	sort.Slice(pilot, func(i, j int) bool { return pilot[i].Key < pilot[j].Key })
	for _, p := range pilot {
		o := byKey[p.Key]
		if o == nil {
			elements.PilotOnly++
			if p.Implied {
				elements.PilotOnlyImplied++
			}
			c.noteUnmatched("pilot", root, p)
			inventory.PilotOnly[p.Key] = p.Type
			continue
		}
		matched[p.Key] = true
		inventory.Matched[p.Key] = p.Type
		elements.Matched++
		if o.Type != p.Type {
			elements.TypeMismatch++
		}
		for _, property := range unionProperties(o.Props, p.Props) {
			if strings.HasPrefix(property, "sysx:") {
				c.extensions++
				continue
			}
			if property == "elementId" {
				continue
			}
			bucket := bucketOf(property, o.Props[property], p.Props[property])
			if isPilotNondeterministic(p.Type, property) {
				bucket = bucketPilotNondeterministic
			}
			buckets.add(bucket)
			c.record(root, p, property, bucket, o.Props[property], p.Props[property])
		}
	}
	sort.Slice(ours, func(i, j int) bool { return ours[i].Key < ours[j].Key })
	for _, o := range ours {
		if !matched[o.Key] {
			elements.OursOnly++
			c.noteUnmatched("ours", root, o)
			inventory.OursOnly[o.Key] = o.Type
		}
	}
	c.elements.addAll(elements)
	c.buckets.addAll(buckets)
	return elements, buckets
}

func unionProperties(a, b map[string]any) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		set[k] = struct{}{}
	}
	for k := range b {
		set[k] = struct{}{}
	}
	return sortedKeys(set)
}

// booleanDefaults are the Boolean attributes whose defaultValueLiteral is not false in
// the pinned jar's model/kerml.ecore and model/SysML.ecore; the rest default to false.
var booleanDefaults = map[string]bool{"isUnique": true}

// undefaultedBooleans have no default in those models (the derived ones, isVariation
// and a LiteralBoolean's value), so leaving one out states nothing.
var undefaultedBooleans = map[string]bool{
	"isConjugated": true, "isLibraryElement": true, "isModelLevelEvaluable": true,
	"isReference": true, "mayTimeVary": true, "isVariation": true, "value": true,
}

// booleanDefault is the value an export states for a Boolean attribute by
// leaving it out, when the metamodel declares one.
func booleanDefault(property string) (value bool, ok bool) {
	if undefaultedBooleans[property] {
		return false, false
	}
	return booleanDefaults[property], true
}

// bucketOf classifies one property: null, a missing key, an empty collection and a
// Boolean at its metamodel default state nothing; stated values go to compareValues.
func bucketOf(property string, ours, pilot any) string {
	if value, ok := booleanDefault(property); ok {
		if pilot == value && ours == nil {
			pilot = nil
		}
		if ours == value && pilot == nil {
			ours = nil
		}
	}
	oursAbsent, pilotAbsent := isAbsent(ours), isAbsent(pilot)
	switch {
	case oursAbsent && pilotAbsent:
		return bucketAbsentBoth
	case oursAbsent:
		return bucketOursMissing
	case pilotAbsent:
		return bucketPilotMissing
	}
	return compareValues(ours, pilot)
}

// compareValues compares two stated values by length and position: a differing
// position decides, and only when none does do unknown targets leave the verdict open.
func compareValues(ours, pilot any) string {
	switch o := ours.(type) {
	case ref:
		p, ok := pilot.(ref)
		switch {
		case !ok:
			return bucketPresentDifferent
		case o.Key == unkeyed || p.Key == unkeyed:
			return bucketIndeterminate
		case o.Key == p.Key:
			return bucketPresentEqual
		}
		return bucketPresentDifferent
	case string:
		p, ok := pilot.(string)
		switch {
		case !ok:
			return bucketPresentDifferent
		case o == unkeyed || p == unkeyed:
			return bucketIndeterminate
		case o == p:
			return bucketPresentEqual
		}
		return bucketPresentDifferent
	case []any:
		p, ok := pilot.([]any)
		if !ok || len(o) != len(p) {
			return bucketPresentDifferent
		}
		verdict := bucketPresentEqual
		for i := range o {
			switch compareValues(o[i], p[i]) {
			case bucketPresentDifferent:
				return bucketPresentDifferent
			case bucketIndeterminate:
				verdict = bucketIndeterminate
			}
		}
		return verdict
	}
	if reflect.DeepEqual(ours, pilot) {
		return bucketPresentEqual
	}
	return bucketPresentDifferent
}

func (c *comparison) record(root string, p *element, property, bucket string, ours, pilot any) {
	id := p.Type + "\x00" + property
	row := c.rows[id]
	if row == nil {
		row = &Row{Metaclass: p.Type, Property: property}
		c.rows[id] = row
	}
	row.add(bucket)
	if mentionsUnkeyed(ours) || mentionsUnkeyed(pilot) {
		c.unkeyedReferences++
	}
	if bucket == bucketPresentEqual || bucket == bucketAbsentBoth || bucket == bucketIndeterminate || bucket == bucketPilotNondeterministic {
		return
	}
	// The first example per row in root, file and key order is deterministic
	// because the roots and the pilot's elements are visited sorted.
	if _, seen := c.examples[id]; !seen {
		c.examples[id] = &Example{
			Metaclass: p.Type, Property: property, Bucket: bucket,
			Root: root, File: p.File, Element: p.Key,
			Ours: render(ours, 160), Pilot: render(pilot, 160),
		}
	}
}

func (c *comparison) noteUnmatched(side, root string, e *element) {
	id := side + "\x00" + e.Type
	entry := c.unmatched[id]
	if entry == nil {
		entry = &Unmatched{Side: side, Metaclass: e.Type}
		c.unmatched[id] = entry
	}
	entry.Count++
	if len(entry.Elements) < unmatchedExamples {
		entry.Elements = append(entry.Elements, root+": "+e.Key)
	}
}

// unmatchedList lists the unmatched metaclasses, the most frequent first.
func (c *comparison) unmatchedList() []Unmatched {
	list := make([]Unmatched, 0, len(c.unmatched))
	for _, entry := range c.unmatched {
		list = append(list, *entry)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count != list[j].Count {
			return list[i].Count > list[j].Count
		}
		if list[i].Side != list[j].Side {
			return list[i].Side < list[j].Side
		}
		return list[i].Metaclass < list[j].Metaclass
	})
	return list
}

func mentionsUnkeyed(v any) bool {
	switch value := v.(type) {
	case ref:
		return value.Key == unkeyed
	case string:
		return value == unkeyed
	case []any:
		for _, item := range value {
			if mentionsUnkeyed(item) {
				return true
			}
		}
	}
	return false
}

// scorecard lists the rows, the most differing first, then by name.
func (c *comparison) scorecard() []Row {
	rows := make([]Row, 0, len(c.rows))
	for _, row := range c.rows {
		rows = append(rows, *row)
	}
	sortRows(rows, Row.Differing)
	return rows
}

// sortRows orders rows by a count, largest first, then by metaclass and property.
func sortRows(rows []Row, count func(Row) int) {
	sort.Slice(rows, func(i, j int) bool {
		if d := count(rows[i]) - count(rows[j]); d != 0 {
			return d > 0
		}
		if rows[i].Metaclass != rows[j].Metaclass {
			return rows[i].Metaclass < rows[j].Metaclass
		}
		return rows[i].Property < rows[j].Property
	})
}

// exampleList lists the examples in scorecard order.
func (c *comparison) exampleList(rows []Row) []Example {
	examples := make([]Example, 0, len(c.examples))
	for _, row := range rows {
		if example := c.examples[row.Metaclass+"\x00"+row.Property]; example != nil {
			examples = append(examples, *example)
		}
	}
	return examples
}
