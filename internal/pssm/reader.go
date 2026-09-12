package pssm

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The names the suite's shared architecture gives its base classes and the
// features its registration activities write.
const (
	baseTarget       = "Target"
	baseTester       = "Tester"
	baseSemanticTest = "SemanticTest"
	featureName      = "name"
	featureExpected  = "expectedTraces"
	primitiveTypes   = "http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#"
)

// registrationActivity matches the name of a package's test registration
// activity (BehaviorTests, DeferredTests, ...).
var registrationActivity = regexp.MustCompile(`^[A-Za-z]+Tests$`)

// alfStatement matches the numbered statement nodes of an Alf-compiled body.
var alfStatement = regexp.MustCompile(`^(\d+):`)

// reader builds a Suite from a Document, resolving every reference through the
// document's id index rather than through XML nesting: the suite's transitions
// cross regions, and its redefining machines reference vertices of the machines
// they extend.
type reader struct {
	doc       *Document
	suite     *Suite
	classes   map[string]*Class // by xmi:id
	machines  map[string]*StateMachine
	regions   map[string]*Region
	vertices  map[string]*Vertex
	signals   map[string]*Signal
	ops       map[string]*Operation
	behaviors map[string]*Behavior
	events    map[string]*Event
}

// ReadFile parses and reads a PSSM suite file.
func ReadFile(path string) (*Suite, error) {
	// #nosec G304 -- the path is the operator-selected suite root.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}

// Read parses an XMI document and reads the suite it contains. Malformed XML
// is an error; anything the reader cannot interpret inside a well-formed
// document is a diagnostic on the suite or the test concerned.
func Read(src io.Reader) (*Suite, error) {
	doc, err := Parse(src)
	if err != nil {
		return nil, err
	}
	return ReadDocument(doc)
}

// ReadDocument reads the suite a parsed document contains.
func ReadDocument(doc *Document) (*Suite, error) {
	r := &reader{
		doc:       doc,
		suite:     &Suite{Signals: make(map[string]*Signal)},
		classes:   make(map[string]*Class),
		machines:  make(map[string]*StateMachine),
		regions:   make(map[string]*Region),
		vertices:  make(map[string]*Vertex),
		signals:   make(map[string]*Signal),
		ops:       make(map[string]*Operation),
		behaviors: make(map[string]*Behavior),
		events:    make(map[string]*Event),
	}
	if doc.Root == nil {
		return nil, fmt.Errorf("pssm: document has no root")
	}
	r.readSignals()
	r.readOperations()
	r.readMachines()
	r.readTransitions()
	r.readClasses()
	r.readRegistrations()
	return r.suite, nil
}

func (r *reader) diag(e *Element, format string, args ...any) {
	r.suite.Diagnostics = append(r.suite.Diagnostics, Diagnostic{Element: e.Describe(), Message: fmt.Sprintf(format, args...)})
}

// typed lists every element of the given xmi:type in document order.
func (r *reader) typed(types ...string) []*Element {
	var out []*Element
	r.doc.Root.Walk(func(e *Element) bool {
		for _, t := range types {
			if e.Type == t {
				out = append(out, e)
				break
			}
		}
		return true
	})
	return out
}

func (r *reader) readSignals() {
	for _, e := range r.typed("uml:Signal") {
		sig := &Signal{ID: e.ID, Name: e.Name(), Attributes: r.readAttributes(e)}
		r.signals[e.ID] = sig
		if prior, dup := r.suite.Signals[sig.Name]; dup {
			r.diag(e, "signal %q is also declared as %s", sig.Name, prior.ID)
			continue
		}
		r.suite.Signals[sig.Name] = sig
	}
}

func (r *reader) readAttributes(owner *Element) []Attribute {
	var out []Attribute
	for _, p := range owner.Tagged("ownedAttribute") {
		if p.Type != "uml:Property" && p.Type != "" {
			continue
		}
		if p.Attr("association") != "" {
			continue
		}
		a := Attribute{Name: p.Name(), Type: r.typeName(p)}
		if dv := p.First("defaultValue"); dv != nil {
			lit, diag := readLiteral(dv)
			if diag != "" {
				r.diag(p, "default value: %s", diag)
			}
			a.Default = lit
		}
		out = append(out, a)
	}
	return out
}

// typeName names a typed element's type: a primitive by the fragment of its
// href, a document element by its name.
func (r *reader) typeName(e *Element) string {
	if t := e.First("type"); t != nil {
		if href := t.Href(); href != "" {
			if strings.HasPrefix(href, primitiveTypes) {
				return strings.TrimPrefix(href, primitiveTypes)
			}
			return href[strings.LastIndex(href, "#")+1:]
		}
		if id := t.Attr("idref"); id != "" {
			if target := r.doc.ByID(id); target != nil {
				return target.Name()
			}
			return id
		}
	}
	if id := e.Attr("type"); id != "" {
		if target := r.doc.ByID(id); target != nil {
			return target.Name()
		}
		return id
	}
	return ""
}

func (r *reader) readParams(owner *Element) []Param {
	var out []Param
	for _, p := range owner.Tagged("ownedParameter") {
		dir := p.Attr("direction")
		if dir == "" {
			dir = "in"
		}
		out = append(out, Param{Name: p.Name(), Type: r.typeName(p), Direction: dir})
	}
	return out
}

// readOperations reads every operation before any behavior, so call actions
// and call events resolve wherever they appear.
func (r *reader) readOperations() {
	for _, e := range r.typed("uml:Operation") {
		r.ops[e.ID] = &Operation{ID: e.ID, Name: e.Name(), Params: r.readParams(e)}
	}
	for _, e := range r.typed("uml:Operation") {
		if id := e.Ref("method"); id != "" {
			if m := r.doc.ByID(id); m != nil {
				r.ops[e.ID].Method = r.behavior(m)
			} else {
				r.diag(e, "method %s is not in the document", id)
			}
		}
	}
}

// behavior reads a behavior element once and memoizes it.
func (r *reader) behavior(e *Element) *Behavior {
	if e == nil {
		return nil
	}
	if b, ok := r.behaviors[e.ID]; ok {
		return b
	}
	b := &Behavior{ID: e.ID, Name: e.Name(), Type: e.Type, Params: r.readParams(e)}
	r.behaviors[e.ID] = b
	for _, c := range e.Tagged("ownedComment") {
		if body := c.Attr("body"); strings.Contains(body, "activity ") || strings.HasPrefix(body, "namespace ") {
			b.Source = strings.ReplaceAll(body, "\r\n", "\n")
			break
		}
	}
	switch e.Type {
	case "uml:Activity":
		b.Body = r.readActivity(e)
	case "uml:OpaqueBehavior":
		b.Opaque = readOpaque(e)
	case "uml:StateMachine":
		// A submachine or classifier behavior; read under readMachines.
	default:
		r.diag(e, "behavior kind %s is not read", e.Type)
	}
	return b
}

// readOpaque reads the body/language pairs of an opaque behavior or
// expression; the suite writes one body per element.
func readOpaque(e *Element) *OpaqueText {
	bodies := e.Tagged("body")
	langs := e.Tagged("language")
	if len(bodies) == 0 {
		return &OpaqueText{}
	}
	out := &OpaqueText{Body: strings.TrimSpace(bodies[0].Text)}
	if len(langs) > 0 {
		out.Language = strings.TrimSpace(langs[0].Text)
	}
	return out
}

// readEvent resolves a trigger's event reference.
func (r *reader) readEvent(id string, at *Element) *Event {
	if ev, ok := r.events[id]; ok {
		return ev
	}
	e := r.doc.ByID(id)
	if e == nil {
		r.diag(at, "event %s is not in the document", id)
		ev := &Event{Kind: EventOther, Type: "unresolved " + id}
		r.events[id] = ev
		return ev
	}
	ev := &Event{Kind: EventOther, Type: e.Type}
	switch e.Type {
	case "uml:SignalEvent":
		ev.Kind = EventSignal
		ev.Signal = r.signals[e.Attr("signal")]
		if ev.Signal == nil {
			r.diag(e, "signal %s is not in the document", e.Attr("signal"))
			ev.Kind, ev.Type = EventOther, "signal event without a signal"
		}
	case "uml:CallEvent":
		ev.Kind = EventCall
		ev.Operation = r.ops[e.Attr("operation")]
		if ev.Operation == nil {
			r.diag(e, "operation %s is not in the document", e.Attr("operation"))
			ev.Kind, ev.Type = EventOther, "call event without an operation"
		}
	}
	r.events[id] = ev
	return ev
}

// readMachines builds every state machine's regions and vertices, indexing
// each vertex by id so transitions can be resolved afterwards.
func (r *reader) readMachines() {
	for _, e := range r.typed("uml:StateMachine") {
		sm := &StateMachine{ID: e.ID, Name: e.Name()}
		if e.Parent != nil && e.Parent.Type == "uml:Class" {
			sm.Owner = e.Parent.Name()
		}
		if id := e.Ref("redefinedBehavior"); id != "" {
			sm.Redefines = r.nameOf(id)
		} else if id := e.Ref("redefinedClassifier"); id != "" {
			sm.Redefines = r.nameOf(id)
		} else if id := e.Ref("extendedStateMachine"); id != "" {
			sm.Redefines = r.nameOf(id)
		}
		for _, cp := range e.Tagged("connectionPoint") {
			sm.ConnectionPoints = append(sm.ConnectionPoints, r.readVertex(cp, nil))
		}
		for _, re := range e.Tagged("region") {
			sm.Regions = append(sm.Regions, r.readRegion(re, nil))
		}
		r.machines[e.ID] = sm
		r.behaviors[e.ID] = &Behavior{ID: e.ID, Name: e.Name(), Type: e.Type}
	}
}

func (r *reader) nameOf(id string) string {
	if e := r.doc.ByID(id); e != nil {
		return e.Name()
	}
	return id
}

func comments(e *Element) []string {
	var out []string
	for _, c := range e.Tagged("ownedComment") {
		if body := c.Attr("body"); body != "" {
			out = append(out, strings.ReplaceAll(body, "\r\n", "\n"))
		}
	}
	return out
}

func (r *reader) readRegion(e *Element, owner *Vertex) *Region {
	reg := &Region{ID: e.ID, Name: e.Name(), owner: owner, Comments: comments(e)}
	if id := e.Ref("extendedRegion"); id != "" {
		reg.ExtendedRegion = r.nameOf(id)
	}
	r.regions[e.ID] = reg
	for _, v := range e.Tagged("subvertex") {
		reg.Vertices = append(reg.Vertices, r.readVertex(v, reg))
	}
	return reg
}

var pseudostateKinds = map[string]VertexKind{
	"":               VertexInitial,
	"initial":        VertexInitial,
	"junction":       VertexJunction,
	"choice":         VertexChoice,
	"fork":           VertexFork,
	"join":           VertexJoin,
	"shallowHistory": VertexShallowHistory,
	"deepHistory":    VertexDeepHistory,
	"entryPoint":     VertexEntryPoint,
	"exitPoint":      VertexExitPoint,
	"terminate":      VertexTerminate,
}

func (r *reader) readVertex(e *Element, region *Region) *Vertex {
	v := &Vertex{ID: e.ID, Name: e.Name(), Region: region, Comments: comments(e)}
	switch e.Type {
	case "uml:State":
		v.Kind = VertexState
		for _, re := range e.Tagged("region") {
			v.Regions = append(v.Regions, r.readRegion(re, v))
		}
		v.Entry = r.behavior(e.First("entry"))
		v.Exit = r.behavior(e.First("exit"))
		v.Do = r.behavior(e.First("doActivity"))
		for _, t := range e.Tagged("deferrableTrigger") {
			v.Deferred = append(v.Deferred, &Trigger{Name: t.Name(), Event: r.readEvent(t.Attr("event"), t)})
		}
		for _, cp := range e.Tagged("connectionPoint") {
			v.ConnectionPoints = append(v.ConnectionPoints, r.readVertex(cp, nil))
		}
		v.ConnectionPointReferences = len(e.Tagged("connection"))
		if id := e.Ref("submachine"); id != "" {
			v.Submachine = r.nameOf(id)
		}
		if id := e.Ref("redefinedState"); id != "" {
			v.Redefines = r.nameOf(id)
		} else if id := e.Ref("redefinedVertex"); id != "" {
			v.Redefines = r.nameOf(id)
		}
	case "uml:FinalState":
		v.Kind = VertexFinal
	case "uml:Pseudostate":
		kind, ok := pseudostateKinds[e.Attr("kind")]
		if !ok {
			r.diag(e, "pseudostate kind %q is not a UML kind", e.Attr("kind"))
			kind = VertexUnknown
		}
		v.Kind = kind
	default:
		r.diag(e, "vertex kind %s is not read", e.Type)
		v.Kind = VertexUnknown
	}
	if prior, dup := r.vertices[e.ID]; dup {
		r.diag(e, "vertex id is also %s", prior.Name)
	}
	r.vertices[e.ID] = v
	return v
}

// readTransitions reads every region's transitions once all vertices exist.
func (r *reader) readTransitions() {
	for _, e := range r.typed("uml:Transition") {
		var region *Region
		if e.Parent != nil {
			region = r.regions[e.Parent.ID]
		}
		if region == nil {
			r.diag(e, "is not owned by a region that was read")
			continue
		}
		t := &Transition{ID: e.ID, Name: e.Name(), Region: region}
		switch e.Attr("kind") {
		case "", "external":
			t.Kind = TransitionExternal
		case "local":
			t.Kind = TransitionLocal
		case "internal":
			t.Kind = TransitionInternal
		default:
			r.diag(e, "transition kind %q is not a UML kind", e.Attr("kind"))
		}
		t.Source = r.vertices[e.Attr("source")]
		t.Target = r.vertices[e.Attr("target")]
		if t.Source == nil {
			r.diag(e, "source %s is not a vertex", e.Attr("source"))
		}
		if t.Target == nil && e.Attr("target") != "" {
			r.diag(e, "target %s is not a vertex", e.Attr("target"))
		}
		for _, trig := range e.Tagged("trigger") {
			t.Triggers = append(t.Triggers, &Trigger{Name: trig.Name(), Event: r.readEvent(trig.Attr("event"), trig)})
		}
		t.Guard = r.readGuard(e)
		t.Effect = r.behavior(e.First("effect"))
		if id := e.Ref("redefinedTransition"); id != "" {
			t.Redefines = r.nameOf(id)
		}
		region.Transitions = append(region.Transitions, t)
	}
}

// readGuard reads a transition's guard constraint, whichever way it is written.
func (r *reader) readGuard(t *Element) *Guard {
	id := t.Ref("guard")
	if id == "" {
		return nil
	}
	c := r.doc.ByID(id)
	if c == nil {
		r.diag(t, "guard %s is not in the document", id)
		return &Guard{Kind: GuardOther, Type: "unresolved"}
	}
	spec := c.First("specification")
	g := &Guard{Name: c.Name(), Kind: GuardOther}
	if spec == nil {
		r.diag(c, "guard has no specification")
		g.Type = "no specification"
		return g
	}
	g.Type = spec.Type
	switch spec.Type {
	case "uml:LiteralBoolean":
		g.Kind = GuardLiteral
		g.Literal = spec.Attr("value") == "true"
	case "uml:Expression":
		if spec.Attr("symbol") == "else" && len(spec.Tagged("operand")) == 0 {
			g.Kind = GuardElse
		}
	case "uml:OpaqueExpression":
		g.Kind = GuardOpaque
		g.Opaque = readOpaque(spec)
		if bid := spec.Ref("behavior"); bid != "" {
			if b := r.doc.ByID(bid); b != nil {
				g.Behavior = r.behavior(b)
			} else {
				r.diag(spec, "behavior %s is not in the document", bid)
			}
		}
	}
	return g
}

// readClasses reads every class and standalone state machine that specializes
// one of the suite's architecture bases.
func (r *reader) readClasses() {
	for _, e := range r.typed("uml:Class", "uml:StateMachine") {
		if e.Type == "uml:StateMachine" && len(e.Tagged("generalization")) == 0 {
			continue
		}
		c := &Class{ID: e.ID, Name: e.Name(), Attributes: r.readAttributes(e), Standalone: e.Type == "uml:StateMachine"}
		for _, g := range e.Tagged("generalization") {
			c.Generals = append(c.Generals, r.nameOf(g.Attr("general")))
		}
		for _, op := range e.Tagged("ownedOperation") {
			if o := r.ops[op.ID]; o != nil {
				c.Operations = append(c.Operations, o)
			}
		}
		cb := e.Attr("classifierBehavior")
		if c.Standalone {
			c.Machine = r.machines[e.ID]
		}
		for _, b := range e.Tagged("ownedBehavior") {
			if b.ID == cb {
				if sm := r.machines[b.ID]; sm != nil {
					c.Machine = sm
				} else {
					c.Activity = r.behavior(b)
				}
				continue
			}
			c.Behaviors = append(c.Behaviors, r.behavior(b))
		}
		if cb != "" && c.Machine == nil && c.Activity == nil {
			r.diag(e, "classifier behavior %s is not an owned behavior", cb)
		}
		r.classes[e.ID] = c
	}
}

// specializes reports whether a class generalizes (transitively) a class of
// the given name.
func (r *reader) specializes(c *Class, base string) bool {
	seen := map[string]bool{}
	var walk func(*Class) bool
	walk = func(cur *Class) bool {
		if seen[cur.ID] {
			return false
		}
		seen[cur.ID] = true
		for _, g := range cur.Generals {
			if g == base {
				return true
			}
			for _, other := range r.classes {
				if other.Name == g && walk(other) {
					return true
				}
			}
		}
		return false
	}
	return walk(c)
}

// readRegistrations reads the per-area registration activities: each creates
// the area's semantic tests and writes their names and expected traces.
func (r *reader) readRegistrations() {
	byName := make(map[string]*Class, len(r.classes))
	for _, c := range r.classes {
		byName[c.Name] = c
	}
	for _, act := range r.typed("uml:Activity") {
		if !registrationActivity.MatchString(act.Name()) || act.Parent == nil || act.Parent.Type != "uml:Package" {
			continue
		}
		body := act.First("node")
		if body == nil {
			continue
		}
		var statements []*Element
		for _, n := range body.Tagged("node") {
			if alfStatement.MatchString(n.Name()) {
				statements = append(statements, n)
			}
		}
		sort.SliceStable(statements, func(i, j int) bool {
			return statementIndex(statements[i]) < statementIndex(statements[j])
		})
		// Statements creating a test leave a fork node named after the local
		// variable; the writes that follow are fed from it.
		created := make(map[string]*Test) // by the id of any element inside the creating statement
		var order []*Test
		for _, st := range statements {
			var test *Test
			st.Walk(func(e *Element) bool {
				if e.Type != "uml:CreateObjectAction" {
					return true
				}
				cls := r.classes[e.Attr("classifier")]
				if cls == nil || !r.specializes(cls, baseSemanticTest) {
					return true
				}
				test = &Test{ID: cls.Name, Area: act.Parent.Name()}
				return false
			})
			if test == nil {
				continue
			}
			st.Walk(func(e *Element) bool {
				if e.ID != "" {
					created[e.ID] = test
				}
				return true
			})
			order = append(order, test)
		}
		for _, st := range statements {
			var write *Element
			st.Walk(func(e *Element) bool {
				if e.Type == "uml:AddStructuralFeatureValueAction" {
					write = e
					return false
				}
				return true
			})
			if write == nil {
				continue
			}
			feature := r.nameOf(write.Attr("structuralFeature"))
			if feature != featureName && feature != featureExpected {
				continue
			}
			test := r.testFedInto(st, created)
			if test == nil {
				r.diag(st, "writes %s of no test created in %s", feature, act.Name())
				continue
			}
			var literals []string
			st.Walk(func(e *Element) bool {
				if e.Type == "uml:LiteralString" {
					literals = append(literals, e.Attr("value"))
				}
				return true
			})
			if len(literals) != 1 {
				r.diag(st, "writes %s with %d string literals, not one", feature, len(literals))
				continue
			}
			if feature == featureName {
				test.Name = literals[0]
			} else {
				test.Expected = append(test.Expected, literals[0])
			}
		}
		for _, test := range order {
			r.completeTest(test, byName)
			r.suite.Tests = append(r.suite.Tests, test)
		}
	}
}

func statementIndex(e *Element) int {
	m := alfStatement.FindStringSubmatch(e.Name())
	n, _ := strconv.Atoi(m[1])
	return n
}

// testFedInto finds the test whose creating statement feeds an object flow into
// the given statement.
func (r *reader) testFedInto(st *Element, created map[string]*Test) *Test {
	var found *Test
	body := st.Parent
	for _, edge := range body.Tagged("edge") {
		src, ok := created[edge.Attr("source")]
		if !ok {
			continue
		}
		target := r.doc.ByID(edge.Attr("target"))
		for cur := target; cur != nil; cur = cur.Parent {
			if cur == st {
				if found != nil && found != src {
					return nil
				}
				found = src
				break
			}
		}
	}
	return found
}

// completeTest attaches the target, tester and machine of a registered test:
// the classes in the semantic test's package specializing Target and Tester.
func (r *reader) completeTest(t *Test, byName map[string]*Class) {
	sem := byName[t.ID]
	if sem == nil {
		t.Diagnostics = append(t.Diagnostics, Diagnostic{Message: "semantic test class is not in the document"})
		return
	}
	if t.Name == "" {
		t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: sem.Name, Message: "no name is registered"})
		t.Name = t.ID
	}
	if len(t.Expected) == 0 {
		t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: sem.Name, Message: "no expected trace is registered"})
	}
	pkg := r.doc.ByID(sem.ID).Parent
	if pkg == nil {
		t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: sem.Name, Message: "is not in a package"})
		return
	}
	for _, e := range pkg.Tagged("packagedElement") {
		c := r.classes[e.ID]
		if c == nil {
			continue
		}
		switch {
		case r.specializes(c, baseTarget):
			if t.Target != nil {
				t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: c.Name, Message: "second Target in the package, after " + t.Target.Name})
			}
			t.Target = c
		case r.specializes(c, baseTester):
			if t.Tester != nil {
				t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: c.Name, Message: "second Tester in the package, after " + t.Tester.Name})
			}
			t.Tester = c
		}
	}
	if t.Target == nil {
		t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: pkg.Name(), Message: "no class specializing Target"})
	} else {
		t.Machine = t.Target.Machine
		if t.Machine == nil {
			t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: t.Target.Name, Message: "classifier behavior is not a state machine"})
		} else {
			t.Notes = machineNotes(t.Machine)
		}
	}
	if t.Tester == nil {
		t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: pkg.Name(), Message: "no class specializing Tester"})
	} else if t.Tester.Activity != nil {
		t.Stimulation = t.Tester.Activity.Body
	} else {
		t.Diagnostics = append(t.Diagnostics, Diagnostic{Element: t.Tester.Name, Message: "classifier behavior is not an activity"})
	}
}

// machineNotes collects the documentation comments on a machine's regions and
// states, top down.
func machineNotes(sm *StateMachine) []string {
	var out []string
	var walk func([]*Region)
	walk = func(regions []*Region) {
		for _, reg := range regions {
			out = append(out, reg.Comments...)
			for _, v := range reg.Vertices {
				out = append(out, v.Comments...)
				walk(v.Regions)
			}
		}
	}
	walk(sm.Regions)
	return out
}
