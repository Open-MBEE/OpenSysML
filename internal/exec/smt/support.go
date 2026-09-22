package smt

import (
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// DefaultUnroll is how many iterations of a body loop the encoding unrolls when
// the engine is given no other bound.
const DefaultUnroll = analysis.DefaultUnroll

// MaxSlots bounds the token slots one encoding declares; a flow that may need
// more within k moves is refused with ErrSlotOverflow rather than encoded partially.
const MaxSlots = 32

// Flow is an action's lowered flow as the encoding numbers it: every node and
// every succession of every frame has a stable index, and the constructs the
// stage encodes have been checked for, so the encoder meets nothing it must refuse.
type Flow struct {
	// Graph is the lowered flow the interpreter runs; it stays the source of truth.
	Graph *lower.ActionGraph
	// Frames lists the root flow first, then each flow a node states of its own,
	// in the order the nodes are met; FrameOf gives the frame each node runs in.
	Frames  []*Frame
	FrameOf map[ast.Node]*Frame
	// Nodes lists every frame's nodes, frame by frame in graph order; Index inverts it.
	Nodes []ast.Node
	Index map[ast.Node]int
	// Labels names each node as a trace does, made unique by position where
	// two nodes share a name.
	Labels []string
	// Edges lists every succession, in the order of its source node then its
	// declaration; EdgeIndex inverts it. Incoming and Outgoing list a node's
	// successions by index.
	Edges     []lower.ActionEdge
	EdgeIndex map[lower.ActionEdge]int
	Incoming  map[ast.Node][]int
	Outgoing  map[ast.Node][]int
	// Loops lists the body loops the encoding unrolls, in the order their
	// statements are met walking the nodes.
	Loops []BodyLoop
	// Slots is how many tokens may be in flight at once within k moves, over
	// every frame: the bound T.
	Slots int
	// Cyclic is set when a fork lies on a cycle, so the tokens in flight are
	// bounded by k rather than by the graph, and the state records a full fork.
	Cyclic bool
	// Delivers is set when an object flow delivers to a node performing in a
	// frame of its own, whose pin queues the deliveries it has yet to take.
	Delivers bool
	// Sends lists the send statements the bodies run; Bus is how many messages
	// may sit on the bus at once within k moves: the bound M.
	Sends []SendSite
	Bus   int
	// Accepts lists the accept nodes; Timed is set when one waits on the clock.
	Accepts []AcceptSite
	Timed   bool
}

// Frame is one flow the encoding runs: the root, or the flow a node states of
// its own, run by that node's performance.
type Frame struct {
	// Index is the frame's position in Flow.Frames; the root is 0.
	Index int
	Graph *lower.ActionGraph
	// Node performs the frame's flow; nil for the root. Parent is its frame.
	Node   ast.Node
	Parent *Frame
	// Nodes lists the frame's own nodes, in graph order.
	Nodes []ast.Node
	// Slots is how many tokens the frame's flow may hold at once within k moves.
	Slots int
}

// SendSite is one send statement and the node whose body runs it.
type SendSite struct {
	Node  ast.Node
	Label string
	Send  lower.Send
}

// AcceptSite is one accept node and what it waits for.
type AcceptSite struct {
	Node   ast.Node
	Label  string
	Accept lower.Accept
}

// Nested reports whether any node states a flow of its own.
func (f *Flow) Nested() bool { return len(f.Frames) > 1 }

// path names a frame as its performing nodes chain, empty for the root.
func (fr *Frame) path() string {
	if fr.Parent == nil {
		return ""
	}
	return fr.Parent.path() + nodeLabel(fr.Node) + "/"
}

// BodyLoop is one body loop the encoding unrolls, and the node whose body it is in.
type BodyLoop struct {
	Node  ast.Node
	Label string
	Loop  lower.Loop
}

// Analyze numbers the flow for an encoding of k moves, refusing with a typed
// error a flow the stage does not encode or the interpreter would not run.
func Analyze(graph *lower.ActionGraph, k int) (*Flow, error) {
	if graph == nil {
		return nil, &FlowError{Reason: "no action flow"}
	}
	if k < 0 {
		return nil, fmt.Errorf("smt: a bound of %d moves", k)
	}
	if graph.Initial == nil {
		return nil, &FlowError{Reason: "the flow has no initial node"}
	}
	f := &Flow{
		Graph:     graph,
		FrameOf:   make(map[ast.Node]*Frame, len(graph.Nodes)),
		Index:     make(map[ast.Node]int, len(graph.Nodes)),
		EdgeIndex: make(map[lower.ActionEdge]int),
		Incoming:  make(map[ast.Node][]int),
		Outgoing:  make(map[ast.Node][]int),
	}
	if err := f.number(&Frame{Graph: graph}); err != nil {
		return nil, err
	}
	for _, fr := range f.Frames {
		if len(fr.Graph.Bindings) > 0 {
			return nil, &UnsupportedError{Node: nodeLabel(fr.Graph.Bindings[0].Node), Construct: "pin binding",
				Reason: "a binding connector at a pin is not encoded; object flows are"}
		}
	}
	for _, node := range f.Nodes {
		if err := f.checkNode(node); err != nil {
			return nil, err
		}
	}
	for _, fr := range f.Frames {
		f.sizeSlots(fr, k)
		f.Slots += fr.Slots
	}
	f.Bus = min(len(f.Sends), k)
	if f.Slots > 1 {
		for _, node := range f.Nodes {
			if err := f.checkImplicitJoin(node); err != nil {
				return nil, err
			}
		}
	}
	if f.Slots > MaxSlots {
		return nil, fmt.Errorf("%w: %d tokens may be in flight within %d moves, %d slots at most",
			ErrSlotOverflow, f.Slots, k, MaxSlots)
	}
	return f, nil
}

// number gives the frame's nodes and successions their indices, then those of
// each flow a node of it states of its own, so every frame's labels stay distinct.
func (f *Flow) number(fr *Frame) error {
	graph := fr.Graph
	if graph == nil || graph.Initial == nil {
		return &FlowError{Node: nodeLabel(fr.Node), Reason: "the flow has no initial node"}
	}
	fr.Index = len(f.Frames)
	f.Frames = append(f.Frames, fr)
	prefix := fr.path()
	first := len(f.Nodes)
	for _, node := range graph.Nodes {
		if _, seen := f.Index[node]; seen {
			continue
		}
		f.Index[node] = len(f.Nodes)
		f.FrameOf[node] = fr
		f.Nodes = append(f.Nodes, node)
		fr.Nodes = append(fr.Nodes, node)
	}
	if f.FrameOf[graph.Initial] != fr {
		return &FlowError{Node: nodeLabel(graph.Initial), Reason: "the initial node is not among the flow's nodes"}
	}
	for _, label := range uniqueLabels(fr.Nodes) {
		f.Labels = append(f.Labels, prefix+label)
	}
	for _, node := range fr.Nodes {
		for _, edge := range graph.Edges[node] {
			if _, seen := f.EdgeIndex[edge]; seen {
				continue
			}
			if f.FrameOf[edge.Target] != fr {
				return &FlowError{Node: f.label(node), Reason: "a succession leaves the flow's nodes"}
			}
			i := len(f.Edges)
			f.Edges = append(f.Edges, edge)
			f.EdgeIndex[edge] = i
			f.Outgoing[node] = append(f.Outgoing[node], i)
			f.Incoming[edge.Target] = append(f.Incoming[edge.Target], i)
		}
	}
	for _, node := range f.Nodes[first:] {
		sub := graph.Subflows[node]
		if sub == nil {
			continue
		}
		if sub.Err != nil || sub.Graph == nil || sub.Graph.Initial == nil {
			return f.refuseNested(node)
		}
		if err := f.number(&Frame{Graph: sub.Graph, Node: node, Parent: fr}); err != nil {
			return err
		}
	}
	return nil
}

// graphOf returns the lowered flow node runs in.
func (f *Flow) graphOf(node ast.Node) *lower.ActionGraph {
	if fr, ok := f.FrameOf[node]; ok {
		return fr.Graph
	}
	return f.Graph
}

// label names node as the encoding does.
func (f *Flow) label(node ast.Node) string {
	if i, ok := f.Index[node]; ok {
		return f.Labels[i]
	}
	return nodeLabel(node)
}

// checkNode refuses what the stage does not encode at node and what the
// interpreter would refuse to run there.
func (f *Flow) checkNode(node ast.Node) error {
	label := f.label(node)
	graph := f.FrameOf[node].Graph
	if graph.Subflows[node] != nil {
		return f.refuseNested(node)
	}
	if accept, ok := graph.Accepts[node]; ok {
		f.Accepts = append(f.Accepts, AcceptSite{Node: node, Label: label, Accept: accept})
		if _, timed := accept.Trigger.(*ast.TimeEvent); timed {
			f.Timed = true
		}
		return &UnsupportedError{Node: label, Construct: "accept", Reason: "messages and the clock are encoded by a later stage"}
	}
	if err := f.checkNodeKind(node, label); err != nil {
		return err
	}
	for _, flow := range graph.DataFlows[node] {
		if _, ok := f.Index[flow.Target]; !ok {
			return &FlowError{Node: label, Reason: "an object flow leaves the flow's nodes"}
		}
		if flow.SourcePin == "" || flow.TargetPin == "" {
			return &FlowError{Node: label, Reason: "an object flow names no pin at one end"}
		}
		if _, performs := flow.Target.(*ast.Usage); performs {
			f.Delivers = true
		}
	}
	return f.checkBody(node, label, graph.Bodies[node])
}

// refuseNested refuses a node stating a flow of its own, whatever that flow holds.
func (f *Flow) refuseNested(node ast.Node) error {
	return &UnsupportedError{Node: f.label(node), Construct: "nested flow", Reason: "a node stating a flow of its own is encoded by a later stage"}
}

// checkNodeKind refuses a node of a kind the stage does not encode, or with
// successors the interpreter would refuse.
func (f *Flow) checkNodeKind(node ast.Node, label string) error {
	out := len(f.Outgoing[node])
	switch n := node.(type) {
	case *ast.InitialNode:
		if out == 0 {
			return &FlowError{Node: label, Reason: "the initial node has no successors"}
		}
	case *ast.FinalNode, *ast.ForkNode, *ast.DecisionNode:
	case *ast.JoinNode:
		if out == 0 {
			return &FlowError{Node: label, Reason: "the join has no successors"}
		}
		if out > 1 {
			return &FlowError{Node: label, Reason: "the join has multiple successors"}
		}
	case *ast.MergeNode:
		if out == 0 {
			return &FlowError{Node: label, Reason: "the merge has no successors"}
		}
		if out > 1 {
			return &FlowError{Node: label, Reason: "the merge has multiple successors"}
		}
	case *ast.ActionExecutionNode:
		if n.ActionRef != nil {
			return &UnsupportedError{Node: label, Construct: "action invocation", Reason: "a node performing another action is encoded by a later stage"}
		}
		if out > 1 {
			return &FlowError{Node: label, Reason: "the action node has multiple successors"}
		}
	case *ast.Usage:
		if lower.IsCaseNode(n) {
			return &UnsupportedError{Node: label, Construct: "case", Reason: "a nested case is not encoded"}
		}
		if performsAction(n) {
			return &UnsupportedError{Node: label, Construct: "action invocation", Reason: "a node performing another action is encoded by a later stage"}
		}
		if out > 1 {
			return &FlowError{Node: label, Reason: "the action node has multiple successors"}
		}
	case *ast.WhileLoopActionNode, *ast.IfActionNode, *ast.AssignmentActionNode:
		if out > 1 {
			return &FlowError{Node: label, Reason: "the statement node has multiple successors"}
		}
	case *ast.SendStatement:
		return &UnsupportedError{Node: label, Construct: "send", Reason: "messages are encoded by a later stage"}
	case *ast.TerminateStatement:
		return &UnsupportedError{Node: label, Construct: "terminate", Reason: "terminating a performance is not encoded"}
	default:
		return &UnsupportedError{Node: label, Construct: fmt.Sprintf("%T", node), Reason: "the interpreter runs no such node"}
	}
	return nil
}

// checkImplicitJoin refuses a node other than a join or a merge that several
// successions enter while forks put several tokens in flight: the interpreter
// synchronizes it over the successions still reachable, which is not encoded.
func (f *Flow) checkImplicitJoin(node ast.Node) error {
	switch node.(type) {
	case *ast.JoinNode, *ast.MergeNode:
		return nil
	}
	if len(f.Incoming[node]) < 2 {
		return nil
	}
	return &UnsupportedError{Node: f.label(node), Construct: "implicit join",
		Reason: "a node several successions enter synchronizes over those still reachable while tokens run concurrently; only a join or a merge is encoded there"}
}

// checkBody refuses the statements of a body the stage does not encode, and
// records the loops it unrolls.
func (f *Flow) checkBody(node ast.Node, label string, body []lower.Statement) error {
	for _, stmt := range body {
		switch s := stmt.(type) {
		case lower.Assign:
			if s.Chain != nil {
				return &UnsupportedError{Node: label, Construct: "assignment through a chain", Reason: "objects and their features are encoded by a later stage"}
			}
			if s.Target == "" {
				return &UnsupportedError{Node: label, Construct: "assignment", Reason: "its target names no feature"}
			}
		case lower.Declare:
		case lower.DeclareUsage:
			return &UnsupportedError{Node: label, Construct: "usage declaration", Reason: "a body declaring a usage is not encoded"}
		case lower.Block:
			if err := f.checkBlock(node, label, s); err != nil {
				return err
			}
		case lower.If:
			if err := f.checkBlock(node, label, s.Then); err != nil {
				return err
			}
			if s.Else != nil {
				if err := f.checkBlock(node, label, *s.Else); err != nil {
					return err
				}
			}
		case lower.Loop:
			if s.Kind == ast.LoopFor {
				return &UnsupportedError{Node: label, Construct: "for loop", Reason: "iteration over a collection is not encoded"}
			}
			if s.Condition == nil && s.Until == nil {
				return &UnsupportedError{Node: label, Construct: "loop", Reason: "a loop with no condition ends only at the step budget"}
			}
			if err := f.checkBlock(node, label, s.Body); err != nil {
				return err
			}
			f.Loops = append(f.Loops, BodyLoop{Node: node, Label: label, Loop: s})
		case lower.Return:
			return &UnsupportedError{Node: label, Construct: "return", Reason: "an action node computes no result to return"}
		case lower.Send:
			f.Sends = append(f.Sends, SendSite{Node: node, Label: label, Send: s})
			return &UnsupportedError{Node: label, Construct: "send", Reason: "messages are encoded by a later stage"}
		case lower.Effect:
			return &UnsupportedError{Node: label, Construct: effectName(s.Kind), Reason: "an effect on the world outside the body is not encoded"}
		case lower.Unsupported:
			return &UnsupportedError{Node: label, Construct: s.Description, Reason: "the interpreter cannot execute it"}
		default:
			return &UnsupportedError{Node: label, Construct: fmt.Sprintf("%T", stmt), Reason: "the interpreter runs no such statement"}
		}
	}
	return nil
}

// checkBlock refuses a block that runs a flow of its own and checks its statements.
func (f *Flow) checkBlock(node ast.Node, label string, block lower.Block) error {
	if block.Graph != nil {
		return &UnsupportedError{Node: label, Construct: "nested flow", Reason: "a block declaring action nodes is encoded by a later stage"}
	}
	return f.checkBody(node, label, block.Statements)
}

// sizeSlots decides how many tokens the frame's flow may hold at once within k
// moves: one, plus what each fork adds per time a token reaches it, at most k times.
func (f *Flow) sizeSlots(fr *Frame, k int) {
	arrivals := f.arrivals(fr, k)
	slots, widest := 1, 0
	for _, node := range fr.Nodes {
		fork, ok := node.(*ast.ForkNode)
		if !ok {
			continue
		}
		extra := len(f.Outgoing[fork]) - 1
		if extra <= 0 {
			continue
		}
		if f.reaches(fork, fork) {
			f.Cyclic = true
		}
		widest = max(widest, extra)
		slots += min(arrivals[fork], k) * extra
	}
	// Each of the k moves performs at most one fork.
	fr.Slots = min(slots, 1+k*widest)
}

// arrivals bounds how often a token may reach each node of the frame within k
// moves: a fork or decision passes on all that reach it, a merge sums them, a
// join passes on the most over one succession, and a cycle multiplying tokens
// passes on k.
func (f *Flow) arrivals(fr *Frame, k int) map[ast.Node]int {
	reached := make(map[ast.Node]int, len(fr.Nodes))
	leaving := make(map[ast.Node]int, len(fr.Nodes))
	for _, comp := range f.components(fr) {
		cyclic := len(comp) > 1 || f.reaches(comp[0], comp[0])
		entering, multiplies := f.entering(fr, comp, cyclic, leaving)
		entering = min(entering, k)
		if cyclic {
			// A token entering the cycle leaves it once at most, unless the
			// cycle forks it, when k moves bound what leaves.
			if multiplies {
				entering = k
			}
			for _, node := range comp {
				reached[node], leaving[node] = entering, entering
			}
			continue
		}
		node := comp[0]
		if _, ok := node.(*ast.JoinNode); ok {
			entering = 0
			for _, ei := range f.Incoming[node] {
				entering = max(entering, leaving[f.Edges[ei].Source])
			}
		}
		reached[node], leaving[node] = entering, entering
	}
	return reached
}

// entering is how many tokens may enter the component comp from outside it, given
// how many leave each node before it, and whether a fork inside a cycle multiplies them.
func (f *Flow) entering(fr *Frame, comp []ast.Node, cyclic bool, leaving map[ast.Node]int) (entering int, multiplies bool) {
	inside := make(map[ast.Node]bool, len(comp))
	for _, node := range comp {
		inside[node] = true
	}
	for _, node := range comp {
		if node == fr.Graph.Initial {
			entering++
		}
		for _, ei := range f.Incoming[node] {
			if source := f.Edges[ei].Source; !inside[source] {
				entering += leaving[source]
			}
		}
		if _, ok := node.(*ast.ForkNode); ok && cyclic && len(f.Outgoing[node]) > 1 {
			multiplies = true
		}
	}
	return entering, multiplies
}

// components are the frame's strongly connected components, each before any
// the successions lead to from it.
func (f *Flow) components(fr *Frame) [][]ast.Node {
	index := make(map[ast.Node]int, len(fr.Nodes))
	low := make(map[ast.Node]int, len(fr.Nodes))
	onStack := make(map[ast.Node]bool, len(fr.Nodes))
	var stack []ast.Node
	var comps [][]ast.Node
	next := 0
	var visit func(node ast.Node)
	visit = func(node ast.Node) {
		index[node], low[node] = next, next
		next++
		stack = append(stack, node)
		onStack[node] = true
		for _, ei := range f.Outgoing[node] {
			target := f.Edges[ei].Target
			if _, seen := index[target]; !seen {
				visit(target)
				low[node] = min(low[node], low[target])
			} else if onStack[target] {
				low[node] = min(low[node], index[target])
			}
		}
		if low[node] != index[node] {
			return
		}
		var comp []ast.Node
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			comp = append(comp, top)
			if top == node {
				break
			}
		}
		comps = append(comps, comp)
	}
	for _, node := range fr.Nodes {
		if _, seen := index[node]; !seen {
			visit(node)
		}
	}
	slices.Reverse(comps)
	return comps
}

