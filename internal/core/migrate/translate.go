package migrate

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/xmi/sysmlv1"
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
	scope   *sysmlv1.Element
	lane    *lane
	clash   string // why no lane applies, when partitions of different dimensions hold the scope
	viaLane bool   // a name resolved against the lane's object
	clock   string // the clock variable the body reads and who names it; "" when it does not
}

// bodyScope makes the scope an opaque body read at scope is translated in.
func (m *migration) bodyScope(scope *sysmlv1.Element) *bodyScope {
	l, clash := m.laneAt(scope)
	return &bodyScope{m: m, scope: scope, lane: l, clash: clash}
}

// laneAt is the partition e, a node, edge or pin of an activity, or something
// one owns, resolves names against; nil outside every partition, or with why
// none applies when partitions of different dimensions hold it.
func (m *migration) laneAt(e *sysmlv1.Element) (*lane, string) {
	ls, act := m.lanesAround(e)
	for cur := e; ls != nil && cur != act; cur = cur.Parent {
		h, role := ls.holder(m, cur)
		if l := ls.pick(h); l != nil {
			return l, ""
		}
		subject := "it"
		if role != "" {
			subject = "its " + role + " " + describe(h)
		}
		if why := ls.clashNote(h, subject); why != "" {
			return nil, why
		}
	}
	return nil, ""
}

// useLane records that a name at e resolved through lane l, the lane laneAt found.
func (m *migration) useLane(e *sysmlv1.Element, l *lane) {
	ls, act := m.lanesAround(e)
	for cur := e; ls != nil && cur != act; cur = cur.Parent {
		if ls.laneOf(m, cur) == l {
			ls.use(cur, l)
			return
		}
	}
	l.used = true
}

// lanesAround is the partition index of the activity e is inside, with the
// activity; nil when e is in none or is the activity itself.
func (m *migration) lanesAround(e *sysmlv1.Element) (*lanes, *sysmlv1.Element) {
	for cur := e; cur != nil; cur = cur.Parent {
		if cur.Type == "Activity" {
			if cur == e {
				return nil, nil
			}
			return m.lanesOf(cur), cur
		}
	}
	return nil, nil
}

