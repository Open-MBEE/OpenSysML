package ast

import "github.com/Open-MBEE/OpenSysML/internal/core/source"

// Behavioral AST nodes for SysML v2 actions and state machines.
// These nodes implement the Node interface and populate Usage.Members
// for action and state usages (UsageAction, UsageState).

// InitialNode is the entry point for action execution.
type InitialNode struct {
	NodeBase
	Name      string         // optional identifier for edge referencing
	NameSpan  source.Span    // span of Name, empty when none is written
	Successor *QualifiedName // optional target for implicit succession (from `first X then Y` syntax)
	Guard     Node           // optional guard condition for succession
	// Members are the members of the body the succession was written with
	// (`first start then continue { … }`), and HasBody that it was written with
	// one rather than ended by ';'.
	Members []Node
	HasBody bool
}

// FinalNode is the termination point for action execution.
type FinalNode struct {
	NodeBase
}

// ForkNode splits execution into concurrent flows (1 incoming → N outgoing).
type ForkNode struct {
	NodeBase
	Name     string
	NameSpan source.Span // span of Name, empty for an unnamed node
	// Members and HasBody carry the body every ControlNode may declare
	// (SysML.xtext ControlNode: `UsageDeclaration? ActionBody`), which is where
	// the node's parameters are written (`fork F { in a; out b; }`).
	Members []Node
	HasBody bool
}

// JoinNode synchronizes concurrent flows (N incoming → 1 outgoing).
type JoinNode struct {
	NodeBase
	Name     string
	NameSpan source.Span // span of Name, empty for an unnamed node
	Members  []Node      // body members, as on ForkNode
	HasBody  bool
}

// MergeNode merges alternative flows (N incoming → 1 outgoing, each arrival passes).
type MergeNode struct {
	NodeBase
	Name     string
	NameSpan source.Span // span of Name, empty for an unnamed node
	Members  []Node      // body members, as on ForkNode
	HasBody  bool
}

// DecisionNode is a conditional branch point (1 incoming → N guarded outgoing).
type DecisionNode struct {
	NodeBase
	Name     string
	NameSpan source.Span // span of Name, empty for an unnamed node
	Members  []Node      // body members, as on ForkNode
	HasBody  bool
}

// NodeBodyMembers returns the members of the body an action node declares, and
// nil for a node that declares no body or admits none.
func NodeBodyMembers(n Node) []Node {
	switch v := n.(type) {
	case *ForkNode:
		return v.Members
	case *JoinNode:
		return v.Members
	case *MergeNode:
		return v.Members
	case *DecisionNode:
		return v.Members
	case *InitialNode:
		return v.Members
	case *TransitionMember:
		return v.Members
	case *SendStatement:
		return v.Members
	}
	return nil
}

// ActionExecutionNode performs action work: invokes nested action or evaluates inline expression.
type ActionExecutionNode struct {
	NodeBase
	Name       string
	ActionRef  *QualifiedName // reference to nested action (mutually exclusive with Expression)
	Expression Node           // inline expression (mutually exclusive with ActionRef)
}

// AssignmentActionNode represents an assignment statement: assign target := value;
type AssignmentActionNode struct {
	NodeBase
	Target Node // feature reference or qualified name
	Value  Node // expression to assign
}

// PerformActionNode represents a perform statement: perform action;
type PerformActionNode struct {
	NodeBase
	ActionRef Node // qualified name or invocation expression
}

// PerformedInvocation is the call the statement performs (`perform tag(x);`);
// nil when it names the action without arguments.
func (n *PerformActionNode) PerformedInvocation() *InvocationExpr {
	if inv, ok := n.ActionRef.(*InvocationExpr); ok && inv.Type != nil {
		return inv
	}
	return nil
}

// LoopKind discriminates the loop forms, which share one node because they
// share a body and a condition but differ in when the condition is tested.
type LoopKind int

