package migrate

import (
	"math"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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
	scan := &snapshotScan{
		typed:          m.classifierClosure(target.classifiers),
		configured:     m.configuredValues(target),
		seenInstance:   map[*sysmlv1.Element]bool{},
		seenObservable: map[string]bool{},
		unread:         map[string]int{},
		others:         map[string]int{},
		statistics:     map[string]int{},
		ranOn:          map[string]int{},
	}
	scan.analysed, scan.analysisNote = m.monteCarloObservable(target.classifiers)
	if scan.analysed != nil {
		r.Analysis = scan.analysed.Name
	}
	seenLocation := map[*sysmlv1.Element]bool{}
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
		if len(scan.typed) == 0 {
			continue
		}
		for _, inst := range m.descendantInstances(pkg) {
			m.instanceSnapshot(r, inst, s, target, scan)
		}
	}
	m.snapshotNotes(r, scan, locations)
	return lost
}

// snapshotScan gathers the snapshots of one location scan: the values the
// target configures, the observable the target's analysis summarises, what was
// already seen, and why slots or snapshots were no results.
type snapshotScan struct {
	typed          map[*sysmlv1.Element]bool
	configured     map[*sysmlv1.Element]scalarValue
	analysed       *sysmlv1.Element
	analysisNote   string
	seenInstance   map[*sysmlv1.Element]bool
	seenObservable map[string]bool
	unread         map[string]int
	others         map[string]int
	statistics     map[string]int
	ranOn          map[string]int
}

// instanceSnapshot reads one snapshot instance into r; it is skipped when
// already seen, of no target classifier, or of another configuration. One of no
// classifier that names a run on another classifier is counted as that.
func (m *migration) instanceSnapshot(r *simresults.ConfigurationResults, inst *sysmlv1.Element, s *sysmlv1.Stereotype, target executionTarget, scan *snapshotScan) {
	if scan.seenInstance[inst] {
		return
	}
	if !m.isSnapshotOf(inst, target.classifiers, scan.typed) {
		if ran := m.ranOnOther(inst, s, scan.typed); ran != nil && len(m.model.Refs(inst, "classifier")) == 0 {
			scan.seenInstance[inst] = true
			scan.ranOn[ranOnOtherNote(inst, ran)]++
		}
		return
	}
	scan.seenInstance[inst] = true
	if differ := m.recordsOtherValues(inst, scan.configured); len(differ) > 0 {
		scan.others[strings.Join(differ, ", ")]++
		return
	}
	snap := simresults.Snapshot{ID: inst.ID, Name: inst.Name, Values: map[string]float64{}}
	held := map[string]int{}
	summary, unread := m.slotValues(inst, &snap, held, scan)
	for name, n := range held {
		if n > 1 {
			delete(snap.Values, name)
			scan.unread[name+" holds "+strconv.Itoa(n)+" numbers over as many slots, and a result is one number"]++
		}
	}
	if scan.analysed != nil {
		stats, note, foreign := monteCarloStatistics(scan.analysed.Name, summary, unread, snap.Values)
		if note != "" {
			scan.statistics[note]++
		}
		if foreign {
			return
		}
		if stats != nil {
			snap.Statistics = stats
			scan.seenObservable[scan.analysed.Name] = true
		}
	}
	for name := range snap.Values {
		scan.seenObservable[name] = true
	}
	r.Snapshots = append(r.Snapshots, snap)
}

// slotValues reads each slot of inst into snap.Values; a slot whose name, kind
// or number is no result is counted in scan.unread under its reason. The slots of
// the analysis's own statistics are returned instead, by statistic; unread marks
// one of them holding no number.
func (m *migration) slotValues(inst *sysmlv1.Element, snap *simresults.Snapshot, held map[string]int, scan *snapshotScan) (summary map[string]float64, unread bool) {
	summary = map[string]float64{}
	for _, slot := range inst.Owned("slot") {
		if stat, value, reason := m.monteCarloSlot(slot); stat != "" {
			switch {
			case scan.analysed == nil:
				scan.unread[monteCarloAnalysisBlock+"::"+stat+" holds a statistic of no observable the target analyses"]++
			case reason != "":
				scan.unread[monteCarloAnalysisBlock+"::"+stat+" "+reason]++
				unread = true
			default:
				summary[stat] = value
			}
			continue
		}
		name, value, reason := m.snapshotSlot(slot)
		if reason != "" {
			scan.unread[name+" "+reason]++
			continue
		}
		if value.kind != kindNumber {
			scan.unread[name+" holds a "+value.spec+", which is no number"]++
			continue
		}
		if !value.carried() {
			held[name]++
			scan.unread[name+" holds "+strconv.Quote(value.text)+", which no float64 spells exactly, and a result is a float64"]++
			continue
		}
		snap.Values[name] = value.number
		held[name]++
	}
	return summary, unread
}

