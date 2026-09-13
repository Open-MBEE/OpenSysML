// Package migrate writes a SysML v1 model, read from XMI, as SysML v2 textual
// notation, and reports what each v1 element became.
package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// The v2 keywords the writer prefixes a declaration or annotation with.
const (
	privatePrefix = "private "
	commentPrefix = "comment "
)

// Result is a migration's output: the v2 notation and the report over it.
type Result struct {
	Notation []byte
	Report   *Report
}

// Migrate reads a SysML v1 model as UML XMI, or a zip archive (such as a
// .mdzip) holding it, and writes it as SysML v2 notation. name labels the
// source in the report.
func Migrate(name string, data []byte) (*Result, error) {
	model, err := xmi.Parse(data)
	if err != nil {
		return nil, err
	}
	return FromModel(name, model), nil
}

// FromModel migrates an already-read XMI model.
func FromModel(name string, model *xmi.Model) *Result {
	m := &migration{
		model:    model,
		report:   &Report{Source: name, Exporter: model.Exporter},
		w:        &writer{},
		names:    map[*xmi.Element]string{},
		extras:   map[*xmi.Element][]func(){},
		flows:    map[*xmi.Element][]*xmi.Element{},
		outcomes: map[*xmi.Element]*flowOutcome{},
		unplaced: map[*xmi.Element]*placement{},
		taken:    map[*xmi.Element]map[string]bool{},
	}
	m.prepare()
	for _, root := range model.Roots {
		m.root(root)
	}
	m.flushFlows()
	m.extensions()
	return &Result{Notation: []byte(m.w.String()), Report: m.report}
}

// flowOutcome gathers what each realizing connector did for one item flow, so
// the flow is reported once however many connectors realize it.
type flowOutcome struct {
	pending int
	written []string
	notes   []string
}

// extensions accounts for the diagrams and other tool-private content the
// reader skipped, so a report never loses them silently.
func (m *migration) extensions() {
	for _, ext := range m.model.Extensions {
		note := "tool-private xmi:Extension content"
		if ext.Extender != "" {
			note += " written by " + ext.Extender
		}
		for _, el := range ext.Elements {
			name := el.Name
			if name == "" {
				name = "<" + el.Type + ">"
			}
			if ext.Owner != nil && ext.Owner.Parent != nil {
				name = qualifiedName(ext.Owner) + "::" + name
			}
			kind := strings.TrimPrefix(el.Type, "uml:")
			m.report.Entries = append(m.report.Entries, Entry{ID: el.ID, Kind: kind, Name: name, Verdict: Skipped, Note: note})
		}
	}
}

// migration holds the state of one run.
type migration struct {
	model  *xmi.Model
	report *Report
	w      *writer
	// names holds the names synthesized for anonymous elements.
	names map[*xmi.Element]string
	// extras are members other elements contribute to a body: a Satisfy is
	// written inside the block that satisfies.
	extras map[*xmi.Element][]func()
	// flows lists the item flows each connector realizes.
	flows map[*xmi.Element][]*xmi.Element
	// outcomes accumulates each item flow's result over its realizing connectors.
	outcomes map[*xmi.Element]*flowOutcome
	// unplaced records where each Satisfy or Verify was placed, and why not.
	unplaced map[*xmi.Element]*placement
	// taken holds synthesized names reserved in a body, by owner.
	taken map[*xmi.Element]map[string]bool
	// scope is the element whose body is being written; nil at the top level.
	scope *xmi.Element
}

func (m *migration) add(e *xmi.Element, v Verdict, target, note string) {
	if n, ok := m.names[e]; ok && e.Name != "" && n != e.Name {
		if v == Mapped {
			v = Approximated
		}
		note = joinNotes(note, "written as "+n+" since a sibling is also named "+e.Name)
	}
	m.report.Entries = append(m.report.Entries, Entry{
		ID: e.ID, Kind: kindOf(e), Name: qualifiedName(e), Target: target, Verdict: v, Note: note,
	})
}

