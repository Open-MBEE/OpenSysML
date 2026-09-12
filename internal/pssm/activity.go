package pssm

import (
	"fmt"
	"sort"
	"strings"
)

// activityReader reads one UML activity graph into a Body. The suite's
// activities come in two shapes: graphs drawn by hand (an InitialNode, actions
// joined by control and object flows, an ActivityFinalNode) and graphs compiled
// from Alf, whose statements are numbered StructuredActivityNodes under a
// "Body" node. Both read the same way: a block's direct nodes are ordered by
// the flows between them, structured nodes are flattened, and the value on a
// pin is found by following the object flow that feeds it.
type activityReader struct {
	r        *reader
	activity *Element
	// incoming maps an element id to the object or control flow edges whose
	// target it is, over the whole activity.
	incoming map[string][]*Element
	// outgoing maps an element id to the edges whose source it is.
	outgoing map[string][]*Element
	body     *Body
	visiting map[string]bool
}

// readActivity reads an activity element (uml:Activity) into a Body.
func (r *reader) readActivity(act *Element) *Body {
	ar := &activityReader{
		r:        r,
		activity: act,
		incoming: make(map[string][]*Element),
		outgoing: make(map[string][]*Element),
		body:     &Body{},
		visiting: make(map[string]bool),
	}
	act.Walk(func(e *Element) bool {
		if e.Tag == "edge" {
			ar.incoming[e.Attr("target")] = append(ar.incoming[e.Attr("target")], e)
			ar.outgoing[e.Attr("source")] = append(ar.outgoing[e.Attr("source")], e)
		}
		return true
	})
	ar.readBlock(act)
	return ar.body
}

// readBlock appends the statements of a block (the activity or a structured
// node) in flow order.
func (ar *activityReader) readBlock(block *Element) {
	nodes := block.Tagged("node")
	for _, n := range ar.order(nodes) {
		ar.readNode(n)
	}
}

// order sorts a block's direct nodes so that every node follows the nodes
// whose flows it depends on, ties broken by document order.
func (ar *activityReader) order(nodes []*Element) []*Element {
	owner := make(map[string]int, len(nodes))
	for i, n := range nodes {
		n.Walk(func(e *Element) bool {
			if e.ID != "" {
				owner[e.ID] = i
			}
			return true
		})
	}
	deps := make([][]int, len(nodes))
	indegree := make([]int, len(nodes))
	seen := make(map[[2]int]bool)
	for i, n := range nodes {
		n.Walk(func(e *Element) bool {
			for _, edge := range ar.incoming[e.ID] {
				src, ok := owner[edge.Attr("source")]
				if ok && src != i && !seen[[2]int{src, i}] {
					seen[[2]int{src, i}] = true
					deps[src] = append(deps[src], i)
					indegree[i]++
				}
			}
			return true
		})
	}
	var ready []int
	for i := range nodes {
		if indegree[i] == 0 {
			ready = append(ready, i)
		}
	}
	out := make([]*Element, 0, len(nodes))
	done := make([]bool, len(nodes))
	for len(ready) > 0 {
		sort.Ints(ready)
		i := ready[0]
		ready = ready[1:]
		done[i] = true
		out = append(out, nodes[i])
		for _, j := range deps[i] {
			indegree[j]--
			if indegree[j] == 0 {
				ready = append(ready, j)
			}
		}
	}
	for i := range nodes {
		if !done[i] {
			ar.unsupported(nodes[i], "lies on a flow cycle")
			out = append(out, nodes[i])
		}
	}
	return out
}

func (ar *activityReader) unsupported(e *Element, why string) {
	ar.body.Unsupported = append(ar.body.Unsupported, fmt.Sprintf("%s %s", e.Describe(), why))
}

// consumed reports whether any of the node's output pins feeds another node,
// in which case the node is an expression read where it is used.
func (ar *activityReader) consumed(n *Element) bool {
	for _, pin := range n.Children {
		if pin.Tag == "result" && len(ar.outgoing[pin.ID]) > 0 {
			return true
		}
	}
	return false
}