// snapshotNotes records on r the scan's observables and its notes.
func (m *migration) snapshotNotes(r *simresults.ConfigurationResults, scan *snapshotScan, locations []string) {
	if scan.analysisNote != "" {
		r.Notes = append(r.Notes, scan.analysisNote)
	}
	for name := range scan.seenObservable {
		r.Observables = append(r.Observables, name)
	}
	sort.Strings(r.Observables)
	for _, k := range sortedKeys(scan.unread) {
		r.Notes = append(r.Notes, "the slot of "+k+" in "+strconv.Itoa(scan.unread[k])+" snapshot(s), so it is not among the results")
	}
	for _, k := range sortedKeys(scan.others) {
		r.Notes = append(r.Notes, strconv.Itoa(scan.others[k])+" snapshot(s) record other values of "+k+" than the target configures, so they are of another configuration and not among the results")
	}
	for _, k := range sortedKeys(scan.statistics) {
		r.Notes = append(r.Notes, strconv.Itoa(scan.statistics[k])+" snapshot(s) "+k)
	}
	for _, k := range sortedKeys(scan.ranOn) {
		r.Notes = append(r.Notes, strconv.Itoa(scan.ranOn[k])+" snapshot(s) are not among the results: "+k+" in the result location")
	}
	r.Location = strings.Join(locations, ", ")
	if len(scan.typed) == 0 && len(locations) > 0 {
		r.Notes = append(r.Notes, "the snapshots in "+r.Location+" are not read: the configuration has no target classifier they could be of")
	} else if len(r.Snapshots) == 0 && len(locations) > 0 {
		r.Notes = append(r.Notes, "the result location "+r.Location+" holds no snapshot of the target's classifier")
	}
}

// sortedKeys is the keys of counts, sorted.
func sortedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// monteCarloObservable is the feature the MonteCarloAnalysis a target classifier inherits
// binds its Mean to; note says why the analysis names none. Both empty without the analysis.
func (m *migration) monteCarloObservable(classifiers []*sysmlv1.Element) (observable *sysmlv1.Element, note string) {
	order := m.classifierOrder(classifiers)
	var analysing *sysmlv1.Element
	for _, c := range order {
		for _, g := range c.Owned("generalization") {
			if isMonteCarloAnalysis(m.model.Ref(g, "general")) {
				analysing = c
				break
			}
		}
		if analysing != nil {
			break
		}
	}
	if analysing == nil {
		return nil, ""
	}
	var bound []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	for _, c := range order {
		for _, conn := range c.Owned("ownedConnector") {
			ends := conn.Owned("end")
			if len(ends) != 2 {
				continue
			}
			for i, end := range ends {
				if monteCarloFeature(m.model.Ref(end, "role")) != monteCarloMean {
					continue
				}
				if f := m.model.Ref(ends[1-i], "role"); f != nil && !f.IsProxy() && f.Type == "Property" && !seen[f] {
					seen[f] = true
					bound = append(bound, f)
				}
			}
		}
	}
	subject := describe(analysing) + " inherits " + monteCarloAnalysisBlock
	switch len(bound) {
	case 0:
		return nil, subject + " but binds its " + monteCarloMean + " to no feature, so its statistics summarise no observable"
	case 1:
		return bound[0], ""
	}
	names := make([]string, len(bound))
	for i, f := range bound {
		names[i] = f.Name
	}
	return nil, subject + " and binds its " + monteCarloMean + " to " + strings.Join(names, ", ") + " alike, so its statistics summarise no one observable"
}

// monteCarloBinding says why a connector with an end on a MonteCarloAnalysis feature
// has no v2 form: it wires the tool's statistic, which the migration results carry;
// "" for a connector on no such feature.
func (m *migration) monteCarloBinding(c *sysmlv1.Element) string {
	ends := c.Owned("end")
	for i, end := range ends {
		stat := monteCarloFeature(m.model.Ref(end, "role"))
		if stat == "" {
			continue
		}
		note := "the connector wires the simulation tool's " + monteCarloAnalysisBlock + "::" + stat + ", a statistic it computes over the runs, which v2 has no analysis pattern for"
		if len(ends) == 2 {
			if f := m.model.Ref(ends[1-i], "role"); f != nil && !f.IsProxy() {
				note = "the connector binds " + f.Name + " to the simulation tool's " + monteCarloAnalysisBlock + "::" + stat + ", the statistic it computes of " + f.Name + " over the runs, which v2 has no analysis pattern for"
			}
		}
		return note + "; the migration results read the statistic from the result snapshots"
	}
	return ""
}