// feature resolves a dotted name: `this` and the names of the lane's object
// first, then what the scope sees, each further step a feature of the last.
// The result is plural once any object the name reads through is, and a
// write through a collection is refused: it would reach several objects.
func (s *bodyScope) feature(path []string, write bool) (opaqueRef, *refusal) {
	m := s.m
	full := strings.Join(path, ".")
	var expr string
	var f *sysmlv1.Element
	var plural bool    // whether the objects the name reads through are a collection
	var carrier string // the first such collection
	if path[0] == "this" {
		switch {
		case s.lane != nil && s.lane.expr != "" && s.lane.typ != nil:
			expr, f, plural, carrier = s.lane.expr, s.lane.typ, s.lane.plural, s.lane.expr
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
			return opaqueRef{expr: expr, plural: plural}, nil
		}
	} else if p, d := m.pinNamed(s.scope, path[0]); p != nil {
		if write && len(path) == 1 && d.dir == "in" {
			return opaqueRef{}, &refusal{kind: refusedConstruct, token: full, why: "an input pin is not assigned"}
		}
		expr, f = writeName(d.name), p
	} else {
		name := path[0]
		if lf := m.laneFeature(s.lane, name); lf != nil {
			expr, f, plural, carrier = s.lane.expr+"."+writeName(m.nameOf(lf)), lf, s.lane.plural, s.lane.expr
			s.viaLane = true
		} else {
			visible, hidden := m.visibleFrom(s.scope)
			f = visible[name]
			// The clock variable is the tool's global; any feature of that name shadows it.
			if by, clock := m.clockNames()[name]; clock && f == nil && hidden[name] == nil && len(path) == 1 {
				if write {
					return opaqueRef{}, &refusal{kind: refusedConstruct, token: name, why: "the simulation clock is read, never assigned"}
				}
				s.clock = name + ", " + by
				return opaqueRef{expr: clockRead, scalar: "Real"}, nil
			}
			switch {
			case f == nil && hidden[name] != nil:
				return opaqueRef{}, &refusal{kind: refusedName, token: name,
					why: "it is private to " + qualifiedName(hidden[name].Parent)}
			case f == nil:
				return opaqueRef{}, &refusal{kind: refusedName, token: name,
					why: joinNotes("nothing visible from "+qualifiedName(s.scope)+" is called "+name, s.clash)}
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
		if !plural && manyValued(f) {
			plural, carrier = true, expr
		}
		typ := m.typedAs(f)
		if typ == nil {
			return opaqueRef{}, &refusal{kind: refusedName, token: full,
				why: qualifiedName(f) + " has no type, so no feature " + step}
		}
		visible, hidden := m.membersOf(typ, memberAny)
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
	if dir, _ := parameterDirection(f); write && f.Type == "Parameter" && dir == "in" {
		return opaqueRef{}, &refusal{kind: refusedConstruct, token: full, why: "an in parameter is not assigned"}
	}
	if write && plural {
		return opaqueRef{}, &refusal{kind: refusedConstruct, token: full,
			why: carrier + " is a collection, so the assignment would write through several objects"}
	}
	if s.lane != nil && s.viaLane {
		m.useLane(s.scope, s.lane)
	}
	return opaqueRef{
		expr:   expr,
		scalar: m.scalarBase(m.typedAs(f)),
		object: m.nonScalar(m.typedAs(f)),
		plural: plural || manyValued(f),
	}, nil
}

// manyValued reports whether feature f holds other than exactly one value.
func manyValued(f *sysmlv1.Element) bool {
	_, upper, ok := bounds(f)
	return ok && upper != 1
}

// wantedOf is what a value of feature f must be: of the type f holds, and
// one value unless f holds several.
func (m *migration) wantedOf(f *sysmlv1.Element) wanted {
	t := m.typedAs(f)
	return wanted{scalar: m.scalarBase(t), object: m.nonScalar(t), single: !manyValued(f)}
}

// nonScalar names t, then every type generalizing it, when its values are
// known to be no scalar: an enumeration, a block or another classifier, or a
// value type whose every base is one such.
func (m *migration) nonScalar(t *sysmlv1.Element) []string {
	type known struct {
		names []string
		ok    bool
	}
	memo := map[*sysmlv1.Element]known{}
	var types func(*sysmlv1.Element) ([]string, bool)
	types = func(t *sysmlv1.Element) ([]string, bool) {
		if t == nil || t.IsProxy() || m.scalarBase(t) != "" {
			return nil, false
		}
		if k, seen := memo[t]; seen {
			return k.names, k.ok
		}
		memo[t] = known{}
		cat, _ := m.classify(t)
		switch cat {
		case catNone, catLibrary, catUnmapped:
			return nil, false
		}
		names := []string{m.v2Name(t)}
		for _, g := range t.Owned("generalization") {
			general, ok := types(m.model.Ref(g, "general"))
			if !ok && cat == catAttributeDef {
				return nil, false
			}
			names = append(names, general...)
		}
		memo[t] = known{names, true}
		return names, true
	}
	names, _ := types(t)
	return names
}

// typedAs is the classifier typing f: a pin's as declared, else its own type;
// for anything but a feature or pin, f itself, whose members are its own.
func (m *migration) typedAs(f *sysmlv1.Element) *sysmlv1.Element {
	if d, ok := m.pins[f]; ok {
		return d.typ
	}
	switch f.Type {
	case "Property", "Port", "Parameter":
		return m.model.Ref(f, "type")
	}
	return f
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
	if s.clock != "" {
		note = joinNotes(note, "the clock variable "+s.clock+", reads the local clock")
	}
	return note
}

// noted records on scope's report entry how a body read there was translated,
// unless scope is a classifier or package, whose entry is not about the body.
func (m *migration) noted(scope *sysmlv1.Element, note string) {
	if note == "" || m.contextClassifier(scope) == scope {
		return
	}
	switch scope.Type {
	case "Package", "Model", "Profile":
		return
	}
	m.add(scope, Mapped, "", note)
}

// translatedExpr translates an opaque body as one expression read at scope
// yielding what want asks for; the note is for the report and the refusal is
// returned when the body has no v2 form, with the v2 text checked to parse.
func (m *migration) translatedExpr(body, lang string, scope *sysmlv1.Element, want wanted) (expr, note string, err *refusal) {
	s := m.bodyScope(scope)
	t, err := translateExpr(body, lang, s, want)
	if err != nil {
		return "", "", err
	}
	expr = spellFor(want.scalar, t)
	if _, ok := parseExpr(expr); !ok {
		return "", "", &refusal{kind: refusedSyntax, token: body, why: "its translation " + strconv.Quote(expr) + " is not v2 expression syntax"}
	}
	return expr, s.note(lang), nil
}

// translatedStatements translates an opaque body as the statements of an action
// body read at scope, each checked to parse.
func (m *migration) translatedStatements(body, lang string, scope *sysmlv1.Element) (lines []string, note string, err *refusal) {
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
	if note = s.note(lang); note == "" {
		note = "the body is translated to v2"
	}
	return lines, note, nil
}

// symbolicDuration reads a duration written as an expression, optionally
// followed by a time unit (`ditSetup s`, `t * 2 min`), as seconds read at scope.
func (m *migration) symbolicDuration(text, lang string, scope *sysmlv1.Element) (expr string, ok bool, note string) {
	body := strings.TrimSpace(durationVariable.ReplaceAllString(strings.TrimSpace(text), ""))
	scale := 1.0
	if i := strings.LastIndexAny(body, " \t"); i >= 0 {
		if s, known := durationUnits[strings.ToLower(body[i+1:])]; known {
			body, scale = strings.TrimSpace(body[:i]), s
		}
	}
	if body == "" {
		return "", false, "the duration has no expression"
	}
	expr, ok, note = m.behaviorExprAs(body, lang, scope, oneOf("Real"))
	if !ok {
		return "", false, note
	}
	if scale != 1 {
		if strings.ContainsAny(expr, " (") {
			expr = "(" + expr + ")"
		}
		expr += " * " + realLiteral(scale)
	}
	return expr, true, "the duration " + strconv.Quote(strings.TrimSpace(text)) + " is read as the expression " + expr + ", in seconds"
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

// defaultClockName is what the simulation toolkit calls the clock variable
// when no configuration renames it.
const defaultClockName = "simtime"

// clockNames maps each name a SimulationConfig gives the clock variable to who
// names it, for the report. The simulation profile calls it simtime unless a
// configuration says otherwise; a document without the profile has no clock variable.
func (m *migration) clockNames() map[string]string {
	if m.clocks != nil {
		return m.clocks
	}
	m.clocks = map[string]string{}
	var configs []*sysmlv1.Element
	profiled := false
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
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
	namers := map[string][]*sysmlv1.Element{}
	for _, e := range configs {
		for _, s := range e.Stereotypes {
			if !isSimulationProfile(s) || s.Name != "SimulationConfig" {
				continue
			}
			name := strings.TrimSpace(s.Tag("timeVariableName"))
			if name == "" {
				name = defaultClockName
			}
			if !slices.Contains(namers[name], e) {
				namers[name] = append(namers[name], e)
			}
		}
	}
	for name, by := range namers {
		if len(by) == 1 {
			m.clocks[name] = "named by the configuration " + qualifiedName(by[0])
		} else {
			m.clocks[name] = "named by " + strconv.Itoa(len(by)) + " simulation configurations"
		}
	}
	if profiled && len(m.clocks) == 0 {
		m.clocks[defaultClockName] = "the simulation profile's default name"
	}
	return m.clocks
}

// isSimulationProfile reports whether s comes from the simulation toolkit's
// profile, told by the namespace it was serialized under.
func isSimulationProfile(s *sysmlv1.Stereotype) bool {
	return strings.Contains(strings.ToLower(s.Namespace), "simulationprofile")
}
