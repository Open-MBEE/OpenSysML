package migrate

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// This file reads opaque bodies where they stand in the model: it answers the
// translator's names from the swimlane, the behavior and the owning classifier,
// and turns a translation into the lines and report notes the writers emit.

// clockRead is the v2 expression reading the simulation clock: the current time
// of the occurrence's local clock, which every executor advances together.
const clockRead = "localClock.currentTime"

// bodyScope answers the names of one opaque body read at scope: a node or edge
// of an activity, a behavior, a constraint or a property's owner.
type bodyScope struct {
	m       *migration
	scope   *xmi.Element
	lane    *lane
	viaLane bool // a name resolved against the lane's object
	clock   bool // the body reads the simulation clock
}

// bodyScope makes the scope an opaque body read at scope is translated in.
func (m *migration) bodyScope(scope *xmi.Element) *bodyScope {
	return &bodyScope{m: m, scope: scope, lane: m.laneAt(scope)}
}

// laneAt is the innermost partition holding e, a node, edge or pin of an
// activity, or something one owns; nil outside every partition.
func (m *migration) laneAt(e *xmi.Element) *lane {
	var act *xmi.Element
	for cur := e; cur != nil; cur = cur.Parent {
		if cur.Type == "Activity" {
			act = cur
			break
		}
	}
	if act == nil || act == e {
		return nil
	}
	ls := m.lanesOf(act)
	for cur := e; cur != nil && cur != act; cur = cur.Parent {
		if l := ls.laneOf(m, cur); l != nil {
			return l
		}
	}
	return nil
}

// feature resolves a dotted name: `this` and the names of the lane's object
// first, then what the scope sees, each further step a feature of the last.
func (s *bodyScope) feature(path []string, write bool) (opaqueRef, *refusal) {
	m := s.m
	full := strings.Join(path, ".")
	var expr string
	var f *xmi.Element
	if path[0] == "this" {
		switch {
		case s.lane != nil && s.lane.expr != "" && s.lane.typ != nil:
			expr, f = s.lane.expr, s.lane.typ
			s.viaLane = true
		case m.contextClassifier(s.scope) != nil:
			expr, f = "this", m.contextClassifier(s.scope)
		default:
			return opaqueRef{}, &refusal{kind: refusedContext, token: "this",
				why: "the body is in no classifier and its partition represents no object"}
		}
		if len(path) == 1 {
			if write {
				return opaqueRef{}, &refusal{kind: refusedContext, token: "this", why: "the object itself is not assigned"}
			}
			return opaqueRef{expr: expr}, nil
		}
	} else {
		name := path[0]
		if len(path) == 1 && m.isClockName(name) {
			if write {
				return opaqueRef{}, &refusal{kind: refusedConstruct, token: name, why: "the simulation clock is read, never assigned"}
			}
			s.clock = true
			return opaqueRef{expr: clockRead, scalar: "Real"}, nil
		}
		if lf := m.laneFeature(s.lane, name); lf != nil {
			expr, f = s.lane.expr+"."+writeName(m.nameOf(lf)), lf
			s.viaLane = true
		} else {
			visible, hidden := m.visibleFrom(s.scope)
			f = visible[name]
			switch {
			case f == nil && hidden[name] != nil:
				return opaqueRef{}, &refusal{kind: refusedName, token: name,
					why: "it is private to " + qualifiedName(hidden[name].Parent)}
			case f == nil:
				return opaqueRef{}, &refusal{kind: refusedName, token: name,
					why: "nothing visible from " + qualifiedName(s.scope) + " is called " + name}
			case f.Type != "Property" && f.Type != "Port" && f.Type != "Parameter":
				return opaqueRef{}, &refusal{kind: refusedName, token: name,
					why: "it is " + kindOf(f) + " " + qualifiedName(f) + ", not a feature a body reads"}
			}
			expr = writeName(m.nameOf(f))
			if m.ownedByClassifier(f, s.scope) {
				expr = "this." + expr
			}
		}
	}
	for _, step := range path[1:] {
		visible, hidden := m.membersOf(f, memberAny)
		next := visible[step]
		switch {
		case next == nil && hidden[step] != nil:
			return opaqueRef{}, &refusal{kind: refusedName, token: full,
				why: step + " is private to " + qualifiedName(hidden[step].Parent)}
		case next == nil:
			return opaqueRef{}, &refusal{kind: refusedName, token: full,
				why: qualifiedName(f) + " has no feature " + step}
		case next.Type != "Property" && next.Type != "Port":
			return opaqueRef{}, &refusal{kind: refusedName, token: full,
				why: step + " is " + kindOf(next) + ", not a feature a body reads"}
		}
		expr += "." + writeName(m.nameOf(next))
		f = next
	}
	if write && f.Type == "Parameter" && f.Attrs["direction"] == "in" {
		return opaqueRef{}, &refusal{kind: refusedConstruct, token: full, why: "an in parameter is not assigned"}
	}
	if s.lane != nil && s.viaLane {
		s.lane.used = true
	}
	_, upper, ok := bounds(f)
	return opaqueRef{
		expr:   expr,
		scalar: m.scalarBase(m.model.Ref(f, "type")),
		plural: ok && upper != 1,
	}, nil
}