func (ar *activityReader) readNode(n *Element) {
	switch n.Type {
	case "uml:StructuredActivityNode", "uml:SequenceNode":
		ar.readBlock(n)
	case "uml:ExpansionRegion":
		ar.unsupported(n, "iterates over a collection")
	case "uml:ConditionalNode", "uml:LoopNode":
		ar.unsupported(n, "branches or loops")
	case "uml:CallOperationAction":
		if ar.consumed(n) {
			return
		}
		op := ar.r.doc.ByID(n.Attr("operation"))
		if op == nil {
			ar.unsupported(n, "calls an operation the document does not define")
			return
		}
		ar.emit(Statement{Kind: StmtCall, Name: op.Name(), Receiver: ar.pinValue(n.First("target")), Args: ar.args(n)})
	case "uml:CallBehaviorAction":
		if ar.consumed(n) {
			return
		}
		ar.emit(Statement{Kind: StmtCall, Name: ar.behaviorName(n), Args: ar.args(n)})
	case "uml:SendSignalAction":
		sig := ar.r.doc.ByID(n.Attr("signal"))
		if sig == nil {
			ar.unsupported(n, "sends a signal the document does not define")
			return
		}
		ar.emit(Statement{Kind: StmtSend, Name: sig.Name(), Receiver: ar.pinValue(n.First("target")), Args: ar.args(n)})
	case "uml:AcceptEventAction", "uml:AcceptCallAction":
		st := Statement{Kind: StmtAccept}
		for _, trig := range n.Tagged("trigger") {
			st.Events = append(st.Events, ar.r.readEvent(trig.Attr("event"), trig))
		}
		if res := n.First("result"); res != nil && len(ar.outgoing[res.ID]) > 0 {
			st.Result = res.Name()
		}
		ar.emit(st)
	case "uml:AddStructuralFeatureValueAction":
		feature := ar.r.doc.ByID(n.Attr("structuralFeature"))
		if feature == nil {
			ar.unsupported(n, "writes a feature the document does not define")
			return
		}
		if n.First("insertAt") != nil && len(ar.incoming[n.First("insertAt").ID]) > 0 {
			ar.unsupported(n, "writes at a position")
			return
		}
		ar.emit(Statement{
			Kind:     StmtAssign,
			Receiver: ar.pinValue(n.First("object")),
			Feature:  feature.Name(),
			Value:    ar.pinValue(n.First("value")),
			Replace:  n.Attr("isReplaceAll") == "true",
		})
	case "uml:ActivityParameterNode":
		// A fed return parameter node is the body's return statement.
		param := ar.r.doc.ByID(n.Attr("parameter"))
		if param != nil && (param.Attr("direction") == "return" || param.Attr("direction") == "out") && len(ar.incoming[n.ID]) > 0 {
			ar.emit(Statement{Kind: StmtReturn, Value: ar.pinValue(n)})
		}
	case "uml:InitialNode", "uml:ActivityFinalNode", "uml:FlowFinalNode", "uml:ForkNode", "uml:JoinNode",
		"uml:MergeNode", "uml:DecisionNode", "uml:ExpansionNode":
	case "uml:ValueSpecificationAction", "uml:ReadSelfAction", "uml:ReadStructuralFeatureAction",
		"uml:ClearStructuralFeatureAction", "uml:TestIdentityAction", "uml:ReadIsClassifiedObjectAction":
		// Values: read where a pin consumes them. An unconsumed one is dead.
	case "uml:CreateObjectAction", "uml:StartObjectBehaviorAction", "uml:DestroyObjectAction",
		"uml:ReadExtentAction", "uml:StartClassifierBehaviorAction", "uml:ReduceAction",
		"uml:RemoveStructuralFeatureValueAction", "uml:CreateLinkAction", "uml:DestroyLinkAction",
		"uml:ReadLinkAction", "uml:ClearAssociationAction", "uml:ReclassifyObjectAction", "uml:UnmarshallAction":
		ar.unsupported(n, "manipulates objects or links")
	default:
		ar.unsupported(n, "is a node kind the reader does not know")
	}
}

func (ar *activityReader) emit(st Statement) {
	ar.body.Statements = append(ar.body.Statements, st)
}

// args reads a call or send action's argument pins in document order.
func (ar *activityReader) args(n *Element) []Expr {
	var out []Expr
	for _, pin := range n.Tagged("argument") {
		v := ar.pinValue(pin)
		if v == nil {
			continue
		}
		out = append(out, *v)
	}
	return out
}

// behaviorName names the behavior a CallBehaviorAction calls: a library
// behavior by the last segment of its href (Concat, ToString, Not), an owned
// behavior by its name.
func (ar *activityReader) behaviorName(n *Element) string {
	if id := n.Attr("behavior"); id != "" {
		if b := ar.r.doc.ByID(id); b != nil {
			return b.Name()
		}
		return id
	}
	if b := n.First("behavior"); b != nil {
		if href := b.Href(); href != "" {
			frag := href[strings.LastIndex(href, "#")+1:]
			return frag[strings.LastIndex(frag, "-")+1:]
		}
		if b.Attr("idref") != "" {
			if bb := ar.r.doc.ByID(b.Attr("idref")); bb != nil {
				return bb.Name()
			}
		}
	}
	return n.Name()
}

// pinValue reads the value flowing into a pin or node: nil when nothing feeds
// it (an absent optional argument), an Expr otherwise.
func (ar *activityReader) pinValue(pin *Element) *Expr {
	if pin == nil {
		return nil
	}
	edges := ar.incoming[pin.ID]
	if len(edges) == 0 {
		return nil
	}
	if len(edges) > 1 {
		return &Expr{Kind: ExprUnknown, Text: fmt.Sprintf("%s is fed by %d flows", pin.Describe(), len(edges))}
	}
	v := ar.value(edges[0].Attr("source"))
	return &v
}