const (
	// LoopWhile is `while <cond> { body }`: the condition is tested before every
	// iteration, so a body may run zero times.
	LoopWhile LoopKind = iota
	// LoopUntil is `loop { body } until <cond>;`: the condition is tested after
	// every iteration, so the body runs at least once. A loop written without an
	// `until` clause has no condition and iterates until the step budget stops it.
	LoopUntil
	// LoopFor is `for <var> in <collection> { body }`: the body runs once per
	// element of the collection, with the element bound to the loop's variable.
	LoopFor
)

// String renders the loop's introducing keyword.
func (k LoopKind) String() string {
	switch k {
	case LoopUntil:
		return "loop"
	case LoopFor:
		return "for"
	default:
		return "while"
	}
}

// WhileLoopActionNode represents a loop in an action body, in any of the three
// forms LoopKind names: `while c { … }`, `loop { … } until c;`, `for x in c { … }`.
type WhileLoopActionNode struct {
	NodeBase
	Kind      LoopKind
	Condition Node   // boolean expression; nil for `for` and for `loop` without `until`
	Body      []Node // statements in loop body
	// Until is the condition an `until` clause of a `while` loop tests after each
	// iteration (`while c { … } until d;`), nil otherwise. A `loop` form records
	// its own `until` condition in Condition.
	Until Node
	// Variable is the iteration variable a `for` loop declares. It is a member of
	// the loop's own body scope, never of the enclosing behavior. Zero for the
	// other loop forms.
	Variable Identification
	// VariableRelationships are the specializations the iteration variable was
	// declared with (`for n : ScalarValues::Integer in …`), empty when it stated
	// none: SysML.xtext ForVariableDeclaration is a full UsageDeclaration.
	VariableRelationships []*Relationship
	// Collection is the expression a `for` loop iterates over, nil otherwise.
	Collection Node
}

// IfBranchKind discriminates the two branches of an if action.
type IfBranchKind int

const (
	// IfBranchThen is the branch taken when the condition holds.
	IfBranchThen IfBranchKind = iota
	// IfBranchElse is the branch taken when it does not.
	IfBranchElse
)

// String renders the branch's introducing keyword.
func (k IfBranchKind) String() string {
	if k == IfBranchElse {
		return "else"
	}
	return "then"
}

// IfBranchNode is the body of one branch of an if action. It exists so that a
// branch is an element in its own right: the declarations a branch body makes
// are members of the branch, not of the enclosing behavior, and only a node can
// own a scope.
type IfBranchNode struct {
	NodeBase
	Kind IfBranchKind
	Body []Node // statements in the branch body
}

// IfActionNode represents conditional: if condition { thenBody } else { elseBody }
type IfActionNode struct {
	NodeBase
	Condition Node          // boolean expression
	Then      *IfBranchNode // branch taken when the condition holds
	Else      *IfBranchNode // branch taken otherwise (nil when there is no else)
}

// Branches returns the branches the conditional declares, in source order,
// skipping an absent else.
func (n *IfActionNode) Branches() []*IfBranchNode {
	var branches []*IfBranchNode
	if n.Then != nil {
		branches = append(branches, n.Then)
	}
	if n.Else != nil {
		branches = append(branches, n.Else)
	}
	return branches
}

// DoneFeature is the name of the end shot every state inherits from the standard
// library (`Systems Library/States.sysml`), which a transition enters to complete.
const DoneFeature = "done"

// StateNode represents a state in a state machine (simple, composite, or orthogonal).
type StateNode struct {
	NodeBase
	Name  string
	Entry []Node // entry behaviors (action sequence)
	Do    []Node // do action (ongoing action)
	Exit  []Node // exit behaviors (action sequence)
	// Defer names the events the state defers while it is active: an event no
	// transition of the active configuration handles is retained instead of
	// dropped, and delivered again once no active state defers it.
	Defer     []Node
	Substates []Node         // nested states (hierarchical)
	Regions   []*StateRegion // orthogonal regions (parallel)
}

