package migrate

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// writeName writes a v1 name as a v2 name: bare when it is a basic identifier
// no keyword reserves, quoted as an unrestricted name otherwise.
func writeName(name string) string {
	if source.IsIdentifier(name) && !source.IsKeyword(name) {
		return name
	}
	return source.UnrestrictedNameText(name)
}

// nameOf returns the v2 name of an element: its own, or the one synthesized
// for an anonymous element that something refers to; "" when it is anonymous
// and stays so.
func (m *migration) nameOf(e *sysmlv1.Element) string {
	if n, ok := m.names[e]; ok {
		return n
	}
	return e.Name
}

// nameFor returns the v2 name of an element, synthesizing one for an anonymous
// element the first time it is asked for, so it can be referred to.
func (m *migration) nameFor(e *sysmlv1.Element) string {
	// An action node is named as its graph's writer names it.
	if n := m.nameNode(e); n != "" {
		return n
	}
	if n := m.nameOf(e); n != "" {
		return n
	}
	// Distinct within the owner: the id is unique, so its tail is too once the
	// whole is used on a clash.
	base := "unnamed"
	if t := m.model.Ref(e, "type"); t != nil && t.Name != "" {
		base = lowerFirst(t.Name)
	}
	name := base
	for i := 2; m.nameTaken(e.Parent, name); i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	m.names[e], m.synthesized[e] = name, true
	return name
}

// madeUp records that the block being written declares e under name, a name
// the migration made up when e's source left it unnamed: the block's
// SynthesizedName marker lists it. written is the name as the notation writes it.
func (m *migration) madeUp(e *sysmlv1.Element, written string) {
	if m.synthesized[e] && written != "" {
		m.w.madeUp(written)
	}
}

// synthesizedNameFQN names the library metadata marking a made-up name.
const synthesizedNameFQN = "MigrationMetadata::SynthesizedName"

// standInFQN names the library metadata marking a member that stands for no
// source element.
const standInFQN = "MigrationMetadata::StandIn"

// synthesizedNames is the metadata usage marking the members of the current
// scope written under made-up names, each as the notation wrote it.
func (m *migration) synthesizedNames(written []string) string {
	return m.migrationMarker(synthesizedNameFQN, written)
}

// standInNames is the metadata usage marking the members of the current scope
// that stand for no source element, each as the notation wrote it.
func (m *migration) standInNames(written []string) string {
	return m.migrationMarker(standInFQN, written)
}

// strictMarkerText words the comment a strict migration writes instead of
// each MigrationMetadata marker, the library being OpenSysML's, not the standard's.
var strictMarkerText = map[string]string{
	synthesizedNameFQN: "names the migration made up",
	standInFQN:         "members standing for no source element",
}

// migrationMarker is the metadata usage of the MigrationMetadata definition
// fqn about the written names, qualified past a member shadowing the library;
// a strict migration writes a comment instead.
func (m *migration) migrationMarker(fqn string, written []string) string {
	if m.strict {
		return "// " + strictMarkerText[fqn] + ": " + strings.Join(written, ", ")
	}
	prefix := ""
	if m.shadowsLibrary("MigrationMetadata", m.scope) {
		prefix = "$::"
	}
	return "metadata " + prefix + fqn + " about " + strings.Join(written, ", ") + ";"
}

// writtenName returns the name e's v2 declaration bears, as a query reads it back:
// nameOf, or the one the write gives an anonymous declaration that needs a name.
func (m *migration) writtenName(e *sysmlv1.Element) string {
	if n := m.nameOf(e); n != "" || !m.written(e) {
		return n
	}
	// A classifier is declared by name; a usage stays anonymous when typed, a
	// connector always, and an association written as an actor or as its ends names nothing.
	switch e.Type {
	case "Connector":
		return ""
	case "Property", "Port":
		if typ, _ := m.typeRef(m.model.Ref(e, "type"), e.Parent); typ != "" {
			return ""
		}
	case "Association", "AssociationClass":
		if !m.associationAsConnectionDef(e) {
			return ""
		}
	}
	return m.nameFor(e)
}

func (m *migration) nameTaken(owner *sysmlv1.Element, name string) bool {
	return m.nameTakenBut(nil, owner, name)
}

func (m *migration) nameTakenExcept(owner *sysmlv1.Element, name string, except *sysmlv1.Element) bool {
	return m.nameTakenBut(except, owner, name)
}