// value reads the value an element produces: an output pin, a fork or
// parameter node, or a structured node's pin.
func (ar *activityReader) value(id string) Expr {
	e := ar.r.doc.ByID(id)
	if e == nil {
		return Expr{Kind: ExprUnknown, Text: "flow from " + id + " which the document does not define"}
	}
	if ar.visiting[id] {
		return Expr{Kind: ExprUnknown, Text: e.Describe() + " feeds itself"}
	}
	ar.visiting[id] = true
	defer delete(ar.visiting, id)

	switch e.Type {
	case "uml:ForkNode", "uml:MergeNode", "uml:ExpansionNode", "uml:CentralBufferNode":
		return ar.passThrough(e)
	case "uml:ActivityParameterNode":
		param := ar.r.doc.ByID(e.Attr("parameter"))
		if param == nil {
			return Expr{Kind: ExprUnknown, Text: e.Describe() + " names no parameter"}
		}
		if dir := param.Attr("direction"); dir == "in" || dir == "inout" || dir == "" {
			return Expr{Kind: ExprParam, Name: param.Name()}
		}
		return ar.passThrough(e)
	}
	if e.Tag == "structuredNodeInput" || e.Tag == "structuredNodeOutput" || e.Type == "uml:InputPin" || e.Tag == "argument" || e.Tag == "object" || e.Tag == "target" || e.Tag == "value" {
		return ar.passThrough(e)
	}
	// An output pin: the value is what its owning action computes.
	owner := e.Parent
	if owner == nil {
		return Expr{Kind: ExprUnknown, Text: e.Describe() + " has no owner"}
	}
	return ar.actionValue(owner, e)
}

func (ar *activityReader) passThrough(e *Element) Expr {
	edges := ar.incoming[e.ID]
	switch len(edges) {
	case 0:
		return Expr{Kind: ExprUnknown, Text: e.Describe() + " is fed by nothing"}
	case 1:
		return ar.value(edges[0].Attr("source"))
	}
	return Expr{Kind: ExprUnknown, Text: fmt.Sprintf("%s is fed by %d flows", e.Describe(), len(edges))}
}

// actionValue is the value an action's output pin carries.
func (ar *activityReader) actionValue(n, pin *Element) Expr {
	deref := func(p *Expr) *Expr {
		if p == nil {
			return &Expr{Kind: ExprUnknown, Text: n.Describe() + " reads an unfed pin"}
		}
		return p
	}
	switch n.Type {
	case "uml:ValueSpecificationAction":
		lit, diag := readLiteral(n.First("value"))
		if diag != "" {
			return Expr{Kind: ExprUnknown, Text: diag}
		}
		return Expr{Kind: ExprLiteral, Literal: lit}
	case "uml:ReadSelfAction":
		return Expr{Kind: ExprSelf}
	case "uml:ReadStructuralFeatureAction":
		feature := ar.r.doc.ByID(n.Attr("structuralFeature"))
		if feature == nil {
			return Expr{Kind: ExprUnknown, Text: n.Describe() + " reads a feature the document does not define"}
		}
		return Expr{Kind: ExprRead, Name: feature.Name(), Object: deref(ar.pinValue(n.First("object")))}
	case "uml:ClearStructuralFeatureAction":
		return *deref(ar.pinValue(n.First("object")))
	case "uml:CallBehaviorAction":
		return Expr{Kind: ExprApply, Name: ar.behaviorName(n), Args: ar.args(n)}
	case "uml:CallOperationAction":
		op := ar.r.doc.ByID(n.Attr("operation"))
		if op == nil {
			return Expr{Kind: ExprUnknown, Text: n.Describe() + " calls an operation the document does not define"}
		}
		return Expr{Kind: ExprCall, Name: op.Name(), Object: deref(ar.pinValue(n.First("target"))), Args: ar.args(n)}
	case "uml:TestIdentityAction":
		return Expr{Kind: ExprApply, Name: "==", Args: []Expr{*deref(ar.pinValue(n.First("first"))), *deref(ar.pinValue(n.First("second")))}}
	case "uml:AcceptEventAction", "uml:AcceptCallAction":
		return Expr{Kind: ExprEvent, Name: pin.Name()}
	case "uml:StructuredActivityNode", "uml:SequenceNode", "uml:ExpansionRegion":
		return ar.passThrough(pin)
	}
	return Expr{Kind: ExprUnknown, Text: n.Describe() + " is a node kind the reader does not evaluate"}
}

// readLiteral reads a literal specification element.
func readLiteral(v *Element) (*Literal, string) {
	if v == nil {
		return nil, "value specification is absent"
	}
	text, present := v.Attrs["value"]
	lit := &Literal{Text: text, Present: present}
	switch v.Type {
	case "uml:LiteralString":
		lit.Kind = LiteralString
	case "uml:LiteralBoolean":
		lit.Kind = LiteralBoolean
	case "uml:LiteralInteger":
		lit.Kind = LiteralInteger
	case "uml:LiteralUnlimitedNatural":
		lit.Kind = LiteralUnlimitedNatural
	case "uml:LiteralReal":
		lit.Kind = LiteralReal
	case "uml:LiteralNull":
		lit.Kind = LiteralNull
	default:
		return nil, v.Describe() + " is not a literal"
	}
	return lit, ""
}
