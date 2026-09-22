// Package migrate writes a SysML v1 model, read from XMI, as SysML v2 textual
// notation, and reports what each v1 element became.
package migrate

import (
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// The v2 keywords the writer prefixes a declaration or annotation with.
const (
	privatePrefix = "private "
	commentPrefix = "comment "
)

// The subjects the report's notes open with.
const (
	classifierSubject = "the instance's classifier "
	individualSubject = "the individual "
	slotValueSubject  = "the slot's value "
)

// scalarValuesPrefix qualifies a name from the standard ScalarValues package.
const scalarValuesPrefix = "ScalarValues::"

// Result is a migration's output: the v2 notation, the report over it, and
// the result snapshots of its run configurations.
type Result struct {
	Notation []byte
	Report   *Report
	Results  *simresults.Results
}

// Migrate reads a SysML v1 model as UML XMI, or a zip archive (such as a
// .mdzip) holding it, and writes it as SysML v2 notation. name labels the
// source in the report.
func Migrate(name string, data []byte) (*Result, error) {
	model, err := sysmlv1.Parse(data)
	if err != nil {
		return nil, err
	}
	return FromModel(name, model), nil
}

// FromModel migrates an already-read XMI model.
func FromModel(name string, model *sysmlv1.Model) *Result {
	m := &migration{
		model:        model,
		report:       &Report{Source: name, Exporter: model.Exporter},
		results:      &simresults.Results{Source: name, Configurations: []simresults.ConfigurationResults{}},
		w:            &writer{},
		names:        map[*sysmlv1.Element]string{},
		nodeNames:    map[*sysmlv1.Element]string{},
		placeholders: map[*sysmlv1.Element]bool{},
		nodeEnds:     map[*sysmlv1.Element]*placement{},
		extras:       map[*sysmlv1.Element][]func(){},
		flows:        map[*sysmlv1.Element][]*sysmlv1.Element{},
		outcomes:     map[*sysmlv1.Element]*flowOutcome{},
		unplaced:     map[*sysmlv1.Element]*placement{},
		taken:        map[*sysmlv1.Element]map[string]bool{},
		parallel:     map[*sysmlv1.Element]string{},
		exposed:      map[*sysmlv1.Element]string{},
		methodOf:     map[*sysmlv1.Element]*sysmlv1.Element{},
		realizes:     map[*sysmlv1.Element]*sysmlv1.Element{},
		opUsage:      map[*sysmlv1.Element]string{},
		deciding:     map[*sysmlv1.Element]bool{},
		bounded:      map[*sysmlv1.Element][]*sysmlv1.Element{},
		allocated:    map[*sysmlv1.Element][]*sysmlv1.Element{},
		triggered:    map[*sysmlv1.Element]bool{},
		snapshots:    map[*sysmlv1.Element]snapshotTyping{},
		contexts:     map[*sysmlv1.Element]*behaviorContext{},
		contextNotes: map[*sysmlv1.Element]string{},
		visiting:     map[*sysmlv1.Element]*contextVisit{},
		invokers:     map[*sysmlv1.Element][]*sysmlv1.Element{},
		unvalued:     map[*sysmlv1.Element]bool{},
		dryOut:       map[*sysmlv1.Element]map[*sysmlv1.Element]bool{},
		admitsNone:   map[*sysmlv1.Element]string{},
		rules:        map[*sysmlv1.Element]ruleForm{},
		carrierOf:    map[*sysmlv1.Element]*carrier{},
		carrierNotes: map[*sysmlv1.Element]string{},
		indexed:      map[string]int{},
		userProfiles: map[*sysmlv1.Element]bool{},
		defsWritten:  map[*sysmlv1.Element]bool{},
		pending:      map[*sysmlv1.Element]*pendingNotes{},
		regionUsed:   map[*sysmlv1.Element]map[string]bool{},
		vertexNames:  map[*sysmlv1.Element]string{},
		points:       map[*sysmlv1.Element]pointForm{},
		incoming:     map[*sysmlv1.Element][]*sysmlv1.Element{},
		outgoing:     map[*sysmlv1.Element][]*sysmlv1.Element{},
		instant:      map[*sysmlv1.Element]map[*sysmlv1.Element]instantValue{},
		self:         "this",
		lanes:        map[*sysmlv1.Element]*lanes{},
		routes:       map[[2]*sysmlv1.Element]partRoute{},
		usageOf:      map[*sysmlv1.Element]string{},
		pins:         map[*sysmlv1.Element]pinDecl{},
		opaque:       map[*sysmlv1.Element]*opaqueResult{},
		monteCarlo:   map[*sysmlv1.Element]*monteCarloCase{},
	}
	m.prepare()
	for _, root := range model.Roots {
		m.root(root)
	}
	m.flushFlows()
	m.placeholderEnds()
	m.unwrittenEvents()
	m.extensions()
	return &Result{Notation: []byte(m.w.String()), Report: m.report, Results: m.results}
}

// unwrittenEvents reports the events whose triggers were never written: those
// belong to behaviors that were not, or to initial transitions, which take none.
func (m *migration) unwrittenEvents() {
	var left []*sysmlv1.Element
	for ev := range m.triggered {
		if !m.reported(ev) && !m.isLibrary(ev) {
			left = append(left, ev)
		}
	}
	sort.Slice(left, func(i, j int) bool { return left[i].ID < left[j].ID })
	for _, ev := range left {
		m.add(ev, Unmapped, "", "every trigger referring to the event is dropped: it belongs to a behavior that is not written, or to an initial transition")
	}
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
	model  *sysmlv1.Model
	report *Report
	// results index the run configurations' result snapshots.
	results *simresults.Results
	w       *writer
	// names holds the names synthesized for anonymous elements; nodeNames the names
	// activity nodes are written under, fixed by their writer or ahead of it.
	names     map[*sysmlv1.Element]string
	nodeNames map[*sysmlv1.Element]string
	// placeholders are the activity nodes written as inert placeholders, and nodeEnds
	// the placements of the relationships ending at activity nodes, judged once all are written.
	placeholders map[*sysmlv1.Element]bool
	nodeEnds     map[*sysmlv1.Element]*placement
	// extras are members other elements contribute to a body: a Satisfy is
	// written inside the block that satisfies.
	extras map[*sysmlv1.Element][]func()
	// flows lists the item flows each connector realizes.
	flows map[*sysmlv1.Element][]*sysmlv1.Element
	// outcomes accumulates each item flow's result over its realizing connectors.
	outcomes map[*sysmlv1.Element]*flowOutcome
	// unplaced records where each Satisfy or Verify was placed, and why not.
	unplaced map[*sysmlv1.Element]*placement
	// taken holds synthesized names reserved in a body, by owner.
	taken map[*sysmlv1.Element]map[string]bool
	// parallel names the parallel state each region of an orthogonal state is
	// written in; a lone region is written inline and has no name of its own.
	parallel map[*sysmlv1.Element]string
	// exposed notes, for each feature reached from outside its owner (through
	// a connector path, a slot or a redefinition), what reaches it.
	exposed map[*sysmlv1.Element]string
	// scope is the element whose body is being written; nil at the top level.
	scope *sysmlv1.Element
	// methodOf maps each behavior that is the method of an operation to it.
	methodOf map[*sysmlv1.Element]*sysmlv1.Element
	// realizes maps a method's parameter to the operation's it stands for.
	realizes map[*sysmlv1.Element]*sysmlv1.Element
	// opUsage names, for each operation, the action usage of its owner that performs it.
	opUsage map[*sysmlv1.Element]string
	// deciding holds each opaque behavior whose body is being checked for names
	// it can see, which is written whichever declaration the check picks.
	deciding map[*sysmlv1.Element]bool
	// bounded lists the duration constraints constraining each element.
	bounded map[*sysmlv1.Element][]*sysmlv1.Element
	// allocated lists the suppliers of the «Allocate» dependencies each element is client of.
	allocated map[*sysmlv1.Element][]*sysmlv1.Element
	// triggered holds each event some trigger refers to, which is reported where it is.
	triggered map[*sysmlv1.Element]bool
	// contexts holds, once asked, the context each activity acts on through a
	// parameter; contextNotes says why an activity naming ports of several gets none.
	contexts     map[*sysmlv1.Element]*behaviorContext
	contextNotes map[*sysmlv1.Element]string
	// visiting is the search settling contexts: each activity it has reached and
	// not settled, and the order it reached them in.
	visiting map[*sysmlv1.Element]*contextVisit
	visits   []*sysmlv1.Element
	// invokers lists, for each behavior, the actions, states, transitions and
	// classifiers that run it without owning it, whose object it then acts on.
	invokers map[*sysmlv1.Element][]*sysmlv1.Element
	// connectors lists the user model's connectors, ports its ports, and portSends its
	// send signal actions going out through a port. arrived indexes, from these, the
	// ports each signal arrives at, once a trigger asks.
	connectors []*sysmlv1.Element
	ports      []*sysmlv1.Element
	portSends  []*sysmlv1.Element
	arrived    *arrivals
	// snapshots types each classifier-less instance under a run configuration's
	// result location by what its slots prove it a snapshot of.
	snapshots map[*sysmlv1.Element]snapshotTyping
	// instanceNames indexes the document's classifiers by the default name of
	// their instances, for namesakes; built on first use.
	instanceNames map[string][]*sysmlv1.Element
	// bound gives, while a transition's effect is written, the expression over
	// the accepted signal each of its parameters is bound to.
	bound map[*sysmlv1.Element]string
	// boundNote says what the expressions in bound are, for the report.
	boundNote string
	// keeping is the statement the effect being written ends with, which keeps
	// the accepted signal for the state the transition enters.
	keeping string
	// carrierOf gives each state the signal its entry and do parameters take
	// their values from; carrierNotes says why a state has none.
	carrierOf    map[*sysmlv1.Element]*carrier
	carrierNotes map[*sysmlv1.Element]string
	// unvalued holds the in parameters nothing passes a value to, so a flow
	// out of one is kept as a comment instead of binding an absent value.
	unvalued map[*sysmlv1.Element]bool
	// dryOut holds, per activity, the out parameters no value reaches; see dryOutputs.
	dryOut map[*sysmlv1.Element]map[*sysmlv1.Element]bool
	// admitsNone says, for each parameter and pin declared admitting no value, why
	// a value may fail to reach it while v1 runs the action; see admitAbsent.
	admitsNone map[*sysmlv1.Element]string
	// indexed locates each element's report entry by id, so an element that
	// several writers account for is reported once.
	indexed map[string]int
	// pending holds the notes on elements annotated before their report entry exists.
	pending map[*sysmlv1.Element]*pendingNotes
	// userProfiles memoizes which profiles are a user's own; see userProfile.
	userProfiles map[*sysmlv1.Element]bool
	// namespacesOf lists the XML namespaces each profile's stereotypes are applied under.
	namespacesOf map[*sysmlv1.Element][]string
	// defsWritten marks the user stereotypes whose metadata def is written so far.
	defsWritten map[*sysmlv1.Element]bool
	// lanes indexes each activity's partitions by the nodes and edges they hold.
	lanes map[*sysmlv1.Element]*lanes
	// routes memoizes, per classifier and target, the chains of composite parts between them.
	routes map[[2]*sysmlv1.Element]partRoute
	// usageOf names, for each activity a lane's object performs, the action
	// usage of the activity's owner that performs it.
	usageOf map[*sysmlv1.Element]string
	// pins records how each declared pin is written, for the bodies that name it.
	pins map[*sysmlv1.Element]pinDecl
	// opaque memoizes what each opaque action's body translates to.
	opaque map[*sysmlv1.Element]*opaqueResult
	// monteCarlo memoizes the analysis def written beside each block; nil for one without.
	monteCarlo map[*sysmlv1.Element]*monteCarloCase
	// rules memoizes how each constraint block's anonymous rule is written.
	rules map[*sysmlv1.Element]ruleForm
	// clocks memoizes the names a simulation configuration gives the clock.
	clocks map[string]string
	// observed memoizes, per observation, the durations and time expressions that read it.
	observed map[*sysmlv1.Element][]*sysmlv1.Element
	// regionUsed holds the vertex names each region's body has taken.
	regionUsed map[*sysmlv1.Element]map[string]bool
	// vertexNames gives the v2 name of every vertex a state machine writes.
	vertexNames map[*sysmlv1.Element]string
	// points says how each connection point of a composite state is written.
	points map[*sysmlv1.Element]pointForm
	// incoming and outgoing list the transitions into and out of each vertex
	// of the machines named so far.
	incoming, outgoing map[*sysmlv1.Element][]*sysmlv1.Element
	// instant names, per state machine, the TimeInstantValue attribute each
	// absolute time event its transitions accept is written as.
	instant map[*sysmlv1.Element]map[*sysmlv1.Element]instantValue
	// self names the object whose features a behavior body reads: `this`, or the
	// subject of a test case while its scenario is written.
	self string
}

// pendingNotes are the notes on an element annotated before it is reported,
// and whether they make its migration an approximation.
type pendingNotes struct {
	notes       []string
	approximate bool
}

// add records e's verdict. An element reported before keeps one entry: the
// weaker verdict, the target that was written, and every distinct note.
func (m *migration) add(e *sysmlv1.Element, v Verdict, target, note string) {
	if p, ok := m.pending[e]; ok {
		delete(m.pending, e)
		for _, n := range p.notes {
			note = joinNotes(note, n)
		}
		if p.approximate && v == Mapped {
			v = Approximated
		}
	}
	if n, ok := m.names[e]; ok && e.Name != "" && n != e.Name && m.realizes[e] == nil {
		if v == Mapped {
			v = Approximated
		}
		note = joinNotes(note, "written as "+n+" since a sibling is also named "+e.Name)
	}
	if i, ok := m.indexed[e.ID]; ok && e.ID != "" {
		en := &m.report.Entries[i]
		if weaker(v, en.Verdict) {
			en.Verdict = v
		}
		if en.Target == "" {
			en.Target = target
		}
		if !strings.Contains(en.Note, note) {
			en.Note = joinNotes(en.Note, note)
		}
		return
	}
	m.indexed[e.ID] = len(m.report.Entries)
	m.report.Entries = append(m.report.Entries, Entry{
		ID: e.ID, Kind: kindOf(e), Name: qualifiedName(e), Target: target, Verdict: v, Note: note,
	})
}

// weaker reports whether verdict a says less was migrated than b.
func weaker(a, b Verdict) bool {
	rank := func(v Verdict) int {
		switch v {
		case Unmapped:
			return 3
		case Skipped:
			return 2
		case Approximated:
			return 1
		}
		return 0
	}
	return rank(a) > rank(b)
}

// prepare walks the model once ahead of writing: it indexes the item flows
// by realizing connector, names every anonymous feature that is referred to,
// types the run configurations' result snapshots, and then exposes the
// features the connectors and slots that will be written reach.
func (m *migration) prepare() {
	var reachers, configs, laned []*sysmlv1.Element
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		m.distinguish(e)
		if simulationConfig(e) != nil {
			configs = append(configs, e)
		}
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
				for _, id := range nce.IDs("propertyPath") {
					if p := m.model.Lookup(id); p != nil {
						m.nameFor(p)
					}
				}
			}
		case "Connector":
			reachers = append(reachers, e)
			m.connectors = append(m.connectors, e)
		case "InstanceSpecification":
			reachers = append(reachers, e)
		case "SendSignalAction":
			if m.model.Ref(e, "onPort") != nil && m.model.Ref(e, "signal") != nil {
				m.portSends = append(m.portSends, e)
			}
		case "OpaqueExpression":
			// A default, rule or slot value is read in the scope of its owner's owner.
			if e.Parent != nil && e.Parent.Parent != nil {
				m.exposeNamed(e, e.Parent.Parent)
			}
		case "Property", "Port":
			if e.Type == "Port" {
				m.ports = append(m.ports, e)
			}
			for _, r := range m.model.Refs(e, "redefinedProperty") {
				m.expose(r, qualifiedName(e)+" redefines it")
			}
			for _, r := range m.model.Refs(e, "subsettedProperty") {
				m.expose(r, qualifiedName(e)+" subsets it")
			}
			if r, ok := m.shadowed(e); ok {
				m.expose(r, qualifiedName(e)+" redefines it by name")
			}
		case "Dependency", "Abstraction", "Realization", "Usage":
			for _, role := range []string{"client", "supplier"} {
				if r := m.model.Ref(e, role); r != nil && (r.Type == "Property" || r.Type == "Port") {
					m.nameFor(r)
				}
			}
			if has(e, "Allocate") {
				for _, c := range m.model.Refs(e, "client") {
					m.allocated[c] = append(m.allocated[c], m.model.Refs(e, "supplier")...)
				}
			}
			m.placeDependency(e)
		case "Operation":
			if method := m.model.Ref(e, "method"); method != nil && method.Parent == e.Parent {
				m.methodOf[method] = e
				m.realizeParameters(e, method)
			}
		case "Activity":
			laned = append(laned, e)
		case "DurationConstraint":
			for _, c := range m.model.Refs(e, "constrainedElement") {
				m.bounded[c] = append(m.bounded[c], e)
			}
		case "Trigger":
			if ev := m.model.Ref(e, "event"); ev != nil {
				m.triggered[ev] = true
			}
		case "CallBehaviorAction":
			m.invoke(e, "behavior")
		case "State":
			m.invoke(e, "entry", "doActivity", "exit")
		case "Transition":
			m.invoke(e, "effect")
		case "Class", "Component", "Node", "Device", "ExecutionEnvironment":
			m.invoke(e, "classifierBehavior")
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
	m.indexSnapshots(configs)
	for _, e := range reachers {
		m.exposeReached(e)
	}
	for _, act := range laned {
		m.prepareLanes(act)
	}
	m.admitAbsent(laned)
}

