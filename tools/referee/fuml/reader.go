package fuml

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
)

// primitiveTypes prefixes the hrefs of UML's primitive types.
const primitiveTypes = "pathmap://UML_LIBRARIES/UMLPrimitiveTypes.library.uml#"

// Library is the fUML foundational model library, read so that hrefs into it
// resolve to qualified names and element kinds.
type Library struct {
	doc *xmi.Document
}

// ReadLibraryFile parses fUML_Library.xmi.
func ReadLibraryFile(path string) (*Library, error) {
	f, err := os.Open(path) // #nosec G304 -- the path is the operator-selected suite root
	if err != nil {
		return nil, err
	}
	defer f.Close()
	doc, err := xmi.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return &Library{doc: doc}, nil
}

// Resolve names the library element with the given id: its qualified name
// below the library model and its UML type, or ok=false when absent.
func (l *Library) Resolve(id string) (name, kind string, ok bool) {
	if l == nil {
		return "", "", false
	}
	e := l.doc.ByID(id)
	if e == nil {
		return "", "", false
	}
	var parts []string
	for at := e; at != nil && at.Type != "uml:Model"; at = at.Parent {
		if n := at.Name(); n != "" {
			parts = append([]string{n}, parts...)
		}
	}
	return strings.Join(parts, "::"), strings.TrimPrefix(e.Type, "uml:"), true
}

// reader builds a Model from a parsed document in three passes: classifiers,
// then activities with their nodes, then the edges and cross-references that
// need every node to exist.
type reader struct {
	doc     *xmi.Document
	lib     *Library
	model   *Model
	assocs  map[string]*Association
	nodes   map[string]*Node
	edges   map[string]*Edge
	params  map[string]*Parameter
	props   map[string]*Property
	ops     map[string]*Operation
	pending []func()
}

// ReadModelFile parses and reads one of the suite's model files. The library
// may be nil, in which case library references keep their href fragments.
func ReadModelFile(path string, lib *Library) (*Model, error) {
	f, err := os.Open(path) // #nosec G304 -- the path is the operator-selected suite root
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m, err := ReadModel(f, lib)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	m.File = filepath.Base(path)
	return m, nil
}

// ReadModel parses an XMI document and reads the model it contains. Malformed
// XML is an error; anything the reader cannot interpret inside a well-formed
// document is a diagnostic on the model.
func ReadModel(src io.Reader, lib *Library) (*Model, error) {
	doc, err := xmi.Parse(src)
	if err != nil {
		return nil, err
	}
	if doc.Root == nil {
		return nil, fmt.Errorf("fuml: document has no root")
	}
	r := &reader{
		doc:    doc,
		lib:    lib,
		model:  &Model{activities: map[string]*Activity{}, classes: map[string]*Class{}},
		assocs: map[string]*Association{},
		nodes:  map[string]*Node{},
		edges:  map[string]*Edge{},
		params: map[string]*Parameter{},
		props:  map[string]*Property{},
		ops:    map[string]*Operation{},
	}
	root := doc.Root
	if root.Tag != "Model" {
		if root = root.First("Model"); root == nil {
			return nil, fmt.Errorf("fuml: document root is %s, not a uml:Model", doc.Root.Describe())
		}
	}
	r.model.Name = root.Name()
	r.readClassifiers(root)
	r.readActivities(root)
	for _, fn := range r.pending {
		fn()
	}
	return r.model, nil
}

func (r *reader) diag(e *xmi.Element, format string, args ...any) {
	r.model.Diagnostics = append(r.model.Diagnostics, e.Describe()+": "+fmt.Sprintf(format, args...))
}

// later defers a resolution until every element has been created.
func (r *reader) later(fn func()) { r.pending = append(r.pending, fn) }