// prepare walks the model once ahead of writing: it indexes the item flows
// by realizing connector and names every anonymous feature that is referred to.
func (m *migration) prepare() {
	var walk func(e *xmi.Element)
	walk = func(e *xmi.Element) {
		m.distinguish(e)
		switch e.Type {
		case "InformationFlow":
			if cs := m.model.Refs(e, "realizingConnector"); len(cs) > 0 {
				m.outcomes[e] = &flowOutcome{pending: len(cs)}
				for _, c := range cs {
					m.flows[c] = append(m.flows[c], e)
				}
			}
		case "ConnectorEnd":
			for _, role := range []string{"role", "partWithPort"} {
				if r := m.model.Ref(e, role); r != nil {
					m.nameFor(r)
				}
			}
			if nce := stereo(e, "NestedConnectorEnd"); nce != nil {
				for _, id := range nce.Tags["propertyPath"] {
					if p := m.model.Lookup(id); p != nil {
						m.nameFor(p)
					}
				}
			}
		case "Dependency", "Abstraction", "Realization", "Usage":
			for _, role := range []string{"client", "supplier"} {
				if r := m.model.Ref(e, role); r != nil && (r.Type == "Property" || r.Type == "Port") {
					m.nameFor(r)
				}
			}
			m.placeDependency(e)
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	for _, r := range m.model.Roots {
		if !m.isLibrary(r) {
			walk(r)
		}
	}
}

// distinguish renames the later of two members of e that share a name, since
// v2 members of one namespace must be distinct while UML allows the clash.
func (m *migration) distinguish(e *xmi.Element) {
	seen := map[string]bool{}
	for _, c := range e.Children {
		if c.Name == "" || ownerWritten(c.Role) {
			continue
		}
		if !seen[c.Name] {
			seen[c.Name] = true
			continue
		}
		name := c.Name
		for i := 2; seen[name] || m.nameTaken(e, name); i++ {
			name = fmt.Sprintf("%s %d", c.Name, i)
		}
		seen[name] = true
		m.names[c] = name
	}
}

// ownerWritten reports whether a child in role is written by its owner rather
// than as a member of its body.
func ownerWritten(role string) bool {
	switch role {
	case "ownedComment", "generalization", "lowerValue", "upperValue", "defaultValue",
		"end", "specification", "type", "general", "annotatedElement", "body", "language",
		"ownedEnd", "memberEnd", "value", "slot", "ownedLiteral", "ownedParameter", "region":
		return true
	}
	return false
}

// root writes a top-level element: a Model's members are written at the top
// level, any other root as a declaration of its own.
func (m *migration) root(e *xmi.Element) {
	if e.Type == "Model" && !m.isLibrary(e) {
		m.add(e, Mapped, "", "the root model's members are written at the top level")
		m.body(e)
		return
	}
	m.member(e)
}

// body writes the members of e's body, in document order, then what other
// elements contribute to it.
func (m *migration) body(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	for _, c := range e.Children {
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.scope = saved
}

// member writes one owned element of the current scope.
func (m *migration) member(e *xmi.Element) {
	if ownerWritten(e.Role) {
		return
	}
	switch e.Role {
	case "profileApplication", "packageImport", "elementImport", "packageMerge":
		m.imports(e)
		return
	case "ownedAttribute":
		m.feature(e)
		return
	case "ownedConnector":
		m.connector(e)
		return
	case "ownedRule":
		m.rule(e)
		return
	case "ownedOperation", "ownedReception":
		m.unmapped(e, "operations and receptions are not migrated; v2 has no operation")
		return
	}
	switch e.Type {
	case "Dependency", "Abstraction", "Realization", "Usage":
		m.dependency(e)
		return
	case "InformationFlow":
		m.informationFlow(e)
		return
	case "Comment":
		// A comment in a non-ownedComment role is still a comment.
		m.comment(e)
		return
	}
	m.classifier(e)
}

// imports writes a package import; profile applications and element imports
// have no v2 counterpart worth writing.
func (m *migration) imports(e *xmi.Element) {
	if e.Type != "PackageImport" {
		m.add(e, Skipped, "", "profile applications and element imports are not written")
		return
	}
	target := m.model.Ref(e, "importedPackage")
	if target == nil || target.IsProxy() || m.isLibrary(target) {
		m.add(e, Skipped, "", "import of a profile or library package")
		return
	}
	vis := ""
	if e.Attrs["visibility"] == "private" {
		vis = privatePrefix
	}
	m.w.line(vis + "import " + m.ref(target, m.scope) + "::*;")
	m.add(e, Mapped, m.v2Name(target), "")
}

// classifier writes a package, classifier or other packaged element.
func (m *migration) classifier(e *xmi.Element) {
	cat, note := m.classify(e)
	switch cat {
	case catNone:
		return
	case catLibrary:
		m.add(e, Skipped, "", "profile or library content")
		return
	case catUnmapped:
		m.unmapped(e, note)
		return
	}
	name := m.nameOf(e)
	if name == "" {
		if cat == catConnectionDef {
			m.association(e)
			return
		}
		name = m.nameFor(e)
		if note == "" {
			note = "the anonymous " + e.Type + " is named " + name
		}
	}
	verdict := Mapped
	if note != "" {
		verdict = Approximated
	}
	var header strings.Builder
	if e.Attrs["isAbstract"] == "true" {
		header.WriteString("abstract ")
	}
	header.WriteString(cat.keyword())
	header.WriteByte(' ')
	if cat == catRequirementDef {
		if id := requirementID(e); id != "" {
			header.WriteString("<" + writeName(id) + "> ")
		}
	}
	header.WriteString(writeName(name))
	gens, n := m.generals(e, cat)
	if gens != "" {
		header.WriteString(" :> " + gens)
	}
	if cat == catConnectionDef {
		n = joinNotes(n, m.dangling(e, "memberEnd"))
	}
	if n != "" {
		verdict = Approximated
		note = joinNotes(note, n)
	}
	m.add(e, verdict, m.v2Name(e), note)
	switch cat {
	case catConnectionDef:
		m.association(e)
		return
	case catIndividualDef:
		m.w.block(header.String(), func() { m.individualBody(e) })
		return
	case catVerificationDef:
		m.w.block(header.String(), func() { m.verificationBody(e) })
		return
	case catConstraintDef:
		m.w.block(header.String(), func() { m.constraintBody(e) })
		return
	case catRequirementDef:
		m.w.block(header.String(), func() { m.requirementBody(e) })
		return
	case catEnumDef:
		m.w.block(header.String(), func() {
			m.comments(e)
			for _, lit := range e.Owned("ownedLiteral") {
				m.w.line(writeName(m.nameOf(lit)) + ";")
				m.add(lit, Mapped, m.v2Name(lit), "")
			}
			m.stereotypeComments(e)
		})
		return
	}
	m.w.block(header.String(), func() {
		m.body(e)
		m.stereotypeComments(e)
	})
}

// generals writes the specializations of a classifier: its generalizations,
// and for a value type the ScalarValues type it derives from.
func (m *migration) generals(e *xmi.Element, cat category) (string, string) {
	var refs []string
	var notes []string
	for _, g := range e.Owned("generalization") {
		target := m.model.Ref(g, "general")
		if target == nil {
			notes = append(notes, "a generalization refers to nothing in the document")
			continue
		}
		if sv := m.scalarValue(target); sv != "" {
			refs = append(refs, "ScalarValues::"+sv)
			continue
		}
		if target.IsProxy() || m.isLibrary(target) {
			notes = append(notes, "generalization of library type "+target.Name+" is not written")
			continue
		}
		if tc, _ := m.classify(target); tc != cat {
			notes = append(notes, "generalization of "+qualifiedName(target)+" is not written: it becomes a "+tc.keyword()+", not a "+cat.keyword())
			continue
		}
		refs = append(refs, m.ref(target, m.scope))
	}
	if cat == catIndividualDef {
		written, _ := m.instanceClassifiers(e)
		for _, c := range written {
			refs = append(refs, m.ref(c, m.scope))
		}
		if d := m.dangling(e, "classifier"); d != "" {
			notes = append(notes, d)
		}
	}
	return strings.Join(refs, ", "), strings.Join(notes, "; ")
}

// dangling notes the references of e in the given roles that resolve to
// nothing in the document; "" when every reference resolves.
func (m *migration) dangling(e *xmi.Element, roles ...string) string {
	var notes []string
	for _, role := range roles {
		if ids := m.model.Unresolved(e, role); len(ids) > 0 {
			notes = append(notes, fmt.Sprintf("%d %s reference(s) resolve to nothing in the document (%s)", len(ids), role, strings.Join(ids, ", ")))
		}
	}
	return strings.Join(notes, "; ")
}

// downgrade marks e's report entry approximated with a further note.
func (m *migration) downgrade(e *xmi.Element, note string) {
	for i := len(m.report.Entries) - 1; i >= 0; i-- {
		en := &m.report.Entries[i]
		if en.ID != e.ID {
			continue
		}
		if en.Verdict == Mapped {
			en.Verdict = Approximated
		}
		en.Note = joinNotes(en.Note, note)
		return
	}
}

func joinNotes(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}

// requirementID reads the requirement's id tag in the profile's spelling or
// the capitalized one some tools write.
func requirementID(e *xmi.Element) string {
	return requirementTag(e, "Id", "id", "ID")
}

func requirementText(e *xmi.Element) string {
	return requirementTag(e, "Text", "text")
}

// requirementTag reads a tag from the standard requirement stereotypes only;
// a custom stereotype's same-named tag stays in its comment.
func requirementTag(e *xmi.Element, tags ...string) string {
	for _, s := range e.Stereotypes {
		if !isStandard(s) || !isRequirementStereotype(s.Name) {
			continue
		}
		for _, tag := range tags {
			if v := s.Tag(tag); v != "" {
				return v
			}
		}
	}
	return ""
}

func (m *migration) requirementBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	text := requirementText(e)
	if text != "" {
		m.w.lines(prefixFirst("doc ", commentLines(commentText(text))))
	}
	m.writeComments(e, text == "")
	for _, c := range e.Children {
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeComments(e)
	m.scope = saved
}

// constraintBody writes a constraint block: its parameters, then its
// anonymous rule as the result expression.
func (m *migration) constraintBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	var result *xmi.Element
	for _, c := range e.Children {
		if c.Role == "ownedRule" && c.Name == "" && result == nil {
			result = c
			continue
		}
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeComments(e)
	if result != nil {
		spec := m.model.Ref(result, "specification")
		if spec == nil {
			spec = firstOwned(result, "specification")
		}
		if spec == nil {
			m.unmapped(result, "the constraint has no specification")
		} else if expr, ok, note := m.valueExpr(spec, e); ok {
			m.w.line(expr)
			m.add(result, verdictFor(note), m.v2Name(e), note)
		} else {
			m.unmappedExpr(result, spec, note)
		}
	}
	m.scope = saved
}

func verdictFor(note string) Verdict {
	if note != "" {
		return Approximated
	}
	return Mapped
}

func firstOwned(e *xmi.Element, role string) *xmi.Element {
	if o := e.Owned(role); len(o) > 0 {
		return o[0]
	}
	return nil
}

// individualBody writes an instance specification's slots as redefinitions
// of the classifier's features with the slot values.
func (m *migration) individualBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	for _, slot := range e.Owned("slot") {
		f := m.model.Ref(slot, "definingFeature")
		if f == nil || f.IsProxy() {
			m.unmapped(slot, "the slot's defining feature is not in the document")
			continue
		}
		kw, _, _ := m.featureKeyword(f, catPartDef)
		if kw != "attribute" {
			m.unmapped(slot, "only slots of value properties are written; "+f.Name+" is a "+kw)
			continue
		}
		var vals []string
		var notes []string
		ok := true
		for _, v := range slot.Owned("value") {
			expr, vok, note := m.valueExpr(v, e)
			if !vok {
				ok = false
				notes = append(notes, note)
				break
			}
			vals = append(vals, expr)
			if note != "" {
				notes = append(notes, note)
			}
		}
		if !ok {
			m.unmapped(slot, strings.Join(notes, "; "))
			continue
		}
		value := ""
		switch len(vals) {
		case 0:
		case 1:
			value = " = " + vals[0]
		default:
			value = " = (" + strings.Join(vals, ", ") + ")"
		}
		m.w.line("attribute :>> " + writeName(m.nameOf(f)) + value + ";")
		m.add(slot, verdictFor(strings.Join(notes, "; ")), m.v2Name(e)+"::"+writeName(f.Name), strings.Join(notes, "; "))
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeComments(e)
	m.scope = saved
}

// verificationBody writes a test case: the requirements it verifies form its
// objective; its behavior is not migrated.
func (m *migration) verificationBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	if extras := m.extras[e]; len(extras) > 0 {
		m.w.block("objective", func() {
			for _, extra := range extras {
				extra()
			}
		})
	}
	m.stereotypeComments(e)
	m.scope = saved
}

// ownsEveryEnd reports whether no classifier property carries the association:
// every member end is owned by the association itself.
func ownsEveryEnd(e *xmi.Element, ends []*xmi.Element) bool {
	for _, end := range ends {
		if end.Parent != e {
			return false
		}
	}
	return true
}

// association writes an association or association block as a connection def
// with its member ends. An anonymous association with a classifier-owned end
// is already written as that property, so it writes nothing.
func (m *migration) association(e *xmi.Element) {
	ends := m.model.Refs(e, "memberEnd")
	name := m.nameOf(e)
	if name == "" {
		missing := m.dangling(e, "memberEnd")
		if e.Type == "Association" && !ownsEveryEnd(e, ends) {
			m.add(e, verdictFor(missing), "", joinNotes("the anonymous association is written as its member-end properties", missing))
			return
		}
		name = m.nameFor(e)
		m.add(e, Approximated, m.v2Name(e), joinNotes("the anonymous "+e.Type+" owns every end, so it is written as connection def "+name, missing))
	}
	header := "connection def " + writeName(name)
	if gens, _ := m.generals(e, catConnectionDef); gens != "" {
		header += " :> " + gens
	}
	m.w.block(header, func() {
		saved := m.scope
		m.scope = e
		m.comments(e)
		used := map[string]bool{}
		for _, end := range ends {
			t := m.model.Ref(end, "type")
			typ, tnote := m.typeRef(t, e)
			endName := m.nameOf(end)
			if endName == "" && t != nil {
				endName = m.nameFor(end)
			}
			// An end named elsewhere yields to a member of the connection def.
			clash := func(n string) bool { return used[n] || (end.Parent != e && m.nameTaken(e, n)) }
			for base, i := endName, 2; endName != "" && clash(endName); i++ {
				endName = fmt.Sprintf("%s%d", base, i)
			}
			if endName != m.nameOf(end) && m.nameOf(end) != "" {
				m.downgrade(e, "end "+m.nameOf(end)+" is written as "+endName+" so the ends and members stay distinct")
			}
			used[endName] = true
			decl := "end"
			if endName != "" {
				decl += " " + writeName(endName)
			}
			if typ != "" {
				decl += " : " + typ
			}
			mult, mnote := m.multiplicity(end)
			decl += mult + collection(end) + ";"
			tnote = joinNotes(tnote, mnote)
			m.w.line(decl)
			if end.Parent == e {
				m.add(end, verdictFor(tnote), m.v2Name(e)+"::"+writeName(endName), tnote)
			}
		}
		for _, c := range e.Children {
			if c.Role != "ownedEnd" {
				m.member(c)
			}
		}
		for _, extra := range m.extras[e] {
			extra()
		}
		m.stereotypeComments(e)
		m.scope = saved
	})
}

// featureKeyword decides the v2 usage keyword of a v1 property from its type
// and aggregation, given the category of its owner; prefix is `ref ` or empty.
func (m *migration) featureKeyword(p *xmi.Element, owner category) (keyword, prefix, note string) {
	t := m.model.Ref(p, "type")
	if p.Type == "Port" {
		return "port", "", ""
	}
	if owner == catConstraintDef {
		return "attribute", "", ""
	}
	if t == nil {
		return "ref", "", "the untyped property is written as a reference usage"
	}
	if m.scalarValue(t) != "" {
		return "attribute", "", ""
	}
	tc, _ := m.classify(t)
	switch tc {
	case catAttributeDef, catEnumDef:
		return "attribute", "", ""
	case catConstraintDef:
		return "constraint", "", ""
	case catPortDef:
		if owner == catPortDef {
			return "attribute", "", "a flow property typed by an interface block is written as an attribute"
		}
		return "port", "", "a property typed by an interface block is written as a port"
	case catRequirementDef:
		return "requirement", "", ""
	case catNone, catLibrary:
		return "attribute", "", "typed by library element " + t.Name + " with no known v2 counterpart"
	case catUnmapped:
		return "ref", "", "typed by " + qualifiedName(t) + ", which is not migrated"
	}
	if owner == catPortDef {
		return "item", "", ""
	}
	switch p.Attrs["aggregation"] {
	case "composite":
		return "part", "", ""
	case "shared":
		return "part", "ref ", "shared aggregation is written as a reference part"
	}
	return "part", "ref ", ""
}

// feature writes a property or port of the current scope.
func (m *migration) feature(p *xmi.Element) {
	ownerCat, _ := m.classify(m.scope)
	kw, prefix, note := m.featureKeyword(p, ownerCat)
	t := m.model.Ref(p, "type")
	typ, tnote := m.typeRef(t, m.scope)
	note = joinNotes(note, tnote)

	var b strings.Builder
	switch p.Attrs["visibility"] {
	case "private":
		b.WriteString(privatePrefix)
	case "protected":
		b.WriteString("protected ")
	case "package":
		b.WriteString(privatePrefix)
		note = joinNotes(note, "package visibility is written as private")
	}
	// The v2 usage prefix orders direction, derived, abstract, constant, ref.
	switch {
	case p.Type == "Port":
		dir, dnote := portDirection(p)
		b.WriteString(dir)
		note = joinNotes(note, dnote)
	case ownerCat == catConstraintDef:
		b.WriteString("in ")
	case ownerCat == catPortDef:
		if fp := stereo(p, "FlowProperty"); fp != nil {
			switch fp.Tag("direction") {
			case "in":
				b.WriteString("in ")
			case "out":
				b.WriteString("out ")
			case "inout":
				b.WriteString("inout ")
			}
		}
	}
	if p.Attrs["isDerived"] == "true" {
		b.WriteString("derived ")
	}
	if p.Attrs["isAbstract"] == "true" {
		b.WriteString("abstract ")
	}
	if p.Attrs["isReadOnly"] == "true" {
		// A value type's features cannot vary, so `constant` is not allowed there.
		if ownerCat == catAttributeDef {
			note = joinNotes(note, "read-only is not written: the features of an attribute definition cannot vary")
		} else {
			b.WriteString("constant ")
		}
	}
	b.WriteString(prefix)
	b.WriteString(kw)
	name := m.nameOf(p)
	if name == "" && typ == "" {
		// A usage needs a name or a type; an anonymous one with no written type gets a name.
		name = m.nameFor(p)
		note = joinNotes(note, "the anonymous property is named "+name+" as it has no v2 type")
	}
	target := ""
	if name != "" {
		b.WriteString(" " + writeName(name))
		target = m.v2Name(p)
	}

	// A port typed by anything but an interface block carries its type as one
	// directed feature, since a v2 port is typed by a port def alone.
	payload := ""
	if kw == "port" && p.Type == "Port" && typ != "" {
		switch tc, _ := m.classify(t); {
		case m.scalarValue(t) != "", tc == catAttributeDef, tc == catEnumDef:
			payload = "attribute"
		case tc == catPartDef:
			payload = "item"
		}
	}
	if typ != "" && payload == "" {
		if p.Type == "Port" && p.Attrs["isConjugated"] == "true" {
			typ = "~" + typ
		}
		b.WriteString(" : " + typ)
	}
	mult, mnote := m.multiplicity(p)
	b.WriteString(mult + collection(p))
	note = joinNotes(note, mnote)
	for _, role := range []string{"redefinedProperty", "subsettedProperty"} {
		op := " :>> "
		if role == "subsettedProperty" {
			op = " :> "
		}
		for _, r := range m.model.Refs(p, role) {
			if !m.written(r) {
				note = joinNotes(note, role+" "+describe(r)+" is not written: it has no v2 declaration in the document")
				continue
			}
			b.WriteString(op + m.featureRef(r))
		}
	}
	note = joinNotes(note, m.dangling(p, "redefinedProperty", "subsettedProperty"))

	var bodyLines []string
	if dv := firstOwned(p, "defaultValue"); dv != nil {
		expr, ok, vnote := m.valueExpr(dv, m.scope)
		if ok {
			b.WriteString(" default = " + expr)
			note = joinNotes(note, vnote)
		} else {
			bodyLines = append(bodyLines, commentLines("default value not migrated: "+describeValue(dv)+" — "+vnote)...)
			note = joinNotes(note, "default value not migrated: "+vnote)
		}
	}
	if payload != "" {
		dir, _ := portDirection(p)
		bodyLines = append(bodyLines, dir+payload+" "+writeName(m.nameFor(p))+" : "+typ+";")
		note = joinNotes(note, "a port typed by a "+t.Type+" is written as a port holding one directed "+payload)
	}

	header := b.String()
	m.add(p, verdictFor(note), target, note)
	m.w.block(header, func() {
		saved := m.scope
		m.scope = p
		m.comments(p)
		m.w.lines(bodyLines)
		for _, c := range p.Children {
			if c.Role != "defaultValue" {
				m.member(c)
			}
		}
		for _, extra := range m.extras[p] {
			extra()
		}
		m.stereotypeComments(p)
		m.scope = saved
	})
}

// portDirection writes the direction prefix of a flow port.
func portDirection(p *xmi.Element) (string, string) {
	fp := stereo(p, "FlowPort")
	if fp == nil {
		return "", ""
	}
	switch fp.Tag("direction") {
	case "in":
		return "in ", ""
	case "out":
		return "out ", ""
	case "inout":
		return "inout ", ""
	}
	return "", ""
}

// written reports whether e becomes a v2 element that can be referred to.
func (m *migration) written(e *xmi.Element) bool {
	if e == nil || e.IsProxy() {
		return false
	}
	// The root Model is the file itself, so nothing can refer to it.
	if e.Parent == nil && e.Type == "Model" {
		return false
	}
	switch e.Type {
	case "Property", "Port":
		return m.written(e.Parent)
	case "Association":
		return e.Name != ""
	}
	cat, _ := m.classify(e)
	return cat.keyword() != ""
}

// featureRef writes a reference to a property from a feature that redefines or
// subsets it: the simple name when it is inherited into the current scope.
func (m *migration) featureRef(r *xmi.Element) string {
	if r.Parent != nil && m.inherits(m.scope, r.Parent) && r.Name != "" {
		return writeName(m.nameOf(r))
	}
	return m.ref(r, m.scope)
}

// inherits reports whether classifier e specializes general, transitively.
func (m *migration) inherits(e, general *xmi.Element) bool {
	seen := map[*xmi.Element]bool{}
	var walk func(*xmi.Element) bool
	walk = func(c *xmi.Element) bool {
		if c == nil || seen[c] {
			return false
		}
		seen[c] = true
		for _, g := range c.Owned("generalization") {
			t := m.model.Ref(g, "general")
			if t == general || walk(t) {
				return true
			}
		}
		return false
	}
	return walk(e)
}

// typeRef writes the type of a feature: a ScalarValues type, a reference to a
// migrated classifier, or nothing with a note when the type is not migrated.
func (m *migration) typeRef(t, scope *xmi.Element) (string, string) {
	if t == nil {
		return "", ""
	}
	if sv := m.scalarValue(t); sv != "" {
		return "ScalarValues::" + sv, ""
	}
	if t.IsProxy() {
		return "", "type " + t.Name + " lives outside the document and is not written"
	}
	cat, _ := m.classify(t)
	switch cat {
	case catLibrary:
		return "", "library type " + qualifiedName(t) + " has no known v2 counterpart and is not written"
	case catUnmapped, catNone:
		return "", "type " + qualifiedName(t) + " is not migrated and is not written"
	}
	return m.ref(t, scope), ""
}

// multiplicity writes a [lower..upper] multiplicity, or nothing for 1..1. A
// bound that is not a natural number (or * above) is dropped with a note.
func (m *migration) multiplicity(p *xmi.Element) (string, string) {
	lower, upper := "", ""
	if lv := firstOwned(p, "lowerValue"); lv != nil {
		lower = lv.Attrs["value"]
		if lower == "" {
			lower = "0"
		}
	}
	if uv := firstOwned(p, "upperValue"); uv != nil {
		upper = uv.Attrs["value"]
		if upper == "" {
			upper = "0"
		}
	}
	// UML defaults an omitted bound to 1, and a bound's omitted value to 0.
	switch {
	case lower == "" && upper == "":
		return "", ""
	case lower == "":
		lower = "1"
	case upper == "":
		upper = "1"
	}
	if !isNatural(lower) || (!isNatural(upper) && upper != "*") {
		return "", "multiplicity " + lower + ".." + upper + " is not a range of natural numbers and is not written"
	}
	if lower == upper {
		if lower == "1" {
			return "", ""
		}
		return "[" + lower + "]", ""
	}
	return "[" + lower + ".." + upper + "]", ""
}

// collection writes the ordered and nonunique modifiers of a property; UML and
// v2 share the defaults (unordered, unique), so only a departure is written.
func collection(p *xmi.Element) string {
	s := ""
	if p.Attrs["isOrdered"] == "true" {
		s += " ordered"
	}
	if p.Attrs["isUnique"] == "false" {
		s += " nonunique"
	}
	return s
}

// isNatural reports whether s spells a natural number.
func isNatural(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// connector writes a connector: a binding connector as `bind`, another as
// `connect`, and an item flow it realizes as `flow`.
func (m *migration) connector(c *xmi.Element) {
	ends := c.Owned("end")
	if len(ends) != 2 {
		m.unmappedConnector(c, fmt.Sprintf("a connector with %d ends is not migrated", len(ends)))
		return
	}
	paths := make([]string, 2)
	var notes []string
	for i, end := range ends {
		path, note := m.endPath(end)
		if path == "" {
			m.unmappedConnector(c, note)
			return
		}
		paths[i] = path
		if note != "" {
			notes = append(notes, note)
		}
	}
	note := strings.Join(notes, "; ")
	decl, kw := "connect "+paths[0]+" to "+paths[1]+";", "connection "
	if has(c, "BindingConnector") {
		decl, kw = "bind "+paths[0]+" = "+paths[1]+";", "binding "
	}
	target := ""
	if m.nameOf(c) != "" {
		decl = kw + writeName(m.nameOf(c)) + " " + decl
		target = m.v2Name(c)
	}
	m.w.line(decl)
	m.add(c, verdictFor(note), target, note)
	m.stereotypeComments(c)
	for _, f := range m.flows[c] {
		m.itemFlow(f, ends, paths)
	}
}

// unmappedConnector records a connector with no v2 form and settles the item
// flows it realizes.
func (m *migration) unmappedConnector(c *xmi.Element, note string) {
	m.unmapped(c, note)
	for _, f := range m.flows[c] {
		m.flowDone(f, nil, []string{"realizing connector " + describe(c) + " is not migrated: " + note})
	}
}

// endPath writes the feature path a connector end names: the nested property
// path, then the part with port, then the role.
func (m *migration) endPath(end *xmi.Element) (string, string) {
	role := m.model.Ref(end, "role")
	if role == nil {
		return "", "a connector end names no role in the document"
	}
	var segs []*xmi.Element
	if nce := stereo(end, "NestedConnectorEnd"); nce != nil {
		for _, id := range nce.Tags["propertyPath"] {
			p := m.model.Lookup(id)
			if p == nil {
				return "", "the nested connector end's property path names " + id + ", which is not in the document"
			}
			segs = append(segs, p)
		}
	} else if pwp := m.model.Ref(end, "partWithPort"); pwp != nil {
		segs = append(segs, pwp)
	}
	segs = append(segs, role)
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = writeName(m.nameFor(s))
	}
	if len(segs) == 1 && role.Parent != m.scope && !m.inherits(m.scope, role.Parent) {
		return "", "the connector end's role " + qualifiedName(role) + " is not a feature of " + qualifiedName(m.scope)
	}
	return strings.Join(parts, "."), ""
}

// itemFlow writes an item flow realized by a connector as a flow between the
// flow properties its ends carry for each conveyed classifier, from the end
// whose role is the flow's source to the end whose role is its target.
func (m *migration) itemFlow(f *xmi.Element, ends []*xmi.Element, paths []string) {
	conveyed := m.model.Refs(f, "conveyed")
	missing := m.dangling(f, "conveyed")
	if len(conveyed) == 0 {
		m.flowDone(f, nil, []string{joinNotes("the item flow conveys nothing", missing)})
		return
	}
	src := m.model.Ref(f, "informationSource")
	dst := m.model.Ref(f, "informationTarget")
	if src == nil || dst == nil {
		m.flowDone(f, nil, []string{"the item flow's source or target is not in the document"})
		return
	}
	from, to := paths[0], paths[1]
	switch {
	case m.model.Ref(ends[0], "role") == src && m.model.Ref(ends[1], "role") == dst:
	case m.model.Ref(ends[1], "role") == src && m.model.Ref(ends[0], "role") == dst:
		from, to = paths[1], paths[0]
	default:
		m.flowDone(f, nil, []string{"the item flow's source and target are not the roles of the ends of realizing connector " + describe(ends[0].Parent)})
		return
	}
	var written, notes []string
	if missing != "" {
		notes = append(notes, missing)
	}
	for _, item := range conveyed {
		sp := m.flowProperty(src, item)
		dp := m.flowProperty(dst, item)
		if sp == nil || dp == nil {
			if typ, tnote := m.typeRef(item, m.scope); typ == "" {
				notes = append(notes, joinNotes("the conveyed classifier "+item.Name+" is not written", tnote))
			} else {
				notes = append(notes, "no flow property typed by "+item.Name+" on both ends of the realizing connector")
			}
			m.w.lines(commentLines("item flow of " + item.Name + " from " + from + " to " + to + " not migrated: " + notes[len(notes)-1]))
			continue
		}
		m.w.line("flow " + from + "." + writeName(m.nameOf(sp)) + " to " + to + "." + writeName(m.nameOf(dp)) + ";")
		written = append(written, item.Name)
	}
	m.flowDone(f, written, notes)
}

// flowDone records one realizing connector's result for f and reports the flow
// once the last of them is in.
func (m *migration) flowDone(f *xmi.Element, written, notes []string) {
	o := m.outcomes[f]
	o.written = append(o.written, written...)
	o.notes = append(o.notes, notes...)
	o.pending--
	if o.pending == 0 {
		m.reportFlow(f, o)
	}
}

// reportFlow adds f's single report entry from what its connectors did.
func (m *migration) reportFlow(f *xmi.Element, o *flowOutcome) {
	delete(m.outcomes, f)
	note := strings.Join(o.notes, "; ")
	switch {
	case len(o.written) == 0:
		m.unmapped(f, note)
	case note != "":
		m.add(f, Approximated, "", "only the flow of "+strings.Join(o.written, ", ")+" is written; "+note)
	default:
		m.add(f, Mapped, "", "")
	}
}

// flushFlows reports the item flows still waiting on a realizing connector
// that was never written because its owner is not migrated.
func (m *migration) flushFlows() {
	var rest []*xmi.Element
	for f := range m.outcomes {
		rest = append(rest, f)
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].ID < rest[j].ID })
	for _, f := range rest {
		o := m.outcomes[f]
		o.notes = append(o.notes, fmt.Sprintf("%d realizing connector(s) are owned by elements that are not migrated", o.pending))
		m.reportFlow(f, o)
	}
}