// isActionNode reports whether e is an action node of an activity graph, written
// as an action usage in the body of its graph.
func isActionNode(e *sysmlv1.Element) bool {
	return e.Role == "node" && nodeKind(e) == nodeAction && e.Parent != nil
}

// nodeGraph returns the activity or structured node whose graph n is an action node
// of, and the definition the graph is written as the body of; nil for another element.
func (m *migration) nodeGraph(n *sysmlv1.Element) (act, def *sysmlv1.Element) {
	if !isActionNode(n) {
		return nil, nil
	}
	act = n.Parent
	switch {
	case isStructured(act):
		return act, act
	case act.Type == "Activity":
		if op := m.methodOf[act]; op != nil {
			return act, op
		}
		return act, act
	}
	return nil, nil
}

// nameNode returns the v2 name of an action node: the one its graph's writer gave
// it, or one fixed ahead of the write that the writer then keeps. "" for another element.
func (m *migration) nameNode(n *sysmlv1.Element) string {
	act, def := m.nodeGraph(n)
	if act == nil {
		return ""
	}
	if name, ok := m.nodeNames[n]; ok {
		return name
	}
	return m.newActivity(act, def).name(n, baseName(n))
}

// exposeReached exposes the features a connector's ends or an instance's slots
// refer to, once the connector or slot resolves as the writer will write it;
// one that is left as a comment reaches nothing.
func (m *migration) exposeReached(e *sysmlv1.Element) {
	switch e.Type {
	case "Connector":
		if e.Parent == nil {
			return
		}
		ends, note := m.connectorEnds(e, e.Parent)
		if note != "" {
			return
		}
		for _, segs := range ends {
			for _, s := range segs {
				if s.Parent != e.Parent {
					m.expose(s, "connector "+describe(e)+" in "+qualifiedName(e.Parent)+" reaches it")
				}
			}
		}
	case "InstanceSpecification":
		for _, slot := range e.Owned("slot") {
			f := m.model.Ref(slot, "definingFeature")
			if _, _, ok := m.slotForm(e, slot, f); ok {
				m.expose(f, "instance "+describe(e)+" has a slot for it")
			}
		}
	}
}

// expose records the first thing found to reach feature f from outside its
// owner, which its v2 declaration must then not hide.
func (m *migration) expose(f *sysmlv1.Element, by string) {
	if _, ok := m.exposed[f]; !ok && !f.IsProxy() {
		m.exposed[f] = by
	}
}