// nameTakenBut is nameTaken disregarding e, whose own name the declaration
// written for it is free to keep.
func (m *migration) nameTakenBut(e, owner *sysmlv1.Element, name string) bool {
	if m.taken[owner][name] {
		return true
	}
	if owner == nil {
		return m.topLevelNamedBut(e, name)
	}
	for _, c := range owner.Children {
		if c != e && m.nameOf(c) == name {
			return true
		}
	}
	return false
}

// topLevelNamedBut is topLevelNamed disregarding e (see nameTakenBut).
func (m *migration) topLevelNamedBut(e *sysmlv1.Element, name string) bool {
	for _, r := range m.model.Roots {
		switch {
		case m.flattened(r):
			if m.nameTakenBut(e, r, name) {
				return true
			}
		case r != e && r.Type != "Model" && m.nameOf(r) == name:
			return true
		}
	}
	return false
}

// take reserves a synthesized member name in owner's body.
func (m *migration) take(owner *sysmlv1.Element, name string) {
	m.takeFor(nil, owner, name)
}

// takeFor reserves name in owner for the declaration written for e, which
// claimName hands it to once.
func (m *migration) takeFor(e, owner *sysmlv1.Element, name string) {
	if m.taken[owner] == nil {
		m.taken[owner] = map[string]bool{}
		m.takenBy[owner] = map[string]*sysmlv1.Element{}
	}
	m.taken[owner][name] = true
	m.takenBy[owner][name] = e
}

// reserveNameFor is freshNameBut reserving the name for e to claim (see claimName).
func (m *migration) reserveNameFor(e, owner *sysmlv1.Element, name string) string {
	name = m.freshNameBut(e, owner, name)
	m.takenBy[owner][name] = e
	return name
}

// claimName is the name a declaration written for e takes in owner: the one
// reserved for it, consumed, or else name made fresh.
func (m *migration) claimName(e, owner *sysmlv1.Element, name string) string {
	if in, ok := m.reservedIn(e, owner, name); ok {
		m.takenBy[in][name] = nil
		return name
	}
	return m.freshNameBut(e, owner, name)
}

// reservedIn is the owner, among those nameTakenBut consults for owner, in
// which name is reserved for e; the top level and a flattened root are one.
func (m *migration) reservedIn(e, owner *sysmlv1.Element, name string) (*sysmlv1.Element, bool) {
	if e == nil {
		return nil, false
	}
	if m.takenBy[owner][name] == e {
		return owner, true
	}
	if owner == nil {
		for _, r := range m.model.Roots {
			if m.flattened(r) && m.takenBy[r][name] == e {
				return r, true
			}
		}
	} else if m.flattened(owner) && m.takenBy[nil][name] == e {
		return nil, true
	}
	return nil, false
}

// identifierSuffix turns a name into the tail of a compound identifier: its
// letters, digits and underscores, each run after a dropped rune capitalized.
func identifierSuffix(name string) string {
	var b strings.Builder
	upper := true
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || r == '_' || (unicode.IsDigit(r) && b.Len() > 0):
			if upper {
				r = unicode.ToUpper(r)
			}
			b.WriteRune(r)
			upper = false
		default:
			upper = true
		}
	}
	if b.Len() == 0 {
		return "Signal"
	}
	return b.String()
}

func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

// segments returns the v2 qualified-name segments of an element: the names
// from the top-level declaration down, the root Model not being written.
// A connection point is a member of its owner, whichever region a tool listed it in;
// a method is the body of its operation.
func (m *migration) segments(e *sysmlv1.Element) []string {
	path := m.path(e)
	segs := make([]string, len(path))
	for i, s := range path {
		segs[i] = s.name
	}
	return segs
}

// segment is one step of a qualified name; feature marks a step that is a
// usage, elem the element it names, nil for a step no element stands for.
type segment struct {
	name    string
	feature bool
	elem    *sysmlv1.Element
}