// flowProperty finds the flow property of a port's type carrying item.
func (m *migration) flowProperty(port, item *xmi.Element) *xmi.Element {
	t := m.model.Ref(port, "type")
	if t == nil {
		return nil
	}
	for _, p := range t.Owned("ownedAttribute") {
		if has(p, "FlowProperty") && m.model.Ref(p, "type") == item && p.Name != "" {
			return p
		}
	}
	return nil
}

// informationFlow writes an item flow with no realizing connector as a flow
// between its source and target when both are features of the scope.
func (m *migration) informationFlow(f *xmi.Element) {
	if len(m.model.Refs(f, "realizingConnector")) > 0 {
		return
	}
	m.unmapped(f, "an information flow with no realizing connector is not migrated")
}

// rule writes a constraint owned by a classifier as a constraint usage.
func (m *migration) rule(r *xmi.Element) {
	spec := firstOwned(r, "specification")
	if spec == nil {
		m.unmapped(r, "the constraint has no specification")
		return
	}
	expr, ok, note := m.valueExpr(spec, m.scope)
	if !ok {
		m.unmappedExpr(r, spec, note)
		return
	}
	decl := "constraint"
	if m.nameOf(r) != "" {
		decl += " " + writeName(m.nameOf(r))
	}
	m.w.line(decl + " { " + expr + " }")
	m.add(r, verdictFor(note), m.v2Name(r), note)
}

