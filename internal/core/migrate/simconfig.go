package migrate

import (
	"math"
	"math/big"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// A simulation tool's run configuration («SimulationConfig» of MagicDraw's
// SimulationProfile) becomes an action def that holds its execution target as
// a part and performs the target's classifier behavior on that part, so a run
// of the def runs the behavior on an object configured as the tool ran it. Its
// settings are recorded by Simulation::Configuration metadata.

// simulationConfig returns e's «SimulationConfig» application, or nil.
func simulationConfig(e *xmi.Element) *xmi.Stereotype {
	for _, s := range e.Stereotypes {
		if isSimulationConfig(s) {
			return s
		}
	}
	return nil
}

// isSimulationConfig recognises a «SimulationConfig» application by the
// simulation profile's provenance, not by its name alone.
func isSimulationConfig(s *xmi.Stereotype) bool {
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

func naturalSetting(v string) (string, string) {
	n, ok := new(big.Int).SetString(strings.TrimSpace(v), 10)
	if !ok || n.Sign() < 0 {
		return "", "is not a natural number"
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
	return lexer.StringText(v), ""
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
func (m *migration) simulationConfig(e *xmi.Element, header, note string) {
	s := simulationConfig(e)
	settings, unread, notes := m.configurationSettings(s)
	target, usage, targetNotes := m.configurationTarget(e, s)
	notes = append(notes, targetNotes...)
	note = joinNotes(note, strings.Join(notes, "; "))
	m.add(e, verdictFor(note), m.v2Name(e), note)
	m.w.block(header, func() {
		saved := m.scope
		m.scope = e
		m.comments(e)
		m.w.block("@Simulation::Configuration", func() { m.w.lines(settings) })
		if target != nil {
			part := m.freshName(e, "target")
			m.w.line("part " + writeName(part) + " : " + m.ref(target, e) + ";")
			if usage != "" {
				run := m.freshName(e, "run")
				m.w.line("perform action " + writeName(run) + " ::> " + writeName(part) + "." + writeName(usage) + ";")
			}
		}
		if len(unread) > 0 {
			m.w.lines(commentLines("«SimulationConfig» settings of the simulation tool: " + strings.Join(unread, "; ")))
		}
		m.members(e)
		m.scope = saved
		m.classifierBehavior(e)
		m.stereotypeComments(e)
	})
}

// configurationSettings writes the Simulation::Configuration attributes a
// «SimulationConfig» application sets, lists the tags it keeps as a comment,
// and notes the tags with no v2 form.
func (m *migration) configurationSettings(s *xmi.Stereotype) (settings, unread, notes []string) {
	recorded := map[string]bool{"executionTarget": true}
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
		settings = append(settings, c.attribute+" = "+lit+";")
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

// configurationTarget resolves the execution target of the configuration e, the
// definition its part is typed by, and the usage of that part by which a run
// performs the target's classifier behavior; it notes whatever it cannot.
func (m *migration) configurationTarget(e *xmi.Element, s *xmi.Stereotype) (target *xmi.Element, usage string, notes []string) {
	ids := s.Tags["executionTarget"]
	switch {
	case len(ids) == 0:
		return nil, "", []string{"the configuration names no execution target, so it runs no behavior"}
	case len(ids) > 1:
		return nil, "", []string{"the configuration names " + strconv.Itoa(len(ids)) + " execution targets, and a run has one object to run on"}
	}
	t := m.model.Lookup(ids[0])
	if t == nil || t.IsProxy() {
		return nil, "", []string{"the execution target " + strconv.Quote(ids[0]) + " is outside the document, so the configuration runs no behavior"}
	}
	if m.isLibrary(t) || !m.written(t) {
		return nil, "", []string{"the execution target " + describe(t) + " is not migrated, so the configuration runs no behavior"}
	}
	var classifiers []*xmi.Element
	cat, why := m.classify(t)
	switch cat {
	case catPartDef:
		classifiers = []*xmi.Element{t}
	case catIndividualDef:
		kind, written, _ := m.individualClassifiers(t)
		if kind != catPartDef {
			return nil, "", []string{"the execution target " + describe(t) + " is written as an " + individualKeyword(kind) + ", which no part can be typed by, so the configuration runs no behavior"}
		}
		classifiers = written
	default:
		return nil, "", []string{joinNotes("the execution target "+describe(t)+" is written as a "+cat.keyword()+", which no part can be typed by, so the configuration runs no behavior", why)}
	}
	if len(classifiers) == 0 {
		return t, "", []string{"the execution target " + describe(t) + " has no written classifier, so no behavior of it is performed"}
	}
	behavior := m.inheritedClassifierBehavior(classifiers)
	if behavior == nil {
		return t, "", []string{"neither " + qualifiedName(classifiers[0]) + " nor any general of it has a classifier behavior, so the configuration only holds " + describe(t)}
	}
	_, usage, bcat := m.classifierBehaviorUsage(behavior)
	switch bcat {
	case catActionDef:
		return t, usage, nil
	case catStateDef:
		return t, "", []string{"the classifier behavior of " + qualifiedName(behavior) + " is a state machine, which a run performs as no action; the configuration only holds " + describe(t)}
	}
	return t, "", []string{"the classifier behavior of " + qualifiedName(behavior) + " is written as a " + bcat.keyword() + ", not an action def, so no action is performed"}
}

// inheritedClassifierBehavior finds, breadth first through generalizations,
// the nearest of the classifiers or their generals with a written classifier behavior.
func (m *migration) inheritedClassifierBehavior(classifiers []*xmi.Element) *xmi.Element {
	seen := map[*xmi.Element]bool{}
	queue := append([]*xmi.Element(nil), classifiers...)
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
