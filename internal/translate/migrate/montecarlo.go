package migrate

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// The library analysis the pattern is written against, and the names its members
// and the written analysis take.
const (
	monteCarloLibraryCase = "Simulation::MonteCarlo"
	monteCarloCaseSuffix  = " Monte Carlo"
	monteCarloSubject     = "analysed"
	monteCarloRun         = "run"
	monteCarloObserved    = "observed"
	monteCarloRecorded    = "Monte Carlo"
)

// monteCarloStatistic maps a MonteCarloAnalysis statistic to the Simulation::MonteCarlo
// member holding it and its scalar; optional when a sample may leave it unbound.
type monteCarloStatistic struct {
	member   string
	scalar   string
	optional bool
}

// monteCarloMembers maps the tool's statistics to the library analysis's members.
var monteCarloMembers = map[string]monteCarloStatistic{
	monteCarloRuns:      {"runs", "Natural", false},
	monteCarloMean:      {"mean", "Real", false},
	monteCarloDeviation: {"deviation", "Real", true},
	monteCarloOutOfSpec: {"outOfSpec", "Natural", false},
}

// numberScalars are the ScalarValues types a feature a statistic is bound to may be
// of: those a count is a value of, and a Real statistic of a Real-valued feature.
var numberScalars = map[string]bool{"Natural": true, "Integer": true, "Rational": true, "Real": true, "Number": true}

// monteCarloCase is the analysis def written beside a block inheriting the tool's
// MonteCarloAnalysis: subject the part def, one run its behavior, returns the bound statistics.
type monteCarloCase struct {
	block    *sysmlv1.Element
	name     string
	segments []string
	// observed is the feature bound to Mean; note says why there is none.
	observed *sysmlv1.Element
	note     string
	// returns are the statistics the block's own connectors bind, in connector order.
	returns []monteCarloReturn
	// bindings settle each of the block's connectors on a MonteCarloAnalysis feature.
	bindings map[*sysmlv1.Element]monteCarloBinding
}

// monteCarloReturn is one statistic the analysis returns, typed by the ScalarValues
// type of the bound feature; note says when that is not the feature's own type.
type monteCarloReturn struct {
	stat    string
	feature *sysmlv1.Element
	typed   string
	note    string
}

// monteCarloBinding is how a connector on a MonteCarloAnalysis feature is written:
// as a member of the analysis, or not at all with the note saying why.
type monteCarloBinding struct {
	stat   string
	member string
	note   string
}

// monteCarloEnd is the statistic a connector's end names on the tool's
// MonteCarloAnalysis, with the feature the other end names; "" for another connector.
func (m *migration) monteCarloEnd(c *sysmlv1.Element) (stat string, other *sysmlv1.Element, otherEnd *sysmlv1.Element) {
	ends := c.Owned("end")
	for i, end := range ends {
		stat = monteCarloFeature(m.model.Ref(end, "role"))
		if stat == "" {
			continue
		}
		if len(ends) == 2 {
			otherEnd = ends[1-i]
			other = m.model.Ref(otherEnd, "role")
		}
		return stat, other, otherEnd
	}
	return "", nil, nil
}

// inheritsMonteCarlo reports whether a classifier or a general of it generalizes
// the tool's MonteCarloAnalysis.
func (m *migration) inheritsMonteCarlo(c *sysmlv1.Element) bool {
	for _, k := range m.classifierOrder([]*sysmlv1.Element{c}) {
		if m.generalizesMonteCarlo(k) {
			return true
		}
	}
	return false
}

// generalizesMonteCarlo reports whether c itself generalizes the tool's MonteCarloAnalysis.
func (m *migration) generalizesMonteCarlo(c *sysmlv1.Element) bool {
	for _, g := range c.Owned("generalization") {
		if isMonteCarloAnalysis(m.model.Ref(g, "general")) {
			return true
		}
	}
	return false
}

// monteCarloCaseOf is the analysis def written beside a block: one that generalizes
// the tool's MonteCarloAnalysis, or binds a statistic of one it inherits; nil for others.
func (m *migration) monteCarloCaseOf(block *sysmlv1.Element) *monteCarloCase {
	if block == nil || block.IsProxy() {
		return nil
	}
	if cs, ok := m.monteCarlo[block]; ok {
		return cs
	}
	var cs *monteCarloCase
	if cat, _ := m.classify(block); cat == catPartDef && m.inheritsMonteCarlo(block) {
		binds := m.generalizesMonteCarlo(block)
		for _, c := range block.Owned("ownedConnector") {
			if stat, _, _ := m.monteCarloEnd(c); stat != "" {
				binds = true
			}
		}
		if binds {
			cs = m.newMonteCarloCase(block)
		}
	}
	m.monteCarlo[block] = cs
	return cs
}