// pair is one client–supplier pair of a dependency; a dependency with several
// clients or suppliers stands for every pair.
type pair struct{ client, supplier *xmi.Element }

// dependencyPairs expands a dependency into the client–supplier pairs that can
// be written and the number that cannot; the note says why, for those.
func (m *migration) dependencyPairs(d *xmi.Element) (pairs []pair, failed int, note string) {
	clients := m.model.Refs(d, "client")
	suppliers := m.model.Refs(d, "supplier")
	missing := m.dangling(d, "client", "supplier")
	if len(clients) == 0 || len(suppliers) == 0 {
		return nil, 1, joinNotes("the dependency's client or supplier is not in the document", missing)
	}
	external := 0
	for _, c := range clients {
		for _, s := range suppliers {
			if c.IsProxy() || s.IsProxy() {
				external++
				continue
			}
			pairs = append(pairs, pair{c, s})
		}
	}
	if external > 0 {
		missing = joinNotes(missing, fmt.Sprintf("%d pair(s) reach outside the document and are not written", external))
	}
	// Every pair with a dangling end fails too.
	total := (len(clients) + len(m.model.Unresolved(d, "client"))) * (len(suppliers) + len(m.model.Unresolved(d, "supplier")))
	return pairs, total - len(pairs), missing
}