// monteCarloSlot reads a snapshot slot of the analysis's own features as the number it
// holds, a blank numeric literal as zero; stat is "" for a slot of anything else, and
// reason says why the slot holds no number.
func (m *migration) monteCarloSlot(slot *sysmlv1.Element) (stat string, value float64, reason string) {
	f := m.model.Ref(slot, "definingFeature")
	switch stat = monteCarloFeature(f); stat {
	case monteCarloRuns, monteCarloMean, monteCarloDeviation, monteCarloOutOfSpec:
	default:
		return "", 0, ""
	}
	values := slot.Owned("value")
	switch len(values) {
	case 0:
		return stat, 0, "holds no value"
	case 1:
	default:
		return stat, 0, holdsNote + strconv.Itoa(len(values)) + " values, and a statistic is one number"
	}
	scalar, reason := m.literalScalar(values[0])
	switch {
	case reason != "":
		return stat, 0, reason
	case scalar.kind != kindNumber:
		return stat, 0, "holds a " + scalar.spec + ", which is no number"
	}
	return stat, scalar.number, ""
}

// monteCarloStatistics makes the statistics a snapshot's N, Mean, Deviation and OutOfSpec
// record of the analysed observable, which the binding leaves holding the Mean; none when
// the analysis left them blank, or when unread marks one holding no number. note says what
// is amiss; foreign marks a snapshot whose Mean another observable holds instead — of
// an analysis of that one, so of another configuration.
func monteCarloStatistics(observable string, summary map[string]float64, unread bool, values map[string]float64) (stats *simresults.Statistics, note string, foreign bool) {
	switch {
	case unread:
		return nil, "record a " + monteCarloAnalysisBlock + " statistic that is no number, so they hold no statistics", false
	case len(summary) == 0:
		return nil, "", false
	}
	runs, hasRuns := summary[monteCarloRuns]
	mean, hasMean := summary[monteCarloMean]
	switch {
	case !hasRuns || !hasMean:
		return nil, "record no " + monteCarloAnalysisBlock + "::" + monteCarloRuns + " and " + monteCarloMean + " together, so they hold no statistics", false
	case runs == 0 && mean == 0:
		return nil, "", false
	case runs != math.Trunc(runs) || runs < 1:
		return nil, "record a " + monteCarloAnalysisBlock + "::" + monteCarloRuns + " of " + strconv.FormatFloat(runs, 'g', -1, 64) + ", which is no count of runs, so they hold no statistics", false
	}
	deviation, hasDeviation := summary[monteCarloDeviation]
	if hasDeviation && deviation < 0 {
		return nil, "record a " + monteCarloAnalysisBlock + "::" + monteCarloDeviation + " of " + strconv.FormatFloat(deviation, 'g', -1, 64) + ", which is no standard deviation, so they hold no statistics", false
	}
	outOfSpec, hasOutOfSpec := summary[monteCarloOutOfSpec]
	if hasOutOfSpec && (outOfSpec != math.Trunc(outOfSpec) || outOfSpec < 0 || outOfSpec > runs) {
		return nil, "record a " + monteCarloAnalysisBlock + "::" + monteCarloOutOfSpec + " of " + strconv.FormatFloat(outOfSpec, 'g', -1, 64) + " over " + strconv.FormatFloat(runs, 'g', -1, 64) + " runs, which is no count of them, so they hold no statistics", false
	}
	if value, recorded := values[observable]; recorded && value == mean {
		stats := &simresults.Statistics{Observable: observable, Runs: int64(runs), Mean: mean}
		if hasDeviation {
			stats.Deviation = simresults.Real(deviation)
		}
		if hasOutOfSpec {
			stats.OutOfSpec = simresults.Count(int64(outOfSpec))
		}
		return stats, "", false
	}
	var holders []string
	for name, value := range values {
		if value == mean {
			holders = append(holders, name)
		}
	}
	sort.Strings(holders)
	if len(holders) > 0 {
		return nil, "hold the " + monteCarloAnalysisBlock + "::" + monteCarloMean + " as " + strings.Join(holders, ", ") + " and not as " + observable + ", which the analysis binds it to, so they are of an analysis of another configuration and not among the results", true
	}
	return nil, "record " + monteCarloAnalysisBlock + " statistics whose " + monteCarloMean + " no value of " + observable + " holds, though the analysis binds the two, so the statistics are not read", false
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
	where := " under the result location of the run configuration " + describe(cfg)
	if ran := m.ranOnOther(inst, simulationConfig(cfg), typed); ran != nil {
		return snapshotTyping{config: cfg, note: ranOnOtherNote(inst, ran) + where}
	}
	names := make([]string, len(owners))
	for i, o := range owners {
		names[i] = qualifiedName(o)
	}
	special := m.mostSpecial(owners)
	switch {
	case special == nil:
		return snapshotTyping{config: cfg, note: "its slots are of features of " + strings.Join(names, ", ") + ", none a special of all the others, so no one classifier is inferred" + where}
	case !typed[special]:
		return snapshotTyping{config: cfg, note: "its slots are of features of " + strings.Join(names, ", ") + ", neither a classifier of the configuration's target nor a general of one, so it is no snapshot of a run on it" + where}
	}
	return snapshotTyping{classifiers: []*sysmlv1.Element{special}, config: cfg}
}