// hasFeature reports whether f is a feature of classifier c, owned or inherited.
func (m *migration) hasFeature(c, f *sysmlv1.Element) bool {
	return f.Parent != nil && (f.Parent == c || m.inherits(c, f.Parent))
}

// slotClassifier returns the classifier instance e is written to specialize
// that has feature f, or nil: a slot of a classifier the v2 form omits (a
// value type beside a block) has no feature to redefine.
func (m *migration) slotClassifier(e, f *sysmlv1.Element) *sysmlv1.Element {
	occurrences, values, _ := m.instanceClassifiers(e)
	classifiers := values
	if len(occurrences) > 0 {
		_, classifiers, _ = m.individualClassifiers(e)
	}
	for _, c := range classifiers {
		if m.hasFeature(c, f) {
			return c
		}
	}
	return nil
}

// distinguish renames the later of two members of e that share a name, since
// v2 members of one namespace must be distinct while UML allows the clash.
func (m *migration) distinguish(e *sysmlv1.Element) {
	seen := map[string]bool{}
	for _, c := range namespaceMembers(e) {
		if c.Name == "" {
			continue
		}
		if !seen[c.Name] {
			seen[c.Name] = true
			continue
		}
		m.names[c] = distinct(seen, func(n string) bool { return m.nameTaken(e, n) }, c.Name)
	}
}

// distinct gives a member named name, which a sibling in seen already bears, the
// first `name 2`, `name 3`, … neither seen nor taken, and marks it seen.
func distinct(seen map[string]bool, taken func(string) bool, name string) string {
	fresh := name
	for i := 2; seen[fresh] || taken(fresh); i++ {
		fresh = fmt.Sprintf("%s %d", name, i)
	}
	seen[fresh] = true
	return fresh
}

// namespaceMembers lists the children of e written as members of its v2 body: its
// own, and the named vertices of its one written region, which v2 puts beside them.
// A state's connection points are left to namePoints, which knows which are written.
func namespaceMembers(e *sysmlv1.Element) []*sysmlv1.Element {
	var members []*sysmlv1.Element
	var inline *sysmlv1.Element
	if written := writtenRegions(e); len(written) == 1 {
		inline = written[0]
	}
	for _, c := range e.Children {
		switch {
		case c.Role == "region":
			if c != inline {
				continue
			}
			for _, v := range c.Owned("subvertex") {
				if vertexBase(v) != "" && memberOwner(v) != e {
					members = append(members, v)
				}
			}
			members = append(members, c.Owned("transition")...)
		case c.Role == "connectionPoint" && e.Type == "State":
		case !ownerWritten(c.Role):
			members = append(members, c)
		}
	}
	return members
}

// ownerWritten reports whether a child in role is written by its owner rather
// than as a member of its body; an action's pins are, and it names them apart.
func ownerWritten(role string) bool {
	switch role {
	case "ownedComment", "generalization", "lowerValue", "upperValue", "defaultValue",
		"end", "specification", "type", "general", "annotatedElement", "body", "language",
		"ownedEnd", "memberEnd", "value", "slot", "ownedLiteral", "ownedParameter", "region",
		"argument", "result", "inputValue", "outputValue", "object", "target", "insertAt", "removeAt":
		return true
	}
	return false
}

// root writes a top-level element: a Model's members are written at the top
// level, any other root as a declaration of its own.
func (m *migration) root(e *sysmlv1.Element) {
	if e.Type == "Model" && !m.isLibrary(e) {
		m.add(e, Mapped, "", "the root model's members are written at the top level")
		m.body(e)
		return
	}
	m.member(e)
}

// body writes the members of e's body, in document order, then what other
// elements contribute to it.
func (m *migration) body(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	m.members(e)
	m.scope = saved
}

// members writes the owned members of e, the current scope, then what other
// elements contribute to its body.
func (m *migration) members(e *sysmlv1.Element) {
	for _, c := range e.Children {
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
}

// member writes one owned element of the current scope.
func (m *migration) member(e *sysmlv1.Element) {
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
	case "ownedReception":
		m.reception(e)
		return
	}
	if op := m.methodOf[e]; op != nil {
		m.methodBehavior(e, op)
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
	case "SignalEvent", "TimeEvent", "ChangeEvent", "CallEvent", "AnyReceiveEvent":
		m.event(e)
		return
	}
	m.classifier(e)
}

// imports writes a package import; profile applications and element imports
// have no v2 counterpart worth writing.
func (m *migration) imports(e *sysmlv1.Element) {
	if e.Type != "PackageImport" {
		m.add(e, Skipped, "", "profile applications and element imports are not written")
		return
	}
	target := m.model.Ref(e, "importedPackage")
	if target == nil || target.IsProxy() || m.isLibrary(target) {
		m.add(e, Skipped, "", "import of a profile or library package")
		return
	}
	// v1 imports are public by default; a bare v2 import is not.
	vis := "public "
	if e.Attrs["visibility"] == "private" {
		vis = privatePrefix
	}
	m.w.line(vis + "import " + m.ref(target, m.scope) + "::*;")
	m.add(e, Mapped, m.v2Name(target), "")
}