// placement is the outcome of placing a Satisfy or Verify: where each pair was
// written, and why the others could not be.
type placement struct {
	written, failed int
	target          string
	notes           []string
}

// placeDependency registers, ahead of writing, a Satisfy or Verify in the body
// of the element each pair is written in, which may precede the dependency
// itself; the dependency's report entry is written where it stands.
func (m *migration) placeDependency(d *xmi.Element) {
	if !has(d, "Satisfy", "Verify") {
		return
	}
	pl := &placement{}
	m.unplaced[d] = pl
	pairs, failed, note := m.dependencyPairs(d)
	pl.failed += failed
	if note != "" {
		pl.notes = append(pl.notes, note)
	}
	name := m.nameOf(d)
	for _, p := range pairs {
		var target, note string
		var ok bool
		if has(d, "Satisfy") {
			target, note, ok = m.satisfy(p.client, p.supplier, name)
		} else {
			target, note, ok = m.verify(p.client, p.supplier, name)
		}
		if ok {
			pl.written++
			pl.target = target
		} else {
			pl.failed++
		}
		if note != "" {
			pl.notes = append(pl.notes, note)
		}
	}
}

// placedName reserves name in scope's body for a Satisfy or Verify written
// there, distinct from the members and from any earlier pair of the same name;
// the note says when the name had to change.
func (m *migration) placedName(scope *xmi.Element, name string) (string, string) {
	if name == "" {
		return "", ""
	}
	fresh := m.freshName(scope, name)
	if fresh != name {
		return fresh, "the pair in " + m.v2Name(scope) + " is named " + fresh + " so it stays distinct"
	}
	return fresh, ""
}