// path returns the segments of e's qualified name, see segments. A behavior
// that is the method of an operation is written as that operation's body, so
// it and its members are named under the operation; an edge's members (a
// transition's effect) are named under the member the edge was written as.
func (m *migration) path(e *sysmlv1.Element) []segment {
	var segs []segment
	for cur := e; cur != nil; cur = memberOwner(cur) {
		if em, ok := m.edgeMembers[cur]; ok && em.name != "" {
			return append(m.edgePath(em.edgePlace), segs...)
		}
		if op := m.methodOf[cur]; op != nil {
			cur = op
		}
		if cur.Parent == nil && cur.Type == "Model" {
			break
		}
		var wrappedPointRegion *sysmlv1.Element
		if owner := pointOwner(cur); owner != nil && owner.Type == "State" {
			if regions := m.populatedRegions(owner); len(regions) == 1 &&
				m.regionWrittenAsState(regions[0]) && m.regionStates[regions[0]] == "" {
				wrappedPointRegion = regions[0]
			}
		}
		if cur.Type == "Region" && cur.Role == "region" {
			if m.regionWrittenAsState(cur) {
				if p := m.regionStates[cur]; p != "" {
					segs = append([]segment{{name: p}, {name: m.nameFor(cur)}}, segs...)
				} else {
					segs = append([]segment{{name: m.nameFor(cur), feature: true, elem: cur}}, segs...)
				}
			}
			continue
		}
		if op := m.methodOf[cur]; op != nil {
			cur = op
		}
		segs = append([]segment{{name: m.nameFor(cur), feature: m.isUsage(cur), elem: cur}}, segs...)
		if within, ok := m.nestedIn[cur]; ok {
			segs = append([]segment{{name: within, feature: true}}, segs...)
		}
		if wrappedPointRegion != nil {
			segs = append([]segment{{name: m.nameFor(wrappedPointRegion), feature: true, elem: wrappedPointRegion}}, segs...)
		}
	}
	return segs
}

// acceptSignalRef qualifies a signal reference when its name matches the payload.
func (m *migration) acceptSignalRef(sig, scope *sysmlv1.Element, payload string) string {
	if payload == "" {
		return m.ref(sig, scope)
	}
	path := m.path(sig)
	if payload != writeName(m.nameFor(sig)) {
		ref := m.ref(sig, scope)
		if strings.HasPrefix(ref, "$::") {
			return ref
		}
		if leadingSegment(ref) != payload {
			return ref
		}
		return "$::" + strings.TrimPrefix(m.qualifiedFrom(path, nil, false), "$::")
	}
	if len(path) == 1 {
		return "$::" + writeName(path[0].name)
	}
	ref := m.qualifiedFrom(path, scopeChain(scope), false)
	if writeName(path[0].name) == payload && !strings.HasPrefix(ref, "$::") {
		return "$::" + ref
	}
	return ref
}

// leadingSegment is the first segment of a written reference, up to its first
// `::` or `.` outside a quoted name.
func leadingSegment(ref string) string {
	quoted := false
	for i := 0; i < len(ref); i++ {
		switch {
		case quoted && ref[i] == '\\':
			i++
		case ref[i] == '\'':
			quoted = !quoted
		case !quoted && (ref[i] == '.' || strings.HasPrefix(ref[i:], "::")):
			return ref[:i]
		}
	}
	return ref
}

// isUsage says whether e is written as a usage whose members are features of
// it: a view or viewpoint, a property, or a behavior written as its block's
// action usage. A feature owned by one is reached by a feature chain, not a
// qualified name.
func (m *migration) isUsage(e *sysmlv1.Element) bool {
	switch e.Type {
	case "Property", "Port":
		return true
	}
	if m.asUsage[e] {
		return true
	}
	cat, _ := m.classify(e)
	return cat == catView || cat == catUseCase || cat == catActor || cat == catRequirement
}

// scopeChain lists scope and its ancestors, innermost first, stopping at the
// root Model, which is no scope of the output.
func scopeChain(scope *sysmlv1.Element) []*sysmlv1.Element {
	var chain []*sysmlv1.Element
	for cur := scope; cur != nil; cur = cur.Parent {
		if cur.Parent == nil && cur.Type == "Model" {
			break
		}
		chain = append(chain, cur)
	}
	return chain
}

// inside writes body inside a synthesized declaration whose members are named
// names: a reference written there resolves through those members first, so
// one naming a member steers clear of them.
func (m *migration) inside(names columnNames, body func()) {
	m.opened = append(m.opened, names)
	body()
	m.opened = m.opened[:len(m.opened)-1]
}