// newMonteCarloCase names the analysis of a block and settles its connectors.
func (m *migration) newMonteCarloCase(block *sysmlv1.Element) *monteCarloCase {
	segs := m.segments(block)
	name := m.freshName(block.Parent, segs[len(segs)-1]+monteCarloCaseSuffix)
	segs[len(segs)-1] = name
	cs := &monteCarloCase{block: block, name: name, segments: segs, bindings: map[*sysmlv1.Element]monteCarloBinding{}}
	cs.observed, cs.note = m.monteCarloObservable([]*sysmlv1.Element{block})
	bound := map[string]*sysmlv1.Element{}
	if cs.observed != nil {
		r, ok := m.monteCarloReturn(monteCarloMean, cs.observed)
		if ok {
			cs.returns = append(cs.returns, r)
		} else {
			cs.observed, cs.note = nil, describe(block)+" binds "+monteCarloMean+" to "+r.note
		}
	}
	for _, c := range block.Owned("ownedConnector") {
		stat, f, end := m.monteCarloEnd(c)
		if stat == "" {
			continue
		}
		b := monteCarloBinding{stat: stat}
		subject := "the connector binds the simulation tool's " + monteCarloAnalysisBlock + "::" + stat
		member, known := monteCarloMembers[stat]
		switch {
		case len(c.Owned("end")) != 2:
			b.note = "a connector with " + strconv.Itoa(len(c.Owned("end"))) + " ends is not migrated"
		case !known:
			b.note = subject + ", a statistic " + monteCarloLibraryCase + " has no counterpart for"
		case f == nil:
			b.note = subject + " to nothing in the document"
		case f.IsProxy():
			b.note = subject + " to " + qualifiedName(f) + ", which lives outside the document"
		case !m.valueProperty(f):
			b.note = subject + " to " + f.Name + ", which is no value property, and the statistic is of a number"
		case stereo(end, "NestedConnectorEnd") != nil:
			b.note = subject + " to " + f.Name + " through a nested path, and the statistic is of a value of the block itself"
		case !m.hasFeature(block, f):
			b.note = subject + " to " + qualifiedName(f) + ", which is no feature of " + qualifiedName(block)
		case stat == monteCarloMean && cs.observed == nil:
			b.note = cs.note + ", so the analysis reads no " + monteCarloObserved + " and returns no " + stat
		case stat == monteCarloMean && f != cs.observed:
			b.note = subject + " to " + f.Name + ", but the analysis reads " + cs.observed.Name + ", which a general of the block binds to it"
		case cs.observed == nil:
			b.note = subject + " to " + f.Name + ", but " + cs.note + ", so the statistic is of nothing and is not returned"
		case bound[stat] != nil:
			b.note = subject + " to " + f.Name + ", which another connector of the block already binds it to"
		case stat == monteCarloMean:
			b.member = monteCarloObserved
			bound[stat] = f
		default:
			r, ok := m.monteCarloReturn(stat, f)
			if !ok {
				b.note = subject + " to " + r.note
				break
			}
			b.member = member.member
			bound[stat] = f
			cs.returns = append(cs.returns, r)
		}
		cs.bindings[c] = b
	}
	return cs
}

// monteCarloReturn types the return of stat bound to f by the ScalarValues type f's
// values are, a Real for a Real statistic; not ok for a feature of no number.
func (m *migration) monteCarloReturn(stat string, f *sysmlv1.Element) (monteCarloReturn, bool) {
	r := monteCarloReturn{stat: stat, feature: f}
	t := m.model.Ref(f, "type")
	if t == nil {
		return r, true
	}
	base := m.scalarBase(t)
	if !numberScalars[base] {
		r.note = f.Name + ", which is of " + t.Name + ", and the statistic is of a number"
		return r, false
	}
	scalar := base
	if monteCarloMembers[stat].scalar == "Real" {
		scalar = "Real"
	}
	r.typed = scalarValuesPrefix + scalar
	if m.scalarValue(t) != scalar {
		r.note = stat + " is returned as a " + r.typed + ": the " + stat + " of " + t.Name + " values, which are " + base + "s"
	}
	return r, true
}

// valueProperty reports whether f is written as an attribute, whose value a statistic can be of.
func (m *migration) valueProperty(f *sysmlv1.Element) bool {
	if f.Type != "Property" {
		return false
	}
	kw, _, _ := m.featureKeyword(f, m.classifyParent(f))
	return kw == "attribute"
}

// qualifiedName is the v2 qualified name of the analysis def.
func (cs *monteCarloCase) qualifiedName(m *migration) string {
	return m.qualified(cs.segments)
}

// returnNote is the note on how the return of stat is typed; "" when as its feature.
func (cs *monteCarloCase) returnNote(stat string) string {
	for _, r := range cs.returns {
		if r.stat == stat {
			return r.note
		}
	}
	return ""
}