// ranOnOther is the classifier inst's name says its run was on, when that is neither s's
// target, a general (typed) nor a special of it; nil when the name names no other classifier.
func (m *migration) ranOnOther(inst *sysmlv1.Element, s *sysmlv1.Stereotype, typed map[*sysmlv1.Element]bool) *sysmlv1.Element {
	named := m.namesakes(inst.Name)
	if len(named) == 0 {
		return nil
	}
	targets := s.IDs("executionTarget")
	for _, c := range named {
		if typed[c] || (len(targets) == 1 && c.ID == targets[0]) {
			return nil
		}
		closure := m.classifierClosure([]*sysmlv1.Element{c})
		for t := range typed {
			if closure[t] {
				return nil
			}
		}
	}
	return named[0]
}

// ranOnOtherNote says that inst, by its name, is the result of a run on ran.
func ranOnOtherNote(inst, ran *sysmlv1.Element) string {
	return "it is named " + strconv.Quote(inst.Name) + " after " + qualifiedName(ran) + ", which is neither the configuration's target nor a general or special of it, and the tool names a result after the classifier it ran, so it is a snapshot of a run on that classifier stored"
}

// namesakes lists the classifiers whose default instance name a result snapshot's name is,
// once a number keeping names apart and an " at <timestamp>" suffix are dropped.
func (m *migration) namesakes(name string) []*sysmlv1.Element {
	if m.instanceNames == nil {
		m.instanceNames = map[string][]*sysmlv1.Element{}
		var walk func(e *sysmlv1.Element)
		walk = func(e *sysmlv1.Element) {
			if isClassifierType(e.Type) && e.Name != "" {
				key := defaultInstanceName(e.Name)
				m.instanceNames[key] = append(m.instanceNames[key], e)
			}
			for _, c := range e.Children {
				walk(c)
			}
		}
		for _, r := range m.model.Roots {
			if !r.IsProxy() {
				walk(r)
			}
		}
	}
	name = resultTimestamp.ReplaceAllString(name, "")
	for {
		if found := m.instanceNames[name]; len(found) > 0 {
			return found
		}
		n := len(name)
		if n == 0 || name[n-1] < '0' || name[n-1] > '9' {
			return nil
		}
		name = name[:n-1]
	}
}

// resultTimestamp is the suffix a tool capturing timestamps gives a result snapshot's name.
var resultTimestamp = regexp.MustCompile(`\s+at\s+\d{4}\.\d{2}\.\d{2}\s+\d{2}\.\d{2}(\.\d{2})?$`)

// defaultInstanceName is the classifier's name with its first letter lowered, as
// a tool names an instance the user does not.
func defaultInstanceName(classifier string) string {
	for i, r := range classifier {
		return string(unicode.ToLower(r)) + classifier[i+utf8.RuneLen(r):]
	}
	return classifier
}

// isClassifierType reports whether a UML element type is a classifier a result snapshot can be of.
func isClassifierType(typ string) bool {
	switch typ {
	case "Class", "Actor", "DataType", "PrimitiveType", "Enumeration", "Signal", "Interface", "AssociationClass":
		return true
	}
	return false
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
		if !isClassifierType(f.Parent.Type) {
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
// by a classifier of the target, a general of one (typed) or a special of one; a
// classifier merely sharing a general with the target's is of another kind.
func (m *migration) isSnapshotOf(inst *sysmlv1.Element, targets []*sysmlv1.Element, typed map[*sysmlv1.Element]bool) bool {
	classifiers := m.classifiersOf(inst)
	for _, c := range classifiers {
		if typed[c] {
			return true
		}
	}
	closure := m.classifierClosure(classifiers)
	for _, t := range targets {
		if closure[t] {
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
		return name, scalarValue{}, holdsNote + strconv.Itoa(len(values)) + " values, and a result is one number"
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

// holdsNote prefixes a reason a snapshot slot is no result.
const holdsNote = "holds "

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
			reason = holdsNote + strconv.Quote(text) + ", which is no Boolean"
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
		return nil, 0, holdsNote + strconv.Quote(text) + ", which is no finite number"
	}
	if exact, ok := new(big.Rat).SetString(text); ok {
		return exact, value, ""
	}
	return new(big.Rat).SetFloat64(value), value, ""
}