// StateRegion is an orthogonal region within a composite state.
type StateRegion struct {
	NodeBase
	Name   string
	States []Node // states in this region
}

// PseudostateKind discriminates pseudostate types.
type PseudostateKind int

const (
	PseudostateChoice   PseudostateKind = iota // conditional branch
	PseudostateJunction                        // merge point
	PseudostateFork                            // parallel split
	PseudostateJoin                            // parallel sync
	// PseudostateShallowHistory re-enters the substate of its composite state
	// that was active when that state was last exited.
	PseudostateShallowHistory
	// PseudostateDeepHistory re-enters the innermost substate that was active
	// when its composite state was last exited.
	PseudostateDeepHistory
)

func (k PseudostateKind) String() string {
	switch k {
	case PseudostateChoice:
		return "choice"
	case PseudostateJunction:
		return "junction"
	case PseudostateFork:
		return "fork"
	case PseudostateJoin:
		return "join"
	case PseudostateShallowHistory:
		return "shallow history"
	case PseudostateDeepHistory:
		return "deep history"
	default:
		return "unknown"
	}
}

// PseudostateNode is a transient state for control flow.
type PseudostateNode struct {
	NodeBase
	Kind PseudostateKind
	Name string
	// Keyword is the notation the pseudostate was written with (`choice`,
	// `deep history`, `fork`).
	Keyword string
}

// SuccessionEdge is sequential control flow in actions (source then target).
type SuccessionEdge struct {
	NodeBase
	Source *QualifiedName // source action node
	Target *QualifiedName // target action node
	// SourceMember and TargetMember are the members a member-attached `then`
	// (SysML.xtext EmptySuccessionMember) sequences when the member declares no
	// name a reference could use — `then send fullyCharged() to self;`, `then
	// loop action { … } until c;`. The notation binds such an end by position, so
	// the end is that member itself rather than a name; they are the members
	// beside the keyword as written, set once when the body is parsed.
	SourceMember Node
	TargetMember Node
	// SourceImplied and TargetImplied mark an end the notation supplied rather
	// than the author writing it: the source a one-name `then <target>;` takes
	// from the member before it, and the ends of the edge a member-attached
	// `then` desugars to. Such a name is a member's own, not a reference written
	// in the model.
	SourceImplied bool
	TargetImplied bool
	// Members are the body an action target succession carries
	// (SysML.xtext:1698 ActionTargetSuccession ends in a UsageBody), and
	// HasBody that it was written with one rather than ended by ';'.
	Members []Node
	HasBody bool
}

// ControlFlowEdge is guarded control flow from decision nodes.
type ControlFlowEdge struct {
	NodeBase
	Source *QualifiedName // source node (typically DecisionNode)
	Target *QualifiedName // target node
	Guard  Node           // boolean guard expression, nil for the default branch
	// IsElse records that the edge was written as the `else <target>;` branch of
	// a decision (SysML.xtext DefaultTargetSuccession), the branch taken when no
	// guarded branch of the same decision is.
	IsElse bool
	// SourceMember and TargetMember bind an end by position rather than by name,
	// as on SuccessionEdge: the branches of a decision node the body declares
	// without a name (`then decide; if x then a; else b;`) reach it this way.
	SourceMember Node
	TargetMember Node
	// SourceImplied and TargetImplied mark an end the notation supplied rather
	// than the author writing it, as on SuccessionEdge.
	SourceImplied bool
	TargetImplied bool
}

// ObjectFlowEdge is data flow between action parameters/pins (Tier 5).
type ObjectFlowEdge struct {
	NodeBase
	Source *QualifiedName // source pin/parameter
	Target *QualifiedName // target pin/parameter
}

// TransitionEdge is a state machine transition.
type TransitionEdge struct {
	NodeBase
	Source  *QualifiedName // source state
	Target  *QualifiedName // target state
	Trigger TriggerEvent   // event that fires transition (interface, see below)
	Guard   Node           // optional guard expression
	Effect  []Node         // optional effect actions
}

