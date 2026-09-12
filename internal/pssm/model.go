package pssm

import (
	"fmt"
	"sort"
	"strings"
)

// Suite is the PSSM test suite as read: every registered test in the order the
// suite registers them, grouped by area in the suite's own area order.
type Suite struct {
	Tests []*Test
	// Signals are the protocol signals the tests' machines accept, by name.
	Signals map[string]*Signal
	// Diagnostics are problems reading the suite that are not tied to one test.
	Diagnostics []Diagnostic
}

// Problems is every diagnostic the reader recorded, the suite's own first and
// then each test's in suite order; a measurement over a suite with any is not
// a measurement of that suite.
func (s *Suite) Problems() []string {
	var out []string
	for _, d := range s.Diagnostics {
		out = append(out, d.String())
	}
	for _, t := range s.Tests {
		for _, d := range t.Diagnostics {
			out = append(out, t.Name+": "+d.String())
		}
	}
	return out
}

// Areas lists the suite's test areas in first-registration order.
func (s *Suite) Areas() []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range s.Tests {
		if !seen[t.Area] {
			seen[t.Area] = true
			out = append(out, t.Area)
		}
	}
	return out
}

// Test is one registered semantic test: its area and name as the suite
// registers them, the target class whose state machine is under test, the
// tester whose behavior stimulates it, and the traces the suite admits.
type Test struct {
	// ID is the name of the suite's SemanticTest class, e.g. "Deferred001_SemanticTest".
	ID string
	// Area is the suite's test area, e.g. "Deferred"; Name its display name, e.g. "Deferred 001".
	Area string
	Name string
	// Expected are the admissible final traces, each segment joined by "::".
	Expected []string
	// Target is the class under test and Tester the class stimulating it. A
	// standalone test has a Machine but no Target class.
	Target  *Class
	Tester  *Class
	Machine *StateMachine
	// Stimulation is the tester's behavior read as a sequence of statements.
	Stimulation *Body
	// Notes are the suite's own documentation comments on the machine and its
	// regions (stimulation sequence, expected execution, run-to-completion
	// steps). They are read for the record and never compared.
	Notes []string
	// Diagnostics are problems reading this test.
	Diagnostics []Diagnostic
}

// Diagnostic is a problem the reader found: which element, and what.
type Diagnostic struct {
	Element string
	Message string
}

func (d Diagnostic) String() string {
	if d.Element == "" {
		return d.Message
	}
	return d.Element + ": " + d.Message
}

// Class is a UML class of the suite: its attributes, operations, owned
// behaviors and classifier behavior, and its generalizations by name.
type Class struct {
	ID       string
	Name     string
	Generals []string
	// Attributes are the class's own properties.
	Attributes []Attribute
	// Operations are the class's own operations, with the method that implements each.
	Operations []*Operation
	// Behaviors are the class's owned behaviors other than the classifier behavior.
	Behaviors []*Behavior
	// ClassifierBehavior is the behavior a started instance runs: a state
	// machine (Machine set) or an activity (Activity set).
	Machine  *StateMachine
	Activity *Behavior
	// Standalone marks a class that is itself a state machine: the suite's
	// standalone tests give the machine features and operations of its own.
	Standalone bool
}

// Attribute is a typed property with an optional default.
type Attribute struct {
	Name    string
	Type    string
	Default *Literal
}

// Operation is a class operation with its parameters and method.
type Operation struct {
	ID     string
	Name   string
	Params []Param
	Method *Behavior
}

// Param is a behavior or operation parameter: its name, type and direction
// ("in", "out", "return", "inout").
type Param struct {
	Name      string
	Type      string
	Direction string
}

// Signal is a signal classifier with its attributes.
type Signal struct {
	ID         string
	Name       string
	Attributes []Attribute
}

// StateMachine is a UML state machine with its regions and connection points.
type StateMachine struct {
	ID   string
	Name string
	// Regions are the machine's top-level regions in document order.
	Regions []*Region
	// ConnectionPoints are the machine's own entry and exit points.
	ConnectionPoints []*Vertex
	// Redefines names the state machine this one redefines, or "".
	Redefines string
	// Owner is the class owning the machine, or "" for a standalone machine.
	Owner string
}

// Region is a region of a state machine or composite state.
type Region struct {
	ID          string
	Name        string
	Vertices    []*Vertex
	Transitions []*Transition
	// ExtendedRegion names the region this one extends, or "".
	ExtendedRegion string
	// Comments are the region's documentation comments.
	Comments []string
	// owner is the state owning the region; nil for a state machine's region.
	owner *Vertex
}

