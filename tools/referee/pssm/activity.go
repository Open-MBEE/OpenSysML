package pssm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
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
	activity *xmi.Element
	// incoming maps an element id to the object or control flow edges whose
	// target it is, over the whole activity.
	incoming map[string][]*xmi.Element
	// outgoing maps an element id to the edges whose source it is.
	outgoing map[string][]*xmi.Element
	body     *Body
	visiting map[string]bool
}

// readActivity reads an activity element (uml:Activity) into a Body.
func (r *reader) readActivity(act *xmi.Element) *Body {
	ar := &activityReader{
		r:        r,
		activity: act,
		incoming: make(map[string][]*xmi.Element),
		outgoing: make(map[string][]*xmi.Element),
		body:     &Body{},
		visiting: make(map[string]bool),
	}
	act.Walk(func(e *xmi.Element) bool {
		if e.Tag == "edge" {
			ar.incoming[e.Attr("target")] = append(ar.incoming[e.Attr("target")], e)
			ar.outgoing[e.Attr("source")] = append(ar.outgoing[e.Attr("source")], e)
		}
		return true
	})
	ar.readBlock(act)
	ar.body.Acts = r.acts(act, map[string]bool{})
	return ar.body
}

// The xmi:type of each activity node, behavior and pin the reader tells apart.
const (
	typeAcceptCallAction                   = "uml:AcceptCallAction"
	typeAcceptEventAction                  = "uml:AcceptEventAction"
	typeActivity                           = "uml:Activity"
	typeActivityFinalNode                  = "uml:ActivityFinalNode"
	typeActivityParameterNode              = "uml:ActivityParameterNode"
	typeAddStructuralFeatureValueAction    = "uml:AddStructuralFeatureValueAction"
	typeCallBehaviorAction                 = "uml:CallBehaviorAction"
	typeCallOperationAction                = "uml:CallOperationAction"
	typeCentralBufferNode                  = "uml:CentralBufferNode"
	typeClearAssociationAction             = "uml:ClearAssociationAction"
	typeClearStructuralFeatureAction       = "uml:ClearStructuralFeatureAction"
	typeConditionalNode                    = "uml:ConditionalNode"
	typeCreateLinkAction                   = "uml:CreateLinkAction"
	typeCreateObjectAction                 = "uml:CreateObjectAction"
	typeDecisionNode                       = "uml:DecisionNode"
	typeDestroyLinkAction                  = "uml:DestroyLinkAction"
	typeDestroyObjectAction                = "uml:DestroyObjectAction"
	typeExpansionNode                      = "uml:ExpansionNode"
	typeExpansionRegion                    = "uml:ExpansionRegion"
	typeFlowFinalNode                      = "uml:FlowFinalNode"
	typeForkNode                           = "uml:ForkNode"
	typeFunctionBehavior                   = "uml:FunctionBehavior"
	typeInitialNode                        = "uml:InitialNode"
	typeInputPin                           = "uml:InputPin"
	typeJoinNode                           = "uml:JoinNode"
	typeLoopNode                           = "uml:LoopNode"
	typeMergeNode                          = "uml:MergeNode"
	typeReadExtentAction                   = "uml:ReadExtentAction"
	typeReadIsClassifiedObjectAction       = "uml:ReadIsClassifiedObjectAction"
	typeReadLinkAction                     = "uml:ReadLinkAction"
	typeReadSelfAction                     = "uml:ReadSelfAction"
	typeReadStructuralFeatureAction        = "uml:ReadStructuralFeatureAction"
	typeReclassifyObjectAction             = "uml:ReclassifyObjectAction"
	typeReduceAction                       = "uml:ReduceAction"
	typeRemoveStructuralFeatureValueAction = "uml:RemoveStructuralFeatureValueAction"
	typeSendSignalAction                   = "uml:SendSignalAction"
	typeSequenceNode                       = "uml:SequenceNode"
	typeStartClassifierBehaviorAction      = "uml:StartClassifierBehaviorAction"
	typeStartObjectBehaviorAction          = "uml:StartObjectBehaviorAction"
	typeStructuredActivityNode             = "uml:StructuredActivityNode"
	typeTestIdentityAction                 = "uml:TestIdentityAction"
	typeUnmarshallAction                   = "uml:UnmarshallAction"
	typeValueSpecificationAction           = "uml:ValueSpecificationAction"
)

// valueNodes are the node kinds that read, compute or route a value; every
// other action acts on the model, whether or not the reading expresses it.
var valueNodes = map[string]bool{
	typeValueSpecificationAction: true, typeReadSelfAction: true, typeReadStructuralFeatureAction: true,
	typeTestIdentityAction: true, typeReadIsClassifiedObjectAction: true, typeReadExtentAction: true,
	typeReadLinkAction: true, typeActivityParameterNode: true,
	typeInitialNode: true, typeActivityFinalNode: true, typeFlowFinalNode: true, typeForkNode: true,
	typeJoinNode: true, typeMergeNode: true, typeDecisionNode: true, typeExpansionNode: true,
	typeStructuredActivityNode: true, typeSequenceNode: true, typeConditionalNode: true,
	typeLoopNode: true, typeExpansionRegion: true,
}