// classifier writes a package, classifier or other packaged element.
func (m *migration) classifier(e *sysmlv1.Element) {
	cat, note := m.classify(e)
	switch cat {
	case catNone:
		return
	case catLibrary:
		m.add(e, Skipped, "", note)
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
	if vis := e.Attrs["visibility"]; vis == "private" || vis == "package" || vis == "protected" {
		// A v2 private member is out of reach of every other package, which v1 tools do not enforce.
		note = joinNotes(note, vis+" visibility is not written: v2 lets nothing outside the package reach a private member")
	}
	verdict := Mapped
	if note != "" {
		verdict = Approximated
	}
	header, n := m.classifierHeader(e, cat, name)
	if n != "" {
		verdict = Approximated
		note = joinNotes(note, n)
	}
	if cat == catSimConfig {
		m.simulationConfig(e, header, note)
		return
	}
	m.add(e, verdict, m.v2Name(e), note)
	m.classifierBody(e, cat, header)
	if cat == catPartDef {
		m.monteCarloAnalysis(e)
	}
}

// classifierHeader builds the declaration line a classifier is written with:
// abstract, its keyword and name, its requirement id, its generalizations; n
// notes what the generalizations and dangling ends leave out.
func (m *migration) classifierHeader(e *sysmlv1.Element, cat category, name string) (header string, n string) {
	var b strings.Builder
	if (e.Attrs["isAbstract"] == "true" && cat != catValue) || m.abstractOperation(e) {
		b.WriteString("abstract ")
	}
	if cat == catIndividualDef {
		kind, _, _ := m.individualClassifiers(e)
		b.WriteString(individualKeyword(kind))
	} else {
		b.WriteString(cat.keyword())
	}
	b.WriteByte(' ')
	if cat == catRequirementDef {
		if id := requirementID(e); id != "" {
			b.WriteString("<" + writeName(id) + "> ")
		}
	}
	b.WriteString(writeName(name))
	var gens string
	if cat == catMetadataDef {
		gens, n = m.metadataGenerals(e)
	} else {
		gens, n = m.generals(e, cat)
	}
	if gens != "" {
		if cat == catValue {
			b.WriteString(" : " + gens)
		} else {
			b.WriteString(" :> " + gens)
		}
	}
	if cat == catConnectionDef {
		n = joinNotes(n, m.dangling(e, "memberEnd"))
	}
	return b.String(), n
}

// classifierBody writes the member block a classifier category carries.
func (m *migration) classifierBody(e *sysmlv1.Element, cat category, header string) {
	switch cat {
	case catConnectionDef:
		m.association(e)
		return
	case catIndividualDef, catValue:
		m.w.block(header, func() { m.individualBody(e) })
		return
	case catVerificationDef:
		m.w.block(header, func() { m.verificationBody(e) })
		return
	case catConstraintDef:
		m.w.block(header, func() { m.constraintBody(e) })
		return
	case catRequirementDef:
		m.w.block(header, func() { m.requirementBody(e) })
		return
	case catMetadataDef:
		m.w.block(header, func() { m.metadataBody(e) })
		return
	case catEnumDef:
		m.w.block(header, func() {
			m.comments(e)
			for _, lit := range e.Owned("ownedLiteral") {
				m.w.line(writeName(m.nameOf(lit)) + ";")
				m.add(lit, Mapped, m.v2Name(lit), "")
			}
			m.stereotypeAnnotations(e)
		})
		return
	}
	if behaviorCategory(cat) {
		m.w.block(header, func() { m.behaviorBody(e, cat) })
		if e.Type == "Operation" {
			m.operationFeature(e)
		}
		return
	}
	m.w.block(header, func() {
		m.body(e)
		m.classifierBehavior(e)
		m.stereotypeAnnotations(e)
	})
}

// generals writes the specializations of a classifier: its generalizations,
// and for a value type the ScalarValues type it derives from.
func (m *migration) generals(e *sysmlv1.Element, cat category) (string, string) {
	var refs []string
	var notes []string
	for _, g := range e.Owned("generalization") {
		target := m.model.Ref(g, "general")
		if target == nil {
			notes = append(notes, "a generalization refers to nothing in the document")
			continue
		}
		if sv := m.scalarValue(target); sv != "" {
			refs = append(refs, scalarValuesPrefix+sv)
			continue
		}
		if cat == catAttributeDef && m.quantityValueType(target) {
			refs = append(refs, "ScalarValues::Real")
			notes = append(notes, "the quantity value type "+qualifiedName(target)+" is written as ScalarValues::Real; its unit is not kept")
			continue
		}
		if isMonteCarloAnalysis(target) {
			notes = append(notes, m.monteCarloGeneralization(e))
			continue
		}
		if target.IsProxy() || m.isLibrary(target) {
			notes = append(notes, "generalization of library type "+qualifiedName(target)+" is not written")
			continue
		}
		if tc, _ := m.classify(target); tc != cat {
			notes = append(notes, "generalization of "+qualifiedName(target)+" is not written: it becomes a "+tc.keyword()+", not a "+cat.keyword())
			continue
		}
		refs = append(refs, m.ref(target, m.scope))
	}
	if cat == catAttributeDef && len(refs) == 0 && quantity(e) {
		refs = append(refs, "ScalarValues::Real")
		notes = append(notes, "a value type with a unit or quantity kind and no base type is written as ScalarValues::Real")
	}
	if cat == catIndividualDef || cat == catValue {
		var written []*sysmlv1.Element
		if cat == catValue {
			_, written, _ = m.instanceClassifiers(e)
		} else {
			var note string
			_, written, note = m.individualClassifiers(e)
			if note != "" {
				notes = append(notes, note)
			}
		}
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
func (m *migration) dangling(e *sysmlv1.Element, roles ...string) string {
	var notes []string
	for _, role := range roles {
		if ids := m.model.Unresolved(e, role); len(ids) > 0 {
			notes = append(notes, fmt.Sprintf("%d %s reference(s) resolve to nothing in the document (%s)", len(ids), role, strings.Join(ids, ", ")))
		}
	}
	return strings.Join(notes, "; ")
}

// downgrade marks e's report entry approximated with a further note.
func (m *migration) downgrade(e *sysmlv1.Element, note string) {
	m.annotate(e, note, true)
}

// note adds a further note to e's report entry without changing its verdict.
func (m *migration) note(e *sysmlv1.Element, note string) {
	m.annotate(e, note, false)
}

// annotate adds a note to the report entry of e, approximating its verdict
// when approximate says so; an element not yet reported keeps the note until add reports it.
func (m *migration) annotate(e *sysmlv1.Element, note string, approximate bool) {
	i, ok := m.indexed[e.ID]
	if !ok {
		p := m.pending[e]
		if p == nil {
			p = &pendingNotes{}
			m.pending[e] = p
		}
		if !slices.Contains(p.notes, note) {
			p.notes = append(p.notes, note)
		}
		p.approximate = p.approximate || approximate
		return
	}
	en := &m.report.Entries[i]
	if approximate && en.Verdict == Mapped {
		en.Verdict = Approximated
	}
	if !strings.Contains(en.Note, note) {
		en.Note = joinNotes(en.Note, note)
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
// the capitalized one some tools write, as one line of plain text.
func requirementID(e *sysmlv1.Element) string {
	return strings.Join(strings.Fields(commentText(requirementTag(e, "Id", "id", "ID"))), " ")
}

func requirementText(e *sysmlv1.Element) string {
	return requirementTag(e, "Text", "text")
}

// requirementTag reads a tag from the applications carrying a standard requirement
// stereotype's meaning; an unrelated stereotype's same-named tag stays with it.
func requirementTag(e *sysmlv1.Element, tags ...string) string {
	for _, s := range e.Stereotypes {
		if !appliesAny(s, requirementStereotypes...) {
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

func (m *migration) requirementBody(e *sysmlv1.Element) {
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
	m.stereotypeAnnotations(e)
	m.scope = saved
}

// ruleForm is how a constraint block's anonymous rule is written: the result
// expression when its specification has a v2 form, else the note saying why not.
type ruleForm struct {
	rule, spec *sysmlv1.Element
	expr, note string
	ok         bool
}

// constraintRule resolves, once, the anonymous rule of constraint block e;
// rule is nil when the block has none.
func (m *migration) constraintRule(e *sysmlv1.Element) ruleForm {
	if f, ok := m.rules[e]; ok {
		return f
	}
	var f ruleForm
	for _, c := range e.Children {
		if c.Role == "ownedRule" && c.Name == "" {
			f.rule = c
			break
		}
	}
	if f.rule != nil {
		f.spec = m.model.Ref(f.rule, "specification")
		if f.spec == nil {
			f.spec = firstOwned(f.rule, "specification")
		}
		if f.spec == nil {
			f.note = "the constraint has no specification"
		} else {
			f.expr, f.ok, f.note = m.valueExprAs(f.spec, e, oneOf("Boolean", "the constraint yields"))
		}
	}
	m.rules[e] = f
	return f
}

// constraintBody writes a constraint block: its parameters, then its
// anonymous rule as the result expression.
func (m *migration) constraintBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	f := m.constraintRule(e)
	for _, c := range e.Children {
		if c != f.rule {
			m.member(c)
		}
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeAnnotations(e)
	switch {
	case f.rule == nil:
	case f.spec == nil:
		m.unmapped(f.rule, f.note)
	case f.ok:
		m.w.line(f.expr)
		m.add(f.rule, verdictFor(f.note), m.v2Name(e), f.note)
	default:
		m.unmappedExpr(f.rule, f.spec, f.note)
	}
	m.scope = saved
}

func verdictFor(note string) Verdict {
	if note != "" {
		return Approximated
	}
	return Mapped
}

func firstOwned(e *sysmlv1.Element, role string) *sysmlv1.Element {
	if o := e.Owned(role); len(o) > 0 {
		return o[0]
	}
	return nil
}

// individualBody writes an instance specification's slots as redefinitions
// of the classifier's features with the slot values.
func (m *migration) individualBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	slots, recorded := m.monteCarloSlots(e, e.Owned("slot"))
	for _, slot := range slots {
		f := m.model.Ref(slot, "definingFeature")
		lines, note, ok := m.slotForm(e, slot, f)
		if !ok {
			m.unmapped(slot, note)
			continue
		}
		for _, l := range lines {
			m.w.line(l)
		}
		m.add(slot, verdictFor(note), m.v2Name(e)+"::"+writeName(m.nameFor(f)), note)
	}
	recorded()
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeAnnotations(e)
	m.scope = saved
}

// slotForm resolves a slot of instance e, of defining feature f, into the v2
// lines that write it and the notes on them; ok is false, and note says why,
// when it has no v2 form.
func (m *migration) slotForm(e, slot, f *sysmlv1.Element) (lines []string, note string, ok bool) {
	if f == nil || f.IsProxy() {
		return nil, "the slot's defining feature is not in the document", false
	}
	if m.slotClassifier(e, f) == nil {
		return nil, "the slot's defining feature " + qualifiedName(f) + " is not a feature of any classifier the instance is written to specialize", false
	}
	owner := m.classifyParent(f)
	kw, prefix, _ := m.featureKeyword(f, owner)
	dir, _ := m.featureDirection(f, owner, kw)
	switch kw {
	case "attribute":
		return m.valueSlot(e, slot, f, dir)
	case "part", "item", "constraint", "requirement":
		return m.instanceSlot(e, slot, f, kw, prefix)
	case "port":
		return nil, "the slot of port " + f.Name + " is not written: v2 has no individual port for it to be typed by", false
	default:
		return nil, "the slot of " + f.Name + " is not written: the property is written as a plain " + kw + ", which cannot be typed by an individual", false
	}
}

// valueSlot resolves a slot of a value property into a redefinition bound to
// its values, with the direction the feature has.
func (m *migration) valueSlot(e, slot, f *sysmlv1.Element, dir string) ([]string, string, bool) {
	var vals []string
	var notes []string
	for _, v := range slot.Owned("value") {
		expr, ok, note := m.featureValue(v, f, e)
		if !ok {
			return nil, note, false
		}
		vals = append(vals, expr)
		if note != "" {
			notes = append(notes, note)
		}
	}
	if conflict := m.slotConflict(f, vals); conflict != "" {
		return nil, conflict + valuesNote(vals), false
	}
	value := ""
	switch len(vals) {
	case 0:
	case 1:
		value = " = " + vals[0]
	default:
		value = " = (" + strings.Join(vals, ", ") + ")"
	}
	line := dir + "attribute :>> " + writeName(m.nameFor(f)) + value + ";"
	return []string{line}, strings.Join(notes, "; "), true
}

// instanceSlot resolves a slot holding instances: one redefines the feature
// typed by its individual; several each subset it under a redefinition counting them.
func (m *migration) instanceSlot(e, slot, f *sysmlv1.Element, kw, prefix string) ([]string, string, bool) {
	t := m.model.Ref(f, "type")
	var refs []string
	for _, v := range slot.Owned("value") {
		if v.Type != "InstanceValue" {
			return nil, article(kw) + kw + " holds instances; the slot's value is a " + v.Type, false
		}
		inst := m.model.Ref(v, "instance")
		switch {
		case inst == nil:
			return nil, slotValueSubject + "names no instance", false
		case inst.Type == "EnumerationLiteral" && (kw == "constraint" || kw == "requirement"):
			return nil, "the slot of " + kw + " " + f.Name + " holds the literal " + qualifiedName(inst) + ", the run's verdict on the " + kw + " rather than an instance of its type; an individual has no slot for a verdict", false
		case inst.IsProxy():
			return nil, slotValueSubject + qualifiedName(inst) + " is outside the document, so it has no individual to type " + f.Name + " by", false
		}
		if cat, note := m.classify(inst); cat != catIndividualDef {
			return nil, slotValueSubject + describe(inst) + " is not written as an individual: " + note, false
		}
		kind, classifiers, _ := m.individualClassifiers(inst)
		if kind == catNone || kind.keyword() != kw+" def" {
			return nil, slotValueSubject + describe(inst) + " is an " + individualKeyword(kind) + ", which cannot type " + article(kw) + kw, false
		}
		if !m.instanceOf(classifiers, t) {
			return nil, slotValueSubject + describe(inst) + " is not an instance of " + qualifiedName(t) + ", the type of " + f.Name, false
		}
		// The default individual types the property, so a slot can only repeat it.
		if d, _ := m.typingIndividual(f, kw); d != nil && d != inst {
			return nil, slotValueSubject + describe(inst) + " is not " + describe(d) + ", " + individualSubject + f.Name + " is typed by for its default", false
		}
		refs = append(refs, m.ref(inst, e))
	}
	if conflict := m.slotConflict(f, refs); conflict != "" {
		return nil, conflict + valuesNote(refs), false
	}
	name := writeName(m.nameFor(f))
	lower, upper, ok := bounds(f)
	mult := ""
	if n := len(refs); !ok || lower != n || upper != n {
		mult = fmt.Sprintf("[%d]", n)
	}
	var lines []string
	switch len(refs) {
	case 0:
		return nil, "the slot holds no value", false
	case 1:
		lines = append(lines, prefix+"individual "+kw+" :>> "+name+" : "+refs[0]+mult+";")
	default:
		if mult != "" {
			lines = append(lines, prefix+kw+" :>> "+name+" "+mult+";")
		}
		for _, r := range refs {
			lines = append(lines, prefix+"individual "+kw+" : "+r+" :> "+name+";")
		}
	}
	return lines, "", true
}

// article is the indefinite article before a word: "an item", "a part".
func article(word string) string {
	if strings.ContainsRune("aeiou", rune(word[0])) {
		return "an "
	}
	return "a "
}

// instanceOf reports whether an instance written to specialize the
// classifiers is an instance of t: one of them is t or specializes it.
func (m *migration) instanceOf(classifiers []*sysmlv1.Element, t *sysmlv1.Element) bool {
	for _, c := range classifiers {
		if c == t || m.inherits(c, t) {
			return true
		}
	}
	return false
}

// slotConflict notes how slot values contradict their feature: a count outside
// its multiplicity or a repeat on a unique feature. Both v1 and v2 reject them.
func (m *migration) slotConflict(f *sysmlv1.Element, vals []string) string {
	n := len(vals)
	lower, upper, ok := bounds(f)
	if ok && (n < lower || (upper >= 0 && n > upper)) {
		return fmt.Sprintf("the slot holds %d value(s) for a feature of multiplicity %s", n, boundsText(lower, upper))
	}
	if dup := repeated(vals); dup != "" && f.Attrs["isUnique"] != "false" {
		return "the slot repeats the value " + dup + " on a unique feature"
	}
	return ""
}

// valuesNote lists a slot's values after a conflict note; nothing for none.
func valuesNote(vals []string) string {
	if len(vals) == 0 {
		return ""
	}
	return "; its values are " + strings.Join(vals, ", ")
}

// bounds returns a property's multiplicity as numbers, upper -1 for unbounded;
// ok is false when a bound is not a literal number.
func bounds(p *sysmlv1.Element) (lower, upper int, ok bool) {
	lower, upper = 1, 1
	var err error
	if lv := firstOwned(p, "lowerValue"); lv != nil {
		if lower, err = strconv.Atoi(boundValue(lv)); err != nil {
			return 0, 0, false
		}
	}
	if uv := firstOwned(p, "upperValue"); uv != nil {
		if uv.Attrs["value"] == "*" {
			upper = -1
		} else if upper, err = strconv.Atoi(boundValue(uv)); err != nil {
			return 0, 0, false
		}
	}
	return lower, upper, true
}

// boundValue reads a multiplicity bound; UML reads an omitted value as 0.
func boundValue(b *sysmlv1.Element) string {
	if v := b.Attrs["value"]; v != "" {
		return v
	}
	return "0"
}

func boundsText(lower, upper int) string {
	if upper < 0 {
		return fmt.Sprintf("%d..*", lower)
	}
	if lower == upper {
		return strconv.Itoa(lower)
	}
	return fmt.Sprintf("%d..%d", lower, upper)
}

// repeated returns the first value written more than once, or "". Numbers
// compare by value, so 1 and 1.0 repeat; anything else by its text.
func repeated(vals []string) string {
	seen := map[string]bool{}
	for _, v := range vals {
		key := v
		if r, ok := new(big.Rat).SetString(v); ok && decimal(v) {
			key = "number " + r.RatString()
		}
		if seen[key] {
			return v
		}
		seen[key] = true
	}
	return ""
}

// verificationBody writes a test case: the requirements it verifies form its
// objective; an interaction's scenario runs on its subject, the interaction's context.
func (m *migration) verificationBody(e *sysmlv1.Element) {
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
	if e.Type == "Interaction" {
		subject := m.subjectName(e)
		if s, note := m.scenario(e, subject); note == "" {
			m.w.line("subject " + writeName(subject) + " : " + m.ref(s.context, e) + ";")
			m.parameters(e, e)
			s.write()
		}
	}
	m.stereotypeAnnotations(e)
	m.scope = saved
}

// subjectName names the subject of a test case written from an interaction:
// `context`, the interaction's context block, unless a member of the case takes the name.
func (m *migration) subjectName(e *sysmlv1.Element) string {
	used := map[string]bool{"start": true, "done": true}
	for _, c := range e.Children {
		if n := m.nameOf(c); n != "" {
			used[n] = true
		}
	}
	return freshIn(used, "context")
}

// ownsEveryEnd reports whether no classifier property carries the association:
// every member end is owned by the association itself.
func ownsEveryEnd(e *sysmlv1.Element, ends []*sysmlv1.Element) bool {
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
func (m *migration) association(e *sysmlv1.Element) {
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
			m.associationEnd(e, end, used)
		}
		for _, c := range e.Children {
			if c.Role != "ownedEnd" {
				m.member(c)
			}
		}
		for _, extra := range m.extras[e] {
			extra()
		}
		m.stereotypeAnnotations(e)
		m.scope = saved
	})
}

// associationEnd writes one member end of a connection def, renaming it past
// the ends and members already written.
func (m *migration) associationEnd(e, end *sysmlv1.Element, used map[string]bool) {
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

// featureKeyword decides the v2 usage keyword of a v1 property from its type
// and aggregation, given the category of its owner; prefix is `ref ` or empty.
func (m *migration) featureKeyword(p *sysmlv1.Element, owner category) (keyword, prefix, note string) {
	t := m.model.Ref(p, "type")
	if owner == catConstraintDef {
		// A constraint block's properties are its parameters, whichever metaclass
		// a tool stores them as: a value, or a reference to what it constrains.
		if t == nil {
			return "attribute", "", ""
		}
		switch kw, note := m.typeKeyword(t); kw {
		case "part", "item":
			return kw, "ref ", note
		default:
			return kw, "", note
		}
	}
	if p.Type == "Port" {
		return "port", "", ""
	}
	if t == nil {
		return "ref", "", "the untyped property is written as a reference usage"
	}
	kw, note := m.typeKeyword(t)
	switch kw {
	case "item":
		switch {
		case owner == catPortDef, p.Attrs["aggregation"] == "composite":
			return "item", "", note
		case p.Attrs["aggregation"] == "shared":
			return "item", "ref ", joinNotes(note, "shared aggregation is written as a reference item")
		}
		return "item", "ref ", note
	case "part":
		if owner == catPortDef {
			return "item", "", note
		}
		switch p.Attrs["aggregation"] {
		case "composite":
			return "part", "", note
		case "shared":
			return "part", "ref ", joinNotes(note, "shared aggregation is written as a reference part")
		}
		return "part", "ref ", note
	case "action", "state", "calc":
		// A property typed by a behavior holds a performance; only a perform
		// or exhibit usage runs one, so the property is a reference.
		return kw, "ref ", joinNotes(note, "a property typed by a behavior is written as a reference "+kw+" usage")
	}
	return kw, "", note
}

// typeKeyword is the usage keyword a property takes from its type alone,
// before its owner and aggregation weigh in.
func (m *migration) typeKeyword(t *sysmlv1.Element) (keyword, note string) {
	if m.scalarValue(t) != "" {
		return "attribute", ""
	}
	tc, _ := m.classify(t)
	switch tc {
	case catAttributeDef, catEnumDef:
		return "attribute", ""
	case catItemDef:
		return "item", ""
	case catConstraintDef:
		return "constraint", ""
	case catPortDef:
		// Only a port is typed by a port def, in an interface block as anywhere.
		return "port", "a property typed by an interface block is written as a port"
	case catRequirementDef:
		return "requirement", ""
	case catActionDef:
		return "action", ""
	case catStateDef:
		return "state", ""
	case catCalcDef:
		return "calc", ""
	case catNone, catLibrary:
		return "attribute", "typed by library element " + t.Name + " with no known v2 counterpart"
	case catUnmapped:
		return "ref", "typed by " + qualifiedName(t) + ", which is not migrated"
	}
	return "part", ""
}

// featureDirection is the direction a feature is written with: a constraint
// parameter is `in`, a port and a flow property carry their own.
func (m *migration) featureDirection(p *sysmlv1.Element, owner category, kw string) (dir, note string) {
	switch {
	case owner == catConstraintDef && kw != "constraint":
		return "in ", ""
	case p.Type == "Port":
		return portDirection(p)
	case owner == catPortDef:
		if fp := stereo(p, "FlowProperty"); fp != nil {
			switch fp.Tag("direction") {
			case "in":
				return "in ", ""
			case "out":
				return "out ", ""
			case "inout":
				return "inout ", ""
			}
		}
	}
	return "", ""
}

// feature writes a property or port of the current scope.
func (m *migration) feature(p *sysmlv1.Element) {
	if reason := toolContent(p); reason != "" {
		m.add(p, Skipped, "", reason)
		return
	}
	ownerCat, _ := m.classify(m.scope)
	kw, prefix, note := m.featureKeyword(p, ownerCat)
	t := m.model.Ref(p, "type")
	typ, tnote := m.typeRef(t, m.scope)
	note = joinNotes(note, tnote)
	param := ownerCat == catConstraintDef && kw != "constraint"

	var b strings.Builder
	visPrefix, note := m.featureVisibility(p, param, note)
	b.WriteString(visPrefix)
	// The v2 usage prefix orders direction, derived, abstract, constant, ref.
	dir, dnote := m.featureDirection(p, ownerCat, kw)
	note = joinNotes(note, dnote)
	b.WriteString(dir)
	prefix, note = m.featureModifiers(&b, p, ownerCat, kw, dir, prefix, note)
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
	payload := m.portPayload(p, kw, typ, t)
	ind, indNote := m.typingIndividual(p, kw)
	m.featureTyping(&b, p, ind, payload, typ)
	mult, mnote := m.multiplicity(p)
	b.WriteString(mult + collection(p))
	note = joinNotes(note, mnote)
	note = m.featureRedefinitions(&b, p, note)
	note = m.featureShadow(&b, p, kw, note)
	note = joinNotes(note, m.dangling(p, "redefinedProperty", "subsettedProperty"))

	var bodyLines []string
	note = m.featureDefault(&b, &bodyLines, p, ind, payload, indNote, note)
	if payload != "" {
		line, plnote := m.portPayloadLine(p, payload, typ, t)
		bodyLines = append(bodyLines, line)
		note = joinNotes(note, plnote)
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
		m.stereotypeAnnotations(p)
		m.scope = saved
	})
}

// featureVisibility returns the visibility prefix written for a feature and
// notes a visibility v2 cannot carry.
func (m *migration) featureVisibility(p *sysmlv1.Element, param bool, note string) (string, string) {
	vis := p.Attrs["visibility"]
	switch {
	case vis != "private" && vis != "protected" && vis != "package":
		return "", note
	case param:
		// A parameter is bound from outside the constraint, so it must stay visible.
		return "", joinNotes(note, vis+" visibility is not written on a constraint parameter")
	case m.exposed[p] != "":
		// v2 neither inherits a private feature nor lets a path reach one.
		return "", joinNotes(note, vis+" visibility is not written: "+m.exposed[p])
	case vis == "protected":
		return "protected ", note
	case vis == "package":
		return privatePrefix, joinNotes(note, "package visibility is written as private")
	default:
		return privatePrefix, note
	}
}

// portPayload is the feature kind a port typed by other than a port def holds
// as its one directed feature, or "".
func (m *migration) portPayload(p *sysmlv1.Element, kw, typ string, t *sysmlv1.Element) string {
	if kw != "port" || p.Type != "Port" || typ == "" {
		return ""
	}
	switch tc, _ := m.classify(t); {
	case m.scalarValue(t) != "", tc == catAttributeDef, tc == catEnumDef:
		return "attribute"
	case tc == catPartDef, tc == catItemDef:
		return "item"
	}
	return ""
}

// featureRedefinitions writes the redefinition and subsetting targets into b,
// noting the targets with no v2 declaration.
func (m *migration) featureRedefinitions(b *strings.Builder, p *sysmlv1.Element, note string) string {
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
	return note
}

// featureDefault writes a feature's default value into b, or keeps it as a
// body comment, noting what became of it.
func (m *migration) featureDefault(b *strings.Builder, bodyLines *[]string, p, ind *sysmlv1.Element, payload, indNote, note string) string {
	dv := firstOwned(p, "defaultValue")
	if dv == nil {
		return note
	}
	expr, ok, vnote := m.featureValue(dv, p, m.scope)
	if indNote != "" {
		vnote = indNote
	}
	switch {
	case ind != nil && payload == "":
		return joinNotes(note, "the default value, the individual "+qualifiedName(ind)+", is written as a type of the usage: a definition is not a v2 value")
	case ok:
		b.WriteString(" default = " + expr)
		return joinNotes(note, vnote)
	default:
		*bodyLines = append(*bodyLines, commentLines("default value not migrated: "+describeValue(dv)+" — "+vnote)...)
		return joinNotes(note, "default value not migrated: "+vnote)
	}
}

// featureModifiers writes the derived/abstract/constant prefixes and forces
// the reference prefix an interface block's usages take; returns the prefix.
func (m *migration) featureModifiers(b *strings.Builder, p *sysmlv1.Element, ownerCat category, kw, dir, prefix, note string) (string, string) {
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
	if ownerCat == catPortDef && dir == "" && prefix == "" && (kw == "item" || kw == "part") {
		// An interface block's usages other than ports must not be composite.
		prefix = "ref "
		note = joinNotes(note, "the undirected "+kw+" of an interface block is written as a reference")
	}
	return prefix, note
}

// featureTyping resolves the individual a feature is typed by into its type,
// conjugating a conjugated port, and writes it into b.
func (m *migration) featureTyping(b *strings.Builder, p, ind *sysmlv1.Element, payload, typ string) {
	if ind != nil && payload == "" {
		// A v2 definition is not a value; the usage is typed by the individual instead.
		if typ == "" {
			typ = m.ref(ind, m.scope)
		} else {
			typ += ", " + m.ref(ind, m.scope)
		}
	}
	if typ != "" && payload == "" {
		if p.Type == "Port" && p.Attrs["isConjugated"] == "true" {
			typ = "~" + typ
		}
		b.WriteString(" : " + typ)
	}
}

// featureShadow writes the redefinition an inherited member of the same name
// forces, or notes why the member cannot be redefined.
func (m *migration) featureShadow(b *strings.Builder, p *sysmlv1.Element, kw, note string) string {
	r, redefinable := m.shadowed(p)
	if redefinable {
		b.WriteString(" :>> " + m.featureRef(r))
		return joinNotes(note, "written as a redefinition of the inherited "+qualifiedName(r)+": v2 does not let a member share an inherited member's name")
	}
	if r != nil {
		rkw, _, _ := m.featureKeyword(r, m.classifyParent(r))
		return joinNotes(note, "shares the name of the inherited "+qualifiedName(r)+", which is written as "+rkw+" and so cannot be redefined by this "+kw+": v2 does not let a member share an inherited member's name")
	}
	return note
}

// portPayloadLine is the directed feature a typed port's body holds, with its note.
func (m *migration) portPayloadLine(p *sysmlv1.Element, payload, typ string, t *sysmlv1.Element) (string, string) {
	dir, _ := portDirection(p)
	if payload == "item" && dir == "" {
		// An undirected item in a port must still not be composite.
		payload = "ref item"
	}
	return dir + payload + " " + writeName(m.nameFor(p)) + " : " + typ + ";",
		"a port typed by a " + t.Type + " is written as a port holding one directed " + payload
}

// portDirection writes the direction prefix of a flow port.
func portDirection(p *sysmlv1.Element) (string, string) {
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

// shadowed returns the written inherited property or port that p, declaring
// no redefinition, would hide by sharing its name, or nil when there is none;
// redefinable says whether both are the same kind of usage, so p can redefine it.
func (m *migration) shadowed(p *sysmlv1.Element) (f *sysmlv1.Element, redefinable bool) {
	if p.Parent == nil || p.Name == "" || len(m.model.Refs(p, "redefinedProperty")) > 0 {
		return nil, false
	}
	ownerCat := m.classifyParent(p)
	if ownerCat.keyword() == "" {
		return nil, false
	}
	seen := map[*sysmlv1.Element]bool{p.Parent: true}
	var walk func(*sysmlv1.Element) *sysmlv1.Element
	walk = func(c *sysmlv1.Element) *sysmlv1.Element {
		for _, g := range c.Owned("generalization") {
			t := m.model.Ref(g, "general")
			if t == nil || seen[t] {
				continue
			}
			seen[t] = true
			for _, f := range t.Children {
				if (f.Type == "Property" || f.Type == "Port") && m.nameOf(f) == m.nameOf(p) && m.written(f) {
					return f
				}
			}
			if f := walk(t); f != nil {
				return f
			}
		}
		return nil
	}
	f = walk(p.Parent)
	if f == nil {
		return nil, false
	}
	pkw, _, _ := m.featureKeyword(p, ownerCat)
	fkw, _, _ := m.featureKeyword(f, m.classifyParent(f))
	return f, pkw == fkw
}

// classifyParent is the category of the element that owns e.
func (m *migration) classifyParent(e *sysmlv1.Element) category {
	if e.Parent == nil {
		return catNone
	}
	cat, _ := m.classify(e.Parent)
	return cat
}

// written reports whether e becomes a v2 element that can be referred to.
func (m *migration) written(e *sysmlv1.Element) bool {
	if e == nil || e.IsProxy() {
		return false
	}
	// The root Model is the file itself, so nothing can refer to it.
	if e.Parent == nil && e.Type == "Model" {
		return false
	}
	switch e.Type {
	case "Property", "Port", "EnumerationLiteral":
		return m.written(e.Parent)
	case "Parameter":
		p := e.Parent
		if p == nil || (p.Type != "Operation" && !isBehavior(p)) {
			return false
		}
		return m.written(p) || inlinedBehavior(p) && hasActionForm(p)
	case "Association":
		return e.Name != ""
	}
	if act, _ := m.nodeGraph(e); act != nil {
		return m.written(act)
	}
	if op := m.methodOf[e]; op != nil {
		return m.written(op)
	}
	if inlinedBehavior(e) {
		return false
	}
	cat, _ := m.classify(e)
	return cat.keyword() != ""
}

// inlinedBehavior reports whether e is a behavior a state or transition owns, written
// as its owner's entry, do, exit or effect action, which nothing else can name.
func inlinedBehavior(e *sysmlv1.Element) bool {
	return isBehavior(e) && e.Parent != nil && (e.Parent.Type == "State" || e.Parent.Type == "Transition")
}

// hasActionForm reports whether behavior b is written inline as an action
// body, parameters included, when a state or transition owns it.
func hasActionForm(b *sysmlv1.Element) bool {
	switch b.Type {
	case "Activity", "OpaqueBehavior", "FunctionBehavior":
		return true
	}
	return false
}

// featureRef writes a reference to a property from a feature that redefines or
// subsets it: the simple name when it is inherited into the current scope.
func (m *migration) featureRef(r *sysmlv1.Element) string {
	if r.Parent != nil && m.inherits(m.scope, r.Parent) && r.Name != "" {
		return writeName(m.nameOf(r))
	}
	return m.ref(r, m.scope)
}

// conform reports whether types a and b may be bound as v2 judges a binding: one
// specializes the other, or both are numbers; an unknown type is trusted.
func (m *migration) conform(a, b *sysmlv1.Element) bool {
	if a == nil || b == nil || a == b || m.inherits(a, b) || m.inherits(b, a) {
		return true
	}
	sa, sb := m.scalarBase(a), m.scalarBase(b)
	switch {
	case sa != "" && sb != "":
		return sa == sb || (numericScalar[sa] && numericScalar[sb])
	case sa != "" || sb != "":
		other := b
		if sb != "" {
			other = a
		}
		return !m.structuredValueType(other) && !m.written(other)
	}
	return !m.written(a) || !m.written(b)
}

// numericScalar lists the ScalarValues types that specialize Number, which
// conform to one another for a binding as far as migration can tell.
var numericScalar = map[string]bool{
	"Natural": true, "Integer": true, "Rational": true, "Real": true, "Complex": true, "Number": true,
}

// inherits reports whether classifier e specializes general, transitively.
func (m *migration) inherits(e, general *sysmlv1.Element) bool {
	seen := map[*sysmlv1.Element]bool{}
	var walk func(*sysmlv1.Element) bool
	walk = func(c *sysmlv1.Element) bool {
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

// signalAttributes lists the attributes a signal's constructor binds by position:
// its own, then the inherited ones no attribute nearer the signal redefines or shadows.
func (m *migration) signalAttributes(sig *sysmlv1.Element) []*sysmlv1.Element {
	var attrs []*sysmlv1.Element
	seen := map[*sysmlv1.Element]bool{}
	redefined := map[*sysmlv1.Element]bool{}
	names := map[string]bool{}
	var walk func(*sysmlv1.Element)
	walk = func(c *sysmlv1.Element) {
		if c == nil || seen[c] {
			return
		}
		seen[c] = true
		for _, p := range c.Owned("ownedAttribute") {
			name := m.nameOf(p)
			if redefined[p] || name != "" && names[name] {
				continue
			}
			attrs = append(attrs, p)
			names[name] = true
			for _, r := range m.model.Refs(p, "redefinedProperty") {
				redefined[r] = true
			}
		}
		for _, g := range c.Owned("generalization") {
			walk(m.model.Ref(g, "general"))
		}
	}
	walk(sig)
	return attrs
}

// typeRef writes the type of a feature: a ScalarValues type, a reference to a
// migrated classifier, or nothing with a note when the type is not migrated.
func (m *migration) typeRef(t, scope *sysmlv1.Element) (string, string) {
	if t == nil {
		return "", ""
	}
	if sv := m.scalarValue(t); sv != "" {
		if _, std := scalarValues[t.Name]; !std {
			return scalarValuesPrefix + sv, "the tool's " + t.Name + " datatype is written as " + scalarValuesPrefix + sv
		}
		return scalarValuesPrefix + sv, ""
	}
	if t.IsProxy() {
		return "", "type " + qualifiedName(t) + " lives outside the document and is not written"
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

// multiplicity writes a parameter's or pin's [lower..upper] multiplicity, with
// the lower bound at 0 for one declared admitting no value; see admitAbsent.
func (m *migration) multiplicity(p *sysmlv1.Element) (string, string) {
	mult, note := m.declaredMultiplicity(p)
	why, ok := m.admitsNone[p]
	if !ok || note != "" {
		return mult, note
	}
	upper := "1"
	if uv := firstOwned(p, "upperValue"); uv != nil && boundValue(uv) != "" {
		upper = boundValue(uv)
	}
	return "[0.." + upper + "]", "it is declared admitting no value: " + why
}

// declaredMultiplicity writes the [lower..upper] multiplicity v1 declares, or nothing
// for 1..1. A bound that is not a natural number (or * above) is dropped with a note.
func (m *migration) declaredMultiplicity(p *sysmlv1.Element) (string, string) {
	lower, upper := "", ""
	if lv := firstOwned(p, "lowerValue"); lv != nil {
		lower = boundValue(lv)
	}
	if uv := firstOwned(p, "upperValue"); uv != nil {
		upper = boundValue(uv)
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
func collection(p *sysmlv1.Element) string {
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

// connector writes a connector: a binding or delegation connector as `bind`,
// an assembly connector as `connect`, and an item flow it realizes as `flow`.
func (m *migration) connector(c *sysmlv1.Element) {
	if m.monteCarloConnector(c) {
		return
	}
	segs, note := m.connectorEnds(c, m.scope)
	if note != "" {
		m.unmappedConnector(c, note)
		return
	}
	paths := make([]string, len(segs))
	for i, end := range segs {
		parts := make([]string, len(end))
		for j, s := range end {
			parts[j] = writeName(m.nameFor(s))
		}
		paths[i] = strings.Join(parts, ".")
	}
	decl, kw := "connect "+paths[0]+" to "+paths[1], "connection "
	note = ""
	switch {
	case has(c, "BindingConnector"):
		decl, kw = "bind "+paths[0]+" = "+paths[1], "binding "
	case delegates(segs):
		decl, kw = "bind "+paths[0]+" = "+paths[1], "binding "
		note = "the connector delegates the owner's port to the part's, so it is written as a binding, which relays a message either way"
	}
	target := ""
	if m.nameOf(c) != "" {
		decl = kw + writeName(m.nameOf(c)) + " " + decl
		target = m.v2Name(c)
	}
	m.w.block(decl, func() { m.metadataUsages(c) })
	m.add(c, Mapped, target, note)
	m.stereotypeComments(c)
	for _, f := range m.flows[c] {
		m.itemFlow(f, c.Owned("end"), paths)
	}
}

// delegates reports whether the connector ends make a UML delegation connector:
// one end is a port of the connector's owner itself, the other a nested part's.
func delegates(segs [][]*sysmlv1.Element) bool {
	own := func(end []*sysmlv1.Element) bool { return len(end) == 1 && end[0].Type == "Port" }
	return own(segs[0]) && len(segs[1]) > 1 || own(segs[1]) && len(segs[0]) > 1
}

// unmappedConnector records a connector with no v2 form and settles the item
// flows it realizes.
func (m *migration) unmappedConnector(c *sysmlv1.Element, note string) {
	m.unmapped(c, note)
	for _, f := range m.flows[c] {
		m.flowDone(f, nil, []string{"realizing connector " + describe(c) + " is not migrated: " + note})
	}
}

// connectorEnds resolves the feature paths the two ends of connector c, owned
// by owner, name; a note says why the connector has no v2 form.
func (m *migration) connectorEnds(c, owner *sysmlv1.Element) ([][]*sysmlv1.Element, string) {
	ends := c.Owned("end")
	if len(ends) != 2 {
		return nil, fmt.Sprintf("a connector with %d ends is not migrated", len(ends))
	}
	segs := make([][]*sysmlv1.Element, len(ends))
	for i, end := range ends {
		var note string
		if segs[i], note = m.endSegments(end, owner); note != "" {
			return nil, note
		}
	}
	return segs, ""
}

// endSegments resolves the features a connector end of owner names, in path
// order, checking each is a feature of the owner or of the preceding segment's
// type where the document knows it; a note says which is not.
func (m *migration) endSegments(end, owner *sysmlv1.Element) ([]*sysmlv1.Element, string) {
	role := m.model.Ref(end, "role")
	if role == nil {
		return nil, "a connector end names no role in the document"
	}
	var segs []*sysmlv1.Element
	if nce := stereo(end, "NestedConnectorEnd"); nce != nil {
		for _, id := range nce.IDs("propertyPath") {
			p := m.model.Lookup(id)
			if p == nil {
				return nil, "the nested connector end's property path names " + id + ", which is not in the document"
			}
			segs = append(segs, p)
		}
	} else if pwp := m.model.Ref(end, "partWithPort"); pwp != nil {
		segs = append(segs, pwp)
	}
	segs = append(segs, role)
	holder := owner
	for i, s := range segs {
		if s.IsProxy() {
			return nil, "the connector end's path names " + qualifiedName(s) + ", which lives outside the document"
		}
		if holder != nil && !m.hasFeature(holder, s) {
			return nil, "the connector end's " + segmentWord(i, len(segs)) + " " + qualifiedName(s) + " is not a feature of " + qualifiedName(holder)
		}
		holder = m.model.Ref(s, "type")
		if holder != nil && holder.IsProxy() {
			holder = nil
		}
	}
	return segs, ""
}

// segmentWord names position i of a connector end's path of n segments.
func segmentWord(i, n int) string {
	if i == n-1 {
		return "role"
	}
	return "path segment"
}

// itemFlow writes an item flow realized by a connector as a flow between the
// flow properties its ends carry for each conveyed classifier, from the end
// whose role is the flow's source to the end whose role is its target.
func (m *migration) itemFlow(f *sysmlv1.Element, ends []*sysmlv1.Element, paths []string) {
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
func (m *migration) flowDone(f *sysmlv1.Element, written, notes []string) {
	o := m.outcomes[f]
	o.written = append(o.written, written...)
	o.notes = append(o.notes, notes...)
	o.pending--
	if o.pending == 0 {
		m.reportFlow(f, o)
	}
}

// reportFlow adds f's single report entry from what its connectors did.
func (m *migration) reportFlow(f *sysmlv1.Element, o *flowOutcome) {
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
	var rest []*sysmlv1.Element
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

// placeholderEnds settles the pairs ending at a node written only as a placeholder:
// each is migrated no better than its end, and the relationship no better than its pairs.
func (m *migration) placeholderEnds() {
	var rels []*sysmlv1.Element
	for d := range m.nodeEnds {
		rels = append(rels, d)
	}
	sort.Slice(rels, func(i, j int) bool { return rels[i].ID < rels[j].ID })
	for _, d := range rels {
		pl := m.nodeEnds[d]
		for _, ends := range pl.nodePairs {
			for _, end := range ends {
				if !m.placeholders[end] {
					continue
				}
				v := Approximated
				if m.verdictOf(end) == Unmapped {
					pl.written--
					pl.failed++
					if pl.written == 0 {
						v = Unmapped
					}
				}
				m.add(d, v, "", "its end "+qualifiedName(end)+" is written only as a placeholder of a node that is not migrated")
				break
			}
		}
	}
}

// verdictOf is the verdict e was reported with; Unmapped for an element not reported.
func (m *migration) verdictOf(e *sysmlv1.Element) Verdict {
	if i, ok := m.indexed[e.ID]; ok {
		return m.report.Entries[i].Verdict
	}
	return Unmapped
}

// flowProperty finds the flow property of a port's type carrying item.
func (m *migration) flowProperty(port, item *sysmlv1.Element) *sysmlv1.Element {
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
func (m *migration) informationFlow(f *sysmlv1.Element) {
	if len(m.model.Refs(f, "realizingConnector")) > 0 {
		return
	}
	m.unmapped(f, "an information flow with no realizing connector is not migrated")
}

// rule writes a constraint owned by a classifier as a constraint usage.
func (m *migration) rule(r *sysmlv1.Element) {
	spec := firstOwned(r, "specification")
	if spec == nil {
		m.unmapped(r, "the constraint has no specification")
		return
	}
	expr, ok, note := m.valueExprAs(spec, m.scope, oneOf("Boolean", "the constraint yields"))
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
type pair struct{ client, supplier *sysmlv1.Element }

// dependencyPairs expands a dependency into the client–supplier pairs that can
// be written and the number that cannot; the note says why, for those.
func (m *migration) dependencyPairs(d *sysmlv1.Element) (pairs []pair, failed int, note string) {
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

// placement is the outcome of placing a relationship: how many pairs were written and
// where, why the others could not be, and the activity nodes each written pair ends at.
type placement struct {
	written, failed int
	target          string
	notes           []string
	nodePairs       [][]*sysmlv1.Element
}

// placeDependency registers, ahead of writing, a Satisfy or Verify in the body
// of the element each pair is written in, which may precede the dependency
// itself; the dependency's report entry is written where it stands.
func (m *migration) placeDependency(d *sysmlv1.Element) {
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
			target, note, ok = m.satisfy(d, p.client, p.supplier, name)
		} else {
			target, note, ok = m.verify(d, p.client, p.supplier, name)
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
func (m *migration) placedName(scope *sysmlv1.Element, name string) (string, string) {
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
func (m *migration) dependency(d *sysmlv1.Element) {
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
		target, written, note := m.dependencyPair(d, pl, name, p.client, p.supplier)
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
	if len(pl.nodePairs) > 0 {
		m.nodeEnds[d] = pl
	}
}

// freshName returns name, or name with a numeric suffix, not yet taken in owner,
// and reserves it.
func (m *migration) freshName(owner *sysmlv1.Element, name string) string {
	base := name
	for i := 2; m.nameTaken(owner, name); i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	m.take(owner, name)
	return name
}

// relationship appends the one report entry of a relationship: mapped when every
// pair was written, approximated when some were, unmapped when none.
func (m *migration) relationship(d *sysmlv1.Element, pl *placement) {
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

// dependencyPair writes one client–supplier pair of a dependency, returning the v2
// target written, if any, whether it was written, and a note; pl keeps its node ends.
func (m *migration) dependencyPair(d *sysmlv1.Element, pl *placement, name string, client, supplier *sysmlv1.Element) (string, bool, string) {
	if has(d, "DeriveReqt") {
		target, note := m.derive(d, name, client, supplier)
		return target, target != "", note
	}
	var nodes []*sysmlv1.Element
	for _, end := range []*sysmlv1.Element{client, supplier} {
		if !m.written(end) {
			return "", false, "its end " + qualifiedName(end) + " is not migrated"
		}
		if act, _ := m.nodeGraph(end); act != nil {
			nodes = append(nodes, end)
		}
	}
	if len(nodes) > 0 {
		pl.nodePairs = append(pl.nodePairs, nodes)
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
			m.metadataUsages(d)
		})
		return target, true, ""
	case has(d, "Allocate"):
		alloc := "allocate " + from + " to " + to
		if name != "" {
			alloc = "allocation " + writeName(name) + " " + alloc
		}
		m.w.block(alloc, func() { m.metadataUsages(d) })
		return target, true, ""
	case has(d, "Trace"):
		m.w.trailed(decl, "; /* «Trace» */", "/* «Trace» */", func() { m.metadataUsages(d) })
		return target, true, "a trace is written as a plain dependency"
	case has(d, "Copy"):
		m.w.trailed(decl, "; /* «Copy» */", "/* «Copy» */", func() { m.metadataUsages(d) })
		return target, true, "a copy is written as a plain dependency; the text is not kept in step"
	}
	m.w.block(decl, func() { m.metadataUsages(d) })
	var names []string
	for _, s := range d.Stereotypes {
		if m.userStereotype(s.Definition) && !isStandard(s) {
			continue
		}
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
func (m *migration) satisfy(d, client, req *sysmlv1.Element, name string) (string, string, bool) {
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
		m.w.block(decl, func() { m.metadataUsages(d) })
	})
	return m.placedTarget(scope, name), note, true
}

// placedTarget is the report target of a Satisfy or Verify written in scope:
// the usage itself when named, else the body it was written in.
func (m *migration) placedTarget(scope *sysmlv1.Element, name string) string {
	if name == "" {
		return m.v2Name(scope)
	}
	return m.qualified(append(m.segments(scope), name))
}

// usageContext finds the body a client's satisfy is written in: the
// classifier itself, or the classifier owning a property, satisfied `by` it.
func (m *migration) usageContext(client *sysmlv1.Element) (*sysmlv1.Element, string) {
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
func (m *migration) verify(d, client, req *sysmlv1.Element, name string) (string, string, bool) {
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
		m.w.block(decl+": "+m.ref(req, client), func() { m.metadataUsages(d) })
	})
	return m.placedTarget(client, name), note, true
}

// derive writes a requirement derivation as a connection def specializing the
// library's Derivation, with the original and derived requirements as ends.
func (m *migration) derive(d *sysmlv1.Element, name string, derived, original *sysmlv1.Element) (string, string) {
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
		m.metadataUsages(d)
	})
	segs := append(m.segments(m.scope), name)
	if m.scope == nil {
		segs = []string{name}
	}
	return m.qualified(segs), ""
}

// comments writes the comments documenting e: the first as doc, the rest as
// comments, and a comment annotating other elements as `comment about`.
func (m *migration) comments(e *sysmlv1.Element) { m.writeComments(e, true) }

// writeComments writes e's comments; the first becomes doc only when e has no
// doc yet.
func (m *migration) writeComments(e *sysmlv1.Element, first bool) {
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
func commentBody(c *sysmlv1.Element) string {
	if text := commentText(c.Attrs["body"]); text != "" {
		return text
	}
	if o := firstOwned(c, "body"); o != nil {
		return commentText(strings.TrimSpace(o.Text))
	}
	return ""
}

// comment writes a comment found outside the ownedComment role.
func (m *migration) comment(c *sysmlv1.Element) {
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

// stereotypeAnnotations writes, in the body of e, what its applied stereotypes
// leave to say: its metadata usages, then its comments.
func (m *migration) stereotypeAnnotations(e *sysmlv1.Element) {
	m.metadataUsages(e)
	m.stereotypeComments(e)
}

// annotated lists the applications on e that the mapping does not consume
// whole: the tool's own markers say nothing a v2 reader needs.
func (m *migration) annotated(e *sysmlv1.Element) []*sysmlv1.Stereotype {
	var out []*sysmlv1.Stereotype
	for _, s := range e.Stereotypes {
		if m.isConstraintParameterMarker(e, s) || isSimulationConfig(s) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// metadataApplied reports whether application s is written as a metadata
// usage: a user stereotype the document defines, whose standard generals, if
// any, classify the element instead.
func (m *migration) metadataApplied(s *sysmlv1.Stereotype) bool {
	return !isStandard(s) && m.userStereotype(s.Definition)
}

// metadataUsages writes, in the body of e, the applications of user
// stereotypes as metadata usages carrying the tags no standard general consumes.
func (m *migration) metadataUsages(e *sysmlv1.Element) {
	for _, s := range m.annotated(e) {
		if !m.metadataApplied(s) {
			continue
		}
		_, consumed := m.consumes(s)
		m.metadataUsage(e, s, consumed)
	}
}

// stereotypeComments keeps as comments the tags a standard stereotype's v2
// form cannot hold, which approximate the element, and the applications of
// stereotypes the document does not define.
func (m *migration) stereotypeComments(e *sysmlv1.Element) {
	var outside []string
	byNamespace := map[string][]string{}
	for _, s := range m.annotated(e) {
		if m.metadataApplied(s) {
			continue
		}
		classifying, consumed := m.consumes(s)
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
		if s.Definition == nil && toolProfile(s.Namespace) == "" {
			if byNamespace[s.Namespace] == nil {
				outside = append(outside, s.Namespace)
			}
			byNamespace[s.Namespace] = append(byNamespace[s.Namespace], "«"+s.Name+"»")
		}
	}
	if len(outside) == 0 {
		return
	}
	var parts []string
	for _, ns := range outside {
		parts = append(parts, strings.Join(byNamespace[ns], " ")+" from "+ns)
	}
	verdict := " is applied from a profile the document does not define; the application is kept as a comment"
	if len(outside) > 1 || len(byNamespace[outside[0]]) > 1 {
		verdict = " are applied from profiles the document does not define; the applications are kept as comments"
	}
	m.note(e, strings.Join(parts, " and ")+verdict)
}

// consumes reports whether the mapping classifies an element by the standard
// stereotypes application s carries, and the tags of those it reads.
func (m *migration) consumes(s *sysmlv1.Stereotype) (classifying bool, consumed map[string]bool) {
	consumed = map[string]bool{}
	for _, n := range standardNames(s) {
		if classifyingStereotypes[n] {
			classifying = true
		}
		for k := range consumedTags[n] {
			consumed[k] = true
		}
		if isRequirementStereotype(n) {
			for k := range requirementTags {
				consumed[k] = true
			}
		}
	}
	return classifying, consumed
}

// isConstraintParameterMarker recognises MagicDraw's «ConstraintParameter» marker,
// which the `in` direction already says; a user profile's same-named stereotype is kept.
func (m *migration) isConstraintParameterMarker(e *sysmlv1.Element, s *sysmlv1.Stereotype) bool {
	if s.Name != "ConstraintParameter" || len(s.Tags) > 0 || e.Parent == nil ||
		!isMagicDrawCustomization(s.Namespace) {
		return false
	}
	cat, _ := m.classify(e.Parent)
	return cat == catConstraintDef
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
func (m *migration) unmapped(e *sysmlv1.Element, note string) {
	note = joinNotes(note, m.stereotypeSummary(e))
	m.w.lines(commentLines("not migrated: " + kindOf(e) + " " + describe(e) + " — " + note))
	m.add(e, Unmapped, "", note)
}

// stereotypeSummary lists every stereotype applied to e with its tags, so an
// element left behind keeps its metadata; "" when none is applied.
func (m *migration) stereotypeSummary(e *sysmlv1.Element) string {
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
func (m *migration) unmappedExpr(r, spec *sysmlv1.Element, note string) {
	m.w.lines(commentLines("not migrated: " + kindOf(r) + " " + describe(r) + " " + describeValue(spec) + " — " + note))
	m.add(r, Unmapped, "", note)
}

func describe(e *sysmlv1.Element) string {
	if e.Name != "" {
		return "'" + e.Name + "'"
	}
	return "(" + e.ID + ")"
}

// describeValue shows a value specification's text for a comment.
func describeValue(v *sysmlv1.Element) string {
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