// statistics names the statistics the analysis returns, in the order written; nil for none.
func (cs *monteCarloCase) statistics() []string {
	var names []string
	for _, r := range cs.returns {
		names = append(names, r.stat)
	}
	return names
}

// monteCarloGeneralization notes how a block's generalization of the tool's
// MonteCarloAnalysis is written: as the sibling analysis def.
func (m *migration) monteCarloGeneralization(block *sysmlv1.Element) string {
	cs := m.monteCarloCaseOf(block)
	if cs == nil {
		return "generalization of the simulation tool's " + monteCarloAnalysisBlock + " is not written: the classifier is written as no part def for an analysis to take as its subject"
	}
	return "generalization of the simulation tool's " + monteCarloAnalysisBlock + " is written as the analysis def " + writeName(cs.name) + " :> " + monteCarloLibraryCase + " beside the part def, which is its subject"
}

// monteCarloAnalysis writes the analysis def of a block beside the block: its subject,
// the run of the block's classifier behavior, the observed feature and the returns.
func (m *migration) monteCarloAnalysis(block *sysmlv1.Element) {
	cs := m.monteCarloCaseOf(block)
	if cs == nil {
		return
	}
	scope := m.scope
	m.w.block("analysis def "+writeName(cs.name)+" :> "+monteCarloLibraryCase, func() {
		m.w.line("subject " + monteCarloSubject + " : " + m.ref(block, scope) + ";")
		if usage, note := m.monteCarloRun(block); usage != "" {
			m.w.line("perform action " + monteCarloRun + " ::> " + monteCarloSubject + "." + writeName(usage) + ";")
		} else {
			m.w.lines(commentLines(note))
		}
		if cs.observed == nil {
			m.w.lines(commentLines(cs.note + ", so " + monteCarloObserved + " is left unbound"))
		} else {
			typed, _ := m.typeRef(m.model.Ref(cs.observed, "type"), scope)
			m.w.line("attribute :>> " + monteCarloObserved + typing(typed) + " = " + monteCarloSubject + "." + writeName(m.nameFor(cs.observed)) + ";")
		}
		for i, r := range cs.returns {
			kw := "out "
			if i == 0 {
				kw = "return "
			}
			member := monteCarloMembers[r.stat]
			multiplicity := ""
			if member.optional {
				multiplicity = "[0..1]"
			}
			m.w.line(kw + writeName(r.stat) + typing(r.typed) + multiplicity + " = " + member.member + ";")
		}
	})
}

// typing writes the typing of a feature, none for an unwritten type.
func typing(typed string) string {
	if typed == "" {
		return ""
	}
	return " : " + typed
}

// monteCarloRun names the usage by which one run of the analysis performs the block's
// classifier behavior; the note says why none does.
func (m *migration) monteCarloRun(block *sysmlv1.Element) (usage, note string) {
	behavior := m.inheritedClassifierBehavior([]*sysmlv1.Element{block})
	if behavior == nil {
		return "", "one run performs no behavior: neither " + qualifiedName(block) + " nor any general of it has a classifier behavior"
	}
	b, usage, cat := m.classifierBehaviorUsage(behavior)
	switch {
	case b == nil:
		return "", "one run performs no behavior: the classifier behavior of " + qualifiedName(behavior) + " is not migrated"
	case cat != catActionDef:
		return "", "one run performs no behavior: the classifier behavior of " + qualifiedName(behavior) + " is written as a " + cat.keyword() + ", not an action def"
	}
	return usage, ""
}

// monteCarloConnector writes a connector with an end on the tool's MonteCarloAnalysis
// as the member of the block's analysis it binds; false for any other connector.
func (m *migration) monteCarloConnector(c *sysmlv1.Element) bool {
	stat, f, _ := m.monteCarloEnd(c)
	if stat == "" {
		return false
	}
	cs := m.monteCarloCaseOf(c.Parent)
	if cs == nil {
		note := "the connector binds the simulation tool's " + monteCarloAnalysisBlock + "::" + stat + ", a statistic of an analysis its owner does not inherit"
		if f != nil && !f.IsProxy() {
			note = "the connector binds " + f.Name + " to the simulation tool's " + monteCarloAnalysisBlock + "::" + stat + ", a statistic of an analysis its owner does not inherit"
		}
		m.unmappedConnector(c, note)
		return true
	}
	b := cs.bindings[c]
	if b.member == "" {
		m.unmappedConnector(c, b.note)
		return true
	}
	target := cs.qualifiedName(m) + "::"
	note := "the binding to the simulation tool's " + monteCarloAnalysisBlock + "::" + stat + " is written in the analysis def " + writeName(cs.name) + " as "
	if stat == monteCarloMean {
		target += monteCarloObserved
		note += "the " + monteCarloObserved + " value, of which " + monteCarloMean + " is returned"
	} else {
		target += writeName(stat)
		note += "the returned " + stat + ", bound to " + b.member
	}
	m.add(c, Approximated, target, joinNotes(note, cs.returnNote(stat)))
	m.stereotypeComments(c)
	for _, fl := range m.flows[c] {
		m.flowDone(fl, nil, []string{"realizing connector " + describe(c) + " is written as no connection: " + note})
	}
	return true
}