// acts reports whether any node under the activity, at any depth, acts on the
// model; control flow only carries what it encloses, a call what it calls.
func (r *reader) acts(act *xmi.Element, visiting map[string]bool) bool {
	if visiting[act.ID] {
		return false
	}
	visiting[act.ID] = true
	defer delete(visiting, act.ID)
	acts := false
	act.Walk(func(e *xmi.Element) bool {
		if e.Tag != "node" {
			return true
		}
		switch e.Type {
		case typeCallBehaviorAction:
			acts = r.calledBehaviorActs(e, visiting)
		case typeCallOperationAction:
			var method *xmi.Element
			if op := r.doc.ByID(e.Attr("operation")); op != nil {
				method = r.doc.ByID(op.Ref("method"))
			}
			acts = r.callActs(method, visiting)
		default:
			acts = !valueNodes[e.Type]
		}
		return !acts
	})
	return acts
}

// calledBehaviorActs reports whether the behavior a CallBehaviorAction calls
// acts on the model: a library primitive function computes, anything else in
// the library (output, say) acts, and a behavior of the document is read.
func (r *reader) calledBehaviorActs(n *xmi.Element, visiting map[string]bool) bool {
	if b := n.First("behavior"); b != nil && b.Href() != "" {
		return !strings.Contains(b.Href(), "PrimitiveBehaviors")
	}
	return r.callActs(r.doc.ByID(n.Ref("behavior")), visiting)
}

// callActs reports whether calling a behavior of the document acts on the
// model: an activity does when its nodes do, a function behavior does not by
// UML's contract (§13.2.3.3); an unresolved or opaque one may.
func (r *reader) callActs(called *xmi.Element, visiting map[string]bool) bool {
	switch {
	case called == nil:
		return true
	case called.Type == typeFunctionBehavior:
		return false
	case called.Type != typeActivity:
		return true
	}
	return r.acts(called, visiting)
}

// readBlock appends the statements of a block (the activity or a structured
// node) in flow order.
func (ar *activityReader) readBlock(block *xmi.Element) {
	nodes := block.Tagged("node")
	for _, n := range ar.order(nodes) {
		ar.readNode(n)
	}
}