// reaches reports whether a token at from can arrive at to over the successions.
func (f *Flow) reaches(from, to ast.Node) bool {
	seen := make(map[ast.Node]bool)
	stack := []ast.Node{}
	for _, edge := range f.Outgoing[from] {
		stack = append(stack, f.Edges[edge].Target)
	}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == to {
			return true
		}
		if seen[node] {
			continue
		}
		seen[node] = true
		for _, edge := range f.Outgoing[node] {
			stack = append(stack, f.Edges[edge].Target)
		}
	}
	return false
}

// performsAction reports whether a usage node performs another action rather
// than stating its own body, as the interpreter decides it.
func performsAction(usage *ast.Usage) bool {
	if usage.PerformedInvocation() != nil {
		return true
	}
	for _, rel := range usage.Relationships {
		if rel.Kind != ast.RelTyping && rel.Kind != ast.RelReferences {
			continue
		}
		if _, ok := rel.Target.(*ast.QualifiedName); ok {
			return true
		}
	}
	return false
}

// effectName names an effect kind as a body writes it.
func effectName(kind lower.EffectKind) string {
	switch kind {
	case lower.EffectPerform:
		return "perform"
	case lower.EffectAccept:
		return "accept"
	case lower.EffectTerminate:
		return "terminate"
	case lower.EffectStart:
		return "start"
	}
	return fmt.Sprintf("effect %d", kind)
}