// Owner is the composite state owning the region, nil for a top-level region.
func (r *Region) Owner() *Vertex { return r.owner }

// VertexKind is the kind of a vertex: a state, a final state, or a pseudostate
// by its kind attribute.
type VertexKind int

// The vertex kinds the suite uses. Every UML pseudostate kind has a value so an
// unknown kind attribute is a diagnostic, not a silent default.
const (
	VertexState VertexKind = iota
	VertexFinal
	VertexInitial
	VertexJunction
	VertexChoice
	VertexFork
	VertexJoin
	VertexShallowHistory
	VertexDeepHistory
	VertexEntryPoint
	VertexExitPoint
	VertexTerminate
	VertexUnknown
)

var vertexKindNames = [...]string{
	VertexState:          "state",
	VertexFinal:          "final",
	VertexInitial:        "initial",
	VertexJunction:       "junction",
	VertexChoice:         "choice",
	VertexFork:           "fork",
	VertexJoin:           "join",
	VertexShallowHistory: "shallowHistory",
	VertexDeepHistory:    "deepHistory",
	VertexEntryPoint:     "entryPoint",
	VertexExitPoint:      "exitPoint",
	VertexTerminate:      "terminate",
	VertexUnknown:        "unknown",
}

func (k VertexKind) String() string {
	if int(k) < len(vertexKindNames) {
		return vertexKindNames[k]
	}
	return fmt.Sprintf("VertexKind(%d)", int(k))
}

// IsPseudostate reports whether the vertex kind is a pseudostate.
func (k VertexKind) IsPseudostate() bool { return k != VertexState && k != VertexFinal }

// Vertex is a state, final state or pseudostate in a region.
type Vertex struct {
	ID   string
	Name string
	Kind VertexKind
	// Region is the region owning the vertex; nil for a connection point owned
	// by a state or state machine.
	Region *Region
	// The state-only parts. Regions are the state's own regions (one for a
	// composite state, several for an orthogonal one).
	Regions          []*Region
	Entry, Exit, Do  *Behavior
	Deferred         []*Trigger
	ConnectionPoints []*Vertex
	// ConnectionPointReferences counts the state's references into a submachine.
	ConnectionPointReferences int
	// Submachine names the state machine a submachine state references, or "".
	Submachine string
	// Redefines names the state this one redefines, or "".
	Redefines string
	// Comments are the state's documentation comments.
	Comments []string
}

// Path is the state's dotted name from the top of its machine, e.g. "S1.S1.1";
// pseudostates and final states are named by their own name.
func (v *Vertex) Path() string {
	var parts []string
	for cur := v; cur != nil; {
		parts = append([]string{cur.Name}, parts...)
		if cur.Region == nil {
			break
		}
		cur = cur.Region.owner
	}
	return strings.Join(parts, ".")
}

// Describe spells the vertex as `kind name` for a diagnostic or report.
func (v *Vertex) Describe() string {
	if v == nil {
		return "<no vertex>"
	}
	if v.Name == "" {
		return v.Kind.String()
	}
	return v.Kind.String() + " " + v.Path()
}

// TransitionKind is a UML transition kind.
type TransitionKind int

// The three UML transition kinds.
const (
	TransitionExternal TransitionKind = iota
	TransitionLocal
	TransitionInternal
)

func (k TransitionKind) String() string {
	switch k {
	case TransitionExternal:
		return "external"
	case TransitionLocal:
		return "local"
	case TransitionInternal:
		return "internal"
	}
	return fmt.Sprintf("TransitionKind(%d)", int(k))
}

// Transition is a UML transition with its triggers, guard and effect.
type Transition struct {
	ID       string
	Name     string
	Kind     TransitionKind
	Source   *Vertex
	Target   *Vertex
	Triggers []*Trigger
	Guard    *Guard
	Effect   *Behavior
	// Redefines names the transition this one redefines, or "".
	Redefines string
	// Region is the region owning the transition.
	Region *Region
}