// TriggerEvent is the interface for state transition triggers.
type TriggerEvent interface {
	Node
	triggerEvent() // unexported marker method (closed set)
}

// TimeEvent fires at a point in time. Absolute distinguishes `accept at <time>`,
// which fires at the given instant, from `accept after <duration>`, which fires
// that far after the source state is entered.
type TimeEvent struct {
	NodeBase
	Duration Node // time expression (literal or variable)
	Absolute bool // true for `at`, false for `after`
}

func (*TimeEvent) triggerEvent() { /* marker: closed TriggerEvent set */ }

// ChangeEvent fires when a condition becomes true.
type ChangeEvent struct {
	NodeBase
	Condition Node // boolean expression
}

func (*ChangeEvent) triggerEvent() { /* marker: closed TriggerEvent set */ }

// AcceptEvent fires when a signal is received. It is the lowered form of the
// payload parameter of an accept (SysML.xtext `PayloadParameter`), whose three
// spellings this node keeps apart:
//
//	accept Warning              — SignalType, an unnamed payload typed by Warning
//	accept msg : Warning        — SignalType with Payload naming the parameter
//	accept :> shutDown          — Subsets, the event feature accepted
//
// Subsetting an event accepts occurrences of *that feature* rather than of a
// type, so the two are matched differently and are recorded separately.
type AcceptEvent struct {
	NodeBase
	SignalType *QualifiedName // signal type to accept
	// Subsets is the event feature the payload parameter subsets
	// (`accept :> shutDown`), nil when it subsets none.
	Subsets *QualifiedName
	// Payload is the payload parameter as it was declared, when the accept named
	// one (`accept msg : Warning`), so the name the received value binds to
	// survives lowering.
	Payload *Usage
}

func (*AcceptEvent) triggerEvent() { /* marker: closed TriggerEvent set */ }

// CallEvent fires when an operation is invoked. Parameters are the argument
// names the trigger declares (`accept op(speed)`); an empty list matches a call
// to that operation whatever arguments it carries.
type CallEvent struct {
	NodeBase
	Operation  *QualifiedName // operation to invoke
	Parameters []NameSegment  // declared argument names, in written order
}

func (*CallEvent) triggerEvent() { /* marker: closed TriggerEvent set */ }

// Phase C1: Calculation and Constraint Body Members

// ConstraintMember represents a condition of a constraint body.
// Syntax: <expression> or assert [not] <reference>; or assert constraint { … }
type ConstraintMember struct {
	NodeBase
	IsAssert bool // true for 'assert', false for 'assume'
	// Keyword is the keyword the condition was written with: "assert",
	// "assume", or "" for a bare condition, which asserts implicitly.
	Keyword    string
	IsNegated  bool   // true if 'not' keyword present (assert not expr)
	Expression Node   // the constraint expression, nil when stated through Body
	Name       string // name of the nested constraint, when it has one
	Body       []Node // conditions of a nested constraint: assert constraint { <expr> }
}

// Phase C2: Requirement Body Members

// SubjectMember represents the subject declaration in a requirement body.
// Syntax: subject [<short>] [name] : <Type>; OR subject = <expr>;
type SubjectMember struct {
	NodeBase
	// Prefixes are the metadata written after the keyword: the `#B` of
	// `subject #B s` (SysML.xtext SubjectUsage UsageExtensionKeyword).
	Prefixes      []*PrefixMetadata
	Ident         Identification  // `<short> name`, either or both optional (SysML.xtext Identification)
	TypeRef       *QualifiedName  // subject type (for declaration form)
	Multiplicity  *Multiplicity   // optional multiplicity
	Relationships []*Relationship // specializations written after the type (`:>> RequirementCheck::subj`)
	Body          []Node          // optional nested members
	BindingExpr   Node            // value part: `subject = <expr>;` or a declaration's `= expr` / `default expr`
	// ValueOperatorSpan, ValueIsDefault and ValueIsInitial describe BindingExpr's
	// operator as Usage's fields of the same names do.
	ValueOperatorSpan source.Span
	ValueIsDefault    bool
	ValueIsInitial    bool
	// HasBody records that the declaration was written with braces, which an
	// empty body does not otherwise show.
	HasBody bool
}