// typed lists every element of the given xmi:type under root, in document
// order, skipping cross-document references.
func (r *reader) typed(root *xmi.Element, types ...string) []*xmi.Element {
	var out []*xmi.Element
	root.Walk(func(e *xmi.Element) bool {
		if e.Href() != "" {
			return true
		}
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

func (r *reader) readClassifiers(root *xmi.Element) {
	for _, e := range r.typed(root, "uml:Association") {
		a := &Association{ID: e.ID, Name: e.Name(), Line: e.Line}
		r.model.Associations = append(r.model.Associations, a)
		r.assocs[e.ID] = a
		owner := TypeRef{ID: e.ID, Name: e.Name(), Kind: "Association"}
		for _, end := range e.Tagged("ownedEnd") {
			p := r.readProperty(end, owner)
			p.Association = a
		}
		r.later(func() {
			for _, id := range e.Refs("memberEnd") {
				p := r.props[id]
				if p == nil {
					r.diag(e, "member end %s is not a property of this model", id)
					continue
				}
				p.Association = a
				a.Ends = append(a.Ends, p)
			}
		})
	}
	for _, e := range r.typed(root, "uml:Signal") {
		s := &Signal{ID: e.ID, Name: e.Name(), Line: e.Line}
		r.model.Signals = append(r.model.Signals, s)
		owner := TypeRef{ID: e.ID, Name: e.Name(), Kind: "Signal"}
		for _, g := range e.Tagged("generalization") {
			s.Generals = append(s.Generals, r.typeRef(g, "general"))
		}
		for _, a := range e.Tagged("ownedAttribute") {
			s.Attributes = append(s.Attributes, r.readProperty(a, owner))
		}
	}
	for _, e := range r.typed(root, "uml:Class") {
		c := &Class{ID: e.ID, Name: e.Name(), Model: r.model, Active: e.Attr("isActive") == "true", Line: e.Line}
		r.model.Classes = append(r.model.Classes, c)
		r.model.classes[e.ID] = c
		owner := TypeRef{ID: e.ID, Name: e.Name(), Kind: "Class"}
		for _, g := range e.Tagged("generalization") {
			c.Generals = append(c.Generals, r.typeRef(g, "general"))
		}
		for _, a := range e.Tagged("ownedAttribute") {
			c.Attributes = append(c.Attributes, r.readProperty(a, owner))
		}
		r.readOperations(e, c)
	}
}

// readOperations reads a class's or an activity's ownedOperations; an activity
// is a class in UML and may own the operations its accept-call actions serve.
func (r *reader) readOperations(e *xmi.Element, owner *Class) {
	for _, o := range e.Tagged("ownedOperation") {
		op := &Operation{ID: o.ID, Name: o.Name(), Owner: owner}
		for _, p := range o.Tagged("ownedParameter") {
			op.Parameters = append(op.Parameters, r.readParameter(p))
		}
		if owner != nil {
			owner.Operations = append(owner.Operations, op)
		}
		r.ops[o.ID] = op
		r.later(func() {
			for _, id := range o.Refs("method") {
				if a := r.model.Activity(id); a != nil {
					op.Methods = append(op.Methods, a)
				} else {
					r.diag(o, "method %s is not an activity of this model", id)
				}
			}
		})
	}
}

func (r *reader) readProperty(e *xmi.Element, owner TypeRef) *Property {
	p := &Property{
		ID:           e.ID,
		Name:         e.Name(),
		Type:         r.typeRef(e, "type"),
		Multiplicity: r.readMultiplicity(e),
		Owner:        owner,
		Composite:    e.Attr("aggregation") == "composite",
	}
	r.props[e.ID] = p
	if id := e.Attr("association"); id != "" {
		r.later(func() {
			if a := r.assocs[id]; a != nil {
				p.Association = a
			} else {
				r.diag(e, "association %s is not declared in this model", id)
			}
		})
	}
	return p
}

// readMultiplicity reads lowerValue/upperValue and the ordering flags, with
// UML's defaults of [1..1] unordered unique.
func (r *reader) readMultiplicity(e *xmi.Element) Multiplicity {
	m := Multiplicity{Lower: 1, Upper: 1, Unique: e.Attr("isUnique") != "false", Ordered: e.Attr("isOrdered") == "true"}
	if lv := e.First("lowerValue"); lv != nil {
		m.Lower = r.bound(lv, 0)
	}
	if uv := e.First("upperValue"); uv != nil {
		m.Upper = r.bound(uv, 1)
	}
	return m
}

// bound reads a multiplicity bound: an absent value is the literal's default,
// `*` is Unbounded.
func (r *reader) bound(e *xmi.Element, absent int) int {
	text, given := e.Attrs["value"]
	if !given {
		if e.Type == "uml:LiteralUnlimitedNatural" {
			return 0
		}
		return absent
	}
	if text == "*" {
		return Unbounded
	}
	n, err := strconv.Atoi(text)
	if err != nil {
		r.diag(e, "multiplicity bound %q is not a number", text)
		return absent
	}
	return n
}

func (r *reader) readParameter(e *xmi.Element) *Parameter {
	p := &Parameter{
		ID:           e.ID,
		Name:         e.Name(),
		Direction:    Direction(e.Attr("direction")),
		Type:         r.typeRef(e, "type"),
		Multiplicity: r.readMultiplicity(e),
	}
	if p.Direction == "" {
		p.Direction = In
	}
	r.params[e.ID] = p
	return p
}

// typeRef resolves the reference `name` on e: an attribute holding an id of
// this model, or a child element carrying an href into another document.
func (r *reader) typeRef(e *xmi.Element, name string) TypeRef {
	if id := e.Attr(name); id != "" {
		return r.localRef(e, id)
	}
	child := e.First(name)
	if child == nil {
		return TypeRef{}
	}
	if href := child.Href(); href != "" {
		return r.externalRef(child, href)
	}
	if id := child.Attr("idref"); id != "" {
		return r.localRef(e, id)
	}
	return TypeRef{}
}

func (r *reader) localRef(e *xmi.Element, id string) TypeRef {
	target := r.doc.ByID(id)
	if target == nil {
		r.diag(e, "reference %s names no element of this model", id)
		return TypeRef{ID: id, Name: id}
	}
	return TypeRef{ID: id, Name: target.Name(), Kind: strings.TrimPrefix(target.Type, "uml:")}
}

// externalRef names an href target: a UML primitive type by its fragment, a
// library element by its qualified name when the library was read.
func (r *reader) externalRef(e *xmi.Element, href string) TypeRef {
	frag := href[strings.LastIndex(href, "#")+1:]
	ref := TypeRef{ID: frag, Name: frag, Kind: strings.TrimPrefix(e.Type, "uml:"), External: true}
	if strings.HasPrefix(href, primitiveTypes) {
		return ref
	}
	if name, kind, ok := r.lib.Resolve(frag); ok {
		ref.Name, ref.Kind = name, kind
	} else if r.lib != nil {
		r.diag(e, "href %s names no element of the library", href)
	}
	return ref
}

func (r *reader) readActivities(root *xmi.Element) {
	for _, e := range r.typed(root, "uml:Activity") {
		a := &Activity{ID: e.ID, Name: e.Name(), Model: r.model, Active: e.Attr("isActive") == "true", Line: e.Line}
		r.model.Activities = append(r.model.Activities, a)
		r.model.activities[e.ID] = a
		if e.Parent != nil && e.Tag == "ownedBehavior" {
			if c := r.model.classes[e.Parent.ID]; c != nil {
				a.Owner = c
				c.Behaviors = append(c.Behaviors, a)
				if e.Parent.Attr("classifierBehavior") == e.ID {
					c.ClassifierBehavior = a
				}
			} else {
				r.diag(e, "owned by %s, which is not a class of this model", e.Parent.Describe())
			}
		}
		for _, p := range e.Tagged("ownedParameter") {
			a.Parameters = append(a.Parameters, r.readParameter(p))
		}
		owner := TypeRef{ID: e.ID, Name: e.Name(), Kind: "Activity"}
		for _, p := range e.Tagged("ownedAttribute") {
			a.Attributes = append(a.Attributes, r.readProperty(p, owner))
		}
		r.readOperations(e, nil)
		for _, child := range e.Children {
			switch child.Tag {
			case "node", "structuredNode", "group":
				if n := r.readNode(child, a, nil); n != nil {
					a.Nodes = append(a.Nodes, n)
				}
			case "edge":
				if edge := r.readEdge(child, a); edge != nil {
					a.Edges = append(a.Edges, edge)
				}
			}
		}
	}
}

// pinRoles are the tags under which actions own their pins.
var pinRoles = map[string]bool{
	"argument": true, "result": true, "object": true, "value": true, "target": true,
	"first": true, "second": true, "inputValue": true, "insertAt": true, "removeAt": true,
	"returnInformation": true, "replyValue": true, "exception": true, "request": true,
	"structuredNodeInput": true, "structuredNodeOutput": true,
}

func (r *reader) readNode(e *xmi.Element, a *Activity, owner *Node) *Node {
	if e.Type == "" || !strings.HasPrefix(e.Type, "uml:") {
		r.diag(e, "node without a uml type")
		return nil
	}
	n := &Node{
		ID:                  e.ID,
		Name:                e.Name(),
		Kind:                NodeKind(strings.TrimPrefix(e.Type, "uml:")),
		Activity:            a,
		Owner:               owner,
		Role:                e.Tag,
		Type:                r.typeRef(e, "type"),
		ReplaceAll:          e.Attr("isReplaceAll") == "true",
		RemoveDuplicates:    e.Attr("isRemoveDuplicates") == "true",
		DestroyLinks:        e.Attr("isDestroyLinks") == "true",
		DestroyOwnedObjects: e.Attr("isDestroyOwnedObjects") == "true",
		Unmarshall:          e.Attr("isUnmarshall") == "true",
		Synchronous:         e.Attr("isSynchronous") != "false",
		Line:                e.Line,
	}
	if n.Kind.Pin() {
		n.Multiplicity = r.readMultiplicity(e)
	}
	r.nodes[e.ID] = n
	for _, child := range e.Children {
		switch {
		case pinRoles[child.Tag] && strings.HasSuffix(child.Type, "Pin"):
			if p := r.readNode(child, a, n); p != nil {
				n.Pins = append(n.Pins, p)
			}
		case child.Tag == "node" || child.Tag == "structuredNode":
			if c := r.readNode(child, a, n); c != nil {
				n.Nodes = append(n.Nodes, c)
			}
		case child.Tag == "edge":
			if edge := r.readEdge(child, a); edge != nil {
				a.Edges = append(a.Edges, edge)
			}
		case child.Tag == "value" && n.Kind == ValueSpecificationAction:
			n.Value = r.readValue(child)
		case child.Tag == "trigger":
			r.readTrigger(child, n)
		case child.Tag == "endData":
			r.readLinkEnd(child, n)
		case child.Tag == "handler":
			r.readHandler(child, n)
		}
	}
	r.readNodeRefs(e, n)
	return n
}

// readNodeRefs resolves the node's references to parameters, behaviors,
// features, classifiers, signals and flows once everything exists.
func (r *reader) readNodeRefs(e *xmi.Element, n *Node) {
	switch n.Kind {
	case ActivityParameterNode:
		r.later(func() {
			id := e.Attr("parameter")
			if n.Parameter = r.params[id]; n.Parameter == nil {
				r.diag(e, "parameter %s is not a parameter of this model", id)
			}
		})
	case CallBehaviorAction:
		n.Behavior = r.readBehaviorRef(e)
	case CallOperationAction:
		r.later(func() { n.Operation = r.operation(e, e.Attr("operation")) })
	case CreateObjectAction, ReadExtentAction, ReadIsClassifiedObjectAction:
		n.Classifier = r.typeRef(e, "classifier")
	case UnmarshallAction:
		n.Classifier = r.typeRef(e, "unmarshallType")
	case ReclassifyObjectAction:
		for _, id := range e.Refs("newClassifier") {
			n.NewClassifiers = append(n.NewClassifiers, r.localRef(e, id))
		}
		for _, id := range e.Refs("oldClassifier") {
			n.OldClassifiers = append(n.OldClassifiers, r.localRef(e, id))
		}
	case SendSignalAction:
		n.Signal = r.typeRef(e, "signal")
	case ReadStructuralFeatureAction, AddStructuralFeatureValueAction, RemoveStructuralFeatureValueAction:
		r.later(func() {
			id := e.Attr("structuralFeature")
			if n.Feature = r.props[id]; n.Feature == nil {
				r.diag(e, "structural feature %s is not a property of this model", id)
			}
		})
	case DecisionNode:
		if id := e.Attr("decisionInputFlow"); id != "" {
			r.later(func() {
				if n.DecisionInputFlow = r.edges[id]; n.DecisionInputFlow == nil {
					r.diag(e, "decision input flow %s is not an edge of this model", id)
				}
			})
		}
	}
}

func (r *reader) readBehaviorRef(e *xmi.Element) *BehaviorRef {
	if id := e.Attr("behavior"); id != "" {
		ref := r.localRef(e, id)
		b := &BehaviorRef{ID: id, Name: ref.Name, Kind: ref.Kind}
		r.later(func() { b.Activity = r.model.Activity(id) })
		return b
	}
	child := e.First("behavior")
	if child == nil {
		r.diag(e, "call behavior action names no behavior")
		return nil
	}
	if href := child.Href(); href != "" {
		ref := r.externalRef(child, href)
		return &BehaviorRef{ID: ref.ID, Name: ref.Name, Kind: ref.Kind, External: true}
	}
	r.diag(e, "call behavior action's behavior is neither an id nor an href")
	return nil
}

func (r *reader) operation(e *xmi.Element, id string) *Operation {
	op := r.ops[id]
	if op == nil {
		r.diag(e, "operation %s is not an operation of this model", id)
	}
	return op
}

func (r *reader) readValue(e *xmi.Element) *Value {
	text, given := e.Attrs["value"]
	v := &Value{Kind: strings.TrimPrefix(e.Type, "uml:"), Text: text, Given: given, Type: r.typeRef(e, "type")}
	if e.Type == "uml:InstanceValue" {
		v.Type = r.typeRef(e, "instance")
		v.Text, v.Given = v.Type.Name, true
	}
	return v
}

// readTrigger appends the trigger to n; a call event's operation resolves
// once every operation exists, in place in n.Triggers.
func (r *reader) readTrigger(e *xmi.Element, n *Node) {
	t := Trigger{ID: e.ID}
	id := e.Attr("event")
	event := r.doc.ByID(id)
	if event == nil {
		r.diag(e, "event %s is not declared in this model", id)
	} else {
		switch event.Type {
		case "uml:SignalEvent":
			t.Signal = r.typeRef(event, "signal")
		case "uml:CallEvent":
			at := len(n.Triggers)
			r.later(func() { n.Triggers[at].Operation = r.operation(event, event.Attr("operation")) })
		default:
			r.diag(e, "event %s is a %s, neither a signal nor a call event", id, event.Type)
		}
	}
	n.Triggers = append(n.Triggers, t)
}

func (r *reader) readLinkEnd(e *xmi.Element, n *Node) {
	r.later(func() {
		end := LinkEnd{End: r.props[e.Attr("end")]}
		if end.End == nil {
			r.diag(e, "end %s is not a property of this model", e.Attr("end"))
		}
		if id := e.Attr("value"); id != "" {
			if end.Value = r.nodes[id]; end.Value == nil {
				r.diag(e, "value %s is not a pin of this model", id)
			}
		}
		n.Ends = append(n.Ends, end)
	})
}

func (r *reader) readHandler(e *xmi.Element, n *Node) {
	h := &Handler{ID: e.ID, Protected: n}
	n.Handlers = append(n.Handlers, h)
	for _, id := range e.Refs("exceptionType") {
		h.Types = append(h.Types, r.localRef(e, id))
	}
	r.later(func() {
		if id := e.Attr("handlerBody"); id != "" {
			if h.Body = r.nodes[id]; h.Body == nil {
				r.diag(e, "handler body %s is not a node of this model", id)
			}
		}
	})
}

func (r *reader) readEdge(e *xmi.Element, a *Activity) *Edge {
	kind := EdgeKind(strings.TrimPrefix(e.Type, "uml:"))
	if kind != ControlFlow && kind != ObjectFlow {
		r.diag(e, "edge of unknown kind")
		return nil
	}
	edge := &Edge{ID: e.ID, Name: e.Name(), Kind: kind, Activity: a, Line: e.Line}
	if g := e.First("guard"); g != nil {
		edge.Guard = r.readValue(g)
	}
	if w := e.First("weight"); w != nil {
		edge.Weight = r.readValue(w)
	}
	r.edges[e.ID] = edge
	r.later(func() {
		edge.Source = r.endpoint(e, "source")
		edge.Target = r.endpoint(e, "target")
		if edge.Source != nil {
			edge.Source.Outgoing = append(edge.Source.Outgoing, edge)
		}
		if edge.Target != nil {
			edge.Target.Incoming = append(edge.Target.Incoming, edge)
		}
	})
	return edge
}

func (r *reader) endpoint(e *xmi.Element, which string) *Node {
	id := e.Attr(which)
	n := r.nodes[id]
	if n == nil {
		r.diag(e, "%s %s is not a node of this model", which, id)
	}
	return n
}