// hidden reports whether a synthesized declaration being written declares a
// member named name, which hides the name outside it.
func (m *migration) hidden(name string) bool {
	for _, names := range m.opened {
		if names[name] {
			return true
		}
	}
	return false
}

// siblingRef writes a reference to a synthesized declaration named name that
// is written beside host's members, from inside whatever is being written.
func (m *migration) siblingRef(host *sysmlv1.Element, name string) string {
	return m.synthesizedRef(host, name, host)
}

// synthesizedRef writes a reference from inside scope's body to a synthesized
// declaration named name that is written beside host's members.
func (m *migration) synthesizedRef(host *sysmlv1.Element, name string, scope *sysmlv1.Element) string {
	return m.refMember(host, name, append(m.path(host), segment{name: name}), scope, false)
}

// ref writes a reference to target from inside scope's body (nil for the top
// level): the shortest qualified name that resolves there, which is the simple
// name when target is a member of an enclosing scope no nearer scope shadows,
// and the full qualified name otherwise. A feature of a feature is chained. An
// edge's member is referred to where the writer placed it, not under its v1 owner.
func (m *migration) ref(target, scope *sysmlv1.Element) string {
	if em, ok := m.edgeMembers[target]; ok && em.name != "" {
		return m.refEdge(em.edgePlace, scope)
	}
	return m.refMember(target.Parent, m.nameOf(target), m.path(target), scope, true)
}

// memberRef writes a reference to target from inside scope's body as an import
// or expose names its member: by qualified name alone, never a feature chain.
func (m *migration) memberRef(target, scope *sysmlv1.Element) string {
	return m.refMember(target.Parent, m.nameOf(target), m.path(target), scope, false)
}

// refMember writes a reference from inside scope's body to the member of owner
// named name, whose qualified name is path: a synthesized declaration written
// beside owner's members refers like one of them. A feature of a feature is
// chained when chained is set.
func (m *migration) refMember(owner *sysmlv1.Element, name string, path []segment, scope *sysmlv1.Element, chained bool) string {
	if owner != nil && owner.Type == "Model" && owner.Parent == nil {
		owner = nil
	}
	chain := scopeChain(scope)
	for i, s := range chain {
		if s != owner {
			continue
		}
		shadowed := m.hidden(name)
		for _, inner := range chain[:i] {
			if name != "" && m.nameTaken(inner, name) {
				shadowed = true
				break
			}
		}
		if !shadowed {
			return writeName(path[len(path)-1].name)
		}
	}
	if owner == nil && len(chain) > 0 {
		// A top-level declaration: visible everywhere unless shadowed.
		if m.hidden(name) {
			return m.qualifiedFrom(path, chain, chained)
		}
		for _, inner := range chain {
			if name != "" && m.nameTaken(inner, name) {
				return m.qualifiedFrom(path, chain, chained)
			}
		}
		return writeName(path[len(path)-1].name)
	}
	return m.qualifiedFrom(path, chain, chained)
}

// namespaces is the path of a qualified name whose every segment is a namespace.
func namespaces(segs []string) []segment {
	path := make([]segment, len(segs))
	for i, s := range segs {
		path[i] = segment{name: s}
	}
	return path
}

// qualifiedFrom writes path, a qualified name, so it resolves from inside the
// scopes of chain: from the global namespace ($::) when one of them declares a
// member named like its first segment, which would shadow the relative path.
// When chained, a feature owned by a feature is reached by a chain: `Outer.inner`,
// since a usage's members are not accessible by qualified name where a feature
// is referred to; an import names them by qualified name alone.
func (m *migration) qualifiedFrom(path []segment, chain []*sysmlv1.Element, chained bool) string {
	var b strings.Builder
	if m.hidden(path[0].name) || m.shadows(chain, path[0].name) {
		b.WriteString("$::")
	}
	for i, s := range path {
		switch {
		case i == 0:
		case chained && s.feature && path[i-1].feature:
			b.WriteString(".")
		default:
			b.WriteString("::")
		}
		b.WriteString(writeName(s.name))
	}
	return b.String()
}

func (m *migration) qualified(segs []string) string {
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = writeName(s)
	}
	return strings.Join(parts, "::")
}

// v2Name is the qualified name a report entry records for a written element.
func (m *migration) v2Name(e *sysmlv1.Element) string {
	return m.qualified(m.segments(e))
}
