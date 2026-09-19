package fuml

import "fmt"

// Model is one of the suite's UML models as read from its Eclipse UML2 XMI: every
// activity it declares, with the classes, signals and associations they use.
type Model struct {
	// File is the model's file name in the suite, TestsFile or ExceptionTestsFile.
	File string
	// Name is the uml:Model's name.
	Name string
	// Activities lists every uml:Activity the file declares, packaged or owned by
	// a class, in document order.
	Activities []*Activity
	// Classes, Signals and Associations are the packaged classifiers, in document order.
	Classes      []*Class
	Signals      []*Signal
	Associations []*Association
	// Diagnostics records what the reader could not make sense of.
	Diagnostics []string

	activities map[string]*Activity
	classes    map[string]*Class
	signals    map[string]*Signal
}

// Exception reports whether this is the exception-test model.
func (m *Model) Exception() bool { return m.File == ExceptionTestsFile }

// Activity returns the activity with the given XMI id, or nil.
func (m *Model) Activity(id string) *Activity {
	return m.activities[id]
}

// Class returns the class with the given XMI id, or nil.
func (m *Model) Class(id string) *Class {
	return m.classes[id]
}

// ClassOf returns the class a type reference names, by ID first and then by
// name (a reference may carry either), or nil when it names none. An external
// reference names an element of another document, never one of the model's.
func (m *Model) ClassOf(t TypeRef) *Class {
	if m == nil || t.Zero() || t.External {
		return nil
	}
	if c := m.classes[t.ID]; c != nil {
		return c
	}
	var found *Class
	for _, c := range m.Classes {
		if c.Name != t.Name || t.Name == "" {
			continue
		}
		if found != nil {
			return nil
		}
		found = c
	}
	return found
}

// SignalOf returns the signal a type reference names, by ID first and then by
// name, or nil when it names none, the name is ambiguous or it is external.
func (m *Model) SignalOf(t TypeRef) *Signal {
	if m == nil || t.Zero() || t.External {
		return nil
	}
	if s := m.signals[t.ID]; s != nil {
		return s
	}
	var found *Signal
	for _, s := range m.Signals {
		if s.Name != t.Name || t.Name == "" {
			continue
		}
		if found != nil {
			return nil
		}
		found = s
	}
	return found
}

// AllAttributes returns the class's attributes with those it inherits, each
// once, generals before the classes specializing them; a property redefined by
// another of them is replaced by it.
func (c *Class) AllAttributes() []*Property {
	seen := map[*Class]bool{}
	var out []*Property
	var visit func(c *Class)
	visit = func(c *Class) {
		if c == nil || seen[c] {
			return
		}
		seen[c] = true
		for _, g := range c.Generals {
			visit(c.Model.ClassOf(g))
		}
		out = append(out, c.Attributes...)
	}
	visit(c)
	return effectiveProperties(out)
}

// effectiveProperties drops from props every property another of them redefines,
// directly or through a chain of redefinitions, keeping the order of the rest.
func effectiveProperties(props []*Property) []*Property {
	redefined := map[*Property]bool{}
	var mark func(p *Property)
	mark = func(p *Property) {
		for _, r := range p.Redefines {
			if r != nil && !redefined[r] {
				redefined[r] = true
				mark(r)
			}
		}
	}
	for _, p := range props {
		mark(p)
	}
	if len(redefined) == 0 {
		return props
	}
	out := make([]*Property, 0, len(props))
	for _, p := range props {
		if !redefined[p] {
			out = append(out, p)
		}
	}
	return out
}

// ActivityNamed returns the activity with the given name, or nil when none or
// more than one carry it.
func (m *Model) ActivityNamed(name string) *Activity {
	var found *Activity
	for _, a := range m.Activities {
		if a.Name != name {
			continue
		}
		if found != nil {
			return nil
		}
		found = a
	}
	return found
}

// Activity is one uml:Activity: a behavior with parameters, nodes and edges.
type Activity struct {
	ID   string
	Name string
	// Model is the model declaring the activity.
	Model *Model
	// Owner is the class whose ownedBehavior the activity is, or nil when it is
	// packaged directly in the model.
	Owner *Class
	// Active is isActive: the activity is itself an active class.
	Active bool
	// Parameters are the ownedParameters, in document order.
	Parameters []*Parameter
	// Attributes are ownedAttributes of the activity itself.
	Attributes []*Property
	// Nodes are the activity's own nodes, in document order; the contents of a
	// structured node and the pins of an action hang under their owner.
	Nodes []*Node
	// Edges are the activity's own edges, in document order.
	Edges []*Edge
	// Line is the element's line in the XMI.
	Line int
}