// Describe spells the transition for a diagnostic or report.
func (t *Transition) Describe() string {
	if t == nil {
		return "<no transition>"
	}
	var b strings.Builder
	if t.Kind != TransitionExternal {
		b.WriteString(t.Kind.String() + " ")
	}
	b.WriteString("transition")
	if t.Name != "" {
		b.WriteString(" " + t.Name)
	}
	fmt.Fprintf(&b, " %s -> %s", t.Source.Describe(), t.Target.Describe())
	for i, trig := range t.Triggers {
		if i == 0 {
			b.WriteString(" on ")
		} else {
			b.WriteString(", ")
		}
		if trig == nil {
			b.WriteString("<no trigger>")
			continue
		}
		b.WriteString(trig.Event.Describe())
	}
	if t.Guard != nil {
		b.WriteString(" [" + t.Guard.Describe() + "]")
	}
	if t.Redefines != "" {
		b.WriteString(" redefines " + t.Redefines)
	}
	return b.String()
}

// Trigger is a transition or deferrable trigger with its event.
type Trigger struct {
	Name  string
	Event *Event
}

// EventKind is the kind of a trigger's event.
type EventKind int

// The event kinds the reader distinguishes; EventOther carries the xmi:type.
const (
	EventSignal EventKind = iota
	EventCall
	EventOther
)

// Event is the event a trigger names: a signal event with its signal, a call
// event with its operation, or another event type named by Type.
type Event struct {
	Kind      EventKind
	Signal    *Signal
	Operation *Operation
	Type      string
}

// Describe names the event for a diagnostic or report.
func (e *Event) Describe() string {
	switch {
	case e == nil:
		return "<no event>"
	case e.Kind == EventSignal && e.Signal != nil:
		return e.Signal.Name
	case e.Kind == EventCall && e.Operation != nil:
		return e.Operation.Name + "()"
	}
	return e.Type
}

// GuardKind is the shape of a transition guard's specification.
type GuardKind int

// The guard shapes the suite uses.
const (
	GuardLiteral GuardKind = iota // a LiteralBoolean
	GuardElse                     // an Expression whose symbol is "else"
	GuardOpaque                   // an OpaqueExpression with a body in some language
	GuardOther                    // any other specification, named by Type
)

// Guard is a transition guard: its literal value, its opaque text, and the
// behavior the opaque expression is compiled to, when any.
type Guard struct {
	Name     string
	Kind     GuardKind
	Literal  bool
	Opaque   *OpaqueText
	Behavior *Behavior
	Type     string
}

// Describe spells the guard for a diagnostic or report.
func (g *Guard) Describe() string {
	switch {
	case g == nil:
		return ""
	case g.Kind == GuardLiteral:
		return fmt.Sprint(g.Literal)
	case g.Kind == GuardElse:
		return "else"
	case g.Kind == GuardOpaque:
		return fmt.Sprintf("%s: %s", g.Opaque.Language, g.Opaque.Body)
	}
	return g.Type
}

// Behavior is a UML behavior: an activity whose graph is read into Body, or an
// opaque behavior whose text is Body's single opaque statement.
type Behavior struct {
	ID     string
	Name   string
	Type   string
	Params []Param
	// Body is the activity's content as statements; nil for an opaque behavior.
	Body *Body
	// Opaque holds an OpaqueBehavior's body and language.
	Opaque *OpaqueText
	// Source is the Alf text the suite records for the behavior in a comment,
	// when it does; it is documentation for a reader, not what is translated.
	Source string
}

// OpaqueText is the body of an opaque behavior or expression in its language.
type OpaqueText struct {
	Language string
	Body     string
}

// Body is an activity read as a sequence of statements in control-flow order,
// with every node the reading could not express listed under Unsupported.
type Body struct {
	Statements  []Statement
	Unsupported []string
}

// Empty reports whether the body has neither statements nor unsupported nodes.
func (b *Body) Empty() bool { return b == nil || (len(b.Statements) == 0 && len(b.Unsupported) == 0) }

// Statement is one step of a behavior. Exactly one kind applies.
type Statement struct {
	Kind StatementKind
	// Call and Send: the operation, behavior or signal named and its arguments
	// in parameter order; Receiver is the object addressed, nil for a behavior.
	Name     string
	Args     []Expr
	Receiver *Expr
	// Accept: the events waited for, and Result the name the accepted
	// occurrence is bound to when the body reads it ("" otherwise).
	Events []*Event
	Result string
	// Assign: the feature written on Receiver and the Value written; Replace
	// distinguishes `x = v` from `x->add(v)`. Return: the Value returned.
	Feature string
	Value   *Expr
	Replace bool
}

// StatementKind is the kind of a Statement.
type StatementKind int

// The statement kinds an activity reading produces.
const (
	StmtCall   StatementKind = iota // Receiver.Name(Args...)
	StmtSend                        // send Name(Args...) to Receiver
	StmtAccept                      // accept one of Events
	StmtAssign                      // Receiver.Feature := Value
	StmtReturn                      // return Value
	StmtStart                       // start Receiver's classifier behavior
)