// monteCarloSlots reports an individual's statistic slots and returns the other slots
// with the writer of the recorded analysis, to follow the individual's values.
func (m *migration) monteCarloSlots(e *sysmlv1.Element, slots []*sysmlv1.Element) (others []*sysmlv1.Element, recorded func()) {
	type held struct {
		slot *sysmlv1.Element
		stat string
	}
	var stats []held
	count := map[string]int{}
	for _, slot := range slots {
		stat := monteCarloFeature(m.model.Ref(slot, "definingFeature"))
		if stat == "" {
			others = append(others, slot)
			continue
		}
		stats = append(stats, held{slot, stat})
		count[stat]++
	}
	if len(stats) == 0 {
		return others, func() {}
	}
	cs := m.recordedCase(e)
	var lines []string
	for _, h := range stats {
		subject := "the slot holds the simulation tool's " + monteCarloAnalysisBlock + "::" + h.stat
		member, known := monteCarloMembers[h.stat]
		switch {
		case cs == nil:
			m.unmapped(h.slot, subject+", a statistic of an analysis no classifier of the instance inherits")
			continue
		case !known:
			m.unmapped(h.slot, subject+", a statistic "+monteCarloLibraryCase+" has no counterpart for")
			continue
		case cs.observed == nil && h.stat != monteCarloRuns:
			m.unmapped(h.slot, subject+", but "+cs.note+", so the statistic is of nothing")
			continue
		case count[h.stat] > 1:
			m.unmapped(h.slot, subject+", which "+strconv.Itoa(count[h.stat])+" slots of the instance hold, and a statistic is one number")
			continue
		}
		line, note, ok := m.monteCarloSlotValue(e, h.slot, member)
		if !ok {
			m.unmapped(h.slot, subject+": "+note)
			continue
		}
		if line != "" {
			lines = append(lines, line)
		}
		m.add(h.slot, verdictFor(note), m.v2Name(e)+"::"+writeName(monteCarloRecorded)+"::"+member.member, note)
	}
	if cs == nil {
		return others, func() {}
	}
	return others, func() {
		m.w.block("analysis "+writeName(monteCarloRecorded)+" : "+m.refMember(cs.block.Parent, cs.name, cs.segments, e), func() {
			m.w.line("subject :>> " + monteCarloSubject + " : " + m.ref(e, e) + ";")
			m.w.lines(lines)
		})
	}
}

// monteCarloSlotValue writes the value a statistic slot holds as the binding of the
// analysis's member; a slot the tool left blank binds nothing, under a note.
func (m *migration) monteCarloSlotValue(e, slot *sysmlv1.Element, member monteCarloStatistic) (line, note string, ok bool) {
	values := slot.Owned("value")
	switch len(values) {
	case 0:
		return "", "the tool recorded no value of " + member.member + ", so none is written", true
	case 1:
	default:
		return "", holdsNote + strconv.Itoa(len(values)) + " values, and a statistic is one number", false
	}
	if blankLiteral(values[0]) {
		return "", "the tool left " + member.member + " blank, so no value is written", true
	}
	expr, ok, note := m.valueExprAs(values[0], e, oneOf(member.scalar, "the statistic holds"))
	if !ok {
		return "", note, false
	}
	return "out :>> " + member.member + " = " + expr + ";", note, true
}

// recordedCase is the analysis an individual's statistics are recorded of: that of the
// nearest classifier the individual is written to specialize that has one.
func (m *migration) recordedCase(e *sysmlv1.Element) *monteCarloCase {
	_, written, _ := m.individualClassifiers(e)
	for _, c := range m.classifierOrder(written) {
		if cs := m.monteCarloCaseOf(c); cs != nil {
			return cs
		}
	}
	return nil
}

// recordAnalysis records on r the analysis def whose statistics the target's
// snapshots hold, and the statistics it returns.
func (m *migration) recordAnalysis(r *simresults.ConfigurationResults, classifiers []*sysmlv1.Element) {
	for _, c := range m.classifierOrder(classifiers) {
		if cs := m.monteCarloCaseOf(c); cs != nil {
			r.AnalysisCase, r.Statistics = cs.qualifiedName(m), cs.statistics()
			return
		}
	}
}