// AssumeMember represents an assumption in a requirement body.
// Syntax: assume <reference>; OR assume constraint { <expression>... } OR assume <Q::r> { body }
type AssumeMember struct {
	NodeBase
	// Prefix metadata written before the declaration: `assume #goal constraint c;`.
	Prefixes   []*PrefixMetadata
	Expression Node           // the reference the member states, when written as one
	Reference  *QualifiedName // referenced constraint/requirement (reference-subsetting form)
	Body       []Node         // ConstraintMembers of the nested constraint (for the braced form)
	// Declaration of the constraint the member owns, when it is written with
	// one: `assume constraint <c> c1 : C;`. DeclSpan covers it from `constraint` on.
	Ident         Identification
	DeclSpan      source.Span
	Relationships []*Relationship
	Multiplicity  *Multiplicity
	Value         Node
	// ValueOperatorSpan covers the value operator; ValueIsDefault and
	// ValueIsInitial are its `default` and `:=` (KerML FeatureValue).
	ValueOperatorSpan source.Span
	ValueIsDefault    bool
	ValueIsInitial    bool
	// HasBody records that the member was written with braces, which an empty
	// body does not otherwise show.
	HasBody bool
}

// RequireMember represents a requirement constraint.
// Syntax: require <reference>; OR require constraint { <expression>... } OR require <Q::r> { body }
type RequireMember struct {
	NodeBase
	// Prefix metadata written before the declaration: `require #goal r;`.
	Prefixes   []*PrefixMetadata
	Expression Node           // the reference the member states, when written as one
	Reference  *QualifiedName // referenced requirement (reference-subsetting form, SysML v2 §7.20)
	Body       []Node         // nested members: ConstraintMembers for the braced form, requirement members for the reference form
	// Declaration of the constraint the member owns, when it is written with
	// one: `require constraint <c> c1 : C;`. DeclSpan covers it from `constraint` on.
	Ident         Identification
	DeclSpan      source.Span
	Relationships []*Relationship
	Multiplicity  *Multiplicity
	Value         Node
	// ValueOperatorSpan covers the value operator; ValueIsDefault and
	// ValueIsInitial are its `default` and `:=` (KerML FeatureValue).
	ValueOperatorSpan source.Span
	ValueIsDefault    bool
	ValueIsInitial    bool
	// HasBody records that the member was written with braces, which an empty
	// body does not otherwise show.
	HasBody bool
}

// OwnedConstraint is the constraint usage an assume or require member declares
// with the `constraint` keyword (SysML.xtext RequirementConstraintUsage).
type OwnedConstraint struct {
	Ident             Identification
	DeclSpan          source.Span
	Relationships     []*Relationship
	Multiplicity      *Multiplicity
	Value             Node
	ValueOperatorSpan source.Span
	ValueIsDefault    bool
	ValueIsInitial    bool
	Body              []Node
}

// OwnedConstraintOf returns the constraint an assume or require member declares,
// or false for one that references a requirement or states a condition instead.
func OwnedConstraintOf(n Node) (OwnedConstraint, bool) {
	switch m := n.(type) {
	case *AssumeMember:
		if m.Reference != nil || m.Expression != nil {
			return OwnedConstraint{}, false
		}
		return OwnedConstraint{
			Ident: m.Ident, DeclSpan: m.DeclSpan, Relationships: m.Relationships,
			Multiplicity: m.Multiplicity, Value: m.Value, ValueOperatorSpan: m.ValueOperatorSpan,
			ValueIsDefault: m.ValueIsDefault, ValueIsInitial: m.ValueIsInitial, Body: m.Body,
		}, true
	case *RequireMember:
		if m.Reference != nil || m.Expression != nil {
			return OwnedConstraint{}, false
		}
		return OwnedConstraint{
			Ident: m.Ident, DeclSpan: m.DeclSpan, Relationships: m.Relationships,
			Multiplicity: m.Multiplicity, Value: m.Value, ValueOperatorSpan: m.ValueOperatorSpan,
			ValueIsDefault: m.ValueIsDefault, ValueIsInitial: m.ValueIsInitial, Body: m.Body,
		}, true
	}
	return OwnedConstraint{}, false
}