// order sorts a block's direct nodes so that every node follows the nodes
// whose flows it depends on, ties broken by document order.
func (ar *activityReader) order(nodes []*xmi.Element) []*xmi.Element {
	owner := make(map[string]int, len(nodes))
	for i, n := range nodes {
		n.Walk(func(e *xmi.Element) bool {
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
		n.Walk(func(e *xmi.Element) bool {
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
	out := make([]*xmi.Element, 0, len(nodes))
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

func (ar *activityReader) unsupported(e *xmi.Element, why string) {
	ar.body.Unsupported = append(ar.body.Unsupported, fmt.Sprintf("%s %s", e.Describe(), why))
}

// consumed reports whether any of the node's output pins feeds another node,
// in which case the node is an expression read where it is used.
func (ar *activityReader) consumed(n *xmi.Element) bool {
	for _, pin := range n.Children {
		if pin.Tag == "result" && len(ar.outgoing[pin.ID]) > 0 {
			return true
		}
	}
	return false
}

func (ar *activityReader) readNode(n *xmi.Element) {
	switch n.Type {
	case typeStructuredActivityNode, typeSequenceNode:
		ar.readBlock(n)
	case typeExpansionRegion:
		ar.unsupported(n, "iterates over a collection")
	case typeConditionalNode, typeLoopNode:
		ar.unsupported(n, "branches or loops")
	case typeCallOperationAction:
		if ar.consumed(n) {
			return
		}
		op := ar.r.doc.ByID(n.Attr("operation"))
		if op == nil {
			ar.unsupported(n, "calls an operation the document does not define")
			return
		}
		ar.emit(Statement{Kind: StmtCall, Name: op.Name(), OperationID: op.ID, Receiver: ar.pinValue(n.First("target")), Args: ar.args(n)})
	case typeCallBehaviorAction:
		if ar.consumed(n) {
			return
		}
		ar.emit(Statement{Kind: StmtCall, Name: ar.behaviorName(n), Args: ar.args(n)})
	case typeSendSignalAction:
		sig := ar.r.doc.ByID(n.Attr("signal"))
		if sig == nil {
			ar.unsupported(n, "sends a signal the document does not define")
			return
		}
		ar.emit(Statement{Kind: StmtSend, Name: sig.Name(), Receiver: ar.pinValue(n.First("target")), Args: ar.args(n)})
	case typeAcceptEventAction, typeAcceptCallAction:
		st := Statement{Kind: StmtAccept}
		for _, trig := range n.Tagged("trigger") {
			st.Events = append(st.Events, ar.r.readEvent(trig.Attr("event"), trig))
		}
		if res := n.First("result"); res != nil && len(ar.outgoing[res.ID]) > 0 {
			st.Result = res.Name()
		}
		ar.emit(st)
	case typeAddStructuralFeatureValueAction:
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
			Kind:      StmtAssign,
			Receiver:  ar.pinValue(n.First("object")),
			Feature:   feature.Name(),
			FeatureID: feature.ID,
			Value:     ar.pinValue(n.First("value")),
			Replace:   n.Attr("isReplaceAll") == "true",
		})
	case typeActivityParameterNode:
		// A fed output parameter node is the body's return statement.
		param := ar.r.doc.ByID(n.Attr("parameter"))
		if param == nil || len(ar.incoming[n.ID]) == 0 {
			return
		}
		if dir := param.Attr("direction"); dir == "return" || dir == "out" || dir == "inout" {
			ar.emit(Statement{Kind: StmtReturn, Feature: param.Attr("name"), Value: ar.pinValue(n)})
		}
	case typeInitialNode, typeActivityFinalNode, typeFlowFinalNode, typeForkNode, typeJoinNode,
		typeMergeNode, typeDecisionNode, typeExpansionNode:
	case typeValueSpecificationAction, typeReadSelfAction, typeReadStructuralFeatureAction,
		typeClearStructuralFeatureAction, typeTestIdentityAction, typeReadIsClassifiedObjectAction,
		typeCreateObjectAction:
		// Values: read where a pin consumes them. An unconsumed one is dead.
	case typeStartObjectBehaviorAction:
		ar.emit(Statement{Kind: StmtStart, Receiver: ar.pinValue(n.First("object"))})
	case typeDestroyObjectAction,
		typeReadExtentAction, typeStartClassifierBehaviorAction, typeReduceAction,
		typeRemoveStructuralFeatureValueAction, typeCreateLinkAction, typeDestroyLinkAction,
		typeReadLinkAction, typeClearAssociationAction, typeReclassifyObjectAction, typeUnmarshallAction:
		ar.unsupported(n, "manipulates objects or links")
	default:
		ar.unsupported(n, "is a node kind the reader does not know")
	}
}

func (ar *activityReader) emit(st Statement) {
	ar.body.Statements = append(ar.body.Statements, st)
}

// args reads a call or send action's argument pins in document order.
func (ar *activityReader) args(n *xmi.Element) []Expr {
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
func (ar *activityReader) behaviorName(n *xmi.Element) string {
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
func (ar *activityReader) pinValue(pin *xmi.Element) *Expr {
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
	case typeForkNode, typeMergeNode, typeExpansionNode, typeCentralBufferNode:
		return ar.passThrough(e)
	case typeActivityParameterNode:
		param := ar.r.doc.ByID(e.Attr("parameter"))
		if param == nil {
			return Expr{Kind: ExprUnknown, Text: e.Describe() + " names no parameter"}
		}
		if dir := param.Attr("direction"); dir == "in" || dir == "inout" || dir == "" {
			return Expr{Kind: ExprParam, Name: param.Name()}
		}
		return ar.passThrough(e)
	}
	if e.Tag == "structuredNodeInput" || e.Tag == "structuredNodeOutput" || e.Type == typeInputPin || e.Tag == "argument" || e.Tag == "object" || e.Tag == "target" || e.Tag == "value" {
		return ar.passThrough(e)
	}
	// An output pin: the value is what its owning action computes.
	owner := e.Parent
	if owner == nil {
		return Expr{Kind: ExprUnknown, Text: e.Describe() + " has no owner"}
	}
	return ar.actionValue(owner, e)
}

func (ar *activityReader) passThrough(e *xmi.Element) Expr {
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
func (ar *activityReader) actionValue(n, pin *xmi.Element) Expr {
	deref := func(p *Expr) *Expr {
		if p == nil {
			return &Expr{Kind: ExprUnknown, Text: n.Describe() + " reads an unfed pin"}
		}
		return p
	}
	switch n.Type {
	case typeValueSpecificationAction:
		lit, diag := readLiteral(n.First("value"))
		if diag != "" {
			return Expr{Kind: ExprUnknown, Text: diag}
		}
		return Expr{Kind: ExprLiteral, Literal: lit}
	case typeReadSelfAction:
		return Expr{Kind: ExprSelf}
	case typeCreateObjectAction:
		classifier := ar.r.doc.ByID(n.Attr("classifier"))
		if classifier == nil {
			return Expr{Kind: ExprUnknown, Text: n.Describe() + " creates an object of a classifier the document does not define"}
		}
		return Expr{Kind: ExprNew, Name: classifier.Name(), TypeID: classifier.ID, ID: n.ID}
	case typeReadStructuralFeatureAction:
		feature := ar.r.doc.ByID(n.Attr("structuralFeature"))
		if feature == nil {
			return Expr{Kind: ExprUnknown, Text: n.Describe() + " reads a feature the document does not define"}
		}
		return Expr{Kind: ExprRead, Name: feature.Name(), Object: deref(ar.pinValue(n.First("object")))}
	case typeClearStructuralFeatureAction:
		return *deref(ar.pinValue(n.First("object")))
	case typeCallBehaviorAction:
		return Expr{Kind: ExprApply, Name: ar.behaviorName(n), Library: ar.library(n), Args: ar.args(n)}
	case typeCallOperationAction:
		op := ar.r.doc.ByID(n.Attr("operation"))
		if op == nil {
			return Expr{Kind: ExprUnknown, Text: n.Describe() + " calls an operation the document does not define"}
		}
		result, ok := ar.resultParam(n, pin, op)
		if !ok {
			return Expr{Kind: ExprUnknown, Text: n.Describe() + " reads a result pin " + op.Name() + " has no output parameter for"}
		}
		return Expr{Kind: ExprCall, Name: op.Name(), OperationID: op.ID, Object: deref(ar.pinValue(n.First("target"))), Args: ar.args(n), Result: result, ID: n.ID}
	case typeTestIdentityAction:
		return Expr{Kind: ExprApply, Name: "==", Args: []Expr{*deref(ar.pinValue(n.First("first"))), *deref(ar.pinValue(n.First("second")))}}
	case typeAcceptEventAction, typeAcceptCallAction:
		return Expr{Kind: ExprEvent, Name: pin.Name()}
	case typeStructuredActivityNode, typeSequenceNode, typeExpansionRegion:
		return ar.passThrough(pin)
	}
	return Expr{Kind: ExprUnknown, Text: n.Describe() + " is a node kind the reader does not evaluate"}
}

// resultParam names the operation's output parameter a call's result pin
// carries: the pins correspond to the out, inout and return parameters in
// order (UML §16.3.3.1). It reports false when the pin is not among them.
func (ar *activityReader) resultParam(call, pin, op *xmi.Element) (string, bool) {
	for i, p := range call.Tagged("result") {
		if p != pin {
			continue
		}
		outputs := (&Operation{Params: ar.r.readParams(op)}).Outputs()
		if i >= len(outputs) {
			return "", false
		}
		return outputs[i].Name, true
	}
	return "", false
}

// library identifies the behavior a call behavior action applies when it is
// one of a library: a fUML or Alf primitive the document references by href,
// or an activity the suite's utility packages own. A class's own behavior is
// not one, and nil says so.
func (ar *activityReader) library(n *xmi.Element) *LibraryBehavior {
	if id := n.Attr("behavior"); id != "" {
		return packagedBehavior(ar.r.doc.ByID(id))
	}
	b := n.First("behavior")
	if b == nil {
		return nil
	}
	if href := b.Href(); href != "" {
		return primitiveBehavior(href)
	}
	return packagedBehavior(ar.r.doc.ByID(b.Attr("idref")))
}

// packagedBehavior is the behavior when packages alone own it, by the
// qualified name below the model; nil when a class owns it or it is absent.
func packagedBehavior(b *xmi.Element) *LibraryBehavior {
	if b == nil || b.Parent == nil || b.Parent.Type != "uml:Package" {
		return nil
	}
	var path []string
	for e := b; e != nil && e.Type != "uml:Model" && e.Tag != "Model"; e = e.Parent {
		path = append([]string{e.Name()}, path...)
	}
	return &LibraryBehavior{Name: b.Name(), Qualified: strings.Join(path, "::")}
}

// primitiveBehavior reads a standard-library reference such as
// fUML_Library.xmi#PrimitiveBehaviors-StringFunctions-Concat or
// Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-SequenceFunctions-Size as the
// primitive's qualified name below PrimitiveBehaviors.
func primitiveBehavior(href string) *LibraryBehavior {
	frag := href[strings.LastIndex(href, "#")+1:]
	segments := strings.Split(frag, "-")
	for i, s := range segments {
		if s == "PrimitiveBehaviors" {
			segments = segments[i+1:]
			break
		}
	}
	return &LibraryBehavior{Name: segments[len(segments)-1], Qualified: strings.Join(segments, "::")}
}

// readLiteral reads a literal specification element.
func readLiteral(v *xmi.Element) (*Literal, string) {
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