// nodeLabel names a node as a trace names it: by its name, else by what it does.
func nodeLabel(node ast.Node) string {
	switch n := node.(type) {
	case nil:
		return "nil"
	case *ast.InitialNode:
		return controlLabel(n.Name(), "initial")
	case *ast.FinalNode:
		return "done"
	case *ast.ForkNode:
		return controlLabel(n.Name, "fork")
	case *ast.JoinNode:
		return controlLabel(n.Name, "join")
	case *ast.MergeNode:
		return controlLabel(n.Name, "merge")
	case *ast.DecisionNode:
		return controlLabel(n.Name, "decision")
	case *ast.ActionExecutionNode:
		return controlLabel(n.Name, "action")
	case *ast.Usage:
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return fmt.Sprintf("usage_%s", n.Kind)
	}
	return fmt.Sprintf("%T", node)
}

func controlLabel(name, kind string) string {
	if name != "" {
		return name
	}
	return kind
}

// uniqueLabels labels the nodes as traces do, suffixing a repeated name with the
// node's position so every constructor of the node sort is distinct.
func uniqueLabels(nodes []ast.Node) []string {
	labels := make([]string, len(nodes))
	count := make(map[string]int, len(nodes))
	for i, node := range nodes {
		labels[i] = nodeLabel(node)
		count[labels[i]]++
	}
	for i, label := range labels {
		if count[label] > 1 {
			labels[i] = fmt.Sprintf("%s#%d", label, i)
		}
	}
	return labels
}