// dependency writes a dependency by the SysML stereotype it carries, one
// relationship per client–supplier pair.
func (m *migration) dependency(d *xmi.Element) {
	if has(d, "Satisfy", "Verify") {
		m.relationship(d, m.unplaced[d])
		return
	}
	pairs, failed, note := m.dependencyPairs(d)
	if len(pairs) == 0 {
		m.unmapped(d, note)
		return
	}
	pl := &placement{failed: failed}
	if note != "" {
		pl.notes = append(pl.notes, note)
	}
	for i, p := range pairs {
		name := m.nameOf(d)
		if name != "" && i > 0 {
			name = m.freshName(m.scope, name)
			pl.notes = append(pl.notes, fmt.Sprintf("pair %d is named %s so the pairs stay distinct", i+1, name))
		}
		target, written, note := m.dependencyPair(d, name, p.client, p.supplier)
		if written {
			pl.written++
			pl.target = target
		} else {
			pl.failed++
		}
		if note != "" {
			pl.notes = append(pl.notes, note)
		}
	}
	m.relationship(d, pl)
	if pl.written > 0 {
		m.stereotypeComments(d)
	}
}

// freshName returns name, or name with a numeric suffix, not yet taken in owner,
// and reserves it.
func (m *migration) freshName(owner *xmi.Element, name string) string {
	base := name
	for i := 2; m.nameTaken(owner, name); i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	m.take(owner, name)
	return name
}