// ConstraintReferenceOf returns the constraint or requirement an assume or
// require member states by reference alone (`require P::r1;`, `require r1;`,
// `require h.r1;`): a qualified name or a feature chain.
func ConstraintReferenceOf(n Node) Node {
	switch m := n.(type) {
	case *AssumeMember:
		if m.Reference != nil {
			return m.Reference
		}
	case *RequireMember:
		if m.Reference != nil {
			return m.Reference
		}
	default:
		return nil
	}
	return ConditionReference(n)
}

// ConditionReference returns the name or feature chain an assume or require
// member's condition consists of alone (`require r1;`, `require h.r1;`): the
// reference form the parser keeps as a condition expression, whose target is
// resolved as a reference subsetting's.
func ConditionReference(n Node) Node {
	var expr Node
	switch m := n.(type) {
	case *AssumeMember:
		expr = m.Expression
	case *RequireMember:
		expr = m.Expression
	default:
		return nil
	}
	switch e := expr.(type) {
	case *FeatureReference:
		if e.Name != nil {
			return e.Name
		}
	case *FeatureChainExpr:
		if e.Member != nil {
			return e
		}
	}
	return nil
}

// NamingFeature returns the relationship naming a constraint declared without a
// name: a requirement constraint is named by the constraint it references
// (SysML ConstraintUsage::namingFeature); a redefinition leaves it anonymous.
func (c OwnedConstraint) NamingFeature() *Relationship {
	if c.Ident.Declared() {
		return nil
	}
	return namingReference(c.Relationships)
}

// EffectiveName returns the name the constraint answers to: its declared name,
// else the name its naming feature supplies.
func (c OwnedConstraint) EffectiveName() (string, source.Span) {
	if c.Ident.Declared() {
		return c.Ident.DeclaredName()
	}
	if rel := c.NamingFeature(); rel != nil {
		return TargetName(rel.Target)
	}
	return "", source.Span{}
}

// NamingFeature returns the relationship naming a subject declared without a
// name: its first redefinition, as for a usage (`subject :>> vehicle;` answers
// to `vehicle`); a subject that merely references a feature stays anonymous.
func (m *SubjectMember) NamingFeature() *Relationship {
	if m == nil || m.Ident.Declared() {
		return nil
	}
	return firstRedefinition(m.Relationships)
}

// EffectiveName returns the name the subject answers to: its declared name,
// else the name its naming feature supplies.
func (m *SubjectMember) EffectiveName() (string, source.Span) {
	if m == nil {
		return "", source.Span{}
	}
	if m.Ident.Declared() {
		return m.Ident.DeclaredName()
	}
	if rel := m.NamingFeature(); rel != nil {
		return TargetName(rel.Target)
	}
	return "", source.Span{}
}

// DeclNamedByReference reports whether an unnamed declaration is named by the
// feature it references, and whether that reference is its only naming feature
// (a requirement's assume/require/verify member, which no redefinition names).
func DeclNamedByReference(decl Node) (byReference, referenceOnly bool) {
	switch decl.(type) {
	case *AssumeMember, *RequireMember:
		return true, true
	}
	if u, ok := decl.(*Usage); ok && u.NamedByReference() {
		return true, u.IsRequirementConstraint() || u.IsVerifiedRequirement()
	}
	return false, false
}