func (s Statement) String() string {
	switch s.Kind {
	case StmtCall:
		if s.Receiver == nil {
			return fmt.Sprintf("%s(%s)", s.Name, exprList(s.Args))
		}
		return fmt.Sprintf("%s.%s(%s)", s.Receiver, s.Name, exprList(s.Args))
	case StmtSend:
		return fmt.Sprintf("send %s(%s) to %s", s.Name, exprList(s.Args), s.Receiver)
	case StmtAccept:
		names := make([]string, len(s.Events))
		for i, e := range s.Events {
			names[i] = e.Describe()
		}
		if s.Result != "" {
			return fmt.Sprintf("accept %s as %s", strings.Join(names, "|"), s.Result)
		}
		return "accept " + strings.Join(names, "|")
	case StmtAssign:
		op := ":="
		if !s.Replace {
			op = "+="
		}
		return fmt.Sprintf("%s.%s %s %s", s.Receiver, s.Feature, op, s.Value)
	case StmtReturn:
		return fmt.Sprintf("return %s", s.Value)
	case StmtStart:
		return fmt.Sprintf("start %s", s.Receiver)
	}
	return fmt.Sprintf("Statement(%d)", int(s.Kind))
}

func exprList(args []Expr) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.String()
	}
	return strings.Join(parts, ", ")
}

// Expr is a value an activity computes: what flows into a pin.
type Expr struct {
	Kind ExprKind
	// Literal: the literal value.
	Literal *Literal
	// Param: the parameter's name. Event: the accept result it reads. Read:
	// the feature read on Object.
	Name   string
	Object *Expr
	// Apply: the behavior applied, by its library name (Concat, ToString, Not,
	// ...), and its arguments in parameter order. Call: the operation called on
	// Object, with its result used as a value. New: the classifier instantiated, by Name.
	Args []Expr
	// Unknown: what the reader could not follow, for the diagnostic.
	Text string
}

// ExprKind is the kind of an Expr.
type ExprKind int

// The expression kinds.
const (
	ExprLiteral ExprKind = iota
	ExprSelf
	ExprParam
	ExprEvent
	ExprRead
	ExprApply
	ExprCall
	ExprNew
	ExprUnknown
)

func (e Expr) String() string {
	switch e.Kind {
	case ExprLiteral:
		return e.Literal.String()
	case ExprSelf:
		return "this"
	case ExprParam, ExprEvent:
		return e.Name
	case ExprRead:
		return e.Object.String() + "." + e.Name
	case ExprApply:
		return e.Name + "(" + exprList(e.Args) + ")"
	case ExprCall:
		return e.Object.String() + "." + e.Name + "(" + exprList(e.Args) + ")"
	case ExprNew:
		return "new " + e.Name
	case ExprUnknown:
		return "?(" + e.Text + ")"
	}
	return fmt.Sprintf("Expr(%d)", int(e.Kind))
}

// Literal is a UML literal specification: its kind and its text as written.
type Literal struct {
	Kind LiteralKind
	// Text is the value attribute as written; Present is false for a literal
	// without one (whose value is the kind's default).
	Text    string
	Present bool
}

// LiteralKind is the UML literal kind.
type LiteralKind int

// The UML literal kinds.
const (
	LiteralString LiteralKind = iota
	LiteralBoolean
	LiteralInteger
	LiteralUnlimitedNatural
	LiteralReal
	LiteralNull
)

func (l *Literal) String() string {
	if l == nil {
		return "<nil>"
	}
	switch l.Kind {
	case LiteralString:
		return fmt.Sprintf("%q", l.Text)
	case LiteralBoolean:
		if !l.Present {
			return "false"
		}
		return l.Text
	case LiteralInteger, LiteralReal, LiteralUnlimitedNatural:
		if !l.Present {
			return "0"
		}
		return l.Text
	case LiteralNull:
		return "null"
	}
	return l.Text
}

// Bool is the value of a boolean literal (false when absent, as UML defaults).
func (l *Literal) Bool() bool {
	return l != nil && l.Kind == LiteralBoolean && l.Present && l.Text == "true"
}

// Constructs are the UML constructs a state machine uses, by the names the
// classifier and the baseline record report.
type Constructs map[string]bool

// Sorted lists the constructs in order.
func (c Constructs) Sorted() []string {
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