// AllNodes lists every node of the activity, pins and structured contents
// included, in document order.
func (a *Activity) AllNodes() []*Node {
	var all []*Node
	var visit func([]*Node)
	visit = func(nodes []*Node) {
		for _, n := range nodes {
			all = append(all, n)
			visit(n.Pins)
			visit(n.Nodes)
		}
	}
	visit(a.Nodes)
	return all
}

// Node returns the node with the given XMI id, at any depth, or nil.
func (a *Activity) Node(id string) *Node {
	for _, n := range a.AllNodes() {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// NodeNamed returns the action node with the given name, or nil when none or
// more than one carry it. Pins are not actions and are not considered.
func (a *Activity) NodeNamed(name string) *Node {
	var found *Node
	for _, n := range a.AllNodes() {
		if n.Name != name || n.Kind.Pin() {
			continue
		}
		if found != nil {
			return nil
		}
		found = n
	}
	return found
}

// Direction is a parameter's direction.
type Direction string

// The UML parameter directions.
const (
	In     Direction = "in"
	InOut  Direction = "inout"
	Out    Direction = "out"
	Return Direction = "return"
)

// Multiplicity is a MultiplicityElement's bounds and ordering.
type Multiplicity struct {
	Lower int
	// Upper is the upper bound, or Unbounded.
	Upper   int
	Ordered bool
	Unique  bool
}

// Unbounded is the Upper of a `*` multiplicity.
const Unbounded = -1

func (m Multiplicity) String() string {
	upper := "*"
	if m.Upper != Unbounded {
		upper = fmt.Sprint(m.Upper)
	}
	s := fmt.Sprintf("[%d..%s]", m.Lower, upper)
	if m.Ordered {
		s += " ordered"
	}
	if !m.Unique {
		s += " nonunique"
	}
	return s
}

// Parameter is one ownedParameter of an activity or operation.
type Parameter struct {
	ID        string
	Name      string
	Direction Direction
	Type      TypeRef
	Multiplicity
}

// TypeRef names a type: one declared in the model (Class, Signal, DataType) or
// referenced by href from the UML primitive types or the fUML library.
type TypeRef struct {
	// ID is the element's XMI id, or the href's fragment for an external type.
	ID string
	// Name is the element's name; for an external type its qualified name when
	// the library it lives in was read, else the href's fragment.
	Name string
	// Kind is the referenced element's UML type, e.g. "PrimitiveType", "Class".
	Kind string
	// External is set for a type outside the model.
	External bool
}

// Zero reports an absent type.
func (t TypeRef) Zero() bool { return t.ID == "" && t.Name == "" }

func (t TypeRef) String() string {
	if t.Zero() {
		return "untyped"
	}
	return t.Name
}

// NodeKind is a node's UML metaclass without the `uml:` prefix.
type NodeKind string

// The node kinds the corpus uses, by family.
const (
	// Pins.
	InputPin  NodeKind = "InputPin"
	OutputPin NodeKind = "OutputPin"
	// Object nodes.
	ActivityParameterNode NodeKind = "ActivityParameterNode"
	CentralBufferNode     NodeKind = "CentralBufferNode"
	DataStoreNode         NodeKind = "DataStoreNode"
	// Control nodes.
	InitialNode       NodeKind = "InitialNode"
	ActivityFinalNode NodeKind = "ActivityFinalNode"
	FlowFinalNode     NodeKind = "FlowFinalNode"
	ForkNode          NodeKind = "ForkNode"
	JoinNode          NodeKind = "JoinNode"
	MergeNode         NodeKind = "MergeNode"
	DecisionNode      NodeKind = "DecisionNode"
	// Structured nodes.
	StructuredActivityNode NodeKind = "StructuredActivityNode"
	// Actions.
	ValueSpecificationAction           NodeKind = "ValueSpecificationAction"
	CallBehaviorAction                 NodeKind = "CallBehaviorAction"
	CallOperationAction                NodeKind = "CallOperationAction"
	CreateObjectAction                 NodeKind = "CreateObjectAction"
	DestroyObjectAction                NodeKind = "DestroyObjectAction"
	ReadSelfAction                     NodeKind = "ReadSelfAction"
	ReadStructuralFeatureAction        NodeKind = "ReadStructuralFeatureAction"
	AddStructuralFeatureValueAction    NodeKind = "AddStructuralFeatureValueAction"
	RemoveStructuralFeatureValueAction NodeKind = "RemoveStructuralFeatureValueAction"
	ClearStructuralFeatureAction       NodeKind = "ClearStructuralFeatureAction"
	ReadExtentAction                   NodeKind = "ReadExtentAction"
	ReadIsClassifiedObjectAction       NodeKind = "ReadIsClassifiedObjectAction"
	ReclassifyObjectAction             NodeKind = "ReclassifyObjectAction"
	TestIdentityAction                 NodeKind = "TestIdentityAction"
	ReadLinkAction                     NodeKind = "ReadLinkAction"
	UnmarshallAction                   NodeKind = "UnmarshallAction"
	SendSignalAction                   NodeKind = "SendSignalAction"
	AcceptEventAction                  NodeKind = "AcceptEventAction"
	AcceptCallAction                   NodeKind = "AcceptCallAction"
	ReplyAction                        NodeKind = "ReplyAction"
	StartObjectBehaviorAction          NodeKind = "StartObjectBehaviorAction"
	RaiseExceptionAction               NodeKind = "RaiseExceptionAction"
)

// Pin reports whether the kind is a pin.
func (k NodeKind) Pin() bool { return k == InputPin || k == OutputPin }

// Control reports whether the kind is a control node.
func (k NodeKind) Control() bool {
	switch k {
	case InitialNode, ActivityFinalNode, FlowFinalNode, ForkNode, JoinNode, MergeNode, DecisionNode:
		return true
	}
	return false
}

// Object reports whether the kind is an object node other than a pin.
func (k NodeKind) Object() bool {
	return k == ActivityParameterNode || k == CentralBufferNode || k == DataStoreNode
}

// Action reports whether the kind is an action, structured nodes included.
func (k NodeKind) Action() bool { return !k.Pin() && !k.Control() && !k.Object() }

// Node is one ActivityNode: a control node, an object node, a pin or an action.
type Node struct {
	ID   string
	Name string
	Kind NodeKind
	// Activity is the activity the node belongs to, however deeply nested.
	Activity *Activity
	// Owner is the action a pin belongs to, or the structured node containing
	// the node; nil for the activity's own nodes.
	Owner *Node
	// Role is the XMI tag a pin or nested node was read from: `argument`,
	// `result`, `object`, `value`, `target`, `first`, `second`, `node`, ...
	Role string
	// Type is the node's type where UML gives it one (pins, object nodes).
	Type TypeRef
	// Multiplicity is a pin's; zero for other nodes.
	Multiplicity Multiplicity
	// Incoming and Outgoing are the edges ending and starting at the node.
	Incoming, Outgoing []*Edge
	// Pins are an action's input and output pins, in document order.
	Pins []*Node
	// Nodes are a structured node's contents, in document order.
	Nodes []*Node
	// Parameter is the parameter an ActivityParameterNode stands for.
	Parameter *Parameter
	// Behavior is the behavior a CallBehaviorAction calls.
	Behavior *BehaviorRef
	// Operation is the operation a CallOperationAction calls, or an
	// AcceptCallAction's call trigger names.
	Operation *Operation
	// Value is a ValueSpecificationAction's value.
	Value *Value
	// Feature is the structural feature an Add/Remove/ReadStructuralFeature action
	// touches.
	Feature *Property
	// Classifier is the class a CreateObject, ReadExtent, ReadIsClassifiedObject
	// or Unmarshall action names; NewClassifiers and OldClassifiers are a
	// ReclassifyObjectAction's.
	Classifier                     TypeRef
	NewClassifiers, OldClassifiers []TypeRef
	// Signal is the signal a SendSignalAction sends.
	Signal TypeRef
	// Triggers are an AcceptEventAction's or AcceptCallAction's.
	Triggers []Trigger
	// Ends are a ReadLinkAction's link end data, in document order.
	Ends []LinkEnd
	// Handlers are the exception handlers protecting the node.
	Handlers []*Handler
	// DecisionInputFlow is a DecisionNode's decision input flow, if any.
	DecisionInputFlow *Edge
	// ReplaceAll, RemoveDuplicates, DestroyLinks, DestroyOwnedObjects, Unmarshall
	// and Synchronous are the actions' like-named flags.
	ReplaceAll, RemoveDuplicates, DestroyLinks, DestroyOwnedObjects, Unmarshall, Synchronous bool
	// Line is the element's line in the XMI.
	Line int
}

// Inputs and Outputs list an action's pins by direction.
func (n *Node) Inputs() []*Node  { return n.pins(InputPin) }
func (n *Node) Outputs() []*Node { return n.pins(OutputPin) }

func (n *Node) pins(kind NodeKind) []*Node {
	var pins []*Node
	for _, p := range n.Pins {
		if p.Kind == kind {
			pins = append(pins, p)
		}
	}
	return pins
}

// Label names the node for a report: its name, else its kind and line.
func (n *Node) Label() string {
	if n.Name != "" {
		return n.Name
	}
	return fmt.Sprintf("%s (line %d)", n.Kind, n.Line)
}

// BehaviorRef is the behavior a CallBehaviorAction calls.
type BehaviorRef struct {
	// ID is the behavior's XMI id, or the href fragment of a library behavior.
	ID string
	// Name is the behavior's name; a library behavior's is qualified when the
	// library was read, e.g. `PrimitiveBehaviors::IntegerFunctions::+`.
	Name string
	// Kind is the behavior's UML type: "Activity", "FunctionBehavior", ...
	Kind string
	// Activity is the called activity when it is declared in the same model.
	Activity *Activity
	// External is set for a behavior outside the model (the fUML library).
	External bool
}

// Value is a ValueSpecification: a literal, or an instance value's referent.
type Value struct {
	// Kind is the specification's UML type without prefix: "LiteralInteger",
	// "LiteralBoolean", "LiteralString", "LiteralReal", "LiteralUnlimitedNatural",
	// "LiteralNull", "InstanceValue".
	Kind string
	// Text is the `value` attribute as written; Given is false when it was
	// omitted, which UML reads as the literal's default (0, false, null).
	Text  string
	Given bool
	// Type is the value's declared type, if any.
	Type TypeRef
}

func (v *Value) String() string {
	if v == nil {
		return "no value"
	}
	if !v.Given {
		return v.Kind + " (default)"
	}
	return v.Kind + " " + v.Text
}

// EdgeKind is an ActivityEdge's UML metaclass.
type EdgeKind string

// The two edge kinds.
const (
	ControlFlow EdgeKind = "ControlFlow"
	ObjectFlow  EdgeKind = "ObjectFlow"
)

// Edge is one ControlFlow or ObjectFlow between two nodes.
type Edge struct {
	ID   string
	Name string
	Kind EdgeKind
	// Activity is the activity owning the edge.
	Activity *Activity
	Source   *Node
	Target   *Node
	// Guard and Weight are the edge's value specifications, if any.
	Guard  *Value
	Weight *Value
	// Line is the element's line in the XMI.
	Line int
}

// Trigger is one trigger of an accept action: a signal event or a call event.
type Trigger struct {
	ID string
	// Signal is set for a SignalEvent, Operation for a CallEvent.
	Signal    TypeRef
	Operation *Operation
}

// LinkEnd is one LinkEndData of a ReadLinkAction.
type LinkEnd struct {
	// End is the association end.
	End *Property
	// Value is the pin supplying the end's value, nil for the open end.
	Value *Node
}

// Handler is an ExceptionHandler.
type Handler struct {
	ID string
	// Protected is the node the handler protects; Body the node handling it.
	Protected *Node
	Body      *Node
	// Types are the exception types handled.
	Types []TypeRef
}

// Class is a uml:Class declared by the model.
type Class struct {
	ID     string
	Name   string
	Model  *Model
	Active bool
	// Generals are the classes it generalizes.
	Generals []TypeRef
	// Attributes are its ownedAttributes, association ends it owns included.
	Attributes []*Property
	// Operations are its ownedOperations.
	Operations []*Operation
	// Behaviors are its ownedBehaviors; ClassifierBehavior the one it runs when
	// started, if any.
	Behaviors          []*Activity
	ClassifierBehavior *Activity
	// Line is the element's line in the XMI.
	Line int
}

// Signal is a uml:Signal declared by the model.
type Signal struct {
	ID    string
	Name  string
	Model *Model
	// Generals are the signals it generalizes.
	Generals   []TypeRef
	Attributes []*Property
	Line       int
}

// AllAttributes returns the signal's attributes with those it inherits, each
// once, generals before the signals specializing them, a redefined one replaced
// by its redefinition: the order a SendSignalAction's argument pins follow.
func (s *Signal) AllAttributes() []*Property {
	seen := map[*Signal]bool{}
	var out []*Property
	var visit func(s *Signal)
	visit = func(s *Signal) {
		if s == nil || seen[s] {
			return
		}
		seen[s] = true
		for _, g := range s.Generals {
			visit(s.Model.SignalOf(g))
		}
		out = append(out, s.Attributes...)
	}
	visit(s)
	return effectiveProperties(out)
}

// Association is a uml:Association declared by the model.
type Association struct {
	ID   string
	Name string
	// Ends are the member ends in memberEnd order.
	Ends []*Property
	Line int
}

// Property is an attribute of a class, signal or activity, or an association end.
type Property struct {
	ID   string
	Name string
	Type TypeRef
	Multiplicity
	// Owner names the class, signal, activity or association owning it.
	Owner TypeRef
	// Redefines are the inherited properties this one redefines, which it
	// replaces among its owner's effective attributes.
	Redefines []*Property
	// Association is set for an association end (owned by either side).
	Association *Association
	// Composite is set for a composite aggregation end.
	Composite bool
}

// Operation is an ownedOperation of a class.
type Operation struct {
	ID         string
	Name       string
	Owner      *Class
	Parameters []*Parameter
	// Methods are the behaviors implementing it.
	Methods []*Activity
}