// relationship appends the one report entry of a relationship: mapped when every
// pair was written, approximated when some were, unmapped when none.
func (m *migration) relationship(d *xmi.Element, pl *placement) {
	note := strings.Join(uniqueStrings(pl.notes), "; ")
	target := ""
	if pl.written == 1 {
		target = pl.target
	}
	switch {
	case pl.written == 0:
		m.unmapped(d, note)
	case pl.failed > 0:
		m.add(d, Approximated, target, fmt.Sprintf("%d of %d relationships written; %s", pl.written, pl.written+pl.failed, note))
	case pl.written > 1:
		m.add(d, Approximated, "", fmt.Sprintf("written as %d relationships, one per client–supplier pair", pl.written))
	default:
		m.add(d, verdictFor(note), target, note)
	}
}

func uniqueStrings(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// dependencyPair writes one client–supplier pair of a dependency, returning
// the v2 target written, if any, whether it was written, and a note.
func (m *migration) dependencyPair(d *xmi.Element, name string, client, supplier *xmi.Element) (string, bool, string) {
	if has(d, "DeriveReqt") {
		target, note := m.derive(d, name, client, supplier)
		return target, target != "", note
	}
	for _, end := range []*xmi.Element{client, supplier} {
		if !m.written(end) {
			return "", false, "its end " + qualifiedName(end) + " is not migrated"
		}
	}
	from, to := m.ref(client, m.scope), m.ref(supplier, m.scope)
	target := ""
	if name != "" {
		target = m.qualified(append(m.segments(m.scope), name))
	}
	decl := "dependency "
	if name != "" {
		decl += writeName(name) + " from "
	}
	decl += from + " to " + to
	switch {
	case has(d, "Refine"):
		m.w.block(decl, func() {
			m.w.line("@ModelingMetadata::Refinement;")
		})
		return target, true, ""
	case has(d, "Allocate"):
		alloc := "allocate " + from + " to " + to + ";"
		if name != "" {
			alloc = "allocation " + writeName(name) + " " + alloc
		}
		m.w.line(alloc)
		return target, true, ""
	case has(d, "Trace"):
		m.w.line(decl + "; /* «Trace» */")
		return target, true, "a trace is written as a plain dependency"
	case has(d, "Copy"):
		m.w.line(decl + "; /* «Copy» */")
		return target, true, "a copy is written as a plain dependency; the text is not kept in step"
	}
	m.w.line(decl + ";")
	var names []string
	for _, s := range d.Stereotypes {
		names = append(names, "«"+s.Name+"»")
	}
	if len(names) > 0 {
		return target, true, strings.Join(names, " ") + " is written as a plain dependency"
	}
	return target, true, ""
}

// satisfy places `satisfy requirement` in the body of the satisfying block, or
// of the block owning the satisfying property, returning the v2 name written,
// a note, and whether it was written.
func (m *migration) satisfy(client, req *xmi.Element, name string) (string, string, bool) {
	if rc, _ := m.classify(req); rc != catRequirementDef {
		return "", "the supplier " + qualifiedName(req) + " is not a requirement", false
	}
	scope, by := m.usageContext(client)
	if scope == nil {
		return "", "a satisfy whose client is a " + kindOf(client) + " has no v2 form", false
	}
	name, note := m.placedName(scope, name)
	m.extras[scope] = append(m.extras[scope], func() {
		decl := "satisfy requirement "
		if name != "" {
			decl += writeName(name) + " "
		}
		decl += ": " + m.ref(req, scope)
		if by != "" {
			decl += " by " + by
		}
		m.w.line(decl + ";")
	})
	return m.placedTarget(scope, name), note, true
}

// placedTarget is the report target of a Satisfy or Verify written in scope:
// the usage itself when named, else the body it was written in.
func (m *migration) placedTarget(scope *xmi.Element, name string) string {
	if name == "" {
		return m.v2Name(scope)
	}
	return m.qualified(append(m.segments(scope), name))
}

// usageContext finds the body a client's satisfy is written in: the
// classifier itself, or the classifier owning a property, satisfied `by` it.
func (m *migration) usageContext(client *xmi.Element) (*xmi.Element, string) {
	if client.Type == "Property" || client.Type == "Port" {
		if client.Parent == nil {
			return nil, ""
		}
		if cat, _ := m.classify(client.Parent); cat == catPartDef || cat == catPortDef || cat == catConnectionDef {
			return client.Parent, writeName(m.nameFor(client))
		}
		return nil, ""
	}
	switch cat, _ := m.classify(client); cat {
	case catPartDef, catPortDef, catConnectionDef, catIndividualDef, catConstraintDef:
		return client, ""
	}
	return nil, ""
}

// verify places `verify requirement` in the objective of the test case.
func (m *migration) verify(client, req *xmi.Element, name string) (string, string, bool) {
	if rc, _ := m.classify(req); rc != catRequirementDef {
		return "", "the supplier " + qualifiedName(req) + " is not a requirement", false
	}
	if cc, _ := m.classify(client); cc != catVerificationDef {
		return "", "a verify whose client is not a test case has no v2 form", false
	}
	name, note := m.placedName(client, name)
	m.extras[client] = append(m.extras[client], func() {
		decl := "verify requirement "
		if name != "" {
			decl += writeName(name) + " "
		}
		m.w.line(decl + ": " + m.ref(req, client) + ";")
	})
	return m.placedTarget(client, name), note, true
}

// derive writes a requirement derivation as a connection def specializing the
// library's Derivation, with the original and derived requirements as ends.
func (m *migration) derive(d *xmi.Element, name string, derived, original *xmi.Element) (string, string) {
	dc, _ := m.classify(derived)
	oc, _ := m.classify(original)
	if dc != catRequirementDef || oc != catRequirementDef {
		return "", "both ends of a derive must be requirements"
	}
	if name == "" {
		name = m.freshName(m.scope, "Derive "+derived.Name)
	} else {
		m.take(m.scope, name)
	}
	if _, ok := m.names[d]; !ok && d.Name == "" {
		m.names[d] = name
	}
	m.w.block("connection def "+writeName(name)+" :> RequirementDerivation::Derivation", func() {
		m.w.line("end #RequirementDerivation::original originalRequirement : " + m.ref(original, d) + ";")
		m.w.line("end #RequirementDerivation::derive derivedRequirement : " + m.ref(derived, d) + ";")
	})
	segs := append(m.segments(m.scope), name)
	if m.scope == nil {
		segs = []string{name}
	}
	return m.qualified(segs), ""
}

// comments writes the comments documenting e: the first as doc, the rest as
// comments, and a comment annotating other elements as `comment about`.
func (m *migration) comments(e *xmi.Element) { m.writeComments(e, true) }

// writeComments writes e's comments; the first becomes doc only when e has no
// doc yet.
func (m *migration) writeComments(e *xmi.Element, first bool) {
	for _, c := range e.Owned("ownedComment") {
		about := m.model.Refs(c, "annotatedElement")
		missing := m.dangling(c, "annotatedElement")
		others := false
		for _, a := range about {
			if a != e {
				others = true
			}
		}
		text := commentBody(c)
		if text == "" {
			m.add(c, Skipped, "", "empty comment")
			continue
		}
		if others {
			refs := make([]string, 0, len(about))
			var omitted []string
			for _, a := range about {
				if !m.written(a) {
					omitted = append(omitted, describe(a))
					continue
				}
				refs = append(refs, m.ref(a, m.scope))
			}
			if len(refs) == 0 {
				m.w.lines(prefixFirst(commentPrefix, commentLines(text)))
			} else {
				m.w.lines(prefixFirst("comment about "+strings.Join(refs, ", ")+" ", commentLines(text)))
			}
			note := missing
			if len(omitted) > 0 {
				note = joinNotes(note, "the comment also annotates "+strings.Join(omitted, ", ")+", which has no v2 declaration in the document and is not written as a subject")
			}
			m.add(c, verdictFor(note), "", note)
			continue
		}
		if first {
			m.w.lines(prefixFirst("doc ", commentLines(text)))
			first = false
		} else {
			m.w.lines(prefixFirst(commentPrefix, commentLines(text)))
		}
		m.add(c, verdictFor(missing), "", missing)
	}
}

// commentBody reads a comment's text, from its body attribute or child element.
func commentBody(c *xmi.Element) string {
	if text := commentText(c.Attrs["body"]); text != "" {
		return text
	}
	if o := firstOwned(c, "body"); o != nil {
		return commentText(strings.TrimSpace(o.Text))
	}
	return ""
}

// comment writes a comment found outside the ownedComment role.
func (m *migration) comment(c *xmi.Element) {
	text := commentBody(c)
	if text == "" {
		m.add(c, Skipped, "", "empty comment")
		return
	}
	m.w.lines(prefixFirst(commentPrefix, commentLines(text)))
	m.add(c, Mapped, "", "")
}

func prefixFirst(prefix string, lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	out[0] = prefix + out[0]
	return out
}

// classifyingStereotypes are the SysML profile stereotypes the mapping
// consumes; any other applied stereotype is kept as a comment.
var classifyingStereotypes = map[string]bool{
	"Block": true, "InterfaceBlock": true, "ConstraintBlock": true, "ValueType": true, "Unit": true,
	"QuantityKind": true, "Requirement": true, "AbstractRequirement": true, "Satisfy": true, "Verify": true,
	"DeriveReqt": true, "Refine": true, "Trace": true, "Copy": true, "Allocate": true, "TestCase": true,
	"FlowPort": true, "FullPort": true, "ProxyPort": true, "FlowProperty": true, "BindingConnector": true,
	"NestedConnectorEnd": true, "ItemFlow": true, "Stakeholder": true, "View": true, "Viewpoint": true,
}

// consumedTags are the tags of the classifying stereotypes the mapping reads;
// any other tag of theirs has no v2 form.
var requirementTags = map[string]bool{"Id": true, "id": true, "ID": true, "Text": true, "text": true}

var consumedTags = map[string]map[string]bool{
	"FlowProperty":       {"direction": true},
	"FlowPort":           {"direction": true},
	"NestedConnectorEnd": {"propertyPath": true},
}

// stereotypeComments keeps the stereotypes the mapping does not consume, and
// the tags it does not read of those it does, as a comment in the element's
// body; an unread tag makes the element's migration an approximation.
func (m *migration) stereotypeComments(e *xmi.Element) {
	for _, s := range e.Stereotypes {
		classifying := isStandard(s) && classifyingStereotypes[s.Name]
		consumed := consumedTags[s.Name]
		if isStandard(s) && isRequirementStereotype(s.Name) {
			if !classifying {
				m.w.line("/* «" + s.Name + "» */")
			}
			classifying, consumed = true, requirementTags
		}
		var tags []string
		for k, vs := range s.Tags {
			if classifying && consumed[k] {
				continue
			}
			tags = append(tags, k+" = "+strings.Join(m.tagValues(vs), ", "))
		}
		sort.Strings(tags)
		if classifying {
			if len(tags) == 0 {
				continue
			}
			m.w.lines(commentLines("«" + s.Name + "» tags with no v2 form: " + strings.Join(tags, "; ")))
			m.downgrade(e, "«"+s.Name+"» "+strings.Join(tags, "; ")+" has no v2 form")
			continue
		}
		text := "applied stereotype «" + s.Name + "»"
		if len(tags) > 0 {
			text += ": " + strings.Join(tags, "; ")
		}
		m.w.lines(commentLines(text))
	}
}

// tagValues writes tag values, an element reference by the element's name.
func (m *migration) tagValues(vs []string) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		if t := m.model.Lookup(v); t != nil && t.Name != "" {
			v = qualifiedName(t)
		}
		out[i] = v
	}
	return out
}

