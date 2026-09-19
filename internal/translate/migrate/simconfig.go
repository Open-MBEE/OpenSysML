package migrate

import (
	"math"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A simulation tool's run configuration («SimulationConfig» of MagicDraw's
// SimulationProfile) becomes an action def that holds its execution target as
// a part and performs the target's classifier behavior on that part, so a run
// of the def runs the behavior on an object configured as the tool ran it. Its
// settings are recorded by Simulation::Configuration metadata.

// simulationConfig returns e's «SimulationConfig» application, or nil.
func simulationConfig(e *sysmlv1.Element) *sysmlv1.Stereotype {
	for _, s := range e.Stereotypes {
		if isSimulationConfig(s) {
			return s
		}
	}
	return nil
}

// isSimulationConfig recognises a «SimulationConfig» application by the
// simulation profile's provenance, not by its name alone.
func isSimulationConfig(s *sysmlv1.Stereotype) bool {
	return s.Name == "SimulationConfig" && isSimulationProfile(s.Namespace)
}

// isSimulationProfile matches, by host and path, MagicDraw's SimulationProfile
// (…/schemas/SimulationProfile.xmi); nothing else.
func isSimulationProfile(ns string) bool {
	u, err := url.Parse(ns)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host != "magicdraw.com" && host != "nomagic.com" {
		return false
	}
	segs := strings.Split(strings.ToLower(u.Path), "/")
	return strings.TrimSuffix(segs[len(segs)-1], ".xmi") == "simulationprofile"
}

// configurationSetting relates a «SimulationConfig» tag to the attribute of
// Simulation::Configuration recording it, and writes the tag's value as that
// attribute's literal; form reports why a value has no literal.
type configurationSetting struct {
	tag, attribute string
	form           func(value string) (literal, reason string)
}

var configurationSettings = []configurationSetting{
	{"numberOfRuns", "runs", naturalSetting},
	{"durationSimulationMode", "draws", drawPolicySetting},
	{"timeVariableName", "timeVariable", stringSetting},
	{"startTime", "startTime", realSetting},
	{"stepSize", "stepSize", realSetting},
	{"timeUnit", "timeUnit", stringSetting},
	{"runForksInParallel", "parallelForks", booleanSetting},
}

// activeObjectSettings are the tags whose true value states what every v2
// object does anyway: run its classifier behavior from its creation.
var activeObjectSettings = map[string]string{
	"treatAllClassifiersAsActive": "every v2 object runs its classifier behavior",
	"autostartActiveObjects":      "a v2 object starts its classifier behavior when it is created",
}

// naturalSetting writes a count the runtime can hold: a natural number within int64.
func naturalSetting(v string) (string, string) {
	v = strings.TrimSpace(v)
	n, ok := new(big.Int).SetString(v, 10)
	if !ok || n.Sign() < 0 {
		return "", "is not a natural number"
	}
	if !n.IsInt64() {
		return "", "exceeds the runs a Monte Carlo can make"
	}
	return n.String(), ""
}

func realSetting(v string) (string, string) {
	v = strings.TrimSpace(v)
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return "", "is not a finite number"
	}
	if _, ok := new(big.Rat).SetString(v); !ok {
		return "", "is not a finite number"
	}
	if !strings.ContainsAny(v, ".eE") {
		v += ".0"
	}
	return v, ""
}

func booleanSetting(v string) (string, string) {
	switch strings.TrimSpace(v) {
	case "true", "1":
		return "true", ""
	case "false", "0":
		return "false", ""
	}
	return "", "is not a boolean"
}

func stringSetting(v string) (string, string) {
	return source.StringText(v), ""
}

// drawPolicySetting writes a duration simulation mode as the draw policy the
// runtime's -draws option and %draws command take.
func drawPolicySetting(v string) (string, string) {
	mode := strings.ToLower(strings.TrimSpace(v))
	switch mode {
	case "random", "min", "max", "average":
		return "Simulation::DrawPolicy::" + mode, ""
	}
	return "", "is not one of the draw policies random, min, max and average"
}