// note says how the body's names were read, for the report entry of what it became.
func (s *bodyScope) note(lang string) string {
	note := ""
	if lang != "" {
		note = "the " + lang + " body is translated to v2"
	}
	if s.viaLane && s.lane != nil {
		note = joinNotes(note, "names resolve against "+strings.TrimPrefix(s.lane.note, "the partition represents "))
	}
	if s.clock {
		note = joinNotes(note, "the clock variable reads the local clock")
	}
	return note
}

// noted records on scope's report entry how a body read there was translated,
// unless scope is a classifier or package, whose entry is not about the body.
func (m *migration) noted(scope *xmi.Element, note string) {
	if note == "" || m.contextClassifier(scope) == scope {
		return
	}
	switch scope.Type {
	case "Package", "Model", "Profile":
		return
	}
	m.add(scope, Mapped, "", note)
}

// translatedExpr translates an opaque body as one expression read at scope,
// wanting a scalar ("" for any); the note is for the report and the refusal
// is returned when the body has no v2 form, with the v2 text checked to parse.
func (m *migration) translatedExpr(body, lang string, scope *xmi.Element, want string) (expr, note string, err *refusal) {
	s := m.bodyScope(scope)
	t, err := translateExpr(body, lang, s, want)
	if err != nil {
		return "", "", err
	}
	if _, ok := parseExpr(t.expr); !ok {
		return "", "", &refusal{kind: refusedSyntax, token: body, why: "its translation " + strconv.Quote(t.expr) + " is not v2 expression syntax"}
	}
	return t.expr, s.note(lang), nil
}

// translatedStatements translates an opaque body as the statements of an action
// body read at scope, each checked to parse.
func (m *migration) translatedStatements(body, lang string, scope *xmi.Element) (lines []string, note string, err *refusal) {
	s := m.bodyScope(scope)
	lines, err = translateStatements(body, lang, s)
	if err != nil {
		return nil, "", err
	}
	for _, line := range lines {
		if !parseStatement(line) {
			return nil, "", &refusal{kind: refusedSyntax, token: body, why: "its translation " + strconv.Quote(line) + " is not v2 syntax"}
		}
	}
	return lines, s.note(lang), nil
}

// parseStatement reports whether line parses, without diagnostics, as one
// member of an action body.
func parseStatement(line string) bool {
	src := source.New("probe.sysml", []byte("action def Probe { "+line+" }"))
	p := parser.New(src)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 || len(root.Members) != 1 {
		return false
	}
	mem, ok := root.Members[0].(*ast.Membership)
	if !ok {
		return false
	}
	def, ok := mem.Member.(*ast.Definition)
	return ok && len(def.Members) == 1
}

// isClockName reports whether name is what a simulation configuration in the
// document calls the clock variable.
func (m *migration) isClockName(name string) bool {
	_, ok := m.clockNames()[name]
	return ok
}

// defaultClockName is what the simulation toolkit calls the clock variable
// when no configuration renames it.
const defaultClockName = "simtime"

// clockNames maps each name a SimulationConfig gives the clock variable to the
// configuration naming it. The simulation profile calls it simtime unless a
// configuration says otherwise; a document without the profile has no clock variable.
func (m *migration) clockNames() map[string]string {
	if m.clocks != nil {
		return m.clocks
	}
	m.clocks = map[string]string{}
	var configs []*xmi.Element
	profiled := false
	var walk func(e *xmi.Element)
	walk = func(e *xmi.Element) {
		for _, s := range e.Stereotypes {
			if isSimulationProfile(s) {
				profiled = true
				if s.Name == "SimulationConfig" {
					configs = append(configs, e)
				}
			}
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	for _, r := range m.model.Roots {
		walk(r)
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].ID < configs[j].ID })
	for _, e := range configs {
		for _, s := range e.Stereotypes {
			if !isSimulationProfile(s) || s.Name != "SimulationConfig" {
				continue
			}
			name := strings.TrimSpace(s.Tag("timeVariableName"))
			if name == "" {
				name = defaultClockName
			}
			if _, ok := m.clocks[name]; !ok {
				m.clocks[name] = qualifiedName(e)
			}
		}
	}
	if profiled && len(m.clocks) == 0 {
		m.clocks[defaultClockName] = "the simulation profile's default"
	}
	return m.clocks
}

// isSimulationProfile reports whether s comes from the simulation toolkit's
// profile, told by the namespace it was serialized under.
func isSimulationProfile(s *xmi.Stereotype) bool {
	return strings.Contains(strings.ToLower(s.Namespace), "simulationprofile")
}