func isRequirementStereotype(name string) bool {
	for _, r := range requirementStereotypes {
		if r == name {
			return true
		}
	}
	return false
}

// unmapped records an element with no v2 form and keeps a trace of it as a
// comment where it would have been written.
func (m *migration) unmapped(e *xmi.Element, note string) {
	note = joinNotes(note, m.stereotypeSummary(e))
	m.w.lines(commentLines("not migrated: " + kindOf(e) + " " + describe(e) + " — " + note))
	m.add(e, Unmapped, "", note)
}

// stereotypeSummary lists every stereotype applied to e with its tags, so an
// element left behind keeps its metadata; "" when none is applied.
func (m *migration) stereotypeSummary(e *xmi.Element) string {
	var parts []string
	for _, s := range e.Stereotypes {
		var tags []string
		for k, vs := range s.Tags {
			tags = append(tags, k+" = "+strings.Join(m.tagValues(vs), ", "))
		}
		sort.Strings(tags)
		part := "«" + s.Name + "»"
		if len(tags) > 0 {
			part += " (" + strings.Join(tags, "; ") + ")"
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return ""
	}
	return "applied stereotypes " + strings.Join(parts, ", ")
}

// unmappedExpr records a constraint whose expression has no v2 form, keeping
// its text.
func (m *migration) unmappedExpr(r, spec *xmi.Element, note string) {
	m.w.lines(commentLines("not migrated: " + kindOf(r) + " " + describe(r) + " " + describeValue(spec) + " — " + note))
	m.add(r, Unmapped, "", note)
}

func describe(e *xmi.Element) string {
	if e.Name != "" {
		return "'" + e.Name + "'"
	}
	return "(" + e.ID + ")"
}

// describeValue shows a value specification's text for a comment.
func describeValue(v *xmi.Element) string {
	if v.Type == "OpaqueExpression" {
		body, lang := opaqueBody(v)
		if lang != "" {
			return "{" + lang + "} " + body
		}
		return body
	}
	if val, ok := v.Attrs["value"]; ok {
		return val
	}
	return "<" + v.Type + ">"
}
