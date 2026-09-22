package migrate

import (
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A simulation tool's run configuration («SimulationConfig» of MagicDraw's
// SimulationProfile) becomes an action def that holds its execution target as
// a part and performs the target's classifier behavior on that part, so a run
// of the def runs the behavior on an object configured as the tool ran it. Its
// settings are recorded by Simulation::Configuration metadata.

// The note prefixes the configuration findings repeat.
const (
	simConfig          = "«SimulationConfig» "
	targetNote         = "the execution target "
	classifierBehavior = "the classifier behavior of "
)

// simulationConfig returns e's «SimulationConfig» application, or nil.
func simulationConfig(e *sysmlv1.Element) *sysmlv1.Stereotype {
	for _, s := range e.Stereotypes {
		if isSimulationConfig(s) {
			return s
		}
	}
	return nil
}

var simulationProvenance = provenance{isSimulationProfile, simulationProfileDefinition}

// isSimulationConfig recognises a «SimulationConfig» application, or one of a
// stereotype specializing it, by the simulation profile's provenance, not by name alone.
func isSimulationConfig(s *sysmlv1.Stereotype) bool {
	return simulationProvenance.applies(s, "SimulationConfig")
}

// simulationProfileDefinition reports whether stereotype definition d belongs
// to the simulation tool's profile, bundled in the document or referred to.
func simulationProfileDefinition(d *sysmlv1.Element) bool {
	if d.IsProxy() {
		return fold(hrefDocument(d.Href)) == fold("SimulationProfile")
	}
	p := enclosingProfile(d)
	return p != nil && isSimulationProfile(p.Attrs["URI"])
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
var activeObjectSettings = []struct{ tag, means string }{
	{"treatAllClassifiersAsActive", "every v2 object runs its classifier behavior"},
	{"autostartActiveObjects", "a v2 object starts its classifier behavior when it is created"},
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
	results := simresults.ConfigurationResults{ID: e.ID, Name: m.v2Name(e), Runs: settings.runs, Draws: settings.draws, ClockStep: settings.clockStep, Observables: []string{}, Snapshots: []simresults.Snapshot{}, Notes: append([]string(nil), notes...)}
	lost := m.resultSnapshots(&results, s, target)
	results.Notes = append(results.Notes, lost...)
	notes = append(notes, lost...)
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
			switch {
			case target.usage != "":
				run := m.freshName(e, "run")
				results.Behavior = run
				m.w.line("perform action " + writeName(run) + " ::> " + writeName(part) + "." + writeName(target.usage) + ";")
			case target.testCase != nil:
				run := m.freshName(e, "run")
				results.Behavior = run
				m.w.line("verification " + writeName(run) + " : " + m.ref(target.testCase, e) + " { subject " + writeName(target.subject) + " = " + writeName(part) + "; }")
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
		m.stereotypeAnnotations(e)
	})
	m.results.Configurations = append(m.results.Configurations, results)
}

// resultsComment says what the tool stored of the configuration's runs and
// where — how many runs, when a snapshot summarises several; the snapshots
// themselves are written as individuals in their package.
func resultsComment(r simresults.ConfigurationResults) string {
	text := "results of the simulation tool: " + strconv.Itoa(len(r.Snapshots)) + " snapshot(s) in " + r.Location
	if runs := r.StoredRuns(); runs != int64(len(r.Snapshots)) {
		text += " standing for " + strconv.FormatInt(runs, 10) + " run(s)"
	}
	if r.Analysis != "" {
		text += " analysing " + r.Analysis
	}
	if len(r.Observables) > 0 {
		text += " holding " + strings.Join(r.Observables, ", ")
	}
	return text
}

// configurationValues are the Simulation::Configuration attributes a
// configuration sets, as written, with the three a harness runs it under.
type configurationValues struct {
	lines     []string
	runs      int64
	draws     string
	clockStep float64
}

// toolClock is the step, in seconds, of the tool's internal clock that startTime sets going — 0 with a
// note when none is derived — and a note on a startTime other than 0; tags are the configuration's as written.
func toolClock(read map[string]string, tags map[string][]string) (step float64, notes []string) {
	if _, enabled := read["startTime"]; !enabled {
		return 0, nil
	}
	start, _ := strconv.ParseFloat(read["startTime"], 64) // finite: realSetting read it
	step = 1
	stepOf := "stepSize"
	if v, ok := read["stepSize"]; ok {
		step, _ = strconv.ParseFloat(v, 64)
	} else if derived, note := stepOfSteps(start, tags); note != "" {
		notes = append(notes, note)
		if derived > 0 {
			step, stepOf = derived, "the stepSize derived as (endTime - startTime) / numberOfSteps"
		}
	}
	unit, stated := read["timeUnit"]
	scale, ok := durationUnits[strings.ToLower(unit)]
	unitReadable := !stated || (ok && unit != "")
	in := " in timeUnit = " + strconv.Quote(unit)
	if !stated {
		in = " in milliseconds, the tool's default for an unstated timeUnit,"
	}
	if start != 0 {
		notes = append(notes, clockStartNote(start, start*scale, in, unitReadable))
	}
	if step <= 0 {
		return 0, append(notes, simConfig+stepOf+" = "+semantics.FormatReal(step)+" is no step the clock can tick by, so the runs' clock is continuous")
	}
	if !unitReadable {
		return 0, append(notes, simConfig+"timeUnit = "+strconv.Quote(unit)+" is no fixed number of seconds, so the clock's step is not derived and the runs' clock is continuous")
	}
	switch seconds := step * scale; {
	case math.IsInf(seconds, 0):
		return 0, append(notes, simConfig+stepOf+" = "+semantics.FormatReal(step)+in+" is more seconds than a number holds, so the clock's step is not derived and the runs' clock is continuous")
	case seconds == 0:
		return 0, append(notes, simConfig+stepOf+" = "+semantics.FormatReal(step)+in+" is fewer seconds than a number tells from none, so the clock's step is not derived and the runs' clock is continuous")
	case !stated:
		return seconds, append(notes, simConfig+"timeUnit is unstated, so "+stepOf+" = "+semantics.FormatReal(step)+" is read in milliseconds, the tool's default")
	default:
		return seconds, notes
	}
}

// stepOfSteps is (endTime − start) / numberOfSteps, the step the tool derives for an unstated stepSize,
// with the note reading it; 0 with the note when the two tell no step; neither when one is unstated.
func stepOfSteps(start float64, tags map[string][]string) (step float64, note string) {
	ends, counts := tags["endTime"], tags["numberOfSteps"]
	if len(ends) != 1 || len(counts) != 1 {
		return 0, ""
	}
	end, count := strings.TrimSpace(ends[0]), strings.TrimSpace(counts[0])
	stated := simConfig + "stepSize is unstated, and endTime = " + end + " with numberOfSteps = " + count
	e, err := strconv.ParseFloat(end, 64)
	if err != nil || math.IsNaN(e) || math.IsInf(e, 0) || e <= start {
		return 0, stated + " spans no time from startTime = " + semantics.FormatReal(start) + ", so the tool's default step, 1.0, stands"
	}
	n, err := strconv.ParseFloat(count, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n != math.Trunc(n) || n <= 0 {
		return 0, stated + " counts no steps, so the tool's default step, 1.0, stands"
	}
	step = (e - start) / n
	return step, stated + " derive it as (endTime - startTime) / numberOfSteps = " + semantics.FormatReal(step)
}

// clockStartNote says the tool's clock started at start, not at 0 as a run's does, so an instant
// read on the clock — an `at` trigger's, the clock variable's — is offset by it in a run.
func clockStartNote(start, seconds float64, in string, unitReadable bool) string {
	note := simConfig + "startTime = " + semantics.FormatReal(start) + in + " started the tool's clock "
	if !unitReadable || math.IsInf(seconds, 0) {
		note += "at an instant no number of seconds tells"
	} else {
		note += "at " + semantics.FormatReal(seconds) + " s"
	}
	return note + ", and a run's clock starts at 0, so an instant read on the clock — by an `at` trigger or the clock variable — is offset by that start here"
}

// configurationSettings writes the Simulation::Configuration attributes a
// «SimulationConfig» application sets, lists the tags it keeps as a comment,
// and notes the tags with no v2 form.
func (m *migration) configurationSettings(s *sysmlv1.Stereotype) (settings configurationValues, unread, notes []string) {
	recorded := map[string]bool{"executionTarget": true, "resultLocation": true}
	read := map[string]string{}
	for _, c := range configurationSettings {
		recorded[c.tag] = true
		vs := s.Tags[c.tag]
		if len(vs) == 0 {
			continue
		}
		if len(vs) > 1 {
			notes = append(notes, simConfig+c.tag+" has "+strconv.Itoa(len(vs))+" values; Simulation::Configuration::"+c.attribute+" takes one")
			unread = append(unread, c.tag+" = "+strings.Join(vs, ", "))
			continue
		}
		lit, reason := c.form(vs[0])
		if reason != "" {
			notes = append(notes, simConfig+c.tag+" = "+strconv.Quote(vs[0])+" "+reason+", so it is not recorded as Simulation::Configuration::"+c.attribute)
			unread = append(unread, c.tag+" = "+vs[0])
			continue
		}
		settings.lines = append(settings.lines, c.attribute+" = "+lit+";")
		read[c.tag] = strings.TrimSpace(vs[0])
		switch c.tag {
		case "numberOfRuns":
			settings.runs, _ = strconv.ParseInt(lit, 10, 64)
		case "durationSimulationMode":
			settings.draws = strings.TrimPrefix(lit, "Simulation::DrawPolicy::")
		}
	}
	var clockNotes []string
	settings.clockStep, clockNotes = toolClock(read, s.Tags)
	notes = append(notes, clockNotes...)
	for _, a := range activeObjectSettings {
		recorded[a.tag] = true
		vs := s.Tags[a.tag]
		if len(vs) == 0 || (len(vs) == 1 && (vs[0] == "true" || vs[0] == "1")) {
			continue
		}
		notes = append(notes, simConfig+a.tag+" = "+strings.Join(vs, ", ")+" has no v2 form: "+a.means)
		unread = append(unread, a.tag+" = "+strings.Join(vs, ", "))
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
// which a run performs their classifier behavior — or the test case a run
// performs on the part as its subject — and what could not be resolved.
type executionTarget struct {
	element     *sysmlv1.Element
	classifiers []*sysmlv1.Element
	usage       string
	testCase    *sysmlv1.Element
	subject     string
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
		return nil, nil, targetNote + strconv.Quote(ids[0]) + " is outside the document, so the configuration runs no behavior"
	}
	if m.isLibrary(t) || !m.written(t) {
		return nil, nil, targetNote + describe(t) + " is not migrated, so the configuration runs no behavior"
	}
	cat, why := m.classify(t)
	switch cat {
	case catPartDef:
		classifiers = []*sysmlv1.Element{t}
	case catIndividualDef:
		kind, written, _ := m.individualClassifiers(t)
		if kind != catPartDef {
			return nil, nil, targetNote + describe(t) + " is written as an " + individualKeyword(kind) + ", which no part can be typed by, so the configuration runs no behavior"
		}
		classifiers = written
	default:
		return nil, nil, joinNotes(targetNote+describe(t)+" is written as a "+cat.keyword()+", which no part can be typed by, so the configuration runs no behavior", why)
	}
	if len(classifiers) == 0 {
		return t, nil, targetNote + describe(t) + " has no written classifier, so no behavior of it is performed"
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
		target.notes = []string{"neither " + qualifiedName(classifiers[0]) + " nor any general of it has a classifier behavior, so the configuration only holds " + describe(t) + m.constraintNetwork(classifiers)}
		return target
	}
	b, usage, bcat := m.classifierBehaviorUsage(behavior)
	if b == nil {
		b = m.model.Ref(behavior, "classifierBehavior")
		_, why := m.classify(b)
		target.notes = []string{joinNotes(classifierBehavior+qualifiedName(behavior)+", "+describe(b)+", is not migrated, so no action is performed; the configuration only holds "+describe(t), why)}
		return target
	}
	switch bcat {
	case catActionDef:
		target.usage = usage
	case catStateDef:
		target.notes = []string{classifierBehavior + qualifiedName(behavior) + " is a state machine, which a run performs as no action; the configuration only holds " + describe(t)}
	case catVerificationDef:
		target.testCase, target.subject, target.notes = m.targetTestCase(t, classifiers, b)
	default:
		target.notes = []string{classifierBehavior + qualifiedName(behavior) + " is written as a " + bcat.keyword() + ", not an action def, so no action is performed"}
	}
	return target
}

// targetTestCase resolves a test case a target's class runs as its classifier
// behavior: the tool runs its scenario on the object, so the configuration
// performs the verification with the target as its subject — when the scenario
// is written and the subject's block is one the target is typed by.
func (m *migration) targetTestCase(t *sysmlv1.Element, classifiers []*sysmlv1.Element, b *sysmlv1.Element) (testCase *sysmlv1.Element, subject string, notes []string) {
	if b.Type != "Interaction" {
		return nil, "", []string{classifierBehavior + qualifiedName(b.Parent) + " is a test case with no scenario to perform, so no action is performed; the configuration only holds " + describe(t)}
	}
	subject = m.subjectName(b)
	s, note := m.scenario(b, subject)
	if note != "" {
		return nil, "", []string{classifierBehavior + qualifiedName(b.Parent) + " is a test case whose scenario is not migrated, so no action is performed; the configuration only holds " + describe(t) + ": " + note}
	}
	for _, c := range classifiers {
		if c == s.context || m.inherits(c, s.context) {
			return b, subject, nil
		}
	}
	return nil, "", []string{classifierBehavior + qualifiedName(b.Parent) + " is a test case whose subject is " + describe(s.context) + ", which " + describe(t) + " is not typed by, so no action is performed; the configuration only holds " + describe(t)}
}

// constraintNetwork lists the constraint properties a target holds, through its
// generals and composite parts, saying per block whether its rule is migrated.
func (m *migration) constraintNetwork(classifiers []*sysmlv1.Element) string {
	var usages []string
	blocks := map[*sysmlv1.Element]bool{}
	seen := map[*sysmlv1.Element]bool{}
	queue := append([]*sysmlv1.Element(nil), classifiers...)
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if seen[c] {
			continue
		}
		seen[c] = true
		for _, g := range c.Owned("generalization") {
			if general := m.model.Ref(g, "general"); general != nil && !general.IsProxy() {
				queue = append(queue, general)
			}
		}
		for _, p := range c.Owned("ownedAttribute") {
			t := m.model.Ref(p, "type")
			if t == nil || t.IsProxy() {
				continue
			}
			switch cat, _ := m.classify(t); cat {
			case catConstraintDef:
				usage := m.v2Name(c) + "::" + writeName(m.nameFor(p)) + " : " + describe(t)
				switch f := m.constraintRule(t); {
				case f.rule == nil:
					usage += ", whose block states no rule"
				case blocks[t]:
					usage += ", as above"
				case f.spec == nil:
					usage += ", whose rule is not migrated: " + f.note
				case !f.ok:
					usage += ", whose rule " + describeValue(f.spec) + " is not migrated: " + f.note
				default:
					usage += ", whose rule is migrated as a constraint"
				}
				blocks[t] = true
				usages = append(usages, usage)
			case catPartDef:
				if p.Attrs["aggregation"] == "composite" {
					queue = append(queue, t)
				}
			}
		}
	}
	if len(usages) == 0 {
		return ""
	}
	return "; the tool solves the constraints it holds for values, which a v2 run checks and does not solve: " + strings.Join(usages, "; ")
}

// inheritedClassifierBehavior finds, breadth first through generalizations,
// the nearest of the classifiers or their generals with a classifier behavior
// of its own, written or not: an object runs the nearest one.
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
		if b := m.model.Ref(c, "classifierBehavior"); b != nil && b.Parent == c {
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