// simulationConfig writes the run configuration e, whose declaration header is
// written, and its report entry; note carries what the header approximated.
func (m *migration) simulationConfig(e *sysmlv1.Element, header, note string) {
	s := simulationConfig(e)
	settings, unread, notes := m.configurationSettings(s)
	target := m.configurationTarget(s)
	notes = append(notes, target.notes...)
	results := simresults.ConfigurationResults{ID: e.ID, Name: m.v2Name(e), Runs: settings.runs, Draws: settings.draws, Observables: []string{}, Snapshots: []simresults.Snapshot{}}
	notes = append(notes, m.resultSnapshots(&results, s, target)...)
	note = joinNotes(note, strings.Join(notes, "; "))
	m.add(e, verdictFor(note), m.v2Name(e), note)
	m.w.block(header, func() {
		saved := m.scope
		m.scope = e
		m.comments(e)
		m.w.block("@Simulation::Configuration", func() { m.w.lines(settings.lines) })
		if target.element != nil {
			part := m.freshName(e, "target")
			results.Target = part
			m.w.line("part " + writeName(part) + " : " + m.ref(target.element, e) + ";")
			if target.usage != "" {
				run := m.freshName(e, "run")
				results.Behavior = run
				m.w.line("perform action " + writeName(run) + " ::> " + writeName(part) + "." + writeName(target.usage) + ";")
			}
		}
		if results.Location != "" {
			m.w.lines(commentLines(resultsComment(results)))
		}
		if len(unread) > 0 {
			m.w.lines(commentLines("«SimulationConfig» settings of the simulation tool: " + strings.Join(unread, "; ")))
		}
		m.members(e)
		m.scope = saved
		m.classifierBehavior(e)
		m.stereotypeComments(e)
	})
	m.results.Configurations = append(m.results.Configurations, results)
}

// resultsComment says what the tool stored of the configuration's runs and
// where; the snapshots themselves are written as individuals in their package.
func resultsComment(r simresults.ConfigurationResults) string {
	text := "results of the simulation tool: " + strconv.Itoa(len(r.Snapshots)) + " snapshot(s) in " + r.Location
	if len(r.Observables) > 0 {
		text += " holding " + strings.Join(r.Observables, ", ")
	}
	return text
}

// configurationValues are the Simulation::Configuration attributes a
// configuration sets, as written, with the two a harness runs it under.
type configurationValues struct {
	lines []string
	runs  int64
	draws string
}

// configurationSettings writes the Simulation::Configuration attributes a
// «SimulationConfig» application sets, lists the tags it keeps as a comment,
// and notes the tags with no v2 form.
func (m *migration) configurationSettings(s *sysmlv1.Stereotype) (settings configurationValues, unread, notes []string) {
	recorded := map[string]bool{"executionTarget": true, "resultLocation": true}
	for _, c := range configurationSettings {
		recorded[c.tag] = true
		vs := s.Tags[c.tag]
		if len(vs) == 0 {
			continue
		}
		if len(vs) > 1 {
			notes = append(notes, "«SimulationConfig» "+c.tag+" has "+strconv.Itoa(len(vs))+" values; Simulation::Configuration::"+c.attribute+" takes one")
			unread = append(unread, c.tag+" = "+strings.Join(vs, ", "))
			continue
		}
		lit, reason := c.form(vs[0])
		if reason != "" {
			notes = append(notes, "«SimulationConfig» "+c.tag+" = "+strconv.Quote(vs[0])+" "+reason+", so it is not recorded as Simulation::Configuration::"+c.attribute)
			unread = append(unread, c.tag+" = "+vs[0])
			continue
		}
		settings.lines = append(settings.lines, c.attribute+" = "+lit+";")
		switch c.tag {
		case "numberOfRuns":
			settings.runs, _ = strconv.ParseInt(lit, 10, 64)
		case "durationSimulationMode":
			settings.draws = strings.TrimPrefix(lit, "Simulation::DrawPolicy::")
		}
	}
	for tag, means := range activeObjectSettings {
		recorded[tag] = true
		vs := s.Tags[tag]
		if len(vs) == 0 || (len(vs) == 1 && (vs[0] == "true" || vs[0] == "1")) {
			continue
		}
		notes = append(notes, "«SimulationConfig» "+tag+" = "+strings.Join(vs, ", ")+" has no v2 form: "+means)
		unread = append(unread, tag+" = "+strings.Join(vs, ", "))
	}
	for tag, vs := range s.Tags {
		if !recorded[tag] {
			unread = append(unread, tag+" = "+strings.Join(m.tagValues(vs), ", "))
		}
	}
	sort.Strings(unread)
	return settings, unread, notes
}