// DeclNamingFeature is NamingFeature over every declaration that may borrow its
// name: a usage, a requirement subject or an assume/require constraint.
func DeclNamingFeature(decl Node) *Relationship {
	if oc, ok := OwnedConstraintOf(decl); ok {
		return oc.NamingFeature()
	}
	switch d := decl.(type) {
	case *Usage:
		return NamingFeature(d)
	case *SubjectMember:
		return d.NamingFeature()
	}
	return nil
}

// Phase C4: State Body Members

// EntryMember represents entry behavior in a state body.
// Syntax: entry { <actions> }
type EntryMember struct {
	NodeBase
	Actions []Node // action sequence
}

// DoMember represents an ongoing do action in a state body.
// Syntax: do { <actions> }
type DoMember struct {
	NodeBase
	Actions []Node // action sequence
}

// ExitMember represents exit behavior in a state body.
// Syntax: exit { <actions> }
type ExitMember struct {
	NodeBase
	Actions []Node // action sequence
}

// DeferMember represents the events a state defers while it is active.
// Syntax: defer <event> [, <event>]* ;
type DeferMember struct {
	NodeBase
	Triggers []Node // deferred triggers, in declaration order
}

// SubstateMember represents a nested state declaration.
// Syntax: state <name>;
type SubstateMember struct {
	NodeBase
	Name     string      // substate name
	NameSpan source.Span // span of Name, empty for an unnamed node
}

// TransitionMember represents a state transition in textual form, in either the
// standard spelling (SysML.xtext `TransitionUsage`)
//
//	transition [<name>] first <source> [accept <trigger>] [if <guard>]
//	    [do <effect>] then <target>;
//
// or the `transition <source> to <target> …` spelling OpenSysML also accepts.
type TransitionMember struct {
	NodeBase
	Name     string         // the transition's own name, empty when anonymous
	NameSpan source.Span    // span of Name, empty for an anonymous transition
	Source   *QualifiedName // source state
	Target   *QualifiedName // target state
	Trigger  Node           // optional trigger event (TimeEvent/ChangeEvent/etc)
	// TriggerSpan spans the accepter the trigger keyword introduces
	// (`accept A`), which is the element the accepter rules are about.
	TriggerSpan source.Span
	Guard       Node   // optional guard expression
	Effect      []Node // optional effect actions
	HasEffect   bool   // a `do` was written, even one whose braces hold nothing
	// Via is the port the trigger's message must arrive at
	// (`accept :> ping via commPort`), nil when the trigger named none.
	Via *QualifiedName
	// Members and HasBody carry the body a transition may declare, since both
	// TransitionUsage and TargetTransitionUsage end in ActionBody
	// (`then starting { … }`).
	Members []Node
	HasBody bool
}

// SendStatement sends a message to a target.
// Syntax: send <message> to <target>; | send <message> via <port>;
//
// The two forms address different things and are not interchangeable: `to`
// names the receiver directly, while `via` names a port of the sender, and the
// message reaches whatever that port is connected to. IsVia records which form
// was written so routing can tell them apart.
type SendStatement struct {
	NodeBase
	Message Node // message expression (often NewExpression); nil when the send declared none
	Target  Node // target expression: the receiver (`to`) or the sending port (`via`)
	IsVia   bool // the target is a port to route through, not a receiver
	// Receiver is the receiver of a send that states both clauses (`send x via p
	// to r`), which SysML.xtext SenderReceiverPart allows.
	Receiver Node
	// Members and HasBody carry the body SysML.xtext SendNode ends in, where the
	// message a send with no argument carries is declared (`send { in :>> payload
	// = s; }`).
	Members []Node
	HasBody bool
}

// TerminateStatement terminates an action or lifecycle.
// Syntax: terminate [<target>];
type TerminateStatement struct {
	NodeBase
	Target Node // occurrence to terminate; nil terminates the performing occurrence
}

// AcceptActionUsage represents an action that waits for a signal.
// Syntax: action <name> accept <param> : Type;
type AcceptActionUsage struct {
	NodeBase
	Name      string
	ParamName string
	ParamType *QualifiedName
}