// executionTarget is a configuration's target as resolved: the element its
// part is typed by, the classifiers of that element, the usage of the part by
// which a run performs their classifier behavior, and what could not be resolved.
type executionTarget struct {
	element     *sysmlv1.Element
	classifiers []*sysmlv1.Element
	usage       string
	notes       []string
}

// targetClassifiers resolves a configuration's execution target and the part
// defs its part is typed by; note says why there are none, t being nil when
// the target itself is unusable.
func (m *migration) targetClassifiers(s *sysmlv1.Stereotype) (t *sysmlv1.Element, classifiers []*sysmlv1.Element, note string) {
	ids := s.IDs("executionTarget")
	switch {
	case len(ids) == 0:
		return nil, nil, "the configuration names no execution target, so it runs no behavior"
	case len(ids) > 1:
		return nil, nil, "the configuration names " + strconv.Itoa(len(ids)) + " execution targets, and a run has one object to run on"
	}
	t = m.model.Lookup(ids[0])
	if t == nil || t.IsProxy() {
		return nil, nil, "the execution target " + strconv.Quote(ids[0]) + " is outside the document, so the configuration runs no behavior"
	}
	if m.isLibrary(t) || !m.written(t) {
		return nil, nil, "the execution target " + describe(t) + " is not migrated, so the configuration runs no behavior"
	}
	cat, why := m.classify(t)
	switch cat {
	case catPartDef:
		classifiers = []*sysmlv1.Element{t}
	case catIndividualDef:
		kind, written, _ := m.individualClassifiers(t)
		if kind != catPartDef {
			return nil, nil, "the execution target " + describe(t) + " is written as an " + individualKeyword(kind) + ", which no part can be typed by, so the configuration runs no behavior"
		}
		classifiers = written
	default:
		return nil, nil, joinNotes("the execution target "+describe(t)+" is written as a "+cat.keyword()+", which no part can be typed by, so the configuration runs no behavior", why)
	}
	if len(classifiers) == 0 {
		return t, nil, "the execution target " + describe(t) + " has no written classifier, so no behavior of it is performed"
	}
	return t, classifiers, ""
}

// configurationTarget resolves the execution target of a configuration; it
// notes whatever it cannot.
func (m *migration) configurationTarget(s *sysmlv1.Stereotype) executionTarget {
	t, classifiers, note := m.targetClassifiers(s)
	if note != "" {
		return executionTarget{element: t, notes: []string{note}}
	}
	target := executionTarget{element: t, classifiers: classifiers}
	behavior := m.inheritedClassifierBehavior(classifiers)
	if behavior == nil {
		target.notes = []string{"neither " + qualifiedName(classifiers[0]) + " nor any general of it has a classifier behavior, so the configuration only holds " + describe(t)}
		return target
	}
	_, usage, bcat := m.classifierBehaviorUsage(behavior)
	switch bcat {
	case catActionDef:
		target.usage = usage
	case catStateDef:
		target.notes = []string{"the classifier behavior of " + qualifiedName(behavior) + " is a state machine, which a run performs as no action; the configuration only holds " + describe(t)}
	default:
		target.notes = []string{"the classifier behavior of " + qualifiedName(behavior) + " is written as a " + bcat.keyword() + ", not an action def, so no action is performed"}
	}
	return target
}

// inheritedClassifierBehavior finds, breadth first through generalizations,
// the nearest of the classifiers or their generals with a written classifier behavior.
func (m *migration) inheritedClassifierBehavior(classifiers []*sysmlv1.Element) *sysmlv1.Element {
	seen := map[*sysmlv1.Element]bool{}
	queue := append([]*sysmlv1.Element(nil), classifiers...)
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if seen[c] {
			continue
		}
		seen[c] = true
		if b, _, _ := m.classifierBehaviorUsage(c); b != nil {
			return c
		}
		for _, g := range c.Owned("generalization") {
			if general := m.model.Ref(g, "general"); general != nil && !general.IsProxy() {
				queue = append(queue, general)
			}
		}
	}
	return nil
}
